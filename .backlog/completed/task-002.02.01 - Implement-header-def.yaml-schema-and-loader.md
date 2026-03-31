---
id: TASK-002.02.01
title: Implement header-def.yaml schema and loader
status: Done
assignee: []
created_date: '2026-03-30 06:58'
labels: []
dependencies:
  - TASK-002.01.01
parent_task_id: TASK-002.02
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/config/headerdef.go` with a `HeaderDefConfig` struct defining per-type field schemas with immutable defaults. Structure includes:
- `defaults` section: system-managed fields (created_date: auto, updated_date: auto, id: immutable)
- `types` section: per-type definitions with prefix, id_format, and field schemas
- Eight artifact types: Epic, Feature, Sub-Epic, User-Story, Task, Sub-Task, Bug, Decision
- Each field defines: type (enum/string/int/list), values (for enums), default, optional flag
- Status enums: To-Do, In-Progress, Blocked, Done
- Priority enums: Low, Medium, High
- Default ID format: `{prefix}{NNN}` with configurable prefix (default OP)

Add `LoadHeaderDef` function using `gopkg.in/yaml.v3` with `go-playground/validator` for schema validation. Integrate with `Workspace` initialization. Mark `id`, `created_date`, `updated_date` as system-managed immutable fields rejected from manual updates.

**Config boundary (review F1)**: `config.yaml` owns workspace-level behavior (routing, naming). `header-def.yaml` owns per-type field schemas.
**Backward compat (review F3)**: Support configurable ID formats. OP prefix applies to new workspaces only.

**Files:** `internal/config/headerdef.go` (new), `internal/config/schema.go`
**Test files:** `internal/config/headerdef_test.go` (new)
**Patterns:** Follow `WorkspaceConfig` loading pattern in `internal/config/loader.go`
**Verification:** `go test ./internal/config/...` passes with header-def loading, validation, and per-type field resolution tests. Invalid files produce descriptive errors.
<!-- SECTION:DESCRIPTION:END -->
