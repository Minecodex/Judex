-- +goose Down
ALTER TABLE confirmation_intents DROP COLUMN IF EXISTS payload_json;
