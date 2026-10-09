---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the corrective CX release unit: Ship Step 6 closure protocol (allowlisted staging, pre/safe-close/post ordering, registry capability predicate), Ship gate scope and argv-safe snapshot fallback, gate-aware Orchestrator eligibility and configured served roots, harness lock-sidecar rename, ship-time validation performance, docs-migrate protection for closure gate keys, and the 140-S closure gate registration that unblocks 141-S.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-10-08-cx-s-ship-closure-gate-correctness-plan.md
title: 'Implementation Plan: CX Ship closure protocol and gate correctness'
---

# Implementation Plan: CX Ship closure protocol and gate correctness

## Objective

Fix the closure and gate defects that every shipment passes through, and
unblock the 141-S `pre_claim` gate, before any other queued shipment is
claimed. Source decision:
`docs/decisions/2026-10-08-cx-s-ship-closure-gate-correctness-deliberation.md`
(decisions D1-D11).

## Problem Frame

* Ship Step 6 stages `.backlogit/` wholesale, so unrelated pre-existing
  backlog changes land in closure commits (75E02C17).
* Ship Step 6 calls `backlogit_ship_shipment` directly between reconcile
  `pre` and `post`, which skips safe-close's baseline, non-member, and
  parentage checks (52D18E44).
* Ship Step 6 item 1 is gated on prose instead of `features.shipments: true`
  (497D20E3).
* Ship Step 4.0 item 1 has a CLI fallback with no ID validation or argv safety
  (FBD6E6F8). Ship applied the 5a lifecycle gate to a closure PR (F05661B1).
* The Orchestrator reports DAG-ready shipments as eligible even when the
  `pre_claim` gate would refuse them (6AB5E7FC), and its served-root manifest
  lookup assumes default queue and archive directories (8F1CF1E1).
* Harness lock scripts use `.<file>.lock`, the same name as backlogit's
  persistent OS-lock sidecars, so every backlogit artifact looks locked
  (67F17B6B).
* `shipment ship` validation costs about 10 s per member with no progress
  output (D116AF58).
* `docs migrate --apply` folds closure gate keys under `docline`, which would
  break topology closure discovery (1293086D).
* 141-S fails `pre_claim` with `PREDECESSOR_CLOSURE_INCOMPLETE` because no
  `docs/closure/140-S-*-post-merge-closure.md` exists.

## Scope

In scope: the units below. Out of scope: `plugin/agents/ship.agent.md`,
autoharness upstream templates and the upstream `post_ship` phase ask,
6387A6A2, EB95F5F7, AC530DE2, F88FE051, and CT-S.

## Requirements Trace

| Req | Source | Requirement | Units |
|---|---|---|---|
| R1 | 75E02C17 | Step 6 stages only an allowlist derived from the safe-close report; any other staged path halts | U1, U8 |
| R2 | 52D18E44 | Step 6 runs reconcile pre, safe-close, post; safe-close is the only caller of the governed close | U1, U1b, U8 |
| R3 | 497D20E3 | Step 6 item 1 is gated on `features.shipments: true` | U1, U8 |
| R4 | FBD6E6F8 | Step 4.0 item 1 CLI fallback validates canonical task IDs and passes them as discrete argv | U2, U9 |
| R5 | F05661B1 | The 5a lifecycle gate applies only to the feature PR; Step 6.0 closure PRs run P-014 and P-018 only | U2, U9 |
| R6 | 6AB5E7FC | Orchestrator eligibility reports show the `pre_claim` gate verdict next to DAG readiness | U3, U10 |
| R7 | 8F1CF1E1 | Served-root manifest lookup uses attested `workspace.queue_path` and `workspace.archive_path` (supersedes 195s plan R4(c)) | U3, U10 |
| R8 | 67F17B6B | Harness lock files are named `.<file>.agent-lock` in all four scripts and both docs | U4, U11, U12, U13 |
| R9 | 1293086D | Normalization keeps `closure_status`, `compaction_status`, and `conditions` top-level for closure docs | U5, U14 |
| R10 | D116AF58 | Ship-time member validation is profiled, fixed, and reports progress | U6, U15, U16a, U16b |
| R11 | 141-S gate | A 140-S gate-registration closure record exists and the 141-S `pre_claim` gate passes the closure check | U7 |
| R12 | harness | Every edited installed artifact has a current drift record | U17 |

