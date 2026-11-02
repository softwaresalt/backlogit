---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-09T00:00:00Z
    origin: .backlogit/queue/047-DL.md
    status: harvested
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-09-schema-discoverability-plan.md
title: Schema Discoverability
---

# Schema Discoverability

## Problem Frame

Agents and operators interacting with backlogit face a schema discoverability gap. The `backlogit_get_metadata_catalog` MCP tool returns workspace config, artifact types, templates, stash config, and tool inventory but not the SQLite schema. Agents using `backlogit_query_sql` must rely on instruction files for schema knowledge. Separately, the telemetry subsystem harvests data into three JSONL fact tables with typed Go structs, but no CLI-accessible reference exists for these schemas.

Both problems share the same root cause: schema metadata lives only in source code and instruction files rather than being programmatically accessible.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Metadata catalog response includes `sql_schema` section with all SQLite tables, columns, types, indexes, and FTS virtual tables | Stash ACDF8C2D |
| R2 | `backlogit telemetry schema` CLI subcommand prints fact table field definitions | Stash 1D5578B5 |
| R3 | Telemetry schema supports text (default), json, and markdown output formats | Stash 1D5578B5 |
| R4 | SQL schema is extracted at runtime from the actual database, not hardcoded | Deliberation 047-DL |
| R5 | Telemetry schema includes SQLite telemetry tables as a separate section | Deliberation 047-DL |

## Scope Boundaries

### In Scope

- SQL schema section in `MetadataCatalog` struct and `BuildMetadataCatalog`
- Runtime schema introspection using `PRAGMA table_info` and `PRAGMA index_list`
- `backlogit telemetry schema` CLI subcommand
- Three output formats for telemetry schema: text, json, markdown
- Unit and contract tests for both features

### Non-Goals

- MCP `backlogit_telemetry_schema` tool (deferred)
- Schema validation or migration tooling
- Example queries in the catalog (instruction files own those)
- Modifying the SQL schema instruction file beyond adding a catalog reference

### Deferred to Implementation

- Exact text formatting for the telemetry schema output
- Whether to use reflection or manual struct-tag metadata for JSONL schemas

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort.

### Unit 1: SQL Schema Introspection in db Package

**Files:** `internal/db/schema.go`
**Test files:** `internal/db/schema_gen_test.go` (extend existing)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `EnsureSchema` in `internal/db/schema.go`, `existingColumns` pattern in `internal/db/schema_gen.go`
**Dependencies:** none

**Approach:**
Add an `IntrospectSchema` function to `internal/db/schema.go` that queries `sqlite_master` for table names and `PRAGMA table_info(tablename)` for columns. Returns a `[]TableSchema` struct containing table name, columns (name, type, not-null, primary key), indexes (from `PRAGMA index_list` + `PRAGMA index_info`), and whether the table is an FTS virtual table.

Signature: `IntrospectSchema(ctx context.Context, db *sql.DB) ([]TableSchema, error)` — context parameter for timeout safety (Plan Review F2). Open a read transaction for schema consistency (Plan Review F5). Validate table names from `sqlite_master` before interpolating into PRAGMA queries (Plan Review F1 defense-in-depth). Add `index_list` and `index_info` to `allowedPragmas` in `gate.go`. Add `slog.Debug` logging for introspection steps (Plan Review F8).

The function takes a `context.Context` and `*sql.DB`, returns schema for all tables including telemetry tables when they exist. Use `sqlite_master WHERE type IN ('table','trigger')` to enumerate, filter system tables (`sqlite_sequence`, etc.).

**Verification:**
- `IntrospectSchema` returns at least `items`, `item_deps`, `commit_links`, `stash_entries`, `item_links`, `stash_links`, `item_logs`, `item_log_entries`, `items_fts`, `item_log_entries_fts` tables
- Column metadata matches the known schema
- FTS virtual tables are identified correctly
- Works on an empty database after `EnsureSchema`

### Unit 2: SQL Schema in Metadata Catalog

