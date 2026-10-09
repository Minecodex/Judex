// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/decision"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	integration "github.com/kakj-go/Judex/tests/integration"
)

// TestConfirmationIntentFlow (D04): CLI 创建意图 → 浏览器同人一次确认 →
// 同事务执行领域命令 → CLI 读取同一 resultRef；重复确认幂等；错人/过期拒绝。
func TestConfirmationIntentFlow(t *testing.T) {
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	projects := project.NewService(fixture.Pool, nil)
	svc := work.NewService(fixture.Pool, nil)
	decisions := decision.NewService(fixture.Pool, nil, 3600)
	ctx := context.Background()

	owner, _, _ := ids.Register(ctx, "IA", "ia@ia.test", "password-ia-ia-1", "10.0.0.1")
	worker, _, _ := ids.Register(ctx, "IB", "ib@ia.test", "password-ib-ib-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "意图项目"})
	if err := projects.JoinDirect(ctx, proj.ID, worker.ID); err != nil {
		t.Fatal(err)
	}
	revPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "验收"})
	revIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, revPos.ID, owner.ID)
	wPos, _ := projects.CreatePosition(ctx, owner.ID, proj.ID, project.PositionDraft{Name: "执行"})
	wIdent, _ := projects.CreateIdentity(ctx, owner.ID, proj.ID, wPos.ID, worker.ID)

	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P", "", "", nil, nil)
	task, err := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{
		Title: "T", PlanID: &plan.ID,
		ParticipantIDs:     []uuid.UUID{wIdent.ID},
		ReviewerIdentityID: &revIdent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Pool.Exec(ctx, `UPDATE tasks SET status='ready', version=2 WHERE id=$1`, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Report(ctx, worker.ID, proj.ID, work.ReportInput{
		TaskID: task.ID, IdentityID: &wIdent.ID, Kind: "delivery",
		Text: "done", ExpectedTaskVersion: 2,
	}); err != nil {
		t.Fatal(err)
	}
	review, _ := svc.TaskAcceptanceReview(ctx, owner.ID, proj.ID, task.ID)

	// CLI (owner) creates the intent.
	intent, err := decisions.CreateIntent(ctx, owner.ID, proj.ID, nil, "task.acceptance", task.ID,
		review.ReviewHash, map[string]any{
			"decision": "accept", "expectedVersion": review.TargetVersion,
			"reviewId": review.ReviewID,
		})
	if err != nil {
		t.Fatal(err)
	}
	intentID := intent["id"].(uuid.UUID)
	nonce := intent["nonce"].(string)

	// Wrong user (worker) cannot see or confirm.
	if _, err := decisions.GetIntent(ctx, worker.ID, proj.ID, intentID); errors.IsCode(err, errors.NotFound) == false {
		t.Fatalf("wrong user get must 404, got %v", err)
	}
	if _, err := decisions.ConfirmIntent(ctx, worker.ID, proj.ID, intentID, nonce, true, nil); errors.IsCode(err, errors.NotFound) == false {
		t.Fatalf("wrong user confirm must 404, got %v", err)
	}
	// Bad nonce rejected.
	if _, err := decisions.ConfirmIntent(ctx, owner.ID, proj.ID, intentID, "bad-nonce", true, nil); errors.IsCode(err, errors.ReviewStale) == false {
		t.Fatalf("bad nonce must be stale, got %v", err)
	}
	// Owner confirms ONCE: the executor accepts the task in the same tx.
	exec := func(ctx context.Context, tx pgx.Tx, userID uuid.UUID, payload map[string]any) (string, error) {
		decisionValue, _ := payload["decision"].(string)
		expected := int64(0)
		if v, ok := payload["expectedVersion"].(float64); ok {
			expected = int64(v)
		}
		taskResult, err := svc.DecideTaskAcceptanceTx(ctx, tx, userID, proj.ID, task.ID,
			payload["reviewHash"].(string), decisionValue == "accept", "", expected)
		if err != nil {
			return "", err
		}
		return "task:" + taskResult.ID.String() + ":" + taskResult.Status, nil
	}
	result, err := decisions.ConfirmIntent(ctx, owner.ID, proj.ID, intentID, nonce, true, exec)
	if err != nil || result["state"] != "committed" {
		t.Fatalf("confirm: %v %+v", err, result)
	}
	// Task really accepted exactly once.
	var status string
	if err := fixture.Pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1`, task.ID).Scan(&status); err != nil || status != "accepted" {
		t.Fatalf("task must be accepted, got %s", status)
	}
	var acceptances int
	if err := fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM task_acceptances WHERE task_id=$1`, task.ID).Scan(&acceptances); err != nil || acceptances != 1 {
		t.Fatalf("exactly one acceptance, got %d", acceptances)
	}
	// Repeat click returns the same committed result, no second effect.
	replay, err := decisions.ConfirmIntent(ctx, owner.ID, proj.ID, intentID, nonce, true, exec)
	if err != nil || replay["state"] != "committed" {
		t.Fatalf("replay: %v %+v", err, replay)
	}
	if err := fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM task_acceptances WHERE task_id=$1`, task.ID).Scan(&acceptances); err != nil || acceptances != 1 {
		t.Fatalf("replay must not double-apply, got %d", acceptances)
	}
	// CLI reads the same resultRef.
	fetched, err := decisions.GetIntent(ctx, owner.ID, proj.ID, intentID)
	if err != nil || fetched["state"] != "committed" || fetched["resultRef"] == nil {
		t.Fatalf("cli poll: %v %+v", err, fetched)
	}
	// Reject path on a fresh intent.
	draft2, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P2", "", "", nil, nil)
	intent2, _ := decisions.CreateIntent(ctx, owner.ID, proj.ID, nil, "plan.acceptance", draft2.ID, "none", nil)
	rejected, err := decisions.ConfirmIntent(ctx, owner.ID, proj.ID, intent2["id"].(uuid.UUID), intent2["nonce"].(string), false, nil)
	if err != nil || rejected["state"] != "rejected" {
		t.Fatalf("reject: %v %+v", err, rejected)
	}
}
