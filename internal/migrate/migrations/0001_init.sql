CREATE TABLE IF NOT EXISTS projects (
  id           TEXT PRIMARY KEY,
  alias        TEXT NOT NULL UNIQUE,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  repo_path    TEXT NOT NULL DEFAULT '',
  archived_at  INTEGER,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
  id            TEXT PRIMARY KEY,
  project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  type          TEXT NOT NULL DEFAULT 'task',
  status        TEXT NOT NULL DEFAULT 'todo',
  priority      INTEGER NOT NULL DEFAULT 3,
  assignee      TEXT NOT NULL DEFAULT '',
  due_at        INTEGER,
  actor_kind    TEXT NOT NULL DEFAULT 'human',
  actor_name    TEXT NOT NULL DEFAULT '',
  source        TEXT NOT NULL DEFAULT '',
  external_ref  TEXT NOT NULL DEFAULT '',
  completed_at  INTEGER,
  archived_at   INTEGER,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  CHECK(type IN ('task','bug','issue','improvement','feature','vulnerability','chore','spike')),
  CHECK(status IN ('todo','doing','done')),
  CHECK(priority BETWEEN 1 AND 5),
  CHECK(actor_kind IN ('human','ai'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);
CREATE INDEX IF NOT EXISTS idx_tasks_actor ON tasks(actor_kind, actor_name);

CREATE TABLE IF NOT EXISTS plans (
  id               TEXT PRIMARY KEY,
  task_id          TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  current_version  INTEGER NOT NULL DEFAULT 1,
  source           TEXT NOT NULL DEFAULT '',
  archived_at      INTEGER,
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_plans_task ON plans(task_id);

CREATE TABLE IF NOT EXISTS plan_versions (
  id           TEXT PRIMARY KEY,
  plan_id      TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  version      INTEGER NOT NULL,
  title        TEXT NOT NULL,
  body         TEXT NOT NULL,
  actor_kind   TEXT NOT NULL DEFAULT 'human',
  actor_name   TEXT NOT NULL DEFAULT '',
  change_note  TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  UNIQUE(plan_id, version),
  CHECK(actor_kind IN ('human','ai'))
);

CREATE INDEX IF NOT EXISTS idx_plan_versions_plan ON plan_versions(plan_id, version DESC);

CREATE TABLE IF NOT EXISTS comments (
  id           TEXT PRIMARY KEY,
  task_id      TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  body         TEXT NOT NULL,
  actor_kind   TEXT NOT NULL DEFAULT 'human',
  actor_name   TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  CHECK(actor_kind IN ('human','ai'))
);

CREATE INDEX IF NOT EXISTS idx_comments_task ON comments(task_id);

CREATE TABLE IF NOT EXISTS labels (
  id          TEXT PRIMARY KEY,
  project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  color       TEXT NOT NULL DEFAULT '',
  UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS task_labels (
  task_id   TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  label_id  TEXT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
  PRIMARY KEY(task_id, label_id)
);

CREATE TABLE IF NOT EXISTS activity_log (
  id           TEXT PRIMARY KEY,
  entity       TEXT NOT NULL,
  entity_id    TEXT NOT NULL,
  action       TEXT NOT NULL,
  actor_kind   TEXT NOT NULL DEFAULT 'human',
  actor_name   TEXT NOT NULL DEFAULT '',
  payload      TEXT NOT NULL DEFAULT '{}',
  created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_activity_entity ON activity_log(entity, entity_id);

CREATE VIRTUAL TABLE IF NOT EXISTS tasks_fts USING fts5(
  title, description,
  content='tasks',
  content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS tasks_ai AFTER INSERT ON tasks BEGIN
  INSERT INTO tasks_fts(rowid, title, description) VALUES (new.rowid, new.title, new.description);
END;

CREATE TRIGGER IF NOT EXISTS tasks_ad AFTER DELETE ON tasks BEGIN
  INSERT INTO tasks_fts(tasks_fts, rowid, title, description) VALUES('delete', old.rowid, old.title, old.description);
END;

CREATE TRIGGER IF NOT EXISTS tasks_au AFTER UPDATE ON tasks BEGIN
  INSERT INTO tasks_fts(tasks_fts, rowid, title, description) VALUES('delete', old.rowid, old.title, old.description);
  INSERT INTO tasks_fts(rowid, title, description) VALUES (new.rowid, new.title, new.description);
END;

CREATE TABLE IF NOT EXISTS schema_meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_meta(key, value) VALUES ('schema_version', '1');
