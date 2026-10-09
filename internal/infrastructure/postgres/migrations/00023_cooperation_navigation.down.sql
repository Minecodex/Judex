-- +goose Down
DROP INDEX IF EXISTS topic_work_scope_idx;
ALTER TABLE topics DROP COLUMN links_version;
ALTER TABLE tasks DROP COLUMN main_topic_id;
