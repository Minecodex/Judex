// SPDX-License-Identifier: Apache-2.0

// Phase 6 场景扩展（真实 GLM-5.3）：验证产品核心假设。
// SCENARIO_GLM_* 环境变量必须提供；否则 skip。
package scenario_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

func scenarioRunner(t *testing.T) (*runner.Runner, string) {
	t.Helper()
	if envOr("SCENARIO_GLM_BASE_URL", "") == "" || envOr("SCENARIO_GLM_API_KEY", "") == "" {
		t.Skip("SCENARIO_GLM_* env not set; real model tests skip per plan rules")
	}
	protocol := envOr("SCENARIO_GLM_PROTOCOL", model.ProtocolOpenAI)
	provider, err := model.NewProvider(model.GatewayConfig{
		Protocol: protocol,
		BaseURL:  envOr("SCENARIO_GLM_BASE_URL", ""),
		APIKey:   envOr("SCENARIO_GLM_API_KEY", ""),
	})
	if err != nil {
		t.Skipf("SCENARIO_GLM_* env: %v", err)
	}
	registry := tools.New()
	tools.RegisterDefaults(registry)
	return runner.NewRunnerForTest(provider, registry), envOr("SCENARIO_GLM_MODEL", "glm-5.3")
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func runScenario(t *testing.T, modelName string, facts agentcontext.Facts, timeout time.Duration) runner.Outcome {
	t.Helper()
	r, _ := scenarioRunner(t)
	_ = modelName
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), ModelName: modelName,
		Manifest: agentcontext.Build(facts),
		Budget:   runner.Budget{MaxRounds: 1, MaxModelAttempts: 8, MaxTotalTokens: 200000, MaxWallClock: timeout},
	})
}

// 场景1：岗位差异化——同一材料，后端岗 vs QA 岗输出角度不同。
func TestPositionDifferentiation(t *testing.T) {
	r, name := scenarioRunner(t)
	material := "【交付材料】用户注册接口新增手机号验证。改动：1) 新增 /verify-phone 端点；2) 密码规则从 8 位改为 12 位；3) 未加幂等键（同一手机号可重复发码）；4) 无速率限制。"

	runWith := func(position, prompt string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		outcome := r.Run(ctx, runner.RunRequest{
			RunID: uuid.New(), ModelName: name,
			Manifest: agentcontext.Build(agentcontext.Facts{
				PositionPrompt: prompt,
				NewMaterial:    material,
			}),
			Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 8, MaxTotalTokens: 200000, MaxWallClock: 3 * time.Minute},
		})
		if outcome.State != "succeeded" {
			t.Fatalf("[%s] state=%s: %s", position, outcome.State, outcome.Summary)
		}
		return outcome.Summary
	}

	backend := runWith("后端", "你是后端工程师，关注实现方案、代码质量、性能和幂等性。")
	qa := runWith("验收", "你是质量验收工程师，关注测试覆盖、验收标准、边界条件和风险。")

	t.Logf("backend head: %.200s", backend)
	t.Logf("qa head: %.200s", qa)

	// 验证差异化：后端应更多提及实现/性能，QA 应更多提及测试/风险
	backendImpl := strings.Contains(backend, "实现") || strings.Contains(backend, "性能") || strings.Contains(backend, "方案")
	qaTest := strings.Contains(qa, "测试") || strings.Contains(qa, "验收") || strings.Contains(qa, "风险") || strings.Contains(qa, "覆盖")

	if !backendImpl {
		t.Errorf("后端岗分析应关注实现/性能/方案:\n%.300s", backend)
	}
	if !qaTest {
		t.Errorf("验收岗分析应关注测试/验收/风险/覆盖:\n%.300s", qa)
	}
	// 两者都应提到幂等问题（材料中的关键缺陷）
	if !strings.Contains(backend, "幂等") && !strings.Contains(qa, "幂等") {
		t.Errorf("两个岗位都应识别幂等缺陷")
	}
}

// 场景2：Agent 交付预审——分析交付材料，建议验收/退回+理由。
func TestDeliveryPreReview(t *testing.T) {
	r, name := scenarioRunner(t)
	delivery := "【交付报告】\n功能：用户注册手机号验证\n代码改动：3 个文件，+120/-15 行\n测试：单元测试 8 个通过（覆盖率 85%）\n已知限制：1) 未做速率限制（计划下版本）；2) 幂等键未加（缺陷#415）\n性能：新接口 P99=180ms"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			PositionPrompt: "你是质量验收工程师，需要对交付材料做预审并给出建议（验收/退回）和理由。",
			NewMaterial:    delivery,
			WorkFacts:      []string{"验收标准：所有已知缺陷必须修复或有明确的下版本计划才能验收"},
		}),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 8, MaxTotalTokens: 200000, MaxWallClock: 3 * time.Minute},
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s: %s", outcome.State, outcome.Summary)
	}
	// 应给出验收/退回建议并引用材料事实
	s := outcome.Summary
	hasVerdict := strings.Contains(s, "验收") || strings.Contains(s, "退回") || strings.Contains(s, "建议")
	hasReference := strings.Contains(s, "幂等") || strings.Contains(s, "速率") || strings.Contains(s, "#415")
	if !hasVerdict {
		t.Errorf("预审应给出验收/退回建议:\n%.400s", s)
	}
	if !hasReference {
		t.Errorf("预审应引用材料中的具体缺陷:\n%.400s", s)
	}
	t.Logf("预审结果: %.300s", s)
}

