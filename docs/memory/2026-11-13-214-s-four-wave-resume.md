---
title: "214-S Stage-amended four-wave execution"
doc_type: memory
source: docs/memory/2026-11-13-214-s-four-wave-resume.md
ingested_at: "2026-11-13T09:30:55Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: 214-S
feature_id: 213-F
---

## Confirmed recovery

The operator explicitly selected and confirmed
`checkpoint-20261113-085831.json`. Full enumeration found 200 valid records,
zero quarantine anomalies, and this one active Ship checkpoint.
Ship loaded its valid owner-matched state. Engram was reachable, bound to
the shipment branch, and not stale. Bounded read-select-summarize retained
the active release cursor, checkpoint identity, prior halt, and gate
verdicts; only superseded action-observation detail was discarded.
After accepting the Stage-amended resume point, Ship resolved the checkpoint
and deleted its file under explicit operator authorization.

The original three tasks remain claim-assigned, with one claim and zero
starts each. No reclaim or restart occurred. Stage's new declaration task
`213.004-T` is queued and has no prerequisites.
The existing shipment branch remains active in the only implementation
worktree. No main-branch write or branch switch occurred.

## Stage-authorized Step 3 refreeze

Live manifest `S` is `213-F`, `213.004-T`, `213.001-T`, `213.002-T`,
`213.003-T`. Typed resolution excludes `213-F` (`feature`).
Under Stage's explicit amendment authority, the replacement frozen task set is
`M = [213.004-T, 213.001-T, 213.002-T, 213.003-T]`, wave budget 4.
The dependency graph is acyclic and yields exactly:

* Wave 1: `213.004-T` / U11a, declaration source-shape harness first
* Wave 2: `213.001-T` / U11, dependent on U11a
* Wave 3: `213.002-T` / U12, dependent on U11
* Wave 4: `213.003-T` / U13, dependent on U12, docs-only exemption

Every green-regression array is exactly `[]`; no canonical block exists.
No red-deliverable contract exists, so the open-red mapping is empty.
The live ten-token status catalog, registry mapping and shipment/SQL flags
were reloaded. Executable statuses are queued/active/blocked;
terminal-success statuses are done/archived; all others are unsupported.
Historical scheduler simulation passed 279/279 assertions across 38 scenarios.

## Wave 1 admission and authority

Initial census: M=4, terminal-success=0, queued=1, active=3,
claim-assigned=3, active-residual=0, blocked=0, unsupported=0.
Only `213.004-T` is ready. The original task logs were read using the supplied
no-follow helper after served-root attestation. P-012 scoped raw-log fallback
continues for the three paths named in the original handoff; the configured
item tool still lacks full event streams.
Metadata binds workspace `.`, storage `.backlogit`, and main index
`.backlogit/backlogit.db`. The amended live manifest is authoritative.

The operator's original dark-mode scope and bounded merge/admin authorization
remain in force, subject to all current-HEAD readiness, CI, P-018, P-009,
P-016 and closure gates. Local session output supplies visibility.
Safety mode remains freeze-scope to U11a-U13. No launcher-script changes,
global configuration edits, or new planning work are authorized.
The prior contract halt is resolved by Stage's split, not by a policy waiver.

## Wave 1 harness milestone

U11a source-shape harness is scaffolded in
`internal/toolchain/launcher_shape_test.go`, with no production stub.
Repository compile passed; `TestU11a_LauncherDeclarationsSourceShape`
failed on the absent launcher source as an assertion, not a panic or build
failure. Harness-ready and the exact fail-closed scoped command are recorded
on `213.004-T`. Vet and lint passed. Next: commit RED, claim and record the
single start, then delegate only declaration implementation.

## Task-start verification halt

`DARK_MODE_HALTED` / `TASK_START_NOT_RECORDED`: no build-feature dispatch.
The U11a harness is committed at `04c8c1b8`, following the Stage amendment
at `8ca93cd0`. `213.004-T` was moved from queued to active.
The `WORK_STARTED: 214-S` append returned success, but the supplied
no-follow helper still reported zero starts.

Bounded read-only helper inspection found its line 62 hardcodes
`WORK_STARTED: 213-S`. Its whole-log fallback for tasks without a shipment
claim event is correct; its shipment-specific comparison is not applicable
here. No helper modification or direct raw-log read occurred.
No duplicate append was attempted. The authoritative matching start count
is unverified, not proven zero.
This also invalidates reliance on that helper's zero-start classification
for the other `214-S` members. Those classifications must be revalidated,
not inferred from a verifier for the prior shipment.

