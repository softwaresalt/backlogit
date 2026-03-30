---
id: TASK-001.11
title: MCP Environment Registration
status: To Do
assignee: []
created_date: '2026-03-30 01:37'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.02
parent_task_id: TASK-001
priority: low
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `backlogit mcp init [environment]` to auto-inject the backlogit MCP server configuration into various IDE and agent configuration files.

Supported environments: VS Code (`.vscode/mcp.json`), GitHub Copilot (`.copilot/mcp-config.json`), Cursor (`.cursor/mcp.json`), Claude Code (CLI execution). Detects existing config, safely injects without overwriting, handles missing directories, prevents duplicates on repeated runs.
<!-- SECTION:DESCRIPTION:END -->
