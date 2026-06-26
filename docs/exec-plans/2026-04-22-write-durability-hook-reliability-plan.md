---
chunk_strategy: h1-h2-h3
description: Implementation plan for crash-durable writes and TOCTOU race fix across hook event, checkpoint, and telemetry subsystems
doc_type: plan
docline:
    date: 2026-04-22T00:00:00Z
    origin: .backlogit/queue/040-F.md
    status: reviewed
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-22-write-durability-hook-reliability-plan.md
title: Write Durability and Hook Reliability
---

## Write Durability and Hook Reliability

## Problem Frame

Backlogit uses a temp-file-then-rename pattern for atomic writes and `O_APPEND` for queue appends across three subsystems: hook events, hook checkpoints, and telemetry harvest. None of these write paths call `fsync` before closing or renaming, which means data reported as successfully written can be lost on OS crash or power failure. The kernel page cache may not have flushed to stable storage at the time of close or rename.

Additionally, the hook event queue's stale-lock recovery has a TOCTOU race: two processes can simultaneously detect a stale lock, remove it, and both proceed to write to the queue file concurrently, risking interleaved or corrupted JSONL records.

The scope is limited to the four files that service these three subsystems. The broader codebase has approximately 20 additional temp-file-then-rename sites without fsync, but those are out of scope for this feature.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Hook event queue appends must be durable before returning success | 040.001-T (stash BDF63CA2) |
| R2 | Hook checkpoint temp files must be synced before rename | 040.002-T (stash 6898DB32) |
| R3 | Telemetry harvest JSONL temp files must be synced before rename | 040.003-T (stash 65941061) |
| R4 | Telemetry harvest checkpoint must also be synced (same package, same pattern) | 040.003-T scope extension |
| R5 | Stale-lock recovery must not permit concurrent writers through the TOCTOU gap | 040.004-T (stash ED8C5707) |
| R6 | All changes must pass existing tests plus new durability-focused regression tests | Constitution: Test-First Development |
| R7 | No new external dependencies | Constitution: Single-Binary Simplicity |

## Scope Boundaries

### In Scope

* `internal/events/hook_events.go`: fsync on queue append (R1), TOCTOU race fix (R5)
* `internal/events/hook_checkpoint.go`: fsync before rename (R2)
* `internal/telemetry/harvest.go`: fsync before close/rename in `writeTelemetryJSONL` (R3)
* `internal/telemetry/checkpoint.go`: fsync before rename in `SaveCheckpoint` (R4)
* New tests in colocated `_test.go` files for all durability fixes
* A shared `syncWrite` helper to consolidate the fsync-before-rename pattern

### Non-Goals

* Fixing the other ~20 temp-file-then-rename sites across `internal/core/` and `internal/cli/`. Those are a separate systemic issue and would be addressed by a future codebase-wide sweep using the helper introduced here.
* Replacing the sidecar lock pattern with OS-level file locking (`flock`/`LockFileEx`). The existing advisory lock pattern with TTL-based stale recovery is validated by compound learnings and is adequate once the TOCTOU gap is closed.
* Adding directory fsync after file creation. While ideal for full POSIX durability guarantees, Go's stdlib does not make this ergonomic, and the existing rename pattern is sufficient for the corruption scenarios we are fixing.

### Deferred to Implementation

* Exact placement of the shared helper package (proposed: `internal/fsutil/` or inline in each package). The implementer should evaluate whether the three call sites justify a shared package or if inline helpers are simpler.
* Whether `f.Sync()` errors should be wrapped with a new sentinel error or reuse existing `backlogiterrors.ErrConfig`. The implementer should decide based on the error handling conventions in each package.

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort: fewer than 3 files modified, fewer than 5 functions changed, fewer than 4 test scenarios.

### Unit 1: Introduce syncWriteFile helper

