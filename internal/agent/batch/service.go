package batch

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/job"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"time"
)

type Service struct{ Pool *postgres.Pool }
type Run struct {
	ID           uuid.UUID       `json:"id"`
	BatchID      uuid.UUID       `json:"batchId"`
	IdentityID   uuid.UUID       `json:"identityId"`
	State        string          `json:"state"`
	Version      int64           `json:"version"`
	Budget       json.RawMessage `json:"budget"`
	WaitingFor   []string        `json:"waitingFor"`
	ArtifactRefs []any           `json:"artifactRefs"`
	CreatedAt    time.Time       `json:"createdAt"`
}

func (s *Service) Membership(ctx context.Context, user, project uuid.UUID) (string, error) {
	var role string
	err := s.Pool.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`, project, user).Scan(&role)
	if err != nil {
		return "", apierrors.New(apierrors.NotFound, "project not found")
	}
	return role, nil
}
func (s *Service) Start(ctx context.Context, user, project, topic, source uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var batchID, runID uuid.UUID
	err := s.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if _, err := s.Membership(postgres.WithTransaction(ctx, s.Pool, tx.Tx), user, project); err != nil {
			return err
		}
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM submissions WHERE id=$1 AND project_id=$2 AND topic_id=$3 AND status='ready')`, source, project, topic).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return apierrors.New(apierrors.InvalidReference, "ready submission in this topic required")
		}
		var rounds int
		if err := tx.QueryRow(ctx, `SELECT max_discussion_rounds FROM projects WHERE id=$1`, project).Scan(&rounds); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO discussion_batches(project_id,id,topic_id,source_submission_id,policy_snapshot,max_rounds,state,trigger_kind,created_at,updated_at)
   VALUES($1,$2,$3,$4,jsonb_build_object('maxRounds',$5::int),$5,'queued','submission',now(),now())
   ON CONFLICT(source_submission_id,trigger_kind) DO UPDATE SET source_submission_id=EXCLUDED.source_submission_id RETURNING id`, project, uuid.New(), topic, source, rounds).Scan(&batchID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT id FROM agent_runs WHERE batch_id=$1 ORDER BY created_at DESC LIMIT 1`, batchID).Scan(&runID); err == nil {
			return nil
		}
		var identityID, sessionID uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM agent_identities WHERE project_id=$1 AND kind='coordinator' AND status='active'`, project).Scan(&identityID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO agent_sessions(project_id,id,topic_id,identity_id,created_at) VALUES($1,$2,$3,$4,now())
   ON CONFLICT(topic_id,identity_id) DO UPDATE SET identity_id=EXCLUDED.identity_id RETURNING id`, project, uuid.New(), topic, identityID).Scan(&sessionID); err != nil {
			return err
		}
		runID = uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,identity_id,state,created_at,updated_at,requested_by) VALUES($1,$2,$3,$4,$5,'queued',now(),now(),$6)`, project, runID, sessionID, batchID, identityID, user); err != nil {
			return err
		}
		unique := "batch:" + batchID.String()
		_, err := job.Enqueue(ctx, tx, "discussion.batch", map[string]string{"projectId": project.String(), "batchId": batchID.String()}, &unique, time.Now().UTC(), time.Now().UTC())
		return err
	})
	return batchID, runID, err
}
func (s *Service) Get(ctx context.Context, user, project, run uuid.UUID) (Run, error) {
	if _, err := s.Membership(ctx, user, project); err != nil {
		return Run{}, err
	}
	out := Run{WaitingFor: []string{}, ArtifactRefs: []any{}}
	err := s.Pool.QueryRow(ctx, `SELECT id,batch_id,identity_id,state,version,budget_snapshot,created_at FROM agent_runs WHERE project_id=$1 AND id=$2`, project, run).Scan(&out.ID, &out.BatchID, &out.IdentityID, &out.State, &out.Version, &out.Budget, &out.CreatedAt)
	if err != nil {
		return out, apierrors.New(apierrors.NotFound, "run not found")
	}
	var budget map[string]any
	_ = json.Unmarshal(out.Budget, &budget)
	if reason, ok := budget["reason"].(string); ok && reason != "" && out.State != "succeeded" {
		out.WaitingFor = append(out.WaitingFor, reason)
	}
	rows, err := s.Pool.Query(ctx, `SELECT v.id,v.material_id,v.revision,v.sha256,m.title FROM material_versions v JOIN materials m ON m.id=v.material_id WHERE v.project_id=$1 AND v.state='ready' AND v.run_id IN(SELECT id FROM agent_runs WHERE id=$2 OR parent_run_id=$2) ORDER BY v.created_at,v.id`, project, run)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var version, material uuid.UUID
		var revision int64
		var sha, title string
		if err = rows.Scan(&version, &material, &revision, &sha, &title); err != nil {
			return out, err
		}
		out.ArtifactRefs = append(out.ArtifactRefs, map[string]any{"materialId": material, "versionId": version, "revision": revision, "sha256": sha, "title": title})
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}
func (s *Service) Cancel(ctx context.Context, user, project, run uuid.UUID, version int64, reason string) error {
	if reason == "" {
		return apierrors.Fields("reason", "required")
	}
	return s.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		if _, err := s.Membership(postgres.WithTransaction(ctx, s.Pool, tx.Tx), user, project); err != nil {
			return err
		}
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_runs r JOIN project_members m ON m.project_id=r.project_id AND m.user_id=$1 AND m.state='active'
    LEFT JOIN agent_identities i ON i.id=r.identity_id LEFT JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version
    WHERE r.id=$2 AND r.project_id=$3 AND (m.role IN ('owner','manager') OR r.requested_by=$1 OR b.user_id=$1))`, user, run, project).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return apierrors.New(apierrors.Forbidden, "only requester, holder or manager can cancel this run")
		}
		tag, err := tx.Exec(ctx, `UPDATE agent_runs SET state='cancelled',version=version+1,updated_at=now() WHERE project_id=$1 AND id=$2 AND version=$3 AND state NOT IN ('succeeded','failed','cancelled')`, project, run, version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.VersionConflict, "run changed or terminal")
		}
		_, err = tx.Exec(ctx, `UPDATE discussion_batches SET state='cancelled',version=version+1,updated_at=now() WHERE id=(SELECT batch_id FROM agent_runs WHERE id=$1)`, run)
		return err
	})
}
func (s *Service) Extend(ctx context.Context, user, project, batch uuid.UUID, version int64, rounds int, reason string) error {
	if rounds < 1 || rounds > 100 || reason == "" {
		return apierrors.Fields("extraRounds/reason", "invalid")
	}
	return s.Pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.String()); err != nil {
			return err
		}
		role, err := s.Membership(postgres.WithTransaction(ctx, s.Pool, tx.Tx), user, project)
		if err != nil {
			return err
		}
		if role != "owner" && role != "manager" {
			return apierrors.New(apierrors.Forbidden, "manager required")
		}

		var state string
		var current int64
		var topic, identity, session uuid.UUID
		if err = tx.QueryRow(ctx, `SELECT state,version,topic_id FROM discussion_batches WHERE id=$1 AND project_id=$2 FOR UPDATE`, batch, project).Scan(&state, &current, &topic); err != nil {
			return apierrors.New(apierrors.NotFound, "batch not found")
		}
		if current != version {
			return apierrors.New(apierrors.VersionConflict, "batch changed")
		}
		if state == "running" || state == "queued" || state == "cancelled" || state == "failed" {
			return apierrors.New(apierrors.InvalidTransition, "only a stopped discussion can be continued")
		}
		var unknown bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tool_calls WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id=$1) AND state IN ('prepared','running','unknown'))`, batch).Scan(&unknown); err != nil {
			return err
		}
		if unknown {
			return apierrors.New(apierrors.RequirementUnmet, "reconcile unknown tool results before continuing")
		}
		tag, err := tx.Exec(ctx, `UPDATE discussion_batches SET max_rounds=max_rounds+$3,version=version+1,state='queued',updated_at=now() WHERE project_id=$1 AND id=$2 AND max_rounds+$3<=100 AND model_attempts<$4 AND used_tokens+reserved_tokens<$5`, project, batch, rounds, runner.BatchMaxModelAttempts, runner.BatchMaxTotalTokens)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apierrors.New(apierrors.BudgetExhausted, "shared batch budget or round cap exhausted")
		}
		if err = tx.QueryRow(ctx, `SELECT identity_id,session_id FROM agent_runs WHERE batch_id=$1 AND parent_run_id IS NULL ORDER BY created_at DESC LIMIT 1`, batch).Scan(&identity, &session); err != nil {
			return err
		}
		run := uuid.New()
		if _, err = tx.Exec(ctx, `INSERT INTO agent_runs(project_id,id,session_id,batch_id,identity_id,state,requested_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'queued',$6,now(),now())`, project, run, session, batch, identity, user); err != nil {
			return err
		}
		unique := "extension:" + run.String()
		now := time.Now().UTC()
		if _, err = job.Enqueue(ctx, tx, "discussion.batch", map[string]any{"projectId": project, "batchId": batch}, &unique, now, now); err != nil {
			return err
		}
		if _, err = events.AppendProjectEvent(ctx, tx, project, "batch.extended", "discussion_batch", batch.String(), nil, map[string]any{"extraRounds": rounds, "runId": run, "reason": reason}, now); err != nil {
			return err
		}
		return audit.Append(ctx, tx, audit.Entry{ProjectID: &project, ActorType: audit.ActorUser, ActorUserID: &user, Source: audit.SourceWeb, Operation: "batch.extend", ObjectType: "discussion_batch", ObjectID: batch.String(), Reason: &reason, OccurredAt: now})

	})
}
