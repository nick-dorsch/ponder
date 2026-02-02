package db

import (
	"context"
	"strings"
	"testing"

	"github.com/nick-dorsch/ponder/pkg/models"
)

func TestFeatureCRUD(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 1. Create
	f := &models.Feature{
		Name:          "Test Feature",
		Description:   "Description",
		Specification: "Specification",
	}

	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	if len(f.ID) != 36 {
		t.Errorf("Expected ID length 36, got %d (%s)", len(f.ID), f.ID)
	}

	if !strings.Contains(f.ID, "-") {
		t.Errorf("Expected ID to contain dashes, got %s", f.ID)
	}

	if f.CreatedAt.IsZero() || f.UpdatedAt.IsZero() {
		t.Errorf("Expected CreatedAt and UpdatedAt to be set")
	}

	// 2. Get
	fetched, err := db.GetFeature(ctx, f.ID)
	if err != nil {
		t.Fatalf("Failed to get feature: %v", err)
	}
	if fetched == nil {
		t.Fatalf("Feature not found")
	}
	if fetched.Name != f.Name {
		t.Errorf("Expected name %s, got %s", f.Name, fetched.Name)
	}

	// 3. List
	features, err := db.ListFeatures(ctx)
	if err != nil {
		t.Fatalf("Failed to list features: %v", err)
	}
	// Note: 'misc' feature is seeded by default
	if len(features) < 2 {
		t.Errorf("Expected at least 2 features, got %d", len(features))
	}

	// 4. Update
	f.Name = "Updated Name"
	if err := db.UpdateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to update feature: %v", err)
	}

	fetched, err = db.GetFeature(ctx, f.ID)
	if err != nil {
		t.Fatalf("Failed to get feature: %v", err)
	}
	if fetched.Name != "Updated Name" {
		t.Errorf("Expected name Updated Name, got %s", fetched.Name)
	}
	// The trigger should update updated_at if it's the same,
	// but RETURNING updated_at in Go might be tricky with SQLite if it happens in the same statement.
	// Actually the RETURNING clause in SQLite should return the value AFTER triggers.
	// Let's verify if UpdatedAt changed (it might not if the test runs too fast and resolution is low)
	// but usually CURRENT_TIMESTAMP should be fine.

	// 5. Delete
	if err := db.DeleteFeature(ctx, f.ID); err != nil {
		t.Fatalf("Failed to delete feature: %v", err)
	}

	fetched, err = db.GetFeature(ctx, f.ID)
	if err != nil {
		t.Fatalf("Failed to get feature after deletion: %v", err)
	}
	if fetched != nil {
		t.Errorf("Expected feature to be deleted, but it still exists")
	}
}
