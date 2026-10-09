package tools

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

const maxArtifactBytes = 512 << 10

func writablePath(raw string) bool {
	return strings.HasPrefix(raw, "/workspace/") && path.Clean(raw) == raw && !strings.HasPrefix(raw, "/workspace/project/") && raw != "/workspace/project"
}
func registerFileTools(r *Registry) {
	for _, name := range []string{"write", "edit"} {
		name := name
		required := []string{"path", "content"}
		if name == "edit" {
			required = append(required, "expectedSha256")
		}
		r.Register(Tool{Name: name, Description: "写入当前沙箱 /workspace 的分析文件；edit 必须匹配原文件 SHA256。共享 project 目录不可写。", Effect: EffectSandbox, InputSchema: objectSchema(required, map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string", "maxLength": maxArtifactBytes}, "expectedSha256": map[string]any{"type": "string", "pattern": "^[a-f0-9]{64}$"}}), Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			file, content := str(args, "path"), str(args, "content")
			if !writablePath(file) || len(content) > maxArtifactBytes {
				return Result{}, fmt.Errorf("artifact path or size rejected")
			}
			if env.Sandbox == nil {
				return Result{}, fmt.Errorf("sandbox unavailable")
			}
			expected := str(args, "expectedSha256")
			prefix := "set -eu; "
			if name == "edit" || expected != "" {
				prefix += `test -f ` + shellEscape(file) + `; actual=$(sha256sum ` + shellEscape(file) + ` | cut -d ' ' -f 1); test "$actual" = ` + shellEscape(expected) + ` || exit 73; `
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(content))
			command := prefix + `mkdir -p ` + shellEscape(path.Dir(file)) + `; tmp=$(mktemp ` + shellEscape(path.Dir(file)+"/.judex-artifact-XXXXXX") + `); trap 'rm -f "$tmp"' EXIT; printf %s ` + shellEscape(encoded) + ` | base64 -d > "$tmp"; mv -f "$tmp" ` + shellEscape(file)
			exit, _, stderr, unknown, err := env.Sandbox.Exec(ctx, command, 30000)
			if err != nil || unknown {
				return Result{Data: map[string]any{"unknown": unknown}}, fmt.Errorf("sandbox write result unavailable: %v", err)
			}
			if exit == 73 {
				return Result{}, fmt.Errorf("file SHA256 changed")
			}
			if exit != 0 {
				return Result{}, fmt.Errorf("sandbox write failed (%d): %s", exit, stderr)
			}
			hash := sha256.Sum256([]byte(content))
			return Result{Data: map[string]any{"path": file, "size": len(content), "sha256": hex.EncodeToString(hash[:])}}, nil
		}})
	}
	r.Register(Tool{Name: "publish", Description: "将当前沙箱产物登记为项目资料的新版本；必须列出作为依据的固定材料版本。不会批准或验收业务。", Effect: EffectDraft, InputSchema: objectSchema([]string{"path", "name", "sourceVersionIds", "idempotencyKey"}, map[string]any{"path": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "kind": map[string]any{"type": "string", "enum": []string{"file", "html_bundle"}}, "mime": map[string]any{"type": "string"}, "entrypoint": map[string]any{"type": "string"}, "sourceVersionIds": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string", "format": "uuid"}}, "materialId": map[string]any{"type": "string", "format": "uuid"}, "baseVersionId": map[string]any{"type": "string", "format": "uuid"}, "idempotencyKey": map[string]any{"type": "string", "minLength": 1, "maxLength": 120}}), Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
		file := str(args, "path")
		if !writablePath(file) || env.Sandbox == nil || env.Publish == nil {
			return Result{}, fmt.Errorf("publish source or service unavailable")
		}
		command := "set -eu; test -f " + shellEscape(file) + "; test ! -L " + shellEscape(file) + "; test $(wc -c < " + shellEscape(file) + ") -le 524288; base64 " + shellEscape(file)
		exit, stdout, _, unknown, err := env.Sandbox.Exec(ctx, command, 30000)
		if err != nil || unknown {
			return Result{Data: map[string]any{"unknown": unknown}}, fmt.Errorf("artifact read unavailable: %v", err)
		}
		if exit != 0 {
			return Result{}, fmt.Errorf("artifact must be a regular file within size limit")
		}
		raw, err := base64.StdEncoding.DecodeString(string(stdout))
		if err != nil || len(raw) > maxArtifactBytes {
			return Result{}, fmt.Errorf("artifact bytes invalid or truncated")
		}
		data, err := env.Publish(ctx, env.RunID, args, raw)
		return Result{Data: data}, err
	}})
}
