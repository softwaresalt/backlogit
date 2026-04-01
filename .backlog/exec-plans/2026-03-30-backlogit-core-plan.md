---
title: "Backlogit Core Implementation Plan"
description: "Structured implementation plan for the backlogit file-backed task management system with MCP server, SQLite CQRS cache, and CLI"
source: ".backlog/research/Backlogit-Architecture-Design.md"
ms.date: 2026-03-30
---

## Problem Statement

backlogit requires a complete Go implementation of a file-backed task management and agent operating system. The architecture defines a hybrid data system (CQRS) combining Markdown files as the source of truth, an ephemeral SQLite cache for token-efficient agent queries, and JSONL streams for event history. A prototype exists in `.context/prototype.go` as a single-file proof of concept with basic config loading, task creation, and two MCP tools. No `go.mod`, tests, or proper package structure exist.

The system must expose its capabilities through two interfaces: a Cobra CLI for developers and an MCP stdio server (via `mcp-go` SDK) for AI agents. The architecture specifies 10+ MCP tools, a read-only SQL query gate, auto-rehydration from Markdown to SQLite, configurable artifact hierarchies, status-based file routing, and a legacy migration pipeline.

## Approach

Implement backlogit as a properly structured Go 1.22+ project following the constitution's mandated architecture. Build bottom-up from foundational packages (errors, config, models) through the data layer (parser, database, events) to the interface layer (MCP server, CLI). Each unit is self-contained with its own tests.

The prototype in `.context/prototype.go` serves as a reference for patterns but will not be copied directly. Instead, each concern will be extracted into its proper package following the project structure defined in AGENTS.md and the constitution.

Key architectural choices preserved from the source document:

- CQRS with three storage tiers (Markdown, SQLite, JSONL)
- Ephemeral SQLite cache rebuilt from Markdown via auto-rehydration
- Read-only SQL gate on the `backlogit_query_sql` MCP tool
- Configurable artifact type hierarchy with parent-child enforcement
- Status-based file routing via `registry.yaml` mappings
- Atomic file writes (temp-file-then-rename) to prevent corruption

## Implementation Units

### Unit 1: Project Foundation and Error Hierarchy

Scaffold the Go project structure, initialize `go.mod`, and establish the error system that all subsequent packages depend on.

**Files to create:**

- `go.mod` — Module declaration (`github.com/backlogit/backlogit`), Go 1.22+
- `cmd/backlogit/main.go` — Minimal CLI entrypoint (imports `internal/cli`)
- `internal/errors/errors.go` — Sentinel errors (`ErrConfig`, `ErrValidation`, `ErrQuery`, `ErrRehydration`, `ErrMigration`, `ErrMCP`) and typed error structs (`ConfigError`, `ValidationError`, `QueryError`)
- `internal/errors/errors_test.go` — Tests for error wrapping, `errors.Is`, `errors.As`
- `Makefile` — Build, test, lint, vet, format targets

**Acceptance criteria:**

- `go build ./cmd/backlogit` compiles successfully
- `go test ./internal/errors/...` passes with 100% coverage
- `go vet ./...` reports no issues
- Error types support `errors.Is` and `errors.As` correctly

### Unit 2: Configuration System

Load and validate `config.yaml`, `registry.yaml`, and `hooks.yaml` from the `.backlogit/` workspace using typed Go structs with `go-playground/validator` tags. Support environment variable overrides with the `BACKLOGIT_` prefix.

**Files to create:**

- `internal/config/schema.go` — Typed config structs: `WorkspaceConfig`, `ArtifactTypeConfig`, `FieldConfig`, `RegistryConfig`, `HooksConfig` with YAML tags and validation tags
- `internal/config/loader.go` — `Load(workspacePath)` function that reads YAML files, applies env var overrides, validates structs
- `internal/config/defaults.go` — Default `config.yaml` and `registry.yaml` template content for `backlogit init`
- `internal/config/loader_test.go` — Table-driven tests for valid configs, missing files, invalid YAML, validation failures, env var overrides

**Key structs from architecture:**

```go
ArtifactTypeConfig {
    Prefix          string
    NameFormat      string
    AllowedChildren []string
}

FieldConfig {
    Type        string   // "enum", "string", "int"
    Values      []string // for enums
    Default     string
    Optional    bool
    ExternalMap map[string]any
}
```

**Acceptance criteria:**

- Loads valid config.yaml with artifact types, hierarchy, and custom fields
- Validates `allowed_children` references exist in `artifact_types`
- Rejects invalid field types and missing required fields
- Supports `BACKLOGIT_WORKSPACE`, `BACKLOGIT_LOG_LEVEL`, `BACKLOGIT_LOG_FORMAT` env vars
- Returns wrapped `ErrConfig` on all failure paths

