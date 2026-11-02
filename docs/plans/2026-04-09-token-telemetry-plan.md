---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-09T00:00:00Z
    origin: .backlogit/queue/005-DL.md
    status: revised
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-09-token-telemetry-plan.md
title: Token and Context Window Efficiency Telemetry
---

## Problem Frame

AI agents operating through MCP tool servers consume tokens and context window
capacity with every interaction. No mechanism exists to measure the relative
efficiency of different tool combinations, harness configurations, or agent
workflows within a project workspace.

The MCP protocol does not expose token usage to tool servers. However, Copilot
CLI's `COPILOT_HOME` environment variable (set via `cpstart.ps1`) scopes all
telemetry data to `.copilot/` within the workspace. Process logs with full
token counts, session event JSONL with per-turn output tokens and compaction
metrics, and a SQLite session store with session metadata all reside inside
the workspace and are captured automatically.

The missing piece is a harvester that reads this workspace-scoped Copilot
telemetry, correlates it with backlogit task completions, attributes tool calls
to their originating MCP servers, and exposes metrics through backlogit's
existing SQL query surface.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Parse `cli.model_call` events from process logs for token counts per model call | 005-DL Chosen Direction, Data Sources |
| R2 | Parse `cli.tool_call` events for tool attribution and duration | 005-DL Chosen Direction, Tool Attribution |
| R3 | Correlate model calls to tool calls via `model_call_id` | 005-DL Chosen Direction, Correlation Keys |
| R4 | Attribute tool calls to MCP servers via prefix registry | 005-DL Chosen Direction, Tool Attribution |
| R5 | Link session token usage to backlogit task completions | 005-DL Chosen Direction, Pipeline Stage 4 |
| R6 | Store harvested metrics in SQLite for query via `backlogit_query_sql` | 005-DL Chosen Direction, MCP Surface |
| R7 | Expose `backlogit telemetry harvest` CLI command | 005-DL Chosen Direction, CLI Surface |
| R8 | Expose `backlogit telemetry report` CLI command | 005-DL Chosen Direction, CLI Surface |
| R9 | Isolate Copilot log parser behind adapter interface | 005-DL Open Question 1 |
| R10 | Define telemetry sentinel errors in `internal/errors/` | Plan Review F11 |
| R11 | GoDoc on all exported symbols; lint/vet/gofmt clean | Plan Review F10 |

## Scope Boundaries

### In Scope

* Copilot CLI process log parser (`cli.model_call`, `cli.tool_call` events)
* Session event parser (compaction metrics from `events.jsonl`)
* Session metadata reader (branch, repository, timing from `session-store.db`)
* Correlation engine joining model calls, tool calls, and sessions
* Tool attribution registry (prefix-based MCP server mapping, hardcoded v1)
* SQLite telemetry tables with rehydration from dedicated JSONL stream
* CLI subcommands: `harvest` and `report`
* MCP tool: `backlogit_telemetry_harvest`
* Contract tests for log format parsing

### Containment Note (Plan Review F1)

The constitution's Principle IV restricts file-system operations to
`.backlogit/`. This feature requires **read-only** access to `.copilot/` for
telemetry ingestion. This is a documented constitutional exception:

* `.copilot/` is workspace-scoped (set via `COPILOT_HOME` in `cpstart.ps1`)
* Access is strictly read-only; all writes go to `.backlogit/`
* The parser adapter isolates `.copilot/` access behind an interface boundary
* A constitution amendment (Principle IV addendum for read-only workspace data
  sources) should be proposed alongside the implementation PR

### Non-Goals

* VS Code Copilot Chat log parsing (different format, needs a future spike)
* Real-time streaming telemetry (batch harvest only)
* Token budget enforcement or alerting
* BS Buster integration (complementary, not overlapping)
* `compare` and `trend` CLI subcommands (defer to a follow-up feature)

### Deferred to Implementation

* Exact regex patterns for log line parsing (depends on format validation
  against multiple log files during test-first development)
