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
	"testing"
)

func TestNativeTaskRejectOnlyCreatesFrozenIntent(t *testing.T) {
	root, _ := filepath.Abs("../..")
	directory := t.TempDir()
	binary := filepath.Join(directory, "judex")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/judex")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/acceptance-review") {
			_, _ = w.Write([]byte(`{"data":{"reviewId":"frozen-review","reviewHash":"current-manifest-hash","targetVersion":7}}`))
			return
		}
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/confirmation-intents") {
			t.Errorf("CLI bypassed intent path: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		var body struct {
			Operation, ObjectId, ReviewHash string
			Payload                         struct {
				Decision, Reason, ReviewId string
				ExpectedVersion            int
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Operation != "task.acceptance" || body.ReviewHash != "current-manifest-hash" || body.Payload.Decision != "reject" || body.Payload.Reason != "actual Unicode mismatch" || body.Payload.ReviewId != "frozen-review" || body.Payload.ExpectedVersion != 7 {
			t.Errorf("frozen rejection differs: %+v", body)
		}
		_, _ = w.Write([]byte(`{"data":{"id":"pending-intent","state":"pending","confirmUrl":"http://example.invalid/confirm/pending-intent"}}`))
	}))
	defer server.Close()
	run := func(reason ...string) (string, error) {
		args := []string{"--json", "--no-wait", "--server", server.URL, "--project", "00000000-0000-0000-0000-000000000001", "task", "reject", "00000000-0000-0000-0000-000000000002"}
		args = append(args, reason...)
		c := exec.Command(binary, args...)
		c.Dir = directory
		c.Env = append(os.Environ(), "JUDEX_CONFIG_DIR="+filepath.Join(directory, "config"), "JUDEX_TOKEN=fixture-client-token")
		out, err := c.CombinedOutput()
		return string(out), err
	}
	if out, err := run(); err == nil || !strings.Contains(out, `"ok":false`) {
		t.Fatalf("missing reason accepted: %v %s", err, out)
	}
	if calls != 0 {
		t.Fatal("missing reason made a network request")
	}
	out, err := run("--reason", "actual Unicode mismatch")
	if err != nil || !strings.Contains(out, `"state":"pending"`) || calls != 2 {
		t.Fatalf("intent flow: %v calls=%d %s", err, calls, out)
	}
}
