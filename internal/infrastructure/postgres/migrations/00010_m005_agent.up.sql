-- +goose Up
-- M005 Agent 执行（docs/plans/v1/01 §3）：会话/运行/模型调用/工具调用/
-- 运行事件/检查点/讨论批次与轮次/知识候选/沙箱租约。

CREATE TABLE agent_sessions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    topic_id uuid REFERENCES topics (id),
    identity_id uuid NOT NULL,
    last_consumed_seq bigint NOT NULL DEFAULT 0,
    lease_epoch bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX agent_sessions_identity_topic ON agent_sessions (topic_id, identity_id);

CREATE TABLE agent_runs (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES agent_sessions (id),
    batch_id uuid,
    round_id uuid,
    parent_run_id uuid,
    identity_id uuid NOT NULL,
    binding_version bigint,
    state text NOT NULL DEFAULT 'queued' CHECK (state IN
        ('queued','provisioning','running','waiting_children','waiting_human','waiting_material',
         'succeeded','failed','cancelled')),
    manifest jsonb NOT NULL DEFAULT '{}',
    budget_snapshot jsonb NOT NULL DEFAULT '{}',
    sandbox_id text,
    lease_token text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX agent_runs_session_idx ON agent_runs (session_id);
CREATE INDEX agent_runs_state_idx ON agent_runs (state);

CREATE TABLE model_calls (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES agent_runs (id),
    attempt int NOT NULL DEFAULT 1,
    request_manifest_hash text NOT NULL DEFAULT '',
    provider_request_id text,
    state text NOT NULL DEFAULT 'running' CHECK (state IN ('running','succeeded','failed','cancelled')),
    usage_json jsonb NOT NULL DEFAULT '{}',
    usage_known boolean NOT NULL DEFAULT false,
    error_code text,
    started_at timestamptz NOT NULL,
    ended_at timestamptz
);

CREATE TABLE tool_calls (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES agent_runs (id),
    tool_call_id text NOT NULL,
    name text NOT NULL,
    schema_version int NOT NULL DEFAULT 1,
    args_hash text NOT NULL DEFAULT '',
    effect_class text NOT NULL,
    state text NOT NULL DEFAULT 'prepared' CHECK (state IN ('prepared','running','succeeded','failed','unknown')),
    output_ref text,
    fencing_token bigint NOT NULL DEFAULT 0,
    started_at timestamptz,
    ended_at timestamptz,
    UNIQUE (run_id, tool_call_id)
);

CREATE TABLE run_events (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES agent_runs (id),
    seq bigint NOT NULL,
    visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','private')),
    type text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    UNIQUE (run_id, seq)
);

CREATE TABLE context_checkpoints (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES agent_sessions (id),
    run_id uuid REFERENCES agent_runs (id),
    covered_seq bigint NOT NULL,
    manifest jsonb NOT NULL DEFAULT '{}',
    summary text NOT NULL DEFAULT '',
    unresolved_json jsonb NOT NULL DEFAULT '[]',
    pending_children_json jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL
);

CREATE TABLE discussion_batches (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    topic_id uuid NOT NULL REFERENCES topics (id),
    source_submission_id uuid,
    policy_snapshot jsonb NOT NULL DEFAULT '{}',
    max_rounds int NOT NULL DEFAULT 3,
    rounds_reserved int NOT NULL DEFAULT 0,
    state text NOT NULL DEFAULT 'queued' CHECK (state IN
        ('queued','running','waiting_human','waiting_material','limit_reached','completed','failed','cancelled')),
    trigger_kind text NOT NULL DEFAULT 'submission',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (source_submission_id, trigger_kind)
);

CREATE TABLE discussion_rounds (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    batch_id uuid NOT NULL REFERENCES discussion_batches (id),
    ordinal int NOT NULL,
    state text NOT NULL DEFAULT 'reserved' CHECK (state IN ('reserved','running','done','failed')),
    coordinator_run_id uuid,
    required_identity_ids uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    UNIQUE (batch_id, ordinal)
);

CREATE TABLE knowledge_candidates (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    text text NOT NULL,
    source_refs jsonb NOT NULL DEFAULT '[]',
    state text NOT NULL DEFAULT 'unverified' CHECK (state IN ('unverified','confirmed','retracted')),
    applicability text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);

CREATE TABLE sandbox_leases (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES agent_runs (id),
    external_id text NOT NULL,
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active','released','cleanup_failed')),
    expires_at timestamptz NOT NULL,
    mount_version_manifest jsonb NOT NULL DEFAULT '{}',
    cleanup_state text NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL
);
