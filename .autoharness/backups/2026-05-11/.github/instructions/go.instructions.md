---
description: 'Go programming language coding conventions and best practices for the backlogit codebase'
applyTo: '**/*.go'
---

# Go Coding Conventions and Best Practices

Follow idiomatic Go 1.22+ practices and community standards when writing code in the backlogit codebase. These conventions enforce type safety, consistent error handling, and maintainability across a hybrid data architecture that combines Markdown files, SQLite, and YAML configuration.

## General Instructions

* Target Go 1.22 or later; use modern syntax including range-over-func iterators and enhanced routing patterns where appropriate.
* Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) as baseline style references.
* Write GoDoc comments on all exported functions, types, constants, and package declarations.
* Break down complex functions into smaller, single-responsibility helpers.
* Handle errors explicitly using the project's sentinel error hierarchy; never discard errors silently.
* Write code that passes `golangci-lint run`, `go vet ./...`, and `gofmt -l .` with zero findings.
* Prefer the standard library over third-party packages when the standard library solution is adequate.
* Keep imports organized in three groups separated by blank lines: standard library, external dependencies, internal packages. Let `goimports` enforce ordering.

## Commands

```bash
go test ./...                          # Run all tests
go test -race ./...                    # Run all tests with race detection (preferred)
go test ./internal/...                 # Run internal package tests only
go test ./tests/contract/...           # Run MCP contract tests only
go test -race -coverprofile=coverage.out ./... # Run with race detection and coverage report
golangci-lint run                      # Lint (style, bugs, complexity)
gofmt -l .                             # Check formatting
goimports -l .                         # Check import ordering
go vet ./...                           # Static analysis
go build ./cmd/backlogit               # Build binary
go install ./cmd/backlogit             # Install to $GOPATH/bin
backlogit init                         # Initialize .backlogit/ workspace
backlogit mcp                          # Start MCP stdio server
```

## Patterns to Follow

### Data Modeling

* Use Go structs with JSON and YAML struct tags for data structures that cross package boundaries, enter or leave MCP tools, or require serialization (frontmatter schemas, API inputs, configuration).
* Use `go-playground/validator/v10` for struct validation at package boundaries; validate on ingress, trust within internal packages.
* Use `const` blocks with a string type alias for fixed sets of states, artifact types, and status values.
* Prefer value receivers on small structs and pointer receivers on large structs or when mutation is required.

### Schema Synchronization

Keep Go struct definitions and YAML frontmatter schemas aligned. **Schema mismatch** — where a
struct field name, type, or tag differs from the frontmatter key written to `.backlogit/` files —
is a recurring source of rehydration failures, silent data loss, and query errors.

Rules:

* When adding a field to a struct, add the corresponding `yaml:` tag and update the frontmatter
  template or migration in `internal/parser/`.
* When renaming a YAML frontmatter key, update the corresponding `yaml:` struct tag and write a
  migration that rewrites existing files (do not leave a silent gap between old files and new code).
* When removing a field from a struct, handle the case where old files still carry the key (use
  `yaml:",omitempty"` or a migration to strip it).
* Validate struct round-trips in unit tests: marshal a known struct to YAML, unmarshal back, and
  assert field equality.
* Use `yaml:",omitempty"` on optional fields to avoid writing zero-value keys to `.backlogit/`
  files that then trip up consumers expecting the key to be absent vs. present-but-empty.

```go
package models

import "github.com/go-playground/validator/v10"

// ArtifactStatus represents the lifecycle state of a backlogit artifact.
type ArtifactStatus string

const (
    StatusTodo       ArtifactStatus = "todo"
    StatusInProgress ArtifactStatus = "in_progress"
    StatusDone       ArtifactStatus = "done"
    StatusBlocked    ArtifactStatus = "blocked"
)

// Artifact holds the current state of a backlogit work item.
type Artifact struct {
    ID           string         `json:"id"            yaml:"id"            validate:"required"`
    Title        string         `json:"title"         yaml:"title"         validate:"required,max=200"`
    Status       ArtifactStatus `json:"status"        yaml:"status"        validate:"required,oneof=todo in_progress done blocked"`
    ArtifactType string         `json:"artifact_type" yaml:"artifact_type" validate:"required"`
}

// Validate checks all struct tags and returns a descriptive error on failure.
func (a Artifact) Validate() error {
    return validator.New().Struct(a)
}
```

