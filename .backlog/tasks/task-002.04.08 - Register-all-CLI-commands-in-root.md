---
id: TASK-002.04.08
title: Register all CLI commands in root
status: done
assignee: []
created_date: '2026-03-30 07:01'
labels: []
dependencies:
  - TASK-002.04.01
  - TASK-002.04.02
  - TASK-002.04.03
  - TASK-002.04.04
  - TASK-002.04.05
  - TASK-002.04.06
  - TASK-002.04.07
parent_task_id: TASK-002.04
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Register all new CLI commands in `NewRootCommand` in `internal/cli/root.go`. Add `root.AddCommand` calls for: add, list, get, update, move, delete, search, query, status. Verify command help text, flag definitions, and that no flag name collisions exist across commands.

**Files:** `internal/cli/root.go`
**Test files:** `internal/cli/root_test.go` (new)
**Patterns:** Follow existing `root.AddCommand` calls at `internal/cli/root.go` lines 37-39
**Verification:** `backlogit --help` lists all commands. Each command's `--help` produces valid documentation.
<!-- SECTION:DESCRIPTION:END -->

