-- +goose Up
ALTER TABLE device_authorizations ADD COLUMN last_polled_at timestamptz, ADD COLUMN tokens_issued_at timestamptz;
