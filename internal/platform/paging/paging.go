// Package paging implements scoped keyset cursors shared by list queries.
package paging

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type key struct{}
type cursor struct {
	Scope   string    `json:"s"`
	Created time.Time `json:"t"`
	ID      uuid.UUID `json:"i"`
}
type Page struct {
	Limit int
	after *cursor
	scope string
	last  cursor
	more  bool
}

func Parse(ctx context.Context, scope string, query url.Values) (context.Context, error) {
	limit := 50
	if raw := query.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 100 {
			return ctx, apierrors.Fields("limit", "range")
		}
		limit = v
	}
	filters := url.Values{}
	for name, values := range query {
		if name != "cursor" && name != "limit" {
			filters[name] = values
		}
	}
	sum := sha256.Sum256([]byte(scope + "?" + filters.Encode()))
	p := &Page{Limit: limit, scope: hex.EncodeToString(sum[:])}
	if raw := query.Get("cursor"); raw != "" {
		if len(raw) > 1024 {
			return ctx, apierrors.Fields("cursor", "invalid")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return ctx, apierrors.Fields("cursor", "invalid")
		}
		var after cursor
		if err = json.Unmarshal(decoded, &after); err != nil || after.Scope != p.scope || after.Created.IsZero() || after.ID == uuid.Nil {
			return ctx, apierrors.Fields("cursor", "scope or format")
		}
		p.after = &after
	}
	return context.WithValue(ctx, key{}, p), nil
}
func Next(ctx context.Context) *string {
	p, _ := ctx.Value(key{}).(*Page)
	if p == nil || !p.more {
		return nil
	}
	raw, _ := json.Marshal(p.last)
	next := base64.RawURLEncoding.EncodeToString(raw)
	return &next
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// Call sites place explicit markers in the SELECT and WHERE clauses. Columns
// are repository constants, never user input. Scan metadata stays out of DTOs.
func Query(ctx context.Context, q queryer, sql, timeColumn, idColumn string, args ...any) (pgx.Rows, error) {
	p, _ := ctx.Value(key{}).(*Page)
	if p == nil {
		p = &Page{Limit: 100}
	}
	var afterTime *time.Time
	var afterID *uuid.UUID
	if p.after != nil {
		afterTime = &p.after.Created
		afterID = &p.after.ID
	}
	n := len(args) + 1
	condition := fmt.Sprintf(" AND ($%d::timestamptz IS NULL OR (%s,%s)<($%d,$%d::uuid)) ", n, timeColumn, idColumn, n, n+1)
	if !strings.Contains(sql, "/*page*/") || !strings.Contains(sql, "/*keys*/") {
		return nil, fmt.Errorf("list query missing paging markers")
	}
	sql = strings.ReplaceAll(sql, "/*keys*/", ", "+timeColumn+", "+idColumn)
	sql = strings.ReplaceAll(sql, "/*page*/", condition) + fmt.Sprintf(" ORDER BY %s DESC,%s DESC LIMIT $%d", timeColumn, idColumn, n+2)
	args = append(args, afterTime, afterID, p.Limit+1)
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &pageRows{Rows: rows, page: p}, nil
}

type pageRows struct {
	pgx.Rows
	page  *Page
	count int
}

func (r *pageRows) Next() bool {
	if !r.Rows.Next() {
		return false
	}
	if r.count >= r.page.Limit {
		r.page.more = true
		r.Rows.Close()
		return false
	}
	r.count++
	return true
}
func (r *pageRows) Scan(dest ...any) error {
	var created time.Time
	var id uuid.UUID
	err := r.Rows.Scan(append(dest, &created, &id)...)
	if err == nil {
		r.page.last = cursor{Scope: r.page.scope, Created: created, ID: id}
	}
	return err
}

func Params(ctx context.Context) (int, *time.Time, *uuid.UUID) {
	p, _ := ctx.Value(key{}).(*Page)
	if p == nil {
		return 50, nil, nil
	}
	if p.after == nil {
		return p.Limit, nil, nil
	}
	return p.Limit, &p.after.Created, &p.after.ID
}
func SetNext(ctx context.Context, created time.Time, id uuid.UUID) {
	if p, ok := ctx.Value(key{}).(*Page); ok {
		p.last = cursor{Scope: p.scope, Created: created, ID: id}
		p.more = true
	}
}

type SequenceCursor struct {
	Scope string `json:"s"`
	Seq   int64  `json:"q"`
	After bool   `json:"a"`
}

func Sequence(scope, raw string) (SequenceCursor, error) {
	var value SequenceCursor
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(raw) > 1024 {
		return value, apierrors.Fields("cursor", "invalid")
	}
	if err = json.Unmarshal(data, &value); err != nil || value.Scope != scope || value.Seq < 1 {
		return value, apierrors.Fields("cursor", "scope or format")
	}
	return value, nil
}
func NextSequence(scope string, seq int64, after bool) *string {
	raw, _ := json.Marshal(SequenceCursor{Scope: scope, Seq: seq, After: after})
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return &encoded
}
