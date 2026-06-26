---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-08T00:00:00Z
    origin: stash:E39D0A34,stash:5EC2B37F
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-08-telemetry-accuracy-call-rate-plan.md
title: Telemetry Accuracy & Call-Rate Metrics
---

# Telemetry Accuracy & Call-Rate Metrics

## Problem Frame

Two related telemetry gaps degrade operator visibility:

1. **Ghost sessions skew averages**: Sessions with `TotalTokens==0 AND ModelCalls==0 AND ToolCalls==0` (initialized but never used) inflate the denominator in `AVG_TOKENS/SESSION` and `AVG_PEAK_UTIL` in trend output. A 64-session harvest showed ~12 ghost sessions.

2. **Missing call-rate columns in trend**: `TrendGroup` aggregates tokens, tokens-per-task, and peak utilization but omits model calls and tool calls — two fields already present in `SessionSummaryRecord`. Operators cannot see call-rate trends over time.

These are tightly coupled: fixing ghost session filtering first ensures the new call-rate columns launch with accurate denominators.

### Scope Boundary

In scope: trend aggregation accuracy and new trend columns. Out of scope: model-name surfacing, model-class derivation, report `--by` options (covered by the Model-Aware Telemetry plan).

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Ghost sessions (`TotalTokens==0 AND ModelCalls==0 AND ToolCalls==0`) excluded from all average calculations in trend and report | stash:E39D0A34 |
| R2 | Ghost sessions still visible in `telemetry list` with a visual indicator | stash:E39D0A34 |
| R3 | Ghost sessions excluded from session count used for `AVG_TOKENS/SESSION` and `AVG_PEAK_UTIL` | stash:E39D0A34 |
| R4 | `TrendGroup` gains `AvgModelCalls` and `AvgToolCalls` fields | stash:5EC2B37F |
| R5 | Trend table, markdown, and JSON output include the new call-rate columns | stash:5EC2B37F |

## Scope Boundaries

### In Scope

- `IsGhostSession` predicate function
- Ghost session filtering in `GenerateTrendReport` and `GenerateReport` aggregation paths
- Visual indicator for ghost sessions in `formatSessionTable`
- `AvgModelCalls` and `AvgToolCalls` fields on `TrendGroup`
- Updated trend table, markdown, and JSON formatters
- Unit tests for all changes

### Non-Goals

- Changing harvest behavior (ghost sessions are still harvested and stored)
- Adding new CLI flags
- Modifying `SessionSummaryRecord` schema
- Model-name or model-class work

### Deferred to Implementation

- Exact visual indicator format for ghost sessions (recommend `[empty]` suffix or dimmed session ID)

## Implementation Units

### Unit 1: Ghost Session Predicate and Trend Filtering

**Files:** `internal/telemetry/validate.go`, `internal/telemetry/reporter.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `ValidateSessionSummary` predicate in `validate.go`
**Dependencies:** none

**Approach:**

1. Add `IsGhostSession(s SessionSummaryRecord) bool` to `validate.go` — returns true when `TotalTokens==0 && ModelCalls==0 && ToolCalls==0`.
2. In `GenerateTrendReport` (reporter.go), filter ghost sessions from the aggregation loop (lines 362-398) and from the average finalization counts (lines 402-433). Ghost sessions must not increment `g.Sessions`, `g.TotalTokens`, or any average accumulator. The session count used for `AvgTokensSession` must exclude ghosts.
3. In `GenerateReport` server aggregation (`proportionalServerTokens`), ghost sessions already have zero tool calls and contribute nothing — no change needed there. But `formatSessionTable` should mark ghosts visually (Unit 2).

**Verification:**

- Test: trend report with 5 sessions (2 ghost) produces averages using only 3 active sessions
- Test: ghost sessions do not appear in trend group session counts
- Test: `IsGhostSession` returns correct results for edge cases (zero tokens but nonzero model calls = not ghost)

### Unit 2: Ghost Session Visual Indicator in List Output

**Files:** `internal/telemetry/reporter.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `formatSessionTable` formatting (reporter.go:165-181)
**Dependencies:** Unit 1 (uses `IsGhostSession`)

**Approach:**

1. In `formatSessionTable`, check `IsGhostSession` for each row. When true, append ` [empty]` to the session ID column so operators can spot ghost sessions at a glance.
2. Apply the same indicator in `formatSessionMarkdown` for consistency.
3. No change to JSON output — consumers can detect ghosts from the zero field values.

**Verification:**

- Test: `formatSessionTable` output contains `[empty]` for ghost sessions
- Test: `formatSessionMarkdown` output contains `[empty]` for ghost sessions
- Test: non-ghost sessions with zero tokens but nonzero model calls do NOT get the marker

