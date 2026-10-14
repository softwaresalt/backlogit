---
chunk_strategy: h1-h2-h3
# closure_status and compaction_status MUST stay at the top level: the pipeline-topology
# gate reads them with fm.get(). conditions entries are gate input (satisfied: true with
# non-empty evidence). Open follow-ups are non-gating and live in the body, not here.
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
description: "198-S (CT: restore test-suite health and runtime diagnostics) post-merge closure for PR #494, merge c315219373e44269a8d7874a2a660aced7a10225. READY_WITH_CONDITIONS: every closure condition is satisfied. The governed archive completed through the P-007 named-limitation procedure after an MCP -32001 on safe-close and an MCP git-launch failure, using the Orchestrator-approved CLI route (decision G2; operator ratification requested, not yet received). Residual follow-ups are non-gating and live in the body."
doc_type: closure
docline:
  date: 2026-10-14T04:44:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 198-S
    - 198-F
feature_id: 198-F
schema_version: "1.0"
shipment_id: 198-S
source: docs/closure/198-S-198-F-post-merge-closure.md
title: "198-S post-merge closure"
conditions:
  - id: feature-pr-494-merged
    description: "PR #494 (198-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 494: state MERGED, mergedAt 2026-10-14T04:18:43Z, mergeCommit c315219373e44269a8d7874a2a660aced7a10225, headRefOid 6ef38c764cc3931a4552045f84929b0dc8f3a197. origin/main equals c315219373e44269a8d7874a2a660aced7a10225; the merge commit is an ancestor of origin/main (merge-base --is-ancestor)."
  - id: final-head-copilot-and-ci
    description: "Copilot review is complete on the merged HEAD with zero unresolved threads, the copilot-review gate is SATISFIED, and all required CI checks pass."
    satisfied: true
    evidence: "PR #494 at 6ef38c76: Copilot reviews at 04:06:55Z (on 3d00a504) and 04:14:59Z (on 6ef38c76). Three threads, all resolved via resolveReviewThread. Gate SATISFIED at the last-mile re-check (run 4, PASS). Checks on 6ef38c76 all SUCCESS: Docline frontmatter gate, test (6m20s), Windows handle/lock tests, CLI Reference Drift, pipeline-topology, Markdown lint, Detect code changes."
  - id: governed-closure-archived
    description: "Shipment 198-S, its covering feature 198-F, its five explicit task members, and the shipment control record are archived with merge provenance (archived_status shipped)."
    satisfied: true
    evidence: "Shipped event verified in .backlogit/logs/198-S.jsonl (shipment_status_changed to shipped at 2026-10-13T21:22:49-07:00; commit_tracked c315219373e44269a8d7874a2a660aced7a10225 at 21:22:59). The governed MCP safe-close timed out (McpError -32001) after those events. Its member archives (198-F and the five tasks) had landed, but its control-record archive did not run. The MCP archive call errored on the git launch (archive error recorded below). Orchestrator decision G2: one governed CLI archive with a process-scoped Team Explorer git on PATH, run as `backlogit archive 198-S --cwd C:/Source/GitHub/backlogit`. Output: Archived 198-S into .backlogit/archive/198-S.md (2026-10-14T04:40:30Z). Verified read-only: archive frontmatter archived_status shipped and commit c315219373e44269a8d7874a2a660aced7a10225; queue record absent; doctor shipped_unarchived_residue for 198-S cleared (0 findings); missing_shipped_event for 198-S 0; backlogit_get_shipment status archived; sync_index indexed 2019."
---

# 198-S (CT: restore test-suite health and runtime diagnostics) post-merge closure

Status: READY_WITH_CONDITIONS. Every closure condition is satisfied. The governed archive was completed through the P-007 named-limitation procedure after the MCP safe-close timeout (Orchestrator decision, 2026-10-14). Residual follow-ups are non-gating and listed below. Compaction is recorded in the section "Compaction (P-020)".