**Files:** `internal/events/fsutil.go` (new)
**Test files:** `internal/events/fsutil_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** The existing temp-file-then-rename pattern in `internal/events/hook_checkpoint.go:89-103` and `internal/core/artifacts.go:264-269`, extended with an `f.Sync()` call before close
**Dependencies:** none

**Approach:**

Introduce two focused helpers in a new file within `internal/events/`:

1. `syncAppendLine(path string, data []byte) error` — Opens a file with `O_CREATE|O_APPEND|O_WRONLY`, writes data, calls `Sync()`, then closes. For use by hook event queue appends.

2. `syncWriteFileAtomic(path string, data []byte, perm os.FileMode) error` — Creates a temp file (`path + ".tmp"`), writes data, calls `Sync()`, closes, then renames over the target. For use by checkpoint writes.

Placing these in `internal/events/` rather than a separate `internal/fsutil/` package avoids a new package for only two call sites in this feature. If the future codebase sweep introduces more consumers, the helpers can be promoted to a shared package at that time.

On Windows, `os.Rename` fails when the destination already exists. The helper must `os.Remove` the destination before renaming, matching the existing pattern in `internal/telemetry/checkpoint.go:81`.

**Verification:**
* `syncAppendLine`: test appends data, reads it back, confirms correct content and trailing newline handling
* `syncWriteFileAtomic`: test writes data, reads it back, confirms no `.tmp` file remains after success
* Error path: test that a failed write to an unwritable directory returns a wrapped error
* `go test ./internal/events/...` passes

### Unit 2: Add fsync to hook event queue writes (040.001-T)

**Files:** `internal/events/hook_events.go`
**Test files:** `internal/events/hook_events_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Unit 1's `syncAppendLine` helper
**Dependencies:** Unit 1

**Approach:**

Replace the manual `os.OpenFile` → `Write` → `Close` sequence at lines 101-109 with a call to `syncAppendLine`. This eliminates the deferred `qf.Close()` on line 105 in favor of the helper's explicit sync-then-close sequence.

The change is mechanical: the write path currently opens with `O_CREATE|O_APPEND|O_WRONLY`, writes the marshaled JSON line, and closes. The helper does the same with an added `Sync()` between write and close.

**Verification:**
* Existing tests `TestHookEventWriter_AppendHookEvent_PersistsToJSONL` and `TestHookEventWriter_AppendHookEvent_ConcurrentAccess_UniqueSeqs` continue to pass
* New test: `TestHookEventWriter_AppendHookEvent_DurableWrite` — append an event, confirm the file contains the event, confirm no data loss on re-read
* `go test ./internal/events/...` passes

### Unit 3: Add fsync to hook checkpoint and telemetry checkpoint writes (040.002-T, 040.003-T partial)

**Files:** `internal/events/hook_checkpoint.go`, `internal/telemetry/checkpoint.go`
**Test files:** `internal/events/hook_checkpoint_test.go`, `internal/telemetry/checkpoint_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Unit 1's `syncWriteFileAtomic` helper for hook checkpoint; inline sync-before-close for telemetry checkpoint
**Dependencies:** Unit 1

**Approach:**

**Hook checkpoint** (`internal/events/hook_checkpoint.go` lines 97-103): Replace `os.WriteFile(tmpPath, payload, 0o644)` with a call to `syncWriteFileAtomic(cpPath, payload, 0o644)`. This consolidates the write-sync-rename sequence into the helper.

**Telemetry checkpoint** (`internal/telemetry/checkpoint.go` lines 75-86): This file is in a different package (`internal/telemetry/`), so it cannot directly use the helper from `internal/events/`. Apply the fix inline: replace `os.WriteFile(tmpPath, data, 0o644)` with an explicit `os.Create(tmpPath)` → `Write` → `Sync()` → `Close()` → `os.Remove(path)` → `os.Rename(tmpPath, path)` sequence. This matches the existing Windows-safe remove-before-rename pattern already in the function.

Both changes preserve the existing error handling structure and return paths.

**Verification:**
* Existing `TestCheckpointStore_SaveAndLoad_RoundTrip` and `TestSaveAndLoadCheckpoint_RoundtripPreservesOffsets` continue to pass
* Existing `TestSaveCheckpoint_AtomicWrite_DoesNotCorruptOnPartialWrite` continues to pass (no `.tmp` file remains)
* New test in hook_checkpoint_test.go: `TestCheckpointStore_SaveCheckpoint_NoDotTmpAfterSuccess` — save, confirm no `.tmp` files remain in the directory
* New test in checkpoint_test.go: `TestSaveCheckpoint_NoDotTmpAfterSuccess` — same pattern
* `go test ./internal/events/... ./internal/telemetry/...` passes

### Unit 4: Add fsync to telemetry JSONL harvest writes (040.003-T)

**Files:** `internal/telemetry/harvest.go`
**Test files:** `internal/telemetry/harvest_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** The existing `writeTelemetryJSONL` function structure at lines 324-411; add `f.Sync()` before `f.Close()`
**Dependencies:** none (independent of Unit 1; inline fix in a different package)

**Approach:**

In `writeTelemetryJSONL` (lines 324-411), insert `f.Sync()` between the last `enc.Encode` call and `f.Close()` at line 402. The current sequence is:

```
for records { enc.Encode(rec) }
f.Close()
os.Rename(tmpPath, jsonlPath)
```

The fixed sequence becomes:

