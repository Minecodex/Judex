-- +goose Up
ALTER TABLE role_runs ADD COLUMN budget_started_at timestamptz NOT NULL DEFAULT now();
