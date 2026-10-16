package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *WorkHandlers) acceptanceHistory(c *gin.Context) {
	project, err := projectParam(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	kind, key := "task", "taskId"
	if c.Param("planId") != "" {
		kind, key = "plan", "planId"
	}
	target, err := uuid.Parse(c.Param(key))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	items, err := h.Work.AcceptanceHistory(c.Request.Context(), principalFrom(c).UserID, project, target, kind)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, items, nil))
}
