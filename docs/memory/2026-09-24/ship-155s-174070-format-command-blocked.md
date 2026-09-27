---
title: Ship 155-S 174.070-T Format Command Blocked
date: 2026-09-24
status: blocked
shipment: 155-S
task: 174.070-T
branch: feat/155-s-s14-resumable-shipment-blocked-lifecycle-status
head: 88b821b0c0cd909c3d6a211eaeeec5e88fd1ead7
---

## Outcome

The reviewed one-line `174.070-T` correction was applied to
`internal/core/shipment_shipped_event_durability_test.go`. The targeted
durability suite and the branch-wide CI-pinned lint gate passed.

The scoped formatting verification command then failed to parse in PowerShell
before it inspected the file. Per the ordered no-retry instruction, validation
stopped immediately.

## Passing evidence

The targeted durability selector passed:

```text
go test ./internal/core -run '^TestShipShipment_(DurabilityFixtureUsesFailOpenFakeGate|ProvenNotAppliedShippedEventCompensates|IndeterminateShippedEventNeverRollsBack|UnrestorableItemReportsPartialCompensation|FailClosedShippedAppendSuppressesMoveStatusPostHook|ShippedEventAppendFailureLogsFixedShape)$' -count=1
```

The Go `1.24.0` and `golangci-lint v1.64.8` new-change gate passed with no
findings:

```text
golangci-lint run --timeout=5m --new-from-rev 37a5cba4713953c855013fafd4fc05e479e5618c ./internal/core/...
```

## Blocking evidence

The LF-normalized scoped formatting command exited `1` with a PowerShell
parser error:

```text
Variable reference is not valid. ':' was not followed by a valid variable
name character. Consider using ${} to delimit the name.
```

The failure occurred in command construction before formatting verification
ran. It is not evidence that the Go file is misformatted.

## Current state

* `174.070-T` remains active and uncommitted
* `174.069-T` remains active with its prior uncommitted fixture changes
* `155-S` remains active
* `154-S` remains queued and unmodified
* Current HEAD remains `88b821b0c0cd909c3d6a211eaeeec5e88fd1ead7`
* `git diff --check`, task-delta confirmation, build, commits, and task
  completion were not performed
* `go test ./...` remains unauthorized and was not run

## Next action

Obtain authorization for one corrected scoped-format verification command,
followed by `git diff --check` and exact one-line task-delta confirmation. If
those pass, commit and complete `174.070-T`, then resume the remaining
`174.069-T` build and scoped-format gates.
