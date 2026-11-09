---
title: "212-S Wave 11 harness manifest"
feature_id: "211-F"
shipment_id: "212-S"
tasks: ["211.020-T", "211.021-T"]
---

## Result and boundary

Compilation: PASS

Red Phase: CONFIRMED

Scaffold only, on the inherited feature branch and worktree. Both tasks remain
queued. Their prerequisite, `211.007-T`, was verified done through backlogit.
No retry implementation, task claim/start/completion, staging, commit, or
full-suite execution occurred. The inherited failed count remains 7;
Ordinal 9 is NOT CONSUMED.

The operator's current authorization supersedes the older zero-production-edit
task prose only for these unexported seams:

* `internal/atomicfile/atomicfile_windows.go`: add `atomicReplaceSleep = time.Sleep`
* `internal/core/shipment_ops_windows.go`: add `shipmentOpsNtRenameFn`,
  `shipmentOpsSetDispositionFn`, and `shipmentOpsRetrySleep`; route the existing
  single rename/disposition calls through the corresponding syscall seams

All other Go edits are tests in the two authorized test files. All edits used
`apply_patch`. No plan or review artifact was modified.

## Harness functions and scenarios

### 211.020-T U1i

File: `internal/atomicfile/atomicfile_windows_test.go`

Only new top-level test:
`TestU1i_AtomicReplaceRetriesTransientWindowsErrorsWithExponentialBackoff`

* `access`, `sharing`, `wrapped_access`, and `wrapped_sharing`, each crossed with
  `durable_{false,true}` and `exhaust_{false,true}`: 16 assertion-RED subtests
* Eventual success delegates to real MoveFileEx on attempt 8
* Exhaustion asserts `ErrWriteNotApplied`, the original error chain, intact
  destination, and intact source
* Every attempt checks source, destination, and durability flags
* Every sleep checks attempt/sleep ordering and destination preservation
* Exact assertion: 8 attempts; 7 sleeps at 5, 10, 20, 40, 80, 160, 320ms;
  635ms total; no extra sleep after the final attempt
* Immediate file-not-found and generic failure: 2 passing control subtests,
  each asserting 1 attempt, 0 sleeps, and not-applied classification

Observed RED example:
`access/durable_false/exhaust_false` reports expected attempts 8, actual 1;
expected seven sleeps, actual empty; expected 635ms, actual 0s; eventual
replacement does not commit. The top-level function genuinely fails on missing
retry behavior, not on a placeholder or build error.

### 211.021-T U1j

File: `internal/core/shipment_ops_security_windows_test.go`

Only new top-level test:
`TestU1j_ShipmentOpsJournalWindowsTransientContentionRetryAndSecurityPreservation`

* `rename` and `remove`, crossed with `errno_access`, `errno_sharing`,
  `status_access`, `status_sharing`, and `status_delete_pending`, each raw and
  wrapped, each eventual-success and exhaustion: 40 assertion-RED subtests
* Both mutation seams assert 8 total attempts and the exact 7-sleep, 635ms
  sequence, including attempt/sleep ordering
* Eventual success delegates to the real syscall on attempt 8
* Exhaustion preserves the journal preimage and writer-owned temp payload
* Mutation retries retain the validated source handle; rename retries retain
  the directory-relative buffer pointer, class, length, and replace flag
* Directory-object binding during backoff: 2 assertion-RED subtests, one per
  mutation, swap the canonical directory on the first sleep and require only
  the originally validated directory object to change; decoys remain intact
* Existing journal and temp opens, each eventual-success and exhaustion:
  4 assertion-RED subtests use a real GENERIC_READ handle denying DELETE
  sharing; an independent kernel probe verifies the fixture; the seventh
  injected sleep releases the blocker only for eventual success
* Immediate `STATUS_FILE_IS_A_DIRECTORY`, file-not-found, generic failure,
  and `ErrValidation`, raw and wrapped, for both mutations: 16 passing control
  subtests asserting 1 attempt, 0 sleeps, and unchanged files
* `validation_and_reparse_never_retried`: passing control checks nil directory,
  invalid child names, mismatched writer ownership, non-writer temp removal,
  directory handle validation, and writer-shaped directory rejection
* Nested `reparse_temp`: passing control checks a real junction handle returns
  `ErrValidation`; rename/remove reach neither mutation seam nor sleep and
  preserve the external-to-ops preimage within the workspace test root

There are 63 direct U1j subtests: 46 assertion-RED and 17 passing controls.
The nested reparse subtest also passes, without a skip on this host.

Observed RED example:
`rename/errno_access/wrapped_false/exhaust_false` reports expected attempts 8,
actual 1; expected seven sleeps, actual empty; expected 635ms, actual 0s.
`open/temp_true/exhaust_true` fails the missing seven-sleep and 635ms assertions.
The top-level function is assertion-RED; validation controls are not fabricated
failures.

All seam-changing tests and subtests are sequential, use `t.Cleanup` to restore
seams, and never call `t.Parallel`. No goroutine or wall-clock sleep is used.

## Exact executed verification commands

Each command ran separately from `C:\Source\GitHub\backlogit`.

Baseline and final compilation, both exit 0; all 46 first-party packages compile,
and no tests execute:

```powershell
& ./bin/tmp/212-go-env.ps1 go test -run=^$ -count=1 ./...
```

Initial non-race RED checks, both exit 1 with assertions:

```powershell
& ./bin/tmp/212-go-env.ps1 go test -v -count=1 -run '^TestU1i_' ./internal/atomicfile
& ./bin/tmp/212-go-env.ps1 go test -v -count=1 -run '^TestU1j_' ./internal/core
```

