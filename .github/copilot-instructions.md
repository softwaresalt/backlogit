---
description: Shared backlogit development guidelines for custom agents.
maturity: stable
---

# Backlogit Development Guidelines

Last updated: 2026-03-29

backlogit is a highly configurable, file-backed task management and agent operating system optimized for AI Agent consumption via the Model Context Protocol (MCP) and developer consumption via CLI/TUI. It stores tasks, OKRs, bugs, and decisions as individual Markdown files with strictly typed YAML frontmatter, backed by an ephemeral SQLite cache for token-efficient querying and JSONL streams for event history and agent telemetry.

The fundamental design tension backlogit resolves is the conflict between human-centric tools and machine-centric requirements. Humans need readable, Git-friendly text files. AI agents need token-efficient, structured queries. backlogit bridges this gap through a Hybrid Data Architecture (CQRS): Markdown files are the source of truth, SQLite provides instant relational querying, and JSONL captures append-only event history.

## Technology Stack

| Layer         | Technology                             | Notes                                                |
|---------------|----------------------------------------|------------------------------------------------------|
| Language      | Go 1.22+                              | Statically typed, single binary, goroutine-native    |
| MCP Protocol  | mcp-go SDK                            | JSON-RPC 2.0 over stdio                             |
| Database      | SQLite 3 (ephemeral cache)            | FTS5 for full-text search, WAL mode, gitignored      |
| File Storage  | Markdown + YAML frontmatter           | Git-friendly source of truth for all artifacts       |
| Event Stream  | JSONL (append-only)                   | events.jsonl, telemetry.jsonl                        |
| Configuration | YAML                                  | config.yaml, registry.yaml, hooks.yaml               |
| CLI           | Cobra                                 | `backlogit` command with subcommands                 |
| TUI           | Bubble Tea (future)                   | Terminal UI for interactive board management         |
| Testing       | go test, testify                      | TDD required; contract, integration, unit tests      |
| Linting       | golangci-lint                         | Comprehensive static analysis and formatting         |
| Packaging     | go.mod, go build                      | Single static binary distribution                    |

## Project Structure

```text
cmd/
  backlogit/
    main.go                 # CLI entrypoint
internal/
  cli/
    root.go                 # Cobra root command
    init.go                 # `backlogit init` workspace scaffolding
    create.go               # `backlogit create` artifact creation
    sync.go                 # `backlogit sync` rehydration
    query.go                # `backlogit query` SQL queries
    mcp.go                  # `backlogit mcp` server start
  config/
    loader.go               # Load and validate config.yaml, registry.yaml, hooks.yaml
    schema.go               # Config struct definitions with validation tags
    defaults.go             # Default config.yaml and registry.yaml templates
  core/
    artifacts.go            # Artifact type definitions and hierarchy enforcement
    fields.go               # Custom field handling, validation, and external_map
    naming.go               # Name format templates ({prefix}{NNN}-{title_slug})
    routing.go              # File routing based on registry.yaml state mappings
  db/
    connection.go           # SQLite connection management, WAL mode, pragma tuning
    schema.go               # CREATE TABLE statements, FTS5 indexes, trigger definitions
    queries.go              # Parameterized read-only query execution
    rehydration.go          # Auto-rehydration engine: scan → parse → rebuild index.db
    gate.go                 # Read-only SQL query gate
  errors/
    errors.go               # Sentinel and typed error definitions
  events/
    stream.go               # JSONL append-only event writer (state changes, comments)
    telemetry.go            # Agent telemetry logging (token usage, execution traces)
    reader.go               # Efficient tail-read for recent events by item_id
  mcp/
    server.go               # MCP stdio server setup and lifecycle
    tools.go                # MCP tool definitions and dispatch
    resources.go            # MCP resource definitions (sprint context, item detail)
  models/
    artifact.go             # Artifact model: frontmatter + body
    frontmatter.go          # YAML frontmatter parser and serializer
    sprint.go               # Sprint container model with goal and date fields
  parser/
    legacy.go               # Legacy backlog.md AST parser (section/checklist heuristics)
    markdown.go             # Markdown file parser with frontmatter extraction
    migration.go            # Transformation pipeline: legacy → atomic files
go.mod
go.sum
Makefile
```

