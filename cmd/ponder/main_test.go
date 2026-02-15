package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteRoutesRootToWorkTUI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-root-work-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		t.Fatalf("failed to create .ponder dir: %v", err)
	}

	configPath := filepath.Join(ponderDir, "config.json")
	config := `{
  "model": "cfg/model",
  "max_concurrency": 7,
  "available_models": ["cfg/model", "backup/model"]
}
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	originalDBPath := dbPath
	originalSnapshotPath := snapshotPath
	originalVerbose := verbose
	originalRunOrchestrator := runOrchestrator
	t.Cleanup(func() {
		dbPath = originalDBPath
		snapshotPath = originalSnapshotPath
		verbose = originalVerbose
		runOrchestrator = originalRunOrchestrator
	})

	called := false
	runOrchestrator = func(maxConcurrency int, initialWorkers int, model string, availableModels []string, interval time.Duration, enableWeb bool, webPort string) error {
		called = true
		if maxConcurrency != 7 {
			t.Errorf("expected max concurrency 7, got %d", maxConcurrency)
		}
		if initialWorkers != 0 {
			t.Errorf("expected initial workers 0, got %d", initialWorkers)
		}
		if model != "cfg/model" {
			t.Errorf("expected model cfg/model, got %s", model)
		}
		if len(availableModels) != 2 {
			t.Fatalf("expected 2 available models, got %d", len(availableModels))
		}
		if availableModels[0] != "cfg/model" || availableModels[1] != "backup/model" {
			t.Errorf("unexpected available models: %v", availableModels)
		}
		if interval != 3*time.Second {
			t.Errorf("expected interval 3s, got %v", interval)
		}
		if enableWeb {
			t.Error("expected web to be disabled")
		}
		if webPort != "9001" {
			t.Errorf("expected web port 9001, got %s", webPort)
		}
		return nil
	}

	dbFilePath := filepath.Join(ponderDir, "ponder.db")
	var stderr bytes.Buffer
	err = execute([]string{"--db-path", dbFilePath, "--interval", "3s", "--web=false", "--port", "9001"}, &stderr)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !called {
		t.Fatal("expected root execution to invoke work orchestrator")
	}
}

func TestExecuteRejectsWorkSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	err := execute([]string{"work"}, &stderr)
	if err == nil {
		t.Fatal("expected error for work subcommand")
	}
	if !strings.Contains(err.Error(), "unknown command: work") {
		t.Fatalf("expected unknown work command error, got: %v", err)
	}
}

func TestExecuteHelpShowsRootWorkFlags(t *testing.T) {
	var stderr bytes.Buffer
	err := execute([]string{"--help"}, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected help error, got: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "Running `ponder` with no command launches the Work TUI.") {
		t.Fatalf("expected root TUI help text, got: %s", output)
	}
	if !strings.Contains(output, "-max_concurrency") {
		t.Fatalf("expected max_concurrency in help output, got: %s", output)
	}
	if !strings.Contains(output, "-model") {
		t.Fatalf("expected model in help output, got: %s", output)
	}
	if !strings.Contains(output, "-interval") {
		t.Fatalf("expected interval in help output, got: %s", output)
	}
	if !strings.Contains(output, "-web") {
		t.Fatalf("expected web in help output, got: %s", output)
	}
	if !strings.Contains(output, "-port") {
		t.Fatalf("expected port in help output, got: %s", output)
	}
}
