---
title: "Orphaned Tasks: Missing Parent Features, Non-Hierarchical IDs, and Incomplete Shipment Manifests"
problem_type: workflow_issue
category: workflow_issue
component: task_manager
root_cause: schema_mismatch
resolution_type: code_fix
severity: high
message: "Level-2 tasks created with null parent_id; adoption did not rewrite IDs or filenames; shipment manifests omitted parent features"
file_path: "internal/core/artifacts.go"
resolved: true
tags:
  - hierarchy
  - orphan
  - parent_id
  - hierarchical-ids
  - filename-convention
  - shipment-manifest
  - stash-harvest
  - wit-enforcement
  - task-creation
  - validation
date: 2026-04-10
---

## Problem

This document covers three related hierarchy violations that compound on each
other and must all be resolved together:

1. Nine tasks (004-T through 012-T) were created as root-level items with
   `parent_id: null` despite the WIT configuration defining tasks as
   `hierarchy_level: 2`, which requires a level-1 parent feature.

2. After those tasks were adopted under parent features, their IDs and
   filenames were not updated to reflect the hierarchical naming convention.
   A task adopted under feature 025-F must be identified as `025.012-T`, not
   `012-T`. The filename must match the ID.

3. Shipment manifests referenced the orphaned task IDs and did not include the
   parent feature artifacts, breaking the contract that a shipment scope is
   defined at the feature level with tasks listed beneath it.

## Symptoms

* All queued tasks appeared at hierarchy level 1 in the queue view.
* Querying `SELECT id, parent_id FROM items WHERE artifact_type='task' AND status='queued'` returned null for every `parent_id`.
* After adoption, task IDs remained flat (e.g. `012-T`) instead of hierarchical (e.g. `025.012-T`). The index and the filesystem were inconsistent with the hierarchy the `parent_id` field declared.
* Filenames in `.backlogit/queue/` did not match the required `{parent}.{NNN}-T.md` convention.
* Shipment manifests listed bare task IDs without their parent features, making the shipment scope ambiguous and the manifest non-canonical.

## What Did Not Work

The creation, adoption, and shipment workflows all had gaps:

1. **No validation at creation time.** `backlogit_create_item` and
   `backlogit_harvest_stash` accept `parent_id` as optional for all artifact
   types, including level-2 tasks. No code path checks `hierarchy_level >= 2`
   against a null `parent_id`.

2. **Harvest workflow is parent-unaware.** Stash entries are harvested into
   tasks one at a time without first verifying or creating a parent feature.
   The harvest skill and MCP tool both allow direct task creation from stash
   without hierarchy scaffolding.

3. **Agent instructions lacked hierarchy guidance.** No instruction file or
   skill protocol required agents to provide `parent_id` when creating tasks
   or to create parent features before harvesting child tasks.

4. **`backlogit_adopt_item` only sets `parent_id`; it does not rename.**
   After adoption, the item's ID and filename remained in their original flat
   form (`012-T`, `012-T.md`). The tool does not rewrite the ID to the
   hierarchical format (`{parent}.{NNN}-T`) or rename the file to match. This
   left the index, filesystem, and log files in an inconsistent state that
   required a separate manual migration.

5. **Shipment manifests must include parent features, not just child tasks.**
   Adding tasks to a shipment without their parent features makes the manifest
   incomplete. The shipment scope is defined at the feature level; tasks are
   subordinate detail. Both must be present for the manifest to accurately
   represent the shipment boundary.

## Solution

### Immediate remediation

Created 4 parent features grouped by theme and adopted all 9 orphaned tasks:

| Feature | Child Tasks |
|---|---|
| 022-F Stash CLI and lifecycle improvements | 022.004-T, 022.005-T, 022.006-T, 022.007-T |
| 023-F Event traceability and observability | 023.008-T |
| 024-F Agent developer experience | 024.009-T, 024.010-T |
| 025-F Workspace governance and integrity | 025.011-T, 025.012-T |

After adoption, the IDs and filenames were not automatically updated, requiring
a manual migration:

```powershell
# For each old-id -> new-id mapping:
# 1. Update frontmatter id field and rename queue .md file
$content = Get-Content "$queue\$old.md" -Raw
$content = $content -replace "(?m)^id: $old$", "id: $new"
Set-Content -Path "$queue\$old.md" -Value $content -NoNewline
Rename-Item -Path "$queue\$old.md" -NewName "$new.md"

# 2. Rename the JSONL log file
Rename-Item -Path "$logs\$old.jsonl" -NewName "$new.jsonl"
```

