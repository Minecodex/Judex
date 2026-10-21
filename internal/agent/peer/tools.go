package peer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/collab"
	"github.com/kakj-go/Judex/internal/infrastructure/model"
	"github.com/kakj-go/Judex/internal/infrastructure/opensandbox"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

type sandboxGetter func(context.Context) (*opensandbox.RunSandbox, error)

func object(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}
func schemas() []model.ToolSchema {
	str := map[string]any{"type": "string"}
	number := map[string]any{"type": "integer", "minimum": 0}
	id := map[string]any{"type": "string", "format": "uuid"}
	mention := object([]string{"kind", "targetId"}, map[string]any{"kind": map[string]any{"type": "string", "enum": []string{"human", "role_agent"}}, "targetId": id, "requiresResponse": map[string]any{"type": "boolean"}})
	return []model.ToolSchema{
		{Name: "read", SchemaVersion: 1, Description: "Read a sandbox text file in pages, including approved /workspace/group files.", InputSchema: object([]string{"path"}, map[string]any{"path": str, "offset": number, "limit": number})},
		{Name: "bash", SchemaVersion: 1, Description: "Run a shell command in this isolated sandbox. Results are public to the group. Unknown effects require human reconciliation, never blind replay.", InputSchema: object([]string{"command"}, map[string]any{"command": str, "timeoutMs": map[string]any{"type": "integer", "minimum": 1, "maximum": 120000}})},
		{Name: "write", SchemaVersion: 1, Description: "Write a working file under /workspace, excluding the immutable /workspace/group projection.", InputSchema: object([]string{"path", "content"}, map[string]any{"path": str, "content": map[string]any{"type": "string", "maxLength": 524288}})},
		{Name: "edit", SchemaVersion: 1, Description: "Replace a working file only when its SHA256 matches expectedSha256.", InputSchema: object([]string{"path", "content", "expectedSha256"}, map[string]any{"path": str, "content": map[string]any{"type": "string", "maxLength": 524288}, "expectedSha256": map[string]any{"type": "string", "pattern": "^[a-f0-9]{64}$"}})},
		{Name: "read_context", SchemaVersion: 1, Description: "Read this group's original public references after compression. List up to 20 bounded reference index entries with after; page complete originals with exact kind/refId.", InputSchema: object([]string{}, map[string]any{"kind": str, "refId": str, "after": number, "offset": number, "limit": number})},
		{Name: "send_message", SchemaVersion: 2, Description: "Speak publicly to this group. Structured mentions alone route to peers or notify humans. To ask for a human decision, set requiresResponse=true on each human mention and optionally supply confirmation.title; the body and immutable files form the confirmation card. Never self mention.", InputSchema: object([]string{"body"}, map[string]any{"body": map[string]any{"type": "string", "maxLength": 65536}, "replyTo": id, "mentions": map[string]any{"type": "array", "items": mention, "maxItems": 100}, "confirmation": object([]string{"title"}, map[string]any{"title": map[string]any{"type": "string", "maxLength": 240}}), "fileVersionIds": map[string]any{"type": "array", "items": id, "maxItems": 20}})},
		{Name: "publish_file", SchemaVersion: 1, Description: "Publish a regular sandbox working file (up to 8 MiB) into this group. Set fileId and expectedVersion only for an explicit update to an owned file.", InputSchema: object([]string{"path", "name"}, map[string]any{"path": str, "name": str, "fileId": id, "expectedVersion": number})},
	}
}
func validateCall(call model.ToolCall) (map[string]any, error) {
	var args map[string]any
	decoder := json.NewDecoder(strings.NewReader(call.Arguments))
	if e := decoder.Decode(&args); e != nil {
		return nil, e
	}
	for _, tool := range schemas() {
		if tool.Name == call.Name {
			raw, _ := json.Marshal(tool.InputSchema)
			var schema openapi3.Schema
			if e := json.Unmarshal(raw, &schema); e != nil {
				return nil, e
			}
			return args, schema.VisitJSON(args)
		}
	}
	return nil, fmt.Errorf("unknown tool %q", call.Name)
}
func (e *Executor) executeTool(ctx context.Context, r *run, call model.ToolCall, get sandboxGetter) error {
	args, err := validateCall(call)
	if err != nil {
		return e.toolResponse(ctx, r, call, map[string]any{"error": "Invalid tool arguments; use the declared schema"})
	}
	var state string
	var replayAuthorized bool
	var result json.RawMessage
	err = e.Service.Pool.QueryRow(ctx, `SELECT state,result,replay_authorized FROM role_tool_calls WHERE run_id=$1 AND call_id=$2`, r.Publication.RunID, call.ID).Scan(&state, &result, &replayAuthorized)
	if err == nil && state == "completed" {
		return e.appendToolResponse(ctx, r, call, result)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if (state == "started" || state == "unknown") && !replayAuthorized {
		if call.Name == "bash" {
			return fmt.Errorf("unknown shell effect requires human reconciliation")
		}
		if call.Name == "write" || call.Name == "edit" {
			sandbox, e2 := get(ctx)
			if e2 != nil {
				return e2
			}
			file := stringArg(args, "path")
			if !workingPath(file) {
				return fmt.Errorf("invalid working path")
			}
			exit, out, _, unknown, e2 := sandbox.Exec(ctx, "sha256sum "+quote(file)+" 2>/dev/null | cut -d ' ' -f 1", 30000)
			if e2 != nil || unknown {
				return fmt.Errorf("cannot reconcile previous write")
			}
			sum := sha256.Sum256([]byte(stringArg(args, "content")))
			if exit == 0 && strings.TrimSpace(string(out)) == hex.EncodeToString(sum[:]) {
				return e.toolResponse(ctx, r, call, map[string]any{"path": file, "sha256": hex.EncodeToString(sum[:]), "reconciled": true})
			}
			if call.Name != "edit" || strings.TrimSpace(string(out)) != stringArg(args, "expectedSha256") {
				return fmt.Errorf("previous file write needs human reconciliation")
			}
		}
	}
	rawArgs, _ := json.Marshal(args)
	if _, err = e.Service.Pool.Exec(ctx, `INSERT INTO role_tool_calls(run_id,call_id,name,args,state) VALUES($1,$2,$3,$4,'started') ON CONFLICT(run_id,call_id) DO NOTHING`, r.Publication.RunID, call.ID, call.Name, rawArgs); err != nil {
		return err
	}
	if _, err = e.Service.Pool.Exec(ctx, `UPDATE role_tool_calls SET replay_authorized=false WHERE run_id=$1 AND call_id=$2`, r.Publication.RunID, call.ID); err != nil {
		return err
	}
	if ok, e2 := e.live(ctx, r); e2 != nil {
		return e2
	} else if !ok {
		return fmt.Errorf("run is no longer active")
	}
	// Database effects and their journal completion share a transaction. A crashed
	// started send/publish can therefore be retried without duplicate publication.
	if call.Name == "send_message" || call.Name == "publish_file" {
		var output any
		err = e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
			if call.Name == "send_message" {
				var in collab.MessageInput
				encoded, _ := json.Marshal(args)
				if e2 := json.Unmarshal(encoded, &in); e2 != nil {
					return e2
				}
				id, e2 := e.Service.Publish(ctx, r.Publication, in)
				if e2 != nil {
					return e2
				}
				output = map[string]any{"messageId": id}
			} else {
				sandbox, e2 := get(ctx)
				if e2 != nil {
					return e2
				}
				file := stringArg(args, "path")
				if !workingPath(file) {
					return fmt.Errorf("publish path must be a working file")
				}
				exit, out, _, unknown, e2 := sandbox.Exec(ctx, "set -eu; test -f "+quote(file)+"; test ! -L "+quote(file)+"; test $(wc -c < "+quote(file)+") -le 8388608; base64 "+quote(file), 30000)
				if e2 != nil || unknown || exit != 0 {
					return fmt.Errorf("artifact cannot be read")
				}
				data, e2 := base64.StdEncoding.DecodeString(string(out))
				if e2 != nil || len(data) > 8<<20 {
					return fmt.Errorf("artifact truncated or too large")
				}
				var fileID *uuid.UUID
				if v := stringArg(args, "fileId"); v != "" {
					parsed, e2 := uuid.Parse(v)
					if e2 != nil {
						return e2
					}
					fileID = &parsed
				}
				version, e2 := e.Service.PublishArtifact(ctx, r.Publication, call.ID, stringArg(args, "name"), data, fileID, int64(intArg(args, "expectedVersion", 0)))
				if e2 != nil {
					return e2
				}
				output = map[string]any{"versionId": version}
			}
			return e.completeTool(ctx, r, call, output)
		})
		if err != nil {
			return err
		}
		encoded, _ := json.Marshal(output)
		return e.appendToolResponse(ctx, r, call, encoded)
	}
	var output any
	if call.Name == "read_context" {
		if stringArg(args, "refId") != "" {
			output, err = e.Service.ReadReference(ctx, r.Publication.GroupID, stringArg(args, "kind"), stringArg(args, "refId"), intArg(args, "offset", 0), intArg(args, "limit", 8000))
		} else {
			output, err = e.Service.KnowledgeIndex(ctx, r.Publication.GroupID, int64(intArg(args, "after", 0)), min(20, intArg(args, "limit", 20)))
		}
	} else {
		output, err = e.sandboxTool(ctx, call.Name, args, get)
	}
	if err != nil {
		var unknown *unknownEffect
		if errors.As(err, &unknown) {
			_, _ = e.Service.Pool.Exec(context.WithoutCancel(ctx), `UPDATE role_tool_calls SET state='unknown' WHERE run_id=$1 AND call_id=$2`, r.Publication.RunID, call.ID)
			return err
		}
		output = map[string]any{"error": err.Error()}
	}
	return e.toolResponse(ctx, r, call, output)
}
func (e *Executor) completeTool(ctx context.Context, r *run, call model.ToolCall, result any) error {
	encoded, _ := json.Marshal(result)
	if _, err := e.Service.Pool.Exec(ctx, `UPDATE role_tool_calls SET state='completed',result=$3 WHERE run_id=$1 AND call_id=$2`, r.Publication.RunID, call.ID, encoded); err != nil {
		return err
	}
	return e.Service.ToolKnowledge(ctx, r.Publication, call.ID, call.Name, result)
}
func (e *Executor) toolResponse(ctx context.Context, r *run, call model.ToolCall, result any) error {
	err := e.Service.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error {
		if _, e2 := e.Service.Pool.Exec(ctx, `SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, r.Publication.GroupID); e2 != nil {
			return e2
		}
		return e.completeTool(ctx, r, call, result)
	})
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(result)
	return e.appendToolResponse(ctx, r, call, encoded)
}
func (e *Executor) appendToolResponse(ctx context.Context, r *run, call model.ToolCall, result json.RawMessage) error {
	body := string(result)
	if len(body) > 16000 {
		excerpt := []byte(body)[:16000]
		for !utf8.Valid(excerpt) {
			excerpt = excerpt[:len(excerpt)-1]
		}
		encoded, _ := json.Marshal(map[string]any{"excerpt": string(excerpt), "original": map[string]any{"kind": "tool", "refId": r.Publication.RunID.String() + ":" + call.ID}, "readMore": "Use read_context to page the original public result"})
		body = string(encoded)
	}
	r.Checkpoint = append(r.Checkpoint, model.Message{Role: "tool", ToolCallID: call.ID, Content: body})
	return e.checkpoint(ctx, r)
}
func (e *Executor) recoverPending(ctx context.Context, r *run, get sandboxGetter) error {
	answered := map[string]bool{}
	pending := []model.ToolCall{}
	for _, m := range r.Checkpoint {
		if m.Role == "tool" {
			answered[m.ToolCallID] = true
		}
		pending = append(pending, m.ToolCalls...)
	}
	for _, call := range pending {
		if !answered[call.ID] {
			if err := e.executeTool(ctx, r, call, get); err != nil {
				return err
			}
		}
	}
	return nil
}
func stringArg(args map[string]any, key string) string { value, _ := args[key].(string); return value }
func intArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key].(float64)
	if !ok {
		return fallback
	}
	return int(value)
}
func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func workingPath(value string) bool {
	return strings.HasPrefix(value, "/workspace/") && path.Clean(value) == value && value != "/workspace/group" && !strings.HasPrefix(value, "/workspace/group/") && !strings.ContainsRune(value, 0)
}

type unknownEffect struct{}

func (*unknownEffect) Error() string {
	return "sandbox command outcome unknown; human reconciliation required"
}
func (e *Executor) sandboxTool(ctx context.Context, name string, args map[string]any, get sandboxGetter) (any, error) {
	sandbox, err := get(ctx)
	if err != nil {
		return nil, err
	}
	command := stringArg(args, "command")
	timeout := intArg(args, "timeoutMs", 30000)
	switch name {
	case "read":
		file := stringArg(args, "path")
		if !strings.HasPrefix(file, "/workspace/") || path.Clean(file) != file {
			return nil, fmt.Errorf("path must be under /workspace")
		}
		offset := intArg(args, "offset", 0)
		limit := min(16000, intArg(args, "limit", 8000))
		command = fmt.Sprintf("dd if=%s bs=1 skip=%d count=%d 2>/dev/null", quote(file), offset, limit)
	case "write", "edit":
		file := stringArg(args, "path")
		if !workingPath(file) {
			return nil, fmt.Errorf("immutable group projection or invalid working path")
		}
		prefix := "set -eu; "
		if name == "edit" {
			prefix += "test $(sha256sum " + quote(file) + " | cut -d ' ' -f 1) = " + quote(stringArg(args, "expectedSha256")) + " || exit 73; "
		}
		content := base64.StdEncoding.EncodeToString([]byte(stringArg(args, "content")))
		command = prefix + "mkdir -p " + quote(path.Dir(file)) + "; printf %s " + quote(content) + " | base64 -d > " + quote(file) + "; sha256sum " + quote(file)
	}
	exit, stdout, stderr, unknown, err := sandbox.Exec(ctx, command, timeout)
	if unknown {
		return nil, &unknownEffect{}
	}
	if err != nil {
		return nil, err
	}
	// bytes.ToValidUTF8 keeps public JSON valid even for binary shell output.
	return map[string]any{"exitCode": exit, "stdout": string(bytes.ToValidUTF8(stdout, []byte("�"))), "stderr": string(bytes.ToValidUTF8(stderr, []byte("�")))}, nil
}
