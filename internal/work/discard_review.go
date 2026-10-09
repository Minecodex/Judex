package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type DraftDiscardReview struct {
	TargetID      uuid.UUID         `json:"targetId"`
	TargetType    string            `json:"targetType"`
	TargetVersion int64             `json:"targetVersion"`
	Title         string            `json:"title"`
	ReviewHash    string            `json:"reviewHash"`
	Tasks         []ExceptionImpact `json:"tasks"`
	Blockers      []Blocker         `json:"blockers"`
}

func (s *Service) DiscardReview(ctx context.Context, user, project, target uuid.UUID, kind string) (DraftDiscardReview, error) {
	return s.discardReview(ctx, poolAsQuery{s.pool}, nil, user, project, target, kind)
}
func (s *Service) discardReview(ctx context.Context, q dbQuery, tx pgx.Tx, user, project, target uuid.UUID, kind string) (DraftDiscardReview, error) {
	out := DraftDiscardReview{TargetID: target, TargetType: kind, Tasks: []ExceptionImpact{}, Blockers: []Blocker{}}
	access, err := workAccess(ctx, q, project, user, target, kind)
	if err != nil {
		return out, err
	}
	if !access.Capabilities.DiscardDraft {
		return out, apierrors.New(apierrors.Forbidden, "draft discard is not allowed")
	}
	table := map[string]string{"task": "tasks", "plan": "plans"}[kind]
	if table == "" {
		return out, apierrors.Fields("kind", "enum")
	}
	if err = q.QueryRow(ctx, fmt.Sprintf(`SELECT title,version FROM %s WHERE project_id=$1 AND id=$2`, table), project, target).Scan(&out.Title, &out.TargetVersion); err != nil {
		return out, err
	}
	ids := []uuid.UUID{target}
	var plan *uuid.UUID
	if kind == "plan" {
		plan = &target
		ids = []uuid.UUID{}
		rows, e := q.Query(ctx, `SELECT id,title,status,version FROM tasks WHERE project_id=$1 AND plan_id=$2 AND discarded_at IS NULL ORDER BY id`, project, target)
		if e != nil {
			return out, e
		}
		for rows.Next() {
			v := ExceptionImpact{Requirements: []Requirement{}}
			if e = rows.Scan(&v.TaskID, &v.Title, &v.Status, &v.Version); e != nil {
				rows.Close()
				return out, e
			}
			out.Tasks = append(out.Tasks, v)
			ids = append(ids, v.TaskID)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
	}
	out.Blockers, err = draftReferenceQuery(ctx, q, project, ids, plan)
	if err != nil {
		return out, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256(raw)
	out.ReviewHash = hex.EncodeToString(sum[:])
	return out, nil
}
