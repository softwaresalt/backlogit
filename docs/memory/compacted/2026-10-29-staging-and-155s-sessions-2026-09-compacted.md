---
title: "Stage and 155-S session memory, 2026-09-13 to 2026-10-01 (compacted)"
source: docs/memory/compacted/2026-10-29-staging-and-155s-sessions-2026-09-compacted.md
doc_type: memory
chunk_strategy: h1-h2-h3
schema_version: "1.0"
---

## Scope

P-020 compaction of seven Stage, Ship, and Orchestrator session notes written 2026-09-13 to 2026-10-01 for completed release units (154-S, 155-S, 149-S to 151-S correction, baseline-convergence decomposition). Uncited originals are archived under `docs/archive/memory/`. Notes still cited by backlog items, checkpoints, plans, or decisions were restored to their original paths and remain in place; only the uncited originals are archived. Retained in place as the most recent checkpoints: `2026-09-21-ship-155s-wave7-early-green-blocker.md`, `2026-09-23-ship-155s-review-circuit-breaker.md`, `2026-09-30-ship-154s-post-merge-closure.md`.

## Decisions and outcomes

* 2026-09-13 Stage reliability and PR 425 security hardening: dark mode (P-017) with merge pre-approval true, admin fallback false, pre-claim force false. Eight stash entries triaged and archived. Work stayed on `chore/stage-154-s-reliability`, harvest commit `88c54c7a`, then review-fix cycle 1. Original: `2026-09-13-stage-reliability-security-hardening.md`.
* 2026-09-16 trust-chain decomposition correction (stash 866FDC8C): shipments 149-S, 150-S, 151-S (features 168-F, 169-F, 170-F) amended in place. Oversized tasks became declaration-only (168.001, 169.001, 170.001) with new split tasks for the loader, external-pin gate, core mutation, thin CLI wiring, parsers, and request-identity-digest design (168.011 to 168.013, 169.010, 170.011, 170.012). Six deferred-scope stash entries consumed and archived. Original: `2026-09-16-stage-866fdc8c-decomposition-correction.md`.
* 2026-09-19 baseline-convergence restage (PR 449): `156-S` (175-F plus 98 tasks) decomposed into wave-aligned replacement shipments `157-S` to `169-S` (RS-W00 to RS-W12), later joined by prerequisite `176-S` (14 shipments), chained by `blocks` edges. `156-S` kept queued with `items: []`. Only Stage-owned backlog and eight planning docs were ported; no Go source. Original: `2026-09-19-baseline-convergence-decomposition-session.md`.
* 2026-09-20 shipment-claim and wave-scheduler convergence (PR 449): historical record only. `155-S` shipped later (PR 450 at `2c8759c3`, closure PR 451); `154-S` was the remaining bootstrap shipment (chain `154-S -> 176-S -> 157-S..169-S`) with its P-002.6 active-residual halt waiver unapproved at that time. Original: `2026-09-20-shipment-claim-wave-scheduler-convergence-session.md`.
* 2026-09-20 155-S wave 1 quality-gate blocker (174-F): operator authorized the P-002.6 active-residual admission bypass only for the bootstrap set `{155-S, 154-S}`; lint, format, review, CI, topology, merge-strategy, and merge-approval gates were not waived. Wave 1 blocked at the quality gate. Original: `2026-09-20/155-s-wave-1-quality-gate-blocker.md`.
* 2026-09-27 Orchestrator PR 452 merge and 153-S halt: Stage resumed a checkpoint and harvested stash `CB8887AF` into shipments `177-S` to `181-S`. Staging PR 452 merged at `505b1bd3` (merge commit). The `ship 153-S` request halted at the pipeline-topology gate. Local review `READY_WITH_FOLLOWUPS`; routing-drift findings captured as stash `FB0A850B`. Original: `2026-09-27/orchestrator-pr452-merge-and-153-s-halt-memory.md`.
* 2026-10-01 shipment ledger clarification: shipment assignment is distinct from actual work start; required tasks need explicit accounting under flat manifests; removal or return is not completion; archiving alone does not prove completion; a completed parent must not hide unfinished required work. The clarification is not a condition-B attestation, claim authorization, merge approval, or dark-mode activation. Original: `2026-10-01/orchestrator-shipment-ledger-clarification-memory.md`.

## Learnings

* Pre-claim topology gate depends on declared sequencing; unsequenced shipments need an explicit DAG-root declaration (see later 197-S and 202-S notes).
* Replacement shipments must be task-ID-only and chained by `blocks` edges; supersede a parent shipment non-destructively.
