// A controlled gateway for isolated acceptance namespaces, never production.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
)

func main() {
	var mode atomic.Value
	mode.Store("normal")
	if value := os.Getenv("JUDEX_FIXTURE_MODE"); value != "" {
		mode.Store(value)
	}
	http.HandleFunc("/mode/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		mode.Store(strings.TrimPrefix(r.URL.Path, "/mode/"))
		w.WriteHeader(204)
	})
	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			w.WriteHeader(400)
			return
		}
		m := mode.Load().(string)
		if m == "quota" {
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		emit := func(s string) { fmt.Fprint(w, "data: "+s+"\n\n"); w.(http.Flusher).Flush() }
		if m == "stall" {
			emit(`{"choices":[{"delta":{"content":"等待故障注入"}}]}`)
			<-r.Context().Done()
			return
		}
		if m == "disconnect" {
			emit(`{"choices":[{"delta":{"content":"只有部分结果"}}]}`)
			return
		}
		if m == "publish" {
			written, published := false, false
			source := ""
			for _, message := range req.Messages {
				if message.Role == "tool" {
					written = written || strings.Contains(message.Content, "fixture.html")
					published = published || strings.Contains(message.Content, "materialVersionId")
				}
				if found := regexp.MustCompile(`versionId=([0-9a-f-]{36})`).FindStringSubmatch(message.Content); len(found) > 1 {
					source = found[1]
				}
			}
			name, id := "write", "fixture-write"
			args := map[string]any{"path": "/workspace/fixture.html", "content": "<!doctype html><meta charset=utf-8><h1>中文发布验收</h1><p>保留异议，等待人工决定。</p><script>document.documentElement.dataset.ready='yes'</script>"}
			if written {
				name, id = "publish", "fixture-publish"
				args = map[string]any{"path": "/workspace/fixture.html", "name": "中文分析页面", "kind": "html_bundle", "mime": "text/html", "entrypoint": "index.html", "sourceVersionIds": []string{source}, "idempotencyKey": "fixture-publication"}
			}
			if published {
				emit(`{"choices":[{"delta":{"content":"HTML 已登记为项目材料，尚未验收业务任务。"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":20}}`)
			} else {
				arguments, _ := json.Marshal(args)
				frame, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": id, "function": map[string]any{"name": name, "arguments": string(arguments)}}}}}}})
				emit(string(frame))
				emit(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":100,"completion_tokens":20}}`)
			}
			emit("[DONE]")
			return
		}
		if m == "collaboration" {
			text := func(value string) {
				frame, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": value}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 100, "completion_tokens": 50}})
				emit(string(frame))
				emit("[DONE]")
			}
			var transcript strings.Builder
			for _, message := range req.Messages {
				transcript.WriteString(message.Content)
				transcript.WriteByte('\n')
			}
			all := transcript.String()
			if len(req.Messages) > 0 && !strings.Contains(req.Messages[0].Content, "你是项目协调者") {
				text("岗位已核对当前任务的事实与证据，未执行正式验收。")
				return
			}
			if strings.Contains(all, "\"analysisId\"") {
				text("公开分析与讨论建议已保存，等待人选择讨论位置。")
				return
			}
			name, id := "record_task_analysis", "controlled-task-analysis"
			args := map[string]any{"summary": "当前任务进展已核对，原始记录保持可追溯。", "disagreements": []string{}}
			if !strings.Contains(all, "岗位已核对") {
				match := regexp.MustCompile(`"identityId"\s*:\s*"([0-9a-f-]{36})"`).FindStringSubmatch(all)
				if len(match) > 1 {
					name, id = "call_agent", "controlled-position"
					args = map[string]any{"position": match[1], "question": "按当前职责核对这项任务，保留缺失项", "material": ""}
				}
			} else if strings.Contains(all, "COLLAB_QUESTION") {
				args["discussion"] = map[string]any{"title": "登录问题核对", "reason": "这项问题需要持续协调开发与测试的依据。"}
			}
			arguments, _ := json.Marshal(args)
			frame, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": id, "function": map[string]any{"name": name, "arguments": string(arguments)}}}}}}})
			emit(string(frame))
			emit(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":100,"completion_tokens":50}}`)
			emit("[DONE]")
			return
		}
		hasTool := false
		for _, msg := range req.Messages {
			hasTool = hasTool || msg.Role == "tool"
		}
		if m == "toolstall" && !hasTool {
			emit(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"controlled-shell","function":{"name":"bash","arguments":"{\"command\":\"sleep 300\",\"timeoutMs\":300000}"}}]}}]}`)
			emit(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":100,"completion_tokens":20}}`)
		} else {
			emit(`{"choices":[{"delta":{"content":"受控验收结果：材料仅供分析，正式验收由人决定。"}}]}`)
			emit(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":20}}`)
		}
		emit("[DONE]")
	})
	addr := os.Getenv("JUDEX_FIXTURE_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}