### Path Handling

* Use `path/filepath` for all filesystem operations; never use raw string concatenation for paths.
* Resolve paths relative to the workspace root; reject any path that escapes the workspace via `..` traversal.

```go
package core

import (
    "fmt"
    "path/filepath"
)

// SafeResolve returns an absolute path within workspaceRoot or an error
// if the target escapes the workspace boundary.
func SafeResolve(workspaceRoot, target string) (string, error) {
    abs := filepath.Join(workspaceRoot, target)
    rel, err := filepath.Rel(workspaceRoot, abs)
    if err != nil {
        return "", fmt.Errorf("resolve path: %w", err)
    }
    if len(rel) >= 2 && rel[:2] == ".." {
        return "", fmt.Errorf("path traversal rejected: %s escapes workspace", target)
    }
    return abs, nil
}
```

### Structured Logging

* Use `log/slog` (Go 1.21+) for all structured logging; never use `fmt.Println` or `log.Printf`.
* Create package-scoped loggers with `slog.With` to attach persistent context fields.
* Log at appropriate levels: `Debug` for internal state, `Info` for lifecycle events, `Warn` for recoverable issues, `Error` for failures.

```go
package db

import (
    "fmt"
    "log/slog"
)

var logger = slog.With("package", "db")

// Rehydrate scans Markdown files and rebuilds the SQLite index.
func Rehydrate(workspacePath string) (int, error) {
    logger.Info("starting rehydration", "workspace", workspacePath)
    count, err := scanAndIndex(workspacePath)
    if err != nil {
        logger.Error("rehydration failed", "error", err)
        return 0, fmt.Errorf("rehydrate: %w", err)
    }
    logger.Info("rehydration complete", "indexed", count)
    return count, nil
}
```

### Database Access

* Use `database/sql` with `modernc.org/sqlite` (CGo-free) as the SQLite driver.
* Maintain one `*sql.DB` per workspace opened in WAL mode with foreign keys enabled.
* Route all queries through `internal/db/queries.go` using parameterized statements; never inline raw SQL outside that package.
* Close connections explicitly using `defer db.Close()`.

```go
package db

import (
    "database/sql"
    "fmt"

    _ "modernc.org/sqlite"
)

// Open returns a configured SQLite connection in WAL mode.
func Open(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }
    pragmas := []string{
        "PRAGMA journal_mode=WAL",
        "PRAGMA foreign_keys=ON",
        "PRAGMA busy_timeout=5000",
    }
    for _, p := range pragmas {
        if _, err := db.Exec(p); err != nil {
            db.Close()
            return nil, fmt.Errorf("set pragma %q: %w", p, err)
        }
    }
    return db, nil
}
```

### Concurrency Patterns

* Use goroutines and channels for concurrent operations; prefer `errgroup.Group` from `golang.org/x/sync/errgroup` for structured fan-out with error propagation.
* Pass `context.Context` as the first parameter to any function that performs I/O or may block.
* Use `sync.WaitGroup` only when errors are not relevant; prefer `errgroup` otherwise.
* Never share mutable state between goroutines without explicit synchronization (`sync.Mutex`, channels, or atomics).

```go
package core

import (
    "context"
    "fmt"

    "golang.org/x/sync/errgroup"
)

// IndexFiles indexes multiple Markdown files concurrently.
func IndexFiles(ctx context.Context, paths []string) error {
    g, ctx := errgroup.WithContext(ctx)
    for _, p := range paths {
        g.Go(func() error {
            return indexSingleFile(ctx, p)
        })
    }
    if err := g.Wait(); err != nil {
        return fmt.Errorf("index files: %w", err)
    }
    return nil
}
```

### Configuration

* Load configuration from YAML files in `.backlogit/` using `gopkg.in/yaml.v3` with struct validation via `go-playground/validator/v10`.
* Support environment variable overrides with the `BACKLOGIT_` prefix using `os.LookupEnv`.
* Fail fast on invalid configuration with descriptive error messages.

