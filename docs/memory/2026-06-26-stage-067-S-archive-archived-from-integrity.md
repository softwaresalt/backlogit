# Stage Session — 067-S ArchiveItem archived_from Integrity

**Date**: 2026-06-26
**Agent**: Stage (stash → backlog pipeline)
**Stash consumed**: `53F22794` (high-priority production bug) → archived
**Shipment produced**: `067-S` (queued) → handoff token to Ship
**Base**: `main` @ `0036a7ee`; staging branch `chore/stage-067-S`

## Intake & classification

- Stash `53F22794`: `core.ArchiveItem` stamps `archived_from` as a **self-reference**
  for pre-archived items (`currentPath == archivePath`), breaking invertible unarchive.
- Classified **feature-shaped** (multi-task coherent fix) → no grouping (single targeted entry).
- Learnings: no archive/unarchive prior art; reused docline authoring contract
  (`docs/compound/2026-06-26-docline-frontmatter-contract.md`).

## Grounded evidence (read-only, no source edits)

- Defect site `internal/core/archive.go:181`; F-006 guard 368-373; UnarchiveItem 358-409.
- Contract test `internal/core/archive_test.go:70` asserts `.backlogit/queue/001-T.md`;
  **no test** for the pre-archived case (the gap that let the defect through).
- **Legacy census** (scanned 602 archive `.md`): **130 self-referential**, 258 canonical,
  211 fieldless, 3 other (1 legit `036-DL`→`deliberations/`, 2 malformed `archived_from: done`).
- `backlogit doctor` → "No issues found" (doctor does NOT yet audit archived_from invertibility).

## Pipeline artifacts

- **Deliberation**: `docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md`
- **Plan**: `docs/exec-plans/2026-06-26-archive-archived-from-integrity-plan.md`
  (Requires plan hardening: yes → `## Plan Hardening` appended; `## Plan Review` records
  attempt-1 FAIL → attempt-2 PASS).

## Review gate (P-006 + Step 4)

- Attempt 1 = **FAIL** (5 grounded P1s): archived_from must be `.backlogit/`-prefixed;
  registry routes by STATUS not TYPE (no type→dir map); U1 helper must be in-package test;
  `--fix` must be CLI-only (Principle VII); UnarchiveItem must self-heal at read-time.
- Revised plan → Attempt 2 = **PASS** (all P1s CLOSED, 2 advisories retained).

## Harvest + shipment

- Feature **`067-F`** + tasks **`067.001-T … 067.007-T`** (TDD-first, atomic, single-domain).
- Dependency edges (blocks): 002→001, 003→001, 003→002, 005→004, 005→001, 006→005, 007→005.
- Shipment **`067-S`** (queued), items `[067-F, 067.001-T..067.007-T]`, feature-first, verified.

## Decisions captured

- `archived_from` MUST be repo-root-relative `.backlogit/`-prefixed (matches `workspaceRelativePath`
  / `archive_test.go:70`); prefix-less `queue/...` would be rejected by F-006 and strand records.
- U1 resolver is pure over `ws.Config.QueueLayout.RootDir` (default `queue`), NOT registry
  (DirectoryRule.Condition is status-keyed; no artifact-type→dir map exists).
- Migration is **CLI-only** `--fix-archived-from` (mirrors `--fix-orphans`), NOT on MCP doctor tool.
- U3 makes UnarchiveItem **self-heal at read-time** (does not depend on migration).

## Open operator decisions (flagged in plan)

1. doctor-audit-with-fix (Option B, recommended) vs one-shot migration (Option A).
2. Malformed `archived_from: done` records (e.g. 038-DL/039-DL) — v1 flags only, no auto-fix.
3. Migration in same shipment vs follow-up.

## Handoff

- **Hand off shipment `067-S` to Ship** (not the feature ID).
- Staging artifacts landed via PR `chore/stage-067-S` → `main` (CI + Copilot readiness gate),
  HALT for operator merge approval (no self-merge; merge-commit only, P-009).

## Deferred

- None. All other active stash entries (AE53BC5C, E4B7767C, 98C4F063) left untouched.
