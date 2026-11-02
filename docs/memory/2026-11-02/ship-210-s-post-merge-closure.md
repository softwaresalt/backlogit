---
title: "Ship 210-S post-merge closure resumed and shipped"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-02/ship-210-s-post-merge-closure.md
description: "Session memory for the 210-S post-merge closure resume: governed provenance repair, governed ship, closure artifacts, and open decisions."
chunk_strategy: h1-h2-h3
shipment_id: 210-S
feature_id: 209-F
---

## Outcome

* Checkpoint `checkpoint-20261102-051638.json` was restored. The lifecycle
  `pipeline-topology` gate passed (exit 0).
* The four member archive records were stamped with governed provenance.
  `backlogit_reconcile_archived_lifecycle` refused them, so
  `backlogit_archive_item` was used instead. Pre-mode reconciliation then
  returned `PROCEED`.
* `210-S` shipped. The governed `backlogit_ship_shipment` call timed out
  client-side but completed server-side. The archive record shows
  `archived_status: shipped`. The doctor check reports no findings for `210-S`.

## Deviations

* `compaction_status` is `degraded`, not `done`. The corpus is over threshold
  and the consolidation pass was not executed.
* The prune-on-restore sweep was not run. The active cursor and checkpoint
  pointer were retained.

## Open items

* Closure PR for `post-merge/210-s-closure` awaits CI, Copilot review, and
  P-014 operator approval.
* Follow-ups `7E14915F`, `DF533572`, `D3C8D13B`, `C2CFE06E`, and `F2BC9C37`
  remain open.
