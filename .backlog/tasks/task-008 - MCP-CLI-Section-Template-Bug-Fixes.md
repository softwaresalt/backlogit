---
id: TASK-008
title: MCP & CLI Section/Template Bug Fixes
status: To Do
assignee: []
created_date: '2026-03-31 05:40'
labels:
  - epic
  - mcp
  - cli
  - templates
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Feature-level task grouping all MCP handler and CLI command bugs related to section handling, template service updates, file routing, and contract test coverage. These were originally filed as TASK-003 through TASK-007 during Copilot code review and need to be addressed before the next feature wave.

Covers:
- MCP section extraction and section param wiring
- Template service and CLI update command missing DB sync after section writes
- CLI move command not relocating files per registry.yaml
- Contract test assertions that only check key existence
<!-- SECTION:DESCRIPTION:END -->
