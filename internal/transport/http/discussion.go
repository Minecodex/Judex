// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/kakj-go/Judex/internal/platform/paging"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// DiscussionHandlers serves topics, messages and submissions (06 §4).
type DiscussionHandlers struct {
	Discussion *discussion.Service
}

func NewDiscussionHandlers(svc *discussion.Service) *DiscussionHandlers {
	return &DiscussionHandlers{Discussion: svc}
}

func (h *DiscussionHandlers) Register(spec *SpecRouter) {
	spec.Register("listTopics", withAuth(h.listTopics))
	spec.Register("createTopic", withAuth(h.createTopic))
	spec.Register("getTopic", withAuth(h.getTopic))
	spec.Register("listMessages", withAuth(h.listMessages))
	spec.Register("linkTopic", withAuth(h.linkTopic))
	spec.Register("createSubmission", withAuth(h.createSubmission))
}

func (h *DiscussionHandlers) listTopics(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	topics, err := h.Discussion.ListTopics(c.Request.Context(), p.UserID, projectID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, respond{}.list(c, topics, nil))
}

func (h *DiscussionHandlers) createTopic(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		Title          string `json:"title" binding:"required"`
		InitialMessage string `json:"initialMessage"`
		Links          []struct {
			ObjectType string `json:"objectType"`
			ObjectID   string `json:"objectId"`
		} `json:"links"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	links := make([]discussion.TopicLink, 0, len(req.Links))
	for _, raw := range req.Links {
		id, err := uuid.Parse(raw.ObjectID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("links[].objectId", "invalid"))
			return
		}
		links = append(links, discussion.TopicLink{ObjectType: raw.ObjectType, ObjectID: id})
	}
	topic, err := h.Discussion.CreateTopic(c.Request.Context(), p.UserID, projectID, req.Title, req.InitialMessage, links)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, topic)
}

func (h *DiscussionHandlers) getTopic(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	topicID, err := uuid.Parse(c.Param("topicId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("topicId", "invalid"))
		return
	}
	topic, err := h.Discussion.GetTopic(c.Request.Context(), p.UserID, projectID, topicID)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, topic)
}

func (h *DiscussionHandlers) listMessages(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	topicID, err := uuid.Parse(c.Param("topicId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("topicId", "invalid"))
		return
	}
	beforeSeq := int64(0)
	if raw := c.Query("beforeSeq"); raw != "" {
		beforeSeq, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || beforeSeq < 0 {
			respond{}.error(c, apierrors.Fields("beforeSeq", "invalid"))
			return
		}
	}
	afterSeq := int64(0)
	if raw := c.Query("afterSeq"); raw != "" {
		afterSeq, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || afterSeq < 0 || beforeSeq > 0 {
			respond{}.error(c, apierrors.Fields("afterSeq", "range or conflicting beforeSeq"))
			return
		}
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > 100 {
			respond{}.error(c, apierrors.Fields("limit", "range"))
			return
		}
	}
	scope := c.Request.URL.Path + ":" + p.UserID.String()
	if raw := c.Query("cursor"); raw != "" {
		if beforeSeq > 0 || afterSeq > 0 {
			respond{}.error(c, apierrors.Fields("cursor", "cannot combine with sequence bounds"))
			return
		}
		cursor, err := paging.Sequence(scope, raw)
		if err != nil {
			respond{}.error(c, err)
			return
		}
		if cursor.After {
			afterSeq = cursor.Seq
		} else {
			beforeSeq = cursor.Seq
		}
	}
	messages, err := h.Discussion.ListMessages(c.Request.Context(), p.UserID, projectID, topicID, beforeSeq, limit+1, afterSeq)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	var next *string
	if len(messages) > limit {
		if afterSeq > 0 {
			messages = messages[:limit]
			next = paging.NextSequence(scope, messages[len(messages)-1].Seq, true)
		} else {
			messages = messages[1:]
			next = paging.NextSequence(scope, messages[0].Seq, false)
		}
	}
	respond{}.ok(c, respond{}.list(c, messages, next))
}

func (h *DiscussionHandlers) linkTopic(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	topicID, err := uuid.Parse(c.Param("topicId"))
	if err != nil {
		respond{}.error(c, apierrors.Fields("topicId", "invalid"))
		return
	}
	var req struct {
		ExpectedVersion int64 `json:"expectedVersion" binding:"required"`
		TargetRefs      []struct {
			ObjectType string `json:"objectType"`
			ObjectID   string `json:"objectId"`
		} `json:"targetRefs" binding:"required"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	links := make([]discussion.TopicLink, 0, len(req.TargetRefs))
	for _, raw := range req.TargetRefs {
		id, err := uuid.Parse(raw.ObjectID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("targetRefs[].objectId", "invalid"))
			return
		}
		links = append(links, discussion.TopicLink{ObjectType: raw.ObjectType, ObjectID: id})
	}
	if err := h.Discussion.LinkTopic(c.Request.Context(), p.UserID, projectID, topicID, links); err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"linked": true})
}

func (h *DiscussionHandlers) createSubmission(c *gin.Context) {
	p := principalFrom(c)
	projectID, err := projectParam(c)
	if err != nil {
		respond{}.error(c, apierrors.Fields("projectId", "invalid"))
		return
	}
	var req struct {
		ClientSubmissionID  string   `json:"clientSubmissionId" binding:"required"`
		ExpectedTaskVersion int64    `json:"expectedTaskVersion"`
		Purpose             string   `json:"purpose" binding:"required"`
		Text                string   `json:"text" binding:"required"`
		TopicID             string   `json:"topicId"`
		TaskID              string   `json:"taskId"`
		IdentityID          string   `json:"identityId"`
		MaterialVersionIDs  []string `json:"materialVersionIds"`
	}
	if err := bindJSON(c, &req); err != nil {
		respond{}.error(c, err)
		return
	}
	sub := discussion.Submission{
		ClientSubmissionID:  req.ClientSubmissionID,
		ExpectedTaskVersion: req.ExpectedTaskVersion,
		Purpose:             req.Purpose,
		Source:              "web",
		Text:                req.Text,
		MaterialVersionIDs:  req.MaterialVersionIDs,
	}
	if req.TopicID != "" {
		id, err := uuid.Parse(req.TopicID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("topicId", "invalid"))
			return
		}
		sub.TopicID = &id
	}
	if req.TaskID != "" {
		id, err := uuid.Parse(req.TaskID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("taskId", "invalid"))
			return
		}
		sub.TaskID = &id
	}
	if req.IdentityID != "" {
		id, err := uuid.Parse(req.IdentityID)
		if err != nil {
			respond{}.error(c, apierrors.Fields("identityId", "invalid"))
			return
		}
		sub.IdentityID = &id
	}
	if p.Kind == "cli" {
		sub.Source = "cli"
	}
	created, err := h.Discussion.CreateSubmission(c.Request.Context(), p.UserID, projectID, sub)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.created(c, created)
}
