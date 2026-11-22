---
title: "222-S served-root handoff and execution authorization"
description: "Served-root handoff outcome record for execution of Shipment 222-S (Feature 221-F)."
doc_type: memory
source: docs/memory/2026-11-22/222-s-served-root-handoff.md
ingested_at: "2026-11-22T05:25:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 222-S
  feature_id: 221-F
  branch: feat/222-s
---

## Served-root handoff outcome

* `handoff_verdict`: `PASS`
* `shipment_id`: `222-S`
* `served_workspace_root`: `.`
* `served_storage_root`: `.backlogit`
* `manifest_path`: `.backlogit/queue/222-S.md` (`status: queued`, `11` explicit items `221-F`, `221.001-T`..`221.010-T`)
* `index_path`: `.backlogit/backlogit.db`
* `pre_claim_topology_gate`: `PASS` (`predecessor_provenance=explicit` via `221-S`, `worktree_cleanliness=CARRY_FORWARD_ELIGIBLE` at `main` `da736a9eee3b8b884ab1c6542f2117d72a9b9080`)

## Binding and branch

Ship verified the metadata workspace root as `.`, storage root as `.backlogit`,
and main SQLite index as `.backlogit/backlogit.db` on branch `feat/222-s`
(with absolute host paths omitted).
