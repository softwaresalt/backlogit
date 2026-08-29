---
chunk_strategy: h1-h2-h3
description: "Implementation plan for removing the syncWriteFileAtomic Windows pre-Remove data-loss window"
doc_type: plan
schema_version: "1.0"
source: stage-cb71b412-bug
title: "Remove syncWriteFileAtomic Windows Pre-Remove — Implementation Plan"
---

# Remove syncWriteFileAtomic Windows Pre-Remove — Implementation Plan

## Metadata

| Field | Value |
|---|---|
| Feature | 149-F |
| Task | 149.001-T |
| Shipment | 132-S |
| Stash source | CB71B412 (high, bug) |
| Priority | HIGH — P1 data-loss risk |
| P-017 scope | Post-activation causal item (131-S audit) |
| FC controls | FC-1 (separate red commit), FC-2 (Ship preflight), FC-3 (audit trail) |

## Problem Statement

`internal/events/fsutil.go` function `syncWriteFileAtomic` contains a
Windows-specific pre-Remove block:

```go
if runtime.GOOS == "windows" {
    _ = os.Remove(path)
}
```

This removes the destination file *before* `os.Rename(tmp, path)`. If the
process crashes, is killed, or `os.Rename` fails after `os.Remove` succeeds,
both the original file and the temp file may be lost — a complete data-loss
event with no recovery path.

### Why the pre-Remove is unnecessary

Go (1.24.0 baseline) `os.Rename` on Windows uses `MoveFileExW` with
`MOVEFILE_REPLACE_EXISTING`, which atomically
replaces an existing destination. The pre-Remove was a legacy workaround
for older Go versions where `os.Rename` could not overwrite on Windows.

The project's own `internal/atomicfile/atomicfile_windows.go` already uses
`MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` correctly — confirming the
team's understanding that pre-Remove is unnecessary.

### Production callers

| Caller | File | Status |
|---|---|---|
| `SaveCheckpoint` | `internal/events/hook_checkpoint.go:96` | ACTIVE — uses `syncWriteFileAtomic` directly |
| `CreateCheckpoint` (via hook) | `internal/events/memory.go` | SAFE — uses `syncWriteFileAtomicHook` var → `atomicfile.WriteFileAtomicWithOptions` (148-F/U3). Confirmed by `checkpoint_writesite_test.go` callsite inventory. |
| `ResolveCheckpoint` | `internal/events/checkpoint_lifecycle.go` | SAFE — uses `RewriteCheckpointFile` → `atomicfile.WriteFileAtomicWithOptions` (130-S). Confirmed by `checkpoint_migration_test.go` AST guard. |

Only `SaveCheckpoint` remains on the direct `syncWriteFileAtomic` code path.

The fix is required to protect `SaveCheckpoint` (hook checkpoint persistence)
from the data-loss window.

## Scope Guard

### In scope

* Remove the `if runtime.GOOS == "windows" { _ = os.Remove(path) }` block
  from `syncWriteFileAtomic` in `internal/events/fsutil.go`
* Add a regression test proving overwrite-without-pre-Remove works
* Update the code comment to remove the deferred-fix language

### Explicitly out of scope