After renaming files and updating frontmatter, run `backlogit sync` to rebuild
the index with the corrected IDs.

Shipment manifests were then updated to include parent features alongside child
tasks, and stale task IDs were replaced with the corrected hierarchical IDs:

```text
# 005-S before:   004-T, 005-T, 006-T, 007-T, 009-T, 010-T, 020-F, 015.009-T, 015.011-T
# 005-S after:    022-F, 022.004-T, 022.005-T, 022.006-T, 022.007-T,
#                 024-F, 024.009-T, 024.010-T,
#                 020-F, 015.009-T, 015.011-T
```

### Systemic prevention

Six areas require attention to prevent recurrence:

1. **Tooling validation.** Add a `validateHierarchyConstraints()` check in
   `backlogit_create_item` and `backlogit_harvest_stash` that rejects level-2+
   items when `parent_id` is null.

2. **`adopt_item` must rewrite IDs and filenames.** When `backlogit_adopt_item`
   sets a new `parent_id`, it must also rewrite the item's ID to the
   `{parent}.{NNN}-T` format, rename the `.md` file, rename the `.jsonl` log
   file, update all references in shipment manifests, and trigger an index sync.

3. **Harvest workflow ordering.** Modify the harvest skill to scan stash
   entries, group them thematically, create parent features first, then
   harvest child tasks with `parent_id` set so IDs are correct from the start.

4. **Agent instructions.** Add explicit hierarchy rules to
   `backlogit.instructions.md` and the harvest skill: "When creating tasks,
   you MUST provide a `parent_id` referencing an existing feature. Create the
   parent feature first if one does not exist."

5. **Shipment manifest contract.** When adding tasks to a shipment, always
   add the parent feature first. The manifest must represent the complete
   feature scope, not just leaf tasks.

6. **Doctor tooling and contract tests.** Implement `backlogit doctor
   --check=orphans` to detect level-2+ items with null `parent_id` or
   non-hierarchical IDs. Add contract tests verifying creation of level-2
   items fails without `parent_id`, adoption rewrites IDs atomically, and
   shipment manifests include parent features.

## Why This Works

The problem stems from three gaps between configuration and enforcement:

1. **Creation gap.** The WIT metadata declares `task.hierarchy_level = 2` but
   no code path enforces that contract during creation. The configuration is
   descriptive, not prescriptive.

2. **Adoption gap.** `backlogit_adopt_item` sets `parent_id` as a metadata
   field but does not cascade the rename that the ID format contract requires.
   An item's ID encodes its position in the hierarchy (`{parent}.{NNN}-T`), so
   changing the parent without changing the ID leaves the system in a
   contradictory state: the metadata says one thing, the ID says another.

3. **Manifest gap.** Shipments are scoped at the feature level. Listing only
   child tasks in a manifest without their parent features makes the scope
   ambiguous and loses the connection between shipment and feature delivery.

The immediate fixes restore consistency by correcting IDs, renaming files,
syncing the index, and updating manifests. The systemic fixes close each gap at
its source.

## Prevention

* Add hierarchy validation to `backlogit_create_item`: reject level-2+ items with null `parent_id`
* Add hierarchy validation to `backlogit_harvest_stash`: require `parent_id` for task-kind harvests
* Fix `backlogit_adopt_item` to atomically rewrite the item ID, rename `.md` and `.jsonl` files, update shipment manifests, and sync the index
* Update `backlogit.instructions.md` with explicit hierarchy enforcement rules
* Update the harvest skill protocol to create parent features before child tasks
* When adding items to a shipment, always add parent features before their child tasks
* Implement `backlogit doctor --check=orphans` for periodic detection of level-2+ items with null `parent_id` or non-hierarchical IDs
* Add contract tests for hierarchy constraint enforcement at creation time and ID rewriting on adoption

## Related Solutions

* [Stash staleness requires custom scripting](stash-staleness-requires-custom-scripting-2026-04-09.md):
  documents harvest workflow gaps including missing CRUD operations
* [Unstaged MCP tool registrations](unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md):
  documents `backlogit_adopt_item` tool used for orphan remediation
* [Stable contract before two-agent adoption](stable-contract-before-two-agent-adoption-2026-04-05.md):
  documents the four-stage workflow pipeline and tool semantics
