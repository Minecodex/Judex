// Deterministic OpenAI-compatible model fixture for V2 infrastructure acceptance.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolCalls []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}
type request struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Tools    []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}
type record struct {
	Model, RoleID       string
	SnapshotVersion     int64
	Tools               []string
	PromptHash          string
	ConfigurationMarker bool
}

var mu sync.Mutex
var records = []record{}
var rolePattern = regexp.MustCompile(`role-agent ID: ([a-f0-9-]+)`)
var directoryPattern = regexp.MustCompile(`(?s)Available role agents:\n(\[.*?\])\nHuman members:`)
var humanPattern = regexp.MustCompile(`(?s)Human members:\n(\[.*?\])`)

func main() {
	address := os.Getenv("GATEWAY_ADDR")
	if address == "" {
		address = ":8080"
	}
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	http.HandleFunc("/records", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(records)
	})
	http.HandleFunc("/v1/chat/completions", serve)
	if err := http.ListenAndServe(address, nil); err != nil {
		panic(err)
	}
}
func serve(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if len(req.Messages) == 0 {
		http.Error(w, "missing messages", 400)
		return
	}
	prompt := req.Messages[0].Content
	role := ""
	if match := rolePattern.FindStringSubmatch(prompt); len(match) == 2 {
		role = match[1]
	}
	hash := sha256.Sum256([]byte(prompt))
	names := []string{}
	for _, tool := range req.Tools {
		names = append(names, tool.Function.Name)
	}
	var snapshot struct {
		Version int64 `json:"version"`
		Entries []struct {
			Kind    string `json:"kind"`
			RefID   string `json:"refId"`
			Content struct {
				VersionID   string `json:"versionId"`
				RoleAgentID string `json:"roleAgentId"`
				Body        string `json:"body"`
			} `json:"content"`
		} `json:"entries"`
	}
	if len(req.Messages) > 1 {
		body := req.Messages[1].Content
		if i := strings.Index(body, "\n{"); i >= 0 {
			_ = json.Unmarshal([]byte(body[i+1:]), &snapshot)
		}
	}
	mu.Lock()
	records = append(records, record{Model: req.Model, RoleID: role, SnapshotVersion: snapshot.Version, Tools: names, PromptHash: hex.EncodeToString(hash[:]), ConfigurationMarker: strings.Contains(prompt, "V2_CONFIGURATION_UPDATED")})
	mu.Unlock()
	agentMessages := 0
	confirmed := false
	for _, entry := range snapshot.Entries {
		if entry.Kind == "message" && entry.Content.RoleAgentID != "" {
			agentMessages++
		}
		confirmed = confirmed || strings.Contains(entry.Content.Body, "HUMAN_REPLY_CONFIRMED")
	}
	if strings.Contains(prompt, "REVOCATION_GATE") {
		completed := 0
		for _, msg := range req.Messages {
			if msg.Role == "tool" {
				completed++
			}
		}
		if completed == 2 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(25 * time.Second):
			}
		}
	}
	if strings.Contains(req.Model, "slow") || (strings.Contains(req.Model, "restart-loop") && agentMessages == 4 && req.Messages[len(req.Messages)-1].Role != "tool") {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
	if strings.Contains(req.Model, "failure") {
		http.Error(w, "controlled model failure", 503)
		return
	}
	if strings.Contains(req.Model, "quota") || strings.Contains(req.Model, "rate-limit") {
		code := "insufficient_quota"
		if strings.Contains(req.Model, "rate-limit") {
			code = "rate_limit_exceeded"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, `{"error":{"code":%q,"message":"Controlled model rejection"}}`, code)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	send := func(delta map[string]any, finish any) {
		frame := map[string]any{"id": "controlled-v2", "model": req.Model, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
		raw, _ := json.Marshal(frame)
		fmt.Fprintf(w, "data: %s\n\n", raw)
	}
	text := func(value string) { send(map[string]any{"content": value}, nil); send(map[string]any{}, "stop") }
	tool := func(name string, args any) {
		raw, _ := json.Marshal(args)
		send(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call-v2-%d", time.Now().UnixNano()), "type": "function", "function": map[string]any{"name": name, "arguments": string(raw)}}}}, nil)
		send(map[string]any{}, "tool_calls")
	}
	if strings.Contains(prompt, "You design an advisory collaboration flowchart") {
		if strings.Contains(req.Messages[len(req.Messages)-1].Content, "FLOW_DRAFT_INVALID") {
			text("invalid graph")
		} else {
			text(`{"name":"需求评审与交付","content":"先明确需求，评审通过再实施；有异议则返回补充。","graph":{"nodes":[{"id":"start","name":"明确需求","kind":"start","phase":"准备","responsibility":"产品经理梳理目标"},{"id":"review","name":"评审通过？","kind":"decision","phase":"评审","responsibility":"设计与研发共同评估"},{"id":"build","name":"研发实施","kind":"activity","phase":"实施","responsibility":"研发推进实现"},{"id":"end","name":"发布总结","kind":"end","phase":"总结","responsibility":"相关岗位同步结论"}],"edges":[{"from":"start","to":"review","kind":"sequence","label":"提交"},{"from":"review","to":"build","kind":"sequence","label":"通过"},{"from":"review","to":"start","kind":"feedback","label":"补充"},{"from":"build","to":"end","kind":"sequence","label":"完成"}]}}`)
		}
	} else if len(req.Tools) == 0 {
		text("Public memory: retain unresolved questions and differing views. Read original message/file/tool kind and refId references through read_context before deciding. No agreement or completion is inferred.")
	} else {
		toolCount := 0
		compacted := false
		for _, msg := range req.Messages {
			if strings.Contains(msg.Content, "Execution protocol compacted after all tool results were recorded") {
				compacted = true
			}
		}
		for _, msg := range req.Messages {
			if msg.Role == "tool" {
				toolCount++
			}
		}
		switch {
		case strings.Contains(req.Model, "human"):
			if toolCount > 0 || confirmed {
				text("")
			} else {
				var people []struct {
					ID string `json:"id"`
				}
				if match := humanPattern.FindStringSubmatch(prompt); len(match) == 2 {
					_ = json.Unmarshal([]byte(match[1]), &people)
				}
				mentions := []any{}
				for _, person := range people {
					mentions = append(mentions, map[string]any{"kind": "human", "targetId": person.ID, "requiresResponse": true})
				}
				tool("send_message", map[string]any{"body": "HUMAN_WAIT_QUESTION: 请分别回复这条消息；需要岗位继续时请明确 @AI。", "mentions": mentions})
			}
		case strings.Contains(req.Model, "tools"):
			fileID := ""
			for _, entry := range snapshot.Entries {
				if entry.Kind == "file" {
					fileID = entry.RefID
					break
				}
			}
			switch toolCount {
			case 0:
				tool("read_context", map[string]any{"after": 0, "limit": 10})
			case 1:
				if fileID != "" {
					tool("read", map[string]any{"path": "/workspace/group/versions/" + fileID + "/original", "limit": 4000})
				} else {
					tool("read_context", map[string]any{"after": 0, "limit": 10})
				}
			case 2:
				tool("bash", map[string]any{"command": "set -eu; test ! -e /workspace/project; test ! -e /var/run/secrets/kubernetes.io/serviceaccount/token; ls /workspace/group/versions; printf 'group-boundary-ok'", "timeoutMs": 30000})
			case 3:
				tool("write", map[string]any{"path": "/workspace/report.md", "content": "# Controlled group report\nOriginal evidence reviewed.\n"})
			case 4:
				sum := sha256.Sum256([]byte("# Controlled group report\nOriginal evidence reviewed.\n"))
				tool("edit", map[string]any{"path": "/workspace/report.md", "content": "# Controlled group report\nOriginal evidence reviewed; updated with group context.\n", "expectedSha256": hex.EncodeToString(sum[:])})
			case 5:
				tool("publish_file", map[string]any{"path": "/workspace/report.md", "name": "controlled-group-report.md"})
			case 6:
				tool("send_message", map[string]any{"body": "Controlled seven-tool group evidence published."})
			default:
				text("")
			}
		case strings.Contains(req.Model, "loop"):
			if toolCount > 0 {
				text("")
			} else {
				var directory []struct {
					ID       string `json:"id"`
					Position string `json:"position"`
				}
				if match := directoryPattern.FindStringSubmatch(prompt); len(match) == 2 {
					_ = json.Unmarshal([]byte(match[1]), &directory)
				}
				if strings.Contains(req.Model, "branch-loop") {
					mentions := []any{}
					isStart := false
					for _, peer := range directory {
						if peer.ID == role && peer.Position == "Branch start" {
							isStart = true
						}
					}
					for _, peer := range directory {
						if peer.ID != role && (isStart || peer.Position == "Branch start") {
							mentions = append(mentions, map[string]any{"kind": "role_agent", "targetId": peer.ID})
						}
					}
					tool("send_message", map[string]any{"body": "A controlled concurrent branch.", "mentions": mentions})
					break
				}
				target := ""
				for _, peer := range directory {
					if peer.ID != role {
						target = peer.ID
						break
					}
				}
				if target == "" {
					text("No peer role is available.")
				} else {
					tool("send_message", map[string]any{"body": "A controlled peer perspective.", "mentions": []any{map[string]any{"kind": "role_agent", "targetId": target}}})
				}
			}
		case strings.Contains(req.Model, "budget"):
			tool("read_context", map[string]any{"after": 0, "limit": 1})
		default:
			if toolCount == 0 && !compacted {
				tool("read_context", map[string]any{"after": 0, "limit": 5})
			} else {
				text("Controlled role response based on the shared group context.")
			}
		}
	}
	fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":40}}\n\ndata: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
