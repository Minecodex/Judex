package collab

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/platform/auth"
	"github.com/kakj-go/Judex/internal/platform/keys"
	"time"
)

func (s *Service) PreviewSession(ctx context.Context, conversation, version uuid.UUID) (string, time.Time, error) {
	if _, e := s.Access(ctx, conversation); e != nil {
		return "", time.Time{}, e
	}
	if e := s.fileAccess(ctx, conversation, version); e != nil {
		return "", time.Time{}, e
	}
	p, e := actor(ctx)
	if e != nil {
		return "", time.Time{}, e
	}
	if !auth.Live(ctx, s.Pool, p) {
		return "", time.Time{}, missing()
	}
	token := keys.NewRandom()
	expires := time.Now().UTC().Add(10 * time.Minute)
	copy := *p
	copy.CSRFToken = ""
	raw, _ := json.Marshal(copy)
	_, e = s.Pool.Exec(ctx, `INSERT INTO file_preview_sessions(token_hash,principal,conversation_id,version_id,expires_at) VALUES($1,$2,$3,$4,$5)`, keys.Hash(token), raw, conversation, version, expires)
	return token, expires, e
}
func (s *Service) PreviewEntry(ctx context.Context, token string, version uuid.UUID, entry string) (FileContent, error) {
	out := FileContent{}
	var principal auth.Principal
	var conversation uuid.UUID
	e := s.Pool.QueryRow(ctx, `SELECT principal,conversation_id FROM file_preview_sessions WHERE token_hash=$1 AND version_id=$2 AND expires_at>now()`, keys.Hash(token), version).Scan(&principal, &conversation)
	if e != nil {
		return out, missing()
	}
	if !auth.Live(ctx, s.Pool, &principal) {
		return out, missing()
	}
	ctx = auth.WithPrincipal(ctx, &principal)
	if entry == "" {
		entry = "index.html"
	}
	var raw bool
	if e = s.Pool.QueryRow(ctx, `SELECT COALESCE((preview_data->>'raw')::boolean,false) FROM file_versions WHERE id=$1`, version).Scan(&raw); e != nil {
		return out, e
	}
	if raw && entry == "index.html" {
		return s.Content(ctx, conversation, version, false)
	}
	return s.Entry(ctx, conversation, version, entry)
}
