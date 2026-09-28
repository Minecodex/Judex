// SPDX-License-Identifier: Apache-2.0

// Package live contains REAL-model acceptance tests (docs/plans/v1/05 §10、
// 10 E11)：全部跳过，除非提供以下环境变量——
//
//	JUDEX_LIVE_PROTOCOL   = anthropic-compatible | openai-compatible
//	JUDEX_LIVE_BASE_URL   = 网关根地址（如 https://open.bigmodel.cn/api/anthropic）
//	JUDEX_LIVE_API_KEY    = 密钥（只从环境读取，绝不写入仓库）
//	JUDEX_LIVE_MODEL      = 模型名（如 glm-5.3）
//
// 覆盖：真实流式/usage（E03）、工具调用循环、中文研发/设计双材料分析
// （E11）、预算熔断（E03）、取消传播、压缩保异议（E05）。
// 沙箱浏览器证据（HTML 交互检查）依赖 OpenSandbox，不在本文件范围。
package live_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/google/uuid"
)

func liveConfig(t *testing.T) (model.GatewayConfig, string) {
	t.Helper()
	cfg := model.GatewayConfig{
		Protocol: os.Getenv("JUDEX_LIVE_PROTOCOL"),
		BaseURL:  os.Getenv("JUDEX_LIVE_BASE_URL"),
		APIKey:   os.Getenv("JUDEX_LIVE_API_KEY"),
	}
	name := os.Getenv("JUDEX_LIVE_MODEL")
	if cfg.BaseURL == "" || cfg.APIKey == "" || name == "" {
		t.Skip("live model env 未配置（JUDEX_LIVE_*）：按计划规则保持 blocked，不 mock 通过")
	}
	if cfg.Protocol == "" {
		cfg.Protocol = model.ProtocolAnthropic
	}
	return cfg, name
}

func newLiveRunner(t *testing.T, env tools.Env) (*runner.Runner, string) {
	t.Helper()
	cfg, name := liveConfig(t)
	provider, err := model.NewProvider(cfg)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	registry := tools.New()
	tools.RegisterDefaults(registry)
	registry.Register(tools.Tool{
		Name:        "fake_read_material",
		Description: "读取固定材料版本内容（live 测试桩，返回注入的中文材料）",
		InputSchema: map[string]any{
			"type": "object", "required": []string{"versionId"},
			"properties": map[string]any{"versionId": map[string]any{"type": "string"}},
		},
		Effect: tools.EffectRead,
		Execute: func(ctx context.Context, args map[string]any, e tools.Env) (tools.Result, error) {
			return tools.Result{Data: map[string]any{"content": env.ReadMaterialContent}}, nil
		},
	})
	// Minimal in-memory projections so the model converges instead of
	// burning attempts on "未接线" errors.
	registry.Register(tools.Tool{
		Name:        "list_agent",
		Description: "列出本项目身份与公开职责",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Effect:      tools.EffectRead,
		Execute: func(ctx context.Context, args map[string]any, e tools.Env) (tools.Result, error) {
			return tools.Result{Data: map[string]any{"agents": []map[string]any{
				{"identityId": "coordinator", "kind": "coordinator", "holder": "甲"},
				{"identityId": "pos-backend", "kind": "position", "name": "后端工程师", "holder": "乙"},
			}}}, nil
		},
	})
	registry.Register(tools.Tool{
		Name:        "query_work",
		Description: "分页查询本项目工作事实",
		InputSchema: map[string]any{
			"type": "object", "required": []string{"objectType"},
			"properties": map[string]any{"objectType": map[string]any{"type": "string"},
				"cursor": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}},
		},
		Effect: tools.EffectRead,
		Execute: func(ctx context.Context, args map[string]any, e tools.Env) (tools.Result, error) {
			objectType, _ := args["objectType"].(string)
			return tools.Result{Data: map[string]any{"items": []map[string]any{
				{"objectType": objectType, "id": "obj-1", "status": "active", "title": "订单服务稳定性"},
				{"objectType": objectType, "id": "obj-2", "status": "pending", "title": "首页改版"},
			}}}, nil
		},
	})
	return runner.NewRunnerForTest(provider, registry), name
}

// E11a 研发中文材料：真实模型经上下文 manifest 分析接口设计材料，产出
// 可审阅建议并引用材料内容（不虚构）。
func TestLiveResearchScenario(t *testing.T) {
	material := "【接口设计文档 v3】\n1. /api/orders 创建订单：入参 userId, items[]；返回 orderId。\n" +
		"2. 并发下单未做幂等：同一请求重试会创建重复订单（已知缺陷 #412）。\n" +
		"3. 库存扣减同步调用 inventory-service，超时阈值 3s，无降级路径。\n" +
		"4. 测试负责人备注：压测显示 P99 已到 2.8s，接近阈值。"
	env := tools.Env{ProjectID: uuid.NewString(), RunID: uuid.NewString(), ReadMaterialContent: material}
	r, name := newLiveRunner(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), SessionID: uuid.New(), ProjectID: uuid.New(), IdentityID: uuid.New(),
		ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			WorkFacts:   []string{"缺陷 #412：下单接口无幂等控制，状态 open"},
			NewMaterial: material,
		}),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 6, MaxTotalTokens: 200000, MaxWallClock: 150 * time.Second},
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s summary=%s", outcome.State, outcome.Summary)
	}
	if !outcome.UsageKnown {
		t.Fatal("真实模型必须返回 usage（缺失即失败，不记零）")
	}
	t.Logf("tokens=%d toolCalls=%d summaryHead=%s", outcome.TokensUsed, outcome.ToolCalls,
		firstN(outcome.Summary, 120))
	// 材料引用核对：模型须提及材料中的关键事实而非虚构。
	for _, keyword := range []string{"幂等", "订单"} {
		if !strings.Contains(outcome.Summary, keyword) {
			t.Errorf("建议未引用材料关键事实 %q:\n%s", keyword, firstN(outcome.Summary, 400))
		}
	}
}

