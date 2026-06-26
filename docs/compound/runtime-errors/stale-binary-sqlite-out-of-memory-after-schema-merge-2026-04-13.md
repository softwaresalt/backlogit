---
chunk_strategy: h1-h2-h3
description: 'After a merge that adds new SQLite columns, an older compiled backlogit binary reports ''unable to open database file: out of memory (14)'' - this is a binary version mismatch, not a real database error'
doc_type: learning
docline:
    ms.date: 2026-04-13T00:00:00Z
    tags:
        - sqlite
        - runtime
        - debugging
        - ship
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/runtime-errors/stale-binary-sqlite-out-of-memory-after-schema-merge-2026-04-13.md
title: Stale backlogit binary causes SQLite 'out of memory (14)' after schema changes
---

## Problem

After merging a branch that introduces new SQLite schema migrations (e.g., `ALTER TABLE ADD COLUMN` in `internal/db/telemetry_schema.go`), subsequent `backlogit` CLI commands fail with:

```text
Error: open workspace: open database: ping database: unable to open database file: out of memory (14)
```

Even after:
- Deleting `backlogit.db` and its WAL/SHM files
- Running `backlogit sync`
- Retrying multiple times

The error persists because the installed `backlogit.exe` binary is stale — compiled before the schema migration code was added. The older binary's SQLite layer cannot handle the new database state.

## Root Cause

SQLite error code 14 (`SQLITE_CANTOPEN`) is reported as "out of memory" by `modernc.org/sqlite` in some failure modes. When the binary predates the schema migration, it may fail to open the database due to a version mismatch in the compiled-in schema expectations.

The stale binary was the `backlogit.exe` sitting in the repository root — a local build artifact that was not updated when the branch was merged.

## Solution

Build and use a fresh binary compiled from the current source:

```powershell
Set-Location <repo-root>
go build -o backlogit-fresh.exe ./cmd/backlogit
.\backlogit-fresh.exe sync
.\backlogit-fresh.exe shipment ship 031-S --sha <sha>

# Then install permanently
go install ./cmd/backlogit
```

Or simply re-run `go install ./cmd/backlogit` from the repo root to overwrite the installed binary with the latest code.

## Prevention

After merging any PR that touches `internal/db/`, run `go install ./cmd/backlogit` before invoking the CLI. The `backlogit.exe` in the repo root is a build artifact that can drift from the installed `backlogit` in `$GOPATH/bin` — always use the installed version or rebuild explicitly after schema-changing merges.

## Context

- Discovered during 031-S post-merge closure (2026-04-13)
- Affected commands: `backlogit sync`, `backlogit shipment ship`
- Fixed by: `go build -o backlogit-test.exe ./cmd/backlogit && .\backlogit-test.exe sync`
