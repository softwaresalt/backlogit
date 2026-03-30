---
id: TASK-001.02.03
title: Create default config and registry templates
status: Done
assignee: []
created_date: '2026-03-30 01:39'
labels: []
dependencies: []
parent_task_id: TASK-001.02
priority: high
ordinal: 2300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/config/defaults.go` with:

1. `DefaultConfig() *WorkspaceConfig` — returns a sensible default configuration with artifact types (task, bug, user_story, epic, feature, decision), field definitions (status, type, parent, sprint, priority), and naming templates
2. `DefaultRegistry() *RegistryConfig` — returns default directory routing rules: `sprint-board/todo/` for todo, `sprint-board/active/` for in_progress, `sprint-board/review/` for review, `completed/` for done
3. `WriteDefaults(workspacePath string) error` — serializes defaults to `config.yaml` and `registry.yaml` files in the workspace directory
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 DefaultConfig() returns a valid WorkspaceConfig with task, bug, story, epic artifact types
- [ ] #2 DefaultRegistry() returns routing rules mapping statuses to directories
- [ ] #3 Generated defaults pass validation when loaded through the config loader
<!-- AC:END -->
