package httptransport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/idempotency"
)

var publicOperations = map[string]bool{
	"getSystem": true, "getCapabilities": true, "register": true, "login": true,
	"recoverPassword": true, "createDeviceAuthorization": true, "pollDeviceToken": true, "refreshToken": true,
}

// CLI grants narrow both the user's authority and channel. Unlisted writes
// require a browser, even if that user's normal web session could perform them.
var cliWrites = map[string]string{
	"prepareFilePreview":  auth.ScopeMaterialsRead,
	"retryTaskAnalysis":   auth.ScopeAgentRequest,
	"createUploadSession": auth.ScopeMaterialsWrite, "uploadPart": auth.ScopeMaterialsWrite,
	"completeUpload": auth.ScopeMaterialsWrite, "cancelUpload": auth.ScopeMaterialsWrite,
	"createReleaseReport": auth.ScopeReportsWrite, "cancelAgentRun": auth.ScopeAgentRequest,
	"createSubmission": auth.ScopeSubmissionsWrite, "createTaskReport": auth.ScopeReportsWrite,
	"createProposal": auth.ScopeProposalsDraft, "updateProposalDraft": auth.ScopeProposalsDraft,
	"createProposalRevision": auth.ScopeProposalsDraft, "createPlan": auth.ScopeProposalsDraft,
	"createTask": auth.ScopeProposalsDraft, "startAgentRun": auth.ScopeAgentRequest,
	"createConfirmationIntent": auth.ScopeIntentsCreate,
}

func authorizeOperation(c *gin.Context, op string) error {
	p := principalFrom(c)
	if publicOperations[op] {
		return nil
	}
	if p == nil {
		return apierrors.New(apierrors.Unauthenticated, "authentication required")
	}
	if p.Kind != auth.KindCLI {
		return nil
	}
	if op == "createGlobalConfirmationIntent" || op == "getGlobalConfirmationIntent" {
		if !p.HasScope(auth.ScopeProjectsCreate) || !p.HasScope(auth.ScopeIntentsCreate) {
			return apierrors.New(apierrors.Forbidden, "projects:create and intents:create required")
		}
		return nil
	}
	if raw := projectScopeParam(c); raw != "" {
		id, e := uuid.Parse(raw)
		if e != nil {
			return apierrors.Fields("projectId", "invalid")
		}
		if !p.InProjectScope(id) {
			return apierrors.New(apierrors.Forbidden, "project outside grant scope")
		}
	}
	if op == "revokeClientGrant" && c.Param("grantId") == p.GrantID.String() {
		return nil
	}
	if strings.HasPrefix(c.FullPath(), "/api/v1/me/") && op != "listMyActions" && op != "listRecentTopics" {
		return apierrors.New(apierrors.Forbidden, "account operation requires browser session")
	}
	if op == "resolveInvitation" {
		return apierrors.New(apierrors.Forbidden, "invitation review requires browser session")
	}
	scope := cliWrites[op]
	if c.Request.Method == http.MethodGet {
		scope = auth.ScopeContextRead
		path := c.FullPath()
		switch {
		case op == "listProjects" || op == "getProject" || op == "getProjectBootstrap":
			scope = auth.ScopeProjectsRead
		case strings.Contains(path, "/material") || op == "listUploadSessions":
			scope = auth.ScopeMaterialsRead
			if op == "listUploadSessions" && p.HasScope(auth.ScopeMaterialsWrite) {
				scope = auth.ScopeMaterialsWrite
			}
		case strings.HasSuffix(path, "/events"):
			scope = auth.ScopeEventsRead
		case op == "getConfirmationIntent":
			scope = auth.ScopeIntentsCreate
		case op == "getSession":
			return nil
		}
	}
	if scope == "" {
		return apierrors.New(apierrors.HumanConfirmNeeded, "this operation requires browser confirmation")
	}
	if !p.HasScope(scope) {
		return apierrors.New(apierrors.Forbidden, "grant lacks scope: "+scope)
	}
	return nil
}

func (s *SpecRouter) guardOperation(op string, h Handler) Handler {
	return func(c *gin.Context) {
		if err := authorizeOperation(c, op); err != nil {
			respond{}.error(c, err)
			return
		}
		p := principalFrom(c)
		if p != nil {
			c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), p))
		}
		if s.CommandPool != nil && p != nil && c.Request.Method == "GET" && projectScopeParam(c) != "" {
			var member bool
			if err := s.CommandPool.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active')`, projectScopeParam(c), p.UserID).Scan(&member); err != nil || !member {
				respond{}.error(c, apierrors.New(apierrors.NotFound, "project not found"))
				return
			}
		}
		if p != nil && p.Kind == auth.KindCLI {
			c.Request = c.Request.WithContext(audit.WithSource(c.Request.Context(), audit.SourceCLI))
		}
		if c.Request.Method == "GET" && p != nil && pagedOperations[op] {
			scope := c.Request.URL.Path + ":" + p.UserID.String() + ":" + op
			ctx, err := paging.Parse(c.Request.Context(), scope, c.Request.URL.Query())
			if err != nil {
				respond{}.error(c, err)
				return
			}
			c.Request = c.Request.WithContext(ctx)
		}
		// No transaction is held for reads, streams, or anonymous authentication.
		if s.CommandPool == nil || p == nil || publicOperations[op] || c.Request.Method == "GET" || c.Request.Method == "HEAD" || op == "uploadPart" || strings.Contains(c.FullPath(), "/auth/") {
			h(c)
			return
		}
		s.runCommand(c, op, h)
	}
}