## Commands

```bash
go test ./...                          # Run all tests
go test ./internal/...                 # Run internal tests only
go test -cover ./...                   # Run with coverage report
golangci-lint run                      # Lint and static analysis
gofmt -l .                             # Format check
go vet ./...                           # Vet for suspicious constructs
go build ./cmd/backlogit               # Build binary
go install ./cmd/backlogit             # Install to GOPATH/bin
backlogit init                         # Initialize .backlogit/ workspace
backlogit create --type task --title "My task"  # Create artifact
backlogit sync                         # Force rehydration of index.db
backlogit mcp                          # Start MCP stdio server
```

## Hybrid Data Architecture (CQRS)

backlogit separates reads from writes using three storage mechanisms to balance Git compatibility with agent token efficiency.

### Source of Truth: Markdown & YAML Frontmatter

Individual `.md` files in `.backlogit/` contain only current state via YAML frontmatter and the current description body. Historical comments, state change logs, and agent thought processes are forbidden from these files. When an agent reads a task, it consumes a few hundred tokens rather than parsing a massive history log.

### Query Engine: SQLite (index.db) & Auto-Rehydration

The `.backlogit/index.db` file is an ephemeral cache managed entirely by the backlogit core. It is gitignored and disposable. If deleted, after a `git pull` that changes ticket files, or after manual file edits, the rehydration engine walks the directory tree, parses all YAML frontmatter, and rebuilds the relational graph in milliseconds. Agents execute targeted queries like `backlogit_query_sql("SELECT id, title FROM items WHERE type='bug' AND status='in_progress'")` instead of dumping thousands of lines into the context window. FTS5 provides full-text search across the workspace.

### Event Stream: JSONL

When a status changes or an agent adds a comment, a single JSON object is appended to `events.jsonl`. JSONL eliminates corruption risk during concurrent writes and supports efficient tail-reading for recent activity. `telemetry.jsonl` captures agent execution metrics separately.

## Code Style and Conventions

### Type Safety

* Go provides built-in static typing; all types are checked at compile time
* Use Go structs with `go-playground/validator` tags for data crossing package boundaries
* Use `golangci-lint` with zero errors as a CI gate
* Use interfaces for abstraction; accept interfaces, return structs
* Prefer generics (Go 1.18+) over `any` when they reduce duplication

### Error Handling

* Define sentinel errors and typed errors in `internal/errors/errors.go`
* Sentinel errors: `ErrConfig`, `ErrValidation`, `ErrQuery`, `ErrRehydration`, `ErrMigration`, `ErrMCP`
* No `panic()` in library code; all errors returned via `(result, error)` tuples
* Wrap errors with context: `fmt.Errorf("operation context: %w", err)`
* MCP tool errors return structured JSON-RPC error responses with descriptive messages
* Use `log/slog` for structured error reporting, never `fmt.Println()`

### Naming

* Packages: `lowercase` (e.g., `rehydration`, `legacy`)
* Types and exported functions: `PascalCase` (e.g., `ArtifactType`, `SprintGoal`)
* Unexported functions and variables: `camelCase` (e.g., `parseFrontmatter`, `itemID`)
* Constants: `PascalCase` for exported, `camelCase` for unexported
* Artifact IDs: prefixed strings matching config (e.g., `T105`, `US042`, `BUG019`, `ADR005`)
* Status values: `snake_case` (e.g., `todo`, `in_progress`, `done`, `blocked`, `review`)

### Documentation

* All exported functions, types, and packages require GoDoc comments
* Package-level comments in `doc.go` describe the package's purpose and architectural role
* Use complete sentences starting with the name of the thing being documented

