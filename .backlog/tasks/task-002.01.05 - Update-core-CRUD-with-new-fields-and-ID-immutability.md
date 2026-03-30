---
id: TASK-002.01.05
title: Update core CRUD with new fields and ID immutability
status: To Do
assignee: []
created_date: '2026-03-30 06:57'
labels: []
dependencies:
  - TASK-002.01.01
  - TASK-002.01.02
  - TASK-002.01.03
parent_task_id: TASK-002.01
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add new `Option` functions to `internal/core/artifacts.go`: `WithAssignedTo`, `WithOwner`, `WithLabels`, `WithDependencies`, `WithReferences`, `WithCommit`. Update `CreateArtifact` to pass these to the `Artifact` struct and include them in frontmatter serialization. Update `UpdateArtifact` to handle new fields in the updates map. Enforce ID immutability: reject any update that attempts to change the `id` field with a descriptive error.

**Files:** `internal/core/artifacts.go`
**Test files:** `internal/core/artifacts_test.go` (new or expand)
**Patterns:** Follow functional options pattern at `internal/core/artifacts.go` lines 14-47
**Verification:** `go test ./internal/core/...` passes with creation tests using new options. Update test confirms ID change is rejected with descriptive error.
<!-- SECTION:DESCRIPTION:END -->