## Implementation Units

### U1: Ship closure protocol contract test (RED)

Domain: tests. Posture: test-first; RED is the deliverable. Requirements
R1-R3. File: `tests/integration/cx_ship_closure_protocol_contract_test.go`
(new).

Add `TestCXS1_ShipClosureProtocolContract` (unit token CXS1). Read
`.github/agents/_ship.agent.md`, normalize whitespace, and slice Step 6 item 1
between the unique anchors `1. **Close the shipment**` and
`2. **Runtime validation and releasability evidence**`. Subtests:

1. `AllowlistedStaging`: the slice does not contain `git add .backlogit/`;
   it contains `safe-close report`, `allowlist`, and
   `git diff --cached --name-only`.
2. `SafeCloseOrdering`: `mode: pre`, `mode: safe-close`, and `mode: post`
   appear in that order, and the slice has no direct
   "Call `backlogit_ship_shipment`" instruction outside the safe-close step.
3. `CapabilityPredicate`: the slice heading contains `features.shipments: true`
   and the slice does not contain `shipments are enabled in this workspace`.

AC:

1. `go test -count=1 -v -run '^TestCXS1_' ./tests/integration` fails on main on
   all three subtests, with the RED output recorded in the task.
2. One new test function, three named subtests, helpers as local closures, no
   production declarations. `gofmt -l` and `go vet ./...` are clean.

### U1b: Re-anchor TestUSR3_ to the safe-close step (RED)

Domain: tests. Posture: test-first. Requirement R2. File:
`tests/integration/shipment_reconcile_feature_member_contract_test.go`
(re-anchor only). Split from U1 at plan review to keep U1 under the
scenario limit.

Re-anchor `TestUSR3_ShipmentReconcileExplicitFeatureMemberContract` subtest
`ProceedAndShipStep6` from the end anchor
"b. Call `backlogit_ship_shipment` with the merge commit SHA" to the new
safe-close step anchor `mode: safe-close`. Keep its
`feature-pending-governed-completion` assertion. This is the D2 contract change.

AC:

1. `TestUSR3_` fails only in `ProceedAndShipStep6` until U8 lands (RED output
   recorded).
2. No other subtest changes; gofmt and vet clean.

### U2: Ship gate-scope contract test (RED)

Domain: tests. Posture: test-first. Requirements R4-R5. File:
`tests/integration/cx_ship_gate_scope_contract_test.go` (new).

Add `TestCXS2_ShipGateScopeContract` with subtests:

1. `SnapshotFallbackArgv`: the Step 4.0 item 1 slice (start anchor
   `1. **Snapshot the whole task wave set from live state.**`, end anchor the
   next numbered item of Step 4.0, which the implementer verifies is unique)
   contains
   `^[0-9]+\.[0-9]+-T$`, `discrete argv`, and `WAVE_SNAPSHOT_UNRELIABLE`.
2. `ClosurePRGateScope`: the Step 5 item 5a text and the Step 6.0 item 4 text
   both contain `does not apply to Step 6.0 closure PRs`. The implementer
   names the start and end anchors for both slices and verifies each is
   unique in the file.

AC:

1. `go test -count=1 -v -run '^TestCXS2_' ./tests/integration` fails on main on
   both subtests.
2. One function, two subtests, no production declarations; gofmt and vet clean.

### U3: Orchestrator eligibility and configured-root contract test (RED)

Domain: tests. Posture: test-first. Requirements R6-R7. Files:
`tests/integration/cx_orchestrator_eligibility_contract_test.go` (new) and
`tests/integration/orchestrator_served_root_handoff_contract_test.go`.

