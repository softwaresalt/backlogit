---
description: "Backlog tool integration instructions for the installed backlogit workflow surface"
applyTo: '**'
---

# Backlog Integration Instructions

This workspace uses **backlogit** for structured backlog management. Use the
backlogit MCP and CLI surface for task tracking instead of creating ad hoc task
lists or standalone markdown trackers.

## Tool configuration

| Setting | Value |
|---|---|
| Tool | backlogit |
| Directory | `.backlogit/` |
| Access | both |
| Registry | `.autoharness/backlog-registry.yaml` |

## Core operations

| Operation | MCP tool | CLI command | Purpose |
|---|---|---|---|
| Create task | `backlogit_create_item` | `backlogit add --type <artifact_type> --title <title>` | Create a new artifact |
| List tasks | `backlogit_list_items` | `backlogit list` | List tasks with filters |
| Get task | `backlogit_get_item` | `backlogit get <id>` | Retrieve task details |
| Update task | `backlogit_update_item` | `backlogit update <id>` | Modify task fields |
| Move task | `backlogit_move_item` | `backlogit move <id> --status <status>` | Change task status |
| Search tasks | `backlogit_search_items` | `backlogit search <query>` | Full-text search |
| Complete task | `backlogit_move_item` | `backlogit move <id> --status done` | Mark work complete |

## Status mapping

| Abstract status | backlogit value |
|---|---|
| To Do | `queued` |
| In Progress | `active` |
| Done | `done` |
| Blocked | `blocked` |

## Extended operations

| Operation | MCP tool | CLI command |
|---|---|---|
| Query state | `backlogit_query_sql` | `backlogit query <sql>` |
| Sync index | `backlogit_sync_index` | `backlogit sync` |
| Append comment | `backlogit_append_comment` | not applicable |
| Save memory | `backlogit_save_memory` | not applicable |
| Create checkpoint | `backlogit_create_checkpoint` | not applicable |
| Get queue | `backlogit_get_queue` | `backlogit queue view` |
| Add dependency | `backlogit_add_dependency` | `backlogit dep add <id> <depends_on> --type <dep_type>` |
| Remove dependency | `backlogit_remove_dependency` | `backlogit dep remove <id> <depends_on>` |
| Get dependencies | `backlogit_get_dependencies` | `backlogit dep list <id>` |
| Track commit | `backlogit_track_commit` | `backlogit update <id> --commit <sha>` |

## Preferred workflow patterns

### Create work

When creating a work item, prefer the MCP tool and provide explicit type, title,
status, and description. Use `parent_id` when the artifact belongs under an
existing feature, task, or review lineage.

### Claim work

Move an item from `queued` to `active` rather than silently assuming ownership.

### Complete work

When work is done, move it to `done` and record commit associations when the
change resulted in a real commit.

### Inspect ready work

Use queue-aware operations or targeted SQL rather than scanning many markdown
files in `.backlogit/queue/`.

### Preserve traceability

When the registry advertises support for comments, checkpoints, memory, or
commit tracking, prefer those operations over free-form notes.

## Rules

1. Always use backlogit for backlog state changes when the tool surface supports the action.
2. Prefer MCP tools over CLI commands when both are available because the MCP surface returns structured results.
3. Use queue and dependency operations for sequencing work instead of hiding critical order only in prose.
4. Refresh the backlog index after out-of-band edits before relying on SQL or queue results.
5. Follow `.github/instructions/backlogit.instructions.md` in addition to this generic integration guide when you need backlogit-specific workflow behavior.