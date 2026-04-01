---
title: backlogit vs Backlog.md
description: Feature comparison and architectural differences
author: backlogit contributors
ms.date: 2026-04-01
ms.topic: concept
keywords:
  - backlogit
  - backlog.md
  - comparison
  - migration
---

## Introduction

Backlog.md is a Markdown-based task management format that stores work items as checklists in a single `Backlog.md` file, with companion tooling to parse and display them. backlogit takes a different approach: it stores each artifact as its own Markdown file with YAML frontmatter, maintains an ephemeral SQLite cache for querying, and exposes a Model Context Protocol server for AI agent integration. Both tools embrace Git-friendly text files, but they target different scales of use and different consumer profiles.

## Feature Comparison

| Feature              | Backlog.md                         | backlogit                                      |
|----------------------|------------------------------------|------------------------------------------------|
| Storage model        | Single checklist file              | Individual Markdown files with YAML frontmatter |
| Query API            | None (file parsing only)           | SQL via SQLite FTS5 index                      |
| Agent integration    | None                               | 21 MCP tools over JSON-RPC 2.0                 |
| Token efficiency     | Full file in context               | Targeted SQL results (20-100 tokens per query) |
| Type system          | Implicit (checklist sections)      | Explicit: task, story, bug, epic (configurable) |
| Dependency tracking  | Not supported                      | First-class via `dep` commands and MCP tools   |
| Work queue           | Not supported                      | `backlogit queue` and `backlogit_get_queue`    |
| Migration support    | N/A                                | `backlogit migrate --adapter backlog-md`       |
| Event history        | Not supported                      | Append-only JSONL event stream                 |
| Status values        | Checked / unchecked                | queued, active, blocked, in_review, done, archived |
| Multi-user Git merge | Section-level conflicts possible   | Per-file, conflict-free concurrent edits       |
| Agent memory         | Not supported                      | `backlogit_save_memory`, checkpoints           |

## Architectural Differences

Backlog.md uses a single-file design. All tasks live in one document, organized by checklist sections. This works well for small projects and solo developers: the file is easy to read, easy to edit, and requires no tooling to understand. The tradeoff is that the file grows as the project grows, making it harder to query, and concurrent edits from multiple contributors or agents produce merge conflicts at the section level.

backlogit stores each artifact as a separate file in a structured directory hierarchy defined by `registry.yaml`. A project with 200 tasks has 200 small Markdown files rather than one large one. Each file merges independently, so concurrent edits from multiple developers or agents produce no conflicts unless two contributors edit the exact same artifact simultaneously.

The SQLite cache is the second architectural distinction. Backlog.md has no query layer: consumers read the whole file. backlogit builds an ephemeral index that supports `SELECT` queries, `WHERE` filtering, `JOIN` across artifact types, and FTS5 full-text search. The cache is disposable and rebuilds from the Markdown files automatically.

## Token Efficiency

For AI agents working inside context-limited LLMs, token cost matters. Backlog.md requires loading the entire file to answer any question about its contents. A mature project file might contain 10,000 tokens or more. An agent that needs to find three blocked bugs must consume all of them to find the three it cares about.

backlogit allows the agent to ask precisely:

```sql
SELECT id, title FROM items WHERE type='bug' AND status='blocked' ORDER BY updated_at DESC
```

This query returns the relevant rows in under 100 tokens. The agent sees only what it asked for. Context window capacity goes toward reasoning and code generation rather than parsing task lists.

## When to Use Each Tool

| Scenario                                          | Recommended Tool |
|---------------------------------------------------|------------------|
| Solo project, small backlog, no AI agents         | Backlog.md       |
| Team project with AI agent integration            | backlogit        |
| Existing Backlog.md project adopting AI tooling   | Migrate to backlogit |
| Quick personal task list, no tooling required     | Backlog.md       |
| MCP-connected workflow with Claude Code or Cursor | backlogit        |
| Project needs dependency graphs and work queue    | backlogit        |
| Minimal setup, single file, Git committed         | Backlog.md       |
| Agent needs audit trail and telemetry             | backlogit        |

## Compatibility Note

backlogit can read and migrate Backlog.md files. The `migrate` command parses the Backlog.md checklist format and converts each checklist item into a typed backlogit artifact with YAML frontmatter. The migration preserves titles, statuses, and section groupings where possible.

```bash
backlogit migrate --source ./Backlog.md --adapter backlog-md --dry-run
```

Run `--dry-run` first to preview the migration output without writing any files. See the [Migration Guide](migration-guide.md) for step-by-step instructions.
