---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-25T00:00:00Z
    origin: .backlogit/queue/041-DL.md
    stash_ids:
        - D001CBE0
        - 144CA2BB
        - 736ABA8A
        - 1FB3E504
        - 6DE63CCD
    status: reviewed
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-25-telemetry-quality-plan.md
title: Telemetry Quality — Parser Fix & Documentation
---

# Telemetry Quality — Parser Fix & Documentation

## Problem Frame

The telemetry subsystem has four outstanding quality gaps remaining after
shipments 043-S and 044-F. These gaps span a parser bug, a CLI behavior defect,
and two documentation omissions that make the telemetry output unreliable or
opaque to consumers.

**Scope boundary**: This plan covers only the telemetry parser, reporter, and
documentation surfaces. It does not change the harvest pipeline's core
architecture, the session correlation engine, or the context-window computation
logic.

(see origin: .backlogit/queue/041-DL.md)

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Oversized Copilot log entries must not produce partial zero-token sessions | stash 144CA2BB |
| R2 | `telemetry top` must rank results by token usage, not server-level call counts | stash 736ABA8A |
| R3 | Document that `telemetry harvest` rehydrates SQLite telemetry tables as a side effect | stash 1FB3E504 |
| R4 | Document harvested telemetry fields: tokens_by_model, tool_calls_by_server, completed_tasks, tokens_per_task, and context-window metrics | stash 6DE63CCD |

## Scope Boundaries

### In Scope

- Fix bufio.Scanner token-too-long handling in `internal/telemetry/parser.go`
- Fix `telemetry top` reporting metric in `internal/cli/telemetry.go` and `internal/telemetry/reporter.go`
- Update `docs/cli-reference/backlogit_telemetry_harvest.md` to document SQLite rehydration
- Create or update telemetry field reference documentation
- Unit tests for both code fixes

### Non-Goals

- Refactoring the harvest pipeline architecture
- Changing the JSONL schema or SessionSummaryRecord format
- Adding new telemetry metrics or context-window fields
- Modifying the incremental checkpoint system
- Singleton MCP server transport (stash 21E17BFC, deferred)

### Deferred to Implementation

- Exact bufio.Scanner replacement strategy: increase MaxScanTokenSize vs
  switch to bufio.NewReader with manual line reading. The implementer should
  evaluate based on the actual log entry sizes observed and Go stdlib behavior.
  Either approach is acceptable as long as the test harness confirms oversized
  entries are handled gracefully.

## Implementation Units

### Unit 1: Fix bufio.Scanner Token-Too-Long Handling

**Files:**
- `internal/telemetry/parser.go` (lines 62-136)
- `internal/telemetry/harvest.go` (lines 231-265, secondary Scanner instance)

**Test files:**
- `internal/telemetry/parser_test.go` (new or existing test for oversized entries)

**Effort size:** small
**Skill domain:** code
**Execution note:** test-first — write a test that feeds an oversized log entry
(>1MB line) to the parser and asserts the session is either skipped cleanly or
handled without producing a partial zero-token result.

**Patterns to follow:**
- Existing `scanner.Buffer(make([]byte, 1024*1024), 1024*1024)` pattern at
  `parser.go:63-64`
- Error handling pattern at `parser.go:88-90` (debug log and skip)
- Atomic write patterns from `docs/compound/crash-safe-delete-rename-rollback-go-2026-04-23.md`

**Dependencies:** None — this unit is independent.

**Approach:**

The current 1MB buffer is likely insufficient for verbose Copilot sessions.
Two viable strategies:

1. **Increase buffer size** to 10MB: `scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)`.
   Simplest change, preserves Scanner semantics. Risk: single-line entries
   exceeding 10MB are still dropped.
2. **Switch to bufio.NewReader**: Use `ReadString('\n')` or `ReadBytes('\n')`
   which do not have a maximum token size. More robust but requires refactoring
   the scan loop.

**Recommendation:** Use option 2 (bufio.NewReader) as the primary approach.
The Scanner has a hard maximum token size regardless of buffer configuration,
and real-world Copilot logs can exceed any fixed limit. Replace
`bufio.NewScanner` with `bufio.NewReader` + `ReadString('\n')` in both
`parser.go:63` and `harvest.go:231`. This eliminates the token-too-long
failure mode entirely.

Additionally, add a **session validation step**: after correlation, reject
any `SessionSummary` where `TotalTokens == 0 && ToolCalls > 0`. These are
partial sessions caused by dropped log entries and must not be written to
JSONL. Log skipped sessions at WARN level (not DEBUG) with session ID and
reason.

When a line read error occurs, the parser must:
- Log a structured warning (slog.Warn) with the file path and byte offset
- Skip the problematic entry cleanly (not produce a partial session)
- Continue processing subsequent entries

