---
chunk_strategy: h1-h2-h3
description: 'Two related locking-correctness lessons from 117-S formal-gate-evidence review: (1) an advisory lock keyed by a resolved file path becomes UNSAFE the moment that file can be relocated during the guarded critical section, since two callers resolving "the current path" at different times acquire different, non-contending mutexes; (2) a fixed-TTL stale-lock reclaim with no heartbeat is only safe if the legitimate hold time is provably well under the TTL -- reuse the established heartbeat-refreshed lock pattern for any new lock whose hold duration is not tightly bounded.'
doc_type: learning
docline:
    date: 2026-08-09T00:00:00Z
    severity: high
    tags:
        - concurrency
        - locking
        - toctou
        - core
        - review
schema_version: "1.0"
source: docs/compound/concurrency-issues/2026-08-09-stable-lock-keys-and-heartbeat-refresh-for-unbounded-holds.md
title: 'Lock keys must be stable identifiers, not resolved file paths; fixed-TTL stale-reclaim locks need a heartbeat unless the hold time is provably short'
---

# Stable Lock Keys and Heartbeat Refresh for Unbounded Holds

Graduated from shipment 117-S (Formal Gate F1 — evidence authenticity and
manifest binding; feature 106-F, tasks 106.003-T through 106.011-T; PR #333,
merge `23d88904faf917a4f4003042f185de9b4e568530`). Two related locking bugs
surfaced across review rounds 3-6, both against the SAME reused primitive
(`task_lock.go`'s per-file-path keyed-mutex-plus-sidecar mechanism).

## Rule 1 — A lock key derived from a resolved file path is unsafe for any resource whose file can be relocated mid-hold

### Problem

`lockShipmentMembership` originally resolved the shipment's CURRENT markdown
file path (`FindArtifactPath`) and used that as the lock key. `persistArtifact`
relocates an artifact's file whenever its registry-configured target directory
changes for the new status (always true at archival; possibly true for other
transitions depending on registry configuration) — and
`moveShipmentStatusWithTopLevel` performs exactly such a relocating persist
INSIDE the very critical section this lock exists to protect. A lock acquired
against the pre-relocation path and a second acquisition (by a different
caller, once the file has already moved) against the post-relocation path are
DIFFERENT mutexes and DIFFERENT sidecar files that never contend with each
other — silently reopening the race the lock was supposed to close, regardless
of how correctly the lock's HOLD SCOPE had otherwise been extended.

### Fix

Key the lock on a STABLE SYNTHETIC identifier — workspace root + artifact ID —
never the artifact's current file path:

```go
stableKey := filepath.Join(WorkspaceStorageRoot(ws.RootPath), ".locks", shipmentID)
```

The sidecar this produces need not (and must not) point at a real artifact
file; the underlying `lockTaskFile` mechanism only ever creates a
`.{name}.lock` sidecar adjacent to the path it is given and never requires
that path to exist.

### Test technique that proved it

Acquire the lock, then perform the SAME relocating operation the real caller
performs (a real status transition through `moveShipmentStatusWithTopLevel`)
WHILE STILL HOLDING the first lock, then attempt a second acquisition and
confirm it still contends. Critically: also update any SIBLING test that
manually re-derives the lock path via the old mechanism (e.g. via
`FindArtifactPath` directly) — a test built against the OLD path-based key
will falsely "pass" after the fix even though it no longer exercises the real
lock function's contention behavior, because it never calls the function
under test at all.

## Rule 2 — A fixed-TTL stale-lock reclaim without a heartbeat only stays safe while the legitimate hold time is provably short

### Problem

`nextGateEvidenceCounter`'s original bespoke sidecar lock used a fixed 60-second
stale-reclaim TTL with no heartbeat: if the guarded sign-and-durably-append
sequence legitimately stalled past 60 seconds (a slow disk, a contended CI
runner), a second caller would see the untouched sidecar as crash residue,
reclaim it, rescan the SAME max counter, and allocate — then durably append —
the IDENTICAL counter value, breaking the counter-uniqueness/anti-replay
guarantee. Worse, since ownership was never tokenized, the original holder's
eventual unlock would then delete the SECOND holder's lock file out from
under it.

### Fix

Reuse the existing heartbeat-refreshed lock mechanism (`lockTaskFileWithHeartbeat`,
already used for the long-lived per-task completion lock) rather than
maintaining a second, bespoke, non-heartbeated implementation. The heartbeat
refreshes the sidecar's ModTime on an interval strictly under the stale-TTL
for as long as the holder is alive, so a live holder's lock is never mistaken
for crash residue regardless of how long the guarded operation actually takes.

Two secondary corrections were needed when migrating to the shared mechanism:

* **Directory creation.** The generic `lockTaskFile` assumes its sidecar's
  parent directory already exists (true for its normal use case: a real
  artifact file). A synthetic proxy path for a resource with no natural parent
  directory (a log file that may not exist yet for a brand-new item) needs an
  explicit `os.MkdirAll` before the first lock attempt.
* **Bounded-wait sizing.** The single-contender default bounded wait (sized
  for a task completion lock that normally has at most one or two contenders)
  was too short for a counter lock that can have MANY legitimate concurrent
  queuers for the same item — a dedicated, more generous bounded wait avoided
  spurious `ErrGateInProgress` failures under a 20-way concurrent-allocation
  test that had previously passed under the old unbounded raw-mutex behavior.

### General test for "does this lock need a heartbeat"

Ask: is the guarded critical section's duration bounded by something FAST and
IN-PROCESS (a memory read, a hash, a small disk write), or can it include a
SLOW, EXTERNALLY-INVOKED operation (a subprocess, a network call, an operation
whose duration depends on system load)? If the latter, a fixed-TTL-only lock
is a latent correctness bug waiting for a slow day, not a hypothetical edge
case — reuse the heartbeat pattern rather than picking a "big enough" TTL,
since there is no TTL that is provably big enough for an externally-invoked
operation's worst case.

## Related

- `docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` —
  the original stale-TTL advisory lock pattern this shipment's counter lock
  initially (incorrectly) mirrored without the heartbeat this document adds.
- `docs/compound/best-practices/timeout-goroutine-lock-ownership-go-2026-07-01.md` —
  a related lock-ownership correctness pattern.
- PR #333 review round 3 (lock-key stability) and round 4 (heartbeat).
