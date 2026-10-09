-- +goose Down
DROP TABLE topic_source_refs;
DROP TABLE discussion_suggestions;
DROP TABLE task_analyses;
DROP INDEX agent_sessions_identity_task;
ALTER TABLE agent_sessions DROP COLUMN task_id;
DROP INDEX discussion_batches_report_trigger;
ALTER TABLE discussion_batches DROP COLUMN source_report_id;
ALTER TABLE discussion_batches DROP COLUMN task_id;
ALTER TABLE work_reports DROP COLUMN source;
ALTER TABLE submissions DROP COLUMN discussion_intent;
ALTER TABLE plans DROP COLUMN main_topic_id;
ALTER TABLE topics DROP COLUMN parent_topic_id;
ALTER TABLE topics DROP COLUMN fork_after_seq;