H1 ordering diagnostic: the disabled telemetry-begin call and a read-only
lifecycle topology check ran before the failed post-append confirmation was
noticed. Telemetry returned disabled and created no context or artifacts.
No implementation delegation followed. P-005 records the ordering diagnostic
and halted disposition; passing topology does not cure missing start proof.

The current task cursor is `213.004-T`, after append and before verified
start confirmation or build dispatch. Preserve the existing start comment.
The original U11-U13 tasks must not be reclaimed or restarted.
Wave 1 remains pending. No review or convergence gate ran.
No PR, CI, merge, or post-merge closure was attempted.

Required recovery: explicitly select and confirm the final Ship checkpoint,
provide a shipment-aware verifier satisfying the same safe-read contract,
then verify the existing epoch has exactly one matching start before any
further task action. Do not re-scaffold, re-append, or relabel on assumption.
No source or production repair is authorized by this operational halt.
The release remains open; `215-S` cannot start.
Final census: M=4, terminal-success=0, queued=0, active=4, blocked=0,
unsupported=0; claim-assigned classification is unverified.
The single active Ship recovery cursor is
`.backlogit/checkpoints/checkpoint-20261113-094836.json`.
The intermediate harness checkpoint was resolved, not deleted.

## Confirmed reader recovery and U11a completion

The operator explicitly selected and confirmed checkpoint
`checkpoint-20261113-094836.json` and supplied the corrected shipment-aware
reader. Full enumeration found 201 valid checkpoints, no anomalies, and
one active Ship checkpoint. Engram was reachable and bound during bounded
read-select-summarize. Cursor, checkpoint identity, prior gate verdicts and
four-task schedule were retained; superseded tool-history detail was dropped.
After served-root attestation, the corrected safe reader independently
confirmed one U11a start and zero starts on each U11-U13 member.
Ship resolved and removed the selected checkpoint under operator approval.
No duplicate start, reclaim or re-scaffold occurred.

Build-feature implemented only the four minimal wrapped ErrNotImplemented
declarations at `d351002d7330f0464cc1ab939e27069c2d8abed1`, in one attempt.
The prior RED harness `04c8c1b8` is unchanged. Scoped U11a verification passed
one named top-level test and four signature subtests. Compile, vet and format
passed. The independent lint recheck timed out during package loading;
Ship stopped authorized workspace Engram daemon PID 43400, then lint passed
with a ten-minute timeout and no source repair. Engram stays stopped for
unfiltered repository test runs.

Standard report-only local review at `d351002d` returned READY:
P0=0, P1=0, P2=0, P3=0, runtime follow-up false.
Constitution, Go and learnings personas found no issues.
New-symbol graph coverage was degraded; exact-file fallback was explicit.
Task `213.004-T` is done and linked to its implementation commit.
Telemetry was disabled, so no task telemetry close was attempted.
Wave 1 convergence is the next pending gate; no full-suite pass is claimed.

## Wave 1 convergence failure and final halt

`DARK_MODE_HALTED`, wave 1, reviewed source HEAD
`d351002d7330f0464cc1ab939e27069c2d8abed1`.
The original bounded dark-mode merge authorization is unchanged but unusable:
the mandatory unfiltered convergence gate did not pass.
No Wave 2 harness, start or implementation was admitted.

The always-on convergence gates passed: repository compile, vet, lint with
ten-minute timeout, format, and the exact non-vacuous U11a scoped command.
Open-red mapping is empty, so the unfiltered `go test -timeout=30m ./...`
was mandatory and was attempted without skips, tags or tolerated failures.
GOFLAGS was `-count=1 -p=1 -parallel=2`; GC controls were cleared.
Workspace Engram daemon PID 43400 had been stopped and remained stopped.

The full suite failed in existing
`TestExternal_LiveAcceptAndVerifyUseExternalStore` under `internal/core`.
The panic says `test timed out after 30m0s`; its printed running-test duration
is `5h24m56s` and package duration is `19925.450s`.
These are observed durations, not a diagnosed timing explanation.
The stack waits in `os/exec.Cmd.Wait` through `custodyGit`,
`custodyBlobDigest` and custody offer/adoption setup.
No launcher declaration call appears in that failing stack.
Root cause remains unconfirmed; neither a baseline defect nor an
environment-only explanation has been proven.
Ship stopped the remaining suite after the observed failure.
This is a failed, incomplete full run, never a full-suite success.
The stopped Go process was independently confirmed absent.

