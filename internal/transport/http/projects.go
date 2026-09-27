// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/project"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// ProjectHandlers serves the project lifecycle operations (06 §3).
type ProjectHandlers struct {
	Projects *project.Service
}

func NewProjectHandlers(svc *project.Service) *ProjectHandlers { return &ProjectHandlers{Projects: svc} }

func (h *ProjectHandlers) Register(spec *SpecRouter) {
	spec.Register("listProjects", withAuth(h.list))
	spec.Register("createProject", withAuth(h.create))
	spec.Register("getProject", withAuth(h.get))
	spec.Register("getProjectBootstrap", withAuth(h.bootstrap))
	spec.Register("archiveProject", withAuth(h.archive(true)))
	spec.Register("restoreProject", withAuth(h.archive(false)))
}

func pageParams(c *gin.Context) (int, *time.Time, *uuid.UUID, error) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return 0, nil, nil, apierrors.Fields("limit", "range")
		}
		limit = n
	}
	var afterTime *time.Time
	var afterID *uuid.UUID
	if raw := c.Query("cursor"); raw != "" {
		t, id, err := decodeCursor(raw)
		if err != nil {
			return 0, nil, nil, apierrors.Fields("cursor", "invalid")
		}
		afterTime, afterID = t, id
	}
	return limit, afterTime, afterID, nil
}

// encodeCursor produces "createdAt,id" opaque-enough keyset cursors.
func encodeCursor(t time.Time, id uuid.UUID) string {
	return t.UTC().Format(time.RFC3339Nano) + "|" + id.String()
}

func decodeCursor(raw string) (*time.Time, *uuid.UUID, error) {
	for i := 0; i < len(raw); i++ {
		if raw[i] == '|' {
			t, err := time.Parse(time.RFC3339Nano, raw[:i])
			if err != nil {
				return nil, nil, err
			}
			id, err := uuid.Parse(raw[i+1:])
			if err != nil {
				return nil, nil, err
			}
			return &t, &id, nil
		}
	}
	return nil, nil, apierrors.Fields("cursor", "format")
}

func (h *ProjectHandlers) list(c *gin.Context) {
	p := principalFrom(c)
	limit, afterTime, afterID, err := pageParams(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	projects, more, err := h.Projects.ListForUser(c.Request.Context(), p.UserID, limit, afterTime, afterID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var next *string
	if more && len(projects) > 0 {
		last := projects[len(projects)-1]
		cur := encodeCursor(last.CreatedAt, last.ID)
		next = &cur
	}
	respond{}.ok(c, respond{}.list(projects, next))
}

func (h *ProjectHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	var req struct {
		Title                 string `json:"title" binding:"required"`
		Description           string `json:"description"`
		Kind                  string `json:"kind"`
		MaxDiscussionRounds   int    `json:"maxDiscussionRounds"`
		ApprovalTimeoutSeconds int   `json:"approvalTimeoutSeconds"`
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
	respond{}.ok(c, gin.H{
		"project": projectValue,
		"identities": []any{},
		"eventCursor": cursor,
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
