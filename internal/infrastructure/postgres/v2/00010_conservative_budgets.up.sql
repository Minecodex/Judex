-- +goose Up
ALTER TABLE role_runs ADD COLUMN budget_model_calls integer NOT NULL DEFAULT 0;
ALTER TABLE role_runs ADD COLUMN budget_tokens bigint NOT NULL DEFAULT 0;
ALTER TABLE role_runs ADD COLUMN budget_epoch bigint NOT NULL DEFAULT 1;
UPDATE role_runs SET budget_model_calls=model_calls,budget_tokens=tokens;
ALTER TABLE role_model_calls ADD COLUMN reserved_tokens bigint NOT NULL DEFAULT 0;
ALTER TABLE role_model_calls ADD COLUMN charged_tokens bigint;
ALTER TABLE role_model_calls ADD COLUMN budget_epoch bigint NOT NULL DEFAULT 1;
