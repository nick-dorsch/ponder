package orchestrator

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/nick-dorsch/ponder/internal/db"
	"github.com/nick-dorsch/ponder/pkg/models"
)

func TestLifecycleIntegration_Recovery(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	f := &models.Feature{
		Name:          "Recovery Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := store.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	task := &models.Task{
		FeatureID:     f.ID,
		Name:          "Recovered Task",
		Description:   "Task Description",
		Specification: "Task Specification",
		Priority:      5,
		Status:        models.TaskStatusPending,
	}
	if err := store.CreateTask(ctx, task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if err := store.UpdateTaskStatus(ctx, task.ID, models.TaskStatusInProgress, nil); err != nil {
		t.Fatalf("Failed to set task to in_progress: %v", err)
	}

	t1, _ := store.GetTask(ctx, task.ID)
	if t1.Status != models.TaskStatusInProgress {
		t.Fatalf("Expected status in_progress, got %s", t1.Status)
	}

	o := NewOrchestrator(store, 1, "test-model")
	o.minSpawnInterval = 0

	runCount := 0
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		runCount++
		summary := "Processed by mock"
		_ = store.UpdateTaskStatus(ctx, task.ID, models.TaskStatusCompleted, &summary)
		return exec.CommandContext(ctx, "true")
	}

	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = o.Start(testCtx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("Start returned error: %v", err)
	}

	if runCount == 0 {
		t.Error("Expected task to be run by orchestrator after recovery")
	}

	finalTask, err := store.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if finalTask.Status != models.TaskStatusCompleted {
		t.Errorf("Expected final status completed, got %s", finalTask.Status)
	}
}
