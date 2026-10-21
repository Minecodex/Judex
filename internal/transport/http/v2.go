package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/collab"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type V2Handlers struct {
	Service       *collab.Service
	PreviewOrigin string
}

func v2id(c *gin.Context, name string) uuid.UUID { id, _ := uuid.Parse(c.Param(name)); return id }
func v2query(c *gin.Context, key string) int64 {
	n, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return n
}
func v2result[T any](c *gin.Context, out T, err error) {
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, out)
}
func v2list(c *gin.Context, out []json.RawMessage, err error) {
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"items": out})
}
func v2done(c *gin.Context, err error) {
	if err != nil {
		respond{}.error(c, err)
		return
	}
	respond{}.ok(c, gin.H{"changed": true})
}
func v2body[T any](c *gin.Context, fn func(T)) {
	var in T
	if err := bindJSON(c, &in); err != nil {
		respond{}.error(c, err)
		return
	}
	fn(in)
}
func (h V2Handlers) Register(spec *SpecRouter) {
	s := h.Service
	spec.ResourceCheck = func(c *gin.Context, op string, raw []byte) error {
		return s.CheckOperation(c.Request.Context(), op, v2id(c, "projectId"), v2id(c, "conversationId"), v2id(c, "groupId"), raw)
	}
	register := func(op string, fn Handler) {
		spec.Register(op, func(c *gin.Context) {
			for _, param := range c.Params {
				if param.Key == "part" || param.Key == "callId" {
					continue
				}
				if _, e := uuid.Parse(param.Value); e != nil {
					respond{}.error(c, apierrors.Fields(param.Key, "uuid"))
					return
				}
			}
			fn(c)
		})
	}
	register("createWorkflowDraft", func(c *gin.Context) {
		v2body(c, func(in collab.WorkflowDraftInput) {
			id, e := s.CreateWorkflowDraft(c.Request.Context(), v2id(c, "projectId"), in)
			v2result(c, gin.H{"id": id}, e)
		})
	})
	register("getWorkflowDraft", func(c *gin.Context) {
		out, e := s.WorkflowDraft(c.Request.Context(), v2id(c, "projectId"), v2id(c, "draftId"))
		v2result(c, out, e)
	})
	register("listModels", func(c *gin.Context) {
		out, e := agent.ListPublicModels(c.Request.Context(), s.Pool)
		if e != nil {
			respond{}.error(c, e)
			return
		}
		respond{}.ok(c, gin.H{"items": out})
	})
	register("searchUsers", func(c *gin.Context) { out, e := s.SearchUsers(c.Request.Context(), c.Query("q")); v2list(c, out, e) })
	register("listTemplatePresets", func(c *gin.Context) {
		out, e := s.Presets(c.Request.Context())
		v2result(c, out, e)
	})
	register("importTemplatePresets", func(c *gin.Context) {
		v2body(c, func(in collab.PresetImport) {
			out, e := s.ImportPresets(c.Request.Context(), v2id(c, "projectId"), in)
			v2result(c, out, e)
		})
	})
	register("listProjects", func(c *gin.Context) { out, e := s.Projects(c.Request.Context()); v2list(c, out, e) })
	register("getProject", func(c *gin.Context) {
		out, e := s.Project(c.Request.Context(), v2id(c, "projectId"))
		v2result(c, out, e)
	})
	for _, op := range []string{"createProject", "updateProject"} {
		register(op, func(c *gin.Context) {
			v2body(c, func(in collab.ProjectInput) {
				out, e := s.SaveProject(c.Request.Context(), v2id(c, "projectId"), in)
				v2result(c, out, e)
			})
		})
	}
	register("listProjectMembers", func(c *gin.Context) {
		out, e := s.Members(c.Request.Context(), v2id(c, "projectId"))
		v2list(c, out, e)
	})
	register("saveProjectMember", func(c *gin.Context) {
		v2body(c, func(in collab.ProjectMemberInput) {
			v2done(c, s.SaveMember(c.Request.Context(), v2id(c, "projectId"), in, false))
		})
	})
	register("removeProjectMember", func(c *gin.Context) {
		v2body(c, func(in collab.Versioned) {
			v2done(c, s.SaveMember(c.Request.Context(), v2id(c, "projectId"), collab.ProjectMemberInput{UserID: v2id(c, "userId"), ExpectedVersion: in.ExpectedVersion}, true))
		})
	})
	for _, item := range []struct{ kind, singular string }{{"positions", "Position"}, {"workflows", "Workflow"}} {
		register("list"+item.singular+"s", func(c *gin.Context) {
			out, e := s.Templates(c.Request.Context(), v2id(c, "projectId"), item.kind)
			v2list(c, out, e)
		})
		register("list"+item.singular+"Versions", func(c *gin.Context) {
			out, e := s.TemplateVersions(c.Request.Context(), v2id(c, "projectId"), v2id(c, "templateId"), item.kind)
			v2list(c, out, e)
		})
		for _, prefix := range []string{"create", "update"} {
			register(prefix+item.singular, func(c *gin.Context) {
				v2body(c, func(in collab.TemplateInput) {
					id, e := s.SaveTemplate(c.Request.Context(), v2id(c, "projectId"), v2id(c, "templateId"), item.kind, in)
					v2result(c, gin.H{"id": id}, e)
				})
			})
		}
	}
	register("listPersonalPrompts", func(c *gin.Context) {
		out, e := s.PersonalPrompts(c.Request.Context(), v2id(c, "projectId"))
		v2list(c, out, e)
	})
	register("savePersonalPrompt", func(c *gin.Context) {
		v2body(c, func(in struct {
			Prompt          string `json:"prompt"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}) {
			v2done(c, s.SavePrompt(c.Request.Context(), v2id(c, "projectId"), v2id(c, "positionId"), in.Prompt, in.ExpectedVersion))
		})
	})
	register("listGroups", func(c *gin.Context) { out, e := s.Groups(c.Request.Context(), v2id(c, "projectId")); v2list(c, out, e) })
	register("getGroup", func(c *gin.Context) { out, e := s.Group(c.Request.Context(), v2id(c, "groupId")); v2result(c, out, e) })
	for _, op := range []string{"createGroup", "updateGroup"} {
		register(op, func(c *gin.Context) {
			v2body(c, func(in collab.GroupInput) {
				out, e := s.SaveGroup(c.Request.Context(), v2id(c, "projectId"), v2id(c, "groupId"), in)
				v2result(c, out, e)
			})
		})
	}
	register("archiveGroup", func(c *gin.Context) {
		v2body(c, func(in struct {
			ExpectedVersion int64 `json:"expectedVersion"`
			Archived        bool  `json:"archived"`
		}) {
			v2done(c, s.Archive(c.Request.Context(), v2id(c, "groupId"), in.ExpectedVersion, in.Archived))
		})
	})
	register("listDirectConversations", func(c *gin.Context) { out, e := s.Directs(c.Request.Context()); v2list(c, out, e) })
	register("createDirectConversation", func(c *gin.Context) {
		v2body(c, func(in struct {
			UserID uuid.UUID `json:"userId"`
		}) {
			out, e := s.Direct(c.Request.Context(), in.UserID)
			v2result(c, out, e)
		})
	})
	register("listMessages", func(c *gin.Context) {
		out, e := s.Messages(c.Request.Context(), v2id(c, "conversationId"), v2query(c, "after"), v2query(c, "before"), c.Query("q"))
		v2list(c, out, e)
	})
	register("getMessage", func(c *gin.Context) {
		out, e := s.Message(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "messageId"))
		v2result(c, out, e)
	})
	register("postMessage", func(c *gin.Context) {
		v2body(c, func(in collab.MessageInput) {
			out, e := s.Post(c.Request.Context(), v2id(c, "conversationId"), in, false)
			v2result(c, out, e)
		})
	})
	register("pushGroup", func(c *gin.Context) {
		v2body(c, func(in collab.MessageInput) {
			out, e := s.Post(c.Request.Context(), v2id(c, "groupId"), in, true)
			v2result(c, out, e)
		})
	})
	register("markConversationRead", func(c *gin.Context) {
		v2body(c, func(in struct {
			Seq int64 `json:"seq"`
		}) {
			v2done(c, s.Read(c.Request.Context(), v2id(c, "conversationId"), in.Seq))
		})
	})
	register("conversationEvents", func(c *gin.Context) { h.stream(c, false) })
	register("personalEvents", func(c *gin.Context) { h.stream(c, true) })
	register("listNotifications", func(c *gin.Context) { out, e := s.Notifications(c.Request.Context()); v2list(c, out, e) })
	register("readNotification", func(c *gin.Context) { v2done(c, s.ReadNotice(c.Request.Context(), v2id(c, "notificationId"))) })
	register("getGroupContext", func(c *gin.Context) {
		out, e := s.Knowledge(c.Request.Context(), v2id(c, "groupId"), v2query(c, "after"), int(v2query(c, "limit")))
		v2list(c, out, e)
	})
	register("readGroupReference", func(c *gin.Context) {
		out, e := s.ReadReference(c.Request.Context(), v2id(c, "groupId"), c.Query("kind"), c.Query("ref"), int(v2query(c, "offset")), int(v2query(c, "limit")))
		v2result(c, out, e)
	})
	register("listContextSnapshots", func(c *gin.Context) {
		out, e := s.Snapshots(c.Request.Context(), v2id(c, "groupId"))
		v2list(c, out, e)
	})
	register("listGroupRuns", func(c *gin.Context) { out, e := s.Runs(c.Request.Context(), v2id(c, "groupId")); v2list(c, out, e) })
	register("listDiscussions", func(c *gin.Context) {
		out, e := s.Discussions(c.Request.Context(), v2id(c, "groupId"))
		v2list(c, out, e)
	})
	register("controlDiscussion", func(c *gin.Context) {
		v2body(c, func(in struct {
			ExpectedVersion int64  `json:"expectedVersion"`
			Action          string `json:"action"`
			Feedback        string `json:"feedback"`
		}) {
			v2done(c, s.Control(c.Request.Context(), v2id(c, "groupId"), v2id(c, "discussionId"), in.ExpectedVersion, in.Action, in.Feedback))
		})
	})
	register("shareGroupContent", func(c *gin.Context) {
		v2body(c, func(in collab.ShareInput) {
			out, e := s.Share(c.Request.Context(), v2id(c, "groupId"), in)
			v2result(c, out, e)
		})
	})
	register("createGroupUpload", func(c *gin.Context) {
		v2body(c, func(in collab.UploadInput) {
			out, e := s.CreateUpload(c.Request.Context(), v2id(c, "conversationId"), in)
			v2result(c, out, e)
		})
	})
	register("uploadGroupPart", func(c *gin.Context) {
		n, e := strconv.Atoi(c.Param("part"))
		if e != nil {
			respond{}.error(c, apierrors.Fields("part", "integer"))
			return
		}
		out, e := s.UploadPart(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "uploadId"), n, c.Request.Body)
		v2result(c, out, e)
	})
	register("completeGroupUpload", func(c *gin.Context) {
		v2body(c, func(in struct {
			PublishMessage *bool `json:"publishMessage"`
		}) {
			announce := in.PublishMessage == nil || *in.PublishMessage
			out, e := s.CompleteUpload(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "uploadId"), announce)
			v2result(c, out, e)
		})
	})
	register("cancelGroupUpload", func(c *gin.Context) {
		v2done(c, s.CancelUpload(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "uploadId")))
	})
	register("listGroupFiles", func(c *gin.Context) {
		out, e := s.Files(c.Request.Context(), v2id(c, "conversationId"))
		v2list(c, out, e)
	})
	register("listFileVersions", func(c *gin.Context) {
		out, e := s.FileVersions(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "fileId"))
		v2list(c, out, e)
	})
	register("getFileVersion", func(c *gin.Context) {
		out, e := s.FileVersion(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "versionId"))
		v2result(c, out, e)
	})
	register("removeGroupFile", func(c *gin.Context) {
		v2body(c, func(in collab.Versioned) {
			v2done(c, s.RemoveFile(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "fileId"), in.ExpectedVersion))
		})
	})
	register("retryFilePreview", func(c *gin.Context) {
		v2done(c, s.RetryPreview(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "versionId")))
	})
	register("copyFileVersion", func(c *gin.Context) {
		v2body(c, func(in struct {
			Name string `json:"name"`
		}) {
			out, e := s.SaveCopy(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "versionId"), in.Name)
			v2result(c, out, e)
		})
	})
	register("listRunToolCalls", func(c *gin.Context) {
		out, e := s.ToolCalls(c.Request.Context(), v2id(c, "groupId"), v2id(c, "runId"))
		v2list(c, out, e)
	})
	register("reconcileToolCall", func(c *gin.Context) {
		v2body(c, func(in collab.ReconcileInput) {
			v2done(c, s.Reconcile(c.Request.Context(), v2id(c, "groupId"), v2id(c, "runId"), c.Param("callId"), in))
		})
	})

	register("createFilePreview", func(c *gin.Context) {
		token, expires, e := s.PreviewSession(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "versionId"))
		v2result(c, gin.H{"previewUrl": strings.TrimRight(h.PreviewOrigin, "/") + "/file-preview/" + token + "/" + c.Param("versionId") + "/index.html", "expiresAt": expires}, e)
	})

	register("fileVersionContent", func(c *gin.Context) { h.content(c, false, false) })
	register("fileVersionPreview", func(c *gin.Context) { h.content(c, true, false) })
	register("fileVersionEntry", func(c *gin.Context) { h.content(c, false, true) })
}
func (h V2Handlers) content(c *gin.Context, preview, entry bool) {
	var out collab.FileContent
	var e error
	if entry {
		out, e = h.Service.Entry(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "versionId"), c.Query("path"))
	} else {
		out, e = h.Service.Content(c.Request.Context(), v2id(c, "conversationId"), v2id(c, "versionId"), preview)
	}
	if e != nil {
		respond{}.error(c, e)
		return
	}
	defer out.Body.Close()
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:; style-src 'unsafe-inline'")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": out.Name}))
	if preview && (out.Mime == "application/pdf" || out.Mime == "image/png" || out.Mime == "image/jpeg" || out.Mime == "image/webp" || out.Mime == "image/gif" || out.Mime == "image/svg+xml" || strings.HasPrefix(out.Mime, "audio/") || strings.HasPrefix(out.Mime, "video/")) {
		c.Header("Content-Disposition", "inline")
	}
	c.Header("Content-Security-Policy", "sandbox; default-src 'none'; img-src data:")
	serveMaterialStream(c, out.Body, out.Size, out.Mime, c.Writer.Header().Get("Content-Disposition"))
}
func (h V2Handlers) stream(c *gin.Context, personal bool) {
	after := v2query(c, "after")
	if value := c.GetHeader("Last-Event-ID"); value != "" {
		n, e := strconv.ParseInt(value, 10, 64)
		if e != nil || n < 0 {
			respond{}.error(c, apierrors.Fields("Last-Event-ID", "cursor"))
			return
		}
		after = n
	}
	if after < 0 {
		respond{}.error(c, apierrors.Fields("after", "cursor"))
		return
	}
	ctx := c.Request.Context()
	p := principalFrom(c)
	if !personal {
		if _, e := h.Service.Access(ctx, v2id(c, "conversationId")); e != nil {
			respond{}.error(c, e)
			return
		}
	}
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if !liveAuthority(ctx, h.Service.Pool, p) {
			return
		}
		var items []collab.Event
		var e error
		next := after
		if personal {
			items, next, e = h.Service.PersonalEvents(ctx, after)
		} else {
			items, e = h.Service.Events(ctx, v2id(c, "conversationId"), after)
		}
		if e != nil {
			return
		}
		for _, item := range items {
			if !personal {
				if _, e = h.Service.Access(ctx, v2id(c, "conversationId")); e != nil {
					return
				}
			}
			_, e = fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", item.Seq, item.Kind, item.Data)
			if e != nil {
				return
			}
			after = item.Seq
		}
		if personal && next > after {
			after = next
			_, _ = fmt.Fprintf(c.Writer, "id: %d\nevent: cursor\ndata: {}\n\n", after)
		}
		_, _ = io.WriteString(c.Writer, ": heartbeat\n\n")
		c.Writer.Flush()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

var _ context.Context
