---
chunk_strategy: h1-h2-h3
description: 'Converge parseItemLogFile (rehydration) and events.ReadAllEvents (doctor fallback) on a single skip-with-warning policy for malformed JSONL event-log lines by extracting a shared events.ParseEventLine helper, with observability (slog.Warn on skip) and test-first verification of convergence.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-07-malformed-jsonl-line-handling-convergence-plan.md
title: 'Unify malformed-JSONL-line handling via a shared events.ParseEventLine helper'
---

## Source

- Stash: `F3844849` (kind=task, priority=low) — "Unify malformed-JSONL-line
  handling."
- Decision: `docs/decisions/2026-07-07-malformed-jsonl-line-handling-convergence-deliberation.md`
  (Policy A: skip-with-warning; Approach 2: shared line-parse helper).
- Origin: `083-S` adversarial review, Reviewer A finding #8 (P3 MEDIUM,
  non-blocking) — `docs/closure/2026-07-06-083-S-feature-pr-adversarial-review.md:90-93,129-131`.

## Problem Frame

Two independent parsers of per-item JSONL event logs disagree on malformed
(non-JSON) lines:

- `internal/db/rehydration.go:495` `parseItemLogFile` — on the first
  `json.Unmarshal` error returns `fmt.Errorf("parse item log line: %w", err)`.
  Its caller `rehydration.go:394-398` logs `slog.Warn("failed to parse item
  log", ...)` then `return nil` from the `filepath.WalkDir` callback, so the
  walk continues but **every event for that item is dropped** from the SQLite
  projection.
- `internal/events/reader.go:24` `ReadAllEvents` — on a `json.Unmarshal` error
  executes `continue`, **silently skipping only the bad line** with no log, and
  keeping all good events.

The projection is a disposable read-model rebuilt in full on every `sync`, so
resilience is preferred over hard failure. Converge both parsers on
skip-with-warning by routing both through one shared helper that centralizes the
parse/skip **decision** (each caller keeps its own observability), with a
convergence test guarding the remaining caller-owned logging. This directly
counters the re-divergence class documented in
`docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`,
where a JSONL-parser fix landed in one package while the independent db-package
parser was missed.

## Requirements Trace

| # | Requirement (from decision) | Implementation action |
|---|---|---|
| R1 | One identical malformed-line policy across both parsers | Add `events.ParseEventLine`; route both callers through it |
| R2 | Policy = skip-and-continue (retain surrounding valid events) | Helper returns `ok=false, err!=nil` on malformed; callers `continue` |
| R3 | Skip is observable (no silent data loss) | Both callers `slog.Warn` with item ID + 1-based line number + unmarshal error before skipping |
| R4 | Valid lines unaffected | Helper returns parsed Event with `ok=true`; item-ID backfill preserved |
| R5 | Prevent silent re-divergence of the malformed-line decision | Single shared helper owns the parse/skip **decision** (both callers call it); the convergence test guards the caller-owned warn loops, which are not centralized |

## Implementation Units

### Unit 1 — Converge malformed-line handling via a shared helper (test-first)

**Execution posture:** test-first. Write the red convergence test that proves
today's divergence, then implement the shared helper + both refactors to turn it
green.

**Files affected (2 source, 1 new test — focused ~2h session):**

1. `internal/events/reader.go`
2. `internal/db/rehydration.go`
3. `internal/db/rehydration_malformed_line_test.go` (new)

#### Step 1 — Red test (proves divergence, then convergence)

Add `internal/db/rehydration_malformed_line_test.go` (internal test,
`package db`, which can see the unexported `parseItemLogFile` and also import
`internal/events`). Use `testify/require` + `testify/assert` with `t.Run`
subtests, and write fixtures under `t.TempDir()` at `logs/<itemID>.jsonl` so both
readers resolve the same file. The fixture has four lines:
`[valid-event, malformed-non-json, whitespace-only, valid-event]` for the same
item.

