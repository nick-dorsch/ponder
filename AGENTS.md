# Agent Guidelines for Ponder

Task dependency graph management system with MCP server integration, web UI, and agent-based task processing.

## Build Commands

```bash
# Build the ponder binary
./task build                    # or: go build -o ponder ./cmd/ponder

# Run tests
./task test                     # All tests with integration tags
go test ./...                   # Unit tests only
go test -tags=integration ./... # Integration tests only

# Run a single test
go test -run TestTaskCRUD ./internal/db/

# Run with coverage or race detector
go test -cover ./...
go test -race ./...

# Lint and format
./task lint                     # golangci-lint run
./task fmt                      # go fmt ./... && goimports -w .

# Install locally
./task install                  # go install ./cmd/ponder
```

## Project Structure

```
cmd/ponder/          # Main application entry point
internal/            # Private implementation
  db/               # Database layer (SQLite)
  mcp/              # MCP server implementation
  orchestrator/     # Task orchestration & workers
  server/           # HTTP web server
  ui/               # Terminal UI (bubbletea)
  worker/           # Worker implementations
  task/             # Task management logic
  graph/            # Graph operations
pkg/models/          # Public data models
embed/              # Embedded assets (SQL, prompts)
sql/                # Schema files
test/               # Test fixtures
```

## Code Style Guidelines

### Imports
- Group imports: stdlib, third-party, internal
- Use `goimports` for formatting
- Blank imports for side effects (e.g., `_ "modernc.org/sqlite"`)

### Naming
- **Exported**: PascalCase (e.g., `CreateTask`, `TaskStatus`)
- **Unexported**: camelCase (e.g., `createTask`, `triggerChange`)
- **Constants**: PascalCase for exported (e.g., `TaskStatusPending`)
- **Interfaces**: -er suffix (e.g., `executor` interface)
- **Test files**: `*_test.go`

### Types & Models
- Use `time.Time` for timestamps
- Pointer types for nullable fields (e.g., `*string`, `*time.Time`)
- JSON tags on struct fields: `` `json:"field_name"` ``
- Enum types as typed strings with const values

### Error Handling
- Wrap errors with context: `fmt.Errorf("...: %w", err)`
- Check `sql.ErrNoRows` explicitly when needed
- Return early on errors to reduce nesting

### Functions
- Context as first parameter: `func (db *DB) Method(ctx context.Context, ...)`
- Keep functions focused and under 50 lines
- Use helper methods for repeated logic
- Comment exported functions with `// FunctionName does...`

### Database
- SQLite with WAL mode enabled
- Use `:memory:` for tests
- Transactions for multi-step operations
- Query parameters always (never string concatenation)

### Comments
- All exported identifiers must have doc comments
- Use inline comments sparingly, prefer clear code
- Section comments with `// Section Name`

### Testing
- Use `t.Fatalf()` for setup failures, `t.Errorf()` for assertions
- Test names should describe behavior (e.g., `TestClaimNextTaskWithDependencies`)
- Create features before tasks (tasks require feature_id)
- Use context.Background() in tests

### General
- Go 1.25+ required
- Pure Go (no CGO) - uses modernc.org/sqlite
- Follow effective Go conventions
- Line length: reasonable, no hard limit but avoid >100 chars
