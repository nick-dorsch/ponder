package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nick-dorsch/ponder/pkg/models"
)

func (db *DB) CommitBatch(ctx context.Context, sessionID string) error {
	items := db.Staging.GetAndClear(sessionID)
	if items == nil {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	featureIDs := make(map[string]string)
	taskIDs := make(map[string]string)

	// 1. Features
	for _, f := range items.Features {
		if err := db.createFeature(ctx, tx, f); err != nil {
			return fmt.Errorf("failed to create staged feature %s: %w", f.Name, err)
		}
		featureIDs[f.Name] = f.ID
	}

	// 2. Tasks
	for _, t := range items.Tasks {
		// Resolve feature ID if it was also staged, otherwise look it up
		if t.FeatureID == "" && t.FeatureName != "" {
			if id, ok := featureIDs[t.FeatureName]; ok {
				t.FeatureID = id
			} else {
				f, err := db.getFeatureByName(ctx, tx, t.FeatureName)
				if err != nil {
					return fmt.Errorf("failed to resolve feature %s for task %s: %w", t.FeatureName, t.Name, err)
				}
				if f == nil {
					return fmt.Errorf("feature %s not found for task %s", t.FeatureName, t.Name)
				}
				t.FeatureID = f.ID
			}
		}

		if err := db.createTask(ctx, tx, t); err != nil {
			return fmt.Errorf("failed to create staged task %s: %w", t.Name, err)
		}
		taskIDs[fmt.Sprintf("%s:%s", t.FeatureName, t.Name)] = t.ID
	}

	// 3. Dependencies
	for _, d := range items.Dependencies {
		// Resolve task IDs
		if d.TaskID == "" {
			key := fmt.Sprintf("%s:%s", d.FeatureName, d.TaskName)
			if id, ok := taskIDs[key]; ok {
				d.TaskID = id
			} else {
				id, err := db.resolveTaskIDTx(ctx, tx, d.FeatureName, d.TaskName)
				if err != nil {
					return fmt.Errorf("failed to resolve task %s for dependency: %w", key, err)
				}
				d.TaskID = id
			}
		}

		if d.DependsOnTaskID == "" {
			key := fmt.Sprintf("%s:%s", d.DependsOnFeatureName, d.DependsOnTaskName)
			if id, ok := taskIDs[key]; ok {
				d.DependsOnTaskID = id
			} else {
				id, err := db.resolveTaskIDTx(ctx, tx, d.DependsOnFeatureName, d.DependsOnTaskName)
				if err != nil {
					return fmt.Errorf("failed to resolve depends_on task %s for dependency: %w", key, err)
				}
				d.DependsOnTaskID = id
			}
		}

		if err := db.createDependency(ctx, tx, d.TaskID, d.DependsOnTaskID); err != nil {
			return fmt.Errorf("failed to create staged dependency: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	db.triggerChange(ctx)
	return nil
}

func (db *DB) resolveTaskIDTx(ctx context.Context, exec executor, featureName, taskName string) (string, error) {
	f, err := db.getFeatureByName(ctx, exec, featureName)
	if err != nil {
		return "", err
	}
	if f == nil {
		return "", fmt.Errorf("feature %s not found", featureName)
	}
	t, err := db.getTaskByName(ctx, exec, taskName, f.ID)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", fmt.Errorf("task %s not found in feature %s", taskName, featureName)
	}
	return t.ID, nil
}

func (db *DB) createFeature(ctx context.Context, exec executor, f *models.Feature) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}

	query := `
		INSERT INTO features (id, name, description, specification)
		VALUES (?, ?, ?, ?)
		RETURNING created_at, updated_at
	`
	err := exec.QueryRowContext(ctx, query, f.ID, f.Name, f.Description, f.Specification).Scan(&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create feature: %w", err)
	}
	return nil
}

func (db *DB) createTask(ctx context.Context, exec executor, t *models.Task) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}

	testsRequired := 0
	if t.TestsRequired {
		testsRequired = 1
	}

	query := `
		INSERT INTO tasks (id, feature_id, name, description, specification, priority, tests_required, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING created_at, updated_at
	`
	err := exec.QueryRowContext(ctx, query,
		t.ID, t.FeatureID, t.Name, t.Description, t.Specification, t.Priority, testsRequired, t.Status,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

func (db *DB) createDependency(ctx context.Context, exec executor, taskID, dependsOnTaskID string) error {
	query := `INSERT INTO dependencies (task_id, depends_on_task_id) VALUES (?, ?)`
	_, err := exec.ExecContext(ctx, query, taskID, dependsOnTaskID)
	if err != nil {
		return fmt.Errorf("failed to create dependency: %w", err)
	}
	return nil
}
