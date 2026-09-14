---
chunk_strategy: h1-h2-h3
description: "Implementation plan for concurrency-safety hardening of backlogit artifact-mutation writers: canonical lock order (FE440C62) then shared stale-write CAS guard generalization (1E0C2251)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-concurrency-writer-hardening-plan.md
title: "Implementation Plan: Concurrency-safety hardening for artifact-mutation writers"
docline:
    stash_id: FE440C62,1E0C2251
    status: approved
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Eliminate two coupled concurrency defects in `internal/core` artifact-mutation
writers without changing the governed reconcile-to-shipped contract:

1. **FE440C62** — remove the `ArchiveItem` B→C lock-order inversion so all
   same-item artifact writers acquire locks in one canonical order.
2. **1E0C2251** — extend the `guardArchivedStatusUnchangedSince` stale-write CAS
   guard to the remaining snapshot-before-lock writers (`RemoveArtifactLink`,
   `BulkUpdateStatus`).

Source: `docs/decisions/2026-09-13-concurrency-writer-hardening-deliberation.md`.

## Requires plan hardening: yes

Concurrency, lock-ordering, and shared-mutation-primitive changes are inherently
high-risk (deadlock / stale-write / availability). See `## Plan Hardening`.

## Implementation Units

### Unit 1 — Canonical lock order (FE440C62)

* **U1.1** Characterize + document the canonical lock-acquisition order across
  **all** artifact-mutation writers — `ArchiveItem`, `AssociateCommit`,
  `RemoveArtifactLink`, `BulkUpdateStatus`, the reconcile transaction, **and the
  batch/multi-item paths**: `ReconcileArchivedLifecycle`
  (`internal/core/archive_reconcile.go:119`, which holds `lockArtifactMutations`
  (B) across the whole batch and then calls `ArchiveItem`/`UnarchiveItem` on the
  same item ids inside `reconcileArchivedItem`) and `ArchiveItem`'s recursive
  cascade (`archiveDescendants`). Produce an analysis note that (a) enumerates
  each writer's current lock order, (b) fixes the canonical order as **C-then-B**
  (matching `AssociateCommit` and the governed reconcile transaction — the
  majority and governed path), (c) states the **load-bearing ordering
  invariants** that keep the batch and cascade paths safe: `lockArtifactMutations`
  is context-reentrant (so an inner `ArchiveItem` under an outer held B does not
  self-deadlock) and lock acquisition is ancestor-before-descendant / sorted-batch
  so the anti-inversion protection holds; and (d) records the residual-and-bound
  for any path that cannot be made strictly C-then-B. Domain: docs/analysis.
* **U1.2** Reorder `ArchiveItem` (`internal/core/archive.go`) so it is never
  holding B while acquiring C. **Preferred strategy**: move the best-effort
  item-log (C) JSONL append out of the B-held region so the two locks are never
  held overlapping in B→C order — this eliminates the inversion **without**
  widening the C critical section and **preserves** the existing *non-fatal*
  append-failure semantics (warn + skip; archive still succeeds). If instead C is
  acquired before B, the note (U1.1) must record the widened C critical section
  and its bounded-contention trade-off. Specify the chosen C scope and failure
  handling explicitly. Domain: code. Depends on U1.1.
* **U1.3** Resolve the batch-path residual: ensure `ReconcileArchivedLifecycle`'s
  outer-B-then-inner-`ArchiveItem`-C shape does not reintroduce a same-item B→C
  inversion after U1.2. **The restructure — acquire C before the batch B (or
  otherwise avoid holding B while acquiring C in the batch path) — is REQUIRED
  unless U1.1's analysis proves no concurrent same-item C-then-B writer can exist
  for the reconcile-lifecycle path.** Context-reentrancy of `lockArtifactMutations`
  prevents self-deadlock on inner B re-acquisition but does NOT cure the cross-lock
  AB-BA ordering against C-then-B writers, so the reentrancy invariant alone is
  insufficient justification to skip the restructure. Domain: code. Depends on
  U1.2.
* **U1.4** Race regression test: `ArchiveItem` vs
  `ReconcileShipmentToShipped`/`AssociateCommit` on the same item id, and the
  `ReconcileArchivedLifecycle` batch path vs a concurrent same-item writer,
  `-race`, asserting no inversion and bounded, deterministic contention
  resolution. Domain: tests. Depends on U1.2, U1.3.

### Unit 2 — Generalized stale-write CAS guard (1E0C2251), depends on Unit 1

* **U2.1** Apply `guardArchivedStatusUnchangedSince` to `RemoveArtifactLink`
  (`internal/core/artifacts.go:981`). NOTE: `RemoveArtifactLink` persists via
  `persistArtifactWithoutDBOnlyLinks` (which strips DB-only link edges from the
  Markdown write), **not** `persistArtifactWithGuard`. Introduce a combined
  **guarded + without-DB-only-links** persist variant (or thread the guard param
  through `persistArtifactWithoutDBOnlyLinks`) so both the stale-write guard AND
  the DB-only-link stripping are preserved. Snapshot `preLockArchivedStatus`
  before the `findArtifact`→persist window. Ensure the early "link not present /
  nothing removed" branch (which calls `db.RemoveLink` directly with no artifact
  persist) is **not** routed through the guard. Domain: code. Depends on U1.2.
  (Scope check: touches `artifacts.go` + the shared persist helper — still ≤ 3
  files; the combined variant is a small helper addition.)
