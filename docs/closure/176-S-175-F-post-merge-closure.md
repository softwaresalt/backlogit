---
chunk_strategy: h1-h2-h3
# closure_status and compaction_status MUST stay at the top level: the pipeline-topology
# gate reads them with fm.get(). conditions entries are gate input (satisfied: true with
# non-empty evidence). Open follow-ups are non-gating and live in the body.
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
conditions:
  - id: feature-pr-504-merged
    description: "PR #504 (176-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 504: state MERGED, mergedAt 2026-10-16T04:50:40Z, mergeCommit d48f15cb83f0ae26b04726995b53bb6dd5ef92c6, headRefOid bc6e181c42d306649241e8dc58d52ed79adafdb5. Local main synchronized to origin/main at d48f15cb (POST_MERGE_SYNC_OK)."
  - id: governed-closure-archived
    description: "Shipment 176-S and tasks 175.099-T, 175.100-T and 175.101-T are archived with merge provenance."
    satisfied: true
    evidence: "backlogit_get_shipment 176-S: status archived, commit d48f15cb83f0ae26b04726995b53bb6dd5ef92c6. .backlogit/archive/176-S.md present; 175.099-T, 175.100-T and 175.101-T archived. No archive file is deleted in the working tree (P-007)."
  - id: final-head-copilot-and-ci
    description: "Copilot review is complete on the final HEAD with zero unresolved threads, the copilot-review gate is SATISFIED, and CI passes."
    satisfied: true
    evidence: "PR #504 at bc6e181c: autoharness gate copilot-review (enforcement required) SATISFIED after 10 fix rounds; all review threads resolved and review-body findings dispositioned. CI run 38852042792: test, CLI Reference Drift, Docline frontmatter gate, Markdown lint (P-008), Windows handle/lock tests and pipeline-topology (ambient) all pass."
description: "176-S (RS-W(-1) shared lint-runner bootstrap) post-merge closure for PR #504, merge d48f15cb. READY_WITH_CONDITIONS: conditions are gate input; open follow-ups are non-gating residuals recorded in the body."
doc_type: closure
docline:
  date: 2026-10-16T05:00:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 176-S
    - 175-F
feature_id: 175-F
schema_version: "1.0"
shipment_id: 176-S
source: docs/closure/176-S-175-F-post-merge-closure.md
title: "176-S post-merge closure"
---

