// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
	"github.com/kakj-go/Judex/internal/project"
)

func TestProjectOverview(t *testing.T) {
	svc, identity, pool := newProjectService(t)
	ctx := context.Background()
	users := make([]uuid.UUID, 5)
	for i := range users {
		user, _, err := identity.Register(ctx, fmt.Sprintf("Person %d", i), fmt.Sprintf("overview%d@project.test", i), "password-overview-11", "10.4.0.1")
		if err != nil {
			t.Fatal(err)
		}
		users[i] = user.ID
	}
	create := func(owner uuid.UUID, title string) project.Project {
		t.Helper()
		p, err := svc.Create(ctx, owner, project.CreateRequest{Title: title, Description: "Design context"})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := create(users[0], "体验 100% 项目")
	second := create(users[0], "Another owned project")
	third := create(users[1], "Joined project")
	for i := 1; i < 5; i++ {
		state := "active"
		if i == 4 {
			state = "removed"
		}
		if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member',$3,$4)`, first.ID, users[i], state, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',$3)`, third.ID, users[0], time.Now()); err != nil {
		t.Fatal(err)
	}
	discussionIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range discussionIDs {
		p := first.ID
		if i == 2 {
			p = third.ID
		}
		if _, err := pool.Exec(ctx, `INSERT INTO topics(project_id,id,title,kind,created_by,created_at) VALUES($1,$2,$3,'discussion',$4,$5)`, p, id, fmt.Sprintf("Discussion %d", i), users[0], time.Now().Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO topics(project_id,id,title,kind,created_at) VALUES($1,$2,'Handoff','handoff',$3)`, first.ID, uuid.New(), time.Now()); err != nil {
		t.Fatal(err)
	}
	// A fresh message in an older discussion must win over a newer empty discussion.
	if _, err := pool.Exec(ctx, `INSERT INTO messages(project_id,id,topic_id,seq,kind,author_user_id,content,created_at) VALUES($1,$2,$3,1,'human',$4,'Progress',$5)`, first.ID, uuid.New(), discussionIDs[0], users[0], time.Now().Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	list := func(options project.ListOptions, limit int) ([]project.Project, bool, int) {
		t.Helper()
		items, more, total, err := svc.ListOverview(ctx, users[0], limit, nil, nil, options)
		if err != nil {
			t.Fatal(err)
		}
		return items, more, total
	}
	items, more, total := list(project.ListOptions{Summary: true}, 1)
	if len(items) != 1 || !more || total != 3 {
		t.Fatalf("pagination/count: %v %v %d", items, more, total)
	}
	afterTime, afterID := items[0].CreatedAt, items[0].ID
	next, more, nextTotal, err := svc.ListOverview(ctx, users[0], 1, &afterTime, &afterID, project.ListOptions{Summary: true})
	if err != nil || len(next) != 1 || !more || next[0].ID == items[0].ID || nextTotal != 3 {
		t.Fatalf("cursor/count: %v %v %d %v", next, more, nextTotal, err)
	}
	owned, _, total := list(project.ListOptions{Ownership: "owned"}, 50)
	if len(owned) != 2 || total != 2 {
		t.Fatal("owned filter", owned, total)
	}
	joined, _, total := list(project.ListOptions{Ownership: "joined"}, 50)
	if len(joined) != 1 || total != 1 || joined[0].ID != third.ID {
		t.Fatal("joined filter", joined, total)
	}
	literal, _, total := list(project.ListOptions{Query: "100%", Summary: true}, 50)
	if total != 1 || len(literal) != 1 || literal[0].ID != first.ID {
		t.Fatal("literal search", literal, total)
	}
	summary := literal[0].Summary
	if summary == nil || summary.MemberCount != 4 || summary.TopicCount != 2 || len(summary.MemberPreview) != 3 {
		t.Fatalf("active member/discussion summary: %+v", summary)
	}
	for _, member := range summary.MemberPreview {
		if member.UserID == users[4] {
			t.Fatal("removed member leaked")
		}
	}
	absent, _, total, err := svc.ListOverview(ctx, users[4], 50, nil, nil, project.ListOptions{Summary: true})
	if err != nil || len(absent) != 0 || total != 0 {
		t.Fatal("membership isolation", absent, total, err)
	}
	recent, err := svc.RecentTopics(ctx, users[0], 3)
	if err != nil || len(recent) != 3 || recent[0].TopicID != discussionIDs[0] {
		t.Fatal("recent activity order", recent, err)
	}
	if _, err := svc.RecentTopics(ctx, users[0], 100); err != nil {
		t.Fatal("contract maximum limit rejected", err)
	}
	if _, err := svc.RecentTopics(ctx, users[0], 101); err == nil {
		t.Fatal("invalid recent limit accepted")
	}
	cli := auth.WithPrincipal(ctx, &auth.Principal{Kind: auth.KindCLI, UserID: users[0], ProjectScope: []uuid.UUID{second.ID}, Scopes: []string{auth.ScopeProjectsRead, auth.ScopeContextRead}})
	scoped, _, total, err := svc.ListOverview(cli, users[0], 50, nil, nil, project.ListOptions{Summary: true})
	if err != nil || len(scoped) != 1 || scoped[0].ID != second.ID || total != 1 {
		t.Fatal("grant scope", scoped, total, err)
	}
	scopedRecent, err := svc.RecentTopics(cli, users[0], 3)
	if err != nil || len(scopedRecent) != 0 {
		t.Fatal("recent grant scope", scopedRecent, err)
	}
	emptyScope := auth.WithPrincipal(ctx, &auth.Principal{Kind: auth.KindCLI, UserID: users[0]})
	if _, _, _, err := svc.ListOverview(emptyScope, users[0], 50, nil, nil, project.ListOptions{Summary: true}); err == nil {
		t.Fatal("scopeless grant exposed summaries")
	}
	if recent, err := svc.RecentTopics(emptyScope, users[0], 3); err != nil || len(recent) != 0 {
		t.Fatal("scopeless grant exposed discussions", recent, err)
	}
	if _, _, _, err := svc.ListOverview(ctx, users[0], 50, nil, nil, project.ListOptions{Ownership: "invalid"}); err == nil {
		t.Fatal("invalid ownership accepted")
	}
	if _, err := pool.Exec(ctx, `UPDATE project_members SET state='removed' WHERE project_id=$1 AND user_id=$2`, third.ID, users[0]); err != nil {
		t.Fatal(err)
	}
	recent, err = svc.RecentTopics(ctx, users[0], 3)
	if err != nil || len(recent) != 2 {
		t.Fatal("revoked membership still visible", recent, err)
	}
}
