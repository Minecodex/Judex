// Package accountimport copies only account credentials and valid Web sessions
// into an empty V2 database, checking every imported value before commit.
package accountimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Record struct {
	Table  string `json:"table"`
	Rows   int64  `json:"rows"`
	SHA256 string `json:"sha256"`
}
type Report struct {
	At     time.Time `json:"at"`
	Tables []Record  `json:"tables"`
}

func Import(ctx context.Context, sourceURL, targetURL string) (Report, error) {
	report := Report{At: time.Now().UTC(), Tables: []Record{}}
	source, err := pgx.Connect(ctx, sourceURL)
	if err != nil {
		return report, err
	}
	defer source.Close(ctx)
	target, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		return report, err
	}
	defer target.Close(ctx)
	var sourceName, targetName string
	_ = source.QueryRow(ctx, `SELECT current_database()||':'||inet_server_addr()::text||':'||inet_server_port()`).Scan(&sourceName)
	_ = target.QueryRow(ctx, `SELECT current_database()||':'||inet_server_addr()::text||':'||inet_server_port()`).Scan(&targetName)
	if sourceName == targetName {
		return report, fmt.Errorf("source and target must be distinct databases")
	}
	src, err := source.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return report, err
	}
	defer src.Rollback(ctx)
	dst, err := target.Begin(ctx)
	if err != nil {
		return report, err
	}
	defer dst.Rollback(ctx)
	var schema, count int
	if err = dst.QueryRow(ctx, `SELECT version FROM judex_schema`).Scan(&schema); err != nil || schema != 2 {
		return report, fmt.Errorf("target must have the clean V2 schema")
	}
	if err = dst.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return report, err
	}
	if count != 0 {
		return report, fmt.Errorf("target contains accounts; refusing to overwrite")
	}
	tables := []struct{ name, key, where string }{{"users", "id", "true"}, {"password_credentials", "user_id", "true"}, {"user_sessions", "id", `revoked_at IS NULL AND expires_at>$1 AND EXISTS(SELECT 1 FROM users u WHERE u.id=t.user_id AND u.status='active' AND u.auth_version=t.auth_version)`}, {"recovery_tokens", "id", "true"}}
	for _, table := range tables {
		args := []any{}
		if table.name == "user_sessions" {
			args = append(args, report.At)
		}
		rows, err := src.Query(ctx, `SELECT * FROM `+table.name+` t WHERE `+table.where+` ORDER BY `+table.key, args...)
		if err != nil {
			return report, err
		}
		columns := []string{}
		for _, f := range rows.FieldDescriptions() {
			columns = append(columns, f.Name)
		}
		values := [][]any{}
		for rows.Next() {
			v, e := rows.Values()
			if e != nil {
				rows.Close()
				return report, e
			}
			values = append(values, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return report, err
		}
		n, err := dst.CopyFrom(ctx, pgx.Identifier{table.name}, columns, pgx.CopyFromRows(values))
		if err != nil {
			return report, err
		}
		digest := func(tx pgx.Tx, filter string, args ...any) (string, error) {
			rows, e := tx.Query(ctx, `SELECT row_to_json(t)::text FROM `+table.name+` t WHERE `+filter+` ORDER BY `+table.key, args...)
			if e != nil {
				return "", e
			}
			defer rows.Close()
			hash := sha256.New()
			for rows.Next() {
				var line string
				if e = rows.Scan(&line); e != nil {
					return "", e
				}
				_, _ = hash.Write([]byte(line + "\n"))
			}
			return hex.EncodeToString(hash.Sum(nil)), rows.Err()
		}
		original, err := digest(src, table.where, args...)
		if err != nil {
			return report, err
		}
		copied, err := digest(dst, "true")
		if err != nil {
			return report, err
		}
		if original != copied {
			return report, fmt.Errorf("%s byte-equivalent check failed; transaction rolled back", table.name)
		}
		report.Tables = append(report.Tables, Record{Table: table.name, Rows: n, SHA256: copied})
	}
	if err = dst.Commit(ctx); err != nil {
		return report, err
	}
	return report, nil
}
