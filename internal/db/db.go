package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	embedsql "github.com/nick-dorsch/ponder/embed/sql"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	Staging          *StagingManager
	onChange         func(ctx context.Context)
	onChangeMu       sync.RWMutex
	onChangeDisabled bool
}

type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (db *DB) SetOnChange(fn func(ctx context.Context)) {
	db.onChangeMu.Lock()
	defer db.onChangeMu.Unlock()
	db.onChange = fn
}

func (db *DB) DisableOnChange() {
	db.onChangeMu.Lock()
	defer db.onChangeMu.Unlock()
	db.onChangeDisabled = true
}

func (db *DB) EnableOnChange() {
	db.onChangeMu.Lock()
	defer db.onChangeMu.Unlock()
	db.onChangeDisabled = false
}

func (db *DB) triggerChange(ctx context.Context) {
	db.onChangeMu.RLock()
	fn := db.onChange
	disabled := db.onChangeDisabled
	db.onChangeMu.RUnlock()

	if fn != nil && !disabled {
		fn(ctx)
	}
}

// Open opens a SQLite database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Foreign keys support
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// SQLite works best with a single writer.
	db.SetMaxOpenConns(1)

	return &DB{
		DB:      db,
		Staging: NewStagingManager(),
	}, nil
}

func (db *DB) Migrate(ctx context.Context, schema string) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	db.triggerChange(ctx)
	return nil
}

func (db *DB) Init(ctx context.Context) error {
	return db.Migrate(ctx, embedsql.Schema)
}

func (db *DB) GetGraphJSON(ctx context.Context) (string, error) {
	var json string
	query := `SELECT graph_json FROM v_graph_json`
	err := db.QueryRowContext(ctx, query).Scan(&json)
	if err != nil {
		return "", fmt.Errorf("failed to get graph json: %w", err)
	}
	return json, nil
}
