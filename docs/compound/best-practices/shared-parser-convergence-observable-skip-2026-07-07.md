---
chunk_strategy: h1-h2-h3
description: Two per-item JSONL event-log parsers diverged on malformed-line handling (one silently skipped, one dropped all of an item's events); converging both onto a shared helper with observable skip-with-warning fixes the data loss and prevents future re-divergence
doc_type: learning
docline:
    category: best-practices
    component: events-rehydration
    date: 2026-07-07T00:00:00Z
    file_path: internal/events/reader.go
    message: parseItemLogFile returned nil,err on a malformed line so the caller dropped ALL of the item's events, while ReadAllEvents silently continued — two parsers, two behaviors, both wrong
    problem_type: data-integrity
    resolution_type: design_change
    resolved: true
    root_cause: duplicated per-record parse logic in two packages drifted into inconsistent malformed-line policies
    severity: high
    tags:
        - jsonl
        - event-log
        - rehydration
        - parser-convergence
        - shared-helper
        - skip-with-warning
        - observable-skip
        - data-integrity
        - silent-failure
        - slog
ingested_at: "2026-07-07T00:00:00Z"
schema_version: "1.0"
source: docs/compound/best-practices/shared-parser-convergence-observable-skip-2026-07-07.md
title: Converge divergent per-record parsers onto one shared helper; make the skip observable
---

# Converge divergent per-record parsers onto one shared helper; make the skip observable

## Problem

Two independent code paths parsed the same per-item JSONL event log and drifted
into **inconsistent malformed-line policies**:

- `internal/events/reader.go:ReadAllEvents` (doctor fallback) — hit a malformed
  line and **silently** `continue`d. No signal at all: silent data loss.
- `internal/db/rehydration.go:parseItemLogFile` (SQLite rehydration) — hit a
  malformed line and `return nil, err`; the caller then discarded **all** of
  that item's events, **bricking rehydration** for the item.

Same input, two behaviors, both wrong. A single corrupt line either vanished
without a trace or nuked an item's entire history. Because the logic was
duplicated across two packages, nothing kept the two policies in sync.

## Root Cause

Per-record parse-and-classify logic was copy-adapted into two packages. There
was no single source of truth for "what does a blank / malformed / valid line
mean", so the two implementations diverged over time.

## Resolution

Extract one shared helper and route **both** parsers through it:

```go
// internal/events/reader.go — single source of truth
func ParseEventLine(line, itemID string) (Event, bool, error) {
    // blank/whitespace-only -> (Event{}, false, nil)   skip, not an error
    // malformed JSON        -> (Event{}, false, err)    skip WITH warning
    // valid                 -> (event,   true,  nil)
}
```

Both `ReadAllEvents` and `parseItemLogFile` now:

1. call `ParseEventLine` per line,
2. on a parse error emit a structured `slog.Warn` carrying `item_id`, `path`,
   the **1-based** line number, and the error, then `continue` (skip that line
   only, retain surrounding valid events),
3. on `ok == false` with no error, silently skip (blank line).

This converges both paths byte-for-byte (line numbering, ItemID backfill, CRLF
trimming, ordering) and un-bricks rehydration: a malformed line no longer drops
the item's good events.

## Prevention

- **One shared helper for one record grammar.** When two packages parse the same
  serialized format, factor the per-record decision into a single exported
  function so policy cannot drift. A table test on the helper is the convergence
  lock.
- **Make every skip observable.** "Skip-with-warning" is only safe if the skip is
  *visible*. Emit a structured `slog.Warn` with enough context (item, path,
  1-based line, reason) to locate the offending record. A silent `continue` is
  data loss you will not notice until an audit.
- **Skip only deterministic per-record errors; surface everything else as a
  returned error.** `json.Unmarshal` on one line is deterministic — retrying
  won't help, so skip it (with a warning). But `os.ReadFile`/`os.Open` failures
  and `scanner.Err()` (I/O, `bufio.ErrTooLong`) are transient/structural: the
  parser MUST surface them as a **returned error**, never silently convert them
  into a per-line skip. That is the guarantee this convergence provides — a
  *parser-level* one ("no structural failure is masked as a silent per-line
  skip"), not a blanket "every I/O failure aborts the whole rebuild." What the
  *caller* does with that error is a separate, out-of-scope decision and it
  differs by call site: `internal/core/shipment_gate.go` propagates a
  `ReadAllEvents` error, whereas the pre-existing rehydration walk callback in
  `internal/db/rehydration.go` logs `failed to parse item log` and skips that one
  item (acceptable because the SQLite index is an ephemeral, re-syncable cache).
  Document that boundary precisely rather than overstating propagation.
- **Test-first for the convergence.** A red test that feeds the same malformed
  line to both parsers and asserts identical behavior (skip + warn), plus a
  whitespace-only convergence-lock and an observability assertion on the warn,
  guards against re-divergence.

## Related Solutions

- `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`
  — the general principle (build-shared-state operations must propagate phase
  failures). This entry identifies the valid log-and-continue exception:
  deterministic per-record parse errors, made observable.
- `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`
  — oversized JSONL lines (`bufio.Scanner` 1 MB limit). The event-log parsers
  converged here do not yet apply that handling; deferred follow-up.
- Citations: `docs/closure/2026-07-07-086-S-closure.md`,
  `docs/exec-plans/2026-07-07-malformed-jsonl-line-handling-convergence-plan.md`,
  `docs/decisions/2026-07-07-malformed-jsonl-line-handling-convergence-deliberation.md`.
