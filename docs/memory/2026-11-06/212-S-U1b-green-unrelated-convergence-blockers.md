---
title: Shipment 212-S U1b repaired and independent convergence blockers diagnosed
date: 2026-11-06
shipment_id: 212-S
feature_id: 211-F
phase: wave1-convergence-blocked-independent-contracts
status: blocked
---

## Same execution and completed repair

Ship continued the existing cursor after Stage released exclusive custody.
No shipment claim, successor claim, historical restore, branch change, or
worktree creation occurred.

Fourteen final Stage/preservation hashes matched. Stage's eleven unchanged
authored files were committed with provenance at `a3fed78f`.
The formal 085-DL / D10 amendment changes S from eight to nine members and
M from seven to eight tasks, adding U1b 211.008-T and its U2/U5 blocks edges.
Typed exact-ID resolution, live status-source validation, Kahn partition,
mixed active/queued start-epoch classification, served-root attestation,
and WAVE_SIM_OK 200/200 across 26 scenarios all passed.

U1 remained done and retained its one start record.
Harness-architect created a genuinely task-bound U1b regression, confirmed
compile and assertion RED, then applied harness-ready and committed
`b86dadec9e22294def4d922cf6e873bc9b097616`.
The queued U1b was moved active once, with no shipment-claimed epoch and
exactly one whole-log `WORK_STARTED: 212-S` record before dispatch.

Build-feature repaired only `internal/cli/mcp_daemon.go`, committing
`834972ab9108321c79c155022c5daaa9a9857d7f`.
The protected harness was not edited. The exact guarded test passed all
four real security refusals with zero skips.
Compile, vet, pinned Go 1.24/v1.64.8 lint, and tracked/new source formatting
passed. One unused compatibility-wrapper lint finding was addressed by a
documented local exemption; no unrelated finding was suppressed.

The required broad Go 1.24 race regression passed with native exit 0:

```text
go test -timeout=30m -count=1 -race -run '^(TestU1b_|TestU1_|TestU2_|TestU3_|TestU4_|TestOpenMCPServer_|TestRegistryParity_)' ./internal/cli
```

Elapsed package time was 60.137 seconds.
Go, Constitution, Security, and Learnings report-only reviewers reported no
concrete findings in inspected scope. Their Git-diff access limitations
were disclosed: these are task-local static results, not full branch or
merge approval. Required final persona and three-model adversarial review
has not occurred because no PR was submitted.

Official U1b completion gate passed at HEAD 834972a, report hash
`8ac746799d4f1672dc8d27729601a8a904e1db8002cd79148549cf16c5f6bbea`,
and the commit was linked. Task completion is not wave convergence.

## Actual unfiltered result

The prescribed cached Go 1.24 environment ran exactly:

```text
go test -timeout=30m ./...
```

Native overall exit: 1. CLI passed in 324.828 seconds, confirming that the
original nested-root daemon failures no longer fail this package.
Core failed in 1802.354 seconds after the unchanged 30-minute alarm.
MCP, policyengine, and integration also reported failures; remaining
reported packages passed. The complete 1.1 MB capture is:

`bin/tmp/1793937249660-copilot-tool-output-7128-71ef1bbe-f07c-4954-a037-f7d919acbe22.txt`.

No failing selector was declared expected RED, no test was skipped or
weakened, and the full-suite budget was not increased.

## Bounded Git-discovery investigation

Several supposed no-repository fixtures under contained TEMP discovered the
outer workspace repository:

* `TestPlanArtifactMove_StrategyDetection/empty_git_directory_fails_closed`
* `TestInGitWorktreeBounded`
* `TestHeadSHABounded`
* `TestU163_002_ExecGitRequiresARepositoryDirectory`

The observed HEAD was the real current 834972a. Core's final alarm occurred
while `ExecGitRunner.Verify` waited inside completion of
`TestCheckChildrenTerminal_AllDone_NoError`; it is not proof that the
currently listed test itself ran for 30 minutes.

A process-local `GIT_CEILING_DIRECTORIES` experiment was tested, not assumed.
Ordinary Git in an owned contained scratch directory changed from workspace
discovery/exit 0 to not-a-repository/exit 128, while workspace Git still
returned its root with exit 0.
However, narrowed core and policyengine tests still failed:
core exit 1 in 11.468 seconds; policyengine exit 1 in 1.462 seconds.
The core all-done test passed in this short probe; that alone does not
establish a full-suite remedy.

Read-only source diagnosis explains why the ceiling did not fix the tests:

* `internal/core/gate/runner.go` MinimalEnv omits the variable
* `internal/core/archive.go` isGitOverrideEnv explicitly removes it
* `internal/policyengine/git.go` gitEnv removes every ambient GIT_* variable
* The archive's separate gitEntryPresentAtOrAbove scan also walks ancestors
  without a matching ceiling

