---
id: TASK-008.06
title: 'Strengthen contract tests: exercise MCP handlers with real assertions'
status: Done
assignee: []
created_date: '2026-03-31 05:42'
updated_date: '2026-03-31 22:13'
labels:
  - testing
  - contract
  - copilot-review
dependencies: []
references:
  - tests/contract/tools_expansion_test.go
parent_task_id: TASK-008
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Copilot review finding D5: `tests/contract/tools_expansion_test.go:60` — contract tests for expanded MCP tools only check that certain JSON keys exist in the response. They do not invoke the actual tool handlers against a real workspace, so they provide weak coverage and would not catch handler regressions.

Fix required:
1. Set up a temporary workspace with config.WriteDefaults in each contract test.
2. Invoke each MCP tool handler directly (or via the MCP server dispatch) rather than constructing mock responses.
3. Assert on actual response values: artifact IDs, titles, status values, section content — not just key presence.
4. Add negative-path assertions: invalid type, missing required params, section-not-found, etc.
<!-- SECTION:DESCRIPTION:END -->
