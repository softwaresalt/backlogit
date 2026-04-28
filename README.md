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

- CQRS architecture: Markdown files as source of truth, ephemeral SQLite for token-efficient agent queries, JSONL for append-only history
- MCP server over JSON-RPC 2.0 stdio — integrates with Claude Code, GitHub Copilot CLI, Cursor, and any MCP-compatible client
- Full CLI for the artifact lifecycle: create, list, query, update, move, archive, stash workflows, queue management, metadata discovery, and workspace integrity checks
- Workspace doctor: `backlogit doctor` detects orphaned artifacts and duplicate IDs across queue and archive, with text and JSON output formats
- Single CGo-free static binary; workspace-contained with path traversal rejection
- Agent-native: version surface, two-layer hooks, token telemetry, commit traceability, and dependency tracking baked in
- Session disaster recovery: standardized checkpoint schema, MCP lifecycle tools (`list_checkpoints`, `get_checkpoint`, `resolve_checkpoint`, `cleanup_checkpoints`), and deterministic agent recovery state machine for interrupted sessions

## Plugin Installation

Install backlogit as a Copilot CLI plugin to get the Stage and Ship agents, 19 universal workflow skills, and the MCP server in one step:

```bash
copilot plugin install softwaresalt/backlogit
```

The plugin uses `npx @backlogit/backlogit-mcp` to run the backlogit binary. On first invocation the binary is downloaded and cached (~10–30 s). To pre-install and avoid the first-run download:

```bash
npm install -g @backlogit/backlogit-mcp   # pre-download
# — or —
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest  # native binary
```

See [docs/plugin-guide.md](docs/plugin-guide.md) for full installation options and troubleshooting.

---

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

**Run workspace integrity checks:**

```bash
backlogit doctor --check-orphans --check-duplicates
backlogit doctor --format json
```

**Discover metadata and export an agent command map:**

```bash
backlogit metadata catalog
backlogit metadata export-command-map .github\instructions\backlogit-command-map.md
```

**Manage agent session checkpoints:**

```bash
backlogit checkpoint list --agent ship --status active
backlogit checkpoint get checkpoint-20260424-083000.json
backlogit checkpoint resolve checkpoint-20260424-083000.json
backlogit checkpoint cleanup --retention-days 7
```

**Start the MCP server:**

```bash
backlogit mcp
```

## Guides

| Topic | Link |
|---|---|
| Installation | [docs/installation.md](docs/installation.md) |
| Workflow overview | [docs/workflow.md](docs/workflow.md) |
| Configuration reference | [docs/configuration.md](docs/configuration.md) |
| Why backlogit | [docs/rationale.md](docs/rationale.md) |
| backlogit vs Backlog.md | [docs/backlogit-vs-backlog-md.md](docs/backlogit-vs-backlog-md.md) |
| Migration guide | [docs/migration-guide.md](docs/migration-guide.md) |
| CLI command reference | [docs/cli-reference/](docs/cli-reference/README.md) |

## Technology Stack

| Component        | Technology                           | Notes                                      |
|------------------|--------------------------------------|--------------------------------------------|
| Language         | Go 1.24                              | Single static binary, no CGo required      |
| MCP protocol     | mark3labs/mcp-go v0.27.0             | JSON-RPC 2.0 over stdio                    |
| Database         | SQLite via modernc.org/sqlite v1.34.0 | WAL mode, FTS5, CGo-free, gitignored       |
| CLI framework    | spf13/cobra v1.10.2                  | Artifact, queue, and stash commands        |
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
