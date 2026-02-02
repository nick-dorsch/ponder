package db

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ldi/ponder/pkg/models"
)

func TestExportSnapshot(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// Create some data
	f := &models.Feature{
		Name:          "Test Feature",
		Description:   "Description",
		Specification: "Specification",
	}
	if err := db.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

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

	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "snapshot.jsonl")

	if err := db.ExportSnapshot(ctx, snapshotPath); err != nil {
		t.Fatalf("Failed to export snapshot: %v", err)
	}

	// Verify snapshot file exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Fatalf("Snapshot file was not created")
	}

	// Read and verify lines
	file, err := os.Open(snapshotPath)
	if err != nil {
		t.Fatalf("Failed to open snapshot file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner error: %v", err)
	}

	// At least 3 lines: meta, feature, task
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(lines))
	}

	// Verify first line is meta
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
		t.Fatalf("Failed to unmarshal meta line: %v", err)
	}
	if meta["record_type"] != "meta" {
		t.Errorf("Expected first line to be meta, got %v", meta["record_type"])
	}

	// Verify feature line
	foundFeature := false
	for _, line := range lines {
		var rec map[string]interface{}
		json.Unmarshal([]byte(line), &rec)
		if rec["record_type"] == "feature" && rec["name"] == "Test Feature" {
			foundFeature = true
			if rec["id"] == nil || rec["id"] == "" {
				t.Errorf("Feature line missing id field")
			}
			break
		}
	}
	if !foundFeature {
		t.Errorf("Feature not found in snapshot")
	}

	// Verify task line
	foundTask := false
	for _, line := range lines {
		var rec map[string]interface{}
		json.Unmarshal([]byte(line), &rec)
		if rec["record_type"] == "task" && rec["name"] == "Test Task" {
			foundTask = true
			if rec["id"] == nil || rec["id"] == "" {
				t.Errorf("Task line missing id field")
			}
			break
		}
	}
	if !foundTask {
		t.Errorf("Task not found in snapshot")
	}
}

func TestImportSnapshot(t *testing.T) {
	ctx := context.Background()

	// 1. Setup source DB and export a snapshot
	srcDB, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open source database: %v", err)
	}
	defer srcDB.Close()

	if err := srcDB.Init(ctx); err != nil {
		t.Fatalf("Failed to init source database: %v", err)
	}

	f := &models.Feature{
		Name:          "Feature 1",
		Description:   "Desc 1",
		Specification: "Spec 1",
	}
	if err := srcDB.CreateFeature(ctx, f); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}

	t1 := &models.Task{
		FeatureID:     f.ID,
		Name:          "Task 1",
		Description:   "T1 Desc",
		Specification: "T1 Spec",
		Status:        models.TaskStatusPending,
	}
	if err := srcDB.CreateTask(ctx, t1); err != nil {
		t.Fatalf("Failed to create task 1: %v", err)
	}

	t2 := &models.Task{
		FeatureID:     f.ID,
		Name:          "Task 2",
		Description:   "T2 Desc",
		Specification: "T2 Spec",
		Status:        models.TaskStatusInProgress,
	}
	if err := srcDB.CreateTask(ctx, t2); err != nil {
		t.Fatalf("Failed to create task 2: %v", err)
	}

	if err := srcDB.CreateDependency(ctx, t2.ID, t1.ID); err != nil {
		t.Fatalf("Failed to create dependency: %v", err)
	}

	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "snapshot.jsonl")
	if err := srcDB.ExportSnapshot(ctx, snapshotPath); err != nil {
		t.Fatalf("Failed to export snapshot: %v", err)
	}

	// 2. Setup destination DB and import the snapshot
	dstDB, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer dstDB.Close()

	if err := dstDB.Init(ctx); err != nil {
		t.Fatalf("Failed to init destination database: %v", err)
	}

	if err := dstDB.ImportSnapshot(ctx, snapshotPath); err != nil {
		t.Fatalf("Failed to import snapshot: %v", err)
	}

	// 3. Verify destination DB state
	// Verify feature
	f_dst, err := dstDB.GetFeatureByName(ctx, "Feature 1")
	if err != nil {
		t.Fatalf("Failed to get feature: %v", err)
	}
	if f_dst == nil {
		t.Fatal("Feature 1 not found in destination DB")
	}
	if f_dst.Description != "Desc 1" {
		t.Errorf("Expected description 'Desc 1', got '%s'", f_dst.Description)
	}

	// Verify tasks
	t1_dst, err := dstDB.GetTaskByName(ctx, "Task 1", f_dst.ID)
	if err != nil {
		t.Fatalf("Failed to get task 1: %v", err)
	}
	if t1_dst == nil {
		t.Fatal("Task 1 not found in destination DB")
	}

	t2_dst, err := dstDB.GetTaskByName(ctx, "Task 2", f_dst.ID)
	if err != nil {
		t.Fatalf("Failed to get task 2: %v", err)
	}
	if t2_dst == nil {
		t.Fatal("Task 2 not found in destination DB")
	}
	if t2_dst.Status != models.TaskStatusInProgress {
		t.Errorf("Expected status 'in_progress', got '%s'", t2_dst.Status)
	}

	// Verify dependency
	deps, err := dstDB.GetDependencies(ctx, t2_dst.ID)
	if err != nil {
		t.Fatalf("Failed to get dependencies: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("Expected 1 dependency for Task 2, got %d", len(deps))
	}
	if deps[0].Name != "Task 1" {
		t.Errorf("Expected Task 2 to depend on Task 1, got %s", deps[0].Name)
	}
}

