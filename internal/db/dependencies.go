package db

import (
	"context"
	"fmt"

	"github.com/nick-dorsch/ponder/pkg/models"
)

func (db *DB) CreateDependency(ctx context.Context, taskID, dependsOnTaskID string) error {
	if err := db.createDependency(ctx, db.DB, taskID, dependsOnTaskID); err != nil {
		return err
	}
	db.triggerChange(ctx)
	return nil
}

func (db *DB) DeleteDependency(ctx context.Context, taskID, dependsOnTaskID string) error {
	query := `DELETE FROM dependencies WHERE task_id = ? AND depends_on_task_id = ?`
	res, err := db.ExecContext(ctx, query, taskID, dependsOnTaskID)
	if err != nil {
		return fmt.Errorf("failed to delete dependency: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("dependency not found: %s -> %s", taskID, dependsOnTaskID)
	}

	db.triggerChange(ctx)
	return nil
}

func (db *DB) GetDependencies(ctx context.Context, taskID string) ([]*models.Task, error) {
	query := `
		SELECT t.id, t.feature_id, t.name, t.description, t.specification, t.priority, t.tests_required, 
		       t.status, t.completion_summary, t.created_at, t.updated_at, t.started_at, t.completed_at,
		       f.name as feature_name
		FROM tasks t
		JOIN dependencies d ON t.id = d.depends_on_task_id
		LEFT JOIN features f ON t.feature_id = f.id
		WHERE d.task_id = ?
		ORDER BY t.priority DESC, t.created_at ASC
	`
	return db.queryTasks(ctx, query, taskID)
}

func (db *DB) GetDependents(ctx context.Context, taskID string) ([]*models.Task, error) {
	query := `
		SELECT t.id, t.feature_id, t.name, t.description, t.specification, t.priority, t.tests_required, 
		       t.status, t.completion_summary, t.created_at, t.updated_at, t.started_at, t.completed_at,
		       f.name as feature_name
		FROM tasks t
		JOIN dependencies d ON t.id = d.task_id
		LEFT JOIN features f ON t.feature_id = f.id
		WHERE d.depends_on_task_id = ?
		ORDER BY t.priority DESC, t.created_at ASC
	`
	return db.queryTasks(ctx, query, taskID)
}
