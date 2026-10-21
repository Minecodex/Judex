package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

func (s *Service) knowledge(ctx context.Context, group uuid.UUID, kind, ref string, content any) error {
	raw, e := json.Marshal(content)
	if e != nil {
		return e
	}
	var exists bool
	if e = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_entries WHERE group_id=$1 AND kind=$2 AND ref_id=$3)`, group, kind, ref).Scan(&exists); e != nil || exists {
		return e
	}
	var seq int64
	if e = s.Pool.QueryRow(ctx, `UPDATE conversations SET knowledge_seq=knowledge_seq+1 WHERE id=$1 RETURNING knowledge_seq`, group).Scan(&seq); e != nil {
		return e
	}
	_, e = s.Pool.Exec(ctx, `INSERT INTO knowledge_entries(group_id,seq,kind,ref_id,content) VALUES($1,$2,$3,$4,$5)`, group, seq, kind, ref, raw)
	return e
}
func (s *Service) Knowledge(ctx context.Context, group uuid.UUID, after int64, limit int) ([]json.RawMessage, error) {
	a, e := s.Access(ctx, group)
	if e != nil {
		return nil, e
	}
	if a.Kind != "group" {
		return nil, apierrors.Fields("conversationId", "private chat has no AI context")
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	return s.list(ctx, `SELECT seq,kind,ref_id AS "refId",content,created_at AS "createdAt" FROM knowledge_entries WHERE group_id=$1 AND seq>$2 ORDER BY seq LIMIT $3`, group, after, limit)
}
func (s *Service) Snapshots(ctx context.Context, group uuid.UUID) ([]json.RawMessage, error) {
	if _, e := s.Access(ctx, group); e != nil {
		return nil, e
	}
	return s.list(ctx, `SELECT version,covered_through AS "coveredThrough",summary,estimated_tokens AS "estimatedTokens",created_at AS "createdAt" FROM context_snapshots WHERE group_id=$1 ORDER BY version DESC LIMIT 20`, group)
}
func (s *Service) ToolKnowledge(ctx context.Context, p Publication, callID, name string, result any) error {
	return s.write(ctx, func(ctx context.Context) error {
		if e := s.lockConversation(ctx, p.GroupID); e != nil {
			return e
		}
		var valid bool
		if e := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_runs run JOIN discussions d ON d.id=run.discussion_id WHERE run.id=$1 AND run.fence=$2 AND run.state='running' AND d.state IN('active','paused'))`, p.RunID, p.Fence).Scan(&valid); e != nil {
			return e
		}
		if !valid {
			return apierrors.New(apierrors.InvalidTransition, "stale tool result rejected")
		}

		return s.knowledge(ctx, p.GroupID, "tool", p.RunID.String()+":"+callID, map[string]any{"roleAgentId": p.RoleID, "userId": p.UserID, "name": name, "result": result})
	})
}

// KnowledgeIndex is the bounded discovery result used by the model tool.
// Returning previous full read_context results would recursively duplicate
// history. Exact originals remain available through ReadReference.
func (s *Service) KnowledgeIndex(ctx context.Context, group uuid.UUID, after int64, limit int) ([]json.RawMessage, error) {
	a, e := s.Access(ctx, group)
	if e != nil {
		return nil, e
	}
	if a.Kind != "group" {
		return nil, missing()
	}
	if limit < 1 || limit > 20 {
		limit = 20
	}
	return s.list(ctx, `SELECT seq,kind,ref_id AS "refId",jsonb_build_object('indexExcerpt',substring(content::text FROM 1 FOR 200),'originalCharacters',length(content::text),'readOriginal',jsonb_build_object('kind',kind,'refId',ref_id),'notice','Reference index only. Use read_context with kind/refId to page the complete original.') AS content FROM knowledge_entries WHERE group_id=$1 AND seq>$2 ORDER BY seq LIMIT $3`, group, after, limit)
}
