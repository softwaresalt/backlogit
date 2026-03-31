---
id: TASK-001.03.02
title: Implement frontmatter parser and serializer
status: Done
assignee: []
created_date: '2026-03-30 01:39'
labels: []
dependencies: []
parent_task_id: TASK-001.03
priority: high
ordinal: 3200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/models/frontmatter.go` with:

1. `ParseFrontmatter(content string) (map[string]any, string, error)` — extracts YAML between `---` delimiters, returns parsed map and remaining body text. Returns `(nil, content, nil)` if no frontmatter found.
2. `ArtifactFromFrontmatter(fm map[string]any, body string) (*Artifact, error)` — converts raw frontmatter map to typed Artifact struct with validation (per review P1-04)
3. `SerializeFrontmatter(fields map[string]any, body string) string` — produces `---\n{yaml}\n---\n\n{body}\n` with sorted YAML keys for deterministic output (per constitution VIII)

Create `internal/models/frontmatter_test.go` with table-driven tests covering round-trips, edge cases, and malformed YAML.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ParseFrontmatter extracts YAML between --- delimiters and returns map[string]any + body
- [ ] #2 ArtifactFromFrontmatter converts map[string]any to typed *Artifact struct
- [ ] #3 SerializeFrontmatter produces valid YAML frontmatter with sorted keys
- [ ] #4 Round-trip test: parse then serialize produces functionally identical output
- [ ] #5 Edge cases handled: no frontmatter, empty body, special characters in values
<!-- AC:END -->


## Implementation Notes

Completed in commit 83cebfc. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.