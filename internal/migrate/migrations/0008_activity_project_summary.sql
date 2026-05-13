ALTER TABLE activity_log ADD COLUMN project_id TEXT NOT NULL DEFAULT '';
ALTER TABLE activity_log ADD COLUMN summary TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_activity_project_created ON activity_log(project_id, created_at DESC);
