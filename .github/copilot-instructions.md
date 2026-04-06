---
description: Shared backlogit development guidelines for custom agents.
maturity: stable
---

# Backlogit Development Guidelines

Last updated: 2026-04-06

backlogit is a highly configurable, file-backed task management and agent
operating system optimized for AI agent consumption through MCP and developer
consumption through CLI workflows. It stores tasks, bugs, decisions, and review
artifacts as Markdown with typed YAML frontmatter, backed by an ephemeral SQLite
cache for token-efficient querying and JSONL streams for history and telemetry.

The core design tension is simple: humans want readable, Git-friendly files,
while agents want surgical, structured queries. backlogit resolves that tension
through a CQRS architecture where Markdown is the source of truth, SQLite serves
reads, and JSONL preserves append-only history.

## Primary workflow

The repository now uses a two-agent path:

* `Groomer` owns `STASH -> BACKLOG`. It triages stash entries, routes
  deliberation and plan review, and harvests shipment-aware backlog.
* `Shipper` owns `SHIPMENT -> SHIPPED`. It claims shipments and drives harness,
  build, review, CI remediation, and pull request flow until the user approves
  merge.

Lifecycle summary: `STASH -> BACKLOG -> SHIPMENT -> SHIPPED`

Read `.github/agents/groomer.agent.md`, `.github/agents/shipper.agent.md`, and
`docs/workflow.md` for the durable workflow map.

## Technology Stack

| Layer | Technology | Notes |
|---|---|---|
| Language | Go 1.22+ | Statically typed, single binary, goroutine-native |
| MCP Protocol | mcp-go SDK | JSON-RPC 2.0 over stdio |
| Database | SQLite 3 | Ephemeral cache, WAL mode, FTS5 |
| File Storage | Markdown + YAML frontmatter | Git-friendly source of truth |
| Event Stream | JSONL | `events.jsonl`, `telemetry.jsonl` |
| Configuration | YAML | `config.yaml`, `registry.yaml`, `hooks.yaml`, `header-def.yaml` |
| CLI | Cobra | `backlogit` command with subcommands |
| Testing | `go test`, `testify` | TDD, contract tests, integration tests |
| Linting | `golangci-lint` | Zero-warning quality gate |

## Harness Surface

The repository carries an installed harness in `.github/` and uses backlogit as
its own operational backlog.

### Primary agents

| Agent | Purpose |
|---|---|
| `groomer` | Primary stash-to-backlog orchestrator |
| `shipper` | Primary backlog-to-shipped orchestrator |

### Supporting and legacy agents

| Agent | Purpose |
|---|---|
| `deliberator` | Routes idea work into deliberate or spike workflows |
| `backlog-harvester` | Converts plans into backlogit work items |
| `harness-architect` | Creates compilable but failing harnesses |
| `build-orchestrator` | Claims and executes ready feature work |
| `pr-review` | Manages review handoff and PR preparation |
| `memory` | Persists and restores session context |
| `doc-ops` | Maintains durable docs and documentation hygiene |
| `go-engineer` | Applies repository-specific Go standards |
| `go-mcp-expert` | Advises on Go MCP server implementation |
| `prompt-builder` | Maintains prompts, agents, instructions, and skills |

Treat `backlog-harvester`, `build-orchestrator`, and `pr-review` as legacy or
supporting surfaces unless the operator explicitly asks for the older path.

### Core skills

| Skill | Purpose |
|---|---|
| `deliberate` | Collaborative idea shaping |
| `spike` | Time-boxed technical investigation |
| `impl-plan` | Implementation plan generation |
| `plan-review` | Multi-persona plan gate |
| `build-feature` | Test-driven implementation loop |
| `review` | Structured code review |
| `fix-ci` | CI and review-comment remediation |
| `compound` | Durable learning capture |
| `compact-context` | Tracking and memory compaction |
| `runtime-verification` | Post-build runtime validation |
| `operational-closure` | Closure, rollout, and rollback capture |
| `safety-modes` | Elevated-risk workflow controls |

## Project Structure

```text
cmd/
  backlogit/
    main.go
internal/
  cli/
  config/
  core/
  db/
  errors/
  events/
  mcp/
  models/
  parser/
tests/
docs/
.backlogit/
```

