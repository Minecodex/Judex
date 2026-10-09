-- +goose Down
DROP INDEX position_templates_active_preset;
ALTER TABLE position_templates DROP COLUMN preset_id;
