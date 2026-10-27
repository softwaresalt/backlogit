---
schema_version: "1.0"
doc_type: memory
title: "Compacted memory: 197-S Orchestrator dark-run preflight and descope resume"
description: "Dense extract of two Orchestrator session notes for shipment 197-S (preflight decisions, U15 descoping ruling, config-commit deviation); originals archived."
chunk_strategy: h1-h2-h3
---

# Compacted memory: 197-S Orchestrator dark-run notes

P-020 compaction (2026-10-27). Originals are preserved under `docs/archive/memory/` at the same date-relative path:

* `2026-10-09/orchestrator-197s-dark-run-memory.md`
* `2026-10-10/orchestrator-197s-descope-resume-memory.md`

## Dark-mode record

* Scope `[197-S]` only, P-017 dark mode. `merge_approval_pre_authorized: true`, `admin_fallback_pre_authorized: false`.
* Required before PR: multi-persona adversarial review. Copilot loop: wait, fix, push, reply to every comment, resolve each thread through GraphQL, repeat until a clean pass. Budget stop at 10000 AIC (measured about 2,700 AIC).

## Preflight decisions (2026-10-09)

* Stale 153-S checkpoint `checkpoint-20261008-073934.json` was out of scope and left alone; PRs #485 and #486 were already merged.
* The operator's uncommitted `.autoharness/config.yaml` routing edit blocked the pre-claim topology gate (`WORKTREE_DIRTY`). It was backed up byte-identically (sha256 `6D5E5614...661F5`) and stashed as a moderate-risk, reversible ProposedAction.
* Routes resolved from the config read: Stage `claude-sonnet-5.5`/anthropic/xhigh; Ship `claude-haiku-5.5`/anthropic/xhigh/`long_context`; Ship escalation `claude-sonnet-5.5`/anthropic/medium.
* The pre-claim gate blocked 197-S as `UNSEQUENCED_SHIPMENT`. A `dag-root` declaration was chosen over a `blocks` edge onto 196-S, which would fail `PREDECESSOR_CLOSURE_INCOMPLETE`. Staging PR #490 carried it.

## Operator ruling and descoping (2026-10-10)

* The operator ruled "Descope U15 as recommended": option 1 of the wave-2 halt note, with M re-frozen at 18 tasks, 197.016-T archived, the 197.018-T to 197.016-T edge removed, and `green_maker_closes_wave` on 197.017-T corrected from 3 to 2.
* Follow-up stash `76553D8D` holds the R10 remainder (real ShipShipment path profile). 197-S became 197-F plus 18 tasks; waves W1 = 001-008 and 017, W2 = 009, 011-015, 018, W3 = 010, W4 = 019.
* Two adversarial reviewers (Correctness, Constitution) returned APPROVE_WITH_NOTES; findings were fixed or recorded.
* Recorded P-010 deviation: the Orchestrator committed the operator's config edit (`45dea2f8`) on the shipment branch at the operator's explicit instruction, byte-for-byte, outside the Step 1.5 carve-out. It is not a precedent and not a P-021 deferred item.
* Resumed Ship run: restore no old checkpoints (they carry the 19-task set), resume at Step 3, commit the staged rename and amendments with explicit paths, and never describe R10 as fully delivered.
