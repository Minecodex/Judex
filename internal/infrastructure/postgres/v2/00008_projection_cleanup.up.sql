-- +goose Up
ALTER TABLE role_runs ADD COLUMN projection_claim text, ADD COLUMN projection_removed boolean NOT NULL DEFAULT false;
