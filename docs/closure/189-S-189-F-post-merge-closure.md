---
chunk_strategy: h1-h2-h3
# gate-required: closure_status and compaction_status MUST remain at the top level
# of this frontmatter (pipeline-topology gate reads them). Do NOT run
# `backlogit docs migrate --apply` against this file.
closure_status: READY
compaction_status: done
description: "Post-merge closure for 189-S / 189-F claim-time shipped-only predecessor guard (PR #546)."
doc_type: closure
schema_version: "1.0"
source: docs/closure/189-S-189-F-post-merge-closure.md
shipment_id: 189-S
title: "189-S / 189-F Post-Merge Closure"
---

# 189-S / 189-F Post-Merge Closure

* Shipment: 189-S (189-F, 189.001-T through 189.007-T), shipped via governed `backlogit shipment ship --sha`.
* PR: #546, merge commit `b56601929b1d5d6d705cbc429d71538a7c7001a2` (merge-commit strategy, P-009). Binary: built from `go build ./cmd/backlogit` at merge.
* Behavior: `ClaimShipment` now refuses with `shipment_predecessor_not_shipped` while a `blocks` predecessor shipment is not shipped (archived `shipped` counts; abandoned or unresolvable fails closed). The guard covers (P) only. 156-S and every other existing shipment are unchanged. The manual (P)+(C) 074-DL policy stays in force until AF1E5075 lands; the 154-S attestation exists (see the 188-S closure).
* F2b fold-in: claim crash recovery accepts a cascaded non-queued member-parent as a rollback candidate. The cascade also clears a stale `blocked_reason` for a blocked preimage, and the candidate matches that write set.
* Local review: READY_WITH_FOLLOWUPS at 70ddea1f (P0 0, P1 0, P2 3, P3 3; single-reviewer pass). PR HEAD cf386549 differs only by `.backlogit/stash.jsonl`. CI all green; Copilot review of #546: zero threads, gate SATISFIED.
* Local note: `golangci-lint` could not run locally (toolchain export-data mismatch); CI lint passed. One load-related lock-contention flake in `TestU20_ReconcileShipmentToShippedConcurrency` passed in isolation.
* Follow-up stash (P-021 C2): 73852299 (committed-phase recovery expectation and `ignoreUpdatedAt` widening), 27B3F6FA (confirm no ungoverned activation route bypasses the guard).
* Source artifacts untouched: stash 6434A4D7, stash E52607F5 provenance, 184-F.

## R14 Live Impact Inventory (read-only, PR-5)

Re-run 2026-10-19 21:35 PT through read-only `backlogit_query_sql` over `item_deps` joined to `items`: queued shipments with a `blocks` predecessor shipment whose status is not `shipped` or `archived`. The A2 inventory (188-S closure) stated every archived predecessor is `archived_status: shipped`; that is unchanged, and 189-S is now shipped too.

* Queued dependents the guard now refuses at claim, each behind a still-queued predecessor: 150-S, 151-S (149-S, 150-S); 161-S..169-S (chain 160-S..168-S); 149-S and 156-S (169-S); 180-S, 181-S (179-S); 185-S (184-S, 194-S); 191-S (190-S); 192-S (191-S); 194-S (192-S, 193-S).
* No dependent in the active dark-mode order (190-S onward) is refused except by its in-order predecessor, which is the intended behavior.

## Releasability

READY. Core claim-path change with MCP error mapping; rollback is a revert of the merge commit; owner: Ship.

## Compaction (P-020)

Done: compact-context (target all) ran. Its first pass archived three memory notes, but Copilot review showed retained decision and plan documents still reference their `docs/memory/` paths, so those moves were reverted and nothing was compacted. Plans and closure compaction found nothing to do.
