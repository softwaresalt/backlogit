---
schema_version: "1.0"
doc_type: plan
title: "Decided plan: Write Durability and Hook Reliability"
description: "Decided-plan consolidation of 2026-04-22-write-durability-hook-reliability-plan.md: final decisions, implementation units, risks, and review verdict."
chunk_strategy: h1-h2-h3
source: docs/exec-plans/2026-10-18-write-durability-hook-reliability-decided-plan.md
---

# Decided plan: Write Durability and Hook Reliability

Consolidated from `docs/archive/plans/2026-04-22-write-durability-hook-reliability-plan.md` (P-020 compaction, 2026-10-18). The owning release units are archived. Full deliberation history stays in the archived original.

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

## Implementation units

* **Implementation Units**: Each unit is scoped to roughly 2 hours of human-equivalent effort: fewer than 3 files modified, fewer than 5 functions changed, fewer than 4 test scenarios.
* **Unit 1: Introduce syncWriteFile helper**: **Files:** `internal/events/fsutil.go` (new) **Test files:** `internal/events/fsutil_test.go` (new) **Effort size:** small **Skill domain:** code **Execution note:** test-first **Patterns to follow:** The existing temp-file-then-rename pattern in ...
* **Unit 2: Add fsync to hook event queue writes (040.001-T)**: **Files:** `internal/events/hook_events.go` **Test files:** `internal/events/hook_events_test.go` **Effort size:** small **Skill domain:** code **Execution note:** test-first **Patterns to follow:** Unit 1's `syncAppendLine` helper **Dependencies:** Unit 1  ...
* **Unit 3: Add fsync to hook checkpoint and telemetry checkpoint writes (040.002-T, 040.003-T partial)**: **Files:** `internal/events/hook_checkpoint.go`, `internal/telemetry/checkpoint.go` **Test files:** `internal/events/hook_checkpoint_test.go`, `internal/telemetry/checkpoint_test.go` **Effort size:** small **Skill domain:** code **Execution note:** test-first ...
* **Unit 4: Add fsync to telemetry JSONL harvest writes (040.003-T)**: **Files:** `internal/telemetry/harvest.go` **Test files:** `internal/telemetry/harvest_test.go` **Effort size:** small **Skill domain:** code **Execution note:** test-first **Patterns to follow:** The existing `writeTelemetryJSONL` function structure at lines ...
* **Unit 5: Fix hook queue stale-lock TOCTOU race (040.004-T)**: **Files:** `internal/events/hook_events.go` **Test files:** `internal/events/hook_events_test.go` **Effort size:** medium **Skill domain:** code **Execution note:** test-first **Patterns to follow:** Compound learning ...

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

**Compound learning applied:** ...

## Plan Review

**Date:** 2026-04-22
**Gate:** PASS (after revision)
**Reviewers:** Constitution Reviewer, Go Quality Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist (GPT-5.4)

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| AS-001 | Architecture Strategist | GPT-5.4 |
| AS-002 | Architecture Strategist | GPT-5.4 |
| AS-003 | Architecture Strategist | GPT-5.4 |
| GQ-001 | Go Quality Reviewer (self) | Claude Opus 4.6 |
| LR-001 | Learnings Researcher | Claude Haiku 4.5 |

## Traceability

| Original path | Archive path | Disposition |
|---|---|---|
| `docs/exec-plans/2026-04-22-write-durability-hook-reliability-plan.md` | `docs/archive/plans/2026-04-22-write-durability-hook-reliability-plan.md` | archived (decided-plan) |