**Files:** `internal/core/metadata_catalog.go`, `internal/mcp/metadata.go`
**Test files:** `internal/core/metadata_catalog_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `MetadataCatalog` struct fields and `BuildMetadataCatalog` signature in `internal/core/metadata_catalog.go`
**Dependencies:** Unit 1

**Approach:**
1. Add `SQLSchema []db.TableSchema` field to `MetadataCatalog` struct with JSON tag `"sql_schema,omitempty"`.
2. Update `BuildMetadataCatalog` signature to accept an optional `*sql.DB` parameter (or a `[]db.TableSchema` slice to keep the function DB-agnostic).
3. In MCP handler `loadMetadataCatalog`, call `db.IntrospectSchema` on the workspace DB and pass the result to `BuildMetadataCatalog`.
4. Update the existing `TestBuildMetadataCatalog_ReturnsUnifiedCatalog` to verify the `sql_schema` field when a DB is provided.

Design decision: Pass `[]db.TableSchema` into `BuildMetadataCatalog` rather than `*sql.DB` to keep core package DB-agnostic. The MCP layer calls `IntrospectSchema` and passes the result.

**Verification:**
- `backlogit_get_metadata_catalog` response includes `sql_schema` array
- Each table entry has name, columns, indexes
- `items_fts` and `item_log_entries_fts` appear as virtual tables
- Existing catalog test still passes with nil schema (omitempty)

### Unit 3: Telemetry Schema Reference Types

**Files:** `internal/telemetry/schema_ref.go` (new)
**Test files:** `internal/telemetry/schema_ref_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/telemetry/records.go` struct definitions, `internal/telemetry/types.go`
**Dependencies:** none

**Approach:**
Create a `FactTableSchema` struct and a `DescribeFactTables` function that returns schema metadata for the three JSONL fact tables:
- `tool-calls.jsonl` → `ToolCallFact` fields
- `session-facts.jsonl` → `SessionFact` fields
- `telemetry-sessions.jsonl` → `SessionSummaryRecord` fields

Each field entry includes: JSON field name, Go type (as string), and a human-readable description. Use a manual registry approach (not reflection) for reliability and control over descriptions.

Also include a `DescribeTelemetrySQLTables` function that returns schema for the two SQLite telemetry tables (`telemetry_sessions`, `telemetry_tool_usage`) with field descriptions.

**Verification:**
- `DescribeFactTables` returns 3 tables with correct field counts matching struct definitions
- `DescribeTelemetrySQLTables` returns 2 tables
- Field names match JSON tags on the Go structs
- No duplicate field names within a table
- Drift-detection test: compare registry field names against struct JSON tags via reflection (Plan Review F3)

### Unit 4: Telemetry Schema CLI Subcommand

**Files:** `internal/cli/telemetry.go`
**Test files:** `internal/cli/telemetry_test.go` (new or extend)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newTelemetryReportCmd` in `internal/cli/telemetry.go`, `newRenderer` / `validateFormat` pattern in `internal/cli/list.go`
**Dependencies:** Unit 3

**Approach:**
Add `newTelemetrySchemaCmd` returning a `*cobra.Command` with:
- `Use: "schema"`
- `Short: "Print telemetry data schema reference"`
- `--format` flag: text (default), json, markdown
- Output: For each fact table and SQLite table, print table name, description, and field listing (name, type, description)
- Register via `cmd.AddCommand(newTelemetrySchemaCmd(cwd))` in `NewTelemetryCmd`

Text format example:
```
## tool-calls.jsonl (Tool Call Facts)
Individual completed tool calls harvested from session events.

  Field              Type      Description
  record_type        string    Record type identifier ("tool_call_fact")
  session_id         string    Session identifier
  ...
```

JSON format: structured array of table objects with fields array.
Markdown format: table with columns Name, Type, Description.

**Verification:**
- `backlogit telemetry schema` prints all fact tables and SQLite tables
- `--format json` produces valid JSON with all tables
- `--format markdown` produces valid markdown tables
- Command appears in `backlogit telemetry --help`

### Unit 5: CLI Reference Regeneration and Quality Gates

**Files:** `docs/cli-reference/` (auto-generated)
**Test files:** none (CI drift check validates)
**Effort size:** small
**Skill domain:** docs
**Execution note:** regenerate after all code units
**Patterns to follow:** `go run ./cmd/gen-docs`
**Dependencies:** Units 1-4

