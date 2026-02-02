package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var mode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("Expected journal_mode wal, got %s", mode)
	}

	var fk int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if err != nil {
		t.Fatalf("Failed to query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("Expected foreign_keys enabled (1), got %d", fk)
	}
}

func TestMigrate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE test (
		id INTEGER PRIMARY KEY,
		name TEXT
	);
	`
	ctx := context.Background()
	if err := db.Migrate(ctx, schema); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	_, err = db.Exec("INSERT INTO test (name) VALUES (?)", "foo")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if name != "foo" {
		t.Errorf("Expected foo, got %s", name)
	}
}

func TestInit(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Check if one of the tables from the schema exists, e.g., features
	_, err = db.Exec("SELECT 1 FROM features LIMIT 1")
	if err != nil {
		t.Fatalf("Features table does not exist or query failed: %v", err)
	}
}
