-- +goose Down
DROP TABLE IF EXISTS topic_work_links;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS model_catalog;
DROP TABLE IF EXISTS repository_links;
DROP TABLE IF EXISTS position_node_bindings;
DROP TABLE IF EXISTS workflow_versions;
DROP TABLE IF EXISTS workflow_definitions;
DROP TABLE IF EXISTS preference_revisions;
DROP TABLE IF EXISTS personal_project_preferences;
DROP TABLE IF EXISTS identity_bindings;
DROP TABLE IF EXISTS agent_identities;
DROP TABLE IF EXISTS invitation_positions;
DROP TABLE IF EXISTS project_invitations;
DROP TABLE IF EXISTS position_versions;
DROP TABLE IF EXISTS position_templates;
DROP TABLE IF EXISTS ownership_transfers;
DROP TABLE IF EXISTS project_members;
