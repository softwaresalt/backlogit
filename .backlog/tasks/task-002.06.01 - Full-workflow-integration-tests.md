---
id: TASK-002.06.01
title: Full workflow integration tests
status: done
assignee: []
created_date: '2026-03-30 07:02'
labels: []
dependencies:
  - TASK-002.04.08
  - TASK-002.05.02
parent_task_id: TASK-002.06
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
End-to-end integration tests covering the complete workflow in `tests/integration/workflow_test.go`:
1. `init` → creates workspace with `header-def.yaml`, templates, config, registry
2. `add --type task --title "Test"` → creates artifact with template sections
3. `list` → shows the created artifact
4. `get <id>` → displays full content with sections
5. `update <id> --status in_progress` → updates frontmatter
6. `move <id> --status done` → relocates file per registry rules
7. `search "Test"` → finds via FTS5
8. `delete <id>` → removes artifact and index entry
9. MCP dynamic tool creation → produces correct artifacts with template structure

All tests use `t.TempDir()` for isolated workspace directories. Validate cross-layer integration: CLI → core → DB → parser → templates.

**Files:** `tests/integration/workflow_test.go` (new)
**Verification:** `go test ./tests/integration/...` passes with the complete workflow. All 9 scenarios produce expected results.
<!-- SECTION:DESCRIPTION:END -->

