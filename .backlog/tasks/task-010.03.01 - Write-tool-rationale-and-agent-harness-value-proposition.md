---
id: TASK-010.03.01
title: Write tool rationale and agent harness value proposition
status: To Do
assignee: []
created_date: '2026-04-01 22:32'
labels:
  - docs
dependencies: []
parent_task_id: TASK-010.03
priority: medium
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `docs/rationale.md` explaining why backlogit exists and its value in the agent harness composition.

Content to include:
- The fundamental tension: human-centric tools vs. machine-centric requirements
- Why existing tools fail AI agents: context window bloat, no structured queries, no token efficiency
- The CQRS solution: Markdown for humans, SQLite for agents, JSONL for history
- Value in agent harness composition: how backlogit serves as the "operating system" for AI coding agents
- MCP as the protocol bridge: why JSON-RPC 2.0 over stdio enables universal agent compatibility
- Design philosophy: Git-friendly persistence, single-binary simplicity, workspace containment
- Comparison with alternative approaches (JIRA API, GitHub Issues API, plain text files)

Files to create:
- `docs/rationale.md` (new)

Reference: `.backlog/research/Backlogit-Architecture-Design.md` for architecture context.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Document articulates why file-backed CQRS with MCP is superior to flat-file-only approaches
- [ ] #2 Agent harness value proposition is clearly explained with concrete examples
- [ ] #3 Design philosophy section grounds rationale in the constitution principles
<!-- AC:END -->
