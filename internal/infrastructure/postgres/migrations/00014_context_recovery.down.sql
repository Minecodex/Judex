-- +goose Down
DROP INDEX context_checkpoints_latest;
DROP INDEX model_calls_run_attempt;
ALTER TABLE model_calls DROP COLUMN reserved_tokens;
