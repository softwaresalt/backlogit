---
title: "backlogit Architecture"
description: "Top-level domain map, dependency direction, and key surface reference for the backlogit repository"
ms.date: 2026-05-22
ms.topic: reference
---

## backlogit Architecture

backlogit is a Go CLI tool and MCP server for structured backlog management. It stores work
items as Markdown files with YAML frontmatter, maintains an ephemeral SQLite index for
token-efficient agent queries, and exposes both a CLI and an MCP (Model Context Protocol)
server surface.

For broader design context and philosophy, see `docs/research/Backlogit-Architecture-Design.md`.

## Domain Map

```text
cmd/backlogit/           ← CLI entry point (cobra root command)
  ↓
internal/cli/            ← CLI commands (cobra subcommands per feature domain)
  ↓
internal/core/           ← Domain logic: metadata catalog, queue, config, types
internal/db/             ← SQLite database layer: schema, migrations, queries
internal/mcp/            ← MCP server: tool registrations, handlers
internal/models/         ← Shared domain models (work item types, statuses)
internal/telemetry/      ← Telemetry harvesting: JSONL fact tables, schema reference
  ↓
.backlogit/              ← Workspace: Markdown source-of-truth, SQLite cache
```

## Dependency Direction

```text
cmd → cli → core, db, mcp, models, telemetry
             core → models, db
             mcp  → core, db, models, telemetry
             cli  → core, db, models, telemetry
             db        → models, events, stash, config, errors, modernc.org/sqlite
             telemetry → db, errors, modernc.org/sqlite
             models    → validator/v10
             events    → errors, validator/v10
             stash     → models, errors
             config    → stash, validator/v10
             errors    → (stdlib only)
```

Cross-cutting rules:

* `db` imports domain-support packages (`models`, `events`, `stash`, `config`, `errors`)
  and `modernc.org/sqlite`; it does not depend on higher-level packages (`cli`, `mcp`, `core`).
* `models` depends only on `validator/v10`; it has no internal package dependencies.
* `core` accesses `db` through typed function calls, not embedded `*sql.DB`.
* `mcp` and `cli` are parallel entry-point layers; neither depends on the other.
* `telemetry` imports `db` and `errors`; it is not fully self-contained.

## Key Surfaces

### CLI (`internal/cli/`)

| Command | Description |
|---|---|
| `backlogit mcp` | Start MCP server (stdio transport) |
| `backlogit add` | Create a new backlog item |
| `backlogit list` | List backlog items |
| `backlogit query` | Run SQL queries against the SQLite index |
| `backlogit sync` | Synchronize SQLite index from Markdown |
| `backlogit queue view` | View prioritized work queue |
| `backlogit shipment` | Shipment lifecycle (create, claim, ship) |
| `backlogit stash` | Stash entry management |
| `backlogit telemetry schema` | Print fact table and SQLite telemetry schemas |

### MCP Tools (`internal/mcp/`)

MCP tools are registered in `internal/mcp/` and exposed via stdio transport. Key
discovery tools include:

| Tool | Description |
|---|---|
| `backlogit_get_metadata_catalog` | Unified workspace metadata including `sql_schema` |
| `backlogit_query_sql` | Read-only SQL against the SQLite index |
| `backlogit_sync_index` | Rebuild the SQLite cache from Markdown |

For the full tool inventory, call `backlogit_get_metadata_catalog` at runtime or see
`docs/cli-reference/` for generated reference docs.

### Schema Discoverability (shipped: 063-S, 2026-05-22)

Two programmatic schema discovery surfaces were added to eliminate reliance on
static instruction files for schema knowledge:

**SQL schema in metadata catalog** (`internal/db/schema.go`, `internal/core/metadata_catalog.go`):

* `db.IntrospectSchema(ctx, db)` extracts live table definitions from the SQLite database
  at runtime using `sqlite_master`, `PRAGMA table_info`, `PRAGMA index_list`, and
  `PRAGMA index_info`. Results include columns (name, type, nullable, primary key),
  indexes (name, unique, columns), and FTS5 virtual table identification.
* `core.MetadataCatalog.SQLSchema` carries the introspected schema as a `[]db.TableSchema`
  field (JSON: `sql_schema`). The MCP handler calls `db.IntrospectSchema` when building
  the catalog response.
* Primary use: agents calling `backlogit_get_metadata_catalog` receive the current SQLite
  schema without needing pre-loaded instruction files.

**Telemetry schema reference** (`internal/telemetry/schema_ref.go`):

* `telemetry.DescribeFactTables()` returns schema metadata for the three JSONL fact tables
  (`tool-calls.jsonl`, `session-facts.jsonl`, `telemetry-sessions.jsonl`).
* `telemetry.DescribeTelemetrySQLTables()` returns schema for the two SQLite telemetry
  tables (`telemetry_sessions`, `telemetry_tool_usage`).
* Schema is maintained as a manually curated registry with a drift-detection test that
  validates field names against Go struct JSON tags.
* Exposed via `backlogit telemetry schema [--format text|json|markdown]`.

### SQLite Index (`internal/db/`)

The SQLite database at `.backlogit/backlogit.db` is an ephemeral query cache rebuilt
from Markdown via `backlogit sync`. It is not committed to Git. See
`.github/instructions/backlogit-sql-schema.instructions.md` for the full table reference.
Use `backlogit_get_metadata_catalog` as the runtime source for current table definitions.

### Workspace Layout (`.backlogit/`)

| Path | Contents |
|---|---|
| `.backlogit/queue/` | Active work items (Markdown + YAML frontmatter) |
| `.backlogit/archive/` | Completed and shipped artifacts |
| `.backlogit/hooks_queue.jsonl` | Hook event queue for inter-agent signals |
| `.backlogit/backlogit.db` | SQLite index cache (gitignored) |

## Quality Grades

| Domain | Grade | Notes |
|---|---|---|
| `internal/db` | B+ | Schema introspection added (063-S); PRAGMA gate validated |
| `internal/core` | B+ | Metadata catalog now self-describing via `sql_schema` |
| `internal/mcp` | B | Tool surface well-tested; MCP contract tests present |
| `internal/cli` | B | CLI surface grows; `telemetry schema` subcommand added |
| `internal/telemetry` | B | Drift-detection test guards schema reference registry |
| `internal/models` | A- | Stable, well-typed, minimal churn |

## Further Reading

| Topic | Location |
|---|---|
| Design philosophy and CQRS rationale | `docs/research/Backlogit-Architecture-Design.md` |
| SQL schema table reference | `.github/instructions/backlogit-sql-schema.instructions.md` |
| CLI reference | `docs/cli-reference/` |
| Compound learnings | `docs/compound/` |
| Decisions and spike findings | `docs/decisions/` |
| Workflow policies | `.github/policies/workflow-policies.md` |
| Constitutional principles | `.github/instructions/constitution.instructions.md` |
