package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkDefaultsUsesConfigFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		t.Fatalf("failed to create .ponder dir: %v", err)
	}

	dbPath = filepath.Join(ponderDir, "ponder.db")
	configPath := filepath.Join(ponderDir, "config.json")
	config := `{
  "model": "test/model",
  "max_concurrency": 9,
  "available_models": ["test/model", "backup/model"]
}
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	defaults, err := loadWorkDefaults()
	if err != nil {
		t.Fatalf("loadWorkDefaults failed: %v", err)
	}

	if defaults.Model != "test/model" {
		t.Errorf("expected model test/model, got %s", defaults.Model)
	}
	if defaults.MaxConcurrency != 9 {
		t.Errorf("expected max concurrency 9, got %d", defaults.MaxConcurrency)
	}
	if len(defaults.AvailableModels) != 2 {
		t.Fatalf("expected 2 available models, got %d", len(defaults.AvailableModels))
	}
	if defaults.AvailableModels[0] != "test/model" || defaults.AvailableModels[1] != "backup/model" {
		t.Errorf("unexpected available models: %v", defaults.AvailableModels)
	}
}

func TestLoadWorkDefaultsWithoutConfigFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-config-defaults-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath = filepath.Join(tmpDir, ".ponder", "ponder.db")

	defaults, err := loadWorkDefaults()
	if err != nil {
		t.Fatalf("loadWorkDefaults failed: %v", err)
	}

	if defaults.Model != defaultWorkModel {
		t.Errorf("expected default model %s, got %s", defaultWorkModel, defaults.Model)
	}
	if defaults.MaxConcurrency != defaultWorkMaxConcurrency {
		t.Errorf("expected default max concurrency %d, got %d", defaultWorkMaxConcurrency, defaults.MaxConcurrency)
	}
	if len(defaults.AvailableModels) != 1 || defaults.AvailableModels[0] != defaultWorkModel {
		t.Errorf("expected default available models [%s], got %v", defaultWorkModel, defaults.AvailableModels)
	}
}

func TestLoadWorkDefaultsAddsConfiguredModelToAvailableList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ponder-config-model-list-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ponderDir := filepath.Join(tmpDir, ".ponder")
	if err := os.MkdirAll(ponderDir, 0755); err != nil {
		t.Fatalf("failed to create .ponder dir: %v", err)
	}

	dbPath = filepath.Join(ponderDir, "ponder.db")
	configPath := filepath.Join(ponderDir, "config.json")
	config := `{
  "model": "missing/model",
  "available_models": ["first/model"]
}
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	defaults, err := loadWorkDefaults()
	if err != nil {
		t.Fatalf("loadWorkDefaults failed: %v", err)
	}

	if len(defaults.AvailableModels) != 2 {
		t.Fatalf("expected 2 available models, got %d", len(defaults.AvailableModels))
	}
	if defaults.AvailableModels[1] != "missing/model" {
		t.Errorf("expected configured model to be appended, got %v", defaults.AvailableModels)
	}
}
