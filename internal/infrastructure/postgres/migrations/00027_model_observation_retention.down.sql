-- +goose Down
DROP INDEX IF EXISTS model_calls_observation_expiry_idx;
