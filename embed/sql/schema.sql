-- Features table to track which features are available/enabled
CREATE TABLE IF NOT EXISTS features (
  id CHAR(36) PRIMARY KEY,
  name VARCHAR(55) NOT NULL UNIQUE,
  description TEXT NOT NULL,
  specification TEXT NOT NULL,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS set_features_updated_at
AFTER UPDATE ON features
WHEN NEW.updated_at = OLD.updated_at
BEGIN
  UPDATE features
  SET updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;

-- Seed the default feature (required for basic operation)
-- Note: id must be provided by the application (Go will generate UUIDs)
INSERT OR IGNORE INTO features (id, name, description, specification) VALUES
(
  '00000000-0000-0000-0000-000000000000',
  'misc',
  'Default feature for uncategorized tasks',
  'Use this feature in cases where a task is minimal and does not require a feature, such as minor hotfixes, tweaks etc.'
);
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
-- Dependencies are edges in the graph between tasks and other tasks they depend on
CREATE TABLE IF NOT EXISTS dependencies (
  task_id CHAR(36) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  depends_on_task_id CHAR(36) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  PRIMARY KEY (task_id, depends_on_task_id),
  CHECK (task_id != depends_on_task_id) -- Prevent self-dependencies
);

-- Trigger to prevent circular dependencies
CREATE TRIGGER IF NOT EXISTS prevent_circular_dependencies
BEFORE INSERT ON dependencies
BEGIN
  -- Check if this would create a circular dependency by recursively checking if
  -- depends_on_task_name already depends on task_name
  SELECT CASE
    WHEN EXISTS (
      WITH RECURSIVE dependency_chain AS (
        -- Start with the task we're trying to depend on
        SELECT depends_on_task_id AS task_id, 1 AS depth
        FROM dependencies
        WHERE task_id = NEW.depends_on_task_id
        
        UNION ALL
        
        -- Recursively follow all dependencies
        SELECT d.depends_on_task_id, dc.depth + 1
        FROM dependencies AS d
        JOIN dependency_chain AS dc ON d.task_id = dc.task_id
        WHERE dc.depth < 500 -- Prevent infinite recursion
      )
      SELECT 1 FROM dependency_chain WHERE task_id = NEW.task_id
    ) THEN
      RAISE(ABORT, 'Circular dependencies are not allowed!')
    END;
END;
-- View for tasks whose dependencies are all completed
DROP VIEW IF EXISTS v_available_tasks;

CREATE VIEW v_available_tasks AS
SELECT t.*, f.name AS feature_name
FROM tasks t
LEFT JOIN features f ON t.feature_id = f.id
WHERE t.status = 'pending'  -- Only tasks that are pending
  AND NOT EXISTS (
    -- Check for any uncompleted dependencies
    SELECT 1
    FROM dependencies d
    JOIN tasks dep_task ON d.depends_on_task_id = dep_task.id
    WHERE d.task_id = t.id
      AND dep_task.status != 'completed'
  )
  AND (
    -- Include tasks with no dependencies
    NOT EXISTS (
      SELECT 1 FROM dependencies d WHERE d.task_id = t.id
    )
    OR
    -- Or tasks that exist in dependencies table
    EXISTS (
      SELECT 1 FROM dependencies d WHERE d.task_id = t.id
    )
  )
ORDER BY t.priority DESC, t.created_at ASC;
-- Recursive view that computes the dependency tree with levels and paths
DROP VIEW IF EXISTS v_dependency_tree;

CREATE VIEW v_dependency_tree AS
WITH RECURSIVE task_tree AS (
    -- Root tasks (no dependencies)
    SELECT 
        t.id,
        t.name,
        t.description,
        t.status,
        t.priority,
        t.completed_at,
        0 as level,
        CAST(t.name AS TEXT) as path,
        CAST(NULL AS TEXT) as parent_name
    FROM tasks t
    WHERE NOT EXISTS (
        SELECT 1 FROM dependencies d 
        WHERE d.task_id = t.id
    )
    
    UNION ALL
    
    -- Dependent tasks (task depends on parent)
    SELECT 
        t.id,
        t.name,
        t.description,
        t.status,
        t.priority,
        t.completed_at,
        tt.level + 1,
        tt.path || '->' || t.name,
        tt.name as parent_name
    FROM tasks t
    JOIN dependencies d ON t.id = d.task_id
    JOIN task_tree tt ON d.depends_on_task_id = tt.id
)
SELECT 
    name,
    description,
    status,
    priority,
    completed_at,
    level,
    path,
    parent_name
FROM task_tree
ORDER BY path;
-- View that outputs the entire task graph as a JSON structure
-- Format: {"nodes": [...], "edges": [...]}
-- Each node includes an is_available flag indicating if all dependencies are complete
DROP VIEW IF EXISTS v_graph_json;

CREATE VIEW v_graph_json AS
SELECT json_object(
    'nodes', (
        SELECT json_group_array(
            json_object(
                'id', t.id,
                'name', t.name,
                'feature_name', f.name,
                'description', t.description,
                'status', t.status,
                'priority', t.priority,
                'completion_summary', t.completion_summary,
                'completed_at', t.completed_at,
                'started_at', t.started_at,
                'completion_seconds', CASE
                    WHEN t.started_at IS NULL OR t.completed_at IS NULL THEN NULL
                    ELSE CAST(ROUND((julianday(t.completed_at) - julianday(t.started_at)) * 24 * 60 * 60) AS INTEGER)
                END,
                'is_available', CASE WHEN t.id IN (SELECT id FROM v_available_tasks) THEN 1 ELSE 0 END
            )
        )
        FROM tasks t
        JOIN features f ON t.feature_id = f.id
    ),
    'edges', (
        SELECT json_group_array(
            json_object(
                'from', d.task_id,
                'to', d.depends_on_task_id
            )
        )
        FROM dependencies d
    )
) as graph_json;
-- View that emits deterministic JSONL snapshot lines using JSON1
-- Columns:
--   record_order: ordering bucket (meta=0, feature=1, task=2, dependency=3)
--   sort_name: primary sort key within bucket
--   sort_secondary: secondary sort key within bucket
--   json_line: JSON text for the snapshot line
DROP VIEW IF EXISTS v_snapshot_jsonl_lines;

CREATE VIEW v_snapshot_jsonl_lines AS
WITH meta AS (
  SELECT CURRENT_TIMESTAMP AS generated_at
)
SELECT
  0 AS record_order,
  '' AS sort_name,
  '' AS sort_secondary,
  json_object(
    'record_type', 'meta',
    'schema_version', '1',
    'generated_at', meta.generated_at,
    'source', 'sqlite'
  ) AS json_line
FROM meta

UNION ALL

SELECT
  1 AS record_order,
  f.name AS sort_name,
  '' AS sort_secondary,
  json_object(
    'record_type', 'feature',
    'id', f.id,
    'name', f.name,
    'description', f.description,
    'specification', f.specification,
    'created_at', strftime('%Y-%m-%dT%H:%M:%SZ', f.created_at),
    'updated_at', strftime('%Y-%m-%dT%H:%M:%SZ', f.updated_at)
  ) AS json_line
FROM features f

UNION ALL

SELECT
  2 AS record_order,
  t.name AS sort_name,
  '' AS sort_secondary,
  json_object(
    'record_type', 'task',
    'id', t.id,
    'name', t.name,
    'description', t.description,
    'specification', t.specification,
    'feature_name', f.name,
    'tests_required', json(CASE WHEN t.tests_required THEN 'true' ELSE 'false' END),
    'priority', t.priority,
    'status', t.status,
    'completion_summary', t.completion_summary,
    'created_at', strftime('%Y-%m-%dT%H:%M:%SZ', t.created_at),
    'updated_at', strftime('%Y-%m-%dT%H:%M:%SZ', t.updated_at),
    'started_at', strftime('%Y-%m-%dT%H:%M:%SZ', t.started_at),
    'completed_at', strftime('%Y-%m-%dT%H:%M:%SZ', t.completed_at)
  ) AS json_line
FROM tasks t
LEFT JOIN features f ON t.feature_id = f.id

UNION ALL

SELECT
  3 AS record_order,
  t.name AS sort_name,
  dep.name AS sort_secondary,
  json_object(
    'record_type', 'dependency',
    'task_id', t.id,
    'task_name', t.name,
    'task_feature_name', tf.name,
    'depends_on_task_id', dep.id,
    'depends_on_task_name', dep.name,
    'depends_on_task_feature_name', df.name
  ) AS json_line
FROM dependencies d
JOIN tasks t ON d.task_id = t.id
JOIN features tf ON t.feature_id = tf.id
JOIN tasks dep ON d.depends_on_task_id = dep.id
JOIN features df ON dep.feature_id = df.id;
