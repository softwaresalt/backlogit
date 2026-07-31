---
chunk_strategy: h1-h2-h3
description: 'Task-only typed metadata (complexity) must enforce artifactType == "task" in the core setter/validator BEFORE schema resolution, because the DB upsert projection only drops the field for non-task artifacts and the projection guard alone cannot protect the source-of-truth frontmatter; without the setter guard a customized header-def that adds the field to another type would silently persist frontmatter. The structurally similar size seam currently lacks both guards (latent gap).'
doc_type: learning
docline:
    date: 2026-07-30T00:00:00Z
    severity: medium
    tags:
        - core
        - db
        - projection
        - typed-metadata
        - size
        - complexity
        - validation
        - task-only
        - header-def
ingested_at: "2026-07-30T00:00:00Z"
schema_version: "1.0"
source: docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md
title: 'typed metadata seam: enforce task-only in the core setter before schema resolution, not just in the DB projection'
---

# Task-only typed metadata: guard `artifactType == "task"` in the setter, not only the projection

## Problem

Backlogit's task `complexity` metadata is a *task-only* concept. The SQLite
projection refuses to persist it for non-task artifacts:

```go
// internal/db/upsert_projection.go
if name == "complexity" && artifact.ArtifactType != "task" {
    continue
}
```

It is tempting to treat that projection guard as sufficient. It is not. The
projection only controls what lands in the `items` table columns. The
authoritative source of truth is the artifact's frontmatter file. If the core
setter/validator does not itself reject non-task types, an operator who
customizes `header-def.yaml` to add a `complexity` field to another WIT type
will get the value written to that artifact's frontmatter — even though the DB
projection drops it. The result is a source-of-truth vs projection divergence
that no query can see.

## Fix

Enforce the task-only invariant in the core mutation seam **before** resolving
the type schema or validating the value:

```go
// internal/core/artifact_complexity.go
func validateComplexityMutation(ws *Workspace, artifactType, complexity string) error {
    if artifactType != "task" {
        return fmt.Errorf("complexity is task-only; artifact type %q cannot store complexity: %w",
            artifactType, blerrors.ErrValidation)
    }
    // ...only now resolve schema and validate the enum value
}
```

Ordering matters: the type guard runs first so a customized schema that *does*
define the field on a non-task type still cannot persist it. Add a
customized-schema regression test that adds the field to a non-task type in
`header-def.yaml` and asserts the setter rejects it.

## Why this recurs

- `size` and `complexity` share the same "reserved key" seam *structure*
  (`internal/core/artifact_size.go`, `internal/core/artifact_complexity.go`).
  But as of this writing only `complexity` implements the task-only invariant:
  `validateComplexityMutation` guards `artifactType == "task"` and the projection
  drops non-task `complexity`. `size` has **neither** guard — `validateSizeValue`
  resolves the schema for whatever type is passed and `projectedCustomFields`
  does not drop non-task `size`. A customized non-task `size` schema would
  therefore be written *and* projected. That asymmetry is a latent gap: any
  future task-only typed field should copy the complexity pattern, and `size`
  should eventually be brought into line (with a regression test) rather than
  cited as an example that already does this.
- The projection guard and the setter guard look redundant but protect two
  different layers: the setter protects the file (source of truth); the
  projection protects the index (disposable cache). Both are required for a
  field to be genuinely task-only.

## Citations

- `internal/core/artifact_complexity.go:90-93` — complexity task-only guard precedes schema resolution
- `internal/db/upsert_projection.go:124-126` — projection drops non-task `complexity` only (no `size` drop)
- `internal/core/artifact_size.go:231-262,314-336` — `size` seam: same reserved-key structure but **no** `artifactType == "task"` guard (latent gap)
- Surfaced by Copilot review on PR #321 (shipment 113-S, feature 132-F), merge `685620ec`
