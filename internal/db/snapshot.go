package db

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/ldi/ponder/pkg/models"
)

// EnableAutoSnapshot sets up a hook that automatically exports a snapshot
// to the given path after every successful write operation.
func (db *DB) EnableAutoSnapshot(path string) {
	db.SetOnChange(func(ctx context.Context) {
		// We ignore the error here as hooks are best-effort in this context,
		// and we don't want to fail the original write operation if the export fails.
		_ = db.ExportSnapshot(ctx, path)
	})
}

// ExportSnapshot queries the v_snapshot_jsonl_lines view and writes the results
// to the given path atomically using a temporary file.
func (db *DB) ExportSnapshot(ctx context.Context, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, "snapshot-*.jsonl")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}
	}()

	rows, err := db.QueryContext(ctx, `
		SELECT json_line 
		FROM v_snapshot_jsonl_lines 
		ORDER BY record_order, sort_name, sort_secondary
	`)
	if err != nil {
		return fmt.Errorf("failed to query snapshot lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return fmt.Errorf("failed to scan snapshot line: %w", err)
		}
		if _, err := tempFile.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write snapshot line: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	filename := tempFile.Name()
	tempFile = nil // Prevent defer from removing it

	if err := os.Rename(filename, path); err != nil {
		os.Remove(filename)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// ImportSnapshot reads a JSONL snapshot and populates the database.
// It uses a transaction and maintains referential integrity by mapping names to new IDs.
func (db *DB) ImportSnapshot(ctx context.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Maps to translate snapshot IDs to local IDs
	featureSnapshotIDToLocalID := make(map[string]string)
	taskSnapshotIDToLocalID := make(map[string]string)

	// Maps to look up existing records by name
	featureNameMap := make(map[string]string)
	taskNameMap := make(map[string]string)

	// Load existing features
	err = func() error {
		rows, err := tx.QueryContext(ctx, "SELECT id, name FROM features")
		if err != nil {
			return fmt.Errorf("failed to query features: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				return err
			}
			featureNameMap[name] = id
		}
		return rows.Err()
	}()
	if err != nil {
		return err
	}

	// Load existing tasks
	err = func() error {
		rows, err := tx.QueryContext(ctx, "SELECT t.id, t.name, f.name FROM tasks t JOIN features f ON t.feature_id = f.id")
		if err != nil {
			return fmt.Errorf("failed to query tasks: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id, name, featureName string
			if err := rows.Scan(&id, &name, &featureName); err != nil {
				return err
			}
			taskNameMap[featureName+"/"+name] = id
		}
		return rows.Err()
	}()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var base struct {
			RecordType string `json:"record_type"`
		}
		if err := json.Unmarshal(line, &base); err != nil {
			return fmt.Errorf("failed to unmarshal base record: %w", err)
		}

		switch base.RecordType {
		case "meta":
			// Skip meta
		case "feature":
			var f models.Feature
			if err := json.Unmarshal(line, &f); err != nil {
				return fmt.Errorf("failed to unmarshal feature: %w", err)
			}

			localID, exists := featureNameMap[f.Name]
			if exists {
				_, err = tx.ExecContext(ctx, `
					UPDATE features 
					SET description = ?, specification = ?, created_at = ?, updated_at = ?
					WHERE id = ?`,
					f.Description, f.Specification, f.CreatedAt, f.UpdatedAt, localID)
			} else {
				if f.ID == "" {
					f.ID = uuid.New().String()
				}
				localID = f.ID
				_, err = tx.ExecContext(ctx, `
					INSERT INTO features (id, name, description, specification, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?)`,
					f.ID, f.Name, f.Description, f.Specification, f.CreatedAt, f.UpdatedAt)
			}
			if err != nil {
				return fmt.Errorf("failed to sync feature %s: %w", f.Name, err)
			}
			if f.ID != "" {
				featureSnapshotIDToLocalID[f.ID] = localID
			}
			featureNameMap[f.Name] = localID

		case "task":
			var t struct {
				ID                string            `json:"id"`
				Name              string            `json:"name"`
				Description       string            `json:"description"`
				Specification     string            `json:"specification"`
				FeatureName       string            `json:"feature_name"`
				TestsRequired     bool              `json:"tests_required"`
				Priority          int               `json:"priority"`
				Status            models.TaskStatus `json:"status"`
				CompletionSummary *string           `json:"completion_summary"`
				CreatedAt         time.Time         `json:"created_at"`
				UpdatedAt         time.Time         `json:"updated_at"`
				StartedAt         *time.Time        `json:"started_at"`
				CompletedAt       *time.Time        `json:"completed_at"`
			}
			if err := json.Unmarshal(line, &t); err != nil {
				return fmt.Errorf("failed to unmarshal task: %w", err)
			}

			featureID, ok := featureNameMap[t.FeatureName]
			if !ok {
				return fmt.Errorf("feature not found for task %s: %s", t.Name, t.FeatureName)
			}

			localID, exists := taskNameMap[t.FeatureName+"/"+t.Name]
			testsRequired := 0
			if t.TestsRequired {
				testsRequired = 1
			}

			if exists {
				_, err = tx.ExecContext(ctx, `
					UPDATE tasks SET 
						feature_id = ?, description = ?, specification = ?, priority = ?, 
						tests_required = ?, status = ?, completion_summary = ?, created_at = ?, 
						updated_at = ?, started_at = ?, completed_at = ?
					WHERE id = ?`,
					featureID, t.Description, t.Specification, t.Priority,
					testsRequired, t.Status, t.CompletionSummary, t.CreatedAt,
					t.UpdatedAt, t.StartedAt, t.CompletedAt, localID)
			} else {
				if t.ID == "" {
					t.ID = uuid.New().String()
				}
				localID = t.ID
				_, err = tx.ExecContext(ctx, `
					INSERT INTO tasks (
						id, feature_id, name, description, specification, priority, 
						tests_required, status, completion_summary, created_at, 
						updated_at, started_at, completed_at
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					t.ID, featureID, t.Name, t.Description, t.Specification, t.Priority,
					testsRequired, t.Status, t.CompletionSummary, t.CreatedAt,
					t.UpdatedAt, t.StartedAt, t.CompletedAt)
			}
			if err != nil {
				return fmt.Errorf("failed to sync task %s: %w", t.Name, err)
			}
			if t.ID != "" {
				taskSnapshotIDToLocalID[t.ID] = localID
			}
			taskNameMap[t.FeatureName+"/"+t.Name] = localID

		case "dependency":
			var d struct {
				TaskID                   string `json:"task_id"`
				TaskName                 string `json:"task_name"`
				TaskFeatureName          string `json:"task_feature_name"`
				DependsOnTaskID          string `json:"depends_on_task_id"`
				DependsOnTaskName        string `json:"depends_on_task_name"`
				DependsOnTaskFeatureName string `json:"depends_on_task_feature_name"`
			}
			if err := json.Unmarshal(line, &d); err != nil {
				return fmt.Errorf("failed to unmarshal dependency: %w", err)
			}

			localTaskID, ok := taskSnapshotIDToLocalID[d.TaskID]
			if !ok {
				localTaskID, ok = taskNameMap[d.TaskFeatureName+"/"+d.TaskName]
			}
			if !ok {
				return fmt.Errorf("task not found for dependency: %s/%s", d.TaskFeatureName, d.TaskName)
			}

			localDependsOnID, ok := taskSnapshotIDToLocalID[d.DependsOnTaskID]
			if !ok {
				localDependsOnID, ok = taskNameMap[d.DependsOnTaskFeatureName+"/"+d.DependsOnTaskName]
			}
			if !ok {
				return fmt.Errorf("dependent task not found for dependency: %s/%s", d.DependsOnTaskFeatureName, d.DependsOnTaskName)
			}

			_, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO dependencies (task_id, depends_on_task_id) VALUES (?, ?)", localTaskID, localDependsOnID)
			if err != nil {
				return fmt.Errorf("failed to insert dependency %s -> %s: %w", d.TaskName, d.DependsOnTaskName, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	db.triggerChange(ctx)
	return nil
}
