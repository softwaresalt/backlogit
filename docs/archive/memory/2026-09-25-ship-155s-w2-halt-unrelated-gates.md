---
title: Ship 155-S Halt During W2 After Unrelated Gate Findings
date: 2026-09-25
status: blocked
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 6f6c34e134de3f643b0fd6cfd1a67ab0e83906b4
---

# Ship 155-S W2 Halt

## Resume and W1 closure

Restored Ship checkpoint `checkpoint-20260925-054533.json` through backlogit;
`get_checkpoint` reported `valid: true`. The operator had selected this checkpoint
and authorized continuation of shipment `155-S` on the existing feature branch.
The pre-existing snapshot is
`logs/diagnostics/155s-w2w3-preexisting-snapshot.json` (46 paths).

Committed the already-completed 174.074-T queue-to-archive relocation as
`6f6c34e134de3f643b0fd6cfd1a67ab0e83906b4`:
`chore(core): archive completed task 174.074-T`. The governed commit association
was recorded. No other task or shipment was claimed.

## W2 174.075-T

Baseline: `6f6c34e134de3f643b0fd6cfd1a67ab0e83906b4`. The exact pre-work exempt
command exited 1 at both Ship Step 4.1a and build-feature Step 0, with no marker,
as required while the W1 contract remained red. The command captures its test
output in `$o` and exits before printing it on this expected failure; both runs
used Go 1.24.0.

Only the declared W2 text edits were made to `.github/agents/_ship.agent.md` and
`.github/policies/workflow-policies.md`. The operator-owned tools line in
`_ship.agent.md` was not edited, staged, or committed; its pre-W2 whole-file
snapshot hash was
`84ae9310879a4593534e9633ba25664fb228cfad6afd1cbb5c0d8caf58c0ca88`.

The post-deliverable exempt command exited 1 without
`EXEMPT_VERIFY_OK:174.075-T`: its content probe passed and it reached the
read-only docs lint, which reported the pre-existing
`docs/scratch/2026-09-24-155-s-session-resumption.md` missing required `source`
and `doc_type` fields. This file is outside the two authorized W2 paths and was
not changed. The required two-selector regression command also exited 1: one
top-level PASS and one FAIL. `TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery`
failed because the preserved operator tools line contains `backlogit/*`.
Existing deferred entry `4A0B7BCF` positively covers that same allowlist
expansion; it was reused, not duplicated or edited.

The unrelated doc-lint expansion was captured threadlessly as P-021 C2 entry
`2F7FCA8B` before any finding was closed. Discovery found ambiguous partial
source-ref candidates, so the capture carries `DISCOVERY-STATUS: AMBIGUOUS` and
all candidate IDs. Its PR and review-thread references are correctly `N/A`
because this was pre-PR and threadless. Task comments were appended to
174.075-T and 174.076-T with the capture and delta status.

## Fail-closed waiver halt

The delta report is
`logs/diagnostics/155s-w2w3-delta-report.json`. It records the W2 scope, gate
evidence, snapshot comparison, reused/captured P-021 entries, and W3 as not run.
The W2 deliverable paths are within the declared docs-only surface after the
approved lifecycle and operator-line exclusions, but the task completion gate
did not pass. No W2 deliverable or lifecycle-completion commit was made.

The closed W-174074-ext comparison then found an unapproved changed snapshot
path: `.backlogit/hooks_queue.jsonl`. Its SHA-256 changed from
`0d1b8e7e1a0a22345264801a1200d1f3a124724005fceff02bab842805f880b9` to
`1741e441ea1f93dfe419aab7ad650a6619bb42b0d7ddba2aafad1929b10bcd78`. This
path is outside the waiver's explicit exclusions. It was not restored or
edited. Stop fail-closed and ask the operator/Stage to review this exact
unexpected path and the unrelated doc-lint finding before resuming.

174.075-T remains active; 174.076-T remains queued. W3 was not claimed, its
pre-work command and delta gate were not run, and the manifest was not changed.
No push or full-suite run occurred. The requested post-W3 checkpoint phase and
completion memory file were not created because W3 did not complete. The
selected checkpoint `054533` was not resolved; other active checkpoints were
not touched.
