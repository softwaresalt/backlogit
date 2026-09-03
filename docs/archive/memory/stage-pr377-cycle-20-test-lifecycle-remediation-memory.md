---
chunk_strategy: h1-h2
description: "Stage cycle-20 remediation of the 20 unresolved PR #377 Copilot threads: one constitutional test lifecycle, two new units, recomputed topology"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-20 test-lifecycle remediation memory
---

## Session frame

* Agent: Stage (planning artifacts only; no production Go, no build loop, no merge, no shipment claim)
* Worktree: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
* Branch: `chore/stage-130-s`; parent head at session start `3bcff086`
* PR: #377, CI 6/6 green, Copilot review fresh on `3bcff086`, **20 unresolved Copilot threads**
* Tooling: worktree-bound `backlogit` CLI only (MCP not used); `gh` GraphQL for thread inspection

## Thread inventory and root-cause mapping

All 20 threads were re-fetched from the GitHub GraphQL API with pagination
(`reviewThreads(first: 100)`, `hasNextPage: false`, 98 total threads, 20 unresolved) and each
comment body was read in full rather than from a prior summary.

| Root cause | Count | Threads |
|---|---|---|
| A — green regression guards committed inside the red harness | 14 P1 | `147.002-T`, `147.003-T`, `147.005-T`, `147.009-T`, `147.011-T`, `147.012-T`, `147.014-T`(line 43), `147.016-T`, `147.022-T`, `147.023-T`, `147.024-T`, `147.025-T`, `147.027-T`, `147.028-T` |
| B — units with no genuine failing RED | 3 P1 | `147.007-T` (U3b), `147.032-T` (U1d), `147.038-T` (U15) |
| C — false published read-surface contract | 1 P2 | `147.014-T` line 32 |
| D — stale halt path / stale archive ownership note | 2 P2 | `147.021-T` line 31, `.backlogit/archive/147.010-T.md` line 35 |

## Decisions

1. **One three-step test lifecycle replaces the two-step posture.** Declaration step (no test
   function) → red harness step (only functions that fail) → green step (implementation plus
   `TestU<unit>Guard_` guards). P-004's precondition is expected failure markers *for every test
   function*, so a green function in the harness commit defeats the gate regardless of the `-run`
   regex quoted. The "declared regression guards" device and the cycle-17 narrowed-red-selector
   device (U2g, U2h) are both withdrawn.
2. **Guard naming contract added.** `TestU<unit>Guard_<Descriptor>`. The mandatory `_` after
   `Guard` plus the fact that no unit label ends in `Guard` makes every guard selector disjoint
   from every red selector and from every sibling unit's.
3. **The cycle-8 "every unit needs one failing assertion" rule is withdrawn.** It is what forced
   the fabricated REDs on U1d and U15. A test that can only fail because a symbol does not exist is
   a build error; a test that passes the moment the declared shape lands was never red.
4. **`harness-exempt` is a closed, enumerated set of ten units** with four classes:
   `declaration-only` (U1d, U15), `covered-by <unit>` (U13 ← U12), `verification-only` (U3b, U2f,
   U10, U10b, U10c), `docs-only` (U9, U9b). Recorded in the plan's Documented deviations table with
   the location of each unit's downstream failing harness.
5. **U3b's production delta is withdrawn.** After U14 routes `ResolveCheckpoint` through
   `RewriteCheckpointFile`, the seam returns the raw `*CheckpointNonConformingError` before any
   write, and that unwraps to `ErrCheckpointNonConforming` — so U3b's "add a conformance call"
   changed nothing observable, and its case 1 passed. The contract is delivered by **U14**; its
   failing harness is the new **U3c** (`147.042-T`), which lands before U14. U3b becomes the
   verb-level contract pin.
6. **U6e (`147.043-T`) makes the remediation-intent contract total** rather than narrowing U7b's
   published description. `ListCheckpoints` sets `NeedsQuarantine: true` on three branches
   (`checkpoint_lifecycle.go:52-54` parse failure, `:72-74` schema-invalid, and U6's conformance
   branch); only the third populated `RemediationIntent`. U6e adds `Reason: "unparseable"` and
   `Reason: "schema_invalid"` on the other two, with precedence recorded: `unparseable` is terminal,
   and `non_conforming` overwrites `schema_invalid` when both apply.
7. **New protected invariant I4** — declaration → harness → implementation monotonicity.
8. **Cycle 18 is formally superseded.** Its `decision: ADVISORY`, `push_allowed: yes`, and
   `operator_authorization: approved` are withdrawn for gate purposes. The current gate is cycle 20,
   `decision: FAIL`, `restage_recommendation: fresh-local-plan-review`.

## Files modified

* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — doctrine rewrite,
  guard naming contract, per-unit Expected red / Green-step guards lines across ~30 units, new U3c
  and U6e sections, U3b rewrite, U1d and U15 declaration-only, partition table, dependency graph,
  edge table, execution order, topology counts, Requirements Trace R3/R11/R12, Constitution Check
  II, Documented deviations (+ closed harness-exempt table), gate-pointer chain, cycle-20
  remediation appendix, cycle-20 Plan Review record
* `.backlogit/queue/147-F.md` — gate state, cycle-20 narrative, task inventory, topology,
  test-lifecycle state
* `.backlogit/queue/147.001-T` … `147.041-T` (40 tasks) — red/green ownership sweep
* `.backlogit/queue/147.042-T.md`, `.backlogit/queue/147.043-T.md` — new
* `.backlogit/queue/130-S.md` — manifest grows to 43 members
* `.backlogit/archive/147.010-T.md` — stale ownership note corrected

## Topology

* 42 queued tasks, 104 executable queued-to-queued edges, 43 shipment members
* Six edges added: `147.042-T -> 147.001-T`, `147.042-T -> 147.004-T`, `147.043-T -> 147.011-T`,
  `147.043-T -> 147.032-T`, `147.037-T -> 147.042-T`, `147.014-T -> 147.043-T`
* Ready roots unchanged: `{147.001-T, 147.032-T}` — both new tasks are interior nodes
* Kahn topological sort: 42/42 ordered, acyclic. `147.042-T` orders at position 12, before
  `147.037-T` at 25; `147.043-T` at 21, before `147.014-T` at 26

## Validation

| Gate | Result |
|---|---|
| `backlogit --cwd . sync` | 1208 artifacts, 0 parse failures |
| `backlogit --cwd . doctor` | 23 issues, all pre-existing orphans outside `147-F`; 0 new |
| `backlogit --cwd . docs lint` | `valid: true, violation_count: 0` |
| `markdownlint-cli2` (45 files) | 0 issues |
| Kahn topological sort | 42 nodes, 104 edges, acyclic, roots `{147.001-T, 147.032-T}` |
| RED/green ownership audit | 42/42 tasks carry an explicit red-harness or `harness-exempt` classification and an explicit guard declaration; exempt set = 10, matching the plan's closed table |
| Width audit | No task exceeds 3 scenarios. `147.037-T` = 3 files (2 modified + 1 new test); `147.014-T` / `147.024-T` = 2 modified + 2 conditionally re-run. All pre-existing and unchanged by this cycle |

## Open questions and next steps

1. **A fresh local plan review of the cycle-20 artifacts has not run.** The remediation is complete
   but unreviewed. This is the blocking next step and the reason the gate is `FAIL`.
2. Only after that review passes should Ship reply to and resolve the 20 PR #377 threads.
3. No push was performed. No merge approval was requested. No shipment was claimed.
4. Watch for the same failure mode recurring: any future unit that describes a scenario as "green
   on landing" inside its harness step is a P-004 violation under the new doctrine.