func TestImportSnapshotMerge(t *testing.T) {
	ctx := context.Background()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	// 1. Create a local feature and task
	f1 := &models.Feature{
		Name:          "Merge Feature",
		Description:   "Local Desc",
		Specification: "Local Spec",
	}
	if err := db.CreateFeature(ctx, f1); err != nil {
		t.Fatalf("Failed to create feature: %v", err)
	}
	localFeatureID := f1.ID

	t1 := &models.Task{
		FeatureID:     localFeatureID,
		Name:          "Merge Task",
		Description:   "Local T Desc",
		Specification: "Local T Spec",
		Status:        models.TaskStatusPending,
	}
	if err := db.CreateTask(ctx, t1); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}
	localTaskID := t1.ID

	// 2. Create a snapshot that matches by name but has DIFFERENT IDs
	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "merge_snapshot.jsonl")

	f_snap_id := "00000000-0000-0000-0000-000000000001"
	t_snap_id := "00000000-0000-0000-0000-000000000002"

	// Manually construct JSONL to control IDs
	lines := []string{
		`{"record_type": "meta", "schema_version": "1"}`,
		fmt.Sprintf(`{"record_type": "feature", "id": "%s", "name": "Merge Feature", "description": "Snapshot Desc", "specification": "Spec"}`, f_snap_id),
		fmt.Sprintf(`{"record_type": "task", "id": "%s", "feature_name": "Merge Feature", "name": "Merge Task", "description": "Snapshot T Desc", "specification": "Spec", "status": "pending"}`, t_snap_id),
	}

	err = os.WriteFile(snapshotPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write manual snapshot: %v", err)
	}

	// 3. Import the snapshot
	if err := db.ImportSnapshot(ctx, snapshotPath); err != nil {
		t.Fatalf("Failed to import snapshot: %v", err)
	}

	// 4. Verify that IDs were preserved (Local IDs kept)
	f_merged, err := db.GetFeatureByName(ctx, "Merge Feature")
	if err != nil {
		t.Fatalf("GetFeatureByName failed: %v", err)
	}
	if f_merged.ID != localFeatureID {
		t.Errorf("Feature ID mismatch: expected %s, got %s", localFeatureID, f_merged.ID)
	}
	if f_merged.Description != "Snapshot Desc" {
		t.Errorf("Feature description not updated: expected 'Snapshot Desc', got '%s'", f_merged.Description)
	}

	t_merged, err := db.GetTaskByName(ctx, "Merge Task", f_merged.ID)
	if err != nil {
		t.Fatalf("GetTaskByName failed: %v", err)
	}
	if t_merged.ID != localTaskID {
		t.Errorf("Task ID mismatch: expected %s, got %s", localTaskID, t_merged.ID)
	}
	if t_merged.Description != "Snapshot T Desc" {
		t.Errorf("Task description not updated: expected 'Snapshot T Desc', got '%s'", t_merged.Description)
	}
}

func TestImportSnapshotPreserveIDs(t *testing.T) {
	ctx := context.Background()

	// 1. Create a snapshot with specific IDs
	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "preserve_id_snapshot.jsonl")

	f_snap_id := "01234567-89ab-cdef-0123-456789abcdef"
	t_snap_id := "fedcba98-7654-3210-fedc-ba9876543210"

	lines := []string{
		`{"record_type": "meta", "schema_version": "1"}`,
		fmt.Sprintf(`{"record_type": "feature", "id": "%s", "name": "Preserve Feature", "description": "Desc", "specification": "Spec"}`, f_snap_id),
		fmt.Sprintf(`{"record_type": "task", "id": "%s", "feature_name": "Preserve Feature", "name": "Preserve Task", "description": "T Desc", "specification": "Spec", "status": "pending"}`, t_snap_id),
	}

	err := os.WriteFile(snapshotPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write manual snapshot: %v", err)
	}

	// 2. Import into a fresh DB
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Init(ctx); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	if err := db.ImportSnapshot(ctx, snapshotPath); err != nil {
		t.Fatalf("Failed to import snapshot: %v", err)
	}

	// 3. Verify IDs
	f, _ := db.GetFeatureByName(ctx, "Preserve Feature")
	if f.ID != f_snap_id {
		t.Errorf("Feature ID not preserved: expected %s, got %s", f_snap_id, f.ID)
	}

	t_task, _ := db.GetTaskByName(ctx, "Preserve Task", f.ID)
	if t_task.ID != t_snap_id {
		t.Errorf("Task ID not preserved: expected %s, got %s", t_snap_id, t_task.ID)
	}
}

func TestImportSnapshotBrokenDependency(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "broken_dep_snapshot.jsonl")

	lines := []string{
		`{"record_type": "meta", "schema_version": "1"}`,
		`{"record_type": "feature", "id": "00000000-0000-0000-0000-00000000000f", "name": "F1", "description": "D", "specification": "S"}`,
		`{"record_type": "task", "id": "00000000-0000-0000-0000-000000000001", "feature_name": "F1", "name": "T1", "description": "D", "specification": "S", "status": "pending"}`,
		`{"record_type": "dependency", "task_id": "00000000-0000-0000-0000-000000000001", "task_name": "T1", "task_feature_name": "F1", "depends_on_task_id": "NON_EXISTENT", "depends_on_task_name": "NON_EXISTENT", "depends_on_task_feature_name": "F1"}`,
	}

	err := os.WriteFile(snapshotPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write manual snapshot: %v", err)
	}

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	db.Init(ctx)

	err = db.ImportSnapshot(ctx, snapshotPath)
	if err == nil {
		t.Fatal("Expected import to fail due to broken dependency, but it succeeded")
	}
	if !strings.Contains(err.Error(), "dependent task not found") {
		t.Errorf("Expected error message to contain 'dependent task not found', got: %v", err)
	}
}
