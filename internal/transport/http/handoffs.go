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
	respond{}.ok(c, respond{}.list(c, handoffs, nil))
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

func (h *HandoffHandlers) send(c *gin.Context) { h.sendAt(c, 200) }
func (h *HandoffHandlers) sendAt(c *gin.Context, status int) {
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
		Summary       string `json:"summary" binding:"required"`
		ReviewHash    string `json:"reviewHash"`
		SourceVersion int64  `json:"sourceVersion"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	expected, err := uuid.Parse(req.ReviewHash)
	if err != nil {
		respond{}.error(c, apierrors.Fields("reviewHash", "report id required"))
		return
	}
	source, err := h.Handoffs.SendReviewed(c.Request.Context(), p.UserID, projectID, handoffID, sourceID, expected, req.SourceVersion, req.Summary)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.data(c, status, source)
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
		Decision      string `json:"decision" binding:"required"`
		ReviewHash    string `json:"reviewHash"`
		SourceVersion int64  `json:"sourceVersion"`
		Reason        string `json:"reason"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "accept" && req.Decision != "reject" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	expected, err := uuid.Parse(req.ReviewHash)
	if err != nil {
		respond{}.error(c, apierrors.Fields("reviewHash", "source version id required"))
		return
	}
	source, err := h.Handoffs.DecideSource(c.Request.Context(), p.UserID, projectID, handoffID, sourceID,
		req.Decision == "accept", req.Reason, expected)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, source)
}

func (h *HandoffHandlers) revise(c *gin.Context) { h.sendAt(c, 201) }

func (h *HandoffHandlers) reminder(c *gin.Context) {
	project, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	handoff, err := uuid.Parse(c.Param("handoffId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("handoffId", "uuid"))
		return
	}
	count, err := h.Handoffs.Remind(c.Request.Context(), principalFrom(c).UserID, project, handoff)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"reminded": true, "recipientCount": count})
}

func (h *HandoffHandlers) listReports(c *gin.Context) {
	project, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	task, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("taskId", "uuid"))
		return
	}
	reports, err := h.Work.ListReports(c.Request.Context(), principalFrom(c).UserID, project, task)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, reports, nil))
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
		CodeRefs            []map[string]any `json:"codeRefs"`
		EnvironmentRefs     []map[string]any `json:"environmentRefs"`
		Kind                string           `json:"kind" binding:"required"`
		SubmissionID        string           `json:"submissionId"`
		IdentityID          string           `json:"identityId"`
		Text                string           `json:"text"`
		MaterialVersionIDs  []string         `json:"materialVersionIds"`
		ExpectedTaskVersion int64            `json:"expectedTaskVersion"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	input := work.ReportInput{TaskID: taskID, CodeRefs: req.CodeRefs, EnvironmentRefs: req.EnvironmentRefs, Kind: req.Kind, Text: req.Text,
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
	reports, err := h.Work.ListReports(c.Request.Context(), p.UserID, projectID, taskID, reportID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	if len(reports) != 1 {
		respond{}.error(c, apierrors.New(apierrors.Internal, "report result missing"))
		return
	}
	respond{}.ok(c, reports[0])
}
