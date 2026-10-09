-- +goose Down
-- A rollback cannot merge distinct position prompts without choosing arbitrarily.
-- Recreate empty legacy preferences; business records are unaffected.
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
DROP TABLE position_preference_revisions;
DROP TABLE personal_position_preferences;
