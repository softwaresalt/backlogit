---
applyTo: "**"
---

# Backlogit Constitution

## Core Principles

### I. Type-Safe Go

All production code MUST be written in Go 1.22+ with complete
GoDoc comments on every exported function, type, and package.
`golangci-lint` MUST pass with zero errors. Go structs with
`go-playground/validator` tags are the canonical choice for all
data structures crossing package boundaries. Custom validation
functions MUST enforce the artifact hierarchy and naming
constraints defined in `config.yaml`. `panic()` is forbidden in
library code; all error handling MUST use the sentinel and typed
error patterns defined in `internal/errors/`.

**Rationale**: backlogit manages user project state through file
writes, database operations, and MCP tool calls. Go's built-in
static type system catches structural errors at compile time
rather than corrupting workspace files at runtime. Struct
validation at boundaries provides runtime safety for dynamic
inputs. Strong typing also improves agent comprehension of the
codebase.

### II. MCP Protocol Fidelity

The server MUST implement the Model Context Protocol via the Go
`mcp-go` SDK (JSON-RPC 2.0 over stdio). All MCP tools MUST be
unconditionally visible to every connected agent regardless of
workspace state. Tools called before workspace initialization MUST
return a descriptive error rather than being hidden. Tool parameter
schemas MUST use Go structs with JSON tags for automatic validation
and JSON Schema generation.

**Rationale**: Consistent tool surface ensures agents can discover
capabilities without conditional logic. Protocol compliance
guarantees interoperability with any MCP-compatible client (Claude
Code, GitHub Copilot CLI, Cursor, VS Code).

### III. Test-First Development (NON-NEGOTIABLE)

Every feature MUST have tests written before implementation code.
The test structure (colocated `_test.go` unit tests,
`tests/integration/`, `tests/contract/`) MUST be maintained.
Contract tests validate MCP tool input/output schemas and error
responses. Integration tests validate cross-module interactions
(rehydration, file routing, event streaming). Unit tests validate
isolated logic. All tests MUST pass via `go test ./...` before
any code is merged.

**Rationale**: backlogit manages the user's agile system of record.
Regressions in artifact creation, rehydration cycles, SQL query
gates, or file routing can silently corrupt the workspace or lose
data. Test-first discipline catches failures before they reach
production.

### IV. Workspace Containment & Security Boundaries

All file-system operations MUST resolve within the `.backlogit/`
directory at the workspace root. Path traversal attempts MUST be
rejected via canonical path validation. The SQLite `index.db` is
ephemeral and gitignored; it MUST NOT contain data that does not
exist in the Markdown source files. The `backlogit_query_sql` MCP
tool MUST enforce a read-only gate: only `SELECT` statements are
permitted. No secrets or credentials MUST appear in `.backlogit/`
files (which may be committed to Git).

**Rationale**: backlogit operates on files that travel with the
codebase in Git. Without strict containment, a misbehaving agent
could write outside the workspace, execute destructive SQL, or
leak sensitive information through serialized state.

### V. Structured Observability

All significant operations MUST emit structured log entries via
Go's `log/slog` package. Log coverage MUST include: MCP tool
call execution, workspace lifecycle events (init, sync, rehydrate),
database operations, file routing decisions, and event stream
writes. Log output MUST support both human-readable text and JSON
formats via `slog.Handler` configuration. Agent telemetry MUST
be captured in `telemetry.jsonl` for operational visibility.

**Rationale**: backlogit runs as a background MCP server during
extended coding sessions. When something goes wrong during
unattended operation, structured logs and telemetry streams are
the primary diagnostic tools. Without them, debugging rehydration
failures, routing errors, or query gate violations would require
reproducing the exact scenario.

### VI. Single-Binary Simplicity

The project MUST be installable via `go install
github.com/backlogit/backlogit/cmd/backlogit@latest` as a single
static binary. Dependencies MUST be managed via `go.mod` with
minimum version selection. New dependencies MUST be justified by
a concrete requirement; do not add modules speculatively. Prefer
the standard library over external packages when the standard
library solution is adequate. SQLite (via `modernc.org/sqlite`,
a CGo-free pure-Go implementation) is the sole query engine; do
not introduce additional databases or caches.

**Rationale**: Operational simplicity is critical for a tool that
developers install alongside their projects. Go's single static
binary model eliminates runtime dependencies, version conflicts,
and environment setup. Deployment is a single `go install` or
binary download.

### VII. CQRS Data Architecture (NON-NEGOTIABLE)

backlogit MUST maintain strict separation between its three storage
layers:

1. **Markdown files** (source of truth): Individual `.md` files
   with YAML frontmatter store current state. These files MUST
   contain only the current state and description; no history,
   comments, or agent traces.
