---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh review for the 067-S archived_from integrity shipment — classifies the one overlapping db-reliability/archive-safety entry as keep (distinct) and records the new invertibility learning'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-27T19:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-067-S-archived-from-integrity-compound-refresh.md
title: 067-S archived_from Integrity — Compound Refresh
---

# Compound Refresh — Shipment 067-S (archived_from Integrity)

- **Scope**: `recent` + entries overlapping the archive-safety / data-integrity surface
- **Mode**: propose (no rewrites required; one new entry authored separately)
- **Context**: Post-merge closure of 067-S (PR #141, merge `41f6ff7d`)

## Entries reviewed

| Entry | Overlap with 067-S | Classification | Rationale |
|---|---|---|---|
| `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md` | Shares the archive-safety theme and the `currentPath == archivePath` carve-out, and the "canonical source-of-truth vs. mutable/current state" principle | **keep** | Distinct problem (root-ID allocation over a PK-collapsed index + archive **overwrite** refusal). It is **not** about `archived_from` invertibility. Still accurate against current `internal/core/canonical_scan.go` and `ArchiveItem`. The new 067-S entry cross-links it rather than superseding it. |

## New entry authored

- `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md` — captures the durable learning that `archived_from` must be the canonical queue restore path (not the record's current path), that read-time self-heal decouples invertibility from bulk migration, and that any recompute-then-write path needs a clobber-refuse guard. This is a **new** learning (`compound`), not a refresh of an existing one.

## Evidence used

- Shipped code at merge `41f6ff7d` (PR #141): `internal/core/archive.go`, `internal/core/doctor.go`, the doctor CLI surface, and the test suite.
- Live workspace evidence from this closure: dogfooding canonical stamping on 9 records; `doctor --check-archived-from` = 0 self-referential.
- Existing entry's own citations (066-S `ArchiveItem` overwrite-refusal) confirm it covers a different defect class.

## Actions taken

- **Updated/consolidated/replaced/deleted**: none.
- **Marked stale**: none.
- **Kept as-is**: `canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md`.

## Follow-ups requiring manual review

- None. The library is small and the two archive-safety entries are complementary, not redundant.
