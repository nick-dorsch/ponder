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
