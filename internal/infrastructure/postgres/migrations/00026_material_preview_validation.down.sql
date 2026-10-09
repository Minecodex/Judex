-- +goose Down
ALTER TABLE material_previews DROP COLUMN validation_revision;
