---
chunk_strategy: h1-h2-h3
# closure_status and compaction_status MUST stay at the top level: the pipeline-topology
# gate reads them with fm.get(). conditions entries are gate input (satisfied: true with
# non-empty evidence). Open follow-ups are non-gating and live in the body, not here.
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
conditions:
  - id: feature-pr-491-merged
    description: "PR #491 (197-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 491: state MERGED, mergedAt 2026-10-11T01:16:04Z, mergeCommit f8d35936deae71a468ccba6d769a3dfa2bdb3e20, headRefOid b0f611eaef2ac3bf2c9baa7d866700eb759e21d5. Local main synchronized to origin/main at f8d35936 (POST_MERGE_SYNC_OK recorded at the halt)."
  - id: governed-closure-archived
    description: "Shipment 197-S, its covering feature 197-F, and its 18 explicit task members are archived with merge provenance; post-mode checks 1-6 pass (the report's overall decision is HALT for the missing safe-close report, and the Orchestrator ratified that exception); the archive commit is pushed."
    satisfied: true
    evidence: "backlogit_get_shipment 197-S: status archived, commit f8d35936. Post-mode report .backlogit/reconcile/197-S-post-20261011T013645Z.md: checks 1-6 PASS. Archive commit 4d6b2189 pushed to origin/post-merge/197-s-ship-closure-protocol. The safe-close envelope and report were not returned (MCP timeout); the Orchestrator ratified the exception after independent change-set verification (see Governed closure)."
  - id: final-head-copilot-and-ci
    description: "Copilot review is complete on the final HEAD with zero unresolved threads, the copilot-review gate is SATISFIED, and all required CI checks pass."
    satisfied: true
    evidence: "PR #491 at b0f611ea: one Copilot review pass, 0 inline comments, 0 threads; copilot-review gate SATISFIED at pass 1 and at the last-mile recheck; required CI 7 of 7 green."
description: "197-S (CX: Ship closure protocol and gate correctness) post-merge closure for PR #491, merge f8d35936. READY_WITH_CONDITIONS: the satisfied conditions are gate input; open follow-ups and accepted partial coverage (R10, R7 non-default layout, P1-4 residual) are non-gating residuals recorded in the body."
doc_type: closure
docline:
  date: 2026-10-11T01:43:41Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 197-S
    - 197-F
feature_id: 197-F
schema_version: "1.0"
shipment_id: 197-S
source: docs/closure/197-S-197-F-post-merge-closure.md
title: "197-S post-merge closure"
---