Add `TestCXS3_OrchestratorGateAwareEligibility`: the Step 0 slice (anchors
`### Step 0` to `### Step 0.0b` or the next unique heading the implementer
verifies) contains
`autoharness gate pipeline-topology --mode agent --shipment {id} --phase pre_claim --json`,
`gate verdict`, and `DAG readiness`.

In `TestUSR1_OrchestratorServedRootHandoffContract` subtest
`ProcedureLiterals`, replace the literal "exactly one of `queue` or `archive`"
with `workspace.queue_path`, `workspace.archive_path`, and
`contained in the served storage root`. Add one assertion that the slice does
not contain `This assumes the default archive directory`.

AC:

1. `TestCXS3_` fails on main. `TestUSR1_` fails only in `ProcedureLiterals`
   until U10 lands.
2. Two files, one new function, one updated subtest; gofmt and vet clean.

### U4: Harness lock-sidecar contract test (RED)

Domain: tests. Posture: test-first. Requirement R8. File:
`tests/integration/cx_lock_sidecar_contract_test.go` (new).

Add `TestCXS4_HarnessLockSidecarName`, table-driven over
`scripts/acquire_lock.ps1`, `scripts/release_lock.ps1`,
`scripts/acquire_lock.sh`, `scripts/release_lock.sh`,
`.github/instructions/concurrency.instructions.md`, and
`.github/skills/file-lock/SKILL.md`. Each must contain `.agent-lock`. The four
scripts must not build a path ending in `.lock` (assert the absence of the
current sidecar construction expressions, which the implementer records
verbatim from each script). Three scenarios: scripts contain the new suffix,
scripts drop the old construction, docs name the new suffix.

AC:

1. `go test -count=1 -v -run '^TestCXS4_' ./tests/integration` fails on main.
2. One file, one function, three scenarios; gofmt and vet clean.

### U5: Docline closure gate-key preservation test (RED)

Domain: tests. Posture: test-first. Requirement R9. File:
`internal/docline/normalize_test.go`.

Add `TestNormalize_ClosureGateKeysStayTopLevel`: normalize a
`docs/closure/141-S-x-post-merge-closure.md` input with top-level
`closure_status`, `compaction_status`, `conditions`, and one unknown key.
Assert the three gate keys stay top-level with unchanged values, the unknown
key folds under `docline`, and a non-closure path (`docs/decisions/x.md`) still
folds `closure_status`. Three scenarios.

AC:

1. `go test -count=1 -run '^TestNormalize_ClosureGateKeysStayTopLevel$' ./internal/docline`
   fails on main.
2. One test function; gofmt and vet clean.

### U6: Ship-time validation benchmark spike

Domain: tests. Posture: spike. Requirement R10. File:
`internal/core/shipment_gate_bench_test.go` (new).

Add `BenchmarkValidateMemberGateEvidence` over a temporary workspace with a
45-member release scope and realistic item logs. Capture a CPU profile and
record in the task notes which calls dominate (`findArtifact` WalkDir,
`loadArtifact`, `events.ReadAllEvents`, lock waits). Verdict: CONFIRMED (the
per-member lookup or log read dominates) or REFUTED (name the real cost).

AC:

1. The benchmark runs with
   `go test -run '^$' -bench BenchmarkValidateMemberGateEvidence -count=1 ./internal/core`.
2. The task notes record ns/op, the top five profile entries, and the verdict.
3. If REFUTED, U15 is re-planned through Stage before it is claimed.

### U7: 140-S closure gate registration record

Domain: docs. Requirement R11. File:
`docs/closure/140-S-158-F-post-merge-closure.md` (new).

