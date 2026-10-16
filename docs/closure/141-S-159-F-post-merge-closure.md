---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: done
conditions:
  - id: feature-pr-512-merged
    description: "PR #512 (141-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 512: state MERGED, merge commit f025708030069cd056e9558f22481034e767dd80; final PR head ca9c4f45."
  - id: governed-closure-archived
    description: "Shipment 141-S and its members are archived."
    satisfied: true
    evidence: "Archive commit fb6d1501 on post-merge/141-s-api-backed-evidence-validator."
description: "141-S (159-F API-backed evidence and documentation validator) post-merge closure for PR #512, merge f0257080. READY_WITH_CONDITIONS: opt-in rules are not enabled in CI."
doc_type: closure
docline:
  date: 2026-10-16T21:00:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 141-S
    - 159-F
feature_id: 159-F
schema_version: "1.0"
shipment_id: 141-S
source: docs/closure/141-S-159-F-post-merge-closure.md
title: "141-S post-merge closure"
---

# 141-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `141-S` |
| Feature | `159-F` |
| Implementation PR | [#512](https://github.com/softwaresalt/backlogit/pull/512) |
| Merge commit | `f025708030069cd056e9558f22481034e767dd80` (merge commit, P-009) |
| Reviewed HEADs | `a6525a60` -> `c2d886e9` -> `ca9c4f45` (final) |
| Closure branch | `post-merge/141-s-api-backed-evidence-validator` |
| Closure status | `READY_WITH_CONDITIONS` |
| compaction status | `done` |

## Summary

A read-only validator. `internal/evidence` gathers GitHub review and CI evidence through `gh`, plus backlogit evidence, and checks claims against that evidence. `internal/docline` gained opt-in structural and reference rules and a shared anchor resolver.

## Review record

* Copilot: 4 reviews over 3 fix cycles; 8 threads resolved; 4 review bodies dispositioned.
* Decisions and gates: the local readiness gate and the copilot-review gate passed on `ca9c4f45`; merge used the merge-commit strategy.
* Adversarial-review pack ran with a single reviewer, so there was no 3-model consensus.

## Runtime verification and releasability

Runtime-verification is bounded and not applicable to a live deployment. The only runtime surface is the external GitHub API through `gh`.

* The live GraphQL review query was verified read-only against PR 512.
* The CI query through `gh pr view` was verified.

Releasability: `READY_WITH_CONDITIONS`.

* Condition: the opt-in docline rules are not enabled in CI.
* Validation window: run the validator in report mode against recent PRs before enforcing anything.
* Rollback: the rules are not wired into CI, so disable the rule or leave it unwired. Alternatively revert merge `f0257080` through a PR that preserves `.backlogit/` state.
* Owner: Ship / operator.

## Residual risk

* Adversarial review was single-reviewer, with no multi-model consensus.
* Local `golangci-lint` is broken by a Go 1.25 toolchain mismatch; lint evidence comes from CI.
* Regenerating `docs/cli-reference` produced CRLF line-ending noise.

## Governed closure

* The shipment claim MCP call timed out and was recovered by re-reading state, not by a blind retry.
* The branch was renamed to satisfy the pipeline-topology gate.
* Provenance is untouched: source stash `E053034D` was not archived by Ship.

## Follow-ups

Deferred stash entries, all requiring Stage triage: `291EE81C`, `B1DD8FA4`, `B597D942`, `1C88D6F8`.

Next shipment `142-S` is pending operator direction.

## P-020 compaction

compact-context ran with `target: all`. `docs/memory` was over the thresholds (199 files, about 1.1 MB). The unreferenced 2026-10-01 and 2026-10-02 checkpoints (six files, the 2A355F83 bootstrap and the 195-S startup halt) were compacted into `docs/memory/compacted/2026-10-16-2a355f83-bootstrap-compacted.md` and moved, not deleted, to `docs/archive/memory/`. Referenced checkpoints stay, and no plans or closure records qualified. Outcome: `done`.

## Provenance

Compound refresh: `docs/closure/2026-10-16-141-s-compound-refresh.md`.
