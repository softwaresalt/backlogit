---
title: Stage 155-S 174.076-T P-002.4 Reclassification (AC6 Return)
date: 2026-09-25
status: complete
agent: stage
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head_at_start: 1a0641757b66094f5ba4d531d0d4bdeb53674f76
---

# Stage: 174.076-T P-002.4 Reclassification

## Trigger

At W3, Ship's P-002.4 path pass halted 174.076-T with `EXEMPT_DELTA_EXCEEDS_CLASS`. The
`docs-only` surface is "Markdown, instruction, prompt, and agent artifacts only", and the
YAML `.autoharness/harness-manifest.yaml` falls outside it. Following AC6, Ship returned the
task to Stage. The evidence is in `logs/diagnostics/174076-p0024-delta-report.json`.

## Decision: Option A (D8, plan rev22.4)

* **Reclassification.** 174.076-T is now a normal harness-required task. This involves no
  policy text change and no waiver.
* **Why no existing harness works.** No existing test fails on the stale manifest. The only
  test that reads the manifest, the `npx ` forbid, stays green.
* **New harness.** harness-architect scaffolds one test function under this task:
  `TestHarnessManifestBudgetReconciliation` in
  `tests/integration/harness_manifest_budget_reconciliation_test.go`. For each entry it checks:
  * the YAML parses with no duplicate keys;
  * `drift_allowed` is true;
  * the `073-DL rev22` sentence appears exactly once;
  * the checksum is 64 hex characters and differs from the stale value.
* **No currency check.** The harness does not check that checksums are current, which meets
  D7's over-coupling objection. Exact currency is covered by the task-gate probe, which is the
  former exempt command with its marker renamed to `CHECKSUM_PROBE_OK:174.076-T`.
* **Read-only verification.** Stage checked the design with python yaml and hashlib. On HEAD
  the harness is red on (c) and (d); on the working-tree deliverable (file SHA-256
  `28d7424a…c306`) it is green.
* **Rejected options:**
  * B: a P-002.4 amendment. This needs operator authority, is out of scope, and is unnecessary.
  * `covered-by`: not smaller.
  * Dropping the task: contradicts the W1–W3 instruction.
  * Waiver: operator-only.

## Mutations

* **174.076-T, via backlogit:**
  * labels are now wave-19r, 155-S, harness-metadata, harness-required, and
    p002-4-reclassified;
  * the exemption block was removed;
  * AC1–AC9 and the implementation notes were rewritten;
  * status moved from active to queued;
  * a decision comment was appended.
* **Unchanged:** the dependencies (`blocks` on 174.075-T) and 155-S membership (39 items).
* **Plan:** the rev22.4 amendment section was appended, and it passes docs lint.
* **Not touched:** the uncommitted manifest, `_ship.agent.md`, the Ship checkpoint
  `checkpoint-20260925-202625.json`, PR #449, 154-S, and E1–E5.

## Next (Ship)

1. Resume from the Ship checkpoint.
2. Set the deliverable aside under a hash-verified backup, then restore the HEAD manifest.
3. Run harness-architect to confirm red, then commit the harness alone.
4. Apply `harness-ready` and claim the task.
5. Restore the backup and run the green gates.
6. Commit the manifest alone.
7. Push, then STOP for the single governed full-suite authorization.

## Operator notes

* **Tools line.** The working-tree `_ship.agent.md` tools line reads
  `vscode/memory, backlogit/*, engram/*`. That is the regression 174.072-T corrected, and it is
  tracked by deferred entry 4A0B7BCF.
* **Full-suite risk.** While that line stays in the tree, any full-suite run over it fails the
  existing `TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery` test. That
  test requires the lifecycle trio and forbids `backlogit/*`. The operator must resolve this or
  authorize the swap procedure for the governed run.
* **Restore step.** The set-aside step restores over an uncommitted file. If Ship's P-002.5 /
  Principle VII screen routes it, it needs operator approval.
