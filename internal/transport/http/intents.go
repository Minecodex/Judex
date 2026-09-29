// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/handoff"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/kakj-go/Judex/internal/workflow"
)

// IntentHandlers serves confirmation intents (06 §6).
type IntentHandlers struct {
	Decisions *decision.Service
	Work      *work.Service
	Projects  *project.Service
	Handoffs  *handoff.Service
	Workflows *workflow.Service
}

func NewIntentHandlers(decisions *decision.Service, workSvc *work.Service) *IntentHandlers {
	return &IntentHandlers{Decisions: decisions, Work: workSvc}
}

func (h *IntentHandlers) Register(spec *SpecRouter) {
	spec.Register("createGlobalConfirmationIntent", withAuth(h.create))
	spec.Register("getGlobalConfirmationIntent", withAuth(h.get))
	spec.Register("confirmGlobalIntent", withAuth(h.confirm))
	spec.Register("createConfirmationIntent", withAuth(h.create))
	spec.Register("getConfirmationIntent", withAuth(h.get))
	spec.Register("confirmIntent", withAuth(h.confirm))
}

func (h *IntentHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := intentProjectParam(c)
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
	projectID, err := intentProjectParam(c)
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
	projectID, err := intentProjectParam(c)
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
		number := func(key string) int64 { v, _ := inner[key].(float64); return int64(v) }
		text := func(key string) string { v, _ := inner[key].(string); return v }
		id := func(key string) uuid.UUID { v, _ := uuid.Parse(text(key)); return v }
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
		case "proposal.decision":
			value := text("decision")
			if value != "approve" && value != "reject" {
				return "", apierrors.Fields("decision", "enum")
			}
			var selection struct {
				SlotIDs  []uuid.UUID              `json:"slotIds"`
				Bindings []decision.ActingBinding `json:"actingBindingVersions"`
			}
			raw, _ := json.Marshal(inner)
			if err := json.Unmarshal(raw, &selection); err != nil {
				return "", apierrors.Fields("decision selection", "invalid")
			}
			result, err := h.Decisions.Decide(ctx, userID, projectID, objectID, reviewHash, value == "approve", text("reason"), decision.DecisionSelection{SlotIDs: selection.SlotIDs, Bindings: selection.Bindings})
			if err != nil {
				return "", err
			}
			return "proposal:" + result.ProposalID.String(), nil
		case "proposal.submit":
			_, err := h.Decisions.Submit(ctx, userID, projectID, objectID, number("expectedVersion"), reviewHash)
			return "proposal:" + objectID.String(), err
		case "task.reopen":
			result, err := h.Work.ReopenTask(ctx, userID, projectID, objectID, id("acceptanceId"), text("reason"), number("expectedVersion"))
			return "task:" + result.ID.String(), err
		case "plan.acceptance":
			value := text("decision")
			if value != "accept" && value != "reject" {
				return "", apierrors.Fields("decision", "enum")
			}
			result, err := h.Work.DecidePlanAcceptance(ctx, userID, projectID, objectID, reviewHash, value == "accept", text("reason"))
			return "plan:" + result.ID.String(), err
		case "plan.reopen":
			result, err := h.Work.ReopenPlan(ctx, userID, projectID, objectID, id("acceptanceId"), text("reason"))
			return "plan:" + result.ID.String(), err
		case "handoff.send", "handoff.decision":
			if h.Handoffs == nil {
				return "", apierrors.New(apierrors.DependencyDown, "handoff service unavailable")
			}
			var hid uuid.UUID
			var current *uuid.UUID
			if err := tx.QueryRow(ctx, `SELECT handoff_id,current_source_version_id FROM handoff_sources WHERE id=$1 AND project_id=$2`, objectID, projectID).Scan(&hid, &current); err != nil {
				return "", apierrors.New(apierrors.NotFound, "source not found")
			}
			if operation == "handoff.send" {
				var report uuid.UUID
				if err := tx.QueryRow(ctx, `SELECT t.latest_report_id FROM tasks t JOIN handoff_sources s ON s.source_task_id=t.id WHERE s.id=$1 AND s.project_id=$2`, objectID, projectID).Scan(&report); err != nil {
					return "", apierrors.New(apierrors.RequirementUnmet, "source report missing")
				}
				if report.String() != reviewHash {
					return "", apierrors.New(apierrors.ReviewStale, "source report changed")
				}
				result, err := h.Handoffs.SendSource(ctx, userID, projectID, hid, objectID, text("summary"))
				return "source:" + result.ID.String(), err
			}
			if current == nil || current.String() != reviewHash {
				return "", apierrors.New(apierrors.ReviewStale, "source version changed")
			}
			value := text("decision")
			if value != "accept" && value != "reject" {
				return "", apierrors.Fields("decision", "enum")
			}
			result, err := h.Handoffs.DecideSource(ctx, userID, projectID, hid, objectID, value == "accept", text("reason"))
			return "source:" + result.ID.String(), err
		case "project.create":
			if h.Projects == nil {
				return "", apierrors.New(apierrors.DependencyDown, "project service unavailable")
			}
			result, err := h.Projects.Create(ctx, userID, project.CreateRequest{Title: text("title"), Description: text("description"), Kind: text("kind"), MaxDiscussionRounds: int(number("maxDiscussionRounds")), ApprovalTimeoutSeconds: int(number("approvalTimeoutSeconds"))})
			return "project:" + result.ID.String(), err
		case "workflow.publish":
			if h.Workflows == nil {
				return "", apierrors.New(apierrors.DependencyDown, "workflow service unavailable")
			}
			result, err := h.Workflows.Publish(ctx, userID, projectID, objectID, number("expectedVersion"), reviewHash)
			return "workflowVersion:" + result.ID.String(), err
		default:
			return "", apierrors.Newf(apierrors.Validation, "unsupported intent operation %q", operation)
		}
	}
}

func intentProjectParam(c *gin.Context) (uuid.UUID, error) {
	if c.Param("projectId") == "" {
		return uuid.Nil, nil
	}
	return projectParam(c)
}
