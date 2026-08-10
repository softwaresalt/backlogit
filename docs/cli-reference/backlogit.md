---
chunk_strategy: h1-h2-h3
description: Backlogit — AI-native agile workspace
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit.md
title: backlogit
---

## backlogit

Backlogit — AI-native agile workspace

### Synopsis

backlogit manages a project-local work item workspace under .backlog by default.

	It stores active work in .backlog\queue, terminal work in .backlog\archive,
	per-item history in .backlog\logs\{item-id}.jsonl, and deferred planning work
	in .backlog\stash.jsonl. Existing .backlogit workspaces remain supported.

Use backlogit to initialize a workspace, create and update artifacts, query the
SQLite cache, migrate from supported backlog sources, manage the work queue, and
stash follow-up work for later planning.

### Examples

```text
  backlogit init
  backlogit add --type feature --title "Authentication hardening"
  backlogit list --status active
  backlogit get 001-F --format json
  backlogit queue view --group-by status
  backlogit stash add "Defer audit dashboard split" --kind feature
  backlogit migrate --source .\.backlog --adapter backlog-md --dry-run
  backlogit mcp
```

### Options

```text
      --cwd string         workspace directory (default ".")
  -h, --help               help for backlogit
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit add](backlogit_add.md)	 - Create a new artifact
* [backlogit adopt](backlogit_adopt.md)	 - Adopt an orphaned item under a new parent feature
* [backlogit archive](backlogit_archive.md)	 - Archive a completed artifact
* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints
* [backlogit comment](backlogit_comment.md)	 - Append comments to an artifact's history
* [backlogit delete](backlogit_delete.md)	 - Delete an artifact
* [backlogit deliberate](backlogit_deliberate.md)	 - Create a deliberation artifact linked to a stash entry
* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies
* [backlogit docs](backlogit_docs.md)	 - Lint and migrate documentation frontmatter (docline base schema)
* [backlogit doctor](backlogit_doctor.md)	 - Check workspace integrity
* [backlogit get](backlogit_get.md)	 - Retrieve an artifact by ID
* [backlogit hooks](backlogit_hooks.md)	 - Poll and acknowledge agent hook events
* [backlogit init](backlogit_init.md)	 - Initialize a new backlogit workspace
* [backlogit link](backlogit_link.md)	 - Manage directed semantic links between artifacts
* [backlogit list](backlogit_list.md)	 - List artifacts in the workspace
* [backlogit manifest](backlogit_manifest.md)	 - Print a JSON-RPC manifest of all backlogit MCP tool definitions
* [backlogit mcp](backlogit_mcp.md)	 - Start the backlogit MCP stdio server
* [backlogit memory](backlogit_memory.md)	 - Persist keyed agent session memory
* [backlogit metadata](backlogit_metadata.md)	 - Discover backlogit metadata for agents and tooling
* [backlogit migrate](backlogit_migrate.md)	 - Migrate backlog data between supported formats and layouts
* [backlogit move](backlogit_move.md)	 - Change artifact status
* [backlogit query](backlogit_query.md)	 - Execute a read-only SQL query against the index
* [backlogit queue](backlogit_queue.md)	 - Manage the work queue
* [backlogit search](backlogit_search.md)	 - Full-text search across artifacts
* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups
* [backlogit stash](backlogit_stash.md)	 - Manage the deferred work stash
* [backlogit status](backlogit_status.md)	 - Show workspace artifact summary
* [backlogit sync](backlogit_sync.md)	 - Rehydrate the SQLite index from Markdown source files
* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry
* [backlogit update](backlogit_update.md)	 - Self-update backlogit or update artifact fields
* [backlogit version](backlogit_version.md)	 - Print version, latest release, commit, build date, and Go runtime information

