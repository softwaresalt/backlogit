---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S execution complete through branch review; PR stage pending (DARK_MODE, P-017)
description: Ship finished all four P-002.6 waves of the amended 197-S (18 tasks), ran the per-task exempt, review, and P-021 handling, passed the wave gates, and completed the branch-wide adversarial review and review-skill passes with three bounded fix cycles. No PR has been opened and no merge has occurred. Resume at the PR stage.
timestamp: "2026-10-10T10:26:00Z"
---

# Ship 197-S: execution complete, branch reviewed, PR stage pending

## Session status

**PARTIAL (resume point: PR stage).** Scope `[197-S]` only. Branch
`feat/cx-ship-closure-protocol-and-gate-correctness`, HEAD `df4a2df9` (pushed up to `79b500df`;
later commits are local until the push at the end of this invocation). `origin/main` is
`d9d0237b`, untouched. No PR exists. Nothing was claimed outside 197-S.

## What is done

| Area | Result |
|---|---|
| Schedule (Step 3, real, read-only checker) | M = 18, excluded `197-F`, W1 = 001-008 and 017; W2 = 009, 011-015, 018; W3 = 010; W4 = 019. Acyclic. 7 red contracts and 10 exempt contracts valid. |
| Wave 1 | Completed in the earlier invocation (`de75c673` ... `c41f504e`). |
| Wave 2 | 009, 011, 012, 013, 014, 015, 018 all `done`. Step 4.6 gate: compile, vet, and golangci-lint exit 0; newly closed red deliverables GREEN; `TestUCXS2_` still RED as required; FULL_SUITE_DEFERRED. |
| Wave 3 | 010 `done`. Step 4.6 gate: compile, vet, lint exit 0; `TestUCXS2_` GREEN; unfiltered full suite exit 0 (40 ok packages, zero failing tests). The open-red set is empty. |
| Wave 4 | 019 `done`. Harness-architect RED drift harness `TestUCXS17_RefreshedDriftRecords` (`e76f6ece`), refresh of eight manifest entries (`fca29082`, LF-normalized SHA-256, independently re-verified by Ship), review comment fix (`caa8691c`). |
| Checkpoints | Read-only enumeration at intake; no checkpoint restored (operator direction). A new Ship checkpoint was written at wave 2 convergence (`checkpoint-20261010-084617.json`). |

## Branch-wide review (operator requirement, before any PR)

