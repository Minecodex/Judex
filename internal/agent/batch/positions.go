package batch

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"sync"
	"time"
)

func (e *Executor) resolve(ctx context.Context, project, identity uuid.UUID) (model.Provider, string, int64, int64, error) {
	if e.ResolveModel != nil {
		return e.ResolveModel(ctx, project, identity)
	}
	if e.Provider == nil {
		return nil, "", 0, 0, fmt.Errorf("project model not configured")
	}
	return e.Provider, e.ModelName, 128000, 8192, nil
}
func (e *Executor) watchCancellation(ctx context.Context, cancel context.CancelFunc, project, run uuid.UUID, lease string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var valid bool
			err := e.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_runs r JOIN projects p ON p.id=r.project_id JOIN agent_identities i ON i.id=r.identity_id
    WHERE r.id=$1 AND r.project_id=$2 AND r.state='running' AND r.lease_token=$3 AND p.status='active' AND i.status='active'
    AND (i.kind='coordinator' OR i.current_binding_version=r.binding_version))`, run, project, lease).Scan(&valid)
			if err != nil || !valid {
				cancel()
				return
			}
		}
	}
}

type positionCaller struct {
	mu                            sync.Mutex
	failures                      []string
	localSessions                 sync.Map
	executor                      *Executor
	project, topic, batch, parent uuid.UUID
}

func (c *positionCaller) CallPositionAgent(ctx context.Context, project, position, question, material string) (answer string, callErr error) {
	defer func() {
		if callErr != nil {
			c.mu.Lock()
			c.failures = append(c.failures, position+": "+callErr.Error())
			c.mu.Unlock()
		}
	}()
	if project != c.project.String() {
		return "", fmt.Errorf("cross-project delegation rejected")
	}
	e := c.executor
	rows, err := e.Pool.Query(ctx, `SELECT i.id,i.current_binding_version,v.prompt,COALESCE(pref.prompt,'')
  FROM agent_identities i JOIN position_templates t ON t.id=i.template_id
  JOIN position_versions v ON v.template_id=t.id AND v.revision=t.current_version
  JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL
  JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active'
  LEFT JOIN personal_project_preferences pref ON pref.project_id=i.project_id AND pref.user_id=b.user_id
  WHERE i.project_id=$1 AND i.status='active' AND (i.id::text=$2 OR t.name=$2)`, c.project, position)
	if err != nil {
		return "", err
	}
	var identity uuid.UUID
	var binding int64
	var prompt, preference string
	count := 0
	for rows.Next() {
		if err = rows.Scan(&identity, &binding, &prompt, &preference); err != nil {
			rows.Close()
			return "", err
		}
		count++
	}
	rows.Close()
	if count != 1 {
		return "", fmt.Errorf("position must resolve to one active identity; use identityId")
	}
	provider, name, maxIn, maxOut, err := e.resolve(ctx, c.project, identity)
	if err != nil {
		return "", err
	}
	session, err := e.ensureSession(ctx, c.project, c.topic, identity)
	if err != nil {
		return "", err
	}
	gateAny, _ := c.localSessions.LoadOrStore(identity.String(), make(chan struct{}, 1))
	gate := gateAny.(chan struct{})
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	run, lease := uuid.New(), uuid.NewString()
	err = e.withProjectTx(ctx, c.project, func(tx pgx.Tx) error {
		if err := claimSession(ctx, tx, session, run); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,parent_run_id,identity_id,binding_version,state,lease_token,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,'running',$8,now(),now())`, c.project, run, session, c.batch, c.parent, identity, binding, lease)
		return err
	})
	if err != nil {
		return "", err
	}
	defer e.releaseSession(session, run)
	registry := tools.New()
	tools.RegisterDefaults(registry) // no recursive delegation
	env := e.toolEnv(c.project, c.topic)
	sandbox := e.newRunSandbox(run, c.project)
	env.Sandbox = sandbox
	defer sandbox.Close(context.WithoutCancel(ctx))
	facts := e.gatherFacts(ctx, c.project, c.topic)
	facts.IdentityID = &identity
	facts.BindingVersion = &binding
	facts.PositionPrompt = prompt
	facts.PreferencePrompt = preference
	facts.NewMaterial = question + "\n" + material
	facts.WorkFacts = append(facts.WorkFacts, e.materialFacts(ctx, c.project, c.batch)...)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go e.watchCancellation(childCtx, cancel, c.project, run, lease)
	harness := runner.Runner{Provider: provider, Registry: registry, Clock: e.Clock}
	journal := &journal{pool: e.Pool, project: c.project, run: run, session: session, batch: c.batch, lease: lease}
	outcome := harness.Run(childCtx, runner.RunRequest{Lease: lease, RunID: run, SessionID: session, ProjectID: c.project, IdentityID: identity, ModelName: name, Manifest: agentcontext.Build(facts),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 8, MaxTotalTokens: 100000, MaxWallClock: 5 * time.Minute}, MaxInputTokens: maxIn, MaxOutputTokens: maxOut, Env: &env,
		Journal: journal, Prepare: e.prepareCall(journal, c.topic, question+"\n"+material),
	})
	state := outcome.State
	if state == "context_blocked" {
		state = "waiting_human"
	}
	_, err = e.Pool.Exec(context.WithoutCancel(ctx), `UPDATE agent_runs SET state=$2,budget_snapshot=$3,version=version+1,updated_at=now() WHERE id=$1 AND lease_token=$4 AND state='running'`, run, state, runBudgetJSON(outcome), lease)
	if err != nil {
		return "", err
	}
	if state != "succeeded" {
		return "", fmt.Errorf("position %s did not complete: %s", identity, outcome.Summary)
	}
	if _, err = e.Pool.Exec(ctx, `UPDATE agent_sessions SET last_consumed_seq=GREATEST(last_consumed_seq,$2) WHERE id=$1`, session, journal.coveredSeq); err != nil {
		return "", err
	}
	return outcome.Summary, nil
}
