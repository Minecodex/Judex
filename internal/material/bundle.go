package material

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"io"
	"mime"
	"path"
	"strings"
)

// registerBundle expands only safe regular entries and preserves their
// relative names. Size is bounded on decompressed bytes, not ZIP metadata.
func (s *Service) registerBundle(ctx context.Context, tx pgx.Tx, project, version uuid.UUID, staging string, parts int, entrypoint string) error {
	var raw bytes.Buffer
	for n := 1; n <= parts; n++ {
		body, err := s.store.Get(ctx, partKey(staging, n))
		if err != nil {
			return err
		}
		_, err = io.Copy(&raw, io.LimitReader(body, s.limits.MaxBundleBytes+1-int64(raw.Len())))
		body.Close()
		if err != nil {
			return err
		}
		if int64(raw.Len()) > s.limits.MaxBundleBytes {
			return apierrors.New(apierrors.PayloadTooLarge, "bundle too large")
		}
	}
	type entry struct {
		name string
		data []byte
	}
	var entries []entry
	if strings.HasPrefix(raw.String(), "PK") {
		archive, err := zip.NewReader(bytes.NewReader(raw.Bytes()), int64(raw.Len()))
		if err != nil {
			return apierrors.Fields("bundle", "invalid zip")
		}
		if len(archive.File) > 2000 {
			return apierrors.New(apierrors.PayloadTooLarge, "too many bundle entries")
		}
		total := int64(0)
		for _, file := range archive.File {
			if file.FileInfo().IsDir() {
				continue
			}
			if !file.Mode().IsRegular() || !safeRelativePath(file.Name) {
				return apierrors.Fields("bundle.path", "unsafe")
			}
			body, err := file.Open()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(body, s.limits.MaxBundleBytes-total+1))
			body.Close()
			if err != nil {
				return err
			}
			total += int64(len(data))
			if total > s.limits.MaxBundleBytes {
				return apierrors.New(apierrors.PayloadTooLarge, "expanded bundle too large")
			}
			entries = append(entries, entry{file.Name, data})
		}
	} else {
		entries = []entry{{entrypoint, raw.Bytes()}}
	}
	names := []string{}
	found := false
	for _, item := range entries {
		names = append(names, item.name)
		if item.name == entrypoint {
			found = true
		}
	}
	if err := ValidateBundleEntries(names, s.limits); err != nil {
		return err
	}
	if !found {
		return apierrors.Fields("entrypoint", "missing")
	}
	for _, item := range entries {
		hash := sha256.Sum256(item.data)
		key := fmt.Sprintf("projects/%s/versions/%s/%s", project, version, item.name)
		contentType := mime.TypeByExtension(path.Ext(item.name))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if err := s.store.Put(ctx, key, bytes.NewReader(item.data), int64(len(item.data)), contentType); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO material_entries(project_id,version_id,relative_path,object_key,size,sha256,mime) VALUES($1,$2,$3,$4,$5,$6,$7)`, project, version, item.name, key, len(item.data), hex.EncodeToString(hash[:]), contentType); err != nil {
			return err
		}
	}
	return nil
}
