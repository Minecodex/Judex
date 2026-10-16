package clitest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestNativeHandoffSummaryIsFrozenInIntent(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "judex")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/judex")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	project := "5e304224-a8b9-4112-a565-52fa3bf8c520"
	source := "5f63989a-6caa-45c5-9c01-19a2789c3985"
	task := "30c9549d-e0cb-48ae-a984-6c72ddd94a5a"
	report := "b5ed21c8-ef35-4caf-a4d1-dd5e71d64c3f"
	var calls atomic.Int32
	var expected string
	var sourceRevision *int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer handoff-summary-fixture" {
			t.Error("selected device credential lost")
			w.WriteHeader(401)
			return
		}
		prefix := "/api/v1/projects/" + project
		switch {
		case r.Method == "GET" && r.URL.Path == prefix+"/handoffs":
			if r.URL.Query().Get("limit") != "100" {
				t.Error("paging limit missing")
			}
			if r.URL.Query().Get("cursor") == "" {
				_, _ = w.Write([]byte(`{"data":{"items":[],"nextCursor":"second-page"}}`))
				return
			}
			if r.URL.Query().Get("cursor") != "second-page" {
				t.Error("cursor changed")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": []any{map[string]any{"sources": []any{map[string]any{"id": source, "sourceTaskId": task, "currentVersion": sourceRevision}}}}, "nextCursor": nil}})
		case r.Method == "GET" && r.URL.Path == prefix+"/tasks/"+task:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": task, "latestReportId": report}})
		case r.Method == "POST" && r.URL.Path == prefix+"/confirmation-intents":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			payload, _ := body["payload"].(map[string]any)
			current := float64(0)
			if sourceRevision != nil {
				current = float64(*sourceRevision)
			}
			if body["operation"] != "handoff.send" || body["objectId"] != source || body["reviewHash"] != report || payload["summary"] != expected || payload["sourceVersion"] != current || r.Header.Get("Idempotency-Key") == "" {
				t.Errorf("source summary/report was not frozen in same reviewed intent: %+v", body)
			}
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "11111111-1111-4111-8111-111111111111", "objectId": source, "operation": "handoff.send", "state": "pending", "confirmUrl": "/confirm/11111111-1111-4111-8111-111111111111"}})
		default:
			t.Errorf("unexpected direct mutation or route: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	t.Run("read_list", func(t *testing.T) {
		cmd := exec.Command(binary, "--json", "--server", server.URL, "--project", project, "handoff", "list", "--limit", "100", "--cursor", "second-page")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "JUDEX_CONFIG_DIR="+filepath.Join(dir, "config"), "JUDEX_TOKEN=handoff-summary-fixture")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("source list: %v %s", err, out)
		}
		var result struct {
			OK   bool `json:"ok"`
			Data struct {
				Items []map[string]any `json:"items"`
			} `json:"data"`
		}
		if json.Unmarshal(out, &result) != nil || !result.OK || len(result.Data.Items) != 1 {
			t.Fatalf("source fixed read/paging lost: %s", out)
		}
	})
	for _, test := range []struct {
		name, summary     string
		explicit, success bool
	}{{"default", "由 CLI 提交", false, true}, {"provided", "本轮固定原件\nSHA与来源已核对，尚待接收。", true, true}, {"revision", "更正后的固定来源摘要", true, true}, {"empty", " \n", true, false}} {
		t.Run(test.name, func(t *testing.T) {
			sourceRevision = nil
			if test.name == "revision" {
				v := int64(1)
				sourceRevision = &v
			}
			expected = test.summary
			before := calls.Load()
			args := []string{"--json", "--no-wait", "--server", server.URL, "--project", project, "handoff", "send", source, "--version", "1"}
			if test.explicit {
				args = append(args, "--summary", test.summary)
			}
			cmd := exec.Command(binary, args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "JUDEX_CONFIG_DIR="+filepath.Join(dir, "config"), "JUDEX_TOKEN=handoff-summary-fixture")
			out, err := cmd.CombinedOutput()
			if test.success {
				if err != nil {
					t.Fatalf("send: %v %s", err, out)
				}
				var result struct {
					OK   bool `json:"ok"`
					Data struct {
						State string `json:"state"`
					} `json:"data"`
				}
				if json.Unmarshal(out, &result) != nil || !result.OK || result.Data.State != "pending" || calls.Load()-before != 4 {
					t.Fatalf("CLI did not stop at reviewed intent: %s", out)
				}
			} else if err == nil || calls.Load() != before {
				t.Fatalf("blank summary reached source/API effect: %s", out)
			}
		})
	}
	t.Run("stale_revision_never_prepares_an_intent", func(t *testing.T) {
		v := int64(2)
		sourceRevision = &v
		before := calls.Load()
		cmd := exec.Command(binary, "--json", "--no-wait", "--server", server.URL, "--project", project, "handoff", "send", source, "--version", "1")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "JUDEX_CONFIG_DIR="+filepath.Join(dir, "config"), "JUDEX_TOKEN=handoff-summary-fixture")
		if out, err := cmd.CombinedOutput(); err == nil || calls.Load()-before != 2 {
			t.Fatalf("stale source reached intent mutation: %v %s", err, out)
		}
	})
}
