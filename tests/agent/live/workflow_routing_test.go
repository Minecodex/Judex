package live_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kakj-go/Judex/internal/infrastructure/model"
)

// This measures model discrimination with complete candidate context. It does
// not implement platform routing, mutate assignments or count as its E2E proof.
const routingPrompt = `你是项目的流程关联分析器。输入是本项目真实公开配置、当前计划/任务和本次上报。仅提出适用流程节点建议，不创建工作、不更新流程绑定、不批准、不验收。
以本次上报事实结合当前工作背景判断适用环节。职位可以参与多个已发布流程和多个节点；候选只取该职位 nodeBindings 与已发布流程节点的交集。workflowId 与 nodeId 必须成对识别，不同流程的相同 nodeId 不是同一节点。
信息充分且能排除其他合理解释时返回 match，matches 列出所有明确适用的节点。同一次上报包含多项独立工作时可返回多个对应关系，不能只按首项或首个流程判断。
多个流程同样合理、背景不足、或上报内容与任务既有流程关联明显冲突时返回 clarify，matches 为空，candidates 可列出备选，question 用一个简短问题询问最少必要信息。不得凭职位名称或流程排列顺序猜测，不覆盖任务既有安排。
没有适用候选则返回 out_of_scope，matches 为空，说明原因。空流程背景不是否定上报；当前计划和任务的目标仍是判断依据。
调用 record_workflow_routing 一次保存结构化建议，reason 简洁引用实际内容与配置依据。不要执行任何业务操作。`

type routePair struct {
	WorkflowID string `json:"workflowId"`
	NodeID     string `json:"nodeId"`
}
type routingDecision struct {
	Decision   string      `json:"decision"`
	Matches    []routePair `json:"matches"`
	Candidates []routePair `json:"candidates,omitempty"`
	Reason     string      `json:"reason"`
	Question   string      `json:"question,omitempty"`
}
type routingCase struct {
	ID       string          `json:"id"`
	Texts    []string        `json:"texts"`
	Task     json.RawMessage `json:"task,omitempty"`
	Plan     json.RawMessage `json:"plan,omitempty"`
	Expected routingDecision `json:"expected"`
}
type routingFixture struct {
	SchemaVersion int               `json:"schemaVersion"`
	Project       json.RawMessage   `json:"project"`
	Position      json.RawMessage   `json:"position"`
	Workflows     []json.RawMessage `json:"workflows"`
	Cases         []routingCase     `json:"cases"`
}
type routingTrial struct {
	CaseID        string           `json:"caseId"`
	Repeat        int              `json:"repeat"`
	Input         json.RawMessage  `json:"input"`
	Expected      routingDecision  `json:"expected"`
	Actual        *routingDecision `json:"actual,omitempty"`
	ToolCallID    string           `json:"toolCallId,omitempty"`
	RawArguments  string           `json:"rawArguments,omitempty"`
	ResponseText  string           `json:"responseText,omitempty"`
	InputTokens   int64            `json:"inputTokens"`
	OutputTokens  int64            `json:"outputTokens"`
	UsageKnown    bool             `json:"usageKnown"`
	DurationMS    int64            `json:"durationMs"`
	MaxOutput     int64            `json:"maxOutputTokens"`
	FinishReason  string           `json:"finishReason,omitempty"`
	Passed        bool             `json:"passed"`
	Error         string           `json:"error,omitempty"`
	RubricFailure string           `json:"rubricFailure,omitempty"`
}

