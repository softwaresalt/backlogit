---
title: "Orchestrator 196-S U7 revert remediation direction"
description: "Decision memory for resolving the U7 EXEMPT_DELTA_EXCEEDS_CLASS halt with an additive revert"
ms.date: 2026-10-05
---

# Orchestrator 196-S U7 revert remediation direction

## Context

Ship halted 196.007-T (U7, `verification-only`) with P-002.4 `EXEMPT_DELTA_EXCEEDS_CLASS`.
After Step 4.1a recorded `exempt_baseline_sha=746217be` and Step 4.1b claimed U7, Ship committed
`c8412ad1`. That commit resolved checkpoint `checkpoint-20261004-235817.json` and put an
out-of-class path inside U7's delta window.

## Rejected option

Re-baselining U7 to `c8412ad1` by re-running Step 4.1a after the claim. An adversarial review
rejected it: the baseline must be captured before the claim and passed unchanged, so a
post-claim recapture launders the out-of-class mutation. History rewrite was also rejected,
because it needs operator approval and the operator is AFK.

## Decision

Make an additive, non-destructive `git revert` of `c8412ad1`, which touches only the checkpoint
path. The checkpoint returns byte-identically to its `746217be` state, so the net delta
`746217be..HEAD` excludes it. The original baseline `746217be` stays unchanged, and U7 keeps its
single `WORK_STARTED` record. The second adversarial review returned APPROVE-WITH-CHANGES, and
both required controls are adopted:

1. U7 path-stages only `internal/mcp/served_root_self_report_test.go`. The cached path set and
   `git diff-tree` of the U7 commit must equal exactly that file. The untracked halt artifacts
   stay unstaged.
2. Checkpoints are resolved individually:
   * 235817 is re-resolved under its prior confirmed resume.
   * 005349 is resolved only after Ship confirms successful continuation from that halt.

## Standing rule

No harness, checkpoint, or memory commit may land between a task's baseline capture and its
Step 4.3 gate. Administrative commits happen after the gate and before the next task's
Step 4.1a.
