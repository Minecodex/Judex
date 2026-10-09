package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func TestOutboxReplaysLostResponseAcrossClients(t *testing.T) {
	var mu sync.Mutex
	var key, submission string
	attempts, effects := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		current := r.Header.Get("Idempotency-Key")
		if key == "" {
			key = current
			submission, _ = payload["clientSubmissionId"].(string)
			effects++
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		if current != key || payload["clientSubmissionId"] != submission {
			effects++
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"original"}}`))
	}))
	defer server.Close()
	directory := t.TempDir()
	first := New(server.URL)
	first.OutboxDirectory = directory
	body := map[string]any{"purpose": "message", "text": "local selected report"}
	var result map[string]any
	if err := first.Do(context.Background(), "POST", "/projects/p/submissions", body, &result, NewKey()); err == nil {
		t.Fatal("expected lost response")
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatal("pending command not persisted")
	}
	second := New(server.URL)
	second.OutboxDirectory = directory
	pending, err := second.PendingCommands()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending list: %v", err)
	}
	resumed, err := second.RetryPending(context.Background(), pending[0]["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	result = resumed.(map[string]any)

	if effects != 1 || attempts != 2 || result["id"] != "original" || submission == "" {
		t.Fatalf("duplicate effect or missing result: effects=%d attempts=%d result=%v", effects, attempts, result)
	}
	entries, _ = os.ReadDir(directory)
	if len(entries) != 0 {
		t.Fatal("acknowledged command not retired")
	}
}
