---
title: "218-S served-root handoff and execution authorization"
description: "Served-root handoff outcome record for execution of Shipment 218-S (Feature 217-F)."
doc_type: memory
source: docs/memory/2026-11-19/218-s-served-root-handoff.md
ingested_at: "2026-11-19T08:45:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 218-S
  feature_id: 217-F
  branch: feat/218-s
---

## Served-root handoff outcome

* `handoff_verdict`: `PASS`
* `shipment_id`: `218-S`
* `served_workspace_root_rel`: `.`
* `served_storage_root_rel`: `.backlogit`
* `manifest_rel`: `.backlogit/queue/218-S.md`
* `index_db_rel`: `.backlogit/backlogit.db`
* `static_check`: `PASS (id=218-S, status=queued, updated_at=2026-11-19T08:34:08.2865857Z, items=[217-F, 217.001-T, 217.004-T, 217.002-T, 217.003-T])`
* `pre_claim_topology_gate`: `PASS (exit_code=0, WORKTREE_CLEAN)`
* `post_claim_topology_gate`: `PASS (exit_code=0, active_shipments=["218-S"])`

## Binding and branch

Ship verified the metadata workspace root as `.`, storage root as `.backlogit`,
and main SQLite index as `.backlogit/backlogit.db` on branch `feat/218-s` (with
absolute host paths omitted).
