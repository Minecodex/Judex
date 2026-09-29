package batch

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"io"
	"unicode/utf8"
)

func (e *Executor) materialFacts(ctx context.Context, project, batch uuid.UUID) []string {
	rows, err := e.Pool.Query(ctx, `SELECT v.id::text,m.title,v.sha256 FROM discussion_batches b
  JOIN submission_materials sm ON sm.submission_id=b.source_submission_id AND sm.project_id=b.project_id
  JOIN material_versions v ON v.id=sm.material_version_id AND v.project_id=b.project_id
  JOIN materials m ON m.id=v.material_id WHERE b.id=$1 AND b.project_id=$2 ORDER BY v.id`, batch, project)
	if err != nil {
		return []string{"材料清单无法读取；不得声称材料核对完成"}
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, title, sha string
		if rows.Scan(&id, &title, &sha) == nil {
			out = append(out, fmt.Sprintf("共享材料 %s versionId=%s sha256=%s；使用 read_material 读取原文；沙箱只读目录 /workspace/project/versions/<versionId>/", title, id, sha))
		}
	}
	return out
}
func (e *Executor) readMaterial(ctx context.Context, project, version string) (map[string]any, error) {
	if e.Objects == nil {
		return nil, fmt.Errorf("object storage unavailable")
	}
	var sha, mime string
	var size int64
	if err := e.Pool.QueryRow(ctx, `SELECT sha256,mime,size FROM material_versions WHERE project_id=$1 AND id=$2 AND state='ready'`, project, version).Scan(&sha, &mime, &size); err != nil {
		return nil, fmt.Errorf("material version not ready in project")
	}
	rows, err := e.Pool.Query(ctx, `SELECT relative_path,object_key FROM material_entries WHERE project_id=$1 AND version_id=$2 ORDER BY relative_path`, project, version)
	if err != nil {
		return nil, err
	}
	type entry struct{ path, key string }
	var entries []entry
	for rows.Next() {
		var item entry
		if err = rows.Scan(&item.path, &item.key); err != nil {
			rows.Close()
			return nil, err
		}
		entries = append(entries, item)
	}
	rows.Close()
	remaining := int64(1 << 20)
	content := ""
	truncated := false
	for _, item := range entries {
		body, err := e.Objects.Get(ctx, item.key)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(body, remaining+1))
		body.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(raw)) > remaining {
			raw = raw[:remaining]
			truncated = true
		}
		remaining -= int64(len(raw))
		if !utf8.Valid(raw) {
			return map[string]any{"versionId": version, "mime": mime, "sha256": sha, "size": size, "binary": true, "note": "binary content requires the sandbox material mount"}, nil
		}
		content += "\n" + item.path + "\n" + string(raw)
		if truncated {
			break
		}
	}
	return map[string]any{"versionId": version, "mime": mime, "sha256": sha, "size": size, "content": content, "truncated": truncated}, nil
}
