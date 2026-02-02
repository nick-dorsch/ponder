# Ponder

<p align="center">
  <img src="media/pondering-orb.png" alt="Pondering My Orb" width="600">
</p>

<p align="center">
  <em>"Pondering my tasks..."</em>
</p>

> Task dependency graph management with MCP server integration, web visualization, and agent-based task processing.

## Overview

Ponder is a lightweight, SQLite-based task management system designed for AI agents and developers. It manages tasks with dependency relationships, tracks work progress, and integrates seamlessly with MCP (Model Context Protocol) servers.

## Features

- **Task Management**: Create, update, and track tasks with priorities, descriptions, and specifications
- **Feature Organization**: Group tasks into features/projects for better organization
- **Dependency Graphs**: Define task dependencies to ensure proper execution order
- **Status Tracking**: Track task states (pending, in_progress, completed, blocked)
- **MCP Integration**: Full MCP server implementation for agent-based task processing
- **Auto-Snapshot**: Automatic JSONL export after every database change
- **Web Server**: Built-in visualization server (port 8000)
- **Pure Go**: Zero CGO dependencies with modernc.org/sqlite

## Installation

```bash
# Download dependencies
go mod download

# Build binary
go build -o ponder ./cmd/ponder

# Install locally
go install ./cmd/ponder

# Release build (stripped)
go build -ldflags "-s -w" -o ponder-linux-amd64 ./cmd/ponder
```

## Quick Start

```bash
# Initialize ponder in your project
ponder init

# Start the MCP server (for agent integration)
ponder mcp
```

## Project Structure

```
.ponder/
├── ponder.db          # SQLite database
├── snapshot.jsonl     # Auto-exported tasks
└── .gitignore         # Ignores database file
```

## Usage

### Commands

```bash
# Initialize Ponder in a directory
ponder init [directory]

# Start MCP server for agent integration
ponder mcp

# Flags (available for all commands)
ponder --db-path /path/to/custom.db --snapshot-path /path/to/snapshot.jsonl --verbose
```

### MCP Tools

Ponder exposes the following MCP tools for agent integration:

**Features**
- `create_feature` - Create a new feature
- `update_feature` - Update an existing feature
- `delete_feature` - Delete a feature (cascades to tasks)
- `list_features` - List all features
- `get_feature` - Get a single feature by ID

**Tasks**
- `create_task` - Create a new task
- `update_task` - Update an existing task
- `update_task_status` - Update task status (pending/in_progress/completed/blocked)
- `delete_task` - Delete a task
- `list_tasks` - List tasks with optional filters
- `get_available_tasks` - Get tasks ready to work on

**Dependencies**
- `create_dependency` - Create a dependency between tasks
- `delete_dependency` - Remove a dependency
- `get_task_dependencies` - Get all tasks a task depends on

**Graph**
- `get_graph_json` - Get the complete task graph as JSON

### Example Task Flow

```bash
# Agent creates a feature
create_feature name="auth-system" description="User authentication" specification="Implement JWT-based auth"

# Agent creates tasks
create_task feature_id="<id>" name="Create login endpoint" priority=8
create_task feature_id="<id>" name="Add password hashing" priority=7

# Agent sets up dependencies
create_dependency task_id="<login-task-id>" depends_on_task_id="<hashing-task-id>"

# Agent marks tasks complete
update_task_status id="<task-id>" status="completed" completion_summary="Implemented bcrypt hashing"
```

## Development

### Testing

```bash
# Run all tests
go test ./...

# Run specific test
go test -run TestTaskCRUD ./internal/db/

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...
```

### Code Style

```bash
# Format all code
go fmt ./...

# Or use goimports
goimports -w .

# Lint
golangci-lint run
```

## Architecture

- **SQLite**: Pure Go SQLite with WAL mode for concurrent access
- **MCP Server**: stdio-based MCP server for agent integration
- **Models**: Clean data models with Pydantic-style patterns
- **Snapshots**: JSONL format for easy versioning and portability
- **Dependencies**: DAG validation to prevent circular dependencies

## Requirements

- Go 1.25+
- SQLite (embedded)

## Dependencies

**Core:**
- `modernc.org/sqlite` - Pure Go SQLite
- `github.com/google/uuid` - UUID generation
- `github.com/mark3labs/mcp-go` - MCP SDK

## License

MIT

---

<p align="center">
  <sub>Built with pure Go and a healthy dose of orb pondering 🧙‍♂️</sub>
</p>
