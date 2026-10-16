package integrationtest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kakj-go/Judex/internal/decision"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
)

func TestProposalCreateFieldContractBeforeStorageRevisionAndSubmission(t *testing.T) {
	svc, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	owner, _, err := ids.Register(ctx, "Field owner", "field-owner@fields.test", "password-fields-1", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Early field validation"})
	if err != nil {
		t.Fatal(err)
	}
	position, err := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "Owner"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := projects.CreateIdentity(ctx, owner.ID, proj.ID, position.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	bad := []decision.Change{{Operation: "create_task", TargetType: "task", Fields: map[string]any{
		"title": "Invalid node field", "workflowNodeId": "aw3-base", "reviewerIdentityId": identity.ID.String(), "participantIdentityIds": []any{identity.ID.String()},
	}}}
	if _, err = svc.CreateDraft(ctx, owner.ID, proj.ID, "work_arrangement", nil, "", bad); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("invalid field stored as draft: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM proposals WHERE project_id=$1`, proj.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid creation persisted: %d %v", count, err)
	}
	valid := []decision.Change{{Operation: "create_plan", TargetType: "plan", Fields: map[string]any{"title": "Valid draft", "ownerIdentityId": identity.ID.String()}}}
	id, err := svc.CreateDraft(ctx, owner.ID, proj.ID, "work_arrangement", nil, "", valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateDraft(ctx, owner.ID, proj.ID, id, 1, bad, "invalid edit"); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("invalid revision allowed: %v", err)
	}
	var version int64
	if err = pool.QueryRow(ctx, `SELECT version FROM proposals WHERE id=$1`, id).Scan(&version); err != nil || version != 1 {
		t.Fatalf("failed revision changed proposal: %d %v", version, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM proposal_versions WHERE proposal_id=$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed revision added version: %d %v", count, err)
	}
	// Owned controlled fixture simulates a draft persisted by the earlier server.
	raw, _ := json.Marshal(bad)
	if _, err = pool.Exec(ctx, `UPDATE proposal_versions SET changes_json=$2 WHERE proposal_id=$1`, id, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Submit(ctx, owner.ID, proj.ID, id, 1, ""); !apierrors.IsCode(err, apierrors.Validation) {
		t.Fatalf("earlier invalid draft entered approval: %v", err)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, id).Scan(&status); err != nil || status != "draft" {
		t.Fatalf("failed submit changed state: %s %v", status, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE project_id=$1`, proj.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid field created formal work: %d %v", count, err)
	}
}
