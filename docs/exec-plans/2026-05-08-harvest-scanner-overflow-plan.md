---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-08T00:00:00Z
    origin: .backlogit/queue/052-F.md
    status: reviewed
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-08-harvest-scanner-overflow-plan.md
title: Fix harvest scanner overflow in session events parser
---

# Fix harvest scanner overflow in session events parser

## Problem Frame

The `backlogit harvest` (telemetry harvest) command fails with `scan session
events: bufio.Scanner: token too long` when Copilot CLI `events.jsonl` files
contain lines exceeding the default `bufio.Scanner` 1 MB limit.

Root cause: `ParseSessionEvents` in `internal/telemetry/session_events.go`
uses `bufio.NewScanner(r)` without increasing the buffer size or switching to
`bufio.NewReader`. This is the same class of bug that was fixed in
`parser.go` (switched to `bufio.NewReader`) and `correlator.go` (increased
buffer to 1 MB) and `db/telemetry_schema.go` (switched to `bufio.NewReader`).

Scope boundary: fix the immediate crash in `session_events.go`, then
defensively harden the remaining `bufio.NewScanner` call sites across the
codebase where the cost is negligible.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | `ParseSessionEvents` must not error on lines exceeding 1 MB | Stash 8F88FABE, user report |
| R2 | Valid compaction events after an oversized line must still be parsed | Correctness invariant |
| R3 | Existing tests must continue to pass | Regression safety |
| R4 | Other `bufio.NewScanner` sites should be defensively hardened | Compound learning: `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md` |

## Scope Boundaries

### In Scope

- Replace `bufio.NewScanner` with `bufio.NewReader` in `session_events.go`
- Add test coverage for oversized lines in `ParseSessionEvents`
- Add `sc.Buffer()` calls to remaining unhardened `bufio.NewScanner` sites
  (`events/reader.go`, `stash/jsonl.go`)

### Non-Goals

- Rewriting `hook_events.go` or `correlator.go` (both already have `sc.Buffer(1MB)`)
- Changing the `parser.go` reader (already uses `bufio.NewReader`)
- Changing `db/telemetry_schema.go` (already uses `bufio.NewReader`)
- Adding integration-level cross-package oversized-line tests (deferred)

### Deferred to Implementation

- Exact EOF handling pattern: follow `parser.go` or `db/telemetry_schema.go` convention
- Whether to log oversized skipped lines at Debug or Warn level

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort.

### Unit 1: Replace bufio.Scanner with bufio.NewReader in ParseSessionEvents

