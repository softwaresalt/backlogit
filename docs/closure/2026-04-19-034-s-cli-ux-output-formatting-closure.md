---
title: "034-S CLI UX & Output Formatting — Post-Merge Closure"
description: "Release-readiness, monitoring, and rollback record for Shipment 034-S (Feature 033-F). PR #43 merged to main on 2026-04-20."
ms.date: 2026-04-19
---

## Closure Summary

* **Shipment**: 034-S — CLI UX & Output Formatting
* **Feature**: 033-F
* **PR**: [#43](https://github.com/softwaresalt/backlogit/pull/43) merged to `main` on 2026-04-20T05:02:01Z
* **Merge SHA**: `fea611f1bb29ab8e0da4232689dfd574e50db4f9`
* **Branch**: `shipment/034-b-cli-ux-output-formatting`
* **Mode**: post-merge

## Change Summary

Introduced a pluggable output formatter system for the backlogit CLI. Added `--format` flag to all list-style commands (`list`, `queue`, `stash`, `shipment`, `get`, `version`). Three renderers: `table` (default), `json`, and `tile`. Formatter system lives in `internal/cli/format/` as a standalone package. CLI reference regenerated for all 50+ commands via a now-deterministic gen-docs pipeline.

## CI Status at Merge

| Check | Result |
|---|---|
| CI/test (1.23) | ✅ PASS |
| CI/test (1.24) | ✅ PASS |
| CLI Reference Drift | ✅ PASS |

All 16 Copilot review threads resolved before merge.

## Invariants to Preserve

* `--format table` MUST remain the default for all list commands; no behavior change for users not passing `--format`
* `--format json` MUST produce valid JSON (array of objects) for all list commands
* `--format tile` MUST produce blank-line-separated property:value blocks
* `backlogit stash list --group-by-priority` MUST reject `--format` values other than `json` (or empty)
* `backlogit version --format json` MUST output structured JSON with `version`, `commit`, and `buildDate` keys
* CLI reference docs MUST remain in sync with binary output — validated by the drift check CI gate

## Pre-Deploy Audits

This is a merge-only release (no deployment surface beyond local binary install). Pre-deploy checks:

* [x] All format variants validated by unit tests in `internal/cli/format/`
* [x] `validateFormat` helper tested across table/json/tile values
* [x] `go test -race ./...` passes on both Go 1.23 and 1.24
* [x] No database schema changes
* [x] No `.backlogit/` workspace file format changes
* [x] No MCP tool schema changes
* [x] CLI reference deterministic (gen-docs drift check passes)
* [x] Makefile ldflags inject `VERSION`, `COMMIT`, `DATE` at build time

## Deployment Path

Merge-only. No deployment infrastructure. Users obtain the updated binary via:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
```

Or by building from source:

```bash
make build  # produces ./bin/backlogit with version/commit/date ldflags injected
```

## Post-Deploy Checks

Run after installing the updated binary:

```bash
# Smoke: table format (default)
backlogit list

# Smoke: JSON output
backlogit list --format json

# Smoke: tile output
backlogit list --format tile

# Smoke: version with JSON
backlogit version --format json

# Smoke: stash group-by-priority (JSON only)
backlogit stash list --group-by-priority

# Negative: invalid format should error
backlogit list --format csv  # must return non-zero exit with clear error message
```

## Risky Action Record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Add `--format` to `get` command | low | auto | applied — table and json only (no tile) |
| stash `--group-by-priority` format guard | low | auto | applied — returns error for non-json format |
| gen-docs frontmatter change (`ms.date` removed) | moderate | auto | applied — drift check now deterministic |
| Makefile ldflags wiring | low | auto | applied — VERSION/COMMIT/DATE injected on build+install |

No destructive actions. No migrations. No external integrations changed.

## Healthy Signals

* `backlogit list --format json` exits 0 and emits a valid JSON array
* `backlogit list` (no flag) shows the same table output as before this change
* `backlogit version --format json` includes `version`, `commit`, and `buildDate` fields
* `make build` produces a binary where `backlogit version` shows the git tag and short SHA
* `go run ./cmd/gen-docs docs/cli-reference && git diff --exit-code docs/cli-reference/` exits 0

## Failure Signals

* Any list command producing malformed JSON when `--format json` is passed
* `--format table` producing blank output for non-empty result sets
* `backlogit stash list --group-by-priority --format table` NOT returning an error
* `backlogit version` showing `dev` for `Version` when installed from a tagged release (indicates ldflags not wiring correctly)
* CLI reference drift check failing after a `go generate` or `make docs` run

## Monitoring Plan

This is a CLI tool with no server surface. Monitoring is community-driven:

* Watch GitHub Issues for reports of broken `--format` output
* Watch CI on `main` for drift check regressions after future CLI changes
* Validate drift check passes on every PR that touches `internal/cli/` or `cmd/gen-docs/`

No dashboards, no alert thresholds, no SLIs beyond CI gates.

## Rollback Trigger

If users report that `--format json` produces malformed output or that the default table output regressed, rollback by reverting the merge commit:

```bash
git revert fea611f1bb29ab8e0da4232689dfd574e50db4f9 --mainline 1
```

## Rollback Procedure

1. `git revert --no-commit fea611f1bb29ab8e0da4232689dfd574e50db4f9 --mainline 1`
2. `git commit -m "revert: roll back 034-S CLI UX output formatting"`
3. Push to a branch, open a PR, merge after CI passes
4. Announce rollback in project channels
5. Create a new backlog task to re-implement with the identified fix

## Validation Window

CLI changes are low-risk and merge-only. Validation window: **7 days** from merge date (2026-04-20 to 2026-04-27).

**Owner**: softwaresalt

## Post-Merge Repository Changes

These uncommitted changes were present in the working tree at closure time and are committed together with this closure artifact as the shipment bookkeeping commit.

### Backlogit Workspace State Transitions

The `backlogit shipment ship` command moved all 034-S and 033-F artifacts from `queue/` to `archive/`:

| File | Change |
|---|---|
| `.backlogit/queue/034-S.md` | deleted → `.backlogit/archive/034-S.md` (new) |
| `.backlogit/queue/033-F.md` | deleted → archived |
| `.backlogit/queue/033.001-T.md` through `033.009-T.md` | deleted → archived |
| `.backlogit/queue/033.001-R-branch-review-shipment-034-b-cli-ux-output-formatting.md` | deleted → archived |
| `.backlogit/queue/033.010-T.md` | deleted → `.backlogit/archive/033.010-T.md` (archived) |
| `.backlogit/queue/033.011-T.md`, `033.012-T.md` | remain in queue — follow-up tasks, not archived |
| `.backlogit/queue/036-DL.md` | deleted → `.backlogit/archive/036-DL.md` (new) |
| `.backlogit/archive/033.006-T.md` through `033.009-T.md` | deleted (superseded by fresh archive writes from ship) |
| `.backlogit/hooks_queue.jsonl` | modified — hook events from shipment lifecycle |
| `.backlogit/archive/034-S.md` | created — archived shipment with merge SHA recorded |
| `.backlogit/archive/036-DL.md` | created — archived deliberation artifact |

### Documentation Created

| File | Purpose |
|---|---|
| `docs/closure/2026-04-19-034-s-cli-ux-output-formatting-closure.md` | This closure artifact |
| `docs/memory/20260419-204036-ship-034-s-pr-green-awaiting-merge.md` | Session memory — PR green, awaiting merge |
| `docs/memory/20260419-214846-ship-034-s-pr-merge-ready.md` | Session memory — final pre-merge state |

### Other Changes

| File | Change |
|---|---|
| `.gitignore` | Added `.env`, `.env.*`, `!.env.example`, `.mcp.json` exclusions |

Note: `.mcp.json` was present in the working tree (local MCP server configuration with developer-specific paths including `D:/Tools/engram.exe`) and has been added to `.gitignore` rather than committed. Developer-specific config files with hardcoded local paths must not be tracked in the repository.

## Follow-Up Backlog Tasks

| ID | Title | Status |
|---|---|---|
| 033.011-T | Move cli-reference-drift workflow permissions to job level | queued |
| 033.012-T | TileRenderer TTY detection (auto-suppress tile on non-TTY) | queued |

## Readiness Status

**READY** — Merge completed. No deployment prerequisites. CLI binary available via `go install`. Validation window open through 2026-04-27.
