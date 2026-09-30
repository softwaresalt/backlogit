---
title: "Feature request: shipment manifest reshuffle and reconstitution"
doc_type: design
created: 2026-09-29
source: autoharness PR #466 (201-S re-split), backlogit 1.11.0
target: backlogit workspace (manual carry-over for feature consideration)
---

# Feature request: shipment manifest reshuffle and reconstitution

## Scenario

In autoharness, Stage harvested feature `195-F` into 12 tasks and assembled them
into a single queued shipment `201-S` (manifest: `195-F`, `195.001-T` .. `195.012-T`).
PR review then found the tasks broke the 2-hour task-granularity rule.
Re-planning produced 58 right-sized tasks, far too many for one shipment. The
operator decided to redistribute them across 6 shipments in a strict dependency
chain:

```text
190-S <- SHIP-2 <- SHIP-3 <- SHIP-4 <- SHIP-5 <- SHIP-6 <- 201-S <- 198-S
```

Closure safety (flat manifest scope, P-015) also requires that a feature never
span shipments. So each new shipment gets its own feature, and 9 of the
original 12 tasks must be re-parented from `195-F` into those features and
moved out of `201-S`.

All of this happens while `201-S` is still `queued` and on an unmerged
staging branch. No work has started, so reshaping it should be safe and routine.

## What backlogit 1.11.0 can and cannot do today

| Need | Available? | Notes |
|---|---|---|
| Add an item to a shipment | Yes | `shipment add` is idempotent, but refuses an item already in *another* shipment |
| Remove an item from a queued shipment | **No** | There is no `shipment remove` CLI or MCP operation |
| Move an item between shipments | **No** | `add` refuses it because the item is still in the source manifest, and nothing removes it |
| Reorder a manifest (feature first, then dependency order) | **No** | `custom_fields.items` is not editable through `backlogit update` |
| Split one shipment into several | **No** | Has to be composed by hand from create + add + (missing) remove |
| Re-parent a task to another feature | Partial | `adopt` re-parents, but the task gets a **new ID** and shipment manifests are **not** updated, so they keep pointing at the old ID |
| Return an item out of a shipment | Narrow | `return-blocked` handles only *blocked* items in *active* shipments. It is not a planning-time operation |

### Current workaround and its risks

1. Run `backlogit adopt` for each task that moves. This produces new IDs.
2. Hand-edit `custom_fields.items` in the shipment markdown to delete the old
   IDs and reorder the list.
3. `backlogit shipment add` each moved task to its new shipment.
4. `backlogit sync`, then `backlogit doctor` to look for dangling references.

Risks:

* **It bypasses validation.** Hand edits skip the core mutation layer:
  ownership checks, the one-shipment-per-item rule, and status rules.
* **It leaves no audit trail.** No event records who removed an item, when,
  or why, so later reconciliation (GI/GR) cannot explain the change.
* **State can be torn.** A failure midway leaves items in two shipments, in
  none, or referenced under a stale ID. `doctor` may not catch a manifest entry
  that names a renamed or nonexistent ID.
* **Renames are not propagated.** `adopt` renames IDs but does not update
  manifests, `blocks` dependencies, typed links, or stash provenance
  (`harvested_artifact_id`).
* **Agent friction.** Harness agents are told to use backlogit operations and
  not hand-edit backlog files. The missing operations force a policy exception
  every time a plan is re-split.

## Proposed capabilities

### 1. Primitive manifest operations (queued shipments only by default)

| Operation | CLI sketch | Behavior |
|---|---|---|
| Remove | `backlogit shipment remove <shipment> <item> --reason <text>` | Drop the item from the manifest. The item stays in the queue, unassigned. |
| Move | `backlogit shipment move <item> --from <S1> --to <S2> --reason <text>` | Atomic remove + add. |
| Reorder | `backlogit shipment reorder <shipment> --items <id,id,...>` or `--auto` | Set an explicit order. `--auto` puts the feature first, then tasks in `blocks` topological order, with subtasks after their parents. |

Each operation needs an MCP counterpart (`backlogit_remove_from_shipment`,
`backlogit_move_shipment_item`, `backlogit_reorder_shipment`), an event record,
and `--dry-run`. On `active` shipments, refuse by default. Allow an explicit
`--force` only with a reason, recorded in the event log.

### 2. Declarative reconstitution (the high-value feature)

```bash
backlogit shipment reconstitute --plan reshuffle.yaml [--dry-run] [--json]
```

