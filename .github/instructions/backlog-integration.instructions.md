---
description: "Backlog tool integration instructions — teaches agents how to interact with the installed backlog management tool using abstracted operations"
applyTo: '**'
---

# Backlog Integration Instructions

This workspace uses **backlogit** for structured backlog management. All agents MUST use the backlog tool for task tracking rather than creating ad-hoc markdown files or static task lists.

## Tool Configuration

| Setting | Value |
|---------|-------|
| Tool | backlogit |
| Directory | `.backlogit/` |
| Access | both |
| Registry | `.autoharness/backlog-registry.yaml` |

## Operation Reference

Use these operations for all backlog interactions. The operation names are abstract — the actual tool names and parameters are mapped through the backlog registry.

### Core Operations (All Tools)

| Operation | MCP Tool | CLI Command | Purpose |
|-----------|----------|-------------|---------|
| Create task | `backlogit_create_item` | `backlogit add --type {{artifact_type}} --title {{title}}` | Create a new task/artifact |
| List tasks | `backlogit_list_items` | `backlogit list` | List tasks with filters |
| Get task | `backlogit_get_item` | `backlogit get {{id}}` | Retrieve task details |
| Update task | `backlogit_update_item` | `backlogit update {{id}}` | Modify task fields |
| Move task | `backlogit_move_item` | `backlogit move {{id}} --status {{status}}` | Change task status |
| Search | `backlogit_search_items` | `backlogit search {{query}}` | Full-text search |
| Complete | `backlogit_move_item` | `backlogit move {{id}} --status done` | Mark task complete |

### Status Values

| Abstract Status | Tool-Specific Value |
|----------------|---------------------|
| Queued | `queued` |
| Active | `active` |
| Done | `done` |
| Blocked | `blocked` |

### Extended Operations (Tool-Dependent)

| Operation | MCP Tool | CLI Command | Purpose |
|---|---|---|---|
| `ack_hook_events` | `backlogit_ack_hook_events` | `` | ack hook events |
| `add_dependency` | `backlogit_add_dependency` | `backlogit dep add {{task_id}} {{depends_on}} --type {{dep_type}}` | add dependency |
| `add_link` | `backlogit_add_link` | `backlogit link add {{source_id}} {{target_id}} --type {{link_type}}` | add link |
| `add_to_shipment` | `backlogit_add_to_shipment` | `` | add to shipment |
| `adopt_item` | `backlogit_adopt_item` | `backlogit adopt {{item_id}} --parent {{new_parent_id}}` | adopt item |
| `append_comment` | `backlogit_append_comment` | `` | append comment |
| `archive_item` | `backlogit_archive_item` | `backlogit archive {{id}}` | archive item |
| `claim_shipment` | `backlogit_claim_shipment` | `backlogit shipment claim {{id}}` | claim shipment |
| `cleanup_checkpoints` | `backlogit_cleanup_checkpoints` | `backlogit checkpoint cleanup` | cleanup checkpoints |
| `create_checkpoint` | `backlogit_create_checkpoint` | `` | create checkpoint |
| `create_shipment` | `backlogit_create_shipment` | `backlogit shipment create --title {{title}} --items {{items}}` | create shipment |
| `deliberate` | `backlogit_deliberate` | `` | deliberate |
| `doctor` | `backlogit_doctor` | `backlogit doctor` | doctor |
| `export_command_map` | `backlogit_export_command_map` | `` | export command map |
| `fetch_stash` | `backlogit_fetch_stash` | `backlogit stash list` | fetch stash |
| `get_checkpoint` | `backlogit_get_checkpoint` | `backlogit checkpoint get {{filename}}` | get checkpoint |
| `get_dependencies` | `backlogit_get_dependencies` | `backlogit dep list {{task_id}}` | get dependencies |
| `get_links` | `backlogit_get_links` | `backlogit link list {{id}}` | get links |
| `get_metadata_catalog` | `backlogit_get_metadata_catalog` | `` | get metadata catalog |
| `get_queue` | `backlogit_get_queue` | `backlogit queue view` | get queue |
| `get_shipment` | `backlogit_get_shipment` | `backlogit shipment get {{id}}` | get shipment |
| `get_version` | `backlogit_get_version` | `` | get version |
| `get_wit_metadata` | `backlogit_get_wit_metadata` | `` | get wit metadata |
| `harvest_stash` | `backlogit_harvest_stash` | `backlogit stash harvest` | harvest stash |
| `list_checkpoints` | `backlogit_list_checkpoints` | `backlogit checkpoint list` | list checkpoints |
| `list_shipments` | `backlogit_list_shipments` | `backlogit shipment list` | list shipments |
| `list_templates` | `backlogit_list_templates` | `` | list templates |
| `list_types` | `backlogit_list_types` | `` | list types |
| `log_telemetry` | `backlogit_log_telemetry` | `` | log telemetry |
| `merge_sync` | `backlogit_merge_sync` | `` | merge sync |
| `poll_hook_events` | `backlogit_poll_hook_events` | `` | poll hook events |
| `query` | `backlogit_query_sql` | `backlogit query {{sql}}` | query |
| `remove_dependency` | `backlogit_remove_dependency` | `backlogit dep remove {{task_id}} {{depends_on}}` | remove dependency |
| `remove_link` | `backlogit_remove_link` | `backlogit link remove {{source_id}} {{target_id}} --type {{link_type}}` | remove link |
| `resolve_checkpoint` | `backlogit_resolve_checkpoint` | `backlogit checkpoint resolve {{filename}}` | resolve checkpoint |
| `return_blocked` | `backlogit_return_blocked` | `backlogit shipment return-blocked --shipment {{shipment_id}} --item {{item_id}} --reason {{reason}}` | return blocked |
| `save_memory` | `backlogit_save_memory` | `` | save memory |
| `ship_shipment` | `backlogit_ship_shipment` | `backlogit shipment ship {{id}}` | ship shipment |
| `stash` | `backlogit_stash` | `backlogit stash add --text {{text}}` | stash |
| `stash_archive` | `backlogit_stash_archive` | `` | stash archive |
| `stash_edit` | `backlogit_stash_edit` | `` | stash edit |
| `stash_get` | `backlogit_stash_get` | `` | stash get |
| `stash_remove` | `backlogit_stash_remove` | `` | stash remove |
| `sync_index` | `backlogit_sync_index` | `backlogit sync` | sync index |
| `telemetry_harvest` | `backlogit_telemetry_harvest` | `` | telemetry harvest |
| `track_commit` | `backlogit_track_commit` | `backlogit update {{task_id}} --commit {{sha}}` | track commit |

