---
id: TASK-010.01
title: Backlog.md Migration Tooling
status: To Do
assignee: []
created_date: '2026-04-01 22:25'
labels:
  - epic
  - go
dependencies: []
parent_task_id: TASK-010
priority: medium
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Enhance the existing migration tooling (`internal/parser/legacy.go`, `internal/parser/migration.go`, `internal/cli/migrate.go`) to provide comprehensive Backlog.md-to-backlogit migration.

Current state: `ParseLegacy()` handles basic heading/checklist heuristics. `Migrate()` reads a legacy file and returns `[]LegacyItem`. CLI `migrate` command exists but is minimal.

Enhancements needed: broader format coverage (specs, plans, decisions), CLI improvements (dry-run, progress, validation), configuration for document class mapping.

Covers queue items: Backlog.md migration tools, scripts, and configuration.
<!-- SECTION:DESCRIPTION:END -->
