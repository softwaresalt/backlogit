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
| R7 | 8F1CF1E1 | Served-root manifest lookup uses attested `workspace.queue_path` and `workspace.archive_path` (supersedes 195s plan R4(c)) (delivered as the Orchestrator text contract; the Go metadata catalog still reports hard-coded `storage/queue` and `storage/archive` for non-default `queue_layout.root_dir`, which can only produce a false fail-closed; named limitation, follow-up capture 00A9D01C; see Erratum E3) | U3, U10 |
| R8 | 67F17B6B | Harness lock files are named `.<file>.agent-lock` in all four scripts and both docs | U4, U11, U12, U13 |
| R9 | 1293086D | Normalization keeps `closure_status`, `compaction_status`, and `conditions` top-level for closure docs | U5, U14 |
| R10 | D116AF58 | Ship-time member validation is profiled and reports progress; the performance fix is NOT delivered (U15 descoped by operator ruling; see Erratum E2; follow-up stash 76553D8D) | U6, U15, U16a, U16b |
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

## Erratum E1 (2026-10-10)

This erratum documents an already-reviewed plan. It records planning fields
that Stage wrote onto the backlog tasks and the covering feature 197-F so that
Ship can admit the wave schedule under the P-002 red-first rules. It was made
under P-017 dark factory mode (scope [197-S] only). It does not change scope:
shipment membership S (197-F plus the 19 tasks, 20 items) and the task set M
(the 19 tasks), dependencies, waves, task titles, and acceptance criteria are
unchanged, no plan line above was edited, and plan-review was not re-run. The
full record is `docs/memory/2026-10-10/stage-197s-red-contract-amendment.md`.

### P-002.2 red-deliverable contracts

Seven red deliverables carry a canonical red-deliverable contract on their
backlog task. The validated red, green-makers, close-wave mapping follows. It
is derived from the unit text and the Dependency Graph above. Every
green-maker is in M, is not the red deliverable itself, and lands in a
strictly later wave. The close wave equals the latest green-maker wave.

| Red deliverable | Green-maker(s) | Green-maker wave | Close wave |
|---|---|---|---|
| 197.001-T (U1, `TestUCXS1_`) | 197.009-T (U8) | 2 | 2 |
| 197.002-T (U1b, `TestUSR3_`) | 197.009-T (U8) | 2 | 2 |
| 197.003-T (U2, `TestUCXS2_`) | 197.010-T (U9) | 3 | 3 |
| 197.004-T (U3, `TestUCXS3_` and `TestUSR1_`) | 197.011-T (U10) | 2 | 2 |
| 197.005-T (U4, `TestUCXS4_`) | 197.012-T, 197.013-T, 197.014-T (U11-U13) | 2 | 2 |
| 197.006-T (U5, `TestUCXS6_NormalizeClosureGateKeysStayTopLevel`) | 197.015-T (U14) | 2 | 2 |
| 197.017-T (U16a, `TestUCXS5_`) | 197.018-T (U16b) | 3 | 3 |

Selector declaration for U16a (197.017-T): the plan leaves the test name to
the implementer, but the red selector command must be exact. Stage declared
the next unused collision-free unit token, `CXS5`, so the test prefix is
`TestUCXS5_` (see the rename below), and the package `./internal/cli`, where
the `shipment ship` command and its tests live. It declares no test body and no
scope. Amendment A3 on 197.017-T records that a differently named test would
match nothing under the declared selector (`WAVE_RED_DELIVERABLE_VACUOUS`).

### Selector prefix rename (P-002.6 requirement 4)

The test names that this plan gives (`TestCXS1_` to `TestCXS4_` in U1 to U4, and
`TestNormalize_ClosureGateKeysStayTopLevel` in U5) do not start with `TestU`.
P-002.6 requirement 4 (`.github/policies/workflow-policies.md`,
`build-feature` Step 0.5, and `_ship.agent.md`) requires every task-scoped
harness command to be anchored to `-run '^TestU<unit>_'`, and the scheduler
replay rejects a red selector that is not (`Test-TaskScopedCommandShape` in
`scripts/wave-scheduler-sim.ps1`; Ship would halt `WAVE_RED_MAPPING_UNRESOLVED`
at wave-1 dispatch of all seven red deliverables). The policy wins over the
plan's names. Stage therefore declared the repository convention `TestU` plus a
unit token on the backlog tasks, and the tasks govern wherever they differ from
the plan text above:

