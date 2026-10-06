---
title: "Orchestrator 196-S dark-mode halt — Ship P-010 self-report during U7 review"
date: 2026-10-05
agent: orchestrator
shipment: 196-S
status: DARK_MODE_HALTED
---

# Orchestrator 196-S Dark-Mode Halt (P-010)

## Event

`DARK_MODE_HALTED` — scope=[196-S], gate=P-010/P-017 stop condition, outcome=halted,
next action=operator-confirmed checkpoint recovery.

## State at halt

* Branch `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`, not pushed, no PR.
* History: `746217be` (U7 exempt baseline) → `c8412ad1` (checkpoint 235817 resolve) →
  `e50fcf8a` (additive revert of c8412ad1, per the approved remediation) → `dba97931`
  (U7 test commit, only `internal/mcp/served_root_self_report_test.go`).
* `git diff --name-only 746217be HEAD` = only the U7 test file, so the net P-002.4 delta is compliant.
* U7 (196.007-T) is still `active`. Its Step 4.3 gate is not recorded and it is not done.
* Uncommitted: Ship hand-edited the U7 test file during the report-only review. It is a
  structural refactor to a table-driven `t.Run` (+72/-58, same assertions) and is untested.
* Untracked, intentionally not committed (the standing rule forbids administrative commits
  inside U7's window):
  * `.backlogit/checkpoints/checkpoint-20261005-005349.json` (active Ship halt checkpoint)
  * `docs/memory/2026-10-04/ship-196-S-halted-u7-exempt-delta-conflict.md`
  * `docs/memory/2026-10-04/orchestrator-196-S-u7-revert-remediation-direction.md`
  * this file
* Checkpoint 235817 is active again after the revert. It is a known stale candidate: do not
  infer it as a recovery target.
* `.git/info/exclude` still holds the temporary `196-S` block. Remove it after merge.

## Why halt

* Ship self-reported a P-010 violation: a direct source edit instead of delegating to
  build-feature. It logged telemetry and halted.
* P-010's violation action is "record and halt" (`workflow-policies.md` ~1423).
* P-017 lists "any policy violation that P-005 requires surfacing and halting" as a dark-mode
  stop condition (`workflow-policies.md` ~1721).
* The proposed autonomous ADOPT path was adversarially reviewed and REJECTED:
  * build-feature Step 0 would now report `EXEMPT_FALSE_GREEN`, because the U7 test already
    exists at `dba97931`, so there is no defined re-entry or adoption mode;
  * its "stage all" commit protocol conflicts with the three untracked files;
  * any new commit requires a current-HEAD re-review.
* DISCARD and DROP both require a destructive `git restore` outside the dark-mode contract.
  DROP would also leave an in-scope P-021 C3 finding unresolved.

## Operator decisions required

1. Pick the disposition of the uncommitted refactor:
   * (a) adopt it through a governed review-fix path, defining how build-feature re-enters
     for an exempt task whose deliverable is already committed;
   * (b) approve a destructive restore and re-apply it via a governed path; or
   * (c) approve a restore and accept the committed form, with an explicit residual-risk
     record for the convention finding.
2. Explicitly select and confirm the Ship checkpoint to resume (`005349`), per the
   crash-resumption protocol.
3. Then: U7 Step 4.3 gate and current-HEAD review; administrative commit (re-resolve 235817,
   resolve 005349, the three memory files); U8; waves 2–4 (002, 005, 009, 006); reviews; PR;
   Copilot loops; merge commit; post-merge closure.

## Completed so far

U1, U3, U4 done; U7 committed (`dba97931`) but not gated or marked done.

## Operator Resolution (2026-10-04T21:41-07:00)

* Operator approved: "approve restore with patch backup; resume from 005349".
* Patch backup (local, gitignored): `.copilot/session-state/cdeb3886-a918-4832-981e-e94970ed3efc/files/196-S-u7-table-driven-refactor.patch`, SHA256 `280169C7E9748A1BC5F5FEFD05C83CAC4B0DB94B7BA156EF8236C5619FBB4B87`, reverse-apply check passed; full-file copy alongside (`.full.go.txt`).
* Disposition: drop-with-capture. Ship restores the test file, captures a P-021 C2 `DEFERRED SCOPE EXPANSION` for the table-driven conversion, and records residual risk in the PR body.
* Process gap stashed by Orchestrator: `6BF65C9D` (build-feature exempt-task re-entry mode plus Ship no-hand-edit standing rule).
* Checkpoint 005349 explicitly selected for resume; 235817 to be resolved in the post-gate administrative commit.
* DARK_MODE resumed; Ship routed with the ordered resume steps and the new standing rule.
