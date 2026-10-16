-- +goose Up
-- Temporary cache measurements expire independently of durable run accounting.
CREATE INDEX model_calls_observation_expiry_idx ON model_calls(ended_at,id)
WHERE usage_json ? 'observation';
