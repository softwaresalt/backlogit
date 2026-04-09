---
title: Workflow Guide
description: Developer and agent workflows with backlogit
author: backlogit contributors
ms.date: 2026-04-06
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

The lifecycle has six stages:

1. Initialize the workspace with `backlogit init`
2. Create artifacts with `backlogit add`
3. Stash deferred work in `.backlogit/stash.jsonl`
4. List and query artifacts with `backlogit list` and `backlogit query`
5. Update status and metadata with `backlogit update` and `backlogit move`
6. Archive completed work with `backlogit archive`

## Primary repository workflow

This repository now uses a two-agent delivery path with progressive disclosure.
`Stage` owns the intake path from stash to reviewed backlog, and `Ship`
owns shipment execution from ready backlog to shipped pull request state. The
durable lifecycle is `STASH -> BACKLOG -> SHIPMENT -> SHIPPED`.

Use the agent files for operational detail:

* [Stage](../.github/agents/stage.agent.md): stash triage, deliberation,
  planning, review gating, and harvest
* [Ship](../.github/agents/ship.agent.md): shipment claim, harness,
  implementation, review, CI remediation, release cleanup, and pull request flow

Shipment is a first-class artifact. Use shipment-aware MCP tools to create,
inspect, and maintain branch scope:

Shipment IDs use the `S` prefix and normally move through `queued`, `active`,
and `shipped`, with `abandoned` available when execution stops permanently.

```text
backlogit_create_shipment
backlogit_get_shipment
backlogit_list_shipments
backlogit_claim_shipment
backlogit_ship_shipment
backlogit_return_blocked
backlogit_add_to_shipment
```

## Legacy orchestration path

The older multi-agent path remains available for migration and targeted
automation, but it is no longer the primary model for this repository.

```text
deliberate or spike
-> impl-plan
-> plan-review
-> backlog-harvester
-> harness-architect
-> build-orchestrator
-> review or pr-review
```

## Developer CLI Workflow

**Initialize a new workspace:**

```bash
backlogit init
```

This creates the `.backlogit/` directory with default `config.yaml`, `header-def.yaml`, `registry.yaml`, `migration.yaml`, and template files. It also creates `.backlogit/stash.jsonl` so deferred work can be captured before it is ready to become a formal work item. The SQLite cache (`backlogit.db`) is created on first use.

**Add artifacts:**

```bash
# Create a task
backlogit add --type task --title "Add rate limiting to API" --status active

# Create a feature
backlogit add --type feature --title "User authentication flow" --status active

# Create a subtask
backlogit add --type subtask --title "Write token validation tests" --status queued
```

**List and filter artifacts:**

```bash
# List all active items
backlogit list --status active

# List features only
backlogit list --type feature

# List items with a specific label
backlogit list --label security
```

**Search by keyword:**

```bash
backlogit search "rate limiting"
```

**Run a SQL query against the index:**

```bash
backlogit query "SELECT id, title, status FROM items WHERE artifact_type='task' ORDER BY created_at DESC LIMIT 10"
```

**Get the work queue (prioritized active items):**

```bash
backlogit queue
```

**Capture deferred work in the stash:**

```bash
# Stash an idea during planning or review
backlogit stash add "Split audit dashboard into a later feature set" --kind feature --priority high
backlogit deliberate ABCD1234 --options "- Keep the current feature set narrow\n- Pull the work into the next feature wave"

# Fetch active stash entries for grouping and planning
backlogit stash fetch-stash --group-by-priority
backlogit stash fetch-stash --priority critical

# Harvest a stash entry into a real work item, carrying any linked deliberation lineage
backlogit stash harvest ABCD1234 --type feature --description "Pulled into the current feature wave"

# Harvest every critical stash item into planned work
backlogit stash harvest --priority critical --type task --description "Pulled forward from stash"
```

**Inspect a specific artifact:**

```bash
backlogit get T042
```

**Update fields on an artifact:**

```bash
backlogit update T042 --status review
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

AI agents connect to backlogit through the Model Context Protocol. The server exposes artifact, queue, stash, and planning tools over JSON-RPC 2.0 via stdio. Start the server with:

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
backlogit_create_item  -- create a feature, task, or subtask
backlogit_list_items   -- list with optional status/type filters
backlogit_query_sql    -- run a read-only SELECT against backlogit.db
backlogit_update_item  -- change status, title, or other fields
backlogit_move_item    -- transition an artifact to a new status
backlogit_search_items -- full-text search across all artifacts
backlogit_get_queue    -- retrieve the prioritized work queue
backlogit_create_shipment -- create a shipment artifact
backlogit_get_shipment -- inspect a shipment by ID
backlogit_list_shipments -- list shipment artifacts
backlogit_claim_shipment -- move a queued shipment to active
backlogit_ship_shipment -- close a released shipment, archive released scope, and record merge commit traceability
backlogit_return_blocked -- return a blocked item from a shipment to backlog
backlogit_add_to_shipment -- attach backlog items to a shipment
backlogit_fetch_stash  -- retrieve active stash entries from stash.jsonl, optionally filtered or grouped by priority, with linked deliberations when present
backlogit_stash        -- add deferred work to the stash with kind and priority
backlogit_deliberate   -- create a deliberation artifact linked to a stash entry
backlogit_harvest_stash -- promote one stash entry or a whole priority band into planned work items
backlogit_save_memory  -- persist agent memory to memories.json
backlogit_create_checkpoint -- save a session state snapshot
backlogit_track_commit -- associate a git commit with an artifact
```

The `backlogit_query_sql` tool only accepts `SELECT` statements. Write operations go through the dedicated mutation tools to preserve data integrity.

## CQRS in Practice

backlogit separates writes from reads at the storage level. Writes always update a Markdown file first, then update the SQLite cache. Reads always go to SQLite. If the cache is missing or stale, `backlogit sync` rebuilds it from the Markdown files in seconds.

This means you can safely delete `backlogit.db` at any time. Running `backlogit sync` or any read command will rebuild it. You can also edit Markdown files directly in your editor; the next sync or read operation will pick up the changes.

## Configuration Overview

Four YAML files currently control workspace behavior:

`config.yaml` defines artifact types, ID patterns, shared field metadata, and queue hierarchy. The default types are `feature`, `task`, and `subtask`.

`header-def.yaml` defines per-type field schemas, enum values, defaults, and immutable system-managed fields.

`registry.yaml` maps statuses to directory paths within `.backlogit/`. By default, active work stays in `.backlogit/queue` and terminal work moves to `.backlogit/archive`.

`migration.yaml` defines source-path classification and default artifact-type mappings for imports from external markdown-backed systems such as Backlog.md.

Templates in `.backlogit/templates/` define the section structure for each artifact type.

For a complete setup guide, examples, and current limitations, see [Configuration Reference](configuration.md).

## Git Integration

The `.backlogit/` directory is committed to your repository. Markdown artifact files are Git-friendly: they have stable field ordering in their YAML frontmatter, deterministic ID-based filenames, and no binary content. The only gitignored file is `backlogit.db`.

When multiple developers or agents make concurrent changes, Markdown files merge cleanly because each artifact is a separate file. Work-item history is appended to `.backlogit/logs/{item-id}.jsonl`, and the stash remains a single hidden planning surface in `.backlogit/stash.jsonl`.

Associate a commit with an artifact using the MCP tool or the CLI:

```bash
backlogit update T042 --commit abc1234
```

The `backlogit_track_commit` MCP tool records commit SHAs against artifact IDs for traceability.
