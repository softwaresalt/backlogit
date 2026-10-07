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
4. Between `07:32Z` and `07:33Z`, `.shipment-lifecycle-global.lock`, `.153-S.lock`, and every 172-F/172.0xx-T member lock were re-touched about every 40 s with no state change.

## Suspected cause

Multiple long-lived `backlogit mcp` servers serve this workspace at the same time:

* `C:\Tools\backlogit.exe` PIDs 18504, 17184, 18612, and 32260, started 2026-10-05 to 2026-10-06.
* `bin\backlogit.exe` PID 37184, this session's server.

The timed-out MCP claim is probably still retrying inside PID 37184 and contending for the lifecycle gate with another server or a pending recovery record. This is not confirmed.

## Circuit breaker

The claim has failed twice for the same operation. One more failure trips the universal breaker. Do not retry blindly.

## Operator next steps

1. Close stale editor and CLI sessions, or stop the stale `backlogit mcp` processes listed above. This is a destructive action and needs operator approval.
2. Confirm that the `.backlogit/.locks` timestamps stop advancing and that `backlogit shipment get 153-S` still reports `queued`. If a stray claim succeeded, it reports `active` instead.
3. Re-run the `pre_claim` topology gate, then re-invoke Ship on 153-S using the existing feat branch.

## Follow-up candidate

backlogit claim path: an MCP client timeout should not leave a server-side claim retrying indefinitely. Multi-server lifecycle-gate contention should surface the holder's identity.
