---
schema_version: "1.0"
doc_type: plan
title: "Decided plan: Fix harvest scanner overflow in session events parser"
description: "Decided-plan consolidation of 2026-05-08-harvest-scanner-overflow-plan.md: final decisions, implementation units, risks, and review verdict."
chunk_strategy: h1-h2-h3
source: docs/exec-plans/2026-10-18-harvest-scanner-overflow-decided-plan.md
---

# Decided plan: Fix harvest scanner overflow in session events parser

Consolidated from `docs/archive/plans/2026-05-08-harvest-scanner-overflow-plan.md` (P-020 compaction, 2026-10-18). The owning release units are archived. Full deliberation history stays in the archived original.

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

## Implementation units

* **Implementation Units**: Each unit is scoped to roughly 2 hours of human-equivalent effort.
* **Unit 1: Replace bufio.Scanner with bufio.NewReader in ParseSessionEvents**: **Files:** `internal/telemetry/session_events.go` **Test files:** `internal/telemetry/session_events_test.go` **Effort size:** small **Skill domain:** code **Execution note:** test-first — write oversized-line tests first, then fix **Dependencies:** none  ...
* **Unit 2: Harden remaining bufio.Scanner sites with Buffer calls**: **Files:** `internal/events/reader.go`, `internal/stash/jsonl.go` **Test files:** (existing tests sufficient; these read backlogit-managed JSONL unlikely to hit 1 MB) **Effort size:** small **Skill domain:** code **Execution note:** characterization-first — ...
* **Unit 1 (session_events.go)**: - **Runtime surface changed:** `backlogit harvest` (telemetry harvest) — the   command that triggers `LoadSessionEvents` → `ParseSessionEvents` - **Verification:** Run `backlogit telemetry harvest` against a workspace with   known oversized `events.jsonl` ...
* **Unit 2 (buffer hardening)**: - **Runtime surface changed:** None observably — defensive hardening for data   sources that do not currently produce oversized lines - **Verification:** Existing test suites pass - **Closure:** None needed beyond test pass

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

## Plan Review

**Gate Decision: PASS**

Reviewed 2026-05-08 by plan-review gate with 6 personas (all same-model).

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F-1 | Go Quality Reviewer | claude-opus-4.6 |

## Traceability

| Original path | Archive path | Disposition |
|---|---|---|
| `docs/exec-plans/2026-05-08-harvest-scanner-overflow-plan.md` | `docs/archive/plans/2026-05-08-harvest-scanner-overflow-plan.md` | archived (decided-plan) |
