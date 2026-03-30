---
id: TASK-001.03
title: 'Models, Frontmatter, and Markdown Parser'
status: To Do
assignee: []
created_date: '2026-03-30 01:36'
updated_date: '2026-03-30 01:46'
labels:
  - epic
dependencies:
  - TASK-001.02
parent_task_id: TASK-001
priority: high
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Define core data models for artifacts, frontmatter parsing/serialization, Markdown file parsing, and sprint containers. These are the data contracts used across all packages.

Per review P1-01: `internal/parser/markdown.go` (ParseMarkdownFile) is included here instead of Unit 9 because Unit 5 (Rehydration) and Unit 7 (MCP get_item) depend on it.

Per review P1-04: `ParseFrontmatter` returns `(map[string]any, string, error)` for raw parse. `ArtifactFromFrontmatter(fm, body)` converts to typed `*Artifact` struct.
<!-- SECTION:DESCRIPTION:END -->
