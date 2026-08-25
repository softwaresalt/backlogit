---
chunk_strategy: h1-h2
description: "Stage cycle-21 bounded remediation: the fresh local plan review cycle 20 required — closed harness-exempt set fully labelled, Ship ready-selection adapter documented, residual ambient docs-lint references replaced"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-21 harness-exempt and Ship adapter remediation memory
---

## Session frame

* Agent: Stage (planning artifacts only; no production Go, no build loop, no push, no merge, no
  shipment claim, no subagent delegation)
* Worktree: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
* Branch: `chore/stage-130-s`; HEAD at session start `f5f745fab2f11a5ba450f7582b31acb93746bacc`
* Tooling: worktree-bound `go run ./cmd/backlogit --cwd .` CLI only (MCP not used)

## What this session did

1. Appended a formal `cycle: 21` `## Plan Review` record to
   `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — the fresh local plan
   review that the existing `cycle: 20` record's own remediation queue named `required before
   push`. `dispatch_mode: single-agent-declared-degradation`,
   `TOOL_DEGRADED: reviewer-subagent-dispatch`, `decision: FAIL`, severity counts P0=0, P1=1, P2=1,
   P3=2, topology **SOUND**, `operator_authorization: pending`, `push_allowed: no`.
2. Fixed all four findings (R1-R4) in the same pass.
3. Updated the current-gate-state pointer chain (six locations) so exactly one record — `cycle:
   21` — reads as authoritative; marked `cycle: 20` `SUPERSEDED by cycle 21` while preserving its
   root-cause remediation narrative (A-D, U3c, U6e, the topology delta) unmodified as history.
4. Added a one-paragraph continuity note to the `cycle: 18` record explaining the cycle-19
   numbering gap (no separate `cycle: 19` gate record exists; its corrections folded into `cycle:
   18`).

## Findings and fixes

| ID | Severity | Finding | Fix |
|---|---|---|---|
| R1 | P1 | The plan's Documented deviations table enumerates a closed, ten-unit `harness-exempt` set. The cycle-20 checkpoint/memory recorded `harness_exempt_count: 10` as already satisfied, but only `147.007-T`, `147.032-T`, and `147.038-T` actually carried the label; `147.017-T`, `147.018-T`, `147.019-T`, `147.021-T`, `147.026-T`, `147.036-T`, and `147.041-T` carried none. Separately, the global ready-queue policy (`.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`) filters strictly on `harness-ready` and has no rule for `harness-exempt` at all. | Applied `harness-exempt` to the seven missing tasks (verified count = 10 exactly by SQL query). Added a **Ship ready-selection adapter** paragraph to the plan's Documented deviations section (immediately after the closed-set table) and mirrored the rule in `147-F.md`'s test-lifecycle state: Ship's ready-queue filter for shipment `130-S` accepts a task as harness-satisfied when `labels` contains `harness-ready` **or** `harness-exempt`, with the exact SQL predicate given inline. Recorded explicitly as a shipment-scoped **P-002 enforcement deviation/adapter**, not a Principle II waiver; behaviour-changing units still require `harness-ready`. Global policy templates (`.github/policies/workflow-policies.md`, `.github/agents/.ship.agent.md`) are intentionally untouched — out of declared scope for this staging pass. |
| R2 | P2 | `147.017-T` (U9) and `147.018-T` (U9b, a HARD MERGE GATE task) invoked the ambient `backlogit docs lint` command 3 and 4 times respectively, rather than the workspace-pinned form cycles 17 and 19 already standardized everywhere else. | Replaced all 7 occurrences with `go run ./cmd/backlogit --cwd . docs lint`, preserving surrounding args and context (CLI Reference Drift clause, merge-gate clause, acceptance-criteria wording). Verified no other live (non-archived) task under `147-F` carries the ambient form. |
| R3 | P3 | The plan narrates cycle 18 → cycle 20 with no mention of the intervening bounded "cycle 19" advisory-closure session, which a careful reader could read as a numbering gap. | Added a "Continuity note (added cycle 21)" paragraph to the cycle-18 record naming the cycle-19 memory artifact and explaining no separate `cycle: 19` gate record exists. |
| R4 | P3 | Stale current-gate-state pointer chain: six locations (the cycles-1-13 admonition, and the cycle-14/15/16/17/18 self-declarations) named cycle 20 as final/current, which becomes stale once cycle 21 is appended — the same defect class cycle 19's N1 fixed for the 16→17→18 transition. | All six locations updated to name `cycle: 21` as the current gate state; `cycle: 20`'s own record marked `SUPERSEDED by cycle 21`. |

## Files modified

* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — Ship ready-selection
  adapter added to Documented deviations; cycle-21 `## Plan Review` record appended; cycle-20
  record marked superseded; six-location pointer-chain update; cycle-19 continuity note added to
  the cycle-18 record.