## Commands

```bash
go test ./...                          # Run all tests
go vet ./...                           # Vet for suspicious constructs
golangci-lint run                      # Lint and static analysis
gofmt -l .                             # Format check
go build ./cmd/backlogit               # Build binary
go install ./cmd/backlogit             # Install binary
backlogit init                         # Initialize .backlogit/ workspace
backlogit create --type task --title "My task"  # Create artifact
backlogit deliberate <stash-id>        # Create deliberation from stash
backlogit sync                         # Force rehydration of index.db
backlogit mcp                          # Start MCP stdio server
```

## Hybrid Data Architecture

### Source of truth

Individual `.md` files in `.backlogit/` contain only current state via YAML
frontmatter and the current body content. Historical comments and event trails
do not belong in those files. The deliberation process converts transient ideas
into durable Markdown artifacts.

### Query engine

`.backlogit/index.db` is an ephemeral cache managed by backlogit. If it is
deleted or stale, the rehydration engine can rebuild it from the Markdown files
and JSONL queues.

### Event stream

When status changes, comments are appended, or telemetry is recorded, backlogit
stores those changes as JSONL entries for durable, append-only history.

### Transient queues

Data that has not yet graduated into a durable artifact may be stored as JSONL.
The stash is the canonical example: entries are transient ideas on their way to
becoming artifacts through deliberation. JSONL queues are Git-tracked,
append-friendly, and machine-parseable, but they are not sources of truth.

## Coding conventions

### Type safety

* Use Go structs with validator tags for data crossing package boundaries.
* Prefer explicit typing and standard Go interfaces over loosely typed maps.
* Treat `golangci-lint` warnings as real defects.

### Error handling

* Define sentinel and typed errors in `internal/errors/errors.go`.
* Wrap errors with context using `fmt.Errorf("context: %w", err)`.
* Do not use `panic()` in library code.
* Use `log/slog` for structured diagnostics.

### Testing

* TDD is required.
* Use colocated `_test.go` files for unit tests.
* Use `tests/contract/` for MCP contract coverage.
* Use `tests/integration/` for end-to-end and workspace-level flows.

## Search and lookup strategy

Use the lightest lookup that answers the question.

### For backlog state

Prefer backlogit-native operations before reading queue files directly:

1. `backlogit_get_metadata_catalog` for the workspace model and tool surface
2. queue-aware operations for ready work
3. `backlogit_get_item` for a specific artifact
4. `backlogit_query_sql` for targeted relational lookup
5. direct file reads in `.backlogit/` only when the tool surface cannot answer

### For code search

Prefer targeted grep, glob, or symbol-aware search over broad file dumping.
Search first, then read only the files that matter.

## Durable knowledge layout

| Path | Purpose |
|---|---|
| `docs/compound/` | Reusable learnings and hard-won fixes |
| `docs/exec-plans/` | Implementation plans |
| `docs/decisions/` | Durable decisions and investigation outputs |
| `docs/memory/` | Session memory and checkpoints |
| `docs/closure/` | Review, runtime verification, and closure artifacts |
| `docs/design-docs/` | Graduated architecture and design rationale |
| `docs/product-specs/` | Product-oriented requirements |

## Backlog workflow expectations

When backlogit is the active backlog tool for this repository:

* prefer queue-aware and dependency-aware operations over prose-only sequencing
* use backlogit comments, checkpoints, and commit-tracking features when they add traceability
* refresh the backlog index after out-of-band edits before trusting query results
* avoid inventing parallel markdown trackers outside the backlogit tool surface

## Session Memory Requirements

* All working agent sessions MUST persist their output to `docs/memory/` using the `memory` agent before the session ends.
* When the context window reaches approximately 65% capacity, invoke the `memory` agent to checkpoint current work before continuing.
* For long sessions, save memory checkpoints after completing each phase or major task group.
* Every memory entry must include task IDs completed, files modified, decisions and rationale, failed approaches, and concrete next steps.
* File convention: `docs/memory/[{YYYYMMDD}-{HHMMSS}]-{descriptive-slug}-memory.md`.
* Invoke `compact-context` when stale tracking or checkpoint volume starts hurting future sessions.
