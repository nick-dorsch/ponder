package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nick-dorsch/ponder/internal/db"
	"github.com/nick-dorsch/ponder/pkg/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates a new MCP server.
func NewServer(database *db.DB) *server.MCPServer {
	s := server.NewMCPServer("Ponder", "0.1.0")

	// Feature Management
	s.AddTool(mcp.NewTool("create_feature",
		mcp.WithDescription("Propose a new feature. Changes are staged and must be committed to take effect."),
		mcp.WithString("name", mcp.Description("Feature name (max 55 chars, unique)"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Feature description"), mcp.Required()),
		mcp.WithString("specification", mcp.Description("Feature specification"), mcp.Required()),
		mcp.WithString("session_id", mcp.Description("Session ID for staging changes (defaults to 'default').")),
	), createFeatureHandler(database))

	s.AddTool(mcp.NewTool("update_feature",
		mcp.WithDescription("Update an existing feature."),
		mcp.WithString("name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("new_name", mcp.Description("New name")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("specification", mcp.Description("New specification")),
	), updateFeatureHandler(database))

	s.AddTool(mcp.NewTool("delete_feature",
		mcp.WithDescription("Delete a feature (cascades to tasks)."),
		mcp.WithString("name", mcp.Description("Feature name"), mcp.Required()),
	), deleteFeatureHandler(database))

	s.AddTool(mcp.NewTool("list_features",
		mcp.WithDescription("List all features."),
	), listFeaturesHandler(database))

	s.AddTool(mcp.NewTool("get_feature",
		mcp.WithDescription("Get a single feature by name."),
		mcp.WithString("name", mcp.Description("Feature name"), mcp.Required()),
	), getFeatureHandler(database))

	// Task Management
	s.AddTool(mcp.NewTool("create_task",
		mcp.WithDescription("Propose a new task. Changes are staged and must be committed to take effect."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name (max 55 chars)"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Task description"), mcp.Required()),
		mcp.WithString("specification", mcp.Description("Task specification"), mcp.Required()),
		mcp.WithNumber("priority", mcp.Description("Priority (0-10)")),
		mcp.WithBoolean("tests_required", mcp.Description("Whether tests are required")),
		mcp.WithString("session_id", mcp.Description("Session ID for staging changes (defaults to 'default').")),
	), createTaskHandler(database))

	s.AddTool(mcp.NewTool("update_task",
		mcp.WithDescription("Update an existing task."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
		mcp.WithString("new_name", mcp.Description("New name")),
		mcp.WithString("new_feature_name", mcp.Description("New feature name")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("specification", mcp.Description("New specification")),
		mcp.WithNumber("priority", mcp.Description("New priority")),
		mcp.WithBoolean("tests_required", mcp.Description("New tests required status")),
	), updateTaskHandler(database))

	s.AddTool(mcp.NewTool("update_task_status",
		mcp.WithDescription("Update task status."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
		mcp.WithString("status", mcp.Description("New status (pending|in_progress|completed|blocked)"), mcp.Required()),
		mcp.WithString("completion_summary", mcp.Description("Summary of work (required if status=completed)")),
	), updateTaskStatusHandler(database))

	s.AddTool(mcp.NewTool("delete_task",
		mcp.WithDescription("Delete a task."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
	), deleteTaskHandler(database))

	s.AddTool(mcp.NewTool("list_tasks",
		mcp.WithDescription("List tasks with optional filters."),
		mcp.WithString("feature_name", mcp.Description("Filter by feature name")),
		mcp.WithString("status", mcp.Description("Filter by status")),
	), listTasksHandler(database))

	s.AddTool(mcp.NewTool("get_available_tasks",
		mcp.WithDescription("Get tasks that are ready to work on."),
	), getAvailableTasksHandler(database))

	s.AddTool(mcp.NewTool("start_task",
		mcp.WithDescription("Start a task by setting its status to in_progress."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
	), startTaskHandler(database))

	s.AddTool(mcp.NewTool("complete_task",
		mcp.WithDescription("Complete a task by setting its status to completed."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
		mcp.WithString("completion_summary", mcp.Description("Summary of the completed task"), mcp.Required()),
	), completeTaskHandler(database))

	s.AddTool(mcp.NewTool("report_task_blocked",
		mcp.WithDescription("Report a task as blocked and provide a reason."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
		mcp.WithString("reason", mcp.Description("Reason why the task is blocked"), mcp.Required()),
	), reportTaskBlockedHandler(database))

	// Dependency Management
	s.AddTool(mcp.NewTool("create_dependency",
		mcp.WithDescription("Propose a dependency between two tasks. Changes are staged and must be committed to take effect."),
		mcp.WithString("feature_name", mcp.Description("Feature name of the dependent task"), mcp.Required()),
		mcp.WithString("task_name", mcp.Description("Task name of the dependent task"), mcp.Required()),
		mcp.WithString("depends_on_task_name", mcp.Description("Task name of the prerequisite task"), mcp.Required()),
		mcp.WithString("depends_on_feature_name", mcp.Description("Feature name of the prerequisite task (defaults to feature_name)")),
		mcp.WithString("session_id", mcp.Description("Session ID for staging changes (defaults to 'default').")),
	), createDependencyHandler(database))

	s.AddTool(mcp.NewTool("delete_dependency",
		mcp.WithDescription("Remove a dependency."),
		mcp.WithString("feature_name", mcp.Description("Feature name of the dependent task"), mcp.Required()),
		mcp.WithString("task_name", mcp.Description("Task name of the dependent task"), mcp.Required()),
		mcp.WithString("depends_on_task_name", mcp.Description("Task name of the prerequisite task"), mcp.Required()),
		mcp.WithString("depends_on_feature_name", mcp.Description("Feature name of the prerequisite task (defaults to feature_name)")),
	), deleteDependencyHandler(database))

	s.AddTool(mcp.NewTool("get_task_dependencies",
		mcp.WithDescription("Get all tasks that a task depends on."),
		mcp.WithString("feature_name", mcp.Description("Feature name"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Task name"), mcp.Required()),
	), getTaskDependenciesHandler(database))

	// Graph Queries
	s.AddTool(mcp.NewTool("get_graph_json",
		mcp.WithDescription("Get the complete task graph as JSON."),
	), getGraphJSONHandler(database))

	// Staging Management
	s.AddTool(mcp.NewTool("commit_staged_changes",
		mcp.WithDescription("Commit all staged changes for a session. This applies all proposed features, tasks, and dependencies at once."),
		mcp.WithString("session_id", mcp.Description("Session ID (defaults to 'default').")),
	), commitStagedChangesHandler(database))

	s.AddTool(mcp.NewTool("list_staged_changes",
		mcp.WithDescription("List all staged changes for a session. Use this to review a proposed plan before committing."),
		mcp.WithString("session_id", mcp.Description("Session ID (defaults to 'default').")),
	), listStagedChangesHandler(database))

	return s
}

// Serve starts the MCP server on stdio.
func Serve(s *server.MCPServer) error {
	return server.ServeStdio(s)
}

func createFeatureHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")
		description := mcp.ParseString(request, "description", "")
		specification := mcp.ParseString(request, "specification", "")
		sessionID := mcp.ParseString(request, "session_id", "default")

		f := &models.Feature{
			Name:          name,
			Description:   description,
			Specification: specification,
		}

		database.Staging.AddFeature(sessionID, f)
		return mcp.NewToolResultText(fmt.Sprintf("Feature '%s' staged for session '%s'. Propose another or call 'commit_staged_changes' to apply.", name, sessionID)), nil
	}
}

func updateFeatureHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")

		f, err := database.GetFeatureByName(ctx, name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Feature with name '%s' not found", name)), nil
		}

		args, _ := request.Params.Arguments.(map[string]any)
		if newName, ok := args["new_name"].(string); ok {
			f.Name = newName
		}
		if description, ok := args["description"].(string); ok {
			f.Description = description
		}
		if specification, ok := args["specification"].(string); ok {
			f.Specification = specification
		}

		if err := database.UpdateFeature(ctx, f); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Feature updated successfully"), nil
	}
}

func deleteFeatureHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")

		f, err := database.GetFeatureByName(ctx, name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Feature with name '%s' not found", name)), nil
		}

		if err := database.DeleteFeature(ctx, f.ID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Feature deleted successfully"), nil
	}
}

func listFeaturesHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		features, err := database.ListFeatures(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, err := json.Marshal(map[string]interface{}{"features": features})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func getFeatureHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := mcp.ParseString(request, "name", "")

		f, err := database.GetFeatureByName(ctx, name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Feature with name '%s' not found", name)), nil
		}

		data, err := json.Marshal(f)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func createTaskHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")
		description := mcp.ParseString(request, "description", "")
		specification := mcp.ParseString(request, "specification", "")
		priority := mcp.ParseInt(request, "priority", 0)
		testsRequired := mcp.ParseBoolean(request, "tests_required", true)
		sessionID := mcp.ParseString(request, "session_id", "default")

		t := &models.Task{
			FeatureName:   featureName, // Store name for staging resolution
			Name:          name,
			Description:   description,
			Specification: specification,
			Priority:      priority,
			TestsRequired: testsRequired,
			Status:        models.TaskStatusPending,
		}

		database.Staging.AddTask(sessionID, t)
		return mcp.NewToolResultText(fmt.Sprintf("Task '%s' staged for session '%s'. Propose another or call 'commit_staged_changes' to apply.", name, sessionID)), nil
	}
}

func updateTaskHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")

		f, err := database.GetFeatureByName(ctx, featureName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Feature with name '%s' not found", featureName)), nil
		}

		t, err := database.GetTaskByName(ctx, name, f.ID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if t == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Task with name '%s' not found in feature '%s'", name, featureName)), nil
		}

		args, _ := request.Params.Arguments.(map[string]any)
		if newName, ok := args["new_name"].(string); ok {
			t.Name = newName
		}
		if newFeatureName, ok := args["new_feature_name"].(string); ok {
			nf, err := database.GetFeatureByName(ctx, newFeatureName)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if nf == nil {
				return mcp.NewToolResultError(fmt.Sprintf("New feature with name '%s' not found", newFeatureName)), nil
			}
			t.FeatureID = nf.ID
		}
		if description, ok := args["description"].(string); ok {
			t.Description = description
		}
		if specification, ok := args["specification"].(string); ok {
			t.Specification = specification
		}
		if priority, ok := args["priority"].(float64); ok {
			t.Priority = int(priority)
		}
		if testsRequired, ok := args["tests_required"].(bool); ok {
			t.TestsRequired = testsRequired
		}

		if err := database.UpdateTask(ctx, t); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Task updated successfully"), nil
	}
}

func updateTaskStatusHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")
		status := mcp.ParseString(request, "status", "")

		f, err := database.GetFeatureByName(ctx, featureName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Feature with name '%s' not found", featureName)), nil
		}

		t, err := database.GetTaskByName(ctx, name, f.ID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if t == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Task with name '%s' not found in feature '%s'", name, featureName)), nil
		}

		var summary *string
		args, _ := request.Params.Arguments.(map[string]any)
		if s, ok := args["completion_summary"].(string); ok {
			summary = &s
		}

		if err := database.UpdateTaskStatus(ctx, t.ID, models.TaskStatus(status), summary); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Task status updated successfully"), nil
	}
}

func deleteTaskHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")

		f, err := database.GetFeatureByName(ctx, featureName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Feature with name '%s' not found", featureName)), nil
		}

		t, err := database.GetTaskByName(ctx, name, f.ID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if t == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Task with name '%s' not found in feature '%s'", name, featureName)), nil
		}

		if err := database.DeleteTask(ctx, t.ID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Task deleted successfully"), nil
	}
}

func listTasksHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := request.Params.Arguments.(map[string]any)
		var status *models.TaskStatus
		if s, ok := args["status"].(string); ok {
			ts := models.TaskStatus(s)
			status = &ts
		}

		var featureName *string
		if fn, ok := args["feature_name"].(string); ok {
			featureName = &fn
		}

		tasks, err := database.ListTasks(ctx, status, featureName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, err := json.Marshal(map[string]interface{}{"tasks": tasks})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func getAvailableTasksHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tasks, err := database.GetAvailableTasks(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, err := json.Marshal(map[string]interface{}{"tasks": tasks})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func createDependencyHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		taskName := mcp.ParseString(request, "task_name", "")
		dependsOnTaskName := mcp.ParseString(request, "depends_on_task_name", "")
		dependsOnFeatureName := mcp.ParseString(request, "depends_on_feature_name", featureName)
		sessionID := mcp.ParseString(request, "session_id", "default")

		database.Staging.AddDependency(sessionID, &models.Dependency{
			TaskName:             taskName,
			FeatureName:          featureName,
			DependsOnTaskName:    dependsOnTaskName,
			DependsOnFeatureName: dependsOnFeatureName,
		})
		return mcp.NewToolResultText(fmt.Sprintf("Dependency %s:%s -> %s:%s staged for session '%s'. call 'commit_staged_changes' to apply.", featureName, taskName, dependsOnFeatureName, dependsOnTaskName, sessionID)), nil
	}
}

func deleteDependencyHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		taskName := mcp.ParseString(request, "task_name", "")
		dependsOnTaskName := mcp.ParseString(request, "depends_on_task_name", "")
		dependsOnFeatureName := mcp.ParseString(request, "depends_on_feature_name", featureName)

		taskID, err := resolveTaskID(ctx, database, featureName, taskName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dependsOnTaskID, err := resolveTaskID(ctx, database, dependsOnFeatureName, dependsOnTaskName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := database.DeleteDependency(ctx, taskID, dependsOnTaskID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Dependency deleted successfully"), nil
	}
}

func getTaskDependenciesHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")

		taskID, err := resolveTaskID(ctx, database, featureName, name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		deps, err := database.GetDependencies(ctx, taskID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, err := json.Marshal(map[string]interface{}{"dependencies": deps})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func getGraphJSONHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		json, err := database.GetGraphJSON(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(json), nil
	}
}

func startTaskHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")

		taskID, err := resolveTaskID(ctx, database, featureName, name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := database.UpdateTaskStatus(ctx, taskID, models.TaskStatusInProgress, nil); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Task started successfully"), nil
	}
}

func completeTaskHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")
		summary := mcp.ParseString(request, "completion_summary", "")

		taskID, err := resolveTaskID(ctx, database, featureName, name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := database.UpdateTaskStatus(ctx, taskID, models.TaskStatusCompleted, &summary); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Task completed successfully"), nil
	}
}

func reportTaskBlockedHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		featureName := mcp.ParseString(request, "feature_name", "")
		name := mcp.ParseString(request, "name", "")
		reason := mcp.ParseString(request, "reason", "")

		f, err := database.GetFeatureByName(ctx, featureName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if f == nil {
			return mcp.NewToolResultError(fmt.Sprintf("feature with name '%s' not found", featureName)), nil
		}

		t, err := database.GetTaskByName(ctx, name, f.ID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if t == nil {
			return mcp.NewToolResultError(fmt.Sprintf("task with name '%s' not found in feature '%s'", name, featureName)), nil
		}

		t.Specification += fmt.Sprintf("\n\n### Blocked Reason\n%s", reason)
		if err := database.UpdateTask(ctx, t); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := database.UpdateTaskStatus(ctx, t.ID, models.TaskStatusBlocked, nil); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText("Task reported as blocked successfully"), nil
	}
}

func commitStagedChangesHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID := mcp.ParseString(request, "session_id", "default")
		if err := database.CommitBatch(ctx, sessionID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Staged changes for session '%s' committed successfully", sessionID)), nil
	}
}

func listStagedChangesHandler(database *db.DB) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID := mcp.ParseString(request, "session_id", "default")

		items := database.Staging.Peek(sessionID)
		data, err := json.Marshal(items)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func resolveTaskID(ctx context.Context, database *db.DB, featureName, taskName string) (string, error) {
	f, err := database.GetFeatureByName(ctx, featureName)
	if err != nil {
		return "", err
	}
	if f == nil {
		return "", fmt.Errorf("feature with name '%s' not found", featureName)
	}

	t, err := database.GetTaskByName(ctx, taskName, f.ID)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", fmt.Errorf("task with name '%s' not found in feature '%s'", taskName, featureName)
	}

	return t.ID, nil
}
