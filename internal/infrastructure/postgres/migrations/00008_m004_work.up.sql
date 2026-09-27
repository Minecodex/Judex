-- +goose Up
-- M004 工作、决定与交接（docs/plans/v1/01 §3 M004 全表）。

CREATE TABLE plans (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    title text NOT NULL,
    goal text NOT NULL DEFAULT '',
    acceptance_criteria text NOT NULL DEFAULT '',
    owner_identity_id uuid,
    workflow_id uuid,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'accepted', 'cancelled')),
    latest_acceptance_id uuid,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE tasks (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    plan_id uuid REFERENCES plans (id),
    parent_task_id uuid REFERENCES tasks (id),
    title text NOT NULL,
    expected_output text NOT NULL DEFAULT '',
    acceptance_criteria text NOT NULL DEFAULT '',
    kind text NOT NULL DEFAULT 'task' CHECK (kind IN ('task', 'bug')),
    reviewer_identity_id uuid,
    workflow_id uuid,
    node_id text,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ready', 'working', 'delivered', 'accepted', 'rework', 'cancelled')),
    latest_report_id uuid,
    latest_acceptance_id uuid,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX tasks_project_status_idx ON tasks (project_id, status);
CREATE INDEX tasks_plan_idx ON tasks (plan_id);

CREATE TABLE task_participants (
    project_id uuid NOT NULL REFERENCES projects (id),
    task_id uuid NOT NULL REFERENCES tasks (id),
    identity_id uuid NOT NULL,
    responsibility text NOT NULL DEFAULT '',
    PRIMARY KEY (task_id, identity_id)
);

CREATE TABLE task_requirements (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    task_id uuid NOT NULL REFERENCES tasks (id),
    phase text NOT NULL CHECK (phase IN ('start', 'accept', 'both')),
    kind text NOT NULL CHECK (kind IN ('task_acceptance', 'handoff_receipt', 'material_ready')),
    target_id uuid NOT NULL,
    material_version_id uuid,
    hard boolean NOT NULL DEFAULT true,
    label text NOT NULL DEFAULT ''
);
CREATE INDEX task_requirements_task_idx ON task_requirements (task_id);

CREATE TABLE plan_task_references (
    project_id uuid NOT NULL REFERENCES projects (id),
    plan_id uuid NOT NULL REFERENCES plans (id),
    task_id uuid NOT NULL REFERENCES tasks (id),
    PRIMARY KEY (plan_id, task_id)
);

CREATE TABLE work_reports (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    task_id uuid NOT NULL REFERENCES tasks (id),
    identity_id uuid NOT NULL,
    binding_version bigint NOT NULL DEFAULT 1,
    submission_id uuid,
    report_kind text NOT NULL CHECK (report_kind IN ('progress', 'delivery')),
    progress_hint text,
    code_refs_json jsonb NOT NULL DEFAULT '[]',
    environment_refs_json jsonb NOT NULL DEFAULT '[]',
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL,
    UNIQUE (task_id, identity_id, submission_id)
);

CREATE TABLE proposals (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    topic_id uuid,
    kind text NOT NULL CHECK (kind IN ('work_arrangement', 'work_change', 'dependency_change',
        'material_link', 'topic_creation', 'formal_conclusion', 'handoff_change', 'release_acceptance')),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending', 'approved', 'cancelled', 'stale')),
    current_review_id uuid,
    previous_proposal_id uuid,
    reason text,
    applied_event_seq bigint,
    version bigint NOT NULL DEFAULT 1,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE proposal_versions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    proposal_id uuid NOT NULL REFERENCES proposals (id),
    revision bigint NOT NULL,
    payload_schema_version int NOT NULL DEFAULT 1,
    changes_json jsonb NOT NULL DEFAULT '[]',
    review_manifest_json jsonb,
    review_hash text,
    sender_user_id uuid,
    submitted_at timestamptz,
    timeout_seconds_snapshot int,
    first_approval_at timestamptz,
    deadline_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (proposal_id, revision)
);

CREATE TABLE approval_slots (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    review_id uuid NOT NULL,
    authority_type text NOT NULL CHECK (authority_type IN ('identity', 'user')),
    authority_id uuid NOT NULL,
    initial_user_id uuid,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'approved', 'rejected', 'timeout', 'delegated')),
    UNIQUE (review_id, authority_type, authority_id)
);

