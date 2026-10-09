-- +goose Up
-- Fixed-version preview caches prepared before structural validation are
-- rebuilt lazily. Originals and existing work evidence are never rewritten.
ALTER TABLE material_previews ADD COLUMN validation_revision integer NOT NULL DEFAULT 0;
