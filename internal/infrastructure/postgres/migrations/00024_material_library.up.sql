-- +goose Up
ALTER TABLE materials ADD COLUMN owner_user_id uuid REFERENCES users(id), ADD COLUMN purpose text NOT NULL DEFAULT '', ADD COLUMN deleted_at timestamptz, ADD COLUMN deleted_by uuid REFERENCES users(id);
ALTER TABLE upload_sessions ADD COLUMN purpose text NOT NULL DEFAULT '';
ALTER TABLE material_versions ADD COLUMN format text NOT NULL DEFAULT 'other', ADD COLUMN upload_source text NOT NULL DEFAULT 'unknown';
ALTER TABLE submissions ADD COLUMN plan_id uuid REFERENCES plans(id);
ALTER TABLE work_reports ADD COLUMN plan_id uuid REFERENCES plans(id);
UPDATE work_reports r SET plan_id=t.plan_id FROM tasks t WHERE t.id=r.task_id;
UPDATE materials m SET owner_user_id=(SELECT v.author_id FROM material_versions v WHERE v.material_id=m.id ORDER BY v.revision LIMIT 1);
UPDATE material_versions v SET format=CASE lower(regexp_replace(m.title,'^.*\.','')) WHEN 'pdf' THEN 'pdf' WHEN 'docx' THEN 'docx' WHEN 'xlsx' THEN 'xlsx' WHEN 'pptx' THEN 'pptx' WHEN 'png' THEN 'image' WHEN 'jpg' THEN 'image' WHEN 'jpeg' THEN 'image' WHEN 'webp' THEN 'image' WHEN 'gif' THEN 'image' WHEN 'md' THEN 'markdown' WHEN 'txt' THEN 'text' WHEN 'json' THEN 'json' WHEN 'zip' THEN 'archive' ELSE 'other' END FROM materials m WHERE m.id=v.material_id;
CREATE TABLE material_previews (
 project_id uuid NOT NULL REFERENCES projects(id), version_id uuid PRIMARY KEY REFERENCES material_versions(id),
 status text NOT NULL CHECK(status IN ('pending','ready','failed','unsupported')), kind text NOT NULL,
 object_key text, mime text NOT NULL DEFAULT '', size bigint NOT NULL DEFAULT 0, error text,
 job_id uuid REFERENCES background_jobs(id) ON DELETE SET NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE INDEX material_library_project ON materials(project_id,created_at,id) WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW material_usage_records AS
 SELECT rm.project_id,rm.material_version_id AS version_id,r.id AS source_id,r.report_kind AS kind,
 r.progress_hint AS description,r.created_by AS actor_user_id,r.created_at,r.source,t.id AS task_id,COALESCE(r.plan_id,t.plan_id) AS plan_id,NULL::uuid AS topic_id
 FROM work_report_materials rm JOIN work_reports r ON r.id=rm.report_id JOIN tasks t ON t.id=r.task_id
 UNION ALL
 SELECT sm.project_id,sm.material_version_id,s.id,s.purpose,s.text,s.actor_user_id,s.created_at,s.source,s.task_id,
 COALESCE(s.plan_id,t.plan_id,CASE WHEN o.context_type='plan' THEN o.context_id END),s.topic_id
 FROM submission_materials sm JOIN submissions s ON s.id=sm.submission_id LEFT JOIN tasks t ON t.id=s.task_id LEFT JOIN topics o ON o.id=s.topic_id
 WHERE NOT EXISTS(SELECT 1 FROM work_reports r WHERE r.submission_id=s.id)
 UNION ALL
 SELECT ml.project_id,ml.version_id,ml.id,'reference',ml.purpose,ml.confirmed_by,ml.created_at,'web',
 CASE WHEN ml.object_type='task' THEN ml.object_id END,CASE WHEN ml.object_type='plan' THEN ml.object_id ELSE t.plan_id END,
 CASE WHEN ml.object_type='topic' THEN ml.object_id END
 FROM material_links ml LEFT JOIN tasks t ON ml.object_type='task' AND t.id=ml.object_id;
