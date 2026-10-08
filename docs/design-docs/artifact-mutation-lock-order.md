---
chunk_strategy: h1-h2-h3
description: "Canonical lock order, stale-write guard obligations, and verification bounds for artifact-mutation writers."
doc_type: design
schema_version: "1.0"
source: docs/design-docs/artifact-mutation-lock-order.md
title: "Artifact mutation lock order and archived_status guard obligations"
docline:
  shipment_id: 153-S
  feature_id: 172-F
  task_id: 172.001-T
---

## Context

Artifact-mutation writers touch two cross-process lock domains:

* **B: artifact-mutation lock** from `lockArtifactMutation` and
  `lockArtifactMutations`
* **C: item-log lock** from `events.LockItemLogCrossProcess` and
  `EventWriter.AppendEvent`

The canonical order is **C before B** for any writer that must hold both locks.
A writer must not acquire C while it is already holding B for the same item. A
writer may avoid overlap by completing the B-held mutation before acquiring C for
a best-effort event append, provided the item-log event remains causally ordered
with the mutation it records.

## Current writer lock order

| Writer | Current order | Notes and bound |
|---|---|---|
| `ArchiveItem` | B, released, then C | `lockArchiveGovernance` holds B for the item and cascade set through the file move and DB update. 172.002-T releases the archive governance locks before the best-effort archive event acquires C, so B and C never overlap. The event is appended after the mutation it records, which preserves causal order. |
| `AssociateCommit` | C, then B | The function acquires C before the compensating frontmatter persist. The later persist uses B while C is already held, matching the canonical order. |
| `RemoveArtifactLink` | B only | The writer loads from Markdown before B and persists through the artifact write path. It does not append an item-log event. 172.005-T guards the persist with `guardArchivedStatusUnchangedSince`. |
| `BulkUpdateStatus` | Lifecycle global, then B per item | The writer takes the shipment lifecycle global lock, loads each artifact before B, and persists with B. It does not acquire C. 172.006-T guards each persist with `guardArchivedStatusUnchangedSince` and reports stale items through the typed `BulkUpdateResult` conflict details declared by 172.011-T. |
| Governed reconcile-to-shipped transaction | C, then B | The reconcile path is the governed majority path for shipment repair. It owns the durable item-log event before mutating artifact state. |
| `ReconcileArchivedLifecycle` | Per-item B, never held across C | 172.003-T removes the batch-wide B. `UnarchiveItem`, the status set step, and `ArchiveItem` each take their own B, and none holds B while acquiring C. The set step re-reads the artifact under B and refuses with `ErrShipmentConflict` when the queued status no longer matches the unarchived status. |
| `archiveDescendants` cascade | Batch B over parent and descendants | Cascade locks are gathered ancestor-before-descendant and sorted by lock key through `lockArtifactMutations`. Inner calls reuse context-held B rather than taking a second independent B lock. |

## Load-bearing invariants

### Context reentrancy

`lockArtifactMutations` records held artifact locks in context. Inner archive,
unarchive, and persist calls on the same item skip reacquiring a B lock already
held by the same call tree. This prevents self-deadlock in cascade flows.

### Ancestor and sorted batch order

Multi-item B acquisition must remain deterministic. Cascade callers enumerate the
intended item set, and `lockArtifactMutations` deduplicates and sorts IDs before
acquiring per-item locks. This avoids item-to-item inversion across concurrent
batch writers. Parent/child semantics are preserved by the caller while the lock
primitive keeps a stable acquisition order.

### Causal item-log ordering

For a given item, C serializes observable JSONL events. Moving C relative to B
must not reorder, duplicate, or drop the event sequence that an operator can read
from the item log. If a mutation records an event, the event must remain causally
attached to the mutation: no event may be visible as completed before the guarded
mutation exists, and no successful governed mutation may lose its required event.
Best-effort events, such as archive logging, may continue to warn and skip on log
failure when that is the existing contract.

## Verification strategy

The primary verification method is deterministic barrier instrumentation at B and
C acquisition points. A test must be able to force the AB-BA interleaving and
assert bounded behavior without depending on scheduler luck. `go test -race` is a
secondary diagnostic only; a passing race run is not proof that a lock-order
inversion is absent.

The existing `persistArtifactPreLockHook` is not sufficient for Unit 1 lock-order
proofs because it fires before B acquisition. Task 172.013-T declares nil-in-production
acquisition hooks for both locks: `artifactMutationLockBarrierHook` for B and
`itemLogLockBarrierHook` for C, which fires immediately before C is acquired.
Task 172.004-T makes the B hook fire immediately
after B is acquired, in both `lockArtifactMutation` and `lockArtifactMutations`, so
tests can pause a writer while it holds B and drive the interleaving
deterministically.

## archived_status shipped clobber audit

| Writer | Can clobber `archived_status: shipped`? | Status |
|---|---|---|
| `AssociateCommit` | Yes, snapshot-before-lock writer | Already guarded by `guardArchivedStatusUnchangedSince`. |
| `AddArtifactLink` | Yes, snapshot-before-lock writer | Already guarded by `guardArchivedStatusUnchangedSince`. |
| `RemoveArtifactLink` | Yes, snapshot-before-lock writer | Guarded by 172.005-T; GREEN regression in 172.007-T. |
| `BulkUpdateStatus` | Yes, snapshot-before-lock writer | Guarded with typed conflict population by 172.006-T; GREEN regression in 172.010-T. 172.011-T declares the result fields and 172.012-T surfaces them in the CLI. |
| `ArchiveItem` | Can stamp or preserve archive provenance | Existing archive reconcile guard preserves non-empty reconciled `archived_status`; Unit 1 handles lock-order risk, not the CAS guard. |
| `ReconcileArchivedLifecycle` | Writes the governed target `archived_status` | Governed source of truth for the repair, not a clobbering stale writer. The set step is compare-and-set on the queued status (172.003-T). |
| `archiveDescendants` cascade | Can archive descendants and stamp provenance | Covered by archive governance and cascade ordering; no new CAS guard is planned in this shipment. |
| `UpdateArtifactWithGate` / direct update path | Can mutate artifacts through locked update flow | Existing shipped-transition and blocked-shipment guards apply. General content-staleness protection is out of scope for this shipment. |

The guard obligation is deliberately narrow: it protects only the
`archived_status` field from stale overwrite of a freshly reconciled `shipped`
value. General lost-update protection for arbitrary artifact fields requires a
revision, canonical-content CAS, or reload-and-merge design and remains out of
scope.

## Residual and bound

* `ArchiveItem` no longer holds B while acquiring C (172.002-T). The archive
  event append remains best-effort: on log failure it warns and the archive
  still succeeds.
* `ReconcileArchivedLifecycle` no longer holds a batch-wide B (172.003-T). Its
  remaining bound is the window between the compare-and-set status step and the
  re-archive. Another writer can change the queued status inside that window,
  and `ArchiveItem` then stamps `archived_status` from the status it finds. The
  set step detects a change made before it runs. A change made after it is
  recorded faithfully by the re-archive rather than overwritten.
* Future snapshot-before-lock writers that can persist `archived_status` must opt
  into `guardArchivedStatusUnchangedSince` or an equivalent source-of-truth
  Markdown comparison before shipment 153-S can claim the class is closed.