```go
package config

import (
    "fmt"
    "os"

    "github.com/go-playground/validator/v10"
    "gopkg.in/yaml.v3"
)

// WorkspaceConfig holds the parsed config.yaml settings.
type WorkspaceConfig struct {
    ArtifactTypes []string          `yaml:"artifact_types" validate:"required,min=1,dive,required"`
    IDPrefixMap   map[string]string `yaml:"id_prefix_map"  validate:"required"`
    MaxSlugLength int               `yaml:"max_slug_length" validate:"gte=10,lte=200"`
}

// Load reads config.yaml from the workspace, applies env var overrides,
// and validates the result.
func Load(workspacePath string) (*WorkspaceConfig, error) {
    data, err := os.ReadFile(workspacePath + "/config.yaml")
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }
    var cfg WorkspaceConfig
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    applyEnvOverrides(&cfg)
    if err := validator.New().Struct(cfg); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }
    return &cfg, nil
}
```

## Patterns to Avoid

* **Bare `panic()`**: Never call `panic()` in library code (`internal/`). Reserve panics for truly unrecoverable states in `main()` initialization only.
* **`any` without justification**: Do not use `any` (or `interface{}`) when a specific type or generic constraint suffices. Document the reason when `any` is unavoidable.
* **Global mutable state**: Do not use package-level mutable variables for shared state. Pass dependencies explicitly through function parameters or struct fields.
* **`fmt.Println` for logging**: Use `log/slog`. `fmt.Println` bypasses log levels, formatting, and output routing.
* **`os.Exit` outside `main()`**: Return errors to callers; only `main()` decides the exit code.
* **Discarded errors**: Never assign errors to `_` without a comment explaining why the error is intentionally ignored.
* **`init()` for complex logic**: Use explicit initialization functions called from `main()`. Reserve `init()` for trivial registration only (driver imports, flag defaults).
* **Naked goroutines**: Never launch a goroutine without cancellation support via `context.Context` or lifecycle management via `errgroup` / `sync.WaitGroup`.

```go
// WRONG: discarded error
_ = db.Close()

// CORRECT: log or propagate
if err := db.Close(); err != nil {
    logger.Warn("failed to close database", "error", err)
}
```

## Code Style and Formatting

* Use `gofmt` as the canonical formatter; all Go files must be `gofmt`-clean.
* Use `goimports` to organize imports into standard library, external, and internal groups.
* Write GoDoc comments on all exported symbols following the GoDoc conventions: start with the symbol name, use complete sentences.
* Use `fmt.Sprintf` for string formatting; avoid manual string concatenation for complex expressions.
* Place package-level documentation in a `doc.go` file when the package comment exceeds a few lines.

```go
// ParseFrontmatter extracts YAML frontmatter from a Markdown file.
// It returns the parsed key-value pairs and the remaining body text.
// An error is returned if the frontmatter contains invalid YAML.
func ParseFrontmatter(content string) (map[string]string, string, error) {
    parts := strings.SplitN(content, "---", 3)
    if len(parts) < 3 {
        return nil, content, nil
    }
    var frontmatter map[string]string
    if err := yaml.Unmarshal([]byte(parts[1]), &frontmatter); err != nil {
        return nil, "", fmt.Errorf("parse frontmatter: %w", err)
    }
    return frontmatter, strings.TrimSpace(parts[2]), nil
}
```

## Error Handling

* Define sentinel errors and structured error types in `internal/errors/errors.go`.
* Use `errors.Is` and `errors.As` for programmatic error inspection.
* Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the chain.
* Provide actionable error messages that help the caller understand what went wrong and how to resolve it.
* Never discard errors without explicit justification in a comment.

### Error Type Selection

Choose the correct error type based on caller needs:

| Situation | Error Type | Example |
|---|---|---|
| Package-level condition callers check with `errors.Is` | Sentinel (`errors.New`) | `ErrNotFound`, `ErrConfig` |
| Caller needs structured data to recover or branch | Typed struct + `errors.As` | `*ConfigError{Field, Err}` |
| Adding context to a propagated error | Wrapped (`fmt.Errorf("%w")`) | `fmt.Errorf("open db: %w", err)` |
| Always-fatal initialization failure | Sentinel in `errors.go` | `ErrRehydration` |

