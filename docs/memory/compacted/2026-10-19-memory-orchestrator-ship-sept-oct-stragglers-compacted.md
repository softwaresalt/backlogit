---
title: Orchestrator and Ship stragglers, 2026-09-27 to 2026-10-04 (compacted)
doc_type: memory
description: P-020 post-merge compaction of three unreferenced session notes for archived shipments 153-S and 196-S and the shipment-ledger clarification.
---

# Compacted memory: 2026-09-27 to 2026-10-04 stragglers

Originals are archived under `docs/archive/memory/` (same date subfolders).

## 2026-09-27 Orchestrator PR 452 merge and 153-S halt

* Stage harvested `CB8887AF` into shipments `177-S` to `181-S`. Staging PR #452 merged at `505b1bd3` with a merge commit; local `main` was synced fast-forward only.
* Review findings were `READY_WITH_FOLLOWUPS` with no P0/P1. Routing drift went to deferred stash `FB0A850B`. Copilot threads led to `176-F` becoming the canonical `source_stash_id` carrier, a constitution bump reclassified from PATCH to MINOR (1.0.0 to 1.1.0), and a stale-text fix.
* `ship 153-S` halted at the pre-claim gate (`UNSEQUENCED_SHIPMENT`). It is blocked on `154-S`. A `blocks` edge encodes only "154-S shipped"; the Orchestrator must also check separately that `154-S` has shipped provenance (`shipped`, or `archived` with `archived_status: shipped`) and that the scheduler consumes the marker. The same rule applies to `141-S`, `152-S`, `147-S`, `177-S`, `178-S` and `179-S`.
* Follow-ups: operator decision on the `154-S` bootstrap waiver, stash `513E62AB` (explicit edges plus a consumption-aware, shipped-only pre-claim check), and `FB0A850B`.

## 2026-10-01 Shipment ledger clarification

* Requirements: shipment assignment is distinct from work start. Required tasks and subtasks need explicit accounting. Removal or return is not completion. Archiving does not prove completion. A completed parent must not hide unfinished work. Shipment completion needs evidence for every ledger obligation.
* The consumer of the work-start marker is Ship's Step 4.0 wave-admission logic. A same-status update (`internal/core/shipment_lifecycle.go:978-1016`) writes no marker, so a per-task move-to-active does not establish work start for members activated by claim.
* The live tool build `2c8759c3` predates the 154-S producer merge `6d233d21`, so consumer verification needs the correct producer version.
* Next step: separate deliberation on claimed-versus-started semantics, reusing `182-F` / `182.001-T`. The bootstrap decision needs operator authorization; do not fabricate an attestation or remove edges. `187-S` is the first foundation, not an authorized dispatch.
* The assessment was read-only. No checkpoints were dispositioned.

## 2026-10-04 Ship 196-S P-002.6 harness-command halt and Amendment 2 resume

* Wave 1 of `196-S` halted because `196.004-T` (U4) and `196.007-T` (U7) lacked `harness-ready` or `harness-exempt` status and did not meet the P-002.6 `^TestU<unit>_` task-scoped command contract. The diagnostic was `P-002.6_TASK_SCOPED_COMMAND_CONFLICT` and the shipment was paused through the governed block (resume checkpoint `checkpoint-20261004-192622.json`).
* Stage's reviewed Amendment 2 (`7cb55c4a`) supplied red deliverables `196.001-T` (`^TestUSR1_`), `196.003-T` (`^TestUSR3_`) and `196.008-T` (`^TestUSR8_`), and a closed exempt set of `196.002-T`, `196.004-T`, `196.005-T`, `196.007-T` and `196.009-T`.
* The governed unblock was run with `confirm: true`. The R14 attestation and topology gate passed with `196-S` as the sole active shipment. Waves: {001, 003, 004, 007, 008}, {002, 005}, {009}, {006}. The scheduler simulation returned `WAVE_SIM_OK` (200/200 assertions).
* Failed approaches: changing tests or acceptance scope to force compliance was rejected, and the claim call timed out and was not retried. A `pre_claim` probe against a blocked shipment returned `BACKLOG_UNAVAILABLE`, which was expected.
