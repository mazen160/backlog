CREATE TABLE IF NOT EXISTS project_memory (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    actor_kind  TEXT NOT NULL DEFAULT 'human' CHECK(actor_kind IN ('human','ai')),
    actor_name  TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_project_memory_project ON project_memory(project_id, created_at DESC);
