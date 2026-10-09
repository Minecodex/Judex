-- +goose Down
DROP INDEX workflow_definitions_project_preset_unique;
ALTER TABLE workflow_definitions DROP COLUMN preset_id;
