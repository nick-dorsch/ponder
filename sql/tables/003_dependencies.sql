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
