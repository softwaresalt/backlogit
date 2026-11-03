---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-07T00:00:00Z
    origin: .backlogit/queue/043-DL.md
    status: reviewed
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-07-telemetry-attribution-analytics-plan.md
title: Telemetry Attribution & Analytics Enhancement
---

# Telemetry Attribution & Analytics Enhancement

## Problem Frame

The telemetry subsystem (046-F/047-F) provides a functioning JSONL datastore
with incremental ingestion, session correlation, context window metrics, and a
multi-format reporter. However, three gaps limit analytical value:

1. **`SessionSummary.TokensByServer` is a dead-weight set.** The intermediate
   type in `types.go` is `map[string]string` — it records which servers were
   active but discards token counts. The JSONL record works around this with
   `ToolCallsByServer` (call counts) and the reporter estimates proportional
   tokens from call ratios, but the data model is lossy.

2. **Attribution registry is hardcoded.** `attribution.go` has a static
   `defaultPrefixes` map. Adding MCP servers requires code changes. Plan Review
   F12 from 046-F explicitly deferred YAML config override.

3. **No cross-session trending.** The reporter shows point-in-time snapshots.
   There is no way to compare token efficiency across sessions, branches, or
   time periods.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Fix `TokensByServer` to carry proportional token counts per server | 043-DL Problem Frame §1 |
| R2 | Persist proportional server tokens in JSONL and SQLite | 043-DL Chosen Direction |
| R3 | Move attribution prefixes to workspace-scoped YAML config | 043-DL Problem Frame §2, Plan Review F12 |
| R4 | Add cross-session trending CLI subcommand | 043-DL Problem Frame §3 |
| R5 | Maintain backward compatibility with existing JSONL records | 043-DL Open Questions §1 |

## Scope Boundaries

### In Scope

- Fix the `SessionSummary.TokensByServer` type from `map[string]string` to
  `map[string]int` with proportional token allocation
- Add `tokens_by_server` field to `SessionSummaryRecord` in JSONL output
- Add `tokens_by_server` column to `telemetry_sessions` SQLite table
- Move attribution prefix registry to config.yaml with hardcoded defaults
  as fallback
- Create `telemetry trend` CLI subcommand with date and branch grouping
- Update existing tests and add new test coverage

### Non-Goals

- Per-tool token attribution (deferred — heuristics unreliable without
  request-level correlation data in log format)
- Session end-time tracking (minor enhancement, not worth a unit)
- Real-time telemetry streaming or dashboards
- External monitoring system integration

### Deferred to Implementation

- JSONL backward compatibility strategy (dual-read vs force-rebuild)
- Exact trending output format (table/JSON/markdown variants)
- Whether `proportionalServerTokens()` in reporter should prefer stored values
  over re-estimation

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort.

### Unit 1: Fix `TokensByServer` data model and correlator

**Files:** `internal/telemetry/types.go`, `internal/telemetry/correlator.go`
**Test files:** `internal/telemetry/correlator_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first — update correlator_test.go assertions first
**Patterns to follow:** existing `tokensByModel map[string]int` accumulation
  in `correlator.go:60`
**Dependencies:** none

**Approach:**

1. Change `SessionSummary.TokensByServer` from `map[string]string` to
   `map[string]int` in `types.go`.
2. Update the accumulator struct in `correlator.go` from
   `tokensByServer map[string]string` to `map[string]int`.
3. Replace `a.tokensByServer[server] = server` (line 70) with proportional
   token accumulation. Use the same heuristic as
   `proportionalServerTokens()`: distribute each session's total tokens across
   servers by call-count ratio. Since the correlator processes events
   sequentially, track server call counts during accumulation, then compute
   proportional tokens in the summary-building loop.
4. Update `correlator_test.go` assertions from string equality to int equality.

**Verification:**
- `correlator_test.go` passes with int-typed `TokensByServer`
- `go vet ./internal/telemetry/...` clean
- No compilation errors in downstream consumers (`harvest.go`, `reporter.go`)

### Unit 2: Persist proportional server tokens in JSONL and SQLite

**Files:** `internal/telemetry/records.go`, `internal/telemetry/harvest.go`,
  `internal/db/telemetry_schema.go`
**Test files:** `internal/telemetry/harvest_test.go`,
  `internal/db/telemetry_schema_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `TokensByModel map[string]int` field in
  `SessionSummaryRecord`, existing `ToolCallsByServer` field wiring in
  `writeTelemetryJSONL` (harvest.go:399-400)
**Dependencies:** Unit 1

**Approach:**

1. Add `TokensByServer map[string]int` field to `SessionSummaryRecord` in
   `records.go` with JSON tag `"tokens_by_server"`.
2. Add the same field to `rawTelemetryRecord` in `db/telemetry_schema.go`.
3. In `writeTelemetryJSONL` (harvest.go), populate `rec.TokensByServer` from
   `s.TokensByServer` (the fixed map from Unit 1).
