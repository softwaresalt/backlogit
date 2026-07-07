---
chunk_strategy: h1-h2-h3
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-07-malformed-jsonl-line-handling-convergence-deliberation.md
title: "Unify malformed-JSONL-line handling across rehydration and doctor parsers"
description: "Converge parseItemLogFile (rehydration) and events.ReadAllEvents (doctor fallback) on a single skip-with-warning policy for malformed JSONL event-log lines, extracted into a shared line-parse helper so the two parsers cannot re-diverge"
topic: "Malformed-JSONL-line handling divergence (F3844849, from 083-S adversarial review finding #8)"
depth: "lightweight"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-07-07-malformed-jsonl-line-handling-convergence-plan.md"
stash_ids:
  - "F3844849"
tags:
  - "data-integrity"
  - "rehydration"
  - "jsonl"
  - "events"
  - "convergence"
---

## Problem Frame

Two independent parsers read per-item JSONL event logs and disagree on how to
handle a malformed (non-JSON) line:

- **Rehydration path** — `parseItemLogFile` (`internal/db/rehydration.go:495`)
  reads the whole file, and on the first `json.Unmarshal` error returns
  `fmt.Errorf("parse item log line: %w", err)`. Its caller
  (`rehydration.go:394-398`) logs `slog.Warn("failed to parse item log", ...)`
  and `return nil` — which **drops every event for that item** from the SQLite
  projection, not just the bad line. A single corrupt line therefore bricks that
  item's rehydration on `backlogit sync`.
- **Doctor fallback path** — `events.ReadAllEvents`
  (`internal/events/reader.go:24`) streams with `bufio.Scanner` and on a
  `json.Unmarshal` error executes `continue` — **silently skipping just the bad
  line** and keeping all the good events, with no log at all.

So the same corrupt log line breaks index rehydration but is tolerated by
doctor. This is a latent, self-healing-only divergence: source `083-S`
adversarial review, Reviewer A finding #8 (P3 MEDIUM, non-blocking), deferred as
a follow-up to "unify the two parsers for byte-for-byte consistency"
(`docs/closure/2026-07-06-083-S-feature-pr-adversarial-review.md:90-93,129-131`).

**Who cares / why:** `backlogit` is an agent-facing tool whose reliability
hinges on `sync` succeeding. A rehydration that fails hard on one corrupt
append-only-log line is the worse failure mode for an autonomous agent than a
skipped line. The projection is a disposable read-model rebuilt in full on every
sync, which further favors resilience over hard-failure.

**Success criteria:** both parsers apply one identical malformed-line policy;
the policy is observable (no silent data loss); valid lines are unaffected; the
malformed-line decision is centralized so the two parsers cannot silently
re-diverge on it (a convergence test guards the remaining caller-owned logging).

**Out of scope:** the pre-existing read-strategy difference (`bufio.Scanner`'s
implicit 1 MB line cap in `ReadAllEvents` vs unbounded `os.ReadFile`+`strings.Split`
in `parseItemLogFile`) is a *separate* oversized-line divergence — see Unresolved
Questions. This task addresses only malformed-JSON content handling.

## Research Findings

Grounded in the compound learnings library (retrieval confidence: **high**):

1. **`docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md`**
   — documents this *exact* divergence class: a JSONL-parser fix was applied to
   `internal/telemetry/` but the independent parser in `internal/db/` was missed,
   silently dropping data during SQLite rehydration. Prevention lesson:
   *"Always search the whole codebase when fixing a pattern"* and *"add a
   cross-package test."* This is a strong argument for a **single shared parse
   helper** rather than two behaviorally-matched copies that can drift again.

2. **`docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`**
   — principle: *"Any operation that builds shared state in phases MUST propagate
   phase failures to callers. Log-and-continue is only appropriate when the
   failure is truly ignorable... A partial SQLite index is neither."* This is the
   core data-integrity tension for this decision. **Reconciliation:** that
   anti-pattern concerns *transient, retryable* batch/transaction failures, where
   erroring lets the caller re-run the rebuild. A **malformed log line is a
   permanent parse failure** — retrying always fails, so erroring gives no
   recovery path and bricks the whole item indefinitely until a human hand-edits
   the log. The two cases are genuinely different; skip-with-warning is correct
   for the permanent case *provided the skip is observable*.

3. **`docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`**
   — precedent: the event *write* path was already centralized into a shared
   `EventWriter` to guarantee serialization consistency. Extracting a shared
   event-*read*/parse helper mirrors that established pattern.

Codebase confirmation: no shared line-parse helper exists today; each function
inlines its own loop. `internal/db/rehydration.go` already imports
`internal/events` and uses package-level `slog`; `internal/events/reader.go`
does not yet import `log/slog`.

## Options Evaluated

### Policy dimension

#### Option A: Skip-with-warning (lenient, skip-and-continue)

A malformed line is skipped and logged (`slog.Warn` with item ID, line number,
and the unmarshal error) in **both** paths; good lines on both sides of the bad
line are preserved.

- **Pros:** one corrupt line cannot brick rehydration/sync; matches the existing
  doctor-fallback behavior and the source doc's explicit recommendation; the
  disposable projection stays maximally complete; adds observability that
  `ReadAllEvents` currently lacks.
- **Cons:** tolerates a corrupt line rather than forcing a fix — mitigated by
  mandatory per-line warn logging so the skip is never silent.
- **Effort:** low. **Fit:** high (agent-tool resilience; source recommendation).

#### Option B: Error (strict, fail-fast)

Both paths return an error on a malformed line, forcing the corrupt log to be
repaired before the index/doctor will proceed.

