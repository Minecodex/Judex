-- +goose Up
-- A single public message may close several causal paths. Once its first
-- closure exhausts the allowance, retain later closures without spending
-- the next allowance before a human explicitly continues the discussion.
ALTER TABLE deliveries ADD COLUMN closure_pending boolean NOT NULL DEFAULT false;
