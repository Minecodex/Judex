package paging

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

// QueryText groups by a constant text column and keeps chronological cursors
// inside each group. The original cursor binds filters and ordering together.
func QueryText(ctx context.Context, q queryer, sql, textColumn, timeColumn, idColumn string, args ...any) (pgx.Rows, error) {
	p, _ := ctx.Value(key{}).(*Page)
	if p == nil {
		p = &Page{Limit: 100}
	}
	var afterTime *time.Time
	var afterID *uuid.UUID
	var afterText *string
	if p.after != nil {
		afterTime = &p.after.Created
		afterID = &p.after.ID
		afterText = &p.after.Text
	}
	n := len(args) + 1
	if !strings.Contains(sql, "/*keys*/") || !strings.Contains(sql, "/*page*/") {
		return nil, fmt.Errorf("list query missing paging markers")
	}
	condition := fmt.Sprintf(" AND ($%d::timestamptz IS NULL OR %s>$%d OR (%s=$%d AND (%s,%s)<($%d,$%d::uuid))) ", n, textColumn, n+2, textColumn, n+2, timeColumn, idColumn, n, n+1)
	sql = strings.ReplaceAll(sql, "/*keys*/", ", "+timeColumn+", "+idColumn+", "+textColumn)
	sql = strings.ReplaceAll(sql, "/*page*/", condition) + fmt.Sprintf(" ORDER BY %s ASC,%s DESC,%s DESC LIMIT $%d", textColumn, timeColumn, idColumn, n+3)
	rows, err := q.Query(ctx, sql, append(args, afterTime, afterID, afterText, p.Limit+1)...)
	if err != nil {
		return nil, err
	}
	return &pageRows{Rows: rows, page: p, textOrder: true}, nil
}
