package db

import (
	"context"
	"strings"
	"testing"

	"github.com/nick-dorsch/ponder/pkg/models"
)

func TestTaskCRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 1. Create a feature first (required for task)
	f := &models.Feature{
		Name:          "Task Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// 2. Create Task
	task := &models.Task{
		FeatureID:     f.ID,
		Name:          "Test Task",
		Description:   "Task Description",
		Specification: "Task Specification",
		Priority:      5,
		TestsRequired: true,
		Status:        models.TaskStatusPending,
	}

	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if len(task.ID) != 36 {
		t.Errorf("Expected ID length 36, got %d (%s)", len(task.ID), task.ID)
	}

	// Verify ID contains dashes (standard UUID format)
	if !strings.Contains(task.ID, "-") {
		t.Errorf("Expected ID to contain dashes, got %s", task.ID)
	}

	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		t.Errorf("Expected CreatedAt and UpdatedAt to be set")
	}

	// 3. Get Task
	fetched, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if fetched == nil {
		t.Fatalf("Task not found")
	}
	if fetched.Name != task.Name {
		t.Errorf("Expected name %s, got %s", task.Name, fetched.Name)
	}
	if fetched.FeatureName != f.Name {
		t.Errorf("Expected feature name %s, got %s", f.Name, fetched.FeatureName)
	}
	if !fetched.TestsRequired {
		t.Errorf("Expected tests_required true")
	}

	// 4. Update Task
	task.Name = "Updated Task Name"
	task.Priority = 8
	if err := db.UpdateTask(ctx, task); err != nil {
		t.Fatalf("Failed to update task: %v", err)
	}

	fetched, err = db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if fetched.Name != "Updated Task Name" {
		t.Errorf("Expected name Updated Task Name, got %s", fetched.Name)
	}
	if fetched.Priority != 8 {
		t.Errorf("Expected priority 8, got %d", fetched.Priority)
	}

	// 5. Update Status
	summary := "Completed successfully"
	err = db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusInProgress, nil)
	if err != nil {
		t.Fatalf("Failed to update status to in_progress: %v", err)
	}

	fetched, err = db.GetTask(ctx, task.ID)
	if fetched.Status != models.TaskStatusInProgress {
		t.Errorf("Expected status in_progress, got %s", fetched.Status)
	}
	if fetched.StartedAt == nil {
		t.Errorf("Expected StartedAt to be set")
	}

	err = db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusCompleted, &summary)
	if err != nil {
		t.Fatalf("Failed to update status to completed: %v", err)
	}

	fetched, err = db.GetTask(ctx, task.ID)
	if fetched.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s", fetched.Status)
	}
	if fetched.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to be set")
	}
	if fetched.CompletionSummary == nil || *fetched.CompletionSummary != summary {
		t.Errorf("Expected summary %s", summary)
	}

	// 6. Invalid Status Transition
	err = db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusPending, nil)
	if err == nil {
		t.Errorf("Expected error for invalid transition from completed to pending")
	}

	// 7. List Tasks
	tasks, err := db.ListTasks(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	status := models.TaskStatusCompleted
	tasks, err = db.ListTasks(ctx, &status, &f.Name)
	if err != nil {
		t.Fatalf("Failed to list tasks with filter: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task with filter, got %d", len(tasks))
	}

	// 8. Delete Task
	if err := db.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	fetched, err = db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task after deletion: %v", err)
	}
	if fetched != nil {
		t.Errorf("Expected task to be deleted, but it still exists")
	}
}

