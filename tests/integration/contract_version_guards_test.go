package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/discussion"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"testing"
)

func TestTopicLinkRejectsStaleVersionWithoutChangingLinks(t *testing.T) {
	service, projects, ids, _ := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Version guard", "topic-version@contract.test", "contract-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, projectRequest("Topic link CAS"))
	if err != nil {
		t.Fatal(err)
	}
	topic, err := service.CreateTopic(ctx, user.ID, p.ID, "Versioned links", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Even an empty additive request must check its reviewed base version.
	if err = service.LinkTopic(ctx, user.ID, p.ID, topic.ID, []discussion.TopicLink{}, topic.LinksVersion+1); !apierrors.IsCode(err, apierrors.VersionConflict) {
		t.Fatalf("stale links accepted: %v", err)
	}
	after, err := service.GetTopic(ctx, user.ID, p.ID, topic.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.LinksVersion != topic.LinksVersion || len(after.Links) != 0 {
		t.Fatalf("stale request changed topic: %+v", after)
	}
}

func TestProposalRevisionRejectsSupersededReviewWithoutCreatingVersion(t *testing.T) {
	service, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Revision guard", "revision-version@contract.test", "contract-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Proposal revision CAS"})
	if err != nil {
		t.Fatal(err)
	}
	changes := []decision.Change{{Operation: "create_plan", TargetType: "plan", Fields: map[string]any{"title": "Reviewed plan"}}}
	proposal, err := service.CreateDraft(ctx, user.ID, p.ID, "work_arrangement", nil, "", changes)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.GetReview(ctx, user.ID, p.ID, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateRevision(ctx, user.ID, p.ID, proposal, changes, "Wrong base", uuid.New()); !apierrors.IsCode(err, apierrors.ReviewStale) {
		t.Fatalf("unknown base accepted: %v", err)
	}
	if _, err = service.CreateRevision(ctx, user.ID, p.ID, proposal, changes, "Current base", review.ReviewID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateRevision(ctx, user.ID, p.ID, proposal, changes, "Superseded base", review.ReviewID); !apierrors.IsCode(err, apierrors.ReviewStale) {
		t.Fatalf("superseded base accepted: %v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM proposal_versions WHERE proposal_id=$1`, proposal).Scan(&count); err != nil || count != 2 {
		t.Fatalf("stale call created a version: count=%d err=%v", count, err)
	}
}
