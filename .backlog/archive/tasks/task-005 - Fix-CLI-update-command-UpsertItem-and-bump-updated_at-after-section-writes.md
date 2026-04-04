---
id: TASK-005
title: 'Fix CLI update command: UpsertItem and bump updated_at after section writes'
status: To Do
assignee: []
created_date: '2026-03-31 02:35'
labels:
  - bug
  - cli
  - copilot-review
dependencies: []
references:
  - internal/cli/update.go
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Copilot review finding D2: `internal/cli/update.go:117` — when `--section` flags are applied, the update command rewrites the artifact file but never calls `db.UpsertItem` to sync the change into the SQLite index, and does not bump `updated_at` on the artifact. Section updates are invisible to `list`, `query`, and MCP tool results until the next `sync`.

**Fix required:**
1. After writing section updates to the file, call `db.UpsertItem(ctx, ws.DB, artifact)` to keep the index current.
2. Set `artifact.UpdatedAt = time.Now()` before writing so the timestamp reflects the mutation.
3. Ensure the updated frontmatter (with new `updated_at`) is written back to the file.

**File:** `internal/cli/update.go` around line 117 in the section-write branch.
<!-- SECTION:DESCRIPTION:END -->
