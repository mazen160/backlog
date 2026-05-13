CREATE TABLE IF NOT EXISTS attachments (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    mime_type   TEXT NOT NULL DEFAULT 'application/octet-stream',
    size        INTEGER NOT NULL DEFAULT 0,
    data        BLOB NOT NULL,
    linked_type TEXT NOT NULL DEFAULT '' CHECK(linked_type IN ('','task','doc')),
    linked_id   TEXT NOT NULL DEFAULT '',
    actor_kind  TEXT NOT NULL DEFAULT 'human' CHECK(actor_kind IN ('human','ai')),
    actor_name  TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attachments_linked ON attachments(linked_type, linked_id, created_at DESC);
