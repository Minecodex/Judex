-- +goose Up
ALTER TABLE context_snapshots ADD COLUMN source_seq bigint NOT NULL DEFAULT 0;
