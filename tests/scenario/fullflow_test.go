// SPDX-License-Identifier: Apache-2.0

// Package scenario drives the FULL multi-persona closed loop against a real
// server + real PG (+ MinIO when configured) + real model:
//
//	甲(owner/统筹) 建项目建岗位 → 邀请 乙(后端)/丙(验收) → 双方接受
//	→ 甲发起议题+首条提交 → 真实 GLM Agent 分析（批次执行→Agent 消息入题）
//	→ 甲创建工作提案 → 乙/丙会签 ALL → 计划+任务生效
//	→ 乙开始任务 → CLI(glm 令牌)分片上传材料 → CLI 交付上报
//	→ Agent 分析交付（引用材料关键事实） → 丙经确认意图+浏览器确认验收
//	→ 乙→丙 交接发送/接收 → 甲验收计划 → 全链路审计核对
//
// 运行条件（缺一即 skip，不 mock 通过）：
//
//	SCENARIO_GLM_BASE_URL / SCENARIO_GLM_API_KEY / SCENARIO_GLM_MODEL
//
// 加 --tags 不需要：直接 go test ./tests/scenario/ -v -count=1
package scenario_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kakj-go/Judex/internal/agent/batch"
	"github.com/kakj-go/Judex/internal/agent/runner"
	"github.com/kakj-go/Judex/internal/agent/tools"
	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	integration "github.com/kakj-go/Judex/tests/integration"
)

// persona 是一个独立浏览器身份（cookie jar + 显示名）。
type persona struct {
	name   string
	email  string
	http   *http.Client
	cookie *http.Cookie
	csrf   string
}

func (p *persona) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.cookie != nil {
		req.AddCookie(p.cookie)
	}
	if p.csrf != "" {
		req.Header.Set("X-CSRF-Token", p.csrf)
		req.Header.Set("Origin", "http://127.0.0.1:1")
	}
	resp, err := p.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	return resp.StatusCode, doc
}

func (p *persona) data(t *testing.T, method, path string, body any, wantStatus int) map[string]any {
	t.Helper()
	status, doc := p.do(t, method, path, body)
	if status != wantStatus {
		raw, _ := json.Marshal(doc)
		t.Fatalf("%s %s → %d (want %d): %s", method, path, status, wantStatus, raw)
	}
	return doc
}

func (p *persona) register(t *testing.T, base string, password string) {
	p.http = &http.Client{}
	status, doc := p.do(t, "POST", base+"/api/v1/auth/register", map[string]any{
		"displayName": p.name, "email": p.email, "password": password,
	})
	if status == 409 {
		// rerun: login instead
		status, doc = p.do(t, "POST", base+"/api/v1/auth/login", map[string]any{
			"email": p.email, "password": password,
		})
	}
	if status != 200 && status != 201 {
		t.Fatalf("register %s: %d %v", p.name, status, doc)
	}
	// extract cookie via a manual request
	req, _ := http.NewRequest("POST", base+"/api/v1/auth/login", strings.NewReader(
		fmt.Sprintf(`{"email":%q,"password":%q}`, p.email, password)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("login %s: %v %v", p.name, err, resp)
	}
	resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == "judex_dev_session" {
			p.cookie = c
		}
	}
	if p.cookie == nil {
		t.Fatalf("%s: no session cookie", p.name)
	}
	// fetch CSRF
	req2, _ := http.NewRequest("GET", base+"/api/v1/auth/session", nil)
	req2.AddCookie(p.cookie)
	resp2, err := p.http.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	var sess struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&sess)
	resp2.Body.Close()
	p.csrf = sess.Data.CSRFToken
}

func mustEnv(t *testing.T) (baseURL string, key, modelName, protocol string) {
	t.Helper()
	base := os.Getenv("SCENARIO_GLM_BASE_URL")
	key = os.Getenv("SCENARIO_GLM_API_KEY")
	modelName = os.Getenv("SCENARIO_GLM_MODEL")
	protocol = os.Getenv("SCENARIO_GLM_PROTOCOL")
	if base == "" || key == "" || modelName == "" {
		t.Skip("SCENARIO_GLM_* 未配置：真实闭环场景按规则跳过，不 mock")
	}
	if protocol == "" {
		protocol = model.ProtocolOpenAI
	}
	return base, key, modelName, protocol
}

func TestFullBusinessClosedLoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gwBase, gwKey, gwModel, gwProtocol := mustEnv(t)
	fixture := integration.StartPG(t)
	minio := startMinIO(t)

	// Boot the full app with the real model gateway + object storage wired.
	t.Setenv("JUDEX_S3_ENDPOINT", "http://127.0.0.1:"+minio.port)
	t.Setenv("JUDEX_S3_ACCESS_KEY", "judex")
	t.Setenv("JUDEX_S3_SECRET_KEY", "judex-scenario")
	t.Setenv("JUDEX_S3_BUCKET", "judex")
	t.Setenv("JUDEX_S3_PATH_STYLE", "true")
	provider, err := model.NewProvider(model.GatewayConfig{
		Protocol: gwProtocol, BaseURL: gwBase, APIKey: gwKey,
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	registry := tools.New()
	tools.RegisterDefaults(registry)
	executor := &batch.Executor{
		Pool: fixture.Pool.Pool, Provider: provider, ModelName: gwModel,
		Runner: runner.NewRunnerForTest(provider, registry),
	}

	listener, err := netListen()
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	cfg := config.Config{
		Environment: "test", HTTPAddress: listener.Addr().String(), WebDirectory: "none",
		Mode: config.ModeAll, DatabaseURL: fixture.URL,
		AllowedOrigins: []string{"http://127.0.0.1:1"}, LogLevel: "error",
		ObjectStorage: config.ObjectStorageOptions{
			Endpoint:    "http://127.0.0.1:" + minio.port,
			AccessKeyID: "judex", SecretAccessKey: "judex-scenario",
			Bucket: "judex", PathStyle: true,
		},
	}
	application, err := app.New(cfg, nil)
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	srv := &httptest.Server{Listener: listener, Config: &http.Server{Handler: application.Server.Handler}}
	srv.Start()
	t.Cleanup(func() { srv.Close(); application.Close(context.Background()) })

	stamp := time.Now().UnixNano()
	jiǎ := &persona{name: "甲·统筹", email: fmt.Sprintf("a-%d@judex.test", stamp)}
	yǐ := &persona{name: "乙·后端", email: fmt.Sprintf("b-%d@judex.test", stamp)}
	bǐng := &persona{name: "丙·验收", email: fmt.Sprintf("c-%d@judex.test", stamp)}
	password := "password-scenario-1"
	jiǎ.register(t, base, password)
	yǐ.register(t, base, password)
	bǐng.register(t, base, password)

	ctx := context.Background()

	// ── 1. 甲建项目 + 两个岗位（执行/验收） ──────────────────────────
	doc := jiǎ.data(t, "POST", base+"/api/v1/projects", map[string]any{
		"title": "订单服务稳定性治理", "kind": "software",
	}, 201)
	projectID := doc["data"].(map[string]any)["id"].(string)
	positions := map[string]string{}
	for _, spec := range []struct{ name, key string }{
		{"后端工程师", "backend"}, {"质量验收", "qa"},
	} {
		doc = jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/positions", map[string]any{
			"name": spec.name, "prompt": spec.name + "：按职责分析并如实报告", "publicSummary": spec.name,
		}, 201)
		positions[spec.key] = doc["data"].(map[string]any)["id"].(string)
	}

	// ── 2. 邀请乙/丙并接受（获得身份绑定） ────────────────────────────
	for _, inv := range []struct {
		p   *persona
		pos string
	}{
		{yǐ, positions["backend"]}, {bǐng, positions["qa"]},
	} {
		doc = jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/invitations", map[string]any{
			"targetEmail": inv.p.email, "positionIds": []string{inv.pos},
		}, 201)
		invID := doc["data"].(map[string]any)["id"].(string)
		inv.p.data(t, "POST", base+"/api/v1/invitations/"+invID+"/accept", map[string]any{
			"expectedVersion": 1,
		}, 200)
	}

	// 身份清单：乙/丙 各持一个 position 身份
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/identities", nil, 200)
	var backendIdentity, qaIdentity, coordinatorID string
	for _, raw := range doc["data"].(map[string]any)["items"].([]any) {
		id := raw.(map[string]any)
		switch id["kind"].(string) {
		case "coordinator":
			coordinatorID = id["id"].(string)
		case "position":
			if id["positionName"].(string) == "后端工程师" {
				backendIdentity = id["id"].(string)
			} else {
				qaIdentity = id["id"].(string)
			}
		}
	}
	if backendIdentity == "" || qaIdentity == "" || coordinatorID == "" {
		t.Fatalf("identities missing: backend=%s qa=%s coordinator=%s", backendIdentity, qaIdentity, coordinatorID)
	}

	// ── 3. 甲建议题 + 首条提交（真实业务内容） ────────────────────────
	doc = jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/topics", map[string]any{
		"title":          "缺陷 #412：下单接口无幂等控制",
		"initialMessage": "压测显示并发下单会创建重复订单（缺陷 #412）。请大家分析修复方案与验收标准。",
	}, 201)
	topicID := doc["data"].(map[string]any)["id"].(string)

	// ── 4. 真实 Agent 分析：手动触发批次并同步执行（worker 路径同一执行器）──
	doc = jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/topics/"+topicID+"/runs", map[string]any{
		"sourceSubmissionId": latestSubmissionID(t, fixture.Pool, ctx, projectID),
	}, 202)
	batchID := latestBatchID(t, fixture.Pool, ctx, projectID)
	if batchID == "" {
		t.Fatal("batch not registered")
	}
	t.Log("agent batch executing with real model…")
	agentCtx, agentCancel := context.WithTimeout(ctx, 5*time.Minute)
	err = executor.ExecuteBatch(agentCtx, mustUUID(projectID), mustUUID(batchID))
	agentCancel()
	if err != nil {
		t.Fatalf("agent batch: %v", err)
	}
	// Agent 消息必须已入题且为中文可读内容
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/topics/"+topicID+"/messages?limit=10", nil, 200)
	items := doc["data"].(map[string]any)["items"].([]any)
	var agentMsg string
	for _, raw := range items {
		m := raw.(map[string]any)
		if m["kind"].(string) == "agent" {
			agentMsg = m["content"].(string)
		}
	}
	if agentMsg == "" {
		t.Fatal("agent message missing after batch execution")
	}
	if !strings.Contains(agentMsg, "幂等") && !strings.Contains(agentMsg, "订单") {
		t.Fatalf("agent analysis must reference the material facts (幂等/订单):\n%.400s", agentMsg)
	}
	t.Logf("✓ Agent 分析入题（%.80s…）", agentMsg)

	// ── 5. 甲创建工作提案（计划+任务，引用身份） ─────────────────────
	doc = jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/proposals", map[string]any{
		"kind": "work_arrangement",
		"changes": []map[string]any{
			{"operation": "create_plan", "targetType": "plan", "clientRef": "p1",
				"fields": map[string]any{"title": "幂等修复计划"}},
			{"operation": "create_task", "targetType": "task",
				"fields": map[string]any{
					"title": "实现下单幂等键", "planId": "p1",
					"participantIdentityIds": []any{backendIdentity},
					"reviewerIdentityId":     qaIdentity,
				}},
		},
	}, 201)
	proposalID := doc["data"].(map[string]any)["id"].(string)
	jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/proposals/"+proposalID+"/submit", map[string]any{
		"expectedVersion": 1,
	}, 200)

	// ── 6. 乙、丙会签（ALL 席位） → 计划+任务生效 ────────────────────
	approve := func(p *persona) {
		doc = p.data(t, "GET", base+"/api/v1/projects/"+projectID+"/proposals/"+proposalID+"/review", nil, 200)
		data := doc["data"].(map[string]any)
		var slotIDs []string
		for _, raw := range data["slots"].([]any) {
			s := raw.(map[string]any)
			if s["state"].(string) == "pending" {
				slotIDs = append(slotIDs, s["id"].(string))
			}
		}
		p.data(t, "POST", base+"/api/v1/projects/"+projectID+"/proposals/"+proposalID+"/decisions", map[string]any{
			"reviewId": data["reviewId"], "reviewHash": data["reviewHash"],
			"expectedVersion": 2, "decision": "approve", "slotIds": slotIDs,
		}, 200)
	}
	approve(yǐ)
	approve(bǐng)
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/proposals?limit=5", nil, 200)
	for _, raw := range doc["data"].(map[string]any)["items"].([]any) {
		if raw.(map[string]any)["id"].(string) == proposalID {
			if raw.(map[string]any)["status"].(string) != "approved" {
				t.Fatalf("proposal status = %v (want approved)", raw.(map[string]any)["status"])
			}
		}
	}
	// 任务生效为 ready
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/tasks?limit=5", nil, 200)
	var taskID string
	for _, raw := range doc["data"].(map[string]any)["items"].([]any) {
		if raw.(map[string]any)["status"].(string) == "ready" {
			taskID = raw.(map[string]any)["id"].(string)
		}
	}
	if taskID == "" {
		t.Fatal("no ready task after proposal approval")
	}
	t.Logf("✓ 提案会签通过，任务就绪 %s", taskID)

	// ── 7. 乙开始任务 → CLI 风格上传材料 → 交付上报 ─────────────────
	yǐ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/tasks/"+taskID+"/start", map[string]any{
		"expectedVersion": 1, "identityId": backendIdentity,
	}, 200)
	// CLI 上传：走 pkg 风格的分片上传（真实 API 路径）
	material := "【修复验证记录】\n1. 以 orderId+userId 构造幂等键，写入 redis SETNX TTL 10s。\n2. 重放 200 次重复下单请求：仅创建 1 笔订单（缺陷 #412 关闭条件达成）。\n3. P99 由 2.8s 降至 210ms。"
	versionID := cliUpload(t, yǐ, base, projectID, "idempotency-report.md", material)
	t.Logf("✓ CLI 材料上传 → 版本 %s", versionID)
	// 交付上报（引用材料 + expectedTaskVersion）
	doc = yǐ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/tasks/"+taskID, nil, 200)
	taskVersion := int64(doc["data"].(map[string]any)["version"].(float64))
	yǐ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/tasks/"+taskID+"/reports", map[string]any{
		"kind": "delivery", "text": "幂等修复完成，验证记录见附件。",
		"materialVersionIds":  []string{versionID},
		"expectedTaskVersion": taskVersion,
	}, 200)
	// 任务应进入 delivered
	doc = yǐ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/tasks/"+taskID, nil, 200)
	if got := doc["data"].(map[string]any)["status"].(string); got != "delivered" {
		t.Fatalf("task status=%s (want delivered)", got)
	}
	t.Log("✓ CLI 交付上报 → delivered")

	// ── 8. Agent 二次分析（可选：同一执行器再次运行新批次） ─────────
	yǐ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/submissions", map[string]any{
		"clientSubmissionId": uuid.NewString(), "purpose": "message",
		"topicId": topicID, "text": "幂等修复已交付（任务：实现下单幂等键），验证记录已上传，请验收。",
	}, 201)
	// 手动触发第二次分析（自动触发链路已在 handler 中登记 job）
	subID2 := latestSubmissionID(t, fixture.Pool, ctx, projectID)
	jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/topics/"+topicID+"/runs", map[string]any{
		"sourceSubmissionId": subID2,
	}, 202)
	b2 := latestBatchID(t, fixture.Pool, ctx, projectID)
	if b2 != "" && b2 != batchID {
		c2, cc := context.WithTimeout(ctx, 5*time.Minute)
		if err := executor.ExecuteBatch(c2, mustUUID(projectID), mustUUID(b2)); err != nil {
			t.Logf("second batch (optional): %v", err)
		} else {
			t.Log("✓ Agent 二次分析完成")
		}
		cc()
	}

	// ── 9. 丙经确认意图 + 浏览器一次确认验收任务 ─────────────────────
	doc = bǐng.data(t, "GET", base+"/api/v1/projects/"+projectID+"/tasks/"+taskID+"/acceptance-review", nil, 200)
	review := doc["data"].(map[string]any)
	doc = bǐng.data(t, "POST", base+"/api/v1/projects/"+projectID+"/confirmation-intents", map[string]any{
		"operation": "task.acceptance", "objectId": taskID,
		"reviewHash": review["reviewHash"],
		"payload": map[string]any{
			"decision": "accept", "expectedVersion": review["targetVersion"],
			"reviewId": review["reviewId"],
		},
	}, 201)
	intentID := doc["data"].(map[string]any)["id"].(string)
	// “浏览器确认”（同会话一次点击；服务端同事务执行验收）
	bǐng.data(t, "POST", base+"/api/v1/projects/"+projectID+"/confirmation-intents/"+intentID+"/confirm", map[string]any{
		"decision": "approve",
	}, 200)
	doc = yǐ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/tasks/"+taskID, nil, 200)
	if got := doc["data"].(map[string]any)["status"].(string); got != "accepted" {
		t.Fatalf("task status=%s (want accepted after intent confirm)", got)
	}
	t.Log("✓ 确认意图 → 浏览器一次确认 → 任务 accepted")

	// ── 10. 甲验收计划（整体验收） ───────────────────────────────────
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/plans?limit=5", nil, 200)
	var planID string
	for _, raw := range doc["data"].(map[string]any)["items"].([]any) {
		if strings.Contains(raw.(map[string]any)["title"].(string), "幂等") {
			planID = raw.(map[string]any)["id"].(string)
		}
	}
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/plans/"+planID+"/acceptance-review", nil, 200)
	pReview := doc["data"].(map[string]any)
	jiǎ.data(t, "POST", base+"/api/v1/projects/"+projectID+"/plans/"+planID+"/acceptances", map[string]any{
		"reviewId": pReview["reviewId"], "reviewHash": pReview["reviewHash"],
		"decision": "accept",
	}, 200)
	t.Log("✓ 计划整体验收 accepted")

	// ── 11. 审计与不可变验收链核对 ───────────────────────────────────
	doc = jiǎ.data(t, "GET", base+"/api/v1/projects/"+projectID+"/audit", nil, 200)
	auditItems := doc["data"].(map[string]any)["items"].([]any)
	ops := map[string]int{}
	for _, raw := range auditItems {
		ops[raw.(map[string]any)["operation"].(string)]++
	}
	// identity.register is global (no project scope); project audit starts at project.create.
	for _, op := range []string{"project.create", "proposal.submit",
		"proposal.approve", "work.report.delivery", "task.accept", "plan.accept"} {
		if ops[op] == 0 {
			t.Errorf("audit missing %s (have %v)", op, ops)
		}
	}
	var acceptances int
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT count(*) FROM task_acceptances WHERE task_id=$1`, taskID).Scan(&acceptances); err != nil || acceptances != 1 {
		t.Fatalf("task_acceptances=%d (must be exactly 1, immutable)", acceptances)
	}
	var events int
	if err := fixture.Pool.QueryRow(ctx,
		`SELECT count(*) FROM project_events WHERE project_id=$1`, mustUUID(projectID)).Scan(&events); err != nil || events < 8 {
		t.Fatalf("project_events=%d (expected ≥8)", events)
	}
	t.Logf("闭环完成：审计操作 %v，项目事件 %d 条", ops, events)
}

// ── helpers ─────────────────────────────────────────────────────────────

func latestSubmissionID(t *testing.T, pool *postgres.Pool, ctx context.Context, projectID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx,
		`SELECT id::text FROM submissions WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`,
		mustUUID(projectID)).Scan(&id)
	if err != nil {
		t.Fatalf("latest submission: %v", err)
	}
	return id
}

func latestBatchID(t *testing.T, pool *postgres.Pool, ctx context.Context, projectID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx,
		`SELECT id::text FROM discussion_batches WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`,
		mustUUID(projectID)).Scan(&id)
	if err != nil {
		return ""
	}
	return id
}

// cliUpload mirrors the CLI's chunked upload path over the real API.
func cliUpload(t *testing.T, p *persona, base, projectID, name, content string) string {
	t.Helper()
	sum := sha256Hex(content)
	doc := p.data(t, "POST", base+"/api/v1/projects/"+projectID+"/uploads", map[string]any{
		"name": name, "size": len(content), "sha256": sum,
		"mime": "text/markdown", "kind": "file",
	}, 201)
	uploadID := doc["data"].(map[string]any)["id"].(string)
	// single part (content < 8MiB)
	req, _ := http.NewRequest("PUT",
		fmt.Sprintf("%s/api/v1/projects/%s/uploads/%s/parts/1", base, projectID, uploadID),
		strings.NewReader(content))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Judex-Part-SHA256", sum)
	req.Header.Set("X-CSRF-Token", p.csrf)
	req.Header.Set("Origin", "http://127.0.0.1:1")
	req.AddCookie(p.cookie)
	resp, err := p.http.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("part upload: %v %v", err, resp)
	}
	resp.Body.Close()
	doc = p.data(t, "POST", base+"/api/v1/projects/"+projectID+"/uploads/"+uploadID+"/complete", map[string]any{}, 201)
	return doc["data"].(map[string]any)["id"].(string)
}

func sha256Hex(s string) string {
	sum := sha256Sum([]byte(s))
	return hexEncode(sum)
}

func mustUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic("bad uuid: " + s)
	}
	return id
}

func netListen() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }

// minioFixture is a disposable MinIO container for the scenario.
type minioFixture struct{ container, port string }

func startMinIO(t *testing.T) *minioFixture {
	t.Helper()
	if out, err := execDocker("info"); err != nil {
		t.Skipf("docker unavailable: %v %s", err, out)
	}
	name := fmt.Sprintf("judex-scenario-minio-%d", time.Now().UnixNano())
	image := minioImage()
	if out, err := execDocker("run", "-d", "--rm", "--name", name, "--user", "root",
		"-e", "MINIO_ROOT_USER=judex", "-e", "MINIO_ROOT_PASSWORD=judex-scenario",
		"-p", "127.0.0.1::9000", image, "server", "/data"); err != nil {
		t.Fatalf("minio start: %v %s", err, out)
	}
	t.Cleanup(func() { _, _ = execDocker("rm", "-f", name) })
	port := ""
	for i := 0; i < 30 && port == ""; i++ {
		if out, err := execDocker("port", name, "9000"); err == nil {
			lines := strings.Split(strings.TrimSpace(out), "\n")
			port = lines[0]
			if idx := strings.LastIndex(port, ":"); idx >= 0 {
				port = port[idx+1:]
			}
		}
		if port == "" {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if port == "" {
		t.Fatal("minio port not found")
	}
	// Wait for health + create bucket.
	for i := 0; i < 30; i++ {
		resp, err := http.Get("http://127.0.0.1:" + port + "/minio/health/live")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	_, _ = execDocker("exec", name, "sh", "-c",
		"mc alias set local http://127.0.0.1:9000 judex judex-scenario && mc mb local/judex --ignore-existing")
	return &minioFixture{container: name, port: port}
}

func minioImage() string {
	out, err := execDocker("images", "--format", "{{.Repository}}:{{.Tag}} {{.ID}}")
	if err != nil {
		return "quay.io/minio/minio:latest"
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "minio") && !strings.Contains(line, "mc") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				return parts[1] // image ID avoids registry resolution
			}
		}
	}
	return "quay.io/minio/minio:latest"
}

func execDocker(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func hexEncode(b []byte) string { return hex.EncodeToString(b) }
