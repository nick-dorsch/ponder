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
