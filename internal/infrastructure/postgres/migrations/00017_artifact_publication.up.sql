-- +goose Up
CREATE TABLE artifact_publications(
 project_id uuid NOT NULL REFERENCES projects(id),run_id uuid NOT NULL REFERENCES agent_runs(id),
 command_key text NOT NULL,request_hash text NOT NULL,material_version_id uuid NOT NULL REFERENCES material_versions(id),
 created_at timestamptz NOT NULL,PRIMARY KEY(run_id,command_key)
);
