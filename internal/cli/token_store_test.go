package cli

import (
	"context"
	"encoding/json"
	"github.com/kakj-go/Judex/pkg/client"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCredentialRefreshHelper(t *testing.T) {
	if os.Getenv("JUDEX_CREDENTIAL_HELPER") != "1" {
		return
	}
	serverFlag = os.Getenv("JUDEX_CREDENTIAL_SERVER")
	c, err := NewClient()
	if err != nil {
		os.Exit(2)
	}
	var result any
	if err = c.Do(context.Background(), "GET", "/projects", nil, &result, ""); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}
func TestConcurrentCLIProcessesRotateCredentialsOnce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JUDEX_CONFIG_DIR", dir)
	t.Setenv("JUDEX_TOKEN", "")
	var rotations atomic.Int32
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/auth/token/refresh" {
			if rotations.Add(1) != 1 {
				w.WriteHeader(401)
				return
			}
			time.Sleep(50 * time.Millisecond)
			json.NewEncoder(w).Encode(map[string]any{"data": client.TokenPair{AccessToken: "fresh-access", RefreshToken: "fresh-refresh", AccessExpiresAt: time.Now().Add(15 * time.Minute)}})
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh-access" {
			w.WriteHeader(401)
			return
		}
		reads.Add(1)
		w.Write([]byte(`{"data":{"items":[],"nextCursor":""}}`))
	}))
	defer server.Close()
	if err := SaveTokenPair(server.URL, client.TokenPair{AccessToken: "expired-access", RefreshToken: "old-refresh", AccessExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			command := exec.Command(os.Args[0], "-test.run=^TestCredentialRefreshHelper$")
			command.Env = append(os.Environ(), "JUDEX_CREDENTIAL_HELPER=1", "JUDEX_CREDENTIAL_SERVER="+server.URL)
			errors <- command.Run()
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if rotations.Load() != 1 || reads.Load() != 2 {
		t.Fatalf("rotation race: refresh=%d reads=%d", rotations.Load(), reads.Load())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]string
	if json.Unmarshal(raw, &stored) != nil || stored[profileKey()] != "fresh-refresh" {
		t.Fatal("new refresh token not persisted")
	}
}
