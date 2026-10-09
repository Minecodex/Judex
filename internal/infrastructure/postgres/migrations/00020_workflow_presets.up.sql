-- +goose Up
ALTER TABLE workflow_definitions ADD COLUMN preset_id text;
CREATE UNIQUE INDEX workflow_definitions_project_preset_unique
    ON workflow_definitions(project_id, preset_id) WHERE preset_id IS NOT NULL;
