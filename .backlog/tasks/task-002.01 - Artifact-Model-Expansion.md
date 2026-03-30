---
id: TASK-002.01
title: Artifact Model Expansion
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
Expand the `Artifact` struct and all dependent layers (DB schema, frontmatter parser, rehydration, core CRUD) with the queue-specified fields: `assigned_to`, `owner`, `labels` ([]string), `dependencies` ([]string), `references` ([]string), and `commit`. Enforce ID immutability in update paths.

Covers plan Units 1-5. This is the foundation that all other sub-epics depend on.
<!-- SECTION:DESCRIPTION:END -->