**Verification:**
- Test: oversized entry (>1MB) does not produce a partial zero-token session
- Test: oversized entry is logged and skipped, subsequent entries parse normally
- Test: normal entries (<1MB) continue to parse correctly (regression guard)
- Test: session with `TotalTokens==0 && ToolCalls>0` is rejected and not written
- Test: old `[telemetry]` format entries continue to parse (backward compat)
- `go test ./internal/telemetry/...` passes

### Unit 2: Fix Telemetry Top Command Ranking

**Files:**
- `internal/cli/telemetry.go` (lines 90-110)
- `internal/telemetry/reporter.go` (lines 178-202, `formatServerTable()`)

**Test files:**
- `internal/telemetry/reporter_test.go` (new or existing test for token-based ranking)

**Effort size:** small
**Skill domain:** code
**Execution note:** test-first — write a test with sessions where server A has
more tool calls but server B has more tokens, and assert the top output ranks
B above A.

**Patterns to follow:**
- Existing `formatServerTable()` aggregation pattern at `reporter.go:181-183`
- `SessionSummary.TotalTokens` field at `types.go:84`
- `SessionSummaryRecord.ToolCallsByServer` at `records.go:27`

**Dependencies:** None — independent of Unit 1.

**Approach:**

The `formatServerTable()` function currently aggregates
`ToolCallsByServer[server] += count`. It must instead aggregate token usage per
server. The `SessionSummaryRecord` carries `TokensByServer` (attribution set,
not counts) and per-session `TotalTokens`.

**Use proportional session-level attribution**: `ToolUsageRecord` carries
`CallCount` and `TotalDurMs` but no per-tool token field. Per-tool token
attribution is not available in the current data model, and adding it is a
non-goal of this plan. Instead, attribute a session's `TotalTokens` to each
server proportionally based on its share of tool calls:

```
tokens_per_server[s] = session.TotalTokens × (ToolCallsByServer[s] / total_tool_calls)
```

This is an approximation that treats all tool calls as consuming tokens equally
within a session. It is adequate for ranking and context-window management use
cases.

Update `formatServerTable()` in `reporter.go` to compute and sort by
proportional token attribution instead of raw call counts. Update the CLI
`telemetry.go` to pass the correct sort-by option. Ensure the limit is applied
after sorting by the token metric (not alphabetically).

**Verification:**
- Test: sessions with different token/call ratios rank correctly by tokens
- Test: output format includes token counts (not just call counts)
- `go test ./internal/telemetry/...` passes
- `go test ./internal/cli/...` passes

### Unit 3: Document Telemetry Harvest Side Effects

**Files:**
- `docs/cli-reference/backlogit_telemetry_harvest.md`

**Test files:** N/A (documentation only)

**Effort size:** small
**Skill domain:** docs
**Execution note:** documentation — update existing CLI reference

**Patterns to follow:**
- Existing CLI reference format in `docs/cli-reference/backlogit_telemetry.md`
- Harvest implementation at `internal/telemetry/harvest.go:131-137`

**Dependencies:** Unit 1 should complete first so documentation reflects the
fixed parser behavior.

**Approach:**

Add a "Side Effects" or "Behavior" section to the harvest command reference
documenting:
- Primary output: writes/merges `telemetry-sessions.jsonl`
- Side effect: calls `EnsureTelemetrySchema()` and `RehydrateTelemetry()` to
  create/refresh `telemetry_sessions` and `telemetry_tool_usage` SQLite tables
- Implication: harvest is not write-only to JSONL; it also refreshes the
  query cache so `telemetry list`, `telemetry top`, and `backlogit_query_sql`
  reflect the latest data immediately

**Verification:**
- Documentation accurately describes the harvest pipeline's SQLite side effect
- Markdown lint passes (if configured)

### Unit 4: Document Harvested Telemetry Fields and Metrics

**Files:**
- `docs/cli-reference/backlogit_telemetry.md` (parent command, add field reference)
  or new file `docs/telemetry-fields.md`

**Test files:** N/A (documentation only)

**Effort size:** small
**Skill domain:** docs
**Execution note:** documentation — create field reference

**Patterns to follow:**
- `SessionSummaryRecord` struct at `internal/telemetry/records.go:11-33`
- `ToolUsageRecord` struct at `internal/telemetry/records.go:38-46`
- SQLite schema at `internal/db/telemetry_schema.go:54-81`
- Existing SQL schema reference at `.github/instructions/backlogit-sql-schema.instructions.md`

**Dependencies:** Unit 1 should complete first so field descriptions reflect
fixed behavior. Independent of Unit 2 and Unit 3.

**Approach:**

Create a telemetry field reference documenting each metric available in the
harvested output:

