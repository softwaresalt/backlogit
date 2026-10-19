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

Artifact-mutation writers touch three cross-process lock domains, in addition to
the shipment lifecycle global lock:

* **A: shipment membership lock** from `lockShipmentMembership` and
  `lockArchiveShipmentMemberships`
* **B: artifact-mutation lock** from `lockArtifactMutation` and
  `lockArtifactMutations`
* **C: item-log lock** from `events.LockItemLogCrossProcess` and
  `EventWriter.AppendEvent`

The canonical order is **lifecycle global, then A, then C, then B** for any
writer that must hold more than one of them. Within C and within B, a writer
acquires locks in sorted item-ID order. 200.002-T fixes this order as the single
rule that ShipShipment (200.005-T) and standalone and cascade `ArchiveItem`
(200.007-T) follow.

A writer must not acquire C while it is already holding B, unless the call tree
already holds that C in its context. A writer that records an event for a
mutation appends it under the C it took before B, so the item-log event stays
causally ordered with the mutation it records. Releasing B before acquiring C for
a best-effort event append, the 172.002-T approach, is superseded by 200.007-T
because it leaves a gap for a concurrent C-then-B writer.

## Current writer lock order

| Writer | Current order | Notes and bound |
|---|---|---|
| `ArchiveItem` | Lifecycle global, A, C, then B | 200.007-T. `lockArchiveGovernance` takes C for the archive set (the item and, for a cascade, its descendants) in sorted item-ID order between the membership locks and `lockArtifactMutations`. The archive event is appended under that C after the mutation, and the locks are released B first, then C. A C request while B is held and not context-held fails closed. This supersedes 172.002-T, which released B before it acquired C and left a gap for a concurrent C-then-B writer. |
| `AssociateCommit` | C, then B | The function acquires C before the compensating frontmatter persist. The later persist uses B while C is already held, matching the canonical order. |
| `RemoveArtifactLink` | B only | The writer loads from Markdown before B and persists through the artifact write path. It does not append an item-log event. 172.005-T guards the persist with `guardArchivedStatusUnchangedSince`. |
| `BulkUpdateStatus` | Lifecycle global, then B per item | The writer takes the shipment lifecycle global lock, loads each artifact before B, and persists with B. It does not acquire C. 172.006-T guards each persist with `guardArchivedStatusUnchangedSince` and reports stale items through the typed `BulkUpdateResult` conflict details declared by 172.011-T. |
| Governed reconcile-to-shipped transaction | C, then B | The reconcile path is the governed majority path for shipment repair. It owns the durable item-log event before mutating artifact state. |
| `ReconcileArchivedLifecycle` | Per-item B, never held across C | 172.003-T removes the batch-wide B. `UnarchiveItem`, the status set step, and `ArchiveItem` each take their own B, and none holds B while acquiring C. The set step re-reads the artifact under B and refuses with `ErrShipmentConflict` when the queued status no longer matches the unarchived status. |
| `archiveDescendants` cascade | C over parent and descendants, then batch B | 200.007-T. The outermost `ArchiveItem` takes C for the whole cascade set in sorted item-ID order, then batch B, which `lockArtifactMutations` sorts by lock key. Inner calls reuse the context-held C and B and never acquire either themselves. Each cascaded child appends its own `archived` event under the reused C right after its mutation, so the deferred event sink is retired. |
| `ShipShipment` | Lifecycle global, A, C, then B | 200.005-T. Under the lifecycle global lock and the shipment membership lock, ShipShipment takes C for every archive-set item (the shipment, the release scope, the explicit member features, and their descendants) in sorted item-ID order before it takes batch B. Nested `ArchiveItem` calls reuse that C. B is released first, then C. |
| `UnarchiveItem` (reconcile step) | B only, compare-and-swap | The reconcile path calls `unarchiveItemExpecting`, which re-reads the archived artifact under B and refuses with `ErrShipmentConflict` when `archived_status` no longer matches the value the reconcile plan observed. |

## Lock-domain decision: reconciliation and membership writers (200.002-T)

**Decision: reconciliation already runs inside the membership lock domain, so
no new lock binding is needed. 200.003-T is closed as not needed.**

Evidence from the 200.001-T characterization test
(`internal/core/shipment_reconcile_membership_interleave_test.go`):

* `ReconcileShipmentToShipped` acquires the lifecycle global lock and then the
  shipment membership lock A in Phase A, and keeps both held across Phases B, C,
  and D. Phase B and Phase D take C and then B on top of them.
* `AddItemToShipment` and `ReturnBlockedItem`, the membership writers, take the
  lifecycle global lock first. The test pauses reconcile while it holds global,
  A, C, and B (through `artifactMutationLockBarrierHook`), starts each writer,
  and observes the writer's global-lock acquisition report contention. The
  manifest is identical at the pause and at the instant reconcile returns, and
  the writer completes only after reconcile returns.

