package db

import (
	"testing"

	"github.com/nick-dorsch/ponder/pkg/models"
)

func TestStagingManager(t *testing.T) {
	sm := NewStagingManager()
	sessionID := "test-session"

	// Test AddFeature
	feature := &models.Feature{ID: "f1", Name: "Feature 1"}
	sm.AddFeature(sessionID, feature)

	// Test AddTask
	task := &models.Task{ID: "t1", Name: "Task 1"}
	sm.AddTask(sessionID, task)

	// Test AddDependency
	dep := &models.Dependency{TaskID: "t1", DependsOnTaskID: "t2"}
	sm.AddDependency(sessionID, dep)

	// Test GetAndClear
	staged := sm.GetAndClear(sessionID)

	if len(staged.Features) != 1 || staged.Features[0].ID != "f1" {
		t.Errorf("expected 1 feature with ID f1, got %v", staged.Features)
	}
	if len(staged.Tasks) != 1 || staged.Tasks[0].ID != "t1" {
		t.Errorf("expected 1 task with ID t1, got %v", staged.Tasks)
	}
	if len(staged.Dependencies) != 1 || staged.Dependencies[0].TaskID != "t1" {
		t.Errorf("expected 1 dependency with TaskID t1, got %v", staged.Dependencies)
	}

	// Test that it's cleared
	staged2 := sm.GetAndClear(sessionID)
	if len(staged2.Features) != 0 || len(staged2.Tasks) != 0 || len(staged2.Dependencies) != 0 {
		t.Errorf("expected empty staged items after GetAndClear, got %v", staged2)
	}
}

func TestStagingManagerMultipleSessions(t *testing.T) {
	sm := NewStagingManager()
	s1 := "session-1"
	s2 := "session-2"

	sm.AddFeature(s1, &models.Feature{ID: "f1"})
	sm.AddFeature(s2, &models.Feature{ID: "f2"})

	staged1 := sm.GetAndClear(s1)
	if len(staged1.Features) != 1 || staged1.Features[0].ID != "f1" {
		t.Errorf("session 1: expected f1, got %v", staged1.Features)
	}

	staged2 := sm.GetAndClear(s2)
	if len(staged2.Features) != 1 || staged2.Features[0].ID != "f2" {
		t.Errorf("session 2: expected f2, got %v", staged2.Features)
	}
}

func TestStagingManagerEmptySession(t *testing.T) {
	sm := NewStagingManager()
	staged := sm.GetAndClear("non-existent")

	if staged == nil {
		t.Fatal("expected non-nil staged items for empty session")
	}
	if len(staged.Features) != 0 || len(staged.Tasks) != 0 || len(staged.Dependencies) != 0 {
		t.Errorf("expected empty staged items, got %v", staged)
	}
}
