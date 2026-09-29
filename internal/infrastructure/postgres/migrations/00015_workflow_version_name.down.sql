-- +goose Down
ALTER TABLE workflow_versions DROP COLUMN name;
