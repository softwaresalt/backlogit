---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: done
conditions:
  - id: feature-pr-529-merged
    description: "PR #529 (152-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "Merge commit 3d566ff2047772313d778710d4dbc7ad11656716 (P-009 merge commit); backlog archived in commit 05a11744."
  - id: non-required-topology-check
    description: "The non-required pipeline-topology (ambient) check failed with PREDECESSOR_NOT_SHIPPED (predecessor 151-S queued); this is a known gate predecessor-inference defect, not fixed here."
    satisfied: true
    evidence: "Captured in archived stash 95379DBA and task 183.004-T; the check is not a required status check."
description: "152-S (171-F checkpoint-create schema reject) post-merge closure for PR #529, merge 3d566ff2047772313d778710d4dbc7ad11656716. READY_WITH_CONDITIONS: CLI/MCP schema behavior change covered by tests, with rollback and compatibility notes."
doc_type: closure
docline:
  date: 2026-10-18T05:30:00Z
  status: draft
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

Checkpoint creation (`backlogit_create_checkpoint` and `backlogit checkpoint create`) now rejects unknown fields at the create boundary for `schema_version: 1` dumps instead of writing them verbatim; extra recovery keys belong inside `context`. Tasks 171.001-T through 171.008-T were completed and archived with the feature. Design and compatibility rationale: `docs/design-docs/checkpoint-administrative-disposition.md`.

## Gates

* Local multi-persona adversarial review: `READY_WITH_FOLLOWUPS`.
* Copilot P-018 gate: PASS.
* Required CI green: `test`, Markdown lint, Docline gate, CLI Reference Drift, Windows handle/lock tests.
* Non-required `pipeline-topology (ambient)` failed with `PREDECESSOR_NOT_SHIPPED` (predecessor `151-S` queued). This is the known CI-pinned gate predecessor-inference defect already captured in archived stash `95379DBA` / `183.004-T`; it was not fixed here.

## Runtime verification

Not applicable as a deployment check: no deployed runtime surface changed. The change is a CLI/MCP schema behavior change covered by unit and integration tests, which passed in CI.

## Releasability: READY_WITH_CONDITIONS

* Owner: Ship / operator.
* Monitoring signal: `checkpoint create` rejections reported by agents (unknown-field errors) and any report of lost recovery state after a rejected create.
* Validation window: the next several Stage/Ship sessions that create checkpoints; a rejected create writes no file, so the remedy is to resubmit with extra keys nested under `context`.
* Rollback trigger: agents unable to create checkpoints they need. Rollback is a revert of merge `3d566ff2047772313d778710d4dbc7ad11656716` through a PR that preserves `.backlogit/` state.
* Compatibility condition: agents holding cached MCP tool schemas will see the new default-reject behavior; see the compatibility note in `docs/design-docs/checkpoint-administrative-disposition.md`.

## Deferred follow-ups

* `2C13E721`: present-empty / zero-value defaulting.
* `3B41DE38`: V1-path null guard.

Stage triage is recommended for both.

## Provenance

Source stashes `6FDC4A49` and `97AB4978` are recorded as provenance only and were not mutated.

## Operational observations

* The governed ship through the MCP client timed out after about 60 seconds but completed server-side in about 9 minutes (per-member gate and telemetry latency). State was confirmed by re-reading, not retrying.
* A CLI re-run was blocked by the shipment-lifecycle lock until the server-side ship finished, which is the intended lock behavior.

## P-020 compaction

compact-context ran with `target: all`. Thresholds were exceeded (over 40 files and 500 KB in `docs/memory/`). The seven 198-S notes were compacted into `docs/memory/compacted/2026-10-18-198s-ship-compacted.md` and moved, not deleted, to `docs/archive/memory/`. Outcome: `done`. Candidate selection remained threshold-gated; other memory notes were left in place.

## Compound refresh

`docs/closure/2026-10-18-152-s-compound-refresh.md`.
