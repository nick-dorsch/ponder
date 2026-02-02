-- Each task is a node in the dependency graph
CREATE TABLE IF NOT EXISTS tasks (
  id CHAR(36) PRIMARY KEY,
  feature_id CHAR(36) NOT NULL REFERENCES features(id) ON DELETE CASCADE,

  name VARCHAR(55) NOT NULL,
  description TEXT NOT NULL,
  specification TEXT NOT NULL,

  priority INTEGER DEFAULT 0 CHECK(priority >= 0 AND priority <= 10),
  tests_required INTEGER NOT NULL DEFAULT 1 CHECK (tests_required IN (0, 1)),
  status TEXT DEFAULT 'pending' CHECK(
    status IN (
      'pending',
      'in_progress',
      'completed',
      'blocked'
    )
  ),
  completion_summary TEXT,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,

  CHECK (status != 'completed' OR completion_summary IS NOT NULL),
  UNIQUE(name, feature_id)
);

-- Triggers to automatically set timestamps based on status changes

-- Trigger to set started_at when status becomes 'in_progress'
CREATE TRIGGER IF NOT EXISTS set_started_at
AFTER UPDATE ON tasks
WHEN NEW.status = 'in_progress' AND OLD.status != 'in_progress'
BEGIN
    UPDATE tasks
    SET started_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;

-- Trigger to set completed_at when status becomes 'completed'
CREATE TRIGGER IF NOT EXISTS set_completed_at
AFTER UPDATE ON tasks
WHEN NEW.status = 'completed' AND OLD.status != 'completed'
BEGIN
    UPDATE tasks
    SET completed_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS set_tasks_updated_at
AFTER UPDATE ON tasks
WHEN NEW.updated_at = OLD.updated_at
BEGIN
  UPDATE tasks
  SET updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;