4. Add `tokens_by_server TEXT` column to `telemetry_sessions` SQLite schema
   (JSON-encoded map, same pattern as `tokens_by_model`).
5. Update SQLite rehydration to read and write the new column.
6. Backward compatibility: existing JSONL records without `tokens_by_server`
   deserialize to nil/empty map — no migration needed.

**Verification:**
- Harvest writes `tokens_by_server` to JSONL records
- SQLite `telemetry_sessions` table includes `tokens_by_server` column
- `backlogit_query_sql` can query `json_extract(tokens_by_server, '$.backlogit')`
- Existing records without the field load without errors

### Unit 3: Config-driven attribution registry

**Files:** `internal/config/schema.go`, `internal/config/loader.go`,
  `internal/telemetry/attribution.go`
**Test files:** `internal/telemetry/attribution_test.go`,
  `internal/config/config_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write attribution_test cases for config override
**Patterns to follow:** existing `FieldConfig` struct and YAML loading in
  `schema.go`, existing `defaultPrefixes` map in `attribution.go`
**Dependencies:** none (parallel with Units 1-2)

**Approach:**

1. Add `TelemetryConfig` struct to `schema.go`:
   ```go
   type TelemetryConfig struct {
       AttributionPrefixes map[string]string `yaml:"attribution_prefixes"`
   }
   ```
2. Add `Telemetry *TelemetryConfig` field to `WorkspaceConfig`.
3. In `attribution.go`, add `AttributeToolWithConfig(toolName string,
   customPrefixes map[string]string) string` that merges custom prefixes
   (higher priority) with `defaultPrefixes` and applies longest-prefix matching.
4. Keep `AttributeTool(toolName)` as backward-compatible wrapper using only
   defaults.
5. Update `Correlate()` and harvest callsites to pass config-provided prefixes
   when available.
6. Add new default entries for `graphtor-` and `adversarial-review-` servers.

**Verification:**
- Config YAML with `telemetry.attribution_prefixes` overrides defaults
- Missing config falls back to hardcoded defaults
- New servers (graphtor, adversarial-review) resolve correctly
- Existing tests pass unchanged

### Unit 4: `telemetry trend` CLI subcommand

**Files:** `internal/cli/telemetry.go`, `internal/telemetry/reporter.go`
**Test files:** `internal/cli/telemetry_test.go`,
  `internal/telemetry/reporter_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `newTelemetryTopCmd` pattern in
  `telemetry.go`, existing `formatReportTable/JSON/Markdown` pattern in
  `reporter.go`
**Dependencies:** Unit 2 (uses `TokensByServer` from JSONL records)

**Approach:**

1. Add `newTelemetryTrendCmd(cwd)` to `telemetry.go`.
2. Flags: `--by date|branch` (default: date), `--format table|json|markdown`,
   `--limit N`.
3. In `reporter.go`, add `GenerateTrendReport(workspacePath, TrendOptions)`:
   - Read `telemetry-sessions.jsonl`
   - Group sessions by date (truncate `harvested_at` to date) or by branch
   - Per group: sum tokens, count sessions, compute avg tokens/session,
     avg tokens/task, avg peak utilization
   - Sort chronologically (date) or alphabetically (branch)
4. Output columns: Group | Sessions | Total Tokens | Avg Tokens/Session |
   Avg Tokens/Task | Avg Peak Util

**Verification:**
- `backlogit telemetry trend` produces table output grouped by date
- `--by branch` groups by branch name
- `--format json` produces valid JSON array
- Empty JSONL produces informative message
- CLI reference docs regenerated

## Dependency Graph

```text
Unit 1 ──→ Unit 2 ──→ Unit 4
                         ↑
Unit 3 ─────────────────╌╌ (soft — trend uses tokens_by_server when available)
```

