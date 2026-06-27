---
chunk_strategy: h1-h2-h3
description: 'Post-merge lightweight runtime verification for shipment 067-S — archived_from invertibility proven via live doctor audit, canonical archive stamping on first ship since the fix, and the core unarchive round-trip/self-heal/clobber-guard test suite'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-27T18:50:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-067-S-archived-from-integrity-runtime-verification.md
title: 067-S archived_from Integrity — Post-Merge Runtime Verification
---

# Runtime Verification — Shipment 067-S (archived_from Integrity)

- **Surface**: CLI / data-integrity library (`internal/core` archive lifecycle + `doctor` audit). No runtime service, web, or background-job surface.
- **Mode**: manual + automated test suite (no live archive mutation)
- **Context**: Ship Step 6 post-merge closure for 067-S; merge commit `41f6ff7d309ccb7c388accd85d2c438205370a77` (PR #141), default branch `main`.
- **Verdict**: **PASS**

## Invariants under test

1. `ArchiveItem` stamps a **canonical** `archived_from` (`.backlogit/queue/<id>.md`) for every archived record — including pre-archived items whose current path already equals the archive path.
2. Archive → unarchive is **invertible**: unarchiving restores the record to `.backlogit/queue/`.
3. `UnarchiveItem` **self-heals** a legacy self-referential `archived_from` at read time, independent of any bulk migration.
4. A read-time self-heal that recomputes the restore path **refuses to clobber** a distinct existing destination.
5. `doctor --check-archived-from` reports **0 self-referential** records on the live workspace.

## Environment prechecks

- Binary under test: repo-root `backlogit.exe` (v1.2.0, go1.26.4), freshly built from `main` @ `41f6ff7d`, carrying the new `doctor --check-archived-from`/`--fix-archived-from` flags and the `docs` subcommand.
- Workspace: `.backlogit/` (637 artifacts indexed). No service/port/credential dependencies — this is a library + CLI change.
- A live archive→unarchive→re-archive round trip on real 067 records was **intentionally not performed** to avoid mutating freshly-shipped canonical archive state. The equivalent invariant is proven by the dedicated round-trip test on isolated fixtures (below) plus the live canonical-stamping evidence from the ship itself.

## Evidence

### E1 — Live canonical stamping (dogfooding, strongest proof)

This was the **first `shipment ship` since the `archived_from` fix merged**. All 9 newly-archived 067 records carry canonical `archived_from`:

```
067-F.md    -> archived_from: .backlogit/queue/067-F.md
067-S.md    -> archived_from: .backlogit/queue/067-S.md
067.001-T.md .. 067.007-T.md -> archived_from: .backlogit/queue/067.00N-T.md
```

The 7 tasks had been archived **fieldless** during the feature build; `ShipShipment` re-archived them and `ArchiveItem` stamped the **canonical queue restore path** — exactly the pre-archived case where the old defect produced a self-reference. No record self-references its archive path.

### E2 — Live doctor audit

```
backlogit doctor --check-orphans=false --check-duplicates=false --check-archived-from --format json
```

Result: **0** `archived_from_self_ref`; **2** `archived_from_malformed` (`038-DL`, `039-DL`, value `done`) — the known flag-only records, unchanged.

### E3 — Core invariant test suite (`go test ./internal/core/ -count=1`)

All PASS:

| Test | Invariant |
|---|---|
| `TestArchiveUnarchiveRoundTrip_PreArchived` | (2) archive→unarchive restores to queue |
| `TestArchiveItem_PreArchivedStampsCanonicalArchivedFrom` | (1) canonical stamp for pre-archived items |
| `TestUnarchiveItem_SelfHealsLegacySelfRef` | (3) read-time self-heal of legacy self-ref |
| `TestUnarchiveItem_RefusesToClobberExistingQueueFile` | (4) clobber-refuse guard |
| `TestUnarchiveItem_RestoresFromArchive` / `_RestoresPreArchiveStatus` / `_RestoredStatusInDB` | (2) restore + status |
| `TestDoctor_ArchivedFromAudit` / `_FixArchivedFromRepairsSelfRef` / `_FixArchivedFromRequiresCheck` | (5) audit + gated repair |

Full package result: `ok github.com/softwaresalt/backlogit/internal/core`.

## Follow-up risks

- The 2 malformed `archived_from: done` records (`038-DL`, `039-DL`) remain flag-only by deliberate operator decision; doctor surfaces them every run. Disposition deferred (stashed for Stage — see closure artifact).

## Handoff to operational-closure

- **Verdict**: PASS
- **Surfaces verified**: archive/unarchive lifecycle + doctor audit (CLI/library)
- **Evidence**: live canonical stamping on 9 records, live doctor audit (0 self-ref), core round-trip/self-heal/clobber-guard tests green
- **Follow-up**: malformed-record disposition (deferred, stashed)