* Token attribution strategy when one model turn invokes tools from multiple
  MCP servers (start with proportional split by tool count; revisit after real
  data analysis)

### Deferred to Future Feature (Plan Review F2)

* Harvest checkpoint with byte offsets and file tracking (YAGNI for v1; full
  re-harvest is acceptable until proven too slow with real-world data)
* Advisory file locking for concurrent harvest (defer until checkpoint exists)
* `--since` date filter for harvest (v1 harvests all available logs)
* `--force` flag (irrelevant without checkpoint)

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units
target a single skill domain and specify a verifiable exit state.

### Unit 1: Copilot Process Log Parser

**Files:** `internal/telemetry/parser.go`, `internal/telemetry/types.go`
**Test files:** `internal/telemetry/parser_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write contract tests against captured log
samples before implementing the parser
**Patterns to follow:** `internal/stash/jsonl.go` (line-by-line JSONL reader),
`internal/events/stream.go` (file reader pattern)
**Dependencies:** none

**Approach:**

Define an adapter interface `LogParser` with a streaming callback boundary
(Plan Review F6). Instead of accumulating all events in memory, the parser
emits normalized events via a callback:

```go
// TelemetryEvent is the normalized event emitted by a LogParser.
type TelemetryEvent struct {
    Kind      EventKind // ModelCall or ToolCall
    ModelCall *ModelCall
    ToolCall  *ToolCall
}

// EventKind distinguishes telemetry event types.
type EventKind string

const (
    EventModelCall EventKind = "model_call"
    EventToolCall  EventKind = "tool_call"
)

// LogParser reads host-specific telemetry logs and emits normalized events.
type LogParser interface {
    Parse(r io.Reader, emit func(TelemetryEvent) error) error
}
```

Implement `CopilotCLIParser` behind this interface.

The parser reads process log files line by line, matching the pattern
`[INFO] [Telemetry] cli.model_call:` followed by a JSON block. Key fields
to extract from `cli.model_call`:

* `prompt_tokens_count`, `completion_tokens_count`, `total_tokens_count`,
  `cached_tokens_count`
* `model`, `duration_ms`, `session_id`, `request_id`, `api_id`
* `created_at` (timestamp)

From `cli.tool_call`:

* `tool_name`, `tool_call_id`, `result_type`, `duration_ms`
* `model_call_id` (correlation to parent model call)
* `session_id`, `created_at`

Types:

```go
type ModelCall struct {
    APIId              string    `json:"api_id"`
    Model              string    `json:"model"`
    PromptTokens       int       `json:"prompt_tokens_count"`
    CompletionTokens   int       `json:"completion_tokens_count"`
    TotalTokens        int       `json:"total_tokens_count"`
    CachedTokens       int       `json:"cached_tokens_count"`
    DurationMs         int       `json:"duration_ms"`
    SessionID          string    `json:"session_id"`
    RequestID          string    `json:"request_id"`
    CreatedAt          time.Time `json:"created_at"`
}

