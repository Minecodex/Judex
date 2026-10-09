-- +goose Down
DROP VIEW IF EXISTS material_usage_records;
DROP TABLE IF EXISTS material_previews;
DROP INDEX IF EXISTS material_library_project;
ALTER TABLE submissions DROP COLUMN IF EXISTS plan_id;
ALTER TABLE work_reports DROP COLUMN IF EXISTS plan_id;
ALTER TABLE material_versions DROP COLUMN IF EXISTS format,DROP COLUMN IF EXISTS upload_source;
ALTER TABLE upload_sessions DROP COLUMN IF EXISTS purpose;
ALTER TABLE materials DROP COLUMN IF EXISTS owner_user_id,DROP COLUMN IF EXISTS purpose,DROP COLUMN IF EXISTS deleted_at,DROP COLUMN IF EXISTS deleted_by;