- **Pros:** loudest possible signal; no partial data is ever indexed.
- **Cons:** a single permanently-corrupt append-only-log line bricks rehydration
  for that item with **no recovery path** (retry re-hits the same line);
  contradicts the source doc recommendation and the doctor-fallback behavior;
  the "no silent data loss" benefit is achievable under Option A via observable
  skips.
- **Effort:** low. **Fit:** low for a resilience-critical agent tool.

### Convergence-approach dimension

#### Approach 1: Behavioral parity (edit both loops independently)

Change `parseItemLogFile`'s `return nil, err` to a warn+skip and add a warn to
`ReadAllEvents`' existing `continue`.

- **Pros:** smallest diff.
- **Cons:** leaves two independent copies of the policy — exactly the shape that
  produced this ticket and the `bufio-scanner-incomplete-fix` regression. High
  risk of silent re-divergence.

#### Approach 2: Shared line-parse helper (recommended)

Add one small helper in `internal/events` that both callers use to classify a
single line, e.g. `ParseEventLine(line, itemID) (Event, ok bool, err error)`:
`ok=false, err=nil` for blank lines (skip silently); `ok=false, err=non-nil` for
malformed JSON (caller logs warn + skips); `ok=true` otherwise. The helper owns
the parse/skip **decision**; each caller owns **observability** (warn with its
own path/item/line context).

- **Pros:** single source of truth — the two parsers cannot re-diverge on the
  malformed-line decision; literal implementation of the source doc's "unify the
  two parsers, byte-for-byte" ask; mirrors the shared-`EventWriter` precedent;
  directly counters the documented incomplete-fix anti-pattern.
- **Cons:** one extra small function and its indirection (proportionate).

## Trade-off Comparison

| Criterion | A: Skip-with-warning | B: Error |
|---|---|---|
| Sync resilience to one bad line | High (skips) | Low (bricks item) |
| Recovery path for permanent corruption | Self-heals on next sync | None until manual fix |
| Silent-data-loss risk | Low (observable skip) | None |
| Matches source doc + doctor behavior | Yes | No |
| Alignment with agent-tool reliability | High | Low |

| Criterion | 1: Parity edits | 2: Shared helper |
|---|---|---|
| Prevents re-divergence | No | Yes |
| Diff size | Smallest | Small |
| Aligns with "unify the parsers" ask | Partial | Full |
| Aligns with prior-art prevention lesson | No | Yes |

## Decision

**Policy: Option A — skip-with-warning (skip-and-continue).**
**Convergence: Approach 2 — a shared `internal/events` line-parse helper** used
by both `parseItemLogFile` and `ReadAllEvents`.

Both parsers converge byte-for-byte on: blank lines skipped silently; malformed
JSON lines skipped **with a `slog.Warn`** carrying the item ID, 1-based line
number, and the unmarshal error; valid lines parsed and retained. This is
strictly better than today for rehydration, which currently discards *all* of an
item's events on one bad line — after this change it retains the good events and
logs the skip.

Rationale: a malformed line is a permanent, non-retryable parse failure, so
erroring (Option B) offers no recovery and bricks `sync` for the item — the
worst outcome for an autonomous agent tool depending on a disposable, rebuildable
projection. The source `083-S` review explicitly recommends converging on the
lenient doctor behavior. The one legitimate concern behind Option B — silent
partial data — is neutralized by making every skip observable, which also closes
the pre-existing silent-skip gap in `ReadAllEvents`. A shared helper (Approach 2)
is chosen over parity edits because the compound library documents this precise
re-divergence failure mode; centralizing the decision is the durable fix.

## Rejected Alternatives

- **Option B (strict error):** rejected — bricks rehydration on one permanently
  corrupt line with no recovery path; contradicts the source recommendation and
  existing doctor behavior. Its silent-data-loss guard is instead met by
  mandatory observable skips under Option A.
- **Approach 1 (independent parity edits):** rejected — preserves two drift-prone
  copies of the policy, the exact anti-pattern behind this ticket and the
  documented `bufio-scanner-incomplete-fix` regression.

## Unresolved Questions

- **Oversized-line divergence (adjacent, out of scope):** `ReadAllEvents` uses
  `bufio.Scanner` with a 1 MB buffer (drops/errors on longer lines) while
  `parseItemLogFile` uses `os.ReadFile`+`strings.Split` (unbounded). A
  string-level shared helper sidesteps this for the malformed-JSON policy, but
  full read-strategy parity is a separate follow-up — candidate stash entry, not
  part of F3844849.
- **Doctor-surfaced skip signal (deferred enhancement):** the batch-failure
  learning warns that agents do not read warn logs. A structured doctor signal
  aggregating skipped-line counts would be stronger observability than a warn
  alone. Deferred: per-line `slog.Warn` in both paths is the proportionate
  observability for this low-risk task; a doctor signal is a larger future
  enhancement.

## Risks and Mitigations

- **Risk:** skip-with-warning masks a systematically corrupt log. **Mitigation:**
  per-line warn with item ID + line number + reason in both paths makes each skip
  traceable; the projection is rebuilt every sync so recovery is automatic once
  the log is repaired.
- **Risk:** helper extraction subtly changes valid-line behavior. **Mitigation:**
  test-first — a red test proving the current divergence (same malformed line:
  one errors, one skips), a green test proving both converge on skip-and-continue
  and retain surrounding valid events, and a test asserting the skip is logged.
- **Risk:** re-divergence in a future edit. **Mitigation:** the shared helper is
  the single decision point; a convergence test exercising both callers guards it.
