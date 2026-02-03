package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nick-dorsch/ponder/embed/graph_assets"
	"github.com/nick-dorsch/ponder/internal/db"
	"github.com/nick-dorsch/ponder/pkg/models"
)

func TestServer_API(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	err = database.Init(ctx)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Seed some data
	feature := &models.Feature{
		Name:        "test-feature",
		Description: "test description",
	}
	err = database.CreateFeature(ctx, feature)
	if err != nil {
		t.Fatalf("CreateFeature failed: %v", err)
	}

	task := &models.Task{
		FeatureID: feature.ID,
		Name:      "test-task",
		Priority:  5,
		Status:    models.TaskStatusPending,
	}
	err = database.CreateTask(ctx, task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	srv := NewServer(database)

	t.Run("GET /api/tasks", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks", nil)
		w := httptest.NewRecorder()
		srv.handleTasks(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
		var tasks []*models.Task
		if err := json.Unmarshal(w.Body.Bytes(), &tasks); err != nil {
			t.Fatalf("Failed to unmarshal tasks: %v", err)
		}
		if len(tasks) != 1 {
			t.Errorf("Expected 1 task, got %d", len(tasks))
		} else if tasks[0].Name != "test-task" {
			t.Errorf("Expected task name test-task, got %s", tasks[0].Name)
		}
	})

	t.Run("GET /api/features", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		srv.handleFeatures(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
		var features []*models.Feature
		if err := json.Unmarshal(w.Body.Bytes(), &features); err != nil {
			t.Fatalf("Failed to unmarshal features: %v", err)
		}
		if len(features) >= 1 {
			found := false
			for _, f := range features {
				if f.Name == "test-feature" {
					found = true
					break
				}
			}
			if !found {
				t.Error("Expected feature test-feature not found")
			}
		} else {
			t.Errorf("Expected at least 1 feature, got %d", len(features))
		}
	})

	t.Run("GET /api/graph", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		srv.handleGraph(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
		var graph map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
			t.Fatalf("Failed to unmarshal graph: %v", err)
		}
		if _, ok := graph["nodes"]; !ok {
			t.Error("Graph missing nodes")
		}
		if _, ok := graph["edges"]; !ok {
			t.Error("Graph missing edges")
		}
		// Verify feature_name is present in nodes
		nodes := graph["nodes"].([]interface{})
		if len(nodes) == 0 {
			t.Error("Graph nodes are empty")
		} else {
			node := nodes[0].(map[string]interface{})
			if _, ok := node["feature_name"]; !ok {
				t.Error("Graph node missing feature_name")
			}
			if node["feature_name"] != "test-feature" {
				t.Errorf("Expected feature_name test-feature, got %v", node["feature_name"])
			}
		}
	})

	t.Run("GET /", func(t *testing.T) {
		mux := testMux()
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("GET / graph.js", func(t *testing.T) {
		mux := testMux()
		req := httptest.NewRequest("GET", "/graph.js", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func testMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(graph_assets.Assets)))
	return mux
}
