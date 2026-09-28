// SPDX-License-Identifier: Apache-2.0

package decision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// Intent operations whitelist (06 §6): every human command that a CLI may
// request but never execute itself.
var intentOperations = map[string]bool{
	"proposal.decision": true, "task.acceptance": true, "task.reopen": true,
	"plan.acceptance": true, "plan.reopen": true, "handoff.send": true,
	"handoff.decision": true, "project.create": true, "workflow.publish": true,
}

// IntentTTL bounds the confirmation window (07 §5 默认 10min).
const IntentTTL = 10 * time.Minute

// CreateIntent binds a CLI request to ONE typed operation with a canonical
// payload hash (07 §5 步骤2): returns confirmUrl (not an approval), nonce.
// The intent carries the requesting grant so the browser page can display
// "由设备 X 请求".
func (s *Service) CreateIntent(ctx context.Context, requester, projectID uuid.UUID, grantID *uuid.UUID, operation string, objectID uuid.UUID, reviewHash string, payload map[string]any) (map[string]any, error) {
	if !intentOperations[operation] {
		return nil, apierrors.Newf(apierrors.Validation, "operation %q 不在白名单", operation)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, apierrors.Fields("payload", "marshal")
	}
	sum := sha256.Sum256(raw)
	canonicalHash := hex.EncodeToString(sum[:])
	if reviewHash == "" {
		reviewHash = "none"
	}
	id := uuid.New()
	nonce := keys.NewRandom()[:16]
	expiresAt := s.now().Add(IntentTTL)
	err = s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		if _, err := memberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var grantColumn any
		if grantID != nil {
			grantColumn = *grantID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO confirmation_intents (project_id, id, user_id, grant_id, operation, object_id,
				review_hash, canonical_payload_hash, payload_json, state, expires_at, nonce, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,'pending',$10,$11,$12)`,
			projectID, id, requester, grantColumn, operation, objectID,
			reviewHash, canonicalHash, string(raw), expiresAt, nonce, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "intent insert failed").Wrap(err)
		}
		return audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceCLI, Operation: "intent.create",
			ObjectType: "confirmation_intent", ObjectID: id.String(), OccurredAt: s.now(),
		})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "state": "pending", "operation": operation, "objectId": objectID,
		"confirmUrl": "/confirm/" + id.String(), "nonce": nonce, "expiresAt": expiresAt,
	}, nil
}

// GetIntent returns the intent state for the browser page / CLI polling;
// only the owner (or the requesting grant's user) sees it.
func (s *Service) GetIntent(ctx context.Context, requester, projectID, intentID uuid.UUID) (map[string]any, error) {
	var (
		userID    uuid.UUID
		operation string
		objectID  uuid.UUID
		state     string
		expiresAt time.Time
		resultRef *string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, operation, object_id, state, expires_at, result_ref
		FROM confirmation_intents WHERE id=$1 AND project_id=$2`, intentID, projectID).
		Scan(&userID, &operation, &objectID, &state, &expiresAt, &resultRef)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierrors.New(apierrors.NotFound, "intent not found")
	}
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "intent lookup failed").Wrap(err)
	}
	if userID != requester {
		return nil, apierrors.New(apierrors.NotFound, "intent not found")
	}
	if state == "pending" && s.now().After(expiresAt) {
		_, _ = s.pool.Exec(ctx, `UPDATE confirmation_intents SET state='expired' WHERE id=$1 AND state='pending'`, intentID)
		state = "expired"
	}
	return map[string]any{
		"id": intentID, "state": state, "operation": operation, "objectId": objectID,
		"expiresAt": expiresAt, "resultRef": resultRef,
	}, nil
}

// Executor abstracts the domain command an intent confirms (07 §5 步骤4:
// 浏览器一次点击 → 同一事务复核并调用原领域命令).
type Executor func(ctx context.Context, tx pgx.Tx, userID uuid.UUID, payload map[string]any) (resultRef string, err error)

