---
id: TASK-002.04
title: CLI Command Suite
status: To Do
assignee: []
created_date: '2026-03-30 06:55'
labels:
  - epic
dependencies: []
parent_task_id: TASK-002
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement all CLI commands needed to fully manage a backlog from the terminal: `add` (with --type flag), `list`, `get`, `update`, `move`, `delete`, `search`, `query`, and `status`. Each command integrates with the workspace, SQLite index, template system, and section parser.

Multi-line markdown input via stdin (triggered by `-` flag value) enables piped content for section writes.

**Observability (from review F4)**: All CLI commands must include slog.Info for command entry/exit, slog.Debug for intermediate steps, and slog.Error for failures.

Covers plan Units 11-18.
<!-- SECTION:DESCRIPTION:END -->