**Approach:**
1. Run `go run ./cmd/gen-docs` to regenerate CLI reference docs
2. Run all quality gates: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
3. Verify `backlogit_telemetry_schema.md` appears in generated docs
4. Verify `backlogit_get_metadata_catalog` doc reflects schema section

**Verification:**
- All quality gates pass
- New CLI reference doc `backlogit_telemetry_schema.md` exists
- CI drift check will pass (docs regenerated from current code)

## Dependency Graph

```
Unit 1 (DB introspection) ──► Unit 2 (Catalog integration)
                                                            ──► Unit 5 (Docs + gates)
Unit 3 (Telemetry schema types) ──► Unit 4 (CLI command)
```

Units 1 and 3 are independent and can be implemented in parallel.
Unit 5 depends on all prior units.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Pass `[]TableSchema` to `BuildMetadataCatalog`, not `*sql.DB` | Keeps core package DB-agnostic; MCP layer owns the introspection call | Passing `*sql.DB` into core — violates layer separation |
| D2 | Manual field registry for telemetry schemas, not reflection | Reflection is fragile with JSON tags and omitempty; manual gives control over descriptions | Reflection-based — harder to maintain descriptions, brittle with unexported fields |
| D3 | Runtime PRAGMA introspection for SQL schema | Catalog always reflects actual DB schema, no sync drift | Hardcoded schema — would drift from `EnsureSchema` and `EnsureTelemetrySchema` changes |
| D4 | Include telemetry SQLite tables in telemetry schema command output | These tables are part of the telemetry data surface operators query | Exclude — operators would miss queryable telemetry tables |

## Risks and Caveats

- **PRAGMA availability**: `PRAGMA table_info` and `PRAGMA index_list` are standard SQLite; `modernc.org/sqlite` supports them.
- **Schema extensions**: Custom fields from `header-def.yaml` add columns via `ALTER TABLE`. `IntrospectSchema` will capture these automatically since it reads the live schema.
- **Telemetry tables may not exist**: `EnsureTelemetrySchema` is lazy; the tables exist only after first harvest. `IntrospectSchema` should handle missing telemetry tables gracefully.
- **FTS virtual tables**: `sqlite_master` lists FTS tables with type `'table'` and SQL containing `fts5`. Use this to identify them.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **present** — `MetadataCatalog` JSON response gains a new `sql_schema` field; new CLI subcommand added
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action, or irreversible step: **absent**
* external integration, operator checkpoint, or external dependency: **absent**
* high runtime, rollout, or rollback risk: **absent**

Requires plan hardening: **no** — The catalog field is additive (omitempty), the CLI subcommand is new, and no data migration is required. The changes are backward-compatible.

## Runtime Verification and Closure

- **Unit 2 (catalog)**: Changes MCP tool response. Verify by calling `backlogit_get_metadata_catalog` and confirming `sql_schema` section is populated with correct table count.
- **Unit 4 (CLI)**: New CLI subcommand. Verify by running `backlogit telemetry schema` and confirming output for all three formats.
- **Closure**: Standard post-merge verification. No monitoring plan needed — these are read-only introspection features with no runtime risk.

## Learnings Applied

- `docs/compound/2026-05-07-mcp-cli-config-parity.md`: Reinforces the pattern of keeping MCP and CLI surfaces aligned; both surfaces should expose schema information through their natural channels.

## Standards Check

- GoDoc comments required on all exported types and functions (Units 1-4)
- Test-first development for all units
- `golangci-lint` zero warnings
- Conventional commits for all commits
- Constitution Principle IX (Agent Context Efficiency): SQL schema in catalog directly supports token-efficient agent queries

## Plan Review

**Gate Decision: PASS**

Reviewed by: Constitution Reviewer, Go Quality Reviewer, Scope Boundary Auditor, Architecture Strategist (GPT-5.4).

### Summary

