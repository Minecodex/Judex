-- +goose Down
-- M001 回滚：按依赖逆序删除（仅用于开发环境；生产回滚策略见 docs/plans/v1/01 §6）。
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS background_jobs;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS recovery_tokens;
DROP TABLE IF EXISTS device_authorizations;
DROP TABLE IF EXISTS client_tokens;
DROP TABLE IF EXISTS client_grants;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS password_credentials;
DROP TABLE IF EXISTS users;