Unit 1 (data model) must precede Unit 2 (JSONL/SQLite persistence).
Unit 2 must precede Unit 4 (trending reads JSONL with new fields).
Unit 3 (config) is independent and can be built in parallel with Units 1-2.
Unit 4 has a soft dependency on Unit 3 (uses config-provided prefixes).

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Proportional token allocation by call-count ratio | Same heuristic the reporter already uses in `proportionalServerTokens()`; consistent and simple | Timing-based allocation (rejected: tool call timestamps don't map to model call token consumption), turn-based (rejected: no turn data in logs) |
| D2 | Add config section to existing `config.yaml` | Single config file, existing YAML loader, no new file discovery | Separate `attribution.yaml` (rejected: adds config file proliferation for a small feature) |
| D3 | `TokensByServer` as `map[string]int` stored as JSON TEXT in SQLite | Consistent with existing `tokens_by_model` pattern | Normalized table (rejected: over-engineering for a derived metric; the `telemetry_tool_usage` table already has per-tool detail) |
| D4 | Backward compatibility via nil/empty map fallback | No migration needed; `--force` rebuild populates retroactively | Schema migration (rejected: JSONL is append-only, migration would require full rewrite which `--force` already provides) |
| D5 | Trend groups by date or branch | Date shows temporal trends; branch shows feature cost comparison | Session-level only (rejected: that's what `list` already does) |

## Risks and Caveats

1. **Proportional token allocation is an estimate.** Call-count ratio does not
   perfectly correlate with token usage per server. Servers with large payloads
   (e.g., engram) may be under-attributed. This is acceptable for v1; per-tool
   attribution is deferred.

2. **SQLite schema change requires fresh table.** The `CREATE TABLE IF NOT
   EXISTS` pattern means the new column won't appear until the table is dropped
   and recreated. The harvest command already calls `EnsureTelemetrySchema`
   which uses `IF NOT EXISTS`. May need `ALTER TABLE ADD COLUMN` fallback or
   document that `--force` harvest is needed after upgrade. Follow the
   `crash-safe-delete-rename-rollback` compound learning for safe schema
   evolution.

3. **Config loading in telemetry package.** The telemetry package currently has
   no dependency on the config package. Unit 3 introduces this coupling. Keep
   it narrow: pass `map[string]string` prefixes, not the full config struct.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **YES** — JSONL record schema adds
  `tokens_by_server` field; SQLite schema adds column; `SessionSummary` type
  changes. However, all changes are additive (new optional field) and
  backward-compatible.
* security, auth, permission, or compliance-sensitive behavior: **NO**
* migration, backfill, destructive data/config action, or irreversible step:
  **NO** — JSONL records are regenerable via `--force`; SQLite is ephemeral
* external integration, operator checkpoint, or external dependency: **NO**
* high runtime, rollout, or rollback risk: **NO**

Requires plan hardening: **no** — all changes are additive, backward-compatible,
and recoverable via `--force` harvest.

## Runtime Verification and Closure

- **Unit 2 (JSONL/SQLite):** Run `backlogit telemetry harvest --force` and
  verify JSONL contains `tokens_by_server` field. Query SQLite via
  `backlogit_query_sql`. Runtime verification mode: CLI.
- **Unit 4 (trend):** Run `backlogit telemetry trend` and `--by branch`.
  Verify output matches expected format. Runtime verification mode: CLI.
- **Operational closure:** Document updated telemetry field reference.
  No monitoring plan needed (offline CLI tool, not a service).

## Learnings Applied

- `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`
  — When adding SQLite columns, check both the schema DDL and the
  rehydration/insert logic in `internal/db/`.
- `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`
  — Atomic file writes for JSONL already use temp-then-rename pattern.
- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`
  — Must regenerate `docs/cli-reference/` after adding the `trend` subcommand.

## Standards Check

- All new code follows Go 1.22+ conventions with GoDoc comments on exports.
- Test-first development: every unit specifies test files and verification.
- `golangci-lint` must pass with zero warnings.
- Struct validation tags on new config fields.
- Sentinel errors from `internal/errors/` for validation failures.
- Single-domain units: no mixing of CLI work with data model work.

## Plan Review

**Gate: PASS** (advisory findings only)

**Date:** 2026-05-07
**Reviewers:** constitution-reviewer, go-quality-reviewer, scope-boundary-auditor,
learnings-researcher, architecture-strategist, agent-native-parity-reviewer

### P0 — Critical

None.

### P1 — High

None.

### P2 — Moderate (addressed inline)

**F1: SQLite schema evolution strategy (Risk 2).** The plan identifies the
risk but does not commit to an approach. **Resolution:** Use `ALTER TABLE
ADD COLUMN` with a column-existence check. The harvest `EnsureTelemetrySchema`
function should check for the `tokens_by_server` column and add it if missing.
This avoids requiring a full table drop-and-recreate on upgrade.

**F2: Reporter dual-path decision.** The plan defers whether
`proportionalServerTokens()` should prefer stored `tokens_by_server` values
over re-estimation. **Resolution:** Always use `ToolCallsByServer` (call
counts) for proportional estimation, as the reporter already does. The stored
`tokens_by_server` field is for direct query access via SQLite, not for
changing the reporter's existing estimation logic. This eliminates the
dual-path complexity.

### P3 — Low (advisory)

**F3:** Unit 1 approach is correct but the two-pass strategy (count calls
during event processing, compute proportional tokens in the summary loop)
should be made explicit in the implementation.

**F4:** Unit 3 should validate config-provided attribution prefixes (non-empty
key and value) during config loading.

**F5:** Consider adding `--since` flag to the trend command for time-bounded
trending. Can be deferred to a follow-up.

### Next Steps

Proceed to `harvest` — plan passes the review gate.
