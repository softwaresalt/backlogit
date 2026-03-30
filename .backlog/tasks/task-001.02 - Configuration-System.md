---
id: TASK-001.02
title: Configuration System
status: To Do
assignee: []
created_date: '2026-03-30 01:36'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.01
parent_task_id: TASK-001
priority: high
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Load and validate `config.yaml`, `registry.yaml`, and `hooks.yaml` from the `.backlogit/` workspace using typed Go structs with `go-playground/validator` tags. Support environment variable overrides with the `BACKLOGIT_` prefix.

Key structs: `WorkspaceConfig`, `ArtifactTypeConfig` (prefix, name_format, allowed_children), `FieldConfig` (type, values, default, optional, external_map), `RegistryConfig`, minimal `HooksConfig` stub. Validates `allowed_children` references, field types, and required fields. Returns wrapped `ErrConfig` on all failure paths.

Per review P2-09: hooks.yaml typing is minimal stub only (external sync is deferred scope).
<!-- SECTION:DESCRIPTION:END -->
