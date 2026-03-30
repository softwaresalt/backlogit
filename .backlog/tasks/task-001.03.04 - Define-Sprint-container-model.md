---
id: TASK-001.03.04
title: Define Sprint container model
status: Done
assignee: []
created_date: '2026-03-30 01:39'
labels: []
dependencies: []
parent_task_id: TASK-001.03
priority: high
ordinal: 3400
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/models/sprint.go` with:

1. `Sprint` struct with fields: ID (string), Goal (string), StartDate (time.Time), EndDate (time.Time), ArtifactIDs ([]string) — all with JSON and YAML struct tags
2. `Validate() error` method enforcing required fields

Create `internal/models/sprint_test.go` with validation tests.

Per review P1-11: Sprints are indexed as regular items (type='sprint') in the database. This task only defines the data model; indexing is handled by the rehydration engine.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Sprint struct has ID, Goal, StartDate, EndDate, ArtifactIDs fields with JSON/YAML tags
- [ ] #2 Sprint.Validate() enforces required fields (ID, Goal)
- [ ] #3 Tests cover valid sprints and missing required fields
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.