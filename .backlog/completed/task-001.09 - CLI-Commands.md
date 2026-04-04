---
id: TASK-001.09
title: CLI Commands
status: Done
assignee: []
created_date: '2026-03-30 01:37'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.08
parent_task_id: TASK-001
priority: high
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement the Cobra CLI with all commands: `init`, `create`, `sync`, `query`, `mcp`, and `migrate`. Wire each command to core business logic and database packages.

Per review P1-09: Includes `backlogit migrate` command wired to the legacy migration pipeline.

Commands: `backlogit init [--legacy]`, `backlogit create --type TYPE --title TITLE`, `backlogit sync`, `backlogit query "SQL"`, `backlogit mcp`, `backlogit migrate [path]`
<!-- SECTION:DESCRIPTION:END -->
