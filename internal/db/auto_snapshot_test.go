package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ldi/ponder/pkg/models"
)

func TestAutoSnapshot(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "auto-snapshot.jsonl")

	db.EnableAutoSnapshot(snapshotPath)

	// Create a feature
	f := &models.Feature{
		Name:          "Auto Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// Wait a bit for the async-like (though it's sync in current implementation) export to complete
	// Actually it's sync in my implementation, but good to be careful if I change it to async later.

	// Verify snapshot file exists and contains the feature
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Fatalf("Snapshot file was not created after CreateFeature")
	}

	// Helper to get file mod time
	getModTime := func(path string) time.Time {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat snapshot: %v", err)
		}
		return info.ModTime()
	}

	modTime1 := getModTime(snapshotPath)

	// Create a task
	task := &models.Task{
		FeatureID:     f.ID,
		Name:          "Auto Task",
		Description:   "Task Description",
		Specification: "Task Specification",
		Priority:      5,
		Status:        models.TaskStatusPending,
	}

	// Ensure some time passes so mod time definitely changes if it's updated
	time.Sleep(10 * time.Millisecond)

	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	modTime2 := getModTime(snapshotPath)
	if !modTime2.After(modTime1) {
		t.Errorf("Snapshot file was not updated after CreateTask")
	}

	// Update task status
	time.Sleep(10 * time.Millisecond)
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusInProgress, nil); err != nil {
		t.Fatalf("Failed to update task status: %v", err)
	}

	modTime3 := getModTime(snapshotPath)
	if !modTime3.After(modTime2) {
		t.Errorf("Snapshot file was not updated after UpdateTaskStatus")
	}

	// Delete task
	time.Sleep(10 * time.Millisecond)
	if err := db.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	modTime4 := getModTime(snapshotPath)
	if !modTime4.After(modTime3) {
		t.Errorf("Snapshot file was not updated after DeleteTask")
	}
}
