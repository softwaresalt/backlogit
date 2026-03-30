---
id: TASK-001.04.05
title: Implement artifact creation with hierarchy enforcement
status: Done
assignee: []
created_date: '2026-03-30 01:40'
labels: []
dependencies: []
parent_task_id: TASK-001.04
priority: high
ordinal: 4500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/core/artifacts.go` with:

1. `CreateArtifact(ctx context.Context, ws *Workspace, title string, artifactType string, opts ...Option) (*models.Artifact, error)` — validates artifact type exists in config, enforces parent-child hierarchy via `allowed_children`, generates name via naming template, resolves target directory via routing, writes Markdown file atomically (temp file then rename), returns created artifact.
2. `UpdateArtifact(ctx context.Context, ws *Workspace, id string, updates map[string]any) (*models.Artifact, error)` — reads existing artifact, applies updates, re-validates, writes atomically, moves file if status changed.
3. Functional options: `WithParent(id)`, `WithSprint(id)`, `WithStatus(status)`, `WithDescription(desc)`, `WithFields(map)`

Uses SafeResolve for all path operations. Wraps errors with `ErrValidation`.

Create `internal/core/artifacts_test.go` with table-driven tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 CreateArtifact produces a Markdown file with valid YAML frontmatter in the correct directory
- [ ] #2 Creating a task under a bug succeeds; creating an epic under a task returns ErrValidation
- [ ] #3 File writes use temp-file-then-rename pattern for atomicity
- [ ] #4 All paths validated through SafeResolve before filesystem access
- [ ] #5 Tests verify hierarchy enforcement, file content, and atomic write behavior
<!-- AC:END -->
