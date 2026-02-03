package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath = ".ponder/ponder.db"
	snapshotPath = ".ponder/snapshot.jsonl"

	err = runInit([]string{tmpDir})
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if _, err := os.Stat(ponderDir); os.IsNotExist(err) {
		t.Errorf(".ponder directory was not created")
	}

	gitignorePath := filepath.Join(ponderDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Errorf("failed to read .gitignore: %v", err)
	}
	if string(content) != "ponder.db*\n" {
		t.Errorf(".gitignore content mismatch: expected 'ponder.db*\\n', got %q", string(content))
	}

	dbFilePath := filepath.Join(ponderDir, "ponder.db")
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		t.Errorf("database file was not created")
	}
}

func TestInitWithExistingSnapshot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-test-snapshot-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		t.Fatalf("failed to create .ponder dir: %v", err)
	}

	snapshotPath := filepath.Join(ponderDir, "snapshot.jsonl")
	snapshotContent := `{"record_type":"feature","name":"test-feature","description":"test","specification":"test"}
`
	if err := os.WriteFile(snapshotPath, []byte(snapshotContent), 0644); err != nil {
		t.Fatalf("failed to create dummy snapshot: %v", err)
	}

	dbPath = ".ponder/ponder.db"
	defer func() {
		dbPath = ".ponder/ponder.db"
		globalsnapshotPath := ".ponder/snapshot.jsonl"
		_ = globalsnapshotPath
	}()

	err = runInit([]string{tmpDir})
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	dbFilePath := filepath.Join(ponderDir, "ponder.db")
	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		t.Errorf("database file was not created")
	}
}

func TestInitOverwritesGitignore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-test-overwrite-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		t.Fatalf("failed to create .ponder dir: %v", err)
	}

	gitignorePath := filepath.Join(ponderDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("old-content\n"), 0644); err != nil {
		t.Fatalf("failed to create initial .gitignore: %v", err)
	}

	dbPath = ".ponder/ponder.db"
	snapshotPath = ".ponder/snapshot.jsonl"

	err = runInit([]string{tmpDir})
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if string(content) != "ponder.db*\n" {
		t.Errorf(".gitignore was not overwritten: expected 'ponder.db*\\n', got %q", string(content))
	}
}