## Completed

* PR #494 merged as merge commit `c315219373e44269a8d7874a2a660aced7a10225` (2026-10-14T04:18:43Z) from the reviewed head `6ef38c764cc3931a4552045f84929b0dc8f3a197`, under the dark-mode merge authorization. Admin fallback not used.
* P-018 copilot-review gate: SATISFIED at the last-mile re-check. Copilot review rounds: one on `c001a935` (two threads, fixed in `3d00a504` and `5a41fd4e`); one on `3d00a504` (one thread, fixed in `6ef38c76`, plus a review-body disposition for two previously missed low-severity findings, captured as `6DB63934`); one on `6ef38c76` (zero open findings). All three threads resolved through `resolveReviewThread`.
* CI on `6ef38c76`: every check SUCCESS.
* Main synchronized: local `main` equals `origin/main` at `c315219373e44269a8d7874a2a660aced7a10225`. Sync-order deviation disclosed by telemetry: the ff-only pull ran on the feature branch before the checkout to main failed; the feature branch was fast-forwarded to the merge commit locally (not pushed). No content lost.
* Lifecycle topology gate: `pass` (BRANCH_POST_MERGE_CLOSURE_ELIGIBLE).
* Pre-mode reconcile report: `.backlogit/reconcile/198-S-pre-20261014T042130Z.md` (PROCEED).
* Governed safe-close invoked (`backlogit_ship_shipment`, sha `c315219373e44269a8d7874a2a660aced7a10225`). The MCP call timed out (McpError -32001). Its shipped event and commit association landed, and so did the member archives (198-F and the five tasks). Its control-record archive did not run; that is the partial release-scope state closed below.

## Archive completion (P-007 named-limitation procedure)

* The Orchestrator reviewed the doctor finding `shipped_unarchived_residue` for 198-S. The shipped event had landed, so the procedure requires completing archival for the shipment and its stranded release-scope items, then re-running the audit. The shipped event was confirmed in the item log before any archive action.
* The governed MCP archive call `backlogit_archive_item(198-S, commit_sha c315219373e44269a8d7874a2a660aced7a10225)` returned `internal: archive item: plan archive move: detect git worktree: git rev-parse --show-toplevel: exit status 1: error launching git: The system cannot find the path specified`. The error occurs before any move. It was not retried. Cause: the backlogit MCP server resolves git through PATH, and PATH resolves to the broken Git for Windows launcher (`C:\Program Files\Git\cmd\git.exe`, with `mingw64\bin` and `bin` missing). Repairing that install needs operator approval; it has not been repaired.
* Orchestrator decision G2 (approved by the Orchestrator under dark mode; operator ratification requested and not yet received): one governed CLI archive, with the Team Explorer git prepended to PATH for that process only, and the same governed operation. Served-root attestation passed (catalog root and storage root match; `pragma_database_list` main file is `.backlogit\backlogit.db`, a direct child of the storage root). Containment checks passed (no reparse point on the archive path or the queue record). Shipment lock acquired and released around the call.
* Result: `Archived 198-S → C:\Source\GitHub\backlogit\.backlogit\archive\198-S.md` (exit 0). The archive frontmatter records `archived_from: .backlogit/queue/198-S.md`, `archived_status: shipped`, `commit: c315219373e44269a8d7874a2a660aced7a10225`. The queue record is removed.
* Verification (read-only): `backlogit doctor --check-shipped-event-completeness` reports no finding for 198-S (the residue cleared); `backlogit_get_shipment 198-S` reports `archived`; `backlogit_sync_index` reports `indexed: 2019`.
* Post-mode checks: (1) no `mutation_partial` envelope was returned (the call timed out). The governed ship did land the shipped event, the commit association, and the member archives for 198-F and the five tasks; the control-record archive was the one step missing. This is the partial release-scope state described in the archive section, not the 143-F halted-archival branch; (2) the shipment is terminal, not nonterminal; (3) the shipment archive is present with `archived_status: shipped`; (4) explicit member archives for 198-F and 198.001-T to 198.005-T are present with provenance; (5) the queue and archive change set names only `M` plus `198-S` (the archive moves and provenance lines; the hooks queue and stash appends are not queue or archive artifacts); (6) no tracked archive deletion. Post-mode report: `.backlogit/reconcile/198-S-post-20261014T044506Z.md`.
* Telemetry: a P-005 note records that closure was completed through the P-007 named-limitation procedure after the MCP -32001 on safe-close, and that the MCP archive op hit the git-launch failure. The CLI route was used under the Orchestrator's approval; it is not a second close path. No event was synthesized.
* The governed ship recorded the merge SHA on the released task artifacts (`commit` now `c315219...`). Their delivering commits (for example `9762e75f` for 198.001-T) remain in the task bodies and in the PR history.

