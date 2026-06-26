---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-13T00:00:00Z
    origin: 001-DL (deliberation), stash DDD1F38A/AB6959FC/73E63809/91D459D8
    requires_plan_hardening: "no"
    status: revised
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-13-telemetry-pipeline-enhancements-plan.md
title: Telemetry Pipeline Enhancements
---

## Overview

Four enhancements to the telemetry subsystem shipped in F021 (Token Telemetry).
These are the Plan Review F2 deferred items: incremental harvest, date-filtered
harvest, context window tracking, and a CLI report subcommand. All share the
code surface `internal/telemetry/`, `internal/cli/telemetry.go`, and
`.backlogit/telemetry-sessions.jsonl`.

## Problem Frame

The v1 telemetry pipeline (F021) performs a full re-harvest of all Copilot CLI
logs on every invocation. As log volume grows, this becomes inefficient. The
pipeline also lacks context window utilization metrics and provides no CLI
reporting — all three subcommands (`harvest`, `list`, `top`) are stubs, and
`report` is not registered at all.

This plan addresses four stash entries as a single cohesive feature:

| Stash ID | Original ID | Enhancement |
|---|---|---|
| DDD1F38A | 23884888 | Context window consumption tracking |
| AB6959FC | E14AB8CC | CLI telemetry report subcommand |
| 73E63809 | 01D115EC | Incremental harvest with byte-offset checkpoints |
| 91D459D8 | CD92F274 | Date-filtered harvest (--since flag) |

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Track byte offsets per log file to avoid re-parsing processed data | Stash 73E63809, Plan F2 |
| R2 | Persist checkpoint state in `.backlogit/.telemetry-checkpoint.json` | Stash 73E63809 |
| R3 | Support `--force` flag to ignore checkpoint and re-harvest all logs | Stash 73E63809 |
| R4 | Accept `--since DATE` flag to scope harvest to logs after a given date | Stash 91D459D8 |
| R5 | Filter telemetry events by parsed timestamp when `--since` is set | Stash 91D459D8 |
| R6 | Derive context window utilization from model-specific limits and token counts | Stash DDD1F38A |
| R7 | Track peak utilization, remaining capacity, and depletion rate per session | Stash DDD1F38A |
| R8 | Persist context window metrics in SessionSummaryRecord and SQLite schema | Stash DDD1F38A |
| R9 | Implement `backlogit telemetry report` with `--session`, `--by`, and `--format` flags | Stash AB6959FC |
| R10 | Replace stub implementations for `harvest`, `list`, and `top` subcommands | Stash AB6959FC |
| R11 | Update MCP tool `backlogit_telemetry_harvest` to accept `--since` and `--force` parameters | Stash 73E63809/91D459D8 |

## Scope Boundaries

### In Scope

* Byte-offset checkpoint file (`.backlogit/.telemetry-checkpoint.json`) with
  per-log-file tracking
* `--force` flag on harvest to bypass checkpoint and re-process all logs
* `--since DATE` flag on harvest to filter by event timestamp
* Context window utilization metrics derived from model-to-limit lookup table
* Peak utilization, remaining capacity, depletion rate calculations per session
* Extended `SessionSummaryRecord` and `SessionSummary` with context window fields
* Extended SQLite `telemetry_sessions` table with context window columns
* Working `telemetry harvest` CLI command with `--since` and `--force` flags
* `telemetry report` CLI command with `--session`, `--by`, `--format` flags
* Working `telemetry list` and `telemetry top` CLI commands
* MCP tool parameter extensions for `--since` and `--force`

### Out of Scope

