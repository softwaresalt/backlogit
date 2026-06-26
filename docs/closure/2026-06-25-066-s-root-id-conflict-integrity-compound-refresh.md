---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh assessment for the root-ID conflict integrity shipment (066-S, PR #132)'
doc_type: closure
docline:
    mode: propose
    ms.date: 2026-06-25T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-06-25-066-s-root-id-conflict-integrity-compound-refresh.md
title: 066-S Compound Refresh Review
---

## Scope

Reviewed the compound entries most likely to intersect the shipped scope of `066-S`
(ID allocation, archive safety, rehydration, atomic file writes):

* `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`
* `docs/compound/db-reliability/sqlite-locked-missing-from-retry-predicate-2026-04-13.md`
* `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
* `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md`
* `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md`
* `docs/compound/best-practices/go-file-write-short-write-guard-2026-04-23.md`

Plus the newly authored entry for this shipment:

* `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md` (NEW)

## Classification

| Entry | Classification | Evidence | Action |
|---|---|---|---|
| `batch-failure-silent-nil-return-anti-pattern-2026-04-13.md` | keep | Same "index/cache state vs caller-visible truth" theme but a distinct failure (silent nil return from `Rehydrate`); the 066 work neither contradicts nor duplicates it. Cross-referenced from the new entry. | No edit |
| `sqlite-locked-missing-from-retry-predicate-2026-04-13.md` | keep | Retry-predicate concern, orthogonal to ID allocation; unaffected by 066. | No edit |
| `atomic-rehydration-sqlite-transaction-2026-04-08.md` | keep | 066.004-T added only an observational duplicate-source **warning** to `Rehydrate`; it does not touch the DELETE+WalkDir transaction atomicity this entry documents. Still accurate. | No edit |
| `crash-safe-delete-rename-rollback-go-2026-04-23.md` | keep | 066 `ArchiveItem` reuses the same atomic tmp+rename + rollback-on-failure pattern this entry prescribes; the new destination-occupied guard is additive and consistent. | No edit |
| `windows-safe-atomic-rename-goos-gate-2026-04-23.md` | keep | Atomic-rename guidance remains correct; 066 archive writes follow it. | No edit |
| `go-file-write-short-write-guard-2026-04-23.md` | keep | Short-write guard guidance is unchanged by 066. | No edit |
| `canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md` (NEW) | keep | Newly captured; distinct root cause (per-type `MAX(ordinal)+1` over a PK-collapsed, archive-blind index). No existing entry overlaps materially. | Created this session |

## Summary

No existing compound entries required `update`, `consolidate`, `replace`, or `delete`
for shipment `066-S`. The shipped work is **additive** to the existing reliability and
atomicity guidance; the one genuinely new, reusable lesson — allocate and check IDs from
the canonical filesystem rather than the collapsible/archive-blind SQLite index — was
captured as a new entry and cross-linked to the related batch-failure anti-pattern.

Mode: `propose` — no existing files were modified. The only file written is the new
compound entry above.
