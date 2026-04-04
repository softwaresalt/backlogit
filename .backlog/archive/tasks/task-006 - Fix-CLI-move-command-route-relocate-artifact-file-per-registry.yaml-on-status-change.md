---
id: TASK-006
title: >-
  Fix CLI move command: route/relocate artifact file per registry.yaml on status
  change
status: To Do
assignee: []
created_date: '2026-03-31 02:35'
updated_date: '2026-03-31 05:41'
labels:
  - bug
  - cli
  - copilot-review
dependencies:
  - TASK-008
references:
  - internal/cli/move.go
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Copilot review finding D3: `internal/cli/move.go:48` — the `move` command changes the status field in the artifact's YAML frontmatter and updates the DB row, but does not physically relocate the file to the directory mapped by the new status in `registry.yaml`. Artifacts remain in their original directory after a status change, breaking directory-based filtering and browsing.

**Fix required:**
1. After updating the status, look up the target directory from routing logic for the new `(type, status)` pair.
2. Move the file from its current path to the new directory using an atomic rename (temp-file-then-rename pattern).
3. Update `db.UpsertItem` with the artifact after relocation so the index stays consistent.
4. If the target directory does not exist, create it with `os.MkdirAll`.

**File:** `internal/cli/move.go` around line 48.
<!-- SECTION:DESCRIPTION:END -->
