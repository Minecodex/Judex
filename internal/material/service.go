// SPDX-License-Identifier: Apache-2.0

// Package material implements upload sessions and immutable material
// versions per docs/plans/v1/04 §2-§3: chunked uploads proxied through the
// API, per-part and whole-file SHA-256 verification, exactly-once
// registration on complete (retry returns the same version), derived
// publishes guarded by source versions.
package material

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/kakj-go/Judex/internal/platform/paging"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/audit"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/events"
)

// ObjectStore is the storage surface materials need; production uses
// internal/infrastructure/objectstore, tests an in-memory fake.
type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Limits mirror 04 §2 defaults (deployment-overridable via capabilities).
type Limits struct {
	MaxFileBytes          int64
	MaxBundleBytes        int64
	MaxFilesPerSubmission int
	MaxTextBytes          int64
	PartSize              int64
	SessionTTL            time.Duration
}

func DefaultLimits() Limits {
	return Limits{
		MaxFileBytes:          100 << 20,
		MaxBundleBytes:        200 << 20,
		MaxFilesPerSubmission: 20,
		MaxTextBytes:          64 << 10,
		PartSize:              8 << 20,
		SessionTTL:            24 * time.Hour,
	}
}

// UploadSession is the API projection (06 §4).
type UploadSession struct {
	ID              uuid.UUID  `json:"id"`
	State           string     `json:"state"`
	Kind            string     `json:"kind"`
	Name            string     `json:"name"`
	ExpectedSize    int64      `json:"expectedSize"`
	Checksum        string     `json:"checksum"`
	Mime            string     `json:"mime"`
	PartSize        int64      `json:"partSize"`
	PartCount       int        `json:"partCount"`
	MaterialID      *uuid.UUID `json:"materialId"`
	ResultVersionID *uuid.UUID `json:"resultVersionId"`
	ExpiresAt       time.Time  `json:"expiresAt"`
}

