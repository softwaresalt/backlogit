---
chunk_strategy: h1-h2-h3
description: Model A owner-scoped extension model for docline frontmatter — backlogit extends the docline base contract as an owner profile nested under the open docline map (docline.backlogit.*), the top level stays additionalProperties:false, a layered ext schema (allOf on base v1) validates only the docline.backlogit subtree, and the .backlogit artifact-codec durability boundary is recorded and deferred to the 109.x spike.
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md
title: 'Decision: docline base-contract-plus-owner-profile extension model (Model A)'
docline:
    date: 2026-07-16T22:40:00Z
    decision_status: decided
    linked_deliberation_id: 052-DL
    linked_stash_ids:
        - F5D1A8AA
---

## Context

The docline base frontmatter contract
(`schemas/docline/base-frontmatter-v1.schema.json`) is the shared, closed
surface that graphtor-docs ingestion and engram read. Its top level is
`additionalProperties: false`, and it requires `title`, `source`,
`ingested_at`, and `doc_type`. The contract also defines a single open
namespace, the `docline` map (`additionalProperties: true`), whose stated
purpose is to hold non-contract metadata so that metadata is never mistaken for
a graphtor-contract field.

backlogit needs to attach owner-specific metadata (the motivating case is the
per-document size estimate from feature 109-F) to documents without polluting
the shared contract. Deliberation 052-DL framed this as an ownership fork:

* **Model A** — nest owner metadata under the open `docline` map as
  `docline.<owner>.*` and keep the base top level closed.
* **Model B** — open the base top level to producer-owned extension keys and
  preserve unknown top-level keys in place.

We choose **Model A**. This decision supersedes the Model-B direction that was
previously scoped in tasks 107.009-T, 107.010-T, 107.011-T, and 107.012-T, and
the top-level-open direction that 109.001-T had recorded.

## Decision

Adopt a base-contract-plus-owner-profile model:

* **docline is the base contract.** Its closed top level
  (`additionalProperties: false`) and its required fields are unchanged. The
  base schema is **not** opened.
* **backlogit is an owner profile** nested under the open `docline` map. Owner
  extensions live at `docline.<owner>.*`; backlogit is the first owner, at
  `docline.backlogit.*`.
* **Per-owner versioning.** Each owner profile carries its own
  `schema_version`, independent of the docline base `schema_version`.
* **Additive, not enveloping.** An owner profile adds keys to the `docline`
  map. It never wraps or relocates docline base fields.
* **Layered extension schema.** `schemas/docline/ext/backlogit-v1.schema.json`
  is composed `allOf` on top of docline base-frontmatter v1 and validates
  **only** the `docline.backlogit` subtree. It is owned and versioned by
  backlogit.

A minimal example on a docline document:

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

## Why nest under docline

Nesting under the `docline` map is the only placement that is both
schema-valid and round-trip-durable for docline documents:

* **Schema-valid.** The base top level is `additionalProperties: false`, so any
  top-level `backlogit` or `x-backlogit` key fails validation. The `docline`
  map is `additionalProperties: true`, so `docline.backlogit` validates and
  remains isolated from the shared contract surface.
* **Round-trip-durable for documents.** The docline codec carries the whole
  `docline` map. `internal/docline` `BaseFrontmatter.Docline` captures it in
  `FromMap` and re-emits it in `ToMap`, so `docline.backlogit.*` survives a
  normal docline read/normalize/write cycle unchanged.

## Engram and graphtor non-interference

Model A leaves the shared contract untouched, so existing consumers are
unaffected:

* The top level keeps the same closed field set and the same required fields.
  engram and graphtor continue to read exactly the contract they read today.
* backlogit metadata lives inside the `docline` bag, whose purpose is to keep
  non-contract metadata off the graphtor-contract surface. Nesting honors that
  purpose rather than working around it.
* A consumer that wants to be dual-aware composes base v1 with the
  `docline.backlogit` ext schema. It can validate both the shared contract and
  the backlogit owner profile without either schema knowing about the other's
  fields.

## Traps to avoid

Two placements are explicitly rejected:

* **No top-level owner key.** Do not add a top-level `backlogit` or
  `x-backlogit` key. It fails `additionalProperties: false` and pollutes the
  surface engram trusts.
* **No enveloping.** Do not wrap the docline core inside a backlogit container.
  engram expects the base contract fields at the top level; relocating them
  breaks that expectation.

## Durability boundary on .backlogit artifacts (loss point)

Model A resolves **placement and schema ownership** for docline documents. It
does **not** by itself make `docline.backlogit.*` durable on `.backlogit`
feature, task, or shipment artifact frontmatter, and this decision does not
claim otherwise.

The reason is a distinct codec. `.backlogit` artifacts use the generic artifact
codec, not the docline codec:

* `models.ArtifactFromFrontmatter` (`internal/models/frontmatter.go`) recognizes
  only the enumerated `Artifact` struct fields plus `custom_fields`.
* `core.WriteArtifactFile` (`internal/core/artifacts.go`) re-emits only those
  struct fields plus `custom_fields`.
* `models.Artifact` has no docline carrier.

Therefore a top-level `docline` map carrying `docline.backlogit.*` is **dropped**
on a normal `.backlogit` artifact update. docline documents are durable via the
docline codec; `.backlogit` artifacts are not. The source-stash grounding note
that "the backlogit artifact codec preserves nested maps" is too broad for
artifacts: it preserves recognized maps such as `custom_fields`, not an
unmodeled top-level `docline` map.

Do not present `docline.backlogit` as already durable on `.backlogit` artifacts.

## Scope and follow-up

* This decision and the deliverables in feature 110-F are scoped to docline
  **documents**: the decision doc, the `docline.backlogit.*` authoring
  convention, and the layered ext schema (`allOf` on base v1). The ext schema
  cannot serve as the contract for `.backlogit` artifact frontmatter, because
  `allOf` on base v1 would reject artifact fields such as `id`, `artifact_type`,
  and `status`.
* Persisting `docline.backlogit` on `.backlogit` artifacts requires an
  **artifact-codec bridge** — either add a `Docline` carrier to
  `models.Artifact` (proven by an executable round-trip test) or route
  artifact-owned metadata through `custom_fields`. That bridge selection is a
  required, still-open prerequisite and is deferred to the 109.x size spike
  (109.004-T) and the 108-F size implementation gate. It is out of scope here.

## Supersedes

* Model-B tasks 107.009-T, 107.010-T, 107.011-T, and 107.012-T (top-level-open
  direction and top-level extension-key preservation).
* The top-level-open direction previously recorded in the 109.001-T ownership
  spike.
