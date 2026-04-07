---
title: backlogit
description: AI-native agile workspace with MCP and CLI interfaces
author: backlogit contributors
ms.date: 2026-04-01
ms.topic: overview
keywords:
  - backlogit
  - mcp
  - agile
  - ai agents
  - task management
---

# backlogit

AI-native agile workspace with MCP and CLI interfaces.

![Go Version](https://img.shields.io/badge/go-1.24-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Build](https://img.shields.io/badge/build-passing-brightgreen)

## Overview

backlogit stores features, tasks, and subtasks as individual Markdown files with strictly typed YAML frontmatter. These files travel with your codebase in Git, remain readable by humans, and merge cleanly without specialized tooling. The Markdown layer is the permanent source of truth: every field, status, and description lives in a file you can open in any editor.

Alongside the Markdown files, backlogit maintains an ephemeral SQLite cache called `backlogit.db`. This cache is gitignored and fully disposable. When agents need to find work, they execute targeted SQL queries against the index rather than scanning hundreds of Markdown files. A query like `SELECT id, title FROM items WHERE artifact_type='task' AND status='active'` costs roughly 20 tokens; reading the equivalent files would cost tens of thousands. The cache rebuilds automatically from the Markdown source whenever it is missing or stale.

A JSONL event model records state transitions, comments, and agent telemetry in append-only files. Work-item history is written per item to `.backlogit/logs/{item-id}.jsonl`, while telemetry stays in `.backlogit/telemetry.jsonl`. This separation keeps the Markdown artifacts concise, the cache disposable, and the history durable. The architecture follows Command Query Responsibility Segregation: writes go to Markdown files, reads go to SQLite, and history flows into JSONL.

## Features

- Hybrid CQRS architecture keeps human-readable Markdown files as the source of truth while SQLite serves token-efficient agent queries
- MCP server exposes backlog, queue, stash, search, and planning tools over JSON-RPC 2.0 for integration with Claude Code, GitHub Copilot CLI, Cursor, and any MCP-compatible client
- CLI commands cover the full artifact lifecycle plus stash workflows: create, list, query, update, move, archive, migrate, the `stash` command group (`add`, `fetch-stash`, `harvest`), and `deliberate`
- FTS5 full-text search across artifact titles and descriptions via the `search` command and `backlogit_search_items` MCP tool
- Dependency tracking between artifacts with `dep add`, `dep list`, and `backlogit_get_dependencies`
- Work queue prioritization via `backlogit queue` and `backlogit_get_queue`
- Stash workflow for deferred work in `.backlogit/stash.jsonl`, including priority-tagged entries, linked deliberation artifacts, filtered or grouped fetch, and single-item or batch harvest flows for feature-set planning
- Unified metadata discovery for agents via `backlogit metadata catalog`, `backlogit metadata export-command-map`, and matching MCP tools
- Agent memory and checkpoint persistence through `backlogit_save_memory` and `backlogit_create_checkpoint`
- Migration from current Backlog.md workspaces and legacy checklist files with `backlogit migrate --source`
- Single CGo-free static binary built with `modernc.org/sqlite`
- Workspace containment: all operations stay within `.backlogit/` with path traversal rejection

## Quick Start

**Install from source:**

```bash
go install github.com/backlogit/backlogit/cmd/backlogit@latest
```

**Initialize a workspace:**

```bash
cd your-project
backlogit init
```

**Create a task:**

```bash
backlogit add --type task --title "Implement authentication" --status active
```

**Stash deferred work for later planning:**

```bash
backlogit stash add "Capture follow-up hardening ideas" --kind feature --priority high
backlogit deliberate ABCD1234 --chosen-direction "Keep the initial scope narrow and defer follow-up polish"
backlogit stash fetch-stash --group-by-priority
backlogit stash harvest --priority critical --type task
```

**Discover metadata and export an agent command map:**

```bash
backlogit metadata catalog
backlogit metadata export-command-map .github\instructions\backlogit-command-map.md
```

**Start the MCP server:**

```bash
backlogit mcp
```

## Table of Contents

- [Installation](docs/installation.md)
- [Workflow Guide](docs/workflow.md)
- [Configuration Reference](docs/configuration.md)
- [Why backlogit](docs/rationale.md)
- [backlogit vs Backlog.md](docs/backlogit-vs-backlog-md.md)
- [Migration Guide](docs/migration-guide.md)

## Technology Stack

| Component        | Technology                           | Notes                                      |
|------------------|--------------------------------------|--------------------------------------------|
| Language         | Go 1.24                              | Single static binary, no CGo required      |
| MCP protocol     | mark3labs/mcp-go v0.27.0             | JSON-RPC 2.0 over stdio                    |
| Database         | SQLite via modernc.org/sqlite v1.34.0 | WAL mode, FTS5, CGo-free, gitignored       |
| CLI framework    | spf13/cobra v1.8.1                   | Artifact, queue, and stash commands        |
| Validation       | go-playground/validator/v10 v10.30.1 | Struct tags on all boundary types          |
| Configuration    | gopkg.in/yaml.v3 v3.0.1              | config.yaml, header-def.yaml, registry.yaml, migration.yaml |
| File format      | Markdown + YAML frontmatter          | Git-friendly source of truth               |
| Event stream     | JSONL (append-only)                  | per-item logs plus telemetry.jsonl         |
| License          | MIT                                  |                                            |

## Contributing

Contributions are welcome. Please read the contributing guidelines before opening a pull request. All code must pass `golangci-lint run`, `go vet ./...`, and `go test ./...` with zero failures before review.

## License

MIT. See [LICENSE](LICENSE) for details.