Follow `docs/closure/136-S-154-F-post-merge-closure.md`: the gate-required
comment block ("do not run `backlogit docs migrate --apply`"), top-level
`closure_status: READY`, `compaction_status: done`, `doc_type: closure`,
`docline.backlogit.gate_registration: true`, `source`, `title`, and a body
that cites the narrative closure
`docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md`, PR #436, and its
merge commit (read from the narrative closure). Do not rename or edit the
narrative closure; three documents reference its path.

AC:

1. `autoharness gate pipeline-topology --mode agent --shipment 141-S --phase pre_claim --json`
   no longer reports `PREDECESSOR_CLOSURE_INCOMPLETE` for 140-S (output
   recorded in the task). Remaining tokens caused by CX itself (141-S blocks on
   CX) are expected until CX ships.
2. `backlogit docs lint --path docs/closure` and P-008 markdownlint are clean
   for the new file.

### U8: Ship Step 6 closure protocol text

Domain: docs (installed agent text). Posture: test-first (U1, U1b). Safety mode:
careful. Requirements R1-R3. File: `.github/agents/_ship.agent.md` (Step 6
item 1 only).

1. Replace the heading clause with
   `1. **Close the shipment** (only when the backlog registry declares features.shipments: true):`
   and add one sentence: when the key is absent or false, skip item 1 and
   record why in the closure artifact.
2. Keep a0 (lifecycle gate) and a (`mode: pre`). Replace b with
   `mode: safe-close`, which is the only caller of `backlogit_ship_shipment`.
   Keep the flat-membership paragraph, the third-branch (halted archival)
   paragraph, and the compensation paragraph, reworded to refer to the
   safe-close result envelope. Do not add any `RECONCILE_FAIL` reference.
3. Keep c (P-007) and d (`mode: post`).
4. Replace e with allowlisted staging. Build the allowlist from the
   safe-close report, keyed by work-item ID rather than by core internals:
   paths under the configured queue, archive, and logs directories whose
   base name begins with an ID in `M` or with `shipment_id`, plus the
   shipment's `.backlogit/reconcile/{shipment_id}-*` reports. Shared index
   or side-effect files that ShipShipment writes are added only by name
   pattern when the safe-close report lists them. Do not hardcode a file list
   copied from `internal/core`. Stage with `git add -- <paths>`,
   verify `git diff --cached --name-only` equals the changed subset of the
   allowlist, and halt on any other staged path. Commit in a separate command.

AC:

1. `TestCXS1_` and `TestUSR3_` pass.
2. `TestUSR6_HarnessManifestDriftRecords`, `ship_post_merge_sync_protocol_test.go`,
   and `shipment_155_harness_contract_test.go` stay green.
3. Only Step 6 item 1 of `.github/agents/_ship.agent.md` changes; markdownlint
   is clean.

### U9: Ship gate-scope text

Domain: docs (installed agent text). Posture: test-first (U2). Safety mode:
careful. Requirements R4-R5. File: `.github/agents/_ship.agent.md` (Step 4.0
item 1, Step 5 item 5a, Step 6.0 item 4). Depends on U8 (same file).

1. Step 4.0 item 1: before any `backlogit get {id} --format json` fallback,
   validate each frozen ID against `^[0-9]+\.[0-9]+-T$` and pass it as a
   discrete argv element, never interpolated into a shell string. A
   non-matching ID halts with `WAVE_SNAPSHOT_UNRELIABLE`.
2. Step 5 item 5a and Step 6.0 item 4: add "The 5a lifecycle gate does not
   apply to Step 6.0 closure PRs." Step 6.0 closure PRs run P-014 and P-018
   only. Step 6.1 a0 is unchanged.

AC:

1. `TestCXS2_` and `TestCXS1_` pass.
2. Only the three named locations change; markdownlint is clean.

### U10: Orchestrator eligibility and configured-root text

Domain: docs (installed agent text). Posture: test-first (U3). Safety mode:
careful. Requirements R6-R7. File: `.github/agents/_orchestrator.agent.md`.

