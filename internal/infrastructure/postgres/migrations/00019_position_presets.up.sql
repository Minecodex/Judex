-- +goose Up
-- Record the source of a copied built-in position without linking its content.
ALTER TABLE position_templates ADD COLUMN preset_id text;
CREATE UNIQUE INDEX position_templates_active_preset
  ON position_templates (project_id, preset_id)
  WHERE preset_id IS NOT NULL AND status = 'active';
