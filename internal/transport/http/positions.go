// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

// PositionHandlers serves positions, identities, invitations and personal
// preferences (06 §3).
type PositionHandlers struct {
	Projects *project.Service
}

func NewPositionHandlers(svc *project.Service) *PositionHandlers {
	return &PositionHandlers{Projects: svc}
}

func (h *PositionHandlers) Register(spec *SpecRouter) {
	spec.Register("getPositionPresetCatalog", withAuth(h.getPositionPresetCatalog))
	spec.Register("importPositionPresets", withAuth(h.importPositionPresets))
	spec.Register("listPositions", withAuth(h.listPositions))
	spec.Register("createPosition", withAuth(h.createPosition))
	spec.Register("updatePosition", withAuth(h.updatePosition))
	spec.Register("listIdentities", withAuth(h.listIdentities))
	spec.Register("createIdentity", withAuth(h.createIdentity))
	spec.Register("replaceIdentityBinding", withAuth(h.replaceIdentity))
	spec.Register("listInvitations", withAuth(h.listInvitations))
	spec.Register("createInvitation", withAuth(h.createInvitation))
	spec.Register("revokeInvitation", withAuth(h.revokeInvitation))
	spec.Register("listMyInvitations", withAuth(h.listMyInvitations))
	spec.Register("resolveInvitation", withAuth(h.resolveInvitation))
	spec.Register("acceptInvitation", withAuth(h.acceptInvitation))
	spec.Register("declineInvitation", withAuth(h.declineInvitation))
	spec.Register("getMyPreferences", withAuth(h.getPreferences))
	spec.Register("updateMyPreferences", withAuth(h.updatePreferences))
}

func (h *PositionHandlers) listPositions(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	positions, err := h.Projects.ListPositions(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, positions, nil))
}

type positionRequest struct {
	Name          string `json:"name" binding:"required"`
	Prompt        string `json:"prompt"`
	PublicSummary string `json:"publicSummary"`
	ModelID       string `json:"modelId"`
	NodeBindings  []struct {
		WorkflowID string `json:"workflowId"`
		NodeID     string `json:"nodeId"`
	} `json:"nodeBindings"`
}

func (r positionRequest) draft() (project.PositionDraft, error) {
	draft := project.PositionDraft{
		Name: r.Name, Prompt: r.Prompt, PublicSummary: r.PublicSummary,
	}
	if r.ModelID != "" {
		id, err := uuid.Parse(r.ModelID)
		if err != nil {
			return draft, apierrors.Fields("modelId", "invalid")
		}
		draft.ModelID = &id
	}
	for _, b := range r.NodeBindings {
		wid, err := uuid.Parse(b.WorkflowID)
		if err != nil {
			return draft, apierrors.Fields("nodeBindings[].workflowId", "invalid")
		}
		draft.NodeBindings = append(draft.NodeBindings, project.NodeBinding{WorkflowID: wid, NodeID: b.NodeID})
	}
	return draft, nil
}

func (h *PositionHandlers) createPosition(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req positionRequest
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	draft, err := req.draft()
	if err != nil {
		respond{}.error(c, err)
		return
	}
	position, err := h.Projects.CreatePosition(c.Request.Context(), p.UserID, projectID, draft)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, position)
}

func (h *PositionHandlers) updatePosition(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	positionID, err := uuid.Parse(c.Param("positionId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("positionId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64 `json:"expectedVersion" binding:"required"`
		positionRequest
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	draft, err := req.draft()
	if err != nil {
		respond{}.error(c, err)
		return
	}
	position, err := h.Projects.UpdatePosition(c.Request.Context(), p.UserID, projectID, positionID, req.ExpectedVersion, draft)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, position)
}

func (h *PositionHandlers) listIdentities(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	identities, err := h.Projects.ListIdentities(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, identities, nil))
}