CREATE TABLE approval_decisions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    review_id uuid NOT NULL,
    decision text NOT NULL CHECK (decision IN ('approve', 'reject')),
    actor_user_id uuid REFERENCES users (id),
    acting_bindings_json jsonb NOT NULL DEFAULT '[]',
    decision_source text NOT NULL CHECK (decision_source IN ('human_web', 'human_cli', 'delegate', 'timeout')),
    intent_id uuid,
    reason text,
    decided_at timestamptz NOT NULL
);

CREATE TABLE approval_decision_slots (
    decision_id uuid NOT NULL REFERENCES approval_decisions (id),
    slot_id uuid NOT NULL REFERENCES approval_slots (id),
    PRIMARY KEY (decision_id, slot_id)
);

CREATE TABLE confirmation_intents (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id),
    grant_id uuid,
    operation text NOT NULL,
    object_id uuid NOT NULL,
    review_hash text NOT NULL,
    canonical_payload_hash text NOT NULL,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'committed', 'rejected', 'expired', 'stale')),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    result_ref text,
    nonce text NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE handoffs (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    target_task_id uuid NOT NULL REFERENCES tasks (id),
    receiver_identity_id uuid NOT NULL,
    workflow_id uuid,
    kind text NOT NULL CHECK (kind IN ('dependency', 'stage')),
    title text NOT NULL DEFAULT '',
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE handoff_sources (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    handoff_id uuid NOT NULL REFERENCES handoffs (id),
    source_task_id uuid NOT NULL REFERENCES tasks (id),
    sender_identity_id uuid NOT NULL,
    current_source_version_id uuid
);

CREATE TABLE source_versions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    source_id uuid NOT NULL REFERENCES handoff_sources (id),
    revision bigint NOT NULL,
    report_id uuid,
    summary text NOT NULL DEFAULT '',
    evidence_manifest jsonb NOT NULL DEFAULT '{}',
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'pending', 'accepted', 'rejected', 'stale')),
    sent_by uuid REFERENCES users (id),
    sent_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (source_id, revision)
);

CREATE TABLE source_decisions (
    id uuid PRIMARY KEY,
    source_version_id uuid NOT NULL REFERENCES source_versions (id),
    actor_user_id uuid NOT NULL REFERENCES users (id),
    decision text NOT NULL CHECK (decision IN ('accept', 'reject')),
    reason text,
    intent_id uuid,
    created_at timestamptz NOT NULL
);

CREATE TABLE task_acceptances (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    task_id uuid NOT NULL REFERENCES tasks (id),
    task_version bigint NOT NULL,
    reviewer_identity_id uuid,
    actor_user_id uuid NOT NULL REFERENCES users (id),
    review_manifest jsonb NOT NULL DEFAULT '{}',
    evidence_hash text NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE plan_acceptances (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    plan_id uuid NOT NULL REFERENCES plans (id),
    plan_version bigint NOT NULL,
    owner_identity_id uuid,
    actor_user_id uuid NOT NULL REFERENCES users (id),
    task_acceptance_refs jsonb NOT NULL DEFAULT '[]',
    criteria_snapshot text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);

CREATE TABLE reopen_records (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    target_type text NOT NULL CHECK (target_type IN ('task', 'plan')),
    target_id uuid NOT NULL,
    previous_acceptance_id uuid NOT NULL,
    reason text NOT NULL,
    actor_user_id uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL
);

CREATE TABLE bug_details (
    project_id uuid NOT NULL REFERENCES projects (id),
    task_id uuid PRIMARY KEY REFERENCES tasks (id),
    source_task_id uuid REFERENCES tasks (id),
    observed_release_ref text,
    environment text NOT NULL DEFAULT '',
    steps text NOT NULL DEFAULT '',
    expected text NOT NULL DEFAULT '',
    actual text NOT NULL DEFAULT '',
    severity text NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

CREATE TABLE release_reports (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    version_label text NOT NULL,
    environment text NOT NULL,
    url text,
    status text NOT NULL CHECK (status IN ('success', 'partial', 'failed')),
    repository_commits_json jsonb NOT NULL DEFAULT '[]',
    submission_id uuid,
    reported_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL
);

CREATE TABLE fix_propagations (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    bug_task_id uuid NOT NULL REFERENCES tasks (id),
    target_release_ref text NOT NULL,
    target_task_id uuid REFERENCES tasks (id),
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'proposed', 'accepted', 'rejected'))
);
