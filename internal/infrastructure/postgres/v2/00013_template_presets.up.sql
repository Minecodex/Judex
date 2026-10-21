-- +goose Up
ALTER TABLE positions ADD COLUMN preset_id text;
ALTER TABLE workflows ADD COLUMN preset_id text;
CREATE UNIQUE INDEX positions_project_preset ON positions(project_id,preset_id) WHERE preset_id IS NOT NULL;
CREATE UNIQUE INDEX workflows_project_preset ON workflows(project_id,preset_id) WHERE preset_id IS NOT NULL;
