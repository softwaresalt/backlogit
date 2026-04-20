---
title: "backlogit telemetry harvest"
description: "Parse Copilot CLI logs and write telemetry-sessions.jsonl"
---

## backlogit telemetry harvest

Parse Copilot CLI logs and write telemetry-sessions.jsonl

```text
backlogit telemetry harvest [flags]
```

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

* [backlogit telemetry](backlogit_telemetry.md)	 - Inspect Copilot CLI token usage and tool telemetry

