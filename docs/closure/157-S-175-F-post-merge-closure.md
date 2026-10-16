---
chunk_strategy: h1-h2-h3
# closure_status and compaction_status MUST stay at the top level: the pipeline-topology
# gate reads them with fm.get(). conditions entries are gate input (satisfied: true with
# non-empty evidence). Open follow-ups are non-gating and live in the body.
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
conditions:
  - id: feature-pr-507-merged
    description: "PR #507 (157-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 507: state MERGED, mergedAt 2026-10-16T07:49:39Z, mergeCommit 8ccd75e084a1e29c7b0754e9bea016195fb0632c, final reviewed HEAD 7f985033 (PR head 43df3590 is a state-only commit). Local main synchronized to origin/main at 8ccd75e0 (POST_MERGE_SYNC_OK); 0 tracked *.go or *.json files with CRLF after the sync."
  - id: governed-closure-archived
    description: "Shipment 157-S and task 175.001-T are archived with merge provenance."
    satisfied: true
    evidence: "backlogit_ship_shipment 157-S: shipped, then archived with commit 8ccd75e084a1e29c7b0754e9bea016195fb0632c. .backlogit/archive/157-S.md and .backlogit/archive/175.001-T.md present; no archive file is deleted in the working tree (P-007)."
  - id: final-head-copilot-and-ci
    description: "Copilot review is complete on the final HEAD with zero unresolved threads, the copilot-review gate is SATISFIED, and CI passes except the known ambient advisory."
    satisfied: true
    evidence: "PR #507: autoharness gate copilot-review SATISFIED with 0 review threads; the review-body note asking the operator to acknowledge the 26 refreshed testdata paths is dispositioned in the body below. CI: test, CLI Reference Drift, Docline frontmatter gate, Markdown lint (P-008) and Windows handle/lock tests pass; the ambient pipeline-topology job fails as expected (F88FE051, advisory, non-gating)."
description: "157-S (baseline line-ending renormalization) post-merge closure for PR #507, merge 8ccd75e0. READY_WITH_CONDITIONS: conditions are gate input; open follow-ups are non-gating residuals recorded in the body."
doc_type: closure
docline:
  date: 2026-10-16T08:30:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 157-S
    - 175-F
feature_id: 175-F
schema_version: "1.0"
shipment_id: 157-S
source: docs/closure/157-S-175-F-post-merge-closure.md
title: "157-S post-merge closure"
---

