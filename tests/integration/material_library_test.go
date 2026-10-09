package integrationtest_test

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/material"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"io"
	"net/url"
	"testing"
)

func TestMaterialLibraryAssociationAndDeletion(t *testing.T) {
	svc, projects, ids, pool := newMaterialEnv(t)
	ctx := context.Background()
	owner, _, e := ids.Register(ctx, "Library", "library@test.local", "password-library-test", "10.3.4.5")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Library project"})
	if e != nil {
		t.Fatal(e)
	}
	outsider, _, e := ids.Register(ctx, "Other", "other-library@test.local", "password-library-test", "10.3.4.6")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',now())`, p.ID, outsider.ID); e != nil {
		t.Fatal(e)
	}
	w := work.NewService(pool, nil)
	plan, e := w.CreatePlanDraft(ctx, owner.ID, p.ID, "Actual plan", "", "", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	task, e := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "Actual task", PlanID: &plan.ID})
	if e != nil {
		t.Fatal(e)
	}
	content := "actual uploaded source"
	up, e := svc.CreateUpload(ctx, owner.ID, p.ID, "evidence.txt", "file", "text/plain", int64(len(content)), digest(content), "", "original purpose")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.UploadPart(ctx, owner.ID, p.ID, up.ID, 1, digest(content), bytes.NewBufferString(content)); e != nil {
		t.Fatal(e)
	}
	version, e := svc.Complete(ctx, owner.ID, p.ID, up.ID, nil)
	if e != nil {
		t.Fatal(e)
	}
	d := discussion.NewService(pool, nil)
	submission := discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "material", Source: "cli", Text: "task reference", TaskID: &task.ID, MaterialVersionIDs: []string{version.ID.String()}}
	saved, e := d.CreateSubmission(ctx, owner.ID, p.ID, submission)
	if e != nil {
		t.Fatal(e)
	}
	if saved.PlanID == nil || *saved.PlanID != plan.ID {
		t.Fatal("task did not resolve its plan")
	}
	if _, e = d.CreateSubmission(ctx, owner.ID, p.ID, submission); e != nil {
		t.Fatal(e)
	}
	rows, e := svc.Library(ctx, owner.ID, p.ID, material.LibraryFilter{ObjectType: "plan", ObjectID: &plan.ID})
	if e != nil || len(rows) != 1 {
		t.Fatalf("library projection: %v %+v", e, rows)
	}
	if rows[0].Purpose != "original purpose" || len(rows[0].Associations) != 2 || !rows[0].CanDelete {
		t.Fatalf("metadata: %+v", rows[0])
	}
	usage, e := svc.Usages(ctx, owner.ID, p.ID, version.ID)
	if e != nil || len(usage) != 1 || usage[0].Source != "cli" {
		t.Fatalf("usages: %v %+v", e, usage)
	}
	if e = svc.DeleteFromLibrary(ctx, outsider.ID, p.ID, version.MaterialID, version.ID); !errors.IsCode(e, errors.Forbidden) {
		t.Fatalf("ordinary member delete: %v", e)
	}
	if e = svc.DeleteFromLibrary(ctx, owner.ID, p.ID, version.MaterialID, uuid.New()); !errors.IsCode(e, errors.VersionConflict) {
		t.Fatalf("version gate: %v", e)
	}
	if e = svc.DeleteFromLibrary(ctx, owner.ID, p.ID, version.MaterialID, version.ID); e != nil {
		t.Fatal(e)
	}
	rows, e = svc.Library(ctx, owner.ID, p.ID, material.LibraryFilter{})
	if e != nil || len(rows) != 0 {
		t.Fatalf("deleted remains: %v %+v", e, rows)
	}
	body, _, e := svc.FileContent(ctx, owner.ID, p.ID, version.MaterialID, version.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer body.Close()
	raw, _ := io.ReadAll(body)
	if string(raw) != content {
		t.Fatal("historical content changed")
	}
	submission.ClientSubmissionID = uuid.NewString()
	if _, e = d.CreateSubmission(ctx, owner.ID, p.ID, submission); e == nil {
		t.Fatal("deleted reference accepted for a new submission")
	}
}
func TestMaterialLibraryTypePagination(t *testing.T) {
	svc, projects, ids, _ := newMaterialEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "Paging", "material-page@test.local", "password-library-test", "10.3.4.7")
	p, e := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Paging"})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"one.json", "two.txt", "three.json", "four.pdf"} {
		content := "actual"
		up, e := svc.CreateUpload(ctx, owner.ID, p.ID, name, "file", "text/plain", int64(len(content)), digest(content), "")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = svc.UploadPart(ctx, owner.ID, p.ID, up.ID, 1, digest(content), bytes.NewBufferString(content)); e != nil {
			t.Fatal(e)
		}
		if _, e = svc.Complete(ctx, owner.ID, p.ID, up.ID, nil); e != nil {
			t.Fatal(e)
		}
	}
	cursor := ""
	seen := map[uuid.UUID]bool{}
	for {
		q := url.Values{"limit": {"1"}, "sort": {"type"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		page, e := paging.Parse(ctx, "materials", q)
		if e != nil {
			t.Fatal(e)
		}
		rows, e := svc.Library(page, owner.ID, p.ID, material.LibraryFilter{Sort: "type"})
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range rows {
			if seen[m.ID] {
				t.Fatal("duplicate page record")
			}
			seen[m.ID] = true
		}
		next := paging.Next(page)
		if next == nil {
			break
		}
		cursor = *next
	}
	if len(seen) != 4 {
		t.Fatalf("missing records: %d", len(seen))
	}
	count, e := svc.LibraryTotal(ctx, owner.ID, p.ID, material.LibraryFilter{Group: "documents"})
	if e != nil || count != 2 {
		t.Fatalf("filtered total: %d %v", count, e)
	}
	if _, e := svc.LibraryTotal(ctx, owner.ID, p.ID, material.LibraryFilter{Group: "invalid"}); !errors.IsCode(e, errors.Validation) {
		t.Fatalf("invalid format filter: %v", e)
	}
}
