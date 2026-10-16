---
chunk_strategy: h1-h2-h3
description: Run the soundness linter over a plan file (report-only by default)
doc_type: reference
ingested_at: "2026-10-16T21:37:27Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_soundness_lint.md
title: backlogit soundness lint
---

## backlogit soundness lint

Run the soundness linter over a plan file (report-only by default)

### Synopsis

Run the soundness engine over a JSON plan of units (plans, features, tasks,
subtasks) as a standalone check. Wiring this check into shipment assembly is a
follow-up.

The default mode is report-only: findings are printed as warnings and the exit
status stays zero. In enforce mode any error finding is printed and the command
exits non-zero; --mode may tighten a report-only config but never relax an
enforce config. An empty plan is an error in enforce mode. A misconfiguration (unknown mode, unknown rule, invalid budget)
is always an error, never a pass. Plan and config paths are confined to the
workspace root.

```text
backlogit soundness lint [flags]
```

### Examples

```text
  backlogit soundness lint --plan plan.json
  backlogit soundness lint --plan plan.json --mode enforce --config soundness.conf
  backlogit soundness lint --plan plan.json --disable-rule budget --format json
```

### Options

```text
      --config string          workspace-relative text config (budgets, mode, rule flags)
      --disable-rule strings   rule IDs to disable
      --format string          output format: text, json (default: text on TTY, json otherwise)
  -h, --help                   help for lint
      --mode string            gate mode: report-only (default), enforce
      --plan string            workspace-relative JSON plan file (required)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit soundness](backlogit_soundness.md)	 - Lint plans and work items for decomposition soundness

