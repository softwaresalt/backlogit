---
chunk_strategy: h1-h2-h3
description: Discover backlogit metadata for agents and tooling
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_metadata.md
title: backlogit metadata
---

## backlogit metadata

Discover backlogit metadata for agents and tooling

### Synopsis

Inspect the unified backlogit metadata catalog that agents need for
programmatic reasoning, including artifact types, field enums, template sections,
registry routing, stash conventions, CLI commands, and MCP tools.

### Options

```text
  -h, --help   help for metadata
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace
* [backlogit metadata catalog](backlogit_metadata_catalog.md)	 - Print the unified metadata catalog
* [backlogit metadata export-command-map](backlogit_metadata_export-command-map.md)	 - Write an agent-readable command map into the workspace
* [backlogit metadata templates](backlogit_metadata_templates.md)	 - List registered template types and their section definitions
* [backlogit metadata types](backlogit_metadata_types.md)	 - List all configured work item types
* [backlogit metadata wit](backlogit_metadata_wit.md)	 - Describe metadata for a single work item type

