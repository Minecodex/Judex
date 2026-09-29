// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
)

// WorkHandlers serves plan/task drafts and queries (06 §5).
type WorkHandlers struct {
	Work *work.Service
}

func NewWorkHandlers(svc *work.Service) *WorkHandlers { return &WorkHandlers{Work: svc} }

func (h *WorkHandlers) Register(spec *SpecRouter) {
	spec.Register("listPlans", withAuth(h.listPlans))
	spec.Register("createPlan", withAuth(h.createPlan))
	spec.Register("listTasks", withAuth(h.listTasks))
	spec.Register("createTask", withAuth(h.createTask))
	spec.Register("getTask", withAuth(h.getTask))
	spec.Register("getPlan", withAuth(h.getPlan))
	spec.Register("discardPlan", withAuth(h.discardPlan))
	spec.Register("discardTask", withAuth(h.discardTask))
	spec.Register("startTask", withAuth(h.startTask))
	spec.Register("getExecutionMap", withAuth(h.executionMap))
	spec.Register("getTaskAcceptanceReview", withAuth(h.taskReview))
	spec.Register("decideTaskAcceptance", withAuth(h.taskDecide))
	spec.Register("reopenTask", withAuth(h.taskReopen))
	spec.Register("getPlanAcceptanceReview", withAuth(h.planReview))
	spec.Register("acceptPlan", withAuth(h.planDecide))
	spec.Register("reopenPlan", withAuth(h.planReopen))
}

func (h *WorkHandlers) taskReview(c *gin.Context) {
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
	review, err := h.Work.TaskAcceptanceReview(c.Request.Context(), p.UserID, projectID, taskID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, review)
}

func (h *WorkHandlers) taskDecide(c *gin.Context) {
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
		ReviewID        string `json:"reviewId" binding:"required"`
		ReviewHash      string `json:"reviewHash" binding:"required"`
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		Decision        string `json:"decision" binding:"required"`
		Reason          string `json:"reason"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "accept" && req.Decision != "reject" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	task, err := h.Work.DecideTaskAcceptance(c.Request.Context(), p.UserID, projectID, taskID,
		req.ReviewHash, req.Decision == "accept", req.Reason, req.ExpectedVersion)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.respondTask(c, projectID, task.ID)
}

func (h *WorkHandlers) taskReopen(c *gin.Context) {
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
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		AcceptanceID    string `json:"acceptanceId"`
		Reason          string `json:"reason" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	acceptanceID, err := uuid.Parse(req.AcceptanceID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("acceptanceId", "invalid"))
		return
	}
	task, err := h.Work.ReopenTask(c.Request.Context(), p.UserID, projectID, taskID, acceptanceID, req.Reason, req.ExpectedVersion)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.respondTask(c, projectID, task.ID)
}

func (h *WorkHandlers) planReview(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("planId", "invalid"))
		return
	}
	review, err := h.Work.PlanAcceptanceReview(c.Request.Context(), p.UserID, projectID, planID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, review)
}

