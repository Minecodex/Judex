package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

func (h *PositionHandlers) getPositionPresetCatalog(c *gin.Context) {
	respond{}.ok(c, project.BuiltinPositionCatalog())
}

func (h *PositionHandlers) importPositionPresets(c *gin.Context) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, errors.Fields("projectId", "invalid"))
		return
	}
	var request project.ImportPositionPresetsRequest
	if err := bindJSON(c, &request); err != nil {
		respond{}.error(c, err)
		return
	}
	result, err := h.Projects.ImportPositionPresets(c.Request.Context(), principalFrom(c).UserID, projectID, request)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, result)
}
