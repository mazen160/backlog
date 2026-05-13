CREATE TABLE IF NOT EXISTS project_docs (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    current_version INTEGER NOT NULL DEFAULT 1,
    actor_kind  TEXT NOT NULL DEFAULT 'human' CHECK(actor_kind IN ('human','ai')),
    actor_name  TEXT NOT NULL DEFAULT '',
    archived_at INTEGER,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS project_doc_versions (
    id          TEXT PRIMARY KEY,
    doc_id      TEXT NOT NULL REFERENCES project_docs(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    actor_kind  TEXT NOT NULL DEFAULT 'human' CHECK(actor_kind IN ('human','ai')),
    actor_name  TEXT NOT NULL DEFAULT '',
    change_note TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    UNIQUE(doc_id, version)
);

CREATE INDEX IF NOT EXISTS idx_project_docs_project ON project_docs(project_id, created_at DESC);
