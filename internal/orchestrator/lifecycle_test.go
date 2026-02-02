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
	// 1. Setup real database
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 2. Create a feature and a task
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

	// 3. Manually set task to in_progress (simulating crash)
	// We use the store's UpdateTaskStatus which validates transitions,
	// but we could also use direct SQL if we wanted to be more "manual".
	// Since pending -> in_progress is valid, this works.
	if err := store.UpdateTaskStatus(ctx, task.ID, models.TaskStatusInProgress, nil); err != nil {
		t.Fatalf("Failed to set task to in_progress: %v", err)
	}

	// Verify it's actually in_progress
	t1, _ := store.GetTask(ctx, task.ID)
	if t1.Status != models.TaskStatusInProgress {
		t.Fatalf("Expected status in_progress, got %s", t1.Status)
	}

	// 4. Initialize Orchestrator
	o := NewOrchestrator(store, 1, "test-model")
	o.minSpawnInterval = 0 // Fast spawning for test

	// Track if the task was actually run
	runCount := 0
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		runCount++
		// Simulate the agent completing the task
		summary := "Processed by mock"
		_ = store.UpdateTaskStatus(ctx, task.ID, models.TaskStatusCompleted, &summary)
		return exec.CommandContext(ctx, "true")
	}

	// 5. Start Orchestrator
	// It should reset in_progress to pending, then pick it up and run it.
	// We'll use a short context and expect it to finish or timeout.
	testCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// We'll run Start in a goroutine because it might block if there are no more tasks
	// (though here it should exit since we didn't set PollingInterval)
	err = o.Start(testCtx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Fatalf("Start returned error: %v", err)
	}

	// 6. Verify task was recovered and processed
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