type ToolCall struct {
    ToolName     string    `json:"tool_name"`
    ToolCallID   string    `json:"tool_call_id"`
    ResultType   string    `json:"result_type"`
    DurationMs   int       `json:"duration_ms"`
    ModelCallID  string    `json:"model_call_id"`
    SessionID    string    `json:"session_id"`
    CreatedAt    time.Time `json:"created_at"`
}
```

**Verification:**

* Contract tests parse a captured log sample and assert correct field
  extraction for at least 3 model calls and 5 tool calls
* Malformed lines are skipped with a warning, not fatal
* Parser returns empty slices for files with no telemetry events

### Unit 2: Session Metadata Reader

**Files:** `internal/telemetry/session_events.go`, `internal/telemetry/session_store.go`
**Test files:** `internal/telemetry/session_events_test.go`, `internal/telemetry/session_store_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/stash/jsonl.go` (JSONL reader), `internal/db/connection.go` (SQLite read)
**Dependencies:** Unit 1 (shared types in `types.go`)

**Approach:**

Two data sources provide session metadata (Plan Review F3):

1. **Session events JSONL**: Parse `.copilot/session-state/*/events.jsonl` for
   compaction events (`session.compaction_complete`) containing
   `preCompactionTokens`, `compactionTokensUsed.*`. Also extract `session.start`
   events for start time and model.

2. **Session store DB**: Read `.copilot/session-store.db` (read-only) for
   authoritative session metadata: `branch`, `repository`, `created_at`,
   `updated_at`, `summary`. This is the primary source for session timing
   boundaries used in per-task token attribution.

Define a `CompactionEvent` struct, a `SessionMeta` struct, and a
`SessionStoreReader` that opens the session-store DB read-only. The parser
walks `session-state/*/events.jsonl` files, and the store reader queries for
session rows. Results merge into a unified session metadata map keyed by
session ID.

**Verification:**

* Parse a captured `events.jsonl` sample and assert compaction fields
* Missing compaction events yield an empty slice (not an error)
* Session metadata correctly extracted from `session.start`
* Session-store.db reader extracts branch, repository, and timing for known sessions
* Graceful fallback when session-store.db is missing (use event-only metadata)

### Unit 3: Tool Attribution Registry

**Files:** `internal/telemetry/attribution.go`
**Test files:** `internal/telemetry/attribution_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/config/loader.go` (YAML config loading)
**Dependencies:** none

**Approach:**

Implement a prefix-based registry that maps tool names to MCP server names:

| Prefix/Pattern | Server |
|---|---|
| `backlogit_*` | backlogit |
| `engram-*` | engram |
| `agent-intercom-*` | agent-intercom |
| `github-*` | github |
| `tavily-*` | tavily |
| `context7-*` | context7 |
| `microsoft-docs-*` | microsoft-docs |
| `view`, `edit`, `create`, `glob`, `grep`, `powershell`, `task`, `report_intent`, `ask_user`, `show_file`, `store_memory`, `skill`, `sql` | copilot_builtin |

The registry is a simple ordered slice of `(prefix, server)` tuples with
longest-prefix-first matching. Load defaults from code; YAML config override
is deferred to a future version (Plan Review F12).

**Verification:**

* All known tool names resolve to the correct server
* Unknown tool names resolve to `"unknown"`

### Unit 4: Correlation Engine

**Files:** `internal/telemetry/correlator.go`
**Test files:** `internal/telemetry/correlator_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** none (new domain logic)
**Dependencies:** Unit 1, Unit 2, Unit 3

**Approach:**

The correlator joins parsed data across three dimensions:

1. **Model-to-tool**: `ToolCall.ModelCallID` → `ModelCall.APIId`
2. **Session aggregation**: group all calls by `SessionID`
3. **Tool attribution**: apply the attribution registry to each `ToolCall`

Output: `SessionSummary` struct per session containing:

* Total prompt/completion/cached tokens
* Token counts grouped by model
* Token counts grouped by MCP server (via attribution)
* Tool call counts and success rates per server
* Compaction events (from Unit 2)
* Duration statistics

The correlator also links to backlogit task completions by scanning
`.backlogit/events.jsonl` for `status_changed` events where the new status
is `done`, extracting the item ID, actor, and timestamp (Plan Review F9:
use the canonical event stream, not internal log files). Sessions are mapped
to tasks by time overlap (session start/end contains task completion timestamp).

**Verification:**

* Given 3 model calls and 8 tool calls across 2 sessions, correctly groups by
  session and attributes to servers
* Token-per-task computed when backlogit events overlap the session window
* Sessions with no task completions report `tokens_per_task: null`

### Unit 5: SQLite Telemetry Tables and Rehydration

**Files:** `internal/db/telemetry_schema.go`, `internal/db/telemetry_queries.go`
**Test files:** `internal/db/telemetry_schema_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/db/schema.go` (EnsureSchema pattern),
`internal/db/rehydration.go` (transactional rehydration), `internal/db/stash.go`
(upsert pattern)
**Dependencies:** Unit 4

