---
id: TASK-001.10.01
title: Implement legacy backlog.md AST parser
status: Done
assignee: []
created_date: '2026-03-30 01:45'
labels: []
dependencies: []
parent_task_id: TASK-001.10
priority: medium
ordinal: 10100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/parser/legacy.go` with:

1. `ParseLegacy(content string) ([]LegacyItem, error)` — parses a monolithic backlog.md file using AST heuristics:
   - Section heading conventions: `# Backlog`, `## In Progress`, `### Done` → status mapping by position
   - Checklist conventions: `[ ] Task Name` → status `todo`, `[x] Completed Task` → status `done`
   - Heading depth → parent-child inference: H2 feature → H3 sub-task
   - Legacy YAML frontmatter extraction
2. `LegacyItem` struct: Title, Status, ParentTitle, Depth, Description, Frontmatter

Create `internal/parser/legacy_test.go` with table-driven tests using sample legacy backlog.md content.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ParseLegacy correctly maps # Backlog, ## In Progress, ### Done headings to status fields
- [ ] #2 ParseLegacy maps [ ] items to todo and [x] items to done
- [ ] #3 ParseLegacy infers parent-child hierarchy from heading depth (H2 parent to H3 child)
- [ ] #4 Handles mixed formatting: nested headings with checklists at various depths
- [ ] #5 Tests cover standard legacy formats with nested headings and checklists
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.