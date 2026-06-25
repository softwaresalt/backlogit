---
title: "Per-type MAX(ordinal)+1 over a PK-collapsed index masks duplicate root IDs — allocate from the canonical filesystem"
description: "Root-ID allocation that derives the next ordinal from the SQLite index can silently reuse an ID, because the index keys on ID (so duplicates collapse to one row) and omits archived/out-of-view artifacts"
problem_type: silent-data-loss
category: db-reliability
component: id-allocation
root_cause: next-ID derived from an index that collapses duplicate IDs and excludes archived/out-of-view artifacts
resolution_type: canonical-filesystem-scan + pre-write-guard + overwrite-refusal
message: "CreateArtifact could allocate a root ID already occupied by an archived or index-invisible artifact, then ArchiveItem could overwrite the distinct occupant"
file_path: internal/core/canonical_scan.go
resolved: true
severity: high
tags: [reliability, id-allocation, archive-safety, sqlite, index-vs-source-of-truth, data-loss, anti-pattern]
date: 2026-06-25
---

## Problem

Top-level work-item IDs (e.g. `066-F`, `066-S`) are allocated by computing the next
per-type ordinal. The original allocation derived that ordinal from the SQLite index
(`backlogit.db`) — effectively `MAX(ordinal)+1` per artifact type over indexed rows.

Two structural facts made this unsafe and able to **silently** reuse a live ID:

1. **The index keys on `id`.** Two artifacts that share a root ID collapse to a single
   row, so a duplicate that already exists is invisible to a `MAX`/count derived from
   the index — the very thing you are trying to prevent disappears from the signal.
2. **The index is a cache, not the source of truth.** Archived artifacts, artifacts on
   another branch, and artifacts present on disk but not yet rehydrated are out of the
   index's view. `Rehydrate` clears and rebuilds the index, so allocation correctness
   was coupled to cache freshness.

The result (bug `0F65FBC9`): `CreateArtifact` could allocate a root ID already occupied
by an archived or index-invisible artifact. Worse, `ArchiveItem` keyed the archive
destination by filename, so archiving the new live item could **overwrite and destroy**
the distinct archived occupant — an undetected data-loss path.

## Fix

Treat the **canonical filesystem** (the `.md` files), not the index, as the source of
truth for ID allocation and collision detection. Three composed guards:

1. **Canonical scan** (`internal/core/canonical_scan.go`): walk and parse every artifact
   across *all* canonical dirs — `queue` + `archive` + routed/nested hierarchy dirs — and
   compute the true max ordinal and the set of occupied root IDs from disk.
2. **Pre-write uniqueness guard** in `CreateArtifact`: before writing, fail loud with
   `ErrIDCollision` if the canonical scan shows the root ID is already taken. Checked
   before any file write, so a refused create has no side effects.
3. **Archive overwrite refusal** in `ArchiveItem`: refuse to overwrite a **distinct**
   item already occupying the path-keyed archive destination (`ErrArchiveDestinationOccupied`),
   while still allowing legitimate same-path / half-archive recovery (the occupant is the
   *same* logical item: equal id and title). `currentPath == archivePath` is never a
   collision.

### Critical subtlety — the registry can blind the scan

A follow-on correctness fix (`5f86ee9d`) forces `.backlogit/archive` into the canonical
scan set **unconditionally**, even when the workspace registry's `directories` rules do
not list it. Otherwise a degraded or minimal registry (e.g. one that routes only `queued`
to a directory and omits archive) would make the collision guard blind to archived IDs —
re-opening the exact data-loss path the guard exists to close. Lesson: a safety scan that
derives its search set from mutable config is only as safe as that config; pin the
safety-critical directories.

## Why It Matters

* **Index ≠ source of truth.** Any correctness invariant (uniqueness, max, existence)
  computed from a cache that collapses keys or omits states is unsound. Derive
  safety-critical facts from the canonical store.
* **A `MAX(...)+1` allocator is only safe if it can see every prior allocation** —
  including archived and out-of-view ones. If it cannot, it will reuse.
* **Fail before the write.** Compute and check the collision before any filesystem
  mutation so a refusal is side-effect-free.
* **Path-keyed destinations need an identity check before overwrite**, with an explicit
  same-item carve-out so legitimate in-place / recovery archival still succeeds.

## Evidence

* Guard tests pass fresh: `TestCreateArtifact_RefusesCanonicalIDCollision`,
  `TestArchiveItem_RefusesDistinctOccupiedDestination`,
  `TestDoctor_DetectsRootIDCollisionAcrossQueueArchive`,
  `TestScanCanonicalArtifacts_IncludesArchiveWhenRegistryOmitsIt`.
* `backlogit doctor --check-duplicates` reports a clean state on the live 621-artifact
  workspace.
* Shipped in `066-S` at merge commit `80ce5f12ef52a68feaecfb9bfdeb94f6f1f79fd3` (PR #132).

## Related

* `docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md`
* `docs/exec-plans/2026-06-23-root-id-conflict-integrity-plan.md`
* `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`
  (same theme: index/cache state vs. caller-visible truth)
* Deferred belt-and-suspenders: durable per-type high-water-mark counter (stash `C55C5158`).
