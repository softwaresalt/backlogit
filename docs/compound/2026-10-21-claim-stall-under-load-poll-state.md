---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Claim stalls under concurrent load: poll state, do not kill processes or retry"
description: "A backlogit_claim_shipment MCP timeout left 189-S half-claimed and every backlogit call failed with gate in progress. No process held the lock handles and recovery completed by itself. CLI claim, move and ship calls took 3 to 15 minutes under load but converged."
doc_type: learning
source: docs/compound/2026-10-21-claim-stall-under-load-poll-state.md
docline:
    date: 2026-10-21T00:00:00Z
    severity: medium
    tags:
        - shipment
        - lock
        - mcp
        - timeout
        - ship-agent
---

# Claim stalls under concurrent load: poll state

## Problem

Claiming 189-S timed out over MCP (-32001). `189-S.md` showed `status: active`, a pending
journal sat in `.backlogit/ops/`, and every CLI or MCP open failed with
`recover shipment operations: lock shipment lifecycle recovery ... gate in progress for item`.
Retries for about 2 minutes did not help. Another session was running concurrent
`backlogit update ... --size` calls.

## Root Cause

The global shipment-lifecycle lock is an exclusive-handle sidecar with a bounded wait of
3 seconds. Under heavy load from a concurrent session and test runs, the claim and its
recovery contended for it. A Windows handle scan across all processes found no holder of
`.backlogit/.locks`, so the stall was transient, not a dead process.

## Resolution

Wait and re-check with a read-only command (`backlogit shipment get <id>`). Recovery
completed without intervention and the claim was confirmed. Do not kill `backlogit mcp`
processes or delete lock or journal files.

## Prevention

* After an MCP timeout on a shipment lifecycle call, poll state instead of re-issuing.
* Use the CLI after the Served-Root Attestation when MCP is saturated; expect 3 to 15 minutes.
* Confirm with the post_claim topology gate that the claim landed.
* See also `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md`.
