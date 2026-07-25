---
chunk_strategy: h1-h2-h3
description: AI-native agile workspace with MCP and CLI interfaces
doc_type: guide
docline:
    author: backlogit contributors
    keywords:
        - backlogit
        - mcp
        - agile
        - ai agents
        - task management
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: overview
ingested_at: "2026-06-26T02:34:51Z"
schema_version: "1.0"
source: README.md
title: backlogit
---

# backlogit

AI-native agile workspace with MCP and CLI interfaces.

![Go Version](https://img.shields.io/badge/go-1.24-blue)
![License](https://img.shields.io/badge/license-Apache--2.0-green)
![Build](https://img.shields.io/badge/build-passing-brightgreen)

backlogit is an AI-native agile workspace. Your backlog lives as plain Markdown in your repository and is served to AI agents through both an MCP server and a full command-line interface. Work items travel with your code in Git, stay readable in any editor, and merge without specialized tooling.

It is built for engineering teams and AI coding agents that want Git-native, human-readable work tracking with no database server to run and no SaaS to subscribe to. Agents query and update the backlog as quickly as they read source, so planning stays in the same loop as the code.

Three concerns stay separate by design: Markdown files are the durable source of truth, an ephemeral SQLite cache keeps agent reads token-cheap, and an append-only JSONL log preserves history. The result is a single static binary that stays contained to your workspace.

## At a glance

| Aspect | Summary |
|---|---|
| **What it is** | An AI-native agile workspace: your backlog as Markdown, served to agents over MCP and a CLI |
| **Who it's for** | Engineering teams and AI coding agents that want Git-native, human-readable work tracking |
| **Interfaces** | An MCP server over stdio and a full command-line interface |
| **Works with** | Claude Code, GitHub Copilot CLI, Cursor, and any MCP-compatible client |
| **Storage** | Markdown with YAML frontmatter as the source of truth, plus an ephemeral SQLite query cache |
| **Runtime** | A single static, CGo-free binary — no database server, no SaaS |
| **License** | Apache-2.0 |

## Features

- CQRS architecture: Markdown files as source of truth, ephemeral SQLite for token-efficient agent queries, JSONL for append-only history
- MCP server over JSON-RPC 2.0 stdio — integrates with Claude Code, GitHub Copilot CLI, Cursor, and any MCP-compatible client
- Full CLI for the artifact lifecycle: create, list, query, update, move, archive, stash workflows, queue management, metadata discovery, and workspace integrity checks
- Workspace doctor: `backlogit doctor` detects orphaned artifacts and duplicate IDs across queue and archive, with text and JSON output formats
- Single CGo-free static binary; workspace-contained with path traversal rejection
- Agent-native: version surface, two-layer hooks, token telemetry, commit traceability, and dependency tracking baked in
- Session disaster recovery: standardized checkpoint schema, MCP lifecycle tools (`list_checkpoints`, `get_checkpoint`, `resolve_checkpoint`, `cleanup_checkpoints`), and deterministic agent recovery state machine for interrupted sessions

## Overview

backlogit stores features, tasks, and subtasks as individual Markdown files with strictly typed YAML frontmatter. These files travel with your codebase in Git, remain readable by humans, and merge cleanly without specialized tooling. The Markdown layer is the permanent source of truth: every field, status, and description lives in a file you can open in any editor.

Alongside the Markdown files, backlogit maintains an ephemeral SQLite cache called `backlogit.db`. This cache is gitignored and fully disposable. When agents need to find work, they execute targeted SQL queries against the index rather than scanning hundreds of Markdown files. A query like `SELECT id, title FROM items WHERE artifact_type='task' AND status='active'` costs roughly 20 tokens; reading the equivalent files would cost tens of thousands. Because the cache is disposable, you rebuild it from the Markdown source with `backlogit sync` whenever it is missing or out of date.

A JSONL event model records state transitions, comments, and telemetry. Work-item history is appended per item to `.backlogit/logs/{item-id}.jsonl`, and agent-operation telemetry is appended to `.backlogit/telemetry.jsonl`. Harvested Copilot CLI session summaries are materialized to `.backlogit/telemetry-sessions.jsonl`, which backlogit rewrites atomically on each harvest. This separation keeps the Markdown artifacts concise, the cache disposable, and the history durable. The architecture follows Command Query Responsibility Segregation: writes go to Markdown files, reads go to SQLite, and history flows into JSONL.

## Plugin Installation

### Which installation path

Choose one path before you run an install command:

* Path A - Standalone backlogit: choose this when you want to use backlogit as
  a self-contained Copilot CLI product. The standalone plugin bundles the Stage
  and Ship agents, 19 universal workflow skills, and MCP server configuration
  for a preinstalled `backlogit` runtime.
* Path B - Autoharness-composed harness: choose this when backlogit is part of
  an Autoharness-generated agent harness. Autoharness composes the
  workspace-specific constitution, policies, instructions, agents, and skills
  from templates and writes them into the repository's `.github/` directory
  through its install or tune flow. This replaces the standalone plugin's
  agent and skill bundle, not the backlogit runtime. Make sure the
  `backlogit` binary is available when the generated harness calls
  `backlogit` or `backlogit mcp`, but do not also install the standalone
  plugin for that same repo harness.

The two paths share backlogit concepts, but they install different surfaces.
The standalone install manifest is
[`.github/plugin/plugin.json`](.github/plugin/plugin.json), and it references
the bundled agents and skills under `plugin/`. This repository also contains
`.autoharness/` and generated `.github/` materials, which are the Autoharness
path.

### Path A - Install standalone backlogit

Before you install or run the plugin, install the native `backlogit` binary and
make sure it is on your PATH. The plugin starts the MCP server with
`backlogit mcp`, using the same direct executable lookup as other native CLI
plugins. It does not require Node.js or a JavaScript package manager.

Use `go install` when you want the Go toolchain workflow:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
```

You can also use the one-line installers in the Quick Start or download a
SHA256-verified binary from
[GitHub Releases](https://github.com/softwaresalt/backlogit/releases).

Then install backlogit as a Copilot CLI plugin with the canonical
cross-platform command:

```bash
copilot plugin install softwaresalt/backlogit
```

The plain `softwaresalt/backlogit` owner/repo form works because Copilot CLI
finds the canonical manifest at `.github/plugin/plugin.json`. For local
development from a clone, use `copilot plugin install ./` from the repository
root.

Copilot CLI is deprecating direct repo/URL/local-path installs in favor of
marketplace installs. To future-proof, register the bundled marketplace and
install from it instead:

```bash
copilot plugin marketplace add softwaresalt/backlogit
copilot plugin install backlogit@softwaresalt
```

See [docs/plugin-guide.md](docs/plugin-guide.md) for full installation options and troubleshooting.

### Path B - Use backlogit through Autoharness

If your repo uses Autoharness, use the Autoharness install or tune flow instead
of the standalone plugin commands above. That flow writes the tailored harness
into `.github/`, including repo-local agents, instructions, and skills. See
[docs/installation.md](docs/installation.md#which-installation-path) and
[docs/plugin-guide.md](docs/plugin-guide.md#skill-locations-and-drift) for the
standalone-versus-Autoharness split, including when to install only the
backlogit binary/runtime.

---

## Quick Start

Start here after you choose the installation path above. Path A users can run
the standalone backlogit commands directly. Path B users should run these
commands only after Autoharness has installed or tuned the repo-local harness.

**Recommended install — Windows PowerShell:**

```powershell
irm https://raw.githubusercontent.com/softwaresalt/backlogit/main/scripts/install/install.ps1 | iex
```

**Recommended install — Linux and macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/softwaresalt/backlogit/main/scripts/install/install.sh | sh
```

These one-line installers mirror
[Installation Method 1](docs/installation.md#method-1-one-line-install) and
match the scripts in [`scripts/install/`](scripts/install/). They download the
latest GitHub Release asset, verify it with `SHA256SUMS`, and install the
`backlogit` binary into a user-writable directory.

If your organization requires review before executing remote scripts, download
the installer script first or use a SHA256-verified binary from GitHub
Releases.

**Secondary install alternatives:**

Use `go install` when you want the Go toolchain workflow:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
```

Or download a SHA256-verified binary from
[GitHub Releases](https://github.com/softwaresalt/backlogit/releases). Path A
users who want the bundled Copilot CLI agents and skills can install the
standalone plugin after `backlogit` is on PATH:

```bash
copilot plugin install softwaresalt/backlogit
```

That owner/repo command is the canonical cross-platform plugin install path. It
installs the bundled plugin from the `.github/plugin/plugin.json` manifest.

**Initialize a workspace:**

```bash
cd your-project
backlogit init
```

**Create a feature and task:**

```bash
backlogit add --type feature --title "Authentication hardening" --status active
backlogit add --type task --title "Implement authentication" --parent 001-F --status active
```

**Stash deferred work for later planning:**

```bash
backlogit stash add "Capture follow-up hardening ideas" --kind feature --priority high
backlogit stash list --kind feature
backlogit stash get ABCD1234
backlogit stash edit ABCD1234 --priority critical
backlogit deliberate ABCD1234 --chosen-direction "Keep the initial scope narrow and defer follow-up polish"
backlogit stash list --group-by-priority
backlogit stash harvest --priority critical --type feature
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
| Pre-task-completion gate | [docs/pre-task-completion-gate.md](docs/pre-task-completion-gate.md) |
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
| Event stream     | JSONL                                | append-only per-item logs and telemetry.jsonl; materialized telemetry-sessions.jsonl summary |
| License          | Apache-2.0                           |                                            |

## Contributing

Contributions are welcome. Please read the contributing guidelines before opening a pull request. All code must pass `golangci-lint run`, `go vet ./...`, and `go test ./...` with zero failures before review.

## License

Apache-2.0. See [LICENSE](LICENSE) for details.