* Changes to `atomicfile` package (already correct)
* Changes to `syncWriteFileAtomicHook` (already routes through atomicfile)
* Any other callers or write paths outside `syncWriteFileAtomic`
* Symlink hardening for `syncWriteFileAtomic` (tracked as `302EFF07`, excluded from dark scope)
* Pre-existing P2 issues in adjacent code (e.g., the `EvilCheckpointRewrite`
  function's use of `syncWriteFileAtomic` — it works correctly after the fix)
* Changes to `SaveCheckpoint`'s locking or error handling beyond the
  `syncWriteFileAtomic` call

### Scope boundary test

Any file diff outside `internal/events/fsutil.go` and
`internal/events/fsutil_test.go` is a scope violation unless explicitly
justified in a plan amendment.

## Wave Structure

### Wave 1 (single task: 149.001-T)

**Commit 1 — Red harness (P-002 FC-1: separate commit)**

File: `internal/events/fsutil_test.go`

Add test function `TestSyncWriteFileAtomic_NoPreRemoveInAST` that:

1. Creates a destination file with known content ("original")
2. Calls `syncWriteFileAtomic` to overwrite it with new content ("updated")
3. Asserts the destination contains "updated" (this passes today — the
   function does overwrite correctly, just unsafely)
4. **Critically**: asserts that `os.Remove` was NOT called on the
   destination path during the write. This requires injecting observability
   into the Remove call.

**Harness design — why it fails red:**

The test must detect that `os.Remove(path)` is called on the destination.
Strategy: use a test helper that wraps `syncWriteFileAtomic` with file
system observation. Specifically:

* Create a sentinel file alongside the destination (e.g., `path + ".sentinel"`)
* Before calling `syncWriteFileAtomic`, set up a filesystem watcher or
  stat-based check that detects if `path` is removed then recreated
* **Simpler approach**: The red test asserts a contract property: after
  `syncWriteFileAtomic` returns successfully, the time window where `path`
  does not exist should be zero. Test this by:
  - Creating `path` with content A
  - Running `syncWriteFileAtomic(path, contentB, 0o644)` from a goroutine
  - Concurrently polling `os.Stat(path)` in a tight loop
  - Asserting that `os.Stat` never returns `os.ErrNotExist`

**Alternative, more reliable approach (recommended):**

Use AST inspection (the project already uses this pattern — see
`checkpoint_migration_test.go`). Write a test that:

1. Parses `internal/events/fsutil.go` via `go/parser`
2. Walks the AST of `syncWriteFileAtomic`
3. Asserts that no `os.Remove` call targets the `path` parameter (not `tmp`)
4. This fails RED because the current code DOES contain `os.Remove(path)`

This is deterministic, platform-independent, and follows the established
project pattern for structural contract tests.

**Commit 2 — Green implementation**

File: `internal/events/fsutil.go`

1. Remove the `if runtime.GOOS == "windows" { _ = os.Remove(path) }` block
   (lines ~109-111)
2. Remove the `runtime` import (now unused after the block removal)
3. Update the comment block above it to remove the deferred-fix language
   and replace with a note confirming the atomic replace guarantee

After this commit, the AST-based test passes because `os.Remove(path)` is gone.
Existing tests (`TestSyncWriteFileAtomic_OverwritesExistingFile` etc.) continue
to pass, confirming the overwrite still works.

## P-002 Compliance

### FC-1: Separate Red-Harness Commits

* Commit 1: `test(core): add red harness for syncWriteFileAtomic pre-Remove removal`
  - Contains ONLY the test file change
  - The test MUST fail (red) at this commit
  - The test MUST compile: `go test -run=^$ -count=1 ./internal/events/...`
* Commit 2: `fix(core): remove syncWriteFileAtomic Windows pre-Remove data-loss block`
  - Contains the production code fix
  - The test MUST pass (green) at this commit

### FC-2: Ship Preflight Source/History Check

Ship agent MUST verify before claiming 149.001-T:
1. `*_test.go` harness exists in a commit that is an ancestor of HEAD
2. That ancestor commit does NOT contain the production fix
3. Red commit compiles: `go test -run=^$ -count=1 ./internal/events/...`
4. Red commit tests fail: `go test -run=TestSyncWriteFileAtomic_NoPreRemoveInAST -count=1 ./internal/events/...` exits non-zero

### FC-3: Release Decision Audit Trail

Shipment 132-S closure MUST include:
* Red-harness commit SHA for 149.001-T
* Confirmation red phase was observed to fail
* Reference to INC-P002-131S-148F

## Security and Reliability Analysis

### ProposedAction

| Field | Value |
|---|---|
| summary | Remove pre-Remove block from syncWriteFileAtomic |
| targets | `internal/events/fsutil.go` (lines ~109-111) |
| change_kind | local edit — bug fix |
| rollback | `git revert <commit>` restores the pre-Remove block |
| approval_required | No (pre-authorized normal merge in dark-factory scope) |

### ActionRisk: moderate

* The change removes code (a 3-line block), which is inherently lower risk
  than adding code
* The removed code was documented as a bug — removing it is the fix
* `os.Rename` on Go 1.24.0/Windows uses `MoveFileExW(MOVEFILE_REPLACE_EXISTING)`,
  which is the correct atomic replace primitive
* Risk of regression: LOW — the overwrite behavior is already tested by
  `TestSyncWriteFileAtomic_OverwritesExistingFile`

### Blast radius

* **Direct**: `syncWriteFileAtomic` function (1 function, 1 file)
* **Callers**: `SaveCheckpoint` in `hook_checkpoint.go`
* **Indirect**: Hook checkpoint persistence (non-critical — ephemeral state)
* **Cross-platform**: The removed block only runs on Windows. Unix behavior
  is unchanged. Test coverage is platform-independent (AST-based).

## Rollback and Monitoring Plan

### Rollback trigger

* Any test failure in `internal/events/...` after the fix lands
* Any CI failure on the Windows matrix (if present) or cross-platform test suite

### Rollback procedure

```bash
git revert <fix-commit-sha>
```

This restores the pre-Remove block. The data-loss risk returns, but no
new functionality is broken.

### Monitoring plan

| SLI | Baseline | Alert threshold | Owner |
|---|---|---|---|
| `go test ./internal/events/...` pass rate | 100% | Any failure | Ship agent |
| `syncWriteFileAtomic` overwrite test | Passes | Fails | Ship agent |

### Post-deploy observation

| Field | Value |
|---|---|
| Window | 24 hours after merge |
| Owner | Operator |
| Watch | CI pipeline pass/fail on main, any checkpoint write errors in test suite |

## Ship Instructions

1. **Claim** shipment 132-S
2. **Branch**: Create `fix/149-syncwritefileatomic-preremove` from `origin/main`
3. **Red commit** (FC-1):
   - Add `TestSyncWriteFileAtomic_NoPreRemoveInAST` to `internal/events/fsutil_test.go`
   - Commit as `test(core): add red harness for syncWriteFileAtomic pre-Remove removal`
   - Verify: compiles (`go test -run=^$ -count=1 ./internal/events/...`) but
     test fails (`go test -run=TestSyncWriteFileAtomic_NoPreRemoveInAST -count=1 ./internal/events/...` exits non-zero)
4. **Green commit** (FC-1):
   - Remove the `if runtime.GOOS == "windows" { _ = os.Remove(path) }` block
   - Update surrounding comments
   - Commit as `fix(core): remove syncWriteFileAtomic Windows pre-Remove data-loss block`
   - Verify: `go test ./internal/events/...` passes, `golangci-lint run`, `gofmt -l .`
5. **Quality gates**: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`
6. **PR**: Create PR targeting `main` with merge-commit-only strategy (P-009)
7. **Review**: Request Copilot review, resolve all threads
8. **Merge**: Pre-authorized normal merge (P-017 dark mode); no admin fallback
9. **Closure**: FC-3 audit trail in shipment closure record
10. **Post-merge**: Reconcile shipment, update grouping ledger

## Adversarial Review Evidence

This plan was reviewed by three independent reviewers across different
model tiers:

| Reviewer | Model Tier | Model |
|---|---|---|
| Reviewer-T1 | Tier 2 | claude-sonnet-4.6 |
| Reviewer-T2 | Tier 2 | gpt-5.6-terra |
| Reviewer-T3 | Tier 3 | gemini-3.1-pro-preview |

### Resolved Findings (applied to plan)

| Finding | Confidence | Severity | Resolution |
|---|---|---|---|
| GREEN commit must remove unused `runtime` import | HIGH (2/3) | P0/P1 | Fixed: added explicit import removal step |
| Go version attribution inaccurate (1.17+) | LOW (1/3) | P3 | Fixed: corrected to Go 1.24.0 baseline |

### Rejected Findings (factually incorrect)

| Finding | Confidence | Reason for rejection |
|---|---|---|
| `syncWriteFileAtomicHook` does not exist (T1/T3) | MEDIUM (2/3) | FALSE — hook exists at `fsutil.go:79`, confirmed by `checkpoint_writesite_test.go` callsite inventory |
| `ResolveCheckpoint` calls `syncWriteFileAtomic` directly (T1) | LOW (1/3) | FALSE — `checkpoint_migration_test.go` AST guard enforces ResolveCheckpoint does NOT call it |
| `EvilCheckpointRewrite` exists (T1) | LOW (1/3) | FALSE — file does not exist; only referenced as expected future entry in writesite test |

### Disclosed Findings (not gate-blocking)

| Finding | Confidence | Severity | Decision |
|---|---|---|---|
| Read-only Windows destination regression | LOW (1/3) | P3 | Checkpoint files are created 0o644, in ephemeral runtime dir — not production-relevant |
| AST test bypassed by helper extraction | LOW (1/3) | P3 | Accepted risk — fix is a deletion, not an addition. Indirection-based regression is contrived |
| Duplicate pre-Remove pattern in other files | LOW (1/3) | P3 | Out of scope — different files, different packages. Tracked for future cleanup |
| FC-2/FC-3 could be more explicit | MEDIUM (1/3) | P2 | Accepted — plan already specifies exact commands; Ship agent owns execution detail |

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Safety-First Go | COMPLIANT | Pure Go, no unsafe, golangci-lint clean |
| II. Test-First (P-002) | COMPLIANT | FC-1 separate red commit, FC-2 preflight |
| III. Workspace isolation | N/A | No file system path changes |
| IV. CLI containment | COMPLIANT | All work within cwd |
| V. Observability | COMPLIANT | Commit messages, PR, closure record |
| VI. Single responsibility | COMPLIANT | No new dependencies |
| VII. Destructive approval | N/A | No destructive operations |
| VIII. Safety modes | COMPLIANT | freeze-scope to `internal/events/fsutil.go` |
| IX. Git-friendly | COMPLIANT | Markdown + YAML frontmatter |
| X. Context efficiency | COMPLIANT | Targeted AST test, no bulk reads |
| XI. Merge commits (P-009) | COMPLIANT | Merge-commit-only enforced |

## Grouping Decision Record

### Why CB71B412 is planned as an isolated group (effectively Group 2)

**Decision**: CB71B412 ships as standalone shipment 132-S, immediately after
the completed 130-S/131-S Groups. All previously-labeled Groups 2-4 in the
dark-factory grouping ledger shift down one position.

**Rationale**:

1. **Severity**: P1 data-loss bug outranks all remaining stash items (the
   next highest are P2 convergence/refactor tasks)
2. **FC-4 mandate**: The P-002 incident decision (INC-P002-131S-148F)
   requires CB71B412 to ship no later than the final dark-factory group
3. **Operator directive**: "reliability/security outrank feature/documentation"
   and "needed composability/reliability refactoring precedes new work"
4. **Independence**: CB71B412 has zero code dependency on any other stash
   item. It touches `internal/events/fsutil.go` exclusively. No other stash
   item modifies this file.
5. **Artifact-class isolation**: Grouping CB71B412 with unrelated docline or
   checkpoint items would inflate scope without reducing risk

**Items NOT grouped with CB71B412**:

* A12BBAFA (quarantine sidecar no-clobber) — shares a "write safety" theme
  but different package (`internal/core`), different function, different risk
  profile. No code dependency.
* 302EFF07 (symlink rejection for read paths) — explicitly excluded from
  dark-factory scope; post-activation, not original.
* All other stash items — different files, different packages, no dependency
  relationship.

### Updated group ordering

| Position | Content | Status |
|---|---|---|
| Group 1 (130-S) | Checkpoint disposition hardening | SHIPPED/ARCHIVED |
| Group 1b (131-S) | Checkpoint create/write security | SHIPPED/ARCHIVED |
| **Group 2 (132-S)** | **CB71B412 — syncWriteFileAtomic pre-Remove fix** | **THIS PLAN** |
| Group 3 (was 2) | Docline decode convergence + contract cleanup | DEFERRED |
| Group 4 (was 3) | Checkpoint governance & disposition hardening | DEFERRED |
| Group 5 (was 4) | CLI/MCP parity + harness hygiene | DEFERRED |
