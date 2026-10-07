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
| `ArchiveItem` | B, then C | `lockArchiveGovernance` holds B for the item and cascade set, then the best-effort archive event acquires C near the end of the function. This is the known inversion targeted by later lock-order tasks. |
| `AssociateCommit` | C, then B | The function acquires C before the compensating frontmatter persist. The later persist uses B while C is already held, matching the canonical order. |
| `RemoveArtifactLink` | B only | The writer loads from Markdown before B and persists through the artifact write path. It does not append an item-log event. The stale `archived_status` guard is added by 172.005-T. |
| `BulkUpdateStatus` | Lifecycle global, then B per item | The writer takes the shipment lifecycle global lock, loads each artifact before B, and persists with B. It does not acquire C. The stale `archived_status` guard and typed conflict population are later tasks. |
| Governed reconcile-to-shipped transaction | C, then B | The reconcile path is the governed majority path for shipment repair. It owns the durable item-log event before mutating artifact state. |
| `ReconcileArchivedLifecycle` | Batch B, then inner archive paths | The batch path takes `lockArtifactMutations` for the affected set and then calls item-level archive/unarchive helpers. Context reentrancy prevents self-deadlock on B, but it does not by itself prove safety against a same-item C-then-B writer. Later Unit 1 tasks must remove or bound any remaining B-to-C overlap. |
| `archiveDescendants` cascade | Batch B over parent and descendants | Cascade locks are gathered ancestor-before-descendant and sorted by lock key through `lockArtifactMutations`. Inner calls reuse context-held B rather than taking a second independent B lock. |

## Load-bearing invariants

### Context reentrancy

`lockArtifactMutations` records held artifact locks in context. Inner archive,
unarchive, and persist calls on the same item skip reacquiring a B lock already
held by the same call tree. This prevents self-deadlock in cascade and reconcile
batch flows.

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
proofs because it fires before B acquisition and there is no matching C acquisition
hook. Task 172.013-T adds no-op acquisition hooks for both B and C so later tests
can drive the interleaving deterministically.

## archived_status shipped clobber audit

| Writer | Can clobber `archived_status: shipped`? | Status |
|---|---|---|
| `AssociateCommit` | Yes, snapshot-before-lock writer | Already guarded by `guardArchivedStatusUnchangedSince`. |
| `AddArtifactLink` | Yes, snapshot-before-lock writer | Already guarded by `guardArchivedStatusUnchangedSince`. |
| `RemoveArtifactLink` | Yes, snapshot-before-lock writer | Planned guard in 172.005-T; GREEN regression in 172.007-T. |
| `BulkUpdateStatus` | Yes, snapshot-before-lock writer | Planned guard and typed conflict population in 172.006-T; GREEN regression in 172.010-T. The compatibility surface is prepared by 172.011-T and 172.012-T. |
| `ArchiveItem` | Can stamp or preserve archive provenance | Existing archive reconcile guard preserves non-empty reconciled `archived_status`; Unit 1 handles lock-order risk, not the CAS guard. |
| `ReconcileArchivedLifecycle` | Writes the governed target `archived_status` | Governed source of truth for the repair, not a clobbering stale writer. Batch lock-order residual is handled by later Unit 1 tasks. |
| `archiveDescendants` cascade | Can archive descendants and stamp provenance | Covered by archive governance and cascade ordering; no new CAS guard is planned in this shipment. |
| `UpdateArtifactWithGate` / direct update path | Can mutate artifacts through locked update flow | Existing shipped-transition and blocked-shipment guards apply. General content-staleness protection is out of scope for this shipment. |

The guard obligation is deliberately narrow: it protects only the
`archived_status` field from stale overwrite of a freshly reconciled `shipped`
value. General lost-update protection for arbitrary artifact fields requires a
revision, canonical-content CAS, or reload-and-merge design and remains out of
scope.

## Residual and bound

* `ArchiveItem` currently retains the B-to-C inversion until the later Unit 1
  implementation tasks move or bound the C append.
* `ReconcileArchivedLifecycle` remains a batch-path residual until later Unit 1
  tasks prove or remove any same-item B-to-C overlap.
* Future snapshot-before-lock writers that can persist `archived_status` must opt
  into `guardArchivedStatusUnchangedSince` or an equivalent source-of-truth
  Markdown comparison before shipment 153-S can claim the class is closed.
