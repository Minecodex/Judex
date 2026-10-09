-- +goose Down
DROP TABLE work_change_records;
DROP TABLE task_execution_exceptions;
ALTER TABLE tasks DROP COLUMN discarded_at;
ALTER TABLE plans DROP COLUMN discarded_at;
