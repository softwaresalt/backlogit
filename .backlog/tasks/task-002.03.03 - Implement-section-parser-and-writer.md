---
id: TASK-002.03.03
title: Implement section parser and writer
status: To Do
assignee: []
created_date: '2026-03-30 06:59'
labels: []
dependencies:
  - TASK-002.03.01
parent_task_id: TASK-002.03
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement section parsing and writing in `internal/parser/sections.go`:

- `ParseSections(content string) (map[string]string, error)` — extracts named sections from markdown content between `<!-- BEGIN:{name} -->` and `<!-- END:{name} -->` tags
- `WriteSections(content string, updates map[string]string) (string, error)` — replaces section content while preserving the rest of the document
- `WriteSection(content string, name string, value string) (string, error)` — single-section updates

Handle edge cases: nested HTML comments, missing end tags (return error), empty sections, sections with leading/trailing whitespace preservation.

**Files:** `internal/parser/sections.go` (new)
**Test files:** `internal/parser/sections_test.go` (new)
**Patterns:** Follow `ParseFrontmatter` pattern in `internal/models/frontmatter.go`
**Verification:** `go test ./internal/parser/...` passes with round-trip section parse/write tests. Edge case tests: empty sections, sections with markdown content, missing tags produce errors.
<!-- SECTION:DESCRIPTION:END -->
