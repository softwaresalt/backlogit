---
id: TASK-002.01.03
title: Update frontmatter parser and serializer for new fields
status: To Do
assignee: []
created_date: '2026-03-30 06:57'
labels: []
dependencies:
  - TASK-002.01.01
parent_task_id: TASK-002.01
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Update `ArtifactFromFrontmatter` to extract `assigned_to`, `owner`, `labels`, `dependencies`, `references`, and `commit` from the frontmatter map. Update `SerializeFrontmatter` to emit these fields. Slice fields serialize as YAML sequences. Ensure round-trip fidelity: parse → serialize → parse produces identical results.

**Files:** `internal/models/frontmatter.go`
**Test files:** `internal/models/frontmatter_test.go`
**Patterns:** Follow `ArtifactFromFrontmatter` and `SerializeFrontmatter` patterns
**Verification:** `go test ./internal/models/...` passes with round-trip frontmatter tests for all new fields.
<!-- SECTION:DESCRIPTION:END -->