* Adversarial round 1 (multi-model, report-only): BLOCKED, P0 0, P1 6, P2 5, P3 3. Fixed in cycle 1
  (`becfca4e`): eligibility labelling, manifest containment (and a direct-child rule I had wrongly
  restored in 197.011-T, now reverted to the contract's containment rule), the closure allowlist name
  and pre-existing-change rules, the CI-item citation, the registry-absence rule, the lock-doc truth, and
  the memory-note cutover advice (which would otherwise have deleted backlogit sidecars).
* Review skill (report-only), round 1: READY_WITH_FOLLOWUPS at `becfca4e` (P0 0, P1 0, P2 2, P3 1).
* Adversarial round 2: BLOCKED with new in-scope P1s. Fixed in cycle 2 (`14bc91c4`): untracked-file
  baseline, the pre-existing-change refusal moved before any write (a0b), the closure allowlist keyed on
  every explicit shipment member (including feature 197-F), the unconditional CI confirmation, the Step 0
  not-installed exception, and the cutover advice.
* Cycle 3 (`3a3dc1be`): "before advancing past item 6" corrected to item 7, and the AC3 manifest refresh
  (`df4a2df9`) re-run for the three manifest-tracked files changed since 197.019-T.
* Review-fix cycles are now exhausted (3 of 3). The remaining items are residual risks or captures, below.

## Out-of-scope findings (P-021 C2, capture-only; each with discovery before capture)

Entries from this invocation: `63233C1C`, `6FF6320C`, `9DA5FBD6`, `405C8574`, `9F2AA687`, `2C8615A5`,
`00A9D01C`, `4AAC11E4`, `F8090390`, `A8122B01`, `8F45E676`, `AFDC480A`, `2C01BD52`, `5E0CCF91`,
`2F152179`, `6AABA22B`, `96F5B958`, `1FDD2E00`, `18B2B8EF`.

## Residual risks (accepted and recorded; not blocking under the reviews' READY_WITH_FOLLOWUPS)

1. `.gitignore` does not ignore `.*.agent-lock` (capture `4AAC11E4`). `.gitignore` is not authorized
   for 197-S. Agents stage explicit paths only, and the concurrency doc now says so truthfully.
2. Plugin mirrors (for example `plugin/skills/file-lock/SKILL.md`) still document the old sidecar
   (capture `96F5B958`). They are a release-sync surface, not an authorized 197-S surface.
3. Lock-name cutover: legacy `.<file>.lock` files left by superseded scripts are invisible to the new
   scripts. Operator decision; no script or agent deletes a lock file during cutover.
4. `shipment ship` progress: stderr is written under the membership lock (advisory); the progress label
   counts features (capture `18B2B8EF`).
5. Routing: Orchestrator Step 2 item 2 checks compaction status only, not closure-PR merge (capture `1FDD2E00`).
6. `.autoharness/config.yaml` (commit `45dea2f8`) is an operator-directed intentional model-routing edit,
   not a 197-S change, and is documented as such by the Orchestrator memory. Not a P-021 item.
7. Runtime verification: required before merge (manual acquire/release round-trip in a disposable
   workspace: sidecar name, token refusal, `-Force` containment). Not yet run; it belongs to the PR
   stage and the closure artifact.
8. Descoped U15 (197.016-T) performance remainder: follow-up stash `76553D8D`. R10 is "profiled plus
   progress reporting"; the "fixed" part is NOT delivered by 197-S and the PR and closure must say so.
9. The adversarial and review passes were report-only and did not run `go test` in a quiet workspace.

## Full-suite gate evidence

* Wave 3 gate at `0fa2f112`: unfiltered `go test -timeout=30m ./...` exit 0 (40 packages OK).
* Branch-review run at `becfca4e`: two load-sensitive lock-timing tests failed while review subagents
  were running concurrently (`TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`,
  `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock`). Both PASS in isolation (exit 0 each). They
  are recorded as a load-sensitive flake, not a regression. A quiet unfiltered full run at the final
  HEAD is required at the PR stage before any merge (Step 5 item 1).

## Deviations recorded (P-010 / file rules)

1. Several review-fix and build subagents wrote gitignored scratch files under `logs/` via
   PowerShell redirection or `Set-Content`, and one deleted its own scratch message file. None is
   committed. Operator review of `logs/` is recommended.
2. One Ship commit-message file (197.011-T) was written with `[IO.File]::WriteAllText` under `logs/`.
3. The commit type `docs(docs)` was used for `*.agent.md` edits for consistency with 197.009-T. The
   repo guideline suggests `chore` for agent files; this is a style point and history was not rewritten.
4. Early in wave 2 a review gate ran a diagnostic beyond its wave scope; its failures were tolerated
   open red and its reads were read-only.

## Resume point (PR stage)

1. Quiet full suite at the final HEAD: `go test -timeout=30m ./...` (must be exit 0; no open red).
2. Local readiness record for the current HEAD (`## Local Review Readiness`, section 1.9.2): reviewed
   HEAD, `READY_WITH_FOLLOWUPS`, P0 0, P1 0, P2 2 and P3 1 (residual risks above), full local build
   (`go build ./cmd/backlogit`) evidence, and runtime-verification follow-up.
3. Re-run AC3 (U17) only if a manifest-tracked file changes again.
4. Feature PR (body via a BOM-less `--body-file` under `logs/`), Copilot review request, patient
   polling, comment handling (P-021 C1, fix, commit and push, reply, resolve), CI green, then
   `gh pr merge --merge` only with explicit operator approval (pre-authorized under the DARK_MODE record
   for the scope above; `admin_fallback_pre_authorized` is false).
5. Post-merge: main sync (`--ff-only`), `shipment-reconcile` pre, safe-close, post, `backlogit_ship_shipment`
   with the merge SHA, runtime verification and operational closure with `closure_status` and
   `compaction_status`, follow-up stash of residual risks, P-020 compaction, then the closure PR with the
   same review, Copilot, CI, and merge discipline.
