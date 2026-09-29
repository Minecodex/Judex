-- +goose Up
ALTER TABLE tasks ADD COLUMN agreement_version bigint NOT NULL DEFAULT 1;
ALTER TABLE work_reports ADD COLUMN agreement_version bigint NOT NULL DEFAULT 1;
CREATE TABLE work_report_materials (
 project_id uuid NOT NULL REFERENCES projects(id),
 report_id uuid NOT NULL REFERENCES work_reports(id),
 material_version_id uuid NOT NULL REFERENCES material_versions(id),
 PRIMARY KEY(report_id,material_version_id)
);
ALTER TABLE agent_runs ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE discussion_batches ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE confirmation_intents ADD COLUMN binding_snapshot jsonb NOT NULL DEFAULT '{}';
ALTER TABLE plans ADD COLUMN created_by uuid REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN created_by uuid REFERENCES users(id);
ALTER TABLE discussion_batches ADD COLUMN model_attempts integer NOT NULL DEFAULT 0;
ALTER TABLE discussion_batches ADD COLUMN reserved_tokens bigint NOT NULL DEFAULT 0;
ALTER TABLE discussion_batches ADD COLUMN used_tokens bigint NOT NULL DEFAULT 0;
ALTER TABLE tool_calls ADD COLUMN result_json jsonb;