* Advisory file locking for concurrent harvest (defer until checkpoint is proven)
* YAML config override for attribution registry (deferred per Plan F12)
* `compare` and `trend` subcommands (deferred per original plan D5)
* Real-time streaming telemetry
* VS Code Copilot Chat log parsing
* Context window data from raw log fields (logs do not emit max_context_length;
  we derive utilization from a model-limit lookup table)

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Checkpoint in `.backlogit/.telemetry-checkpoint.json` | Checkpoint is mutable state but is derived/reconstructable from source logs. Living in `.backlogit/` keeps it co-located with other telemetry state. File is gitignored alongside `backlogit.db`. | Store in `.copilot/` (rejected: violates read-only `.copilot/` access), store in memory only (rejected: no persistence across runs) |
| D2 | Rewrite semantics with merge on incremental harvest | When incremental data is harvested, merge new session data with existing JSONL by session ID, then atomic rewrite. Avoids growing append-only file and deduplication complexity on read. Consistent with current atomic temp-file-then-rename pattern. | Append-only (rejected: requires dedup on read, growing file), in-place update (rejected: corruption risk) |
| D3 | Model-to-context-limit lookup table for utilization | Copilot CLI logs do not emit `max_context_length`. Derive utilization from hardcoded model limits (e.g., `claude-sonnet-4` → 200k, `gpt-4.1` → 1M). Table is extensible via code; YAML config deferred. | Require log format change (rejected: external dependency), skip utilization (rejected: loses key metric) |
| D4 | Depletion rate = total tokens consumed / model calls in session | Simple, deterministic formula. Measures average token consumption per model turn. More nuanced formulas (per-minute, per-tool) deferred until real-world data validates the need. | Per-minute rate (rejected: requires precise timestamps not always available), per-tool (rejected: attribution already exists separately) |
| D5 | `report` replaces `list` and `top` as primary reporting surface | `report` with `--by` dimension covers the use cases of both `list` and `top`. Keep `list` and `top` as aliases/thin wrappers rather than independent implementations. | Three separate commands with duplicated query logic (rejected: maintenance burden) |

## Task Decomposition

### Task 1: Incremental Harvest with Byte-Offset Checkpoints

**Stash origin:** 73E63809 (orig 01D115EC)
**Files:** `internal/telemetry/checkpoint.go` (new), `internal/telemetry/harvest.go`,
`internal/telemetry/types.go`, `internal/mcp/tools.go`
**Test files:** `internal/telemetry/checkpoint_test.go` (new),
`internal/telemetry/harvest_test.go`, `tests/contract/telemetry_tool_test.go`
**Effort:** medium
**Dependencies:** none (foundational)

**Approach:**

Define a checkpoint struct and persistence functions:

```go
// HarvestCheckpoint tracks the last-read byte offset per log file
// to enable incremental harvest.
type HarvestCheckpoint struct {
    FileOffsets map[string]int64  `json:"file_offsets"` // logFileName → byteOffset
    LastHarvest time.Time         `json:"last_harvest"`
    Version     int               `json:"version"`      // schema version for forward compat
}
```

New file `checkpoint.go`:

* `LoadCheckpoint(workspacePath string) (*HarvestCheckpoint, error)` — read from
  `.backlogit/.telemetry-checkpoint.json`; return zero checkpoint if file missing
* `SaveCheckpoint(workspacePath string, cp *HarvestCheckpoint) error` — atomic
  write via temp-file-then-rename (match `writeTelemetryJSONL` pattern)

Modify `parseLogFiles` to accept a checkpoint and return an updated checkpoint:

```go
func parseLogFiles(logsDir string, cp *HarvestCheckpoint) ([]TelemetryEvent, *HarvestCheckpoint, error)
```

For each log file:
* If `cp.FileOffsets[filename] > 0`, seek to that offset before parsing
* If file size is less than the checkpoint offset, reset offset to 0 with
  a slog warning (handles file truncation/rotation)
* After parsing, record the final byte offset in the returned checkpoint
* New files (not in checkpoint) are parsed from offset 0

Modify `HarvestTelemetry` signature to accept options:

```go
type HarvestOptions struct {
    Force bool       // ignore checkpoint, re-process all logs
    Since *time.Time // filter events after this timestamp
}

func HarvestTelemetry(ctx context.Context, workspacePath, copilotPath string,
    sqlDB *sql.DB, opts HarvestOptions) (HarvestResult, error)
```

When `opts.Force` is false:
1. Load checkpoint
2. Parse with offsets
3. Merge new summaries with existing JSONL data (load existing, merge by
   session ID, rewrite)
4. Save updated checkpoint

When `opts.Force` is true:
1. Skip checkpoint load
2. Parse all files from offset 0
3. Overwrite JSONL (current behavior)
4. Save fresh checkpoint

**MCP handler update (F1 fix):** Update the `backlogit_telemetry_harvest` handler
in `internal/mcp/tools.go` to pass `HarvestOptions{}` (zero value) when calling
the new signature. This ensures the MCP tool compiles and behaves identically to
v1 (full re-harvest, no `--since`). Task 4 later extends the handler to parse
optional `since` and `force` parameters from the MCP request.

Update the contract test in `tests/contract/telemetry_tool_test.go` to verify
the handler still works with the new signature.

**Verification:**

* Unit test: `LoadCheckpoint` returns zero checkpoint on missing file
* Unit test: `LoadCheckpoint` returns zero checkpoint on corrupt JSON (log warning
  via slog, treat as missing — checkpoint is derived state)