type commandResponse struct {
	gin.ResponseWriter
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *commandResponse) Header() http.Header    { return w.header }
func (w *commandResponse) WriteHeader(status int) { w.status = status }
func (w *commandResponse) WriteHeaderNow() {
	if w.status == 0 {
		w.status = 200
	}
}
func (w *commandResponse) Write(b []byte) (int, error)       { w.WriteHeaderNow(); return w.body.Write(b) }
func (w *commandResponse) WriteString(v string) (int, error) { return w.Write([]byte(v)) }
func (w *commandResponse) Status() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}
func (w *commandResponse) Size() int     { return w.body.Len() }
func (w *commandResponse) Written() bool { return w.status != 0 }

func (s *SpecRouter) runCommand(c *gin.Context, op string, h Handler) {
	ctx := c.Request.Context()
	p := principalFrom(c)
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBody))
	if err != nil {
		respond{}.error(c, apierrors.New(apierrors.PayloadTooLarge, "request body too large"))
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	// Canonical JSON makes insignificant whitespace irrelevant on retry.
	var value any
	canonical := raw
	if json.Unmarshal(raw, &value) == nil {
		canonical, _ = json.Marshal(value)
	}
	hash := sha256.Sum256(append([]byte(c.Request.URL.RequestURI()+"\n"), canonical...))
	key := c.GetHeader("Idempotency-Key")
	if key == "" && s.requiredKeys[op] {
		respond{}.error(c, apierrors.Fields("Idempotency-Key", "required"))
		return
	}
	if key != "" {
		if _, err := uuid.Parse(key); err != nil {
			respond{}.error(c, apierrors.Fields("Idempotency-Key", "uuid"))
			return
		}
	}
	tx, err := s.CommandPool.Begin(ctx)
	if err != nil {
		respond{}.error(c, err)
		return
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	ctx = postgres.WithTransaction(ctx, s.CommandPool, tx)
	c.Request = c.Request.WithContext(ctx)
	var status string
	var version int
	err = tx.QueryRow(ctx, `SELECT status,auth_version FROM users WHERE id=$1 FOR SHARE`, p.UserID).Scan(&status, &version)
	if err != nil || status != "active" || version != p.AuthVersion {
		respond{}.error(c, apierrors.New(apierrors.Unauthenticated, "session changed"))
		return
	}
	if rawID := c.Param("projectId"); rawID != "" {
		var role string
		if err = tx.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`, rawID, p.UserID).Scan(&role); err != nil {
			respond{}.error(c, apierrors.New(apierrors.NotFound, "project not found"))
			return
		}
		guard := postgres.Tx{Tx: tx}
		if op == "restoreProject" {
			err = guard.LockProjectForUpdate(ctx, rawID)
		} else {
			err = guard.LockActiveProject(ctx, rawID)
		}
		if err != nil {
			respond{}.error(c, err)
			return
		}
	}
	actor := string(p.Kind) + ":" + p.UserID.String()
	if key != "" {
		decision, status, body, e := idempotency.Begin(ctx, tx, actor, op, key, hash[:], 24*time.Hour, time.Now().UTC())
		if e != nil {
			respond{}.error(c, e)
			return
		}
		if decision == idempotency.Replay {
			if e = tx.Commit(ctx); e != nil {
				respond{}.error(c, e)
				return
			}
			c.Data(status, "application/json", body)
			return
		}
	}
	writer := c.Writer
	capture := &commandResponse{ResponseWriter: writer, header: writer.Header().Clone()}
	c.Writer = capture
	defer func() { c.Writer = writer }()
	h(c)
	if capture.Status() < 400 || c.GetBool("judex.commit_conflict") {
		if key != "" {
			err = idempotency.Complete(ctx, tx, actor, op, key, capture.Status(), capture.body.Bytes())
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
	} else {
		err = tx.Rollback(ctx)
	}
	c.Writer = writer
	if err != nil && !errors.Is(err, context.Canceled) {
		respond{}.error(c, err)
		return
	}
	for k, values := range capture.header {
		writer.Header()[k] = values
	}
	writer.WriteHeader(capture.Status())
	_, _ = writer.Write(capture.body.Bytes())
}

func projectScopeParam(c *gin.Context) string {
	if p := c.Param("projectId"); p != "" {
		return p
	}
	return c.Query("projectId")
}

var pagedOperations = map[string]bool{
	"listMaterialUsages": true,
	"listTaskActivity":   true, "listDiscussionSuggestions": true,
	"listDeliveries": true, "listProjects": true, "listRecentTopics": true, "listModels": true, "listMyActions": true, "listNotifications": true,
	"listPlans": true, "listTasks": true, "listProposals": true, "listTopics": true, "listHandoffs": true, "listMaterials": true, "listMaterialVersions": true, "listUploadSessions": true, "listPositions": true, "listProjectMembers": true, "listMembers": true, "listIdentities": true, "listInvitations": true, "listMyInvitations": true, "listSessions": true, "listClientGrants": true, "listRepositories": true, "listReleaseReports": true, "listAudit": true, "listTaskReports": true, "listWorkflows": true, "listWorkflowVersions": true,
}
