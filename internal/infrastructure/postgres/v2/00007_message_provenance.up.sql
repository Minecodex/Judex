-- +goose Up
ALTER TABLE messages ADD COLUMN origin jsonb NOT NULL DEFAULT '{}';
ALTER TABLE role_tool_calls ADD COLUMN replay_authorized boolean NOT NULL DEFAULT false;
ALTER TABLE role_tool_calls ADD COLUMN reconciliation jsonb;
