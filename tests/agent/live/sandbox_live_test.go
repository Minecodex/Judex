// SPDX-License-Identifier: Apache-2.0

//go:build live

// Live test against a real OpenSandbox deployment (verified contract,
// server v0.2.2). Requires:
//
//	JUDEX_OPENSANDBOX_ENDPOINT  e.g. http://127.0.0.1:18096
//	JUDEX_OPENSANDBOX_API_KEY    the OPEN-SANDBOX-API-KEY value
//
// Run: go test -tags live ./tests/agent/live/ -run TestLiveSandbox -v
package live_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	agentcontext "github.com/kakj-go/Judex/internal/agent/context"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
)

func liveSandboxClient(t *testing.T) opensandbox.Sandbox {
	t.Helper()
	endpoint := os.Getenv("JUDEX_OPENSANDBOX_ENDPOINT")
	key := os.Getenv("JUDEX_OPENSANDBOX_API_KEY")
	if endpoint == "" || key == "" {
		t.Skip("JUDEX_OPENSANDBOX_ENDPOINT / JUDEX_OPENSANDBOX_API_KEY not set")
	}
	return opensandbox.New(opensandbox.Config{Endpoint: endpoint, APIKey: key})
}

func TestLiveSandboxLifecycleAndExec(t *testing.T) {
	c := liveSandboxClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	id, err := c.Create(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = c.Kill(context.WithoutCancel(ctx), id) }()
	t.Logf("sandbox %s running", id)

	res, err := c.Exec(ctx, id, "echo hello-judex && uname -s", 30*time.Second)
	if err != nil || res.Unknown {
		t.Fatalf("exec: %v unknown=%v", err, res.Unknown)
	}
	if res.ExitCode != 0 || !strings.Contains(string(res.Stdout), "hello-judex") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", res.ExitCode, res.Stdout, res.Stderr)
	}
	t.Logf("stdout: %s", res.Stdout)

	// Failure carries the real exit code through error.evalue.
	fail, err := c.Exec(ctx, id, "echo oops >&2; exit 3", 30*time.Second)
	if err != nil {
		t.Fatalf("failed exec should not be transport error: %v", err)
	}
	if fail.ExitCode != 3 || !strings.Contains(string(fail.Stderr), "oops") {
		t.Fatalf("exit=%d stderr=%q", fail.ExitCode, fail.Stderr)
	}

	// File round-trip via base64 write + read (concrete-client helpers).
	fc, ok := c.(interface {
		ReadFile(ctx context.Context, externalID, path string) ([]byte, error)
		WriteFile(ctx context.Context, externalID, path string, content []byte) error
	})
	if !ok {
		t.Fatal("client does not expose file helpers")
	}
	if err := fc.WriteFile(ctx, id, "/tmp/judex.txt", []byte("材料内容-123")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := fc.ReadFile(ctx, id, "/tmp/judex.txt")
	if err != nil || string(got) != "材料内容-123" {
		t.Fatalf("read: %v got=%q", err, got)
	}
}

func TestLiveRunSandboxLazyAndKill(t *testing.T) {
	c := liveSandboxClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	rs := &opensandbox.RunSandbox{Client: c, RunID: uuid.New(), ProjectID: uuid.New()}
	defer rs.Close(context.WithoutCancel(ctx))

	exit, stdout, _, unknown, err := rs.Exec(ctx, "echo via-run-sandbox", 30000)
	if err != nil || unknown || exit != 0 || !strings.Contains(string(stdout), "via-run-sandbox") {
		t.Fatalf("exec=%v unknown=%v exit=%d out=%q", err, unknown, exit, stdout)
	}
	t.Logf("run-sandbox exec ok: %s", stdout)
}

// TestLiveAgentBashInRealSandbox: the REAL model must drive the bash tool
// through the REAL OpenSandbox — write a CSV, compute row count and the
// amount sum in the sandbox, then quote the numbers in its conclusion.
// The amounts are deliberately awkward (137+289+376+158+943=1903) so the
// answer cannot be produced reliably without executing code.
func TestLiveAgentBashInRealSandbox(t *testing.T) {
	if os.Getenv("JUDEX_LIVE_BASE_URL") == "" {
		t.Skip("JUDEX_LIVE_* 未配置：真实模型 blocked")
	}
	c := liveSandboxClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rs := &opensandbox.RunSandbox{Client: c, RunID: uuid.New(), ProjectID: uuid.New()}
	defer rs.Close(context.WithoutCancel(ctx))

	env := tools.Env{ProjectID: uuid.NewString(), RunID: uuid.NewString(), Sandbox: rs}
	r, name := newLiveRunner(t, env)
	env.Sandbox = rs

	csv := "order,amount\nA1,137\nA2,289\nA3,376\nA4,158\nA5,943\n"
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), SessionID: uuid.New(), ProjectID: uuid.New(),
		ModelName: name,
		Manifest: agentcontext.Build(agentcontext.Facts{
			WorkFacts: []string{"测试数据集 5 笔订单金额如新材料所示"},
			NewMaterial: "【orders.csv】\n" + csv +
				"\n任务：请用 bash 工具把上表原样写入 /tmp/orders.csv（含表头），" +
				"然后用命令统计：1) 除表头外的数据行数；2) amount 列总和。" +
				"最终结论必须引用你计算出的两个数值。",
		}),
		Budget: runner.Budget{MaxRounds: 6, MaxModelAttempts: 12,
			MaxTotalTokens: 200000, MaxWallClock: 4 * time.Minute},
		Env: &env,
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s summary=%s", outcome.State, outcome.Summary)
	}
	if outcome.ToolCalls == 0 {
		t.Fatal("真实沙箱场景必须有工具调用")
	}
	summary := outcome.Summary
	for _, want := range []string{"1903", "5"} {
		if !strings.Contains(summary, want) {
			t.Errorf("结论缺少沙箱计算结果 %q（行数5/总和1903）:\n%s", want, summary)
		}
	}
	// The sandbox must have actually been provisioned (lazy create fired).
	if rs.ExternalID() == "" {
		t.Fatal("bash 调用未触发沙箱创建")
	}
	t.Logf("state=%s toolCalls=%d tokens=%d summary=%s", outcome.State,
		outcome.ToolCalls, outcome.TokensUsed, firstN(summary, 200))
}

