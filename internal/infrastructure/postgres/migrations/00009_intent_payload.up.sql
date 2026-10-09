-- +goose Up
-- 确认意图载荷快照（07 §5）：执行时按原载荷调用领域命令。
ALTER TABLE confirmation_intents ADD COLUMN payload_json jsonb NOT NULL DEFAULT '{}';
