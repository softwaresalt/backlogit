---
title: "Compacted Memory — 2026-08-17 (143-F / 127-S Restage and Review Cycle 1)"
doc_type: memory
schema_version: "1.0"
ingested_at: "2026-08-20T05:00:00Z"
---

Source files (archived to `docs/archive/memory/2026-08-17/`):

* `stage-restage-143-shipment-audit-log-memory.md`
* `stage-pr366-review-cycle1-memory.md`

## Why This Session Existed

A prior staging attempt (`chore/stage-143-shipment-audit-log-reconciled`,
`2188bab2`) tripped the 3-cycle circuit breaker: the plan asserted a
baseline (`ws.shipmentEventAppend`, an error-returning shipped path) that
did not exist on `origin/main`. Operator authorized a fresh replan from
clean base `3ec95ee3` in a new worktree `.worktrees/143-restage`. The old
worktree `.worktrees/127-s-reconcile` (a separate uncommitted implementation
attempt) was explicitly **not touched**, preserved byte-for-byte.

## 143-F / 127-S — Shipment Shipped-Event Audit Log (COMPLETE, shipped — see 2026-08-18/19 entries for merge)

* Deliberation `059-DL`, feature `143-F` (11 tasks `143.001-T`…`143.011-T`,
  later 12 after `143.012-T` split out in review), shipment `127-S`.
* **Detection ships now; prevention deferred** to stash `47B48DB0` — **this
  is the direct precursor to feature 144-F / shipment 128-S** (the
  shipped-transition prevention hardening just merged as PR #370 in this
  same closure session). `47B48DB0` also absorbed the deliberation's
  `UpdateArtifactWithGate` minimum-floor idea. The 143-F guarantee is
  path-scoped to the governed `ShipShipment` archival path only — NOT
  universal prevention (that's what 144-F later added).
* **Classification contract** (corrected during PR #366 review cycle 1
  after being wrong twice): `EventWriter.AppendEvent` returns only `error`
  and discards the `fmt.Fprintf` byte count, so pre-write status is
  unobservable. Only a **proven** not-applied outcome (lock failure inside
  the appender, or a writer error explicitly tagged
  `ErrWriteNotApplied`) may compensate; every other append error — writer
  errors tagged `ErrWriteIndeterminate` AND any untagged error — is
  `indeterminate` and must NOT be compensated. Indeterminate-first
  precedence: an error carrying both sentinels can never be compensated.
  **This is the same "never compensate an indeterminate write" principle
  documented in the durable-writes two-class-contract compound learning**
  (`docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`)
  — 143-F is a concrete application of it to the shipped-event append path.
* Accepted residual cost: a genuine pre-write `open` failure (default
  non-durable config) strands the shipment as `shipped`-and-unarchived
  instead of reverting — detected by the new `shipped_unarchived_residue`
  doctor finding (distinct from `missing_shipped_event`, which flags an
  *archived* shipment with `archived_status: shipped` and no shipped
  event). **These two `doctor` findings are directly relevant to future
  shipment-reconcile work** — the shipment-reconcile skill's post-mode
  Step 0 halted-archival branch (`mutation_partial`/`indeterminate`/
  `failed_step: shipped-event-append`) exists because of this contract.
* Corrected test-first task graph, structural on both tracks (no scaffold
  task changes production before its RED harness exists):
  `143.001-T`(RED)→`143.002-T`(appender)→`143.003-T`(fail-closed routing)→
  `143.004-T`(classify+halt)→`143.012-T`(compensation, split out from
  original Unit 4 for exceeding the 2-hour/5-function rule); parallel track
  `143.005-T`(RED)→`143.006-T`(missing_shipped_event)→`143.007-T`
  (shipped_unarchived_residue); surfaces `143.008-T`(CLI)→`143.009-T`(MCP)→
  `143.010-T`(recovery guidance); `143.011-T`(contract doc, P-007, reconcile
  skill, Ship agent) as sink.
* Gate rigor: plan review 3 cycles (FAIL P0=3/P1=16 → FAIL P0=2/P1=12 → PASS),
  adversarial review PASS (0 HIGH-confidence P0/P1; 2 MEDIUM fixed; 13/15 LOW
  fixed, 2 rejected with rationale). PR #366 Copilot review cycle 1: 9
  comments, ALL accepted — most substantively, comment #1 forced the
  classification-contract correction above (the writer API cannot be
  expanded to expose short-write detection), and comment #9 forced the
  144-F/47B48DB0 scope split (universal prevention explicitly deferred, not
  claimed).

## Designs Proposed Then Rejected (valuable — avoid re-proposing)

1. Hoisting the item-log lock to the top of `ShipShipment`'s locked
   closure — rejected: makes the ownership marker outlive the lock,
   `restoreShipArtifacts` would rewrite an append-only log without
   exclusion, inverts lock order vs. `lockArtifactMutations`.
2. Retrying the append before classifying — rejected: `AppendEvent` is
   explicitly not safe to blindly retry (risks a duplicate shipped event).
3. Nilling `releaseArtifactLocks` in its own defer — rejected: LIFO release
   order already runs before the non-member-feature fallback; would make
   the fallback unconditionally dead, silently deleting the 133.004-T
   guarantee. Fixed by swapping the two defer registrations instead.
4. Real-filesystem injection in the RED harness (planting a directory at
   the log/lock path) — rejected: aborts in `snapshotShipArtifacts` before
   the append runs, so the scenario could never fail for the right reason.
   Seam injection used instead.
5. An `internal/core` test proving CLI-vs-MCP parity — rejected: the
   package cannot reach either surface; the claim would restate its premise.

## Incident (recovery pattern worth remembering)

A PowerShell array-splatting bug in a body-normalization helper emptied the
template sections of all 11 task files + `127-S` after creation. Recovered
because no `sync` had run yet — descriptions were still in the SQLite
index (`items.description`); re-authored acceptance criteria/implementation
notes inline from that index data, then re-verified all 13 artifacts
(`begin=3 end=3 empty=0`). **Lesson**: the SQLite index can serve as a
recovery source for accidentally-corrupted queue files, provided no `sync`
has overwritten it since the corruption.

## Status

143-F / 127-S subsequently completed the full Ship lifecycle and closed
(see the 2026-08-18/19 compacted memory below for the merge and closure
detail — PR #368 was 127-S's post-merge closure). Stash `47B48DB0` was
harvested into feature 144-F, which is the subject of THIS closure
session's shipment 128-S (merged as PR #370, `461b670c`).
