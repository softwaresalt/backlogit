---
schema_version: "1.0"
doc_type: memory
title: Stage 196-S harvest and shipment assembly (195-S follow-up)
description: Operator authorized the ADVISORY plan review; Stage resumed at Step 5, harvested 196-F with six tasks, assembled shipment 196-S at queue_position 100, archived 227A2930 and B83081F5, and stashed follow-up CDBCB258.
timestamp: "2026-10-03T08:11:00Z"
---

# Stage 196-S harvest and shipment assembly

## Session status

Complete. This session resumed Stage checkpoint `checkpoint-20261003-075017.json`
(session `stage-2026-10-03-195s-followup`, phase `plan-review-advisory-awaiting-operator`)
and is the continuation of `stage-195s-followup-dispatch-closure-staging.md`.

* **Recovery.** The Orchestrator's Step 0.0b scan found one active `stage`-owned
  checkpoint and no quarantine anomalies. The operator selected it, confirmed the resume,
  and authorized the ADVISORY outcome with the reply "Approved"
  (2026-10-03T08:00:44Z, via the Orchestrator). The checkpoint was valid and conforming.
  Prune-on-restore kept the cursor, the plan path, and the gate verdict, and pruned
  nothing else.
* **Gate.** `operator_authorization: approved` and its provenance were appended to the
  final `## Plan Review` section of the plan. The `skip_review` gate then validated that
  section: `dispatch_mode: multi-agent-dispatch`, `decision: ADVISORY`, and
  `operator_authorization: approved`.
* **P-003.** The source decision exists, the plan cites it, each task cites the plan and
  its parent `196-F`, and each task has acceptance criteria.
* **Checkpoint.** Resolved at 2026-10-03T08:10:51Z, after the harvest commit.

## Artifacts

| ID | Unit | Wave | Blocks on | Source stash | Harness note |
|---|---|---|---|---|---|
| `196-F` | feature (high) | n/a | n/a | n/a (description cites both) | n/a |
| `196.001-T` | U1 Orchestrator handoff contract test | 1 | none | n/a | Red deliverable; green maker `196.002-T`, closes wave 2 |
| `196.003-T` | U3 shipment-reconcile contract test | 1 | none | n/a | Red deliverable; green maker `196.005-T`, closes wave 2 |
| `196.004-T` | U4 ShipShipment characterization test | 1 | none | n/a | GREEN on arrival; a RED result is a P-021 halt |
| `196.002-T` | U2 Served-Root Handoff Procedure | 2 | `196.001-T` | `227A2930` | Careful mode; dry-run on one queued and one archived shipment |
| `196.005-T` | U5 reconcile contract text and Ship Step 6 | 2 | `196.003-T`, `196.004-T` | `B83081F5` | Careful mode |
| `196.006-T` | U6 harness-manifest drift records | 3 | `196.002-T`, `196.005-T` | n/a | Hash the bytes as checked out |

* **Shipment `196-S`.** Priority high, `custom_fields.queue_position: 100`, and no
  shipment dependency. Items in this order: `196-F`, `196.001-T`, `196.003-T`,
  `196.004-T`, `196.002-T`, `196.005-T`, `196.006-T`. The read-back confirmed seven items,
  and `covering_feature` is `196-F`.
* **Scope guard.** `harvest_ids` were exactly the seven IDs above. No pre-existing queue
  item was added.
* **Stash archival.** `227A2930` and `B83081F5` were archived by the harvest operation
  itself (reason `harvested`, linked to `196.002-T` and `196.005-T`). Nothing was
  destructively removed.
* **Follow-up stash.** `CDBCB258` (feature, medium) covers served-root intake on direct
  Ship invocation, Ship-side re-verification of the binding evidence, the Orchestrator's
  terminal `SERVED_ROOTS_UNRESOLVED` mapping for the Ship halt, and pinning the payload key
  names. Stage it after `196-S` ships.

## Execution preconditions carried to Ship

* **Bootstrap dispatch.** `196-S` dispatches before U2 lands. The Orchestrator applies plan
  R1–R8 only after it records explicit operator authorization. A P-017 bounded dark-mode
  scope that names `196-S` counts as authorization. Otherwise it halts with
  `SERVED_ROOTS_UNRESOLVED`.
* **Closure.** Before pre-close, Ship confirms three things: the merge commit is local,
  `SKILL.md` contains `feature-pending-governed-completion`, and the binary that runs
  ShipShipment descends from `47dfcc93`. Otherwise Ship halts with
  `RECONCILE_CONTRACT_NOT_IN_FORCE`.

## Deferred entries (still triaged, not harvested)

* **`731CE551`** (model-routing re-render; `DEFERRED SCOPE EXPANSION`). Needs
  deliberation. **Planned ordering (note only):** its future shipment should take a
  `blocks` dependency on `196-S`, so the re-render follows the U6 drift records.
* **`41FE00A1`** (Step 1.5 continuity allowlist). Needs a secret-safety deliberation and
  co-deliberation with `2B8B3E84`. **Planned ordering (note only):** its future shipment
  should take a `blocks` dependency on `196-S`.
* **`359D8F32`** (docs compaction, low). Independent.

No shipment exists for these entries yet, so no edge was created.

## Next step

The Orchestrator publishes the Stage commits through its Step 1.5 staging PR. After that
merges, `196-S` is ready for Ship under the bootstrap dispatch precondition.

## Amendment 1 (PR #474 review, 2026-10-03T21:07-07:00 operator directive)

* **Trigger:** Copilot thread `PRRT_kwDORzozKM6olYEw` on `196.002-T`. The served-root
  binding compared copyable content, so it could not tell a copied workspace from the store
  the MCP server is bound to. Ship never re-attested the MCP root.
* **Authorization:** the operator directed "PR 474: Additional Copilot review comments to
  fix", relayed by the Orchestrator as authorization to amend the approved plan and the
  harvested items.
* **Design:** a read-only Served-Root Attestation against existing MCP self-report surfaces:
  `backlogit_get_metadata_catalog` (`workspace.root_path`, `workspace.storage_root`) and
  `backlogit_query_sql` `pragma_database_list`. It runs at dispatch (U2) and at Ship use
  time (U9), and halts with `SERVED_ROOT_ATTESTATION_FAILED`. It needs no write, no Go
  change, and no new MCP surface. The nonce comment-write design and the new Go field were
  rejected. The Orchestrator brief's premise that no MCP tool reports the root was wrong.
* **Review:** a focused delta review by two personas (Security Lens, Agent-Native Parity)
  returned ADVISORY: 0 P0, 0 P1, 6 P2, 5 P3, all applied to the plan body. The plan's final
  `## Plan Review` section records `operator_authorization: approved` with provenance.
* **Backlog changes:**
  * New tasks: `196.007-T` (U7, wave 1), `196.008-T` (U8, RED, wave 1), and `196.009-T`
    (U9, wave 3).
  * New `blocks` edges: `196.002-T` on `196.007-T`; `196.009-T` on `196.008-T`,
    `196.007-T`, and `196.005-T`; `196.006-T` on `196.009-T`.
  * Updated `196-F`, `196.001-T`, `196.002-T`, and `196.006-T` (now wave 4).
  * `196-S` now has 10 items, reordered parent-first and by dependency.
  * Stash `CDBCB258` was rescoped.
* **Deferred residuals (`CDBCB258`):** direct-Ship intake; halt mappings for both tokens;
  the P-002.2 and P-002.6 mirror; payload key pinning; and attestation of Ship backlogit
  invocations outside Step 4.0 and Step 4.1b.