# 176-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `176-S` |
| Parent feature of members | `175-F` (not a shipment member; task-only manifest) |
| Members | `175.099-T`, `175.100-T`, `175.101-T` |
| Implementation PR | [#504](https://github.com/softwaresalt/backlogit/pull/504) |
| Merge commit | `d48f15cb83f0ae26b04726995b53bb6dd5ef92c6` (merge commit, P-009; merged 2026-10-16T04:50:40Z; no `--admin`) |
| Final reviewed HEAD | `bc6e181c` (local review `READY_WITH_FOLLOWUPS`, P0 0, P1 0; Copilot 10 rounds, 0 open threads; `copilot-review` gate `SATISFIED`) |
| Closure branch | `post-merge/176-baseline-bootstrap-runners` |
| Closure status | `READY_WITH_CONDITIONS` |
| Context compaction | `degraded` (P-020: compact-context invoked; the backlog active-work cross-check was not completed, so no file was compacted); see the P-020 section |
| Mode | P-017 dark mode, scope sequence `202-S -> 176-S -> 157-S -> 158-S`; this invocation covered `176-S` only |

## Summary

Delivers the three shared lint runners (`scripts/verify-task-lint.ps1`,
`scripts/verify-baseline-lint.ps1`, `scripts/verify-terminal-lint.ps1`) and their
uniquely named Go harnesses (`TestU175_099_`, `TestU175_100_`, `TestU175_101_`) that the
replacement baseline sequence `157-S`..`169-S` depends on. Each harness was written red
first; the runners are fixture-driven, fail closed (exit 2 on invalid input, exit 1 on a
violation), use ordinal case-sensitive collision-free fingerprints with multiplicity
(`line-stable-group-multiset/v1`), strict JSON decoding (duplicate and case-colliding keys,
exact numeric tokens, throwing UTF-8, string-preserving dates) and never mutate the
repository. Copilot review ran ten rounds; each round found a further validation gap and all
were fixed with regressions (see the compound entry
`docs/compound/2026-10-15-powershell-fixture-runner-strict-input-validation.md`).

Process notes:

* The shipment carried stale labels `readiness-blocked` and `do-not-claim-until-convergence`.
  The operator-ordered sequence and a passing `pipeline-topology` `pre_claim` gate authorized
  the claim. The labels remain on the archived record.
* The review-comment fix-cycle limit (3) was exceeded (10 rounds) under the operator's explicit
  standing instruction to iterate on Copilot feedback until the gate passes. Every finding was
  in scope (completing validation of the authorized deliverable).
* Adversarial review ran as a single-reviewer pass because the nested subagent depth limit
  prevented the 3-reviewer fan-out; a code-review agent re-reviewed the final HEAD (READY).
* The three task completions were recorded in one batch after implementation rather than per
  wave. `WORK_STARTED` comments were appended after the implementation commits.

## Runtime verification and releasability

No runtime surface is touched (PowerShell verification scripts and Go test harnesses only),
so runtime-verification is not applicable and no validator evidence is produced.
Releasability: `READY_WITH_CONDITIONS`.

* Monitoring: none required. The runners are exercised by the harnesses in CI.
* Rollback: Source-only rollback: a rollback PR limited to the three runners and their harnesses (`scripts/verify-{task,baseline,terminal}-lint.ps1`, `tests/runner_bootstrap_*_175_0{99,100,101}_test.go`), preserving `.backlogit/` state. Do not revert merge `d48f15cb` wholesale: it also moved the three tasks into the archive and changed `.backlogit/stash.jsonl`.
* Owner: Ship / operator.
* Validation window: the next successor `pre_claim` probe and the first replacement-baseline
  shipment that invokes the runners.

## Governed closure

* `backlogit_ship_shipment 176-S --sha d48f15cb` timed out at the MCP layer (-32001) but
  completed server-side. No blind retry was made. State was verified instead: shipment
  `archived` with commit `d48f15cb`, `.backlogit/archive/176-S.md` present, all three members
  archived, no deleted archive files (P-007). The safe-close result envelope was not returned;
  the change set was verified independently (same exception as 197-S and 202-S).
* A worktree-wide `go test ./...` is impractical here: the ignored `logs/` directory holds
  about 160k files, so package loading and test-cache validation run for tens of minutes. The
  suite was run with `-count=1 ./cmd/... ./internal/... ./tests/...`. `internal/core` failed
  once under machine contention and passed on isolated rerun (632s).

## Closure-evidence gate posture

`autoharness gate closure-evidence --path docs/closure/176-S-175-F-post-merge-closure.md --shipment 176-S` fails with `close_path` / `close_evidence` absent. The record would have to come from `autoharness shipment cascade-close --classify-only`, which cannot run post-hoc: 176-S is already archived, and the tool also refuses the workspace-resident `bin/backlogit.EXE`. The installed harness closes through flat-manifest `backlogit_ship_shipment` (the 197-S, 199-S and 202-S posture), so neither key nor an evidence record is fabricated. Successor acceptance is NOT asserted by this record: the 157-S `pre_claim` verdict is determined by the post-merge probe `autoharness gate pipeline-topology --mode agent --shipment 157-S --phase pre_claim --json` run on clean `main` after this closure PR merges and is reported in the Ship return message, not here. At authoring time the probe returned `WORKTREE_DIRTY` only because this closure was uncommitted and `.autoharness/config.yaml` carried an unrelated local edit; it is unverified until the post-merge run.

## P-020 compaction

compact-context was invoked with `target: all` (P-020). `docs/memory` remains over the
thresholds (198 files, about 1.07 MB); 41 stale files are not referenced by name, but the backlog active-work cross-check required by compact-context was not completed in this run, so none of them was assessed or removed. The run made no changes. Outcome: `degraded` (assessment incomplete; the over-threshold state remains and is tracked in stash E2B351F3, whose scope now also covers completing the active-work check for these 41 files).

## Follow-ups

| ID | Summary |
|---|---|
| E32ADEC0 | DEFERRED SCOPE EXPANSION: runners are fixture-only; a real-lint / per-surface GOOS path is needed before production `task_lint_cmd` / terminal gate invocations can pass |
| E2B351F3 | Existing: docs/memory compaction threshold exceeded |
| F88FE051 | Existing: CI ambient pipeline-topology job does not honor the dag-root waiver |

## Provenance

The 176-S plan, the baseline-convergence decomposition decision and the parent feature
`175-F` are provenance only and were not mutated.
