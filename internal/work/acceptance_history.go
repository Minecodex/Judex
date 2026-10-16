package work

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/audit"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
	"github.com/kakj-go/Judex/internal/platform/paging"
)

type AcceptanceHistoryRecord struct {
	ID                   uuid.UUID       `json:"id"`
	TargetID             uuid.UUID       `json:"targetId"`
	TargetType           string          `json:"targetType"`
	TargetVersion        *int64          `json:"targetVersion"`
	Decision             string          `json:"decision"`
	ActorUserID          uuid.UUID       `json:"actorUserId"`
	IdentityID           *uuid.UUID      `json:"identityId"`
	Reason               string          `json:"reason"`
	AcceptanceID         *uuid.UUID      `json:"acceptanceId"`
	PreviousAcceptanceID *uuid.UUID      `json:"previousAcceptanceId"`
	ReviewHash           *string         `json:"reviewHash"`
	Snapshot             json.RawMessage `json:"snapshot"`
	Provenance           string          `json:"provenance"`
	CreatedAt            time.Time       `json:"createdAt"`
}

// Public immutable work records retain failed reviews as well as accepted
// evidence. Accepted snapshots and reopen_records remain their original
// lifecycle authorities; this adds the missing public feedback projection.
func recordFormalFeedback(ctx context.Context, tx pgx.Tx, project, user, target uuid.UUID, kind, decision, reason string, version int64, identity *uuid.UUID, manifest json.RawMessage, hash string, acceptance, previous *uuid.UUID, now time.Time) error {
	details, err := json.Marshal(map[string]any{"decision": decision, "targetVersion": version, "identityId": identity, "reviewManifest": manifest, "reviewHash": hash, "acceptanceId": acceptance, "previousAcceptanceId": previous})
	if err != nil {
		return err
	}
	source := string(audit.ContextSource(ctx))
	if source != "cli" {
		source = "web"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO work_change_records(project_id,id,object_type,object_id,operation,actor_user_id,text,created_at,details_json,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, project, uuid.New(), kind, target, "acceptance."+decision, user, reason, now, details, source); err != nil {
		return err
	}
	if kind == "task" {
		_, err = events.AppendProjectEvent(ctx, tx, project, "task.activity.changed", "task", target.String(), nil, map[string]any{"change": "acceptance." + decision}, now)
	}
	return err
}

// Legacy rejected decisions only retained their audit reason. Expose that
// actual source with an explicit provenance and null snapshot, never invent a
// historical review manifest or rewrite any accepted/source record.
func acceptanceHistorySQL(kind string) (string, error) {
	if kind != "task" && kind != "plan" {
		return "", apierrors.Fields("targetType", "task or plan required")
	}
	role := "reviewer_identity_id"
	if kind == "plan" {
		role = "owner_identity_id"
	}
	return fmt.Sprintf(`WITH history AS (
 SELECT w.id,w.object_id AS target_id,w.object_type AS target_type,(w.details_json->>'targetVersion')::bigint AS target_version,
 w.details_json->>'decision' AS decision,w.actor_user_id,(w.details_json->>'identityId')::uuid AS identity_id,w.text AS reason,
 (w.details_json->>'acceptanceId')::uuid AS acceptance_id,(w.details_json->>'previousAcceptanceId')::uuid AS previous_acceptance_id,
 NULLIF(w.details_json->>'reviewHash','') AS review_hash,w.details_json->'reviewManifest' AS snapshot,'work_record'::text AS provenance,w.created_at
 FROM work_change_records w WHERE w.project_id=$1 AND w.object_type=$2 AND w.object_id=$3 AND w.operation IN('acceptance.accept','acceptance.reject','acceptance.reopen')
 UNION ALL
 SELECT a.id,a.%[1]s_id,$2,a.%[1]s_version,'accept',a.actor_user_id,a.%[2]s,''::text,a.id,NULL::uuid,a.evidence_hash,a.review_manifest,'accepted_snapshot',a.created_at
 FROM %[1]s_acceptances a WHERE a.project_id=$1 AND a.%[1]s_id=$3 AND NOT EXISTS(SELECT 1 FROM work_change_records w WHERE w.project_id=a.project_id AND w.object_type=$2 AND w.object_id=$3 AND w.details_json->>'acceptanceId'=a.id::text)
 UNION ALL
 SELECT r.id,r.target_id,r.target_type,NULL::bigint,'reopen',r.actor_user_id,NULL::uuid,r.reason,NULL::uuid,r.previous_acceptance_id,NULL::text,NULL::jsonb,'reopen_record',r.created_at
 FROM reopen_records r WHERE r.project_id=$1 AND r.target_type=$2 AND r.target_id=$3 AND NOT EXISTS(SELECT 1 FROM work_change_records w WHERE w.project_id=r.project_id AND w.object_type=$2 AND w.object_id=$3 AND w.operation='acceptance.reopen' AND w.actor_user_id=r.actor_user_id AND w.created_at=r.created_at)
 UNION ALL
 SELECT a.id::uuid,$3,$2,NULL::bigint,'reject',a.actor_user_id,a.identity_id,COALESCE(a.reason,''),NULL::uuid,NULL::uuid,NULL::text,NULL::jsonb,'legacy_audit',a.occurred_at
 FROM audit_events a WHERE a.project_id=$1 AND a.object_type=$2 AND a.object_id=$3::text AND a.actor_type='user' AND a.operation=$2||'.reject'
 AND NOT EXISTS(SELECT 1 FROM work_change_records w WHERE w.project_id=a.project_id AND w.object_type=$2 AND w.object_id=$3 AND w.operation='acceptance.reject' AND w.actor_user_id=a.actor_user_id AND w.created_at=a.occurred_at)
 ) `, kind, role), nil
}

func (s *Service) AcceptanceHistory(ctx context.Context, user, project, target uuid.UUID, kind string) ([]AcceptanceHistoryRecord, error) {
	if _, err := memberTx(ctx, s.pool, project, user); err != nil {
		return nil, err
	}
	prefix, err := acceptanceHistorySQL(kind)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err = s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %ss WHERE project_id=$1 AND id=$2)`, kind), project, target).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.New(apierrors.NotFound, "work not found")
	}
	rows, err := paging.Query(ctx, s.pool, prefix+`SELECT h.id,h.target_id,h.target_type,h.target_version,h.decision,h.actor_user_id,h.identity_id,h.reason,h.acceptance_id,h.previous_acceptance_id,h.review_hash,h.snapshot,h.provenance,h.created_at /*keys*/ FROM history h WHERE true /*page*/`, "h.created_at", "h.id", project, kind, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AcceptanceHistoryRecord{}
	for rows.Next() {
		var r AcceptanceHistoryRecord
		if err = rows.Scan(&r.ID, &r.TargetID, &r.TargetType, &r.TargetVersion, &r.Decision, &r.ActorUserID, &r.IdentityID, &r.Reason, &r.AcceptanceID, &r.PreviousAcceptanceID, &r.ReviewHash, &r.Snapshot, &r.Provenance, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type feedbackReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Agent calls already validate active authority. Return only the latest public
// cause and reference metadata, keeping large review snapshots out of prompts.
func LatestFormalFeedback(ctx context.Context, q feedbackReader, project, target uuid.UUID, kind string) (json.RawMessage, error) {
	prefix, err := acceptanceHistorySQL(kind)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = q.QueryRow(ctx, prefix+`SELECT to_jsonb(h)-'snapshot' FROM history h ORDER BY h.created_at DESC,h.id DESC LIMIT 1`, project, kind, target).Scan(&raw)
	if err == pgx.ErrNoRows {
		return json.RawMessage("null"), nil
	}
	return raw, err
}
