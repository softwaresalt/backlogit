---
id: TASK-002.03.01
title: Implement template schema and loader
status: Done
assignee: []
created_date: '2026-03-30 06:58'
labels: []
dependencies:
  - TASK-002.02.01
parent_task_id: TASK-002.03
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Define a `TemplateConfig` struct in `internal/config/templates.go` representing a Markdown template with named sections delimited by `<!-- BEGIN:{section_name} -->` and `<!-- END:{section_name} -->` tags. Each template file lives in `.backlogit/templates/` and declares in its YAML frontmatter:
- `type`: artifact type this template applies to
- `sections`: list of section definitions, each with: name, flag (CLI flag name), required (bool)

Create `LoadTemplates(templatesDir string) ([]*TemplateConfig, error)` to discover and parse all template files. Validate: section names are unique within a template, flags are valid CLI flag names (lowercase, hyphenated), no template has both BEGIN and END tags mismatched.

Integrate template discovery into the registry so `registry.yaml` can declare which templates are active.

**Files:** `internal/config/templates.go` (new)
**Test files:** `internal/config/templates_test.go` (new)
**Patterns:** Follow `WorkspaceConfig` loader in `internal/config/loader.go`
**Verification:** `go test ./internal/config/...` passes with template loading, parsing, and section extraction tests. Invalid templates (missing END tags, duplicate section names) produce descriptive errors.
<!-- SECTION:DESCRIPTION:END -->

