---
description: "Compound-refresh report for shipment 105-S post-merge closure — graduated the backlogit storage-contract learning (explicit index rebuild + materialized telemetry-sessions.jsonl) and reviewed adjacent rehydrate/telemetry/append-serialization entries for drift or overlap."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-25T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-25-105-S-compound-refresh.md
title: "105-S compound-refresh report"
---

## Scope

Compound-library maintenance triggered by shipment 105-S post-merge closure
(feature 125-F, "Documentation consistency and README brochure rework for
v1.7.0"; tasks 125.001-T–125.006-T), PR #298, merge
`ae0f205377bf0843ce3f11307c53187538459c02`. Mode: **propose** (no existing
entries edited).

Although 125-F was a documentation feature, the Copilot review surfaced two
code-level storage-contract facts that were misstated in the README and are worth
graduating so they do not re-drift across docs, agent guidance, and operator
runbooks.

## New entry captured

`docs/compound/best-practices/storage-contract-index-rebuild-and-telemetry-sessions-materialization-2026-07-25.md`

Two verified facts:

1. **Index rebuild is explicit, not automatic on read.** `core.NewWorkspace`
   opens the DB and ensures schema only; `PersistentPreRunE` does no auto-sync;
   read paths (`list` → `db.QueryItems`) query immediately. `db.Rehydrate` runs
   only via explicit paths — the `backlogit sync` CLI command, the
   `backlogit_sync_index` MCP tool (`internal/mcp/tools.go` `handleSyncIndex`),
   and `migrate` — plus incremental reconciliation via the explicit
   `backlogit_merge_sync` MCP tool (`handleMergeSync` → `db.MergeSync`), which is a
   tool call, not an automatic git-triggered path. A missing/stale `backlogit.db`
   yields empty/outdated reads until one of them runs.
2. **`telemetry-sessions.jsonl` is a materialized summary, not append-only.**
   `writeTelemetryJSONL` rewrites it via temp-file-then-rename each harvest and
   `--force` resets it, unlike the `os.O_APPEND` per-item logs and
   `telemetry.jsonl`.

## Entries evaluated

| Entry | Outcome | Rationale |
|---|---|---|
| `best-practices/storage-contract-index-rebuild-and-telemetry-sessions-materialization-2026-07-25.md` (new) | **keep** | Net-new capture; no existing entry states these two facts as a coherent storage-contract learning. |
| `2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` | **keep** | Complementary, not overlapping: it covers *why* per-item JSONL writes must thread one shared `EventWriter` (append serialization / concurrency). The new entry cites the append-only logs but does not restate that concurrency rule. Recommend a future cross-reference. |
| `database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md` | **keep** | Complementary: covers Rehydrate transaction atomicity/batching. The new entry adds *when* Rehydrate fires (explicit `sync`, not on read). No contradiction. |
| `runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md` | **keep** | Consistent: notes `telemetry-sessions.jsonl` is *read* to rebuild the SQLite index. That is compatible with the new entry's point that the file is a materialized projection (written by harvest, read by rehydrate). No drift. |

## Drift / duplication findings

- No stale or contradicted entries found. The new entry does not supersede or
  duplicate any existing learning; it fills a genuine gap (rebuild *timing* and
  per-file *write discipline*).
- No `apply`-mode edits required. Two low-priority cross-reference additions are
  recommended (new entry ⇄ eventwriter-append; new entry ⇄ atomic-rehydration)
  but are deferred to avoid expanding this closure diff.

## Result

- 1 net-new compound entry graduated (docline lint: 0 violations repo-wide).
- 3 adjacent entries reviewed → all **keep** (accurate, distinct).
- 0 entries updated / consolidated / replaced / deleted.
