package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nick-dorsch/ponder/internal/db"
	"github.com/nick-dorsch/ponder/pkg/models"
)

func setupTestDB(t *testing.T) (string, string) {
	tmpDir, err := os.MkdirTemp("", "ponder-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		t.Fatalf("failed to create .ponder dir: %v", err)
	}

	dbFilePath := filepath.Join(ponderDir, "ponder.db")

	dbPath = dbFilePath

	database, err := db.Open(dbFilePath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	if err := database.Init(context.Background()); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	ctx := context.Background()
	f1 := &models.Feature{Name: "feature1", Description: "desc1"}
	if err := database.CreateFeature(ctx, f1); err != nil {
		t.Fatalf("failed to create feature: %v", err)
	}

	t1 := &models.Task{
		FeatureID: f1.ID,
		Name:      "task1",
		Priority:  10,
		Status:    models.TaskStatusPending,
	}
	if err := database.CreateTask(ctx, t1); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	return tmpDir, dbFilePath
}

func TestListFeatures(t *testing.T) {
	tmpDir, _ := setupTestDB(t)
	defer os.RemoveAll(tmpDir)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runListFeatures([]string{})
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runListFeatures failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "feature1") {
		t.Errorf("output missing feature1: %s", output)
	}
}

func TestListTasks(t *testing.T) {
	tmpDir, _ := setupTestDB(t)
	defer os.RemoveAll(tmpDir)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runListTasks([]string{})
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runListTasks failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "task1") {
		t.Errorf("output missing task1: %s", output)
	}
}

func TestStatus(t *testing.T) {
	tmpDir, _ := setupTestDB(t)
	defer os.RemoveAll(tmpDir)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runStatus([]string{})
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runStatus failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Total Tasks:     1") {
		t.Errorf("output missing total tasks count: %s", output)
	}
}

func TestDBStatus(t *testing.T) {
	tmpDir, _ := setupTestDB(t)
	defer os.RemoveAll(tmpDir)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDB([]string{"status"})
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runDB status failed: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Total Tasks:     1") {
		t.Errorf("output missing total tasks count: %s", output)
	}
}