### Unit 3: Models and Frontmatter

Define the core data models for artifacts, frontmatter parsing/serialization, and sprint containers. These models are the data contracts used across all packages.

**Files to create:**

- `internal/models/artifact.go` — `Artifact` struct with JSON/YAML tags, validation, and status constants (`StatusTodo`, `StatusInProgress`, `StatusBlocked`, `StatusReview`, `StatusDone`)
- `internal/models/frontmatter.go` — `ParseFrontmatter(content) (map, body, error)` and `SerializeFrontmatter(fields, body) string` functions
- `internal/models/sprint.go` — `Sprint` struct with goal, start/end dates, and linked artifact IDs
- `internal/models/artifact_test.go` — Validation tests for all artifact fields
- `internal/models/frontmatter_test.go` — Round-trip parse/serialize tests, edge cases (no frontmatter, empty body, special characters)
- `internal/models/sprint_test.go` — Sprint model validation tests

**Acceptance criteria:**

- Frontmatter round-trips faithfully (parse then serialize produces identical output)
- Artifact validation enforces required fields (id, title, status, artifact_type)
- Status constants match the architecture's defined values
- Sprint model captures goal, dates, and artifact links

### Unit 4: Core Business Logic

Implement artifact creation with hierarchy enforcement, naming template resolution, custom field validation, and status-based file routing. This is the domain logic that both CLI and MCP layers invoke.

**Files to create:**

- `internal/core/artifacts.go` — `CreateArtifact(workspace, title, artifactType, opts...)` with hierarchy enforcement via `config.yaml` `allowed_children`, atomic file writes
- `internal/core/naming.go` — `ResolveName(config, artifactType, title, nextID)` implementing `{prefix}{NNN}-{title_slug}` templates with configurable slug length
- `internal/core/fields.go` — `ValidateFields(config, fields)` checking custom field types, enum values, required fields, and `external_map` translation
- `internal/core/routing.go` — `ResolveTargetDir(registry, artifactType, status)` mapping artifacts to directories based on `registry.yaml` state/type conditions
- `internal/core/artifacts_test.go` — Tests for creation, hierarchy rejection (e.g., task as parent of epic), valid parent-child chains
- `internal/core/naming_test.go` — Tests for name generation, slug truncation, sequential ID assignment
- `internal/core/fields_test.go` — Tests for field validation, enum enforcement, external_map translation
- `internal/core/routing_test.go` — Tests for directory routing with various status/type combinations

**Acceptance criteria:**

- Creating a task under a bug succeeds; creating an epic under a task fails with `ErrValidation`
- Name format `{prefix}{NNN}-{title_slug}` produces correct output (e.g., `T001-implement-jwt`)
- File routing places `todo` tasks in the `sprint-board/todo/` directory per registry config
- Atomic writes use temp-file-then-rename pattern
- Custom field validation rejects invalid enum values

### Unit 5: Database Layer and Rehydration

Implement the SQLite ephemeral cache with WAL mode, FTS5 full-text search, schema bootstrapping, parameterized queries, and the auto-rehydration engine that rebuilds the index from Markdown files.

**Files to create:**

- `internal/db/connection.go` — `Open(dbPath)` with WAL mode, foreign keys, busy timeout pragmas; connection pool management
- `internal/db/schema.go` — `EnsureSchema(db)` creating `items` table (id, title, status, type, parent_id, sprint, priority, created_at, updated_at), FTS5 virtual table for full-text search, indexes
- `internal/db/queries.go` — Parameterized query functions: `QueryItems(ctx, db, filters)`, `GetItem(ctx, db, id)`, `UpsertItem(ctx, db, artifact)`, `DeleteItem(ctx, db, id)`
- `internal/db/rehydration.go` — `Rehydrate(workspacePath, db)` that walks the `.backlogit/` directory tree, parses all Markdown frontmatter, and rebuilds the SQLite index; concurrent using `errgroup`
- `internal/db/gate.go` — `ValidateQuery(sql)` read-only gate rejecting INSERT/UPDATE/DELETE/DROP/ALTER/ATTACH statements; `ExecuteGatedQuery(db, sql, params)` with row limit
- `internal/db/connection_test.go` — WAL mode verification, pragma checks
- `internal/db/schema_test.go` — Schema creation, FTS5 table verification
- `internal/db/queries_test.go` — CRUD operations with test fixtures
- `internal/db/rehydration_test.go` — Full rehydration cycle with temp workspace containing sample Markdown files
- `internal/db/gate_test.go` — Gate allows SELECT, rejects INSERT/UPDATE/DELETE/DROP/ATTACH, pattern-based SQL injection detection

