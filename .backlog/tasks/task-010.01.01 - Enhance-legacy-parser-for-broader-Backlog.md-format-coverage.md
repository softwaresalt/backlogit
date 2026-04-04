---
id: TASK-010.01.01
title: Enhance legacy parser for broader Backlog.md format coverage
status: Done
assignee: []
created_date: '2026-04-01 22:34'
updated_date: '2026-04-01 23:38'
labels:
  - go
dependencies: []
parent_task_id: TASK-010.01
priority: medium
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Enhance `internal/parser/legacy.go` to handle a broader range of Backlog.md formats beyond basic heading/checklist heuristics.

Current state: `ParseLegacy()` handles H1-H3 headings and `- [x]`/`- [ ]` checklist items. Returns `[]LegacyItem` with title, status, parent, depth, description.

Enhancements needed:
- Parse nested heading hierarchies (H1 through H4) to preserve artifact parent-child relationships
- Recognize section-based document types: specs (requirements), plans (implementation), decisions (ADRs)
- Extract metadata from inline patterns: priority markers, assignee mentions, date references
- Handle Backlog.md-specific conventions: sprint groupings, milestone headers, tag annotations
- Preserve description content (paragraph text between checklist items) as artifact body

Files to modify:
- `internal/parser/legacy.go` (extend `ParseLegacy()`)
- `internal/parser/legacy_test.go` (new test cases)

Verification: `go test ./internal/parser/...` passes with new edge case coverage.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Parser handles nested heading hierarchies (H1-H4) for section-based artifact extraction
- [ ] #2 Specs, plans, and decision documents are recognized and categorized correctly
- [ ] #3 Existing tests continue to pass; new tests cover extended format coverage
<!-- AC:END -->
