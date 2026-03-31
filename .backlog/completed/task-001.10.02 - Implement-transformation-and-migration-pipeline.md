---
id: TASK-001.10.02
title: Implement transformation and migration pipeline
status: Done
assignee: []
created_date: '2026-03-30 01:45'
labels: []
dependencies: []
parent_task_id: TASK-001.10
priority: medium
ordinal: 10200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/parser/migration.go` with:

1. `Migrate(ctx context.Context, legacyPath string, workspace *core.Workspace) (int, error)` — orchestrates the full transformation pipeline:
   - Extraction: reads legacy file content
   - Decomposition: calls `ParseLegacy` to slice into `LegacyItem` list
   - Attribution: assigns IDs via naming templates, infers artifact types from heading depth, resolves parent references
   - Scaffolding: writes atomic `.md` files to proper directories via `core.CreateArtifact`
   - Rehydration: triggers `db.Rehydrate` to build index.db
   - Archiving: renames original to `.bak` (e.g., `backlog.legacy.md.bak`)
2. Returns count of migrated items. Wraps errors with `ErrMigration`.

Create `internal/parser/migration_test.go` with end-to-end test using temp workspace, sample legacy content, and verification of output files and archive.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Migrate reads legacy file and produces atomic .md files in .backlogit/ directories
- [ ] #2 Each output file has valid YAML frontmatter with id, title, status, type
- [ ] #3 Parent-child hierarchy preserved via parent_id in frontmatter
- [ ] #4 Original file renamed to .bak extension with zero data loss
- [ ] #5 End-to-end test verifies complete migration pipeline with temp workspace
<!-- AC:END -->


## Implementation Notes

Completed in commit 90b24e6. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.