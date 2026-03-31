---
id: TASK-001.02.01
title: Define configuration schema structs
status: Done
assignee: []
created_date: '2026-03-30 01:38'
labels: []
dependencies: []
parent_task_id: TASK-001.02
priority: high
ordinal: 2100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/config/schema.go` with typed configuration structs:

1. `WorkspaceConfig` — top-level config holding artifact types, fields, and metadata
2. `ArtifactTypeConfig` — Prefix (string, required), NameFormat (string, required), AllowedChildren ([]string)
3. `FieldConfig` — Type (enum: "enum"|"string"|"int"), Values ([]string for enums), Default (string), Optional (bool), ExternalMap (map[string]any with justification comment per P2-09)
4. `RegistryConfig` — directory routing rules with path and condition (status/type matching)
5. `HooksConfig` — minimal stub struct (external sync is deferred scope per P2-09)

All structs use `yaml:"..."` and `validate:"..."` tags from `go-playground/validator/v10`. Package-level cached validator instance: `var validate = validator.New()` (per P3-02).
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 WorkspaceConfig struct has YAML tags and validator tags for all fields
- [ ] #2 ArtifactTypeConfig validates Prefix, NameFormat, AllowedChildren
- [ ] #3 FieldConfig supports enum, string, int types with validation
- [ ] #4 RegistryConfig maps directory conditions to paths
- [ ] #5 All structs pass go vet and golangci-lint
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.