func (h *WorkHandlers) planDecide(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("planId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		ReviewID        string `json:"reviewId" binding:"required"`
		ReviewHash      string `json:"reviewHash" binding:"required"`
		Decision        string `json:"decision" binding:"required"`
		Reason          string `json:"reason"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "accept" && req.Decision != "reject" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	review, err := h.Work.PlanAcceptanceReview(c.Request.Context(), p.UserID, projectID, planID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	if review.TargetVersion != req.ExpectedVersion {
		respond{}.error(c, apierrors.New(apierrors.VersionConflict, "plan version changed"))
		return
	}
	_, err = h.Work.DecidePlanAcceptance(c.Request.Context(), p.UserID, projectID, planID,
		req.ReviewHash, req.Decision == "accept", req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.getPlan(c)
}

func (h *WorkHandlers) planReopen(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("planId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		AcceptanceID    string `json:"acceptanceId"`
		Reason          string `json:"reason" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	acceptanceID, err := uuid.Parse(req.AcceptanceID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("acceptanceId", "invalid"))
		return
	}
	_, err = h.Work.ReopenPlan(c.Request.Context(), p.UserID, projectID, planID, acceptanceID, req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.getPlan(c)
}

func (h *WorkHandlers) startTask(c *gin.Context) {
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
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		IdentityID      string `json:"identityId"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	identity, err := optionalUUID(req.IdentityID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	task, err := h.Work.Start(c.Request.Context(), p.UserID, projectID, taskID, identity, req.ExpectedVersion)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	h.respondTask(c, projectID, task.ID)
}

func (h *WorkHandlers) executionMap(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	planID, err := optionalUUID(c.Query("planId"))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	m, err := h.Work.ExecutionMap(c.Request.Context(), p.UserID, projectID, planID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, m)
}

func (h *WorkHandlers) listPlans(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	plans, err := h.Work.ListPlans(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, plans, nil))
}

func optionalUUID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apierrors.Fields("id", "invalid")
	}
	return &id, nil
}

func (h *WorkHandlers) createPlan(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Title              string `json:"title" binding:"required"`
		Goal               string `json:"goal"`
		AcceptanceCriteria string `json:"acceptanceCriteria"`
		OwnerIdentityID    string `json:"ownerIdentityId"`
		WorkflowID         string `json:"workflowId"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	owner, err := optionalUUID(req.OwnerIdentityID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	workflow, err := optionalUUID(req.WorkflowID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	plan, err := h.Work.CreatePlanDraft(c.Request.Context(), p.UserID, projectID,
		req.Title, req.Goal, req.AcceptanceCriteria, owner, workflow)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, plan)
}

func (h *WorkHandlers) listTasks(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	planID, err := optionalUUID(c.Query("planId"))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	tasks, err := h.Work.ListTasks(c.Request.Context(), p.UserID, projectID, planID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, tasks, nil))
}

func (h *WorkHandlers) createTask(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Title              string           `json:"title" binding:"required"`
		PlanID             string           `json:"planId"`
		ParentTaskID       string           `json:"parentTaskId"`
		ExpectedOutput     string           `json:"expectedOutput"`
		AcceptanceCriteria string           `json:"acceptanceCriteria"`
		Kind               string           `json:"kind"`
		ReviewerIdentityID string           `json:"reviewerIdentityId"`
		WorkflowID         string           `json:"workflowId"`
		NodeID             string           `json:"nodeId"`
		ParticipantIDs     []string         `json:"participantIdentityIds"`
		BugDetails         *work.BugDetails `json:"bugDetails"`
		Requirements       []struct {
			Phase             string     `json:"phase"`
			Kind              string     `json:"kind"`
			TargetID          string     `json:"targetId"`
			MaterialVersionID *uuid.UUID `json:"materialVersionId"`
			Hard              bool       `json:"hard"`
			Label             string     `json:"label"`
		} `json:"requirements"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	draft := work.TaskDraft{
		Title: req.Title, ExpectedOutput: req.ExpectedOutput,
		AcceptanceCriteria: req.AcceptanceCriteria, Kind: req.Kind, BugDetails: req.BugDetails,
	}
	if draft.PlanID, err = optionalUUID(req.PlanID); err != nil {
		respond{}.error(c, err)
		return
	}
	if draft.ParentTaskID, err = optionalUUID(req.ParentTaskID); err != nil {
		respond{}.error(c, err)
		return
	}
	if draft.ReviewerIdentityID, err = optionalUUID(req.ReviewerIdentityID); err != nil {
		respond{}.error(c, err)
		return
	}
	if draft.WorkflowID, err = optionalUUID(req.WorkflowID); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.NodeID != "" {
		draft.NodeID = &req.NodeID
	}
	for _, participant := range req.ParticipantIDs {
		id, err := uuid.Parse(participant)
		if err != nil {
			respond{}.error(c, apierrors.Fields("participantIdentityIds[]", "invalid"))
			return
		}
		draft.ParticipantIDs = append(draft.ParticipantIDs, id)
	}
	for _, raw := range req.Requirements {
		target, err := uuid.Parse(raw.TargetID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("requirements[].targetId", "invalid"))
			return
		}
		draft.Requirements = append(draft.Requirements, work.Requirement{
			Phase: raw.Phase, Kind: raw.Kind, TargetID: target, MaterialVersionID: raw.MaterialVersionID, Hard: raw.Hard, Label: raw.Label,
		})
	}
	task, err := h.Work.CreateTaskDraft(c.Request.Context(), p.UserID, projectID, draft)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	task, err = h.Work.GetTask(c.Request.Context(), p.UserID, projectID, task.ID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, task)
}

func (h *WorkHandlers) getTask(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("taskId", "uuid"))
		return
	}
	task, err := h.Work.GetTask(c.Request.Context(), p.UserID, projectID, taskID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, task)
}

func (h *WorkHandlers) getPlan(c *gin.Context) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	id, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("planId", "uuid"))
		return
	}
	plans, err := h.Work.ListPlans(c.Request.Context(), principalFrom(c).UserID, projectID, id)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	if len(plans) != 1 {
		respond{}.error(c, apierrors.New(apierrors.NotFound, "plan not found"))
		return
	}
	respond{}.ok(c, plans[0])
}

func (h *WorkHandlers) discardPlan(c *gin.Context) { h.discard(c, "plan", "planId") }
func (h *WorkHandlers) discardTask(c *gin.Context) { h.discard(c, "task", "taskId") }
func (h *WorkHandlers) discard(c *gin.Context, kind, param string) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	target, err := uuid.Parse(c.Param(param))
	if err != nil {
		respond{}.error(c, apierrors.Fields(param, "uuid"))
		return
	}
	var req struct {
		ExpectedVersion int64 `json:"expectedVersion"`
	}
	if err = bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if err = h.Work.Discard(c.Request.Context(), principalFrom(c).UserID, projectID, target, kind, req.ExpectedVersion); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"id": target, "status": "cancelled", "version": req.ExpectedVersion + 1})
}

func (h *WorkHandlers) respondTask(c *gin.Context, projectID, taskID uuid.UUID) {
	task, err := h.Work.GetTask(c.Request.Context(), principalFrom(c).UserID, projectID, taskID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, task)
}
