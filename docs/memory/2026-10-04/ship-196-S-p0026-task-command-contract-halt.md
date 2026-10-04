# Ship checkpoint — 196-S P-002.6 harness-command contract halt

- **Date:** 2026-10-04
- **Shipment:** `196-S` (the only authorized shipment in this P-017 dark-mode run)
- **Branch:** `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
- **HEAD at halt:** `d53c1a10`
- **Shipment state:** `blocked`
- **Resume checkpoint:** `.backlogit/checkpoints/checkpoint-20261004-192622.json`
- **Wave:** 1 of 4

## Completed before the halt

- Resumed the operator-selected Ship checkpoint under explicit Orchestrator confirmation.
- Re-attested the served workspace and storage roots and applied plan R14.
- Created the one permitted feature branch from the synchronized `main`.
- Preserved and committed the six approved continuity/checkpoint files as the first
  feature-branch commit; `.github/copilot/` remains excluded and untouched.
- Claimed only `196-S`; the claim call timed out, but a subsequent shipment read showed
  `active`, and the post-claim topology gate confirmed it was the sole active shipment.
  No claim retry was made.
- Intake reconciliation returned `PROCEED`. Frozen task set `M` has nine tasks; `196-F`
  is the sole explicitly listed non-task member. The scheduler simulation returned
  `WAVE_SIM_OK` (200/200 assertions across 26 scenarios).
- Performed the R14 attestation and safe raw-log reads for all nine task IDs. Every task
  was claim-assigned before the governed shipment block; no task start record was written
  and no build or implementation was dispatched.
- Committed the successful recovery-checkpoint resolution. The older wave-schedule
  checkpoint `checkpoint-20261004-192256.json` was subsequently resolved after the newer
  wave-1 checkpoint became the resumption point.

## Halt reason

Wave 1 cannot pass the harness admission contract as currently written. Tasks `196.004-T`
(U4) and `196.007-T` (U7) have neither `harness-ready` nor `harness-exempt` satisfaction,
and their current acceptance selectors do not meet P-002.6's required task-scoped,
task-owned `^TestU<unit>_` command contract. Changing tests or acceptance scope to force
compliance would contradict their reviewed task constraints. Ship cannot amend backlog
planning fields or invent a substitute harness contract.

This is recorded as the local diagnostic
`P-002.6_TASK_SCOPED_COMMAND_CONFLICT`, not represented as a canonical policy halt token.
No harness was generated, no task was started, and no code was changed. Stage must supply
a reviewed contract amendment before Ship can continue.

## Governed pause and reconciliation

Ship used the governed shipment block operation. The shipment is now `blocked`; its nine
task members and feature record are currently `queued`, while the committed block envelope
records their exact pre-block `active` statuses. The lifecycle log contains correlated
intent, applied, and committed evidence. No `return_blocked` operation was used.

- Intake reconciliation: `.backlogit/reconcile/196-S-pre-20261004T192015Z.md`
- Blocked-envelope pre-mode audit: `.backlogit/reconcile/196-S-pre-blocked-20261004T192946Z.md`

The blocked envelope is resumable but normal execution remains paused. Do not unblock,
claim, scaffold, or execute any task until Stage provides an approved amendment and the
Orchestrator authorizes resumption. Then validate the blocked envelope, use the explicit
confirmed governed unblock path, and repeat shipment/wave admission and the R14 checks.
Keep the same feature branch and the immutable task manifest.

## Delivery state and risks

- No tasks completed; all 9 remain queued after the block.
- No PR, local/adversarial review, CI, Copilot review, merge, or post-merge closure.
- No product code or harness changes.
- Scope remains exactly `196-S`; no additional shipment may be claimed.
- Operator-local `.github/copilot/` files remain untouched.
- The user-requested stash discharge note for `227A2930`, `B83081F5`, and open
  `359D8F32` remains pending post-merge closure.
