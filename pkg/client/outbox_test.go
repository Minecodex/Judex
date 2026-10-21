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
	var key, message string
	attempts, effects := 0, 0
	allowRetry := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/system" {
			w.Write([]byte(`{"data":{"protocolVersion":"2"}}`))
			return
		}
		mu.Lock()
		defer mu.Unlock()
		attempts++
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		current := r.Header.Get("Idempotency-Key")
		if key == "" {
			key = current
			message, _ = payload["body"].(string)
			effects++
		} else if current != key || payload["body"] != message {
			effects++
		}
		if !allowRetry {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"original"}}`))
	}))
	defer server.Close()
	directory := t.TempDir()
	first := New(server.URL)
	first.OutboxDirectory = directory
	body := map[string]any{"body": "local selected report"}
	var result map[string]any
	if err := first.Do(context.Background(), "POST", "/groups/g/push", body, &result, NewKey()); err == nil {
		t.Fatal("expected lost response")
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatal("pending command not persisted")
	}
	mu.Lock()
	allowRetry = true
	mu.Unlock()
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

	if effects != 1 || attempts < 2 || result["id"] != "original" || message == "" {
		t.Fatalf("duplicate effect or missing result: effects=%d attempts=%d result=%v", effects, attempts, result)
	}
	entries, _ = os.ReadDir(directory)
	if len(entries) != 0 {
		t.Fatal("acknowledged command not retired")
	}
}