**Files:** `internal/telemetry/session_events.go`
**Test files:** `internal/telemetry/session_events_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first — write oversized-line tests first, then fix
**Dependencies:** none

**Patterns to follow:**

- `internal/telemetry/parser.go:63-80` — `bufio.NewReader` + `ReadString('\n')` + EOF loop
- `internal/db/telemetry_schema.go:149-170` — same pattern applied to JSONL rehydration
- Compound learning: `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`

**Approach:**

1. Add new test cases to `session_events_test.go`:
   - `TestParseSessionEvents_OversizedNonCompactionLine_Skipped`: feed a 2 MB
     non-compaction line and verify no error and no events returned
   - `TestParseSessionEvents_OversizedLine_SubsequentEventsPreserved`: feed
     valid compaction → 2 MB junk → valid compaction, verify both compaction
     events are returned
   - `TestParseSessionEvents_OversizedLineBeforeCompaction_EventStillParsed`:
     2 MB junk → valid compaction, verify compaction event parsed

2. Replace `bufio.NewScanner(r)` loop in `ParseSessionEvents` (lines 33-65)
   with `bufio.NewReader(r)` + `ReadString('\n')` loop:
   - Use `reader.ReadString('\n')` in a for loop
   - Handle `io.EOF` correctly: process the last partial line, then break
   - Trim `\r\n` from each line
   - Skip empty and malformed lines as before
   - Remove `scanner.Err()` check; replace with read-error handling in the loop

3. Run existing `session_events_test.go` to verify backward compatibility.

**Verification:**

- All new oversized-line tests pass
- All existing `TestParseSessionEvents_*` tests pass
- `go test ./internal/telemetry/ -run TestParseSessionEvents -v` green
- `go vet ./internal/telemetry/`
- `gofmt -l internal/telemetry/session_events.go` reports no drift

### Unit 2: Harden remaining bufio.Scanner sites with Buffer calls

**Files:** `internal/events/reader.go`, `internal/stash/jsonl.go`
**Test files:** (existing tests sufficient; these read backlogit-managed JSONL unlikely to hit 1 MB)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first — verify existing tests pass before and after
**Dependencies:** Unit 1 (establishes the pattern and validates the fix approach)

**Approach:**

Add `sc.Buffer(make([]byte, 1<<20), 1<<20)` after `bufio.NewScanner()` in:

1. `internal/events/reader.go:36` — `ReadAllEvents` reads per-item JSONL logs.
   Backlogit's own event logs are unlikely to exceed 1 MB per line, but the
   cost of hardening is one line of code.

2. `internal/stash/jsonl.go:38` — `ReadJSONL` reads stash entries. Stash
   entries are short text, but the same defensive pattern costs nothing.

Already hardened (no changes needed):
- `internal/events/hook_events.go:129-131` — already has `sc.Buffer(1<<20, 1<<20)`
- `internal/telemetry/correlator.go:215-216` — already has `sc.Buffer(1024*1024, 1024*1024)`

**Verification:**

- `go test ./internal/events/ -v` passes
- `go test ./internal/stash/ -v` passes
- `go vet ./internal/events/ ./internal/stash/`
- `gofmt -l internal/events/reader.go internal/stash/jsonl.go` clean

## Dependency Graph

```text
Unit 1 (session_events.go fix) ← no deps
Unit 2 (buffer hardening)      ← depends on Unit 1
```

Unit 1 is the critical path — it fixes the reported bug. Unit 2 is defensive
hardening that should land in the same shipment but can be done independently.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Use `bufio.NewReader` + `ReadString` for session_events.go, not `sc.Buffer()` | Matches the established fix in `parser.go` and `db/telemetry_schema.go`. Eliminates the hard ceiling entirely rather than raising it. | `sc.Buffer(bigSize)` — still has a ceiling; the learning doc recommends `NewReader` for JSONL from external sources |
| D2 | Use `sc.Buffer()` (not `NewReader`) for `events/reader.go` and `stash/jsonl.go` | These read backlogit's own small JSONL. Full `NewReader` rewrite is unnecessary; `Buffer()` is a 1-line defensive add. | Full `NewReader` rewrite — overkill for data known to be small |
| D3 | Do not touch `hook_events.go` or `correlator.go` | Both already have `sc.Buffer(1MB)`. No change needed. | Rewrite to `NewReader` — unnecessary churn |

## Risks and Caveats

- **EOF handling subtlety**: `ReadString('\n')` returns `io.EOF` on the last
  line if it lacks a trailing newline. The loop must process that last line
  before breaking. The compound learning notes that `db/telemetry_schema.go`
  uses direct `readErr == io.EOF` comparison (safe when stdlib `errors` is not
  shadowed).
- **Compound learning warning**: The original 047-F fix missed `db/telemetry_schema.go`
  because the search was scoped to one package. This plan explicitly audits all
  packages (R4) to avoid repeating that mistake.

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change: **absent** — internal implementation only
- security, auth, permission, or compliance-sensitive behavior: **absent**
- migration, backfill, destructive data/config action, or irreversible step: **absent**
- external integration, operator checkpoint, or external dependency: **absent**
- high runtime, rollout, or rollback risk: **absent** — the change is a reader swap

Requires plan hardening: no

## Runtime Verification and Closure

### Unit 1 (session_events.go)

- **Runtime surface changed:** `backlogit harvest` (telemetry harvest) — the
  command that triggers `LoadSessionEvents` → `ParseSessionEvents`
- **Verification:** Run `backlogit telemetry harvest` against a workspace with
  known oversized `events.jsonl` lines and confirm it completes without error.
  Confirm session data appears in `telemetry-sessions.jsonl`.
- **Closure:** No monitoring or rollback needed — this is a bug fix for a
  crash. Successful harvest run is sufficient closure.

### Unit 2 (buffer hardening)

- **Runtime surface changed:** None observably — defensive hardening for data
  sources that do not currently produce oversized lines
- **Verification:** Existing test suites pass
- **Closure:** None needed beyond test pass

## Learnings Applied

- `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`:
  Always search the whole codebase when fixing a `bufio.Scanner` pattern.
  Follow data flow for JSONL files across all packages. This plan's Unit 2
  and the full audit scope are directly informed by this learning.

## Standards Check

- Go 1.22+ conventions: `bufio.NewReader` + `ReadString` is idiomatic for
  unbounded line reading. No new dependencies introduced.
- Error handling: wraps errors with `fmt.Errorf("context: %w", err)` per
  `internal/errors/` conventions.
- Testing: test-first development with colocated `_test.go` files.
- GoDoc: no new exported symbols; existing GoDoc comments remain accurate.
- Lint: changes must pass `golangci-lint run` and `gofmt -l .`

## Plan Review

**Gate Decision: PASS**

Reviewed 2026-05-08 by plan-review gate with 6 personas (all same-model).

### Summary

- P0 findings: 0
- P1 findings: 0
- P2 findings: 0
- P3 findings: 1

### Findings

#### P0 — Critical

None.

#### P1 — High

None.

#### P2 — Moderate

None.

#### P3 — Low (advisory)

**F-1 (Go Quality Reviewer):** The plan defers EOF handling pattern choice to
implementation. Both `parser.go` (uses `errors.Is(readErr, io.EOF)`) and
`db/telemetry_schema.go` (uses `readErr == io.EOF`) are valid patterns.
`session_events.go` does not import `internal/errors`, so direct `==`
comparison is safe and preferred. Implementer should follow the
`db/telemetry_schema.go` convention.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F-1 | Go Quality Reviewer | claude-opus-4.6 |

### Plan Hardening

Plan hardening: not required. All five hardening signals are absent.

### Next Steps

Plan is clean. Proceed to `harvest` to decompose into backlogit work items.