| Field | Source | Description |
|---|---|---|
| `tokens_by_model` | SessionSummaryRecord | Token count per LLM model |
| `tool_calls_by_server` | SessionSummaryRecord | Tool invocation count per MCP server |
| `completed_tasks` | SessionSummaryRecord | Task IDs correlated to the session |
| `tokens_per_task` | SessionSummaryRecord | Derived: total tokens / completed task count |
| `peak_utilization` | SessionSummaryRecord | Peak context window usage ratio |
| `remaining_capacity` | SessionSummaryRecord | Tokens remaining at session end |
| `depletion_rate` | SessionSummaryRecord | Token consumption rate over session lifetime |
| `max_context_tokens` | SessionSummaryRecord | Model's maximum context window size |

Include the SQLite column mappings in `telemetry_sessions` and
`telemetry_tool_usage` tables so agents can write targeted queries.

**Verification:**
- Every field in `SessionSummaryRecord` and `ToolUsageRecord` is documented
- SQLite column names match `telemetry_schema.go`
- Cross-reference with existing SQL schema reference is consistent

## Dependency Graph

```text
Unit 1 (parser fix)  ──┐
                        ├──> Unit 3 (harvest docs)
Unit 2 (top cmd fix) ──┤
                        └──> Unit 4 (field reference)
```

- Units 1 and 2 are independent and may execute in parallel
- Units 3 and 4 depend on Unit 1 (docs should describe fixed behavior)
- Unit 3 and 4 are independent of each other

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Single shipment for all 4 items | Coherent telemetry quality story, avoids documentation drift | Split code/docs shipments — two review cycles for tightly coupled work |
| D2 | Use bufio.NewReader to eliminate token size limit | Eliminates hard token-size ceiling; real-world logs exceed any fixed buffer | Increasing Scanner buffer to 10MB — still has a ceiling, wastes memory |
| D3 | Replace top command metric (not add flag) | Token usage is the useful metric for context-window management; call counts mislead | Dual-mode flag — adds complexity for a niche use case |
| D4 | Field reference as standalone doc section | Centralized reference is more discoverable than inline CLI help | Inline in each subcommand doc — duplicates content |

## Risks and Caveats

1. **Scanner buffer size**: Even 10MB may not cover all edge cases. The
   implementer should add a structured log warning when entries are skipped so
   the buffer can be tuned in the future.
2. **Token attribution accuracy**: If `ToolUsageRecord` does not carry per-tool
   token counts, the top command must use proportional attribution, which is an
   approximation.
3. **Stale binary risk**: Per `docs/compound/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md`,
   schema changes in telemetry tables require rebuilding the binary. This plan
   does not change the schema, but the implementer should verify no migration
   is needed.

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change: **No** — no schema changes, no new
  MCP tools, no public API modifications
* Security, auth, permission, or compliance-sensitive behavior: **No**
* Migration, backfill, destructive data/config action, or irreversible step: **No**
* External integration, operator checkpoint, or external dependency: **No**
* High runtime, rollout, or rollback risk: **No** — changes are internal to
  the telemetry subsystem with no external consumers

**Requires plan hardening: no**

## Runtime Verification and Closure

### Unit 1 (Parser Fix)
- **Runtime surface changed**: Telemetry harvest pipeline behavior
- **Verification**: Run `backlogit telemetry harvest` against a workspace with
  known oversized log entries; confirm no partial zero-token sessions in output
- **Closure**: No monitoring needed — this is a correctness fix with test coverage

### Unit 2 (Top Command Fix)
- **Runtime surface changed**: CLI `telemetry top` output
- **Verification**: Run `backlogit telemetry top` and confirm ranking is by
  token usage
- **Closure**: No monitoring needed — CLI output change with test coverage

### Units 3-4 (Documentation)
- **Runtime surface changed**: None (documentation only)
- **Verification**: Review documentation for accuracy against implementation
- **Closure**: N/A

## Learnings Applied

- `docs/compound/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md`:
  Verified this plan does not introduce schema changes that would trigger the
  stale binary issue.
- `docs/compound/crash-safe-delete-rename-rollback-go-2026-04-23.md`: Atomic
  write patterns already applied in harvest.go; no changes needed.
- `docs/compound/go-file-write-short-write-guard-2026-04-23.md`: Relevant to
  file sync in harvest.go; no changes needed for this plan.
- `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`:
  RehydrateTelemetry() wraps DELETE + rebuild in a single transaction. Unit 3
  documentation should reference this transactional atomicity pattern.
- `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`:
  Batch rehydration must propagate errors, not log-and-continue. Implementer
  should verify RehydrateTelemetry() error handling follows this pattern.
- `docs/compound/db-reliability/sqlite-locked-missing-from-retry-predicate-2026-04-13.md`:
  Verify `isSQLiteBusy()` retry predicate includes SQLITE_LOCKED alongside
  SQLITE_BUSY for harvest reliability under concurrent access.
