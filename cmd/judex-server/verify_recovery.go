package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/config"
	"github.com/kakj-go/Judex/internal/infrastructure/objectstore"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

// Run against a quiesced source or isolated restore before starting workers.
// Output contains counts and hashes only, never credentials or business text.
func verifyRecovery() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := postgres.Open(ctx, postgres.Options{URL: cfg.DatabaseURL}, nil)
	if err != nil {
		return err
	}
	defer pool.Close()
	store, err := objectstore.New(ctx, objectstore.Options{Endpoint: cfg.ObjectStorage.Endpoint, Region: cfg.ObjectStorage.Region, AccessKeyID: cfg.ObjectStorage.AccessKeyID, SecretAccessKey: cfg.ObjectStorage.SecretAccessKey, Bucket: cfg.ObjectStorage.Bucket, PathStyle: cfg.ObjectStorage.PathStyle})
	if err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename<>'goose_db_version' ORDER BY tablename`)
	if err != nil {
		return err
	}
	tables := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		tables = append(tables, name)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	type digest struct {
		Rows   int    `json:"rows"`
		SHA256 string `json:"sha256"`
	}
	result := map[string]digest{}
	for _, table := range tables {
		rows, err = tx.Query(ctx, `SELECT to_jsonb(t)::text FROM `+pgx.Identifier{table}.Sanitize()+` t ORDER BY to_jsonb(t)::text`)
		if err != nil {
			return err
		}
		hash := sha256.New()
		count := 0
		for rows.Next() {
			var row string
			if err = rows.Scan(&row); err != nil {
				rows.Close()
				return err
			}
			fmt.Fprintln(hash, row)
			count++
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return err
		}
		result[table] = digest{Rows: count, SHA256: hex.EncodeToString(hash.Sum(nil))}
	}
	rows, err = tx.Query(ctx, `SELECT e.object_key,e.size,e.sha256 FROM material_entries e JOIN material_versions v ON v.id=e.version_id WHERE v.state='ready' ORDER BY e.object_key`)
	if err != nil {
		return err
	}
	verified := 0
	for rows.Next() {
		var key, expected string
		var size int64
		if err = rows.Scan(&key, &size, &expected); err != nil {
			rows.Close()
			return err
		}
		body, e := store.Get(ctx, key)
		if e != nil {
			rows.Close()
			return fmt.Errorf("material object unavailable: %w", e)
		}
		h := sha256.New()
		n, e := io.Copy(h, body)
		body.Close()
		if e != nil {
			rows.Close()
			return e
		}
		if n != size || hex.EncodeToString(h.Sum(nil)) != expected {
			rows.Close()
			return fmt.Errorf("material object checksum mismatch")
		}
		verified++
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"format": 1, "tables": result, "verifiedMaterialEntries": verified})
}
