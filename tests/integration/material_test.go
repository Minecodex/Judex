// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/material"
	"github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/project"
	integration "github.com/kakj-go/Judex/tests/integration"
)

// memStore is an in-memory ObjectStore fake for integration tests (real PG,
// fake objects — object-level behavior is covered by deploy-time checks).
type memStore struct {
	mu    sync.Mutex
	data  map[string][]byte
}

func newMemStore() *memStore { return &memStore{data: map[string][]byte{}} }

func (m *memStore) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.data[key] = raw
	m.mu.Unlock()
	return nil
}

func (m *memStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.data[key]
	if !ok {
		return nil, errors.Newf(errors.DependencyDown, "missing object %s", key)
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
	return nil
}

func newMaterialEnv(t *testing.T) (*material.Service, *project.Service, *identity.Service, *postgres.Pool) {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	ids := identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
	svc := material.NewService(fixture.Pool, newMemStore(), material.Limits{PartSize: 64, SessionTTL: 3600e9}, nil)
	return svc, project.NewService(fixture.Pool, nil), ids, fixture.Pool
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestMaterialUploadCompleteIdempotent (C01/C03 核心): 分片上传、摘要校验、
// complete 幂等（重试返回同版本）、派生发布版本冲突、下载。
func TestMaterialUploadCompleteIdempotent(t *testing.T) {
	svc, projects, ids, _ := newMaterialEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "MAT", "mat@mat.test", "password-mat-mat", "10.0.0.1")
	proj, err := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "材料项目"})
	if err != nil {
		t.Fatal(err)
	}

	content := strings.Repeat("judex-material-", 20) // 300 bytes at 64B part size -> 5 parts
	sum := digest(content)
	session, err := svc.CreateUpload(ctx, owner.ID, proj.ID, "报告.md", "file", "text/markdown", int64(len(content)), sum, "")
	if err != nil {
		t.Fatal(err)
	}
	parts := session.PartCount
	if parts != 5 {
		t.Fatalf("expected 5 parts for %d bytes at 64B part size, got %d", len(content), parts)
	}
	// Upload parts.
	for n := 0; n < parts; n++ {
		end := (n + 1) * 64
		if end > len(content) {
			end = len(content)
		}
		chunk := content[n*64 : end]
		if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session.ID, n+1, digest(chunk), strings.NewReader(chunk)); err != nil {
			t.Fatalf("part %d: %v", n+1, err)
		}
		// Idempotent re-upload of the same part.
		if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session.ID, n+1, digest(chunk), strings.NewReader(chunk)); err != nil {
			t.Fatalf("re-part %d: %v", n+1, err)
		}
	}
	// Same number, different content -> conflict.
	if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session.ID, 1, digest("tampered!"), strings.NewReader("tampered!")); errors.IsCode(err, errors.IdempotencyConflict) == false {
		t.Fatalf("tampered part must conflict, got %v", err)
	}
	version, err := svc.Complete(ctx, owner.ID, proj.ID, session.ID, nil)
	if err != nil || version.State != "ready" || version.Revision != 1 {
		t.Fatalf("complete: %v %+v", err, version)
	}
	// Retry complete returns the SAME version (lost response case).
	retry, err := svc.Complete(ctx, owner.ID, proj.ID, session.ID, nil)
	if err != nil || retry.ID != version.ID {
		t.Fatalf("complete retry must return same version: %v %+v", err, retry)
	}

	// Derived publish with a behind source version conflicts.
	content2 := content + "-v2"
	sum2 := digest(content2)
	session2, err := svc.CreateUpload(ctx, owner.ID, proj.ID, "报告.md", "file", "text/markdown", int64(len(content2)), sum2, "")
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < len(content2); n += 64 {
		end := n + 64
		if end > len(content2) {
			end = len(content2)
		}
		chunk := content2[n:end]
		if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session2.ID, n/64+1, digest(chunk), strings.NewReader(chunk)); err != nil {
			t.Fatalf("v2 part: %v", err)
		}
	}
	v2, err := svc.Complete(ctx, owner.ID, proj.ID, session2.ID, &version.ID)
	if err != nil || v2.Revision != 2 {
		t.Fatalf("derived publish: %v %+v", err, v2)
	}
	// Publishing against the OLD version now conflicts.
	session3, err := svc.CreateUpload(ctx, owner.ID, proj.ID, "报告.md", "file", "text/markdown", int64(len(content)), sum, "")
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < parts; n++ {
		end := (n + 1) * 64
		if end > len(content) {
			end = len(content)
		}
		chunk := content[n*64 : end]
		if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session3.ID, n+1, digest(chunk), strings.NewReader(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Complete(ctx, owner.ID, proj.ID, session3.ID, &version.ID); errors.IsCode(err, errors.SourceVersionConf) == false {
		t.Fatalf("behind source must be SOURCE_VERSION_CONFLICT, got %v", err)
	}

	// Versions immutable: old version still listed and downloadable.
	versions, err := svc.ListVersions(ctx, owner.ID, proj.ID, v2.MaterialID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions: %v %+v", err, versions)
	}
	body, _, err := svc.VersionObject(ctx, owner.ID, proj.ID, v2.MaterialID, version.ID, "part-000001")
	if err != nil {
		t.Fatalf("download old version: %v", err)
	}
	raw, _ := io.ReadAll(body)
	body.Close()
	if string(raw) != content[0:64] {
		t.Fatalf("download content mismatch: %q", string(raw))
	}
}

// TestMaterialIncompleteAndWrongDigest (C01): 缺片/整文件摘要不符拒绝。
func TestMaterialIncompleteAndWrongDigest(t *testing.T) {
	svc, projects, ids, _ := newMaterialEnv(t)
	ctx := context.Background()
	owner, _, _ := ids.Register(ctx, "MT", "mt@mt.test", "password-mt-mt-1", "10.0.0.1")
	proj, _ := projects.Create(ctx, owner.ID, project.CreateRequest{Title: "材料项目2"})

	content := "0123456789abcdef"
	session, err := svc.CreateUpload(ctx, owner.ID, proj.ID, "a.bin", "file", "application/octet-stream", 16, digest(content), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(ctx, owner.ID, proj.ID, session.ID, nil); errors.IsCode(err, errors.RequirementUnmet) == false {
		t.Fatalf("missing parts must be REQUIREMENT_UNMET, got %v", err)
	}
	if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session.ID, 1, digest(content), strings.NewReader("WRONGCONTENT!!")); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("declared digest mismatch must be VALIDATION_ERROR, got %v", err)
	}
	// Whole-file checksum wrong: parts present but declared sum differs.
	session2, _ := svc.CreateUpload(ctx, owner.ID, proj.ID, "b.bin", "file", "application/octet-stream", 16, digest("another-16bytes"), "")
	if _, err := svc.UploadPart(ctx, owner.ID, proj.ID, session2.ID, 1, digest(content), strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(ctx, owner.ID, proj.ID, session2.ID, nil); errors.IsCode(err, errors.RequirementUnmet) == false {
		t.Fatalf("whole-file mismatch must be REQUIREMENT_UNMET, got %v", err)
	}
}
