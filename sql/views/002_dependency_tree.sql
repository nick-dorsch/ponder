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
