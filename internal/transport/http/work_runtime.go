package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
	"log/slog"
)

func workTarget(c *gin.Context) (uuid.UUID, uuid.UUID, string, error) {
	project, e := projectParam(c)
	if e != nil {
		return uuid.Nil, uuid.Nil, "", e
	}
	kind, param := "plan", "planId"
	if c.Param("taskId") != "" {
		kind, param = "task", "taskId"
	}
	target, e := uuid.Parse(c.Param(param))
	return project, target, kind, e
}
func (h *WorkHandlers) updateDraft(c *gin.Context) {
	project, target, kind, e := workTarget(c)
	if e != nil {
		respond{}.error(c, apierrors.Fields("target", "uuid"))
		return
	}
	var in work.DraftUpdate
	if e = bindJSON(c, &in); e != nil {
		respond{}.error(c, e)
		return
	}
	if e = h.Work.UpdateDraft(c.Request.Context(), principalFrom(c).UserID, project, target, kind, in); e != nil {
		respond{}.error(c, e)
		return
	}
	h.respondWork(c, project, target, kind)
}
func (h *WorkHandlers) draftDiscardReview(c *gin.Context) {
	project, target, kind, e := workTarget(c)
	if e != nil {
		respond{}.error(c, apierrors.Fields("target", "uuid"))
		return
	}
	result, e := h.Work.DiscardReview(c.Request.Context(), principalFrom(c).UserID, project, target, kind)
	if e != nil {
		if apierrors.From(e).Code == apierrors.Internal {
			slog.ErrorContext(c.Request.Context(), "draft discard review failed", "requestId", c.GetString(requestIDKey), "error", e)
		}
		respond{}.error(c, e)
		return
	}
	respond{}.ok(c, result)
}
func (h *WorkHandlers) executionReview(c *gin.Context) {
	project, target, _, e := workTarget(c)
	if e != nil {
		respond{}.error(c, apierrors.Fields("target", "uuid"))
		return
	}
	result, e := h.Work.ExecutionExceptionReview(c.Request.Context(), principalFrom(c).UserID, project, target, c.DefaultQuery("operation", "skip"))
	if e != nil {
		respond{}.error(c, e)
		return
	}
	respond{}.ok(c, result)
}
func (h *WorkHandlers) executionSkip(c *gin.Context)    { h.executionChange(c, "skip") }
func (h *WorkHandlers) executionRestore(c *gin.Context) { h.executionChange(c, "restore") }
func (h *WorkHandlers) executionChange(c *gin.Context, operation string) {
	if !principalFrom(c).CanConfirmHumanDecision() {
		respond{}.error(c, apierrors.New(apierrors.HumanConfirmNeeded, "execution exception requires browser confirmation"))
		return
	}
	project, target, _, e := workTarget(c)
	if e != nil {
		respond{}.error(c, apierrors.Fields("target", "uuid"))
		return
	}
	var in work.ExceptionCommand
	if e = bindJSON(c, &in); e != nil {
		respond{}.error(c, e)
		return
	}
	if e = h.Work.ChangeExecutionException(c.Request.Context(), principalFrom(c).UserID, project, target, operation, in); e != nil {
		respond{}.error(c, e)
		return
	}
	h.respondTask(c, project, target)
}
func (h *WorkHandlers) respondWork(c *gin.Context, project, target uuid.UUID, kind string) {
	if kind == "task" {
		h.respondTask(c, project, target)
		return
	}
	items, e := h.Work.ListPlans(c.Request.Context(), principalFrom(c).UserID, project, target)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	if len(items) == 0 {
		respond{}.error(c, apierrors.New(apierrors.NotFound, "plan not found"))
		return
	}
	respond{}.ok(c, items[0])
}
