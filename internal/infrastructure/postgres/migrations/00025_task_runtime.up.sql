-- +goose Up
ALTER TABLE plans ADD COLUMN discarded_at timestamptz;
ALTER TABLE tasks ADD COLUMN discarded_at timestamptz;
CREATE TABLE task_execution_exceptions (
 project_id uuid NOT NULL REFERENCES projects(id),
 id uuid PRIMARY KEY,
 task_id uuid NOT NULL REFERENCES tasks(id),
 previous_status text NOT NULL CHECK(previous_status IN ('ready','working','delivered','rework')),
 reason text NOT NULL CHECK(length(trim(reason))>0),
 actor_user_id uuid NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL,
 waivers_json jsonb NOT NULL DEFAULT '[]',
 restored_at timestamptz,
 restored_by uuid REFERENCES users(id),
 restore_reason text,
 CHECK ((restored_at IS NULL)=(restored_by IS NULL))
);
CREATE UNIQUE INDEX task_execution_active ON task_execution_exceptions(project_id,task_id) WHERE restored_at IS NULL;
CREATE TABLE work_change_records (
 project_id uuid NOT NULL REFERENCES projects(id),
 id uuid PRIMARY KEY,
 object_type text NOT NULL CHECK(object_type IN ('task','plan')),
 object_id uuid NOT NULL,
 operation text NOT NULL,
 source text NOT NULL DEFAULT 'web' CHECK(source IN ('web','cli','unknown')),
 actor_user_id uuid NOT NULL REFERENCES users(id),
 text text NOT NULL,
 created_at timestamptz NOT NULL,
 details_json jsonb NOT NULL DEFAULT '{}'
);
CREATE INDEX work_change_records_task ON work_change_records(project_id,object_id,created_at,id);
