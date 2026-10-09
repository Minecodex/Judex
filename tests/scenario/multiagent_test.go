// SPDX-License-Identifier: Apache-2.0

// Package multiagent_test verifies coordinator + position sub-agent
// coordination with a real model (SCENARIO_GLM_* env required).
package scenario_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/agent/multiagent"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// TestMultiAgentCoordination: coordinator 调用后端+QA 两个岗位子Agent
// 并行分析同一材料，汇总给出综合结论。
func TestMultiAgentCoordination(t *testing.T) {
	if envOr("SCENARIO_GLM_BASE_URL", "") == "" {
		t.Skip("SCENARIO_GLM_* env not set")
	}

	provider, err := model.NewProvider(model.GatewayConfig{
		Protocol: envOr("SCENARIO_GLM_PROTOCOL", model.ProtocolOpenAI),
		BaseURL:  envOr("SCENARIO_GLM_BASE_URL", ""),
		APIKey:   envOr("SCENARIO_GLM_API_KEY", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	modelName := envOr("SCENARIO_GLM_MODEL", "glm-5.3")

	// 注册两个岗位
	positions := tools.NewPositionRegistry()
	positions.Register("后端工程师", "你是后端工程师。关注实现方案、性能、幂等性和技术风险。给出具体的技术建议。")
	positions.Register("质量验收", "你是质量验收工程师。关注测试覆盖、边界条件、验收标准和潜在风险。给出验收建议。")

	// 构建 coordinator
	coordinator := &multiagent.Coordinator{
		ModelName: modelName,
		Positions: positions,
		Provider:  provider,
	}

	material := "【交付材料】用户注册接口新增手机号验证。改动：1) 新增 /verify-phone 端点；2) 密码规则从 8 位改为 12 位；3) 未加幂等键（同一手机号可重复发码）；4) 无速率限制。"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectID := uuid.NewString()
	result, err := coordinator.AnalyzeWithPositions(ctx, projectID, material, []string{"后端工程师", "质量验收"})
	if err != nil {
		// 多Agent 是新功能，如果超时可以记录但不硬失败
		t.Fatalf("coordinator: %v (state=%s)", err, result.State)
	}

	t.Logf("coordinator summary: %.400s", result.Summary)
	t.Logf("tokens=%d toolCalls=%d", result.TokensUsed, result.ToolCalls)

	// 验证 coordinator 引用了岗位视角
	if !strings.Contains(result.Summary, "后端") && !strings.Contains(result.Summary, "质量") {
		t.Errorf("coordinator 应引用岗位名称:\n%.300s", result.Summary)
	}
	// 验证识别了关键缺陷
	if !strings.Contains(result.Summary, "幂等") {
		t.Errorf("coordinator 应识别幂等缺陷:\n%.300s", result.Summary)
	}
}

// TestMultiAgentSinglePosition: 只调用一个岗位也能工作。
func TestMultiAgentSinglePosition(t *testing.T) {
	if envOr("SCENARIO_GLM_BASE_URL", "") == "" {
		t.Skip("SCENARIO_GLM_* env not set")
	}
	provider, err := model.NewProvider(model.GatewayConfig{
		Protocol: envOr("SCENARIO_GLM_PROTOCOL", model.ProtocolOpenAI),
		BaseURL:  envOr("SCENARIO_GLM_BASE_URL", ""),
		APIKey:   envOr("SCENARIO_GLM_API_KEY", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	modelName := envOr("SCENARIO_GLM_MODEL", "glm-5.3")

	positions := tools.NewPositionRegistry()
	positions.Register("数据分析", "你是数据分析师。关注数据趋势、异常值和统计显著性。")

	coordinator := &multiagent.Coordinator{
		ModelName: modelName,
		Positions: positions,
		Provider:  provider,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result, err := coordinator.AnalyzeWithPositions(ctx, uuid.NewString(),
		"以下是用户数据：日活从1200降到900，错误率从0.5%升到5.8%",
		[]string{"数据分析"})
	if err != nil {
		t.Fatalf("single position: %v", err)
	}
	if !strings.Contains(result.Summary, "数据") && !strings.Contains(result.Summary, "下降") {
		t.Errorf("分析应引用数据趋势:\n%.300s", result.Summary)
	}
	t.Logf("single position result: %.200s", result.Summary)
}
