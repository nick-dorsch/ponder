package db

import (
	"context"
	"testing"

	"github.com/ldi/ponder/pkg/models"
)

func TestDependencies(t *testing.T) {
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
		Name:          "Dep Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// 2. Create two tasks
	task1 := &models.Task{
		FeatureID:     f.ID,
		Name:          "Task 1",
		Description:   "Description",
		Specification: "Specification",
		Status:        models.TaskStatusPending,
	}
	if err := db.CreateTask(ctx, task1); err != nil {
		t.Fatalf("Failed to create task 1: %v", err)
	}

	task2 := &models.Task{
		FeatureID:     f.ID,
		Name:          "Task 2",
		Description:   "Description",
		Specification: "Specification",
		Status:        models.TaskStatusPending,
	}
	if err := db.CreateTask(ctx, task2); err != nil {
		t.Fatalf("Failed to create task 2: %v", err)
	}

	// 3. Create dependency: Task 2 depends on Task 1
	if err := db.CreateDependency(ctx, task2.ID, task1.ID); err != nil {
		t.Fatalf("Failed to create dependency: %v", err)
	}

	// 4. Get dependencies for Task 2
	deps, err := db.GetDependencies(ctx, task2.ID)
	if err != nil {
		t.Fatalf("Failed to get dependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(deps))
	} else if deps[0].ID != task1.ID {
		t.Errorf("Expected dependency on Task 1, got %s", deps[0].ID)
	}

	// 5. Get dependents for Task 1
	dependents, err := db.GetDependents(ctx, task1.ID)
	if err != nil {
		t.Fatalf("Failed to get dependents: %v", err)
	}
	if len(dependents) != 1 {
		t.Errorf("Expected 1 dependent, got %d", len(dependents))
	} else if dependents[0].ID != task2.ID {
		t.Errorf("Expected dependent Task 2, got %s", dependents[0].ID)
	}

	// 6. Circular dependency (should fail via DB trigger)
	err = db.CreateDependency(ctx, task1.ID, task2.ID)
	if err == nil {
		t.Errorf("Expected error when creating circular dependency, got nil")
	}

	// 7. Delete dependency
	if err := db.DeleteDependency(ctx, task2.ID, task1.ID); err != nil {
		t.Fatalf("Failed to delete dependency: %v", err)
	}

	deps, err = db.GetDependencies(ctx, task2.ID)
	if err != nil {
		t.Fatalf("Failed to get dependencies after deletion: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("Expected 0 dependencies after deletion, got %d", len(deps))
	}
}
