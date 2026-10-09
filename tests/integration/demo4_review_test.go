package integrationtest_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	"testing"
)

func TestDemo4ReviewDraftProposalWithdrawsOldReview(t *testing.T) {
	d, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	s := work.NewService(pool, nil)
	a, _, e := ids.Register(ctx, "Author", "review-author@test.local", "review-password-123", "10.17.1.1")
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := ids.Register(ctx, "Member", "review-member@test.local", "review-password-123", "10.17.1.2")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, a.ID, project.CreateRequest{Title: "Draft review audit"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at)VALUES($1,$2,'member','active',now())`, p.ID, b.ID); e != nil {
		t.Fatal(e)
	}
	role, e := projects.CreatePosition(ctx, a.ID, p.ID, project.PositionDraft{Name: "Contributors"})
	if e != nil {
		t.Fatal(e)
	}
	ia, e := projects.CreateIdentity(ctx, a.ID, p.ID, role.ID, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	ib, e := projects.CreateIdentity(ctx, a.ID, p.ID, role.ID, b.ID)
	if e != nil {
		t.Fatal(e)
	}
	task, e := s.CreateTaskDraft(ctx, a.ID, p.ID, work.TaskDraft{Title: "Original", ParticipantIDs: []uuid.UUID{ia.ID, ib.ID}, ReviewerIdentityID: &ia.ID})
	if e != nil {
		t.Fatal(e)
	}
	old, e := d.CreateDraft(ctx, a.ID, p.ID, "work_arrangement", nil, "Activate", []decision.Change{{Operation: "activate_object", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1}})
	if e != nil {
		t.Fatal(e)
	}
	oldReview, e := d.Submit(ctx, a.ID, p.ID, old, 1, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Decide(ctx, a.ID, p.ID, old, oldReview.ReviewHash, true, ""); e != nil {
		t.Fatal(e)
	}
	fresh, e := d.CreateDraft(ctx, b.ID, p.ID, "work_change", nil, "Revise draft", []decision.Change{{Operation: "update_scope", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1, Fields: map[string]any{"title": "Revised by proposal"}}})
	if e != nil {
		t.Fatal(e)
	}
	freshReview, e := d.Submit(ctx, b.ID, p.ID, fresh, 1, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Decide(ctx, b.ID, p.ID, fresh, freshReview.ReviewHash, true, ""); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Decide(ctx, a.ID, p.ID, fresh, freshReview.ReviewHash, true, ""); e != nil {
		t.Fatal(e)
	}
	var status string
	var votes int
	if e = pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, old).Scan(&status); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM approval_decisions WHERE review_id=(SELECT current_review_id FROM proposals WHERE id=$1)`, old).Scan(&votes); e != nil {
		t.Fatal(e)
	}
	t.Logf("After approved draft revision: old review status=%s, retained votes=%d", status, votes)
	if status != "cancelled" || votes != 1 {
		t.Fatalf("old review must withdraw and retain its historical consent: %s %d", status, votes)
	}
	if _, e = d.Decide(ctx, b.ID, p.ID, old, oldReview.ReviewHash, true, ""); !apierrors.IsCode(e, apierrors.InvalidTransition) {
		t.Fatalf("old final signature must be rejected: %v", e)
	}
	current, e := s.GetTask(ctx, a.ID, p.ID, task.ID)
	if e != nil || current.Status != "draft" || current.Title != "Revised by proposal" {
		t.Fatalf("old arrangement unexpectedly applied %+v %v", current, e)
	}
	if e = pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, fresh).Scan(&status); e != nil || status != "approved" {
		t.Fatalf("the committing proposal must remain approved: %s %v", status, e)
	}
	if current.Version != 2 {
		t.Fatalf("draft revision must increment version: %d", current.Version)
	}
	if e = d.SettleTimeout(ctx, p.ID, old, oldReview.ReviewID); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, old).Scan(&status); e != nil || status != "cancelled" {
		t.Fatalf("a withdrawn review cannot be reopened by its timeout: %s %v", status, e)
	}
}

