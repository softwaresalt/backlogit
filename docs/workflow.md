---
title: Workflow Guide
description: Developer and agent workflows with backlogit
author: backlogit contributors
ms.date: 2026-04-01
ms.topic: tutorial
keywords:
  - backlogit
  - workflow
  - mcp
  - cli
  - agent
---

## Workspace Lifecycle

A backlogit workspace follows a predictable lifecycle. You initialize it once, then add artifacts, query them, update their status as work progresses, and eventually archive completed items. The workspace lives entirely within the `.backlogit/` directory at the root of your project.

The lifecycle has five stages:

1. Initialize the workspace with `backlogit init`
2. Create artifacts with `backlogit add`
3. List and query artifacts with `backlogit list` and `backlogit query`
4. Update status and metadata with `backlogit update` and `backlogit move`
5. Archive completed work with `backlogit archive`

## Developer CLI Workflow

**Initialize a new workspace:**

```bash
backlogit init
```

This creates the `.backlogit/` directory with default `config.yaml`, `registry.yaml`, `hooks.yaml`, and `migration.yaml` files. The SQLite cache (`index.db`) is created on first use and is listed in `.gitignore` automatically.

**Add artifacts:**

```bash
# Create a task
backlogit add --type task --title "Add rate limiting to API" --status active

# Create a bug with a description
backlogit add --type bug --title "Login fails on Safari" --status queued

# Create a story linked to an epic
backlogit add --type story --title "User authentication flow" --status active
```

**List and filter artifacts:**

```bash
# List all active items
backlogit list --status active

# List bugs only
backlogit list --type bug

# List items with a specific label
backlogit list --label security
```

**Search by keyword:**

```bash
backlogit search "rate limiting"
```

**Run a SQL query against the index:**

```bash
backlogit query "SELECT id, title, status FROM items WHERE type='bug' ORDER BY created_at DESC LIMIT 10"
```

**Get the work queue (prioritized active items):**

```bash
backlogit queue
```

**Inspect a specific artifact:**

```bash
backlogit get T042
```

**Update fields on an artifact:**

```bash
backlogit update T042 --status in_review
backlogit update T042 --title "Add rate limiting to public API"
```

**Move an artifact to a new status:**

```bash
backlogit move T042 done
```

**Add a dependency between artifacts:**

```bash
backlogit dep add T042 --depends-on T038
```

**Archive a completed artifact:**

```bash
backlogit archive T042
```

**Force-rebuild the SQLite index from Markdown files:**

```bash
backlogit sync
```

## Agent MCP Workflow

AI agents connect to backlogit through the Model Context Protocol. The server exposes 21 tools over JSON-RPC 2.0 via stdio. Start the server with:

```bash
backlogit mcp
```

The server runs until terminated and communicates over standard input and output. Agents discover all tools automatically through the MCP `initialize` handshake.

### Connecting Claude Code

Add the following to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "backlogit": {
      "command": "backlogit",
      "args": ["mcp"]
    }
  }
}
```

### Connecting GitHub Copilot CLI

Add backlogit to `.copilot/mcp-config.json` or `.vscode/mcp.json` in your workspace:

```json
{
  "servers": {
    "backlogit": {
      "type": "stdio",
      "command": "backlogit",
      "args": ["mcp"]
    }
  }
}
```

### Connecting Cursor

Add the server entry to Cursor's MCP settings under Settings > MCP:

```json
{
  "backlogit": {
    "command": "backlogit",
    "args": ["mcp"]
  }
}
```

### Core Agent Operations

Once connected, agents call tools by name. Common patterns include:

```
backlogit_create_item  -- create a task, bug, story, or epic
backlogit_list_items   -- list with optional status/type filters
backlogit_query_sql    -- run a read-only SELECT against index.db
backlogit_update_item  -- change status, title, or other fields
backlogit_move_item    -- transition an artifact to a new status
backlogit_search_items -- full-text search across all artifacts
backlogit_get_queue    -- retrieve the prioritized work queue
backlogit_save_memory  -- persist agent memory to memories.json
backlogit_create_checkpoint -- save a session state snapshot
backlogit_track_commit -- associate a git commit with an artifact
```

The `backlogit_query_sql` tool only accepts `SELECT` statements. Write operations go through the dedicated mutation tools to preserve data integrity.

## CQRS in Practice

backlogit separates writes from reads at the storage level. Writes always update a Markdown file first, then update the SQLite cache. Reads always go to SQLite. If the cache is missing or stale, `backlogit sync` rebuilds it from the Markdown files in seconds.

This means you can safely delete `index.db` at any time. Running `backlogit sync` or any read command will rebuild it. You can also edit Markdown files directly in your editor; the next sync or read operation will pick up the changes.

## Configuration Overview

Four YAML files control workspace behavior:

`config.yaml` defines artifact types, naming templates, ID prefixes, and custom fields. The default types are `task`, `story`, `bug`, and `epic`.

`registry.yaml` maps artifact types and statuses to directory paths within `.backlogit/`. You can route `done` bugs to a different directory than `active` bugs, for example.

`hooks.yaml` configures external integration triggers, such as syncing state changes to Jira or Azure DevOps.

`migration.yaml` defines source-path classification and default artifact-type mappings for imports from external markdown-backed systems such as Backlog.md.

## Git Integration

The `.backlogit/` directory is committed to your repository. Markdown artifact files are Git-friendly: they have stable field ordering in their YAML frontmatter, deterministic slug generation for filenames, and no binary content. The only gitignored file is `index.db`.

When multiple developers or agents make concurrent changes, Markdown files merge cleanly because each artifact is a separate file. The event stream (`events.jsonl`) is append-only and accumulates entries without conflict.

Associate a commit with an artifact using the MCP tool or the CLI:

```bash
backlogit update T042 --commit abc1234
```

The `backlogit_track_commit` MCP tool records commit SHAs against artifact IDs for traceability.
