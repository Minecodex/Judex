-- +goose Up
ALTER TABLE model_catalog ADD COLUMN quota_fallback_ids jsonb NOT NULL DEFAULT '[]';
