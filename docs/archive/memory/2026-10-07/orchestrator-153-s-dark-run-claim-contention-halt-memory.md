---
title: Orchestrator 153-S dark run halted on claim lock contention
date: 2026-10-07
type: session-memory
---

# Orchestrator 153-S dark run halted on claim lock contention

## Outcome

`DARK_MODE_HALTED` for scope 153-S. No tasks, PR, or merge were produced.

## Completed before the halt

* Operator attested 154-S closure condition (b) (`CONDITION_B_ATTESTED`, `2026-10-07T06:31:36Z`). The attestation was recorded and merged in PR #484 (merge `0201e770`) after three resolved Copilot review rounds.
* The 153-S `pre_claim` pipeline-topology gate passes on `main`.
* The Served-Root Handoff Procedure passed: workspace root `.`, storage root `.backlogit`, the catalog and `pragma_database_list` attested, and the manifest matched MCP (16 items, queued).
* Ship was invoked. It created branch `feat/153-s-concurrency-safety-hardening-for-artifact-mutation-writers` at `0201e770`.

## Halt chain (UTC)

1. MCP `backlogit_claim_shipment(153-S)` timed out.
2. The single bounded CLI re-claim at `07:23:33Z` failed: `lock shipment lifecycle recovery: task .backlogit/.locks/shipment-lifecycle-global: backlogit: gate in progress for item`.
3. The post-claim gate returned `CLAIM_NOT_OBSERVED` (exit 3). 153-S is still `queued`, `updated_at` is unchanged, and there are zero active shipments.
4. Between `07:32Z` and `07:33Z`, `.shipment-lifecycle-global.lock`, `.153-S.lock`, and every 172-F/172.0xx-T member lock were re-touched about every 40 s while the in-flight claim progressed.

## Correction (2026-10-07T20:14Z)

The original diagnosis in this record was wrong. Verified evidence:

* Process working directories (read from each process PEB) show only PID 37184 serves this workspace. PIDs 18504 and 18612 serve `engram`, PID 17184 serves `autoharness`, and PID 32260 had exited before it could be checked. There was no multi-server contention.
* The timed-out MCP claim kept running inside PID 37184 and **succeeded**: `153-S.jsonl` records `shipment_status_changed` at `07:34:13Z`, and member `status_changed` events run through `07:36:11Z`. 153-S and all 16 members (172-F, 172.001-T to 172.015-T) are `active`.
* Root cause: Ship's CLI re-claim at `07:23:33Z` collided with this session's own still-running MCP claim on the global lifecycle lock (self-contention). The post-claim gate sampled state before the background claim committed, so it returned `CLAIM_NOT_OBSERVED`.

## Resume guidance

* Do not re-claim. The shipment is already claimed.
* The claim state (18 queue files plus `hooks_queue.jsonl`) is committed on this feat branch.
* Resume Ship from the post-claim gate, then from the WORK_STARTED marker (Step 4.0), on this branch.

## Follow-up candidate

backlogit claim path: when an MCP client times out, the server-side claim continues and can complete later, so a client retry races it. Either cancel the server-side operation on client timeout, or have the lifecycle-gate error identify the in-flight operation so callers wait instead of retrying. Ship's claim convergence should also poll for a delayed claim result before declaring `CLAIM_NOT_OBSERVED`.