* Unit test: `SaveCheckpoint` + `LoadCheckpoint` roundtrip preserves offsets
* Integration test: harvest with checkpoint skips already-processed bytes
* Integration test: `--force` ignores checkpoint and re-processes everything
* Integration test: new log file added between harvests is fully processed
* Integration test: file truncated below checkpoint offset resets to 0 with warning
* Regression test: existing `TestHarvestTelemetry_ProducesSessionSummaries` still
  passes with default `HarvestOptions{}`
* Contract test: MCP tool handler compiles and returns structured JSON with
  `HarvestOptions{}`

### Task 2: Date-Filtered Harvest (--since flag)

**Stash origin:** 91D459D8 (orig CD92F274)
**Files:** `internal/telemetry/harvest.go`, `internal/telemetry/parser.go`
**Test files:** `internal/telemetry/harvest_test.go`, `internal/telemetry/parser_test.go`
**Effort:** small
**Dependencies:** Task 1 (shares `HarvestOptions` struct)

**Approach:**

The `--since` filter operates at the event level during parsing. Raw log lines
contain timestamps (e.g., `2026-04-09T00:00:02.000Z [telemetry] {...}`). The
parser already scans each line but discards the timestamp prefix.

Extend the `CopilotCLIParser` to extract the line timestamp and pass it to the
emit callback via a new field on `TelemetryEvent`:

```go
type TelemetryEvent struct {
    Kind      EventKind  `json:"kind"`
    ModelCall *ModelCall `json:"model_call,omitempty"`
    ToolCall  *ToolCall  `json:"tool_call,omitempty"`
    Timestamp time.Time  `json:"timestamp,omitempty"` // parsed from log line prefix
}
```

In `parseLogFiles`, when `opts.Since` is set, skip events where
`event.Timestamp.Before(*opts.Since)`. This filtering happens after parsing but
before accumulation, so only qualifying events enter the correlation pipeline.

For incremental harvest (Task 1 checkpoint), `--since` acts as an additional
filter on top of byte-offset resumption: events from resumed parsing that
predate `--since` are still skipped.

**Verification:**

* Unit test: parser extracts timestamp from log line prefix
* Unit test: events before `--since` are excluded from harvest results
* Unit test: malformed timestamp prefix assigns `time.Time{}` (zero value);
  events with zero timestamps are always included when `--since` is set
  (safe default — never silently drop events)
* Integration test: harvest with `--since` produces fewer sessions than full harvest
* Edge case: `--since` in the future returns zero sessions (not an error)

### Task 3: Context Window Consumption Tracking

**Stash origin:** DDD1F38A (orig 23884888)
**Files:** `internal/telemetry/types.go`, `internal/telemetry/context_window.go` (new),
`internal/telemetry/correlator.go`, `internal/telemetry/records.go`,
`internal/db/telemetry_schema.go`
**Test files:** `internal/telemetry/context_window_test.go` (new),
`internal/telemetry/correlator_test.go`
**Effort:** medium
**Dependencies:** none (extends data model independently; ordered after T1/T2 to
avoid merge conflicts in shared files)

**Approach:**

New file `context_window.go` with model-to-limit lookup:

```go
// ModelContextLimits maps model identifiers to their maximum context window
// size in tokens. Used to derive utilization metrics when raw context window
// data is unavailable from log sources.
var ModelContextLimits = map[string]int{
    "claude-sonnet-4":     200000,
    "claude-sonnet-4.5":   200000,
    "claude-haiku-4.5":    200000,
    "claude-opus-4":       200000,
    "gpt-4.1":            1000000,
    "gpt-4.1-mini":        500000,
    "gpt-5":              1000000,
    "o4-mini":             200000,
}

// ContextWindowMetrics holds derived context utilization for a session.
type ContextWindowMetrics struct {
    PeakUtilization    float64 `json:"peak_utilization"`     // highest prompt_tokens/max_tokens across model calls
    RemainingCapacity  int     `json:"remaining_capacity"`   // max_tokens - peak_prompt_tokens at peak
    DepletionRate      float64 `json:"depletion_rate"`       // total_tokens / model_calls (avg tokens per turn)
    MaxContextTokens   int     `json:"max_context_tokens"`   // model limit used for calculations
    PeakPromptTokens   int     `json:"peak_prompt_tokens"`   // highest prompt token count observed
    CompactionCount    int     `json:"compaction_count"`      // number of compaction events
}

// ComputeContextMetrics derives context window utilization from model calls
// and the model-to-limit lookup table.
func ComputeContextMetrics(modelCalls []ModelCall, compactionEvents []CompactionEvent) *ContextWindowMetrics
```