- `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md`:
  Windows atomic rename requires pre-remove gated on `runtime.GOOS`. Already
  applied in harvest.go; Unit 3 documentation should reference this pattern.

## Standards Check

- **Go quality gates**: All code changes must pass `go test ./...`,
  `go vet ./...`, `golangci-lint run`, and `gofmt -l .`
- **Test-first**: Units 1 and 2 specify test-first execution
- **GoDoc**: Any new exported functions must have GoDoc comments
- **Error handling**: Use `fmt.Errorf("context: %w", err)` wrapping per
  constitution
- **Structured logging**: Use `log/slog` for skip warnings in parser
- **Documentation**: Follows existing CLI reference format in `docs/cli-reference/`

## Plan Review

<!-- plan-review-attempt: 1 -->

### Gate Decision: PASS (revised from initial FAIL)

Initial review identified 3 P1 findings that required plan revision. All P1
items have been addressed in-place (revision cycle 1 of 2). The plan now
passes the gate.

### Summary

14 deduplicated findings from 5 reviewer personas (Constitution Reviewer,
Go Quality Reviewer, Scope Boundary Auditor, Learnings Researcher,
Architecture Strategist). After revision: 0 P0, 0 P1, 5 P2 (advisory),
6 P3 (informational).

### P0 — Critical

None (after revision).

### P1 — High (resolved in revision)

1. **[RESOLVED] Token attribution must use proportional approach** — The data
   model lacks per-tool token fields. Plan revised to commit to proportional
   session-level attribution as the only viable approach.
   *Reviewers: Go Quality, Architecture Strategist*

2. **[RESOLVED] Zero-token session validation missing** — Plan lacked an
   explicit step to reject sessions with `TotalTokens==0 && ToolCalls>0`.
   Added session validation step to Unit 1 approach and verification criteria.
   *Reviewers: Go Quality*

3. **[RESOLVED] Missing compound learnings citations** — 4 relevant learnings
   from `docs/compound/` were not cited. Added all 4 to the Learnings Applied
   section: atomic rehydration pattern, batch error propagation, SQLite retry
   predicate coverage, and Windows atomic rename.
   *Reviewer: Learnings Researcher*

### P2 — Moderate (advisory, noted for implementer)

1. **Unit 2 verification criteria could be more specific** — Tests should
   validate proportional attribution math, not just ranking order. Implementer
   should include a test with known token/call ratios to verify the formula.
   *Reviewer: Scope Boundary Auditor*

2. **Unit 3 dependency on Unit 1 is soft** — Unit 3 can document intended
   behavior and be verified post-implementation. The dependency graph shows
   the relationship but it is not a hard block.
   *Reviewer: Constitution Reviewer*

3. **Non-goals should explicitly exclude schema extensions** — If proportional
   attribution is insufficient in the future, adding a token field to
   `ToolUsageRecord` is a separate feature, not part of this plan.
   *Reviewer: Scope Boundary Auditor*

4. **Reporter limit applied after alphabetic sort** — `formatServerTable()`
   applies limit after sorting alphabetically. Unit 2 fix should sort by
   token metric before applying limit.
   *Reviewer: Go Quality*

5. **Timestamp parse failures logged at wrong level** — `extractTimestamp`
   returns zero time silently on parse failure. Implementer should log at
   DEBUG level for diagnostic visibility.
   *Reviewer: Go Quality*

### P3 — Low (informational)

1. Add regression test for old `[telemetry]` format in Unit 1 test suite.
2. Remove unused `TokensByServer` field from `SessionSummary` (types.go:95)
   to reduce confusion — defer to separate cleanup task.
3. Consider concurrent harvest read/write test for future hardening.
4. Unit 4 effort may be closer to small-medium; implementer should allocate
   accordingly.
5. Scanner strategy decision resolved in revision: bufio.NewReader is now
   the recommended approach.
6. Plan-to-code alignment on token aggregation resolved in revision.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| Token attribution approach | Go Quality + Architecture Strategist | Claude Haiku 4.5 + GPT-5.4 |
| Zero-token validation | Go Quality | Claude Haiku 4.5 |
| Missing learnings | Learnings Researcher | Claude Haiku 4.5 |
| Verification criteria | Scope Boundary Auditor | Claude Haiku 4.5 |
| Dependency softness | Constitution Reviewer | Claude Haiku 4.5 |
| Schema non-goal clarity | Scope Boundary Auditor | Claude Haiku 4.5 |
| Reporter sort order | Go Quality | Claude Haiku 4.5 |
| Timestamp logging | Go Quality | Claude Haiku 4.5 |

### Next Steps

Plan passes the review gate after revision. Proceed to `harvest` to decompose
into backlogit feature, task, and subtask items.
