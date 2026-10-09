// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

// MemberHandlers serves membership and owner-transfer operations (06 §3).
type MemberHandlers struct {
	Projects *project.Service
}

func NewMemberHandlers(svc *project.Service) *MemberHandlers { return &MemberHandlers{Projects: svc} }

func (h *MemberHandlers) Register(spec *SpecRouter) {
	spec.Register("listMembers", withAuth(h.list))
	spec.Register("updateMemberRole", withAuth(h.updateRole))
	spec.Register("removeMember", withAuth(h.remove))
	spec.Register("leaveProject", withAuth(h.leave))
	spec.Register("requestOwnerTransfer", withAuth(h.requestTransfer))
	spec.Register("getOwnerTransfer", withAuth(h.getTransfer))
	spec.Register("decideOwnerTransfer", withAuth(h.decideTransfer))
}

func (h *MemberHandlers) list(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	members, err := h.Projects.ListMembers(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, members, nil))
}

func (h *MemberHandlers) updateRole(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	target, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("userId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		Role            string `json:"role" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	member, err := h.Projects.UpdateMemberRole(c.Request.Context(), p.UserID, projectID, target, req.ExpectedVersion, req.Role)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, member)
}

func (h *MemberHandlers) remove(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	target, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("userId", "invalid"))
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
	affected, err := h.Projects.RemoveMember(c.Request.Context(), p.UserID, projectID, target, req.ExpectedVersion, req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"affectedResponsibilities": affected})
}

func (h *MemberHandlers) leave(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64 `json:"expectedVersion" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if err := h.Projects.LeaveProject(c.Request.Context(), p.UserID, projectID, req.ExpectedVersion); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"left": true})
}

func (h *MemberHandlers) requestTransfer(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		TargetUserID    uuid.UUID `json:"targetUserId" binding:"required"`
		ExpectedVersion int64     `json:"expectedVersion" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	transfer, err := h.Projects.RequestOwnerTransfer(c.Request.Context(), p.UserID, projectID, req.TargetUserID, req.ExpectedVersion)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, transfer)
}

func (h *MemberHandlers) getTransfer(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	transferID, err := uuid.Parse(c.Param("transferId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("transferId", "invalid"))
		return
	}
	transfer, err := h.Projects.GetOwnerTransfer(c.Request.Context(), p.UserID, projectID, transferID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, transfer)
}

func (h *MemberHandlers) decideTransfer(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	transferID, err := uuid.Parse(c.Param("transferId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("transferId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		Decision        string `json:"decision" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "accept" && req.Decision != "decline" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	transfer, err := h.Projects.DecideOwnerTransfer(c.Request.Context(), p.UserID, projectID, transferID, req.Decision == "accept")
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, transfer)
}
