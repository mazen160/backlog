ALTER TABLE tasks ADD COLUMN task_seq INTEGER;

UPDATE tasks SET task_seq = (
  SELECT rn FROM (
    SELECT id, ROW_NUMBER() OVER (ORDER BY created_at ASC, id ASC) AS rn
    FROM tasks
  ) AS ranked
  WHERE ranked.id = tasks.id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_task_seq ON tasks(task_seq);