// Material / MaterialVersion projections (06 §4).
type Material struct {
	ID               uuid.UUID  `json:"id"`
	Title            string     `json:"title"`
	Kind             string     `json:"kind"`
	Visibility       string     `json:"visibility"`
	CurrentVersionID *uuid.UUID `json:"currentVersionId"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type MaterialVersion struct {
	ID         uuid.UUID       `json:"id"`
	MaterialID uuid.UUID       `json:"materialId"`
	Revision   int64           `json:"revision"`
	State      string          `json:"state"`
	SHA256     string          `json:"sha256"`
	Size       int64           `json:"size"`
	Mime       string          `json:"mime"`
	Entrypoint *string         `json:"entrypoint"`
	Entries    []MaterialEntry `json:"entries"`
	AuthorID   *uuid.UUID      `json:"authorId"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type MaterialEntry struct {
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	Mime         string `json:"mime"`
}

type Service struct {
	pool   *postgres.Pool
	store  ObjectStore
	limits Limits
	now    func() time.Time
}

// unavailableStore makes missing object storage an explicit 503 instead of a
// nil-interface panic (11 §2: S3 故障禁止上传成功，但不拖垮其它路径).
type unavailableStore struct{}

func (unavailableStore) Put(context.Context, string, io.Reader, int64, string) error {
	return apierrors.New(apierrors.DependencyDown, "对象存储未配置").WithRetryable(true)
}
func (unavailableStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, apierrors.New(apierrors.DependencyDown, "对象存储未配置").WithRetryable(true)
}
func (unavailableStore) Delete(context.Context, string) error {
	return apierrors.New(apierrors.DependencyDown, "对象存储未配置").WithRetryable(true)
}

func NewService(pool *postgres.Pool, store ObjectStore, limits Limits, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	// Per-field defaults: tests override part size without zeroing caps.
	defaults := DefaultLimits()
	if limits.MaxFileBytes == 0 {
		limits.MaxFileBytes = defaults.MaxFileBytes
	}
	if limits.MaxBundleBytes == 0 {
		limits.MaxBundleBytes = defaults.MaxBundleBytes
	}
	if limits.MaxFilesPerSubmission == 0 {
		limits.MaxFilesPerSubmission = defaults.MaxFilesPerSubmission
	}
	if limits.MaxTextBytes == 0 {
		limits.MaxTextBytes = defaults.MaxTextBytes
	}
	if limits.PartSize == 0 {
		limits.PartSize = defaults.PartSize
	}
	if limits.SessionTTL == 0 {
		limits.SessionTTL = defaults.SessionTTL
	}
	return &Service{pool: pool, store: store, limits: limits, now: now}
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func isMemberTx(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, projectID, requester uuid.UUID) error {
	var role string
	err := q.QueryRow(ctx, `
		SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2 AND state='active'`,
		projectID, requester).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return apierrors.New(apierrors.NotFound, "project not found")
	}
	return err
}

// safeRelativePath rejects absolute paths, traversal, drive/UNC and NUL
// (04 §4); case collisions fall to the DB unique key.
func safeRelativePath(p string) bool {
	if p == "" || strings.ContainsRune(p, 0) {
		return false
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") || strings.HasPrefix(p, "\\\\") {
		return false
	}
	if p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") || strings.Contains(p, "\\..\\") {
		return false
	}
	if len(p) >= 2 && p[1] == ':' {
		return false
	}
	return true
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

type countingReader struct {
	inner io.Reader
	n     int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	c.n += int64(n)
	return n, err
}

func partKey(stagingKey string, partNumber int) string {
	return fmt.Sprintf("%s/part-%06d", stagingKey, partNumber)
}

// CreateUpload opens a session and computes the part plan (04 §2 step 1).
func (s *Service) CreateUpload(ctx context.Context, requester, projectID uuid.UUID, name, kind, mime string, size int64, checksum, entrypoint string) (UploadSession, error) {
	if l := utf8.RuneCountInString(strings.TrimSpace(name)); l < 1 || l > 500 {
		return UploadSession{}, apierrors.Fields("name", "length")
	}
	if kind != "file" && kind != "html_bundle" {
		return UploadSession{}, apierrors.Fields("kind", "enum")
	}
	limit := s.limits.MaxFileBytes
	if kind == "html_bundle" {
		limit = s.limits.MaxBundleBytes
	}
	if size < 1 || size > limit {
		return UploadSession{}, apierrors.Newf(apierrors.PayloadTooLarge, "size must be 1..%d", limit)
	}
	if !isHex64(checksum) {
		return UploadSession{}, apierrors.Fields("sha256", "format")
	}
	if kind == "html_bundle" && entrypoint == "" {
		return UploadSession{}, apierrors.Fields("entrypoint", "required")
	}
	if entrypoint != "" && !safeRelativePath(entrypoint) {
		return UploadSession{}, apierrors.Fields("entrypoint", "path")
	}
	var out UploadSession
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if err := isMemberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		now := s.now()
		id := uuid.New()
		partCount := int((size + s.limits.PartSize - 1) / s.limits.PartSize)
		stagingKey := fmt.Sprintf("staging/%s/%s", projectID, id)
		if err := tx.QueryRow(ctx, `
			INSERT INTO upload_sessions (project_id, id, owner_user_id, state, kind, name, staging_key,
				expected_size, checksum, mime, entrypoint, part_size, part_count, expires_at, created_at)
			VALUES ($1,$2,$3,'open',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			RETURNING id, state, kind, name, expected_size, checksum, mime, part_size, part_count, expires_at`,
			projectID, id, requester, kind, name, stagingKey, size, checksum,
			mime, nullable(entrypoint), s.limits.PartSize, partCount, now.Add(s.limits.SessionTTL), now).
			Scan(&out.ID, &out.State, &out.Kind, &out.Name, &out.ExpectedSize, &out.Checksum,
				&out.Mime, &out.PartSize, &out.PartCount, &out.ExpiresAt); err != nil {
			return apierrors.New(apierrors.Internal, "session insert failed").Wrap(err)
		}
		return nil
	})
	return out, err
}