Final task-scoped RED commands, both exit 1 with the intended assertions and no
race findings:

```powershell
& ./bin/tmp/212-go-env.ps1 go test -v -timeout=30m -count=1 -race -run '^TestU1i_' ./internal/atomicfile
& ./bin/tmp/212-go-env.ps1 go test -v -timeout=30m -count=1 -race -run '^TestU1j_' ./internal/core
```

Final verbose outputs were captured by the terminal tool:

* U1i: `bin/tmp/1794251165429-copilot-tool-output-12284-e7875ba6-066c-454a-8873-98e4c39b6991.txt`
* U1j: `bin/tmp/1794251184872-copilot-tool-output-12284-49d61da3-6345-4057-bdc0-aa788098d89e.txt`

Initial U1j fixture defects were corrected before final verification: the
sharing blocker now requests GENERIC_READ and proves denial independently;
directory and junction fixtures now use accepted journal filename classes.
The final evidence above replaces those preliminary runs.

Existing safety regressions, both exit 0 under race instrumentation:

```powershell
& ./bin/tmp/212-go-env.ps1 go test -v -timeout=30m -count=1 -race -run '^(TestAtomicReplace_|TestWriteFileAtomic_|TestMoveFileExFlags_)' ./internal/atomicfile
& ./bin/tmp/212-go-env.ps1 go test -v -timeout=30m -count=1 -race -run '^(TestUR1B_|TestP021ClaimSerialization_|TestShipmentOpsJournal|TestWriteShipmentLifecycleJournal_|TestNewWorkspace_Rejects)' ./internal/core
```

Atomic regressions retain three existing Windows/POSIX-permission skips.
Core regressions pass, including directory-object binding and unsafe-residue
rejection.

Quality gates, each exit 0 with no findings, rerun after final Go edits:

```powershell
& ./bin/tmp/212-go-env.ps1 ./bin/golangci-lint.exe run --timeout=10m
& ./bin/tmp/212-go-env.ps1 go vet ./...
& ./bin/tmp/212-go-env.ps1 gofmt -l cmd internal tests scripts
```

Read-only formatting inspection also exited 0 with an empty diff:

```powershell
& ./bin/tmp/212-go-env.ps1 gofmt -d internal/atomicfile/atomicfile_windows.go internal/atomicfile/atomicfile_windows_test.go internal/core/shipment_ops_windows.go internal/core/shipment_ops_security_windows_test.go
```

## Same-contract P-002.6 scaffold review correction

Replaced only the new `runU1jValidation/reparse_temp` junction-creation skip
branch with `require.NoError`. Junction fixture failures now fail closed.
No retry behavior or refactor was added; existing test skips remain unchanged.

Verification ran as separate terminal calls through the existing environment
wrapper, without chained commands or output redirection:

```powershell
& ./bin/tmp/212-go-env.ps1 go test -v -race -count=1 -timeout=30m -run '^TestU1j_' ./internal/core
& ./bin/tmp/212-go-env.ps1 go test -run=^$ -count=1 ./...
& ./bin/tmp/212-go-env.ps1 go vet ./...
& ./bin/tmp/212-go-env.ps1 ./bin/golangci-lint.exe run --timeout=10m
& ./bin/tmp/212-go-env.ps1 gofmt -l cmd internal tests scripts
```

U1j remains assertion-RED (exit 1), with 46 failing direct subtests and
17 passing controls; nested `reparse_temp` passes. No panic, skip, or race
finding was reported. Compilation, vet, lint, and formatting checks each
exited 0 with no findings. Baseline compilation also passed before the edit.
The terminal captured U1j output at
`bin/tmp/1794251630908-copilot-tool-output-12284-cd35e456-aa59-4f7b-ae5b-c02188af8ebb.txt`.

This correction touched only the named test and this memory file.
No full suite, retry implementation, claim/start/status change, staging,
commit, or agent delegation occurred. Protected checkpoint210317 remains
untouched; Ordinal 9 remains unconsumed and the inherited failed count remains 7.

## Downstream green verification commands

These are the Stage acceptance selectors, not additional executions in this
scaffold session:

```powershell
& ./bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -race -run '^(TestU1i_|TestAtomicReplace_|TestWriteFileAtomic_|TestMoveFileExFlags_)' ./internal/atomicfile
& ./bin/tmp/212-go-env.ps1 go test -timeout=30m -count=1 -race -run '^(TestU1j_|TestUR1B_|TestP021ClaimSerialization_|TestShipmentOpsJournal|TestWriteShipmentLifecycleJournal_|TestNewWorkspace_Rejects)' ./internal/core
```

## Instrumentation limits and custody

There is no existing NtCreateFile seam in `shipment_ops_windows.go`. No new open
seam was introduced. Open tests therefore cover deterministic real sharing
contention, exact sleep sequence, eventual success, exhaustion, and preservation;
they cannot directly count NtCreateFile invocations or inject access-denied and
delete-pending statuses at that call. Those raw/wrapped statuses and exact
8-attempt counts are covered at both authorized mutation seams.

The new path-swap-on-backoff success assertions intentionally await GREEN before
their success branch can execute. Existing directory-binding security tests
already pass against the single-shot code.

Only harness-ready labels and evidence comments are authorized backlog changes;
no planning fields or task status changes are part of this manifest.
No further agents were spawned. Intercom was unavailable; progress was reported
in the caller channel.

Protected checkpoint `.backlogit/checkpoints/checkpoint-20261106-210317.json`
remains unstaged and untouched, with SHA-256
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
All changes remain uncommitted for parent scaffold review.
