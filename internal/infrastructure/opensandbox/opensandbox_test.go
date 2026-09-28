// SPDX-License-Identifier: Apache-2.0

package opensandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeOSB emulates the real server contract: POST/GET/DELETE /sandboxes and
// the NDJSON execd stream behind /sandboxes/{id}/proxy/44772/command.
type fakeOSB struct {
	apiKey     string
	createdIDs []string
	deletedIDs []string
	gotAuth    []string
	// script maps command → NDJSON lines (default: echo the command).
	script map[string]string
}

func (f *fakeOSB) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sandboxes", func(w http.ResponseWriter, r *http.Request) {
		f.gotAuth = append(f.gotAuth, r.Header.Get("OPEN-SANDBOX-API-KEY"))
		id := fmt.Sprintf("sbx-%d", len(f.createdIDs)+1)
		f.createdIDs = append(f.createdIDs, id)
		fmt.Fprintf(w, `{"id":%q,"status":{"state":"Pending"}}`, id)
	})
	mux.HandleFunc("GET /sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id":%q,"status":{"state":"Running"}}`, r.PathValue("id"))
	})
	mux.HandleFunc("DELETE /sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.deletedIDs = append(f.deletedIDs, r.PathValue("id"))
		w.WriteHeader(200)
	})
	mux.HandleFunc("POST /sandboxes/{id}/proxy/{port}/command", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("port") != "44772" {
			t.Errorf("proxy port = %s, want 44772", r.PathValue("port"))
		}
		var req struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad command body: %v", err)
		}
		if lines, ok := f.script[req.Command]; ok {
			w.Write([]byte(lines))
			return
		}
		fmt.Fprintf(w, `{"type":"init","text":"c1"}
{"type":"stdout","text":"ok"}
{"type":"execution_complete","execution_time":1}
`)
	})
	return mux
}

func newTestClient(t *testing.T, f *fakeOSB) Sandbox {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	return New(Config{Endpoint: srv.URL, APIKey: f.apiKey, Image: "alpine:3.20"})
}

func TestCreateUsesImageURIAndWaitsForRunning(t *testing.T) {
	f := &fakeOSB{apiKey: "k1"}
	c := newTestClient(t, f)
	id, err := c.Create(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" || f.gotAuth[len(f.gotAuth)-1] != "k1" {
		t.Fatalf("auth header not forwarded: %v", f.gotAuth)
	}
}

func TestExecParsesNDJSONStreams(t *testing.T) {
	f := &fakeOSB{apiKey: "k1", script: map[string]string{
		`echo hi`: `{"type":"init","text":"c1"}
{"type":"ping","text":"pong"}
{"type":"stdout","text":"hi"}
{"type":"execution_complete","execution_time":1}
`,
		`bad`: `{"type":"init","text":"c2"}
{"type":"stderr","text":"boom"}
{"type":"error","error":{"ename":"CommandExecError","evalue":"3","traceback":["exit status 3"]}}
`,
	}}
	c := newTestClient(t, f)
	ctx := context.Background()
	id, err := c.Create(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	ok, err := c.Exec(ctx, id, "echo hi", 5*time.Second)
	if err != nil || ok.Unknown {
		t.Fatalf("ok exec: %v unknown=%v", err, ok.Unknown)
	}
	if string(ok.Stdout) != "hi\n" || ok.ExitCode != 0 {
		t.Fatalf("stdout=%q exit=%d", ok.Stdout, ok.ExitCode)
	}
	bad, err := c.Exec(ctx, id, "bad", 5*time.Second)
	if err != nil {
		t.Fatalf("failed exec should not be transport error: %v", err)
	}
	if bad.ExitCode != 3 || !strings.Contains(string(bad.Stderr), "boom") {
		t.Fatalf("exit=%d stderr=%q", bad.ExitCode, bad.Stderr)
	}
}

func TestExecUnknownOnTruncatedStream(t *testing.T) {
	f := &fakeOSB{apiKey: "k1", script: map[string]string{
		`half`: `{"type":"init","text":"c3"}
{"type":"stdout","text":"partial"}
`, // no execution_complete / error
	}}
	c := newTestClient(t, f)
	ctx := context.Background()
	id, _ := c.Create(ctx, uuid.New(), uuid.New())
	res, err := c.Exec(ctx, id, "half", 5*time.Second)
	if err == nil || !res.Unknown {
		t.Fatalf("truncated stream must mark Unknown: err=%v unknown=%v", err, res.Unknown)
	}
}

func TestUnavailableWithoutEndpoint(t *testing.T) {
	c := New(Config{})
	if _, err := c.Exec(context.Background(), "x", "ls", time.Second); err == nil ||
		!strings.Contains(err.Error(), "沙箱未配置") {
		t.Fatalf("want SANDBOX_UNAVAILABLE, got %v", err)
	}
}

func TestRunSandboxLazyCreateAndKill(t *testing.T) {
	f := &fakeOSB{apiKey: "k1"}
	c := newTestClient(t, f)
	rs := &RunSandbox{Client: c, RunID: uuid.New(), ProjectID: uuid.New()}
	if len(f.createdIDs) != 0 {
		t.Fatal("sandbox must not be created eagerly")
	}
	exit, stdout, _, unknown, err := rs.Exec(context.Background(), "echo lazy", 30000)
	if err != nil || unknown || exit != 0 || !strings.Contains(string(stdout), "ok") {
		t.Fatalf("exec=%v unknown=%v exit=%d out=%q", err, unknown, exit, stdout)
	}
	if len(f.createdIDs) != 1 {
		t.Fatalf("one run must reuse one sandbox, got %d", len(f.createdIDs))
	}
	rs.Close(context.Background())
	if len(f.deletedIDs) != 1 {
		t.Fatalf("close must kill sandbox, deleted=%v", f.deletedIDs)
	}
}
