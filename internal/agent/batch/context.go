package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"strings"
)

// prepareCall rereads authority, prompts and published workflows at every model
// boundary. Only version/hash metadata and public layers leave this function.
func (e *Executor) prepareCall(j *journal, topic uuid.UUID, newMaterial string) func(context.Context) (runner.CallContext, error) {
	return func(ctx context.Context) (runner.CallContext, error) {
		var out runner.CallContext
		tx, err := e.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return out, err
		}
		defer tx.Rollback(ctx)
		facts := agentcontext.Facts{ProjectID: j.project, NewMaterial: newMaterial}
		var identity uuid.UUID
		var binding *int64
		var kind string
		err = tx.QueryRow(ctx, `SELECT i.id,r.binding_version,i.kind FROM agent_runs r JOIN agent_identities i ON i.id=r.identity_id JOIN projects p ON p.id=r.project_id WHERE r.id=$1 AND r.project_id=$2 AND r.state='running' AND r.lease_token=$3 AND i.status='active' AND p.status='active' AND (i.kind='coordinator' OR i.current_binding_version=r.binding_version)`, j.run, j.project, j.lease).Scan(&identity, &binding, &kind)
		if err != nil {
			return out, apierrors.New(apierrors.Forbidden, "run authority or binding changed")
		}
		if kind == "coordinator" {
			facts.PositionPrompt = "你是项目协调者。依据已发布流程与公开职责调用岗位，保留反对、缺料和未完成事项；不得替人批准或验收。"
		} else {
			facts.IdentityID = &identity
			facts.BindingVersion = binding
			err = tx.QueryRow(ctx, `SELECT v.prompt,t.current_version,COALESCE(pref.prompt,''),COALESCE(pref.revision,0) FROM agent_identities i JOIN position_templates t ON t.id=i.template_id JOIN position_versions v ON v.template_id=t.id AND v.revision=t.current_version JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active' JOIN users u ON u.id=b.user_id AND u.status='active' LEFT JOIN personal_project_preferences pref ON pref.project_id=i.project_id AND pref.user_id=b.user_id WHERE i.id=$1 AND i.project_id=$2 AND i.current_binding_version=$3`, identity, j.project, binding).Scan(&facts.PositionPrompt, &facts.PositionVersion, &facts.PreferencePrompt, &facts.PreferenceRevision)
			if err != nil {
				return out, apierrors.New(apierrors.Forbidden, "current position holder required")
			}
		}
		// Freeze the source message cutoff; a newer submission is a separate batch.
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT max(m.seq) FROM messages m WHERE m.submission_id=b.source_submission_id AND m.project_id=b.project_id),0) FROM discussion_batches b WHERE b.id=$1 AND b.project_id=$2`, j.batch, j.project).Scan(&facts.CoveredSeq); err != nil {
			return out, err
		}
		rows, err := tx.Query(ctx, `SELECT seq,kind,content FROM messages WHERE topic_id=$1 AND project_id=$2 AND seq<=$3 AND state='committed' ORDER BY seq`, topic, j.project, facts.CoveredSeq)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var seq int64
			var kind, text string
			if err = rows.Scan(&seq, &kind, &text); err != nil {
				rows.Close()
				return out, err
			}
			facts.RecentMessages = append(facts.RecentMessages, fmt.Sprintf("message:%s:%d [%s] %s", topic, seq, kind, text))
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return out, err
		}
		versions := map[string]string{}
		rows, err = tx.Query(ctx, `SELECT d.id,v.id,v.instructions,v.nodes_json,v.hard_rules_json,v.approval_policies_json FROM workflow_definitions d JOIN workflow_versions v ON v.id=d.published_version_id WHERE d.project_id=$1 ORDER BY d.id`, j.project)
		if err != nil {
			return out, err
		}
		workflows := []string{}
		for rows.Next() {
			var id, version uuid.UUID
			var instructions string
			var nodes, rules, policies []byte
			if err = rows.Scan(&id, &version, &instructions, &nodes, &rules, &policies); err != nil {
				rows.Close()
				return out, err
			}
			versions[id.String()] = version.String()
			workflows = append(workflows, fmt.Sprintf("workflow=%s version=%s\n%s\nnodes=%s rules=%s policies=%s", id, version, instructions, nodes, rules, policies))
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return out, err
		}
		facts.WorkflowPrompt = strings.Join(workflows, "\n")
		rows, err = tx.Query(ctx, `SELECT p.id,p.status,COALESCE(p.reason,''),COALESCE(v.review_hash,''),COALESCE(v.changes_json,'[]'::jsonb) FROM proposals p LEFT JOIN proposal_versions v ON v.id=p.current_review_id WHERE p.project_id=$1 AND (p.topic_id=$2 OR p.topic_id IS NULL) AND p.status IN ('pending','cancelled','stale') ORDER BY p.id`, j.project, topic)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var id uuid.UUID
			var state, reason, hash string
			var changes []byte
			if err = rows.Scan(&id, &state, &reason, &hash, &changes); err != nil {
				rows.Close()
				return out, err
			}
			facts.Disagreements = append(facts.Disagreements, fmt.Sprintf("proposal=%s state=%s reason=%s reviewHash=%s changes=%s", id, state, reason, hash, changes))
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return out, err
		}
		rows, err = tx.Query(ctx, `SELECT d.reason,d.decision,d.review_id FROM approval_decisions d JOIN proposals p ON p.current_review_id=d.review_id WHERE p.project_id=$1 AND (p.topic_id=$2 OR p.topic_id IS NULL) AND d.decision='reject' ORDER BY d.id`, j.project, topic)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var reason *string
			var decision string
			var review uuid.UUID
			if err = rows.Scan(&reason, &decision, &review); err != nil {
				rows.Close()
				return out, err
			}
			if reason != nil {
				facts.Disagreements = append(facts.Disagreements, fmt.Sprintf("review=%s rejected: %s", review, *reason))
			}
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return out, err
		}
		if err = tx.Commit(ctx); err != nil {
			return out, err
		}
		facts.WorkFacts = e.gatherFacts(ctx, j.project, topic).WorkFacts
		facts.WorkFacts = append(facts.WorkFacts, e.materialFacts(ctx, j.project, j.batch)...)
		provider, name, maxIn, maxOut, err := e.resolve(ctx, j.project, identity)
		if err != nil {
			return out, err
		}
		manifest := agentcontext.Build(facts)
		metadata := map[string]any{"schemaVersion": 1, "identityId": identity, "bindingVersion": binding, "positionVersion": facts.PositionVersion, "preferenceRevision": facts.PreferenceRevision, "preferenceHash": manifest.PreferenceHash, "workflowVersions": versions, "coveredSeq": facts.CoveredSeq, "model": name, "maxInputTokens": maxIn, "maxOutputTokens": maxOut, "layers": manifest.SharedLayers()}
		raw, err := json.Marshal(metadata)
		if err != nil {
			return out, err
		}
		if err = j.transaction(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `UPDATE agent_runs SET manifest=$2 WHERE id=$1`, j.run, raw)
			return err
		}); err != nil {
			return out, err
		}
		j.coveredSeq = facts.CoveredSeq
		j.contextMetadata = metadata
		return runner.CallContext{Manifest: manifest, Provider: provider, ModelName: name, MaxInputTokens: maxIn, MaxOutputTokens: maxOut}, nil
	}
}
