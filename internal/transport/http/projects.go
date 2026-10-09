// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"context"
	"github.com/kakj-go/Judex/internal/platform/paging"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/agent"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
)

// ProjectHandlers serves the project lifecycle operations (06 §3).
type ProjectHandlers struct {
	Projects   *project.Service
	Work       *work.Service
	ModelCheck func(ctx context.Context, id uuid.UUID) error
	Models     func(ctx context.Context) ([]agent.PublicModel, error)
}

func NewProjectHandlers(svc *project.Service, modelCheck func(ctx context.Context, id uuid.UUID) error, models func(ctx context.Context) ([]agent.PublicModel, error)) *ProjectHandlers {
	return &ProjectHandlers{Projects: svc, ModelCheck: modelCheck, Models: models}
}

func (h *ProjectHandlers) Register(spec *SpecRouter) {
	spec.Register("listProjects", withAuth(h.list))
	spec.Register("listRecentTopics", withAuth(h.recentTopics))
	spec.Register("createProject", withAuth(h.create))
	spec.Register("getProject", withAuth(h.get))
	spec.Register("getProjectBootstrap", withAuth(h.bootstrap))
	spec.Register("archiveProject", withAuth(h.archive(true)))
	spec.Register("restoreProject", withAuth(h.archive(false)))
	spec.Register("updateProject", withAuth(h.patch))
	spec.Register("listModels", withAuth(h.listModels))
}

func (h *ProjectHandlers) list(c *gin.Context) {
	p := principalFrom(c)
	limit, afterTime, afterID := paging.Params(c.Request.Context())
	include := c.Query("include")
	if include != "" && include != "summary" {
		respond{}.error(c, apierrors.Fields("include", "invalid"))
		return
	}
	projects, more, total, err := h.Projects.ListOverview(c.Request.Context(), p.UserID, limit, afterTime, afterID, project.ListOptions{Query: c.Query("q"), Ownership: c.Query("ownership"), Summary: include == "summary"})
	if err != nil {
		respond{}.error(c, err)
		return
	}
	if more && len(projects) > 0 {
		last := projects[len(projects)-1]
		paging.SetNext(c.Request.Context(), last.CreatedAt, last.ID)
	}
	data := respond{}.list(c, projects, nil)
	data["totalCount"] = total
	respond{}.ok(c, data)
}

func (h *ProjectHandlers) recentTopics(c *gin.Context) {
	limit, _, _ := paging.Params(c.Request.Context())
	topics, err := h.Projects.RecentTopics(c.Request.Context(), principalFrom(c).UserID, limit)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"items": topics})
}

func (h *ProjectHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	var req struct {
		Title                  string `json:"title" binding:"required"`
		Description            string `json:"description"`
		Kind                   string `json:"kind"`
		MaxDiscussionRounds    int    `json:"maxDiscussionRounds"`
		ApprovalTimeoutSeconds int    `json:"approvalTimeoutSeconds"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	created, err := h.Projects.Create(c.Request.Context(), p.UserID, project.CreateRequest{
		Title: req.Title, Description: req.Description, Kind: req.Kind,
		MaxDiscussionRounds: req.MaxDiscussionRounds, ApprovalTimeoutSeconds: req.ApprovalTimeoutSeconds,
	})
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, created)
}

func (h *ProjectHandlers) get(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	projectValue, err := h.Projects.Get(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, projectValue)
}

func (h *ProjectHandlers) bootstrap(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	projectValue, err := h.Projects.Get(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var cursor int64
	if err := h.Projects.BootstrapCursor(c.Request.Context(), projectID, &cursor); err != nil {
		respond{}.error(c, err)
		return
	}
	identities, err := h.Projects.ListIdentities(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	pending := 0
	if h.Work != nil {
		actions, err := h.Work.MyActions(c.Request.Context(), p.UserID, &projectID)
		if err != nil {
			respond{}.error(c, err)
			return
		}
		pending = len(actions)
	}
	respond{}.ok(c, gin.H{
		"pendingActionsCount": pending,
		"project":             projectValue,
		"identities":          identities,
		"eventCursor":         cursor,
	})
}

func (h *ProjectHandlers) archive(archive bool) Handler {
	return func(c *gin.Context) {
		p := principalFrom(c)
		projectID, err := uuid.Parse(c.Param("projectId"))
		if err != nil {
			respond{}.error(c, apierrors.Fields("projectId", "invalid"))
			return
		}
		var req struct {
			ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
			Reason          string `json:"reason" binding:"required"`
		}
		if err := bindJSON(c, &req); err != nil {
			respond{}.error(c, err)
			return
		}
		updated, err := h.Projects.Archive(c.Request.Context(), p.UserID, projectID, req.ExpectedVersion, req.Reason, archive)
		if err != nil {
			respond{}.error(c, err)
			return
		}
		respond{}.ok(c, updated)
	}
}

func (h *ProjectHandlers) patch(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion        int64   `json:"expectedVersion" binding:"required"`
		Title                  *string `json:"title"`
		Description            *string `json:"description"`
		MaxDiscussionRounds    *int    `json:"maxDiscussionRounds"`
		ApprovalTimeoutSeconds *int    `json:"approvalTimeoutSeconds"`
		DefaultModelID         *string `json:"defaultModelId"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	update := project.UpdateRequest{
		Title: req.Title, Description: req.Description,
		MaxDiscussionRounds: req.MaxDiscussionRounds, ApprovalTimeoutSeconds: req.ApprovalTimeoutSeconds,
		DefaultModelSet: req.DefaultModelID != nil,
	}
	if req.DefaultModelID != nil && *req.DefaultModelID != "" {
		id, err := uuid.Parse(*req.DefaultModelID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("defaultModelId", "invalid"))
			return
		}
		update.DefaultModelID = &id
	}
	updated, err := h.Projects.Update(c.Request.Context(), p.UserID, projectID, req.ExpectedVersion, update, h.ModelCheck)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, updated)
}

func (h *ProjectHandlers) listModels(c *gin.Context) {
	models, err := h.Models(c.Request.Context())
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, models, nil))
}
