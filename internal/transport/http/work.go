// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/work"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
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
	respond{}.ok(c, respond{}.list(plans, nil))
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
	respond{}.ok(c, respond{}.list(tasks, nil))
}

func (h *WorkHandlers) createTask(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Title              string `json:"title" binding:"required"`
		PlanID             string `json:"planId"`
		ParentTaskID       string `json:"parentTaskId"`
		ExpectedOutput     string `json:"expectedOutput"`
		AcceptanceCriteria string `json:"acceptanceCriteria"`
		Kind               string `json:"kind"`
		ReviewerIdentityID string `json:"reviewerIdentityId"`
		WorkflowID         string `json:"workflowId"`
		NodeID             string `json:"nodeId"`
		ParticipantIDs     []struct {
			IdentityID string `json:"identityId"`
		} `json:"participants"`
		Requirements []struct {
			Phase    string `json:"phase"`
			Kind     string `json:"kind"`
			TargetID string `json:"targetId"`
			Hard     bool   `json:"hard"`
			Label    string `json:"label"`
		} `json:"requirements"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	draft := work.TaskDraft{
		Title: req.Title, ExpectedOutput: req.ExpectedOutput,
		AcceptanceCriteria: req.AcceptanceCriteria, Kind: req.Kind,
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
		id, err := uuid.Parse(participant.IdentityID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("participants[].identityId", "invalid"))
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
			Phase: raw.Phase, Kind: raw.Kind, TargetID: target, Hard: raw.Hard, Label: raw.Label,
		})
	}
	task, err := h.Work.CreateTaskDraft(c.Request.Context(), p.UserID, projectID, draft)
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
	tasks, err := h.Work.ListTasks(c.Request.Context(), p.UserID, projectID, nil)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("taskId", "invalid"))
		return
	}
	for _, task := range tasks {
		if task.ID == taskID {
			respond{}.ok(c, task)
			return
		}
	}
	respond{}.error(c, apierrors.New(apierrors.NotFound, "task not found"))
}

func (h *WorkHandlers) getPlan(c *gin.Context) {
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
	planID, err := uuid.Parse(c.Param("planId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("planId", "invalid"))
		return
	}
	for _, plan := range plans {
		if plan.ID == planID {
			respond{}.ok(c, plan)
			return
		}
	}
	respond{}.error(c, apierrors.New(apierrors.NotFound, "plan not found"))
}

func (h *WorkHandlers) discardPlan(c *gin.Context) {
	respond{}.error(c, apierrors.New(apierrors.NotImplemented, "plan discard lands with P3-03 proposals"))
}

func (h *WorkHandlers) discardTask(c *gin.Context) {
	respond{}.error(c, apierrors.New(apierrors.NotImplemented, "task discard lands with P3-03 proposals"))
}