* **U2.2** Apply the guard to `BulkUpdateStatus` (`internal/core/queue.go:404`)
  via `persistArtifactWithGuard` with a per-item pre-lock `preLockArchivedStatus`
  snapshot. Observable behavior change to assert: a per-item stale-write
  (`ErrShipmentConflict`) now surfaces as a `BulkUpdateResult.Failed` entry
  (fail-closed) rather than aborting or silently clobbering the batch. Domain:
  code. Depends on U1.2.
* **U2.3** Stale-write regression tests: concurrent `RemoveArtifactLink` /
  `BulkUpdateStatus` vs reconcile-to-shipped cannot clobber
  `archived_status: shipped`; `ErrNotFound` is not treated as success;
  `BulkUpdateStatus` surfaces per-item conflicts as `Failed` entries; the
  `RemoveArtifactLink` DB-only-link stripping is still honored under the guarded
  path. Domain: tests. Depends on U2.1, U2.2.

## Constitution Check

* **Single-domain tasks**: each unit is code OR docs OR tests. Pass.
* **2-hour rule**: each unit < 3 files / < 5 functions / < 4 test scenarios.
  Pass.
* **Workspace containment (P-017)**: all edits in `internal/core`; no autoharness
  change. Pass.
* **No governed-contract regression**: reconcile-to-shipped observable behavior
  unchanged; only lock order + guard coverage change. Pass.
* **Fail-closed**: CAS guard rejects stale writes; ErrNotFound must not be
  swallowed as success. Pass.

Constitution Check: pass

## Plan Hardening

* **ProposedAction**: reorder `ArchiveItem` locks C→B. **ActionRisk**: high
  (lock-order change on a system-wide primitive). Mitigation: U1.1 canonical-
  order analysis precedes the change; U1.3 `-race` regression proves no new
  inversion and that contention stays bounded (documented ~3s bounded waits, no
  unbreakable deadlock).
* **ProposedAction**: generalize CAS guard to two more writers. **ActionRisk**:
  medium (could surface latent stale-write callers as new CAS rejections).
  Mitigation: guard only rejects genuinely-stale `archived_status` transitions;
  U2.3 covers the ErrNotFound-as-success failure mode that the in-scope 167-F
  fix already corrected for the guard itself.
* **Rollback**: each unit is independently revertible; Unit 2 depends on Unit 1
  so a Unit 1 revert implies reverting Unit 2.
* **Blast-radius bound**: no change to public API signatures; no new lock
  primitives introduced.
* **Residual risk**: the fully-automatic shared-persist-path enforcement
  (deliberation Option C) is intentionally deferred (see Follow-ups); until then,
  any *future* new snapshot-before-lock writer must opt in — U1.1's canonical-
  order note documents this obligation.

## Verification

* `go test ./internal/core/... -race` including the new U1.4 and U2.3 tests.
* Existing `TestU20_*` concurrency suite remains green.

## Follow-ups (out of this shipment)

* Option C: make the CAS guard automatic for **every** snapshot-before-lock
  writer via shared persist-path enforcement (larger architectural pass).
* Related deferred entry A0C733C6 (`findArtifact` TOCTOU) is a **separate**
  shared-infrastructure hardening and is NOT in this shipment's scope.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 1. Personas dispatched: Concurrency Reviewer (specialist), Correctness
Reviewer (cross-cut). Coverage complete.

**Gate rationale**: one P1 finding → FAIL per rubric (any P0/P1).

Findings:
* **P1 (Concurrency)** — `ReconcileArchivedLifecycle`
  (`archive_reconcile.go:119`) holds batch lock B then calls `ArchiveItem` on the
  same item ids; omitted from Unit 1, so canonicalizing `ArchiveItem` alone leaves
  a residual same-item B→C inversion (bounded contention, not a hang) on which
  Unit 2's CAS guard is premised.
* **P2 (Concurrency)** — U2.1 mis-stated the persist path: `RemoveArtifactLink`
  uses `persistArtifactWithoutDBOnlyLinks`, not `persistArtifactWithGuard`; needs
  a combined guarded + without-DB-only-links variant.
* **P2 (Concurrency)** — U1.2 under-specified the C-lock scope/failure semantics
  (late best-effort C vs early mandatory C widening the critical section).
* **P2 (Concurrency)** — U1.1 should cover cascade/multi-item ordering invariants.
* **P3 (Correctness)** — U2→U1 edge is rollback-coupling, not a build dependency.

Plan hardening required: yes; present. Remediation applied in this revision (see
revised Units 1–2: added U1.1 batch/cascade coverage + invariants, U1.2 C-scope
+ failure-semantics preservation, new U1.3 batch-path residual task, corrected
U2.1 persist path, U2.2 behavior-change assertion). Re-review below.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2. Personas: Concurrency Reviewer (specialist, re-review). Coverage
complete. Prior P1 (batch B→C inversion) confirmed resolved: now covered by U1.1
(analysis + invariants), U1.3 (code remediation, restructure now REQUIRED unless
proven unnecessary), and U1.4 (batch-path -race regression). Prior P2s (persist
path, C-scope/failure semantics, cascade invariants) all resolved.

**Gate rationale**: no P0/P1/P2 remain after the U1.3 tightening; residual items
are P3 implementation-detail advisories. Plan hardening required: yes; present
and adequate.

Findings (residual, non-blocking):
* P3 — AB-BA deadlocks are timing-flaky; U1.4 batch-path -race test is a partial
  (not exhaustive) mitigation. Accepted; the U1.3 restructure is the primary
  control.

<!-- plan-review-attempt: 2 -->
