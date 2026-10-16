-- +goose Down
ALTER TABLE model_catalog DROP COLUMN request_options;
