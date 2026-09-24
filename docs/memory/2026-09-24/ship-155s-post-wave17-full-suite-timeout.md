---
title: Ship 155-S Post-Wave-17 Full Suite Timeout
date: 2026-09-24
status: blocked
shipment: 155-S
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: eedc48a16711fc5e3cd5b94b5619c61e2f006659
---

# Ship 155-S Post-Wave-17 Full Suite Timeout

## Outcome

The single operator-authorized execution of `go test ./...` exited `1`.
The command was not rerun. Final standard review, adversarial review, PR, CI,
Copilot review, merge, shipment archival, runtime verification, and operational
closure did not start.

## Captured evidence

The workspace-contained capture is complete and untruncated:

* Output: `logs/diagnostics/155-s-go-test-post-wave17-20260924.txt`
* Metadata:
  `logs/diagnostics/155-s-go-test-post-wave17-20260924.metadata.json`
* Native exit: `1`
* Duration: `803.433` seconds
* Captured: 597 lines, 77,279 bytes
* Limit: 10,000 lines or 1 MiB

## Failure

Package `github.com/softwaresalt/backlogit/internal/core` timed out after ten
minutes. The test reported as running was:

```text
TestSetArtifactSize_BusyLockReturnsErrTaskBusy (0s)
```

The active test goroutine was in:

```text
artifact_size_test.go:166
  -> setupSizeWorkspace
  -> NewWorkspace
  -> newWorkspace
  -> config.LoadTemplates
  -> filepath.WalkDir
  -> os.ReadFile
```

The goroutine was in a Windows file-open syscall while loading templates. The
stack does not show a lock waiter or a ten-minute per-test hang. The `0s`
duration shows the test had only begun when the package-wide deadline expired.

Task `174.071-T` authorized conversion of exactly
`setupTestWorkspace` and `setupTestWorkspaceWithBugLevel`; it explicitly
excluded all other fixtures. Changing `setupSizeWorkspace`, package timeout
policy, template loading, or broader fixture construction is outside that
task's P-021 C1 contract surface.

Deferred scope expansion: `62C6A469`.

## Current state

* `155-S`: active
* `154-S`: queued and unmodified
* `174.071-T`: archived
* `174.072-T`: archived
* Implementation PR: none
* PR #449: untouched
* Engram: runtime-degraded; two daemon startup probes timed out
* Concurrent unrelated configuration and agent edits remain preserved

## Next action

Stage must deliberate and create a reviewed corrective work unit for
`62C6A469`. The correction must distinguish package-wide Windows test-runtime
cost from a genuinely blocked fixture before changing fixture constructors,
test partitioning, or timeout policy.

Do not rerun `go test ./...` without a new explicit authorization after the
governed correction.
