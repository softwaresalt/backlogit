---
title: "Spike: shipment sequence manifest for dark factory mode"
source: docs/decisions/2026-07-29-ship-sequence-manifest-spike.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Findings on whether to add a ship_sequence.jsonl manifest for long-running dark factory shipment ordering."
docline:
    type: spike
    date: 2026-07-29
    time_box: "4h"
    conclusion: "defer"
    confidence: "medium"
    linked_parent_work_item: null
    promoted_to:
        - "queue"
    tags:
        - "orchestrator"
        - "shipment"
        - "dark-factory"
        - "backlogit"
---

## Goal

Should backlogit add a `.backlogit/ship_sequence.jsonl` manifest so the
Orchestrator can declare and persist the logical execution order of shipments
across a long-running dark factory mode run?

## Success Criteria

* Current shipment ordering and dependency surfaces are identified from code,
  not inferred from workflow prose
* The spike distinguishes what a sequence manifest would uniquely record from
  what `queue view`, `item_deps`, and shipment manifests already record
* Storage, schema, sync, and dark-mode visibility implications are explicit
* The recommendation is one of build, do-not-build, or defer, with rationale

## Scope Constraints

* Stage-only investigation. No product implementation, feature branch, build,
  test suite, PR, or Ship work
* Process exactly stash entry `16FD6CC0`; leave `7F0A6E89`, `6FA0829B`,
  `113-S`, and `132-F` plus children untouched
* No writes outside `C:\Source\GitHub\backlogit`
* Backlog mutations are limited to archiving or harvesting the spike outcome

## Investigation Approach

1. Search prior compound learnings and decisions for shipment sequencing,
   dark factory mode, `DARK_MODE_SCOPE`, and Orchestrator sequencing.
2. Read the Orchestrator dark factory contract and P-017 policy.
3. Inspect current queue ordering, item dependency, shipment manifest, and
   shipment lifecycle code paths.
4. Inspect the SQLite projection and rehydration paths for storage impact.
5. Compare options and recommend build, do-not-build, or defer.

## Findings

### What Was Discovered

#### 1. No prior ship-sequence artifact exists

Searches across `docs/compound/`, `docs/decisions/`, and `.backlogit/queue/`
found no existing `ship_sequence`, `shipment sequence`, or
`DARK_MODE_SCOPE` design. The closest reusable learning is that shipment
membership stored in `custom_fields.items` must be normalized on every read
because SQLite JSON projection loses concrete slice type
(`docs/compound/go-patterns/f015-shipment-stash-patterns.md`). A second related
learning warns that DB-backed re-persist paths can drop non-projected metadata,
so new coordination metadata must have a clearly owned source of truth before it
is projected (`docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`).

#### 2. P-017 needs declared scope and visibility, not a new scheduler by itself

P-017 requires Orchestrator to record `DARK_MODE_ACTIVE` and surface bounded
scope, merge approval, admin fallback authority, stop conditions, and visibility
mode (`.github/policies/workflow-policies.md:452-459`). Its scope rule forbids
silent expansion to unrelated stash entries, queued tasks, or shipments
(`.github/policies/workflow-policies.md:461`). Its required telemetry includes
`DARK_MODE_SCOPE` (`.github/policies/workflow-policies.md:491-495`).

The Orchestrator mirrors this contract: dark mode activation records bounded
scope (`.github/agents/_orchestrator.agent.md:132-139`) and emits
`DARK_MODE_SCOPE` (`.github/agents/_orchestrator.agent.md:150-154`). The current
state assessment reads active and queued shipments with `list_shipments`
(`.github/agents/_orchestrator.agent.md:179-180`). Ship routing then says to
select the "highest-priority queued shipment"
(`.github/agents/_orchestrator.agent.md:254`), but shipment artifacts commonly
do not carry priority. The current queued shipment, `113-S`, has no priority in
the SQLite row and its manifest only records `custom_fields.items`
(`.backlogit/queue/113-S.md:4-17`).

This creates a real audit/resume gap for a long dark-mode series: the selected
multi-shipment order and the rationale for that order are not persisted as a
single durable activation record. However, P-017 does not require that record to
be a new scheduler table or a new source of truth.

