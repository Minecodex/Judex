-- +goose Up
ALTER TABLE tasks ADD COLUMN main_topic_id uuid;
ALTER TABLE tasks ADD CONSTRAINT tasks_main_topic_fk FOREIGN KEY(project_id,main_topic_id) REFERENCES topics(project_id,id);
CREATE UNIQUE INDEX tasks_main_topic_unique ON tasks(main_topic_id) WHERE main_topic_id IS NOT NULL;
ALTER TABLE topics ADD COLUMN links_version bigint NOT NULL DEFAULT 1 CHECK(links_version>0);
CREATE INDEX topic_work_scope_idx ON topic_work_links(project_id,object_type,object_id,topic_id);
