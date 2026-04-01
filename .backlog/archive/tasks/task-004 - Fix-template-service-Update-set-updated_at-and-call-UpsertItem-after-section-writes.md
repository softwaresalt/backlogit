---
id: TASK-004
title: >-
  Fix template service Update: set updated_at and call UpsertItem after section
  writes
status: To Do
assignee: []
created_date: '2026-03-31 02:34'
labels:
  - bug
  - templates
  - copilot-review
dependencies: []
references:
  - internal/core/templates/service.go
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Copilot review finding D4: `internal/core/templates/service.go:125` — the `Service.Update` method writes modified section content back to the artifact file but does not update the `updated_at` timestamp in the frontmatter and does not call `db.UpsertItem` to sync the change into the SQLite index. This is the same class of bug as was fixed in the MCP handler (P0/P1 fix, prior commit).

**Fix required:**
1. After computing the new body content, set `artifact.UpdatedAt = time.Now()` on the parsed artifact.
2. Reserialize the frontmatter (with updated `updated_at`) and the new body together before writing.
3. Call `db.UpsertItem(ctx, ws.DB, artifact)` so the index reflects the mutation immediately.

**File:** `internal/core/templates/service.go` around line 125 in the `Update` function.
<!-- SECTION:DESCRIPTION:END -->