| Plan unit | Unit token | Declared test prefix or name | Package |
|---|---|---|---|
| U1 | `CXS1` | `TestUCXS1_` (was `TestCXS1_`) | `./tests/integration` |
| U2 | `CXS2` | `TestUCXS2_` (was `TestCXS2_`) | `./tests/integration` |
| U3 | `CXS3` | `TestUCXS3_` (was `TestCXS3_`); also `TestUSR1_` | `./tests/integration` |
| U4 | `CXS4` | `TestUCXS4_` (was `TestCXS4_`) | `./tests/integration` |
| U16a | `CXS5` | `TestUCXS5_` | `./internal/cli` |
| U5 | `CXS6` | `TestUCXS6_NormalizeClosureGateKeysStayTopLevel` (was `TestNormalize_ClosureGateKeysStayTopLevel`) | `./internal/docline` |

`TestUSR1_`, `TestUSR3_`, and `TestUSR6_` are unchanged. Union selectors keep the
required anchor: `'^TestU(CXS3_|SR1_)'` (197.004-T red selector and the 197.011-T
owner command) and `'^TestU(CXS1_|SR3_)'` (197.009-T owner command, because U8
turns both 197.001-T and 197.002-T green). Every covered-by owner command is
byte-identical to its owner's red selector, except that documented union.
Subtests and the function suffixes are unchanged.

### P-002.1 exemption contracts and the closed exempt set

Ten tasks carry a P-002.1 exemption contract. The closed exempt set is
recorded on 197-F and is exactly these ten tasks:

* 197.007-T (U6), verification-only: the new benchmark file only.
* 197.008-T (U7), docs-only: one new markdown file.
* 197.009-T (U8), 197.010-T (U9), 197.011-T (U10), 197.012-T (U11),
  197.013-T (U12), 197.014-T (U13), 197.015-T (U14), and 197.018-T (U16b),
  covered by the red deliverable that each one turns green (table above).

No other task may claim an exemption without a reviewed amendment.

### U15 harness posture (supersedes the Constitution Check deviation)

The Constitution Check line "Justified deviation: U15 is harness-exempt" and
the U15 unit line "Harness: exempt, covered by the U6 benchmark" above are
superseded. P-002.1 withdrew a prose-only exemption, and U6 is itself exempt,
so it cannot own U15's harness. U15 is harness-required. It carries no
exemption label and no exemption contract. Ship's harness-architect scaffolds
its RED harness at the start of wave 2, after U6 is done and its verdict,
ns/op, profile, and target are recorded in the U6 task notes. The harness must
fail on main for the right reason and pass only after the refactor, derive its
failing condition from the U6 verdict and target, and be task-scoped. Its shape
is the harness-architect's choice. For example, (a) a work-count
characterization test, (b) a U6-derived performance budget, or (c) a
source-shape assertion. These three are examples, not a closed list. A
work-count RED may use `_test.go`-only seams and must add no production seam and
no production stub, because P-002.1 (cycle 31) forbids a production declaration
ahead of its harness. A timing or performance budget may only corroborate a RED
and is never the sole RED condition, because timing is nondeterministic. Every
existing shipment gate test, verdict, error classification, and lock order must
stay unchanged. A REFUTED U6 verdict, a missed target, or no valid
RED harness halts and returns to Stage; U15 is never exempted and no substitute
harness is scaffolded. The deliverable, file, acceptance criteria, and
dependencies of U15 are unchanged. The tightened wording is Amendment A1 on
197.016-T, which also adds a pointer line after the stale "Harness: exempt"
prose there.

### U17 harness posture (Amendment A2)