func TestClaimNextTask(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 1. Create a feature
	f := &models.Feature{
		Name:          "Claim Test Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// 2. Test claiming when no tasks available
	claimed, err := db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed != nil {
		t.Errorf("Expected nil when no tasks available, got %v", claimed)
	}

	// 3. Create multiple tasks with different priorities
	task1 := &models.Task{
		FeatureID: f.ID,
		Name:      "Low Priority Task",
		Status:    models.TaskStatusPending,
		Priority:  1,
	}
	task2 := &models.Task{
		FeatureID: f.ID,
		Name:      "High Priority Task",
		Status:    models.TaskStatusPending,
		Priority:  10,
	}
	task3 := &models.Task{
		FeatureID: f.ID,
		Name:      "Medium Priority Task",
		Status:    models.TaskStatusPending,
		Priority:  5,
	}

	if err := db.CreateTask(ctx, task1); err != nil {
		t.Fatalf("Failed to create task1: %v", err)
	}
	if err := db.CreateTask(ctx, task2); err != nil {
		t.Fatalf("Failed to create task2: %v", err)
	}
	if err := db.CreateTask(ctx, task3); err != nil {
		t.Fatalf("Failed to create task3: %v", err)
	}

	// 4. Claim should return highest priority task (task2)
	claimed, err = db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed == nil {
		t.Fatalf("Expected to claim a task, got nil")
	}
	if claimed.ID != task2.ID {
		t.Errorf("Expected to claim task2 (high priority), got %s", claimed.Name)
	}
	if claimed.Status != models.TaskStatusInProgress {
		t.Errorf("Expected status in_progress, got %s", claimed.Status)
	}

	// 5. Claim should now return next highest priority (task3)
	claimed, err = db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed == nil {
		t.Fatalf("Expected to claim a task, got nil")
	}
	if claimed.ID != task3.ID {
		t.Errorf("Expected to claim task3 (medium priority), got %s", claimed.Name)
	}

	// 6. Claim should now return task1
	claimed, err = db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed == nil {
		t.Fatalf("Expected to claim a task, got nil")
	}
	if claimed.ID != task1.ID {
		t.Errorf("Expected to claim task1 (low priority), got %s", claimed.Name)
	}

	// 7. No more tasks available
	claimed, err = db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed != nil {
		t.Errorf("Expected nil when all tasks claimed, got %v", claimed)
	}
}

func TestClaimNextTaskWithDependencies(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 1. Create a feature
	f := &models.Feature{
		Name:          "Dependency Test Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// 2. Create tasks with dependency chain
	// Task B depends on Task A
	taskA := &models.Task{
		FeatureID: f.ID,
		Name:      "Task A",
		Status:    models.TaskStatusPending,
		Priority:  5,
	}
	taskB := &models.Task{
		FeatureID: f.ID,
		Name:      "Task B",
		Status:    models.TaskStatusPending,
		Priority:  10, // Higher priority but has dependency
	}

	if err := db.CreateTask(ctx, taskA); err != nil {
		t.Fatalf("Failed to create taskA: %v", err)
	}
	if err := db.CreateTask(ctx, taskB); err != nil {
		t.Fatalf("Failed to create taskB: %v", err)
	}

	// Create dependency: Task B depends on Task A
	if err := db.CreateDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("Failed to create dependency: %v", err)
	}

	// 3. Claim should return Task A (Task B is blocked by dependency)
	claimed, err := db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed == nil {
		t.Fatalf("Expected to claim a task, got nil")
	}
	if claimed.ID != taskA.ID {
		t.Errorf("Expected to claim taskA (no dependencies), got %s", claimed.Name)
	}

	// 4. Complete Task A
	summary := "Task A completed"
	if err := db.UpdateTaskStatus(ctx, taskA.ID, models.TaskStatusCompleted, &summary); err != nil {
		t.Fatalf("Failed to complete taskA: %v", err)
	}

	// 5. Now claim should return Task B (dependency is completed)
	claimed, err = db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed == nil {
		t.Fatalf("Expected to claim a task, got nil")
	}
	if claimed.ID != taskB.ID {
		t.Errorf("Expected to claim taskB (dependency completed), got %s", claimed.Name)
	}

	// 6. No more tasks available
	claimed, err = db.ClaimNextTask(ctx)
	if err != nil {
		t.Fatalf("Failed to claim next task: %v", err)
	}
	if claimed != nil {
		t.Errorf("Expected nil when all tasks claimed, got %v", claimed)
	}
}

func TestResetInProgressTasks(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 1. Create a feature
	f := &models.Feature{
		Name: "Reset Test Feature",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// 2. Create some tasks as pending
	task1 := &models.Task{
		FeatureID: f.ID,
		Name:      "Task 1",
		Status:    models.TaskStatusPending,
	}
	task2 := &models.Task{
		FeatureID: f.ID,
		Name:      "Task 2",
		Status:    models.TaskStatusPending,
	}
	task3 := &models.Task{
		FeatureID: f.ID,
		Name:      "Task 3",
		Status:    models.TaskStatusPending,
	}

	if err := db.CreateTask(ctx, task1); err != nil {
		t.Fatalf("Failed to create task1: %v", err)
	}
	if err := db.CreateTask(ctx, task2); err != nil {
		t.Fatalf("Failed to create task2: %v", err)
	}
	if err := db.CreateTask(ctx, task3); err != nil {
		t.Fatalf("Failed to create task3: %v", err)
	}

	// Set desired statuses
	if err := db.UpdateTaskStatus(ctx, task1.ID, models.TaskStatusInProgress, nil); err != nil {
		t.Fatalf("Failed to set task1 to in_progress: %v", err)
	}
	summary := "Done"
	if err := db.UpdateTaskStatus(ctx, task3.ID, models.TaskStatusInProgress, nil); err != nil {
		t.Fatalf("Failed to set task3 to in_progress: %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, task3.ID, models.TaskStatusCompleted, &summary); err != nil {
		t.Fatalf("Failed to set task3 to completed: %v", err)
	}

	// 3. Reset in_progress tasks
	if err := db.ResetInProgressTasks(ctx); err != nil {
		t.Fatalf("Failed to reset in_progress tasks: %v", err)
	}

	// 4. Verify results
	t1, _ := db.GetTask(ctx, task1.ID)
	if t1.Status != models.TaskStatusPending {
		t.Errorf("Expected task1 status pending, got %s", t1.Status)
	}

	t2, _ := db.GetTask(ctx, task2.ID)
	if t2.Status != models.TaskStatusPending {
		t.Errorf("Expected task2 status pending, got %s", t2.Status)
	}

	t3, _ := db.GetTask(ctx, task3.ID)
	if t3.Status != models.TaskStatusCompleted {
		t.Errorf("Expected task3 status completed, got %s", t3.Status)
	}
}
