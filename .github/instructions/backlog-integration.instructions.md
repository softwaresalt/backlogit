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
| Create task | `backlogit_create_item` | `backlogit add` | Create a new task/artifact |
| List tasks | `backlogit_list_items` | `backlogit list` | List tasks with filters |
| Get task | `backlogit_get_item` | `backlogit get {id}` | Retrieve task details |
| Update task | `backlogit_update_item` | `backlogit update {id}` | Modify task fields |
| Move task | `backlogit_move_item` | `backlogit move {id} --status {status}` | Change task status |
| Search | `backlogit_search_items` | `backlogit search {query}` | Full-text search |
| Complete | `backlogit_move_item` | `backlogit move {id} --status done` | Mark task complete |

### Status Values

| Abstract Status | Tool-Specific Value |
|----------------|---------------------|
| Queued | `queued` |
| Active | `active` |
| Done | `done` |
| Blocked | `blocked` |

### Extended Operations (Tool-Dependent)

| Query SQL | `backlogit_query_sql` | `backlogit query {sql}` | Read-only SQL against index |
| Sync Index | `backlogit_sync_index` | `backlogit sync` | Rehydrate SQLite from Markdown |
| Append Comment | `backlogit_append_comment` | — | Add comment to item event log |
| Log Telemetry | `backlogit_log_telemetry` | — | Write agent telemetry event |
| Save Memory | `backlogit_save_memory` | — | Persist key-value agent memory |
| Create Checkpoint | `backlogit_create_checkpoint` | — | Save session state |
| List Checkpoints | `backlogit_list_checkpoints` | — | List session checkpoints |
| Get Queue | `backlogit_get_queue` | `backlogit queue view` | Priority-ordered work queue |
| Add Dependency | `backlogit_add_dependency` | `backlogit dep add` | Add dependency edge |
| Track Commit | `backlogit_track_commit` | — | Associate commit with artifact |
| Archive Item | `backlogit_archive_item` | — | Archive completed artifact |
| Fetch Stash | `backlogit_fetch_stash` | — | Get active stash entries |
| Stash | `backlogit_stash` | — | Defer work item to stash |
| Harvest Stash | `backlogit_harvest_stash` | — | Promote stash to work item |
| Create Shipment | `backlogit_create_shipment` | — | Create release shipment |
| Get Shipment | `backlogit_get_shipment` | — | Retrieve shipment details |
| List Shipments | `backlogit_list_shipments` | — | List shipments by status |
| Claim Shipment | `backlogit_claim_shipment` | — | Activate a queued shipment |
| Ship Shipment | `backlogit_ship_shipment` | — | Close released shipment |
| Poll Hook Events | `backlogit_poll_hook_events` | — | Poll for hook signal events |
| Ack Hook Events | `backlogit_ack_hook_events` | — | Acknowledge hook events |

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