The membership snapshot that reconcile freezes in Phase A is therefore the
membership it finishes with. A membership writer is excluded at the global lock
before it can reach A. The deadlock argument is the existing one: every writer
that needs A takes the lifecycle global lock first, so there is no
A-then-global edge, and reconcile takes C before B, matching the canonical
order. Adding a second membership acquisition to the reconcile closure would
add no exclusion and would add an acquisition site to audit. 200.003-T
therefore enables the target assertion in the characterization test and makes
no production change. A future reconcile path that releases the lifecycle global
lock between phases must revisit this decision. The position for any such A
acquisition is after the lifecycle global lock and before C, as the canonical
order above states.

## Archive event mechanism for ShipShipment and ArchiveItem (200.002-T)

**Decision: archive writers take C for the whole archive set, in sorted item-ID
order, before they take B. They append each archive event under C after the
mutation. They release B first and then C.**

* The C set is computed from the same scope-ID snapshot that sizes B. For
  standalone `ArchiveItem` that is the target item. For cascade `ArchiveItem` it
  is the target and every descendant. For `ShipShipment` it is the shipment, the
  release scope, the explicit member features, and their descendants.
* Where a writer already holds C in its context, the nested acquisition is a
  reuse and is not a new lock request. A nested `ArchiveItem` under a
  `ShipShipment` or a cascade parent reuses the context-held C.
* A request for C while B is held, for an item whose C is not context-held, fails
  closed with an error. It never acquires C. This keeps the B-then-C overlap that
  172.002-T removed from returning through a nested call.
* The deferred archive event sink is retired for these paths. Each archive event
  is appended in the call that performs the mutation, under C.

Rejected alternative: a sequence stamp on archive events, so that readers can
reorder events appended after B is released. Nothing in the event writer or the
event readers would consume the stamp, so it would add a field with no reader.

Deadlock argument: every writer that holds more than one of the three domains
acquires them in the order global, A, C, B, and sorts item IDs inside C and
inside B. `AssociateCommit` and the reconcile-to-shipped transaction already
follow that order. With archive writers on the same order there is no
B-then-C edge anywhere, so no cycle can form between an archive writer and a
C-then-B writer.

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

For a given item, C serializes observable JSONL events. For mutations whose
event append is serialized under C together with the mutation, moving C relative
to B must not reorder, duplicate, or drop the event sequence that an operator can
read from the item log. If a mutation records an event, the event must remain
causally attached to the mutation: no event may be visible as completed before
the guarded mutation exists, and no successful governed mutation may lose its
required event. Archive events follow the same rule since 200.007-T: they are
appended under C after the mutation they record, so no concurrent writer's event
can land between the archive mutation and the archive event.
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
`itemLogLockBarrierHook` for C, which fires immediately before the cross-process
item-log file lock is acquired, after the in-process item-log mutex is held.
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
| `ArchiveItem` | Can stamp or preserve archive provenance | Existing archive reconcile guard preserves non-empty reconciled `archived_status`; Unit 1 handles lock-order risk, not the CAS guard. 200.009-T adds a compare-and-swap guard to the post-archive stash side effect. |
| `ReconcileArchivedLifecycle` | Writes the governed target `archived_status` | Governed source of truth for the repair, not a clobbering stale writer. The set step is compare-and-set on the queued status (172.003-T). |
| `archiveDescendants` cascade | Can archive descendants and stamp provenance | Covered by archive governance and cascade ordering; no new CAS guard is planned in this shipment. |
| `UpdateArtifactWithGate` / direct update path | Can mutate artifacts through locked update flow | Existing shipped-transition and blocked-shipment guards apply. General content-staleness protection is out of scope for this shipment. |

The guard obligation is deliberately narrow: it protects only the
`archived_status` field from stale overwrite of a freshly reconciled `shipped`
value. General lost-update protection for arbitrary artifact fields requires a
revision, canonical-content CAS, or reload-and-merge design and remains out of
scope.

## Residual and bound

### Closed by CY-B

* **ShipShipment B-across-C residual: closed by 200.005-T.** ShipShipment no
  longer holds batch B while it acquires C. It takes C for its whole archive set,
  in sorted item-ID order, before B, appends each archive event under C after the
  mutation, and releases B, then C. The 200.004-T probe, built on
  `artifactMutationLockBarrierHook` and `itemLogLockBarrierHook`, shows no C
  acquisition under B during a ship.