#### 3. Current queue ordering already has two durable ordering primitives

`backlogit queue view` uses queued, active, blocked, and review items by
default, with priority as the secondary sort after manually assigned queue
positions (`internal/cli/queue_cmd.go:54-55`). The core queue query appends the
order clause from `buildQueueOrderClause`
(`internal/core/queue.go:94`), where `custom_fields.queue_position` is the first
ordering key and priority, title, type, status, updated time, or created time can
serve as the secondary ordering (`internal/core/queue.go:174-207`).

The same queue query filters out items with unresolved execution-blocking
dependencies (`internal/core/queue.go:120-124`). The dependency filter reads
`item_deps` and suppresses items whose `blocks`, `parent_of`, or `relates_to`
dependencies are not in a terminal state (`internal/core/queue.go:435-495`).
The SQLite schema already stores those edges in `item_deps`
(`internal/db/schema.go:262-269`).

Because shipments are normal artifacts in the `items` table, these primitives
can already express shipment ordering if the Orchestrator consumes
`queue view --type shipment --status queued` instead of only `shipment list`.
That path would use the existing `queue_position` and `item_deps` semantics
without adding a second ordering authority.

#### 4. Shipment manifests already own membership and lifecycle scope

`CreateShipment` stores the shipment's item IDs under `custom_fields.items`
(`internal/core/shipment.go:54-60`). `AddItemToShipment` appends to that same
ordered list after validating assignment conflicts (`internal/core/shipment.go:178-200`).
`NormalizeShipmentItems` is the single read edge for that list
(`internal/core/shipment.go:568-625`).

Lifecycle code consumes the manifest as explicit scope. `ClaimShipment` moves a
queued shipment to active and activates each manifest item in manifest order
(`internal/core/shipment_lifecycle.go:43-83`). `ShipShipment` derives its
`explicitScope` from `NormalizeShipmentItems(shipment)` before resolving the
release scope (`internal/core/shipment_lifecycle.go:135-161`).

Therefore a `ship_sequence.jsonl` must not repeat shipment membership, member
task order, or item status. Those are already owned by the shipment manifest and
artifact lifecycle. Duplicating them would create stale split-brain state.

#### 5. A new JSONL file is feasible but not free

The current index schema has tables for `items`, `item_deps`, `stash_entries`,
`stash_links`, semantic links, logs, commit links, and gate evidence
(`internal/db/schema.go:262-341`). There is no table for a shipment sequence.

`backlogit sync` rehydrates by walking `.backlogit` for Markdown artifacts and
explicitly skipping non-Markdown files in that walk
(`internal/db/rehydration.go:117-132`). It then clears and rebuilds the
`items`, `item_deps`, and `item_links` projections
(`internal/db/rehydration.go:165-171`). Stash JSONL has a separate rehydration
path. A new `.backlogit/ship_sequence.jsonl` would need either:

* direct Orchestrator file reads, with no SQLite query support
* a new projection table plus sync/rehydration support

The second option is feasible, but it is a schema and lifecycle change. It must
define owner, append/update semantics, stale shipment handling, and conflict
behavior before implementation.

### Options Considered

#### Option A: Do nothing and keep using `shipment list`

This keeps the system simple but leaves the dark-mode multi-shipment order
implicit. It also leaves Orchestrator's "highest-priority queued shipment"
selection under-specified when queued shipments do not carry priority.

#### Option B: Reuse `queue view` plus `item_deps` for shipment order

This is the lowest-risk path. It makes `queue_position` the durable manual order
and `item_deps` the durable blocking order. It also keeps the SQLite index as
the query surface the Orchestrator already knows how to read.

The gap is rationale: `queue_position` and `item_deps` explain order only by
position and dependency, not why a dark-mode scope was chosen.

#### Option C: Build `ship_sequence.jsonl` as the authoritative scheduler

This is not recommended. It would duplicate `queue_position`, `item_deps`,
shipment status, and shipment membership. It also introduces a new stale-state
problem: the manifest could say shipment `A` precedes shipment `B` after `A` was
archived, descoped, or blocked unless every lifecycle operation updates both
surfaces.

