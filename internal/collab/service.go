// Package collab owns V2 projects, conversations and their shared authorization boundary.
package collab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"github.com/kakj-go/Judex/internal/platform/auth"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
type Service struct {
	Pool         *postgres.Pool
	Objects      ObjectStore
	ConverterURL string
}
type Versioned struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}
type GroupAccess struct {
	ID        uuid.UUID
	Kind      string
	ProjectID *uuid.UUID
}
type Mention struct {
	Kind             string    `json:"kind"`
	TargetID         uuid.UUID `json:"targetId"`
	RequiresResponse bool      `json:"requiresResponse"`
}
type MessageInput struct {
	suppressTriggers bool
	origin           json.RawMessage
	Body             string        `json:"body"`
	ReplyTo          *uuid.UUID    `json:"replyTo,omitempty"`
	Mentions         []Mention     `json:"mentions"`
	FileVersionIDs   []uuid.UUID   `json:"fileVersionIds"`
	RecordOnly       bool          `json:"recordOnly"`
	Confirmation     *Confirmation `json:"confirmation,omitempty"`
	Decision         *Decision     `json:"decision,omitempty"`
}
type GroupInput struct {
	Name            string        `json:"name"`
	Purpose         string        `json:"purpose"`
	ParentID        *uuid.UUID    `json:"parentId,omitempty"`
	WorkflowIDs     []uuid.UUID   `json:"workflowIds"`
	Members         []MemberRoles `json:"members"`
	ExpectedVersion int64         `json:"expectedVersion"`
}
type MemberRoles struct {
	UserID      uuid.UUID   `json:"userId"`
	PositionIDs []uuid.UUID `json:"positionIds"`
}
type ProjectInput struct {
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	DefaultModelID  *uuid.UUID `json:"defaultModelId"`
	LoopLimit       int        `json:"loopLimit"`
	ExpectedVersion int64      `json:"expectedVersion"`
}

func actor(ctx context.Context) (*auth.Principal, error) {
	p := auth.FromContext(ctx)
	if p == nil {
		return nil, apierrors.New(apierrors.Unauthenticated, "authentication required")
	}
	return p, nil
}
func missing() error { return apierrors.New(apierrors.NotFound, "resource not found") }
func conflict() error {
	return apierrors.New(apierrors.VersionConflict, "resource changed; reload and retry")
}
func validText(value, field string, max int, required bool) error {
	if len(value) > max || (required && strings.TrimSpace(value) == "") {
		return apierrors.Fields(field, "length")
	}
	return nil
}
func nonnil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
func (s *Service) write(ctx context.Context, fn func(context.Context) error) error {
	return s.Pool.Transact(ctx, func(ctx context.Context, _ postgres.Tx) error { return fn(ctx) })
}
func (s *Service) list(ctx context.Context, query string, args ...any) ([]json.RawMessage, error) {
	rows, err := s.Pool.Query(ctx, "SELECT to_jsonb(q) FROM ("+query+") q", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}
func (s *Service) one(ctx context.Context, query string, args ...any) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.Pool.QueryRow(ctx, "SELECT to_jsonb(q) FROM ("+query+") q", args...).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, missing()
	}
	return out, err
}

// Project membership is always checked even when the caller has a valid CLI grant.
func (s *Service) projectAccess(ctx context.Context, id uuid.UUID, manage bool) error {
	p, err := actor(ctx)
	if err != nil {
		return err
	}
	if !p.InProjectScope(id) {
		return missing()
	}
	var role string
	err = s.Pool.QueryRow(ctx, `SELECT m.role FROM projects p JOIN project_members m ON m.project_id=p.id JOIN users u ON u.id=m.user_id
 WHERE p.id=$1 AND m.user_id=$2 AND m.state='active' AND u.status='active' FOR SHARE OF p,m`, id, p.UserID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return missing()
	}
	if err != nil {
		return err
	}
	if manage && role != "owner" && role != "admin" {
		return apierrors.New(apierrors.Forbidden, "project manager required")
	}
	return nil
}
func (s *Service) Access(ctx context.Context, id uuid.UUID) (GroupAccess, error) {
	out := GroupAccess{ID: id}
	p, err := actor(ctx)
	if err != nil {
		return out, err
	}
	err = s.Pool.QueryRow(ctx, `SELECT c.kind,g.project_id FROM conversations c LEFT JOIN groups g ON g.id=c.id WHERE c.id=$1`, id).Scan(&out.Kind, &out.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, missing()
	}
	if err != nil {
		return out, err
	}
	if out.ProjectID != nil {
		if err = s.projectAccess(ctx, *out.ProjectID, false); err != nil {
			return out, err
		}
	}
	var active bool
	err = s.Pool.QueryRow(ctx, `SELECT active FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 FOR SHARE`, id, p.UserID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || !active {
		return out, missing()
	}
	return out, err
}
func (s *Service) lockConversation(ctx context.Context, id uuid.UUID) error {
	var ignored uuid.UUID
	return s.Pool.QueryRow(ctx, `SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, id).Scan(&ignored)
}

func (s *Service) lockConversations(ctx context.Context, ids []uuid.UUID) error {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		if e := s.lockConversation(ctx, id); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) requireHuman(ctx context.Context) error {
	p, err := actor(ctx)
	if err != nil {
		return err
	}
	if p.Kind != auth.KindWeb {
		return apierrors.New(apierrors.Forbidden, "browser session required")
	}
	return nil
}
