-- Redesign project_memory: drop key/value columns, add free-form body + tags.
-- Existing rows are migrated: body = key || ': ' || value, tags = ''.
CREATE TABLE IF NOT EXISTS project_memory_new (
    id         TEXT    PRIMARY KEY,
    project_id TEXT    NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    body       TEXT    NOT NULL,
    tags       TEXT    NOT NULL DEFAULT '',
    actor_kind TEXT    NOT NULL DEFAULT 'human' CHECK(actor_kind IN ('human','ai')),
    actor_name TEXT    NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

INSERT INTO project_memory_new(id, project_id, body, tags, actor_kind, actor_name, created_at)
    SELECT id, project_id,
           CASE WHEN key != '' THEN key || ': ' || value ELSE value END,
           '',
           actor_kind, actor_name, created_at
    FROM project_memory;

DROP TABLE project_memory;
ALTER TABLE project_memory_new RENAME TO project_memory;

CREATE INDEX IF NOT EXISTS idx_project_memory_project
    ON project_memory(project_id, created_at DESC);
