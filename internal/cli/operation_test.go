package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/pkg/client"
)

func TestCompositeSendRecoveryAndNewIntent(t *testing.T) {
	for _, fault := range []string{"second-create", "second-complete", "final-truncated", "final-envelope"} {
		t.Run(fault, func(t *testing.T) {
			cache := map[string]json.RawMessage{}
			var uploads, versions, messages int
			failed, revoked := false, false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/system" {
					io.WriteString(w, `{"data":{"protocolVersion":"2"}}`)
					return
				}
				if revoked {
					w.WriteHeader(403)
					io.WriteString(w, `{"error":{"code":"FORBIDDEN","message":"access revoked"}}`)
					return
				}
				key := r.Header.Get("Idempotency-Key")
				if _, err := uuid.Parse(key); err != nil {
					t.Errorf("missing stable stage key: %s", r.URL)
				}
				scope := r.Method + r.URL.Path + key
				if raw, ok := cache[scope]; ok {
					w.Write(raw)
					return
				}
				var value any = map[string]any{}
				body, _ := io.ReadAll(r.Body)
				if strings.HasSuffix(r.URL.Path, "/uploads") {
					if !failed && fault == "second-create" && uploads == 1 {
						failed = true
						w.WriteHeader(503)
						io.WriteString(w, `{"error":{"code":"INTERNAL","message":"transient"}}`)
						return
					}
					uploads++
					value = map[string]any{"id": uuid.NewString(), "partSize": 8, "partCount": 1}
				} else if strings.HasSuffix(r.URL.Path, "/complete") {
					versions++
					value = map[string]any{"id": uuid.NewString()}
				} else if strings.HasSuffix(r.URL.Path, "/push") {
					messages++
					var in struct {
						FileVersionIDs []string `json:"fileVersionIds"`
					}
					if err := json.Unmarshal(body, &in); err != nil || len(in.FileVersionIDs) != 2 || in.FileVersionIDs[0] == in.FileVersionIDs[1] {
						t.Errorf("two independent fixed file versions required: %s", body)
					}
					value = map[string]any{"id": uuid.NewString()}
				}
				raw, _ := json.Marshal(map[string]any{"data": value})
				cache[scope] = raw
				if !failed && ((fault == "second-complete" && versions == 2 && strings.HasSuffix(r.URL.Path, "/complete")) ||
					(strings.HasPrefix(fault, "final-") && strings.HasSuffix(r.URL.Path, "/push"))) {
					failed = true
					if fault == "final-envelope" {
						io.WriteString(w, `{"unexpected":"body"}`)
					} else {
						io.WriteString(w, `{"data":`)
					}
					return
				}
				w.Write(raw)
			}))
			defer server.Close()
			directory := t.TempDir()
			paths := []string{filepath.Join(directory, "a.md"), filepath.Join(directory, "b.md")}
			for _, path := range paths {
				if err := os.WriteFile(path, []byte("same"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			send := func(key string) (any, error) {
				// A fresh client simulates a process with no in-memory state.
				c := client.New(server.URL)
				c.OutboxDirectory = filepath.Join(directory, "outbox")
				return sendWithFiles(context.Background(), c, "group", "/groups/group/push",
					map[string]any{"body": "work", "recordOnly": true, "fileVersionIds": []string{}}, paths, key)
			}
			if _, err := send(""); err == nil || !failed {
				t.Fatal("fault was not exercised")
			}
			if _, err := send(""); err != nil {
				t.Fatal(err)
			}
			if uploads != 2 || versions != 2 || messages != 1 {
				t.Fatalf("retry duplicated effects: %d uploads, %d versions, %d messages", uploads, versions, messages)
			}
			if _, err := send(""); err != nil {
				t.Fatal(err)
			}
			if uploads != 4 || messages != 2 {
				t.Fatal("confirmed default operation prevented new intent")
			}
			key := uuid.NewString()
			first, err := send(key)
			if err != nil {
				t.Fatal(err)
			}
			replay, err := send(key)
			if err != nil || fmt.Sprint(first) != fmt.Sprint(replay) || uploads != 6 || messages != 3 {
				t.Fatalf("explicit composite intent changed: %v", err)
			}
			revoked = true
			if _, err = send(key); err == nil {
				t.Fatal("local confirmed receipt bypassed current permissions")
			}
			revoked = false
			if err = os.WriteFile(paths[0], []byte("changed"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = send(key); err == nil {
				t.Fatal("same explicit ID silently accepted changed content")
			}
		})
	}
}