Failure-output evidence is retained in already-ignored tool storage:
`bin/tmp/1794584938334-copilot-tool-output-10696-cc58b189-c0ef-478e-bdfe-4a14233efd64.txt`,
SHA-256
`4f2b3db6957328dfe91fc15c39cdf7d4222e51fd5cfe7a6816e767aad8abcfa5`.
No raw failure log is added to the repository.

P-021 deferred investigation entry `9CFC88E5` records the out-of-contract
custody/Git timeout surface. P-021 C1 permits only completion of U11a's exact
declaration surface here, not a custody implementation expansion.
No custody fix, test weakening, timeout removal, or skip was applied.
`DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` records that active source-ref lookup
found no match, while the configured index provides no archived-stash read
surface. Stage owns deliberation and duplicate reconciliation.
The entry was created once with full task/feature/shipment refs, pre-PR
and threadless N/A markers, and re-read to verify persistence.
Task-, run-, and future closure-level residual risk is entry `9CFC88E5`.
It remains open and does not discharge the convergence gate.

A separate safety incident also requires the dark-mode halt:
a broad environment-name diagnostic inadvertently displayed a
credential-bearing value in local tool output.
The value is not repeated here, stored in a repository artifact, or sent
to GitHub. The operator was notified to treat that session credential as
exposed and revoke or rotate it as appropriate.
P-005 records the sanitized incident and halted disposition.
No further implementation, GitHub operation, or test rerun followed it.

Final census: M=4, terminal-success=1 (`213.004-T`, completed),
queued=0, active=3, claim-assigned=3 at last validated read,
active-residual=0, blocked=0, unsupported=0.
Completion produced the explicit member's archive-side file and removed
its queue-side file; archive integrity shows no archive deletions.
The live index reports the task done; the queue/archive change is preserved.
The shipment and feature remain active. Frozen M is unchanged.
Build attempts=1; consecutive task failures=0; review-fix cycles=0;
CI-fix cycles=0; wave-convergence failures=1.

Resume only after explicit checkpoint selection/confirmation and
operator reconciliation of the safety incident and failed convergence.
Engram must be reachable for prune-on-restore, then stopped again before
an unfiltered suite. Resume the pending Wave 1 gate, not U11a implementation.
Do not silently expand into custody code; use Stage's separately authorized
work unit if code remediation is required.
No PR, CI check, Copilot gate, merge or closure has occurred.
Shipment `215-S` remains ineligible while this release is open.
The single active Ship recovery cursor is
`.backlogit/checkpoints/checkpoint-20261113-155328.json`.
The intermediate Wave 1 convergence checkpoint was resolved and retained.

## Operator-reconciled awake-host convergence resumption

The operator explicitly selected and confirmed
`checkpoint-20261113-155328.json`. Full enumeration found 202 valid
checkpoints, no anomalies, and this one active Ship checkpoint.
Engram was reachable and bound for bounded read-select-summarize.
Ship preserved the release cursor, four-task M, gate verdicts and counters,
discarded only superseded tool-history detail, and resolved the checkpoint.
No checkpoint deletion was requested for this recovery.

The operator attributes the prior custody timeout and reported five-hour
duration to Windows Modern Standby/sleep and the resulting wall-clock jump,
citing `089-DL` / `F2509AE3`. The operator reports the named custody test
passed awake in 7.51 seconds, exit 0.
This attribution is recorded as operator reconciliation, not as an
independently proven code defect or a substitute for full-suite convergence.
Entry `9CFC88E5` remains unchanged; no custody change is authorized.

The operator also acknowledged the local-only diagnostic notice, confirmed
no credential values persisted in repository artifacts, and explicitly
authorized resumption. Future environment inspection is limited to
allowlisted names; no broad environment-value listing is permitted.
All dark-mode merge, CI, review and closure gates remain in force.

Immediately after resolution, Ship verified and stopped only workspace
Engram daemon PID 7472. The pending Wave 1 convergence gate is resumed
without restarting U11a. Its implementation remains complete and reviewed
at `d351002d`. The original failure count of one is preserved.

## Wave 1 convergence passed

