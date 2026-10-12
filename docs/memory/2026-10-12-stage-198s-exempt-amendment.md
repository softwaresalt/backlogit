---
schema_version: "1.0"
doc_type: memory
title: Stage 198-S exempt-set amendment and operator-ruled T1/T3 red phase
description: Stage added the closed harness-exempt set and exemption contracts for 198.004-T and 198.005-T (labels and blocks applied and read back; stash captures moved out of the gated delta; 40-minute timeouts), recorded the operator's Option A ruling for 198.001-T and 198.003-T with conditions R1 to R6 and green-regression contracts, and ran a second plan-review series after the ruling. Working-tree only; Ship commits by explicit path. Records the P-005 events, the expected dirty set, and Stage's own disclosures.
timestamp: "2026-10-12T19:40:00.0000000Z"
---

# Stage 198-S exempt-set amendment and operator-ruled T1/T3 red phase

## Status

Scope `[198-S]` only (dark mode, P-017). Amendment A1 of
`docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md` was written in two
revisions on 2026-10-12.

* **First revision (06:30Z).** Escalated Ship's mutation-kill request to the
  operator. Plan review ended FAIL after three cycles (final cause: the draft
  claimed label withdrawal would halt Ship's intake, and it does not), a
  stop condition. T4 and T5 were applied; T1 and T3 were held.
* **Operator ruling (17:35:08Z, "Ruling: option A").** Recorded verbatim in plan
  A1.4 as `T1=A; T3=A`. The operator answered nothing else. The Orchestrator
  relayed it in its dispatch to Stage (2026-10-12T10:35 PDT): "OPERATOR RULING
  RECEIVED (dark mode, scope 198-S unchanged). Channel: Copilot CLI session
  a3f32a34-316b-4580-965d-f0a3e5ca2ab4, operator message timestamp
  2026-10-12T17:35:08Z (10:35 PDT). Verbatim operator words: "Ruling: option A".
  Interpretation (Orchestrator, stated for the record): Option A as defined in
  your A1.4 applies to BOTH 198.001-T and 198.003-T (Option B is T1-only; the
  operator did not choose B or C)." The session is the retrievable source.
* **Second revision (after the ruling).** Records the ruling with conditions R1 to
  R6 and the Principle II conflict record with the VII and VIII checks; defines
  R6's run mechanics and the green-regression contracts; resolves the remaining
  A1.7 items inside policy (stash-capture ordering moved to the post-merge
  closure, 40-minute timeouts); and ran a second plan-review series after the
  ruling, authorized by the Orchestrator's dispatch (the operator's own words on
  record are only "Ruling: option A"). The final `## Plan Review` section of the
  plan carries the outcome and governs.

The shipment manifest S and frozen wave set M, the task set, task scope, and
dependencies are unchanged, and the acceptance-criterion meaning is unchanged.
Two criteria carry errata: T4 AC1 (where the stash IDs are recorded) and T5 AC2
(which the evidence artifact and the closure share), plus T5's `-timeout` wording
(plan A1.1 and A1.3). A third erratum is a disclosed replaced outcome: T5's
"record CT closure as verification-incomplete" branch cannot complete an exempt
task, so Ship leaves T5 non-done, reports it through P-005, and halts with 198-S
left active (no `backlogit_return_blocked` call; A1.3 Erratum, A1.5, A1.7 item
6). Ship passes the criterion-touching errata to its Step 4.4 review.

## P-005 events

No `agent-intercom` is installed, so visibility is degraded and this note is the
record.

1. `POLICY_GAP P-004 / INC-P002-131S-148F FC-1,FC-2.4,FC-3 / 198.001-T, 198.003-T`.
   Stage could not relax; escalated. **Resolved by the operator's Option A
   ruling** (plan A1.4), on conditions R1 to R6.
2. `PLAN_REVIEW_FAIL / Amendment A1 / first series, three cycles`. Superseded by
   the fresh series after the ruling (final Plan Review section).
3. `PRINCIPLE_IV / Stage's first exempt-command fixtures were written under
   %TEMP%\stage-198s, outside the workspace.` **Still open for the operator.**
   Stage did not delete the directory (destructive, outside the workspace); the
   Orchestrator's dispatch directed that nothing be deleted. Later fixtures are in the ignored
   `logs/stage-198s-scratch`.
4. `P-010 / Stage ran backlogit docs lint, scripts/md-lint.ps1, go list ./..., and
   pwsh fixtures to test Stage-authored exempt commands` in the first revision.
   **Still open for the operator** to confirm the reading or record the event.
   In the second revision Stage ran pwsh and git fixtures only, with a
   fixture-only `go` shim, and no Go toolchain, linter, or build.

## Superseded Ship hand-off text

Ship's halt note (`docs/memory/2026-10-12-ship-198s-wave1-halt.md`) said the
`harness-ready` labels on 198.001-T through 198.003-T were reusable and that Ship
should resume at wave 1 admission after a Stage exempt-set amendment. Both halves
now hold: the exempt-set amendment is done, and the labels stand on A1.4's terms
(Option A, R1 to R6), which the checkpoint `resume_hint` omits.

## Changes (all working tree; Stage ran no state-changing git operation)

Committed-by-Ship paths (one docs commit before Step 4.1a, after Ship runs
`backlogit docs lint` and `scripts/md-lint.ps1` on them):

* `docs/exec-plans/2026-10-08-ct-test-suite-health-plan.md` (Amendment A1)
* `docs/decisions/2026-10-12-ct-harness-lifecycle-ruling.md`
* `docs/memory/2026-10-12-stage-198s-exempt-amendment.md` (this note)