Computation logic:
1. For each model call, look up the model in `ModelContextLimits`
2. Track the highest `PromptTokens` as peak utilization
3. `PeakUtilization` = `PeakPromptTokens / MaxContextTokens`
4. `RemainingCapacity` = `MaxContextTokens - PeakPromptTokens`
5. `DepletionRate` = `TotalTokens / len(modelCalls)` (average per turn)
6. If model is unknown, use a conservative default (200k)

Extend `SessionSummary`:

```go
type SessionSummary struct {
    // ... existing fields ...
    ContextWindow *ContextWindowMetrics `json:"context_window,omitempty"`
}
```

Extend `SessionSummaryRecord`:

```go
type SessionSummaryRecord struct {
    // ... existing fields ...
    PeakUtilization   *float64 `json:"peak_utilization,omitempty"`
    RemainingCapacity *int     `json:"remaining_capacity,omitempty"`
    DepletionRate     *float64 `json:"depletion_rate,omitempty"`
    MaxContextTokens  *int     `json:"max_context_tokens,omitempty"`
}
```

Extend SQLite `telemetry_sessions` schema. Since the SQLite cache is ephemeral
(Principle VII) and rebuilt via `RehydrateTelemetry`, update the `CREATE TABLE`
statement in `EnsureTelemetrySchema` to include the new columns directly rather
than using ALTER TABLE migrations:

```sql
-- Add to existing CREATE TABLE IF NOT EXISTS telemetry_sessions:
peak_utilization    REAL,
remaining_capacity  INTEGER,
depletion_rate      REAL,
max_context_tokens  INTEGER
```

Update rehydration code in `internal/db/telemetry_schema.go`:
* Extend `rawTelemetryRecord` struct with new context window fields for JSON
  decoding from `telemetry-sessions.jsonl`
* Update the `INSERT` statement in `RehydrateTelemetry` to include the four new
  columns
* Update the SELECT in any read helpers to include the new columns

Wire `ComputeContextMetrics` into the correlator after session aggregation.

**Verification:**

* Unit test: `ComputeContextMetrics` with known model returns correct utilization
* Unit test: unknown model uses default limit
* Unit test: zero model calls returns nil metrics (not division by zero)
* Unit test: compaction events are counted correctly
* Integration test: harvested SessionSummaryRecord includes context window fields
* Integration test: SQLite query `SELECT peak_utilization FROM telemetry_sessions`
  returns non-null for sessions with known models

### Task 4: CLI Telemetry Report Subcommand

**Stash origin:** AB6959FC (orig E14AB8CC)
**Files:** `internal/cli/telemetry.go`, `internal/telemetry/reporter.go` (new)
**Test files:** `internal/cli/telemetry_test.go`, `internal/telemetry/reporter_test.go` (new),
`tests/contract/telemetry_tool_test.go`
**Effort:** medium
**Dependencies:** Task 1 (harvest must work), Task 3 (context window fields in
data model)

**Approach:**

New file `reporter.go` with report generation logic:

```go
// ReportOptions configures the telemetry report output.
type ReportOptions struct {
    SessionID string // filter to single session (empty = all)
    GroupBy   string // "server", "model", "session", "tool"
    Format    string // "table", "json"
}

// GenerateReport reads telemetry-sessions.jsonl and produces a formatted report.
func GenerateReport(workspacePath string, opts ReportOptions) (string, error)
```

Replace stub implementations in `telemetry.go`:

**`telemetry harvest`:**
* Accept `--since DATE` and `--force` flags
* Locate `.copilot/` via `COPILOT_HOME` env or default `.copilot`
* Open workspace DB, call `HarvestTelemetry` with options
* Print summary: sessions harvested, tool calls indexed, total tokens

**`telemetry report`:**
* Accept `--session ID`, `--by server|model|session|tool`, `--format table|json`
* Default: aggregate report across all sessions grouped by session
* Table format: aligned columns with headers (use `text/tabwriter`)
* JSON format: structured output to stdout
* Include context window metrics when available

**`telemetry list`:**
* Thin wrapper: equivalent to `report --by session --format table`

**`telemetry top`:**
* Accept `--n N` flag (default 10)
* Thin wrapper: equivalent to `report --by tool --format table` with top-N limit

Extend the MCP tool `backlogit_telemetry_harvest` handler (already updated in
Task 1 for the new signature) to parse optional `since` (string, ISO 8601) and
`force` (boolean) parameters from MCP requests and pass them through to
`HarvestTelemetry` via `HarvestOptions`. Update the contract test to cover the
new parameters.

