-- +goose Down
ALTER TABLE model_catalog DROP COLUMN quota_fallback_ids;
