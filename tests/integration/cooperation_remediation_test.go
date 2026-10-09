package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/collaboration"
	"github.com/kakj-go/Judex/internal/discussion"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	integration "github.com/kakj-go/Judex/tests/integration"
	"testing"
	"time"
)

// Regression tests for the failed independent cooperation review.
func TestPersonalDeliveryFiltersIgnoreOtherSendersOutstandingSources(t *testing.T) {
	fixture := integration.StartPG(t)
	pool := fixture.Pool
	ctx := context.Background()
	ids := identity.NewService(pool, identity.NewRateLimiter(pool.Pool, nil), identity.Options{RegisterPerIP: 1000}, nil)
	owner, _, err := ids.Register(ctx, "Sender A", "sender-a@review.test", "review-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := ids.Register(ctx, "Sender B", "sender-b@review.test", "review-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	projects := project.NewService(pool, nil)
	w := work.NewService(pool, nil)
	p, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "Delivery filter review"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',$3)`, p.ID, other.ID, now); err != nil {
		t.Fatal(err)
	}
	pos, err := projects.CreatePosition(ctx, owner.ID, p.ID, project.PositionDraft{Name: "Sender"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := projects.CreateIdentity(ctx, owner.ID, p.ID, pos.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := projects.CreateIdentity(ctx, owner.ID, p.ID, pos.ID, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.CreatePlanDraft(ctx, owner.ID, p.ID, "Plan", "", "", &a.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	sourceA, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "A source", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	sourceB, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "B source", PlanID: &plan.ID, ParticipantIDs: []uuid.UUID{b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	target, err := w.CreateTaskDraft(ctx, owner.ID, p.ID, work.TaskDraft{Title: "Receiver", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	handoff := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO handoffs(project_id,id,target_task_id,receiver_identity_id,kind,title,created_at,updated_at) VALUES($1,$2,$3,$4,'dependency','Two source review',$5,$5)`, p.ID, handoff, target.ID, a.ID, now); err != nil {
		t.Fatal(err)
	}
	var foreignVersion uuid.UUID
	for i, source := range []struct {
		task, seat uuid.UUID
		state      string
	}{{sourceA.ID, a.ID, "accepted"}, {sourceB.ID, b.ID, "rejected"}} {
		sid, vid := uuid.New(), uuid.New()
		if i == 1 {
			foreignVersion = vid
		}
		if _, err = pool.Exec(ctx, `INSERT INTO handoff_sources(project_id,id,handoff_id,source_task_id,sender_identity_id) VALUES($1,$2,$3,$4,$5)`, p.ID, sid, handoff, source.task, source.seat); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO source_versions(project_id,id,source_id,revision,summary,state,created_at) VALUES($1,$2,$3,1,$4,$5,$6)`, p.ID, vid, sid, source.state, source.state, now); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `UPDATE handoff_sources SET current_source_version_id=$2 WHERE id=$1`, sid, vid); err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range []struct{ filter, foreignState string }{{"revise", "rejected"}, {"waiting", "pending"}} {
		t.Run(scenario.filter, func(t *testing.T) {
			if _, err = pool.Exec(ctx, `UPDATE source_versions SET state=$2 WHERE id=$1`, foreignVersion, scenario.foreignState); err != nil {
				t.Fatal(err)
			}
			cards, err := w.ListDeliveries(ctx, owner.ID, p.ID, work.DeliveryFilter{Filter: scenario.filter, Type: "handoff"})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("A source=accepted, B source=%s, personal filter=%s, returned=%d", scenario.foreignState, scenario.filter, len(cards))
			if len(cards) != 0 {
				t.Errorf("Sender A has no outstanding source, but appears in personal %s: %+v", scenario.filter, cards)
			}
		})
	}
}

func TestSuggestionAssociationAdvancesLinksVersionAndRejectsStaleEditor(t *testing.T) {
	fixture := integration.StartPG(t)
	pool := fixture.Pool
	ctx := context.Background()
	ids := identity.NewService(pool, identity.NewRateLimiter(pool.Pool, nil), identity.Options{RegisterPerIP: 1000}, nil)
	user, _, err := ids.Register(ctx, "Association review", "association@review.test", "review-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	projects := project.NewService(pool, nil)
	w := work.NewService(pool, nil)
	d := discussion.NewService(pool, nil)
	c := collaboration.NewService(pool)
	p, err := projects.Create(ctx, user.ID, project.CreateRequest{Title: "Association version review"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.CreatePlanDraft(ctx, user.ID, p.ID, "Origin plan", "", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	task, err := w.CreateTaskDraft(ctx, user.ID, p.ID, work.TaskDraft{Title: "Origin task", PlanID: &plan.ID})
	if err != nil {
		t.Fatal(err)
	}
	topic, err := d.CreateTopic(ctx, user.ID, p.ID, "Existing discussion in another scope", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := d.CreateSubmission(ctx, user.ID, p.ID, discussion.Submission{ClientSubmissionID: uuid.NewString(), Purpose: "message", DiscussionIntent: "question", Source: "web", Text: "Question to link", TaskID: &task.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	analysis, suggestion := uuid.New(), uuid.New()
	err = pool.QueryRow(ctx, `INSERT INTO task_analyses(project_id,id,task_id,source_type,source_id,state,summary,created_at,updated_at) VALUES($1,$2,$3,'submission',$4,'completed','Public summary',$5,$5) ON CONFLICT(project_id,source_type,source_id) DO UPDATE SET state='completed',summary='Public summary' RETURNING id`, p.ID, analysis, task.ID, sub.ID, now).Scan(&analysis)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO discussion_suggestions(project_id,id,analysis_id,task_id,title,reason,created_at) VALUES($1,$2,$3,$4,'Continue existing discussion','Human confirmation required',$5)`, p.ID, suggestion, analysis, task.ID, now); err != nil {
		t.Fatal(err)
	}
	resolved, err := c.Resolve(ctx, user.ID, p.ID, suggestion, collaboration.ResolveInput{ExpectedVersion: 1, Mode: "link", TopicID: &topic.ID})
	if err != nil {
		t.Fatal(err)
	}
	after, err := d.GetTopic(ctx, user.ID, p.ID, topic.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("suggestion=%s, linksVersion %d -> %d, links=%d", resolved.State, topic.LinksVersion, after.LinksVersion, len(after.Links))
	if after.LinksVersion <= topic.LinksVersion {
		t.Errorf("suggestion added associations without advancing linksVersion")
	}
	staleErr := d.ReplaceTopicLinks(ctx, user.ID, p.ID, topic.ID, topic.LinksVersion, nil)
	final, err := d.GetTopic(ctx, user.ID, p.ID, topic.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("stale editor error=%v, remaining links=%d", staleErr, len(final.Links))
	if !apierrors.IsCode(staleErr, apierrors.VersionConflict) {
		t.Errorf("stale editor silently removed the suggestion's new associations")
	}
	replay, err := c.Resolve(ctx, user.ID, p.ID, suggestion, collaboration.ResolveInput{ExpectedVersion: 1, Mode: "link", TopicID: &topic.ID})
	if err != nil || replay.ResultTopicID == nil || *replay.ResultTopicID != topic.ID {
		t.Fatalf("suggestion replay: %+v %v", replay, err)
	}
	unchanged, err := d.GetTopic(ctx, user.ID, p.ID, topic.ID)
	if err != nil || unchanged.LinksVersion != final.LinksVersion || len(unchanged.Links) != 2 {
		t.Fatalf("suggestion replay duplicated associations: %+v %v", unchanged, err)
	}
}

func TestApprovedArrangementTopicLinksUseAssociationVersion(t *testing.T) {
	d, p, ids, pool := newDiscussionEnv(t)
	ctx := context.Background()
	user, _, err := ids.Register(ctx, "Arrangement", "arrangement-links@review.test", "review-password-123", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	project, err := p.Create(ctx, user.ID, projectRequest("Arrangement links"))
	if err != nil {
		t.Fatal(err)
	}
	w := work.NewService(pool, nil)
	task, err := w.CreateTaskDraft(ctx, user.ID, project.ID, work.TaskDraft{Title: "Task"})
	if err != nil {
		t.Fatal(err)
	}
	topic, err := d.CreateTopic(ctx, user.ID, project.ID, "Existing", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, project.ID.String()); err != nil {
			return err
		}
		_, err := work.ApplyChanges(ctx, tx.Tx, project.ID, user.ID, []work.Change{{Operation: "link_topic", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: task.Version, Fields: map[string]any{"topicId": topic.ID.String()}}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := d.GetTopic(ctx, user.ID, project.ID, topic.ID)
	if err != nil || updated.LinksVersion != topic.LinksVersion+1 || len(updated.Links) != 1 {
		t.Fatalf("arrangement association: %+v %v", updated, err)
	}
	if err = d.ReplaceTopicLinks(ctx, user.ID, project.ID, topic.ID, topic.LinksVersion, nil); !apierrors.IsCode(err, apierrors.VersionConflict) {
		t.Fatalf("old association editor: %v", err)
	}
	if err = d.LinkTopic(ctx, user.ID, project.ID, topic.ID, updated.Links); err != nil {
		t.Fatal(err)
	}
	replayed, err := d.GetTopic(ctx, user.ID, project.ID, topic.ID)
	if err != nil || replayed.LinksVersion != updated.LinksVersion {
		t.Fatalf("no-op append changed associations: %+v %v", replayed, err)
	}
}