2. **SQLite cache** (query engine): The `index.db` is ephemeral,
   gitignored, and disposable. The rehydration engine MUST be able
   to rebuild it entirely from the Markdown files at any time.
3. **JSONL streams** (event history): `events.jsonl` captures
   state changes and comments. `telemetry.jsonl` captures agent
   metrics. Both are append-only.

**Rationale**: This architecture allows humans to work with readable
Markdown files in Git while agents query the SQLite cache for
token-efficient lookups. The JSONL streams provide audit history
without polluting the Markdown source of truth. The ephemeral
cache ensures no data loss if `index.db` is deleted or corrupted.

### VIII. Git-Friendly Persistence

All workspace state in `.backlogit/` MUST be serializable to
human-readable, Git-mergeable files. Markdown with YAML frontmatter
is the canonical format for artifacts. No binary files in
`.backlogit/` (except the gitignored `index.db`). File formats MUST
minimize merge conflicts (sorted YAML keys, stable field ordering
in frontmatter, deterministic slug generation). File writes MUST
use atomic temp-file-then-rename to prevent corruption.

**Rationale**: Workspace state travels with the codebase in Git.
Human-readable files enable code review of agent-managed state,
conflict resolution during merges, and manual editing when needed.
Atomic writes prevent half-written state from corrupting the
workspace during crashes or concurrent access.

### IX. Agent Context Efficiency

MCP tools MUST return minimal, targeted data to preserve agent
context windows. The `backlogit_query_sql` tool exists specifically
so agents can execute surgical SQL queries against `index.db`
rather than scanning directories of Markdown files. Tool responses
MUST be structured JSON, not raw file content. When an agent needs
a list of tasks, it queries `SELECT id, title, status FROM items
WHERE type='bug'` (50 tokens) rather than reading 50 Markdown
files (50,000 tokens).

**Rationale**: AI agents operate within finite context windows.
Every token consumed by raw file content is a token unavailable
for reasoning and code generation. The CQRS architecture exists
precisely to serve token-efficient query results to agents.

## Technical Constraints

- **Language**: Go 1.22+, statically typed
- **Concurrency**: goroutines, channels, `sync` package,
  `context.Context`
- **MCP SDK**: `github.com/mark3labs/mcp-go` (JSON-RPC 2.0 over
  stdio)
- **Database**: SQLite 3 via `modernc.org/sqlite` (CGo-free),
  WAL mode, FTS5 for full-text search
- **Configuration**: YAML via `gopkg.in/yaml.v3`
- **Data Validation**: `github.com/go-playground/validator/v10`
  for struct validation
- **Markdown Parsing**: `github.com/adrg/frontmatter` for YAML
  frontmatter extraction
- **CLI**: `github.com/spf13/cobra` for command-line interface
- **Linting**: `golangci-lint` (format + lint + vet)
- **Testing**: `testing` package, `github.com/stretchr/testify`,
  `go test -cover`
- **Build verification**: `go test ./...` and `golangci-lint run`
  and `go vet ./...` MUST pass before merge
- **License**: MIT

## Development Workflow

1. **Research before code**: Feature explorations and architecture
   decisions MUST be documented in `.backlog/research/` before
   implementation begins.
2. **Plan before code**: Implementation plans MUST be generated via
   the planning workflow and stored in `.backlog/plans/`.
3. **Branch per feature**: Each feature MUST be developed on a
   dedicated branch with a descriptive name.
4. **Contract-first design**: MCP tool schemas and Go structs
   MUST be defined before implementation. Changes to tool schemas
   require updating corresponding contract tests.
5. **Commit discipline**: Each commit MUST represent a coherent,
   passing-tests change. Commit messages MUST follow conventional
   commits format (e.g., `feat:`, `fix:`, `docs:`, `test:`).
6. **No dead code**: Placeholder functions (e.g., `panic("not
   implemented")` stubs) MUST be replaced with real
   implementations or removed before a feature is considered
   complete.

## Governance

This constitution supersedes all other development practices for
the backlogit project. All code reviews and automated checks MUST
verify compliance with these principles.

- **Amendments**: Any change to this constitution MUST be documented
  with a version bump, rationale, and sync impact report. Principle
  removals or redefinitions require a MAJOR version bump. New
  principles or material expansions require MINOR. Clarifications
  and wording fixes require PATCH.
- **Compliance review**: Every implementation plan MUST include a
  "Constitution Check" section that maps the proposed work against
  these principles and documents any justified violations.
- **Conflict resolution**: When a principle conflicts with a
  practical implementation need, the conflict MUST be documented
  with the specific principle violated, the justification, and the
  simpler alternative that was rejected.

**Version**: 2.0.0 | **Ratified**: 2026-03-29 | **Last Amended**: 2026-03-29
