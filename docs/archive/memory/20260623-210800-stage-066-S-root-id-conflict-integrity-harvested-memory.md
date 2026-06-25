# Stage Session Memory — Root-ID Conflict Integrity (066)

- **Date:** 2026-06-23
- **Agent:** Stage
- **Status:** Harvest + shipment assembly COMPLETE; staging-artifact landing (commit/PR) in progress
- **Source stash:** `0F65FBC9` (bug, high) — undetected top-level work-item root-ID
  conflicts between queue and archive → **archived as consumed**

## Pipeline outcome

| Step | Result |
|---|---|
| 0.0 Tool gate | ALL_TOOLS_OK (CLI-backed; MCP-only ops via file-backed docs) |
| 0.1 Index sync | INDEX_SYNC_OK |
| 1 Triage | Single targeted task-shaped bug `0F65FBC9` |
| 1.5 Grouping | Single-entry fallback → solo group (implicit covering feature) |
| 1.8 Learnings | Found atomic-rehydration-sqlite-transaction-2026-04-08 (constrains U4) |
| 2 Deliberation | `docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md` |
| 3 Plan + harden | `docs/exec-plans/2026-06-23-root-id-conflict-integrity-plan.md` (hardened) |
| 4 Plan review | FAIL (attempt 1, convergent P1) → revised → **PASS** (attempt 2) |
| 5 Harvest | Feature `066-F` + 5 tasks + 4 dep edges |
| 5.5 Shipment | `066-S` (queued) — handoff token to Ship |
| 5.6 Archive | `0F65FBC9` archived |

## Backlog created

- **Feature:** `066-F` "Root-ID Conflict Integrity: Detection, Allocation, and
  Archive Safety" (queued)
- **Tasks** (all queued, parent `066-F`):
  - `066.001-T` U1 — doctor canonical root-ID audit + shared recursive scanner
  - `066.002-T` U2 — pre-write ID uniqueness guard (`ErrIDCollision`)
  - `066.003-T` U3 — archive distinct-destination overwrite refusal (`ErrArchiveDestinationOccupied`)
  - `066.004-T` U4 — rehydrate duplicate-source warning (no transaction change)
  - `066.005-T` U6 — end-to-end repro (queue + archive same root ID)
- **Dependency graph:** `066.002-T → 066.001-T`; `066.005-T → {066.001-T,
  066.002-T, 066.003-T}`. `066.003-T`, `066.004-T` independent.
- **Shipment:** `066-S` items `[066-F, 066.001-T, 066.002-T, 066.003-T,
  066.004-T, 066.005-T]` (parent-first, topological).

## Follow-up stash entries (deferred, NOT in 066-S)

- `B8FF7590` (bug, high) — manifest-drift data repair for `060-S`/`061-S`/`062-F`.
  Decided SEPARATE (one-time data repair vs code hardening; mutates live queued
  shipment state; Stage role boundary). Operator decision pending on ownership.
- `C55C5158` (task, medium) — durable per-type high-water-mark counter (U5),
  externalized; design-gated on persistence model (Git-committed vs local).

## Key technical findings (carry forward)

- **`dep add` frontmatter persistence / stale binary:** The repo-root prebuilt
  `backlogit.exe` (built 2026-05-11) is STALE relative to source — it predates the
  `core.AddDependency` frontmatter-persistence fix (`internal/core/dependencies.go`,
  committed 2026-06-19, with regression test
  `TestAddDependency_PersistsToFrontmatterAndSurvivesSync`). With the stale binary,
  `dep add` only mutated the disposable SQLite index, so `sync` (rebuild from
  canonical) wiped the 066 edges — which is what was observed empirically. In
  CURRENT source, `AddDependency` writes the target IDs into the item `.md`
  `dependencies:` frontmatter (source of truth) and survives sync; only the
  `dep_type` is non-durable (frontmatter stores target IDs without a type, so a
  non-`blocks` type is rebuilt as `blocks` on Rehydrate). Workaround applied: the
  066 edges were written directly into frontmatter and verified in `item_deps`
  after sync — equivalent to what a current-source `dep add` produces (all edges
  are `blocks`, so no dep_type loss). **Operator action: rebuild/refresh the
  repo-root `backlogit.exe` from source so CLI behavior matches the code.**
- Detection (doctor `CheckDuplicates`) already runs by default — U1 is EXTEND not
  build-new. U2 must guard the `NextTypedHierarchicalID(parentID="")` root path
  (the actual bug site), full `artifactSearchDirs` scope, fail-loud. U3 must key
  refusal on PATH equality (two different items can share a root ID). U4 stays in
  `internal/db` (no core import), transaction untouched.

## Open operator questions

1. Durable-counter persistence model (carried with `C55C5158`; non-blocking).
2. Manifest-drift repair ownership — Stage-planned + Ship-executed vs operator
   direct (carried with `B8FF7590`).
3. Stale repo-root `backlogit.exe` (2026-05-11) lags source (2026-06-19 dep
   persistence fix). Rebuild it so `dep add` persists to frontmatter directly.

## Next steps (landing)

Commit `.backlogit/` + `docs/` artifacts (conventional commit + Copilot
co-author trailer) → branch `chore/stage-066-S` → PR to `main` (required check
`test (1.24)`) → drive CI green → Copilot review-readiness gate → **HALT for
operator merge approval** (merge commit only; no self-merge).