// UploadPart streams one chunk to storage, verifying the declared digest
// while streaming; same number with a different digest conflicts (06 §4).
func (s *Service) UploadPart(ctx context.Context, requester, projectID, uploadID uuid.UUID, partNumber int, declaredSHA string, body io.Reader) (int64, error) {
	if partNumber < 1 {
		return 0, apierrors.Fields("partNumber", "range")
	}
	if !isHex64(declaredSHA) {
		return 0, apierrors.Fields("partSha256", "format")
	}
	var (
		partCount  int
		state      string
		owner      uuid.UUID
		expiresAt  time.Time
		stagingKey string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT part_count, state, owner_user_id, expires_at, staging_key
		FROM upload_sessions WHERE id=$1 AND project_id=$2`, uploadID, projectID).
		Scan(&partCount, &state, &owner, &expiresAt, &stagingKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apierrors.New(apierrors.NotFound, "upload not found")
	}
	if err != nil {
		return 0, apierrors.New(apierrors.Internal, "upload lookup failed").Wrap(err)
	}
	if owner != requester {
		return 0, apierrors.New(apierrors.Forbidden, "only the session owner uploads")
	}
	if state != "open" {
		return 0, apierrors.New(apierrors.InvalidTransition, "upload "+state)
	}
	if s.now().After(expiresAt) {
		return 0, apierrors.New(apierrors.InvalidTransition, "upload expired")
	}
	if partNumber > partCount {
		return 0, apierrors.Fields("partNumber", "range")
	}
	var existingSHA *string
	err = s.pool.QueryRow(ctx, `
		SELECT sha256 FROM upload_parts WHERE upload_id=$1 AND part_number=$2`, uploadID, partNumber).Scan(&existingSHA)
	if err == nil && existingSHA != nil {
		if *existingSHA != declaredSHA {
			return 0, apierrors.New(apierrors.IdempotencyConflict, "part number already stored with different content")
		}
		var size int64
		if err := s.pool.QueryRow(ctx, `SELECT size FROM upload_parts WHERE upload_id=$1 AND part_number=$2`, uploadID, partNumber).Scan(&size); err == nil {
			return size, nil
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, apierrors.New(apierrors.Internal, "part lookup failed").Wrap(err)
	}

	key := partKey(stagingKey, partNumber)
	hasher := sha256.New()
	counter := &countingReader{inner: body}
	if err := s.store.Put(ctx, key, io.TeeReader(counter, hasher), -1, "application/octet-stream"); err != nil {
		return 0, err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != declaredSHA {
		_ = s.store.Delete(ctx, key)
		return 0, apierrors.Newf(apierrors.Validation, "part digest mismatch: declared %s actual %s", declaredSHA, actual)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO upload_parts (project_id, upload_id, part_number, size, sha256, received_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (upload_id, part_number) DO UPDATE SET size=EXCLUDED.size, sha256=EXCLUDED.sha256, received_at=EXCLUDED.received_at`,
		projectID, uploadID, partNumber, counter.n, actual, s.now()); err != nil {
		return 0, apierrors.New(apierrors.Internal, "part insert failed").Wrap(err)
	}
	return counter.n, nil
}