**Approach:**

Add telemetry tables to the SQLite schema, following the existing migration
pattern in `EnsureSchema`:

```sql
CREATE TABLE IF NOT EXISTS telemetry_sessions (
    session_id          TEXT PRIMARY KEY,
    model               TEXT NOT NULL,
    branch              TEXT,
    repository          TEXT,
    start_time          DATETIME,
    end_time            DATETIME,
    total_prompt_tokens  INTEGER NOT NULL DEFAULT 0,
    total_completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_cached_tokens  INTEGER NOT NULL DEFAULT 0,
    total_duration_ms    INTEGER NOT NULL DEFAULT 0,
    model_call_count     INTEGER NOT NULL DEFAULT 0,
    tool_call_count      INTEGER NOT NULL DEFAULT 0,
    compaction_count     INTEGER NOT NULL DEFAULT 0,
    tasks_completed      INTEGER NOT NULL DEFAULT 0,
    tokens_per_task      REAL,
    harvested_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS telemetry_tool_usage (
    session_id      TEXT NOT NULL REFERENCES telemetry_sessions(session_id),
    server_name     TEXT NOT NULL,
    tool_name       TEXT NOT NULL,
    call_count      INTEGER NOT NULL DEFAULT 0,
    success_count   INTEGER NOT NULL DEFAULT 0,
    total_duration_ms INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (session_id, server_name, tool_name)
);

CREATE INDEX IF NOT EXISTS idx_telemetry_tool_server
    ON telemetry_tool_usage(server_name);
```

Plan Review F7: `telemetry_tool_usage` uses a composite primary key
`(session_id, server_name, tool_name)` for deterministic rebuilds and
idempotent upserts. No AUTOINCREMENT.

Provide `RehydrateTelemetry` that clears and rebuilds all telemetry tables
from `.backlogit/telemetry-sessions.jsonl` (Plan Review F5: single write
path — harvest writes only to JSONL, rehydration rebuilds SQLite). No direct
SQLite upserts during harvest.

**Verification:**

* Schema creates without error on a fresh database
* Rehydration from JSONL is idempotent (running twice produces identical data)
* `backlogit_query_sql` can query telemetry tables (read-only gate allows
  SELECT against new tables)

### Unit 6: Harvest Pipeline and Typed JSONL Output

**Files:** `internal/telemetry/harvest.go`, `internal/telemetry/records.go`
**Test files:** `internal/telemetry/harvest_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/stash/jsonl.go` (typed JSONL writer/reader),
`internal/db/rehydration.go` (transactional indexing)
**Dependencies:** Unit 4, Unit 5

**Approach:**

The harvest pipeline orchestrates: parse → correlate → attribute → persist.

Plan Review F4 + F5: Define concrete typed records in `records.go`:

```go
// SessionSummaryRecord is the durable JSONL record for a harvested session.
type SessionSummaryRecord struct {
    SessionID            string    `json:"session_id"`
    Model                string    `json:"model"`
    Branch               string    `json:"branch,omitempty"`
    Repository           string    `json:"repository,omitempty"`
    StartTime            time.Time `json:"start_time"`
    EndTime              time.Time `json:"end_time,omitempty"`
    TotalPromptTokens    int       `json:"total_prompt_tokens"`
    TotalCompletionTokens int      `json:"total_completion_tokens"`
    TotalCachedTokens    int       `json:"total_cached_tokens"`
    TotalDurationMs      int       `json:"total_duration_ms"`
    ModelCallCount       int       `json:"model_call_count"`
    ToolCallCount        int       `json:"tool_call_count"`
    CompactionCount      int       `json:"compaction_count"`
    TasksCompleted       int       `json:"tasks_completed"`
    TokensPerTask        *float64  `json:"tokens_per_task,omitempty"`
    ToolUsage            []ToolUsageRecord `json:"tool_usage"`
    HarvestedAt          time.Time `json:"harvested_at"`
}

// ToolUsageRecord is an embedded record for per-tool metrics.
type ToolUsageRecord struct {
    ServerName    string `json:"server_name"`
    ToolName      string `json:"tool_name"`
    CallCount     int    `json:"call_count"`
    SuccessCount  int    `json:"success_count"`
    TotalDurationMs int  `json:"total_duration_ms"`
}
```