### Database

* One `*sql.DB` connection pool per workspace, opened in WAL mode for concurrent reads
* Schema bootstrapped on every connection via `EnsureSchema()`
* All queries go through parameterized functions in `internal/db/queries.go`, never raw SQL in tool handlers
* Read-only query gate: reject INSERT, UPDATE, DELETE, DROP via the `backlogit_query_sql` MCP tool
* FTS5 virtual table for full-text search across artifact titles and descriptions

### MCP Tool Pattern

Every MCP tool follows this pattern:

1. Validate workspace is initialized (return descriptive error if `.backlogit/` missing)
2. Parse parameters from the MCP request into a validated Go struct
3. Acquire database connection from the connection pool
4. Execute business logic through `internal/core/` and `internal/db/` packages
5. Return structured JSON response or return an MCP error with descriptive message

### Testing

* TDD required: write tests first, verify they fail, then implement
* Three test tiers:
  * Unit tests: colocated `_test.go` files in each package
  * Contract tests: `tests/contract/` for MCP tool input/output schema validation
  * Integration tests: `tests/integration/` for end-to-end flows with temp workspaces and real SQLite
* Test fixtures create temporary `.backlogit/` directories via `t.TempDir()`
* Use table-driven tests for comprehensive edge case coverage
* Target 90%+ code coverage for core packages

## MCP Tools Registry

| Tool                         | Package | Purpose                                                  |
|------------------------------|---------|----------------------------------------------------------|
| `backlogit_create_item`      | mcp     | Create a new artifact (task, bug, story, etc.)           |
| `backlogit_update_item`      | mcp     | Update artifact fields (status, priority, parent, etc.)  |
| `backlogit_get_item`         | mcp     | Retrieve the current Markdown state of an artifact       |
| `backlogit_get_item_history` | mcp     | Tail events.jsonl for recent activity on an item         |
| `backlogit_query_sql`        | mcp     | Execute read-only parameterized SQL against index.db     |
| `backlogit_sync_index`       | mcp     | Force rehydration: rescan Markdown, rebuild SQLite cache |
| `backlogit_append_comment`   | mcp     | Append a comment event to events.jsonl                   |
| `backlogit_log_telemetry`    | mcp     | Write agent telemetry to telemetry.jsonl                 |
| `backlogit_save_memory`      | mcp     | Update agent semantic memory in memories.json            |
| `backlogit_create_checkpoint`| mcp     | Save agent session state snapshot                        |

## Configuration

backlogit uses three YAML configuration files in `.backlogit/`:

| File             | Purpose                                                              |
|------------------|----------------------------------------------------------------------|
| `config.yaml`    | Artifact types, hierarchy, naming templates, custom fields           |
| `registry.yaml`  | Maps artifact states and types to specific directory paths           |
| `hooks.yaml`     | External integration configuration (Jira, Azure DevOps sync)        |

### Environment Variables

| Env Var                  | Default         | Description                              |
|--------------------------|-----------------|------------------------------------------|
| `BACKLOGIT_WORKSPACE`    | `.backlogit/`   | Workspace root directory                 |
| `BACKLOGIT_LOG_LEVEL`    | `INFO`          | Logging verbosity                        |
| `BACKLOGIT_LOG_FORMAT`   | `json`          | `json` or `text`                         |

## Session Memory Requirements

* All working agent sessions MUST persist their output to `.copilot-tracking/memory/` using the `memory` agent before the session ends.
* When the context window reaches approximately 65% capacity, invoke the `memory` agent to checkpoint current work before continuing.
* For long sessions, save memory checkpoints after completing each phase or major task group.
* Every memory entry must include task IDs completed, files modified, decisions and rationale, failed approaches, and concrete next steps.
* File convention: `.copilot-tracking/memory/{YYYY-MM-DD}/{descriptive-slug}-memory.md`.
