package httptransport

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/work"
)

type CooperationHandlers struct {
	Work       *work.Service
	Discussion *discussion.Service
}

func NewCooperationHandlers(w *work.Service, d *discussion.Service) *CooperationHandlers {
	return &CooperationHandlers{w, d}
}
func (h *CooperationHandlers) Register(spec *SpecRouter) {
	spec.Register("ensureTaskMainTopic", withAuth(h.mainTopic))
	spec.Register("listDeliveries", withAuth(h.deliveries))
}
func (h *CooperationHandlers) mainTopic(c *gin.Context) {
	project, task, ok := collaborationIDs(c, "taskId")
	if !ok {
		return
	}
	id, created, e := h.Work.EnsureTaskMainTopic(c.Request.Context(), principalFrom(c).UserID, project, task)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	topic, e := h.Discussion.GetTopic(c.Request.Context(), principalFrom(c).UserID, project, id)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	out := gin.H{"mainTopicId": id, "topic": topic}
	if created {
		respond{}.created(c, out)
	} else {
		respond{}.ok(c, out)
	}
}
func (h *CooperationHandlers) deliveries(c *gin.Context) {
	project, e := projectParam(c)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	out, e := h.Work.ListDeliveries(c.Request.Context(), principalFrom(c).UserID, project, work.DeliveryFilter{Type: c.Query("type"), Filter: c.Query("filter")})
	if e != nil {
		respond{}.error(c, e)
		return
	}
	respond{}.ok(c, respond{}.list(c, out, nil))
}
func (h *DiscussionHandlers) replaceLinks(c *gin.Context) {
	project, topic, ok := collaborationIDs(c, "topicId")
	if !ok {
		return
	}
	var in struct {
		ExpectedLinksVersion int64 `json:"expectedLinksVersion"`
		TargetRefs           []struct {
			ObjectType string `json:"objectType"`
			ObjectID   string `json:"objectId"`
		} `json:"targetRefs"`
	}
	if e := bindJSON(c, &in); e != nil {
		respond{}.error(c, e)
		return
	}
	links := []discussion.TopicLink{}
	for _, raw := range in.TargetRefs {
		id, e := uuid.Parse(raw.ObjectID)
		if e != nil {
			respond{}.error(c, apierrors.Fields("targetRefs.objectId", "uuid"))
			return
		}
		links = append(links, discussion.TopicLink{ObjectType: raw.ObjectType, ObjectID: id})
	}
	if e := h.Discussion.ReplaceTopicLinks(c.Request.Context(), principalFrom(c).UserID, project, topic, in.ExpectedLinksVersion, links); e != nil {
		respond{}.error(c, e)
		return
	}
	out, e := h.Discussion.GetTopic(c.Request.Context(), principalFrom(c).UserID, project, topic)
	if e != nil {
		respond{}.error(c, e)
		return
	}
	respond{}.ok(c, out)
}
