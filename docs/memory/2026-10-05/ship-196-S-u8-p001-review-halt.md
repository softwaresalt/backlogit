---
title: "Ship 196-S halted at U8 review gate"
description: "P-001 sequencing conflict and blocked local review readiness at U8."
doc_type: memory
schema_version: "1.0"
---

# Ship 196-S — U8 review halt

## Halt state

- Halt token: `P-001`.
- Shipment scope: `196-S` only; covering feature `196-F`.
- Branch: `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`.
- HEAD: `e380ff3b819aee7dbba81fb4e0b2db6317355b1a`.
- Structured checkpoint: `.backlogit/checkpoints/checkpoint-20261005-052350.json`
  (valid, conforming, active; `agent: ship`).
- No PR was created and no merge was attempted.

## U8 execution evidence

`196.008-T` is a red-deliverable task. The operator-selected resume checkpoint
was already resolved after successful continuation. R14 was repeated at the
task claim boundary; `pragma_database_list` returned exactly one `main` row at
`C:\Source\GitHub\backlogit\.backlogit\backlogit.db`.

The served-log read used the scoped no-follow reader for
`C:\Source\GitHub\backlogit\.backlogit\logs\196.008-T.jsonl`. It found one
shipment-claim event and zero current-epoch start records. Ship then appended
exactly one `WORK_STARTED: 196-S` event through MCP and safely re-read the log;
the current epoch now has exactly one valid start record. Do not append another.

The clean red baseline is `e380ff3b819aee7dbba81fb4e0b2db6317355b1a`.

- `go test -run=^$ -count=1 ./...`: PASS.
- `go test -count=1 -v -run '^TestUSR8_' ./tests/integration`: assertion RED
  in `DefinitionLiterals` and `CallSites`; `PreservedInvariants` passed; exit 1.
- Zero-delta check against the baseline: PASS; tracked unstaged, staged, and
  untracked sets were all empty.
- CI-pinned golangci-lint v1.64.8: PASS, zero findings. The first parallel
  invocation hit a local linter lock collision and was rerun serially.
- golangci-lint v2.13.2 no-new-debt check against merge-base
  `2741626afdd92e16262c365c689a54545e298f6e`: PASS, 0 new issues.
- LF-canonical `gofmt -l .`: PASS, empty output.
- Ship Step 4.3 repeated the pinned lint, no-new-debt, LF-canonical format,
  and U8 selector gates; all passed with the declared selector still RED.
- Step 4.1c telemetry begin returned `disabled`; no epoch context was carried.

U8 was not marked done. No task status or source file was changed.

## Review gate and policy finding

The report-only multi-persona review of `origin/main...HEAD` produced a
`BLOCKED` readiness result with unresolved P1 findings. The blocking P-001
finding is supported by these records:

- The 195-S shipment is archived at merge SHA
  `58f5bdbac22d2c051aea41a9b0a2d508b5a7b9d0`.
- Its closure artifact
  `docs/closure/195-S-claim-start-proof-post-merge-closure.md` remains
  `READY_WITH_CONDITIONS`; compaction is `degraded`, and the closure-PR
  readiness gates remain open.
- `gh pr list --repo softwaresalt/backlogit --state open` returned no open PR.
- The 196-S description says its `dag-root` waiver bypasses the topology
  predecessor-closure check only. It does not record an explicit
  `skip_policy: P-001` override.
- Workflow policy P-001 requires halt unless the operator explicitly overrides
  with `skip_policy: P-001`.

Ship recorded a P-005 telemetry event for the P-001 halt. Dark-mode status:
`DARK_MODE_HALTED`. Do not execute U9 or later waves until the operator resolves
the P-001 gate by completing the required predecessor closure or providing the
explicit policy override.

The review also flagged the 196-S `resume_checkpoint_ref` as pointing to
`checkpoint-20261004-192622.json`, a superseded historical checkpoint. Ship did
not use that pointer: recovery proceeded only from the operator-selected
`checkpoint-20261005-005349.json`. Do not change shipment planning/recovery
fields; route this finding to Stage/Orchestrator.

Other same-surface U8 harness-coverage findings remain unresolved. The U8
red-deliverable branch is no-write and forbids fix iterations, so Ship did not
edit the test harness. The follow-up layout suggestion for U7 remains captured
as stash `3B25D37F`; do not duplicate it.

## Resume requirements

1. Perform the anomaly-first checkpoint scan and require explicit operator
   selection and confirmation before restoring this checkpoint.
2. Do not resume shipment execution until P-001 is explicitly resolved.
3. Re-read the live shipment and task snapshot after the resolution. U8 is still
   active and has one valid current-epoch start record; it will be an active
   residual at the next wave admission. Do not append a second start record.
4. Preserve the U8 baseline SHA above and do not widen shipment scope.
