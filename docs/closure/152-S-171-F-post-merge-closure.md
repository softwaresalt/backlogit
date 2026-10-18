---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: done
conditions:
  - id: feature-pr-529-merged
    description: "PR #529 (152-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "Merge commit 3d566ff2047772313d778710d4dbc7ad11656716 (P-009 merge commit); backlog archived in commit 05a11744."
  - id: topology-check-disclosure
    marker: DISCLOSURE (not an open gating condition)
    owner: Stage
    description: "The non-required pipeline-topology (ambient) check failed with PREDECESSOR_NOT_SHIPPED (predecessor 151-S queued). This is a new instance of the same predecessor-inference defect captured in archived stash 95379DBA (which cited 153-S/152-S); it is not fixed here and does not gate this closure."
    satisfied: true
    evidence: "Task 183.004-T is still queued; the check is not a required status check."
  - id: stage-triage-deferred-follow-ups
    marker: OPEN CONDITION
    owner: Stage
    description: "Stage triages deferred follow-ups 2C13E721, 3B41DE38, and 85AE5A2D."
    satisfied: false
    evidence: "Listed under Deferred follow-ups."
  - id: validation-window-observation
    marker: OPEN CONDITION
    owner: operator
    description: "Operator watches the validation window for checkpoint_schema_* rejections from existing producers."
    satisfied: false
    evidence: "See Releasability."
description: "152-S (171-F checkpoint-create schema reject) post-merge closure for PR #529, merge 3d566ff2047772313d778710d4dbc7ad11656716. READY_WITH_CONDITIONS: CLI/MCP schema behavior change covered by tests, with rollback and compatibility notes."
doc_type: closure
docline:
  date: 2026-10-18T05:30:00Z
  status: final
  tags:
    - operational-closure
    - post-merge
    - 152-S
    - 171-F
feature_id: 171-F
schema_version: "1.0"
shipment_id: 152-S
source: docs/closure/152-S-171-F-post-merge-closure.md
title: "152-S post-merge closure"
---

# 152-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `152-S` |
| Feature | `171-F` |
| Implementation PR | [#529](https://github.com/softwaresalt/backlogit/pull/529) |
| Merge commit | `3d566ff2047772313d778710d4dbc7ad11656716` (merge commit, P-009) |
| Closure branch | `post-merge/152-checkpoint-create-schema-reject` |
| Backlog archive commit | `05a11744` |
| Closure status | `READY_WITH_CONDITIONS` |
| compaction status | `done` |

## Summary

Checkpoint creation (`backlogit_create_checkpoint` and `backlogit checkpoint create`) is now reject-by-default for schema-less dumps and for missing, `0`, or unsupported `schema_version`. Errors are `checkpoint_schema_rejected` (`retry_with_opt_in: true`; remediation `allow_legacy_import` / `--allow-legacy-import`) and `checkpoint_schema_unsupported`. Legacy import is an explicit opt-in that upgrades in-window legacy shapes to canonical V1 and never writes them verbatim. Present-null `status`, `created_at`, and `updated_at` are rejected on the legacy-import dry-run path only (171.007-T / 171.008-T); an explicit `schema_version: 1` dump still has null values defaulted, and that hardening is deferred as `3B41DE38`. Unknown-field rejection inside V1 dumps was delivered earlier by 137-S and is not part of this change. Tasks 171.001-T through 171.008-T were completed and archived with the feature. Design and compatibility rationale: `docs/design-docs/checkpoint-administrative-disposition.md`.

## Gates

* Local multi-persona adversarial review: `READY_WITH_FOLLOWUPS`.
* Copilot P-018 gate: PASS.
* Required CI green: `test`, Markdown lint, Docline gate, CLI Reference Drift, Windows handle/lock tests.
* Non-required `pipeline-topology (ambient)` failed with `PREDECESSOR_NOT_SHIPPED` (predecessor `151-S` queued). This is a new instance of the same predecessor-inference defect captured in archived stash `95379DBA` (which cited 153-S/152-S); task `183.004-T` is still queued. Disclosure only; not fixed here.

## Runtime verification

Not applicable as a deployment check: no deployed runtime surface changed. The change is a CLI/MCP schema behavior change covered by unit and integration tests, which passed in CI.

## Releasability: READY_WITH_CONDITIONS

* Owner: Ship / operator.
* Monitoring signal: `checkpoint_schema_rejected` and `checkpoint_schema_unsupported` rejections from `checkpoint create` / `backlogit_create_checkpoint`, and any report of lost recovery state after a rejected create.
* Validation window: the next several Stage/Ship sessions that create checkpoints; a rejected create writes no file. The primary remedy is to resubmit the dump as `schema_version: 1`; use `allow_legacy_import` / `--allow-legacy-import` only for in-window legacy shapes, which are upgraded to canonical V1.
* Rollback trigger: a spike in `checkpoint_schema_*` rejections from existing producers that cannot be fixed by resubmitting as V1 or opting in to legacy import. Rollback is a revert of merge `3d566ff2047772313d778710d4dbc7ad11656716` through a PR that preserves `.backlogit/` state.
* Compatibility condition: producers emitting schema-less or legacy-shaped dumps (including agents holding cached MCP tool schemas) will now be rejected unless they resubmit as V1 or opt in to legacy import; see the compatibility note in `docs/design-docs/checkpoint-administrative-disposition.md`.

## Deferred follow-ups

* `2C13E721`: present-empty / zero-value defaulting.
* `3B41DE38`: V1-path null guard.
* `85AE5A2D`: advisory test rows, two load-sensitive flaky tests, and `ship_shipment` taking about 9 minutes against the roughly 60 second MCP timeout (see Operational observations).

Stage triage is recommended for all three.

## Provenance

Source stashes `6FDC4A49` and `97AB4978` are recorded as provenance only and were not mutated.

## Operational observations

* Stash `85AE5A2D` tracks the ship_shipment latency versus MCP timeout gap and the flaky tests.
* The governed ship through the MCP client timed out after about 60 seconds but completed server-side in about 9 minutes (per-member gate and telemetry latency). State was confirmed by re-reading, not retrying.
* A CLI re-run was blocked by the shipment-lifecycle lock until the server-side ship finished, which is the intended lock behavior.

## P-020 compaction

compact-context ran with `target: all`. Thresholds were exceeded (over 40 files and 500 KB in `docs/memory/`). Eight 198-S notes were compacted into `docs/memory/compacted/2026-10-18-198s-ship-compacted.md` and moved, not deleted, to `docs/archive/memory/` (all eight originals preserved; `2026-10-12-ship-198s-wave1-converged.md` was also covered by the earlier 2026-10-16 compaction). Outcome: `done`. Candidate selection remained threshold-gated; other memory notes were left in place.

## Compound refresh

`docs/closure/2026-10-18-152-s-compound-refresh.md`.