func (h *PositionHandlers) createIdentity(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		PositionID string `json:"positionId" binding:"required"`
		UserID     string `json:"userId" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	positionID, err := uuid.Parse(req.PositionID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("positionId", "invalid"))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("userId", "invalid"))
		return
	}
	identity, err := h.Projects.CreateIdentity(c.Request.Context(), p.UserID, projectID, positionID, userID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, identity)
}

func (h *PositionHandlers) replaceIdentity(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	identityID, err := uuid.Parse(c.Param("identityId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("identityId", "invalid"))
		return
	}
	var req struct {
		ExpectedBindingVersion int64  `json:"expectedBindingVersion" binding:"required"`
		NewUserID              string `json:"newUserId" binding:"required"`
		Reason                 string `json:"reason" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	newUser, err := uuid.Parse(req.NewUserID)
	if err != nil {
		respond{}.error(c, apierrors.Fields("newUserId", "invalid"))
		return
	}
	identity, err := h.Projects.ReplaceIdentityBinding(c.Request.Context(), p.UserID, projectID, identityID, newUser, req.ExpectedBindingVersion, req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, identity)
}

func (h *PositionHandlers) listInvitations(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	invitations, err := h.Projects.ListInvitations(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, invitations, nil))
}

func (h *PositionHandlers) createInvitation(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		TargetEmail string   `json:"targetEmail" binding:"required"`
		PositionIDs []string `json:"positionIds" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	positions := make([]uuid.UUID, 0, len(req.PositionIDs))
	for _, raw := range req.PositionIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			respond{}.error(c, apierrors.Fields("positionIds", "invalid"))
			return
		}
		positions = append(positions, id)
	}
	invitation, err := h.Projects.CreateInvitation(c.Request.Context(), p.UserID, projectID, req.TargetEmail, positions)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, gin.H{
		"id": invitation.ID, "targetEmail": invitation.TargetEmail, "state": invitation.State,
		"expiresAt": invitation.ExpiresAt,
		"inviteUrl": "/invite/" + invitation.Token,
	})
}

func (h *PositionHandlers) revokeInvitation(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	invitationID, err := uuid.Parse(c.Param("invitationId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("invitationId", "invalid"))
		return
	}
	if err := h.Projects.RevokeInvitation(c.Request.Context(), p.UserID, projectID, invitationID); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"revoked": true})
}

func (h *PositionHandlers) listMyInvitations(c *gin.Context) {
	p := principalFrom(c)
	invitations, err := h.Projects.ListMyInvitations(c.Request.Context(), p.UserID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, invitations, nil))
}

func (h *PositionHandlers) resolveInvitation(c *gin.Context) {
	p := principalFrom(c)
	token := c.Query("token")
	invitation, email, err := h.Projects.ResolveInvitationByToken(c.Request.Context(), p.UserID, token)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{
		"id": invitation.ID, "targetEmail": invitation.TargetEmail, "state": invitation.State,
		"expiresAt": invitation.ExpiresAt, "emailMatches": email == invitation.TargetEmail,
		"projectTitle": invitation.ProjectTitle, "positionNames": invitation.PositionNames,
	})
}

func (h *PositionHandlers) acceptInvitation(c *gin.Context) {
	p := principalFrom(c)
	invitationID, err := uuid.Parse(c.Param("invitationId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("invitationId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64  `json:"expectedVersion" binding:"required"`
		Token           string `json:"token"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	identities, projectID, err := h.Projects.AcceptInvitation(c.Request.Context(), p.UserID, invitationID, req.Token)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"projectId": projectID, "identities": identities})
}

func (h *PositionHandlers) declineInvitation(c *gin.Context) {
	p := principalFrom(c)
	invitationID, err := uuid.Parse(c.Param("invitationId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("invitationId", "invalid"))
		return
	}
	if err := h.Projects.DeclineInvitation(c.Request.Context(), p.UserID, invitationID); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"declined": true})
}

func (h *PositionHandlers) getPreferences(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	preferences, err := h.Projects.GetMyPreferences(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"items": preferences})
}

func (h *PositionHandlers) updatePreferences(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		PositionID       uuid.UUID `json:"positionId"`
		ExpectedRevision int64     `json:"expectedRevision" binding:"required"`
		Prompt           string    `json:"prompt"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	preferences, err := h.Projects.UpdateMyPreferences(c.Request.Context(), p.UserID, projectID, req.PositionID, req.ExpectedRevision, req.Prompt)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, preferences)
}
