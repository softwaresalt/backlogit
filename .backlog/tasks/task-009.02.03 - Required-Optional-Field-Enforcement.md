---
id: TASK-009.02.03
title: Required/Optional Field Enforcement
status: Done
assignee: []
created_date: '2026-03-31 06:05'
updated_date: '2026-04-01 05:18'
labels:
  - task
  - phase-2
dependencies:
  - TASK-009.02.02
parent_task_id: TASK-009.02
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 5 — Required/Optional Field Enforcement**

Enforce required/optional field constraints from WIT template definitions at artifact creation and update time.

Key deliverables:

- Extend `FieldDef` in `internal/config/headerdef.go` with `Required bool`, `Default string`

- New `internal/core/validation.go`: `ValidateArtifactFields(artifact *Artifact, typeDef *TypeDefConfig) error`

- Wire validation into `CreateArtifact` and `UpdateArtifact` in `internal/core/artifacts.go`

- Wire validation into MCP tool handlers (`handleCreateItem`, `handleUpdateItem`) 

- Wire validation into CLI commands (`create`, `update`)

- Backward compatibility: existing artifacts without new required fields pass validation (fields added after creation are not retroactively required)

- Default values applied when optional fields are omitted
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Required fields missing from create request produce descriptive validation error

- [x] #2 Optional fields with defaults auto-populated when omitted

- [x] #3 Validation runs on both MCP tool calls and CLI commands

- [x] #4 backlogit create --type bug enforces required fields per bug template

- [x] #5 Field optionality metadata readable via backlogit_get_wit_metadata

- [x] #6 Existing artifacts without new required fields pass validation (backward compat)
<!-- AC:END -->
