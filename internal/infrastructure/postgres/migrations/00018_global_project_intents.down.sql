-- +goose Down
-- Global intents cannot be represented in the older schema; rollback requires
-- the operator to resolve/remove those records explicitly first.
ALTER TABLE confirmation_intents DROP CONSTRAINT intent_scope;
ALTER TABLE confirmation_intents ALTER COLUMN project_id SET NOT NULL;
