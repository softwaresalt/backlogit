---
id: TASK-008.02
title: Wire sections param through handleCreateItem to template service
status: Done
assignee: []
created_date: '2026-03-31 05:41'
updated_date: '2026-03-31 22:13'
labels:
  - mcp
  - sections
  - templates
dependencies: []
parent_task_id: TASK-008
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Fix handleCreateItemSections stub (always returns nil), construct live template service in NewServer, and wire sections param through handleCreateItem to write section content. Currently the sections parameter is accepted by the MCP tool schema but silently ignored during item creation.
<!-- SECTION:DESCRIPTION:END -->
