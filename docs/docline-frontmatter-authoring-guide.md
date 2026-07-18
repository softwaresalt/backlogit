---
chunk_strategy: h1-h2-h3
description: How to author documentation frontmatter on the docline base schema — required and optional fields, the doc_type taxonomy, the docline namespace, ownership tiers, and the backlogit docs workflow.
doc_type: guide
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/docline-frontmatter-authoring-guide.md
title: Docline Frontmatter Authoring Guide
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

> **Required in repository source.** Although `chunk_strategy` and
> `schema_version` are schema-optional (the validator defaults them when
> absent), every Git-tracked, in-scope Markdown document MUST declare them
> explicitly with their canonical values — `chunk_strategy: h1-h2-h3` and
> `schema_version: "1.0"` (a YAML string). The live corpus guard
> `TestDoclineSoftKeys_LiveTrackedCorpus`
> (`tests/integration/docline_soft_keys_test.go`) fails CI — including on
> docs-only pull requests, via the Docline frontmatter gate — when a tracked
> in-scope document omits or misdeclares either key.

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

## Owner-scoped extension profiles

Tools that consume this repository's documents sometimes need their own metadata
on a document without touching the shared contract. The **owner profile**
convention gives each tool a reserved sub-namespace inside the `docline` map.

An owner's extension properties live nested under the `docline` map at
`docline.<owner>.*`, never at the top level. backlogit is the first owner, at
`docline.backlogit.*`:

```yaml
---
title: Example Decision
source: docs/decisions/example.md
doc_type: decision
docline:
  date: 2026-07-16T00:00:00Z
  backlogit:
    schema_version: "1.0"
    size: 4212
---
```

The convention has three rules:

* **Owner key.** Each owner reserves one key under `docline` (here, `backlogit`).
  All of that owner's metadata lives under it.
* **Per-owner `schema_version`.** Each owner profile versions itself,
  independently of the docline base `schema_version`.
* **Additive, not enveloping.** An owner profile only adds keys to the `docline`
  map. It never wraps or relocates the docline base fields. The base contract
  fields stay at the top level and the `docline` map stays in place.

Ownership splits cleanly:

* **docline** owns and validates only the base fields, and preserves the
  `docline` map in place through its read/write cycle.
* **The owner** (backlogit) owns and validates its own subtree,
  `docline.backlogit.*`, through a derived extension schema
  (`schemas/docline/ext/backlogit-v1.schema.json`) composed `allOf` on top of
  base v1.

The top level stays `additionalProperties: false`. Do not add a top-level
`backlogit` or `x-backlogit` key, and do not wrap the base fields inside an owner
container — both break the shared contract that ingestion and downstream
consumers read.

> [!NOTE]
> This convention applies to docline **documents**. Persisting owner profiles on
> `.backlogit` feature, task, or shipment artifact frontmatter is a separate,
> still-open question: the generic artifact codec carries only `custom_fields`,
> so a top-level `docline` map is dropped on those artifacts. See the
> [Model A decision doc](decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md).

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
* Owner-profile decision (Model A): `docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md`
* backlogit owner-profile ext schema: `schemas/docline/ext/backlogit-v1.schema.json`
