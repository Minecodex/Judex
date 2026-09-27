// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/work"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newResearchEnv(t *testing.T) (*work.Service, *project.Service, *identity.Service) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return work.NewService(fixture.Pool, nil), project.NewService(fixture.Pool, nil), ids
}

// TestBugReleaseFix (B14): Bug 草稿+严重度；发布上报为事实记录；修复传播
// 按用户目标建独立任务；仓库元数据 manager 权限。
func TestBugReleaseFix(t *testing.T) {
	svc, projects, ids := newResearchEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "BA", "ba@ba.test", "password-ba-ba-1", "10.0.0.1")
	memberUser, _, _ := ids.Register(ctx, "BB", "bb@ba.test", "password-bb-bb-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "研发项目"})
	if err := projects.JoinDirect(ctx, proj.ID, memberUser.ID); err != nil {
		t.Fatal(err)
	}

	// Repository: member cannot write.
	if _, err := svc.CreateRepository(ctx, memberUser.ID, proj.ID, "主仓", "https://git/x", "", ""); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("member repo write must be forbidden, got %v", err)
	}
	repo, err := svc.CreateRepository(ctx, owner.ID, proj.ID, "主仓", "https://git/x", "gitlab", "release")
	if err != nil || repo.DefaultBranch != "release" {
		t.Fatalf("repo: %v %+v", err, repo)
	}
	updated, err := svc.UpdateRepository(ctx, owner.ID, proj.ID, repo.ID, repo.Version, "主仓2", "", "", "")
	if err != nil || updated.DisplayName != "主仓2" || updated.Version != 2 {
		t.Fatalf("repo update: %v %+v", err, updated)
	}
	if _, err := svc.UpdateRepository(ctx, owner.ID, proj.ID, repo.ID, repo.Version, "", "", "", ""); errors.IsCode(err, errors.VersionConflict) == false {
		t.Fatalf("stale repo must conflict, got %v", err)
	}

	// Bug draft with severity whitelist fallback.
	bug, err := svc.CreateBug(ctx, memberUser.ID, proj.ID, "列表崩溃", work.BugDetails{
		Environment: "staging", Steps: "1..2..3", Expected: "正常", Actual: "500",
		Severity: "critical",
	}, nil)
	if err != nil || bug.Kind != "bug" || bug.Status != "draft" {
		t.Fatalf("bug: %v %+v", err, bug)
	}

	// Release report: fact record.
	release, err := svc.ReportRelease(ctx, owner.ID, proj.ID, "v1.2.0", "staging", "https://x/v1.2.0", "success",
		[]map[string]any{{"repositoryId": repo.ID.String(), "branch": "release", "commit": "abc123"}})
	if err != nil || release.Status != "success" {
		t.Fatalf("release: %v %+v", err, release)
	}
	releases, err := svc.ListReleases(ctx, memberUser.ID, proj.ID)
	if err != nil || len(releases) != 1 {
		t.Fatalf("releases list: %v %+v", err, releases)
	}

	// Fix propagation to user-chosen targets creates independent draft tasks.
	targets, err := svc.CreateFixPropagation(ctx, owner.ID, proj.ID, bug.ID, []string{"v1.1.0", "v1.0.0"})
	if err != nil || len(targets) != 2 {
		t.Fatalf("fix propagation: %v %v", err, targets)
	}
	// Non-bug task cannot start propagation.
	plan, _ := svc.CreatePlanDraft(ctx, owner.ID, proj.ID, "P", "", "", nil, nil)
	normalTask, _ := svc.CreateTaskDraft(ctx, owner.ID, proj.ID, work.TaskDraft{Title: "N", PlanID: &plan.ID})
	if _, err := svc.CreateFixPropagation(ctx, owner.ID, proj.ID, normalTask.ID, []string{"v1"}); errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("non-bug propagation must be rejected, got %v", err)
	}
	_ = uuid.Nil
}
