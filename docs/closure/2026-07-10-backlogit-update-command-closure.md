---
chunk_strategy: h1-h2-h3
description: Closure record for backlogit update command feature 093-F.
doc_type: closure
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-10-backlogit-update-command-closure.md
title: Backlogit Update Command Closure
---

## Summary

Feature `093-F` shipped `backlogit update` as an explicit self-update command.
The command checks GitHub Releases for the latest or requested tag, downloads
the platform-matched asset, verifies it against `SHA256SUMS`, and replaces the
running binary on disk without executing the downloaded asset.

## Changed surfaces

| Surface | Closure note |
|---|---|
| `internal/release/` | Added release-by-tag lookup, asset enumeration, asset download, and `SHA256SUMS` parsing |
| `internal/cli/update.go` | Added no-argument self-update mode while preserving artifact update behavior |
| `internal/cli/self_update.go` | Added update orchestration, checksum verification, same-directory temp writes, replacement, rollback, and manual-install failures |
| `internal/cli/self_update_lock_*.go` | Added OS-backed update lock implementations for supported platforms |
| `docs/cli-reference/` | Regenerated command reference for `backlogit update` |
| `.backlogit/` | Archived feature `093-F` and tasks `093.001-T` through `093.003-T`; added follow-up `094-F` |

## Validation

* TDD red phase: release helper and update command tests were written before implementation and initially failed
* Green phase: tests passed after the implementation and review fixes
* Local gates passed: `go build ./cmd/backlogit`, `go test ./...`, `go vet ./...`, `golangci-lint run`, and `gofmt -l .`
* CI passed on PR #211 after commit `d2a1bba9eaf51f1e71b8f711477362c6de612f29`
* Local adversarial review reached `READY_WITH_FOLLOWUPS`; `094-F` tracks the accepted Windows residual crash window
* Copilot review covered `d2a1bba9eaf51f1e71b8f711477362c6de612f29` with zero unresolved Copilot threads before merge

## Release readiness

* SHA256 verification fails closed when checksums are missing, malformed, or mismatched
* Asset names are constrained to `backlogit-{os}-{arch}` with `.exe` only on Windows
* Downloads use a bounded timeout and size limit
* Temp files are written in the target directory for same-volume replacement
* The self-update lock is OS-backed on supported platforms
* Unwritable targets fail before replacement and print manual install guidance

## Rollback and limitations

Unix replacement uses one same-volume rename from the verified temp file to the
target path, so the target path has no absent window.

Windows cannot rename over a running `.exe`. The implementation uses the
standard two-step pattern: rename the current executable to `.old`, rename the
verified temp file into place, and roll back `.old` to the original target if
the second rename fails. The two metadata calls create a short crash window, but
`.old` remains recoverable and stale `.old` cleanup is attempted on later update
runs. This residual risk is tracked by follow-up `094-F`.

## Merge and closure

* Main PR: #211
* Merge strategy: normal merge commit
* Merge commit: `e076255b8521668c7cb2067d5f86b556fb71a87e`
* Stash: `3762BC38` was harvested and archived
* Scope note: `--source` mode was omitted because it was optional and not needed for the release-grade binary update path
