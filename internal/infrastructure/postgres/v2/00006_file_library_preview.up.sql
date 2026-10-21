-- +goose Up
ALTER TABLE file_grants ADD COLUMN listed boolean NOT NULL DEFAULT true, ADD COLUMN version bigint NOT NULL DEFAULT 1;
CREATE TABLE file_preview_sessions (
 token_hash bytea PRIMARY KEY, principal jsonb NOT NULL, conversation_id uuid NOT NULL REFERENCES conversations(id),
 version_id uuid NOT NULL REFERENCES file_versions(id), expires_at timestamptz NOT NULL
);
