---
chunk_strategy: h1-h2-h3
description: Multi-persona review gate results for the backlogit core implementation plan
doc_type: review
docline:
    gate_decision: ADVISORY
    ms.date: 2026-03-30T00:00:00Z
ingested_at: "2026-06-26T02:33:53Z"
schema_version: "1.0"
source: docs/reviews/2026-03-30-backlogit-core-plan-review.md
title: 'Plan Review: Backlogit Core Implementation'
---

## Review Summary

Four reviewer personas evaluated the implementation plan against architectural soundness, Go quality, constitutional compliance, and scope boundaries.

No P0 (critical) findings were raised by any reviewer. The plan is architecturally sound and aligns with constitutional principles. All reviewers confirmed the core CQRS architecture, package layout, and MCP tool surface are correct.

Eleven deduplicated P1 findings identify plan gaps that must be addressed via plan amendments before harvesting. All P1 findings are addressable through targeted amendments without requiring a fundamental redesign.

## Gate Decision: ADVISORY

All P1 findings are addressed through plan amendments below. The amended plan passes review. Proceed to Phase 3 harvesting.

## Deduplicated P1 Findings (11)

### P1-01: Markdown parser scheduled after consumers

Reviewers: Architecture Strategist, Scope Boundary Auditor

`internal/parser/markdown.go` (ParseMarkdownFile) is in Unit 9 (Legacy Migration) but Unit 5 (Rehydration) and Unit 7 (MCP get_item) need it.

**Amendment**: Move `internal/parser/markdown.go` and its tests from Unit 9 to Unit 3 (Models and Frontmatter). Unit 9 retains only `legacy.go` and `migration.go`.

### P1-02: Missing context.Context on I/O function signatures

Reviewers: Go Quality Reviewer, Scope Boundary Auditor

Functions performing file I/O or database operations (CreateArtifact, Rehydrate, AppendEvent, TailEvents, Migrate) lack `context.Context` as their first parameter.

**Amendment**: All I/O functions in Units 4, 5, 6, and 9 must accept `context.Context` as their first parameter. Rehydration errgroup uses `errgroup.WithContext(ctx)` for cancellation propagation.

### P1-03: Global mutable state in MCP server

Reviewers: Go Quality Reviewer, Scope Boundary Auditor

Prototype uses `var appManager *BacklogitManager`. Plan must specify dependency injection.

**Amendment**: Unit 7 MCP server uses a `Server` struct holding `*sql.DB`, `*BacklogitConfig`, and workspace path. Tool handlers are method-value closures on the Server struct. No package-level mutable state.

### P1-04: Frontmatter ParseFrontmatter return type ambiguous

Reviewers: Go Quality Reviewer, Scope Boundary Auditor

Return type `(map, body, error)` is too vague. `map[string]string` is too restrictive for real YAML (arrays, nested maps).

**Amendment**: `ParseFrontmatter` returns `(map[string]any, string, error)` for the raw parse. A separate `ArtifactFromFrontmatter(fm map[string]any, body string) (*Artifact, error)` converts to typed struct. Both functions live in Unit 3.

### P1-05: Database connection lifecycle unspecified

Reviewers: Go Quality Reviewer, Scope Boundary Auditor

Conflicting patterns: conventions say one `*sql.DB` per workspace, but MCP tool examples show per-call connections.

**Amendment**: One `*sql.DB` opened at MCP server startup (or CLI command start), stored in Server struct, passed to all tool handlers, closed on shutdown via `defer`. No per-tool-call connection opening.

### P1-06: No orchestration layer for cross-store writes

Reviewers: Architecture Strategist, Scope Boundary Auditor

No application layer coordinates writes spanning Markdown, SQLite, and JSONL. Partial failures cause inconsistency.

**Amendment**: Add `internal/core/workspace.go` with a `Workspace` struct that holds the config, DB connection, and event stream path. Write operations (CreateArtifact, UpdateArtifact) on this struct coordinate file write → DB upsert → event append as a single workflow. If the DB upsert fails after file write, log a warning and mark index as stale (rehydration self-heals).

### P1-07: Memory and checkpoint persistence missing

Reviewers: Architecture Strategist, Scope Boundary Auditor

`backlogit_save_memory` and `backlogit_create_checkpoint` MCP tools are listed but no package persists `memories.json` or checkpoint files.

**Amendment**: Add `internal/events/memory.go` with `SaveMemory(path, key, summary)` writing to `memories.json` (JSON object keyed by memory key) and `CreateCheckpoint(dir, stateDump)` writing timestamped files to `checkpoints/`. Add to Unit 6 file list with tests.

