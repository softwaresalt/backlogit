---
id: TASK-001.04.03
title: Implement custom field validation
status: Done
assignee: []
created_date: '2026-03-30 01:40'
labels: []
dependencies: []
parent_task_id: TASK-001.04
priority: high
ordinal: 4300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/core/fields.go` with:

1. `ValidateFields(fieldConfigs map[string]*config.FieldConfig, fields map[string]any) error` — validates custom field values against their configured types and allowed values. Returns `ErrValidation` with details on failure.
2. `TranslateExternalMap(fieldConfig *config.FieldConfig, value any) (any, error)` — translates a local field value to its external system representation using the `external_map` configuration (e.g., priority "critical" → ADO severity "1 - Critical")

Create `internal/core/fields_test.go` with table-driven tests for validation and translation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ValidateFields rejects invalid enum values with ErrValidation
- [ ] #2 ValidateFields accepts valid enum values from config
- [ ] #3 TranslateExternalMap converts local field values to external system format
- [ ] #4 Tests cover enum validation, string fields, int fields, external_map translation
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.