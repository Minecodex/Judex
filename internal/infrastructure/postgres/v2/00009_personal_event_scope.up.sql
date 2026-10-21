-- +goose Up
ALTER TABLE user_events ADD COLUMN project_id uuid REFERENCES projects(id);
