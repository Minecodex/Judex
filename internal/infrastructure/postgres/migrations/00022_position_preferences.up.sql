-- +goose Up
CREATE TABLE personal_position_preferences (
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    position_id uuid NOT NULL REFERENCES position_templates (id),
    revision bigint NOT NULL CHECK (revision > 0),
    prompt text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, user_id, position_id)
);

CREATE TABLE position_preference_revisions (
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    position_id uuid NOT NULL REFERENCES position_templates (id),
    revision bigint NOT NULL CHECK (revision > 0),
    prompt text NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (project_id, user_id, position_id, revision)
);

-- Preserve the previous effective preference for each currently held position.
-- These become independent copies; later assignments have no inherited prompt.
INSERT INTO personal_position_preferences (project_id,user_id,position_id,revision,prompt,updated_at)
SELECT DISTINCT pref.project_id,pref.user_id,i.template_id,pref.revision,pref.prompt,pref.updated_at
FROM personal_project_preferences pref
JOIN project_members m ON m.project_id=pref.project_id AND m.user_id=pref.user_id AND m.state='active'
JOIN identity_bindings b ON b.project_id=pref.project_id AND b.user_id=pref.user_id AND b.valid_until IS NULL
JOIN agent_identities i ON i.id=b.identity_id AND i.current_binding_version=b.binding_version AND i.kind='position' AND i.status='active'
JOIN position_templates t ON t.id=i.template_id AND t.project_id=pref.project_id AND t.status='active';

INSERT INTO position_preference_revisions (project_id,user_id,position_id,revision,prompt,created_at)
SELECT pref.project_id,pref.user_id,pref.position_id,r.revision,r.prompt,r.created_at
FROM personal_position_preferences pref
JOIN preference_revisions r ON r.project_id=pref.project_id AND r.user_id=pref.user_id;

DROP TABLE preference_revisions;
DROP TABLE personal_project_preferences;
