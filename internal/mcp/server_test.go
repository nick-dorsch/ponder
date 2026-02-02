package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nick-dorsch/ponder/internal/db"
	"github.com/nick-dorsch/ponder/pkg/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestServerInitialization(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := database.Init(context.Background()); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	s := NewServer(database)
	stdio := server.NewStdioServer(s)

	r, w := io.Pipe()
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- stdio.Listen(ctx, r, stdout)
	}()

	// Send initialize request
	initReq := mcp.InitializeRequest{}
	initReq.Method = "initialize"
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	// Use a map for the raw JSON-RPC message because mcp.InitializeRequest
	// doesn't have the "jsonrpc" and "id" fields in the way we want for manual writing.
	rawReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  initReq.Params,
	}

	data, err := json.Marshal(rawReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	w.Write(data)
	w.Write([]byte("\n"))

	// Give it a moment to process
	time.Sleep(200 * time.Millisecond)

	if stdout.Len() == 0 {
		t.Fatal("Expected response from server, got none")
	}

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v\nOutput: %s", err, stdout.String())
	}

	if resp.ID != 1 {
		t.Errorf("Expected id 1, got %v", resp.ID)
	}

	if resp.Result.ServerInfo.Name != "Ponder" {
		t.Errorf("Expected server name Ponder, got %v", resp.Result.ServerInfo.Name)
	}
}