* **Archive event ordering exception: closed by 200.007-T.** Standalone and
  cascade `ArchiveItem` take C for the archive set before B and append the
  archive event under C. The deferred event sink is retired, so there is no gap
  in which a concurrent C-then-B writer can append its event between the archive
  mutation and the archive event. The 200.006-T test forces that interleaving
  with the existing barrier hooks. A C request made while B is held, for a C the
  call tree does not already hold, fails closed with an error.
* **Post-archive side-effect residual: closed by 200.009-T.** The linked-stash
  archive that follows the archive commit is a compare-and-swap guarded side
  effect. Under the item's artifact-mutation lock it re-reads the item from
  Markdown and refuses the stash update when the item is no longer archived with
  the provenance this archive stamped. The 200.008-T test injects a concurrent
  unarchive through the post-archive hook point. A hook-queue append failure is
  unchanged: post-archive hooks stay non-fatal.
* **Reconciliation versus membership writers: closed by 200.003-T as not
  needed.** See the lock-domain decision above.

### Remaining bounds

* The archive event append remains best-effort: on log failure it warns and the
  archive still succeeds. The faultline declaration reflects this since
  200.012-T, which marks EventsJSONL conditional for `CreateItem` and
  `ArchiveItem`.
* `ReconcileArchivedLifecycle` no longer holds a batch-wide B (172.003-T). Its
  remaining bound is the window between the compare-and-set status step and the
  re-archive. Another writer can change the queued status inside that window.
  The set step detects a change made before it runs. The step-12 re-archive
  and the failed-update rollback re-archive both re-validate the expected
  status under B. When the status changed, they refuse with
  `ErrShipmentConflict`, record a reconciliation conflict event, and leave the
  item in the queue as found. A concurrent writer's status is never archived
  over.
* Reconcile isolation is weaker than the removed batch-wide B. The unarchive,
  status set, and re-archive steps are each individually guarded, but another
  writer can interleave between them. Conflicts are reported per item and
  recorded as reconciliation conflict events rather than prevented.
* Post-archive hooks run after the governance locks are released and are not
  serialized against concurrent writers. Only the stash side effect is guarded.
* Cascaded child hooks run while the parent's batch B is still held. A hook that
  takes C, or a B for an item outside the cascade set, would reintroduce an
  inversion. No current hook does this.
* Future snapshot-before-lock writers that can persist `archived_status` must opt
  into `guardArchivedStatusUnchangedSince` or an equivalent source-of-truth
  Markdown comparison before shipment 153-S can claim the class is closed.

## Hardening audit (CY-B review follow-up)

### Audited `ArchiveItem` callers

No caller reaches `ArchiveItem` with B in its context and C not held, except the
`ShipShipment` archive loop, which holds C for its whole scope first.

| Caller | Locks in context at the call | Result |
|---|---|---|
| CLI `archive`, CLI `migrate`, MCP `archive_item`, doctor `--fix-orphans`, retention archive | None | Outermost call; takes C then B itself. |
| `archiveDescendants` cascade | Parent's C and B | C covers the whole set, so the nested call reuses it. |
| `ReconcileArchivedLifecycle` rollback and step-12 re-archive | None | The set step's B lives only inside `setItemStatusAndMetaLocked` and its `lockedCtx` is not returned to the caller. |
| `BulkUpdateStatus`, hooks, reconcile-to-shipped transaction | n/a | None of them calls `ArchiveItem` or `archiveItems`. |
| `ShipShipment` via `archiveItems` | A, C for the ship set, B | C-first; see below. |

### ShipShipment C set is complete for parent cascades

The ship closure's parent cascade runs with `shipmentMemberCascadeBoundaryContextKey`
set to the explicit member set, and `cascadePersistedParentStatuses` returns at
the first ancestor that is not a member. Every ancestor that can receive a
`status_changed` event is therefore a member, and every member is a root of the
C set. A non-member parent (the covering feature in the probe fixture) is never
touched, and the B4 probe asserts its C is never requested. `ShipShipment` does
not write `returned_to_backlog` events; only the doctor reads them. No C set
change was needed.

### Stale-descendant race

A child created after `shipItemLogScopeIDs` snapshots the C set but before the
nested `ArchiveItem` computes its scope makes that scope larger than the held C.
The nested call fails closed before any mutation, with an error that wraps
`errArchiveScopeChanged` ("archive item-log lock set incomplete ... retry if transient") and
`errItemLogLockUnderArtifactLocks`. The failure leaves the archive untouched;
the caller retries `ShipShipment`, which recomputes the C set. The window is not
closed by redesign.

### Probe soundness

The B4 probe detects a held B with `lockTaskFile`, which first calls
`TryLock` on a process-wide per-path mutex and returns `ErrTaskBusy` before any
OS-level lock, so detection is identical on Windows and Linux. A control test
proves held B reads busy and released B reads free, and the probe asserts the
barrier fired.
