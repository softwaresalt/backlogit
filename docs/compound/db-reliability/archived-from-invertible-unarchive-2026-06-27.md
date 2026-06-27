---
chunk_strategy: h1-h2-h3
description: 'ArchiveItem stamped archived_from with the record current path; for an item already at its archive path that self-references and breaks invertible unarchive. Fix: resolve the canonical queue restore path at stamp time, self-heal legacy self-refs at read time, and refuse to clobber a distinct restore destination.'
doc_type: learning
docline:
    category: db-reliability
    component: archive-lifecycle
    date: 2026-06-27T00:00:00Z
    file_path: internal/core/archive.go
    message: ArchiveItem stamping archived_from with the record current path self-references when the item is already at its archive path, leaving UnarchiveItem unable to restore it to the queue
    problem_type: broken-invertibility
    resolution_type: canonical-restore-path-resolver + read-time-self-heal + clobber-refusal-guard
    resolved: true
    root_cause: archived_from derived from the record current path instead of the canonical queue restore path; for pre-archived items current path == archive path
    severity: high
    tags:
        - reliability
        - archive-safety
        - invertibility
        - unarchive
        - self-heal
        - data-integrity
        - index-vs-source-of-truth
    shipped_in: 067-S
    merge_commit: 41f6ff7d309ccb7c388accd85d2c438205370a77
ingested_at: "2026-06-27T18:58:00Z"
schema_version: "1.0"
source: docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md
title: archived_from must be the canonical queue restore path, not the record current path — pre-archived items self-reference and break unarchive
---

# archived_from must name the canonical restore path, not the current path

## Problem

`core.ArchiveItem` stamped each archived record's `archived_from` frontmatter with the
record's **current** path. That is correct for the common case (archive a queue item:
current path == queue path). But an item can already be at its **archive** path when
`ArchiveItem` runs — e.g. a task archived at done-time and then re-archived during
`shipment ship`, or any "pre-archived" item. In that case the current path equals the
archive path, so `archived_from` **self-referenced** the archive location
(`.backlogit/archive/<id>.md`).

`UnarchiveItem` then saw `source == destination` and skipped the queue restore, leaving
those items **un-restorable** — archive→unarchive was no longer invertible. On disk this
manifested as 130 legacy self-referential records across the live workspace.

## Fix (three composed guards)

1. **Canonical restore-path resolver** (`canonicalRestorePath`, `internal/core/archive.go`):
   compute the `.backlogit/`-prefixed queue restore path purely from the workspace queue
   layout — independent of the record's *current* location. `ArchiveItem` stamps **this**
   canonical path as `archived_from`, so a pre-archived item gets
   `.backlogit/queue/<id>.md`, never its own archive path.
2. **Read-time self-heal** (`UnarchiveItem`): when a record already on disk carries a
   legacy self-referential `archived_from`, recompute the canonical restore path at read
   time and restore there anyway. This makes invertibility **independent of any bulk
   migration** — old records become restorable without first being rewritten.
3. **Clobber-refuse guard**: a self-heal that *recomputes* a destination must verify that
   destination is not a **distinct** existing file. Before the atomic rename,
   `UnarchiveItem` refuses to overwrite a different queue file at the recomputed path
   (`TestUnarchiveItem_RefusesToClobberExistingQueueFile`) — on POSIX an atomic rename
   would silently clobber it; on Windows it would fail. The guard turns a latent
   data-loss / platform-divergent path into an explicit refusal.

A CLI-only `doctor --check-archived-from` audit detects self-referential and malformed
records, and `--fix-archived-from` performs a gated, body-preserving repair (only the
`archived_from` field changes; CRLF and body bytes are preserved).

## Why it matters

- **Stamp invariants from the canonical store, not the mutable current state.** A field
  that records "where this came from" must be derived from where it *belongs* (the
  canonical queue layout), not from where the record *happens to be* when you write it.
  This is the same lesson as deriving ID-allocation facts from the canonical filesystem
  rather than the index (see Related).
- **Read-time self-heal decouples correctness from migration.** Making the reader tolerant
  of legacy bad values means invertibility is restored immediately, and a bulk migration
  becomes an optimization/cleanup, not a correctness prerequisite.
- **Any recompute-then-write path needs a clobber check.** When you compute a destination
  rather than reading it back, an existing distinct occupant is a data-loss hazard; refuse
  loudly and carve out only the genuine same-item case.

## Evidence

- Shipped in `067-S` at merge commit `41f6ff7d309ccb7c388accd85d2c438205370a77` (PR #141).
- **Dogfooding (strongest):** the first `shipment ship` after the fix (067-S's own closure)
  re-archived 7 pre-archived (fieldless) tasks plus the feature and shipment; all 9 records
  received canonical `archived_from: .backlogit/queue/<id>.md` — none self-referenced.
- Live `doctor --check-archived-from`: **0** `archived_from_self_ref` (2 malformed `done`
  records remain flag-only).
- Tests pass fresh: `TestArchiveUnarchiveRoundTrip_PreArchived`,
  `TestArchiveItem_PreArchivedStampsCanonicalArchivedFrom`,
  `TestUnarchiveItem_SelfHealsLegacySelfRef`,
  `TestUnarchiveItem_RefusesToClobberExistingQueueFile`,
  `TestDoctor_ArchivedFromAudit`, `TestDoctor_FixArchivedFromRepairsSelfRef`,
  `TestDoctor_FixArchivedFromRequiresCheck`.

## Related

* `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md`
  — same theme (canonical source-of-truth vs. mutable/current state for an archive-safety
  invariant; archive overwrite refusal with a same-item carve-out).
* `docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md` (deliberation)
* `docs/exec-plans/2026-06-26-archive-archived-from-integrity-plan.md` (plan)
* `docs/closure/2026-06-26-archived-from-migration-closure.md` (legacy-record migration runbook)
* Deferred follow-up: extract the shared body-preserving frontmatter codec + atomic-write
  helper to a leaf package to remove the `internal/docline ↔ internal/core` duplication the
  import-cycle workaround introduced (stash `8863C6C8`).