func TestLiveWorkflowRouting(t *testing.T) {
	fixturePath := os.Getenv("JUDEX_ROUTING_FIXTURE")
	if fixturePath == "" {
		t.Skip("explicit persisted project fixture required")
	}
	cfg, modelName := liveConfig(t)
	provider, err := model.NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture routingFixture
	if err = json.Unmarshal(raw, &fixture); err != nil || fixture.SchemaVersion != 1 || len(fixture.Workflows) < 3 || len(fixture.Cases) < 10 {
		t.Fatalf("incomplete live fixture: %v", err)
	}
	resultsPath := filepath.Join(filepath.Dir(fixturePath), "model-routing-results.json")
	if custom := os.Getenv("JUDEX_ROUTING_RESULTS"); custom != "" {
		resultsPath = custom
	}
	maxOutput := int64(4096)
	if configured := os.Getenv("JUDEX_ROUTING_MAX_OUTPUT_TOKENS"); configured != "" {
		maxOutput, err = strconv.ParseInt(configured, 10, 64)
		if err != nil || maxOutput < 4096 || maxOutput > 16384 {
			t.Fatal("invalid explicit output token budget")
		}
	}
	selected := map[string]bool{}
	if filter := os.Getenv("JUDEX_ROUTING_CASE_IDS"); filter != "" {
		for _, id := range strings.Split(filter, ",") {
			selected[strings.TrimSpace(id)] = true
		}
	}
	results := struct {
		SchemaVersion int            `json:"schemaVersion"`
		Scope         string         `json:"scope"`
		Model         string         `json:"model"`
		Protocol      string         `json:"protocol"`
		FixtureHash   string         `json:"fixtureSha256"`
		Prompt        string         `json:"prompt"`
		MaxOutput     int64          `json:"maxOutputTokens"`
		SelectedCases []string       `json:"selectedCases,omitempty"`
		Trials        []routingTrial `json:"trials"`
		Passed        int            `json:"passed"`
		Failed        int            `json:"failed"`
	}{SchemaVersion: 1, Scope: "real-model candidate-context experiment; no platform route mutation", Model: modelName,
		Protocol: cfg.Protocol, FixtureHash: fmt.Sprintf("%x", sha256.Sum256(raw)), Prompt: routingPrompt, MaxOutput: maxOutput, Trials: []routingTrial{}}
	for _, sample := range fixture.Cases {
		if len(selected) == 0 || selected[sample.ID] {
			results.SelectedCases = append(results.SelectedCases, sample.ID)
		}
	}
	if len(results.SelectedCases) == 0 {
		t.Fatal("no registered cases match the explicit selection")
	}
	jobs := make(chan routingTrial)
	var workers sync.WaitGroup
	var mu sync.Mutex
	for worker := 0; worker < 2; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for trial := range jobs {
				trial = runRoutingTrial(provider, modelName, trial)
				mu.Lock()
				results.Trials = append(results.Trials, trial)
				if trial.Passed {
					results.Passed++
				} else {
					results.Failed++
				}
				encoded, encodeErr := json.MarshalIndent(results, "", "  ")
				if encodeErr == nil {
					encodeErr = os.WriteFile(resultsPath, encoded, 0600)
				}
				if encodeErr != nil {
					t.Errorf("persist evidence: %v", encodeErr)
				}
				t.Logf("case=%s repeat=%d passed=%t duration=%dms failure=%s error=%s", trial.CaseID, trial.Repeat, trial.Passed, trial.DurationMS, trial.RubricFailure, trial.Error)
				mu.Unlock()
			}
		}()
	}
	for repeat := 0; repeat < 2; repeat++ {
		for _, sample := range fixture.Cases {
			if len(selected) > 0 && !selected[sample.ID] {
				continue
			}
			// Expected labels and case IDs stay outside the model input. Reverse
			// candidate order on repeat, and use an independent natural phrasing.
			flows := append([]json.RawMessage(nil), fixture.Workflows...)
			if repeat == 1 {
				for left, right := 0, len(flows)-1; left < right; left, right = left+1, right-1 {
					flows[left], flows[right] = flows[right], flows[left]
				}
			}
			input, _ := json.Marshal(map[string]any{"project": fixture.Project, "position": fixture.Position, "workflows": flows,
				"task": sample.Task, "plan": sample.Plan, "submission": map[string]string{"text": sample.Texts[repeat%len(sample.Texts)]}})
			jobs <- routingTrial{CaseID: sample.ID, Repeat: repeat + 1, Input: input, Expected: sample.Expected, MaxOutput: maxOutput}
		}
	}
	close(jobs)
	workers.Wait()
	if results.Failed > 0 {
		t.Errorf("live routing discrimination: %d/%d passed; inspect %s", results.Passed, len(results.Trials), resultsPath)
	}
	t.Logf("real-model results: passed=%d failed=%d evidence=%s", results.Passed, results.Failed, resultsPath)
}

