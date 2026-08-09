---
chunk_strategy: h1-h2-h3
description: "F4 — durable dep_type: contract for typed dependency edges in frontmatter"
doc_type: design-doc
schema_version: "1.0"
source: docs/design-docs/dependency-type-durability.md
title: "Dependency Type Durability — F4"
---

# Dependency Type Durability — F4

Source: `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md` (Q5)
Implemented: 118-S (106.012-T through 106.018-T)

## Problem Statement

Before F4, `Artifact.Dependencies` was a bare `[]string` of target IDs. Rehydrate
rebuilt every dep edge with `dep_type = "blocks"`, silently collapsing `relates_to`
and `parent_of` edges on every `backlogit sync`. `RemoveDependency` recovered the
type from the SQLite cache — a disposable projection — making the type ephemeral.

## Accepted Frontmatter Shapes

Both shapes are permanently accepted (forward-only compatibility rule).

### Legacy bare-string list (type defaults to `blocks`)

```yaml
dependencies:
  - T001
  - T002
```

### Typed object list (explicit dep_type)

```yaml
dependencies:
  - id: T001
    type: blocks
  - id: T002
    type: relates_to
```

### Mixed list (each entry chooses its own form)

```yaml
dependencies:
  - T001
  - id: T002
    type: parent_of
```

## Fixed YAML Keys

The object shape uses exactly two lowercase keys: `id` and `type`.
No other key names are accepted at the load edge.

## Accepted Type Values

| Value | Meaning | Surface |
|---|---|---|
| `blocks` | Default; this item cannot start until the target is done | dependency |
| `relates_to` | Execution-blocking but not strictly sequential | dependency |
| `parent_of` | Hierarchy relationship; item is a logical child of target | dependency |

## Serialization Rule

`ToFrontmatterMap` emits dependencies using the compact rule:

* If **all** edges carry the default type (`blocks` or empty), serialize as a bare
  string list. This keeps existing artifacts byte-identical on round-trip.
* If **any** edge carries a non-default type (`relates_to`, `parent_of`), serialize
  **all** edges as typed objects so the type survives rehydration.

## Load-Edge Validation

`toDependencyEdges` (in `internal/models/frontmatter.go`) validates the `type` field
against the accepted set at parse time and returns `ErrInvalidDependencyType` for
unknown values. Callers do not need to re-validate.

## Dependency vs Link Disambiguation

Agents frequently confuse the two relationship surfaces:

| Surface | YAML key | Go type | Semantics |
|---|---|---|---|
| **Dependency** | `dependencies` | `[]DependencyEdge` | Execution-blocking; `dep_type` ∈ `{blocks, relates_to, parent_of}` |
| **Link** | `links` | `[]ArtifactLink` | Informational only; `link_type` ∈ `{related_to, duplicate_of, informs, supersedes, spike_ref}` |

Note: **dependency `relates_to`** (execution-blocking, causal) is NOT the same as
**link `related_to`** (informational, non-blocking). Choosing the wrong surface will
either fail to enforce execution ordering (if a blocking dep is added as a link) or
incorrectly block execution (if an informational link is added as a dep).

Rule: "Does this relationship prevent execution of the dependent until the target
is done?" → use dependency. "Is this a reference or informational relationship?" →
use link.

## Rehydration Contract

After F4, `Rehydrate` reads `dep_type` from `artifact.Dependencies` (the durable
frontmatter source) rather than hardcoding `blocks`. The SQLite `item_deps` table
remains a disposable projection that is rebuilt from frontmatter on every sync.

## CLI and MCP Parity

`backlogit dep list` and the MCP `backlogit_get_dependencies` tool both return
`db.DependencyEdge{ItemID, DependsOn, DepType}` — a single canonical shape. The
parity contract test (`tests/contract/dep_type_parity_test.go`) asserts both
surfaces return identical edge sets with identical dep_type values.