Both JSONL persistence and SQLite rehydration (Unit 5) consume these same
types, eliminating the dual-write path and hidden dependency cycle.

Pipeline steps:

1. Discover `.copilot/logs/process-*.log` files
2. Parse all log files via the `LogParser` streaming interface
3. Parse session events and metadata from `.copilot/session-state/`
4. Read session-store.db for authoritative session timing (Unit 2)
5. Run the correlator to produce `SessionSummaryRecord` results
6. Write summaries to `.backlogit/telemetry-sessions.jsonl` (dedicated file,
   not the existing `telemetry.jsonl` which is for agent telemetry)
7. Trigger `RehydrateTelemetry` to rebuild SQLite from the JSONL stream

Plan Review F2: No checkpoint file in v1. Full re-harvest on each invocation.
The JSONL file is overwritten (not appended) since the parser re-processes all
available logs. Checkpoint and incremental harvest are deferred.

**Verification:**

* Harvesting a workspace with 2 log files produces correct session summaries
* Re-harvesting produces identical JSONL output (deterministic)
* Harvested data is queryable via SQL after rehydration
* Missing `.copilot/` returns descriptive error, not panic

### Unit 7: CLI Subcommands

**Files:** `internal/cli/telemetry.go`
**Test files:** `internal/cli/telemetry_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/cli/stash.go` (subcommand group pattern),
`internal/cli/root.go` (AddCommand registration)
**Dependencies:** Unit 6

**Approach:**

Register `NewTelemetryCmd(&cwd)` in `root.go` as a command group with two
subcommands:

`backlogit telemetry harvest`

* No flags in v1 (full re-harvest; checkpoint deferred per Plan Review F2)
* Output: summary of sessions harvested, total tokens indexed

`backlogit telemetry report [--session <id>] [--by server|model|session|tool]`

* Default: aggregate report across all harvested sessions
* `--session`: filter to a specific session
* `--by`: grouping dimension (Plan Review F13: preserve `--by tool` from
  origin doc; defer `--format table|json` to follow-up)
* Metrics: total tokens, tokens per task, cache efficiency, tool call
  distribution by server

**Verification:**

* `harvest` command exits 0 and prints summary on a workspace with log files
* `report` command produces valid JSON when `--format json` is specified
* Commands fail gracefully with a descriptive message when `.copilot/` does
  not exist

### Unit 8: MCP Tool

**Files:** `internal/mcp/tools.go` (add to existing tool registration)
**Test files:** `tests/contract/telemetry_tool_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/mcp/tools.go` (five-step handler pattern),
`handleLogTelemetry` handler
**Dependencies:** Unit 6

**Approach:**

Register `backlogit_telemetry_harvest` as an MCP tool:

```go
mcplib.NewTool("backlogit_telemetry_harvest",
    mcplib.WithDescription("Harvest Copilot token telemetry from workspace logs"),
)
```

No required parameters in v1 (checkpoint and date filters deferred).

Handler follows the five-step pattern: require workspace → validate `.copilot/`
exists → call harvest pipeline → trigger rehydration → return JSON summary.

After harvest, agents query telemetry data via existing `backlogit_query_sql`:

```sql
SELECT session_id, model, total_prompt_tokens, tokens_per_task
FROM telemetry_sessions
ORDER BY start_time DESC
LIMIT 10
```

**Verification (Plan Review F8, F14):**

* Contract test: tool appears in `ListTools()` before workspace init
  (unconditional visibility per Principle II)
* Contract test: tool returns descriptive `workspace_not_initialized` error
  before init, not tool absence
