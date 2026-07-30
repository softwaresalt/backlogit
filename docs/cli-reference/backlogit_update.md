---
chunk_strategy: h1-h2-h3
description: Self-update backlogit or update artifact fields
doc_type: reference
ingested_at: "2026-06-26T02:27:58Z"
schema_version: "1.0"
source: docs/cli-reference/backlogit_update.md
title: backlogit update
---

## backlogit update

Self-update backlogit or update artifact fields

### Synopsis

Update the installed backlogit binary when called without an artifact ID,
or update frontmatter fields and template-backed body sections on an existing
artifact when an ID is supplied.

Use repeated --section name=value flags to update named sections without
replacing the rest of the document body.

Complexity is task-only planning metadata:
size = implementation volume; complexity = implementation difficulty and uncertainty;
priority = importance and scheduling urgency. Default queue ordering does not change
when complexity is set.

```text
backlogit update [id] [flags]
```

### Examples

```text
  backlogit update
  backlogit update --check
  backlogit update --to v1.2.3
  backlogit update 001.001-T --status review
  backlogit update 001.001-T --priority high
  backlogit update 001-F --section goals="Ship passwordless sign-in"
  backlogit update 001-F --harness-status passing
```

### Options

```text
      --assigned-to string            assignee
      --check                         check whether a backlogit binary update is available without applying it
      --commit string                 commit SHA
      --complexity string             implementation difficulty/uncertainty (trivial, low, medium, high); body-preserving, mutually exclusive with other field flags
      --description string            new description
      --force-gates                   operator-only: force completion past the gate (requires --force-reason)
      --force-reason string           justification recorded in the forced-gate audit event
      --gate-base string              operator-only base ref override for the completion gate (audited)
      --harness-status string         harness status (pending, scaffolded, passing, failing)
  -h, --help                          help for update
      --id string                     artifact ID (immutable, always rejected)
      --json                          emit the machine-readable gate outcome contract on a gated completion
      --labels string                 comma-separated labels
      --owner string                  owner
      --priority string               new priority
      --section stringArray           section update as name=value (repeatable)
      --size string                   T-shirt size (XS, S, M, L, XL); body-preserving, mutually exclusive with other field flags
      --size-ruleset-version string   size ruleset version
      --size-source string            size provenance source (human, agent, derived)
      --sprint string                 sprint ID
      --status string                 new status
      --title string                  new title
      --to string                     update the backlogit binary to a specific release tag
```

### Options inherited from parent commands

```text
      --cwd string         workspace directory (default ".")
      --jsonrpc            wrap all output in a JSON-RPC 2.0 response envelope
      --log-level string   log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)
      --no-update-check    skip the remote latest-release check
```

### SEE ALSO

* [backlogit](backlogit.md)	 - Backlogit — AI-native agile workspace

