-- +goose Up
CREATE TABLE file_entries (
 version_id uuid REFERENCES file_versions(id), path text NOT NULL, object_key text NOT NULL UNIQUE,
 mime text NOT NULL, size bigint NOT NULL, sha256 text NOT NULL, PRIMARY KEY(version_id,path)
);
