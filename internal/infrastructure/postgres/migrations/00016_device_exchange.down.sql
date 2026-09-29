-- +goose Down
ALTER TABLE device_authorizations DROP COLUMN last_polled_at,DROP COLUMN tokens_issued_at;
