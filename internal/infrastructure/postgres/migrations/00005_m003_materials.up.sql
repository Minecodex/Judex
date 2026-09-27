-- +goose Up
-- M003 材料部分（docs/plans/v1/01 §3、04 §2-§3）：上传会话、不可变材料版本。

CREATE TABLE upload_sessions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    owner_user_id uuid,
    grant_id uuid,
    state text NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'completed', 'cancelled', 'expired')),
    kind text NOT NULL CHECK (kind IN ('file', 'html_bundle')),
    name text NOT NULL,
    staging_key text NOT NULL,
    expected_size bigint NOT NULL,
    checksum text NOT NULL,
    mime text NOT NULL DEFAULT 'application/octet-stream',
    entrypoint text,
    part_size bigint NOT NULL DEFAULT 8388608,
    part_count int NOT NULL DEFAULT 0,
    material_id uuid,
    result_version_id uuid,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX upload_sessions_expiry_idx ON upload_sessions (expires_at);

CREATE TABLE upload_parts (
    project_id uuid NOT NULL REFERENCES projects (id),
    upload_id uuid NOT NULL REFERENCES upload_sessions (id),
    part_number int NOT NULL,
    size bigint NOT NULL,
    sha256 text NOT NULL,
    storage_etag text,
    state text NOT NULL DEFAULT 'stored' CHECK (state IN ('stored', 'failed')),
    received_at timestamptz NOT NULL,
    PRIMARY KEY (upload_id, part_number)
);

CREATE TABLE materials (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    title text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('file', 'html_bundle', 'text')),
    visibility text NOT NULL DEFAULT 'project',
    current_version_id uuid,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE material_versions (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    material_id uuid NOT NULL REFERENCES materials (id),
    revision bigint NOT NULL,
    manifest_key text NOT NULL,
    sha256 text NOT NULL,
    size bigint NOT NULL,
    mime text NOT NULL,
    entrypoint text,
    author_id uuid,
    run_id uuid,
    source_manifest_json jsonb,
    state text NOT NULL DEFAULT 'staged' CHECK (state IN ('staged', 'ready', 'unavailable')),
    created_at timestamptz NOT NULL,
    UNIQUE (material_id, revision)
);

CREATE TABLE material_entries (
    project_id uuid NOT NULL REFERENCES projects (id),
    version_id uuid NOT NULL REFERENCES material_versions (id),
    relative_path text NOT NULL,
    object_key text NOT NULL,
    size bigint NOT NULL,
    sha256 text NOT NULL,
    mime text NOT NULL,
    PRIMARY KEY (version_id, relative_path)
);

CREATE TABLE submission_materials (
    project_id uuid NOT NULL REFERENCES projects (id),
    submission_id uuid NOT NULL,
    material_version_id uuid NOT NULL REFERENCES material_versions (id),
    PRIMARY KEY (submission_id, material_version_id)
);

CREATE TABLE material_links (
    project_id uuid NOT NULL REFERENCES projects (id),
    id uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES material_versions (id),
    object_type text NOT NULL,
    object_id uuid NOT NULL,
    purpose text NOT NULL DEFAULT 'reference',
    confirmed_by uuid,
    proposal_id uuid,
    created_at timestamptz NOT NULL
);