U17 (197.019-T) is harness-required, not exempt. It edits a repository
configuration file, `.autoharness/harness-manifest.yaml`, which no exempt class
admits. The U17 unit line "Posture: verification-only" above is therefore only
its execution posture. Ship's harness-architect scaffolds its harness at the
start of wave 4, after U8 to U14 (including U9) have landed. The RED harness
follows the 196 drift-record shape
(`tests/integration/harness_manifest_196_drift_records_test.go`). Per manifest
entry for the files this release changes, it asserts exactly one entry,
`drift_allowed: true`, a 64-hex lowercase checksum that differs from the
pre-refresh value pinned at wave-4 start, and a `drift_reason` that keeps
`Do not auto-revert.` and the existing citations and cites this release's
governing stash IDs. It fails at wave-4 start because those entries are still
stale and passes only after the refresh. It must not assert that the checksum
equals the current file hash, because that would redden every later shipment
that edits those files, and like the 196 test it does not recompute hashes.
`TestUSR6_HarnessManifestDriftRecords` (U17 AC1) is already green on main, so it
is a regression guard and not the RED harness. The one-time currency check in
U17 AC2 uses the manifest's LF-normalized SHA-256 convention (the recipe in the
archived 196.006-T AC1), because raw `Get-FileHash` on a CRLF checkout does not
match it. If no valid RED shape can be formed, the task halts and returns to
Stage. U17 carries no exemption label and no contract.

### Other task-body amendments

* Amendment A3 on 197.017-T: the declared selector is
  `-run '^TestUCXS5_'` in package `./internal/cli`; it supersedes "implementer
  names it" for the test name and package.
* 197.008-T (U7): the content probe in its exemption command was loosened so a
  line-wrapped gate-required comment still matches. It was checked as a
  positive control against a document structured like
  `docs/closure/136-S-154-F-post-merge-closure.md` and still fails before the
  deliverable exists. An evidence note records that U7 AC1's gate-run output can
  be observed only after 197-S leaves `active` (the known
  `PRECLAIM_ACTIVE_SHIPMENT_PRESENT`); the AC text is unchanged.
* 197.007-T (U6): a sentence records that its verdict, ns/op, top-five
  profile, and target are the input Amendment A1 of 197.016-T requires, and that
  missing evidence is a wave-2 halt for 197.016-T.
* 197.005-T (U4): one line requires every table-row subtest to be named after
  its file path, because the exempt gates of 197.012-T to 197.014-T filter
  `--- FAIL:` rows by path.

### Unchanged

Shipment membership S (197-F plus the 19 tasks), the task set M (the 19 tasks),
dependencies, waves (W1 U1, U1b, U2-U7, and U16a; W2 U8 and
U10-U15; W3 U9 and U16b; W4 U17), titles, and acceptance criteria are
unchanged.

## Erratum E2 (2026-10-10)

This erratum records an operator-authorized manifest amendment: unit U15
(197.016-T) is descoped from 197-S. It was made under P-017 dark factory mode
(scope [197-S] only). Unlike Erratum E1, it changes scope (shipment membership
S and the task set M shrink by one task), so it cites the operator ruling
verbatim. No plan line above was edited, and plan-review was not re-run: the
change removes one unit and one dependency edge under explicit operator
authority and adds no scope. The full record is
`docs/memory/2026-10-10/stage-197s-red-contract-amendment.md`.

### Operator ruling (verbatim)

> "Descope U15 as recommended."

Ruled 2026-10-09T22:47-07:00 (2026-10-10T05:47Z). It is the explicit operator
authorization for a Stage manifest amendment and an explicit re-freeze of M.
"As recommended" is option 1 ("Descope U15 (recommended)") under "Decisions
needed" in `docs/memory/2026-10-10/orchestrator-197s-wave2-halt-memory.md`,
and the last section of the Stage note named above. The other two options
(re-plan U15 as a new profiling spike plus a retargeted fix, or keep U15 as
written with no RED harness) were not chosen.

### Why U15 is descoped

The U6 benchmark (197.007-T, done) and Stage's code analysis refuted U15's
premise:

* One `validateMemberGateEvidence` call, which is what U6 timed, already reads
  each member's event log once (`events.ReadAllEvents`, one `os.Open` and one
  scan, no lock) and resolves its artifact once (`loadArtifact`, database
  first, filesystem walk only on a miss). The U6 profile attributes about
  80.2% to `ReadAllEvents`, 13.6% to `loadArtifact`, and 0% to the
  `findArtifact` WalkDir. The redundant share inside the function U15 may
  change is 0%, so the share of time attributable to per-member lookup and log
  reads (93.8%) is essential cost, and U15 AC1 cannot be met by removing
  redundant work.
