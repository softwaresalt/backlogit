---
id: TASK-010.01.02
title: 'Add migration CLI enhancements (dry-run, progress, validation)'
status: To Do
assignee: []
created_date: '2026-04-01 22:35'
labels:
  - go
dependencies:
  - TASK-010.01.01
parent_task_id: TASK-010.01
priority: medium
ordinal: 2000
implementation_notes: |
  Harness command: go test ./internal/parser/... -run "TestMigrateWithOptions|TestFormatReport|TestMigrationReport" -v
  Test file: internal/parser/migrate_options_test.go
  Stub file: internal/parser/adapter.go (MigrateWithOptions, FormatReport functions)
  Execution note: test-first
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add CLI enhancements to `internal/cli/migrate.go` for better migration UX.

Current state: `migrate` command exists but is minimal — reads a legacy file and creates artifacts.

Enhancements needed:
- `--dry-run` flag: show what would be migrated without writing files
- `--progress` flag: display item-by-item progress with counts
- `--validate` flag: run validation checks without creating artifacts
- Validation report: summary of successful, skipped (duplicate), and failed items
- `--format` flag: output report as text or JSON
- Error recovery: continue processing remaining items when one fails, collect errors

Files to modify:
- `internal/cli/migrate.go` (add flags and reporting)
- `internal/parser/migration.go` (add dry-run and validation support)

Verification: `backlogit migrate --dry-run` produces accurate preview output.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 --dry-run flag shows migration plan without writing files
- [ ] #2 --progress flag displays item-by-item progress during migration
- [ ] #3 Validation report summarizes successful, skipped, and failed items
<!-- AC:END -->
