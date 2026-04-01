---
id: TASK-010.01.04
title: Write Backlog.md migration integration tests
status: To Do
assignee: []
created_date: '2026-04-01 22:36'
labels:
  - go
  - test
dependencies:
  - TASK-010.01.01
  - TASK-010.01.02
  - TASK-010.01.03
parent_task_id: TASK-010.01
priority: medium
ordinal: 4000
implementation_notes: |
  Harness command: go test ./tests/integration/... -run "TestMigration_EndToEnd" -v
  Test file: tests/integration/migration_test.go
  Stub files: internal/parser/adapter.go, internal/config/migration.go
  Execution note: test-first
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Write comprehensive integration tests for the Backlog.md migration pipeline covering the enhanced parser, CLI enhancements, and configuration.

Tests to implement:
- End-to-end migration: sample Backlog.md → backlogit artifacts (verify file creation, frontmatter, status mapping)
- Dry-run mode: verify preview output without file writes
- Document class detection: verify specs, plans, decisions are classified correctly per migration.yaml
- Error recovery: verify migration continues after individual item failures
- Duplicate detection: verify existing artifacts are skipped during re-migration
- Hierarchical preservation: verify parent-child relationships survive migration
- Edge cases: empty files, malformed Markdown, missing headings, mixed formats

Test fixtures:
- Sample Backlog.md files representing common layouts
- Sample migration.yaml configurations

Files to create:
- `tests/integration/migration_test.go` (new)
- `tests/integration/testdata/` (sample Backlog.md fixtures)

Verification: `go test ./tests/integration/...` passes with all migration scenarios.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Integration tests cover end-to-end migration from sample Backlog.md files to backlogit artifacts
- [ ] #2 Tests verify dry-run mode produces correct preview without file writes
- [ ] #3 Tests validate document class detection with migration configuration
<!-- AC:END -->
