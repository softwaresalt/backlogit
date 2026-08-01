---
title: "Stage memory: ship sequence manifest spike"
source: docs/memory/2026-07-29/ship-sequence-spike-memory.md
doc_type: guide
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Session memory for Stage spike 16FD6CC0 on a dark-mode shipment sequence manifest."
docline:
    agent: stage
    date: 2026-07-29
    stash_id: 16FD6CC0
    outcome: defer
---

## Summary

Processed exactly stash `16FD6CC0` as a spike. The findings recommend deferring
a standalone `.backlogit/ship_sequence.jsonl` manifest because the authoritative
ordering surfaces should remain `queue view`, `queue_position`, `item_deps`, and
shipment manifests. A future manifest should be non-authoritative audit evidence
only if real multi-shipment dark-mode runs prove activation/checkpoint evidence
insufficient.

## Artifacts

* Findings: `docs/decisions/2026-07-29-ship-sequence-manifest-spike.md`
* Harvested spike: `001-SP`
* Build shipment: not created

## Backlog State

* `16FD6CC0` harvested to `001-SP`
* `7F0A6E89` left active
* `6FA0829B` left active
* `113-S` left queued and unmodified
* `132-F` and child tasks left queued and unmodified

## Evidence Highlights

* Orchestrator currently checks queued shipments via `list_shipments` and says
  to select the highest-priority queued shipment.
* `queue view` already supports durable manual ordering through
  `custom_fields.queue_position` and dependency-aware filtering through
  `item_deps`.
* Shipment manifests own membership through `custom_fields.items`; lifecycle
  claim and ship operations consume that explicit manifest scope.
* The current SQLite rehydration path has no `ship_sequence` projection and
  would need a new table or direct Orchestrator file reads.

## Next Steps

No immediate build work was queued. If the operator later wants to pursue this
area, start with an Orchestrator follow-up that consumes
`queue view --type shipment --status queued` and records ordered shipment IDs in
`DARK_MODE_SCOPE` activation evidence.