4 personas returned 37+ findings total. After deduplication and validation against the actual codebase, the merged finding set contains 0 P0, 2 P1, 4 P2, and 3 P3 findings. The most critical initial finding (core→db dependency violation) was dismissed after confirming that `internal/core` already imports `internal/db` extensively in production code (`archive.go`, `artifacts.go`, `commits.go`, `queue.go`, `shipment.go`, `stash.go`, `workspace.go`). No architectural boundary is violated.

### P0 — Critical

None after validation.

### P1 — High

**F1 (Go Quality): PRAGMA allowlist awareness.**
`IntrospectSchema` uses `PRAGMA index_list` and `PRAGMA index_info` which are not in the `allowedPragmas` slice in `internal/db/gate.go`. While `IntrospectSchema` calls the DB directly (not through the gate), the plan should note these PRAGMAs are trusted internal calls. If agents ever need `PRAGMA index_list` via `backlogit_query_sql`, the allowlist must be updated.
**Resolution**: Add implementation note to Unit 1: validate table names from `sqlite_master` before interpolating into PRAGMA queries. Add `index_list` and `index_info` to `allowedPragmas`.

**F2 (Go Quality): Add `context.Context` parameter to `IntrospectSchema`.**
Standard Go practice for database operations. Enables timeout safety and cancellation.
**Resolution**: Adopt. `IntrospectSchema(ctx context.Context, db *sql.DB) ([]TableSchema, error)`.

### P2 — Moderate

**F3 (Scope): Manual field registry drift risk.**
If telemetry struct fields change, the manual registry in `schema_ref.go` could drift from `records.go`. Recommendation: add a test that compares registry field names against struct JSON tags via reflection.
**Resolution**: Adopt as a test strategy in Unit 3 — add a drift-detection test.

**F4 (Constitution): Schema staleness strategy.**
Is schema introspected per-request or cached? For catalog, per-request is appropriate since `loadMetadataCatalog` is called on each `backlogit_get_metadata_catalog` invocation and the catalog is already assembled fresh each time.
**Resolution**: No change needed — per-request is consistent with existing catalog behavior.

**F5 (Go Quality): Wrap PRAGMA queries in a read transaction.**
Ensures consistent schema view during introspection.
**Resolution**: Adopt — `IntrospectSchema` should open a read transaction.

**F6 (Scope): Unit 4 effort may be underestimated.**
Three output formats (text/json/markdown) plus tests. Consider medium-to-large.
**Resolution**: Already marked as medium. Acceptable.

### P3 — Low

**F7 (Architecture): Dependency graph is otherwise consistent.** No action needed.

**F8 (Constitution): Add `slog` debug logging for schema introspection.** Adopt as implementation-time detail.

**F9 (Scope): Backward compatibility of catalog response.** The `sql_schema` field uses `omitempty` so older clients are unaffected. No action needed.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F1 | Go Quality Reviewer | Claude Haiku 4.5 |
| F2 | Go Quality Reviewer | Claude Haiku 4.5 |
| F3 | Scope Boundary Auditor | Claude Haiku 4.5 |
| F4 | Constitution Reviewer | Claude Haiku 4.5 |
| F5 | Go Quality Reviewer | Claude Haiku 4.5 |
| F6 | Scope Boundary Auditor | Claude Haiku 4.5 |
| F7 | Architecture Strategist | GPT-5.4 |
| F8 | Constitution Reviewer | Claude Haiku 4.5 |
| F9 | Scope Boundary Auditor | Claude Haiku 4.5 |

### Dismissed Findings

| Finding | Reviewer | Reason |
|---|---|---|
| core→db dependency violation (P0) | Architecture Strategist | Invalid — `internal/core` already imports `internal/db` in 7+ production files |
| Low cohesion mixing schema with catalog (P1) | Architecture Strategist | Schema metadata is a natural extension of the workspace model catalog |
| Test plan missing (P0) | Constitution, Scope | Plan specifies test-first execution and verification criteria per unit; exact test cases are implementation-time decisions |
| SQL injection in PRAGMA (P0) | Go Quality | Table names sourced from `sqlite_master` are trusted system values; validation added as P1 defense-in-depth |

### Next Steps

Plan passes review. Proceed to harvest to decompose into backlogit work items.