func runRoutingTrial(provider model.Provider, name string, trial routingTrial) routingTrial {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	pair := map[string]any{"type": "object", "required": []string{"workflowId", "nodeId"}, "properties": map[string]any{
		"workflowId": map[string]any{"type": "string"}, "nodeId": map[string]any{"type": "string"}}}
	schema := map[string]any{"type": "object", "required": []string{"decision", "matches", "reason"}, "properties": map[string]any{
		"decision": map[string]any{"type": "string", "enum": []string{"match", "clarify", "out_of_scope"}},
		"matches":  map[string]any{"type": "array", "items": pair}, "candidates": map[string]any{"type": "array", "items": pair},
		"reason": map[string]any{"type": "string"}, "question": map[string]any{"type": "string"}}}
	stream, err := provider.Stream(ctx, model.Request{Model: name, MaxOutputTokens: trial.MaxOutput, Messages: []model.Message{
		{Role: "system", Content: routingPrompt}, {Role: "user", Content: string(trial.Input)}},
		Tools: []model.ToolSchema{{Name: "record_workflow_routing", SchemaVersion: 1, Description: "记录只读流程节点判定建议，不执行任何业务变更", InputSchema: schema}}})
	if err != nil {
		trial.Error = err.Error()
		trial.DurationMS = time.Since(started).Milliseconds()
		return trial
	}
	toolCount := 0
	for event := range stream {
		if event.FinishReason != "" {
			trial.FinishReason = event.FinishReason
		}
		switch event.Type {
		case "textDelta":
			trial.ResponseText += event.TextDelta
		case "toolCallReady":
			toolCount++
			trial.ToolCallID, trial.RawArguments = event.ToolCallID, event.ArgsJSON
			if event.ToolName != "record_workflow_routing" {
				trial.Error = "unexpected tool: " + event.ToolName
				continue
			}
			var result routingDecision
			if err = json.Unmarshal([]byte(event.ArgsJSON), &result); err != nil {
				trial.Error = "invalid structured output: " + err.Error()
			} else {
				trial.Actual = &result
			}
		case "usage":
			trial.UsageKnown = true
			trial.InputTokens, trial.OutputTokens = event.InputTokens, event.OutputTokens
		case "error":
			trial.Error = event.Err.Error()
		}
	}
	trial.DurationMS = time.Since(started).Milliseconds()
	if ctx.Err() != nil && trial.Error == "" {
		trial.Error = ctx.Err().Error()
	}
	if trial.Error != "" {
		return trial
	}
	if toolCount != 1 || trial.Actual == nil {
		trial.RubricFailure = fmt.Sprintf("expected exactly one structured decision, got %d", toolCount)
	} else if trial.Actual.Decision != trial.Expected.Decision {
		trial.RubricFailure = fmt.Sprintf("decision=%s expected=%s", trial.Actual.Decision, trial.Expected.Decision)
	} else if !reflect.DeepEqual(routeKeys(trial.Actual.Matches), routeKeys(trial.Expected.Matches)) {
		trial.RubricFailure = "matched workflow/node pairs differ from registered expectation"
	} else if strings.TrimSpace(trial.Actual.Reason) == "" {
		trial.RubricFailure = "missing reasoning basis"
	} else if trial.Actual.Decision == "clarify" && strings.TrimSpace(trial.Actual.Question) == "" {
		trial.RubricFailure = "ambiguous input did not produce a clarification question"
	} else {
		trial.Passed = true
	}
	return trial
}

func routeKeys(pairs []routePair) []string {
	keys := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		keys = append(keys, pair.WorkflowID+"/"+pair.NodeID)
	}
	sort.Strings(keys)
	return keys
}
