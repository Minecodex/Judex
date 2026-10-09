-- +goose Up
-- 事件基础（docs/plans/v1/01 §5、04 §5）：项目顺序事件、outbox 与承载
-- event_seq 的 projects 表。projects 完整列按 M002 合同（01 §3）；其余
-- M002 表（成员/邀请/职位等）由后续迁移补充。

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    kind text NOT NULL DEFAULT 'software' CHECK (kind IN ('software', 'design')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    creator_user_id uuid NOT NULL,
    owner_user_id uuid NOT NULL,
    event_seq bigint NOT NULL DEFAULT 0,
    max_discussion_rounds int NOT NULL DEFAULT 3 CHECK (max_discussion_rounds BETWEEN 1 AND 100),
    approval_timeout_seconds int NOT NULL DEFAULT 86400 CHECK (approval_timeout_seconds >= 60),
    default_model_id uuid,
    sandbox_profile_id text,
    version bigint NOT NULL DEFAULT 1,
    archived_at timestamptz,
    archived_reason text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE project_events (
    project_id uuid NOT NULL REFERENCES projects (id),
    seq bigint NOT NULL,
    event_id uuid NOT NULL UNIQUE,
    type text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL,
    version bigint,
    payload jsonb NOT NULL DEFAULT '{}',
    occurred_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, seq)
);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    project_seq bigint,
    kind text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL,
    delivered_at timestamptz
);
CREATE INDEX outbox_events_pending_idx ON outbox_events (created_at) WHERE delivered_at IS NULL;
