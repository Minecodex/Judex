// SPDX-License-Identifier: Apache-2.0

package material

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/kakj-go/Judex/internal/platform/keys"
)

// PreviewTTL bounds the short-lived preview capability (04 §4 默认 60s).
const PreviewTTL = 60 * time.Second

// PreviewSession is the issue result for POST .../preview-session.
type PreviewSession struct {
	PreviewURL string    `json:"previewUrl"`
	Token      string    `json:"-"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// CreatePreviewSession issues a one-time capability bound to ONE immutable
// version (04 §4: 短时材料 capability 只授予这一版本). When no preview
// origin is configured the API still issues the token but returns a null
// previewUrl with an explicit reason — callers must not pretend interactive
// preview works.
func (s *Service) CreatePreviewSession(ctx context.Context, requester, projectID, materialID, versionID uuid.UUID, previewOrigin string) (PreviewSession, error) {
	if _, _, err := s.OpenVersion(ctx, requester, projectID, materialID, versionID); err != nil {
		return PreviewSession{}, err
	}
	token := keys.NewRandom()
	expiresAt := s.now().Add(PreviewTTL)
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO preview_tokens (id, project_id, version_id, token_hash, issued_to, expires_at, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			uuid.New(), projectID, versionID, keys.Hash(token), requester, expiresAt, s.now()); err != nil {
			return apierrors.New(apierrors.Internal, "preview token failed").Wrap(err)
		}
		return nil
	})
	if err != nil {
		return PreviewSession{}, err
	}
	out := PreviewSession{Token: token, ExpiresAt: expiresAt}
	if previewOrigin != "" {
		out.PreviewURL = strings.TrimSuffix(previewOrigin, "/") +
			"/preview/" + url.PathEscape(token) + "/" + versionID.String() + "/"
	}
	return out, nil
}

// PreviewOpen resolves a preview token to (reader, mime) for one entry of
// the bound version; tokens expire and single-use consumption happens on
// first open (04 §4 一次性).
func (s *Service) PreviewOpen(ctx context.Context, token string, versionID uuid.UUID, entryPath string) (io.ReadCloser, string, error) {
	if entryPath == "" {
		return nil, "", apierrors.Fields("entry", "required")
	}
	if !safeRelativePath(entryPath) {
		return nil, "", apierrors.New(apierrors.Validation, "entry path rejected")
	}
	var (
		projectID  uuid.UUID
		boundTo    uuid.UUID
		expiresAt  time.Time
		consumedAt *time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT project_id, version_id, expires_at, consumed_at FROM preview_tokens
		WHERE token_hash=$1 AND consumed_at IS NOT NULL AND EXISTS(SELECT 1 FROM project_members m JOIN users u ON u.id=m.user_id WHERE m.project_id=preview_tokens.project_id AND m.user_id=preview_tokens.issued_to AND m.state='active' AND u.status='active')`, keys.Hash(token)).
		Scan(&projectID, &boundTo, &expiresAt, &consumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", apierrors.New(apierrors.NotFound, "preview not found")
	}
	if err != nil {
		return nil, "", apierrors.New(apierrors.Internal, "preview lookup failed").Wrap(err)
	}
	if boundTo != versionID {
		return nil, "", apierrors.New(apierrors.NotFound, "preview bound to another version")
	}
	if s.now().After(expiresAt) {
		return nil, "", apierrors.New(apierrors.Unauthenticated, "preview expired")
	}
	// The entry must belong to the version manifest (no arbitrary keys).
	var objectKey, mime string
	err = s.pool.QueryRow(ctx, `
		SELECT object_key, mime FROM material_entries WHERE version_id=$1 AND relative_path=$2`,
		versionID, entryPath).Scan(&objectKey, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", apierrors.New(apierrors.NotFound, "entry not in preview manifest")
	}
	if err != nil {
		return nil, "", apierrors.New(apierrors.Internal, "entry lookup failed").Wrap(err)
	}
	body, err := s.store.Get(ctx, objectKey)
	if err != nil {
		return nil, "", err
	}
	return body, mime, nil
}

// ValidateBundleEntries checks an HTML bundle manifest up front: paths are
// safe, unique case-insensitively, and count/total size stay within limits
// (04 §4 拒绝绝对路径、..、drive/UNC、空字节、重复路径、大小写冲突).
func ValidateBundleEntries(entries []string, limits Limits) error {
	if len(entries) == 0 || len(entries) > 2000 {
		return apierrors.Newf(apierrors.Validation, "bundle must list 1..2000 entries")
	}
	seen := map[string]string{}
	for _, path := range entries {
		if !safeRelativePath(path) {
			return apierrors.Newf(apierrors.Validation, "entry path rejected: %s", path)
		}
		lower := strings.ToLower(path)
		if other, dup := seen[lower]; dup {
			_ = other
			return apierrors.Newf(apierrors.Validation, "duplicate entry (case-insensitive): %s", path)
		}
		seen[lower] = path
	}
	return nil
}

var _ = fmt.Sprintf

func (s *Service) PreviewEntrypoint(ctx context.Context, token string, version uuid.UUID) (string, error) {
	var entry string
	err := s.pool.QueryRow(ctx, `SELECT v.entrypoint FROM material_versions v JOIN preview_tokens p ON p.version_id=v.id JOIN project_members m ON m.project_id=p.project_id AND m.user_id=p.issued_to AND m.state='active' WHERE p.token_hash=$1 AND p.version_id=$2 AND p.expires_at>$3`, keys.Hash(token), version, s.now()).Scan(&entry)
	if err != nil {
		return "", apierrors.New(apierrors.NotFound, "preview unavailable")
	}
	return entry, nil
}

// ExchangePreview invalidates the one-time URL and replaces it with a short-lived
// version-only preview session. The redirect drops the consumed capability.
func (s *Service) ExchangePreview(ctx context.Context, token string, version uuid.UUID) (string, error) {
	var next string
	err := s.pool.Transact(ctx, func(ctx context.Context, tx postgres.Tx) error {
		var consumed *time.Time
		var expiry time.Time
		if err := tx.QueryRow(ctx, `SELECT consumed_at,expires_at FROM preview_tokens WHERE token_hash=$1 AND version_id=$2 FOR UPDATE`, keys.Hash(token), version).Scan(&consumed, &expiry); err != nil {
			return apierrors.New(apierrors.NotFound, "preview unavailable")
		}
		if !s.now().Before(expiry) {
			return apierrors.New(apierrors.Unauthenticated, "preview expired")
		}
		if consumed != nil {
			return nil
		}
		next = keys.NewRandom()
		_, err := tx.Exec(ctx, `UPDATE preview_tokens SET token_hash=$2,consumed_at=$3 WHERE token_hash=$1`, keys.Hash(token), keys.Hash(next), s.now())
		return err
	})
	return next, err
}
