---
title: "220-S served-root handoff and execution authorization"
description: "Served-root handoff outcome record for execution of Shipment 220-S (Feature 219-F)."
doc_type: memory
source: docs/memory/2026-11-21/220-s-served-root-handoff.md
ingested_at: "2026-11-21T04:21:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 220-S
  feature_id: 219-F
  branch: feat/220-s
---

## Served-root handoff outcome

* `handoff_verdict`: `PASS`
* `shipment_id`: `220-S`
* `served_workspace_root`: `.`
* `served_storage_root`: `.backlogit`
* `manifest_path`: `.backlogit/queue/220-S.md`
* `index_path`: `.backlogit/backlogit.db`
* `static_check`: `PASS (id=220-S, status=queued, updated_at=2026-11-21T03:11:23.2322287Z, items=[219-F, 219.001-T, 219.002-T, 219.003-T, 219.004-T, 219.005-T, 219.006-T, 219.007-T, 219.008-T, 219.009-T, 219.010-T, 219.011-T, 219.012-T])`
* `pre_claim_topology_gate`: `PASS (predecessor_provenance=declared_root, worktree_cleanliness=CARRY_FORWARD_ELIGIBLE at main 3aa5c5112e1e4512887ab95666426c6d5a58823c)`

## Binding and branch

Ship verified the metadata workspace root as `.`, storage root as `.backlogit`,
and main SQLite index as `.backlogit/backlogit.db` on branch `feat/220-s`
(with absolute host paths omitted).
