---
title: Docline Frontmatter Authoring Guide
source: docs/docline-frontmatter-authoring-guide.md
doc_type: guide
description: How to author documentation frontmatter on the docline base schema — required and optional fields, the doc_type taxonomy, the docline namespace, ownership tiers, and the backlogit docs workflow.
---

## Docline Frontmatter Authoring Guide

This guide explains how to author YAML frontmatter for documentation in this
repository so it conforms to the **docline base frontmatter v1** contract
(`schemas/docline/base-frontmatter-v1.schema.json`). The contract makes the
durable knowledge surface ingestible by the `graphtor-docs` pipeline while
keeping day-to-day authoring lightweight.

> **Tooling**: the `backlogit docs` command (and the equivalent MCP tools)
> lint, plan, and apply this contract. See [Workflow](#workflow) below.

## Field reference

The contract surface is the **top level** of the frontmatter. It holds only the
fields below; everything else belongs under the [`docline` namespace](#the-docline-namespace).

### Required fields (authoring profile)

| Field | Type | Meaning |
|---|---|---|
| `title` | string | Human-readable document title. **Authored.** |
| `source` | string | Repo-relative POSIX path to this file (e.g. `docs/decisions/x.md`). Derived by `backlogit docs migrate`. |
| `doc_type` | string | A member of the [closed taxonomy](#the-doc_type-taxonomy). Derived from the path. |

### Optional / defaulted fields

| Field | Type | Default | Meaning |
|---|---|---|---|
| `description` | string | `""` | One-line summary. **Authored.** |
| `ingested_at` | string (RFC3339) | seeded at migration | When the doc entered the knowledge surface. See [ownership](#ownership-tiers). |
| `content_sha256` | string (hex) | `""` | Content hash. **Pipeline-owned** — never hand-set. |
| `source_path` | string | `""` | Pipeline ingestion path. **Pipeline-owned.** |
| `chunk_strategy` | string | `h1-h2-h3` | Chunking hint for ingestion. |
| `schema_version` | string | `1.0` | Contract version. |
| `docline` | map | omitted when empty | Namespace for all non-contract metadata. |

## The `doc_type` taxonomy

`doc_type` is a **closed vocabulary** of 11 values. It is **derived from the
file path** (longest-prefix match), not authored by hand — the migrator sets it
for you. The full mapping lives in the
[taxonomy decision doc](decisions/2026-06-22-docline-taxonomy-and-field-mapping.md).

| Path | `doc_type` |
|---|---|
| `docs/cli-reference/**` | `reference` |
| `docs/decisions/**` | `decision` |
| `docs/exec-plans/**` | `plan` |
| `docs/closure/**` | `closure` |
| `docs/research/**` | `research` |
| `docs/reviews/**` | `review` |
| `docs/compound/**` | `learning` |
| `docs/design-docs/**` | `design` |
| `docs/product-specs/**` | `spec` |
| `docs/spikes/**` | `spike` |
| `docs/ARCHITECTURE.md` | `reference` |
| `README.md`, `AGENTS.md` | `guide` |
| `docs/*.md` (direct child) | `guide` |

Inspect the derived type for any path with `backlogit docs classify <path>`.

## The `docline` namespace

Documentation in this repo historically carried heterogeneous frontmatter keys
(`tags`, `severity`, `date`, `ms.date`, `ms.topic`, `gate_decision`, `status`,
`type`, and others). The contract keeps the top level clean by **moving every
non-contract key under the `docline` namespace** — a nested map. The migration
is **move, never drop**: no authored metadata is lost.

```yaml
---
title: Example Review
source: docs/reviews/example.md
doc_type: review
docline:
  gate_decision: approved   # was a top-level key; relocated here
  severity: low
  ms.date: 2026-06-01
---
```

When you add bespoke metadata, place it under `docline.<key>` yourself, or just
add it at the top level and let `backlogit docs migrate` relocate it.

## Ownership tiers

Three tiers resolve the tension between the schema's required fields and what a
human actually maintains:

| Tier | Fields | Who sets them |
|---|---|---|
| **Authored** | `title`, `description`, `doc_type` intent, `docline.*` | You, the document author |
| **Repo-derived** | `source`, path-derived `doc_type`, seeded `ingested_at` | `backlogit docs migrate`, deterministically |
| **Pipeline-enriched** | `content_sha256`, `source_path`, authoritative `ingested_at` | The external `graphtor-docs` ingestion pipeline |

You only ever hand-write the **authored** fields. The migrator fills the
repo-derived fields; the ingestion pipeline owns the enriched fields and the
repo never fabricates them.

### Validation profiles

* **authoring** (default, and the CI gate): requires `title`, `source`,
  `doc_type` with a valid taxonomy value. Does **not** require pipeline-owned
  fields, because the repo does not own them.
* **ingestion**: requires the full schema set (`title`, `source`,
  `ingested_at`, `doc_type`) — used to smoke-check that a migrated file
  satisfies the external contract.

## Workflow

| Command | MCP tool | What it does |
|---|---|---|
| `backlogit docs lint [--profile authoring\|ingestion] [--path P]` | `backlogit_docs_lint` | Reports frontmatter violations; non-zero exit on findings (CI gate). |
| `backlogit docs migrate [--path P]` | `backlogit_docs_migrate` | **Dry-run by default**: prints the planned changes without writing. |
| `backlogit docs migrate --apply --yes --path P` | `backlogit_docs_migrate apply=true` | Writes changes atomically. Requires an explicit scoped path; refuses a whole-tree apply. |
| `backlogit docs scope` | `backlogit_docs_scope` | Prints the active scope globs, taxonomy, and profiles. |
| `backlogit docs classify <path>` | — | Prints the derived `doc_type` for a path. |

The migration is **idempotent** (running it twice yields no further changes) and
**body-preserving** (only the frontmatter block is rewritten; the markdown body
bytes are never touched).

> **Apply is guarded.** The CLI requires `--apply --yes` *and* an explicit
> `--path`. The MCP `backlogit_docs_migrate` tool refuses `apply=true` unless the
> `BACKLOGIT_DOCS_ALLOW_APPLY` environment flag is set, and likewise requires a
> scoped path.

## Scope

In scope: `docs/**` (except `docs/memory/**` and `docs/archive/**`), plus the
root knowledge files `README.md` and `AGENTS.md`. Out of scope: `.github/**` and
`prompt.md`. Run `backlogit docs scope` for the authoritative list.

## See also

* Taxonomy and field-mapping decision: `docs/decisions/2026-06-22-docline-taxonomy-and-field-mapping.md`
* Standardization plan: `docs/exec-plans/2026-06-22-docline-frontmatter-standardization-plan.md`
* Contract schema: `schemas/docline/base-frontmatter-v1.schema.json`
