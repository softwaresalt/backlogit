---
title: "backlogit telemetry harvest"
description: "Parse Copilot CLI logs and write telemetry-sessions.jsonl"
---

## backlogit telemetry harvest

Parse Copilot CLI logs and write telemetry-sessions.jsonl

```text
backlogit telemetry harvest [flags]
```

### Behavior

Each harvest run performs two writes:

1. **Primary output**: appends new `session_summary` and `tool_usage` JSONL records to
   `.backlogit/telemetry-sessions.jsonl`. Incremental by default — only sessions
   seen since the last checkpoint are appended. Use `--force` to rewrite from scratch.

2. **SQLite rehydration** *(side effect)*: after writing the JSONL, harvest calls
   `EnsureTelemetrySchema` and `RehydrateTelemetry` to rebuild the `telemetry_sessions`
   and `telemetry_tool_usage` tables in `.backlogit/backlogit.db`. The tables are
   cleared and repopulated from the full JSONL on every run. This means the SQLite
   tables are always consistent with the JSONL file after a successful harvest.

> The SQLite tables are ephemeral cache. They can be deleted and will be recreated
> on the next `telemetry harvest` or `backlogit sync`.

Use `backlogit telemetry report` or `backlogit telemetry top` to query the harvested
data after running harvest.

### Checkpoint

A harvest checkpoint is saved to `.backlogit/telemetry-checkpoint.json` after each
successful run. The checkpoint records file offsets for each parsed log file so
subsequent runs read only new log entries. Delete the checkpoint or use `--force` to
reparse all logs from the beginning.

### Options

```text
      --force          Re-process all logs from the beginning, ignoring the saved checkpoint
  -h, --help           help for harvest
      --since string   Exclude events before this RFC3339 timestamp (e.g. 2026-04-01T00:00:00Z)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
```

### SEE ALSO

* [backlogit telemetry](backlogit_telemetry.md) - Inspect Copilot CLI token usage and tool telemetry

