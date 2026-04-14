---
title: backlogit
description: AI-native agile workspace with MCP and CLI interfaces
author: backlogit contributors
ms.date: 2026-04-14
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
- MCP server exposes backlog, queue, stash, search, planning, and integrity tools over JSON-RPC 2.0 for integration with Claude Code, GitHub Copilot CLI, Cursor, and any MCP-compatible client
- CLI commands cover the full artifact lifecycle plus stash workflows: create, list, query, update, move, archive, migrate, the `stash` command group (`add`, `list`, `get`, `edit`, `remove`, `harvest`), and `deliberate`
- Workspace integrity diagnostics via `backlogit_doctor` MCP tool: detects orphaned level-2+ artifacts (tasks and subtasks missing a parent) and duplicate artifact IDs across workspace directories, with an exemption for items that were intentionally returned to the backlog
- Hierarchy enforcement ensures level-2+ artifact types always require a `parent_id` at creation time, catching orphaned work items before they enter the queue
- Post-shipment consistency verification runs automatically inside `ShipShipment`: after archiving, every archived ID is confirmed absent from active queue directories to detect partial archive failures early
- FTS5 full-text search across artifact titles and descriptions via the `search` command and `backlogit_search_items` MCP tool
- Dependency tracking between artifacts with `dep add`, `dep list`, and `backlogit_get_dependencies`
- Work queue prioritization via `backlogit queue` and `backlogit_get_queue`, with stable pagination (`ORDER BY id ASC`), orphan filtering (excludes items whose parent feature is done or archived), and a compact list mode that returns only `id`, `title`, `status`, `type`, and `parent_id` for token-efficient agent queries
- Duplicate title detection via `backlogit_query_sql` to surface items with matching titles across different IDs, useful after migrations and manual edits
- Stash workflow for deferred work in `.backlogit/stash.jsonl`, including priority-tagged entries, linked deliberation artifacts, full CRUD operations (`add`, `list`, `get`, `edit`, `remove`), filtered or grouped fetch, and single-item or batch harvest flows for feature-set planning
- Unified metadata discovery for agents via `backlogit metadata catalog`, `backlogit metadata export-command-map`, and matching MCP tools
- Agent memory and checkpoint persistence through `backlogit_save_memory` and `backlogit_create_checkpoint`
- Migration from current Backlog.md workspaces and legacy checklist files with `backlogit migrate --source`
- Single CGo-free static binary built with `modernc.org/sqlite`
- Workspace containment: all operations stay within `.backlogit/` with path traversal rejection
- Token telemetry pipeline via the `backlogit telemetry` command group and `backlogit_telemetry_harvest` MCP tool: `harvest` parses Copilot CLI session logs (with optional `--since` and `--force` flags for incremental or forced re-harvest), correlates model calls and tool calls into per-session summaries with context-window utilization metrics, attributes tool calls to MCP servers, writes typed JSONL records, and rehydrates queryable `telemetry_sessions` and `telemetry_tool_usage` SQLite tables; `report` renders tabular or JSON summaries grouped by session or server with configurable `--limit`; `list` shows a per-session summary table; `top` shows top servers by call volume
- Commit traceability on event log entries: MCP mutation tools (`backlogit_move_item`, `backlogit_archive_item`, `backlogit_append_comment`) accept an optional `commit_sha` parameter that is recorded on the emitted JSONL event, enabling external tooling to correlate backlogit state changes with git history
- Two-layer hooks system for lifecycle governance: synchronous pre/post hooks fire on all lifecycle operations (create, update, archive, ship, adopt) with priority ordering and error-stops-chain semantics on pre-hooks; built-in `ValidateStatusTransition` pre-hook enforces config-driven status transition rules; `EmitHookEvent` and `LogIndexStale` post-hooks emit structured JSONL events and mark the index stale; external webhook dispatch via `WebhookNotifier` sends async HTTP POST notifications to configured endpoints with rate limiting, event filtering, and environment-variable header expansion; hooks configuration in `.backlogit/hooks.yaml` controls transition maps, webhook endpoints, and notification settings

## Quick Start

**Install from source:**

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
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
backlogit stash list --kind feature
backlogit stash get ABCD1234
backlogit stash edit ABCD1234 --priority critical
backlogit deliberate ABCD1234 --chosen-direction "Keep the initial scope narrow and defer follow-up polish"
backlogit stash list --group-by-priority
backlogit stash harvest --priority critical --type task
backlogit stash remove ABCD1234
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
| Configuration    | gopkg.in/yaml.v3 v3.0.1              | config.yaml, header-def.yaml, registry.yaml, hooks.yaml, migration.yaml |
| Rate limiting    | golang.org/x/time v0.11.0            | Webhook dispatch backpressure via rate.Limiter |
| File format      | Markdown + YAML frontmatter          | Git-friendly source of truth               |
| Event stream     | JSONL (append-only)                  | per-item logs plus telemetry.jsonl         |
| License          | MIT                                  |                                            |

## Contributing

Contributions are welcome. Please read the contributing guidelines before opening a pull request. All code must pass `golangci-lint run`, `go vet ./...`, and `go test ./...` with zero failures before review.

## License

MIT. See [LICENSE](LICENSE) for details.