## Agent Workflow Patterns

### Creating a Task

```text
Call backlogit_create_item with:
  title: "Task title"
  artifact_type: "task"
  status: "queued"
  description: "Task description"
  parent_id: "parent-task-id"  (if applicable)
  labels: "label1,label2"      (if applicable)
```

### Claiming a Task (Status → Active)

```text
Call backlogit_move_item with:
  id: "task-id"
  status: "active"
```

### Completing a Task

```text
Call backlogit_move_item with:
  id: "task-id"
```

### Listing Ready Tasks

```text
Call backlogit_list_items with:
  status: "queued"
```

### Adding a Label

```text
Call backlogit_update_item with:
  id: "task-id"
  labels: "existing-label,harness-ready"
```

## Advanced Patterns When Supported

If the registry advertises advanced features, prefer them over ad hoc workarounds:

* **Token-efficient lookup** — use the query operation when `features.sql_query` is true
* **Ready-work selection** — use queue-aware operations when `features.queue` is true
* **Dependency reasoning** — use dependency operations when `features.dependencies` is true
* **Agent continuity** — use memory and checkpoint operations when `features.memory` or `features.checkpoints` are true
* **Traceability** — use comment or commit-tracking operations when `features.comments` or `features.commit_tracking` are true
* **Index freshness** — use sync / rehydration operations when the workspace was edited outside normal mutation tools

If a tool-specific overlay instruction file is installed (for example,
`.github/instructions/backlogit.instructions.md`), follow it in addition to this generic guide.

## Rules

1. **Always use the backlog tool** for task management. Do not create markdown task files outside the `.backlogit/` directory.
2. **Use abstract status values** mapped through the registry, not hardcoded strings.
3. **Check the registry** (`.autoharness/backlog-registry.yaml`) for the exact field names and operation parameters when unsure.
4. **Prefer MCP tools** over CLI when both are available — MCP returns structured JSON, CLI returns human-readable text.
5. **Feature gating**: Before calling an extended operation, verify the feature is supported by checking the `features` section in the registry.

Generated by autoharness | Template: backlog-integration.instructions.md.tmpl
