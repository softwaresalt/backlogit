---
chunk_strategy: h1-h2-h3
description: 'Lightweight runtime verification for shipment 066-S after PR #132 merged, exercising the doctor root-ID collision audit and the allocation/archive guards'
doc_type: closure
docline:
    ms.date: 2026-06-25T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-06-25-066-s-root-id-conflict-integrity-runtime-verification.md
title: 066-S Runtime Verification — Root-ID Conflict Integrity
---

## Scope

Runtime verification for the merged scope of shipment `066-S`
("Root-ID Conflict Integrity"), feature `066-F`, tasks `066.001-T`..`066.005-T`,
shipped at merge commit `80ce5f12ef52a68feaecfb9bfdeb94f6f1f79fd3` (PR #132).

* **Surface**: `cli` / library (the `backlogit` Go binary and `internal/core`,
  `internal/db` packages). No network service or browser surface.
* **Mode**: `manual` — representative CLI invocations against the live workspace
  plus targeted test execution of the merged guards.
* **Verification base**: local checkout on `main` at `80ce5f12`, `backlogit.exe`
  v1.2.0 (built 2026-06-24 from current source).

## Invariants Verified

1. Top-level work-item root IDs are unique across the **full canonical surface**
   (queue + archive + routed dirs), not just the SQLite index.
2. `CreateArtifact` fails loud (`ErrIDCollision`) before writing when a root ID is
   already taken on the canonical filesystem.
3. `ArchiveItem` refuses to overwrite a **distinct** occupied destination
   (`ErrArchiveDestinationOccupied`) while still allowing legitimate same-path /
   half-archive recovery.
4. `doctor` detects root-ID collisions across queue and archive and does not
   mislabel level-2 (child ordinal) duplicates as root collisions.
5. `Rehydrate` warns (observationally) on duplicate source IDs without failing.

## Environment Prechecks

| Check | Result |
|---|---|
| Build under test is expected binary | passed — `backlogit.exe` reports `version 1.2.0` |
| Workspace reachable | passed — `.backlogit` workspace indexes 621 artifacts |
| Merge present in base | passed — `80ce5f12` is an ancestor of `origin/main` |
| Go toolchain available | passed — `internal/core` and `internal/db` build and test |

## Executed Verification

### 1. Doctor audit on the real workspace (066.001-T)

```text
backlogit doctor --check-duplicates --check-orphans
→ No issues found.
```

The canonical root-ID-collision audit ran against the live 621-artifact workspace
(which now contains the freshly archived `066-*` scope) and reported a clean state.
This exercises the hardened audit on real, archive-inclusive data — the exact path
that fix `5f86ee9d` repaired (forcing `.backlogit/archive` into the canonical scan
set so a degraded registry cannot blind the collision guard).

### 2. Targeted guard tests (066.001-T..066.005-T)

`go test -count=1 ./internal/core/... ./internal/db/...` — fresh run, all PASS:

| Test | Guard | Result |
|---|---|---|
| `TestScanCanonicalArtifacts_ReturnsParsedRefsAcrossQueueAndArchive` | canonical scanner | PASS |
| `TestScanCanonicalArtifacts_RecursesNestedHierarchyDirs` | canonical scanner | PASS |
| `TestScanCanonicalArtifacts_IncludesArchiveWhenRegistryOmitsIt` | fix `5f86ee9d` | PASS |
| `TestCreateArtifact_RefusesCanonicalIDCollision` | `ErrIDCollision` (066.002-T) | PASS |
| `TestCreateArtifact_NoCollisionPathUnchanged` | non-regression | PASS |
| `TestArchiveItem_RefusesDistinctOccupiedDestination` | `ErrArchiveDestinationOccupied` (066.003-T) | PASS |
| `TestArchiveItem_DuplicateSourcePaths_*` (4) | half-archive recovery preserved | PASS |
| `TestDoctor_DetectsRootIDCollisionAcrossQueueArchive` | doctor audit (066.001-T) | PASS |
| `TestDoctor_Level2DuplicateIsNotRootCollision` | false-positive guard | PASS |
| `TestRehydrate_WarnsOnDuplicateSourceIDs` | observational warning (066.004-T) | PASS |
| `TestRehydrate_HarvestedStashUsesCanonicalSourcePath` | provenance | PASS |

### 3. Live exercise of the same-path archive guard (066.003-T)

The closure's own `backlogit shipment ship 066-S` operation archived feature
`066-F` after it had already been relocated to `.backlogit/archive/066-F.md` by the
done-status rollup. `ArchiveItem` correctly treated `currentPath == archivePath`
as an **in-place** archival (never a collision) and produced
`archived_from: .backlogit/archive/066-F.md` with `status: archived`. This is a
real, end-to-end exercise of the 066.003-T same-path branch on production data — it
neither raised a false `ErrArchiveDestinationOccupied` nor destroyed the file.

### 4. Static gates

| Gate | Result |
|---|---|
| `go vet ./...` | passed — no findings |
| `gofmt` | local working tree flags Go files due to **CRLF line endings** only (Windows autocrlf checkout). CR-stripped content is byte-identical to gofmt output; CI on Linux (LF) is green, as evidenced by PR #132's passing checks. No Go files are modified by this closure. |

## Verdict

**PASS.**

The merged root-ID conflict integrity guards behave correctly on a real workspace:
the doctor audit is clean, all guard tests pass on a fresh run, and the closure's own
ship operation exercised the same-path archive guard end-to-end without false refusal
or data loss.

## Follow-Up Risks (handed to operational-closure)

* Performance: `scanCanonicalArtifacts` walks/parses every queue + archive `.md` on
  each `CreateArtifact`, which is O(files) per create and O(N^2) for bulk-create
  loops. Tracked as stash `D6B44FF6` (low). Not a correctness risk.
* Belt-and-suspenders durable high-water-mark counter for archived ordinals that are
  temporarily out of view. Tracked as stash `C55C5158` (medium, design-gated).
* Test-ergonomics: `internal/db` log capture currently mutates the global slog
  default. Tracked as stash `2797E9F8` (low).
