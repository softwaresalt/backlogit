---
id: TASK-010.01.03
title: Create migration configuration for document class mapping
status: Done
assignee: []
created_date: '2026-04-01 22:35'
updated_date: '2026-04-01 23:38'
labels:
  - go
dependencies: []
parent_task_id: TASK-010.01
priority: medium
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create a migration configuration schema that tells the migration engine where to find specific classes of Markdown documents in a Backlog.md workspace.

Content to implement:
- YAML configuration schema (`migration.yaml`) defining source path patterns
- Document class definitions: specs, plans, work items, decisions, notes
- Path pattern mapping: which directories/glob patterns contain which document classes
- Artifact type mapping: how each document class maps to backlogit artifact types (task, bug, story, epic, adr)
- Default configuration template for common Backlog.md layouts
- Integration with existing config loader pattern in `internal/config/`

Files to create/modify:
- `internal/config/migration.go` (new: migration config schema and loader)
- `internal/config/migration_test.go` (new: tests)
- `internal/config/defaults.go` (extend: add default migration.yaml template)

Verification: `go test ./internal/config/...` passes; default migration config loads successfully.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 YAML configuration schema defines source path patterns for document class detection
- [ ] #2 Document classes (specs, plans, work items, decisions) map to backlogit artifact types
- [ ] #3 Configuration loads and validates via existing config loader pattern
<!-- AC:END -->
