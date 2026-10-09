-- +goose Up
ALTER TABLE topics ADD COLUMN parent_topic_id uuid;
ALTER TABLE topics ADD COLUMN fork_after_seq bigint NOT NULL DEFAULT 0 CHECK(fork_after_seq>=0);
ALTER TABLE topics ADD FOREIGN KEY(project_id,parent_topic_id) REFERENCES topics(project_id,id);
ALTER TABLE topics ADD CHECK(parent_topic_id IS NOT NULL OR fork_after_seq=0);
CREATE INDEX topics_parent_idx ON topics(project_id,parent_topic_id);
ALTER TABLE plans ADD COLUMN main_topic_id uuid;
ALTER TABLE plans ADD FOREIGN KEY(project_id,main_topic_id) REFERENCES topics(project_id,id);
CREATE UNIQUE INDEX plans_main_topic_unique ON plans(main_topic_id) WHERE main_topic_id IS NOT NULL;
INSERT INTO topics(project_id,id,title,kind,context_type,context_id,created_by,created_at)
SELECT p.project_id,gen_random_uuid(),p.title,'discussion','plan',p.id,p.created_by,p.created_at
FROM plans p;
UPDATE plans p SET main_topic_id=t.id FROM topics t
WHERE t.project_id=p.project_id AND t.context_type='plan' AND t.context_id=p.id;
INSERT INTO topic_work_links(project_id,topic_id,object_type,object_id,created_at)
SELECT project_id,main_topic_id,'plan',id,created_at FROM plans ON CONFLICT DO NOTHING;
ALTER TABLE submissions ADD COLUMN discussion_intent text NOT NULL DEFAULT 'auto'
 CHECK(discussion_intent IN ('auto','question','reply'));
ALTER TABLE work_reports ADD COLUMN source text NOT NULL DEFAULT 'unknown';
UPDATE work_reports r SET source=s.source FROM submissions s WHERE s.id=r.submission_id;
ALTER TABLE discussion_batches ALTER COLUMN topic_id DROP NOT NULL;
ALTER TABLE discussion_batches ADD COLUMN task_id uuid;
ALTER TABLE discussion_batches ADD COLUMN source_report_id uuid;
ALTER TABLE discussion_batches ADD FOREIGN KEY(project_id,task_id) REFERENCES tasks(project_id,id);
ALTER TABLE discussion_batches ADD FOREIGN KEY(project_id,source_report_id) REFERENCES work_reports(project_id,id);
CREATE UNIQUE INDEX discussion_batches_report_trigger ON discussion_batches(source_report_id,trigger_kind)
 WHERE source_report_id IS NOT NULL;
ALTER TABLE agent_sessions ADD COLUMN task_id uuid;
ALTER TABLE agent_sessions ADD FOREIGN KEY(project_id,task_id) REFERENCES tasks(project_id,id);
CREATE UNIQUE INDEX agent_sessions_identity_task ON agent_sessions(task_id,identity_id) WHERE task_id IS NOT NULL;
CREATE TABLE task_analyses(
 project_id uuid NOT NULL REFERENCES projects(id),id uuid PRIMARY KEY,
 task_id uuid NOT NULL,batch_id uuid UNIQUE REFERENCES discussion_batches(id),
 source_type text NOT NULL CHECK(source_type IN ('report','submission')),source_id uuid NOT NULL,
 state text NOT NULL DEFAULT 'queued',summary text NOT NULL DEFAULT '',
 disagreements jsonb NOT NULL DEFAULT '[]',basis jsonb NOT NULL DEFAULT '[]',discussion_json jsonb NOT NULL DEFAULT '{}',output_json jsonb NOT NULL DEFAULT '{}',
 error_code text,parent_topic_id uuid REFERENCES topics(id),fork_after_seq bigint NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,
 UNIQUE(project_id,id),UNIQUE(project_id,source_type,source_id),
 FOREIGN KEY(project_id,task_id) REFERENCES tasks(project_id,id)
);
CREATE TABLE discussion_suggestions(
 project_id uuid NOT NULL REFERENCES projects(id),id uuid PRIMARY KEY,
 analysis_id uuid NOT NULL UNIQUE REFERENCES task_analyses(id),task_id uuid NOT NULL,
 title text NOT NULL,reason text NOT NULL,parent_topic_id uuid,fork_after_seq bigint NOT NULL DEFAULT 0,
 suggested_topic_id uuid,state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','handled','dismissed')),
 version bigint NOT NULL DEFAULT 1,result_topic_id uuid,resolved_by uuid REFERENCES users(id),
 created_at timestamptz NOT NULL,resolved_at timestamptz,
 FOREIGN KEY(project_id,task_id) REFERENCES tasks(project_id,id),
 FOREIGN KEY(project_id,parent_topic_id) REFERENCES topics(project_id,id),
 FOREIGN KEY(project_id,suggested_topic_id) REFERENCES topics(project_id,id),
 FOREIGN KEY(project_id,result_topic_id) REFERENCES topics(project_id,id)
);
CREATE TABLE topic_source_refs(
 project_id uuid NOT NULL REFERENCES projects(id),topic_id uuid NOT NULL,
 source_type text NOT NULL CHECK(source_type IN ('report','submission','message')),source_id uuid NOT NULL,
 created_at timestamptz NOT NULL,PRIMARY KEY(topic_id,source_type,source_id),
 FOREIGN KEY(project_id,topic_id) REFERENCES topics(project_id,id)
);
