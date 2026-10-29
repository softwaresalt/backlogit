---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S claim and Step 3 schedule-construction halt
description: Ship claimed 197-S under P-017 dark mode and halted at wave schedule construction because seven red-deliverable tasks carry no canonical red-deliverable-contract and no execution-readiness metadata. No task was dispatched and no PR was opened.
timestamp: "2026-10-10T02:48:00Z"
---

# Ship 197-S claim and Step 3 schedule-construction halt

## Session status

**HALTED (DARK_MODE_HALTED) at Ship Step 3, schedule construction.** Scope
`[197-S]` only. The shipment is claimed and `active`, is the sole active
shipment, and has no task work or start records. No PR exists. Branch
`feat/cx-ship-closure-protocol-and-gate-correctness` was created from
`main` at `d9d0237b62759ac0fe130e2a6868b9b469a12ff5`.

## Served-root handoff outcome (SERVED_ROOT_HANDOFF_PASSED)

Recorded in workspace-relative form, after `BRANCH_CREATED`, as the Orchestrator
requested. The roots were re-attested at claim time, not taken from the handoff:

* `backlogit_get_metadata_catalog`: `workspace.root_path` is the served workspace
  root, and `workspace.storage_root` is `.backlogit` under it.
* `pragma_database_list` (`main`): `.backlogit/backlogit.db`, one row.
* Manifest 197-S resolved through `backlogit_get_shipment` after one
  `backlogit_sync_index` (`indexed: 1964`).

## Gate and claim log

| Step | Result |
|---|---|
| P-011 branch gate | `BRANCH_CREATED: feat/cx-ship-closure-protocol-and-gate-correctness` (tree clean, `main` fast-forwarded and already equal to `origin/main`). |
| TOPOLOGY pre_claim (before branch) | exit 0, `topology gate pass`. |
| TOPOLOGY pre_claim (immediately before claim) | exit 0, `topology gate pass`. |
| `backlogit_claim_shipment` | MCP `-32001` request timeout. Follow-up reads showed the claim had landed: `status: active`, `updated_at` 2026-10-10T02:42:09Z. The shipment lifecycle lock was held by the timed-out call and cleared on its own. No retry was needed. |
| TOPOLOGY post_claim (global) | exit 0, `CLAIM_VERIFY_OK`, 197-S is the sole active shipment. |

The claim cascaded all 19 member tasks from `queued` to `active`, with a
`status_changed` event (reason `shipment claimed`) in each item log at
2026-10-10T02:42Z to 02:44Z. Each member carries `scheduler_baseline_claim: 197-S`
and no `WORK_STARTED` record. They are therefore **claim-assigned** under Step
4.0, not active residuals. Nothing has been dispatched.

## Expected wave partition (from `item_deps`, all 19 members in M)

| Wave | Members |
|---|---|
| 1 | 197.001-T (U1), 197.002-T (U1b), 197.003-T (U2), 197.004-T (U3), 197.005-T (U4), 197.006-T (U5), 197.007-T (U6), 197.008-T (U7), 197.017-T (U16a) |
| 2 | 197.009-T (U8), 197.011-T (U10), 197.012-T (U11), 197.013-T (U12), 197.014-T (U13), 197.015-T (U14), 197.016-T (U15) |
| 3 | 197.010-T (U9), 197.018-T (U16b) |
| 4 | 197.019-T (U17) |

`M` is 19 task IDs. Feature `197-F` is in the manifest but is excluded from M
(not a task wave). The scheduler replay (`scripts/wave-scheduler-sim.ps1
-VerifyAgainstQueue`) reported `WAVE_SIM_OK` (200/200). That replay checks the
scheduler's logic against its fixtures, not this shipment's contract state.

## Halt reason

