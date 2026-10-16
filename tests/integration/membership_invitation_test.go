package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

func TestMembershipOnlyInvitationPreservesHandoffIdentity(t *testing.T) {
	service, _, accounts, pool := newPositionEnv(t)
	ctx := context.Background()
	owner, _, err := accounts.Register(ctx, "Owner", "owner@member-only.test", "member-only-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := accounts.Register(ctx, "Old QA", "old@member-only.test", "member-only-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := accounts.Register(ctx, "New QA", "new@member-only.test", "member-only-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := service.Create(ctx, owner.ID, project.CreateRequest{Title: "Handoff"})
	if err != nil {
		t.Fatal(err)
	}
	position, err := service.CreatePosition(ctx, owner.ID, workspace.ID, project.PositionDraft{Name: "QA", Prompt: "Independent QA"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CreateInvitation(ctx, owner.ID, workspace.ID, "old@member-only.test", []uuid.UUID{position.ID})
	if err != nil {
		t.Fatal(err)
	}
	seats, _, err := service.AcceptInvitation(ctx, old.ID, first.ID, first.Token)
	if err != nil || len(seats) != 1 {
		t.Fatalf("original seat: %+v %v", seats, err)
	}
	before, err := service.ListIdentities(ctx, owner.ID, workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	invite, err := service.CreateInvitation(ctx, owner.ID, workspace.ID, "new@member-only.test", []uuid.UUID{})
	if err != nil {
		t.Fatalf("membership-only invitation: %v", err)
	}
	if _, _, err = service.AcceptInvitation(ctx, old.ID, invite.ID, invite.Token); !apierrors.IsCode(err, apierrors.Forbidden) {
		t.Fatalf("wrong account membership invitation: %v", err)
	}
	for i := 0; i < 2; i++ {
		created, id, err := service.AcceptInvitation(ctx, next.ID, invite.ID, invite.Token)
		if err != nil || id != workspace.ID || len(created) != 0 {
			t.Fatalf("membership-only accept/replay: %+v %s %v", created, id, err)
		}
	}
	after, err := service.ListIdentities(ctx, owner.ID, workspace.ID)
	if err != nil || len(after) != len(before) {
		t.Fatalf("invitation created extra identity: %d/%d %v", len(before), len(after), err)
	}
	member, err := service.MembershipFor(ctx, next.ID, workspace.ID)
	if err != nil || member.Role != "member" || member.State != "active" {
		t.Fatalf("new member: %+v %v", member, err)
	}
	replaced, err := service.ReplaceIdentityBinding(ctx, owner.ID, workspace.ID, seats[0].ID, next.ID, 1, "接续原测试身份")
	if err != nil || replaced.ID != seats[0].ID || replaced.CurrentBindingVersion != 2 {
		t.Fatalf("stable identity replacement: %+v %v", replaced, err)
	}
	var identities, bindings int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM agent_identities WHERE project_id=$1),(SELECT count(*) FROM identity_bindings WHERE identity_id=$2)`, workspace.ID, seats[0].ID).Scan(&identities, &bindings); err != nil || identities != len(before) || bindings != 2 {
		t.Fatalf("handoff history: identities=%d bindings=%d %v", identities, bindings, err)
	}
}
