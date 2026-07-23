---
title: backlogit update command dark-mode halt memory
type: session-memory
date: 2026-07-10
feature_id: 093-F
stash_id: 3762BC38
status: halted
---

## Summary

Dark factory Ship work for stash `3762BC38` halted before PR creation. The
implementation reached green local gates, but local adversarial concurrency
review retained a P1 blocker: the Windows-compatible rename-current then
move-new update path leaves the target executable absent between renames, so
crash-safe atomic replacement cannot be proven.

## Work items

* Feature `093-F`: Add self-update CLI command, blocked
* Task `093.001-T`: Implement self-update release helpers, blocked
* Task `093.002-T`: Write self-update harness tests, blocked
* Task `093.003-T`: Wire update command and atomic replace, blocked

## Files changed but not committed

* `internal/release/release.go`
* `internal/release/self_update_test.go`
* `internal/cli/update.go`
* `internal/cli/self_update.go`
* `internal/cli/update_self_test.go`
* `.backlogit/queue/093-F.md`
* `.backlogit/queue/093.001-T.md`
* `.backlogit/queue/093.002-T.md`
* `.backlogit/queue/093.003-T.md`
* `.backlogit/stash.jsonl`
* `.backlogit/archive/stash.jsonl`
* `.backlogit/hooks_queue.jsonl`

## Verification completed

* Red phase confirmed with `go test ./internal/release ./internal/cli`
* Targeted tests passed: `go test ./internal/release ./internal/cli`
* Full tests passed: `go test ./...`
* Build passed: `go build ./cmd/backlogit`
* Vet passed: `go vet ./...`
* Lint passed: `golangci-lint run`
* Changed-file formatting passed

## Review outcome

Security, Go, scope, and learnings review findings were addressed or cleared.
Final concurrency re-review retained this P1 blocker:

```text
replaceSelfUpdateBinary still renames the current binary to .old before moving
the new binary into place, leaving the target path absent between the two
renames. A crash/power loss or concurrent invocation during this window can
observe or leave no executable at the target path, so the replacement is not
atomic.
```

## Next steps

* Decide whether to redesign self-update around a helper, deferred installer,
  package-manager update, or explicit manual install path
* Resume from branch `feat/backlogit-update-command` only after the atomicity
  requirement is clarified or changed
* Do not open a PR from the current diff while the P1 blocker remains
