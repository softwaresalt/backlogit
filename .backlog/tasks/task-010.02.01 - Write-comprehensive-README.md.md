---
id: TASK-010.02.01
title: Write comprehensive README.md
status: done
assignee: []
created_date: '2026-04-01 22:30'
labels:
  - docs
dependencies: []
parent_task_id: TASK-010.02
priority: high
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Rewrite `README.md` from its current placeholder (`# backlogit`) into a comprehensive project README.

Content to include:
- Project name, tagline, and badges (Go version, license, build status)
- Architecture overview: hybrid data architecture (Markdown source of truth → SQLite cache → JSONL events)
- Feature highlights: 21 MCP tools, 16 CLI commands, WIT type system, dependency graph, work queue
- Quick-start example: `go install`, `backlogit init`, `backlogit add`, `backlogit mcp`
- Table of contents linking to docs/ guides
- Technology stack table (Go, SQLite, mcp-go, Cobra, YAML)
- Contributing section pointing to future CONTRIBUTING.md
- License section (MIT)

Files to modify:
- `README.md` (rewrite)

Verification: README renders correctly on GitHub with all links resolving.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 README.md contains project overview, architecture summary, feature highlights, and quick-start
- [ ] #2 All internal links resolve to existing files or planned doc paths
- [ ] #3 Renders correctly as GitHub-flavored Markdown
<!-- AC:END -->
