---
chunk_strategy: h1-h2-h3
description: Closure record for version update check feature 092-F.
doc_type: closure
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-10-version-update-check-closure.md
title: Version Update Check Closure
---

## Summary

Feature `092-F` delivered stash `3C09D8A7`: `backlogit version`, root
`backlogit --version`, and `backlogit_get_version` now report the installed
version, latest GitHub release, update availability, and a bounded update-check
state. The reusable `internal/release` package performs the stdlib-only GitHub
latest-release lookup and strict SemVer comparison. The remote check uses a
short timeout, honors `GITHUB_TOKEN` for rate limits, and degrades to installed
version output when offline, slow, rate limited, malformed, or skipped.

## Changed surfaces

| Surface | Closure note |
|---|---|
| `internal/release/` | Added latest-release client, SemVer comparison, update availability status, and hermetic tests |
| `internal/cli/version_cmd.go` and `internal/cli/root.go` | Added latest/update output, JSON fields, `--no-update-check`, env skip, and root `--version` wiring |
| `internal/mcp/version_tool.go` | Mirrored version fields for agents with env, server, and per-call skip controls |
| `.autoharness/backlog-registry.yaml` | Changed `get_version` fallback to `backlogit version --format json` for structured CLI/MCP parity |
| `docs/cli-reference/` | Regenerated command reference after CLI help and inherited flag changes |
| `.backlogit/` | Feature `092-F` and tasks `092.001-T` through `092.003-T` were completed and archived |

## Validation

* TDD red phase: new release, CLI, root-version, MCP, and contract tests were
  written first and failed before implementation.
* Green phase: targeted tests passed after implementation and review fixes.
* `go test ./...` passed after the final code changes.
* `go build ./cmd/backlogit` passed.
* `go vet ./...` passed.
* `golangci-lint run` passed.
* `gofmt -l` on changed Go files returned empty output.
* `gofmt -l .` was previously run and returned pre-existing repository-wide
  formatting output across unchanged files. This closure keeps that as a known
  repository condition instead of widening this feature into a formatting PR.
* PR #209 CI passed on final head `c4b071081847e3930d376ecfe7ccce080da87be4`:
  * `Detect code changes`
  * `test`
  * `Docline frontmatter gate`
  * `CLI Reference Drift`

## Review record

* `LOCAL_REVIEW_READY`: the mandatory review skill ran before PR creation and
  found no remaining P0/P1 blockers after remediation.
* Hosted Copilot review raised SemVer validation, MCP parity, timeout,
  documentation, and test-hermeticity comments. Each actionable comment was
  fixed, replied to, and resolved through GraphQL review-thread resolution.
* A post-fix local review pass found MCP skip-control and registry fallback
  parity gaps. These were fixed in `c4b071081847e3930d376ecfe7ccce080da87be4`.
* The `run 'backlogit update'` hint was retained because it was an explicit
  acceptance criterion for this unit and is intended for the companion update
  unit. No self-update implementation was added in this scope.
* Final Copilot review covered current head
  `c4b071081847e3930d376ecfe7ccce080da87be4`; there were zero unresolved
  Copilot threads before merge.

## Merge and release record

* Feature branch: `feat/version-update-check`
* Feature PR: #209
* Reviewed HEAD: `c4b071081847e3930d376ecfe7ccce080da87be4`
* Merge commit: `7b0e5db430150ff48b8b7243c362531c91fa46b6`
* Merge mode: normal `gh pr merge 209 --merge`; admin fallback was not needed.
* P-009 preserved: merge commit was used; no squash or rebase merge was used.
* Dark-mode authority: `merge_approval_pre_authorized=true` and
  `admin_fallback_pre_authorized=true`; only normal merge was exercised.

## Monitoring and rollback

| Evidence | Value |
|---|---|
| Signals / queries | `backlogit version`, `backlogit --version`, `backlogit version --format json`, `backlogit_get_version`, PR #209 GitHub Actions |
| Baseline | Version output previously showed only installed/build metadata and MCP returned only build fields |
| Healthy threshold | Version commands return quickly; skipped/offline checks exit zero; JSON and MCP include `current`, `latest`, `update_available`, and `update_check` |
| Alert threshold | Version output hangs, exits non-zero on update-check failure, MCP performs an unskippable network call, or CLI/MCP fields drift |
| Owner | Repository maintainer during the next release and update-command unit |
| Observation window | Next release tag and the companion `backlogit update` unit |
| Rollback trigger | Confirmed CLI/MCP version regression, unbounded network wait, or incorrect update availability for valid SemVer tags |
| Rollback procedure | Revert merge commit `7b0e5db430150ff48b8b7243c362531c91fa46b6`, rerun `go test ./...`, and restore the previous version output behavior |

## Backlog state

* `092-F` archived
* `092.001-T` archived
* `092.002-T` archived
* `092.003-T` archived
* `backlogit sync` indexed 796 artifacts after completion moves and after
  archive relocation
