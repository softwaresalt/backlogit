---
id: TASK-008.01
title: >-
  Fix template service Update: set updated_at and call UpsertItem after section
  writes
status: To Do
assignee: []
created_date: '2026-03-31 05:40'
labels:
  - bug
  - templates
  - copilot-review
dependencies: []
references:
  - internal/core/templates/service.go
parent_task_id: TASK-008
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Copilot review finding: `internal/core/templates/service.go:125` — Service.Update method writes sections but does not update updated_at or call db.UpsertItem. Changes are invisible to list, query, and MCP tool results until next sync.

Fix required:
1. After writing section updates, call db.UpsertItem(ctx, ws.DB, artifact) to keep the index current.
2. Set artifact.UpdatedAt = time.Now() before writing so the timestamp reflects the mutation.
3. Ensure the updated frontmatter (with new updated_at) is written back to the file.
<!-- SECTION:DESCRIPTION:END -->