**Acceptance criteria:**

- SQLite opens in WAL mode with foreign keys enabled
- Schema bootstraps idempotently via `EnsureSchema()`
- Rehydration rebuilds the full index from Markdown files using concurrent goroutines
- FTS5 search returns results matching title and description content
- Read-only gate rejects all write operations with descriptive errors
- Gated queries enforce a configurable row limit (default 500)

### Unit 6: Event and Telemetry Streams

Implement the JSONL append-only event stream for state changes and comments, the telemetry stream for agent execution metrics, and efficient tail-reading for recent events by item ID.

**Files to create:**

- `internal/events/stream.go` — `AppendEvent(path, event)` writing JSON objects to `events.jsonl`; event schema: `{timestamp, actor, item_id, event_type, delta}`
- `internal/events/telemetry.go` — `LogTelemetry(path, entry)` writing to `telemetry.jsonl`; telemetry schema: `{timestamp, event_type, payload}`
- `internal/events/reader.go` — `TailEvents(path, itemID, limit)` reading recent events for a specific item by scanning from the end of the file
- `internal/events/stream_test.go` — Append events, verify JSONL format, concurrent write safety
- `internal/events/telemetry_test.go` — Telemetry logging, format validation
- `internal/events/reader_test.go` — Tail reading with filtering by item_id, limit enforcement

**Acceptance criteria:**

- Events append as single JSON lines without corrupting existing content
- Telemetry entries capture timestamp, event_type, and arbitrary payload
- Tail reader efficiently retrieves the last N events for a specific item_id
- Concurrent appends do not corrupt the JSONL file
- All event types from the architecture are supported (state_change, comment, checkpoint)

### Unit 7: MCP Server and Tool Handlers

Implement the MCP stdio server with all tool definitions, resource handlers, and prompt templates as defined in the architecture document. Wire tools to the core business logic, database, and event packages.

**Files to create:**

- `internal/mcp/server.go` — `CreateServer()` factory with tool/resource/prompt capabilities and `WithRecovery()`; `RunStdio()` lifecycle function
- `internal/mcp/tools.go` — Tool registration and dispatch for all 10 tools: `backlogit_create_item`, `backlogit_update_item`, `backlogit_get_item`, `backlogit_get_item_history`, `backlogit_query_sql`, `backlogit_sync_index`, `backlogit_append_comment`, `backlogit_log_telemetry`, `backlogit_save_memory`, `backlogit_create_checkpoint`
- `internal/mcp/resources.go` — Resource handlers for `backlogit://config` (YAML), `backlogit://schema` (SQLite schema)
- `internal/mcp/prompts.go` — Prompt templates for `create_sprint` and `triage_bug`
- `internal/mcp/errors.go` — MCP-specific error response helpers (`workspaceNotInitialized`, `validationFailed`, `internalError`)
- `tests/contract/tools_test.go` — Contract tests validating tool input/output schemas and error responses for all 10 tools
- `tests/contract/resources_test.go` — Contract tests for resource handlers

**MCP tools from architecture (Section 6):**

| Tool | Category | Write |
|------|----------|-------|
| `backlogit_create_item` | Agile Management | Yes |
| `backlogit_update_item` | Agile Management | Yes |
| `backlogit_get_item` | Agile Management | No |
| `backlogit_get_item_history` | Agile Management | No |
| `backlogit_query_sql` | Agile Management | No (gated) |
| `backlogit_sync_index` | System Admin | Yes |
| `backlogit_append_comment` | Agent Operations | Yes |
| `backlogit_log_telemetry` | Agent Operations | Yes |
| `backlogit_save_memory` | Agent Operations | Yes |
| `backlogit_create_checkpoint` | Agent Operations | Yes |

**Acceptance criteria:**

- All 10 tools are registered and discoverable by MCP clients
- Every tool follows the five-step handler pattern (validate workspace → parse params → get DB → execute → return JSON)
- Tools called before `backlogit init` return descriptive `workspace_not_initialized` errors
- `backlogit_query_sql` enforces the read-only gate from Unit 5
- Contract tests validate input/output schemas for all tools
- Resources return valid YAML and schema content

### Unit 8: CLI Commands

Implement the Cobra CLI with all commands: `init`, `create`, `sync`, `query`, and `mcp`. Wire each command to the core business logic and database packages.

**Files to create:**