func TestToolHandlers(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.Init(ctx); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	s := NewServer(database)

	t.Run("create_feature", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "create_feature"
		req.Params.Arguments = map[string]interface{}{
			"name":          "test-feature",
			"description":   "test description",
			"specification": "test spec",
		}

		tool := s.GetTool("create_feature")
		if tool == nil {
			t.Fatal("Tool create_feature not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		// Commit staged changes
		s.GetTool("commit_staged_changes").Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "commit_staged_changes",
				Arguments: map[string]interface{}{"session_id": "default"},
			},
		})

		// Verify in DB
		f, err := database.GetFeatureByName(ctx, "test-feature")
		if err != nil {
			t.Fatalf("Failed to get feature: %v", err)
		}
		if f == nil {
			t.Fatal("Feature not found in DB")
		}
	})

	t.Run("list_features", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "list_features"
		req.Params.Arguments = map[string]interface{}{}

		tool := s.GetTool("list_features")
		if tool == nil {
			t.Fatal("Tool list_features not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		var resp struct {
			Features []interface{} `json:"features"`
		}

		text := result.Content[0].(mcp.TextContent).Text
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(resp.Features) != 2 {
			t.Errorf("Expected 2 features (including 'misc'), got %d", len(resp.Features))
		}
	})

	t.Run("create_task", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "create_task"
		req.Params.Arguments = map[string]interface{}{
			"feature_name":  "test-feature",
			"name":          "test-task",
			"description":   "task description",
			"specification": "task spec",
			"priority":      5.0,
		}

		tool := s.GetTool("create_task")
		if tool == nil {
			t.Fatal("Tool create_task not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		// Commit staged changes
		s.GetTool("commit_staged_changes").Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "commit_staged_changes",
				Arguments: map[string]interface{}{"session_id": "default"},
			},
		})

		// Verify in DB
		tasks, _ := database.ListTasks(ctx, nil, nil)
		found := false
		for _, task := range tasks {
			if task.Name == "test-task" {
				found = true
				if task.Priority != 5 {
					t.Errorf("Expected priority 5, got %d", task.Priority)
				}
				break
			}
		}
		if !found {
			t.Fatal("Task not found in DB")
		}
	})

	t.Run("get_available_tasks", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "get_available_tasks"
		req.Params.Arguments = map[string]interface{}{}

		tool := s.GetTool("get_available_tasks")
		if tool == nil {
			t.Fatal("Tool get_available_tasks not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		var resp struct {
			Tasks []interface{} `json:"tasks"`
		}
		text := result.Content[0].(mcp.TextContent).Text
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(resp.Tasks) != 1 {
			t.Errorf("Expected 1 available task, got %d", len(resp.Tasks))
		}
	})

	t.Run("update_task", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "update_task"
		req.Params.Arguments = map[string]interface{}{
			"feature_name": "test-feature",
			"name":         "test-task",
			"new_name":     "updated-task",
			"priority":     8.0,
		}

		tool := s.GetTool("update_task")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Handler failed: %v, %v", err, result.Content)
		}

		// Verify
		f, _ := database.GetFeatureByName(ctx, "test-feature")
		task, err := database.GetTaskByName(ctx, "updated-task", f.ID)
		if err != nil {
			t.Fatalf("Failed to get task: %v", err)
		}
		if task == nil {
			t.Fatal("Updated task not found")
		}
		if task.Priority != 8 {
			t.Errorf("Expected priority 8, got %d", task.Priority)
		}
	})

	t.Run("move_task_between_features", func(t *testing.T) {
		// Create another feature
		if err := database.CreateFeature(ctx, &models.Feature{
			Name:          "other-feature",
			Description:   "d",
			Specification: "s",
		}); err != nil {
			t.Fatalf("Failed to create other-feature: %v", err)
		}

		req := mcp.CallToolRequest{}
		req.Params.Name = "update_task"
		req.Params.Arguments = map[string]interface{}{
			"feature_name":     "test-feature",
			"name":             "updated-task",
			"new_feature_name": "other-feature",
		}

		tool := s.GetTool("update_task")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Handler failed: %v, %v", err, result.Content)
		}

		// Verify it moved
		nf, _ := database.GetFeatureByName(ctx, "other-feature")
		task, _ := database.GetTaskByName(ctx, "updated-task", nf.ID)
		if task == nil {
			t.Fatal("Task not found in new feature")
		}

		// Verify it's gone from old feature
		of, _ := database.GetFeatureByName(ctx, "test-feature")
		oldTask, _ := database.GetTaskByName(ctx, "updated-task", of.ID)
		if oldTask != nil {
			t.Fatal("Task still exists in old feature")
		}

		// Move it back for subsequent tests if necessary
		req.Params.Arguments = map[string]interface{}{
			"feature_name":     "other-feature",
			"name":             "updated-task",
			"new_feature_name": "test-feature",
		}
		tool.Handler(ctx, req)
	})

	t.Run("update_task_status", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "update_task_status"
		req.Params.Arguments = map[string]interface{}{
			"feature_name": "test-feature",
			"name":         "updated-task",
			"status":       "in_progress",
		}

		tool := s.GetTool("update_task_status")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Handler failed: %v, %v", err, result.Content)
		}

		// Verify
		f, _ := database.GetFeatureByName(ctx, "test-feature")
		task, _ := database.GetTaskByName(ctx, "updated-task", f.ID)
		if task.Status != "in_progress" {
			t.Errorf("Expected status in_progress, got %s", task.Status)
		}
	})

	t.Run("list_tasks", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "list_tasks"
		req.Params.Arguments = map[string]interface{}{
			"feature_name": "test-feature",
		}

		tool := s.GetTool("list_tasks")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Handler failed: %v, %v", err, result.Content)
		}

		var resp struct {
			Tasks []interface{} `json:"tasks"`
		}
		text := result.Content[0].(mcp.TextContent).Text
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if len(resp.Tasks) != 1 {
			t.Errorf("Expected 1 task, got %d", len(resp.Tasks))
		}
	})

	t.Run("delete_task", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "delete_task"
		req.Params.Arguments = map[string]interface{}{
			"feature_name": "test-feature",
			"name":         "updated-task",
		}

		tool := s.GetTool("delete_task")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Handler failed: %v, %v", err, result.Content)
		}

		// Verify
		f, _ := database.GetFeatureByName(ctx, "test-feature")
		task, _ := database.GetTaskByName(ctx, "updated-task", f.ID)
		if task != nil {
			t.Fatal("Task still exists after deletion")
		}
	})

	t.Run("get_feature", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "get_feature"
		req.Params.Arguments = map[string]interface{}{
			"name": "test-feature",
		}

		tool := s.GetTool("get_feature")
		if tool == nil {
			t.Fatal("Tool get_feature not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		var f struct {
			Name string `json:"name"`
		}
		text := result.Content[0].(mcp.TextContent).Text
		if err := json.Unmarshal([]byte(text), &f); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if f.Name != "test-feature" {
			t.Errorf("Expected feature name test-feature, got %s", f.Name)
		}
	})

	t.Run("update_feature", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "update_feature"
		req.Params.Arguments = map[string]interface{}{
			"name":     "test-feature",
			"new_name": "updated-feature",
		}

		tool := s.GetTool("update_feature")
		if tool == nil {
			t.Fatal("Tool update_feature not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		// Verify in DB
		f, err := database.GetFeatureByName(ctx, "updated-feature")
		if err != nil {
			t.Fatalf("Failed to get feature: %v", err)
		}
		if f == nil {
			t.Fatal("Updated feature not found in DB")
		}
	})

	t.Run("delete_feature", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "delete_feature"
		req.Params.Arguments = map[string]interface{}{
			"name": "updated-feature",
		}

		tool := s.GetTool("delete_feature")
		if tool == nil {
			t.Fatal("Tool delete_feature not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		// Verify in DB
		f, err := database.GetFeatureByName(ctx, "updated-feature")
		if err != nil {
			t.Fatalf("Failed to check feature deletion: %v", err)
		}
		if f != nil {
			t.Fatal("Feature still exists in DB after deletion")
		}
	})

	t.Run("dependencies", func(t *testing.T) {
		// Create features and tasks for dependency tests
		if err := database.CreateFeature(ctx, &models.Feature{Name: "feat1", Description: "d", Specification: "s"}); err != nil {
			t.Fatalf("Failed to create feat1: %v", err)
		}
		if err := database.CreateFeature(ctx, &models.Feature{Name: "feat2", Description: "d", Specification: "s"}); err != nil {
			t.Fatalf("Failed to create feat2: %v", err)
		}

		f1, err := database.GetFeatureByName(ctx, "feat1")
		if err != nil || f1 == nil {
			t.Fatalf("Failed to get feat1: %v", err)
		}
		f2, err := database.GetFeatureByName(ctx, "feat2")
		if err != nil || f2 == nil {
			t.Fatalf("Failed to get feat2: %v", err)
		}

		if err := database.CreateTask(ctx, &models.Task{FeatureID: f1.ID, Name: "task1", Description: "d", Specification: "s", Status: models.TaskStatusPending}); err != nil {
			t.Fatalf("Failed to create task1: %v", err)
		}
		if err := database.CreateTask(ctx, &models.Task{FeatureID: f2.ID, Name: "task2", Description: "d", Specification: "s", Status: models.TaskStatusPending}); err != nil {
			t.Fatalf("Failed to create task2: %v", err)
		}

		// Verify tasks exist
		t1, err := database.GetTaskByName(ctx, "task1", f1.ID)
		if err != nil || t1 == nil {
			t.Fatalf("Task1 not found after create: %v", err)
		}

		// Test create_dependency
		req := mcp.CallToolRequest{}
		req.Params.Name = "create_dependency"
		req.Params.Arguments = map[string]interface{}{
			"feature_name":            "feat1",
			"task_name":               "task1",
			"depends_on_task_name":    "task2",
			"depends_on_feature_name": "feat2",
		}

		tool := s.GetTool("create_dependency")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("create_dependency failed: %v, %v", err, result.Content)
		}

		// Commit staged changes
		s.GetTool("commit_staged_changes").Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "commit_staged_changes",
				Arguments: map[string]interface{}{"session_id": "default"},
			},
		})

		if err := database.CreateTask(ctx, &models.Task{FeatureID: f1.ID, Name: "task1-bis", Description: "d", Specification: "s", Status: models.TaskStatusPending}); err != nil {
			t.Fatalf("Failed to create task1-bis: %v", err)
		}

		// Test create_dependency within same feature (default depends_on_feature_name)
		req = mcp.CallToolRequest{}
		req.Params.Name = "create_dependency"
		req.Params.Arguments = map[string]interface{}{
			"feature_name":         "feat1",
			"task_name":            "task1",
			"depends_on_task_name": "task1-bis",
		}

		result, err = tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("create_dependency (same feature) failed: %v, %v", err, result.Content)
		}

		// Commit staged changes
		s.GetTool("commit_staged_changes").Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "commit_staged_changes",
				Arguments: map[string]interface{}{"session_id": "default"},
			},
		})

		// Verify we have 2 dependencies now
		tool = s.GetTool("get_task_dependencies")
		req.Params.Name = "get_task_dependencies"
		req.Params.Arguments = map[string]interface{}{
			"feature_name": "feat1",
			"name":         "task1",
		}
		result, _ = tool.Handler(ctx, req)

		var depsResp struct {
			Dependencies []interface{} `json:"dependencies"`
		}
		text := result.Content[0].(mcp.TextContent).Text
		json.Unmarshal([]byte(text), &depsResp)
		if len(depsResp.Dependencies) != 2 {
			t.Errorf("Expected 2 dependencies, got %d", len(depsResp.Dependencies))
		}

		// Test delete_dependency
		req = mcp.CallToolRequest{}
		req.Params.Name = "delete_dependency"
		req.Params.Arguments = map[string]interface{}{
			"feature_name":            "feat1",
			"task_name":               "task1",
			"depends_on_task_name":    "task2",
			"depends_on_feature_name": "feat2",
		}

		tool = s.GetTool("delete_dependency")
		result, err = tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("delete_dependency failed: %v, %v", err, result.Content)
		}

		// Verify deletion
		tool = s.GetTool("get_task_dependencies")
		req.Params.Name = "get_task_dependencies"
		req.Params.Arguments = map[string]interface{}{
			"feature_name": "feat1",
			"name":         "task1",
		}
		result, _ = tool.Handler(ctx, req)
		text = result.Content[0].(mcp.TextContent).Text
		json.Unmarshal([]byte(text), &depsResp)
		if len(depsResp.Dependencies) != 1 {
			t.Errorf("Expected 1 dependency after deleting one, got %d", len(depsResp.Dependencies))
		}
	})

	t.Run("get_graph_json", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "get_graph_json"
		req.Params.Arguments = map[string]interface{}{}

		tool := s.GetTool("get_graph_json")
		if tool == nil {
			t.Fatal("Tool get_graph_json not found")
		}

		result, err := tool.Handler(ctx, req)
		if err != nil {
			t.Fatalf("Handler failed: %v", err)
		}

		if result.IsError {
			t.Fatalf("Tool returned error: %v", result.Content[0])
		}

		text := result.Content[0].(mcp.TextContent).Text
		var graph map[string]interface{}
		if err := json.Unmarshal([]byte(text), &graph); err != nil {
			t.Fatalf("Failed to unmarshal graph JSON: %v", err)
		}

		if _, ok := graph["nodes"]; !ok {
			t.Error("Graph JSON missing 'nodes'")
		}
		if _, ok := graph["edges"]; !ok {
			t.Error("Graph JSON missing 'edges'")
		}
	})

	t.Run("lifecycle_tools", func(t *testing.T) {
		// Setup
		fName := "lifecycle-feat"
		tName := "lifecycle-task"
		if err := database.CreateFeature(ctx, &models.Feature{Name: fName, Description: "d", Specification: "s"}); err != nil {
			t.Fatalf("Failed to create feature: %v", err)
		}
		f, _ := database.GetFeatureByName(ctx, fName)
		if err := database.CreateTask(ctx, &models.Task{
			FeatureID:     f.ID,
			Name:          tName,
			Description:   "d",
			Specification: "initial spec",
			Status:        models.TaskStatusPending,
		}); err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		t.Run("start_task", func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "start_task"
			req.Params.Arguments = map[string]interface{}{
				"feature_name": fName,
				"name":         tName,
			}

			tool := s.GetTool("start_task")
			result, err := tool.Handler(ctx, req)
			if err != nil || result.IsError {
				t.Fatalf("Handler failed: %v, %v", err, result.Content)
			}

			// Verify
			task, _ := database.GetTaskByName(ctx, tName, f.ID)
			if task.Status != models.TaskStatusInProgress {
				t.Errorf("Expected status in_progress, got %s", task.Status)
			}
		})

		t.Run("report_task_blocked", func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "report_task_blocked"
			req.Params.Arguments = map[string]interface{}{
				"feature_name": fName,
				"name":         tName,
				"reason":       "missing API key",
			}

			tool := s.GetTool("report_task_blocked")
			result, err := tool.Handler(ctx, req)
			if err != nil || result.IsError {
				t.Fatalf("Handler failed: %v, %v", err, result.Content)
			}

			// Verify
			task, _ := database.GetTaskByName(ctx, tName, f.ID)
			if task.Status != models.TaskStatusBlocked {
				t.Errorf("Expected status blocked, got %s", task.Status)
			}
			if !strings.Contains(task.Specification, "### Blocked Reason") || !strings.Contains(task.Specification, "missing API key") {
				t.Errorf("Specification not updated correctly: %s", task.Specification)
			}
		})

		t.Run("complete_task", func(t *testing.T) {
			// Move it back to in_progress first because blocked -> completed is not allowed
			tk, _ := database.GetTaskByName(ctx, tName, f.ID)
			database.UpdateTaskStatus(ctx, tk.ID, models.TaskStatusInProgress, nil)

			req := mcp.CallToolRequest{}
			req.Params.Name = "complete_task"
			req.Params.Arguments = map[string]interface{}{
				"feature_name":       fName,
				"name":               tName,
				"completion_summary": "done everything",
			}

			tool := s.GetTool("complete_task")
			result, err := tool.Handler(ctx, req)
			if err != nil || result.IsError {
				t.Fatalf("Handler failed: %v, %v", err, result.Content)
			}

			// Verify
			task, _ := database.GetTaskByName(ctx, tName, f.ID)
			if task.Status != models.TaskStatusCompleted {
				t.Errorf("Expected status completed, got %s", task.Status)
			}
			if task.CompletionSummary == nil || *task.CompletionSummary != "done everything" {
				t.Errorf("Completion summary not saved correctly")
			}
		})
	})

	t.Run("error_handling", func(t *testing.T) {
		t.Run("non_existent_feature", func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "get_feature"
			req.Params.Arguments = map[string]interface{}{
				"name": "does-not-exist",
			}
			tool := s.GetTool("get_feature")
			result, err := tool.Handler(ctx, req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}
			if !result.IsError {
				t.Error("Expected error for non-existent feature, got success")
			}
		})

		t.Run("non_existent_task", func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "update_task_status"
			req.Params.Arguments = map[string]interface{}{
				"feature_name": "test-feature",
				"name":         "does-not-exist",
				"status":       "in_progress",
			}
			tool := s.GetTool("update_task_status")
			result, err := tool.Handler(ctx, req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}
			if !result.IsError {
				t.Error("Expected error for non-existent task, got success")
			}
		})

		t.Run("dependency_on_non_existent_task", func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "create_dependency"
			req.Params.Arguments = map[string]interface{}{
				"feature_name":         "feat1",
				"task_name":            "task1",
				"depends_on_task_name": "does-not-exist",
			}
			tool := s.GetTool("create_dependency")
			result, err := tool.Handler(ctx, req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}
			if result.IsError {
				t.Error("Expected success during staging, got error")
			}

			// Committing should fail
			req = mcp.CallToolRequest{}
			req.Params.Name = "commit_staged_changes"
			req.Params.Arguments = map[string]interface{}{"session_id": "default"}
			result, err = s.GetTool("commit_staged_changes").Handler(ctx, req)
			if err != nil {
				t.Fatalf("Commit failed: %v", err)
			}
			if !result.IsError {
				t.Error("Expected error during commit for non-existent dependency task, got success")
			}
		})
	})

	t.Run("staging_and_batch_commit", func(t *testing.T) {
		sessionID := "test-session"

		// 1. Stage a feature
		req := mcp.CallToolRequest{}
		req.Params.Name = "create_feature"
		req.Params.Arguments = map[string]interface{}{
			"name":          "staged-feature",
			"description":   "d",
			"specification": "s",
			"session_id":    sessionID,
		}

		tool := s.GetTool("create_feature")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Failed to stage feature: %v, %v", err, result.Content)
		}

		// Verify NOT in DB
		f, _ := database.GetFeatureByName(ctx, "staged-feature")
		if f != nil {
			t.Fatal("Feature should not be in DB yet")
		}

		// 2. Stage a task for that feature
		req = mcp.CallToolRequest{}
		req.Params.Name = "create_task"
		req.Params.Arguments = map[string]interface{}{
			"feature_name":  "staged-feature",
			"name":          "staged-task",
			"description":   "d",
			"specification": "s",
			"session_id":    sessionID,
		}

		tool = s.GetTool("create_task")
		result, err = tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Failed to stage task: %v, %v", err, result.Content)
		}

		// 3. List staged changes
		req = mcp.CallToolRequest{}
		req.Params.Name = "list_staged_changes"
		req.Params.Arguments = map[string]interface{}{
			"session_id": sessionID,
		}

		tool = s.GetTool("list_staged_changes")
		result, err = tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Failed to list staged changes: %v, %v", err, result.Content)
		}

		var items struct {
			Features []interface{} `json:"features"`
			Tasks    []interface{} `json:"tasks"`
		}
		text := result.Content[0].(mcp.TextContent).Text
		if err := json.Unmarshal([]byte(text), &items); err != nil {
			t.Fatalf("Failed to unmarshal staged items: %v", err)
		}

		if len(items.Features) != 1 {
			t.Errorf("Expected 1 staged feature, got %d", len(items.Features))
		}

		// 4. Commit staged changes
		req = mcp.CallToolRequest{}
		req.Params.Name = "commit_staged_changes"
		req.Params.Arguments = map[string]interface{}{
			"session_id": sessionID,
		}

		tool = s.GetTool("commit_staged_changes")
		result, err = tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Failed to commit staged changes: %v, %v", err, result.Content)
		}

		// Verify in DB
		f, err = database.GetFeatureByName(ctx, "staged-feature")
		if err != nil || f == nil {
			t.Fatal("Feature should be in DB now")
		}

		task, err := database.GetTaskByName(ctx, "staged-task", f.ID)
		if err != nil || task == nil {
			t.Fatal("Task should be in DB now")
		}
	})

	t.Run("mandatory_staging_verification", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = "create_feature"
		req.Params.Arguments = map[string]interface{}{
			"name":          "mandatory-staged-feature",
			"description":   "d",
			"specification": "s",
		}

		tool := s.GetTool("create_feature")
		result, err := tool.Handler(ctx, req)
		if err != nil || result.IsError {
			t.Fatalf("Handler failed: %v, %v", err, result.Content)
		}

		// Verify NOT in DB
		f, _ := database.GetFeatureByName(ctx, "mandatory-staged-feature")
		if f != nil {
			t.Fatal("Feature should not be in DB before commit")
		}

		// Verify in list_staged_changes
		req = mcp.CallToolRequest{}
		req.Params.Name = "list_staged_changes"
		req.Params.Arguments = map[string]interface{}{"session_id": "default"}
		result, _ = s.GetTool("list_staged_changes").Handler(ctx, req)
		text := result.Content[0].(mcp.TextContent).Text
		if !strings.Contains(text, "mandatory-staged-feature") {
			t.Fatal("Feature not found in staged changes")
		}

		// Commit
		s.GetTool("commit_staged_changes").Handler(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "commit_staged_changes",
				Arguments: map[string]interface{}{"session_id": "default"},
			},
		})

		// Now verify in DB
		f, _ = database.GetFeatureByName(ctx, "mandatory-staged-feature")
		if f == nil {
			t.Fatal("Feature should be in DB after commit")
		}
	})
}
