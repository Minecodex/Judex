package integrationtest_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"net/url"
	"testing"
)

func TestScopedKeysetPaginationIncludesTiedRowsAndRejectsReusedCursor(t *testing.T) {
	_, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Paging", "paging@test.local", "paging-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Paging"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO tasks(project_id,id,title,kind,status,created_at,updated_at) SELECT $1,gen_random_uuid(),'Task '||n,'task','draft',now(),now() FROM generate_series(1,237) n`, p.ID); err != nil {
		t.Fatal(err)
	}
	svc := work.NewService(pool, nil)
	seen := map[uuid.UUID]bool{}
	cursor := ""
	scope := fmt.Sprintf("/projects/%s/tasks:%s", p.ID, user.ID)
	for page := 0; page < 20; page++ {
		paged, err := paging.Parse(ctx, scope, url.Values{"limit": {"17"}, "cursor": {cursor}})
		if err != nil {
			t.Fatal(err)
		}
		tasks, err := svc.ListTasks(paged, user.ID, p.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) > 17 {
			t.Fatal("page limit ignored")
		}
		for _, task := range tasks {
			if seen[task.ID] {
				t.Fatal("duplicate task at keyset boundary")
			}
			seen[task.ID] = true
		}
		next := paging.Next(paged)
		if next == nil {
			break
		}
		cursor = *next
		if _, err = paging.Parse(ctx, scope+"other", url.Values{"cursor": {cursor}}); err == nil {
			t.Fatal("cursor crossed scope")
		}
		if _, err = paging.Parse(ctx, scope, url.Values{"cursor": {cursor}, "planId": {uuid.NewString()}}); err == nil {
			t.Fatal("cursor crossed filter")
		}
	}
	if len(seen) != 237 {
		t.Fatalf("pagination lost rows: %d", len(seen))
	}
	for _, limit := range []string{"0", "101", "garbage"} {
		if _, err := paging.Parse(ctx, scope, url.Values{"limit": {limit}}); err == nil {
			t.Fatal("invalid limit accepted")
		}
	}
}