- `internal/cli/root.go` — Cobra root command with `--cwd` persistent flag, version info, structured logging setup
- `internal/cli/init.go` — `backlogit init` with interactive scaffolding: methodology selection, default config/registry generation, directory creation, index.db initialization; `--legacy` flag for read-only adapter mode
- `internal/cli/create.go` — `backlogit create --type task --title "..." [--description "..."] [--parent ID] [--sprint ID]` with full artifact creation pipeline
- `internal/cli/sync.go` — `backlogit sync` triggering forced rehydration of index.db
- `internal/cli/query.go` — `backlogit query "SELECT ..."` executing gated SQL queries with formatted table output
- `internal/cli/mcp.go` — `backlogit mcp` starting the MCP stdio server; `backlogit mcp init [environment]` for auto-injecting MCP config into VS Code, Copilot, Cursor, Claude
- `internal/cli/root_test.go` — CLI command wiring tests
- `internal/cli/init_test.go` — Init with defaults, init with legacy flag, reinit protection
- `internal/cli/create_test.go` — Create with valid/invalid parameters
- `internal/cli/sync_test.go` — Sync triggers rehydration
- `internal/cli/query_test.go` — Query with valid/invalid SQL

**Acceptance criteria:**

- `backlogit init` creates `.backlogit/` with default `config.yaml`, `registry.yaml`, and directory structure
- `backlogit create --type bug --title "Login crash"` generates properly named Markdown file in the correct directory
- `backlogit sync` rebuilds index.db from Markdown files
- `backlogit query "SELECT id, title FROM items"` outputs formatted results
- `backlogit mcp` starts the MCP stdio server
- All commands respect `--cwd` flag and `BACKLOGIT_WORKSPACE` env var

### Unit 9: Legacy Migration Pipeline

Implement the legacy backlog.md AST parser and transformation pipeline for migrating from monolithic backlog files to atomic `.backlogit/` artifacts.

**Files to create:**

- `internal/parser/legacy.go` — `ParseLegacy(content)` AST parser using section conventions (headings → status), checklist conventions (`[ ]` → todo, `[x]` → done), and legacy YAML frontmatter extraction
- `internal/parser/markdown.go` — `ParseMarkdownFile(path)` extracting frontmatter and body from individual Markdown files (used by rehydration and read paths)
- `internal/parser/migration.go` — `Migrate(legacyPath, workspace, config)` transformation pipeline: extraction → decomposition → attribution → scaffolding → archiving (renames original to `.bak`)
- `internal/parser/legacy_test.go` — Tests with sample legacy backlog.md content: nested headings, mixed checklists, implicit status mapping
- `internal/parser/markdown_test.go` — Tests for frontmatter extraction, no-frontmatter files, malformed YAML
- `internal/parser/migration_test.go` — End-to-end migration test with temp workspace, verifying atomic file output and archive creation

**Legacy parsing heuristics from architecture (Section 10):**

- `# Backlog`, `## In Progress`, `### Done` → status field mapping by structural position
- `[ ] Task Name` → status `todo`
- `[x] Completed Task` → status `done`
- Markdown heading depth → parent-child hierarchy inference (H2 feature → H3 sub-task)

**Acceptance criteria:**

- Parses standard legacy backlog.md with nested headings and checklists
- Correctly infers status from heading names and checklist markers
- Infers hierarchy from heading depth (H2 parent → H3 child)
- Migration produces atomic Markdown files with valid YAML frontmatter
- Original file is archived as `.bak` with zero data loss
- `backlogit init --legacy` reads existing backlog.md without modifying it

### Unit 10: MCP Environment Registration

Implement `backlogit mcp init [environment]` to auto-inject the backlogit MCP server configuration into various IDE and agent configuration files.

**Files to create:**

- `internal/cli/mcp_init.go` — Environment detection and config injection for VS Code (`.vscode/mcp.json`), GitHub Copilot (`.copilot/mcp-config.json`), Cursor (`.cursor/mcp.json`), Claude Code (CLI command)
- `internal/cli/mcp_init_test.go` — Tests for each environment: config file discovery, safe JSON injection without overwriting existing settings, missing config directory handling

**Supported environments from architecture (Section 7.3):**

| Environment | Config Path | Method |
|-------------|-------------|--------|
| VS Code | `.vscode/mcp.json` | JSON injection |
| GitHub Copilot | `.copilot/mcp-config.json` | JSON injection |
| Cursor | `.cursor/mcp.json` | JSON injection |
| Claude Code | N/A | CLI execution |

**Acceptance criteria:**

- Detects and injects into existing config files without data loss
- Creates config directories if they do not exist
- Handles missing, empty, and malformed JSON config files gracefully
- Does not duplicate entries on repeated runs
- Supports `backlogit mcp init ghcp`, `vscode`, `cursor`, `claude` environment names

