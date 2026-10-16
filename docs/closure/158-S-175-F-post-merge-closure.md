---
chunk_strategy: h1-h2-h3
# closure_status and compaction_status MUST stay at the top level: the pipeline-topology
# gate reads them with fm.get(). conditions entries are gate input (satisfied: true with
# non-empty evidence). Open follow-ups are non-gating and live in the body.
closure_status: READY_WITH_CONDITIONS
compaction_status: done
conditions:
  - id: feature-pr-509-merged
    description: "PR #509 (158-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 509: state MERGED, mergedAt 2026-10-16T15:31:49Z, mergeCommit ba2c333a8be44851cadd97ac38cdc5d605d7d715, PR head 94709ba9. Local main synchronized to origin/main at ba2c333a (POST_MERGE_SYNC_OK)."
  - id: governed-closure-archived
    description: "Shipment 158-S and its ten task members are archived with merge provenance."
    satisfied: true
    evidence: "backlogit_ship_shipment 158-S --sha ba2c333a: MCP timeout, completed server-side; .backlogit/archive/158-S.md has status archived, archived_status shipped, commit ba2c333a. Members 175.002-T..175.010-T and 175.040-T archived with commit ba2c333a. No archive file deleted in the working tree (P-007)."
  - id: final-head-copilot-and-ci
    description: "Copilot review is complete on the final HEAD with zero unresolved threads, the copilot-review gate is SATISFIED, and all CI checks pass."
    satisfied: true
    evidence: "PR #509 head 94709ba9: autoharness gate copilot-review SATISFIED; the single Copilot thread was replied to and resolved. CI: test, Windows handle/lock tests, CLI Reference Drift, Docline frontmatter gate, Markdown lint (P-008) and pipeline-topology (ambient) all pass."
description: "158-S (baseline convergence RS-W01 remediation wave 1 + U40 line-ending guard) post-merge closure for PR #509, merge ba2c333a. READY_WITH_CONDITIONS: conditions are gate input; open follow-ups are non-gating residuals in the body."
doc_type: closure
docline:
  date: 2026-10-16T16:00:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 158-S
    - 175-F
feature_id: 175-F
schema_version: "1.0"
shipment_id: 158-S
source: docs/closure/158-S-175-F-post-merge-closure.md
title: "158-S post-merge closure"
---

# 158-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `158-S` |
| Parent feature of members | `175-F` (not a shipment member; task-only manifest) |
| Members | `175.002-T` .. `175.010-T`, `175.040-T` (10 tasks, one wave) |
| Implementation PR | [#509](https://github.com/softwaresalt/backlogit/pull/509) |
| Merge commit | `ba2c333a8be44851cadd97ac38cdc5d605d7d715` (merge commit, P-009; merged 2026-10-16T15:31:49Z) |
| Final HEAD | `94709ba9` (local review `READY_WITH_FOLLOWUPS` on code HEAD `b882c008`; Copilot `copilot-review` gate `SATISFIED`) |
| Closure branch | `post-merge/158-175-remediation-wave-1` |
| Closure status | `READY_WITH_CONDITIONS` |
| Context compaction | `done` (P-020: compact-context invoked; see the P-020 section) |
| Mode | P-017 dark mode, scope sequence `202-S -> 176-S -> 157-S -> 158-S`; 158-S is last in scope |

## Summary

Nine file-owned errcheck/staticcheck remediations (`internal/atomicfile`, `internal/canonical`,
`internal/cli`) and the new line-ending guard `scripts/check-line-endings.ps1` with its harness.
Harnesses were scaffolded red first (commit `b2485113`), then each task was driven green.
CI's `test / Lint` job found one extra errcheck finding (a test caller of the changed
`writeBulkStatusResult` signature), fixed in `94709ba9`.

Review record: adversarial review (3 reviewers: gpt-6-sol Correctness, claude-sonnet-5 Go,
gpt-6-luna Maintainability; no Tier-3 or alternate-provider slot) returned
`READY_WITH_FOLLOWUPS` with P0=0, P1=0, P2=2, P3=11 and no behavior regressions. One Copilot
thread (effective `.gitattributes` attribute validation in the guard) was out of scope under
P-021 C1 and captured as deferred stash `56490CA9`, with a reply citing the ID and the thread resolved.

## Runtime verification and releasability

No runtime surface changes (lint remediation plus a CI guard script), so runtime-verification
is not applicable. Releasability: `READY_WITH_CONDITIONS`.

* Monitoring: none required beyond CI, which passes on the merged tree.
* Rollback: revert merge `ba2c333a` via a rollback PR that preserves `.backlogit/` state (the merge also moved 158-S and its members into the archive), or revert the individual `fix(lint)` commits. The guard script and its harness revert together.
* Owner: Ship / operator.
* Validation window: the next baseline-convergence shipment's CI lint run.

## Governed closure

* `backlogit_ship_shipment 158-S --sha ba2c333a` timed out at MCP (-32001) but completed server-side; state was verified independently, with no blind retry. Shipment is `archived` with `archived_status: shipped`.
* Pre-close: all ten members were pre-archived (auto-archived on `done`); shipment `active`; recommendation `PROCEED`. Allowed changed-ID set `{158-S, 10 members}` equals the observed set; no non-member mutation. P-007: no deleted archive files.
* Post-merge main sync: `main == origin/main @ ba2c333a`.
* Local `golangci-lint` v1.64.8 (go1.24-built) reports nothing even on unfixed code; lint evidence is CI. `task_lint_cmd` (`scripts/verify-task-lint.ps1`) fails closed without a FixtureRoot and was not run green.
* Full suite: `internal/core` flaked once under contention (`TestU172_LockOrder...`) and a `./tests` run was killed by timeout after a long machine stall; both and the whole `internal/core` package re-ran green in isolation.

## Closure-evidence gate posture

`autoharness gate closure-evidence` requires `close_path`, which cannot be produced post-hoc by `cascade-close --classify-only` on an archived shipment; same posture as 157-S and 176-S. No key or record is fabricated. This record asserts no successor acceptance; 158-S is last in the dark-mode scope and no successor was claimed.

## P-020 compaction

compact-context was invoked with `target: all`. `docs/memory` was under the thresholds (28 files, about 184 KB). Of the 198-S group, one checkpoint had no inbound reference by name
(`2026-10-12-ship-198s-wave1-converged.md`); it was compacted into
`docs/memory/compacted/2026-10-16-198s-wave1-converged-compacted.md` and moved, not deleted, to
`docs/archive/memory/2026-10-12/`. The other 198-S checkpoints and the 154-S, 155-S, 138-S and 140-S items are referenced or already assessed and stay. Outcome: `done`.

## Follow-ups

| ID | Summary |
|---|---|
| 56490CA9 | New: DEFERRED SCOPE EXPANSION. Harden the line-ending guard to validate effective Git eol/text attributes (nested overrides, `-text`) with regression cases, plus P3 items from adversarial review (CR-only lines, `pwsh`-absent skip policy, harness blank-assignment coverage, wrapping consistency). Requires deliberation. |
| F88FE051 | Existing: CI ambient pipeline-topology job advisory (passed on this PR). |
| golangci-lint env | Local `golangci-lint` is go1.24-built with go1.25+ dependencies; rebuild or pin. |
| task_lint_cmd | `scripts/verify-task-lint.ps1` needs a FixtureRoot; the remaining 175-F tasks should define how it is run. |

## Provenance

`175-F` and the baseline-convergence plan are provenance only and were not mutated.
Compound refresh: `docs/closure/2026-10-16-158-s-compound-refresh.md`.