* `.backlogit/queue/147-F.md` — plan-review-state paragraph updated to `cycle: 21`; test-lifecycle
  state paragraph records all ten `harness-exempt` labels now applied; Ship ready-selection query
  added.
* `.backlogit/queue/147.017-T.md`, `147.018-T.md`, `147.019-T.md`, `147.021-T.md`, `147.026-T.md`,
  `147.036-T.md`, `147.041-T.md` — `harness-exempt` label added (7 tasks; `147.007-T`, `147.032-T`,
  `147.038-T` already carried it).
* `.backlogit/queue/147.017-T.md` (3 occurrences), `.backlogit/queue/147.018-T.md` (4
  occurrences) — ambient `backlogit docs lint` replaced with
  `go run ./cmd/backlogit --cwd . docs lint`.
* `docs/memory/2026-08-24/stage-d3ce9e81-checkpoint-toplevel-keys-memory.md` — appended a
  "Canonical status update (cycle 21)" addendum; the cycle-19 addendum is left unedited as
  history.

## Topology (unchanged, re-verified)

* 42 queued tasks under `147-F`, 104 queued-to-queued executable edges, 43 shipment members.
* Ready roots unchanged: `{147.001-T, 147.032-T}`.
* Independent Kahn topological sort (re-run against the live index, not assumed from cycle 20):
  42 nodes, 104 edges, 42/42 ordered, **acyclic**.
* No dependency edge, task count, or shipment membership changed this cycle — only `labels`,
  docs-lint command text, and plan/feature prose.

## Validation (worktree-bound `go run ./cmd/backlogit --cwd .` CLI only)

| Gate | Result |
|---|---|
| `sync` | 1208 artifacts indexed, 0 parse failures |
| `doctor` | 23 issues, all pre-existing orphans (`106.0xx-T`, `016.001-R`) outside `147-F`; 0 new |
| `docs lint` | `valid: true, violation_count: 0` |
| `npx markdownlint-cli2@0.23.1` (plan + `147-F.md` + all 42 `147.0*.md` tasks, 44 files) | 0 issues, exit code 0 |
| `query`: `labels LIKE '%harness-exempt%'` under `147-F` | exactly 10 rows, IDs match the plan's closed table exactly |
| `query`: `(labels LIKE '%harness-ready%' OR labels LIKE '%harness-exempt%')` under `147-F` | exactly the same 10 rows — Ship adapter query semantics confirmed |
| `query`: queued task count under `147-F` | 42 |
| `query`: queued-to-queued executable edge count | 104 |
| `query`: shipment `130-S` member count | 43 |
| `query`: root tasks (no dependency row) | exactly `147.001-T`, `147.032-T` |
| Independent Kahn topological sort (re-derived from live `item_deps`, not cached) | 42/42 ordered, acyclic |

## Open questions and next steps

1. **An independent confirmation review of these cycle-21 fixes has not run.** Per the cycle-17
   precedent, a same-pass fix of a P1 finding is remediation evidence, not a substitute for an
   independent confirming pass — the gate remains `FAIL` until that review runs.
2. Only after that confirmation review passes should the branch be pushed and PR #377 reconciled
   (reply to and resolve its 20 unresolved threads).
3. No push was performed. No merge approval was requested. No shipment was claimed. No subagent
   was invoked at any point in this session.
4. Ship must not claim shipment `130-S` until the confirmation review above passes and Ship's own
   build/review/PR-lifecycle gates run against the pushed branch. When Ship's harness-architect
   runs, it MUST use the Ship ready-selection adapter query (`harness-ready` OR `harness-exempt`)
   recorded in the plan and in `147-F.md`, not the global `harness-ready`-only policy text alone.
