-- +goose Down
DROP TABLE work_report_materials;
ALTER TABLE work_reports DROP COLUMN agreement_version;
ALTER TABLE tasks DROP COLUMN agreement_version;
ALTER TABLE agent_runs DROP COLUMN version;
ALTER TABLE discussion_batches DROP COLUMN version;
ALTER TABLE confirmation_intents DROP COLUMN binding_snapshot;
ALTER TABLE plans DROP COLUMN created_by;
ALTER TABLE tasks DROP COLUMN created_by;
ALTER TABLE discussion_batches DROP COLUMN model_attempts, DROP COLUMN reserved_tokens, DROP COLUMN used_tokens;
ALTER TABLE tool_calls DROP COLUMN result_json;
