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
- **Regenerated natively on main:** feature, 13 tasks, 22 task-dependency edges,
  shipment, cross-shipment dependency, stash archival, and the session
  checkpoint — all via backlogit operations so sequence/provenance is native to
  main. No `.backlogit/` file copies, no cherry-pick.

> **Corrected current state (2026-09-17):** the regenerated release unit now
> carries **13 tasks (U1–U13)**, not the 12 (U1–U12) first recorded below. U13
> (`175.013-T` — errcheck remediation of the residual closed package set) was
> added, raising the dependency-edge count to **22** and the shipment `156-S`
> manifest to **14 items**. The corrected figures are authoritative; the
> historical wording beneath is preserved for provenance and superseded where it
> conflicts.

## Native backlog artifacts (IDs assigned by current main)

- Covering **feature `175-F`** (feature required by `isRootCoveringFeature()` +
  manifest-binding digest).
- Tasks **`175.001-T` … `175.013-T`** (plan units U1–U13), each with acceptance
  criteria + plan reference. *(Corrected: U1–U13 / 13 tasks; originally recorded
  as U1–U12 / 12 tasks.)*
- Dependency edges (22, corrected from 20): `175.002-T`–`175.011-T` **and
  `175.013-T`** each depends-on `175.001-T`
  (U1 renormalize strict predecessor, 11 edges); `175.012-T` (terminal
  verification sink) depends-on `175.002-T`–`175.011-T` **plus `175.013-T`**
  (11 edges).
- **Shipment `156-S`** "Repository baseline convergence" (queued, high). Manifest
  = **14 items (corrected from 13)**: `175-F` (parent-first) + `175.001-T`…`175.013-T`.
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
