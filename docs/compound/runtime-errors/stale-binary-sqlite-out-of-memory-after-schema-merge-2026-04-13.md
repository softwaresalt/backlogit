---
chunk_strategy: h1-h2-h3
description: 'After a merge, an older compiled backlogit binary can report misleading runtime errors or hide newly shipped behavior until the CLI is rebuilt or reinstalled.'
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
title: Stale backlogit binary can mimic database errors or hide merged behavior
---

## Problem

A merged feature is not active in whatever `backlogit.exe` happens to be on PATH. After source changes merge, an older compiled binary can either emit misleading errors or keep using the previous behavior.

The original 2026-04-13 symptom appeared after merging SQLite schema migrations (e.g., `ALTER TABLE ADD COLUMN` in `internal/db/telemetry_schema.go`), when subsequent `backlogit` CLI commands failed with:

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

A second 2026-07-10 instance surfaced during 088-S git-aware archival: `internal/core/archive.go` on current `main` used `git mv` for tracked backlog artifacts, but the installed `C:\Tools\backlogit.exe` was still v1.4.1. The shipped behavior did not appear until a fresh v1.4.2 binary (`.\backlogit-dev.exe`) was built from current `main` and used for CLI operations.

## Solution

First verify the binary you are about to use:

```powershell
.\backlogit-dev.exe --version
C:\Tools\backlogit.exe --version
```

Prefer a freshly built binary when validating or relying on behavior that just merged:

```powershell
Set-Location <repo-root>
go build -o backlogit-fresh.exe ./cmd/backlogit
.\backlogit-fresh.exe sync
.\backlogit-fresh.exe shipment ship 031-S --sha <sha>

# Then install permanently
go install ./cmd/backlogit
```

Or re-run `go install ./cmd/backlogit` from the repo root to overwrite the installed binary with the latest code.

## Prevention

After merging any PR that touches CLI-dispatched behavior, `internal/core/`, `internal/db/`, archive/restore logic, migrations, or workflow-critical commands, rebuild or reinstall before invoking the CLI. The repository build artifact, a PATH install, and an npm-wrapper cached binary can all drift independently. Always check `backlogit --version` or build a named fresh binary such as `.\backlogit-dev.exe` when validating current `main`.

## Context

- Discovered during 031-S post-merge closure (2026-04-13)
- Affected commands: `backlogit sync`, `backlogit shipment ship`
- Fixed by: `go build -o backlogit-test.exe ./cmd/backlogit && .\backlogit-test.exe sync`
- Reinforced during 088-S git-aware archival (2026-07-10): v1.4.1 `C:\Tools\backlogit.exe` did not contain the shipped `git mv` archival behavior; v1.4.2 `.\backlogit-dev.exe` did.
- Evidence: `docs/closure/2026-07-09-088-S-git-aware-archival-closure.md`, `internal/core/archive.go`, `internal/core/archive_git_test.go`.
