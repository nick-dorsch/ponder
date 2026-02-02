package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ldi/ponder/pkg/models"
)

// CreateTask inserts a new task into the database.
// If t.ID is empty, a new UUID (hex) is generated.
func (db *DB) CreateTask(ctx context.Context, t *models.Task) error {
	if err := db.createTask(ctx, db.DB, t); err != nil {
		return err
	}

	db.triggerChange(ctx)
	return nil
}

// GetTask retrieves a task by its ID.
func (db *DB) GetTask(ctx context.Context, id string) (*models.Task, error) {
	query := `
		SELECT t.id, t.feature_id, t.name, t.description, t.specification, t.priority, t.tests_required, 
		       t.status, t.completion_summary, t.created_at, t.updated_at, t.started_at, t.completed_at,
		       f.name as feature_name
		FROM tasks t
		LEFT JOIN features f ON t.feature_id = f.id
		WHERE t.id = ?
	`
	t := &models.Task{}
	var testsRequired int
	err := db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.FeatureID, &t.Name, &t.Description, &t.Specification, &t.Priority, &testsRequired,
		&t.Status, &t.CompletionSummary, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
		&t.FeatureName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	t.TestsRequired = testsRequired == 1
	return t, nil
}

// GetTaskByName retrieves a task by its name and feature_id.
func (db *DB) GetTaskByName(ctx context.Context, name string, featureID string) (*models.Task, error) {
	return db.getTaskByName(ctx, db.DB, name, featureID)
}

func (db *DB) getTaskByName(ctx context.Context, exec executor, name string, featureID string) (*models.Task, error) {
	query := `
		SELECT t.id, t.feature_id, t.name, t.description, t.specification, t.priority, t.tests_required, 
		       t.status, t.completion_summary, t.created_at, t.updated_at, t.started_at, t.completed_at,
		       f.name as feature_name
		FROM tasks t
		LEFT JOIN features f ON t.feature_id = f.id
		WHERE t.name = ? AND t.feature_id = ?
	`
	t := &models.Task{}
	var testsRequired int
	err := exec.QueryRowContext(ctx, query, name, featureID).Scan(
		&t.ID, &t.FeatureID, &t.Name, &t.Description, &t.Specification, &t.Priority, &testsRequired,
		&t.Status, &t.CompletionSummary, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
		&t.FeatureName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task by name: %w", err)
	}

	t.TestsRequired = testsRequired == 1
	return t, nil
}

// ListTasks returns tasks, optionally filtered by status or feature_name.
func (db *DB) ListTasks(ctx context.Context, status *models.TaskStatus, featureName *string) ([]*models.Task, error) {
	query := `
		SELECT t.id, t.feature_id, t.name, t.description, t.specification, t.priority, t.tests_required, 
		       t.status, t.completion_summary, t.created_at, t.updated_at, t.started_at, t.completed_at,
		       f.name as feature_name
		FROM tasks t
		LEFT JOIN features f ON t.feature_id = f.id
		WHERE 1=1
	`
	args := []interface{}{}

	if status != nil {
		query += " AND t.status = ?"
		args = append(args, *status)
	}

	if featureName != nil {
		query += " AND f.name = ?"
		args = append(args, *featureName)
	}

	query += " ORDER BY t.priority DESC, t.created_at ASC"

	return db.queryTasks(ctx, query, args...)
}