// TestLiveScriptedAgentBashInRealSandbox drives the same runner→bash tool→
// REAL OpenSandbox chain with a scripted OpenAI-compatible gateway. This
// verifies the sandbox wiring without a paid model; the GLM variant above
// covers model-driven behaviour when JUDEX_LIVE_* is configured.
func TestLiveScriptedAgentBashInRealSandbox(t *testing.T) {
	c := liveSandboxClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	// Scripted gateway: step 1 writes the CSV via bash, step 2 computes
	// row count + amount sum via bash, step 3 quotes both numbers.
	csv := "order,amount\nA1,137\nA2,289\nA3,376\nA4,158\nA5,943\n"
	writeCmd := "cat > /tmp/orders.csv <<'EOF'\n" + csv + "EOF"
	sumCmd := "tail -n +2 /tmp/orders.csv | cut -d, -f2 | awk '{s+=$1; n++} END {print n, s}'"
	step := 0
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		w.Header().Set("Content-Type", "text/event-stream")
		var payload string
		switch step {
		case 1:
			payload = toolCallChunk("bash", fmt.Sprintf(`{"command":%q}`, writeCmd))
		case 2:
			payload = toolCallChunk("bash", fmt.Sprintf(`{"command":%q}`, sumCmd))
		default:
			payload = "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"统计完成：数据行数=5，amount 总和=1903。\"}}]}\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20,\"total_tokens\":120}}\n" +
				"data: [DONE]\n\n"
		}
		io.WriteString(w, payload)
	}))
	defer gateway.Close()

	provider, err := model.NewProvider(model.GatewayConfig{
		Protocol: model.ProtocolOpenAI, BaseURL: gateway.URL, APIKey: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.New()
	tools.RegisterDefaults(registry)

	rs := &opensandbox.RunSandbox{Client: c, RunID: uuid.New(), ProjectID: uuid.New()}
	defer rs.Close(context.WithoutCancel(ctx))
	env := tools.Env{ProjectID: uuid.NewString(), RunID: uuid.NewString(), Sandbox: rs}

	r := runner.NewRunnerForTest(provider, registry)
	outcome := r.Run(ctx, runner.RunRequest{
		RunID: uuid.New(), SessionID: uuid.New(), ProjectID: uuid.New(),
		ModelName: "scripted", Manifest: agentcontext.Build(agentcontext.Facts{
			NewMaterial: "统计 /tmp/orders.csv 行数与金额总和",
		}),
		Budget: runner.Budget{MaxRounds: 5, MaxModelAttempts: 8,
			MaxTotalTokens: 50000, MaxWallClock: 3 * time.Minute},
		Env: &env,
	})
	if outcome.State != "succeeded" {
		t.Fatalf("state=%s summary=%s", outcome.State, outcome.Summary)
	}
	if outcome.ToolCalls < 2 {
		t.Fatalf("expected >=2 bash calls, got %d", outcome.ToolCalls)
	}
	if rs.ExternalID() == "" {
		t.Fatal("bash 调用未触发真实沙箱创建")
	}
	for _, want := range []string{"1903", "5"} {
		if !strings.Contains(outcome.Summary, want) {
			t.Errorf("结论缺少真实沙箱计算结果 %q:\n%s", want, outcome.Summary)
		}
	}
	t.Logf("scripted run: toolCalls=%d sandbox=%s summary=%s",
		outcome.ToolCalls, rs.ExternalID(), firstN(outcome.Summary, 160))
}

// toolCallChunk renders one SSE turn where the model calls a tool.
func toolCallChunk(name, args string) string {
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":%q,\"arguments\":%q}}]}}]}\n"+
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":50,\"completion_tokens\":10,\"total_tokens\":60}}\n"+
		"data: [DONE]\n\n", name, args)
}