// 场景3：退回→修订→再分析。
func TestRejectReviseReanalyze(t *testing.T) {
	r, name := scenarioRunner(t)
	original := "【方案 v1】用 Redis SETNX 实现幂等。"
	feedback := "退回理由：未说明 Redis 不可用时的降级方案，且未量化 QPS 目标。"
	revised := "【方案 v2】用 Redis SETNX 实现幂等，QPS 目标 5000。Redis 不可用时降级为 PG advisory lock（P99 预估 45ms，可接受）。已加压测脚本。"

	// 第一轮分析
	ctx1, c1 := context.WithTimeout(context.Background(), 2*time.Minute)
	defer c1()
	v1 := r.Run(ctx1, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{NewMaterial: original}),
		Budget:   runner.Budget{MaxRounds: 1, MaxModelAttempts: 5, MaxTotalTokens: 150000, MaxWallClock: 2 * time.Minute},
	})
	if v1.State != "succeeded" {
		t.Fatalf("v1 state=%s", v1.State)
	}

	// 第二轮：带退回理由 + 修订版
	ctx2, c2 := context.WithTimeout(context.Background(), 2*time.Minute)
	defer c2()
	v2 := r.Run(ctx2, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			Disagreements:  []string{feedback},
			NewMaterial:    revised,
			RecentMessages: []string{"原始方案：" + original},
		}),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 5, MaxTotalTokens: 150000, MaxWallClock: 2 * time.Minute},
	})
	if v2.State != "succeeded" {
		t.Fatalf("v2 state=%s: %s", v2.State, v2.Summary)
	}

	// 修订版应被识别为改进
	s := v2.Summary
	if !strings.Contains(s, "降级") && !strings.Contains(s, "advisory") {
		t.Errorf("修订分析应引用降级方案:\n%.300s", s)
	}
	if !strings.Contains(s, "5000") && !strings.Contains(s, "QPS") {
		t.Errorf("修订分析应引用 QPS 目标:\n%.300s", s)
	}
	t.Logf("修订分析: %.300s", s)
}

// 场景4：Bug 根因分析。
func TestBugRootCause(t *testing.T) {
	r, name := scenarioRunner(t)
	bugReport := "【Bug 报告】\n标题：偶发订单金额为 0\n严重度：高\n复现步骤：1) 并发提交两个不同商品到购物车；2) 快速连续点击结算\n预期：两笔订单金额正确\n实际：约 5% 概率其中一笔金额为 0\n环境：staging v2.3.1\n日志：结算时购物车服务返回空 items 列表"

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			PositionPrompt: "你是后端工程师，分析 Bug 根因和影响范围。",
			NewMaterial:    bugReport,
		}),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 8, MaxTotalTokens: 200000, MaxWallClock: 3 * time.Minute},
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s: %s", outcome.State, outcome.Summary)
	}
	// 应分析出并发/竞态相关根因
	s := outcome.Summary
	hasRace := strings.Contains(s, "并发") || strings.Contains(s, "竞态") || strings.Contains(s, "race") || strings.Contains(s, "锁")
	if !hasRace {
		t.Errorf("Bug 根因分析应识别并发/竞态问题:\n%.400s", s)
	}
	t.Logf("根因分析: %.300s", s)
}

// 场景5：Agent 读材料原文（通过上下文注入实际内容）。
func TestAgentReadsMaterialContent(t *testing.T) {
	r, name := scenarioRunner(t)
	// 模拟一个 CSV 数据文件内容
	csvData := `date,user_count,revenue,error_rate
2026-09-01,1200,45000,0.5%
2026-09-02,1350,52300,0.4%
2026-09-03,1100,38200,2.1%
2026-09-04,1400,61000,0.3%
2026-09-05,1280,49000,0.6%
2026-09-06,900,28500,5.8%`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			PositionPrompt: "你是数据分析师。",
			NewMaterial:    "以下是用户增长数据 CSV：\n" + csvData + "\n请分析趋势和异常。",
		}),
		Budget: runner.Budget{MaxRounds: 1, MaxModelAttempts: 8, MaxTotalTokens: 200000, MaxWallClock: 3 * time.Minute},
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s: %s", outcome.State, outcome.Summary)
	}
	// 应引用具体数据点
	s := outcome.Summary
	if !strings.Contains(s, "5.8") && !strings.Contains(s, "900") && !strings.Contains(s, "28500") {
		t.Errorf("分析应引用 CSV 中的具体数据点（9月6日异常）:\n%.400s", s)
	}
	if !strings.Contains(s, "9月6") && !strings.Contains(s, "09-06") && !strings.Contains(s, "周末") {
		t.Errorf("分析应识别9月6日为异常日:\n%.400s", s)
	}
	t.Logf("数据分析: %.300s", s)
}
