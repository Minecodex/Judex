// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/work"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// ResearchHandlers serves repositories, releases and fix propagation (06 §3/§5).
type ResearchHandlers struct {
	Work *work.Service
}

func NewResearchHandlers(workSvc *work.Service) *ResearchHandlers {
	return &ResearchHandlers{Work: workSvc}
}

func (h *ResearchHandlers) Register(spec *SpecRouter) {
	spec.Register("listRepositories", withAuth(h.listRepos))
	spec.Register("createRepository", withAuth(h.createRepo))
	spec.Register("updateRepository", withAuth(h.updateRepo))
	spec.Register("listReleaseReports", withAuth(h.listReleases))
	spec.Register("createReleaseReport", withAuth(h.createRelease))
	spec.Register("createFixPropagation", withAuth(h.fixPropagation))
	spec.Register("listAudit", withAuth(h.listAudit))
	spec.Register("listMyActions", withAuth(h.myActions))
}

func (h *ResearchHandlers) myActions(c *gin.Context) {
	p := principalFrom(c)
	var projectFilter *uuid.UUID
	if raw := c.Query("projectId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respond{}.error(c, apierrors.Fields("projectId", "invalid"))
			return
		}
		projectFilter = &id
	}
	actions, err := h.Work.MyActions(c.Request.Context(), p.UserID, projectFilter)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(actions, nil))
}

func (h *ResearchHandlers) listRepos(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	repos, err := h.Work.ListRepositories(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(repos, nil))
}

func (h *ResearchHandlers) createRepo(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		DisplayName  string `json:"displayName" binding:"required"`
		URL          string `json:"url" binding:"required"`
		Provider     string `json:"provider"`
		DefaultBranch string `json:"defaultBranch"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	repo, err := h.Work.CreateRepository(c.Request.Context(), p.UserID, projectID,
		req.DisplayName, req.URL, req.Provider, req.DefaultBranch)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, repo)
}

func (h *ResearchHandlers) updateRepo(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	repoID, err := uuid.Parse(c.Param("repositoryId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("repositoryId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		DisplayName     string `json:"displayName"`
		URL             string `json:"url"`
		Provider        string `json:"provider"`
		DefaultBranch   string `json:"defaultBranch"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	repo, err := h.Work.UpdateRepository(c.Request.Context(), p.UserID, projectID, repoID,
		req.ExpectedVersion, req.DisplayName, req.URL, req.Provider, req.DefaultBranch)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, repo)
}

func (h *ResearchHandlers) listReleases(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	releases, err := h.Work.ListReleases(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(releases, nil))
}

func (h *ResearchHandlers) createRelease(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		VersionLabel      string           `json:"versionLabel" binding:"required"`
		Environment       string           `json:"environment" binding:"required"`
		URL               string           `json:"url"`
		Status            string           `json:"status" binding:"required"`
		RepositoryCommits []map[string]any `json:"repositoryCommits"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	release, err := h.Work.ReportRelease(c.Request.Context(), p.UserID, projectID,
		req.VersionLabel, req.Environment, req.URL, req.Status, req.RepositoryCommits)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, release)
}

func (h *ResearchHandlers) fixPropagation(c *gin.Context) {
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
		TargetReleaseRefs []string `json:"targetReleaseRefs" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	targets, err := h.Work.CreateFixPropagation(c.Request.Context(), p.UserID, projectID, taskID, req.TargetReleaseRefs)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, gin.H{"targetTaskIds": targets})
}

func (h *ResearchHandlers) listAudit(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	entries, err := h.Work.ListAudit(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(entries, nil))
}
