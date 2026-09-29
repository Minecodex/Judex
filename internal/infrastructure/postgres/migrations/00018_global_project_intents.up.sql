-- +goose Up
ALTER TABLE confirmation_intents ALTER COLUMN project_id DROP NOT NULL;
UPDATE confirmation_intents SET project_id=NULL WHERE operation='project.create';
ALTER TABLE confirmation_intents ADD CONSTRAINT intent_scope CHECK ((project_id IS NULL)=(operation='project.create'));
