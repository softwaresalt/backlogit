---
title: "Post-Merge Closure Must Archive Source Stash Entries and Deliberations"
description: "Protocol for archiving source stash entries and deliberations during ship post-merge closure to prevent stale-scope accumulation."
problem_type: workflow_issue
category: workflow_issue
component: ship_agent
root_cause: missing_protocol_step
resolution_type: agent_protocol
severity: medium
message: "ShipShipment archives implemented features and tasks but ship agent protocol never traces back to source stash entries or deliberations, leaving orphaned artifacts that clutter future triage."
file_path: ".github/agents/ship.agent.md"
resolved: true
tags: [stash, ship-cycle, closure, archival, source-artifacts, deliberation, workflow-hygiene]
date: 2026-04-20
---

## Post-Merge Closure Must Archive Source Stash Entries and Deliberations

## Problem

When the Ship agent completes post-merge closure for a shipment, it archives the
implemented features, tasks, and the shipment artifact itself. However, it does
not trace back to the **source artifacts** that originated the work:

* the stash entry that was harvested into the feature (linked via
  `custom_fields.source_stash_id`)
* the deliberation artifact that shaped the approach (linked via
  `custom_fields.source_deliberation_id`)

These source artifacts remain active indefinitely after their scope ships, slowly
accumulating as orphaned records that confuse future Stage triage.

## Symptoms

* `backlogit_fetch_stash` returns entries with state `harvested` whose
  corresponding work has already shipped (e.g., `backlogit_query_sql` shows
  `stash_links.item_id` pointing to archived features).
* Stage triage sweeps encounter deliberation artifacts (e.g., `033-DL`,
  `034-DL`) in the queue even though their originating shipments closed months
  earlier.
* Stale-scope sweeps during Stage sessions repeatedly surface the same archived
  deliberations and harvested stash entries as false positives.

## Evidence

During the 2026-04-19 Stage session for shipment 036-S, three stale-scope sweeps
identified source artifacts that should have been archived when their shipments
closed:

1. `033-DL` (Event Traceability deliberation) — still queued after `006-S`
   shipped its scope.
2. `031-DL` (Telemetry Pipeline Enhancements deliberation) — still queued after
   shipment `031-S` shipped (note: `031-S` title was misleading — its
   `custom_fields.items` revealed the actual shipped scope).
3. `034-DL` and `035-DL` (Stash Archive deliberations) — still queued after
   `014-S` shipped on 2026-04-12.

Additionally, a `backlogit_query_sql` audit of `stash_entries WHERE state =
'harvested'` returned 15+ entries, all corresponding to work that had shipped.

## What Did Not Work

* **Relying on `ShipShipment` to clean up source artifacts.** The Go function
  archives the feature/task/shipment hierarchy but has no visibility into stash
  or deliberation provenance.
* **Assuming stash entries auto-expire.** Harvested entries are not automatically
  removed — they require an explicit `backlogit_stash_remove` call to transition
  to `removed` state (which archives them to `.backlogit/archive/stash.jsonl`).
* **Deferring cleanup to Stage.** Stage triage catches these as anomalies but
  wastes triage cycles on artifacts that should never appear in the active queue.

## Solution

Add a source artifact archival step to the Ship agent's post-merge closure
protocol (`.github/agents/ship.agent.md`). After invoking `operational-closure`,
and before the documentation evaluation step, the agent now:

1. For each shipped feature, reads `custom_fields.source_stash_id` and calls
   `backlogit_stash_remove` with reason `superseded by {shipment_id}`.
2. Reads `custom_fields.source_deliberation_id`, verifies the deliberation exists
   via `backlogit_get_item`, and calls `backlogit_archive_item` if it does.
3. Handles idempotently: already-removed stash entries and already-archived
   deliberations are skipped and logged rather than causing errors.
4. Broadcasts the cleanup count and logs archived IDs in the closure artifact.

The operational-closure skill (`operational-closure/SKILL.md`) was also updated
to include a **Source Artifact Cleanup** checklist item in Step 2, so closure
artifacts document which source stash entries and deliberations were archived
during this closure.

### After (correct protocol)

```text
# During post-merge closure, after operational-closure invocation:

backlogit_get_item(id="026-F")  # read source_stash_id from custom_fields
# → custom_fields.source_stash_id = "B155D9DA"

backlogit_stash_remove(stash_id="B155D9DA")
# → removes and archives entry, returns {state: "removed"}

backlogit_get_item(id="026-F")  # read source_deliberation_id
# → custom_fields.source_deliberation_id = "026-DL" (if present)

backlogit_get_item(id="026-DL")  # verify deliberation exists
backlogit_archive_item(id="026-DL")
# → archives deliberation, returns {status: "archived"}

# broadcast:
[SHIP] Source artifacts archived: 1 stash, 1 deliberation
```

## Why This Works

* `backlogit_stash_remove` already archives removed entries to
  `.backlogit/archive/stash.jsonl` and sets DB state to `removed`. Using it for
  superseded entries is the correct semantic — they are consumed, not deleted.
* `custom_fields.source_stash_id` and `source_deliberation_id` are already
  populated by the harvest workflow. The archival step closes the loop by
  consuming them.
* Idempotent handling means the step is safe to run multiple times without
  double-archiving or breaking the closure flow.

## Prevention

* The Ship agent post-merge closure protocol now includes source artifact
  archival as step 2 (before documentation evaluation).
* The operational-closure skill checklist includes a **Source Artifact Cleanup**
  section so closure artifacts document the archived source IDs for auditability.
* During the 036-S ship cycle, a one-time retroactive cleanup removed all 15+
  harvested stash entries corresponding to already-shipped work.

**Ship agent checklist addition (now in protocol):**

```text
[ ] Source stash entries for shipped features are removed via backlogit_stash_remove
[ ] Source deliberation artifacts for shipped features are archived via backlogit_archive_item
[ ] Archived source artifact IDs logged in closure artifact follow-up section
[ ] Broadcast: [SHIP] Source artifacts archived: {stash_count} stash, {delib_count} deliberations
```

## Related Solutions

* [`docs/compound/workflow-issues/ship-agent-unrealized-follow-up-stash-2026-04-12.md`](../workflow-issues/ship-agent-unrealized-follow-up-stash-2026-04-12.md) — agents must actually call tools, not just report that they did
* [`docs/compound/workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md`](../workflow-issues/shipment-ready-before-stage-gates-2026-04-10.md) — workflow gate discipline
* [`docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md`](../workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md) — ship agent protocol compliance patterns
