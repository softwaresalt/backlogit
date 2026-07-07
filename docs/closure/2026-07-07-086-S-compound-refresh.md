---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh (propose) for shipment 086-S — unify malformed-JSONL-line handling across the two per-item event-log parsers. Reviews existing compound entries touching internal/db/rehydration.go and JSONL parsing for supersession/duplication. Outcome: two in-scope entries classified keep (still accurate; complemented not superseded); one new learning recommended for capture (shared-parser convergence + observable skip-with-warning for deterministic per-record parse errors).'
doc_type: closure
docline:
    ms.date: 2026-07-07T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-07T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-07-086-S-compound-refresh.md
title: 086-S Malformed-JSONL-Line Convergence — Compound Refresh (Propose)
---

# 086-S Malformed-JSONL-Line Convergence — Compound Refresh (Propose)

- **Mode**: propose (no compound files edited)
- **Scope**: `docs/compound/` entries touching `internal/db/rehydration.go`,
  JSONL parsing, event-log rehydration, and silent-failure/data-integrity
  patterns.
- **Context**: shipment 086-S converged two divergent per-item JSONL parsers
  (`internal/events/reader.go:ReadAllEvents`, `internal/db/rehydration.go:parseItemLogFile`)
  onto a single skip-with-warning policy via a shared `events.ParseEventLine`
  helper. See `docs/closure/2026-07-07-086-S-closure.md`.

## Entries reviewed

### 1. `db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`

- **Classification: keep**
- **Evidence**: entry documents `internal/db/rehydration.go:Rehydrate` returning
  `nil` after batch-transaction retry exhaustion (silent partial index). Its
  Principle — "any operation that builds shared state in phases MUST propagate
  phase failures to callers; log-and-continue is only appropriate when the
  failure is truly ignorable AND the caller documents partial results are
  acceptable" — remains accurate and is **not** contradicted by 086-S.
- **Relationship to 086-S**: complementary. 086-S addresses a *different* failure
  mode in the same file (a single malformed per-item log line). It refines the
  principle by identifying the valid log-and-continue case: a **deterministic
  per-record parse error** where (a) the alternative is worse (old
  `parseItemLogFile` dropped ALL of the item's events), (b) the skip is made
  observable via `slog.Warn`, and (c) transient/structural failures
  (`os.ReadFile`/`os.Open`, `scanner.Err()`, `bufio.ErrTooLong`) still propagate.
  The new learning cites this entry rather than superseding it.

### 2. `runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`

- **Classification: keep**
- **Evidence**: entry documents the `bufio.Scanner` 1 MB line-limit fix
  (oversized JSONL lines silently dropped) in `internal/db/telemetry_schema.go`.
  Still accurate; 086-S did not touch telemetry rehydration.
- **Relationship to 086-S**: related to a **deferred follow-up** candidate.
  The event-log parsers converged by 086-S still use `bufio.Scanner`
  (`ReadAllEvents`) / line slices (`parseItemLogFile`) and do not yet apply the
  oversized-line handling from this entry. This is explicitly out of 086-S scope
  and recorded as a follow-up in the closure artifact.

## New learning recommended for capture

A `compound` entry (best-practices) captured alongside this refresh:
`docs/compound/best-practices/shared-parser-convergence-observable-skip-2026-07-07.md`
— "Converge divergent per-record parsers onto one shared helper; skip-with-warning
is the correct policy for deterministic per-record parse errors when the skip is
observable and transient/structural failures still propagate."

## Files updated / consolidated / replaced / archived

None (propose mode). Both in-scope entries classified **keep**. One new entry
captured via `compound`.

## Follow-up requiring manual review

- Oversized-line handling for the event-log parsers (apply the pattern from
  entry #2 to `ReadAllEvents` / `parseItemLogFile`) — deferred, recorded in the
  086-S closure artifact.
