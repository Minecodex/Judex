-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS project_events;
DROP TABLE IF EXISTS projects;
