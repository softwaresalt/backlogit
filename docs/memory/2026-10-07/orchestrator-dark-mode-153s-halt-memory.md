---
title: "Orchestrator dark-mode run for 153-S halted at pre-claim"
description: "Dark factory run scoped to 153-S halted before Ship routing: predecessor 154-S successor-routing attestation (074-DL condition b) is absent."
doc_type: memory
schema_version: "1.0"
---

# Orchestrator Dark-Mode 153-S Halt

## Outcome

`DARK_MODE_HALTED` before any Ship invocation. No shipment was claimed, no
branch was created, and no backlog item changed status. The only mutation was a
derived-index `backlogit sync`.

## Activation record

- `DARK_MODE_SCOPE`: ordered `[153-S]`; cursor last=none, next=153-S.
- `merge_approval_pre_authorized`: true. `admin_fallback_pre_authorized`: false.
- Repo: `main` at `8abc4c97` (equal to `origin/main`), single main worktree.
- Served roots: workspace `C:\Source\GitHub\backlogit`, storage `.backlogit`.

## Halt reason

`autoharness gate pipeline-topology --mode agent --shipment 153-S --phase pre_claim`
exited 1 with `PREDECESSOR_CLOSURE_INCOMPLETE` for predecessor `154-S`.

This is a correct block, not a gate defect:

- `docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md`
  records `closure_status: READY_WITH_CONDITIONS`, `compaction_status: degraded`,
  and invariant 4: do not route any successor behind 154-S until the external
  scheduler owner attests the deployed consumer uses the marker contract.
- Decision `074-DL` (condition b) requires that no multi-member shipment is
  claimed before that consumption is attested. 153-S has 16 members.
- No `CONDITION_B_ATTESTED` event exists in `.backlogit/logs/154-S.jsonl` or
  under `docs/`.

Using `--force` would override an operator-owned decision and the recorded
rollback trigger, so the run halted per P-017 stop conditions.

## Operator action required

1. The external autoharness scheduler owner records the condition-b
   attestation for 154-S (tracked by stash `AF1E5075`).
2. Update the 154-S closure evidence so the topology gate observes the cleared
   condition. Recording the attestation alone does not clear
   `PREDECESSOR_CLOSURE_INCOMPLETE`: the gate reads `closure_status` from the
   predecessor closure artifact. Either promote `closure_status` from
   `READY_WITH_CONDITIONS` to `READY`, or record the condition as satisfied in
   the artifact's `conditions:` block, citing the attestation evidence.
3. `compaction_status: degraded` is non-blocking for routing. Completing the
   degraded 154-S P-020 compaction pass (stash `A17EE897`) is still
   recommended hygiene, but it is not a routing precondition.
4. Re-run the pre-claim topology gate for 153-S and confirm exit 0, then
   re-run `Run pipeline in dark mode` scoped to 153-S.

## Alternatives not taken

DAG-ready shipments without the 154-S hold (for example 141-S, 152-S, 177-S)
were out of the declared scope; some of them are also covered by
074-DL condition b. Silently substituting them would violate P-017
no-silent-scope-expansion.
