-- +goose Up
ALTER TABLE model_calls ADD COLUMN reserved_tokens bigint NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX model_calls_run_attempt ON model_calls(run_id,attempt);
CREATE INDEX context_checkpoints_latest ON context_checkpoints(session_id,created_at DESC);