1. Step 0 item 2 and every "eligible shipments" report: for each DAG-ready
   queued candidate, run
   `autoharness gate pipeline-topology --mode agent --shipment {id} --phase pre_claim --json`
   and show the gate verdict and token next to DAG readiness. Report a
   candidate as eligible only when both pass. When the gate is not installed,
   say so and do not invent a verdict. Reporting never claims.
2. Served-Root Handoff Procedure: the manifest must exist in exactly one of
   the attested `workspace.queue_path` or `workspace.archive_path`
   directories, each contained in the served storage root with no symlink or
   reparse-point component. Remove the default-directory caveat. Replace the
   direct-child requirement for the manifest directory with containment.

Stage already appended the R4(c) amendment pointer to the 195s plan; this unit
does not edit that plan.

AC:

1. `TestCXS3_` and `TestUSR1_` pass.
2. Only `.github/agents/_orchestrator.agent.md` changes; markdownlint is clean.

### U11: PowerShell lock scripts rename

Domain: config (scripts). Posture: test-first (U4). Requirement R8. Files:
`scripts/acquire_lock.ps1`, `scripts/release_lock.ps1`.

Change the sidecar name to `.<file>.agent-lock`. Never delete or rewrite a
`.<file>.lock` file. Keep token semantics unchanged.

AC:

1. The PowerShell rows of `TestCXS4_` pass.
2. Manual smoke recorded in the task: with a backlogit sidecar
   `.backlogit/queue/.X.md.lock` present, `scripts/acquire_lock.ps1` on
   `.backlogit/queue/X.md` succeeds and `release_lock.ps1` with the token
   removes only `.X.md.agent-lock`.

### U12: Bash lock scripts rename

Domain: config (scripts). Posture: test-first (U4). Requirement R8. Files:
`scripts/acquire_lock.sh`, `scripts/release_lock.sh`. Same change and
constraints as U11.

AC:

1. The Bash rows of `TestCXS4_` pass.
2. `bash -n` passes for both scripts; a manual smoke under Git Bash or WSL is
   recorded in the task.

### U13: Lock documentation rename

Domain: docs. Posture: test-first (U4). Requirement R8. Files:
`.github/instructions/concurrency.instructions.md`,
`.github/skills/file-lock/SKILL.md`.

Name the `.<file>.agent-lock` sidecar, state that `.<file>.lock` belongs to
backlogit and is never created or removed by harness scripts.

AC:

1. The documentation rows of `TestCXS4_` pass (the script rows pass once U11
   and U12 land).
2. Markdownlint is clean for both files.

### U14: Docline closure gate-key fix

Domain: code. Posture: test-first (U5). Requirement R9. Files:
`internal/docline/normalize.go`, `internal/docline/policy.go`.

Add a closure gate-key set (`closure_status`, `compaction_status`,
`conditions`) in `policy.go`. In `Normalize`, when the classified doc type is
`closure`, skip folding those keys and copy them back to the top level of the
encoded frontmatter unchanged.

AC:

1. U5 passes. `go test -count=1 ./internal/docline/...` passes.
2. Running `backlogit docs migrate` in dry-run over `docs/closure` reports no
   relocation of the three keys (output recorded).

### U15: Ship-time validation performance fix

Domain: code. Posture: characterization-first (U6 verdict CONFIRMED is a
precondition). Harness: exempt, covered by the U6 benchmark and the existing
shipment gate tests (behavior-preserving refactor). Requirement R10. File:
`internal/core/shipment_gate.go`.

Resolve member artifacts once per gate pass instead of one WalkDir per member,
and read each member's event log once. Do not change verdicts, error
classification, or lock order.

AC:

1. The U6 benchmark improves by at least the share of time U6 attributes to
   per-member lookup and log reads (target recorded from U6 before work
   starts; before and after recorded). A miss routes back to Stage like a
   REFUTED verdict.
2. `go test -count=1 ./internal/core/...` passes, including every existing
   shipment gate test.

### U16a: Ship-time validation progress test (RED)

