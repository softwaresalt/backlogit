---
chunk_strategy: h1-h2-h3
description: "Deliberation for the concurrency-safety hardening group (FE440C62 lock-order inversion + 1E0C2251 non-universal CAS guard) across backlogit artifact-mutation writers"
doc_type: learning
schema_version: "1.0"
source: docs/decisions/2026-09-13-concurrency-writer-hardening-deliberation.md
title: "Deliberation: Concurrency-safety hardening for artifact-mutation writers"
docline:
    stash_id: FE440C62,1E0C2251
    status: decided
    created_at: 2026-09-13T17:31:00Z
---

## Deliberation: Concurrency-safety hardening for artifact-mutation writers

**Depth**: standard. **Route**: `deliberate` (both entries carry
`requires deliberation: true`). **Grouping rationale** (Step 1.5): FE440C62 and
1E0C2251 touch the **same code surface** — the artifact-mutation persist path
(`persistArtifactWithGuard`, `lockArtifactMutations`, the item-log lock) and the
same writer set (`ArchiveItem`, `AssociateCommit`, `RemoveArtifactLink`,
`BulkUpdateStatus`, the reconcile transaction). Shipping them as one covering
feature / one PR avoids two PRs editing the same lines and lets the CAS
generalization build on a canonical lock order.

### Problem Frame

Two independently-filed but coupled reliability defects in `internal/core`:

* **FE440C62 (ABBA lock-order inversion).** `ArchiveItem`
  (`internal/core/archive.go:98`) acquires artifact-mutation lock **B** first,
  then item-log lock **C** near function end (B→C). `AssociateCommit`
  (`internal/core/commits.go:56`) and the governed reconcile transaction
  (`internal/core/shipment_reconcile_transaction.go`) both acquire **C→B**. When
  `ArchiveItem` and `ReconcileShipmentToShipped` target the **same item**
  concurrently this is a real lock-order inversion. Empirically (30+ `-race`
  repeats, `TestU20_ReconcileShipmentToShippedConcurrency/ReconcileVsArchiveItem`)
  it resolves to **bounded contention** (one side gets `ErrGateInProgress` /
  item-log-lock-busy from ~3s bounded waits), **not** an unbreakable deadlock —
  so **not P0**, but a legitimate spurious-failure / availability risk and a
  system-wide lock-ordering inconsistency.
* **1E0C2251 (non-universal CAS guard).** `guardArchivedStatusUnchangedSince`
  (`internal/core/shipment_reconcile_cas.go:50`) is an **opt-in** stale-write
  guard, wired only where callers explicitly pass it (`AssociateCommit`
  `commits.go:135`, `dependencies.go:100/191`, reconcile). Other pre-existing
  **snapshot-before-lock** writers with the identical read-before-persist shape
  — `RemoveArtifactLink` (`artifacts.go:981`) and `BulkUpdateStatus`
  (`queue.go:404`) — do NOT apply it, so a concurrent call through either can
  overwrite a freshly-reconciled `archived_status: shipped` back to a stale
  value.

Verified in-repo: the guard is present at `artifacts.go:966` (a *different*
function above `RemoveArtifactLink` at 981) and in `commits.go`/`dependencies.go`,
but NOT in `RemoveArtifactLink`/`BulkUpdateStatus`. Contracts are real.

### Real-Contract Dependency (NOT numeric adjacency)

The stale-write CAS generalization (1E0C2251) should land on a **canonical,
consistent lock-acquisition order** (FE440C62). A shared persist-path guard that
assumes an ordering is only sound once the ordering is uniform. Therefore
**FE440C62 → 1E0C2251** is a genuine contract edge: canonicalize lock order
first, then generalize the CAS guard across the same writers. This is derived
from the shared persist path, not from ID numbering.

### Options

**Option A — One covering feature, canonical-order-first, then generalize CAS
(CHOSEN).** Sub-work 1: characterize + document the canonical lock order, then
reorder `ArchiveItem` to C→B with a race regression test. Sub-work 2 (depends on
Sub-work 1): apply the shared CAS guard to `RemoveArtifactLink` and
`BulkUpdateStatus`, with stale-write regression tests. One shipment / one PR.

* Pros: single PR over one code surface; no conflicting edits; the dependency
  edge encodes the real ordering; each task stays single-domain and 2-hour-sized.
* Cons: slightly larger shipment.

**Option B — Two separate features / two shipments with a shipment-level edge.**

* Pros: finer-grained release units.
* Cons: two PRs editing the **same files/functions** → merge conflicts and
  redundant review; contradicts Step 1.5 contextual-grouping (same surface).
  Rejected.

**Option C — Single "make the guard automatic for every snapshot-before-lock
writer" refactor (shared persist-path enforcement).** Attractive as the durable
end-state, but it is a larger architectural change touching every writer at once;
too big for the 2-hour rule as a single unit and higher blast radius than the
bounded defect fixes the operator authorized. Recorded as a **follow-up**, not
this shipment's scope.

### Decision

Adopt **Option A**. Covering feature: *Concurrency-safety hardening for
artifact-mutation writers*. Internal dependency FE440C62-work → 1E0C2251-work.
Independent root shipment (no real edge into the 141-S fault-line chain; the
operator's reliability-first ordering is a scheduling preference, expressed in
the session summary, not a DAG edge). Option C (automatic shared enforcement)
recorded as a future follow-up in the plan's Follow-ups section.

### Scope Boundary

In-repo `internal/core` only. No autoharness change. No behavioral change to the
reconcile-to-shipped governed contract other than making concurrent writers
respect the same lock order and stale-write guard.