### Unit 3: Add Call-Rate Columns to TrendGroup and Formatters

**Files:** `internal/telemetry/reporter.go`
**Test files:** `internal/telemetry/reporter_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `AvgTokensTask` and `AvgPeakUtil` accumulation pattern in `GenerateTrendReport` (reporter.go:380-433)
**Dependencies:** Unit 1 (ghost filtering must be in place so call-rate averages exclude ghosts)

**Approach:**

1. Add two fields to `TrendGroup`: `AvgModelCalls float64` and `AvgToolCalls float64` with JSON tags `avg_model_calls` and `avg_tool_calls`.
2. In `GenerateTrendReport` aggregation loop, accumulate `ModelCalls` and `ToolCalls` from each non-ghost session.
3. In the finalization step, divide accumulated totals by active session count to compute averages.
4. Update `formatTrendTable` to add `AVG_MODEL_CALLS` and `AVG_TOOL_CALLS` columns.
5. Update `formatTrendMarkdown` to add corresponding columns.
6. JSON output inherits the new fields automatically from struct marshaling.

**Verification:**

- Test: trend JSON output includes `avg_model_calls` and `avg_tool_calls`
- Test: trend table output includes formatted call-rate columns
- Test: call-rate averages exclude ghost sessions
- Test: existing trend fields unchanged for backward compatibility

## Dependency Graph

```text
Unit 1 (ghost predicate + trend filtering)
  └── Unit 2 (visual indicator in list — uses IsGhostSession)
  └── Unit 3 (call-rate columns — depends on ghost filtering being active)
```

Unit 1 is foundational. Units 2 and 3 can proceed in parallel after Unit 1.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Filter ghosts at report time, not harvest time | Ghost sessions should remain in JSONL for auditability; filtering at display preserves data | Rejecting at harvest — would lose the data permanently |
| D2 | Use `[empty]` suffix marker in list output | Minimal, grep-friendly indicator that works in both table and markdown | Dim/ANSI colors — not portable to markdown; separate section — breaks table flow |
| D3 | Use unconditional `float64` for call-rate fields (not `*float64`) | Model calls and tool calls are always available (zero is meaningful); no nil sentinel needed | Pointer fields — adds complexity with no benefit since zero is valid |

## Risks and Caveats

- **Test data setup**: Tests need both ghost and active sessions to validate filtering. Use table-driven tests with explicit session fixtures.
- **Backward compatibility**: Adding new columns to trend table output may affect scripts that parse column positions. JSON output is additive and safe. Table parsers should use headers, not positions.

## Plan Hardening Signals (REQUIRED)

- Public API, schema, or contract change: **Yes** — `TrendGroup` struct gains two new JSON fields. Additive and backward-compatible.
- Security, auth, permission, or compliance-sensitive behavior: **No**
- Migration, backfill, destructive data/config action, or irreversible step: **No**
- External integration, operator checkpoint, or external dependency: **No**
- High runtime, rollout, or rollback risk: **No**

Requires plan hardening: **no** — changes are additive, no data migration, no destructive actions.

## Runtime Verification and Closure

- **Changed runtime surface**: `backlogit telemetry trend` CLI output and `backlogit telemetry list` CLI output
- **Verification**: Run `backlogit telemetry trend` after harvest and confirm ghost sessions are excluded from averages and new call-rate columns appear. Run `backlogit telemetry list` and confirm ghost sessions show `[empty]` marker.
- **Closure**: No monitoring or rollback needed — these are CLI display changes with no persistent side effects.

## Learnings Applied

No directly applicable learnings from `docs/compound/` — this is a new feature area.

## Standards Check

- Go quality gates: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
- Test-first development per constitution Principle III
- Struct fields use typed Go with JSON tags per Principle I
- No new dependencies required

## Plan Review

**Gate Decision: PASS**

**Date:** 2026-05-08
**Reviewers:** constitution-reviewer, go-quality-reviewer, scope-boundary-auditor, learnings-researcher

### Summary

4 personas reviewed. 1 P3 advisory finding. No P0, P1, or P2 findings.

### P0 — Critical

None

### P1 — High

None

### P2 — Moderate

None

### P3 — Low (advisory)

**F1 (scope-boundary-auditor):** Unit 2 (ghost session visual indicator) is very small and could be merged with Unit 1 to reduce implementation overhead. Kept separate for width isolation per plan guidelines. Advisory only.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F1 | scope-boundary-auditor | claude-opus-4.6 |

### Next Steps

Plan passes review. Proceed to `harvest` to decompose into backlogit work items.