Domain: tests. Posture: test-first. Requirement R10. File: the CLI
`shipment ship` command test file (implementer names it).

AC:

1. A CLI test asserts `validating member i/n: <id>` lines on stderr and an
   unchanged stdout JSON envelope. It fails on main (RED output recorded).
2. gofmt and vet are clean.

### U16b: Ship-time validation progress output

Domain: code. Depends on U16a and U15 (same file). Requirement R10. Files:
`internal/core/shipment_gate.go` (optional progress callback) and the CLI
`shipment ship` command file.

Report progress on stderr from the CLI surface only. MCP output and JSON
stdout stay unchanged. The progress callback is optional, so MCP call sites
are not edited.

AC:

1. U16a passes.
2. `go test -count=1 ./internal/core/... ./internal/cli/... ./internal/mcp/...`
   passes.

### U17: Harness-manifest drift records

Domain: config. Posture: verification-only. Requirement R12. File:
`.autoharness/harness-manifest.yaml`.

For `.github/agents/_ship.agent.md`, `.github/agents/_orchestrator.agent.md`,
the four lock scripts, `concurrency.instructions.md`, and the `file-lock`
skill: update `checksum` to the current SHA-256, keep `drift_allowed: true`,
and append to `drift_reason` the governing stash IDs. Preserve the existing
citations (227A2930, B83081F5) and the phrase "Do not auto-revert.".

AC:

1. `TestUSR6_HarnessManifestDriftRecords` passes.
2. Each updated checksum equals `Get-FileHash -Algorithm SHA256` of the file
   (recorded).
3. U17 is re-run as the last step before merge, so review fixes after Wave 4
   do not leave stale checksums.

## Dependency Graph

```text
Wave 1: U1, U1b, U2, U3, U4, U5, U6, U7, U16a
Wave 2: U8 <- U1, U1b; U10 <- U3; U11 <- U4; U12 <- U4; U13 <- U4; U14 <- U5; U15 <- U6
Wave 3: U9 <- U2, U8; U16b <- U15, U16a
Wave 4: U17 <- U8, U9, U10, U11, U12, U13
```

Shipment-level edges: 152-S blocks on CX, 141-S blocks on CX. CX takes
`queue_position: 100`.

## Decisions and Rationale

See the deliberation D1-D11. Additional plan decisions:

* The 195s plan requirement R4(c) is superseded by R7. Stage appended an
  amendment pointer to the 195s plan.
* Contract tests use the collision-free unit tokens CXS1-CXS4.
* U9 depends on U8 because both edit `_ship.agent.md`.

## Risks

* CX's own post-merge closure is the first run of the new Step 6 protocol. If
  safe-close or allowlisted staging halts, Ship follows the halt path; there
  is no fallback to `git add .backlogit/`.
* The lock rename changes the harness concurrency contract. A stale
  `.<file>.lock` held by an old script during the transition is not honored by
  new scripts. Mitigation: concurrent-access mode is rarely enabled; release
  notes in the closure.
* The U6 spike may refute the hypothesis; U15 is then re-planned.

## Constitution Check

* Single-domain tasks within the 2-hour rule: yes (19 units after plan
  review split U1 and U16).
* Test-first for every behavior change: yes (U1, U1b, and U2-U5 RED before
  U8-U14; U16a before U16b).
* Justified deviation: U15 is harness-exempt. It is a behavior-preserving
  performance refactor covered by the U6 benchmark and the existing shipment
  gate tests, and a benchmark cannot be a failing-first harness.
* No new dependencies, no schema change, no MCP contract change: yes.
* Stage does not perform the 140-S rename itself: yes (U7 is a Ship task).

Constitution Check: pass

## Plan Hardening Signals

* Changes installed agent text that governs closure and claim (contract).
* Changes a concurrency primitive (lock scripts).
* Touches the governed close path performance (D116AF58).

Requires plan hardening: yes

## Runtime Verification and Closure

