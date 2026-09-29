package integrationtest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kakj-go/Judex/internal/material"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/platform/auth"
	"github.com/kakj-go/Judex/internal/project"
	httptransport "github.com/kakj-go/Judex/internal/transport/http"
	"github.com/kakj-go/Judex/internal/transport/http/middleware"
	"github.com/kakj-go/Judex/internal/work"
)

func TestReviewDocumentInvariants(t *testing.T) {
	svc, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, session, err := ids.Register(ctx, "Owner", "review-owner@test.local", "review-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	member, _, err := ids.Register(ctx, "Member", "review-member@test.local", "review-password-456", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	ws := work.NewService(pool, nil)
	makeProject := func(t *testing.T) uuid.UUID {
		p, e := projects.Create(ctx, owner.ID, project.CreateRequest{Title: t.Name()})
		if e != nil {
			t.Fatal(e)
		}
		_, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',now())`, p.ID, member.ID)
		if e != nil {
			t.Fatal(e)
		}
		return p.ID
	}
	draft := func(t *testing.T, pid uuid.UUID, c decision.Change) (uuid.UUID, decision.Review) {
		if c.Operation == "create_task" || c.Operation == "create_plan" {
			pos, e := projects.CreatePosition(ctx, owner.ID, pid, project.PositionDraft{Name: "Responsible"})
			if e != nil {
				t.Fatal(e)
			}
			ident, e := projects.CreateIdentity(ctx, owner.ID, pid, pos.ID, owner.ID)
			if e != nil {
				t.Fatal(e)
			}
			if c.Operation == "create_plan" {
				c.Fields["ownerIdentityId"] = ident.ID.String()
			} else {
				c.Fields["reviewerIdentityId"] = ident.ID.String()
				c.Fields["participantIdentityIds"] = []any{ident.ID.String()}
			}
		}
		id, e := svc.CreateDraft(ctx, owner.ID, pid, "work_change", nil, "review", []decision.Change{c})
		if e != nil {
			t.Fatal(e)
		}
		_, e = svc.Submit(ctx, owner.ID, pid, id, 1, "")
		if e != nil {
			t.Fatal(e)
		}
		r, e := svc.GetReview(ctx, owner.ID, pid, id)
		if e != nil {
			t.Fatal(e)
		}
		return id, r
	}
	insertTask := func(t *testing.T, pid uuid.UUID, status string) uuid.UUID {
		id := uuid.New()
		_, e := pool.Exec(ctx, `INSERT INTO tasks(project_id,id,title,status,created_at,updated_at) VALUES($1,$2,'original',$3,now(),now())`, pid, id, status)
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spec, e := httptransport.NewSpecRouter(logger)
	if e != nil {
		t.Fatal(e)
	}
	httptransport.NewAgentHandlers(pool).Register(spec)
	ms := material.NewService(pool, newMemStore(), material.Limits{PartSize: 64}, nil)
	httptransport.NewMaterialHandlers(ms).Register(spec)
	spec.CommandPool = pool
	httptransport.NewProposalHandlers(svc).Register(spec)
	httptransport.NewProjectHandlers(projects, nil, nil).Register(spec)
	router, e := httptransport.NewRouter(httptransport.Options{Logger: logger, Auth: &httptransport.AuthOptions{Config: middleware.AuthConfig{Development: true}, Resolver: ids.ResolveSession, GrantResolver: ids.ResolveGrant}}, spec)
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, path, token, key string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(raw))
		if token == "web" {
			r.AddCookie(&http.Cookie{Name: middleware.DevSessionCookie, Value: session.Secret})
			r.Header.Set("X-CSRF-Token", session.CSRFToken)
		} else {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	scoped := makeProject(t)
	dc, uc, _, _, e := ids.StartDeviceAuthorization(ctx, "review", []string{auth.ScopeProjectsRead}, []string{scoped.String()})
	if e != nil {
		t.Fatal(e)
	}
	if e = ids.ConfirmDeviceAuthorization(ctx, owner.ID, uc, true, nil); e != nil {
		t.Fatal(e)
	}
	_, pair, e := ids.PollDeviceToken(ctx, dc)
	token := pair.AccessToken
	if e != nil || token == "" {
		t.Fatalf("token: %v", e)
	}
	t.Run("CLI_cannot_approve_outside_readonly_grant", func(t *testing.T) {
		pid := makeProject(t)
		id, r := draft(t, pid, decision.Change{Operation: "create_task", TargetType: "task", Fields: map[string]any{"title": "cli task"}})
		w := call("POST", fmt.Sprintf("/projects/%s/proposals/%s/decisions", pid, id), token, uuid.NewString(), map[string]any{"reviewId": r.ReviewID, "reviewHash": r.ReviewHash, "decision": "approve"})
		if w.Code != 403 {
			t.Errorf("wanted 403 for readonly CLI grant scoped to another project; got %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("Idempotency_same_key_same_project_creation", func(t *testing.T) {
		key := uuid.NewString()
		a := call("POST", "/projects", "web", key, map[string]any{"title": "idempotent review"})
		b := call("POST", "/projects", "web", key, map[string]any{"title": "idempotent review"})
		var n int
		pool.QueryRow(ctx, `SELECT count(*) FROM projects WHERE title='idempotent review'`).Scan(&n)
		if n != 1 {
			t.Errorf("same Idempotency-Key created %d projects; HTTP %d/%d", n, a.Code, b.Code)
		}
	})
	t.Run("Approved_scope_change_must_apply_fields", func(t *testing.T) {
		pid := makeProject(t)
		tid := insertTask(t, pid, "ready")
		id, r := draft(t, pid, decision.Change{Operation: "update_scope", TargetType: "task", TargetID: tid.String(), ExpectedVersion: 1, Fields: map[string]any{"title": "changed", "expectedOutput": "new output"}})
		if _, e := svc.Decide(ctx, owner.ID, pid, id, r.ReviewHash, true, ""); e != nil {
			t.Fatal(e)
		}
		var title, status string
		pool.QueryRow(ctx, `SELECT title FROM tasks WHERE id=$1`, tid).Scan(&title)
		pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, id).Scan(&status)
		if title != "changed" {
			t.Errorf("proposal=%s but task title=%q (expected changed)", status, title)
		}
	})
	t.Run("Affected_reviewer_must_get_approval_slot", func(t *testing.T) {
		pid := makeProject(t)
		pos, e := projects.CreatePosition(ctx, owner.ID, pid, project.PositionDraft{Name: "Reviewer"})
		if e != nil {
			t.Fatal(e)
		}
		ident, e := projects.CreateIdentity(ctx, owner.ID, pid, pos.ID, member.ID)
		if e != nil {
			t.Fatal(e)
		}
		tid := insertTask(t, pid, "ready")
		pool.Exec(ctx, `UPDATE tasks SET reviewer_identity_id=$2 WHERE id=$1`, tid, ident.ID)
		_, r := draft(t, pid, decision.Change{Operation: "cancel_task", TargetType: "task", TargetID: tid.String(), ExpectedVersion: 1})
		found := false
		for _, s := range r.Slots {
			if s.AuthorityID == ident.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("affected reviewer omitted: %+v", r.Slots)
		}
	})
	t.Run("Nonparticipant_cannot_deliver_with_unmet_start_condition", func(t *testing.T) {
		pid := makeProject(t)
		tid := insertTask(t, pid, "ready")
		pre := insertTask(t, pid, "ready")
		pos, e := projects.CreatePosition(ctx, owner.ID, pid, project.PositionDraft{Name: "Developer"})
		if e != nil {
			t.Fatal(e)
		}
		ident, e := projects.CreateIdentity(ctx, owner.ID, pid, pos.ID, owner.ID)
		if e != nil {
			t.Fatal(e)
		}
		pool.Exec(ctx, `INSERT INTO task_participants(project_id,task_id,identity_id) VALUES($1,$2,$3)`, pid, tid, ident.ID)
		_, e = pool.Exec(ctx, `INSERT INTO task_requirements(project_id,id,task_id,phase,kind,target_id) VALUES($1,$2,$3,'start','task_acceptance',$4)`, pid, uuid.New(), tid, pre)
		if e != nil {
			t.Fatal(e)
		}
		rid, _, e := ws.Report(ctx, member.ID, pid, work.ReportInput{TaskID: tid, Kind: "delivery", Text: "not my task", ExpectedTaskVersion: 1, MaterialVersionIDs: []string{uuid.NewString()}})
		if e == nil {
			t.Errorf("unassigned member delivered despite unmet start and nonexistent material: report=%s", rid)
		}
		if _, _, err := ws.Report(ctx, owner.ID, pid, work.ReportInput{TaskID: tid, IdentityID: &ident.ID, Kind: "delivery", Text: "owner blocked", ExpectedTaskVersion: 1}); err == nil {
			t.Error("assigned owner bypassed start prerequisite")
		}
		if _, err := pool.Exec(ctx, `UPDATE tasks SET status='accepted' WHERE id=$1`, pre); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ws.Report(ctx, owner.ID, pid, work.ReportInput{TaskID: tid, IdentityID: &ident.ID, Kind: "delivery", Text: "missing evidence", ExpectedTaskVersion: 1, MaterialVersionIDs: []string{uuid.NewString()}}); err == nil {
			t.Error("assigned owner referenced nonexistent material")
		}
		if _, _, err := ws.Report(ctx, owner.ID, pid, work.ReportInput{TaskID: tid, IdentityID: &ident.ID, Kind: "delivery", Text: "valid delivery", ExpectedTaskVersion: 1}); err != nil {
			t.Fatalf("valid authorized delivery: %v", err)
		}
	})
	t.Run("Accepted_plan_must_not_be_cancelled", func(t *testing.T) {
		pid := makeProject(t)
		plan, e := ws.CreatePlanDraft(ctx, owner.ID, pid, "accepted plan", "", "", nil, nil)
		if e != nil {
			t.Fatal(e)
		}
		pool.Exec(ctx, `UPDATE plans SET status='accepted' WHERE id=$1`, plan.ID)
		id, r := draft(t, pid, decision.Change{Operation: "cancel_plan", TargetType: "plan", TargetID: plan.ID.String(), ExpectedVersion: 1})
		_, e = svc.Decide(ctx, owner.ID, pid, id, r.ReviewHash, true, "")
		if e == nil {
			t.Error("approved cancellation changed accepted plan to cancelled")
		}
	})
	t.Run("Nonowner_cannot_reopen_plan", func(t *testing.T) {
		pid := makeProject(t)
		pos, e := projects.CreatePosition(ctx, owner.ID, pid, project.PositionDraft{Name: "Plan owner"})
		if e != nil {
			t.Fatal(e)
		}
		ident, e := projects.CreateIdentity(ctx, owner.ID, pid, pos.ID, owner.ID)
		if e != nil {
			t.Fatal(e)
		}
		plan, e := ws.CreatePlanDraft(ctx, owner.ID, pid, "plan", "", "", &ident.ID, nil)
		if e != nil {
			t.Fatal(e)
		}
		pool.Exec(ctx, `UPDATE plans SET status='accepted' WHERE id=$1`, plan.ID)
		_, e = ws.ReopenPlan(ctx, member.ID, pid, plan.ID, uuid.New(), "not owner")
		if e == nil {
			t.Error("ordinary member reopened another identity's accepted plan")
		}
	})
	t.Run("Outsider_cannot_start_other_project_agent", func(t *testing.T) {
		p, e := projects.Create(ctx, member.ID, project.CreateRequest{Title: "Private to member"})
		if e != nil {
			t.Fatal(e)
		}
		var topic uuid.UUID
		if e = pool.QueryRow(ctx, `SELECT id FROM topics WHERE project_id=$1 LIMIT 1`, p.ID).Scan(&topic); e != nil {
			t.Fatal(e)
		}
		w := call("POST", fmt.Sprintf("/projects/%s/topics/%s/runs", p.ID, topic), "web", uuid.NewString(), map[string]any{"sourceSubmissionId": uuid.NewString()})
		if w.Code != 404 && w.Code != 403 {
			t.Errorf("nonmember with invented submission started private project's agent: HTTP %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("Multipart_download_must_return_entire_file", func(t *testing.T) {
		pid := makeProject(t)
		content := strings.Repeat("abcdefghij", 20)
		upload, e := ms.CreateUpload(ctx, owner.ID, pid, "report.txt", "file", "text/plain", int64(len(content)), digest(content), "")
		if e != nil {
			t.Fatal(e)
		}
		for start := 0; start < len(content); start += 64 {
			end := start + 64
			if end > len(content) {
				end = len(content)
			}
			chunk := content[start:end]
			if _, e = ms.UploadPart(ctx, owner.ID, pid, upload.ID, start/64+1, digest(chunk), strings.NewReader(chunk)); e != nil {
				t.Fatal(e)
			}
		}
		v, e := ms.Complete(ctx, owner.ID, pid, upload.ID, nil)
		if e != nil {
			t.Fatal(e)
		}
		w := call("GET", fmt.Sprintf("/projects/%s/materials/%s/versions/%s/content", pid, v.MaterialID, v.ID), "web", "", nil)
		if w.Code != 200 {
			t.Fatalf("download HTTP %d %s", w.Code, w.Body.String())
		}
		if w.Body.String() != content {
			t.Errorf("complete file length=%d but HTTP download length=%d", len(content), w.Body.Len())
		}
	})
	t.Run("Archived_project_rejects_business_write", func(t *testing.T) {
		pid := makeProject(t)
		_, e := pool.Exec(ctx, `UPDATE projects SET status='archived' WHERE id=$1`, pid)
		if e != nil {
			t.Fatal(e)
		}
		_, e = ws.CreatePlanDraft(ctx, member.ID, pid, "after archive", "", "", nil, nil)
		if e == nil {
			t.Error("draft creation succeeded after archive")
		}
	})
}
