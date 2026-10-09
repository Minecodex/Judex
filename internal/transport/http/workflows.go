// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project/catalog"
	"github.com/kakj-go/Judex/internal/workflow"
)

// WorkflowHandlers serves workflow draft/publish operations (06 §3).
type WorkflowHandlers struct {
	Workflows *workflow.Service
}

func NewWorkflowHandlers(svc *workflow.Service) *WorkflowHandlers {
	return &WorkflowHandlers{Workflows: svc}
}

func (h *WorkflowHandlers) Register(spec *SpecRouter) {
	spec.Register("listWorkflows", withAuth(h.list))
	spec.Register("createWorkflow", withAuth(h.create))
	spec.Register("listWorkflowVersions", withAuth(h.versions))
	spec.Register("updateWorkflowDraft", withAuth(h.updateDraft))
	spec.Register("publishWorkflow", withAuth(h.publish))
	spec.Register("getWorkflowPresetCatalog", withAuth(h.presets))
	spec.Register("importWorkflowPresets", withAuth(h.importPresets))
}

func (h *WorkflowHandlers) presets(c *gin.Context) {
	respond{}.ok(c, catalog.Workflows())
}

func (h *WorkflowHandlers) importPresets(c *gin.Context) {
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var request workflow.ImportPresetsRequest
	if err = bindJSON(c, &request); err != nil {
		respond{}.error(c, err)
		return
	}
	result, err := h.Workflows.ImportPresets(c.Request.Context(), principalFrom(c).UserID, projectID, request)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, result)
}

func projectParam(c *gin.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("projectId"))
}

func bindBody(c *gin.Context) (workflow.Body, error) {
	var body workflow.Body
	if err := bindJSON(c, &body); err != nil {
		return body, err
	}
	return body, nil
}

func (h *WorkflowHandlers) list(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	definitions, err := h.Workflows.List(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, definitions, nil))
}

func (h *WorkflowHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	body, err := bindBody(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	definition, err := h.Workflows.Create(c.Request.Context(), p.UserID, projectID, body)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, definition)
}

func (h *WorkflowHandlers) versions(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("workflowId", "invalid"))
		return
	}
	versions, err := h.Workflows.ListVersions(c.Request.Context(), p.UserID, projectID, workflowID, true)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, versions, nil))
}

func (h *WorkflowHandlers) updateDraft(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("workflowId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64 `json:"expectedVersion" binding:"required"`
		Body            workflow.Body
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	version, err := h.Workflows.UpdateDraft(c.Request.Context(), p.UserID, projectID, workflowID, req.ExpectedVersion, req.Body)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, version)
}

func (h *WorkflowHandlers) publish(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	workflowID, err := uuid.Parse(c.Param("workflowId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("workflowId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		DraftHash       string `json:"draftHash"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	version, err := h.Workflows.Publish(c.Request.Context(), p.UserID, projectID, workflowID, req.ExpectedVersion, req.DraftHash)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, version)
}
