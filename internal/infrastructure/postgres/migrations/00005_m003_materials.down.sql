-- +goose Down
DROP TABLE IF EXISTS material_links;
DROP TABLE IF EXISTS submission_materials;
DROP TABLE IF EXISTS material_entries;
DROP TABLE IF EXISTS material_versions;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS upload_parts;
DROP TABLE IF EXISTS upload_sessions;
