---
chunk_strategy: h1-h2-h3
description: Repair stale gate evidence for a shipment member
doc_type: reference
ingested_at: "2026-08-24T07:49:32Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_shipment_repair-evidence.md
title: backlogit shipment repair-evidence
---

## backlogit shipment repair-evidence

Repair stale gate evidence for a shipment member

### Synopsis

Append an audited forced gate pass event for a single shipment member whose
recorded gate evidence head_sha is stale (dangling or divergent from the
current workspace HEAD) but whose implementation is verified present in the
merged scope.

This is an operator-only break-glass for the narrow scenario where a PR
review-fix cycle orphaned the original task completion commit while the diff
content survived into the merged branch. After the repair, backlogit shipment
ship will accept the member in its ancestry check.

The --reason flag is required and is recorded verbatim in the audit log.

```text
backlogit shipment repair-evidence <shipment-id> [flags]
```

### Examples

```text
  backlogit shipment repair-evidence 129-S --member 146.006-T --reason "orphaned by PR review-fix rebase; diff verified at current main HEAD"
```

### Options

```text
  -h, --help            help for repair-evidence
      --member string   member item ID to repair
      --reason string   operator justification for the evidence repair (required)
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit shipment](backlogit_shipment.md)	 - Manage shipment work groups

