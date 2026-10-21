-- +goose Up
ALTER TABLE file_versions ADD COLUMN name text NOT NULL DEFAULT '';
UPDATE file_versions v SET name=f.name FROM files f WHERE f.id=v.file_id;
