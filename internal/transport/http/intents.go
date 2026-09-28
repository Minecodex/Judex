// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
)

// IntentHandlers serves confirmation intents (06 §6).
type IntentHandlers struct {
	Decisions *decision.Service
	Work      *work.Service
}

func NewIntentHandlers(decisions *decision.Service, workSvc *work.Service) *IntentHandlers {
	return &IntentHandlers{Decisions: decisions, Work: workSvc}
}

func (h *IntentHandlers) Register(spec *SpecRouter) {
	spec.Register("createConfirmationIntent", withAuth(h.create))
	spec.Register("getConfirmationIntent", withAuth(h.get))
	spec.Register("confirmIntent", withAuth(h.confirm))
}

func (h *IntentHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Operation  string         `json:"operation" binding:"required"`
		ObjectID   string         `json:"objectId" binding:"required"`
		ReviewHash string         `json:"reviewHash" binding:"required"`
		Payload    map[string]any `json:"payload"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	objectID, err := uuid.Parse(req.ObjectID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("objectId", "invalid"))
		return
	}
	intent, err := h.Decisions.CreateIntent(c.Request.Context(), p.UserID, projectID, grantPtr(p),
		req.Operation, objectID, req.ReviewHash, req.Payload)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, intent)
}

func grantPtr(p *auth.Principal) *uuid.UUID {
	if p.Kind == auth.KindCLI && p.GrantID != uuid.Nil {
		return &p.GrantID
	}
	return nil
}

func (h *IntentHandlers) get(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	intentID, err := uuid.Parse(c.Param("intentId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("intentId", "invalid"))
		return
	}
	intent, err := h.Decisions.GetIntent(c.Request.Context(), p.UserID, projectID, intentID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, intent)
}

// confirm executes the original domain command inside the intent
// transaction; only WEB sessions reach here (CLI grants are rejected by the
// human-confirmation gate).
func (h *IntentHandlers) confirm(c *gin.Context) {
	p := principalFrom(c)
	if !p.CanConfirmHumanDecision() {
		respond{}.error(c, apierrors.New(apierrors.Forbidden, "人工确认仅限浏览器会话"))
		return
	}
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	intentID, err := uuid.Parse(c.Param("intentId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("intentId", "invalid"))
		return
	}
	var req struct {
		IntentHash string `json:"intentHash"`
		Decision   string `json:"decision" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "approve" && req.Decision != "reject" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	result, err := h.Decisions.ConfirmIntent(c.Request.Context(), p.UserID, projectID, intentID,
		req.IntentHash, req.Decision == "approve", h.executorFor(projectID))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, result)
}

// executorFor maps intent operations to domain commands. The intent payload
// is stored canonically at create time; the executor re-reads it from the
// row via the Executor contract (07 §5 步骤4: 同一事务调用原领域命令).
func (h *IntentHandlers) executorFor(projectID uuid.UUID) decision.Executor {
	return func(ctx context.Context, tx pgx.Tx, userID uuid.UUID, payload map[string]any) (string, error) {
		if payload == nil {
			payload = map[string]any{}
		}
		operation, _ := payload["operation"].(string)
		objectRaw, _ := payload["objectId"].(string)
		objectID, err := uuid.Parse(objectRaw)
		if err != nil {
			return "", apierrors.Fields("objectId", "invalid")
		}
		reviewHash, _ := payload["reviewHash"].(string)
		// The stored payload IS the command body (flat); some clients may
		// wrap it once more under "payload" — accept both shapes.
		inner := payload
		if nested, ok := payload["payload"].(map[string]any); ok {
			inner = nested
		}
		switch operation {
		case "task.acceptance":
			expected := int64(0)
			if v, ok := inner["expectedVersion"].(float64); ok {
				expected = int64(v)
			}
			decisionValue, _ := inner["decision"].(string)
			reason, _ := inner["reason"].(string)
			task, err := h.Work.DecideTaskAcceptanceTx(ctx, tx, userID, projectID, objectID,
				reviewHash, decisionValue == "accept", reason, expected)
			if err != nil {
				return "", err
			}
			return "task:" + task.ID.String() + ":" + task.Status, nil
		default:
			return "", apierrors.Newf(apierrors.Validation, "operation %q 暂未接入意图执行", operation)
		}
	}
}
