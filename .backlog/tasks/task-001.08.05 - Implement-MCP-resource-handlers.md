---
id: TASK-001.08.05
title: Implement MCP resource handlers
status: To Do
assignee: []
created_date: '2026-03-30 01:44'
labels: []
dependencies: []
parent_task_id: TASK-001.08
priority: high
ordinal: 8500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/mcp/resources.go` with:

1. `backlogit://config` resource — returns the workspace `config.yaml` content as `application/x-yaml` MIME type
2. `backlogit://schema` resource — returns the SQLite table definitions (CREATE TABLE statements) as `text/plain`
3. `registerResources(s *server.MCPServer)` method on Server struct

Both resources validate workspace existence before returning content.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit://config resource returns workspace config.yaml as YAML text
- [ ] #2 backlogit://schema resource returns SQLite CREATE TABLE statements as text
- [ ] #3 Resources return error when workspace is not initialized
- [ ] #4 Tests verify resource content format and error handling
<!-- AC:END -->
