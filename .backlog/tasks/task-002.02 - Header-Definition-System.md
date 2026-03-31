---
id: TASK-002.02
title: Header Definition System
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
Introduce `header-def.yaml` as the per-type field schema definition file. Defines immutable defaults (created_date, updated_date, id), configurable ID prefixes (default OP + 3 digits), per-type field schemas with validation rules, and the eight artifact types from the queue (Epic, Feature, Sub-Epic, User-Story, Task, Sub-Task, Bug, Decision).

**Config boundary (from review F1)**: `config.yaml` owns workspace-level behavior (routing, naming). `header-def.yaml` owns per-type field schemas (immutable defaults, field types, validation rules).

**Backward compatibility (from review F3)**: `header-def.yaml` supports configurable ID formats. Default `OP{NNN}` applies to new workspaces. Existing workspaces retain their `config.yaml` naming patterns.

Covers plan Units 6-7.
<!-- SECTION:DESCRIPTION:END -->
