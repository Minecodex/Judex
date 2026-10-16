package clitest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNativeAcceptanceHistoryReadsReasonAndPagination(t *testing.T) {
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
	project := "40a9eb39-b405-4a7c-932c-a920d58e8db1"
	object := "9b1cd83d-569e-4f77-92b8-382039dbb2ac"
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer acceptance-history-fixture" || r.URL.Query().Get("limit") != "2" || r.URL.Query().Get("cursor") != "previous-page" {
			t.Error("history changed method, credential or pagination")
			w.WriteHeader(400)
			return
		}
		if r.URL.Path != "/api/v1/projects/"+project+"/tasks/"+object+"/acceptances" && r.URL.Path != "/api/v1/projects/"+project+"/plans/"+object+"/acceptances" {
			t.Error("history escaped the selected project or object")
			w.WriteHeader(404)
			return
		}
		reads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":"frozen-review","decision":"reject","reason":"缺少实际执行日志","taskVersion":4}],"nextCursor":"next-page"},"meta":{}}`))
	}))
	defer server.Close()
	for _, kind := range []string{"task", "plan"} {
		cmd := exec.Command(binary, "--json", "--server", server.URL, "--project", project, kind, "acceptances", object, "--limit", "2", "--cursor", "previous-page")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "JUDEX_CONFIG_DIR="+filepath.Join(dir, "credentials"), "JUDEX_TOKEN=acceptance-history-fixture")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s history: %v %s", kind, err, out)
		}
		var envelope struct {
			OK   bool `json:"ok"`
			Data struct {
				Items      []map[string]any `json:"items"`
				NextCursor string           `json:"nextCursor"`
			} `json:"data"`
		}
		if json.Unmarshal(out, &envelope) != nil || !envelope.OK || len(envelope.Data.Items) != 1 || envelope.Data.Items[0]["reason"] != "缺少实际执行日志" || envelope.Data.NextCursor != "next-page" || !strings.Contains(string(out), "reject") {
			t.Fatalf("formal feedback lost: %s", out)
		}
	}
	if reads.Load() != 2 {
		t.Fatalf("history made unexpected requests: %d", reads.Load())
	}
}