# 197-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `197-S` |
| Covering feature | `197-F`: CX: Ship closure protocol and gate correctness |
| Implementation PR | [#491](https://github.com/softwaresalt/backlogit/pull/491) |
| Merge commit | `f8d35936deae71a468ccba6d769a3dfa2bdb3e20` (merge commit, P-009; merged 2026-10-11T01:16:04Z; no `--admin`) |
| Final reviewed HEAD | `b0f611ea` (local review readiness `READY_WITH_FOLLOWUPS`, P0 0, P1 0; Copilot 0 threads; `copilot-review` gate `SATISFIED`; required CI 7 of 7) |
| Archive commit | `4d6b2189` (`chore(harness): archive 197-S backlog artifacts`), pushed to `post-merge/197-s-ship-closure-protocol` |
| Closure branch | `post-merge/197-s-ship-closure-protocol` |
| Closure PR | The PR opened from this branch. Its number is in the PR body and the Ship return message. |
| Closure status | `READY_WITH_CONDITIONS` |
| Context compaction | `degraded` (non-blocking, P-020); see the P-020 section |
| Routing | `ROUTING_DEGRADED`; see Routing and reviewers |
| Mode | P-017 dark mode, scope `[197-S]`; `merge_approval_pre_authorized` true; `admin_fallback_pre_authorized` false, not used |

## Summary

PR #491 delivered the 197-F corrective release unit: the Ship Step 6 closure
protocol (allowlisted exact-path staging, safe-close ordering, and the capability
predicate); gate-scope text; Orchestrator gate-aware eligibility and configured-root
lookup; the harness lock-sidecar rename to `.<file>.agent-lock`; docline closure
gate-key preservation; ship-time validation profiling and progress reporting; the
140-S closure gate registration record; and harness-manifest drift records.

Scope changes that bound this record:

* Task 197.016-T (U15) was descoped by operator ruling and archived. The frozen task
  set M is 18 tasks plus the covering feature 197-F, which is 19 explicit members.
* **R10 is partial.** The release delivers the profiling (U6) and progress reporting
  (U16a, U16b). The ship-time validation performance fix is NOT delivered. It is
  follow-up stash `76553D8D`. No document may describe R10 as fully delivered.
* **R7 is partial** for non-default queue layouts. The Go metadata catalog still
  hard-codes `storage/queue` and `storage/archive`. The default layout (this
  workspace) reports correct values. The gap can only cause a false fail-closed, never
  an unsafe pass. Capture `00A9D01C`.

## Shipment reconciliation and governed closure

| Phase | Report or evidence | Result |
|---|---|---|
| Pre-close (`expected_status: done`) | `.backlogit/reconcile/197-S-pre-20261011T011730Z.md` | `PROCEED` |
| Governed ship (`backlogit_ship_shipment`, safe-close) | Item log `.backlogit/logs/197-S.jsonl`; MCP call timed out (`-32001`) | Server completed: `shipped` 18:21:45 PDT, `commit_tracked f8d35936` 18:21:59 PDT, `archived` (archive_path `.backlogit/archive/197-S.md`) 18:34:48 PDT |
| Safe-close result envelope and report | Not returned to Ship | `SAFE_CLOSE_REPORT_UNAVAILABLE`; see the exception below |
| Post-close (`mode: post`) | `.backlogit/reconcile/197-S-post-20261011T013645Z.md` | Overall decision HALT for the missing safe-close report (exception ratified). Checks 1-6 PASS: no `mutation_partial`; no live control record; `archive/197-S.md` has `archived_status: shipped`; all explicit members archived with `commit: f8d35936`; no non-member change; zero tracked archive deletions (P-007 guard not triggered) |

### Exception: `SAFE_CLOSE_REPORT_UNAVAILABLE` (Orchestrator-ratified)

The closure allowlist is normally derived from the safe-close report. That report
was never returned, so the commit could not be built the normal way. The Orchestrator
ratified a documented exception, subject to operator veto, after independently
verifying that the working tree matched the deterministic expected closure set
exactly. Ship re-verified the same set before staging:

* The 197-S control record move to `archive/` (staged by the server as `R100`).
* The 197-F queue removal and the `archive/197-F.md` addition.
* 18 explicit task archive files: `197.001-T` through `197.015-T`, `197.017-T`
  through `197.019-T`. `197.016-T` is not in the set; it was archived by the descope.
* `.backlogit/hooks_queue.jsonl`, staged by explicit path under the accepted P1-4
  residual risk.

Verification: the expected and actual sets each held 23 paths (the rename counts as
two), with no path only expected and no path only present. Every ID matched
`^\d{3,}(\.\d{3,})*-[A-Z]{1,2}$`. Each path was staged with `git add -- <path>`.
`git add` for `queue/197-S.md` returned "pathspec did not match" because the server
had already staged that removal. The cached diff (`--no-renames`) showed it as `D`,
which is the expected state. The commit was made in a separate command. The shipment
control record and feature remain at their server-written archive state. Nothing was
reconstructed beyond the verified set.

### P-005 event: `backlogit_ship_shipment` MCP timeout and lock hold

* Operation: `backlogit_ship_shipment` for `197-S` at `f8d35936` (safe-close).
* Symptom: MCP request timed out (`-32001`). The envelope was not returned.
* During the operation, the global shipment lifecycle lock
  (`.backlogit/.locks/shipment-lifecycle-global.lock`) was held. The CLI reported
  `gate in progress for item` until the server finished.
* The server completed the governed operation afterwards. The item log shows the
  archive event at 18:34:48 PDT.
* Ship did not retry the ship operation and did not delete or edit any lock file.
  Doctor evidence. The pre-archive run (`logs/doctor-197s-shipped-events.txt`, 18:27:36 PDT) reported 42 issues: 23 `orphaned_artifact`, 18 `missing_shipped_event` for 18 historical shipments from 060-S to 134-S (not contiguous), and one `shipped_unarchived_residue` for 197-S itself, which the archive event cleared. A post-archive run reported 23 issues, all `orphaned_artifact` for the pre-existing `016.001-R` and `106.012-T` to `106.033-T`, with none for 197-S. No data loss was observed, and post-mode checks 1-6 passed.
* Telemetry recorded through `backlogit_log_telemetry` (P-005).

## Release gate evidence

### Unfiltered full-suite history (`go test -timeout=30m ./...`)

| HEAD | Run context | Exit | Result |
|---|---|---|---|
| `0fa2f112` | Wave 3 convergence gate | 0 | 40 packages ok |
| `becfca4e` | Branch-review run, concurrent with review subagents | 1 | Two load-sensitive lock-timing tests failed (`TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`, `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock`). Both PASS in isolation. |
| `df4a2df9` | Quiet run | 1 | One package (`internal/core`) failed with one panic line; the test name was not captured. `go test -count=1 ./internal/core` alone exited 0 (907.6s). |
| `de1f8adf` | Orchestrator diagnostic run | 0 | 40 packages ok |
| `b0f611ea` | Final reviewed HEAD, unfiltered (quietness not separately recorded) | 0 | 40 packages ok, 1 without tests (`logs/full-suite-197s-b0f611ea.txt`) |

The unnamed panic in `df4a2df9` was not reproduced in the two later unfiltered runs (`de1f8adf`, `b0f611ea`). Its
origin is not proven. It is captured as follow-up `63909DAE`. The unfiltered `b0f611ea` run (exit 0, 40 packages ok) is body evidence only. Its quietness was not recorded, so it is not a closure condition.

### Other gates

* `go build ./cmd/backlogit`: exit 0.
* `go test ./tests/integration/`: exit 0 at `b0f611ea`.
* Manual lock round-trip, PowerShell (`scripts/acquire_lock.ps1`, `scripts/release_lock.ps1`):
  pass. Checked the sidecar name `.<file>.agent-lock`, wrong-token refusal, correct
  release, `-Force` release, and out-of-root refusal.
* Bash round-trip (`scripts/acquire_lock.sh`, `scripts/release_lock.sh`): **not run on
  this host**. CI ubuntu rows of `TestUCXS4_` passed in PR #491. The manual Bash
  round-trip is follow-up `F6322B45`.
* Model routing config commit `45dea2f8` (`.autoharness/config.yaml`) is an
  operator-directed routing edit. It is not a 197-S scope item.

### Pipeline-topology probe, successor gate (U7 AC1, read-only)

Run after 197-S left `active`, with `autoharness gate pipeline-topology --mode agent
--shipment 141-S --phase pre_claim --json`. Only the verdict fields are recorded:

* `exit_code`: 1; `blocked`: true; `invalid`: false
* `token`: `PREDECESSOR_CLOSURE_UNRECOGNIZED`
* Blocking predecessor: `154-S`, not 197-S. The 154-S closure candidates
  (`docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md` and
  siblings) do not match `{shipment_id}-{feature_id}-post-merge-closure.md`.
* The 141-S predecessor list is `140-S`, `154-S`, `197-S`. The gate stops at the first
  blocking predecessor, `154-S`, so 197-S is not evaluated in this probe. This closure is
  named to the expected pattern (`197-S-197-F-post-merge-closure.md`). Its recognition
  is unverified until 154-S is resolved. Follow-up `BB669732` routes the 154-S naming
  question to Stage.
* Re-run on the clean committed tree (after the closure docs commit): same verdict.

141-S was not claimed. This probe is read-only.

## Reviews, readiness, and reviewer models (PR #491)

| Stage | Reviewer and model | Result |
|---|---|---|
| Pre-PR round 1, adversarial (report-only) | Anchor `gpt-6.1-sol` (high); Tier 1 `claude-haiku-5.5` (author family, not counted); Tier 3 `claude-sonnet-5.5` | BLOCKED: P0 0, P1 5, P2 11, P3 7. The P1s were verified by Ship, fixed, or recorded as Stage or operator items. |
| Pre-PR round 1, review skill (report-only) | Model not pinned; not verified as independent (not counted) | READY_WITH_FOLLOWUPS |
| Confirming pass at `af46609f` | Adversarial anchor `gpt-6.1-sol` (high; runtime identity not exposed to Ship); review skill `claude-sonnet-5.5` | 0 P0, 0 P1; P2 4, P3 6. The single extra review-fix cycle authorized by Orchestrator decision 3 was not used, because the confirming review found no P0 or P1. |
| Final local readiness record at `b0f611ea` | `## Local Review Readiness` in the PR body | `READY_WITH_FOLLOWUPS`; P0 0, P1 0; full local build evidence; follow-ups listed below |
| Copilot P-018 (`enforcement: required`) | One pass requested on `b0f611ea` | 0 inline comments, 0 threads; `SATISFIED` at pass 1 and at the last-mile recheck |
| CI | Required checks | 7 of 7 green at `b0f611ea` |

The orchestrator decisions that bounded the PR stage are in
`docs/memory/2026-10-10/orchestrator-197s-pr-stage-decisions-memory.md`.

## Runtime validation

Not applicable. The workspace profile declares `runtime_validation.validator_manifest.surfaces: []`
and `validation_expectations.required: false`, so no runtime validator was required or
run. The release changed harness scripts, agent and skill text, Go CLI and core code,
integration tests, and lock scripts. The PowerShell lock round-trip above is manual
verification, not a runtime validator result.

## Validator evidence

| Field | Evidence |
|---|---|
| Affected runtime surfaces | None declared by the profile |
| Applicability | Not applicable |
| Verdict | Not applicable. This is not a passing validator result. |
| Surface adapters | None required |
| Probe outcomes | None |
| Manual checkpoint evidence | PowerShell lock round-trip (PR stage). Bash round-trip not run (`F6322B45`). |
| Blocked prerequisites | None for this closure |
| Runtime follow-up recommendations | None from runtime verification |

## Invariants to preserve

* The shipment manifest is explicit and flat. Only explicit members are archived; no
  implicit descendants or source artifacts are archived.
* Closure staging uses exact paths only. Archive directories are never staged as a
  whole, and the ID grammar is checked before any path is built.
* A `backlogit_ship_shipment` timeout means the result is unknown. Observe the state
  first. Never retry the governed ship or restore archives blindly.
* Lock sidecars are named `.<file>.agent-lock`. Agents never delete a lock file.
* Merges to `main` are merge commits only (P-009), gated by P-014 readiness and the
  P-018 Copilot gate for the current HEAD.

## Pre-deploy audits and release path

* No data migration, feature flag, or rollout prerequisite.
* Merge-commit only, through PR #491 (`f8d35936`).

## Post-deploy checks

Not applicable to a runtime. The post-merge checks are the reconcile reports, the
`backlogit doctor` orphan set, and the topology probe above.

## Risky action record

| ProposedAction | ActionRisk | Approval | ActionResult | Rollback |
|---|---|---|---|---|
| Governed safe-close of 197-S at `f8d35936` (`backlogit_ship_shipment`) | High | P-017 dark-mode activation, scope `[197-S]` | Applied server-side; MCP timed out; envelope not returned; post-mode checks 1-6 PASS, overall HALT (exception ratified) | P-007 applies. Restore archives only on a missing-archive signal. None observed. |
| Archive commit `4d6b2189` (exact-path staging of 23 paths) | Medium | Orchestrator ratification of the SAFE_CLOSE exception, subject to operator veto | Applied; pushed to the closure branch | `git revert` on a branch plus a PR |
| Merge PR #491 (merge commit) | High | `merge_approval_pre_authorized: true`; current-HEAD readiness, Copilot gate, and CI all passed | Applied; `f8d35936` | `git revert -m 1 f8d35936` on a branch plus a PR |
| Lock files under `.backlogit/.locks/` | Low | None needed | Not touched by Ship | Not applicable |

## Operational signals

| Area | Evidence or action |
|---|---|
| Healthy signal | Shipment `archived`; all explicit members archived with merge provenance; post-mode checks 1-6 PASS; topology probe verdict recorded |
| Failure signal | Missing archive, non-member delta, `mutation_partial`, P-007 deletion, or a `doctor` finding beyond the pre-existing orphans |
| Monitoring plan | Not applicable to a runtime. Reconcile and doctor checks in the next closure. |
| Rollback trigger | A contract regression observed in a later Orchestrator-to-Ship handoff, reconcile pass, or successor gate (141-S, 152-S, 198-S, 199-S) that traces to 197-F |
| Rollback procedure | `git revert -m 1 f8d35936deae71a468ccba6d769a3dfa2bdb3e20` on a branch, then a PR through the same P-014 and P-018 gates |
| Validation window | Until the closure PR merges and the first successor pre-claim gate evaluates cleanly |
| Owner | Ship for closure PR readiness. Stage and the Orchestrator for the follow-ups below. |

## Source artifact provenance

* `197-F` `custom_fields.source_stash_id`: `none`. `custom_fields.source_deliberation_id`: `none`.
* Prose provenance in the 197-F description (source stash list and deliberation path) is
  unchanged. No source stash entry or deliberation was mutated by this closure.

## Residual risks

| Item | Status | Note |
|---|---|---|
| R10 partial | Accepted partial (operator ruling 2026-10-09T22:47-07:00, "Descope U15 as recommended"; 197-F amendment; plan Erratum E2) | Profiled and progress reporting delivered. The performance fix is NOT delivered: follow-up `76553D8D`. |
| R7 partial, non-default queue layouts | Accepted partial (Orchestrator decision 1) | Default layout correct. Capture `00A9D01C`. |
| P1-4 safe-close allowlist completeness | Accepted residual (Orchestrator decision 2) | The safe-close report does not list exact tracked side-effect paths such as `.backlogit/hooks_queue.jsonl`. This closure staged it by explicit path. Capture `2C8615A5`. |
| Bash lock round-trip | Not run on this host | CI ubuntu rows of `TestUCXS4_` passed. Follow-up `F6322B45`. |
| Load-sensitive full suite; unnamed panic | Open, not proven as regression | Unfiltered runs at `de1f8adf` and `b0f611ea` exited 0. Follow-up `63909DAE`. |
| Legacy lock sidecars and cutover | Operator decision | Legacy `.<file>.lock` files are not removed by any script or agent. Capture `B2B4BE2A`. |
| Model-routing config `45dea2f8` | Not 197-S scope | Operator-directed routing edit. |

## Routing and reviewers

`ROUTING_DEGRADED`, non-blocking:

* Pre-PR round 1: the alternate `google/gemini-3.8-flash` reviewer was not honored. The
  Tier 1 slot fell back to `claude-haiku-5.5`, which shares the author's model family and
  was not counted.
* Confirming pass: the anchor was dispatched as `gpt-6.1-sol` (high). The runtime-reported
  identity was not exposed to Ship.
* Closure PR reviewer: dispatched with model `claude-sonnet-5.5` (non-haiku), per Orchestrator PR-stage decision 6 in `docs/memory/2026-10-10/orchestrator-197s-pr-stage-decisions-memory.md`. The local reviewer reported it could not confirm its runtime model; the dispatch pin is recorded here.
  The model is recorded in the closure PR readiness block.

## Follow-ups (non-gating)

These are open follow-ups and accepted limitations. They are not gate conditions and
do not block successors.

| ID | Kind | Status | Owner | Note |
|---|---|---|---|---|
| `63909DAE` | Stash, high | Open | Stage | Load-sensitive suite failures and the unnamed `internal/core` panic. The stash text calls `de1f8adf` and `b0f611ea` quiet runs; this artifact records them only as unfiltered runs. |
| `F6322B45` | Stash, medium | Open | Stage | Bash lock round-trip on a Bash host |
| `BB669732` | Stash, high | Open | Stage or operator | `PREDECESSOR_CLOSURE_UNRECOGNIZED` for 154-S blocks 141-S pre-claim |
| `3FF72EB8` | Stash, medium | Open | Stage or harness owner | Closure-evidence gate requires `close_path` (`cascade` or `safe_close`); the closure template does not carry it (140-S fails the same way) |
| `BFB44A08` | Stash, high (stale duplicate) | Open, stale | Stage | Written by the timed-out first stash call. Its text ("queue/197-S.md is NOT archived") is now false. Stage should remove or archive it. Ship does not remove stash entries. |
| `76553D8D` | Stash | Open | Stage | R10 ship-time validation performance fix (descoped U15) |
| `00A9D01C` | Capture | Open | Stage | R7 catalog hard-coded `storage/queue` and `storage/archive` for non-default layouts |
| `2C8615A5` | Capture (reused) | Open | Stage | P1-4 exact side-effect paths in the safe-close report |
| `3386D498`, `FECEA4B0`, `3F058389`, `0365F693`, `5EE1CABB` | Stash | Open | Stage | Confirming-review P2 and P3 follow-ups (`3F058389` is the review-cycle cap note) |
| `B2B4BE2A`, `0D1B5D5C`, `8E484B5D`, `40F841E6`, `DBD92867` | Captures | Open | Stage | Legacy lock sidecars; unchecked stderr write; human-thread resolution wording; stale section pointers; shipment-ID grammar |
| `63233C1C`, `405C8574`, `8F45E676`, `2F152179` (reused) | Captures | Open | Stage | P-007 policy text; vacuous assertion; 140-S gate keys; `WAVE_SNAPSHOT` taxonomy |
| `6FF6320C`, `9DA5FBD6`, `9F2AA687`, `4AAC11E4`, `F8090390`, `A8122B01`, `AFDC480A`, `2C01BD52`, `5E0CCF91`, `6AABA22B`, `96F5B958`, `1FDD2E00`, `18B2B8EF` | Captures | Open | Stage | Branch-review captures (see `docs/memory/2026-10-10/ship-197s-branch-review-complete-memory.md`) |
| `B53056C0` | Operator reconciliation stash, high | Discharged by this closure's governed reconciliation | Stage | Stash left for Stage triage. Ship does not edit stash entries. |

## Releasability evidence

**Status: `READY_WITH_CONDITIONS`.** The three conditions in the frontmatter are satisfied
with evidence. The release is complete for P-001 once this closure PR merges. The
non-gating follow-ups and accepted partial coverage above are residual risks for Stage
and the operator, not blockers.

## P-020 context compaction

`compact-context` ran with `target: all` at closure, before the closure PR. Result:
`compaction_status: degraded`, non-blocking.

* Additive summary: `docs/memory/compacted/2026-10-11-197-S-compacted.md`.
* Report: `docs/closure/2026-10-11-197-s-compaction-report.md`.
* Verbose originals were not moved. Checkpoints, stash entries, and the Stage-owned
  plan reference them by path. Archive moves are deferred to Stage. This is the same
  reason and status as the 196-S closure.
* Decided-plan consolidation was not performed. It is a plan artifact, Stage-owned
  under P-010.
* Nine superseded 197-S `ship` checkpoints were resolved when closure PR 492 opened (2026-10-11T02:07Z). The first pointer (`checkpoint-20261011-020732.json`) went stale at the next Copilot pass and was resolved. The current pointer is `checkpoint-20261011-021224.json`. It records no head SHA, because the PR head is authoritative. It is resolved at completion, after the merge and the main sync.
