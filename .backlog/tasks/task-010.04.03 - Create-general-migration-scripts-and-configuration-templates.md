---
id: TASK-010.04.03
title: Create general migration scripts and configuration templates
status: To Do
assignee: []
created_date: '2026-04-01 22:38'
labels:
  - go
dependencies:
  - TASK-010.04.01
  - TASK-010.04.02
parent_task_id: TASK-010.04
priority: medium
ordinal: 3000
implementation_notes: |
  Harness command: go test ./tests/integration/... -run "TestGeneralMigration_MigrateWithAdapterFlag" -v
  Test file: tests/integration/migration_test.go
  Stub files: internal/parser/adapter.go, internal/cli/migrate.go
  Execution note: test-first
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create migration scripts and configuration templates that users can customize for their specific project layouts.

Content to implement:
- CLI `migrate` command enhancement: `--adapter` flag to select migration adapter, `--config` to specify migration configuration file
- Default migration configuration templates for common layouts:
  - Flat directory (all markdown in one folder)
  - Structured directory (separate folders for specs, plans, work items)
  - Mixed (markdown scattered throughout a codebase)
- Migration recipe templates in `docs/migration-recipes/`:
  - Generic markdown project → backlogit
  - GitHub Issues export → backlogit (future adapter placeholder)
- Integration of file classifier with CLI: `backlogit migrate --detect` auto-classifies documents

Files to create/modify:
- `internal/cli/migrate.go` (extend: --adapter, --config, --detect flags)
- `internal/config/defaults.go` (extend: add migration template configs)
- `docs/migration-recipes/` (new: recipe templates)

Verification: `backlogit migrate --detect --dry-run` correctly classifies and previews a sample project.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Script templates exist for common migration scenarios (directory scan, single-file, batch)
- [ ] #2 Configuration templates provide starting points for common project layouts
- [ ] #3 Scripts use the adapter interface and migration configuration schema
<!-- AC:END -->
