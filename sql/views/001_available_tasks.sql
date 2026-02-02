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
