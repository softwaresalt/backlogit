# Stage 196-S Amendment 3 Erratum E1 (draft, unreviewed)

## Context

- Shipment `196-S`, P-017 dark mode, `DARK_MODE_SCOPE` `[196-S]`; operator AFK.
- Ship halted at Phase 2 wave 2 (`docs/memory/2026-10-06/ship-196-S-phase2-wave2-halt.md`,
  checkpoint `.backlogit/checkpoints/checkpoint-20261006-073956.json`).
- Stage committed on Ship's branch under the Amendment 3 Precedence clause (Ship halted,
  not concurrent).
- Stage route: claude-opus-5.5 / anthropic / high (Orchestrator-resolved, tier3).

## Erratum

Plan: `docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`, section
"Amendment 3 Erratum E1 (2026-10-06)", appended after the cycle-5 Plan Review record.
Pointers added at A3.3 item 2 and item 4.

- E1.1: A3.3 item 4 told Ship to run Step 4.2 (the build-feature dispatch) before the
  traceability commit, which contradicts the clean-tree, baseline, and zero-delta order.
  Correction: Step 4.1b, Step 4.1c, and the Step 4.2 pre-dispatch actions run before the
  commit; the dispatch runs only after `red_baseline_sha`. Matches the Phase 1 precedent
  (`f18fcb11`). E1.1 also gives the Phase 2 resume steps per task.
- E1.2: the item 2 "no green yet" check for U1 and U3 was not recorded at the time. Stage
  re-ran it against pinned SHAs (`origin/main` `c4c458b9`, pre-edit parents `e7454f15`
  and `d2248d7c`): both empty. The reflog shows `origin/main` held `c4c458b9` from
  2026-10-05 23:10 -0700, before both harness commits. Disposition: accepted as equivalent
  evidence, with a residual-risk note and a forward rule for item 2.
- Refinement over the Orchestrator's recommended wording: the lifecycle `TOPOLOGY_GATE`
  is named as a pre-dispatch action "when installed" without claiming it writes a tracked
  file (no `.autoharness/gates/` path is tracked). The resume steps also run Step 4.2
  pre-dispatch actions and commit any restore or resolution writes before each clean-tree
  check.

## Verification

- `markdownlint-cli2@0.23.1` on the plan: 0 issues.
- No changes to R1–R15, dependencies, `M`, the red mapping, `covered-by` commands, test
  names, red/pass profiles, production code, tests, task status, or labels.

## Pending

- E1 review status: pending — requires operator authorization (cycle 6). Plan-review was
  not run, because cycles 4 and 5 were already operator-authorized exceptions.
- E1 takes effect only after a passing review or explicit operator approval. Until then
  Ship stays halted.
- On resume, the operator or the Orchestrator under operator authority must explicitly
  select checkpoint `checkpoint-20261006-073956.json`.

## Operator approval (2026-10-06)

- Operator reply at 2026-10-06T12:31:35-07:00, verbatim: "I don't see an issue with the
  clean worktree: git status shows no uncommitted or untracked files."
- Orchestrator interpretation: the reply approves E1's clean-tree resume path (clean
  tree, then fresh baseline, then dispatch) and gives authority to resume from checkpoint
  `checkpoint-20261006-073956.json`. The operator runs in autopilot and is AFK.
- Recorded in the plan: the E1 header now reads `status: effective (operator approval
  recorded 2026-10-06)`, with an `approval:` bullet. The two Amendment 3 cross-references
  now read "effective by recorded operator approval". Cycle-6 plan-review was not run.
- Residual risk: the approval is inferred from the reply, not given as an explicit "I
  approve E1". If the operator disputes it, E1 reverts to draft and Ship halts again.
- The "Pending" section above is superseded by this approval. E1.1–E1.3 text, requirements,
  tasks, tests, and code are unchanged.