// E11b 设计中文材料：设计稿审阅意见需保留异议（压缩前不应丢失）。
func TestLiveDesignScenarioDisagreement(t *testing.T) {
	env := tools.Env{ProjectID: uuid.NewString(), RunID: uuid.NewString(),
		ReadMaterialContent: "【首页改版设计稿】采用三栏布局；主色 #34483b；客服入口移到右上角。"}
	r, name := newLiveRunner(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), SessionID: uuid.New(), ProjectID: uuid.New(), IdentityID: uuid.New(),
		ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			WorkFacts:     []string{"设计岗异议：主色过深，夜间模式对比度不足（待决）"},
			Disagreements: []string{"主色对比度异议：设计岗认为不可用，产品岗认为可接受"},
			NewMaterial:   env.ReadMaterialContent,
		}),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 6, MaxTotalTokens: 200000, MaxWallClock: 150 * time.Second},
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s summary=%s", outcome.State, outcome.Summary)
	}
	// E05：压缩摘要必须保留异议。
	compressed := agentcontext.CompressSummary(
		[]string{"设计稿分析"}, []string{"主色对比度异议未决"}, []string{"等待产品岗回复"}, "复核对比度后回复")
	if !strings.Contains(compressed, "主色对比度异议未决") {
		t.Fatal("压缩摘要丢失异议")
	}
	if !strings.Contains(compressed, "等待产品岗回复") {
		t.Fatal("压缩摘要丢失待决事项")
	}
	t.Logf("design summary head: %s", firstN(outcome.Summary, 120))
}

// E03 预算熔断：真实 usage 计入预算，上限后停止（合法续开语义保留）。
func TestLiveBudgetEnforcement(t *testing.T) {
	r, name := newLiveRunner(t, tools.Env{})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{NewMaterial: "测试预算熔断"}),
		// 极小 token 预算：真实模型第一步 usage 即超。
		Budget: runner.Budget{MaxRounds: 3, MaxModelAttempts: 10, MaxTotalTokens: 1, MaxWallClock: 100 * time.Second},
	})
	if outcome.State != "failed" || !strings.Contains(outcome.Summary, "预算") {
		t.Fatalf("预算必须在真实 usage 后熔断: %+v", outcome)
	}
	if !outcome.UsageKnown {
		t.Fatal("熔断前必须已获得真实 usage")
	}
}

// 取消传播：真实流式中途取消 → cancelled（不是 failed）。
func TestLiveCancellation(t *testing.T) {
	r, name := newLiveRunner(t, tools.Env{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(3 * time.Second)
		cancel()
	}()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{NewMaterial: "写一篇 800 字的短文，主题：软件工程中的权衡"}),
		Budget:  runner.DefaultBudget(),
	})
	if outcome.State != "cancelled" {
		t.Fatalf("state=%s（应为 cancelled）summary=%s", outcome.State, outcome.Summary)
	}
}

// Provider 直连：真实网关流式 + 工具调用 + usage 一次核对。
func TestLiveProviderContract(t *testing.T) {
	cfg, name := liveConfig(t)
	provider, err := model.NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ch, err := provider.Stream(ctx, model.Request{
		Model: name, MaxOutputTokens: 2048,
		Messages: []model.Message{{Role: "user", Content: "查看 release-notes.md 的内容"}},
		Tools: []model.ToolSchema{{
			Name: "read", InputSchema: map[string]any{
				"type": "object", "required": []string{"path"},
				"properties": map[string]any{"path": map[string]any{"type": "string"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("stream open: %v (class=%d)", err, model.Classify(err))
	}
	var text strings.Builder
	var toolsReady []model.ToolCall
	var usage *model.Event
	finish := false
	for ev := range ch {
		switch ev.Type {
		case "textDelta":
			text.WriteString(ev.TextDelta)
		case "toolCallReady":
			toolsReady = append(toolsReady, model.ToolCall{ID: ev.ToolCallID, Name: ev.ToolName, Arguments: ev.ArgsJSON})
		case "usage":
			u := ev
			usage = &u
		case "finish":
			finish = true
		case "error":
			t.Fatalf("stream error: %v (class=%d)", ev.Err, model.Classify(ev.Err))
		}
	}
	if !finish {
		t.Fatal("finish missing")
	}
	if usage == nil || usage.InputTokens <= 0 {
		t.Fatalf("usage=%+v（真实网关必须返回正数 usage）", usage)
	}
	t.Logf("provider contract: finish=%v tools=%d text=%q usage=%d/%d",
		finish, len(toolsReady), firstN(text.String(), 40), usage.InputTokens, usage.OutputTokens)
}

func firstN(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
