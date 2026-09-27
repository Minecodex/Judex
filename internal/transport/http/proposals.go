// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/decision"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// ProposalHandlers serves proposal operations (06 §5).
type ProposalHandlers struct {
	Decisions *decision.Service
}

func NewProposalHandlers(svc *decision.Service) *ProposalHandlers {
	return &ProposalHandlers{Decisions: svc}
}

func (h *ProposalHandlers) Register(spec *SpecRouter) {
	spec.Register("listProposals", withAuth(h.list))
	spec.Register("createProposal", withAuth(h.create))
	spec.Register("updateProposalDraft", withAuth(h.updateDraft))
	spec.Register("submitProposal", withAuth(h.submit))
	spec.Register("getProposalReview", withAuth(h.review))
	spec.Register("decideProposal", withAuth(h.decide))
	spec.Register("delegateProposalDecision", withAuth(h.delegate))
	spec.Register("createProposalRevision", withAuth(h.revision))
}

func (h *ProposalHandlers) list(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposals, err := h.Decisions.ListProposals(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(proposals, nil))
}

func bindChanges(c *gin.Context) ([]decision.Change, error) {
	var req struct {
		Kind    string `json:"kind" binding:"required"`
		TopicID string `json:"topicId"`
		Reason  string `json:"reason"`
		Changes []struct {
			Operation       string         `json:"operation"`
			TargetType      string         `json:"targetType"`
			TargetID        string         `json:"targetId"`
			ClientRef       string         `json:"clientRef"`
			ExpectedVersion int64          `json:"expectedVersion"`
			Fields          map[string]any `json:"fields"`
		} `json:"changes" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		return nil, err
	}
	changes := make([]decision.Change, 0, len(req.Changes))
	for _, raw := range req.Changes {
		changes = append(changes, decision.Change{
			Operation: raw.Operation, TargetType: raw.TargetType, TargetID: raw.TargetID,
			ClientRef: raw.ClientRef, ExpectedVersion: raw.ExpectedVersion, Fields: raw.Fields,
		})
	}
	c.Set("proposal_kind", req.Kind)
	c.Set("proposal_topic", req.TopicID)
	c.Set("proposal_reason", req.Reason)
	return changes, nil
}

func (h *ProposalHandlers) create(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	changes, err := bindChanges(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var topicID *uuid.UUID
	if raw := c.GetString("proposal_topic"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respond{}.error(c, apierrors.Fields("topicId", "invalid"))
			return
		}
		topicID = &id
	}
	id, err := h.Decisions.CreateDraft(c.Request.Context(), p.UserID, projectID,
		c.GetString("proposal_kind"), topicID, c.GetString("proposal_reason"), changes)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, gin.H{"id": id, "status": "draft", "version": 1})
}

func (h *ProposalHandlers) updateDraft(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposalID, err := uuid.Parse(c.Param("proposalId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("proposalId", "invalid"))
		return
	}
	_ = p
	_ = projectID
	_ = proposalID
	respond{}.error(c, apierrors.New(apierrors.NotImplemented, "draft 更新走 create-revision 语义：废弃后重建草稿"))
}

func (h *ProposalHandlers) submit(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposalID, err := uuid.Parse(c.Param("proposalId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("proposalId", "invalid"))
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
	if _, err := h.Decisions.Submit(c.Request.Context(), p.UserID, projectID, proposalID, req.ExpectedVersion, req.DraftHash); err != nil {
		respond{}.error(c, err)
		return
	}
	review, err := h.Decisions.GetReview(c.Request.Context(), p.UserID, projectID, proposalID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, review)
}

func (h *ProposalHandlers) review(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposalID, err := uuid.Parse(c.Param("proposalId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("proposalId", "invalid"))
		return
	}
	review, err := h.Decisions.GetReview(c.Request.Context(), p.UserID, projectID, proposalID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, review)
}

func (h *ProposalHandlers) decide(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposalID, err := uuid.Parse(c.Param("proposalId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("proposalId", "invalid"))
		return
	}
	var req struct {
		ReviewID   string `json:"reviewId" binding:"required"`
		ReviewHash string `json:"reviewHash" binding:"required"`
		Decision   string `json:"decision" binding:"required"`
		Reason     string `json:"reason"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	if req.Decision != "approve" && req.Decision != "reject" {
		respond{}.error(c, apierrors.Fields("decision", "enum"))
		return
	}
	if req.Decision == "reject" && req.Reason == "" {
		respond{}.error(c, apierrors.Fields("reason", "required"))
		return
	}
	result, err := h.Decisions.Decide(c.Request.Context(), p.UserID, projectID, proposalID,
		req.ReviewHash, req.Decision == "approve", req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, result)
}

func (h *ProposalHandlers) delegate(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposalID, err := uuid.Parse(c.Param("proposalId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("proposalId", "invalid"))
		return
	}
	var req struct {
		ReviewID   string `json:"reviewId" binding:"required"`
		ReviewHash string `json:"reviewHash" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	result, err := h.Decisions.Delegate(c.Request.Context(), p.UserID, projectID, proposalID, req.ReviewHash, req.Reason)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, result)
}

func (h *ProposalHandlers) revision(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	proposalID, err := uuid.Parse(c.Param("proposalId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("proposalId", "invalid"))
		return
	}
	changes, err := bindChanges(c)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	revision, err := h.Decisions.CreateRevision(c.Request.Context(), p.UserID, projectID, proposalID,
		changes, c.GetString("proposal_reason"))
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, gin.H{"id": proposalID, "revision": revision, "status": "draft"})
}