```
for records { enc.Encode(rec) }
f.Sync()
f.Close()
os.Rename(tmpPath, jsonlPath)
```

If `f.Sync()` fails, remove the temp file and return the error (matching the existing error handling pattern for `enc.Encode` failures on lines 396-398).

**Verification:**
* Existing `TestHarvestTelemetry_ProducesSessionSummaries` and `TestHarvestTelemetry_ReHarvestIsIdempotent` continue to pass
* New test: `TestWriteTelemetryJSONL_NoDotTmpAfterSuccess` — harvest, confirm no `.tmp` files remain
* `go test ./internal/telemetry/...` passes

### Unit 5: Fix hook queue stale-lock TOCTOU race (040.004-T)

**Files:** `internal/events/hook_events.go`
**Test files:** `internal/events/hook_events_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Compound learning `docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` — TTL-based stale detection with bounded retry
**Dependencies:** Unit 2 (both modify `hook_events.go`; apply TOCTOU fix after the fsync change)

**Approach:**

The current stale-lock recovery at lines 62-71 has a TOCTOU race:

```
1. O_CREATE|O_EXCL fails (lock exists)
2. os.Stat(lockPath) — check if stale
3. os.Remove(lockPath) — remove stale lock
4. O_CREATE|O_EXCL retry — create fresh lock
```

Between steps 3 and 4, another process can complete the same sequence and create its own lock, then both proceed to write.

The fix uses rename-based atomic recovery to close the TOCTOU gap:

```
1. O_CREATE|O_EXCL fails (lock exists)
2. os.Stat(lockPath) — check if stale (age > hookLockStaleTTL)
3. os.Remove(lockPath+".recovering") — reap any leftover recovery file from a prior crash
4. os.Rename(lockPath, lockPath+".recovering") — atomically claim ownership of stale lock
   - If rename succeeds: this process owns cleanup
   - If rename fails (ENOENT): another process already cleaned up
5. O_CREATE|O_EXCL retry — create fresh lock
6. os.Remove(lockPath+".recovering") — clean up the renamed stale file
```

Step 3 is critical for Windows correctness: `os.Rename` fails on Windows when the destination already exists. If a prior process crashed between steps 4 and 6, a `.recovering` file persists. Without pre-reaping it, the rename in step 4 would fail on Windows, leaving the stale lock in place permanently. The pre-reap in step 3 is safe because a `.recovering` file is by definition orphaned (the process that created it either completed cleanup or crashed).

The rename in step 4 is atomic on both POSIX and Windows (for same-directory renames). Only one process can successfully rename the stale lock; the loser gets `ENOENT` and falls through to the retry, which may succeed if the winner hasn't created its fresh lock yet, or fail with a proper "locked by another process" error.

The `.recovering` suffix is deterministic (not PID-based) because the in-process mutex already serializes within a single process. The cross-process race is the only concern.

**Verification:**
* Existing `TestHookEventWriter_AppendHookEvent_ConcurrentAccess_UniqueSeqs` continues to pass
* New test: `TestHookEventWriter_StaleLockRecovery_SingleWriter` — create a stale lock file (old mtime), confirm AppendHookEvent succeeds and cleans up
* New test: `TestHookEventWriter_StaleLockRecovery_ConcurrentRecovery` — create a stale lock, launch two goroutines attempting AppendHookEvent simultaneously, confirm exactly one succeeds and the other gets a lock-contention error or succeeds after the first releases
* New test: `TestHookEventWriter_FreshLock_NotRemoved` — create a recent lock file, confirm AppendHookEvent returns a lock-contention error without removing it
* New test: `TestHookEventWriter_StaleLockRecovery_PreExistingRecoveringFile` — create both a stale `.lock` and a leftover `.recovering` file, confirm AppendHookEvent succeeds (pre-reap handles the orphaned recovery file)
* `go test -race ./internal/events/...` passes (race detector clean)

## Dependency Graph

```text
Unit 1 (fsutil helper)
  ├── Unit 2 (hook event fsync)     ─── depends on Unit 1
  ├── Unit 3 (checkpoint fsync)     ─── depends on Unit 1 (hook checkpoint uses helper)
  └── Unit 5 (TOCTOU race fix)     ─── depends on Unit 2 (both modify hook_events.go)

