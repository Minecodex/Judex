-- +goose Up
CREATE TABLE projects (
 id uuid PRIMARY KEY, name text NOT NULL, description text NOT NULL DEFAULT '',
 owner_id uuid NOT NULL REFERENCES users(id), status text NOT NULL DEFAULT 'active',
 default_model_id uuid REFERENCES model_catalog(id), loop_limit integer NOT NULL DEFAULT 10 CHECK(loop_limit BETWEEN 1 AND 100),
 version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE project_members (
 project_id uuid REFERENCES projects(id), user_id uuid REFERENCES users(id),
 role text NOT NULL CHECK(role IN ('owner','admin','member')), state text NOT NULL DEFAULT 'active' CHECK(state IN ('active','removed')),
 version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(project_id,user_id)
);
CREATE TABLE positions (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES projects(id), name text NOT NULL,
 current_version integer NOT NULL DEFAULT 1, version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE position_versions (
 position_id uuid REFERENCES positions(id), revision integer NOT NULL, prompt text NOT NULL,
 model_id uuid REFERENCES model_catalog(id), created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(position_id,revision)
);
CREATE TABLE workflows (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES projects(id), name text NOT NULL,
 current_version integer NOT NULL DEFAULT 1, version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE workflow_versions (
 workflow_id uuid REFERENCES workflows(id), revision integer NOT NULL, content text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(workflow_id,revision)
);
CREATE TABLE personal_prompts (
 project_id uuid REFERENCES projects(id), user_id uuid REFERENCES users(id), position_id uuid REFERENCES positions(id),
 prompt text NOT NULL DEFAULT '', version bigint NOT NULL DEFAULT 1, PRIMARY KEY(project_id,user_id,position_id)
);
CREATE TABLE conversations (
 id uuid PRIMARY KEY, kind text NOT NULL CHECK(kind IN ('group','direct')), message_seq bigint NOT NULL DEFAULT 0,
 event_seq bigint NOT NULL DEFAULT 0, knowledge_seq bigint NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE groups (
 id uuid PRIMARY KEY REFERENCES conversations(id), project_id uuid NOT NULL REFERENCES projects(id),
 parent_id uuid REFERENCES groups(id), depth integer NOT NULL CHECK(depth BETWEEN 1 AND 3),
 name text NOT NULL, purpose text NOT NULL DEFAULT '', archived boolean NOT NULL DEFAULT false,
 workflow_ids uuid[] NOT NULL DEFAULT '{}', version bigint NOT NULL DEFAULT 1, created_by uuid NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX groups_project ON groups(project_id);
CREATE INDEX groups_parent ON groups(parent_id);
CREATE TABLE conversation_members (
 conversation_id uuid REFERENCES conversations(id), user_id uuid REFERENCES users(id), active boolean NOT NULL DEFAULT true,
 read_seq bigint NOT NULL DEFAULT 0, version bigint NOT NULL DEFAULT 1, joined_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(conversation_id,user_id)
);
CREATE TABLE direct_pairs (
 low_user_id uuid REFERENCES users(id), high_user_id uuid REFERENCES users(id),
 conversation_id uuid UNIQUE NOT NULL REFERENCES conversations(id), PRIMARY KEY(low_user_id,high_user_id), CHECK(low_user_id<high_user_id)
);
CREATE TABLE role_agents (
 id uuid PRIMARY KEY, group_id uuid NOT NULL REFERENCES groups(id), user_id uuid NOT NULL REFERENCES users(id),
 position_id uuid NOT NULL REFERENCES positions(id), active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(group_id,user_id,position_id)
);
CREATE TABLE messages (
 id uuid PRIMARY KEY, conversation_id uuid NOT NULL REFERENCES conversations(id), seq bigint NOT NULL,
 author_user_id uuid NOT NULL REFERENCES users(id), role_agent_id uuid REFERENCES role_agents(id),
 source text NOT NULL CHECK(source IN ('web','cli','agent','share','system')), body text NOT NULL DEFAULT '',
 reply_to uuid REFERENCES messages(id), discussion_id uuid, run_id uuid,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(conversation_id,seq)
);
CREATE INDEX messages_timeline ON messages(conversation_id,seq DESC);
CREATE TABLE message_mentions (
 message_id uuid REFERENCES messages(id), kind text NOT NULL CHECK(kind IN ('human','role_agent')),
 target_id uuid NOT NULL, requires_response boolean NOT NULL DEFAULT false, PRIMARY KEY(message_id,kind,target_id)
);
CREATE TABLE files (
 id uuid PRIMARY KEY, conversation_id uuid NOT NULL REFERENCES conversations(id), name text NOT NULL,
 latest_revision integer NOT NULL DEFAULT 0, removed boolean NOT NULL DEFAULT false, version bigint NOT NULL DEFAULT 1,
 created_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE file_versions (
 id uuid PRIMARY KEY, file_id uuid NOT NULL REFERENCES files(id), revision integer NOT NULL,
 object_key text NOT NULL UNIQUE CHECK(object_key LIKE 'v2/%'), sha256 text NOT NULL, size bigint NOT NULL,
 mime text NOT NULL, content_text text NOT NULL DEFAULT '', preview_state text NOT NULL DEFAULT 'pending',
 preview_data jsonb NOT NULL DEFAULT '{}', author_user_id uuid NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(file_id,revision)
);
CREATE TABLE message_files (message_id uuid REFERENCES messages(id), version_id uuid REFERENCES file_versions(id), PRIMARY KEY(message_id,version_id));
CREATE TABLE file_grants (
 conversation_id uuid REFERENCES conversations(id), version_id uuid REFERENCES file_versions(id),
 granted_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(conversation_id,version_id)
);
CREATE TABLE shares (
 id uuid PRIMARY KEY, source_message_id uuid REFERENCES messages(id), target_conversation_id uuid NOT NULL REFERENCES conversations(id),
 message_id uuid NOT NULL REFERENCES messages(id), shared_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE uploads (
 id uuid PRIMARY KEY, conversation_id uuid NOT NULL REFERENCES conversations(id), user_id uuid NOT NULL REFERENCES users(id),
 file_id uuid REFERENCES files(id), name text NOT NULL, mime text NOT NULL, size bigint NOT NULL CHECK(size BETWEEN 0 AND 104857600),
 sha256 text NOT NULL, expected_version bigint, state text NOT NULL DEFAULT 'pending', result_version_id uuid REFERENCES file_versions(id),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '1 day', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE upload_parts (
 upload_id uuid REFERENCES uploads(id), part integer NOT NULL CHECK(part>=1), object_key text NOT NULL,
 sha256 text NOT NULL, size bigint NOT NULL, PRIMARY KEY(upload_id,part)
);
CREATE TABLE conversation_events (
 conversation_id uuid REFERENCES conversations(id), seq bigint NOT NULL, kind text NOT NULL, data jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(conversation_id,seq)
);
CREATE TABLE user_event_counters (user_id uuid PRIMARY KEY REFERENCES users(id), seq bigint NOT NULL DEFAULT 0);
CREATE TABLE user_events (
 user_id uuid REFERENCES users(id), seq bigint NOT NULL, kind text NOT NULL, conversation_id uuid REFERENCES conversations(id),
 data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(user_id,seq)
);
CREATE TABLE notifications (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), conversation_id uuid REFERENCES conversations(id),
 project_id uuid REFERENCES projects(id), kind text NOT NULL, data jsonb NOT NULL, dedupe_key text NOT NULL,
 read_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(user_id,dedupe_key)
);
CREATE TABLE discussions (
 id uuid PRIMARY KEY, group_id uuid NOT NULL REFERENCES groups(id),
 state text NOT NULL CHECK(state IN ('active','paused','settled','terminated')), loop_count integer NOT NULL DEFAULT 0,
 allowance integer NOT NULL, pause_reason text, version bigint NOT NULL DEFAULT 1,
 model_calls integer NOT NULL DEFAULT 0, tokens bigint NOT NULL DEFAULT 0, started_at timestamptz NOT NULL DEFAULT now(), ended_at timestamptz
);
CREATE UNIQUE INDEX one_open_discussion ON discussions(group_id) WHERE state IN ('active','paused');
ALTER TABLE messages ADD CONSTRAINT messages_discussion FOREIGN KEY(discussion_id) REFERENCES discussions(id);
CREATE TABLE discussion_controls (
 id uuid PRIMARY KEY, discussion_id uuid NOT NULL REFERENCES discussions(id), user_id uuid NOT NULL REFERENCES users(id),
 action text NOT NULL, feedback text NOT NULL DEFAULT '', allowance integer NOT NULL, loop_count integer NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE deliveries (
 id uuid PRIMARY KEY, discussion_id uuid NOT NULL REFERENCES discussions(id), role_agent_id uuid NOT NULL REFERENCES role_agents(id),
 trigger_message_id uuid NOT NULL REFERENCES messages(id), causal_path uuid[] NOT NULL DEFAULT '{}',
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','running','waiting_human','done','cancelled')),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(discussion_id,role_agent_id,trigger_message_id)
);
CREATE TABLE human_waits (
 delivery_id uuid REFERENCES deliveries(id), message_id uuid REFERENCES messages(id), user_id uuid REFERENCES users(id),
 resolved boolean NOT NULL DEFAULT false, PRIMARY KEY(delivery_id,message_id,user_id)
);
CREATE TABLE role_runs (
 id uuid PRIMARY KEY, delivery_id uuid UNIQUE NOT NULL REFERENCES deliveries(id), role_agent_id uuid NOT NULL REFERENCES role_agents(id),
 discussion_id uuid NOT NULL REFERENCES discussions(id), state text NOT NULL DEFAULT 'pending',
 lease_owner text, lease_until timestamptz, fence bigint NOT NULL DEFAULT 0, checkpoint jsonb NOT NULL DEFAULT '[]',
 snapshot_version bigint, model_calls integer NOT NULL DEFAULT 0, tokens bigint NOT NULL DEFAULT 0,
 error_code text, error_message text, sandbox_id text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE messages ADD CONSTRAINT messages_run FOREIGN KEY(run_id) REFERENCES role_runs(id);
CREATE UNIQUE INDEX one_running_role ON role_runs(role_agent_id) WHERE state='running';
CREATE TABLE role_tool_calls (
 run_id uuid REFERENCES role_runs(id), call_id text NOT NULL, name text NOT NULL, args jsonb NOT NULL,
 state text NOT NULL CHECK(state IN ('started','completed','unknown')), result jsonb,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(run_id,call_id)
);
CREATE TABLE role_model_calls (
 id uuid PRIMARY KEY, run_id uuid NOT NULL REFERENCES role_runs(id), snapshot_version bigint NOT NULL,
 model_id text NOT NULL, input_hash text NOT NULL, usage jsonb NOT NULL DEFAULT '{}', state text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE knowledge_entries (
 group_id uuid REFERENCES groups(id), seq bigint NOT NULL, kind text NOT NULL, ref_id text NOT NULL,
 content jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(group_id,seq), UNIQUE(group_id,kind,ref_id)
);
CREATE TABLE context_snapshots (
 group_id uuid REFERENCES groups(id), version bigint NOT NULL, covered_through bigint NOT NULL DEFAULT 0,
 summary text NOT NULL DEFAULT '', entries jsonb NOT NULL, estimated_tokens bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(group_id,version)
);
CREATE TABLE context_heads (
 group_id uuid PRIMARY KEY REFERENCES groups(id), version bigint NOT NULL DEFAULT 0,
 covered_through bigint NOT NULL DEFAULT 0, summary text NOT NULL DEFAULT '',
 compress_owner text, compress_until timestamptz, error_message text
);