## Key Decisions

1. **CGo-free SQLite**: Use `modernc.org/sqlite` (pure Go) for zero CGo dependency, enabling true single-binary distribution across all platforms without C compiler requirements.

2. **Concurrent rehydration with errgroup**: The rehydration engine uses `golang.org/x/sync/errgroup` for structured fan-out when parsing Markdown files, with `context.Context` for cancellation. This matches the architecture's emphasis on Golang's goroutine concurrency.

3. **Read-only SQL gate via regex patterns**: The `backlogit_query_sql` gate uses compiled regex patterns to reject forbidden SQL operations. This is simpler and more maintainable than a full SQL parser while covering the security requirement.

4. **Atomic file writes**: All Markdown file creation and modification uses the temp-file-then-rename pattern to prevent corruption during concurrent agent access or unexpected process termination.

5. **Functional options for API design**: Core functions like `CreateArtifact` and `QueryItems` use the functional options pattern for extensible configuration without breaking API compatibility.

6. **JSONL over structured database for events**: Events use append-only JSONL as specified in the architecture, enabling concurrent writes without locking and efficient tail-reading. The SQLite index does not store event history.

7. **Deferred scope**: TUI (Bubble Tea), slash command integrations (Section 9), and external sync hooks (Jira, Azure DevOps) are excluded from this plan. They are listed as future work in the architecture document and do not affect the core system.

## Dependency Graph

```text
Unit 1 (Foundation + Errors)
  ├── Unit 2 (Configuration) ← depends on errors
  │   ├── Unit 3 (Models) ← depends on config for validation rules
  │   │   ├── Unit 4 (Core Logic) ← depends on models, config
  │   │   │   ├── Unit 5 (Database) ← depends on core, models
  │   │   │   │   ├── Unit 7 (MCP Server) ← depends on db, core, events
  │   │   │   │   └── Unit 8 (CLI) ← depends on db, core, mcp
  │   │   │   └── Unit 6 (Events) ← depends on models
  │   │   │       └── Unit 7 (MCP Server) ← depends on events
  │   │   └── Unit 9 (Legacy Migration) ← depends on models, core
  │   └── Unit 10 (MCP Env Registration) ← depends on config (standalone)
```

**Critical path**: 1 → 2 → 3 → 4 → 5 → 7 → 8

**Parallel tracks after Unit 3**:
- Track A: 4 → 5 → 7 → 8 (core data path)
- Track B: 6 (events, joins at Unit 7)
- Track C: 9 (legacy migration, independent after Unit 4)
- Track D: 10 (MCP env registration, independent after Unit 2)

## Constitution Check

| Principle | Compliance | Notes |
|-----------|-----------|-------|
| I. Type-Safe Go | ✅ | All structs use validator tags; golangci-lint required |
| II. MCP Protocol Fidelity | ✅ | All 10 tools unconditionally visible; descriptive errors before init |
| III. Test-First Development | ✅ | Every unit specifies test files; contract tests for MCP tools |
| IV. Workspace Containment | ✅ | Path traversal rejected via `SafeResolve`; read-only SQL gate |
| V. Structured Observability | ✅ | `log/slog` throughout; telemetry.jsonl for agent metrics |
| VI. Single-Binary Simplicity | ✅ | `modernc.org/sqlite` (CGo-free); single `go install` |
| VII. CQRS Data Architecture | ✅ | Markdown source of truth, SQLite cache, JSONL streams |
| VIII. Git-Friendly Persistence | ✅ | Atomic writes, sorted YAML keys, deterministic slugs |
| IX. Agent Context Efficiency | ✅ | Gated SQL queries return minimal JSON; no raw file dumps |

No principle violations identified.

## Success Criteria

1. `go build ./cmd/backlogit` produces a single static binary
2. `go test -cover ./...` reports 90%+ coverage on core packages
3. `golangci-lint run` passes with zero findings
4. `backlogit init` scaffolds a complete workspace from defaults
5. `backlogit create --type task --title "Test"` creates a properly formatted Markdown file with YAML frontmatter in the correct directory
6. `backlogit sync` rebuilds index.db from all Markdown files in the workspace
7. `backlogit query "SELECT id, title FROM items WHERE status='todo'"` returns formatted results from the SQLite cache
8. `backlogit mcp` starts an MCP stdio server that responds to all 10 tool calls
9. `backlogit mcp init ghcp` injects server configuration into `.copilot/mcp-config.json`
10. Legacy migration converts a monolithic backlog.md into atomic files without data loss
11. Concurrent agent writes to events.jsonl do not corrupt the file
12. Read-only SQL gate rejects all write operations
