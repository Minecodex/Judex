-- +goose Up
ALTER TABLE task_analyses ALTER COLUMN task_id DROP NOT NULL;
CREATE UNIQUE INDEX agent_sessions_project_scope ON agent_sessions(project_id,identity_id)
 WHERE topic_id IS NULL AND task_id IS NULL;