* U6 measured about 0.9 to 3.4 ms per member (39 to 153 ms per op over 45
  members; the host noise band is 3.9x). Stash D116AF58 reports about 10 s per
  member. The real cost sits outside `validateMemberGateEvidence`, in code U6
  did not profile: `snapshotShipArtifacts` (`findArtifact` WalkDir at
  `shipment_lifecycle.go:238`, `FindArtifactPath` at `:246`,
  `LockItemLogCrossProcess` at `:254`) and `attachCommitToItems`
  (`findArtifact` at `:877`). Those are outside U15's declared file
  (`internal/core/shipment_gate.go`).
* The only duplicate read on the real path is the deliberate second gate pass
  (`shipment_lifecycle.go:607` and `:627`, re-validation after the artifact
  locks are taken). Removing it changes verdict timing and lock order, which
  U15 forbids.
* A timing-only target is not defensible, so no valid RED harness exists
  (Amendment A1 HALT class). The 1 MiB scanner buffer in
  `internal/events/reader.go` (13.0% of the U6 loop, `memclrNoHeapPointers`) is
  a different file and also outside U15.

### Amended manifest and task set

| | Before E2 | After E2 |
|---|---|---|
| Shipment membership S | 197-F plus 19 tasks (20 items) | 197-F plus 18 tasks (19 items) |
| Frozen task set M (Ship re-derives it at its next Step 3) | 19 tasks | 18 tasks |
| 197.016-T (U15) | member, `active`, claim-assigned | removed from S and M; archived (governed `backlogit_archive_item`) with a comment citing this ruling |

The old wording "197-F plus exactly 19 atomic tasks" in the 197-S description
is superseded: the manifest is 197-F plus exactly 18 atomic tasks. Order,
`queue_position`, status, priority, labels (including `dag-root`), and links of
197-S are unchanged. The follow-up for the performance fix is stash entry
76553D8D ("Profile the real ShipShipment path and optimize per-member
snapshot/lock cost (R10 remainder)"), which cites D116AF58, this plan, the U6
evidence, the unprofiled suspects, and the constraints (keep verdicts, error
classification, and lock order; needs deliberation and a new profiling spike
first). It is an ordinary Stage follow-up capture, not a P-021 Ship capture.

### Removed dependency edge

The edge 197.018-T to 197.016-T (U16b depends on U15) is removed with the
governed `backlogit_remove_dependency`. U16b's "same file" ordering was never a
real dependency: its progress callback is optional and does not need U15's
refactor. 197.018-T now depends only on 197.017-T (U16a). Amendment A4 on
197.018-T records this. The unit text above, "Depends on U16a and U15 (same
file)", and the Dependency Graph line "U16b <- U15, U16a" are superseded: U16b
depends on U16a only. Its deliverable, files, acceptance criteria, and
`covered-by` exemption contract (owner 197.017-T) are unchanged.

### R10 coverage

Requirement R10 ("profiled, fixed, and reports progress") is now covered as
follows:

* Profiled: U6 (197.007-T, done), with the limits stated above.
* Reports progress: U16a (197.017-T, done, RED) and U16b (197.018-T).
* Fixed: moved to follow-up stash 76553D8D. It is not delivered by 197-S.

U15's own amendments are moot: Amendment A1 stays on the archived 197.016-T
as history, and no Amendment A1.1 (a numeric U6-derived target) is written.

### Recomputed wave partition

Topological layering of the 18-task amended graph (verified against
`item_deps`; the graph is acyclic and every dependency of a member is a
member):

| Wave | Tasks | Notes |
|---|---|---|
| W1 | 197.001-T to 197.008-T (U1, U1b, U2 to U7) and 197.017-T (U16a) | done |
| W2 | 197.009-T (U8), 197.011-T (U10), 197.012-T to 197.014-T (U11 to U13), 197.015-T (U14), 197.018-T (U16b) | 197.018-T moves up from W3: it depends only on 197.017-T (W1) |
| W3 | 197.010-T (U9) | depends on 197.003-T (W1) and 197.009-T (W2) |
| W4 | 197.019-T (U17) | depends on 197.009-T to 197.014-T; the real edge to 197.010-T (W3) keeps it in W4 |

