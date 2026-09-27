-- +goose Up
-- M003 讨论部分（docs/plans/v1/01 §3、04 §1）：统一提交、消息与站内提醒。

CREATE TABLE submissions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    actor_user_id uuid REFERENCES users (id),
    identity_id uuid,
    source text NOT NULL CHECK (source IN ('web', 'cli', 'agent')),
    client_submission_id text NOT NULL,
    text text NOT NULL DEFAULT '',
    topic_id uuid REFERENCES topics (id),
    task_id uuid,
    purpose text NOT NULL CHECK (purpose IN ('message', 'progress', 'delivery', 'material')),
    status text NOT NULL DEFAULT 'ready' CHECK (status IN ('preparing', 'ready', 'failed')),
    payload_hash text NOT NULL DEFAULT '',
    expected_task_version bigint,
    code_refs_json jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL,
    UNIQUE (project_id, actor_user_id, client_submission_id)
);

CREATE TABLE messages (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    topic_id uuid NOT NULL REFERENCES topics (id),
    seq bigint NOT NULL,
    kind text NOT NULL CHECK (kind IN ('human', 'agent', 'system')),
    author_user_id uuid REFERENCES users (id),
    identity_id uuid,
    submission_id uuid REFERENCES submissions (id),
    run_id uuid,
    content text NOT NULL,
    state text NOT NULL DEFAULT 'committed' CHECK (state IN ('committed', 'superseded')),
    created_at timestamptz NOT NULL,
    UNIQUE (topic_id, seq)
);

CREATE TABLE notifications (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id),
    type text NOT NULL,
    object_ref jsonb NOT NULL DEFAULT '{}',
    event_id uuid NOT NULL,
    read_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, event_id, type)
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);
