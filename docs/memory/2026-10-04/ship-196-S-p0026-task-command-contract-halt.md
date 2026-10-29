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

## Resumption under Amendment 2

- **Orchestrator re-dispatch:** 2026-10-04, P-017 dark mode remains bounded to `196-S`.
  Stage's reviewed Amendment 2 is on this branch at `7cb55c4a`; its plan section,
  Stage memory, task contracts, and closed exemption set were read. Stage reports ADVISORY
  reviews with no P0/P1 findings and recorded operator authorization under the bounded
  scope.
- **Blocked-envelope pre-mode:** `.backlogit/reconcile/196-S-pre-blocked-resume-20261004T200843Z.md`.
  The exact ten explicit members were present once in queue, all `queued`, no archive
  copies existed, and the exact-manifest active-status snapshot and correlated committed
  block-operation record matched. The report recommended the governed unblock.
- **R14 resume attestation:** the metadata catalog reported the supplied canonical workspace
  and storage roots; `pragma_database_list` returned exactly one `main` row at
  `<served_storage_root>/backlogit.db`; path-component and direct-child checks passed; and
  the static shipment frontmatter agreed with the MCP shipment ID, blocked status,
  timestamp, and ordered manifest.
- **Resume:** `backlogit_unblock_shipment` was invoked with `confirm: true`, target `active`,
  actor `ship`. An independent shipment read showed `active`; the global post-claim topology
  gate passed with `196-S` as the sole active shipment and the expected branch/worktree.
  The extra `pre_claim` gate probe made while the shipment was still blocked returned
  `BACKLOG_UNAVAILABLE` because that claim-admission phase rejects blocked status; it was
  not a claim gate and was not used to authorize or bypass the separate governed unblock.
- **Fresh schedule inputs:** after unblocking, the live ordered manifest still has 10
  members; exact type resolution gives 9 task members (`M`) and excluded `196-F` (`feature`).
  All nine tasks currently read `active`. The re-read dependency graph is acyclic and
  yields waves `{196.001-T, 196.003-T, 196.004-T, 196.007-T, 196.008-T}`,
  `{196.002-T, 196.005-T}`, `{196.009-T}`, `{196.006-T}`.
- **Amended contracts:** red deliverables are `196.001-T` (`^TestUSR1_`, closes wave 2
  via `196.002-T`), `196.003-T` (`^TestUSR3_`, closes wave 2 via `196.005-T`), and
  `196.008-T` (`^TestUSR8_`, closes wave 3 via `196.009-T`). The closed exempt set is
  `{196.002-T, 196.004-T, 196.005-T, 196.007-T, 196.009-T}`; each label, contract class,
  owner/dependency where applicable, exact command, and plan membership passed static
  intake. All nine frozen green-regression arrays are `[]`.
- **Checks so far:** the preflight compile-only command passed. The required tracked
  scheduler simulation returned `WAVE_SIM_OK` (200/200 assertions, 26 scenarios).
- **Next:** perform Step 4.0's fresh whole-`M` snapshot and active-member classification.
  Repeat full R14 immediately before any raw task-log read; use only the previously
  declared nine exact paths and the no-follow path-safety procedure. No task start or
  implementation has been dispatched yet.
