-- +goose Down
ALTER TABLE proposal_versions DROP COLUMN created_ids_json;
ALTER TABLE work_reports DROP COLUMN payload_hash;
ALTER TABLE work_reports DROP COLUMN task_version;
ALTER TABLE agent_runs DROP COLUMN requested_by;
ALTER TABLE agent_sessions DROP COLUMN active_run_id;
ALTER TABLE plan_acceptances DROP COLUMN review_manifest, DROP COLUMN evidence_hash;