# 157-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `157-S` |
| Parent feature of members | `175-F` (not a shipment member; task-only manifest) |
| Members | `175.001-T` |
| Implementation PR | [#507](https://github.com/softwaresalt/backlogit/pull/507) |
| Merge commit | `8ccd75e084a1e29c7b0754e9bea016195fb0632c` (merge commit, P-009; merged 2026-10-16T07:49:39Z) |
| Final reviewed HEAD | `7f985033` (local review `READY` after remediation; Copilot `copilot-review` gate `SATISFIED`, 0 threads) |
| Closure branch | `post-merge/157-175-baseline-line-ending` |
| Closure status | `READY_WITH_CONDITIONS` |
| Context compaction | `degraded` (P-020: compact-context invoked; one group compacted, assessment of the remaining candidates incomplete); see the P-020 section |
| Mode | P-017 dark mode, scope sequence `202-S -> 176-S -> 157-S -> 158-S`; this invocation covered `157-S` only |

## Summary

Renormalizes the tracked Go and JSON baseline to LF so the replacement baseline sequence
sees a stable line-ending surface. The pinned baseline digest `c986c983...6b574` was
verified, and the live set (564 files: 528 Go and 36 JSON) equals the baseline set.
`.gitattributes` gained nine rule lines. Working-tree files were refreshed by a scoped
delete plus `git restore`, because a plain `git restore` is a no-op for files whose
index content is already LF (see Follow-ups). The 26 `testdata` paths that the refresh
rewrote were refreshed under digest-pinned operator approval.

Review record:

* The initial adversarial review was `BLOCKED`: two P1 harness findings (fixed) and one
  disputed P0 about the 26 `testdata` paths. The P0 was downgraded to an acknowledged
  note because the refresh was operator-approved against a pinned digest and altered
  only line endings.
* Local review is `READY` on `7f985033` after remediation. Copilot's review body asks the
  operator to acknowledge the 26 `testdata` paths; no thread was opened.

## Runtime verification and releasability

No runtime surface changes (line endings and `.gitattributes` only), so
runtime-verification is not applicable and no validator evidence is produced.
Releasability: `READY_WITH_CONDITIONS`.

* Monitoring: none required beyond CI, which passes on the merged tree.
* Rollback: Source-only rollback: a rollback PR that reverts the nine `.gitattributes` rule lines AND the LF harness `tests/lineending_baseline_175_test.go` together (the harness rejects CRLF working trees, so reverting only the attributes would leave the suite failing on CRLF-configured checkouts), preserving `.backlogit/` state. Do not revert merge `8ccd75e0` wholesale: it also moved `157-S` and `175.001-T` into the archive.
* Owner: Ship / operator.
* Validation window: the next successor `pre_claim` probe and the first replacement-baseline
  shipment that runs on the renormalized tree.

## Governed closure

* `backlogit_ship_shipment 157-S` moved the shipment to `shipped`; it was then archived
  with commit `8ccd75e0`. `.backlogit/archive/157-S.md` and `.backlogit/archive/175.001-T.md`
  are present and no archive file is deleted (P-007).
* Post-merge main sync: `main == origin/main @ 8ccd75e0`; the tree was not re-dirtied
  (0 tracked `*.go` or `*.json` files with CRLF after the sync).
* `golangci-lint` is unusable locally: the installed binary is built with go1.24 while the
  dependencies need go1.25. Lint evidence comes from CI. `internal/core` tests need
  `-timeout=20m` under machine contention.

## Closure-evidence gate posture

`autoharness gate closure-evidence --path docs/closure/157-S-175-F-post-merge-closure.md --shipment 157-S` fails with `closure artifact must declare close_path: cascade | safe_close (got None)`, the same posture as 176-S. `close_path` and its evidence record would come from `autoharness shipment cascade-close --classify-only`, which cannot run post-hoc on an already archived shipment; the installed harness closes through flat-manifest `backlogit_ship_shipment`. Neither key nor an evidence record is fabricated. Successor acceptance is NOT asserted by this record: the 158-S `pre_claim` verdict comes from the post-merge probe `autoharness gate pipeline-topology --mode agent --shipment 158-S --phase pre_claim --json` run on clean `main` after this closure PR merges.

## P-020 compaction

compact-context was invoked with `target: all` (P-020). `docs/memory` was over the
thresholds (201 files, about 1.1 MB). The backlog active-work cross-check was run for the
`154-S` group: `154-S` and `173-F` are archived, and six of its seventeen checkpoint
memories had no inbound reference by name. Those six were compacted into
`docs/memory/compacted/2026-10-16-154s-ship-wave-checkpoints-compacted.md` and moved, not
deleted, to `docs/archive/memory/2026-09-30/`. The other eleven are cited by checkpoints,
plans, decisions or closure records and stay. The `184-S` and `185-S` groups belong to
queued work and were excluded. The remaining older candidates (the `2a355f83` stash
group, the `2026-09-30` and `2026-10-01` Stage and Orchestrator notes) and the 19 already
compacted summaries were not assessed in this run. Outcome: `degraded` (compaction made
progress, the over-threshold state remains and stays tracked in stash E2B351F3).

## Follow-ups

| ID | Summary |
|---|---|
| Operator acknowledgement | Operator to acknowledge the interpretation of the 26 refreshed `testdata` paths (line endings only, digest-pinned approval); raised in the Copilot review body. |
| golangci-lint env | Local `golangci-lint` is built with go1.24 but the dependencies need go1.25; rebuild or pin the toolchain so lint runs locally. |
| F88FE051 | Existing: CI ambient pipeline-topology job fails advisory (does not honor the dag-root waiver). |
| E2B351F3 | Existing: docs/memory compaction threshold exceeded; remaining candidates are listed in the P-020 section. |
| Future renormalizations | Plain `git restore` is a no-op under `eol=lf` when the index is already LF; use a scoped delete plus `git restore` (see `docs/compound/2026-10-16-gitattributes-eol-renormalization-worktree-refresh.md`). |

## Provenance

The 157-S plan, the baseline-convergence decomposition decision and the parent feature
`175-F` are provenance only and were not mutated.
