-- +goose Down
DELETE FROM task_analyses WHERE task_id IS NULL;
DROP INDEX IF EXISTS agent_sessions_project_scope;
ALTER TABLE task_analyses ALTER COLUMN task_id SET NOT NULL;
