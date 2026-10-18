---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
description: "Post-merge closure and compound refresh for 186-S (condition (b) pre-claim hold-label and attestation harness contract)."
doc_type: closure
docline:
  date: 2026-10-18T19:00:00Z
  status: final
  tags:
    - closure
    - compound-refresh
    - 186-S
schema_version: "1.0"
shipment_id: 186-S
source: docs/closure/186-S-186-F-post-merge-closure.md
title: "186-S closure and compound refresh"
---

# 186-S closure and compound refresh

Context: feature 186-F (stash AF1E5075, 074-DL L1), PR #533, merge `95b2b91240004a8e0e8698ea4a1ff6dc7de5f521`. Contract-only change: three source-shape tests, Orchestrator Step 2 and Ship Step 0.5 text, and a manifest drift record. No runtime surface, so runtime-verification is not applicable.

## Releasability

`closure_status: READY_WITH_CONDITIONS`. Monitoring: none (agent-contract text). Rollback: revert PR #533. Owner: operator. Validation window: next routed shipment claim.

Outstanding conditions (the 186.007-T verification is only partly met):

1. Live fixture-bound Ship sessions per halt row (`ship <dependent-shipment-id>` told to stop before any claim) have NOT been run. The three rows were hand simulations of Step 0.5 items 1 to 3.1 in isolated fixtures; they do not prove the installed agents enforce the gate at runtime.
2. The post-merge Orchestrator dry routing pass (stops before `TOPOLOGY_GATE: pre_claim`, no gate, no claim) has NOT been run. The attestation on `154-S` was only read and checked.
3. Fixture row directories under `logs/af1e5075-fixture/` await operator approval for removal.

## Compaction status (P-020)

`compaction_status: degraded`. compact-context ran with `target: all`, but its proposed archival of 30 memory files was reverted before this PR merged: live consumers (`.backlogit/queue/072-DL.md`, resolved checkpoints, `docs/memory/2026-10-04/...`, `docs/archive/plans/...`) still reference the original paths. `docs/memory/` is unchanged by this closure. The compaction defect is captured as a follow-up stash.

## Halt-row and proceed-row evidence (186.007-T)

* Halt rows `queued`, `abandoned`, `shipped-unattested` ran in isolated CLI-only fixtures under `logs/af1e5075-fixture/<row>/` (`--cwd <row root>`, storage root asserted as `<row root>\.backlogit`, `154-S` and `176-S` not found before seeding). Each halted `CONDITION_B_UNSATISFIED` with its row-specific reason (not shipped; archived without `archived_status: shipped`; shipped without attestation).
* Existing attestation on `154-S` (read-only check): timestamp `2026-10-06T23:31:36.0953115-07:00`, actor `operator`, leading line `CONDITION_B_ATTESTED: 154-S shipped 2026-09-30T15:44:25Z; ...`. It is after the `154-S` `shipment_status_changed` event to `shipped` (2026-09-30T08:44:25-07:00 equals 15:44:25Z). The 186-S claim was authorized on this basis.
* L1 covers agent-mediated claims only. Direct claims depend on L2 for (P). (C) stays an operator attestation until a recorded decision says otherwise.

## Source artifact provenance (untouched)

`custom_fields.source_stash_id` AF1E5075 is provenance only and was not mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` is kept: the 186-S claim, five task moves and the ship all timed out client-side and converged server-side, as documented. Observation only: backlogit MCP calls queue behind a busy server (a concurrent `go test` made every call exceed 60 seconds), so poll state rather than re-issuing.

## Follow-ups

* Stash `E62ED538` (DEFERRED SCOPE EXPANSION): step 4a retry and resume-intake scope of item 3.1.
* The live Ship-session and dry-routing checks listed under conditions above.
* Residual P3 items listed in PR #533.
