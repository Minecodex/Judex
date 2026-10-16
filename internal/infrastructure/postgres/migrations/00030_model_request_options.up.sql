-- +goose Up
ALTER TABLE model_catalog ADD COLUMN request_options jsonb NOT NULL DEFAULT '{}';
