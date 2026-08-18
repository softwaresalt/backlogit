---
type: session-memory
date: 2026-07-31
agent: stage
command: Stage next
stash:
  - C0909DB5
feature: 133-F
shipment: 115-S
tasks:
  - 133.002-T
  - 133.003-T
  - 133.004-T
  - 133.005-T
  - 133.006-T
deliberation: docs/decisions/2026-07-31-shipshipment-partial-feature-archive-cascade-deliberation.md
plan: docs/exec-plans/2026-07-31-shipshipment-partial-feature-archive-cascade-plan.md
branch: main
mode: degraded
---

# Stage — ShipShipment partial-feature archive cascade (C0909DB5)

## Scope

Executed `Stage next` end-to-end in Stage-only scope: stash to reviewed backlog to
queued shipment. Selected the highest-priority actionable stash entry **C0909DB5**
(high, bug) as a coherent solo batch. The three deferred entries (`0B5FA82B`,
`6FA0829B`, `7F0A6E89`) were left active — two are external-repo-blocked under
Constitution Principle IV and one is an optional deferred product enhancement, so
none was contextually coherent with the C0909DB5 bug.

## Problem staged

`core.ShipShipment` over-archives and over-closes the covering feature on
partial-feature (children-only) shipments. Root cause spans **three** mutation seams
in `internal/core/shipment_lifecycle.go`:

1. `featureIDs` loop (`:191`) — `setArtifactStatus(feature, StatusDone)` unconditionally.
2. `collectArchiveCandidateIDs` (`:292`) — appends each covering feature plus its
   linked deliberation with no membership gate.
3. Transitive parent-status rollup — `completeReleaseScope` (`:241-258`) marks shipped
   children done, and `setArtifactStatus` (`:519`) unconditionally calls
   `cascadePersistedParentStatuses` (`:525-559`) which, via
   `ComputeParentStatus` (`harness_status.go:82`), rolls a covering feature to
   `StatusDone` and relocates it out of `.backlogit/queue/` when all its *recorded*
   children are done.

Chosen fix (deliberation Option A): gate the two direct seams on explicit manifest
membership and neutralize the transitive rollup for non-member covering features via a
narrow pre-ship status/location snapshot-and-restore (primary) or a cascade-suppression
variant. No new terminal-status predicate.

## Artifacts produced

- Deliberation: `docs/decisions/2026-07-31-shipshipment-partial-feature-archive-cascade-deliberation.md` (lint clean).
- Plan: `docs/exec-plans/2026-07-31-shipshipment-partial-feature-archive-cascade-plan.md` (lint clean; `## Plan Review` appended).
- Feature **133-F**, tasks **133.002-T … 133.006-T**, shipment **115-S** (status `queued`).

## Backlog decomposition

| Task | Domain | Unit | Depends on |
|---|---|---|---|
| 133.002-T | tests | 1a — update 3 existing ship tests to membership contract | — |
| 133.003-T | tests | 1b — feature-inclusive + body-planned regression tests | — |
| 133.004-T | code | 2 — neutralize all three seams on manifest membership | 133.002-T, 133.003-T |
| 133.005-T | code | 3 — check-only doctor audit for over-archived features | 133.004-T |
| 133.006-T | docs | 4 — retire P-015 manual workaround in `.ship.agent.md` 6.1.b | 133.004-T |

Four `blocks` dependency edges recorded via `backlogit_add_dependency` and verified in
`item_deps`. Shipment manifest (`custom_fields.items`) =
`133-F, 133.002-T, 133.003-T, 133.004-T, 133.005-T, 133.006-T`.

## Plan review outcome

Three review cycles (circuit-breaker limit reached). Cycles 1 and 2 were FAIL; cycle 3
converged on the third (rollup) seam. The finding was verified against source and
incorporated into the plan spec rather than dispatched a fourth time. Final gate:
`decision: ADVISORY`, `operator_authorization: approved`, satisfying the harvest gate.
Residual risk (the rollup-neutralization mechanism is unverified by review) is carried
forward as a mandatory Ship-phase test-first item on 133.004-T.

## Failed approaches and gotchas

- **`backlogit_create_item` `sections` param rejects whitespace in section-name keys.**
  "Acceptance Criteria" fails with `section name ... must not contain whitespace`. The
  first Task 1a call errored on this but still persisted a partial record (`133.001-T`).
  Workaround: omit `sections`; fold acceptance criteria into the `description` body as a
  `## Acceptance Criteria` heading. This is why task IDs start at `.002`.
- **Orphaned partial `133.001-T`** was archived via `backlogit_archive_item` (authorized
  archival rollback, non-destructive). It is excluded from the shipment manifest.
- **`size_composition.members` overlay on the shipment lists the archived `133.001-T`**
  because it enumerates task-children of `133-F` without filtering archived status. This
  is a derived-field cosmetic artifact only; the authoritative manifest
  (`custom_fields.items`) is correct and is what Ship consumes.

## Degraded / tool notes

- Intercom and engram MCP surfaces not exposed — operated in degraded visibility/search
  mode; no broadcasts emitted; used `backlogit_query_sql` and direct source reads.
- `backlogit query` CLI stalls intermittently under contention with the pre-existing
  `backlogit mcp` processes; MCP `query_sql` was reliable and used throughout.
- MCP `docs_lint` has a path-Rel bug; linting used the CLI fallback
  `backlogit docs lint --profile authoring --no-update-check` (0 violations each file).

## Commit boundary

Committed only Stage artifacts: the deliberation, the plan, this memory file, the new
`.backlogit/queue/` and `.backlogit/archive/` backlog markdown, and the coherent stash
archival pair (`stash.jsonl` removal + `archive/stash.jsonl` add) plus session
`hooks_queue.jsonl` events. Explicitly excluded the four pre-existing dirty files
(`.autoharness/config.yaml`, `.github/agents/.ship.agent.md`,
`.github/agents/_orchestrator.agent.md`); `stash.jsonl`'s only diff was the authorized
C0909DB5 archival, so the three deferred entries were preserved intact. No PR created
(Role Boundary).

## Compaction

`docs/memory/` held 39 files / 167 KB at session end — under the compact-context
numeric triggers (40 files / 500 KB), with a single checkpoint in today's directory.
Compaction was intentionally deferred to avoid churning unrelated memory files.

## Next steps (Ship)

Ship claims shipment **115-S** and executes 133.002-T → 133.006-T in dependency order,
landing the test units red first (1a + 1b) before the Unit 2 fix. The body-planned
regression must assert covering-feature status (not done) **and** queue-directory
location, never merely `result.ArchivedIDs` absence, which false-greens against the
rollup seam.
