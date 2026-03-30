---
id: TASK-001.08.06
title: Write MCP contract tests
status: Done
assignee: []
created_date: '2026-03-30 01:44'
labels: []
dependencies: []
parent_task_id: TASK-001.08
priority: high
ordinal: 8600
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `tests/contract/tools_test.go` with contract tests for all 10 MCP tools:

1. Validate tool registration: all 10 tools discoverable via server.ListTools()
2. For each tool, test valid input → expected JSON output schema
3. For each tool, test missing required params → validation_failed error
4. For each tool, test pre-init state → workspace_not_initialized error
5. Specific contract: backlogit_query_sql with forbidden SQL → query_rejected error

Create `tests/contract/resources_test.go` with contract tests for both MCP resources:
1. backlogit://config returns valid YAML
2. backlogit://schema returns valid SQL DDL
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Contract tests validate input schema for all 10 MCP tools
- [ ] #2 Contract tests verify JSON output format for successful tool calls
- [ ] #3 Contract tests verify error response format for workspace_not_initialized
- [ ] #4 Contract tests verify error response format for validation_failed
- [ ] #5 All contract tests pass via go test ./tests/contract/...
<!-- AC:END -->
