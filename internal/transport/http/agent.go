// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// AgentHandlers serves run start/status/cancel and batch extensions (06 §6).
// The actual harness runs in worker processes; these endpoints register the
// intent and return 202 — never a fake completion.
type AgentHandlers struct {
	Pool *pgxpool.Pool
}

func NewAgentHandlers(pool *pgxpool.Pool) *AgentHandlers { return &AgentHandlers{Pool: pool} }

func (h *AgentHandlers) Register(spec *SpecRouter) {
	spec.Register("startAgentRun", withAuth(h.start))
	spec.Register("getAgentRun", withAuth(h.status))
	spec.Register("cancelAgentRun", withAuth(h.cancel))
	spec.Register("extendDiscussionBatch", withAuth(h.extend))
}

func (h *AgentHandlers) start(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	topicID, err := uuid.Parse(c.Param("topicId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("topicId", "invalid"))
		return
	}
	var req struct {
		SourceSubmissionID string `json:"sourceSubmissionId" binding:"required"`
		Reason             string `json:"reason"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	sourceID, err := uuid.Parse(req.SourceSubmissionID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("sourceSubmissionId", "invalid"))
		return
	}
	// Register the batch (UNIQUE(source, trigger) dedups); worker picks up.
	if _, err := h.Pool.Exec(c.Request.Context(), `
		INSERT INTO discussion_batches (project_id, id, topic_id, source_submission_id,
			policy_snapshot, max_rounds, state, trigger_kind, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'{}',3,'queued','submission',now(),now())
		ON CONFLICT (source_submission_id, trigger_kind) DO NOTHING`,
		projectID, uuid.New(), topicID, sourceID); err != nil {
		respond{}.error(c, apierrors.New(apierrors.Internal, "batch register failed").Wrap(err))
		return
	}
	respond{}.accepted(c, gin.H{"state": "queued", "source": req.SourceSubmissionID, "requestedBy": p.UserID.String()}, nil)
}

func (h *AgentHandlers) status(c *gin.Context) {
	if _, err := projectParam(c); err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("runId", "invalid"))
		return
	}
	var state string
	err = h.Pool.QueryRow(c.Request.Context(),
		`SELECT state FROM agent_runs WHERE id=$1`, runID).Scan(&state)
	if err != nil {
		respond{}.error(c, apierrors.New(apierrors.NotFound, "run not found"))
		return
	}
	respond{}.ok(c, gin.H{"id": runID, "state": state})
}

func (h *AgentHandlers) cancel(c *gin.Context) {
	if _, err := projectParam(c); err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("runId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		Reason          string `json:"reason" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	tag, err := h.Pool.Exec(c.Request.Context(), `
		UPDATE agent_runs SET state='cancelled', updated_at=now()
		WHERE id=$1 AND state IN ('queued','provisioning','running','waiting_children','waiting_human','waiting_material')`,
		runID)
	if err != nil || tag.RowsAffected() == 0 {
		respond{}.error(c, apierrors.New(apierrors.InvalidTransition, "run not cancellable"))
		return
	}
	respond{}.ok(c, gin.H{"cancelling": true})
}

func (h *AgentHandlers) extend(c *gin.Context) {
	if _, err := projectParam(c); err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	batchID, err := uuid.Parse(c.Param("batchId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("batchId", "invalid"))
		return
	}
	var req struct {
		ExtraRounds     int    `json:"extraRounds" binding:"required"`
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		Reason          string `json:"reason" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.ExtraRounds < 1 || req.ExtraRounds > 100 {
		respond{}.error(c, apierrors.Fields("extraRounds", "range"))
		return
	}
	tag, err := h.Pool.Exec(c.Request.Context(), `
		UPDATE discussion_batches SET max_rounds = max_rounds + $2, updated_at=now()
		WHERE id=$1 AND max_rounds + $2 <= 100`,
		batchID, req.ExtraRounds)
	if err != nil || tag.RowsAffected() == 0 {
		respond{}.error(c, apierrors.New(apierrors.RateLimited, "续开超出部署上限或批次不存在"))
		return
	}
	respond{}.ok(c, gin.H{"extended": req.ExtraRounds})
}