// Cancel aborts an open session (staging objects are cleaned by the orphan
// job; formal materials are never deleted here).
func (s *Service) Cancel(ctx context.Context, requester, projectID, uploadID uuid.UUID) error {
	return s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE upload_sessions SET state='cancelled'
			WHERE id=$1 AND project_id=$2 AND owner_user_id=$3 AND state='open'`,
			uploadID, projectID, requester)
		if err != nil {
			return apierrors.New(apierrors.Internal, "cancel failed").Wrap(err)
		}
		if tag.RowsAffected() == 0 {
			return apierrors.New(apierrors.NotFound, "open upload not found")
		}
		return nil
	})
}

// ListMySessions returns the caller's open sessions (06 §4 GET /uploads).
func (s *Service) ListMySessions(ctx context.Context, requester, projectID uuid.UUID) ([]UploadSession, error) {
	if err := isMemberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, state, kind, name, expected_size, checksum, mime, part_size, part_count, expires_at
		/*keys*/ FROM upload_sessions WHERE project_id=$1 AND owner_user_id=$2 AND state='open'
		/*page*/`, "created_at", "id", projectID, requester)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "sessions failed").Wrap(err)
	}
	defer rows.Close()
	var out []UploadSession
	for rows.Next() {
		var u UploadSession
		if err := rows.Scan(&u.ID, &u.State, &u.Kind, &u.Name, &u.ExpectedSize, &u.Checksum, &u.Mime, &u.PartSize, &u.PartCount, &u.ExpiresAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Complete verifies all parts and registers the immutable version exactly
// once; a lost response retried with the same upload returns the same
// version (04 §2 step 4). sourceVersionID targets an existing material for
// a derived publish — behind current is SOURCE_VERSION_CONFLICT (04 §3).
func (s *Service) Complete(ctx context.Context, requester, projectID, uploadID uuid.UUID, sourceVersionID *uuid.UUID) (MaterialVersion, error) {
	var out MaterialVersion
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, uploadID.String()); err != nil {
			return err
		}
		if err := tx.LockActiveProject(ctx, projectID.String()); err != nil {
			return err
		}
		if err := isMemberTx(ctx, tx, projectID, requester); err != nil {
			return err
		}
		var (
			state, kind, name, checksum, mime, stagingKey string
			entrypoint                                    *string
			expectedSize                                  int64
			partCount                                     int
			resultVersion                                 *uuid.UUID
			owner                                         uuid.UUID
		)
		err := tx.QueryRow(ctx, `
			SELECT state, kind, name, checksum, mime, entrypoint, expected_size, part_count,
			       result_version_id, owner_user_id, staging_key
			FROM upload_sessions WHERE id=$1 AND project_id=$2 FOR UPDATE`,
			uploadID, projectID).
			Scan(&state, &kind, &name, &checksum, &mime, &entrypoint, &expectedSize, &partCount,
				&resultVersion, &owner, &stagingKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return apierrors.New(apierrors.NotFound, "upload not found")
		}
		if err != nil {
			return apierrors.New(apierrors.Internal, "upload lookup failed").Wrap(err)
		}
		if owner != requester {
			return apierrors.New(apierrors.Forbidden, "only the session owner completes")
		}
		if resultVersion != nil {
			return s.loadVersionTx(ctx, tx, projectID, *resultVersion, &out)
		}
		if state != "open" {
			return apierrors.New(apierrors.InvalidTransition, "upload "+state)
		}
		var storedParts int
		var storedBytes int64
		rows, err := tx.Query(ctx, `
			SELECT part_number, size, sha256 FROM upload_parts WHERE upload_id=$1 ORDER BY part_number`, uploadID)
		if err != nil {
			return apierrors.New(apierrors.Internal, "parts query failed").Wrap(err)
		}
		partDigests := map[int]string{}
		partSizes := map[int]int64{}
		for rows.Next() {
			var n int
			var size int64
			var digest string
			if err := rows.Scan(&n, &size, &digest); err != nil {
				rows.Close()
				return apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
			}
			partDigests[n] = digest
			partSizes[n] = size
			storedParts++
			storedBytes += size
		}
		rows.Close()
		if storedParts != partCount {
			return apierrors.Newf(apierrors.RequirementUnmet, "parts incomplete: %d/%d", storedParts, partCount)
		}
		if storedBytes != expectedSize {
			return apierrors.Newf(apierrors.RequirementUnmet, "size mismatch: declared %d stored %d", expectedSize, storedBytes)
		}
		// Whole-file digest for plain files: stream parts back through the
		// store (server-side verification, no client involvement).
		if kind == "file" || kind == "html_bundle" {
			hasher := sha256.New()
			for n := 1; n <= partCount; n++ {
				body, err := s.store.Get(ctx, partKey(stagingKey, n))
				if err != nil {
					return err
				}
				if _, err := io.Copy(hasher, body); err != nil {
					body.Close()
					return apierrors.New(apierrors.DependencyDown, "part read failed").WithRetryable(true).Wrap(err)
				}
				body.Close()
			}
			if got := hex.EncodeToString(hasher.Sum(nil)); got != checksum {
				return apierrors.Newf(apierrors.RequirementUnmet, "whole-file checksum mismatch")
			}
		}
		now := s.now()
		versionID := uuid.New()
		materialID := uuid.New()
		nextRevision := int64(1)
		if sourceVersionID != nil {
			var sourceMaterial uuid.UUID
			var sourceRevision int64
			var currentVersionID *uuid.UUID
			if err := tx.QueryRow(ctx, `
				SELECT material_id, revision FROM material_versions
				WHERE id=$1 AND project_id=$2 AND state='ready'`, *sourceVersionID, projectID).
				Scan(&sourceMaterial, &sourceRevision); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return apierrors.New(apierrors.InvalidReference, "source version not found")
				}
				return apierrors.New(apierrors.Internal, "source lookup failed").Wrap(err)
			}
			if err := tx.QueryRow(ctx, `SELECT current_version_id FROM materials WHERE id=$1`, sourceMaterial).
				Scan(&currentVersionID); err != nil {
				return apierrors.New(apierrors.Internal, "material lookup failed").Wrap(err)
			}
			if currentVersionID == nil || *currentVersionID != *sourceVersionID {
				return apierrors.New(apierrors.SourceVersionConf, "source version is behind current")
			}
			materialID = sourceMaterial
			nextRevision = sourceRevision + 1
			if _, err := tx.Exec(ctx, `
				UPDATE materials SET current_version_id=$2, updated_at=$3 WHERE id=$1`,
				materialID, versionID, now); err != nil {
				return apierrors.New(apierrors.Internal, "material update failed").Wrap(err)
			}
		} else {
			if _, err := tx.Exec(ctx, `
				INSERT INTO materials (project_id, id, title, kind, current_version_id, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$6)`, projectID, materialID, name, kind, versionID, now); err != nil {
				return apierrors.New(apierrors.Internal, "material insert failed").Wrap(err)
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO material_versions (project_id, id, material_id, revision, manifest_key, sha256, size, mime,
				entrypoint, author_id, state, source_manifest_json, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ready',$11,$12)`,
			projectID, versionID, materialID, nextRevision,
			fmt.Sprintf("versions/%s/%s", projectID, versionID),
			checksum, expectedSize, mime, entrypoint, requester,
			fmt.Sprintf(`{"uploadId":%q}`, uploadID.String()), now); err != nil {
			return apierrors.New(apierrors.Internal, "version insert failed").Wrap(err)
		}
		if kind == "html_bundle" {
			if entrypoint == nil {
				return apierrors.Fields("entrypoint", "required")
			}
			if err := s.registerBundle(ctx, tx, projectID, versionID, stagingKey, partCount, *entrypoint); err != nil {
				return err
			}
		} else {
			for n := 1; n <= partCount; n++ {
				if _, err := tx.Exec(ctx, `
				INSERT INTO material_entries (project_id, version_id, relative_path, object_key, size, sha256, mime)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`,
					projectID, versionID, fmt.Sprintf("part-%06d", n), partKey(stagingKey, n),
					partSizes[n], partDigests[n], mime); err != nil {
					return apierrors.New(apierrors.Internal, "entry insert failed").Wrap(err)
				}
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE upload_sessions SET state='completed', material_id=$2, result_version_id=$3 WHERE id=$1`,
			uploadID, materialID, versionID); err != nil {
			return apierrors.New(apierrors.Internal, "session complete failed").Wrap(err)
		}
		if _, err := events.AppendProjectEvent(ctx, tx, projectID, "material.ready", "material", materialID.String(), &nextRevision,
			map[string]any{"versionId": versionID}, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, audit.Entry{
			ProjectID: &projectID, ActorType: audit.ActorUser, ActorUserID: &requester,
			Source: audit.SourceWeb, Operation: "material.complete",
			ObjectType: "material", ObjectID: materialID.String(), OccurredAt: now,
		}); err != nil {
			return err
		}
		out = MaterialVersion{ID: versionID, MaterialID: materialID, Revision: nextRevision,
			State: "ready", SHA256: checksum, Size: expectedSize, Mime: mime, Entrypoint: entrypoint,
			AuthorID: &requester, CreatedAt: now}
		return nil
	})
	return out, err
}

func (s *Service) loadVersionTx(ctx context.Context, tx pgx.Tx, projectID, versionID uuid.UUID, out *MaterialVersion) error {
	return tx.QueryRow(ctx, `
		SELECT id, material_id, revision, state, sha256, size, mime, entrypoint, author_id, created_at
		FROM material_versions WHERE id=$1 AND project_id=$2`, versionID, projectID).
		Scan(&out.ID, &out.MaterialID, &out.Revision, &out.State, &out.SHA256, &out.Size, &out.Mime,
			&out.Entrypoint, &out.AuthorID, &out.CreatedAt)
}

// ListMaterials returns shared materials for the project (04 §3 项目内统一可读).
func (s *Service) ListMaterials(ctx context.Context, requester, projectID uuid.UUID, kind string) ([]Material, error) {
	if err := isMemberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT id, title, kind, visibility, current_version_id, created_at
		/*keys*/ FROM materials WHERE project_id=$1 AND ($2='' OR kind=$2)
		/*page*/`, "created_at", "id", projectID, kind)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "materials failed").Wrap(err)
	}
	defer rows.Close()
	var out []Material
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.Title, &m.Kind, &m.Visibility, &m.CurrentVersionID, &m.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListVersions returns the immutable history of one material.
func (s *Service) ListVersions(ctx context.Context, requester, projectID, materialID uuid.UUID) ([]MaterialVersion, error) {
	if err := isMemberTx(ctx, s.pool, projectID, requester); err != nil {
		return nil, err
	}
	rows, err := paging.Query(ctx, s.pool, `
		SELECT v.id, v.material_id, v.revision, v.state, v.sha256, v.size, v.mime, v.entrypoint, v.author_id, v.created_at
		/*keys*/ FROM material_versions v WHERE v.project_id=$1 AND v.material_id=$2
		/*page*/`, "v.created_at", "v.id", projectID, materialID)
	if err != nil {
		return nil, apierrors.New(apierrors.Internal, "versions failed").Wrap(err)
	}
	defer rows.Close()
	var out []MaterialVersion
	for rows.Next() {
		var v MaterialVersion
		if err := rows.Scan(&v.ID, &v.MaterialID, &v.Revision, &v.State, &v.SHA256, &v.Size, &v.Mime, &v.Entrypoint, &v.AuthorID, &v.CreatedAt); err != nil {
			return nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// OpenVersion loads a version and entries for authorized download proxying.
func (s *Service) OpenVersion(ctx context.Context, requester, projectID, materialID, versionID uuid.UUID) (MaterialVersion, []MaterialEntry, error) {
	if err := isMemberTx(ctx, s.pool, projectID, requester); err != nil {
		return MaterialVersion{}, nil, err
	}
	var v MaterialVersion
	err := s.pool.QueryRow(ctx, `
		SELECT id, material_id, revision, state, sha256, size, mime, entrypoint, author_id, created_at
		FROM material_versions WHERE id=$1 AND material_id=$2 AND project_id=$3`,
		versionID, materialID, projectID).
		Scan(&v.ID, &v.MaterialID, &v.Revision, &v.State, &v.SHA256, &v.Size, &v.Mime, &v.Entrypoint, &v.AuthorID, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MaterialVersion{}, nil, apierrors.New(apierrors.NotFound, "version not found")
	}
	if err != nil {
		return MaterialVersion{}, nil, apierrors.New(apierrors.Internal, "version lookup failed").Wrap(err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT relative_path, size, sha256, mime FROM material_entries WHERE version_id=$1 ORDER BY relative_path`, versionID)
	if err != nil {
		return MaterialVersion{}, nil, apierrors.New(apierrors.Internal, "entries failed").Wrap(err)
	}
	defer rows.Close()
	var entries []MaterialEntry
	for rows.Next() {
		var e MaterialEntry
		if err := rows.Scan(&e.RelativePath, &e.Size, &e.SHA256, &e.Mime); err != nil {
			return MaterialVersion{}, nil, apierrors.New(apierrors.Internal, "scan failed").Wrap(err)
		}
		entries = append(entries, e)
	}
	return v, entries, rows.Err()
}

// VersionObject streams one entry's object through the authorized proxy.
func (s *Service) VersionObject(ctx context.Context, requester, projectID, materialID, versionID uuid.UUID, entryPath string) (io.ReadCloser, MaterialVersion, error) {
	v, entries, err := s.OpenVersion(ctx, requester, projectID, materialID, versionID)
	if err != nil {
		return nil, v, err
	}
	found := ""
	for _, e := range entries {
		if e.RelativePath == entryPath {
			found = e.RelativePath
		}
	}
	if found == "" {
		return nil, v, apierrors.New(apierrors.NotFound, "entry not found")
	}
	var objectKey string
	if err := s.pool.QueryRow(ctx, `
		SELECT object_key FROM material_entries WHERE version_id=$1 AND relative_path=$2`,
		versionID, entryPath).Scan(&objectKey); err != nil {
		return nil, v, apierrors.New(apierrors.Internal, "key lookup failed").Wrap(err)
	}
	body, err := s.store.Get(ctx, objectKey)
	if err != nil {
		return nil, v, err
	}
	return body, v, nil
}
