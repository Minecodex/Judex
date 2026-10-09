// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// AgentHandlers serves run start/status/cancel and batch extensions (06 §6).
// The actual harness runs in worker processes; these endpoints register the
// intent and return 202 — never a fake completion.
type AgentHandlers struct {
	Pool *postgres.Pool
}

func NewAgentHandlers(pool *postgres.Pool) *AgentHandlers { return &AgentHandlers{Pool: pool} }

func (h *AgentHandlers) Register(spec *SpecRouter) {
	spec.Register("startAgentRun", withAuth(h.start))
	spec.Register("getAgentRun", withAuth(h.status))
	spec.Register("subscribeRunEvents", withAuth(h.events))
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
	svc := batch.Service{Pool: h.Pool}
	batchID, runID, err := svc.Start(c.Request.Context(), p.UserID, projectID, topicID, sourceID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.accepted(c, gin.H{"batchId": batchID, "runId": runID}, nil)
}

func (h *AgentHandlers) status(c *gin.Context) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("runId", "uuid"))
		return
	}
	svc := batch.Service{Pool: h.Pool}
	run, err := svc.Get(c.Request.Context(), principalFrom(c).UserID, projectID, runID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, run)
}
func (h *AgentHandlers) cancel(c *gin.Context) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("runId", "uuid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		Reason          string `json:"reason"`
	}
	if err = bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	svc := batch.Service{Pool: h.Pool}
	if err = svc.Cancel(c.Request.Context(), principalFrom(c).UserID, projectID, runID, req.ExpectedVersion, req.Reason); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"cancelling": true})
}
func (h *AgentHandlers) extend(c *gin.Context) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	batchID, err := uuid.Parse(c.Param("batchId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("batchId", "uuid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		ExtraRounds     int    `json:"extraRounds"`
		Reason          string `json:"reason"`
	}
	if err = bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	svc := batch.Service{Pool: h.Pool}
	if err = svc.Extend(c.Request.Context(), principalFrom(c).UserID, projectID, batchID, req.ExpectedVersion, req.ExtraRounds, req.Reason); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"extended": req.ExtraRounds})
}