* Contract test: tool returns structured JSON with session count and token totals
* Contract test: tool returns descriptive error when `.copilot/` does not exist
* Edge cases: empty sessions (zero model calls), corrupt/truncated JSON lines,
  no compaction events, partial `.copilot/` directory structure

## Dependency Graph

```text
Unit 1 (Process Log Parser)
  ↓
Unit 2 (Session Event Parser) ──→ depends on Unit 1 types
  ↓
Unit 3 (Attribution Registry) ──→ independent, can parallel with 1-2
  ↓
Unit 4 (Correlation Engine) ────→ depends on Units 1, 2, 3
  ↓
Unit 5 (SQLite Tables) ────────→ depends on Unit 4 types
  ↓
Unit 6 (Harvest Pipeline) ─────→ depends on Units 4, 5
  ↓                  ↓
Unit 7 (CLI)    Unit 8 (MCP) ──→ both depend on Unit 6, can parallel
```

Recommended execution order: 1 → 3 (parallel) → 2 → 4 → 5 → 6 → 7 + 8
(parallel)

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Adapter interface for log parser | Copilot CLI log format is undocumented and may change. Interface isolation limits blast radius to one implementation file. Future non-Copilot hosts (Claude Code, Cursor) get their own adapter. | Direct parsing without interface (rejected: couples core to format) |
| D2 | Prefix-based tool attribution | Tool names follow consistent `{server}_` naming. Simple, fast, and handles all observed tools. | MCP server config parsing (rejected: requires reading MCP config files and understanding server registration), per-call attribution via MCP protocol (rejected: protocol doesn't support this) |
| D3 | Session-level token-to-task linking via time overlap | No per-turn task tracking exists. Session start/end bracketing task completion timestamps is the coarsest reliable signal. | Per-turn attribution (rejected: requires backlogit to know which task each turn belongs to; no data source for this), even split across all workspace tasks (rejected: too imprecise) |
| D4 | Full re-harvest for v1, defer checkpoint (Plan Review F2) | Process logs total ~50-200MB; full parse completes in seconds. Checkpoint/offset tracking adds constitutional exceptions (mutable state) and complexity without proven need. | Byte-offset checkpoint (rejected for v1: YAGNI, violates CQRS), timestamp watermark (acceptable future step) |
| D5 | Defer `compare` and `trend` subcommands | Core value is in `harvest` + `report` + SQL query. Comparative and trending analysis can be built on the SQL surface without new code. | Ship all four subcommands at once (rejected: scope creep, `compare` and `trend` can be SQL queries) |
| D6 | New `internal/telemetry/` package | Telemetry harvesting is a distinct domain from backlogit's existing event streaming (`internal/events/`). Separate package prevents coupling and keeps the parser adapter boundary clean. | Extend `internal/events/` (rejected: conflates output telemetry with input harvesting) |
| D7 | Hyphen-delimited tool prefixes like `engram-*`, `agent-intercom-*` | Real Copilot tool names use hyphens for MCP server prefixes (e.g., `engram-query_memory`, `agent-intercom-broadcast`), not underscores | Assume all prefixes use underscores (rejected: doesn't match observed data) |
| D8 | Dedicated `telemetry-sessions.jsonl` (Plan Review F4) | Existing `telemetry.jsonl` stores generic agent telemetry with `map[string]any` payloads. Mixing harvested session summaries would cause schema collision and type-safety violations. | Reuse TelemetryWriter (rejected: schema collision, Principle I violation) |
| D9 | Single write path: JSONL → rehydrate → SQLite (Plan Review F5) | Dual write (direct upsert + rehydration) creates competing write paths and hidden dependency cycles. CQRS requires JSONL as durable source, SQLite as ephemeral cache. | Direct SQLite upsert during harvest (rejected: violates CQRS, creates dual-write inconsistency) |
| D10 | Streaming parser callback interface (Plan Review F6) | Full in-memory accumulation for 50MB+ files risks memory pressure and prevents future checkpoint-aware parsing. | `Parse() ([]ModelCall, []ToolCall)` return style (rejected: full accumulation, inflexible) |
| D11 | Hardcoded attribution registry for v1 (Plan Review F12) | YAML config override is premature flexibility before default mappings are validated with real data. | YAML override on v1 (rejected: premature) |

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Copilot CLI log format changes without notice | High | Contract tests against captured samples; adapter interface limits blast radius to one file |
| Large log files (50MB+) cause memory pressure | Medium | Streaming callback parser; never accumulate full file into memory |
| Session-to-task attribution is imprecise | Medium | Document limitation; provide raw per-session data for users who need exact attribution |
| `.copilot/` directory may not exist (no `cpstart.ps1`) | Low | Fail gracefully with descriptive error via `ErrTelemetrySourceMissing` sentinel |
| `session-store.db` schema changes in future Copilot versions | Low | Read-only access with graceful fallback to event-only metadata when DB unavailable |
| Full re-harvest becomes slow as log volume grows | Low | Acceptable for v1; checkpoint system designed for future addition when needed |

## Learnings Applied

* **Atomic SQLite rehydration**
  (`docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`):
  wrap telemetry table population in a transaction; use `Tx` suffix naming for
  transactional helpers

* **F015 shipment/stash patterns**
  (`docs/compound/go-patterns/f015-shipment-stash-patterns.md`): follow JSONL
  append-only patterns for `telemetry-sessions.jsonl` output; handle JSON
  round-trip type normalization for SQLite storage

## Standards Check

| Principle | Compliance |
|---|---|
| I. Type-Safe Go | All structs use typed fields with JSON tags; no `any` in public API. GoDoc on all exports; `golangci-lint`, `go vet`, `gofmt` in final verification (Plan Review F10). Telemetry sentinel errors in `internal/errors/` (Plan Review F11). |
| II. MCP Protocol Fidelity | New tool unconditionally visible; returns descriptive error before workspace init. Contract tests for unconditional registration (Plan Review F8). |
| III. Test-First Development | Every unit specifies test files and verification criteria first. Edge-case tests for corrupt JSON, empty sessions, missing data (Plan Review F14). |
| IV. Workspace Containment | **Documented exception**: read-only access to `.copilot/` for telemetry ingestion. All writes to `.backlogit/`. Constitution amendment proposed alongside implementation (Plan Review F1). |
| V. Structured Observability | Harvest progress logged via `slog`; results stored in `telemetry-sessions.jsonl` |
| VI. Single-Binary Simplicity | No new dependencies; uses stdlib `bufio`, `encoding/json`, `database/sql` |
| VII. CQRS Data Architecture | Dedicated `telemetry-sessions.jsonl` is append-only durable stream (Plan Review F4). SQLite is ephemeral query cache rebuilt via rehydration only (Plan Review F5). No mutable checkpoint file (Plan Review F2). |
| VIII. Git-Friendly Persistence | `telemetry-sessions.jsonl` is human-readable and Git-friendly; `index.db` is gitignored. No mutable state files. |
| IX. Agent Context Efficiency | Agents query via `backlogit_query_sql` (50 tokens) instead of parsing raw logs |

## Prerequisites

* `cpstart.ps1` must set `COPILOT_HOME=.copilot` so telemetry data is
  workspace-scoped
* At least one Copilot CLI session must have generated process logs in
  `.copilot/logs/`

## Plan Review History

* **2026-04-09**: Multi-persona review (Constitution, Go Quality, Architecture
  Strategist, Scope Boundary Auditor) returned FAIL with 1 P0 + 6 P1 + 7 P2.
  Plan revised to address all findings. Key changes: documented constitutional
  exception for `.copilot/` reads (F1), deferred checkpoint system (F2), added
  session-store.db reader (F3), dedicated typed JSONL stream (F4), single
  write path JSONL → rehydrate (F5), streaming parser interface (F6), composite
  primary key for tool usage (F7). See
  `.copilot-tracking/plan-review/2026-04-09-token-telemetry-plan-review.md`.
