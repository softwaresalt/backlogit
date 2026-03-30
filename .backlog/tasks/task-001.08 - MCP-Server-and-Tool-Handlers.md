---
id: TASK-001.08
title: MCP Server and Tool Handlers
status: To Do
assignee: []
created_date: '2026-03-30 01:37'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.05
  - TASK-001.06
  - TASK-001.07
parent_task_id: TASK-001
priority: high
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement the MCP stdio server with all 10 tool handlers, resource handlers, and error helpers. Wire tools to core business logic, database, and event packages.

Per review P1-03: Uses `Server` struct with DI (holds `*sql.DB`, `*WorkspaceConfig`, workspace path). No global mutable state.
Per review P1-05: One `*sql.DB` opened at startup, stored in Server struct, closed on shutdown.
Per review P2-07: Error response helpers use `json.Marshal` for safe escaping.
Per review P2-08: Prompt templates deferred to follow-up (not included).

Tools: `backlogit_create_item`, `backlogit_update_item`, `backlogit_get_item`, `backlogit_get_item_history`, `backlogit_query_sql`, `backlogit_sync_index`, `backlogit_append_comment`, `backlogit_log_telemetry`, `backlogit_save_memory`, `backlogit_create_checkpoint`

Resources: `backlogit://config`, `backlogit://schema`
<!-- SECTION:DESCRIPTION:END -->