#### Option D: Build `ship_sequence.jsonl` later as a non-authoritative audit plan

This is the only version of the manifest that appears safe. It would record a
dark-mode activation plan, not replace the queue. Candidate fields:

* `run_id` or `sequence_id`
* `position`
* `shipment_id`
* `declared_status`
* `depends_on_shipments`
* `source_scope` such as stash IDs, feature IDs, or shipment IDs
* `rationale`
* `declared_by`
* `created_at`
* `dark_mode_event` such as `DARK_MODE_SCOPE`

Consumption rule: the Orchestrator may use the plan only after validating that
each shipment still exists, remains in the declared scope, passes P-001/P-016
preconditions, and is not blocked by current `item_deps` or shipment status.
If validation disagrees with the plan, queue/dependency state wins.

### What Was Tried and Failed

No prototype was built because the spike scope was read-only planning and
backlog artifact work. The investigation found that making a sequence manifest
authoritative would fail the ownership test: it repeats data already owned by
shipment manifests, `item_deps`, queue ordering, and lifecycle status.

### Remaining Unknowns

* Whether dark factory mode will frequently run with more than one queued
  shipment in scope. The current workspace has one queued shipment, `113-S`.
* Whether operators need rationale to be machine-queryable, or whether a
  dark-mode activation checkpoint and session summary are sufficient.
* Whether shipment-to-shipment dependencies should be first-class UI/CLI
  affordances over existing `item_deps`, or remain an advanced operation.
* Whether a future sequence audit should be projected into SQLite or read
  directly by the Orchestrator.

## Recommendation

**Conclusion**: defer

**Confidence**: medium

Defer a standalone `ship_sequence.jsonl` build. The manifest is feasible and
could complement P-017 as a dark-mode audit plan, but making it authoritative
would duplicate existing queue, dependency, shipment-manifest, and lifecycle
surfaces. The lower-risk next step is to make Orchestrator consume the existing
shipment queue ordering (`queue view --type shipment --status queued`), use
`item_deps` for true blocking order, and record the ordered dark-mode scope in
activation/checkpoint evidence. Revisit a non-authoritative
`ship_sequence.jsonl` only after a real multi-shipment dark-mode run shows that
the activation evidence is insufficient for resume and audit needs.

## Next Steps

* Do not create a build shipment for `ship_sequence.jsonl` now.
* Keep spike work item `001-SP` as the completed backlog trace for stash
  `16FD6CC0`.
* If the operator later wants to pursue this, start with a narrow Orchestrator
  follow-up: select queued shipments through `queue view --type shipment
  --status queued`, include the ordered shipment IDs in `DARK_MODE_SCOPE`, and
  validate against current shipment status before each Ship handoff.
* Treat any future `ship_sequence.jsonl` as non-authoritative audit evidence,
  not as the source of truth for queue or dependency ordering.

## Backlog Linkage

* Stash entry: `16FD6CC0`
* Harvested spike work item: `001-SP`
* Build shipment: not created, because the recommendation is defer rather than
  build

## References

* `.github/agents/_orchestrator.agent.md:120-154` - dark factory activation and
  visibility contract
* `.github/agents/_orchestrator.agent.md:179-180` - queued shipment state
  assessment
* `.github/agents/_orchestrator.agent.md:254` - current "highest-priority queued
  shipment" selection rule
* `.github/policies/workflow-policies.md:435-495` - P-017 dark factory autonomy
  contract and `DARK_MODE_SCOPE`
* `internal/cli/queue_cmd.go:54-55` - queue view default ordering intent
* `internal/core/queue.go:94,174-207,435-495` - queue ordering and dependency
  filtering
* `internal/core/shipment.go:54-60,178-200,568-625` - shipment membership
  storage and normalization
* `internal/core/shipment_lifecycle.go:43-83,135-161` - claim and ship consume
  manifest items as lifecycle scope
* `internal/db/schema.go:262-341` - current SQLite projection tables
* `internal/db/rehydration.go:117-171` - sync scans Markdown artifacts and
  rebuilds existing projections
* `.backlogit/queue/113-S.md:4-17` - current queued shipment manifest shape
