-- +goose Up
-- 限流窗口（docs/plans/v1/02 §1）：注册/登录按账号与 IP 维度的固定窗口
-- 计数，多副本共享 PG 状态。窗口键由服务端拼接。

CREATE TABLE rate_limit_windows (
    bucket text PRIMARY KEY,
    window_start timestamptz NOT NULL,
    count bigint NOT NULL DEFAULT 0
);
CREATE INDEX rate_limit_windows_expiry_idx ON rate_limit_windows (window_start);
