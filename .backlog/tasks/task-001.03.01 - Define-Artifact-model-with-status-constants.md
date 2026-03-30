---
id: TASK-001.03.01
title: Define Artifact model with status constants
status: Done
assignee: []
created_date: '2026-03-30 01:39'
labels: []
dependencies: []
parent_task_id: TASK-001.03
priority: high
ordinal: 3100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/models/artifact.go` with:

1. `ArtifactStatus` string type alias with constants: `StatusTodo`, `StatusInProgress`, `StatusBlocked`, `StatusReview`, `StatusDone`, `StatusAccepted`, `StatusRejected`
2. `Artifact` struct with fields: ID, Title, Status, ArtifactType, ParentID, Sprint, Priority, Description, CustomFields (map[string]any), CreatedAt, UpdatedAt — all with JSON and YAML struct tags and validator tags
3. `Validate() error` method using cached validator instance

Create `internal/models/artifact_test.go` with table-driven tests for validation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Artifact struct has id, title, status, artifact_type, parent_id, sprint, priority, description fields with JSON/YAML tags
- [ ] #2 Status constants defined: StatusTodo, StatusInProgress, StatusBlocked, StatusReview, StatusDone
- [ ] #3 Artifact.Validate() enforces required fields (id, title, status, artifact_type)
- [ ] #4 Table-driven tests cover valid artifacts, missing fields, invalid status values
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.