**Incorrect error type** is the most common mistake: using a bare sentinel when the caller
needs structured data, or using a typed struct when a simple sentinel would suffice. Match
the error type to what callers actually need to inspect.

```go
package errors

import "errors"

// Sentinel errors for the backlogit error hierarchy.
var (
    ErrConfig      = errors.New("backlogit: configuration error")
    ErrQuery       = errors.New("backlogit: query error")
    ErrValidation  = errors.New("backlogit: validation error")
    ErrRehydration = errors.New("backlogit: rehydration error")
    ErrMCP         = errors.New("backlogit: mcp error")
)

// ConfigError wraps a configuration failure with additional context.
type ConfigError struct {
    Field   string
    Message string
    Err     error
}

func (e *ConfigError) Error() string {
    return "config: " + e.Field + ": " + e.Message
}

func (e *ConfigError) Unwrap() error {
    return e.Err
}

// Is reports whether the target matches ErrConfig.
func (e *ConfigError) Is(target error) bool {
    return target == ErrConfig
}
```

## Type Safety

* Go provides static typing at compile time; leverage the compiler as the primary safety gate.
* Define interfaces for abstraction at the consumer site, not the provider. Keep interfaces small (one to three methods).
* Use generics (Go 1.18+) for type-safe collections, result wrappers, and utility functions where appropriate.
* Define named types for complex expressions that appear in multiple places.
* Avoid `any` without a documented justification; prefer specific types or constrained generics.

```go
package db

import "context"

// ItemRow represents a single row from the items table.
type ItemRow struct {
    ID     string
    Title  string
    Status string
    Type   string
}

// QueryExecutor abstracts database query execution for testability.
type QueryExecutor interface {
    Query(ctx context.Context, sql string, args ...any) ([]ItemRow, error)
}
```

## Testing and Documentation

* Use the `testing` package with `stretchr/testify` for assertions and `require` for fatal checks.
* Use `t.TempDir()` for temporary workspace directories; never create persistent test artifacts.
* Write table-driven tests to cover edge cases and reduce duplication.
* Organize tests into colocated unit tests (`internal/*/`), integration tests (`tests/integration/`), and contract tests (`tests/contract/`).
* Follow TDD: write a failing test first, then implement the minimum code to pass it.
* Target 90% or higher line coverage; critical paths (MCP tools, rehydration, query execution) require 100%.
* Write contract tests that validate MCP tool input/output schemas and error codes.

```go
package core_test

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/softwaresalt/backlogit/internal/core"
)

func setupWorkspace(t *testing.T) string {
    t.Helper()
    ws := filepath.Join(t.TempDir(), ".backlogit")
    require.NoError(t, os.MkdirAll(ws, 0o755))
    require.NoError(t, os.WriteFile(
        filepath.Join(ws, "config.yaml"),
        []byte("artifact_types: [task, bug]"),
        0o644,
    ))
    return ws
}

func TestCreateArtifact(t *testing.T) {
    tests := []struct {
        name         string
        title        string
        artifactType string
        wantErr      string
    }{
        {
            name:         "creates markdown file for valid bug",
            title:        "Fix login",
            artifactType: "bug",
        },
        {
            name:         "rejects empty type",
            title:        "Test",
            artifactType: "",
            wantErr:      "unknown artifact type",
        },
        {
            name:         "rejects invalid type",
            title:        "Test",
            artifactType: "invalid",
            wantErr:      "unknown artifact type",
        },
        {
            name:         "rejects uppercase type",
            title:        "Test",
            artifactType: "TASK",
            wantErr:      "unknown artifact type",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ws := setupWorkspace(t)
            artifact, err := core.CreateArtifact(ws, tt.title, tt.artifactType)

            if tt.wantErr != "" {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.wantErr)
                return
            }

            require.NoError(t, err)
            mdPath := filepath.Join(ws, "bugs", artifact.ID+".md")
            assert.FileExists(t, mdPath)
        })
    }
}
```

## Project Organization

* Use the standard Go project layout: `cmd/` for executables, `internal/` for private packages.
* Each sub-package (`cli/`, `core/`, `db/`, `mcp/`, `models/`, `parser/`, `config/`, `events/`, `errors/`) exposes its public API via exported symbols.
* Keep `cmd/backlogit/main.go` minimal; delegate to `internal/cli/` for command wiring.
* Colocate unit tests with source files (`*_test.go`). Integration and contract tests live under `tests/`.
* Configuration schemas live in `internal/config/` as validated structs.

