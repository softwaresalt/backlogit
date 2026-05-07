---
title: "backlogit"
description: "Backlogit — AI-native agile workspace"
---

## backlogit

Backlogit — AI-native agile workspace

### Synopsis

backlogit manages a project-local work item workspace under .backlogit.

	It stores active work in .backlogit\queue, terminal work in .backlogit\archive,
	per-item history in .backlogit\logs\{item-id}.jsonl, and deferred planning work
	in .backlogit\stash.jsonl.

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
```

### SEE ALSO

* [backlogit add](backlogit_add.md)	 - Create a new artifact
* [backlogit adopt](backlogit_adopt.md)	 - Adopt an orphaned item under a new parent feature
* [backlogit archive](backlogit_archive.md)	 - Archive a completed artifact
* [backlogit checkpoint](backlogit_checkpoint.md)	 - Manage session state checkpoints
* [backlogit delete](backlogit_delete.md)	 - Delete an artifact
* [backlogit deliberate](backlogit_deliberate.md)	 - Create a deliberation artifact linked to a stash entry
* [backlogit dep](backlogit_dep.md)	 - Manage artifact dependencies
* [backlogit doctor](backlogit_doctor.md)	 - Check workspace integrity
* [backlogit get](backlogit_get.md)	 - Retrieve an artifact by ID
* [backlogit init](backlogit_init.md)	 - Initialize a new backlogit workspace
* [backlogit list](backlogit_list.md)	 - List artifacts in the workspace
* [backlogit manifest](backlogit_manifest.md)	 - Print a JSON-RPC manifest of all backlogit MCP tool definitions
* [backlogit mcp](backlogit_mcp.md)	 - Start the backlogit MCP stdio server
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
* [backlogit update](backlogit_update.md)	 - Update artifact fields or sections
* [backlogit version](backlogit_version.md)	 - Print version, commit, build date, and Go runtime information