- **True red gate (Principle II):** the **convergence assertion** is the test
  observed FAILING before implementation and PASSING after — with today's code
  `parseItemLogFile` returns a non-nil error and 0 events while
  `events.ReadAllEvents(context.Background(), logsDir, itemID)` returns 2 events;
  after the refactor BOTH return exactly the 2 valid events and a nil error.
  Run it and confirm it fails first, then implement Steps 2-3 to make it pass.
- The parallel divergence assertion (parseItemLogFile errors / ReadAllEvents
  skips on today's code) is retained only as **documentation** of the starting
  state, not as the red gate (it passes on unmodified code).

#### Step 2 — Shared helper (`internal/events/reader.go`)

Add the shared line-parse helper (single source of truth for the policy). Add
`log/slog` and `strings` to the import block:

```go
// ParseEventLine parses a single JSONL line from an item event log into an
// Event under the unified malformed-line policy shared by ReadAllEvents
// (doctor fallback) and rehydration's parseItemLogFile (SQLite projection):
//
//   - blank / whitespace-only line -> ok=false, err=nil   (skip silently)
//   - malformed JSON               -> ok=false, err!=nil  (caller logs + skips)
//   - valid event                  -> ok=true,  err=nil
//
// itemID backfills Event.ItemID when the serialized line omits it.
func ParseEventLine(line, itemID string) (Event, bool, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{}, false, nil
	}
	var e Event
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return Event{}, false, err
	}
	if e.ItemID == "" {
		e.ItemID = itemID
	}
	return e, true, nil
}
```

The `events` package has no logger-injection seam and `ReadAllEvents` has many
callers, so its signature stays fixed and it logs via the package-level
`slog.Warn` (adding `log/slog`) — the idiomatic choice for a free function in
this package. Refactor `ReadAllEvents` to use the helper and log the skip
(replacing the silent `continue`), tracking a 1-based line number:

```go
	var result []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		e, ok, perr := ParseEventLine(scanner.Text(), itemID)
		if perr != nil {
			slog.Warn("skipping malformed event log line",
				"item", itemID, "path", path, "line", lineNum, "error", perr)
			continue
		}
		if !ok {
			continue
		}
		result = append(result, e)
	}
	return result, scanner.Err()
```

#### Step 3 — Rehydration refactor (`internal/db/rehydration.go`)

Refactor `parseItemLogFile` to delegate to `events.ParseEventLine`, changing the
malformed-line `return nil, err` into a warn+skip while preserving the
file-read error path. `rehydration.go` already imports `log/slog` and `strings`:

```go
func parseItemLogFile(path, itemID string) ([]events.Event, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read item log %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	result := make([]events.Event, 0, len(lines))
	for i, line := range lines {
		e, ok, perr := events.ParseEventLine(line, itemID)
		if perr != nil {
			slog.Warn("skipping malformed event log line",
				"item", itemID, "path", path, "line", i+1, "error", perr)
			continue
		}
		if !ok {
			continue
		}
		result = append(result, e)
	}
	return result, nil
}
```

**Import hygiene (deterministic):** `encoding/json` is used at exactly one site
in `rehydration.go` — line 508 (`json.Unmarshal` inside `parseItemLogFile`), the
only `json.` reference in the file. After delegating to `events.ParseEventLine`,
`encoding/json` becomes unused and **MUST be removed from the import block** or
`go build ./...` fails. Note `gofmt -l` does NOT flag unused imports; rely on
`goimports`, `go build ./...`, and `go vet ./...` as the guard. Remove the
import definitively — do not treat it as conditional.

**Logger seam (accepted deviation):** `rehydration.go` has a `WithLogger`
injection seam (lines 35-54) used by `Rehydrate`/`warnOnDuplicateSourceIDs`, but
`rehydrateItemLogs`/`parseItemLogFile` already use package-level `slog` (existing
`slog.Warn("failed to parse item log", ...)` at line 396, `slog.Debug` at 386).
The new skip-warn matches that co-located existing convention. Threading the
injected `cfg.logger` through `rehydrateItemLogs → parseItemLogFile` is a
separate, broader consistency refactor — noted as an **optional follow-up**, not
pulled into this low-risk task. The observability test therefore captures via
`slog.SetDefault` (see Step 4 test-safety constraints).

#### Step 4 — Green + observability + regression tests

In `internal/db/rehydration_malformed_line_test.go`, complete the scenarios
(`testify/require` for fatal preconditions, `testify/assert` for value checks,
`t.Run` subtests, all fixtures under `t.TempDir()`):

- **Scenario 1 (convergence):** post-implementation, both `parseItemLogFile` and
  `events.ReadAllEvents` return exactly the 2 valid events and a nil error for
  the `[valid, malformed, whitespace-only, valid]` fixture — they agree
  byte-for-byte. (This is the red gate from Step 1: it fails on today's code.)
- **Scenario 2 (observability):** assert the malformed line is skipped with a
  warning containing the item ID and the 1-based line number (`line=2`) for each
  path. **Test-safety constraints (Go/Constitution review):** capture by
  installing `slog.New(slog.NewTextHandler(buf, ...))` via `slog.SetDefault`,
  restore the prior default with `t.Cleanup(func(){ slog.SetDefault(prev) })`,
  and DO NOT call `t.Parallel()` in this test (global-default mutation is not
  parallel-safe).
- **Scenario 3 (valid lines unaffected):** an all-valid fixture returns all
  events, nil error, and emits no warning from either parser.
- **Scenario 4 (whitespace-only line, convergence lock):** assert BOTH parsers
  skip the whitespace-only line **silently with no warning** (the helper's
  `TrimSpace` classifies it as blank → `ok=false, err=nil`). This locks the
  CRLF/blank convergence and guards against regression.

Optionally add a focused table test for `events.ParseEventLine` in
`internal/events/reader_test.go` (blank / whitespace-only / malformed / valid →
expected `(ok, err)` shape). Keep it small; it is consolidatable into Scenario
coverage if file count is a concern.

**Verification (atomic milestone):**

- `go build ./...` succeeds (also proves `encoding/json` was correctly removed
  from `rehydration.go`).
- `go vet ./internal/events/... ./internal/db/...` is clean.
- `go test ./internal/events/... ./internal/db/...` passes, including the new
  convergence, observability, valid-unaffected, and whitespace-only scenarios.
- `gofmt -l internal/events/reader.go internal/db/rehydration.go` reports no
  diffs (note: `gofmt` does not detect unused imports — `go build`/`go vet`
  above are the guard for the `encoding/json` removal).

## Dependency Graph

Single implementation unit; internal step order is Step 1 (red test) → Step 2
(helper) → Step 3 (rehydration refactor) → Step 4 (green/observability tests).
No cross-unit dependencies. No cycles.

## Decisions and Rationale

- **Skip-with-warning over strict error:** a malformed line is a permanent,
  non-retryable parse failure; erroring bricks the item's rehydration with no
  recovery path. The projection is disposable/rebuildable, so retaining good
  events and logging the skip is strictly better. (Decision doc, Option A.)
- **Shared helper over parity edits:** centralizing the parse/skip decision is
  the durable fix for a documented re-divergence anti-pattern
  (`bufio-scanner-incomplete-fix-missed-db-package`) and mirrors the existing
  shared-`EventWriter` extraction precedent. (Decision doc, Approach 2.)
- **Caller-owned observability:** the helper classifies (parse/skip); each caller
  logs with its own path/item/line context, so warn messages stay caller-relevant
  while the policy stays centralized.
- **String-level helper seam:** operating on an already-read line string
  sidesteps the callers' differing read strategies (Scanner vs ReadFile+Split),
  keeping the convergence scoped to malformed-JSON policy only.
- **Bare unmarshal error from the helper (intentional):** `ParseEventLine`
  returns the raw `json.Unmarshal` error rather than wrapping it with
  `fmt.Errorf("...: %w", err)`. This is deliberate: the error is not propagated
  up the stack — it is consumed immediately by each caller's structured
  `slog.Warn` (item + path + 1-based line), which supplies richer context than a
  wrapped string would. (Principle I "wrap errors" deviation, reviewed and
  accepted because the error terminates at the caller's structured log.)

## Risks and Caveats

- **Silent partial data masked by skips** — mitigated by mandatory per-line
  `slog.Warn` (item + line + reason) in both paths; the projection rebuilds each
  sync once the log is repaired.
- **Import hygiene after refactor** — `parseItemLogFile` stops calling
  `json.Unmarshal` directly, making `encoding/json` unused (sole use is line
  508); remove it from the import block. `go build ./...`/`go vet ./...` are the
  guard (`gofmt -l` does not detect unused imports).
- **Adjacent oversized-line divergence (out of scope)** — `ReadAllEvents`'
  `bufio.Scanner` 1 MB line cap vs `parseItemLogFile`'s unbounded read is a
  separate concern; the string-level helper does not change it. Candidate
  follow-up stash entry, not part of F3844849.
- **Warn-only observability is a weak agent signal** — the decision doc's
  Unresolved Question #2 notes agents do not read warn logs; an aggregated
  doctor skipped-line signal would be stronger. Deferred as a follow-up; per-line
  warn is the proportionate observability for this low-risk task.

## Constitution Check

Mapped against `.github/instructions/constitution.instructions.md` (Principles
I–XI):

| Principle | Status | Notes |
|---|---|---|
| I. Safety-First Go | PASS | Pure Go, no `unsafe`. Helper returns a bare error consumed by caller structured logging (intentional deviation from "wrap errors", justified in Decisions). |
| II. Test-First (NON-NEGOTIABLE) | PASS | Convergence assertion is the red gate (fails today, passes after); `testify/require`+`assert`, `t.Run` subtests, `t.TempDir()` fixtures. |
| III. Workspace Isolation | PASS | Reads only per-item JSONL logs under the workspace `logs/` dir; no path traversal; no new FS write surface. Test fixtures under `t.TempDir()`. |
| IV. CLI Workspace Containment | PASS | No new file create/modify/delete outside workspace; behavior change is read-model completeness only. |
| V. Structured Observability | PASS | Adds `slog.Warn` (item + path + 1-based line + reason) where `ReadAllEvents` previously skipped silently — a net observability improvement. Doctor-aggregate signal deferred (Risks). |
| VI. Single Responsibility | PASS | No new dependencies; one small helper with a single job (classify one line). |
| VII. Destructive Command Approval | N/A | No destructive commands; disposable read-model, revert-safe. |
| VIII. Explicit Safety Modes | N/A | No elevated-risk/large-blast-radius action; hardening not required (see below). |
| IX. Git-Friendly Persistence | PASS | No serialized-state format change; append-only logs untouched. |
| X. Agent Context Efficiency | PASS | No change to query/tooling surfaces. |
| XI. Merge Commit History | N/A | Stage does not merge; Ship owns PR/merge strategy. |

## Plan Hardening Signals

- **Public API, schema, or contract change:** ABSENT. `ParseEventLine` is a new
  function in a module-`internal` package (not a public API); no wire/schema/CLI
  contract changes. The behavior change is data-completeness only (rehydration
  now retains good events instead of dropping the whole item's events); the top
  level `sync` already did not hard-fail on this path.
- **Security, auth, permission, or compliance-sensitive behavior:** ABSENT.
- **Migration, backfill, destructive/irreversible action:** ABSENT. Append-only
  logs are untouched; the SQLite projection is a disposable cache rebuilt on
  every sync; the change is trivially reversible by revert.
- **External integration, operator checkpoint, external dependency:** ABSENT.
- **High runtime, rollout, or rollback risk:** ABSENT. Disposable read-model;
  rollback = git revert, self-heals on next sync.

Requires plan hardening: no

## Runtime Verification and Closure

- **Runtime surface touched:** the `backlogit sync`/rehydration projection path
  and the doctor event-read fallback path (both read per-item JSONL logs).
- **Runtime verification before absorption:** `go test ./internal/events/...
  ./internal/db/...` green (convergence + observability + valid-unaffected);
  `go build ./...` green; a manual `backlogit sync` against a workspace whose log
  contains a deliberately malformed line indexes the item's good events and emits
  the skip warning (rather than dropping all of the item's events).
- **Operational closure:** covered by the new tests; observability is the new
  `slog.Warn` skip lines; rollback trigger is any regression in event counts
  after sync (revert restores prior behavior; the read-model rebuilds next sync).
  No monitoring dashboard or ownership handoff needed for a disposable-cache read
  path.

## Plan Review

<!-- plan-review-attempt: 1 -->

**Gate decision: PASS** (after one revision that absorbed all P2 findings; no P0/P1 findings at any point).

Reviewed by three independent personas (multi-persona plan-review gate). Plan
hardening was **not required** (`Requires plan hardening: no`, all five signals
absent and verified) — consistent with the P-006 gate.

### Reviewer verdicts (initial pass)

| Persona | Verdict | Findings |
|---|---|---|
| Go Reviewer | PASS | 1×P2 (import hygiene), 3×P3 |
| Scope Boundary Auditor | PASS | 2×P3 (affirmed shared-helper justification, scope boundary, sizing, verification) |
| Constitution Reviewer | ADVISORY | 2×P2 (missing Constitution Check; slog seam), 4×P3 |

Merged initial gate: **ADVISORY** (P2s present, zero P0/P1). Revised to **PASS**
by resolving every P2 and the high-value P3s below.

### Findings and resolutions

- **[P2 · Go] `encoding/json` deterministically unused after refactor** (sole
  use is line 508; `gofmt` won't catch it). **Resolved:** plan now mandates
  removing the import and adds `go build`/`go vet` as the guard (Step 3 import
  hygiene note; Verification; Risks).
- **[P2 · Constitution] Missing explicit Constitution Check.** **Resolved:**
  added the `## Constitution Check` section mapping Principles I–XI.
- **[P2 · Constitution / P3 · Go / P3 · Scope] Global `slog.SetDefault` in tests
  vs the `WithLogger` injection seam.** **Resolved:** documented that
  `rehydrateItemLogs`/`parseItemLogFile` already use package-level `slog`
  (existing warn at line 396), so the new skip-warn matches the co-located
  convention; logger threading noted as an optional follow-up; the observability
  test now requires `t.Cleanup` restore and forbids `t.Parallel` (Step 3 logger
  seam note; Step 4 test-safety constraints).
- **[P3 · Constitution] Red-test framing** (the divergence assertion passes on
  today's code). **Resolved:** Scenario 1 (convergence) is designated the red
  gate observed failing first; the divergence assertion is documentation only.
- **[P3 · Go] Whitespace-only line reclassification** (helper `TrimSpace`
  treats it as blank). **Resolved:** added Scenario 4 asserting both parsers skip
  a whitespace-only line silently with no warning.
- **[P3 · Scope] R5 "cannot re-diverge" overclaim.** **Resolved:** reworded R5,
  Problem Frame, and the re-divergence risk to "centralizes the parse/skip
  decision; the convergence test guards the caller-owned logging."
- **[P3 · Constitution] Test conventions.** **Resolved:** plan now specifies
  `testify/require`+`assert`, `t.Run` subtests, and `t.TempDir()` fixtures.
- **[P3 · Constitution] Bare error from helper (Principle I).** **Resolved:**
  documented as an intentional, reviewed deviation in Decisions.
- **[P3 · deferred, both] Doctor-aggregate skip signal.** Accepted as a deferred
  follow-up (Risks; decision doc Unresolved Question #2) — warn-per-line is the
  proportionate observability for this low-risk task.

### Data-integrity lens (operator-required)

The chosen skip-with-warning policy does **not** silently drop meaningful events:
every skipped malformed line emits a structured `slog.Warn` (item + path + 1-based
line + reason) on **both** paths, closing the pre-existing silent-skip gap in
`ReadAllEvents`. The batch-failure anti-pattern (transient/retryable → propagate
error) is correctly distinguished from a permanent malformed line
(non-retryable → skip + observe). Rehydration data completeness strictly
improves (retains good events instead of dropping the whole item). No P0/P1
data-integrity findings.

**Verdict: PASS — proceed to harvest.**
