package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

func TestUnclaimedAgentDraftAdoptionPreservesInitiatorAuthority(t *testing.T) {
	for _, action := range []string{"edit", "submit"} {
		t.Run(action, func(t *testing.T) {
			svc, projects, users, pool := newDecisionEnv(t)
			ctx := context.Background()
			owner, _, err := users.Register(ctx, "Owner", action+"-owner@adoption.test", "adoption-password-1", "127.0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			other, _, err := users.Register(ctx, "Member", action+"-member@adoption.test", "adoption-password-2", "127.0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			p, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Agent adoption"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',now())`, p.ID, other.ID); err != nil {
				t.Fatal(err)
			}
			position, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "Reviewer"})
			if err != nil {
				t.Fatal(err)
			}
			identity, err := projects.CreateIdentity(ctx, owner.ID, p.ID, position.ID, other.ID)
			if err != nil {
				t.Fatal(err)
			}
			changes := []decision.Change{{Operation: "create_task", TargetType: "task", Fields: map[string]any{"title": "Review original", "reviewerIdentityId": identity.ID.String(), "participantIdentityIds": []any{identity.ID.String()}}}}
			id, err := svc.CreateDraft(ctx, owner.ID, p.ID, "work_arrangement", nil, "controlled agent draft fixture", changes)
			if err != nil {
				t.Fatal(err)
			}
			// Reproduce the existing Agent draft shape; this controlled fixture
			// creates no formal task or accepted business state.
			if _, err = pool.Exec(ctx, `UPDATE proposals SET created_by=NULL WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
			if action == "edit" {
				if _, err = svc.UpdateDraft(ctx, owner.ID, p.ID, id, 1, changes, "adopt and correct node"); err != nil {
					t.Fatal(err)
				}
				if _, err = svc.UpdateDraft(ctx, other.ID, p.ID, id, 2, changes, "take over"); !apierrors.IsCode(err, apierrors.Forbidden) {
					t.Fatalf("other member edit: %v", err)
				}
				if _, err = svc.UpdateDraft(ctx, owner.ID, p.ID, id, 1, changes, "stale edit"); !apierrors.IsCode(err, apierrors.VersionConflict) {
					t.Fatalf("stale edit: %v", err)
				}
			} else {
				if _, err = svc.Submit(ctx, owner.ID, p.ID, id, 1, ""); err != nil {
					t.Fatal(err)
				}
				if _, err = svc.Submit(ctx, other.ID, p.ID, id, 1, ""); !apierrors.IsCode(err, apierrors.Forbidden) {
					t.Fatalf("other member submit: %v", err)
				}
			}
			var creator uuid.UUID
			if err = pool.QueryRow(ctx, `SELECT created_by FROM proposals WHERE id=$1`, id).Scan(&creator); err != nil || creator != owner.ID {
				t.Fatalf("creator %v: %v", creator, err)
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE project_id=$1`, p.ID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("draft/submit created formal work: %d %v", count, err)
			}
		})
	}
}