```yaml
# reshuffle.yaml
source_shipments: [201-S]
shipments:
  - id: new            # backlogit allocates the ID
    title: "Engine-semantics gate and planner core"
    feature: { create: { title: "...", related_to: 195-F } }
    items: [195.001-T, 195.013-T, ...]
    blocks_on: [190-S]
  - id: 201-S          # existing shipment keeps its ID
    feature: 195-F
    items: [195.004-T, ...]
    blocks_on: [prev]  # symbolic reference to the previous entry
rewire:
  - { item: 198-S, blocks_on: [201-S] }
```

Required semantics:

* **All-or-nothing.** Validate the whole plan first, then apply it in one
  transaction, or roll everything back.
* **Validation:**
  * every item lands in exactly one shipment;
  * no feature spans shipments;
  * every item `blocks` dependency points to the same or an earlier shipment;
  * the shipment-level `blocks` graph is acyclic;
  * every new shipment has an explicit sequencing edge (none `unsequenced`);
  * source shipments are `queued` (or `--force` with a reason).
* **Re-parenting included.** Moving a task under a new feature does the adopt
  step and propagates the ID change (see §3).
* **Ordering.** Apply `--auto` manifest ordering unless the plan sets an order.
* **Reporting.** `--dry-run --json` returns the full diff: created IDs, the
  mapping from old ID or plan label to real ID, manifest before and after,
  dependency edges added or removed.
* **Audit.** One reconstitution event that links all child events, with the
  plan file hash and the reason.

### 3. Rename propagation for `adopt`

When `adopt` assigns a new ID, update every reference atomically:

* shipment manifests (`custom_fields.items`);
* `blocks` dependencies, in both directions;
* typed links (`related_to`, `informs`, and so on);
* stash provenance (`harvested_artifact_id`);
* commit tracking and comments where they are indexed.

Record the old-to-new ID alias so lookups by the old ID resolve, or fail with a
clear "renamed to X" message.

Alternatively, offer `adopt --keep-id` when the ID scheme allows re-parenting
without renumbering.

### 4. Integrity checks in `doctor`

* A manifest entry names an ID that doesn't exist or was renamed.
* An item appears in more than one shipment manifest.
* A feature's tasks are spread across more than one shipment.
* An item `blocks` dependency points into a *later* shipment in the
  shipment DAG (a forward edge).
* A queued shipment has no sequencing edge and no root declaration.

## 5. Stash provenance correction gap

Observed in autoharness PR #466 (review thread `PRRT_kwDORzpWpM6ndtZm`):

* Stash entry `5CA04218` was harvested into task `196.001-T`. That task, its
  feature `196-F`, and its shipment `202-S` were deleted before merge.
* backlogit later **reused** those IDs for unrelated new items. The archived
  stash record (`archive/stash.jsonl`, `harvested_artifact_id: 196.001-T`) now
  points at the wrong artifact.
* `backlogit stash correct` could not repair it. It requires the target
  artifact to carry `custom_fields.source_stash_id: 5CA04218`, and no CLI or
  MCP operation sets `source_stash_id` on an existing item. Hand-editing the
  field would invent provenance that no harvest produced, so it was rejected.
  The correction exists only as prose in the plan and in session memory.

Proposed fix:

* **Operator-reasoned correction.** Let `stash correct` accept a target that
  lacks `source_stash_id` when the operator supplies `--reason`. Record the
  reason, the old and new target, and the actor as an audited event.
* **Or `stash repoint`.** A dedicated command that repoints an archived stash
  entry's `harvested_artifact_id` to one or more delivery artifacts, with the
  same audit event and a `--dry-run --json` preview.
* **No recycling of IDs with history.** ID allocation should never reuse the
  ID of a deleted item that still has an event log or stash provenance
  (`harvested_artifact_id`) pointing at it. At minimum, `doctor` should flag
  a stash record whose harvested target was deleted or reused.

## Acceptance criteria (suggested)

1. The 201-S scenario above (1 queued shipment → 6 chained shipments, with 9
   re-parented tasks) can be done with one `reconstitute --dry-run` and one
   `reconstitute` call, with no hand edits, and ends with `doctor` clean.
2. A failure injected midway leaves the workspace identical to its
   pre-command state.
3. Every mutation emits an event that GI/GR shipment reconciliation can use to
   explain manifest differences.
4. CLI and MCP surfaces are at parity, and `--json` output is stable for agents.
5. `remove`, `move` and `reconstitute` refuse `active`/`shipped`/`abandoned`
   shipments unless given `--force` with a reason.

## Why it matters

Re-planning after review is normal: granularity findings, scope splits,
dependency discoveries. Without manifest-level operations, every re-plan
becomes a manual, unaudited edit of backlog files, which is exactly what the
backlogit-first harness policy is meant to prevent.
