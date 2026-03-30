---
id: TASK-001.03.03
title: Implement Markdown file parser
status: Done
assignee: []
created_date: '2026-03-30 01:39'
labels: []
dependencies: []
parent_task_id: TASK-001.03
priority: high
ordinal: 3300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/parser/markdown.go` with:

1. `ParseMarkdownFile(path string) (*models.Artifact, string, error)` — reads a Markdown file from disk, calls `ParseFrontmatter` to extract YAML, then `ArtifactFromFrontmatter` to convert to typed struct. Returns the artifact and raw body text.

This was moved from Unit 9 (Legacy Migration) to Unit 3 per review P1-01 because Unit 5 (Rehydration) and Unit 7 (MCP get_item) depend on it.

Create `internal/parser/markdown_test.go` with tests for valid files, files without frontmatter, malformed YAML, empty files.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ParseMarkdownFile reads a file and returns *Artifact and body text
- [ ] #2 Returns ErrValidation for files with malformed frontmatter
- [ ] #3 Correctly handles files without frontmatter (returns nil artifact, full body)
- [ ] #4 Tests use t.TempDir() with sample .md files
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.