* Before merge: `go test -race ./...`, `go vet ./...`, golangci-lint,
  `make docs-lint`, `scripts/md-lint.ps1`, and the CI cli-reference-drift job.
  Known flake 8F41D60D (`TestAppendGateEvidence_ConcurrentSameItem_NoDuplicateCounters`,
  fixed by CT, which runs after CX) may fail internal/core under load. One
  rerun of internal/core is allowed when that test is the only failure, and
  the rerun cites 8F41D60D. Any other failure halts.
* After merge: CX's own closure exercises the new Step 6 text. The closure
  record states the allowlist used and the safe-close report path.
* After closure: the Orchestrator runs the 141-S `pre_claim` gate and records
  the verdict.

## Plan Hardening

### Context consulted

* `.github/skills/shipment-reconcile/SKILL.md` Safe-Close Mode steps 1-10.
* `docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md`.
* `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md`.
* `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`.
* Owner harnesses `TestUSR1_`, `TestUSR3_`, `TestUSR6_`.

### Protected invariants

* Safe-close remains the only caller of the governed close; it is called
  exactly once per closure.
* Post-mode keeps the halted-archival branch: no `git restore` when
  `failed_step: shipped-event-append`.
* The 5a, P-014, and P-018 gates are never weakened for feature PRs.
* Served-root attestation stays MCP-only and fail-closed.
* Harness scripts never touch backlogit `.lock` sidecars.
* Normalization of non-closure docs is unchanged.

### Risky actions

* U8 rewrites the closure sequence. Mitigation: careful mode, U1 and USR3
  re-anchor as RED-first harness, and the plan forbids any fallback to
  directory staging.
* U15 touches the governed close path. Mitigation: verdicts, errors, and lock
  order unchanged; the full `internal/core` suite gates it.
* U11 and U12 change lock semantics. Mitigation: suffix rename only; token
  semantics unchanged; manual smoke recorded.

### Added verification

* U8 AC requires `git diff --cached --name-only` to equal the changed subset
  of the allowlist.
* U7 AC requires the live `pre_claim` gate output.
* U17 AC requires recomputed checksums.

### Closure, monitoring, and rollback

* Rollback is a revert of the CX merge. The 140-S registration record is
  additive and can stay.
* The CX closure record lists the allowlist, the safe-close report path, and
  the lock-rename note for operators.
* Upstream asks recorded in the closure: the autoharness `post_ship` phase
  (F05661B1) and the lock-suffix rename in the autoharness templates (R8).

### Review-gate capability risks

* Copilot review may flag the new Step 6 wording as diverging from the plugin
  Ship agent. The plugin agent is out of scope; reply with this plan section.

### Unresolved operator decisions

* None. The operator approved the scope on 2026-10-08.

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: FAIL
* attempt: 1
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* findings: U16 touched three files and mixed test and code; U16 AC omitted ./internal/core/...; U15 lacked a failing harness and used an arbitrary 5x target; U1 was at the scenario limit; U8 allowlist was coupled to core internals; U17 checksums could go stale; CX quality gate exposed to flake 8F41D60D.

<!-- plan-review-attempt: 1 -->

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: ADVISORY
* attempt: 2
* reviewers: Constitution Reviewer (re-review of revisions); prior round Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist
* revisions: U1b split from U1; U16 split into U16a (RED) and U16b; U15 harness exemption recorded in Constitution Check with a U6-derived target; U8 allowlist keyed by work-item ID; U17 AC3 re-run before merge; 8F41D60D single-rerun rule; U13 AC limited to documentation rows.
* residual advisories: Go Reviewer note that U14 must write closure keys back after ToMap (already stated in U14); Scope Boundary Auditor note that U16b could depend only on U6 (kept on U15 because both edit shipment_gate.go).
* operator_authorization: approved (operator APPROVED this scope and directed autonomous work without routine confirmations, relayed by the Orchestrator on 2026-10-08)