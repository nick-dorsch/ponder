package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ldi/ponder/pkg/models"
)

func TestGetGraphJSON(t *testing.T) {
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
		Name:          "Test Feature",
		Description:   "Feature Description",
		Specification: "Feature Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	// 2. Create a task
	task := &models.Task{
		FeatureID:     f.ID,
		Name:          "Test Task",
		Description:   "Task Description",
		Specification: "Task Specification",
		Priority:      5,
		Status:        models.TaskStatusPending,
	}
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 3. Get Graph JSON
	graphJSON, err := db.GetGraphJSON(ctx)
	if err != nil {
		t.Fatalf("Failed to get graph JSON: %v", err)
	}

	// 4. Verify JSON structure and content
	var data struct {
		Nodes []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			FeatureName string `json:"feature_name"`
		} `json:"nodes"`
	}

	if err := json.Unmarshal([]byte(graphJSON), &data); err != nil {
		t.Fatalf("Failed to unmarshal graph JSON: %v", err)
	}

	if len(data.Nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(data.Nodes))
	}

	node := data.Nodes[0]
	if node.Name != task.Name {
		t.Errorf("Expected node name %s, got %s", task.Name, node.Name)
	}
	if node.FeatureName != f.Name {
		t.Errorf("Expected feature name %s, got %s", f.Name, node.FeatureName)
	}
}
