---
id: TASK-010.02.03
title: Write process and workflow documentation
status: To Do
assignee: []
created_date: '2026-04-01 22:31'
labels:
  - docs
dependencies:
  - TASK-010.02.01
parent_task_id: TASK-010.02
priority: medium
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `docs/workflow.md` documenting how backlogit integrates into developer and agent workflows.

Content to include:
- Workspace lifecycle: `init` → `add` → `list`/`query` → `update`/`move` → `archive`
- Developer workflow: using CLI commands for daily task management
- Agent workflow: how AI agents connect via MCP stdio and use the 21 tools
- CQRS in practice: when to use Markdown files vs. SQL queries vs. JSONL events
- Configuration walkthrough: `config.yaml`, `registry.yaml`, `hooks.yaml`
- WIT type system: how artifact types, custom fields, and templates work together
- Dependency graph and work queue: prioritization and blocking workflows
- Integration with Git: how `.backlogit/` travels with the codebase

Files to create:
- `docs/workflow.md` (new)

Verification: Workflow examples use actual CLI commands and MCP tool names from the codebase.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Guide covers developer workflow with CLI commands end-to-end
- [ ] #2 Agent integration section explains MCP server usage with AI clients
- [ ] #3 Workspace lifecycle (init, add, query, sync, archive) is documented with examples
<!-- AC:END -->
