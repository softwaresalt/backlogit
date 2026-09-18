---
doc_type: memory
source: docs/exec-plans/2026-09-17-baseline-convergence-plan.md
date: 2026-09-17
session: stage-baseline-convergence-main
status: complete
---

# Stage session — Baseline convergence release unit recreated from current main

## Why recreated

The first attempt branched off the paused 149-S feature branch, so a direct
staging PR would have carried unrelated Wave 1 source commits. A clean
cherry-pick was aborted because `.backlogit/hooks_queue.jsonl` (tool-managed
event provenance) could not be safely transplanted. The original branch
`stage/baseline-convergence-release-unit` and its commits remain preserved.

This session recreates the SAME reviewed release unit natively from current
`main` (`37a5cba4`, = origin/main) so all backlog artifacts, event streams, and
checkpoint provenance are internally consistent with main.

## Reuse vs native regeneration

- **Reused byte-identical (current-main compatible):** the reviewed deliberation
  (`docs/decisions/2026-09-17-baseline-convergence-deliberation.md`) and plan
  (`docs/exec-plans/2026-09-17-baseline-convergence-plan.md`), restored via
  `git checkout stage/baseline-convergence-release-unit -- <docs>` and verified
  byte-identical. Current-main `.gitattributes` is byte-identical to the reviewed
  baseline (`* text=auto`), so the reviewed plan's root-cause analysis and the
  PASS verdict remain valid without re-review.
- **Review verdict reused:** `## Plan Review` records
  `dispatch_mode: multi-agent-dispatch` / `decision: PASS` /
  `operator_authorization: approved` (Cycle 1 FAIL 2×P1 remediated, Cycle 2
  PASS). Review-record contract re-validated on this branch; docline lint = 0
  violations on both docs.
- **Regenerated natively on main:** feature, 13 tasks, 22 task-dependency edges
  (as first regenerated; now **14 tasks (U1–U14) / 41 edges** after cycle-1 +
  cycle-2 corrections — see the current-state blocks below),
  shipment, cross-shipment dependency, stash archival, and the session
  checkpoint — all via backlogit operations so sequence/provenance is native to
  main. No `.backlogit/` file copies, no cherry-pick.

> **Corrected current state (2026-09-17) — HISTORICAL, SUPERSEDED:** the
> regenerated release unit then
> carried **13 tasks (U1–U13)**, not the 12 (U1–U12) first recorded below. U13
> (`175.013-T` — errcheck remediation of the residual closed package set) was
> added, raising the dependency-edge count to **22** and the shipment `156-S`
> manifest to **14 items**. These figures were authoritative at the U13
> correction but are now SUPERSEDED by the cycle-1 (U14 → 14 tasks / 32 edges /
> 15 items) and cycle-2 (exactly-one-owner redesign → **41 edges**) blocks below;
> the historical wording beneath is preserved for provenance and superseded where
> it conflicts.

