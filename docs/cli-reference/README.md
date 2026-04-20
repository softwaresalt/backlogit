---
title: backlogit CLI Reference
description: Auto-generated Markdown reference for all backlogit CLI commands
ms.date: 2026-04-19
ms.topic: reference
---

## backlogit CLI Reference

This directory contains auto-generated Markdown pages for every `backlogit` command and subcommand, produced by `cmd/gen-docs` using cobra's `GenMarkdownTree`.

## Command Groups

| Group | Entry Page |
|---|---|
| Root | [backlogit.md](backlogit.md) |
| `dep` | [backlogit_dep.md](backlogit_dep.md) |
| `metadata` | [backlogit_metadata.md](backlogit_metadata.md) |
| `queue` | [backlogit_queue.md](backlogit_queue.md) |
| `shipment` | [backlogit_shipment.md](backlogit_shipment.md) |
| `stash` | [backlogit_stash.md](backlogit_stash.md) |
| `telemetry` | [backlogit_telemetry.md](backlogit_telemetry.md) |

## Regenerating the Reference

Run `make docs` from the repository root to regenerate all pages from the current command tree:

```bash
make docs
```

The CI pipeline checks for drift automatically on every pull request. If a flag, description, or subcommand changes and `make docs` is not re-run, the pull request will fail with a diff report.
