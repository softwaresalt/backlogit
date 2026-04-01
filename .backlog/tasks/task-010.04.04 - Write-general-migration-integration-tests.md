---
id: TASK-010.04.04
title: Write general migration integration tests
status: To Do
assignee: []
created_date: '2026-04-01 22:39'
labels:
  - go
  - test
dependencies:
  - TASK-010.04.01
  - TASK-010.04.02
  - TASK-010.04.03
parent_task_id: TASK-010.04
priority: medium
ordinal: 4000
implementation_notes: |
  Harness command: go test ./tests/integration/... -run "TestGeneralMigration" -v
  Test file: tests/integration/migration_test.go
  Stub files: internal/parser/adapter.go, internal/config/migration.go
  Execution note: test-first
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Write integration tests for the general purpose migration framework covering the adapter interface, file classifier, and configuration templates.

Tests to implement:
- Adapter registry: register, discover, and select adapters
- Adapter interface compliance: verify BacklogMdAdapter implements interface correctly
- File classifier accuracy: test against sample documents of each class (spec, plan, work item, decision, note)
- Classifier confidence thresholds: verify ambiguous documents score below threshold
- Configuration template loading: verify default templates parse and validate
- End-to-end: `--adapter backlog-md --detect --dry-run` on a sample project
- Edge cases: empty directories, non-markdown files, binary files, deeply nested structures

Files to create:
- `tests/integration/general_migration_test.go` (new)
- `tests/integration/testdata/general-migration/` (sample project fixtures)

Verification: `go test ./tests/integration/...` passes with all general migration scenarios.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Integration tests cover adapter interface with at least two adapter implementations
- [ ] #2 Tests verify file classifier accuracy across representative document samples
- [ ] #3 End-to-end migration test exercises --adapter and --detect flags
<!-- AC:END -->