> **Corrected current state (Copilot PR #448 review cycle 1):** the release unit
> now carries **14 tasks (U1–U14)**. U14 (`175.014-T` — persistent CI
> line-ending guard owning `.github/workflows/ci.yml`) was split out of U1 so U1
> stays content-identical (EOL-only). New dependency edges: U14→U1 and U12→U14
> (+2), and U11→each errcheck unit U4–U10,U13 (+8, conservative staticcheck
> pre-partition), raising the dependency-edge count from 22 to **32**. Shipment
> `156-S` manifest is now **15 items** (`175-F` + `175.001-T`…`175.014-T`). These
> are the authoritative figures; earlier 13-task / 22-edge / 14-item wording is
> superseded. All mutations via governed backlogit operations.

> **Corrected current state (Copilot PR #448 review cycle 2):** the
> exactly-one-owner DAG redesign REVERSED the staticcheck/errcheck edges — U11
> (`175.011-T`) now runs IMMEDIATELY after U1 as the overlap-inventory owner, and
> `175.004-T`–`175.010-T`, `175.013-T` depend on U11 (NOT the reverse); U2
> (`175.002-T`, residual gofmt) now runs LAST, depending on
> `175.003-T`,`175.004-T`–`175.011-T`,`175.013-T`. The dependency-edge count is now
> **41** (queried via `backlogit dep list`), superseding the cycle-1 **32** figure
> and its "U11→errcheck units / U2→U1-only" wording. Task count (**14**, U1–U14)
> and shipment `156-S` manifest (**15 items**, `175-F` + `175.001-T`…`175.014-T`)
> are UNCHANGED. All mutations via governed backlogit operations.

## Native backlog artifacts (IDs assigned by current main) — current (cycle 2)

- Covering **feature `175-F`** (feature required by `isRootCoveringFeature()` +
  manifest-binding digest).
- Tasks **`175.001-T` … `175.014-T`** (plan units U1–U14 / **14 tasks**), each
  with acceptance criteria + plan reference. *(Historical, superseded: originally
  U1–U12 / 12 tasks, then U1–U13 / 13 tasks; current is U1–U14 / 14 tasks.)*
- Dependency edges: **41** (queried via `backlogit dep list`), under the cycle-2
  exactly-one-owner DAG: `175.003-T`, `175.004-T`–`175.011-T`, `175.013-T`,
  `175.014-T` each depend-on `175.001-T` (U1 strict predecessor);
  `175.004-T`–`175.010-T` and `175.013-T` additionally depend-on `175.011-T`
  (U11 overlap-inventory owner); `175.002-T` (residual gofmt, runs last)
  depends-on `175.003-T`, `175.004-T`–`175.011-T`, `175.013-T` (10 edges);
  `175.012-T` (terminal verification sink) depends-on all fix units
  `175.002-T`–`175.011-T`, `175.013-T`, `175.014-T` (12 edges). *(Historical,
  superseded: 20 → 22 → 32 → 41 edges across corrections.)*
- **Shipment `156-S`** "Repository baseline convergence" (queued, high). Manifest
  = **15 items**: `175-F` (parent-first) + `175.001-T`…`175.014-T`. *(Historical,
  superseded: 13 → 14 → 15 items.)*
- Cross-shipment dependency **`149-S → 156-S (blocks)`**. `149-S` members/manifest
  NOT mutated (only `dependencies: [156-S]` added).

## Stash disposition

- `92F79833` and `4DB1DFF1` archived via governed `stash archive` (attempt 1).
- Deferred-expansion obligations (P-021 C5/C6): duplicate detection CLEAN ×2;
  late-identifier reconciliation no-op ×2 (N/A stands, non-blocking). Recorded in
  the deliberation artifact under Stage authority; no stash edits, no Ship writes.

### Residual-risk follow-up (provenance gap)

- `92F79833` and `4DB1DFF1` were archived through the **plain (non-harvest)**
  archive path, so they carry no canonical `harvested_artifact_id` linkage to
  feature `175-F` / shipment `156-S` / the deliberation + plan. Canonical
  harvest provenance CANNOT be claimed retroactively with current tooling, and
  manual JSONL/SQLite/frontmatter edits are prohibited. This acknowledged gap is
  captured as a governed, audited, idempotent provenance-backfill follow-up:
  **stash `5A016581`** (`DEFERRED SCOPE EXPANSION`, kind chore, priority high,
  `requires deliberation`). It requests supplementary audited linkage — NOT a
  canonical-provenance rewrite — and is an explicit residual risk for a future
  Stage deliberation, not a blocker on this release unit.

## Branch / remote

- Branch `stage/baseline-convergence-main` off `37a5cba4` (= origin/main).
- Verified `git diff origin/main...HEAD` contains ONLY Stage-owned
  backlog/planning/docs artifacts — no `internal/**`, no tests, no 149-S
  implementation history.
- Not on origin/main; reaching main requires a staging PR/merge (outside Stage's
  role boundary — Orchestrator/operator action).

## Next Orchestrator action

Open + merge staging PR `stage/baseline-convergence-main → main`, then route
`156-S` to Ship. `149-S` stays blocked until `156-S` ships.
