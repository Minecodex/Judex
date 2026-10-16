package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

func registerMaterialAnalysis(r *Registry) {
	r.Register(Tool{Name: "analyze_material", Description: "在当前运行沙箱读取固定版本原件并验证 SHA256；文本/Markdown/JSON 返回行号，源码 ZIP 返回条目/行号；DOCX 段落/表格、PPTX 页/文字/表格、XLSX 单元格/公式及既有缓存值、PDF 每页文字或图片元数据。含解析器版本、范围和限制。不执行源码；不能以文本解析证明排版或以公式缓存值证明重算；图片元数据不等于视觉理解。", Effect: EffectSandbox,
		InputSchema: objectSchema([]string{"versionId"}, map[string]any{
			"versionId": map[string]any{"type": "string", "format": "uuid"},
			"offset":    map[string]any{"type": "integer", "minimum": 0, "maximum": 100000},
			"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
		}), Execute: func(ctx context.Context, args map[string]any, env Env) (Result, error) {
			id, err := uuid.Parse(str(args, "versionId"))
			if err != nil || env.Sandbox == nil || env.MaterialInfo == nil {
				return Result{}, fmt.Errorf("original analysis or fixed version unavailable")
			}
			info, err := env.MaterialInfo(ctx, env.ProjectID, id.String())
			if err != nil {
				return Result{}, err
			}
			sha, mime := str(info, "sha256"), str(info, "mime")
			offset, limit := intVal(args, "offset", 0), intVal(args, "limit", 50)
			command := "python /opt/judex/analyze_material.py --version " + shellEscape(id.String()) + " --sha " + shellEscape(sha) + " --mime " + shellEscape(mime) + " --offset " + strconv.Itoa(offset) + " --limit " + strconv.Itoa(limit)
			exit, stdout, stderr, unknown, err := env.Sandbox.Exec(ctx, command, 60000)
			if err != nil || unknown {
				return Result{Data: map[string]any{"unknown": unknown}}, fmt.Errorf("sandbox original analysis unavailable: %v", err)
			}
			if exit != 0 {
				return Result{}, fmt.Errorf("original analysis failed: %s", stderr)
			}
			if len(stdout) > 256<<10 {
				return Result{}, fmt.Errorf("analysis output exceeds limit; use a smaller page")
			}
			var out map[string]any
			if err = json.Unmarshal(stdout, &out); err != nil {
				return Result{}, fmt.Errorf("analysis parser response invalid")
			}
			if str(out, "versionId") != id.String() || str(out, "sha256") != sha || str(out, "parserVersion") == "" {
				return Result{}, fmt.Errorf("analysis provenance mismatch")
			}
			return Result{Data: out}, nil
		}})
}
