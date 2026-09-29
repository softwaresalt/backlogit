---
title: Stage C29EBEE5 re-validation (154-S gate) and FB0A850B archive session memory
doc_type: memory
source: docs/memory/2026-09-28/stage-c29ebee5-154s-revalidation-memory.md
---

## Context

The Orchestrator routed Stage (P-013.5 route claude-opus-5.5 / anthropic /
high) to do two things. Part 1: archive stash `FB0A850B`, because PR #456 has
merged. Part 2: run the `C29EBEE5` re-validation of the `173-F` task
contracts, which is the last gate before the `154-S` hold comes off.

* Base: local `main` matched `origin/main` at `ba303ee2`, the merge of PR #456.
* Branch: `stage/c29ebee5-154s-revalidation`. Single branch, no worktrees
  (P-016).
* Constraints held:
  * Nothing was claimed, shipped or unblocked.
  * `154-S` status and dependencies are unchanged.
  * The untracked `.backlogit/reconcile/155-S-*.md` and
    `.backlogit/telemetry.jsonl` files were left alone.

## Startup

* Tools: the backlogit MCP probes succeeded (`ALL_TOOLS_OK`). The index sync
  succeeded at session start (`INDEX_SYNC_OK`, 1770 items) and again after
  the mutations (1769 items).
* Checkpoint recovery: I enumerated all `stage` checkpoints with no filter.
  None had anomalies and none was active, so this was a normal startup.
* Hook events: I polled 104 events. All were `create_artifact`,
  `update_artifact`, or the `155-S` `ship_shipment`, and there was no
  `feature_review_ready` or `blocked_stale` signal. I acked seq 3307.
* No shipment is active.

## Part 1: FB0A850B archived

The archive criterion (the auto-tune PR merges) is met by PR #456 (merge
`ba303ee2`, commits `b855cfc3`, `ce40a29a` and `bddead5b`). I re-checked
the three findings on `ba303ee2`:

* R1: `harness-manifest.yaml:615-617` `ESCALATION_*` is now gpt-6-sol /
  openai / xhigh.
* R2: `alt_doc_review` is restored as google / gemini-3.8-flash in config and
  in the manifest (`:532-533`), and `doc-review/SKILL.md` has been re-rendered.
* R3: the stale config comments are cleaned.

I appended an archive note citing PR #456, `ba303ee2` and `075-DL`, then ran
`backlogit_stash_archive`, which is non-destructive. No late identifier
surfaced, so the N/A values stand.

## Part 2: C29EBEE5 re-validation — FAIL

This was plan-review attempt 5 on
`docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md`,
run with `dispatch_mode: multi-agent-dispatch`. Six personas returned
findings: Architecture Strategist, Go Reviewer, Scope Boundary Auditor,
Learnings Researcher, Constitution Reviewer, and Agent-Native Parity. All six
returned FAIL. Stage verified the core evidence directly in
`shipment_lifecycle.go`, `shipment.go` and `shipment_recovery.go`.

### Findings

* **F1 (P1, new work).** Claim crash-recovery CAS:
  * `memberRecoveryCandidates` compares members by full JSON against the
    preimage with only the status set to Active. A member that already carries
    the marker fails that comparison, and recovery returns
    `ErrShipmentConflict`.
  * Claim, Block, Unblock, ReturnBlocked and Ship all run recovery first, so
    this failure blocks every lifecycle operation until someone fixes it.
  * No task covers it. It needs a new unit (U1b) and its own RED test.
* **F2 (P1, design decision).** The marker's lifecycle is undefined:
  * Block, Unblock and `ReturnBlockedItem` preserve `custom_fields`.
  * So blocked members stay queued but marked. Unblocking to active restores
    them active and still marked, which refutes the C29EBEE5 "active +
    unmarked" hypothesis. Returned items keep a stale marker.
  * The U5 rule "non-empty ⇒ claim-activated" is therefore unsafe.
  * The operator must choose between (A) keeping the marker and hardening the
    rule, or (B) clearing it on block/return and re-marking on unblock.
* **F3–F8 (P1/P2):**
  * 173.003-T has stale rollback references and a mechanism that no longer
    exists; snapshot restore already clears the marker.
  * 173.006-T scenario 3 is not RED.
  * The marker is order-dependent when a manifest member is also a parent.
  * 173.004-T needs a wider test command and is exposed to the flaky test.
  * 173.007-T has 4 scenarios, which exceeds the task limit.
  * 173-F has stale text.
* **Per-task results:**
  * PASS: 173.002-T.
  * Defects: 173-F, 173.001-T, 173.003-T, 173.004-T, 173.005-T, 173.006-T,
    173.007-T.
* **Binary condition: satisfied.** The MCP server is `2c8759c3`-dirty-debug.
  The PATH CLI is v1.11.0 `131577c`, which descends from `2c8759c3`.
* **Stash `24D693E1`** does not affect 173-F: none of its commands use
  `GOOS=linux`.
* **Stash `46A898B8`** affects 173.004-T AC(3). Stash `BDA56ED8` (174.078-T,
  captured two days earlier) looks like a duplicate of `46A898B8` with the
  same root cause. Stage did not triage either one in this session.

### Records written

* Plan: an appended `## Plan Review` (attempt 5, `decision: FAIL`) and
  `<!-- plan-review-attempt: 5 -->`. This is the only earlier FAIL, at
  attempt 1, so the consecutive-FAIL circuit is not tripped.
* `154-S`: the Remaining-gate banner paragraph now records the FAIL, the
  findings summary, and the next steps. A Stage comment event was appended; it
  is git-ignored, so the banner is the tracked record.
  * Labels are unchanged: `bootstrap-exception`, `convergence-prerequisite`,
    `bootstrap-bypass-approved-conditional`, `do-not-claim-until-convergence`.
  * Status and dependencies are unchanged.
* `C29EBEE5`: a re-validation note was appended.
  * The duplicate scan (A) was clean. `6434A4D7` and `AF1E5075` are related,
    not duplicates.
  * Late-ID reconciliation (B) found no late identifier, so the N/A stands.
  * The entry stays **ACTIVE** as the tracker for the re-plan.
* The `173-F` task contracts were **not** edited. They need redesign, so this
  is not a text-only fix.

## Steps skipped (with reasons)

* 1.5 grouping: the Orchestrator targeted these entries explicitly.
* 2 deliberation: F2 is now the deliberation subject, but it needs an
  operator decision. It is listed as the next step.
* 3–5.5 plan revision, harvest and shipment: the gate FAILed and the redesign
  waits on F2. `154-S` already exists, and its manifest will change on
  re-harvest.
* 5.6 archive: `C29EBEE5` was not consumed; it stays active.

## Next steps

1. Operator: decide F2. (A) Keep the marker through block, unblock and return,
   and use the hardened consumer rule: `status == active` AND the marker
   equals the currently active shipment ID AND the item is in that shipment's
   manifest. (B) Clear the marker on block and return, and re-mark on unblock
   to active. The Scope and Parity reviewers recommend (A).
2. Stage: deliberate F2 (P-021 C6).
   * Revise U1 (F5, seam form), add U1b plus RED (F1), and re-scope
     U3/U0a/U4/U0b/U5.
   * Re-harden, re-run the Constitution Check and plan-review attempt 6.
   * Update the harvest: task text, new tasks, dependency edges, and the
     `154-S` manifest.
3. Optional: fix `46A898B8` (and triage the probable duplicate `BDA56ED8`)
   before `173.004-T` runs.
4. The `154-S` hold stays until a later plan-review attempt records PASS. The
   manual post-ship policy in the banner is unchanged.
