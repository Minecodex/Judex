package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/collaboration"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/workflow"
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
		var canManage bool
		err = tx.QueryRow(ctx, `SELECT i.id,r.binding_version,i.kind,EXISTS(SELECT 1 FROM project_members m WHERE m.project_id=r.project_id AND m.user_id=r.requested_by AND m.state='active' AND m.role IN ('owner','manager')) FROM agent_runs r JOIN agent_identities i ON i.id=r.identity_id JOIN projects p ON p.id=r.project_id WHERE r.id=$1 AND r.project_id=$2 AND r.state='running' AND r.lease_token=$3 AND i.status='active' AND p.status='active' AND (i.kind='coordinator' OR i.current_binding_version=r.binding_version)
 AND (r.requested_by IS NULL OR EXISTS(SELECT 1 FROM project_members m JOIN users u ON u.id=m.user_id WHERE m.project_id=r.project_id AND m.user_id=r.requested_by AND m.state='active' AND u.status='active'))`, j.run, j.project, j.lease).Scan(&identity, &binding, &kind, &canManage)
		if err != nil {
			return out, apierrors.New(apierrors.Forbidden, "run authority or binding changed")
		}
		if kind == "coordinator" {
			facts.PositionPrompt = "你是项目协调者。依据已发布流程与公开职责调用岗位，保留反对、缺料和未完成事项；不得替人批准或验收。继承聊天只作背景，旧判断不得当作当前业务状态；当前正式工作事实与决定为准，不复述私有提示词或个人偏好。"
		} else {
			facts.IdentityID = &identity
			facts.BindingVersion = binding
			err = tx.QueryRow(ctx, `SELECT v.prompt,t.current_version,COALESCE(pref.prompt,''),COALESCE(pref.revision,0) FROM agent_identities i JOIN position_templates t ON t.id=i.template_id AND t.project_id=i.project_id AND t.status='active' JOIN position_versions v ON v.template_id=t.id AND v.revision=t.current_version JOIN identity_bindings b ON b.identity_id=i.id AND b.binding_version=i.current_binding_version AND b.valid_until IS NULL JOIN project_members m ON m.project_id=i.project_id AND m.user_id=b.user_id AND m.state='active' JOIN users u ON u.id=b.user_id AND u.status='active' LEFT JOIN personal_position_preferences pref ON pref.project_id=i.project_id AND pref.user_id=b.user_id AND pref.position_id=i.template_id WHERE i.id=$1 AND i.project_id=$2 AND i.current_binding_version=$3`, identity, j.project, binding).Scan(&facts.PositionPrompt, &facts.PositionVersion, &facts.PreferencePrompt, &facts.PreferenceRevision)
			if err != nil {
				return out, apierrors.New(apierrors.Forbidden, "current position holder required")
			}
		}
		// Freeze the source message cutoff; a newer submission is a separate batch.
		if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT max(m.seq) FROM messages m WHERE m.submission_id=COALESCE(b.source_submission_id,(SELECT submission_id FROM work_reports WHERE id=b.source_report_id)) AND m.project_id=b.project_id),0) FROM discussion_batches b WHERE b.id=$1 AND b.project_id=$2`, j.batch, j.project).Scan(&facts.CoveredSeq); err != nil {
			return out, err
		}
		scopeTask, err := e.prepareScope(ctx, tx, j.project, topic, j.batch, &facts)
		if err != nil {
			return out, err
		}
		scope, err := collaboration.ReadAnalysisScope(ctx, tx, j.project, j.batch)
		if err != nil {
			return out, err
		}
		if scopeTask == nil && scope.PlanID == nil {
			facts.WorkFacts = e.gatherFacts(ctx, j.project, topic).WorkFacts
		}
		var rows pgx.Rows
		versions := map[string]string{}
		candidateIdentity := identity
		if kind == "coordinator" && scope.IdentityID != nil {
			candidateIdentity = *scope.IdentityID
		}
		candidates, err := workflow.PublishedCandidatesForScope(ctx, tx, j.project, scopeTask, scope.PlanID, candidateIdentity)
		if err != nil {
			return out, err
		}
		workflows := []string{}
		for _, candidate := range candidates {
			id := candidate.WorkflowID
			version := candidate.VersionID
			versions[id.String()] = version.String()
			raw, err := json.Marshal(candidate)
			if err != nil {
				return out, err
			}
			workflows = append(workflows, string(raw))
		}
		facts.WorkflowPrompt = "已发布流程参考；formalTaskBinding 是当前任务的正式绑定，不得覆盖。currentNodeCandidates：有任务时只列已绑定节点；仅计划时限于该计划当前工作；仅职位或计划加职位时还须符合该职位 allowedPositionIds；无任务/计划/职位的自由提交应为空，参考路径放 suggestedNextNodes。历史事实放 relatedEvidenceNodes。引用 workflowId/versionId/nodeId 和公开 reason；冲突或含糊时 needsClarification=true 并完整保留异议，不能以澄清标记绕过范围校验。候选和建议都不推进正式节点。\n" + strings.Join(workflows, "\n")
		if kind == "coordinator" {
			opinions, err := loadPublicOpinions(ctx, tx, j.project, j.batch, versions)
			if err != nil {
				return out, err
			}
			facts.WorkFacts = append(facts.WorkFacts, opinions)
		}
		references, err := scopedObjects(ctx, tx, j.project, topic, scopeTask, scope.PlanID)
		if err != nil {
			return out, err
		}
		projectWide := false
		if scopeTask == nil && topic != uuid.Nil {
			if err = tx.QueryRow(ctx, `SELECT kind='project_room' FROM topics WHERE project_id=$1 AND id=$2`, j.project, topic).Scan(&projectWide); err != nil {
				return out, err
			}
		}
		// Task analysis includes only affected work. Plan/branch discussion also
		// includes proposals explicitly filed there; unlinked project proposals
		// are visible only in a project-level discussion.
		const proposalScope = `($4::boolean OR ($5::boolean AND p.topic_id=$2) OR EXISTS(SELECT 1 FROM jsonb_array_elements(COALESCE(v.changes_json,'[]'::jsonb)) c WHERE c->>'targetId'=ANY($3::text[]) OR ($5::boolean AND c->'fields'->>'planId'=ANY($3::text[]))))`
		rows, err = tx.Query(ctx, `SELECT p.id,p.status,COALESCE(p.reason,''),COALESCE(v.review_hash,''),COALESCE(v.changes_json,'[]'::jsonb) FROM proposals p LEFT JOIN proposal_versions v ON v.id=p.current_review_id WHERE p.project_id=$1 AND `+proposalScope+` AND p.status IN ('pending','cancelled','stale') ORDER BY p.id`, j.project, topic, references, projectWide, scopeTask == nil)
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
		rows, err = tx.Query(ctx, `SELECT d.reason,d.decision,d.review_id FROM approval_decisions d JOIN proposals p ON p.current_review_id=d.review_id JOIN proposal_versions v ON v.id=p.current_review_id WHERE p.project_id=$1 AND `+proposalScope+` AND d.decision='reject' ORDER BY d.id`, j.project, topic, references, projectWide, scopeTask == nil)
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
		facts.WorkFacts = append(facts.WorkFacts, e.materialFacts(ctx, j.project, j.batch)...)
		provider, name, maxIn, maxOut, err := e.resolve(ctx, j.project, identity)
		if err != nil {
			return out, err
		}
		manifest := agentcontext.Build(facts)
		metadata := map[string]any{"schemaVersion": 1, "identityId": identity, "bindingVersion": binding, "positionVersion": facts.PositionVersion, "preferenceRevision": facts.PreferenceRevision, "preferenceHash": manifest.PreferenceHash, "workflowVersions": versions, "coveredSeq": facts.CoveredSeq, "model": name, "maxInputTokens": maxIn, "maxOutputTokens": maxOut, "layers": manifest.SharedLayers()}
		metadata["analysisScope"] = scope
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
		return runner.CallContext{CanManage: canManage, PrincipalKind: kind, Manifest: manifest, Provider: provider, ModelName: name, MaxInputTokens: maxIn, MaxOutputTokens: maxOut}, nil
	}
}
