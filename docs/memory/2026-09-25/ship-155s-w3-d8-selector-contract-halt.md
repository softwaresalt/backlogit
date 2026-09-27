---
title: Ship 155-S W3 D8 Selector Contract Halt
date: 2026-09-25
status: blocked
shipment: 155-S
task: 174.076-T
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: edb8d88be551e84d439354aeb685693755994766
---

# Ship 155-S W3 D8 Continuation

## Resume and amended contract

The operator explicitly selected `checkpoint-20260925-202625.json`; backlogit
returned `valid: true`. The amended task was re-read through backlogit and the
rev22.4 plan amendment D8 was reviewed. D8 reclassifies 174.076-T as a normal
harness-required task. Its dependency 174.075-T is `done`; 174.076-T was queued
and now carries `harness-required`, `p002-4-reclassified`, and `harness-ready`.
The task was moved to `active`.

## Manifest preservation

The Stage-reviewed deliverable initially matched SHA-256
`28d7424a2969e5beffe654824f35779cda6bc8f9ac29fa23bc6c4cfcf494c306`.
It was copied byte-exactly to
`logs/diagnostics/174076-manifest-deliverable-backup.yaml`; source and backup
hashes matched, and `logs/` is ignored. With the operator's explicit Option C
approval, Ship restored the manifest to HEAD for the red phase, verified
`git diff --quiet HEAD -- .autoharness/harness-manifest.yaml` exited 0, then
restored the backup after claiming the task. The working manifest and backup
now both hash to the Stage-reviewed value.

Safety action records:

* `ProposedAction`: copy the W3 manifest to the ignored backup path.
  `ActionRisk: moderate`; `ActionResult: applied`; source and backup hashes
  match.
* `ProposedAction`: restore the manifest to HEAD for red verification.
  `ActionRisk: destructive`; `ActionResult: approved` by operator Option C,
  then `applied`; backup retained and verified.
* `ProposedAction`: restore the verified W3 manifest after task claim.
  `ActionRisk: destructive`; `ActionResult: approved` by the same Option C
  scope, then `applied`; backup and restored hashes match.

## Harness red evidence

The delegated harness-architect execution created and committed only
`tests/integration/harness_manifest_budget_reconciliation_test.go`:

* `go vet ./tests/integration`: exit 0.
* `go test ./tests/integration -run
  '^TestHarnessManifestBudgetReconciliation$' -count=1 -timeout=5m -v`: exit
  1, exactly one top-level FAIL and zero top-level PASS. The
  `workflow-policies` and `ship-agent` subtests failed assertions (c) and (d)
  on the expected stale manifest, with no build or YAML parse error.
* Harness commit: `edb8d88be551e84d439354aeb685693755994766`
  (`test(core): add manifest budget reconciliation harness`).
* Claim-time baseline after harness commit: `edb8d88be551e84d439354aeb685693755994766`.

## Halt before green

The amended task requires the exact selector
`^TestHarnessManifestBudgetReconciliation$`. Ship's P-002.6 task-scoped
command contract requires a selector anchored to `^TestU<unit>_`. The task's
exact selector does not meet that requirement. Ship cannot rename the
Stage-specified test, substitute a different selector, or relax P-002.6 in
session. Halt occurred before AC4/AC5/AC6, the implementation commit, task
completion, or push. The harness-only commit remains local; the task remains
active; the manifest deliverable remains uncommitted in the working tree.
Ship recorded a P-002.6 telemetry event and a task comment requesting Stage
amendment. No full or repository-wide test command was run.

The explicit checkpoint `checkpoint-20260925-202625.json` remains unresolved
because the requested post-W3 push sequence did not complete. Its previous
state should not be auto-selected next session; select a checkpoint by
filename under the Ship recovery protocol.

Created replacement halt checkpoint `checkpoint-20260925-205309.json` only
after strict JSON pre-validation (payload SHA-256
`6e324104d4621dcdda4f85a3db472bbce83e7913b778346a3e9099ad36386049`) and
confirmed `backlogit checkpoint get` returned `valid: true`. The selected
`202625` checkpoint remains unresolved per the requested post-push ordering;
`checkpoint-20260923-222732.json` remains untouched.
