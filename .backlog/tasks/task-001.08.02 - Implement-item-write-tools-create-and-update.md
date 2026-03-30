---
id: TASK-001.08.02
title: Implement item write tools (create and update)
status: To Do
assignee: []
created_date: '2026-03-30 01:43'
labels: []
dependencies: []
parent_task_id: TASK-001.08
priority: high
ordinal: 8200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Add to `internal/mcp/tools.go`:

1. `backlogit_create_item` tool — parameters: type (required, enum), title (required), description, parent_id, sprint, status, priority, custom fields. Handler: validates workspace → parses params → calls `core.CreateArtifact` → returns JSON with id, title, status.
2. `backlogit_update_item` tool — parameters: item_id (required), updates (status, title, description, parent_id, sprint, priority). Handler: validates workspace → parses params → calls `core.UpdateArtifact` → returns JSON with updated artifact.

Both handlers follow the five-step pattern and use the Server struct's DB and config (no global state).
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit_create_item accepts type, title, description, parent_id, sprint and creates artifact
- [ ] #2 backlogit_update_item accepts item_id and updates map, returns updated artifact
- [ ] #3 Both tools follow five-step handler pattern
- [ ] #4 Both tools return workspace_not_initialized error when .backlogit/ missing
- [ ] #5 Tests verify parameter validation and successful creation/update
<!-- AC:END -->
