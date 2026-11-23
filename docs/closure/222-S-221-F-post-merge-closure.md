---
title: "222-S / 221-F post-merge closure"
description: "Post-merge operational closure and releasability evidence for Shipment 222-S (Feature 221-F)."
doc_type: closure
source: docs/closure/222-S-221-F-post-merge-closure.md
ingested_at: "2026-11-23T22:17:10Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
docline:
  shipment_id: 222-S
  feature_id: 221-F
  merge_commit: 740dba66604973a8adfd1f19e2489c389a64e207
  close_path: safe-close
  close_evidence: docs/closure/evidence/222-S-221-F-close-evidence.json
  closure_status: READY
  compaction_status: done
---

## Release and governed scope

Implementation PR `#673` reviewed `headRefOid`
`685057c58841947819bab6c8d65d5d05a19722de` and reported local review
`READY_WITH_FOLLOWUPS` with `P0=0, P1=0`. All `8/8` implementation CI checks
passed. P-018 passed with zero unresolved findings and zero unresolved threads.
The PR used merge-commit-only policy, merged normally without admin fallback,
and produced merge commit
`740dba66604973a8adfd1f19e2489c389a64e207` at
`2026-11-23T22:06:34Z`.

Local `main` was synchronized exactly to that merge commit before closure.
Archive commit `9984de55274e` was the closure-artifact generation HEAD.

The explicit feature and task membership is `221-F` plus ten validly
pre-archived tasks. Safe close archived only `221-F` and the `222-S` control
record; `archived_status` is `done` for the feature and `shipped` for the
shipment. `returned_ids` is empty. No non-member artifact was mutated.

| Governing report | Result |
| --- | --- |
| `.backlogit/reconcile/222-S-pre-20261123T221145Z.md` | `PROCEED` |
| `.backlogit/reconcile/222-S-safe-close-20261123T221233Z.md` | `CLOSED` |
| `.backlogit/reconcile/222-S-post-20261123T221303Z.md` | `PROCEED` |

Canonical evidence is [222-S close evidence](evidence/222-S-221-F-close-evidence.json).
P-007 passed, doctor found zero shipped-event findings for `222-S`, and the
governed close result contains no failures.

## Source artifact provenance

`source_stash_id: DF533572` and
`source_deliberation_id: 079-DL` are provenance records for Feature
`221-F`. Closure preserved them unchanged. They were not rewritten,
reclassified, or otherwise mutated.

Existing follow-ups `443131E9`, `08CD4496`, and `603B9D88` remain active for
later triage. Closure did not add, remove, or resolve these follow-ups.

## Verification and releasability

The tested implementation baseline `f309820a268f9db2cf4229c421ce258bab24645a`
completed
`go test -timeout=30m -p 2 -parallel 4 ./...` with exit code `0` for all
`47/47` listed packages. The final reviewed PR HEAD
`685057c58841947819bab6c8d65d5d05a19722de` differed only in two checkpoint
records and one continuity-memory file, so the production and test source
surface is identical across the tested and reviewed heads. The pre-merge full
build exited `0`. The
post-merge build, CLI version probe, and docs lint all exited `0`.

Runtime requirements in `.gsd/workspace-profile.yaml` define
`validator_manifest.surfaces: []`, validator `required: false`, minimum
verdict `PASS`, and releasability `required: true`. The runtime verification
verdict is `PASS`; releasability is therefore satisfied as `READY`.

See [222-S post-merge runtime verification](222-S-post-merge-runtime-verification.md).

## Pre-deploy audit and rollout

| Check | State |
| --- | --- |
| Merge and release path | Normal merge commit; admin fallback not used |
| Implementation CI | `8/8` checks passed |
| Local review | `READY_WITH_FOLLOWUPS`, `P0=0, P1=0` |
| P-018 | `PASS`; zero unresolved findings and threads |
| Build and test evidence | Full build exit `0`; `47/47` packages passed |
| Governed archive close | `P-007 PASS`; no non-member mutation |
| Follow-up continuity | `443131E9`, `08CD4496`, and `603B9D88` preserved |

Rollout evidence is limited to the implementation merge and the supplied CLI
probes. The runtime profile declares no validator surfaces and no additional
deployment, service, migration, feature-flag, or dependent-system validator.

## Monitoring plan, owner, and rollback

* **Owner**: Ship for closure verification; Stage for follow-up triage;
  operator for rollback authorization.
* **Validation window**: Immediate post-merge build, CLI version, and docs lint
  probes, with later observation limited to the same healthy signals.
* **Healthy signals**: CLI build succeeds, version JSON is emitted with exit
  code `0`, and docs lint remains valid with zero findings.
* **Failure signals and rollback trigger**: Build failure, CLI version command
  failure, or docs lint reporting invalid output or any finding.
* **Rollback procedure**: Open an operator-approved revert of
  `740dba66604973a8adfd1f19e2489c389a64e207` without modifying the governed
  `.backlogit/archive/` history, then rerun the build, version, and docs lint
  probes.

## Knowledge refresh and P-020 compaction

`compound-refresh` ran in `scope: recent`, `mode: propose` form. Six relevant
recent compound entries were classified, and no `docs/compound/` file was
edited. See [222-S compound refresh](222-S-compound-refresh.md).

`compact-context` ran with `target: all` before this status was finalized.
The required `threshold_days: 14`, `max_files: 40`, and
`max_size_kb: 500` thresholds were applied to the corpus. No qualifying
222-S artifact was selected for archival: the current 222-S memory records are
recent, the most recent checkpoint state is preserved, and unrelated plans,
checkpoints, and active work remain untouched. The invocation completed
successfully with zero files archived, zero space recovered, zero plans
consolidated, and zero closure records compacted. P-020
`compaction_status` is `done`.
See [222-S P-020 compaction report](222-S-compaction-report.md) for the
assessment and candidate counts.

## Dark-mode closure summary

* `DARK_MODE_ACTIVE` visibility was recorded locally because
  `agent-intercom` was unavailable.
* `ENGRAM_DEGRADED` applied: no Engram CLI or MCP surface was invoked.
* Local events recorded the reviewed implementation HEAD, green gate state,
  normal merge authorization, exact main synchronization, governed close,
  runtime `PASS`, and closure `READY`.
* `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, and `.kbdb/`
  remain untouched.
* Final closure status is `READY`; P-020 compaction is `done`.

## Structured releasability

| Evidence | Profile requirement | State |
| --- | --- | --- |
| Validator evidence | `required: false`, minimum `PASS` | Satisfied |
| Releasability | `required: true` | `READY` |
| Monitoring and rollback | Required for closure traceability | Recorded |
| Archive integrity | Governed close requirement | `P-007 PASS` |
| P-020 compaction | Mandatory post-merge duty | `done` |
