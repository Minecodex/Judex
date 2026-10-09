-- +goose Up
-- M002 剩余表（docs/plans/v1/01 §3）+ M003 的 topics 骨架（项目创建需要主会场）。

CREATE TABLE project_members (
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    role text NOT NULL CHECK (role IN ('owner', 'manager', 'member')),
    state text NOT NULL DEFAULT 'active' CHECK (state IN ('active', 'left', 'removed')),
    joined_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, user_id)
);
-- 恰好一个 active owner（02 §4）。
CREATE UNIQUE INDEX project_members_single_owner
    ON project_members (project_id) WHERE role = 'owner' AND state = 'active';

CREATE TABLE ownership_transfers (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    from_user_id uuid NOT NULL REFERENCES users (id),
    to_user_id uuid NOT NULL REFERENCES users (id),
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'accepted', 'declined', 'cancelled', 'expired')),
    project_version bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    confirmed_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE TABLE position_templates (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    name text NOT NULL,
    current_version bigint NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz
);

CREATE TABLE position_versions (
    project_id uuid NOT NULL REFERENCES projects (id),
    template_id uuid NOT NULL REFERENCES position_templates (id),
    revision bigint NOT NULL,
    prompt text NOT NULL DEFAULT '',
    public_summary text NOT NULL DEFAULT '',
    model_id uuid,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (template_id, revision)
);

CREATE TABLE project_invitations (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    target_user_id uuid REFERENCES users (id),
    target_email_normalized text NOT NULL,
    inviter_user_id uuid NOT NULL REFERENCES users (id),
    token_hash bytea NOT NULL UNIQUE,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'accepted', 'declined', 'revoked', 'expired')),
    expires_at timestamptz NOT NULL,
    accepted_user_id uuid REFERENCES users (id),
    created_at timestamptz NOT NULL
);

CREATE TABLE invitation_positions (
    project_id uuid NOT NULL REFERENCES projects (id),
    invitation_id uuid NOT NULL REFERENCES project_invitations (id),
    position_template_id uuid NOT NULL REFERENCES position_templates (id),
    PRIMARY KEY (invitation_id, position_template_id)
);

CREATE TABLE agent_identities (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    template_id uuid REFERENCES position_templates (id),
    kind text NOT NULL CHECK (kind IN ('coordinator', 'position')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    current_binding_version bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL
);
-- 项目 coordinator 唯一（02 §4）。
CREATE UNIQUE INDEX agent_identities_single_coordinator
    ON agent_identities (project_id) WHERE kind = 'coordinator';

CREATE TABLE identity_bindings (
    project_id uuid NOT NULL REFERENCES projects (id),
    identity_id uuid NOT NULL REFERENCES agent_identities (id),
    binding_version bigint NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id),
    valid_from timestamptz NOT NULL,
    valid_until timestamptz,
    PRIMARY KEY (identity_id, binding_version)
);

CREATE TABLE personal_project_preferences (
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    revision bigint NOT NULL DEFAULT 1,
    prompt text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE preference_revisions (
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    revision bigint NOT NULL,
    prompt text NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, project_id, revision)
);

CREATE TABLE workflow_definitions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    name text NOT NULL,
    published_version_id uuid,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE workflow_versions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    definition_id uuid NOT NULL REFERENCES workflow_definitions (id),
    revision bigint NOT NULL,
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'published')),
    instructions text NOT NULL DEFAULT '',
    nodes_json jsonb NOT NULL DEFAULT '[]',
    advisory_edges_json jsonb NOT NULL DEFAULT '[]',
    approval_policies_json jsonb NOT NULL DEFAULT '{}',
    hard_rules_json jsonb NOT NULL DEFAULT '[]',
    mermaid text NOT NULL DEFAULT '',
    author_user_id uuid REFERENCES users (id),
    published_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (definition_id, revision)
);

CREATE TABLE position_node_bindings (
    project_id uuid NOT NULL REFERENCES projects (id),
    template_id uuid NOT NULL REFERENCES position_templates (id),
    workflow_id uuid NOT NULL REFERENCES workflow_definitions (id),
    node_id text NOT NULL,
    PRIMARY KEY (template_id, workflow_id, node_id)
);

CREATE TABLE repository_links (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    url text NOT NULL,
    provider text NOT NULL DEFAULT 'git',
    default_branch text NOT NULL DEFAULT 'main',
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz
);

CREATE TABLE model_catalog (
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    provider text NOT NULL,
    endpoint_config_ref text NOT NULL DEFAULT '',
    credential_secret_ref text NOT NULL DEFAULT '',
    model_name text NOT NULL,
    capabilities jsonb NOT NULL DEFAULT '{}',
    limits jsonb NOT NULL DEFAULT '{}',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- M003 骨架：议题（项目主会场在项目创建事务中生成）。
CREATE TABLE topics (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    title text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('project_room', 'discussion', 'handoff')),
    context_type text,
    context_id uuid,
    last_message_seq bigint NOT NULL DEFAULT 0,
    created_by uuid REFERENCES users (id),
    created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX topics_single_project_room
    ON topics (project_id) WHERE kind = 'project_room';

CREATE TABLE topic_work_links (
    project_id uuid NOT NULL REFERENCES projects (id),
    topic_id uuid NOT NULL REFERENCES topics (id),
    object_type text NOT NULL CHECK (object_type IN ('plan', 'task')),
    object_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (topic_id, object_type, object_id)
);