// queryTasks is a helper to execute a query that returns a list of tasks.
func (db *DB) queryTasks(ctx context.Context, query string, args ...interface{}) ([]*models.Task, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t := &models.Task{}
		var testsRequired int
		err := rows.Scan(
			&t.ID, &t.FeatureID, &t.Name, &t.Description, &t.Specification, &t.Priority, &testsRequired,
			&t.Status, &t.CompletionSummary, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
			&t.FeatureName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		t.TestsRequired = testsRequired == 1
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return tasks, nil
}

// UpdateTask updates an existing task.
func (db *DB) UpdateTask(ctx context.Context, t *models.Task) error {
	testsRequired := 0
	if t.TestsRequired {
		testsRequired = 1
	}

	query := `
		UPDATE tasks
		SET name = ?, description = ?, specification = ?, priority = ?, tests_required = ?, feature_id = ?
		WHERE id = ?
		RETURNING updated_at
	`
	err := db.QueryRowContext(ctx, query,
		t.Name, t.Description, t.Specification, t.Priority, testsRequired, t.FeatureID, t.ID,
	).Scan(&t.UpdatedAt)
	if err == sql.ErrNoRows {
		return fmt.Errorf("task not found: %s", t.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	db.triggerChange(ctx)
	return nil
}

// UpdateTaskStatus updates the status and completion summary of a task.
func (db *DB) UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, summary *string) error {
	// Validate status transition
	current, err := db.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("task not found: %s", id)
	}

	if err := validateStatusTransition(current.Status, status); err != nil {
		return err
	}

	query := `
		UPDATE tasks
		SET status = ?, completion_summary = ?
		WHERE id = ?
		RETURNING updated_at, started_at, completed_at
	`
	var t models.Task
	err = db.QueryRowContext(ctx, query, status, summary, id).Scan(&t.UpdatedAt, &t.StartedAt, &t.CompletedAt)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	db.triggerChange(ctx)
	return nil
}

// DeleteTask deletes a task by its ID.
func (db *DB) DeleteTask(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = ?`
	res, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	db.triggerChange(ctx)
	return nil
}

// GetAvailableTasks returns tasks that are ready to work on (all dependencies completed).
func (db *DB) GetAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	query := `
		SELECT id, feature_id, name, description, specification, priority, tests_required,
		       status, completion_summary, created_at, updated_at, started_at, completed_at,
		       feature_name
		FROM v_available_tasks
		ORDER BY priority DESC, created_at ASC
	`
	return db.queryTasks(ctx, query)
}

// CountAvailableTasks returns the number of tasks that are ready to work on
// without claiming them. This is used to determine how many workers to spawn.
func (db *DB) CountAvailableTasks(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM v_available_tasks
	`

	var count int
	err := db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count available tasks: %w", err)
	}

	return count, nil
}

// ClaimNextTask atomically claims the next available task by marking it as 'in_progress'.
// It uses an UPDATE ... RETURNING query to prevent race conditions where multiple
// workers might claim the same task. Returns nil if no tasks are available.
func (db *DB) ClaimNextTask(ctx context.Context) (*models.Task, error) {
	query := `
		UPDATE tasks
		SET status = 'in_progress'
		WHERE id IN (
			SELECT t.id
			FROM tasks t
			WHERE t.status = 'pending'
			  AND NOT EXISTS (
				SELECT 1
				FROM dependencies d
				JOIN tasks dep_task ON d.depends_on_task_id = dep_task.id
				WHERE d.task_id = t.id
				  AND dep_task.status != 'completed'
			)
			ORDER BY t.priority DESC, t.created_at ASC
			LIMIT 1
		)
		RETURNING id, feature_id, name, description, specification, priority, tests_required,
		          status, completion_summary, created_at, updated_at, started_at, completed_at
	`

	t := &models.Task{}
	var testsRequired int
	err := db.QueryRowContext(ctx, query).Scan(
		&t.ID, &t.FeatureID, &t.Name, &t.Description, &t.Specification, &t.Priority, &testsRequired,
		&t.Status, &t.CompletionSummary, &t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to claim next task: %w", err)
	}

	t.TestsRequired = testsRequired == 1
	db.triggerChange(ctx)
	return t, nil
}

// ResetInProgressTasks resets all tasks with status 'in_progress' to 'pending'.
// This is typically called on startup to recover from orphaned tasks.
func (db *DB) ResetInProgressTasks(ctx context.Context) error {
	query := `UPDATE tasks SET status = 'pending' WHERE status = 'in_progress'`
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to reset in_progress tasks: %w", err)
	}

	db.triggerChange(ctx)
	return nil
}

func validateStatusTransition(from, to models.TaskStatus) error {
	if from == to {
		return nil
	}

	switch from {
	case models.TaskStatusPending:
		if to != models.TaskStatusInProgress && to != models.TaskStatusBlocked {
			return fmt.Errorf("invalid transition from %s to %s", from, to)
		}
	case models.TaskStatusInProgress:
		if to != models.TaskStatusCompleted && to != models.TaskStatusBlocked && to != models.TaskStatusPending {
			return fmt.Errorf("invalid transition from %s to %s", from, to)
		}
	case models.TaskStatusCompleted:
		// Completed tasks can maybe be moved back to in_progress if needed, but usually they are final.
		// For now let's allow moving back to in_progress.
		if to != models.TaskStatusInProgress {
			return fmt.Errorf("invalid transition from %s to %s", from, to)
		}
	case models.TaskStatusBlocked:
		if to != models.TaskStatusPending && to != models.TaskStatusInProgress {
			return fmt.Errorf("invalid transition from %s to %s", from, to)
		}
	}

	return nil
}