// ConfirmIntent is the browser-only atomic confirmation: web session
// required, intent pending + unexpired + same user, nonce bound; executes
// the whitelisted domain command INSIDE the same transaction and consumes
// the intent exactly once. Repeat clicks see the same result.
func (s *Service) ConfirmIntent(ctx context.Context, requester, projectID, intentID uuid.UUID, nonce string, approve bool, exec Executor) (map[string]any, error) {
	var out map[string]any
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		// Advisory lock serializes double-clicks across connections.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, intentID.String()); err != nil {
			return err
		}
		if err := tx.LockProjectForUpdate(ctx, projectID.String()); err != nil {
			return err
		}
		var (
			userID           uuid.UUID
			operation        string
			objectID         uuid.UUID
			state            string
			expiresAt        time.Time
			nonceHash        string
			payloadHash      string
			resultRef        *string
			storedReviewHash string
		)
		if err := tx.QueryRow(ctx, `
			SELECT user_id, operation, object_id, state, expires_at, nonce, canonical_payload_hash, result_ref, review_hash
			FROM confirmation_intents WHERE id=$1 AND project_id=$2 FOR UPDATE`, intentID, projectID).
			Scan(&userID, &operation, &objectID, &state, &expiresAt, &nonceHash, &payloadHash, &resultRef, &storedReviewHash); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apierrors.New(apierrors.NotFound, "intent not found")
			}
			return apierrors.New(apierrors.Internal, "intent lookup failed").Wrap(err)
		}
		if userID != requester {
			return apierrors.New(apierrors.NotFound, "intent not found")
		}
		if state == "committed" || state == "rejected" {
			// Idempotent replay of the same click.
			out = map[string]any{"id": intentID, "state": state, "resultRef": resultRef}
			return nil
		}
		if state != "pending" {
			return apierrors.New(apierrors.InvalidTransition, "intent "+state)
		}
		if s.now().After(expiresAt) {
			_, _ = tx.Exec(ctx, `UPDATE confirmation_intents SET state='expired' WHERE id=$1`, intentID)
			return apierrors.New(apierrors.InvalidTransition, "intent 已过期")
		}
		if nonce != "" && nonce != nonceHash {
			return apierrors.New(apierrors.ReviewStale, "nonce 不匹配")
		}
		now := s.now()
		if !approve {
			if _, err := tx.Exec(ctx, `UPDATE confirmation_intents SET state='rejected', consumed_at=$2 WHERE id=$1`, intentID, now); err != nil {
				return apierrors.New(apierrors.Internal, "reject failed").Wrap(err)
			}
			if err := audit.Append(ctx, tx, audit.Entry{
				ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
				Source: audit.SourceWeb, Operation: "intent.reject",
				ObjectType: "confirmation_intent", ObjectID: intentID.String(), OccurredAt: now,
			}); err != nil {
				return err
			}
			out = map[string]any{"id": intentID, "state": "rejected"}
			return nil
		}
		// Execute the original domain command in THIS transaction.
		var payloadRaw []byte
		if err := tx.QueryRow(ctx, `SELECT payload_json FROM confirmation_intents WHERE id=$1`, intentID).Scan(&payloadRaw); err != nil {
			return apierrors.New(apierrors.Internal, "payload load failed").Wrap(err)
		}
		var payload map[string]any
		_ = json.Unmarshal(payloadRaw, &payload)
		payload["operation"] = operation
		payload["objectId"] = objectID.String()
		payload["reviewHash"] = storedReviewHash
		command, err := exec(ctx, tx, requester, payload)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE confirmation_intents SET state='committed', consumed_at=$2, result_ref=$3 WHERE id=$1`,
			intentID, now, command); err != nil {
			return apierrors.New(apierrors.Internal, "commit failed").Wrap(err)
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceCLI, Operation: "intent.confirm." + operation,
			ObjectType: "confirmation_intent", ObjectID: intentID.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = map[string]any{"id": intentID, "state": "committed", "resultRef": command}
		return nil
	})
	return out, err
}

func reviewHashFromIntent(payload map[string]any) string {
	v, _ := payload["reviewHash"].(string)
	return v
}
