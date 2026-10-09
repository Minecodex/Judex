-- +goose Up
ALTER TABLE workflow_versions ADD COLUMN name text NOT NULL DEFAULT '';
UPDATE workflow_versions v SET name=d.name FROM workflow_definitions d WHERE v.definition_id=d.id;