## Runtime verification

Not applicable. The shipped change touches test files (five Go test files in `internal/core` and `internal/release`) and documentation only. It changes no production code and no runtime surface, so no runtime validator evidence applies. No production `.go` file changed after `3c76a544`.

## Operational closure

* Invariants to preserve: the TestUT1_, TestUT2_, and TestUT3_ selectors and the CT-scope test names stay green on main; no production code changes; shipment archive integrity under P-007.
* Validator evidence: not applicable (no runtime surface).
* Pre-deploy audits: not applicable (no deployment, migration, flag, or configuration change).
* Deployment or rollout path: merge only. No deploy.
* Post-deploy checks: CI on the merge commit and on the next two pushes to main. Doctor audit for 198-S after closure.
* Risky action record: the governed archive by CLI with process-scoped PATH (Orchestrator decision G2; ActionRisk medium; approval source Orchestrator; ActionResult success, the archive recorded above). No system PATH or Git installation was changed.
* Healthy signals: main CI green; 198-S remains archived with `archived_status shipped`; no new `missing_shipped_event` for 198-S.
* Failure signals: a test failure on main in the CT-touched packages that is not the already-tracked TestU172 flake (63909DAE); a `missing_shipped_event` for 198-S; a new `shipped_unarchived_residue`.
* Monitoring plan: watch the merge commit's CI and the next two main CI runs for TestU172 recurrence (63909DAE) and for failures in `internal/core` or `internal/release`; rerun the doctor audit once after the closure PR merges.
* Rollback trigger: a CT-attributable failure on main that is not the tracked flake.
* Rollback procedure: revert the merge commit through a new PR; do not force-push. Re-open the 198-S state through the Orchestrator.
* Validation window: until the next two CI runs on main complete after the closure PR merges. Owner: Ship (closure owner), with the operator for any rollback decision.
* Source artifact cleanup (provenance only): `custom_fields.source_stash_id` and `custom_fields.source_deliberation_id` are not set on 198-F (`none`). The feature description names source stash entries `8F41D60D`, `D8EF5443`, and `92AB6879` and the deliberation `docs/decisions/2026-10-08-ct-test-suite-health-deliberation.md`. None was archived or mutated by this closure; they are recorded here for Stage's later triage.
* Compaction status (P-020): see "Compaction (P-020)".

## Residual risks (non-gating)

