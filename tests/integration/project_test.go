// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newProjectService(t *testing.T) (*project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000, LoginPerIP: 1000, LoginPerAccount: 1000}, nil)
	return project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

// TestProjectLifecycle (A04/A08): creation writes the full atomic set; B
// cannot see A's project; archive requires owner + correct version and keeps
// history; restore works.
func TestProjectLifecycle(t *testing.T) {
	svc, ids, pool := newProjectService(t)
	ctx := context.Background()

	userA, _, err := ids.Register(ctx, "A", "a@proj.test", "password-aaaaa-11", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	userB, _, err := ids.Register(ctx, "B", "b@proj.test", "password-bbbbb-11", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	created, err := svc.Create(ctx, userA.ID, project.CreateRequest{Title: "主线项目", Kind: "software"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "active" || created.ViewerRole == nil || *created.ViewerRole != "owner" {
		t.Fatalf("unexpected creation result: %+v", created)
	}

	// Atomic set: owner membership, main venue, coordinator identity, event.
	var members, topics, coords, events int
	for _, q := range []struct {
		sql   string
		count *int
	}{
		{`SELECT count(*) FROM project_members WHERE project_id=$1 AND role='owner' AND state='active'`, &members},
		{`SELECT count(*) FROM topics WHERE project_id=$1 AND kind='project_room'`, &topics},
		{`SELECT count(*) FROM agent_identities WHERE project_id=$1 AND kind='coordinator'`, &coords},
		{`SELECT count(*) FROM project_events WHERE project_id=$1`, &events},
	} {
		if err := pool.QueryRow(ctx, q.sql, created.ID).Scan(q.count); err != nil {
			t.Fatal(err)
		}
	}
	if members != 1 || topics != 1 || coords != 1 || events != 1 {
		t.Fatalf("atomic set wrong: members=%d topics=%d coords=%d events=%d", members, topics, coords, events)
	}
	// No positions auto-bound to owner (02 §4).
	var bindings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM identity_bindings WHERE project_id=$1`, created.ID).Scan(&bindings); err != nil {
		t.Fatal(err)
	}
	if bindings != 0 {
		t.Fatal("owner must not receive automatic position bindings")
	}

	// B (non-member) sees nothing: list empty, get 404 with existence hidden.
	listB, _, err := svc.ListForUser(ctx, userB.ID, 50, nil, nil)
	if err != nil || len(listB) != 0 {
		t.Fatalf("B must see no projects: %v %+v", err, listB)
	}
	if _, err := svc.Get(ctx, userB.ID, created.ID); errors.IsCode(err, errors.NotFound) == false {
		t.Fatalf("B get must be NOT_FOUND, got %v", err)
	}

	// A sees it; archive by non-owner forbidden.
	listA, _, err := svc.ListForUser(ctx, userA.ID, 50, nil, nil)
	if err != nil || len(listA) != 1 {
		t.Fatalf("A must list own project: %v %+v", err, listA)
	}
	if _, err := svc.Archive(ctx, userB.ID, created.ID, created.Version, "bad actor", true); errors.IsCode(err, errors.NotFound) == false {
		t.Fatalf("non-member archive attempt must 404, got %v", err)
	}

	// Version conflict path.
	if _, err := svc.Archive(ctx, userA.ID, created.ID, created.Version+5, "stale", true); errors.IsCode(err, errors.VersionConflict) == false {
		t.Fatalf("stale version must conflict, got %v", err)
	}
	archived, err := svc.Archive(ctx, userA.ID, created.ID, created.Version, "季度结束", true)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != "archived" || archived.Version != created.Version+1 {
		t.Fatalf("archive result: %+v", archived)
	}
	// History retained: members/topics/events still present.
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM project_events WHERE project_id=$1`, created.ID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Fatalf("archive must append an event, got %d", events)
	}
	restored, err := svc.Archive(ctx, userA.ID, created.ID, archived.Version, "继续推进", false)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != "active" {
		t.Fatalf("restore result: %+v", restored)
	}
}

// TestProjectCreateConcurrentSingleOwner: two concurrent creates for the
// same user both succeed as independent projects (no global constraints).
func TestProjectCreateConcurrentSingleOwner(t *testing.T) {
	svc, ids, _ := newProjectService(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "C", "c@proj.test", "password-ccccc-11", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		id  uuid.UUID
		err error
	}
	ch := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			p, err := svc.Create(ctx, user.ID, project.CreateRequest{Title: "并行项目"})
			ch <- result{id: p.ID, err: err}
		}()
	}
	for i := 0; i < 2; i++ {
		r := <-ch
		if r.err != nil || r.id == uuid.Nil {
			t.Fatalf("concurrent create failed: %v", r.err)
		}
	}
}

// TestMemberManagementAndOwnerTransfer (A05/A07 后端部分): manager 授权边界、
// owner 移除保护、双人转移保持唯一 owner、离开阻塞、运维转移。
func TestMemberManagementAndOwnerTransfer(t *testing.T) {
	svc, ids, pool := newProjectService(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "Owner", "owner@mt.test", "password-owner-11", "10.0.0.1")
	manager, _, _ := ids.Register(ctx, "Manager", "manager@mt.test", "password-manager1", "10.0.0.1")
	member, _, _ := ids.Register(ctx, "Member", "member@mt.test", "password-member1", "10.0.0.1")
	outsider, _, _ := ids.Register(ctx, "Outsider", "out@mt.test", "password-outsider", "10.0.0.1")

	proj, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "成员管理"})
	if err != nil {
		t.Fatal(err)
	}
	// Ad hoc joins for the test (invitations arrive in P2-01; membership row
	// shape is what matters here).
	join := func(u uuid.UUID) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO project_members (project_id, user_id, role, state, joined_at)
			VALUES ($1,$2,'member','active',now())`, proj.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	join(manager.ID)
	join(member.ID)

	// member cannot grant manager.
	if _, err := svc.UpdateMemberRole(ctx, member.ID, proj.ID, manager.ID, proj.Version, "manager"); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("member must not manage roles, got %v", err)
	}
	// owner grants manager.
	m, err := svc.UpdateMemberRole(ctx, owner.ID, proj.ID, manager.ID, proj.Version, "manager")
	if err != nil || m.Role != "manager" {
		t.Fatalf("grant manager: %v %+v", err, m)
	}
	// owner cannot be demoted via role route.
	if _, err := svc.UpdateMemberRole(ctx, owner.ID, proj.ID, owner.ID, proj.Version+1, "member"); err == nil {
		t.Fatal("owner demote via role route must fail")
	}
	// manager cannot remove owner.
	if _, err := svc.RemoveMember(ctx, manager.ID, proj.ID, owner.ID, proj.Version+2, "越权"); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("manager cannot remove owner, got %v", err)
	}
	// owner cannot leave while sole owner.
	if err := svc.LeaveProject(ctx, owner.ID, proj.ID, proj.Version+3); errors.IsCode(err, errors.InvalidTransition) == false {
		t.Fatalf("sole owner leave must be blocked, got %v", err)
	}
	// outsider sees nothing.
	if _, err := svc.ListMembers(ctx, outsider.ID, proj.ID); errors.IsCode(err, errors.NotFound) == false {
		t.Fatalf("outsider member list must 404, got %v", err)
	}

	// Owner transfer: target accepts, exactly one owner remains.
	transfer, err := svc.RequestOwnerTransfer(ctx, owner.ID, proj.ID, manager.ID, proj.Version+1)
	if err != nil {
		t.Fatal(err)
	}
	if transfer.State != "pending" {
		t.Fatalf("transfer state: %+v", transfer)
	}
	// Only the target decides.
	if _, err := svc.DecideOwnerTransfer(ctx, owner.ID, proj.ID, transfer.ID, true); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("non-target decide must be forbidden, got %v", err)
	}
	decided, err := svc.DecideOwnerTransfer(ctx, manager.ID, proj.ID, transfer.ID, true)
	if err != nil || decided.State != "accepted" {
		t.Fatalf("accept transfer: %v %+v", err, decided)
	}
	members, err := svc.ListMembers(ctx, manager.ID, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	ownerCount := 0
	for _, mm := range members {
		if mm.Role == "owner" && mm.State == "active" {
			ownerCount++
		}
	}
	if ownerCount != 1 {
		t.Fatalf("exactly one owner expected after transfer, got %d", ownerCount)
	}
	// Old owner can now leave (project version advanced twice since create).
	current, err := svc.Get(ctx, manager.ID, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LeaveProject(ctx, owner.ID, proj.ID, current.Version); err != nil {
		t.Fatalf("former owner should be able to leave: %v", err)
	}
}

// TestOperatorTransferOwner: audited operator path for a stuck owner.
func TestOperatorTransferOwner(t *testing.T) {
	svc, ids, pool := newProjectService(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "O2", "o2@mt.test", "password-o2-o2-11", "10.0.0.1")
	next, _, _ := ids.Register(ctx, "N2", "n2@mt.test", "password-n2-n2-11", "10.0.0.1")
	proj, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "运维转移"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, next.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.OperatorTransferOwner(ctx, proj.ID, next.ID, "op@host", "离职恢复"); err != nil {
		t.Fatal(err)
	}
	members, err := svc.ListMembers(ctx, next.ID, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mm := range members {
		if mm.UserID == next.ID && mm.Role != "owner" {
			t.Fatal("operator transfer did not promote target")
		}
	}
}