The awake-host gate completed with exit 0 and
`WAVE1_CONVERGENCE_PASS` in shell `214-wave1-awake`.
Compile, vet, lint (`--concurrency=1 --timeout=15m`), format and the exact
non-vacuous U11a scoped command passed. The unfiltered
`go test -timeout=30m ./...` passed every repository package, including
`internal/core` (1030.270s), `tests` (445.903s), and integration (295.382s).
No selector, short mode, build tag, skip, or tolerated failure narrowed
the full run. Workspace Engram remained stopped.
The former full-suite obligation is discharged without custody code changes.
Captured entry `9CFC88E5` remains untouched for Stage's reconciliation.
The failure counter remains historically one; it is not reset by advancing.
Next: fresh whole-M snapshot and served-root attestation for Wave 2.

## Wave 2 admission and harness milestone

Fresh attestation again bound workspace `.`, storage `.backlogit`, and its
direct-child main index. Whole-M snapshot returned exactly four task IDs.
The unchanged live manifest and sole active shipment are `214-S`.
Census: M=4, terminal-success=1, queued=0, active=3, claim-assigned=3,
active-residual=0, blocked=0, unsupported=0.
Corrected no-follow reads found one claim and zero starts on each U11-U13
task. P-012 scoped raw-log fallback remains explicit for those three paths.
Only U11 is ready, because U11a is done; U12 and U13 still wait.

Harness-architect wrote only `internal/cli/mcp_toolchain_test.go`.
Repository compile passed. All three required U11 top-level functions failed
on assertions against the existing ErrNotImplemented declarations or missing
CLI flags, without panics or build failures. Production files and U11a's
shape harness remain untouched. Vet, lint and format passed.
The harness-ready label, named test evidence and exact fail-closed scoped
command are recorded on `213.001-T`; the command requires all three named
PASS lines and the Go exit status, not U11a or an empty selector.
Every green-regression array and tolerated-red selector set remains empty.
Next: commit this RED harness, confirm the single task start, then delegate
auditor/CLI behavior only.

## Served-root handoff and Wave 2 completion

* `handoff_status: PASS`, `shipment_id: 214-S`, `served_workspace_root_rel: .`,
  `served_storage_root_rel: .backlogit`, `manifest_path_rel: .backlogit/queue/214-S.md`,
  `index_db_rel: .backlogit/backlogit.db`,
  `ordered_items: ["213-F", "213.004-T", "213.001-T", "213.002-T", "213.003-T"]`

Following the host memory-commit remediation, the operator selected and
confirmed `checkpoint-20261113-171033.json`. Full checkpoint enumeration found
203 records with zero anomalies and that single active Ship checkpoint.
Engram (`daemon-status`) was reachable and bound during bounded
read-select-summarize; Ship reconciled the pre-commit cursor with landed
commits `bf268094` (RED U11 harness) and `1333ecae` (U11 implementation),
verified the single `WORK_STARTED: 214-S` entry on `213.001-T` via
`bin/tmp/213-safe-log-read.py`, and resolved `checkpoint-20261113-171033.json`.

U11 verification passed at `1333ecae`: fail-closed `U11_HARNESS_PASS`
(all 3 required named `TestU11_` tests plus 3 additional suites under `-race`),
`TestU11a_LauncherDeclarationsSourceShape`, `go vet`, `gofmt`, and
`golangci-lint` (v1.64.8 with cached `go1.26.5`). Five-persona report-only
review (`Constitution Reviewer`, `Go Reviewer`, `Learnings Researcher`,
`Security Reviewer`, `Scope Boundary Auditor`) returned `READY_WITH_FOLLOWUPS`
(`P0=0`, `P1=0`). Two out-of-scope P2 findings were captured once via P-021 C2:
`0CD5F543` (`verify.go` `COPILOT_HOME`/`ENGRAM_DATA_DIR` directory check) and
`E231EA13` (`Execute()` `--jsonrpc --strict` `error.data` forwarding).
`213.001-T` was linked to `1333ecae`, moved to `done`, and annotated.

## Wave 2 convergence passed

Workspace Engram daemon PID 57032 was verified and stopped before Wave 2
convergence. Compile (`go test -run=^$ -count=1 ./...`), `go vet ./...`,
`gofmt -l .`, and `golangci-lint run --concurrency=1 --timeout=15m`
(`GOTOOLCHAIN=go1.26.5`) passed. An initial unfiltered `go test -timeout=30m ./...`
run failed solely on 40 `TestU175_*_LintFix` tests (`exit status 7`) because
host `go1.27.4` emits export data v5 while installed `golangci-lint` v2.13.2
was built with `go1.27.0` (max export data v4); `wave_gate_failures`
incremented from `1` to `2` (limit `3`). Pinning `GOTOOLCHAIN=go1.27.0` (cached
locally) with `GOFLAGS="-count=1 -p=1 -parallel=2"`, the unfiltered
`go test -timeout=30m ./...` passed every repository package with exit 0.