func TestDemo4ReviewMixedPlanAndTaskDraftRevision(t *testing.T) {
	f := newDraftReviewFixture(t)
	ctx := context.Background()
	plan, e := f.work.CreatePlanDraft(ctx, f.author, f.project, "Original plan", "Goal", "Criteria", &f.authorSeat, nil)
	if e != nil {
		t.Fatal(e)
	}
	task, e := f.work.CreateTaskDraft(ctx, f.author, f.project, work.TaskDraft{PlanID: &plan.ID, Title: "Original task", ParticipantIDs: []uuid.UUID{f.authorSeat, f.memberSeat}, ReviewerIdentityID: &f.authorSeat})
	if e != nil {
		t.Fatal(e)
	}
	old, e := f.decisions.CreateDraft(ctx, f.author, f.project, "work_arrangement", nil, "Activate plan and task", []decision.Change{{Operation: "activate_object", TargetType: "plan", TargetID: plan.ID.String(), ExpectedVersion: 1}, {Operation: "activate_object", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1}})
	if e != nil {
		t.Fatal(e)
	}
	oldReview, e := f.decisions.Submit(ctx, f.author, f.project, old, 1, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.decisions.Decide(ctx, f.author, f.project, old, oldReview.ReviewHash, true, ""); e != nil {
		t.Fatal(e)
	}
	fresh, e := f.decisions.CreateDraft(ctx, f.member, f.project, "work_change", nil, "Revise both drafts", []decision.Change{{Operation: "update_scope", TargetType: "plan", TargetID: plan.ID.String(), ExpectedVersion: 1, Fields: map[string]any{"title": "Revised plan"}}, {Operation: "update_scope", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1, Fields: map[string]any{"title": "Revised task"}}})
	if e != nil {
		t.Fatal(e)
	}
	review, e := f.decisions.Submit(ctx, f.member, f.project, fresh, 1, "")
	if e != nil {
		t.Fatal(e)
	}
	for _, user := range []uuid.UUID{f.member, f.author} {
		if _, e = f.decisions.Decide(ctx, user, f.project, fresh, review.ReviewHash, true, ""); e != nil {
			t.Fatal(e)
		}
	}
	var oldStatus, newStatus, planTitle, taskTitle string
	var planVersion, taskVersion, withdrawnEvents, votes int
	e = f.pool.QueryRow(ctx, `SELECT p.status,n.status,l.title,t.title,l.version,t.version,
		(SELECT count(*) FROM project_events WHERE project_id=$3 AND type='proposal.changed' AND payload->>'change'='draft_review_withdrawn'),
		(SELECT count(*) FROM approval_decisions WHERE review_id=p.current_review_id)
		FROM proposals p,proposals n,plans l,tasks t WHERE p.id=$1 AND n.id=$2 AND l.id=$4 AND t.id=$5`, old, fresh, f.project, plan.ID, task.ID).Scan(&oldStatus, &newStatus, &planTitle, &taskTitle, &planVersion, &taskVersion, &withdrawnEvents, &votes)
	if e != nil {
		t.Fatal(e)
	}
	if oldStatus != "cancelled" || newStatus != "approved" || planTitle != "Revised plan" || taskTitle != "Revised task" || planVersion != 2 || taskVersion != 2 || withdrawnEvents != 1 || votes != 1 {
		t.Fatalf("mixed draft revision must commit once and preserve review history: %s %s %s %s %d %d events=%d votes=%d", oldStatus, newStatus, planTitle, taskTitle, planVersion, taskVersion, withdrawnEvents, votes)
	}
}

func TestDemo4ReviewDraftRevisionFailureRollsBack(t *testing.T) {
	f := newDraftReviewFixture(t)
	ctx := context.Background()
	task, e := f.work.CreateTaskDraft(ctx, f.author, f.project, work.TaskDraft{Title: "Original", ParticipantIDs: []uuid.UUID{f.authorSeat, f.memberSeat}, ReviewerIdentityID: &f.authorSeat})
	if e != nil {
		t.Fatal(e)
	}
	old, e := f.decisions.CreateDraft(ctx, f.author, f.project, "work_arrangement", nil, "Activate", []decision.Change{{Operation: "activate_object", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1}})
	if e != nil {
		t.Fatal(e)
	}
	review, e := f.decisions.Submit(ctx, f.author, f.project, old, 1, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.decisions.Decide(ctx, f.author, f.project, old, review.ReviewHash, true, ""); e != nil {
		t.Fatal(e)
	}
	var before int
	if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM project_events WHERE project_id=$1`, f.project).Scan(&before); e != nil {
		t.Fatal(e)
	}
	e = f.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if e := tx.LockActiveProject(ctx, f.project.String()); e != nil {
			return e
		}
		_, e := work.ApplyChanges(ctx, tx, f.project, f.author, []work.Change{
			{Operation: "update_scope", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1, Fields: map[string]any{"title": "Must roll back"}},
			{Operation: "set_requirements", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 2, Fields: map[string]any{"requirements": []work.Requirement{{Kind: "task_acceptance", Phase: "start", TargetID: uuid.New(), Hard: true}}}},
		})
		return e
	})
	if !apierrors.IsCode(e, apierrors.InvalidReference) {
		t.Fatalf("invalid second change must reject the entire group: %v", e)
	}
	current, e := f.work.GetTask(ctx, f.author, f.project, task.ID)
	if e != nil || current.Title != "Original" || current.Version != 1 || len(current.Requirements) != 0 {
		t.Fatalf("partial draft mutation escaped rollback: %+v %v", current, e)
	}
	var status string
	var votes, after int
	if e = f.pool.QueryRow(ctx, `SELECT p.status,(SELECT count(*) FROM approval_decisions WHERE review_id=p.current_review_id),
		(SELECT count(*) FROM project_events WHERE project_id=p.project_id) FROM proposals p WHERE p.id=$1`, old).Scan(&status, &votes, &after); e != nil {
		t.Fatal(e)
	}
	if status != "pending" || votes != 1 || after != before {
		t.Fatalf("failed revision must keep the old review, consent and event cursor: %s %d %d/%d", status, votes, after, before)
	}
	// Also cover a rollback AFTER the shared withdrawal has run.
	e = f.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if e := tx.LockActiveProject(ctx, f.project.String()); e != nil {
			return e
		}
		if _, e := work.ApplyChanges(ctx, tx, f.project, f.author, []work.Change{{Operation: "update_scope", TargetType: "task", TargetID: task.ID.String(), ExpectedVersion: 1, Fields: map[string]any{"title": "Commit failure"}}}); e != nil {
			return e
		}
		return errIntentionalRollback
	})
	if e != errIntentionalRollback {
		t.Fatalf("expected transaction rollback: %v", e)
	}
	if e = f.pool.QueryRow(ctx, `SELECT status FROM proposals WHERE id=$1`, old).Scan(&status); e != nil || status != "pending" {
		t.Fatalf("withdrawal must be transactional: %s %v", status, e)
	}
	// Ensure the frozen contents were never rewritten while withdrawing.
	var raw []byte
	if e = f.pool.QueryRow(ctx, `SELECT changes_json FROM proposal_versions WHERE id=$1`, review.ReviewID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var frozen []decision.Change
	if e = json.Unmarshal(raw, &frozen); e != nil || len(frozen) != 1 || frozen[0].ExpectedVersion != 1 {
		t.Fatalf("frozen review changed: %s %v", raw, e)
	}
}

type draftReviewFixture struct {
	decisions                                       *decision.Service
	work                                            *work.Service
	pool                                            *postgres.Pool
	project, author, member, authorSeat, memberSeat uuid.UUID
}

func newDraftReviewFixture(t *testing.T) draftReviewFixture {
	t.Helper()
	d, projects, ids, pool := newDecisionEnv(t)
	ctx := context.Background()
	a, _, e := ids.Register(ctx, "Author", "fixture-author@test.local", "review-password-123", "10.17.2.1")
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := ids.Register(ctx, "Member", "fixture-member@test.local", "review-password-123", "10.17.2.2")
	if e != nil {
		t.Fatal(e)
	}
	p, e := projects.Create(ctx, a.ID, project.CreateRequest{Title: "Atomic draft review"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO project_members(project_id,user_id,role,state,joined_at) VALUES($1,$2,'member','active',now())`, p.ID, b.ID); e != nil {
		t.Fatal(e)
	}
	role, e := projects.CreatePosition(ctx, a.ID, p.ID, project.PositionDraft{Name: "Contributors"})
	if e != nil {
		t.Fatal(e)
	}
	ia, e := projects.CreateIdentity(ctx, a.ID, p.ID, role.ID, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	ib, e := projects.CreateIdentity(ctx, a.ID, p.ID, role.ID, b.ID)
	if e != nil {
		t.Fatal(e)
	}
	return draftReviewFixture{decisions: d, work: work.NewService(pool, nil), pool: pool, project: p.ID, author: a.ID, member: b.ID, authorSeat: ia.ID, memberSeat: ib.ID}
}
