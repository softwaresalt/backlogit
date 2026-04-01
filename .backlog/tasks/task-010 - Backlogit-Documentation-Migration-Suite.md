---
id: TASK-010
title: Backlogit Documentation & Migration Suite
status: To Do
assignee: []
created_date: '2026-04-01 22:24'
labels:
  - epic
dependencies: []
references:
  - .backlog/queue.md
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Comprehensive documentation and migration tooling for the backlogit project. The codebase is feature-complete (21 MCP tools, 16 CLI commands, full CQRS architecture) but README.md is a placeholder, `docs/` is empty, and migration tooling only handles basic legacy backlog.md parsing.

This epic covers four areas:
1. Core documentation: README, installation guide, process/workflow docs
2. Positioning documentation: rationale, Backlog.md comparison, migration guide
3. Backlog.md migration tooling: enhanced parser, CLI improvements, configuration
4. General purpose migration tooling: pluggable adapters, file classification, scripts

Source queue items from `.backlog/queue.md`:
- README documentation
- Install documentation
- Process and workflow documentation
- Rationale for tool existence and its value in agent harness composition
- Documentation differentiating backlogit from Backlog.md
- How to migrate from Backlog.md to backlogit
- Backlog.md migration tools, scripts, and configuration
- General purpose migration tools, scripts, and configuration
<!-- SECTION:DESCRIPTION:END -->