Those security/environment contracts require explicit adjudication.
Canonicalizing a path does not eliminate its real Git ancestor.
Requiring every workspace to equal repository top-level would break existing
nested-workspace support. Creating fake repository boundaries or removing
Git from PATH would alter fixture semantics; neither was attempted.
No supported source-free remedy was established under contained-TEMP rules.

`TestRepairShipmentMemberEvidence_NoRepoHead_Errors` is separately understood:
the implementation resolves HEAD before checking prior evidence. The fixture
passed HEAD via ancestor discovery, then correctly refused absent evidence.
The observed error does not prove that the implementation orders evidence
before HEAD or fails open.

## Independent lifecycle-fixture mismatch

`TestDoctor_IgnoresIntentionalOrphans` reproduced its active-member ship refusal
in one narrow probe. The fixture creates a task, claims its shipment, and
ships without completing the activated task. Doctor is not called until
after that failed ship step.
The full integration `TestShipmentWorkflow_ShipmentReleaseCleanup` failed
the same active-member refusal.

Relevant files are `internal/core/025_doctor_harness_test.go:85-101`,
`internal/core/shipment_lifecycle.go:170-175`,
`internal/core/shipment_gate.go:750-754`, and
`tests/integration/shipment_workflow_test.go:199`.
The minimal independent fix is proper fixture completion/evidence before
shipping, not weakening production ship checks or auto-completing members.

## Intermittent replacement observations

The full run also observed Windows AccessDenied before the intended
normalization-journal and U20C5 commit-journal fault steps.
One narrow test per surface subsequently passed.
Journal replacement uses NtSetInformationFile; artifact replacement uses
MoveFileEx. A common external handle/filter cause is plausible, not proven.
No antivirus change, broad process kill, durability weakening, or silent
retry remedy was performed. Passing narrow probes do not clear the full run.

## Capture-first residual handling

Configured read-only stash queries checked current source refs and broader
similarity candidates. The index exposed active and harvested records but
no archived-stash source. Residual records contained no confirmed identity.
Capture therefore used the P-021 discovery fail-safe, with
DISCOVERY-STATUS: LOOKUP-UNAVAILABLE in expansion field 2.
Each full six-field capture was written once and officially re-read:

* `60D799D9`: Git discovery and sanitized runner boundary contracts
* `19D55778`: Doctor/release fixture member completion sequencing
* `96F1DFFD`: unresolved intermittent Windows replacement observations

All cite task 211.008-T, feature 211-F, shipment 212-S, and independently
mark PR/thread N/A because no PR exists.
These IDs are authoritative task/run/closure-handoff residual references.
No stash edit, triage, reprioritization, or discretionary archive occurred.
Stage owns duplicate confirmation, deliberation, and separate-contract work
authorization; Ship did not expand U1b or change planning fields.

## Preserved cursor and pending work

Frozen M8 remains exactly tasks 211.001-T through 211.008-T.
Current census: terminal success 2 (U1/U1b), queued 0, active 6,
claim-assigned 6, active residual 0, blocked task statuses 0, unsupported 0.
U2-U7 remain unscaffolded and unstarted, with zero start records.
Wave index stays 1; no convergence PASS and no U2 admission are claimed.
The immutable budget remains eight tasks.

Counters remain cumulative: two tasks attempted, zero consecutive task
failures, two wave-convergence failures, no review-fix or CI-fix cycles.
The new unfiltered run occurred once, followed only by bounded named probes
and read-only diagnosis. No predictably failing third full run was made.

The branch remains
`feat/212-s-backlogit-multi-session-singleton-mcp-runtime-stdio-session-shim-and-channel-operation-coordinator`
at 834972a, with existing dirt preserved and no staged paths.
No push, PR, Copilot request, merge, live-daemon replacement, runtime
rollout, safe-close, closure PR, or successor claim occurred.
Normal merges remain pre-authorized within scope; admin fallback is not.

## Knowledge and next authority

The verified repair updated
`docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`.
The supported toolchain learning remains
`docs/compound/build-errors/match-ci-go-and-linter-before-source-repair-2026-11-06.md`.
The new Git experiment is recorded as an insufficient remedy, not promoted
into guidance.

Ship returns bounded evidence for Stage adjudication of the distinct
contracts. It does not halt the parent dark-run scheduler on a guessed
first error, but cannot silently implement independent Git-security or
lifecycle-fixture changes inside U1b. Resume this same wave cursor only
after authorized resolution and genuine unfiltered convergence GREEN.

The sole current Ship checkpoint is `checkpoint-20261106-040526.json`,
officially read back valid and conforming. Its predecessor
`checkpoint-20261106-032243.json` was individually resolved after successful
continuation through U1b completion and the recorded convergence outcome.
No Stage-owned checkpoint was restored, resolved, or pruned.