* T5 full-suite evidence is GREEN-AFTER-RERUN. The first run failed `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog` (outside the five CT-touched files). The rerun passed. This recurs the intermittent failure tracked by stash `63909DAE`; its cause is undiagnosed, and the test does not surface the error from `ArchiveItem`. No new stash entry is created for it (D-6). The gated scalar `CT_ISOLATION_FIXES_HELD: DB48A817, FF1F3AC7, 885263D2` is a gate-required key whose value is the three IDs only. It is not a claim that the isolation fixes held, and this closure does not state that they held.
* R3 detection-power findings: 0 (`R3_FINDINGS 0`). The direct R6 re-run on HEAD `0244f60b` reproduced the mutation evidence: T1 fails 10 of 10 on the mutant and passes on the control; T3 fails 3 of 3 and passes on the control.
* Stash `5A6F54BB`: T1's relaxed assertion does not detect a never-released counter lock.
* Stash `8640432F`: a fourth `slog.SetDefault` site (`silenceBenchLogs`, benchmark-only), out of scope.
* The literal AC2 selector for 198.002-T matches no test; the eight-test exact-name selector is the disposition (D-4, A1.8 E-1).
* Stash `F8936891`: Ship's wave and final gates pin `go test -timeout=30m ./...`; internal/core took about 27 minutes in the loaded 2026-10-08 run. The unfiltered run for this shipment took 1009.8 s.
* Environment: the Git for Windows install is damaged (`mingw64\bin` and `bin` missing). The MCP server cannot run git. The CLI route used a process-scoped PATH. Repair needs operator approval.
* Observation for the backlog owner: the governed ship replaced the task artifacts' `commit` field with the merge SHA (the `sha` parameter's documented behavior), so the delivering-commit field no longer points to the delivering commit. The delivering commits are in the task bodies and PR history.

## Follow-ups captured (stash)

* Spike follow-ups FU-1 to FU-9 from `docs/decisions/2026-10-08-ct-core-runtime-budget-spike.md`: `5C185050` (FU-1), `96FC7CD9` (FU-2), `39B5EB54` (FU-3), `79F743DA` (FU-4), `30F6A010` (FU-5), `7CED062B` (FU-6), `0E8EC7AE` (FU-7), `AD7039AD` (FU-8), `9A07B24C` (FU-9, medium; gates any fix).
* Carried advisories: `C7FAE4D5` (durability-fixture advisories GO-01, GO-02, GO-07, GO-05), `965C5C05` (TestGateBaseNoShadow_NoWarn liveness), `1D919906` (spike advisories: forced classes, causal claims, the quiet label, arithmetic, wall time).
* Round-3 captures: `4685ECD4`, `DC6ADACB`, `A03C7EB0`, `2BF9DC26`, `A69EEF4C`, `0F3C563E`, `9111589C`.
* Copilot-review capture (from the review of `3d00a504`, not round 3): `6DB63934`.
* Carried, still open: `8640432F`, `FBD0BB95`, `7802D798`, `901C79CC`, `5A6F54BB`, `253E2505`, `F8936891`, `DAEE9D03`.
* T5 flake: cited as `63909DAE`; no new capture (D-6).

## Compaction (P-020)

Status: `degraded` (non-blocking). The memory compaction pass (`102c2670`) was reverted by `5df583e9`: it moved records that prior P-020 audits protect (the 138-S report and the Azure design record, which live closure records and the 2026-09-20 compaction report reference), and its summary Sources pointed at removed paths. Every original is restored, so memory is unchanged in this PR.

* Memory (`docs/memory`): the pass (58 candidates older than 14 days; cutoff 2026-09-30T04:46Z UTC; 273.4 KB; 18 date summaries) is reverted, and all originals are restored. A new pass is required, and it must first audit protected records and live references (for example the 138-S report, the Azure design record, and any path that a closure or compaction report names) and update the Sources of every summary to the path where each original actually lives.
* Closure records (`docs/closure`): 159 files older than 14 days (1.17 MB) are NOT compacted in this pass. Moving them could break consumers that read closure artifacts, including the closure-evidence gate, so a separate pass should check those consumers first.
* Plans (`docs/exec-plans`): not compacted by Ship. Plan files and decided-plan artifacts are Stage-owned (P-010).

## Compound refresh

* New learning: `docs/compound/2026-10-14-ship-governed-archive-git-launch-fallback.md`. A governed archive that errors on the git launch is recovered by the approved CLI with a process-scoped git on PATH, not by a retry or a hand move.
* Existing entries evaluated: `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` still holds (it covers observing state after an MCP timeout); it is not superseded, so no refresh.
