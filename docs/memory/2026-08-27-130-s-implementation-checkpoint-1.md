---
type: session-memory
session_date: 2026-08-27
shipment: 130-S
feature: 147-F
phase: implementation (wave-driven TDD execution)
---

# 130-S / 147-F Implementation Memory — Checkpoint 1 (Waves 1-4 complete)

## Context

Acting as Ship agent under DARK_MODE_ACTIVE (P-017), executing shipment
`130-S` (feature `147-F`: refuse to rewrite checkpoints carrying unmodeled
top-level keys) end-to-end per `.github/agents/.ship.agent.md` and
`docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`.
43 tasks across 18 dependency waves.

## Critical environment findings

1. **Primary worktree `C:\Source\GitHub\backlogit` is dirty/forbidden** —
   never touched. All work happens in the repurposed worktree at
   `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`,
   now on branch `feat/147-f-checkpoint-toplevel-key-disposition` (based on
   `origin/main` @ `d125565b`, the PR #377 merge commit).
2. **backlogit MCP server is bound to the dirty primary worktree** (confirmed
   via stale `backlogit_get_shipment` results showing pre-PR#377 state). All
   backlog mutations in this session use the **CLI** built from the worktree
   (`backlogit.exe`, built via `go build -o backlogit.exe ./cmd/backlogit`)
   with explicit `--cwd .`, NEVER the `backlogit_*` MCP tools, which would
   silently mutate the wrong (forbidden) workspace.
3. Found and cleaned 126 stale `.lock` files under this worktree's
   `.backlogit/.locks/`, `logs/`, `queue/`, `archive/` (residue from prior
   Stage cycles 1-38 that ran in this same worktree before it was
   repurposed). These are ephemeral, gitignored, never committed — cleanup
   was safe and necessary to unblock `backlogit shipment claim`.
4. `backlogit shipment claim 130-S` cascades ALL manifest members
   (feature + 43 tasks) from `queued` to `active` in one call (confirmed via
   `ClaimShipment` source + its own test suite). My first invocation hung
   (~3+ min, likely due to the stale locks above) and was killed after
   partially completing (tasks 001-030 activated); I manually finished
   activating 031-044 to reach the tool's guaranteed post-claim state.
5. Backlog task status/commit bookkeeping is done via **direct frontmatter
   edits** to `.backlogit/queue/*.md` (batched via a `Set-TaskDone`
   PowerShell helper: sets `status: done`, adds `commit: <sha>`, refreshes
   `updated_at`, adds `harness-ready` label) rather than per-task CLI/MCP
   calls, because the CLI's `move`/`update` commands take ~15-20s each due
   to full workspace re-init — 86+ individual calls would be prohibitively
   slow. Markdown files are the source of truth per project convention; the
   SQLite cache is disposable and rebuilt via `sync` (not yet re-run this
   session — **TODO: run `backlogit sync` before any query-based validation
   or before post-merge closure**).
6. `gofmt -l .` on this Windows checkout falsely flags nearly the entire
   repo (CRLF working-tree vs LF-canonical gofmt output) — verified via
   git-blob byte inspection this is NOT a real formatting issue (see stored
   memory fact). Real compliance verified per-file by stripping `\r` from a
   copy and re-running gofmt.
7. `go test ./...` (full suite) takes ~10-20 minutes wall-clock on this
   machine (internal/core alone ~5-10 min). Running it only at wave
   boundaries where `open_red_deliverables` is empty (waves 1-3, and again
   from wave 13 per the plan's own schedule table); waves 4-12 defer it per
   Step 4.6, running only compile+vet+lint+fmt+scoped commands instead.

## Process correction logged

Made two significant process errors, both caught and corrected before
merge-relevant state was reached:

- **U2 initially absorbed U2b's progress-recursion logic** ahead of its own
  wave. Caught immediately after implementing wave 1/2; reverted with a
  dedicated `fix(core):` commit (`475c1d1e`) before continuing.
- **Wave 3's implementation (U2b, U2c, U1c, U15) was never actually
  committed** before I moved on to drafting wave 4's tests — I had run the
  green checks but skipped the commit step. Caught before wave 4's harness
  commit by checking `git log`; recovered by reverting
  `checkpoint_conformance.go` to `HEAD`, re-verifying genuine RED for wave 4
  units (temporarily disabling one guard test that referenced an
  undeclared symbol to avoid a build-failure false read), then replaying
  wave 3's commits properly before wave 4's.

**Lesson for remaining waves**: commit RED harness → verify still-red →
commit GREEN implementation → update backlog status, in that literal
sequence, before touching the next wave's files. Do not batch multiple
waves' file edits before committing.

## Progress: waves 1-4 complete (9 of 43 tasks done)

| Wave | Task | Unit | Status | Key commit(s) |
|---|---|---|---|---|
| 1 | 147.001-T | U1 | done | 4dc01945 (ErrCheckpointNonConforming, QuarantineIsRemedy) |
| 1 | 147.032-T | U1d | done | 7aab949f (RemediationIntent) |
| 2 | 147.002-T | U2 | done | 154e892f, 475c1d1e fix (CheckConformingTopLevelNamespace) |
| 2 | 147.030-T | U1b | done | 215f10de (BoundedFieldPathSet) |
| 3 | 147.003-T | U2b | done | 13911ec8 (progress recursion) |
| 3 | 147.004-T | U2c | done | 13911ec8 (top-level duplicate detection) |
| 3 | 147.031-T | U1c | done | efa8d1c5 (FieldPathsForDisplay) |
| 3 | 147.038-T | U15 | done | 5919d528 (CheckpointReadResult/GetCheckpointResult) |
| 4 | 147.005-T | U2d | done | 35ce95ab (checkpointV1AllTopLevelKeys) |
| 4 | 147.020-T | U2e | done | 35ce95ab (nested progress duplicates) |
| 4 | 147.028-T | U2g | done | 35ce95ab (nested context duplicates) |
| 4 | 147.042-T | U3c | done (red-deliverable, stays RED until wave 8 / U14) | cb4a5a24 |
| 4 | 147.016-T | U8b | done (red-deliverable, stays RED until wave 13) | cb4a5a24 |

Open red-deliverables as of end of wave 4: `147.042-T` (closes wave 8 via
`147.037-T`/U14), `147.016-T` (closes wave 13 via `147.024-T`/U7c, the last
of its 13 green-makers). Both re-confirmed RED at wave 4's convergence gate.
Full suite deferred since wave 4 (per plan's own schedule: deferred waves
4-12, required again waves 1-3 [done] and 13-18).

## Files touched so far

- `internal/errors/checkpoint_errors.go` (+test): `ErrCheckpointNonConforming`,
  `CheckpointNonConformingError`, `QuarantineIsRemedy`, `BoundedFieldPathSet`,
  `BoundedFieldPaths()`, `FieldPathsForDisplay()`.
- `internal/events/checkpoint_schema.go`: `RemediationIntent`,
  `CheckpointSummary.RemediationIntent` field (deprecated `RemediationCommand`).
- `internal/events/checkpoint_conformance.go` (new, +test):
  `CheckConformingTopLevelNamespace`, `checkpointV1AllTopLevelKeys`,
  `duplicateNestedMemberKeys`.
- `internal/events/checkpoint_lifecycle.go` (+test): `CheckpointReadResult`,
  `GetCheckpointResult`.
- `internal/events/checkpoint_lifecycle_conformance_test.go` (new, U3c,
  red-deliverable, no production change).
- `internal/cli/checkpoint_parity_test.go` (new, U8b, red-deliverable, no
  production change — cross-surface CLI/MCP/events parity harness).
- `internal/events/checkpoint_astshape_test.go` (new): shared go/ast
  source-shape harness helpers for the `events` package.

## Next steps

1. Wave 5: `147.011-T` (U6), `147.029-T` (U7e), `147.033-T` (U2h),
   `147.034-T` (U11). Then waves 6-18 per the plan's wave table (see
   `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md`
   "Wave schedule" table, ~line 2519).
2. `147.036-T` (U13, wave 7) is `harness-exempt: covered-by 147.035-T` (U12).
   `147.007-T` (U3b, wave 10) and `147.021-T` (U2f, wave 9) are
   `harness-exempt: verification-only`. `147.017-T`/`147.018-T` (U9/U9b,
   waves 14-15) are `harness-exempt: docs-only`. `147.019-T`/`147.026-T`/
   `147.041-T` (U10/U10b/U10c, waves 16-18) are `harness-exempt:
   verification-only` — runtime verification, run in a scratch workspace,
   never against live `.backlogit/checkpoints/`.
3. `147.018-T` (U9b) carries the **hard merge gate**: the
   `.github/instructions/backlogit.instructions.md` delta MUST land in the
   same merge commit as `147.007-T`/`147.008-T`/`147.009-T` (U3b/U4/U5).
4. After all 43 tasks: final full-suite run (Step 5, unfiltered, zero
   tolerated red), review skill (escalate to adversarial-review per
   checkpoint disposition being data-integrity/security-adjacent), PR
   creation via `pr-lifecycle`, Copilot review loop, CI green, merge-commit
   only, post-merge closure (`shipment ship 130-S` via CLI --cwd against a
   clean post-merge worktree — NOT the MCP tool, same binding problem as
   item 2 above), `compound-refresh`, `compact-context`.
5. Remember to run `backlogit.exe --cwd . sync` before any SQL-cache-reliant
   verification, and before the final closure step.
