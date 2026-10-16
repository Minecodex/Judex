// SPDX-License-Identifier: Apache-2.0
package clitest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeCLIProcessErrors(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
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
	run := func(args ...string) (int, string, string) {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir = directory
		command.Env = append(os.Environ(), "JUDEX_CONFIG_DIR="+filepath.Join(directory, "credentials"), "JUDEX_TOKEN=")
		var stdout, stderr strings.Builder
		command.Stdout, command.Stderr = &stdout, &stderr
		code := 0
		if err := command.Run(); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		return code, stdout.String(), stderr.String()
	}
	checkJSON := func(code int, out, stderr string, success bool) {
		t.Helper()
		var result struct {
			SchemaVersion string `json:"schemaVersion"`
			OK            bool   `json:"ok"`
			Error         string `json:"error"`
		}
		decoder := json.NewDecoder(strings.NewReader(out))
		if err := decoder.Decode(&result); err != nil {
			t.Fatalf("JSON: %v stdout=%q stderr=%q exit=%d", err, out, stderr, code)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			t.Fatalf("more than one result: %v %q", err, out)
		}
		if result.SchemaVersion != "1" || result.OK != success || stderr != "" || (code == 0) != success {
			t.Fatalf("result=%+v stderr=%q exit=%d", result, stderr, code)
		}
		if !success && result.Error == "" {
			t.Fatal("failure omitted error")
		}
	}
	for _, args := range [][]string{
		{"--json", "report"}, {"report", "--json"},
		{"--not-a-flag", "--json"}, {"no-such-command", "--json"},
		{"--json=invalid", "report"}, {"--json", "task", "skip"},
		{"context", "00000000-0000-0000-0000-000000000002", "--json"},
		{"auth", "not-a-command", "--json"}, {"project", "not-a-command", "--json"},
		{"decision", "not-a-command", "--json"}, {"agent", "run", "not-a-command", "--json"},
		{"workflow", "get", "bad", "--version", "00000000-0000-0000-0000-000000000002", "--json"},
		{"workflow", "get", "00000000-0000-0000-0000-000000000002", "--version", "bad", "--json"},
		{"workflow", "get", "00000000-0000-0000-0000-000000000000", "--version", "00000000-0000-0000-0000-000000000002", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, out, stderr := run(args...)
			checkJSON(code, out, stderr, false)
			if code != 2 {
				t.Fatalf("parameter error exit=%d want=2", code)
			}
		})
	}
	if code, out, stderr := run("--json", "version"); code != 0 {
		t.Fatalf("version: %d %s %s", code, out, stderr)
	} else {
		checkJSON(code, out, stderr, true)
	}
	for _, args := range [][]string{{"--help"}, {"version"}, {"context"}, {"auth", "--help"}} {
		if code, out, stderr := run(args...); code != 0 || out == "" || stderr != "" {
			t.Fatalf("normal command %v: exit=%d out=%q err=%q", args, code, out, stderr)
		}
	}
	for _, args := range [][]string{{"report", "--json=false"}, {"report", "--file", "--json"}, {"report", "--", "--json"}} {
		if code, out, stderr := run(args...); code == 0 || out != "" || stderr == "" {
			t.Fatalf("text failure %v: exit=%d out=%q err=%q", args, code, out, stderr)
		}
	}
	badFile := filepath.Join(directory, "invalid.json")
	if err := os.WriteFile(badFile, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{badFile, filepath.Join(directory, "missing.json")} {
		code, out, stderr := run("--json", "--project", "00000000-0000-0000-0000-000000000001", "report", "--task", "00000000-0000-0000-0000-000000000002", "--file", file)
		checkJSON(code, out, stderr, false)
	}
	for _, status := range []int{400, 401, 409, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"code":"FIXTURE_ERROR","message":"fixture failure"}}`))
		}))
		code, out, stderr := run("--json", "--server", server.URL, "doctor")
		server.Close()
		checkJSON(code, out, stderr, false)
		want := map[int]int{400: 2, 401: 3, 409: 5, 500: 7}[status]
		if code != want {
			t.Fatalf("HTTP %d exit=%d want=%d", status, code, want)
		}
	}
}