Before E2 the partition was W2 = U8 and U10 to U15, W3 = U9 and U16b, and
W4 = U17. The plan's Dependency Graph lines "Wave 2 ... U15 <- U6" and
"Wave 3 ... U16b <- U15, U16a" are superseded for U15 and U16b. U17 never
depended on U15 or U16b (its dependencies are U8 to U13), so its wave and its
Amendment A2 are unchanged.

### 197.017-T close-wave correction

The red-deliverable contract of 197.017-T (U16a) names 197.018-T as its
green-maker. 197.018-T now lands in wave 2, so `green_maker_closes_wave` moves
from 3 to 2. This is the only contract change. The line is the single key
`green_maker_closes_wave` in the red-deliverable contract block of the archived
`.backlogit/archive/197.017-T.md`. That archived file is edited only for this
key, under the operator authorization for this amendment, and the diff is one
line. Two prose spots on that task stay as written and are historical: the
`red_deliverable_reason` phrase "in wave 3" and the Harness Manifest record in
its implementation notes ("green_maker_closes_wave: 3"). They are superseded by
this erratum. The scheduler parses only the contract keys, so they do not
affect admission.

| Red deliverable | Green-maker(s) | Green-maker wave | Close wave |
|---|---|---|---|
| 197.001-T (U1) | 197.009-T (U8) | 2 | 2 |
| 197.002-T (U1b) | 197.009-T (U8) | 2 | 2 |
| 197.003-T (U2) | 197.010-T (U9) | 3 | 3 |
| 197.004-T (U3) | 197.011-T (U10) | 2 | 2 |
| 197.005-T (U4) | 197.012-T, 197.013-T, 197.014-T (U11 to U13) | 2 | 2 |
| 197.006-T (U5) | 197.015-T (U14) | 2 | 2 |
| 197.017-T (U16a) | 197.018-T (U16b) | 2 (was 3) | 2 (was 3) |

All seven red contracts and all ten exemption contracts were re-validated
against the amended partition: every green-maker is in M, is not the task
itself, and lands in a strictly later wave than its red deliverable; each close
wave equals the latest green-maker wave; every `covered-by` owner is a declared
dependency of the exempt task, is a red deliverable, and is not itself exempt;
every task-scoped selector matches the `^TestU` anchored shape; and the closed
exempt set on 197-F is unchanged (ten tasks, equal to the `harness-exempt`
labels). 197-F was not edited. Its statement that 197.016-T (U15) is
deliberately not an exempt member remains true. 197.019-T (U17) remains the
only harness-required member that is neither a red deliverable nor exempt.

### Unchanged

The acceptance criteria, deliverables, files, titles, and dependencies of every
other unit, the P-002.1 exemption contracts, Amendments A2 (U17) and A3 (U16a),
and all statuses and claims of the remaining members are unchanged. No other
shipment or stash entry was touched. The only archived task files affected are
197.016-T (moved to the archive by the governed archive operation) and
197.017-T (the single key above).

## Erratum E3 (2026-10-10)

* **Row annotations.** Ship's branch-wide adversarial review found the Requirements Trace
  over-claimed. Finding P1-5: the R10 row said validation was "fixed", but U15 was descoped (E2),
  so the row now states profiling and progress only, with follow-up stash 76553D8D. Finding P1-1:
  the R7 row now notes the Go metadata catalog still hard-codes `storage/queue` and
  `storage/archive` for non-default `queue_layout.root_dir`, with follow-up capture 00A9D01C.
* **Orchestrator disposition (P-017 dark mode; operator AFK, sound-judgement authority; subject
  to operator veto).** R7 is accepted as PARTIAL for non-default queue layouts, with 00A9D01C as
  the named limitation; it fails closed only, with no unsafe behavior. Finding P1-4 (the
  safe-close report does not list exact tracked side-effect paths such as
  `.backlogit/hooks_queue.jsonl`, so the closure allowlist never auto-stages them; the closure
  commit stages them by explicit path) is accepted as documented residual risk, practical severity
  P2, to be captured by Ship.
* **Cycle cap.** The review-fix cycle cap (3) is extended by exactly ONE additional cycle,
  limited to in-scope P0/P1 findings of the confirming review. After it, Ship must halt rather
  than iterate.
* **Unchanged.** Acceptance criteria, membership M (18 tasks), dependencies, and waves.