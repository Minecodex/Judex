package batch

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/projectvolume"
	"time"
)

func (e *Executor) ProjectFiles(ctx context.Context, project uuid.UUID) ([]projectvolume.File, error) {
	signer, ok := e.Objects.(interface {
		PresignGet(context.Context, string, time.Duration) (string, error)
	})
	if !ok {
		return nil, fmt.Errorf("object store cannot issue scoped read capabilities")
	}
	rows, err := e.Pool.Query(ctx, `SELECT v.id::text,m.kind,v.sha256,e.relative_path,e.object_key,e.sha256
  FROM material_versions v JOIN materials m ON m.id=v.material_id AND m.project_id=v.project_id
  JOIN material_entries e ON e.version_id=v.id AND e.project_id=v.project_id
  WHERE v.project_id=$1 AND v.state='ready' ORDER BY v.id,e.relative_path`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []projectvolume.File{}
	indices := map[string]int{}
	for rows.Next() {
		var version, kind, hash, path, key, partHash string
		if err = rows.Scan(&version, &kind, &hash, &path, &key, &partHash); err != nil {
			return nil, err
		}
		filePath := "versions/" + version + "/content"
		if kind == "html_bundle" {
			filePath = "versions/" + version + "/" + path
			hash = partHash
		}
		url, err := signer.PresignGet(ctx, key, 10*time.Minute)
		if err != nil {
			return nil, err
		}
		index, found := indices[filePath]
		if !found {
			index = len(out)
			indices[filePath] = index
			out = append(out, projectvolume.File{Path: filePath, SHA256: hash})
		}
		out[index].Parts = append(out[index].Parts, projectvolume.Part{URL: url, SHA256: partHash})
	}
	return out, rows.Err()
}
