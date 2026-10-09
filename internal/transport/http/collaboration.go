package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type CollaborationHandlers struct {
	Service    *collaboration.Service
	Discussion *discussion.Service
}

func NewCollaborationHandlers(s *collaboration.Service, d *discussion.Service) *CollaborationHandlers {
	return &CollaborationHandlers{s, d}
}
func (h *CollaborationHandlers) Register(spec *SpecRouter) {
	spec.Register("forkTopic", withAuth(h.fork))
	spec.Register("listTaskActivity", withAuth(h.activity))
	spec.Register("getPlanDiscussionSummary", withAuth(h.summary))
	spec.Register("listDiscussionSuggestions", withAuth(h.suggestions))
	spec.Register("getDiscussionSuggestion", withAuth(h.suggestion))
	spec.Register("resolveDiscussionSuggestion", withAuth(h.resolve))
	spec.Register("getTaskAnalysis", withAuth(h.analysis))
	spec.Register("retryTaskAnalysis", withAuth(h.retry))
}
func collaborationIDs(c *gin.Context, key string) (uuid.UUID, uuid.UUID, bool) {
	project, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "uuid"))
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		respond{}.error(c, apierrors.Fields(key, "uuid"))
		return uuid.Nil, uuid.Nil, false
	}
	return project, id, true
}
func (h *CollaborationHandlers) fork(c *gin.Context) {
	project, parent, ok := collaborationIDs(c, "topicId")
	if !ok {
		return
	}
	var in collaboration.ForkInput
	if err := bindJSON(c, &in); err != nil {
		respond{}.error(c, err)
		return
	}
	out, err := h.Discussion.Fork(c.Request.Context(), principalFrom(c).UserID, project, parent, in)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, out)
}
func (h *CollaborationHandlers) activity(c *gin.Context) {
	project, task, ok := collaborationIDs(c, "taskId")
	if !ok {
		return
	}
	out, err := h.Service.ListActivity(c.Request.Context(), principalFrom(c).UserID, project, task)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, out, nil))
}
func (h *CollaborationHandlers) summary(c *gin.Context) {
	project, plan, ok := collaborationIDs(c, "planId")
	if !ok {
		return
	}
	out, err := h.Service.PlanSummary(c.Request.Context(), principalFrom(c).UserID, project, plan)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
func (h *CollaborationHandlers) suggestions(c *gin.Context) {
	project, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	plan, err := optionalUUID(c.Query("planId"))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	task, err := optionalUUID(c.Query("taskId"))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	out, err := h.Service.ListSuggestions(c.Request.Context(), principalFrom(c).UserID, project, plan, task)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, out, nil))
}
func (h *CollaborationHandlers) suggestion(c *gin.Context) {
	project, id, ok := collaborationIDs(c, "suggestionId")
	if !ok {
		return
	}
	out, err := h.Service.GetSuggestion(c.Request.Context(), principalFrom(c).UserID, project, id)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
func (h *CollaborationHandlers) resolve(c *gin.Context) {
	project, id, ok := collaborationIDs(c, "suggestionId")
	if !ok {
		return
	}
	var in collaboration.ResolveInput
	if err := bindJSON(c, &in); err != nil {
		respond{}.error(c, err)
		return
	}
	out, err := h.Service.Resolve(c.Request.Context(), principalFrom(c).UserID, project, id, in)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
func (h *CollaborationHandlers) analysis(c *gin.Context) {
	project, id, ok := collaborationIDs(c, "analysisId")
	if !ok {
		return
	}
	out, err := h.Service.GetAnalysis(c.Request.Context(), principalFrom(c).UserID, project, id)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
func (h *CollaborationHandlers) retry(c *gin.Context) {
	project, id, ok := collaborationIDs(c, "analysisId")
	if !ok {
		return
	}
	out, err := h.Service.Retry(c.Request.Context(), principalFrom(c).UserID, project, id)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