Unit 4 (telemetry harvest fsync)    ─── independent (different package, inline fix)
```

**Sequencing rationale:** Unit 1 provides the shared helper. Units 2 and 3 consume it. Unit 4 is independent because it's in `internal/telemetry/` and applies the fix inline. Unit 5 modifies the same file as Unit 2 and should be applied after to avoid merge conflicts. Units 2, 3, and 4 can be implemented in parallel once Unit 1 is complete.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Place helpers in `internal/events/fsutil.go` not a separate `internal/fsutil/` package | Only 2 call sites in this feature scope; avoids premature package creation. Can be promoted later during codebase-wide sweep. | Shared `internal/fsutil/` package — over-engineering for 2 consumers |
| D2 | Inline fsync fix in `internal/telemetry/` rather than importing from `internal/events/` | Cross-package import from telemetry→events creates a coupling that doesn't exist today. The fix is 5 lines. | Import helper across packages — creates unwanted dependency direction |
| D3 | Use rename-based TOCTOU fix, not OS-level flock | Compound learning validates the advisory lock + TTL pattern. Rename is atomic on all platforms Go supports. `flock` is not portable to Windows and would require platform-specific code. | `flock`/`LockFileEx` — not portable, overkill for the single-writer-at-a-time pattern, would require CGo or platform build tags |
| D4 | Use deterministic `.recovering` suffix, not PID-based | In-process mutex already prevents same-process races. Cross-process rename atomicity is the only gap. PID-based suffix adds complexity without benefit. | PID-based suffix — unnecessary given in-process mutex; harder to clean up stale recovery files |
| D5 | Remove destination before rename on Windows (hook checkpoint, telemetry checkpoint) | `os.Rename` fails on Windows when destination exists. The telemetry checkpoint already does this. Apply consistently. | Rely on `os.Rename` overwrite — only works on POSIX |

## Risks and Caveats

**Performance impact:** `fsync` adds I/O latency (typically 1-10ms on SSD, up to 100ms on spinning disk). For hook events and checkpoints that fire infrequently (tens per session, not thousands per second), this is negligible. For telemetry harvest, which writes once per harvest run, the impact is unmeasurable.

**Windows rename semantics:** `os.Rename` does not atomically overwrite on Windows. The existing `Remove` → `Rename` two-step has a tiny window where neither file exists. This is an accepted limitation of Go's stdlib on Windows and is already present in the telemetry checkpoint code. The window is small enough (microseconds) that it is not a practical data loss risk.

**Stale `.recovering` files:** If a process crashes between renaming the stale lock to `.recovering` and cleaning it up, a `.recovering` file persists. This is handled by the pre-reap step (step 3 in the TOCTOU fix): the next recovery attempt removes any leftover `.recovering` file before attempting its own rename. This makes the recovery sequence self-healing across crashes.

**Testing crash durability:** True crash simulation (killing a process mid-write) is not feasible in Go unit tests. The tests verify that `Sync()` is called by confirming data survives a read-back and that no `.tmp` files remain. They do not prove that a power loss at any arbitrary point is safe, which would require integration testing with a fault-injection filesystem.

**Compound learning applied:** `docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` — validates the TTL-based stale lock approach and single-retry boundary. The TOCTOU fix closes the gap identified in that learning's recovery sequence.

## Plan Hardening Signals (REQUIRED)

* **Public API, schema, or contract change:** absent — no MCP tool, CLI, or file format changes. Internal behavior only.
* **Security, auth, permission, or compliance-sensitive behavior:** absent — no auth or permission changes.
* **Migration, backfill, destructive data/config action, or irreversible step:** absent — purely additive (fsync calls). No data format or schema changes.
* **External integration, operator checkpoint, or external dependency:** absent — no new dependencies, no external service calls.
* **High runtime, rollout, or rollback risk:** absent — fsync adds negligible latency to infrequent operations. Rollback is trivial (revert the fsync calls). No feature flags needed.

**Requires plan hardening: no**

## Runtime Verification and Closure

### Hook event queue (Units 1, 2, 5)

* **Runtime surface changed:** Background hook event writer (used by MCP tools that emit hook events)
* **Verification:** After implementation, manually trigger `backlogit_stash` or another hook-emitting MCP tool. Confirm `hooks_queue.jsonl` contains the expected event. Kill and restart the MCP server; confirm events persist across restart.
* **Closure:** No monitoring artifact needed. The fix is invisible to operators. Regression is caught by the new unit tests.

### Hook checkpoint (Unit 3)

* **Runtime surface changed:** Hook consumer checkpoint persistence (used by `backlogit_ack_hook_events`)
* **Verification:** Ack a hook event, kill the MCP server, restart, confirm the ack position is preserved (consumer does not replay).
* **Closure:** No monitoring artifact needed. Covered by regression tests.

### Telemetry harvest (Units 3, 4)

* **Runtime surface changed:** Telemetry harvest JSONL output and checkpoint (used by `backlogit_telemetry_harvest`)
* **Verification:** Run a telemetry harvest, confirm `telemetry-sessions.jsonl` is written. Run a second harvest, confirm incremental processing resumes from checkpoint.
* **Closure:** No monitoring artifact needed. Covered by regression tests.

## Learnings Applied

| Learning | File | How Applied |
|---|---|---|
| Advisory file lock stale TTL recovery | `docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` | Validates TTL approach; TOCTOU fix closes the identified race gap using rename-based atomic recovery |
| Atomic rehydration SQLite transaction | `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md` | Reinforces the principle of atomic state transitions; applied analogously to file writes via sync-before-rename |

## Standards Check

| Standard | Compliance | Notes |
|---|---|---|
| GoDoc on exports | Compliant | New helpers and test functions will have GoDoc comments |
| `golangci-lint` zero warnings | Compliant | No lint suppressions needed |
| `go vet` clean | Compliant | No unsafe patterns introduced |
| `gofmt` clean | Compliant | All new code formatted |
| Error wrapping with `%w` | Compliant | All new errors use `fmt.Errorf("context: %w", err)` |
| No `panic()` in library code | Compliant | No panics introduced |
| TDD: tests before implementation | Compliant | All units specify test-first execution posture |
| `log/slog` for diagnostics | Compliant | Stale-lock recovery logging uses existing `slog.Warn` pattern |
| Parameterized SQL | N/A | No database changes |
| `path/filepath` for paths | Compliant | All path construction uses `filepath.Join` |

## Plan Review

**Date:** 2026-04-22
**Gate:** PASS (after revision)
**Reviewers:** Constitution Reviewer, Go Quality Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist (GPT-5.4)

### Gate Decision: PASS (after revision)

Initial gate was FAIL due to one P1 finding (AS-001: Windows `.recovering` file collision in the TOCTOU fix). The plan was revised in-place to add a pre-reap step and corresponding test coverage. After revision, the P1 is resolved and only P2/P3 advisory findings remain.

### Summary

5 findings total: 1 P1 (resolved), 3 P2 (advisory), 1 P3 (advisory).

### P0 — Critical

None.

### P1 — High (resolved in revision)

**AS-001 (Architecture Strategist):** The rename-based stale-lock recovery used a deterministic `.recovering` suffix but did not handle the case where a prior crash left a `.recovering` file behind. On Windows, `os.Rename` fails when the destination exists, which would cause the recovery rename to fail permanently.

**Resolution:** Added step 3 (pre-reap of stale `.recovering` files) to the TOCTOU fix algorithm in Unit 5. Added `TestHookEventWriter_StaleLockRecovery_PreExistingRecoveringFile` to the verification criteria.

### P2 — Moderate (advisory, accepted)

**AS-002 (Architecture Strategist):** Placing `fsutil.go` in `internal/events/` gives a cross-cutting durability primitive a domain-specific home. The plan acknowledges this tradeoff in Decision D1 and explicitly defers extraction to a shared package until the future codebase-wide sweep creates more consumers. **Accepted as-is** — premature extraction is worse than bounded duplication for 2 call sites.

**AS-003 (Architecture Strategist):** Unit 3 combines work across two package boundaries (`internal/events/` and `internal/telemetry/`). **Accepted as-is** — the two checkpoint fixes share identical root cause, identical fix pattern, and identical review surface. Splitting them adds unit overhead without improving reviewability. The implementer should apply them as separate commits within the unit for clean git history.

**GQ-001 (Go Quality Reviewer, self-identified):** The plan does not explicitly address the `f.Sync()` → `f.Close()` error precedence pattern. When `Sync()` fails, `Close()` must still run for resource cleanup, but the `Sync()` error must be the returned error (not masked by a deferred `Close()`). **Accepted as advisory** — the implementer must use explicit close (not defer) after sync, or use a named return with deferred close that does not overwrite a prior sync error.

### P3 — Low (advisory)

**LR-001 (Learnings Researcher):** No missed learnings. The plan already references the two most relevant compound entries.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| AS-001 | Architecture Strategist | GPT-5.4 |
| AS-002 | Architecture Strategist | GPT-5.4 |
| AS-003 | Architecture Strategist | GPT-5.4 |
| GQ-001 | Go Quality Reviewer (self) | Claude Opus 4.6 |
| LR-001 | Learnings Researcher | Claude Haiku 4.5 |

### Next Steps

Plan review gate: **PASS**. The plan is ready for harvest into shipment-ready backlog. The existing 040-F task hierarchy (040.001-T through 040.004-T) already matches the implementation units, so harvest decomposition is pre-satisfied.