```text
cmd/
  backlogit/
    main.go               # CLI entrypoint: flag parsing, command dispatch
internal/
  cli/
    commands.go            # Cobra command definitions
    init.go                # backlogit init workspace scaffolding
  config/
    loader.go              # Load and validate config.yaml
    schema.go              # Struct definitions with validation tags
    defaults.go            # Default config.yaml and registry.yaml templates
  core/
    artifacts.go           # Artifact creation, hierarchy enforcement
    fields.go              # Custom field handling and validation
    naming.go              # Name format templates ({prefix}{NNN}-{title_slug})
    routing.go             # File routing based on registry.yaml state mappings
  db/
    connection.go          # SQLite connection management, WAL mode, pragmas
    schema.go              # CREATE TABLE statements, FTS5 indexes, triggers
    queries.go             # Parameterized read-only query execution
    rehydration.go         # Auto-rehydration engine: scan, parse, rebuild
  errors/
    errors.go              # Sentinel errors and structured error types
  events/
    stream.go              # JSONL append-only event writer
    telemetry.go           # Agent telemetry logging
    reader.go              # Efficient tail-read for recent events
  mcp/
    server.go              # MCP stdio server setup and lifecycle
    tools.go               # MCP tool definitions and dispatch
    resources.go           # MCP resource definitions
    gate.go                # Read-only SQL query gate
  models/
    artifact.go            # Artifact struct with frontmatter tags
    frontmatter.go         # YAML frontmatter parser and serializer
    sprint.go              # Sprint container model
  parser/
    legacy.go              # Legacy backlog.md parser (section/checklist heuristics)
    markdown.go            # Markdown file parser with frontmatter extraction
    migration.go           # Transformation pipeline: legacy to atomic files
tests/
  integration/             # Cross-package integration tests
  contract/                # MCP tool contract tests (schema validation)
go.mod
go.sum
Makefile
```

## API Design Guidelines

* Use structs for all data entering or leaving package boundaries; avoid returning raw `map[string]any`.
* Use the functional options pattern for constructors with more than two configuration parameters.
* Accept `context.Context` as the first parameter for any function that performs I/O.
* Return `(result, error)` tuples; never use output parameters.
* Define consumer-side interfaces for testability; keep them narrow (one to three methods).

```go
// Option configures a QueryRequest.
type Option func(*QueryRequest)

// WithStatus filters results by artifact status.
func WithStatus(status ArtifactStatus) Option {
    return func(q *QueryRequest) {
        q.Status = &status
    }
}

// WithLimit sets the maximum number of results.
func WithLimit(n int) Option {
    return func(q *QueryRequest) {
        q.Limit = n
    }
}

// QueryItems retrieves artifacts matching the provided options.
func QueryItems(ctx context.Context, db *sql.DB, opts ...Option) ([]Artifact, error) {
    req := QueryRequest{Limit: 50}
    for _, opt := range opts {
        opt(&req)
    }
    return executeQuery(ctx, db, req)
}
```

## Quality Checklist

Before submitting Go code for review, verify:

### Core Requirements

* [ ] All exported symbols have GoDoc comments starting with the symbol name
* [ ] `golangci-lint run` passes with zero findings
* [ ] `go vet ./...` passes with zero findings
* [ ] `gofmt -l .` reports no unformatted files
* [ ] Error handling follows sentinel/wrapping pattern; no discarded errors

### Safety and Quality

* [ ] No `panic()` in library code (`internal/`)
* [ ] No `any` without documented justification
* [ ] No global mutable state
* [ ] `log/slog` used instead of `fmt.Println` or `log.Printf`
* [ ] Parameterized queries for all database access
* [ ] `path/filepath` used for all filesystem operations

### Testing

* [ ] Tests written before implementation (TDD)
* [ ] `t.TempDir()` used for filesystem tests
* [ ] Table-driven tests cover edge cases
* [ ] Contract tests validate MCP tool schemas
* [ ] `go test -race -cover ./...` reports 90%+ coverage