`.backlogit/**` carry-forward written by Stage through backlogit (Ship commits it
separately before capturing `exempt_baseline_sha`):

* `198.004-T`, `198.005-T`: `harness-exempt` label, the A1.3 contract block
  (re-applied after the stash-ordering and timeout changes), a note on the
  amended details, and a comment each.
* `198.001-T`, `198.003-T`: a P-002.6 `green-regression-contract` block, a note on
  the ruling, and a comment each. `harness-ready` untouched.
* Stash entries 920BCA5B, 253E2505 (edited 2026-10-12 to say it no longer blocks
  198-S), 5A6F54BB, F8936891, DAEE9D03, 5F5D7384, 2462F1B0.
* Stage's hook events in `.backlogit/hooks_queue.jsonl`. Ship's own uncommitted
  claim, label, and hook-queue writes (events 3722 through 3726) and its
  `198-F.md`, `198-S.md`, `198.002-T.md` writes are untouched.

Expected `git status --short` set at hand-off: the three docs paths above,
`.backlogit/stash.jsonl`, `.backlogit/hooks_queue.jsonl`,
`.backlogit/queue/198-F.md`, `198-S.md`, `198.001-T.md` through `198.005-T.md`,
and Ship's untracked checkpoint `.backlogit/checkpoints/checkpoint-20261012-050053.json`.
Any other dirty path is a Ship halt.

## Why Stage did not commit

`_orchestrator.agent.md` (Concurrency Control) keeps Stage and Ship on separate
branches, and P-010 names the default branch or a dedicated admin branch as
Stage's commit venue. The shipment branch is neither, and Ship's tree already
carries uncommitted writes. The 197-S precedent
(`docs/memory/2026-10-10/stage-197s-red-contract-amendment.md`) is working-tree
only, for Ship to commit, and the Orchestrator re-approved that landing in the
2026-10-12 dispatch. Stage created no branch, worktree, or PR.

## Open for the operator and the Orchestrator

Gate state first. The final `## Plan Review` is `decision: ADVISORY`, now carrying
the approval marker. Cycle 3 returned three PASS and three ADVISORY (five P2s).
The Orchestrator approved a fourth cycle, which returned Correctness PASS and five
ADVISORY (six P2s: operator attribution, stash reads not read-only, T5
enforcement point, staging and status mechanics, T5 non-green mechanics, and a
false claim that Ship's intake halts). Stage fixed all six in text; those fixes
are unreviewed. On 2026-10-12T22:15:18Z the Orchestrator, as dark-mode proxy under
the P-017 activation for 198-S, accepted the ADVISORY with no fifth cycle and
accepted the residual risk; the mitigation is that Ship's Step 4.4 adversarial
review re-checks A1.5's dispatch constraints and T4 and T5 bullets and captures
wording defects as stash entries (P-021 C2) instead of editing the plan. The
acceptance waives no NON-NEGOTIABLE control; the T1 and T3 relaxation rests solely
on the operator's "Ruling: option A". Ship's intake has no check of the review
decision of its own; the plan's Intake bullet is the check.

1. Ship's own gates keep `-timeout=30m` against an `internal/core` run that
   reached 92% of it (stash F8936891); a halt there returns to the operator
   (A1.7 item 3).
2. Dispose of `%TEMP%\stage-198s`, or authorize Stage to; nothing was deleted
   (A1.7 item 4(a)).
3. Confirm the P-010 reading of Stage's first-revision authoring probes, or record
   a P-010 event (item 4(b)).
4. Confirm the two Ship Role Boundary readings, or have the resume dispatch say
   otherwise: T4's spike deliverable through `build-feature` (item 4(c)), and the
   R6 mutation executor as a general-purpose subagent writing only ignored scratch
   files (item 4(d)). Without confirmation Ship halts with P-010 at the first
   affected step.
5. Answer the one question A1.4 left open: whether P-002's own red-phase
   statements are covered by the Option A ruling (A1.7 item 1).

Resolved by Stage inside policy: stash captures (A1.3 ordering; P-002.4 not
relaxed), T4 and T5 timeouts (40m), checkpoint handling (the Orchestrator resumes
the same Ship session; Stage touches no checkpoint).

## Stage's own disclosures

* To validate the two exempt commands in the first revision Stage ran read-only
  authoring probes: `backlogit docs lint`, `scripts/md-lint.ps1`, `go list ./...`,
  and pwsh fixture runs. P-010 literally lists build systems and linters among
  what Stage must not run. Ship's Step 4.1a probe and post-deliverable gate
  remain the observations of record.
* The first fixtures were written under `%TEMP%\stage-198s` (outside the working
  tree, against Constitution Principle IV). Later fixtures live in the ignored
  `logs/stage-198s-scratch`, which is git-ignored and uncommitted.
* The second revision's 85 fixture cases (T4 v5, T5 v6) ran in pwsh with read-only
  git only. The `go list` call was answered by a fixture-only shim over a package
  list derived from `git ls-files`; the `backlogit docs lint` arm was stripped.
  Neither arm was exercised for real. Ship's Step 4.1a probe never reaches them
  (the deliverable is absent), so plan A1.5 asks Ship for two stand-alone smoke
  runs before the post-deliverable gate, the first full execution.
* Stage ran no `go test -overlay`, linter, or markdown lint after the first
  revision. Ship runs `backlogit docs lint` and `scripts/md-lint.ps1` on the three
  docs before committing them.
* The task-body text edited under Stage's authority is limited to notes and
  parameters: T4 and T5 gained an amendment note (T5's `-timeout 30m` became 40m
  in its description and reason line), and T1 and T3 gained a ruling note and a
  green-regression block. No acceptance-criteria block was changed.
