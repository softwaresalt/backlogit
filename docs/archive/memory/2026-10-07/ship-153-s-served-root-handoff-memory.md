---
title: "Ship 153-S served-root handoff outcome"
description: "Served-root attestation outcome recorded by the resumed Ship session for shipment 153-S."
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

# Ship 153-S Served-Root Handoff Outcome

* Recorded: 2026-10-07T20:20Z (UTC), after `BRANCH_OK` on
  `feat/153-s-concurrency-safety-hardening-for-artifact-mutation-writers`.
* Served workspace root: `.` (repository root, main worktree).
* Served storage root: `.backlogit` (`.backlog` absent).
* MCP attestation: `get_metadata_catalog.workspace` root and storage root
  matched the served roots; `pragma_database_list` main file is
  `.backlogit/backlogit.db`. Outcome: `SERVED_ROOT_ATTESTATION_OK`.
* Index: `backlogit_sync_index` after checkout indexed 1918 artifacts.
* Shipment: 153-S `active` (claim committed 2026-10-07T07:34:08Z); all 16
  members `active` with `scheduler_baseline_claim: 153-S`.
* Topology gate: `pipeline-topology --phase post_claim` exit 0
  (`BRANCH_OK`, `WORKTREE_TOPOLOGY_OK`, sole active shipment 153-S).