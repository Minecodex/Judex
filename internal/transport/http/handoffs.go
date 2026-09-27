// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/handoff"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
)

// HandoffHandlers serves handoff operations (06 §5).
type HandoffHandlers struct {
	Handoffs *handoff.Service
	Work     *work.Service
}

func NewHandoffHandlers(svc *handoff.Service, workSvc *work.Service) *HandoffHandlers {
	return &HandoffHandlers{Handoffs: svc, Work: workSvc}
}

func (h *HandoffHandlers) Register(spec *SpecRouter) {
	spec.Register("listHandoffs", withAuth(h.list))
	spec.Register("createHandoff", withAuth(h.create))
	spec.Register("sendHandoffSource", withAuth(h.send))
	spec.Register("decideHandoffSource", withAuth(h.decide))
	spec.Register("createHandoffSourceRevision", withAuth(h.revise))
	spec.Register("sendHandoffReminder", withAuth(h.reminder))
	spec.Register("listTaskReports", withAuth(h.listReports))
	spec.Register("createTaskReport", withAuth(h.createReport))
}

func (h *HandoffHandlers) list(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	handoffs, err := h.Handoffs.List(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(handoffs, nil))
}

func (h *HandoffHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Title              string `json:"title"`
		TargetTaskID       string `json:"targetTaskId" binding:"required"`
		ReceiverIdentityID string `json:"receiverIdentityId" binding:"required"`
		Kind               string `json:"kind" binding:"required"`
		Sources            []struct {
			SourceTaskID     string `json:"sourceTaskId" binding:"required"`
			SenderIdentityID string `json:"senderIdentityId" binding:"required"`
		} `json:"sources" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	target, err := uuid.Parse(req.TargetTaskID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("targetTaskId", "invalid"))
		return
	}
	receiver, err := uuid.Parse(req.ReceiverIdentityID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("receiverIdentityId", "invalid"))
		return
	}
	specs := make([]struct {
		SourceTaskID     uuid.UUID
		SenderIdentityID uuid.UUID
	}, 0, len(req.Sources))
	for _, raw := range req.Sources {
		src, err := uuid.Parse(raw.SourceTaskID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("sources[].sourceTaskId", "invalid"))
			return
		}
		sender, err := uuid.Parse(raw.SenderIdentityID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("sources[].senderIdentityId", "invalid"))
			return
		}
		specs = append(specs, struct {
			SourceTaskID     uuid.UUID
			SenderIdentityID uuid.UUID
		}{src, sender})
	}
	created, err := h.Handoffs.Create(c.Request.Context(), p.UserID, projectID, req.Title, target, receiver, req.Kind, specs)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, created)
}

func handoffSourceParams(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	handoffID, err := uuid.Parse(c.Param("handoffId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apierrors.Fields("handoffId", "invalid")
	}
	sourceID, err := uuid.Parse(c.Param("sourceId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apierrors.Fields("sourceId", "invalid")
	}
	return handoffID, sourceID, nil
}

func (h *HandoffHandlers) send(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	handoffID, sourceID, err := handoffSourceParams(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var req struct {
		Summary string `json:"summary" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	source, err := h.Handoffs.SendSource(c.Request.Context(), p.UserID, projectID, handoffID, sourceID, req.Summary)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, source)
}

func (h *HandoffHandlers) decide(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	handoffID, sourceID, err := handoffSourceParams(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var req struct {
		Decision string `json:"decision" binding:"required"`
		Reason   string `json:"reason"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "accept" && req.Decision != "reject" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	source, err := h.Handoffs.DecideSource(c.Request.Context(), p.UserID, projectID, handoffID, sourceID,
		req.Decision == "accept", req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, source)
}

func (h *HandoffHandlers) revise(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	handoffID, sourceID, err := handoffSourceParams(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var req struct {
		Summary string `json:"summary" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	source, err := h.Handoffs.SendSource(c.Request.Context(), p.UserID, projectID, handoffID, sourceID, req.Summary)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, source)
}

func (h *HandoffHandlers) reminder(c *gin.Context) {
	respond{}.accepted(c, gin.H{"reminded": true}, nil)
}

func (h *HandoffHandlers) listReports(c *gin.Context) {
	_, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	if _, err := uuid.Parse(c.Param("taskId")); err != nil {
		respond{}.error(c, apierrors.Fields("taskId", "invalid"))
		return
	}
	respond{}.ok(c, respond{}.list([]any{}, nil))
}

func (h *HandoffHandlers) createReport(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("taskId", "invalid"))
		return
	}
	var req struct {
		Kind                string   `json:"kind" binding:"required"`
		SubmissionID        string   `json:"submissionId"`
		IdentityID          string   `json:"identityId"`
		Text                string   `json:"text"`
		MaterialVersionIDs  []string `json:"materialVersionIds"`
		ExpectedTaskVersion int64    `json:"expectedTaskVersion"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	input := work.ReportInput{TaskID: taskID, Kind: req.Kind, Text: req.Text,
		MaterialVersionIDs: req.MaterialVersionIDs, ExpectedTaskVersion: req.ExpectedTaskVersion}
	if req.SubmissionID != "" {
		id, err := uuid.Parse(req.SubmissionID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("submissionId", "invalid"))
			return
		}
		input.SubmissionID = &id
	}
	if req.IdentityID != "" {
		id, err := uuid.Parse(req.IdentityID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("identityId", "invalid"))
			return
		}
		input.IdentityID = &id
	}
	reportID, _, err := h.Work.Report(c.Request.Context(), p.UserID, projectID, input)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"id": reportID})
}
