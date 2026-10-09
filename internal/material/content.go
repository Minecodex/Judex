package material

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"io"
	"strings"
)

// FileContent streams the original immutable file in part order. At most
// one object reader is open, including when a client cancels a large download.
func (s *Service) FileContent(ctx context.Context, user, project, material, version uuid.UUID) (io.ReadCloser, MaterialVersion, error) {
	v, _, err := s.OpenVersion(ctx, user, project, material, version)
	if err != nil {
		return nil, v, err
	}
	var kind string
	if err = s.pool.QueryRow(ctx, `SELECT kind FROM materials WHERE id=$1 AND project_id=$2`, material, project).Scan(&kind); err != nil {
		return nil, v, err
	}
	if kind == "html_bundle" {
		var staging string
		var count int
		err = s.pool.QueryRow(ctx, `SELECT staging_key,part_count FROM upload_sessions WHERE project_id=$1 AND result_version_id=$2`, project, version).Scan(&staging, &count)
		if err == pgx.ErrNoRows {
			var manifest string
			err = s.pool.QueryRow(ctx, `SELECT manifest_key FROM material_versions WHERE project_id=$1 AND id=$2 AND run_id IS NOT NULL`, project, version).Scan(&manifest)
			if err == nil && strings.HasSuffix(manifest, "/manifest.json") {
				staging = strings.TrimSuffix(manifest, "/manifest.json")
				count = 1
			}
		}
		if err != nil || count < 1 {
			return nil, v, apierrors.New(apierrors.NotFound, "original bundle unavailable")
		}
		keys := []string{}
		for part := 1; part <= count; part++ {
			keys = append(keys, partKey(staging, part))
		}
		first, err := s.store.Get(ctx, keys[0])
		if err != nil {
			return nil, v, err
		}
		return &fileReader{ctx: ctx, store: s.store, keys: keys[1:], current: first}, v, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT object_key FROM material_entries WHERE project_id=$1 AND version_id=$2 ORDER BY relative_path`, project, version)
	if err != nil {
		return nil, v, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, v, err
		}
		keys = append(keys, key)
	}
	if err = rows.Err(); err != nil {
		return nil, v, err
	}
	if len(keys) == 0 {
		return nil, v, apierrors.New(apierrors.NotFound, "file content missing")
	}
	first, err := s.store.Get(ctx, keys[0])
	if err != nil {
		return nil, v, err
	}
	return &fileReader{ctx: ctx, store: s.store, keys: keys[1:], current: first}, v, nil
}

type fileReader struct {
	ctx     context.Context
	store   ObjectStore
	keys    []string
	current io.ReadCloser
	closed  bool
}

func (r *fileReader) Read(p []byte) (int, error) {
	for !r.closed {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		if r.current == nil {
			if len(r.keys) == 0 {
				return 0, io.EOF
			}
			body, err := r.store.Get(r.ctx, r.keys[0])
			if err != nil {
				return 0, err
			}
			r.keys = r.keys[1:]
			r.current = body
		}
		n, err := r.current.Read(p)
		if err == io.EOF {
			r.current.Close()
			r.current = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
	return 0, io.EOF
}
func (r *fileReader) Close() error {
	r.closed = true
	if r.current != nil {
		return r.current.Close()
	}
	return nil
}

func (s *Service) MaterialForVersion(ctx context.Context, user, project, version uuid.UUID) (uuid.UUID, error) {
	if err := isMemberTx(ctx, s.pool, project, user); err != nil {
		return uuid.Nil, err
	}
	var material uuid.UUID
	if err := s.pool.QueryRow(ctx, `SELECT material_id FROM material_versions WHERE project_id=$1 AND id=$2`, project, version).Scan(&material); err != nil {
		return uuid.Nil, apierrors.New(apierrors.NotFound, "material version not found")
	}
	return material, nil
}
