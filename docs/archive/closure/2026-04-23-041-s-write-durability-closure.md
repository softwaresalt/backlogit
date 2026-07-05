---
chunk_strategy: h1-h2-h3
description: Post-merge closure artifact for shipment 041-S covering release readiness, monitoring, and rollback plan
doc_type: closure
docline:
    author: backlogit ship agent
    ms.date: 2026-04-23T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-04-23-041-s-write-durability-closure.md
title: Operational Closure — 041-S Write Durability and Hook Reliability
---

## Release Summary

| Field | Value |
|---|---|
| Shipment | 041-S — Write Durability and Hook Reliability |
| Feature | 040-F |
| Merge SHA | `1b92794` |
| PR | [#58](https://github.com/softwaresalt/backlogit/pull/58) |
| Merged | 2026-04-23 |
| Mode | Merge-only (single binary; no migration, no infra change) |
| Review Gate | PASS — 040.001-R; 0 P0/P1; 15 Copilot comments resolved across 2 rounds |
| CI | ✅ test (1.23), test (1.24), CLI Reference Drift |

## Change Summary

Added fsync-before-close/rename durability to all JSONL write paths and fixed
a TOCTOU stale-lock race in the hook event queue. Changes are confined to
background write helpers — no public API surface, schema, or configuration
changed.

Affected files:

- `internal/events/fsutil.go` — new `syncAppendLine` / `syncWriteFileAtomic` helpers
- `internal/events/hook_events.go` — wired `syncAppendLine`; rename-based lock claim
- `internal/events/hook_checkpoint.go` — wired `syncWriteFileAtomic`
- `internal/telemetry/checkpoint.go` — inline fsync + short-write guard
- `internal/telemetry/harvest.go` — inline fsync + Windows pre-remove gate
- `.gitignore` — `.telemetry-checkpoint.json` untracked

## Invariants to Preserve

1. **JSONL append durability** — every `AppendHookEvent` call must survive an OS
   crash or power failure occurring after the call returns successfully.
2. **Atomic checkpoint replacement** — a reader must never observe a partially
   written checkpoint file; the old file must remain valid until the new one is
   fully flushed and renamed into place.
3. **TOCTOU-safe lock recovery** — two concurrent processes performing stale-lock
   recovery must not both succeed; exactly one must claim the `.recovering` file
   via an atomic `Rename(lock → lock.recovering)`.
4. **No short writes** — `f.Write` returning `n < len(data)` with a nil error
   must be treated as an error; partial writes must not be fsynced and renamed.
5. **Cross-platform rename** — `os.Rename` on Windows must be preceded by
   `os.Remove(dest)` gated on `runtime.GOOS == "windows"`; POSIX must not
   pre-remove (POSIX rename is natively atomic).

## Pre-Deploy Audit Checklist

- [x] No schema migrations required
- [x] No config file changes required
- [x] No new environment variables or feature flags
- [x] No external service dependencies introduced
- [x] `go test ./...` passes on Go 1.23 and 1.24
- [x] `golangci-lint run` clean
- [x] All 15 Copilot review threads resolved before merge
- [x] `.telemetry-checkpoint.json` removed from git tracking (was machine-local state)

## Deployment Path

Merge-only. The change ships as part of the next `go install` or binary download.
No maintenance window, migration, or rollout gate required.

## Post-Deploy Checks

1. Run `backlogit mcp` and append a hook event via a test client; confirm the
   events file is non-empty and the line is valid JSON.
2. Run `backlogit telemetry harvest` against a known log directory; confirm
   `telemetry-sessions.jsonl` is created and contains valid records.
3. Confirm no `.tmp` residue files in `.backlogit/` after a successful harvest or
   checkpoint write.
4. On Windows: confirm `os.Rename` succeeds over an existing checkpoint file
   without `ERROR_ALREADY_EXISTS` (the pre-remove gate covers this).

## Risky Action Record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Rename-based TOCTOU lock claim (replaces Remove+retry) | moderate — changes concurrent lock recovery behavior | Implicit in review gate PASS | applied |
| `runtime.GOOS == "windows"` gate on pre-remove | low — POSIX loses no atomicity; Windows keeps compatibility | Copilot comment addressed | applied |
| Untrack `.telemetry-checkpoint.json` | low — file stays on disk; only removed from git | User merge approval | applied |

## Source Artifact Cleanup

040-F `custom_fields` contained only `harness_status: pending` — no
`source_stash_id` or `source_deliberation_id` was set. No stash entries or
deliberation artifacts required cleanup for this shipment.

## Healthy Signals

- No `syncAppendLine`, `syncWriteFileAtomic`, or `SaveCheckpoint` errors in slog output
- Hook event queue files grow monotonically; no truncation observed
- Telemetry harvest produces valid JSONL with correct record counts
- No duplicate events in hook queue after concurrent lock recovery
- No `.tmp` residue files left in `.backlogit/` after normal operation

## Failure Signals

- `syncAppendLine sync` or `syncWriteFileAtomic sync` error in slog → fsync failing (storage issue)
- Corrupted JSONL line (incomplete JSON or two records on one line) → short write not caught
- Missing checkpoint file after restart → crash occurred between Remove and Rename on Windows
- Two lock recovery sessions both succeed → TOCTOU regression
- Duplicate events in hook queue → concurrent write succeeded when it should have been blocked

## Monitoring Plan

| Signal | Where | Action |
|---|---|---|
| slog error lines matching `syncAppendLine\|syncWriteFileAtomic\|SaveCheckpoint` | stderr / log file | Investigate immediately; likely storage error |
| `.tmp` files persisting in `.backlogit/` | `ls .backlogit/*.tmp` | Indicates aborted write; safe to remove manually |
| Duplicate JSONL records | `jq -c 'del(.seq, .timestamp)' .backlogit/hooks_queue.jsonl \| sort \| uniq -d` | Indicates TOCTOU regression; detects duplicates by stable payload identity |
| Hook queue file size decreasing | filesystem monitor | Indicates truncation; check for concurrent writers |

No new dashboard or alert infrastructure required — this is a background write
reliability improvement with no external observability surface.

## Rollback Trigger

Any of:
- Corrupted hook queue file (invalid JSON or duplicate events) observed after release
- Checkpoint file missing after clean restart
- `sync` error rate non-zero in slog

## Rollback Procedure

```bash
# Revert the merge commit on main
git revert 1b92794 --no-edit
git push origin main

# Reinstall previous binary
go install github.com/softwaresalt/backlogit/cmd/backlogit@<prior-tag>
```

The revert restores the previous write paths. Existing queue files remain valid
(append-only JSONL is backward compatible). No data migration required.

## Validation Window

**Duration**: 7 days from merge date (2026-04-23 → 2026-04-30)
**Owner**: softwaresalt
**Method**: Passive observation — no new monitoring infrastructure needed.
Watch slog output during normal MCP server and telemetry harvest sessions.

## Readiness Status

**READY** — merge proceeded with CI green and all review threads resolved.
No blocking conditions identified. Monitoring plan is passive (slog observation)
as no runtime surface or external contract changed.
