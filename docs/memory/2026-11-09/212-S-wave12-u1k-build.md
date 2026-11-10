---
title: "212-S Wave 12 U1k build evidence"
---

## Same-contract scope

Task `211.022-T`, feature `211-F`, shipment `212-S`, Wave 12.
RED scaffold: `f31e768a6a7ea1313b5bc85fcfb7916f36c565ce`.
Stage authorization:
[M22 amendment](stage-212-s-089-dl-u1k-m22-amendment.md).
Harness contract: [U1k harness evidence](212-S-wave12-u1k-harness.md).

Only `internal/core/gate/baseref.go` and `internal/core/gate/probe.go`
changed in production; the harness is unchanged. No new exported symbols
or dependencies. Git verification adds a ceiling only strictly inside TEMP,
resolves symlinks where possible, and preserves explicit ceilings (including
empty values and Windows key casing). Temporary initialized repositories
remain discoverable. Successful parsed version probes use the authorized
`sync.Map` exception, keyed by resolved path, mtime, size, Dir and child env;
errors and unparseable versions are not cached.

Single existing worktree/branch, one writer, careful/freeze-scope posture.
No claim, restart, status, archive, backlog, push, PR or merge operation.
Parent-owned task/hook edits remain excluded. Telemetry begin was disabled;
no context/close telemetry was sent. Lifecycle topology PASS was supplied
by the parent, including its independent single-start verification.

## Harness loop

Two harness attempts, one implementation fix, no circuit breaker.
The exact fail-closed command below was used unchanged on both attempts:

```powershell
pwsh -NoProfile -Command '$output = @(& pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -race -v -run "^TestU1k_" ./internal/core/gate 2>&1); $code = $LASTEXITCODE; $output; if ($code -ne 0) { exit $code }; foreach ($name in @("TestU1k_ExecGitRunnerVerifyIsolatesTempDirFromOuterCheckout", "TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity")) { if (-not ($output -match ("^--- PASS: " + [regex]::Escape($name) + " "))) { throw "Missing named PASS: $name" } }'
```

| Attempt | Exit | Result | Go package elapsed |
|---|---|---|---|
| 1 | 1 | Both intended named assertion REDs | 2.663s |
| 2 | 0 | Both required named PASS records; nonvacuous GREEN | 5.306s |

Attempt 1 observed ancestor HEAD leakage and subprocess counts 2/3/4/5
instead of 1/2/3/3. Attempt 2 printed:

```text
--- PASS: TestU1k_ExecGitRunnerVerifyIsolatesTempDirFromOuterCheckout (1.28s)
--- PASS: TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity (0.28s)
```

Harness wall times were not separately measured. Compile-only checks passed
before implementation and after the fix; the final timed check is below.

## Scoped and static gates

All commands exited 0. Go tooling used `bin/tmp/212-go-env.ps1`, pinning
Go 1.24 with workspace-contained caches and TEMP. Wall times include wrapper
and build overhead; package times are Go's reported elapsed values.
Vet, lint and format ran concurrently.

| Gate | Exact tool arguments | Package elapsed | Measured wall |
|---|---|---|---|
| Compile only, 46 packages | `go test -run=^$ -count=1 ./...` | 0.446-2.731s | 30.683s |
| Scoped race regression | Command below | 5.975s | 11.865s |
| Archive regression | `go test -timeout=5m -count=1 -v -run '^TestArchiveItem_' ./internal/core` | 29.079s | 40.882s |
| Vet | `go vet ./...` | N/A | 26.705s |
| Lint | `golangci-lint run --timeout=5m` | N/A | 34.228s |
| Tracked-first-party format | `git ls-files -- "*.go"`; batches of 100 to `gofmt -l` inside wrapper | 1,400 files; no findings | 1.638s |
| Baseline EOL | `go test -timeout=2m -count=1 -v -run '^TestU175_001_BaselineEol$' ./tests` | 1.151s | 4.834s |

Required scoped race command:

```powershell
pwsh -NoProfile -File bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -race -run '^(TestU1k_|TestProbe|TestVersionAtLeast|TestExecVersionRunner_|TestResolveBaseRef|TestExecRunner_|TestResolveChildEnv_)' ./internal/core/gate
```

