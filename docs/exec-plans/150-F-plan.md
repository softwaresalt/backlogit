---
title: "150-F Implementation Plan: Remove CleanupCheckpoints Windows pre-Remove data-loss window"
feature_id: 150-F
shipment_id: 133-S
stash_id: 11FFF601
status: approved
created_at: 2026-08-29T09:44:00Z
---

# 150-F Implementation Plan

## Problem

`CleanupCheckpoints` in `internal/events/checkpoint_lifecycle.go` (lines 434-438) calls
`os.Remove(dst)` before `os.Rename(path, dst)` on Windows. If `os.Rename` fails after
`os.Remove` succeeds, any previously archived copy at `dst` is destroyed — a data-loss
window identical to CB71B412 in `syncWriteFileAtomic` (149-F, PR #387).

## Root Cause

Legacy Windows compatibility code assumed `os.Rename` could not atomically replace an
existing destination. Go 1.24.0 uses `MoveFileExW(MOVEFILE_REPLACE_EXISTING)` on Windows,
making the pre-Remove unnecessary and harmful.

## Fix

Remove the Windows pre-Remove block (3 lines) and the now-unused `runtime` import from
`checkpoint_lifecycle.go`. This is the exact same fix pattern applied to
`syncWriteFileAtomic` in 149-F/CB71B412.

## Scope Guard

### In Scope
- `internal/events/checkpoint_lifecycle.go`: Remove lines 434-436 (runtime.GOOS guard + os.Remove(dst))
- `internal/events/checkpoint_lifecycle.go`: Remove `runtime` from import block
- `internal/events/checkpoint_lifecycle_test.go` or colocated test: New AST-based structural assertion

### Out of Scope (Explicit Exclusions)
- `internal/events/fsutil.go` — already fixed by 149-F/CB71B412
- `internal/events/hook_events.go` — pre-Remove targets a transient `.recovering` temp file, not primary data
- `internal/events/checkpoint_rewrite.go` — already uses atomicfile.WriteFileAtomic (no pre-Remove)
- All other stash items (1787FD85, 360A183F, EC987334, 6CE00B88, 5F4E0FC3, A12BBAFA, F350503F, 6FA45E69, DBBA62AA, EB93E236, 63E810D9, 5672D73E, 66834D9E, BE32CAE2, 633818E1, 302EFF07)

### Inseparability Analysis
No other checkpoint-governance artifact is code-inseparable from this fix. The 
`CleanupCheckpoints` function is self-contained — it does not share the pre-Remove 
pattern with any other function through a shared helper or call chain.

## Waves (P-002 Contract)

### Wave 1: RED Harness (150.001-T)
- Write `TestCleanupCheckpoints_NoPreRemoveInAST` in `checkpoint_lifecycle_test.go`
- Uses existing `parseEventsSource` and `findPackageFuncIn` helpers from `checkpoint_astshape_test.go`
- Asserts no `os.Remove(dst)` call exists in `CleanupCheckpoints` body
- **FC-1**: Separately committed as harness-only (no production code changes)
- **FC-2**: Compiled and run — observed assertion failure with expected marker
- **FC-3**: Immutable evidence via commit SHA association

### Wave 2: GREEN Fix (150.002-T) — blocks on 150.001-T
- Remove the 3-line Windows pre-Remove block from `CleanupCheckpoints`
- Remove unused `runtime` import
- Verify `TestCleanupCheckpoints_NoPreRemoveInAST` turns GREEN
- Run full `go test ./internal/events/...`
- Run `go vet ./...`

## Rollback Plan
Revert the fix commit. The pre-Remove block is self-contained; reverting restores the
prior behavior with no side effects beyond re-introducing the data-loss window.

## Monitoring / Release Observability
- No runtime monitoring required — this is a code-path removal, not a behavioral change
- No SLI/dashboard changes needed
- Rollback trigger: any regression in `go test ./internal/events/...`
- Post-deploy observation: N/A (CLI tool, no deployed service)

## Constitution Check
- **P-I (Safety-First Go)**: Fix removes unsafe code pattern; no new unsafe usage
- **P-II (Test-First)**: AST harness written and committed RED before production fix
- **P-VII (Destructive Approval)**: No destructive commands; file modifications only
- **P-IX (Git-Friendly)**: All artifacts are Markdown + YAML frontmatter
