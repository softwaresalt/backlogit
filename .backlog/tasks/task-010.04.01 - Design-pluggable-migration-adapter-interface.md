---
id: TASK-010.04.01
title: Design pluggable migration adapter interface
status: Done
assignee: []
created_date: '2026-04-01 22:37'
updated_date: '2026-04-01 23:29'
labels:
  - go
dependencies: []
parent_task_id: TASK-010.04
priority: medium
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Design and implement a pluggable migration adapter interface in `internal/parser/` that generalizes migration beyond Backlog.md.

Content to implement:
- `MigrationAdapter` interface with methods: `Detect(path) bool`, `Parse(ctx, path) ([]MigrationItem, error)`, `Name() string`
- `MigrationItem` struct: generalized intermediate representation with title, body, metadata, source type, parent reference
- Adapter registry: register adapters by name, discover available adapters, select adapter by source detection
- Refactor existing `ParseLegacy()` into a `BacklogMdAdapter` that implements the interface
- Factory function: `NewAdapter(name string) (MigrationAdapter, error)`

Files to create/modify:
- `internal/parser/adapter.go` (new: interface, registry, MigrationItem)
- `internal/parser/adapter_test.go` (new: registry tests)
- `internal/parser/legacy.go` (refactor: implement MigrationAdapter interface)
- `internal/parser/migration.go` (refactor: use adapter registry)

Verification: `go test ./internal/parser/...` passes; existing migration behavior unchanged.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 MigrationAdapter interface is defined with Source, Transform, and Load methods
- [ ] #2 Adapter registry supports registering and discovering adapters by name
- [ ] #3 Existing Backlog.md migration is refactored to implement the adapter interface
<!-- AC:END -->
