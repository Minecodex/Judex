-- +goose Up
-- HTML 预览短时能力（docs/plans/v1/04 §4）：一次性 token，固定版本，
-- TTL 默认 60s；只授权该版本的清单条目。
CREATE TABLE preview_tokens (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    version_id uuid NOT NULL REFERENCES material_versions (id),
    token_hash bytea NOT NULL UNIQUE,
    issued_to uuid NOT NULL REFERENCES users (id),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX preview_tokens_expiry_idx ON preview_tokens (expires_at);
