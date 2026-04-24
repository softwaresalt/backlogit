---
title: "043-S Doctor Completion & Telemetry Parser — Post-Merge Closure"
description: "Operational closure for PR #64: doctor orphan fix, CLI doctor command, and telemetry parser multi-line format support"
ms.date: 2026-04-23
shipment: 043-S
merge_sha: afe1a16
pr: "https://github.com/softwaresalt/backlogit/pull/64"
status: READY
---

## Summary

Shipment 043-S shipped three tasks across two features via PR #64 (merged `afe1a16`):

| Task | Feature | Description |
|---|---|---|
| 043.001-T | 043-F | Fix doctor false-positive orphan detection for legacy root artifacts |
| 043.002-T | 043-F | Wire `backlogit doctor` CLI command |
| 044.001-T | 044-F | Update telemetry parser for Copilot multi-line log format |

Two rounds of Copilot review comments were addressed before merge. All CI checks were green throughout.

## Invariants to Preserve

- `backlogit doctor` returns no false positives for valid root-level legacy artifacts (those with IDs like `001-T` having no dot-separated parent prefix)
- Orphan detection must not regress for genuinely orphaned child tasks (level ≥ 2 with no parent in the workspace)
- Duplicate-ID detection continues to function independently of the orphan fix
- Telemetry parser must continue to parse old single-line `[telemetry]` format records (backward compatibility)
- `backlogit_telemetry_harvest` MCP tool must continue to function with both log formats

## Changed Runtime Surfaces

| Surface | Change | Risk |
|---|---|---|
| `internal/core/doctor.go` | `levelFromID()` replaces always-false `info.level == 1` guard | Moderate — behavior change in orphan check; was silently broken before |
| `internal/cli/doctor.go` | New CLI command wired into `root.go` | Low — additive only |
| `internal/telemetry/parser.go` | Multi-line JSON accumulation for new Copilot log format | Moderate — parser path changed; backward compatible |
| `README.md` | Added doctor command to Features and Quick Start | Low — documentation only |

## Pre-Deploy Audit

- [x] All quality gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
- [x] No schema migrations; no data shape changes
- [x] No new external dependencies introduced
- [x] Backward compatibility verified: old `[telemetry]` log format still parsed correctly
- [x] `backlogit doctor` produces no false positives on the repository's own `.backlogit/` workspace (verified via test suite covering legacy fixtures without `level:` frontmatter)
- [x] No feature flags or rollout gates required — purely additive CLI capability plus bug fix

## Deployment / Rollout Path

Merge-only. No deploy step. Consumers obtain the fix by running:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
```

## Post-Deploy Checks

1. Run `backlogit doctor` on a workspace that includes legacy root artifacts (e.g., `001-T`) — must report no orphans
2. Run `backlogit doctor --format json` — must return valid JSON with `checked_at`, `orphans`, and `duplicates` fields
3. Run `backlogit telemetry harvest` with a Copilot log directory containing multi-line format entries — must parse `assistant_usage` and `tool_call_executed` events correctly
4. Run `backlogit telemetry harvest` against old single-line `[telemetry]` format — must continue to succeed (backward compatibility)

## Risky Action Record

| Action | ActionRisk | ActionResult | Notes |
|---|---|---|---|
| Replace `info.level == 1` orphan guard with `effectiveLevel` + `levelFromID()` | moderate | applied | Was a silent bug (guard never fired); fix makes orphan detection actually work for real archive files |
| Telemetry parser format detection | moderate | applied | New brace-depth accumulation path is additive; old path retained for backward compatibility |
| `backlogit doctor` CLI command wired into root | low | applied | Additive; no existing commands changed |

## Source Artifact Cleanup

| Stash ID | Feature | State at Closure |
|---|---|---|
| `CCAEF17B` | 043-F | Already `harvested` before Ship engagement; cannot be removed via `backlogit_stash_remove` (tool only operates on `active` entries). No further action needed. |
| `9F5ACF94` | 044-F | Already `harvested` before Ship engagement; same as above. |

No deliberation artifacts existed for 043-F or 044-F (`source_deliberation_id` not populated in either feature's `custom_fields`).

## Healthy Signals

- `backlogit doctor` exits 0 with no orphan or duplicate findings on a clean workspace
- `backlogit telemetry harvest` processes both log formats without error or data loss
- CI on `main` remains green after merge
- No regressions in existing `doctor_test.go` or `parser_test.go` test coverage

## Failure Signals

- `backlogit doctor` reports orphans for every root-level task in a workspace → `levelFromID()` regression; roll back to prior binary
- `backlogit telemetry harvest` silently drops events from new-format logs → parser detection or accumulation logic regressed
- `backlogit doctor --format json` panics or produces malformed JSON → output marshaling regression

## Monitoring Plan

This is a CLI-only tool; there is no persistent runtime service to monitor. Manual observation is appropriate.

| Signal | How to Observe |
|---|---|
| Doctor correctness | Run `backlogit doctor` on a workspace with known-clean legacy artifacts; verify zero false positives |
| Telemetry parse coverage | Compare parsed event count to raw log line count after `backlogit telemetry harvest` |
| Backward compatibility | Run harvest against an old-format log file; confirm event count matches expectations |

No alert rules required. Review during next session if user-reported issues emerge.

## Rollback Trigger

Any of the following conditions justify rolling back to the previous binary (`go install` of prior tag or commit):

- Orphan detection returns false positives for root artifacts after upgrade
- Telemetry harvest produces fewer events than the previous version with the same log file
- `backlogit doctor` crashes or produces invalid JSON output

## Rollback Procedure

```bash
# Roll back to the commit immediately before this merge
go install github.com/softwaresalt/backlogit/cmd/backlogit@<prior-tag-or-sha>
```

The prior commit SHA before this shipment is the parent of `afe1a16`. The `levelFromID()` function can be reverted by removing it from `internal/core/doctor.go` and restoring the original `info.level == 1` guard (which was effectively a no-op for all real archive files, so reverting is safe if needed).

## Validation Window

- **Duration**: 1 sprint (observe during next active use of `backlogit doctor` or `backlogit telemetry harvest`)
- **Owner**: softwaresalt

## Readiness Status

**READY** — all CI gates green, two rounds of Copilot review comments addressed and resolved, merge approved by user, shipment archived. No deployment gates required. README updated with doctor command documentation.
