// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/agent"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	"github.com/kakj-go/Judex/internal/workflow"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newPositionEnv(t *testing.T) (*project.Service, *workflow.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	return project.NewService(fixture.Pool, nil), workflow.NewService(fixture.Pool, nil), ids, fixture.Pool
}

// TestPositionsInvitationsIdentities (A05/A06/A08 后端核心): 职位版本化、
// 节点绑定校验（须已发布 workflow）、邀请 token 接受（重试不复制身份）、
// 同岗多人/多岗、替换保留历史、偏好版本化。
func TestPositionsInvitationsIdentities(t *testing.T) {
	svc, wf, ids, pool := newPositionEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "PO", "po@po.test", "password-po-po-11", "10.0.0.1")

	// 未发布 workflow 前绑节点应被拒。
	proj, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "岗位项目"})
	if err != nil {
		t.Fatal(err)
	}
	wfDef, err := wf.Create(ctx, owner.ID, proj.ID, workflow.Body{
		Name:  "流程",
		Nodes: []workflow.Node{{ID: "dev", Name: "开发", DefaultApprovalPolicy: "all"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wf.Publish(ctx, owner.ID, proj.ID, wfDef.ID, wfDef.Version, ""); err != nil {
		t.Fatal(err)
	}

	draft := project.PositionDraft{
		Name: "后端工程师", Prompt: "负责服务端实现",
		NodeBindings: []project.NodeBinding{{WorkflowID: wfDef.ID, NodeID: "dev"}},
	}
	if _, err := svc.CreatePosition(ctx, owner.ID, proj.ID, draft); err != nil {
		t.Fatal(err)
	}
	badDraft := draft
	badDraft.NodeBindings = []project.NodeBinding{{WorkflowID: wfDef.ID, NodeID: "ghost"}}
	if _, err := svc.CreatePosition(ctx, owner.ID, proj.ID, badDraft); errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("unpublished/unknown node binding must be rejected, got %v", err)
	}
	positions, err := svc.ListPositions(ctx, owner.ID, proj.ID)
	if err != nil || len(positions) != 1 {
		t.Fatalf("positions: %v %+v", err, positions)
	}
	positionID := positions[0].ID
	// 修改产生 revision 2。
	draft.Prompt = "负责服务端实现与评审"
	updated, err := svc.UpdatePosition(ctx, owner.ID, proj.ID, positionID, 1, draft)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update: %v %+v", err, updated)
	}
	if _, err := svc.UpdatePosition(ctx, owner.ID, proj.ID, positionID, 1, draft); errors.IsCode(err, errors.VersionConflict) == false {
		t.Fatalf("stale position version must conflict, got %v", err)
	}

	// 邀请：未注册邮箱 → 注册 → token 接受。
	invitation, err := svc.CreateInvitation(ctx, owner.ID, proj.ID, "Newbie@PO.test", []uuid.UUID{positionID})
	if err != nil {
		t.Fatal(err)
	}
	if invitation.Token == "" {
		t.Fatal("invitation must carry a one-time token")
	}
	newbie, _, err := ids.Register(ctx, "NB", "newbie@po.test", "password-nb-nb-11", "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	// 错邮箱账号不能接受。
	other, _, _ := ids.Register(ctx, "OT", "other@po.test", "password-ot-ot-11", "10.0.0.2")
	if _, _, err := svc.AcceptInvitation(ctx, other.ID, invitation.ID, invitation.Token); errors.IsCode(err, errors.Forbidden) == false {
		t.Fatalf("wrong email accept must be forbidden, got %v", err)
	}
	gotIdentities, gotProject, err := svc.AcceptInvitation(ctx, newbie.ID, invitation.ID, invitation.Token)
	if err != nil || gotProject != proj.ID || len(gotIdentities) != 1 {
		t.Fatalf("accept: %v %d identities", err, len(gotIdentities))
	}
	// 重试接受不复制身份/成员。
	if _, _, err := svc.AcceptInvitation(ctx, newbie.ID, invitation.ID, invitation.Token); err != nil {
		t.Fatalf("idempotent accept retry: %v", err)
	}
	allIdentities, err := svc.ListIdentities(ctx, newbie.ID, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	positionIdentityCount := 0
	for _, id := range allIdentities {
		if id.Kind == "position" {
			positionIdentityCount++
		}
	}
	if positionIdentityCount != 1 {
		t.Fatalf("retry must not duplicate identities, got %d", positionIdentityCount)
	}
	var members int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`, proj.ID, newbie.ID).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if members != 1 {
		t.Fatalf("expected exactly 1 membership row, got %d", members)
	}

	// 非成员不能被直接任职。
	if _, err := svc.CreateIdentity(ctx, owner.ID, proj.ID, positionID, other.ID); errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("identity for non-member must be INVALID_REFERENCE, got %v", err)
	}
	// 邀请 other 加入（同岗第二人）。
	inv2, err := svc.CreateInvitation(ctx, owner.ID, proj.ID, "other@po.test", []uuid.UUID{positionID})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AcceptInvitation(ctx, other.ID, inv2.ID, inv2.Token); err != nil {
		t.Fatalf("second accept: %v", err)
	}
	allIdentities, _ = svc.ListIdentities(ctx, owner.ID, proj.ID)
	positionIdentityCount = 0
	for _, id := range allIdentities {
		if id.Kind == "position" {
			positionIdentityCount++
		}
	}
	if positionIdentityCount != 2 {
		t.Fatalf("expected 2 same-position identities after both invites, got %d", positionIdentityCount)
	}
	// 显式第三席（同岗多人）：CreateIdentity 生成独立 identity。
	third, _, _ := ids.Register(ctx, "TH", "third@po.test", "password-th-th-11", "10.0.0.2")
	if _, err := pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role, state, joined_at)
		VALUES ($1,$2,'member','active',now())`, proj.ID, third.ID); err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateIdentity(ctx, owner.ID, proj.ID, positionID, third.ID)
	if err != nil || second.CurrentBindingVersion != 1 {
		t.Fatalf("third seat: %v %+v", err, second)
	}

	// 替换任职：保留 identity，binding version+1，旧 binding 关闭。
	replaced, err := svc.ReplaceIdentityBinding(ctx, owner.ID, proj.ID, gotIdentities[0].ID, other.ID, 1, "换人接手")
	if err != nil || replaced.CurrentBindingVersion != 2 {
		t.Fatalf("replace: %v %+v", err, replaced)
	}
	if _, err := svc.ReplaceIdentityBinding(ctx, owner.ID, proj.ID, gotIdentities[0].ID, newbie.ID, 1, "旧版本"); errors.IsCode(err, errors.VersionConflict) == false {
		t.Fatalf("stale binding must conflict, got %v", err)
	}
	var closedBindings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM identity_bindings WHERE identity_id=$1 AND valid_until IS NOT NULL`, gotIdentities[0].ID).Scan(&closedBindings); err != nil {
		t.Fatal(err)
	}
	if closedBindings != 1 {
		t.Fatalf("old binding must be closed exactly once, got %d", closedBindings)
	}

	// 个人偏好：版本化 + 本人可见（其他成员也是 null 默认）。
	prefs, err := svc.GetMyPreferences(ctx, newbie.ID, proj.ID)
	if err != nil || prefs.Revision != 0 || prefs.Prompt != "" {
		t.Fatalf("default prefs: %+v %v", prefs, err)
	}
	updated1, err := svc.UpdateMyPreferences(ctx, newbie.ID, proj.ID, 0, "偏好简洁报告")
	if err != nil || updated1.Revision != 1 {
		t.Fatalf("prefs update: %v %+v", err, updated1)
	}
	if _, err := svc.UpdateMyPreferences(ctx, newbie.ID, proj.ID, 0, "旧版本"); errors.IsCode(err, errors.VersionConflict) == false {
		t.Fatalf("stale prefs must conflict, got %v", err)
	}
	otherView, err := svc.GetMyPreferences(ctx, other.ID, proj.ID)
	if err != nil || otherView.Prompt != "" {
		t.Fatalf("other member sees own empty prefs, not newbie's: %+v %v", otherView, err)
	}
	var revisions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM preference_revisions WHERE project_id=$1 AND user_id=$2`, proj.ID, newbie.ID).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if revisions != 1 {
		t.Fatalf("expected 1 preference revision, got %d", revisions)
	}
}

// TestModelCatalogAndProjectConfig (E03/A08 基础): catalog 幂等同步、禁用
// 缺失项；项目默认模型必须存在且启用；配置上限校验。
func TestModelCatalogAndProjectConfig(t *testing.T) {
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	svc := project.NewService(fixture.Pool, nil)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "MC", "mc@mc.test", "password-mc-mc-11", "10.0.0.1")
	proj, err := svc.Create(ctx, owner.ID, project.CreateRequest{Title: "模型项目"})
	if err != nil {
		t.Fatal(err)
	}

	m1 := uuid.NewString()
	entries := []agent.CatalogEntry{{
		ID: m1, DisplayName: "内部网关模型", Provider: "openai-compatible",
		BaseURL: "http://gateway.internal/v1", APIKeyEnv: "JUDEX_MODEL_KEY_1",
		ModelName: "gpt-internal", MaxInputTokens: 128000, MaxOutputTokens: 8192,
		ToolCalling: true, Streaming: true,
	}}
	if err := agent.SyncCatalog(ctx, fixture.Pool, entries, time.Now()); err != nil {
		t.Fatal(err)
	}
	// 幂等重跑 + 移除后停用。
	entries2 := []agent.CatalogEntry{{
		ID: uuid.NewString(), DisplayName: "替代模型", Provider: "openai-compatible",
		BaseURL: "http://gateway2.internal/v1", APIKeyEnv: "JUDEX_MODEL_KEY_2",
		ModelName: "alt", MaxInputTokens: 32000, MaxOutputTokens: 4096,
	}}
	if err := agent.SyncCatalog(ctx, fixture.Pool, entries2, time.Now()); err != nil {
		t.Fatal(err)
	}
	models, err := agent.ListPublicModels(ctx, fixture.Pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 catalog rows kept, got %d", len(models))
	}
	enabled := 0
	for _, m := range models {
		if m.Enabled {
			enabled++
		}
		if m.Limits == nil || m.Limits["maxInputTokens"] == 0 {
			t.Fatalf("public model missing limits: %+v", m)
		}
	}
	if enabled != 1 {
		t.Fatalf("expected exactly 1 enabled after resync, got %d", enabled)
	}

	// 项目配置：非法模型被拒。
	badModel := uuid.New()
	_, err = svc.Update(ctx, owner.ID, proj.ID, proj.Version, project.UpdateRequest{
		DefaultModelID: &badModel, DefaultModelSet: true,
	}, func(ctx context.Context, id uuid.UUID) error { _, err := agent.ModelEnabled(ctx, fixture.Pool, id); return err })
	if errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("unknown model must be InvalidReference, got %v", err)
	}
	// 禁用模型被拒。
	disabledID, _ := uuid.Parse(m1)
	if _, err = svc.Update(ctx, owner.ID, proj.ID, proj.Version, project.UpdateRequest{
		DefaultModelID: &disabledID, DefaultModelSet: true,
	}, func(ctx context.Context, id uuid.UUID) error { _, err := agent.ModelEnabled(ctx, fixture.Pool, id); return err }); errors.IsCode(err, errors.InvalidReference) == false {
		t.Fatalf("disabled model must be rejected, got %v", err)
	}
	// 启用模型 + 轮次超范围拒绝。
	enabledID := models[0].ID
	if models[0].Enabled {
		enabledID = models[0].ID
	} else {
		enabledID = models[1].ID
	}
	rounds := 101
	_, err = svc.Update(ctx, owner.ID, proj.ID, proj.Version, project.UpdateRequest{
		MaxDiscussionRounds: &rounds,
	}, nil)
	if errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("rounds > 100 must be rejected, got %v", err)
	}
	updated, err := svc.Update(ctx, owner.ID, proj.ID, proj.Version, project.UpdateRequest{
		DefaultModelID: &enabledID, DefaultModelSet: true,
	}, func(ctx context.Context, id uuid.UUID) error { _, err := agent.ModelEnabled(ctx, fixture.Pool, id); return err })
	if err != nil || updated.DefaultModelID == nil || *updated.DefaultModelID != enabledID {
		t.Fatalf("model set: %v %+v", err, updated)
	}
}