**Verification:**

* Unit test: `GenerateReport` with table format produces aligned output
* Unit test: `GenerateReport` with JSON format produces valid JSON
* Unit test: `--by server` groups tool calls by attributed server
* Unit test: `--session` filters to single session
* Integration test: `harvest` command exits 0 and prints summary
* Integration test: `report` after harvest shows session data
* Integration test: `list` and `top` produce equivalent output to `report`
* Edge case: report with no harvested data prints informative message (not error)

## Dependency Graph

```text
Task 1 (Incremental Checkpoint)
  ↓
Task 2 (Date Filter --since) ──→ depends on Task 1 HarvestOptions struct
  ↓
Task 3 (Context Window) ────────→ independent data model extension; ordered after
  ↓                                T1/T2 to avoid merge conflicts in types.go
Task 4 (CLI Report) ────────────→ depends on T1 (working harvest), T3 (context fields)
```

Recommended execution order: T1 → T2 → T3 → T4

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Type-Safe Go | New structs use typed fields with JSON tags. `ContextWindowMetrics` uses typed float64/int, no `any`. GoDoc on all exports. |
| II. MCP Protocol Fidelity | Extended MCP tool parameters are optional; existing callers unaffected. Tool remains unconditionally visible. |
| III. Test-First Development | Every task specifies test files and verification criteria. |
| IV. Workspace Containment | `.copilot/` remains read-only. Checkpoint file in `.backlogit/` (write-only). |
| V. Structured Observability | Harvest progress logged via `slog`; checkpoint state observable via JSON file. |
| VI. Single-Binary Simplicity | No new dependencies; uses stdlib `text/tabwriter`, `encoding/json`, `os`. |
| VII. CQRS Data Architecture | Checkpoint is derived/reconstructable state (not authoritative data). JSONL remains source of truth; SQLite rebuilt via rehydration. Merge-rewrite on incremental harvest preserves single-write-path semantics. |
| VIII. Git-Friendly Persistence | Checkpoint file is gitignored (ephemeral). JSONL format unchanged. |
| IX. Agent Context Efficiency | CLI report reduces need for agents to craft raw SQL queries. |

## Risk Assessment

| Risk | Severity | Mitigation |
|---|---|---|
| Byte-offset checkpoint becomes stale if log file is truncated or rotated | Medium | Detect file size < checkpoint offset; reset to 0 with slog warning. `--force` always available as recovery. |
| Model-to-limit lookup table becomes outdated as new models ship | Low | Conservative default (200k) for unknown models. Table is a single `var` declaration, trivial to update. |
| Incremental merge produces duplicate session data | Medium | Merge by session ID (map key); existing session data is replaced by newer harvest. Unit test for merge idempotency. |
| `--since` timestamp parsing rejects valid date formats | Low | Use `time.Parse` with multiple layout attempts (RFC3339, date-only YYYY-MM-DD). |
| Large number of sessions degrades CLI report performance | Low | Report reads from JSONL (not raw logs); sub-second for thousands of sessions. |

## Requires plan hardening

No — all four tasks are well-scoped with clear implementation paths. No
external system integrations, no schema migrations requiring coordination, and
no constitutional amendments needed (checkpoint is derived state, `.copilot/`
read exception already documented in F021).

## Plan Review History

<!-- plan-review-attempt: 1 -->

* **2026-04-13**: Plan review returned ADVISORY with 1 P0 + 3 P1 + 4 P2.
  Plan revised to address all P0/P1 findings:
  - **F1 (P0)**: Moved MCP handler update to Task 1 to prevent compilation break
    between T1 and T4. Handler passes `HarvestOptions{}` for backward compat.
  - **F2 (P1)**: Added `internal/mcp/tools.go` to Task 1 file list.
  - **F3 (P1)**: Added `tests/contract/telemetry_tool_test.go` to Task 1 and
    Task 4 file lists.
  - **F4 (P1)**: Added explicit rehydration code updates (rawTelemetryRecord,
    INSERT statement) to Task 3 approach.
  - **F5 (P2)**: Replaced ALTER TABLE with CREATE TABLE update (ephemeral cache).
  - **F6 (P2)**: Added malformed timestamp handling to Task 2 verification.
  - **F7 (P2)**: Added corrupt checkpoint JSON handling to Task 1 verification.
  - **F8 (P2)**: Noted as advisory for Ship to address during implementation.
  Status changed from `draft` to `revised`.
