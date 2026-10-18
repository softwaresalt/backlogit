---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 198-S ship sessions (CT test-suite health)"
description: "Dense summary of the 198-S (feature 198-F) ship checkpoints from wave-1 halt through operator-ratified closure."
chunk_strategy: h1-h2-h3
---

# 198-S ship sessions (compacted, 2026-10-18 UTC)

Compacts eight 198-S memory notes (2026-10-12 to 2026-10-14); `2026-10-12-ship-198s-wave1-converged.md` was also covered by the earlier 2026-10-16 compaction. All eight originals were moved, not deleted, to `docs/archive/memory/2026-10-12/`, `2026-10-13/` and `2026-10-14/`. The authoritative closure record is `docs/closure/198-S-198-F-post-merge-closure.md`.

## Sequence and outcomes

* **Wave 1 halt (2026-10-12)**: dark mode halted at a P-002 harness gap on 198.004-T; no task was dispatched, no PR opened. Stage then recorded an exempt-set amendment and an operator-ruled red phase for T1/T3.
* **Mutation-kill evidence (R6)**: T1 (198.001-T) and T3 (198.003-T) mutation-kill runs were recorded against the delivering commit.
* **Review rounds (2026-10-13)**: pre-PR review halt, delta review round 2 with build evidence and a readiness halt, and a round-3 review with a direct R6 re-run that ended in an operator-decision halt. Each halt preserved the `active` shipment state per P-017 (no `return_blocked`).
* **Closure (2026-10-14, PR #495)**: the `ship_shipment` MCP call returned -32001 on safe-close; a governed CLI `backlogit archive 198-S` completed the P-007 named-limitation closure. The operator ruled "Ratify" on 2026-10-14T05:25:03Z, which counts as the P-007 reconciliation.

## Key facts retained

* No `ship_shipment` hook event exists for 198-S; none was synthesized. The only hook entries are `move_shipment_status` and `archive_item`. See `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md`.
* Quiescence was verified read-only before ratification: no lock or archive signal advanced after 04:40:30Z.
* Copilot review body 5513401133: the high finding was operator-decided by the ratification; the low finding (stash delta count 13, not 12) was corrected in the reconcile report.
* `doctor --check-shipped-event-completeness` reported zero 198-S findings; 41 pre-existing unrelated findings remained.

## Failed approaches

Treating a timed-out MCP ship as complete without completion-proof evidence; relying on wording that said ratification was pending after it was received (corrected in four places).
