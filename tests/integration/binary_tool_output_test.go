package integrationtest_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/kakj-go/Judex/internal/agent/tools"
)

type binaryCommandSandbox struct{ raw []byte }

func (s binaryCommandSandbox) Exec(context.Context, string, int) (int, []byte, []byte, bool, error) {
	return 0, s.raw, []byte("normal stderr"), false, nil
}

// Reproduces head ZIP -> NUL -> PostgreSQL 22P05 using the production tools.
func TestBinaryToolOutputRoundTripsThroughJSONB(t *testing.T) {
	_, _, _, pool := newDecisionEnv(t)
	ctx := context.Background()
	registry := tools.New()
	tools.RegisterDefaults(registry)
	for _, raw := range [][]byte{[]byte("PK\x03\x04\x00zip"), {0xff, 0xfe, 0, 0x41}} {
		for _, name := range []string{"read", "bash"} {
			args := map[string]any{"path": "/workspace/content"}
			if name == "bash" {
				args = map[string]any{"command": "head -50 content"}
			}
			result, err := registry.Execute(ctx, name, "agent", args, tools.Env{Sandbox: binaryCommandSandbox{raw}})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result.Data)
			if err != nil {
				t.Fatal(err)
			}
			var stored []byte
			if err = pool.QueryRow(ctx, `SELECT $1::jsonb`, encoded).Scan(&stored); err != nil {
				t.Fatalf("%s binary output cannot be journaled: %v", name, err)
			}
			var restored map[string]any
			if err = json.Unmarshal(stored, &restored); err != nil {
				t.Fatal(err)
			}
			key := "binary"
			if name == "bash" {
				key = "stdoutBinary"
			}
			binary := restored[key].(map[string]any)
			decoded, err := base64.StdEncoding.DecodeString(binary["data"].(string))
			if err != nil || !bytes.Equal(decoded, raw) || binary["encoding"] != "base64" {
				t.Fatal("binary bytes were lost or represented as natural language")
			}
		}
	}
}