**`WAVE_RED_MAPPING_UNRESOLVED` (P-002.2, schedule construction).** Seven members
declare in their own bodies that their deliverable is a red harness ("RED is the
deliverable", "fails on main ... RED output recorded"): 197.001-T, 197.002-T,
197.003-T, 197.004-T, 197.005-T, 197.006-T, and 197.017-T. None carries a
canonical `red-deliverable-contract` block with its five keys. Ship cannot
supply those blocks (planning fields belong to Stage), and it must not infer the
green-maker mapping from prose or dependency direction.

Execution-readiness gaps found in the same pass, which Stage must also resolve
before the schedule can be admitted:

* **No member carries `harness-ready` or `harness-exempt`.** No member has a
  `harness-exemption-contract` block either. Under P-002.1, the docs-only and
  verification-only exemptions are admissible only with a complete contract block.
* **197.016-T (U15)** says "harness: exempt" in prose only. It changes production
  code (`internal/core/shipment_gate.go`). P-002.1 admits behavior-changing
  production code only through `covered-by` with a red, dependency-linked owner.
  As written, the exemption is not admissible.
* **197.007-T (U6)** is a benchmark spike with no harness class; its admissibility
  needs Stage's decision.
* **197.008-T (U7)** is a closure-registration docs task. It needs a docs-only
  exemption contract, which is absent.
* **197.019-T (U17)** is verification-only with a checksum refresh. It needs a
  verification-only contract (exact command and precondition), which is absent.

Plan-derived green-maker mapping, offered for Stage to validate. It is not
recorded in the backlog, and Ship has not used it:

| Red deliverable | Declared green-maker(s) | Green-maker wave |
|---|---|---|
| 197.001-T (U1) | 197.009-T (U8) | 2 |
| 197.002-T (U1b) | 197.009-T (U8) | 2 |
| 197.003-T (U2) | 197.010-T (U9) | 3 |
| 197.004-T (U3) | 197.011-T (U10) | 2 |
| 197.005-T (U4) | 197.012-T, 197.013-T, 197.014-T | 2 |
| 197.006-T (U5) | 197.015-T (U14) | 2 |
| 197.017-T (U16a) | 197.018-T (U16b) | 3 |

## Crash-resumption check

`backlogit_list_checkpoints` (consumer `ship`) returned 83 checkpoints. Exactly
one is ship-owned and active: `checkpoint-20261008-073934.json` (153-S, already
shipped and archived). Per the Orchestrator's decision it was not restored,
resumed, or resolved. No 197-S checkpoint existed, so there was nothing to
resume. No checkpoint needs quarantine.

## Decisions

1. Halted at Step 3 instead of dispatching `harness-architect` to the non-RED
   tasks. Partial execution would violate P-002.6 (no wave partial admission),
   and the RED tasks would otherwise be driven green in their own wave.
2. Left 197-S `active`, not `blocked`. The claim is resumable under Step 4.0 as
   claim-assigned, and no task has a start record. This avoids an extra governed
   mutation while the MCP server is degraded.
3. Did not commit the claim-driven queue-file changes (`.backlogit/queue/197*.md`).
   They stay in the working tree for the next invocation. The unrelated
   `.backlogit/hooks_queue.jsonl` change was also left unstaged.
4. Copilot review gate: the operator asked for `--enforcement auto`, but the
   workspace profile sets `copilot_review.enforcement: required` (900 s). Ship
   uses the stricter profile value, `required`, which fails closed.
5. The operator's parked `.autoharness/config.yaml` edit (`stash@{0}`) was not
   touched.

## Resume point (for Orchestrator / Stage)

1. Stage amends the contracts: add canonical `red-deliverable-contract` blocks
   to the seven RED tasks, and resolve the harness-exempt or covered-by
   admissibility gaps above (including 197.016-T's prose-only exemption).
2. Orchestrator re-invokes Ship for 197-S, resuming at Step 3 with the claim
   already in place. Ship re-checks the active claim, which must still be the
   sole active shipment (post_claim), and runs the wave schedule again.

## Follow-ups

None captured yet. No P-021 deferred-scope entries were created in this halt.