Next: Wave 3 admission (`213.002-T` / U12).

## Wave 3 completion and convergence circuit breaker halt

Wave 3 admission passed served-root attestation, whole-M snapshot (`M=4`,
`terminal_success=2`, `queued=0`, `active=2`, `claim-assigned=2`,
`active_residual=0`, `blocked=0`, `unsupported=0`), and no-follow safe log read
(`start_count=0` on `213.002-T` and `213.003-T`). `harness-architect` scaffolded
ONLY `tests/integration/multi_session_workspace_containment_test.go`, confirmed
compilation (`go test -run=^$ -count=1 ./...` exit 0) and genuine RED
(`TestU12_*` failing on absent `.backlogit/toolchain.yaml` and
`ErrNotImplemented`), and committed RED at `a10edad6`.

Single `WORK_STARTED: 214-S` was appended to `213.002-T` and verified
(`start_count=1`) before implementation. `build-feature` (`Go Engineer`)
implemented `WriteToolchainManifest` and `RenderPinnedMCPConfig` in
`internal/toolchain/launcher.go` and created `.backlogit/toolchain.yaml`.
Five-persona report-only review surfaced two P1 findings and one P2 finding in
`launcher.go` (missing `backlogit`/pin validation, `--cwd`/URL-ID checks, and
non-atomic pre-remove write) plus an out-of-scope P2 finding on `verify.go`
relative `workspace_root: "."` resolution (captured via P-021 C2 stash
`D8EE8389`). After fixing all in-scope findings (atomic temp-file +
`os.Rename`, pin/URL/flag-injection validation, clean test cleanup/deadlines),
3-persona re-review returned `READY_WITH_FOLLOWUPS` (`P0=0`, `P1=0`), with a P2
intra-process unique temp-file suffix fixed and re-verified (`U12_HARNESS_PASS`,
`TestU11_`, `TestU11a_`, `go vet`, `gofmt`, `golangci-lint` all exit 0).
`213.002-T` was committed at `84e830e1eeed96173f53273072083ffcdd0367dd`,
linked, moved to `done`, and annotated.

Before Wave 3 convergence, workspace Engram daemon PID 60864 was verified and
stopped. Compile (`go test -run=^$ -count=1 ./...`), `go vet ./...`,
`gofmt -l .`, and `golangci-lint run --concurrency=1 --timeout=15m`
(`GOTOOLCHAIN=go1.26.5`) passed. Unfiltered `go test -timeout=30m ./...`
(`GOTOOLCHAIN=go1.27.0`, `GOFLAGS="-count=1 -p=1 -parallel=2"`) passed 49 of
50 packages (`tests` 537.279s, `tests/contract` 145.872s, `tests/integration`
432.243s including all `TestU12_`), failing only on one pre-existing test:
`TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks/preliminary_plan_reads_wait_for_stash_global_and_membership_locks`
in `internal/core` (`14.17s`; 10s `reconCtx` timeout hit `context deadline exceeded`
in `query database links: get links for 003-S`).

This brought cumulative `wave_gate_failures` across `214-S` to `3` (limit `3`),
tripping the P-002.6 Wave convergence gate circuit breaker:

* `DARK_MODE_HALTED` emitted; Wave 4 (`213.003-T` / U13) is NOT admitted.
* P-021 C2 deferred entry `040826D4` captured for the out-of-scope
  `internal/core/shipment_reconstitute_hardening_test.go:142` timeout.
* Census at halt: `M=4`, `terminal_success=3` (`213.004-T`, `213.001-T`,
  `213.002-T`), `queued=0`, `active=1` (`213.003-T`, `claim-assigned`,
  `start_count=0`), `active_residual=0`, `blocked=0`, `unsupported=0`.
* Pending next step after operator review/reset: re-run Wave 3 convergence
  gate (`go test -timeout=30m ./...` with `GOTOOLCHAIN=go1.27.0` and Engram
  stopped), then proceed to Wave 4 (`213.003-T` / U13) and PR lifecycle.


