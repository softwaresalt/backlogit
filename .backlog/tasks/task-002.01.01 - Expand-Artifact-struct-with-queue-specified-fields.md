---
id: TASK-002.01.01
title: Expand Artifact struct with queue-specified fields
status: To Do
assignee: []
created_date: '2026-03-30 06:56'
labels: []
dependencies: []
parent_task_id: TASK-002.01
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add missing fields to the `Artifact` struct in `internal/models/artifact.go`:
- `AssignedTo string` — json:"assigned_to,omitempty" yaml:"assigned_to,omitempty"
- `Owner string` — json:"owner,omitempty" yaml:"owner,omitempty"
- `Labels []string` — json:"labels,omitempty" yaml:"labels,omitempty"
- `Dependencies []string` — json:"dependencies,omitempty" yaml:"dependencies,omitempty"
- `References []string` — json:"references,omitempty" yaml:"references,omitempty"
- `Commit string` — json:"commit,omitempty" yaml:"commit,omitempty"

Add validator tags. Labels, Dependencies, References use `[]string`. Update `Validate()`.

**Files:** `internal/models/artifact.go`
**Test files:** `internal/models/artifact_test.go`
**Patterns:** Follow existing struct tag style at `internal/models/artifact.go` lines 25-37
**Verification:** `go test ./internal/models/...` passes with new field validation tests. Table-driven tests cover valid artifacts with all new fields, empty optional fields.
<!-- SECTION:DESCRIPTION:END -->