Baseline EOL test elapsed: 0.70s; `checked=1746 excluded=0 index-not-lf=0
work-not-lf=0 w/mixed=0` before adding this evidence note.
A post-staging EOL rerun also passed with the same counts: test 0.69s,
package 1.211s, wall 4.878s. `git diff --check` and the staged whitespace
check passed. Only the two production files and this note were staged.

Inputs remained `wave_scoped:true`, `green_regression_cmds:[]`,
`sibling_red_selectors:[]`, `open_red_selectors:[]`,
`red_deliverable:false`. No unfiltered suite ran; Ordinal 11 and postcommit
review/completion/archive remain parent-owned.

## Archive per-test timings

All 38 top-level tests passed. Names below omit the common `TestArchiveItem_`
prefix. The under-2s target was not uniformly met: three top-level tests and
one subtest exceeded it. No before/after speedup claim is supported.

| Test suffix | Elapsed |
|---|---|
| ArchiveEventCannotBeSplitFromMutationByCThenBWriter | 3.63s |
| ArchiveEventCannotBeSplitFromMutationByCThenBWriter/cascade | 2.08s |
| ArchiveEventCannotBeSplitFromMutationByCThenBWriter/standalone | 1.55s |
| TrackedGitArtifactUsesGitMoveAndPreservesFollowHistory | 2.40s |
| TrackedShortArtifactStagesPureMoveForFollowHistory | 2.75s |
| UntrackedGitArtifactUsesFilesystemFallback | 1.73s |
| GitMoveDBFailureRollsBackWorktreeAndIndex | 1.87s |
| PostArchiveStashSideEffectIsCASGuarded | 1.06s |
| PostArchiveStashSideEffectIsCASGuarded/without_concurrent_writer_linked_stash_is_archived | 0.44s |
| PostArchiveStashSideEffectIsCASGuarded/concurrent_unarchive_between_commit_and_side_effect_refuses_stale_stash_archive | 0.62s |
| CascadeStashSideEffectRunsAfterItemLogLocksReleased | 0.90s |
| DurableFsyncsSourceParent | 0.47s |
| SourceFsyncFailureDoesNotRollBack | 0.59s |
| DurableFsyncsNewArchiveDirParent | 0.64s |
| DurableOffSkipsNewDirFsync | 0.43s |
| QueuePathAbsentAfterArchive | 0.35s |
| NoDuplicateAcrossQueueArchive | 0.34s |
| DuplicateSourcePaths_PreservesSuccess | 0.37s |
| DuplicateSourcePaths_NoStaleQueueCopy | 0.35s |
| DuplicateSourcePaths_ArchiveFileCorrect | 0.35s |
| DuplicateSourcePaths_UsesConfiguredQueueRoot | 0.41s |
| RefusesDistinctOccupiedDestination | 0.19s |
| SameItemHalfArchiveRecoverySucceeds | 0.43s |
| Success_FileAtArchivePathDBUpdated | 0.35s |
| DBFailureAfterFileMove_OriginalFileRestored | 0.32s |
| DBFailure_DBStatusUnchanged | 0.32s |
| ReArchivePreservesReconciledArchivedStatus | 0.30s |
| GenuineQueuedToArchiveStillStampsPreArchiveStatus | 0.39s |
| ReArchiveWithEmptyArchivedStatusStillStamps | 0.21s |
| MovesToArchive | 0.39s |
| ExcludedFromDefaultList | 0.39s |
| IncludedWhenExplicitlyRequested | 0.42s |
| ItemAlreadyInArchiveDir | 0.56s |
| PreArchivedStampsCanonicalArchivedFrom | 0.19s |
| CascadeArchivesChildren | 1.39s |
| NoCascadeByDefault | 0.35s |
| CascadeSkipsAlreadyArchived | 1.83s |
| ArchivesLinkedStashEntries | 0.42s |
| NoStashLinkIsNoOp | 0.37s |
| ShippedShipmentWithoutDurableEvent_Refused | 0.22s |
| ShippedShipmentWithDurableEvent_Allowed | 0.44s |
| NonShippedShipmentClosure_Unaffected | 0.45s |
