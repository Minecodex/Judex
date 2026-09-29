-- +goose Up
ALTER TABLE proposal_versions ADD COLUMN created_ids_json jsonb NOT NULL DEFAULT '{}';
ALTER TABLE work_reports ADD COLUMN payload_hash text NOT NULL DEFAULT '';
ALTER TABLE work_reports ADD COLUMN task_version bigint NOT NULL DEFAULT 1;
ALTER TABLE agent_runs ADD COLUMN requested_by uuid REFERENCES users(id);
ALTER TABLE agent_sessions ADD COLUMN active_run_id uuid;
ALTER TABLE plan_acceptances ADD COLUMN review_manifest jsonb NOT NULL DEFAULT '{}';
ALTER TABLE plan_acceptances ADD COLUMN evidence_hash text NOT NULL DEFAULT '';
