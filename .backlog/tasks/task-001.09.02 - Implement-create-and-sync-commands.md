---
id: TASK-001.09.02
title: Implement create and sync commands
status: To Do
assignee: []
created_date: '2026-03-30 01:44'
labels: []
dependencies: []
parent_task_id: TASK-001.09
priority: high
ordinal: 9200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/cli/create.go` with:

1. `backlogit create` command — flags: `--type` (required), `--title` (required), `--description`, `--parent`, `--sprint`, `--status`, `--priority`
2. Resolves workspace, loads config, opens Workspace, calls `core.CreateArtifact` with options
3. Outputs: `Created {artifact_id} at {file_path}`

Create `internal/cli/sync.go` with:

1. `backlogit sync` command — no additional flags
2. Resolves workspace, opens DB, calls `db.Rehydrate`
3. Outputs: `Rehydrated {count} items in {duration}`

Create `internal/cli/create_test.go` and `internal/cli/sync_test.go` with tests.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit create --type bug --title 'Login crash' creates properly named Markdown file
- [ ] #2 backlogit create validates --type against config artifact_types
- [ ] #3 backlogit sync triggers rehydration and reports indexed count
- [ ] #4 Tests verify file creation, validation errors, and sync output
<!-- AC:END -->
