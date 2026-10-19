# Ship 196-S Intake Halt — Served-Root Handoff Missing

- **Date:** 2026-10-04
- **Shipment:** `196-S` (`196-F`, nine task members)
- **Phase:** Pre-claim intake; no task was claimed, no branch was created, and no implementation began.
- **Branch / HEAD:** `main` at `2741626afdd92e16262c365c689a54545e298f6e`
- **Checkpoint:** `.backlogit/checkpoints/checkpoint-20261004-190203.json`

## Blocker

Halted with `SERVED_ROOTS_UNRESOLVED`. The Ship invocation did not provide either absolute
`served_workspace_root` or `served_storage_root`, and did not include the dispatch evidence
required by the bootstrap contract in
`docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md` (R1–R8 authorization
record, Amendment 1 carrier commit, and workspace-relative pass results for steps a–f).
Ship must not infer either served root from the repository worktree or `.backlogit/`.

The user-authorized scope names only `196-S`, satisfying the plan's bounded-scope authorization
condition, but that does not supply the missing roots or the required handoff record. No
shipment claim or task status transition occurred. The shipment remains queued.

## Preflight

- Backlog registry present; `backlogit_sync_index` succeeded (`INDEX_SYNC_OK`).
- Shipment manifest read: `196-F` plus tasks `196.001-T` through `196.009-T`, in explicit order
  from `196-S`; no member set was frozen for execution.
- Checkpoint recovery scan: 66 summaries; no validation/quarantine anomalies and no active
  Ship-owned checkpoint.
- Hook queue returned events through sequence 3549; acknowledged only the highest concrete event.
- Pipeline-topology `pre_claim` pass was pre-verified by the Orchestrator; branch creation and
  claim gates were not reached.
- Graphtor-docs status was reachable (3 sources, 197739 chunks, idle sync).
- Engram workspace binding timed out; indexed Engram analysis was not used.
- Existing operator-local untracked paths were left untouched.

## Resume

The Orchestrator must complete the named Served-Root Handoff Procedure, pass both absolute served
roots plus the required plan/R1–R8 evidence, and then route `196-S` again. On resumption, Ship must
re-attest the roots before any raw task-log read and continue only within `196-S`.
