-- +goose Up
-- M001 账户与技术基础（docs/plans/v1/01 §3）：账户、凭据、会话、CLI 授权、
-- 幂等记录、持久任务与不可变审计。

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email_normalized text NOT NULL,
    email_display text NOT NULL,
    display_name text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    auth_version bigint NOT NULL DEFAULT 1,
    version bigint NOT NULL DEFAULT 1,
    locale text NOT NULL DEFAULT 'zh-CN',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX users_email_normalized_key ON users (email_normalized);

CREATE TABLE password_credentials (
    user_id uuid PRIMARY KEY REFERENCES users (id),
    argon2id_hash text NOT NULL,
    password_changed_at timestamptz NOT NULL
);

CREATE TABLE user_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id),
    secret_hash bytea NOT NULL UNIQUE,
    auth_version bigint NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    revoked_at timestamptz,
    csrf_secret_hash bytea NOT NULL
);
CREATE INDEX user_sessions_expiry_idx ON user_sessions (expires_at);
CREATE INDEX user_sessions_user_idx ON user_sessions (user_id);

CREATE TABLE client_grants (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id),
    device_name text NOT NULL,
    scopes text[] NOT NULL DEFAULT '{}',
    project_scope uuid[] NOT NULL DEFAULT '{}',
    auth_version bigint NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX client_grants_user_idx ON client_grants (user_id);

CREATE TABLE client_tokens (
    id uuid PRIMARY KEY,
    grant_id uuid NOT NULL REFERENCES client_grants (id),
    access_hash bytea NOT NULL UNIQUE,
    access_expires_at timestamptz NOT NULL,
    refresh_hash bytea NOT NULL UNIQUE,
    refresh_family_id uuid NOT NULL,
    refresh_expires_at timestamptz NOT NULL,
    rotated_at timestamptz,
    revoked_at timestamptz
);
CREATE INDEX client_tokens_grant_idx ON client_tokens (grant_id);
CREATE INDEX client_tokens_family_idx ON client_tokens (refresh_family_id);

CREATE TABLE device_authorizations (
    id uuid PRIMARY KEY,
    device_code_hash bytea NOT NULL,
    user_code_hash bytea NOT NULL,
    device_name text NOT NULL,
    requested_scopes text[] NOT NULL DEFAULT '{}',
    requested_project_scope uuid[] NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'consumed')),
    interval_seconds int NOT NULL DEFAULT 5,
    expires_at timestamptz NOT NULL,
    user_id uuid REFERENCES users (id),
    grant_id uuid REFERENCES client_grants (id),
    created_at timestamptz NOT NULL
);
CREATE INDEX device_authorizations_expiry_idx ON device_authorizations (expires_at);

CREATE TABLE recovery_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id),
    code_hash bytea NOT NULL UNIQUE,
    issued_by_operator text NOT NULL,
    reason text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX recovery_tokens_expiry_idx ON recovery_tokens (expires_at);

CREATE TABLE idempotency_records (
    actor_key text NOT NULL,
    operation text NOT NULL,
    key text NOT NULL,
    request_hash bytea NOT NULL,
    status text NOT NULL CHECK (status IN ('in_flight', 'completed', 'failed')),
    response_status int,
    response_body text,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (actor_key, operation, key)
);
CREATE INDEX idempotency_expiry_idx ON idempotency_records (expires_at);

CREATE TABLE background_jobs (
    id uuid PRIMARY KEY,
    kind text NOT NULL,
    payload_version int NOT NULL DEFAULT 1,
    payload jsonb NOT NULL,
    unique_key text,
    state text NOT NULL DEFAULT 'queued'
        CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'dead', 'cancelled')),
    run_after timestamptz NOT NULL,
    attempt int NOT NULL DEFAULT 0,
    max_attempts int NOT NULL DEFAULT 8,
    lease_owner text,
    lease_until timestamptz,
    fencing_token bigint NOT NULL DEFAULT 0,
    last_error_code text,
    last_error text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX background_jobs_unique_key_idx
    ON background_jobs (kind, unique_key) WHERE unique_key IS NOT NULL;
CREATE INDEX background_jobs_claimable_idx
    ON background_jobs (run_after) WHERE state = 'queued';

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    project_id uuid,
    actor_type text NOT NULL CHECK (actor_type IN ('user', 'operator', 'agent', 'system')),
    actor_user_id uuid,
    identity_id uuid,
    binding_version bigint,
    source text NOT NULL CHECK (source IN ('web', 'cli', 'delegate', 'timeout', 'worker', 'operator', 'agent')),
    operation text NOT NULL,
    object_type text NOT NULL,
    object_id text,
    review_id uuid,
    request_id text,
    reason text,
    occurred_at timestamptz NOT NULL
);
CREATE INDEX audit_events_project_idx ON audit_events (project_id, occurred_at DESC);
CREATE INDEX audit_events_object_idx ON audit_events (object_type, object_id);