### P1-08: Custom fields have no database representation

Reviewers: Architecture Strategist, Scope Boundary Auditor

Custom fields from `config.yaml` exist in Markdown frontmatter but have no queryable storage in SQLite.

**Amendment**: Add a `custom_fields` JSON column to the `items` table in Unit 5 schema. Rehydration extracts non-standard frontmatter keys into this column. The gated SQL query can filter on `json_extract(custom_fields, '$.priority')`.

### P1-09: Missing CLI migrate command

Reviewers: Scope Boundary Auditor

Architecture defines `backlogit migrate` CLI but plan only has the parser pipeline (Unit 9) without a CLI wiring.

**Amendment**: Add `internal/cli/migrate.go` to Unit 8 with `backlogit migrate [path]` command that invokes the Unit 9 migration pipeline.

### P1-10: SafeResolve missing for workspace containment

Reviewers: Go Quality Reviewer, Scope Boundary Auditor

Constitution mandates all file operations resolve within `.backlogit/` with path traversal rejection. Plan includes `routing.go` but no `SafeResolve` function.

**Amendment**: Add `internal/core/workspace.go` function `SafeResolve(workspaceRoot, target) (string, error)` that rejects `..` traversal. All file operations in Units 4, 5, 6, and 9 must use SafeResolve before any filesystem access.

### P1-11: Sprint containers modeled but not integrated

Reviewers: Architecture Strategist, Scope Boundary Auditor

Sprint struct exists in models but no indexing, retrieval API, or MCP tool to query sprints with their linked artifacts.

**Amendment**: Add sprint rows to the `items` table (type='sprint'). Rehydration indexes sprint files. `backlogit_query_sql` can already query sprints via SQL. Defer a dedicated `backlogit_get_sprint` tool to a follow-up plan. Record as P2 follow-up.

## Deduplicated P2 Findings (12)

| ID | Finding | Units | Disposition |
|------|---------|-------|-------------|
| P2-01 | Rehydration needs TRUNCATE + bulk INSERT in single transaction | 5 | Address during implementation |
| P2-02 | FieldConfig.ExternalMap uses map[string]any without comment | 2 | Add justification comment |
| P2-03 | Correct SQLite blank import pattern `_ "modernc.org/sqlite"` | 5 | Note in unit spec |
| P2-04 | JSONL concurrent append safety mechanism unspecified | 6 | Specify sync.Mutex for goroutine safety, O_APPEND for process safety |
| P2-05 | Error wrapping with sentinels inconsistent across units | 4, 6, 9 | Each unit specifies its sentinel |
| P2-06 | filepath.Glob("**") doesn't recurse in Go | 4 | Use filepath.WalkDir instead |
| P2-07 | MCP error helpers use fmt.Sprintf with raw JSON templates | 7 | Use json.Marshal for safe escaping |
| P2-08 | Over-scoped prompt templates (create_sprint, triage_bug) | 7 | Defer to follow-up; remove from Unit 7 |
| P2-09 | Over-engineered hooks.yaml typing for deferred scope | 2 | Reduce to minimal stub |
| P2-10 | Vague acceptance criteria language | 3, 8, 9, 10 | Sharpen during task decomposition |
| P2-11 | Unit 5 oversized (connection + schema + queries + rehydration + gate) | 5 | Split during harvesting into DB Foundation + Rehydration |
| P2-12 | Missing doc.go files in unit file lists | All | Add during implementation |

## P3 Findings (2)

| ID | Finding | Disposition |
|------|---------|-------------|
| P3-01 | Missing doc.go files | Add during implementation |
| P3-02 | Validator instance should be cached singleton | Note in implementation |

## Reviewer Concordance

| Area | Arch Strategist | Go Quality | Constitution | Scope Auditor |
|------|:-:|:-:|:-:|:-:|
| Core CQRS architecture | ✅ | ✅ | ✅ | ✅ |
| Package layout | ✅ | ✅ | ✅ | ✅ |
| MCP tool surface | ✅ | ✅ | ✅ | ✅ |
| Parser dependency order | ❌ P1 | — | — | ❌ P1 |
| context.Context usage | — | ❌ P1 | — | ❌ P1 |
| DI vs global state | — | ❌ P1 | — | ❌ P1 |
| Cross-store consistency | ❌ P1 | — | — | ❌ P1 |
| Memory/checkpoint gaps | ❌ P1 | — | — | ❌ P1 |
