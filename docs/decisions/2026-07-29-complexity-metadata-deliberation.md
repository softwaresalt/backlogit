---
chunk_strategy: h1-h2-h3
description: Deliberation on adding optional work-item complexity metadata alongside size and priority, grounded in the current task-only size implementation and its CLI, MCP, WIT metadata, and index surfaces.
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-29-complexity-metadata-deliberation.md
title: Complexity metadata attribute deliberation
---

## Problem Frame

Stash entry `D46F3B0C` asks whether backlogit should add `complexity` as an
additional work-item metadata attribute similar to the existing `size` field.
The requested scope is additive:

* frontmatter metadata
* a body-preserving core setter in the `SetArtifactSize` style
* CLI and MCP update surfaces
* discovery through WIT metadata and allowed values
* SQLite index and query support
* an explicit explanation of how complexity complements size and priority

The design question is not whether backlogit needs another arbitrary label.
The question is whether a dedicated, validated ordinal gives useful planning
signal that neither size nor priority captures.

## Research Findings

### Current size implementation

The existing size implementation is a task-only logical field:

* `internal/config/defaults.go` defines task-only `size`, `size_source`, and
  `size_ruleset_version` fields. `size` is optional, has no default, and allows
  `XS`, `S`, `M`, `L`, `XL`.
* `.backlogit/header-def.yaml` carries the live WIT metadata for the same
  task-only size enum.
* `internal/core/artifact_size_schema_test.go` documents the storage contract:
  `size` is a logical task field physically stored in
  `custom_fields.size`.
* `internal/core/artifact_size.go` is the key implementation precedent:
  `SetArtifactSize` delegates to `SetArtifactSizeWithProvenance`, validates
  against WIT metadata, preserves the Markdown body byte-for-byte through
  `mdfront`, appends an estimate-history event before the durable write, and
  re-upserts a fully populated artifact into SQLite.
* `internal/cli/update.go` exposes `--size`, `--size-source`, and
  `--size-ruleset-version`. Size mutation flags are mutually exclusive with
  generic field or section updates so the body-preserving seam cannot be
  bypassed.
* `internal/mcp/tools.go` exposes `size`, `size_source`, and
  `size_ruleset_version` on `backlogit_update_item`. The MCP handler routes the
  same reserved fields through the audited size seam and rejects
  `size_source=human` from the agent transport.
* `internal/core/size_composition.go`, `internal/cli/list.go`, and
  `internal/mcp/tools.go` project a computed-on-read `size_composition` rollup
  for feature and shipment read surfaces. The stored size remains task-only.
* `internal/db/schema_gen.go` generates SQLite columns from header-def fields,
  and the live index already has `size`, `size_source`, and
  `size_ruleset_version` columns. `internal/db/queries.go` still scans artifacts
  from the modeled columns plus `custom_fields`; it does not currently expose a
  `QueryFilters.Size` helper.

### Prior learnings

The learnings search returned high-confidence guidance:

* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`:
  generic typed rewrites can drop unmodeled frontmatter keys. A new metadata
  field should either be fully modeled or use a raw, body-preserving setter like
  size.
* `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`:
  re-persist seams must reload from Markdown/source of truth or fully project
  new fields, otherwise DB fast paths lose data.
* `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`:
  CLI, MCP, and index updates must land together.
* `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`:
  any new query/list filter needs CLI/MCP parity tests.
* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`:
  machine-readable fields need exact names, allowed values, and producer and
  consumer contracts.
* `docs/compound/2026-06-26-docline-frontmatter-contract.md`:
  frontmatter updates must be body-preserving and idempotent.

## Options Evaluated

### Option A: Add task-only optional complexity with lower-case ordinal values

Add `complexity` as an optional task-only logical field stored in
`custom_fields.complexity`, with allowed values:

* `trivial`
* `low`
* `medium`
* `high`

Use a `SetArtifactComplexity` seam patterned after `SetArtifactSize`, but
without provenance fields unless a future ruleset needs them. Add CLI/MCP update
parameters, WIT metadata, SQLite projection/query support, and list/read
projection.

Pros:

* Mirrors size's task-only storage and validation model
* Avoids overloading priority with risk or uncertainty semantics
* Uses readable values for operator and agent workflows
* Leaves absent complexity as "unknown" without creating a misleading default
* Keeps queue policy stable while enabling explicit filtering and custom
  ordering

Cons:

* Adds one more metadata dimension operators must understand
* Requires careful CLI/MCP/index parity work
* Does not solve feature-level aggregation unless a later rollup is added

Effort estimate: medium.

Fit: high.

### Option B: Add complexity as a feature/task field with default `medium`

Define `complexity` on features and tasks, defaulting to `medium`, and allow
queue ordering to use it directly.

Pros:

* Every item has an explicit value
* Feature-level queue ordering becomes straightforward
* Easier for simple SQL sorting

Cons:

* Defaults create false certainty for legacy work
* Feature-level complexity can conflict with task-level reality
* Diverges from size's task-only design, where aggregate items use read-time
  composition instead of stored estimates
* More likely to change queue behavior unexpectedly

Effort estimate: medium.

Fit: medium.

### Option C: Do not add a field; use labels or priority

Keep complexity as labels such as `complexity-high`, or fold it into priority.

Pros:

* No schema, CLI, MCP, or index changes
* Avoids adding another metadata concept

Cons:

* Labels are not a closed vocabulary
* Priority becomes overloaded with urgency and difficulty
* Query and discovery surfaces cannot advertise the contract reliably
* Agents cannot validate values fail-closed

Effort estimate: low.

Fit: low.

## Trade-off Comparison

| Criterion | Option A: task-only optional | Option B: defaulted feature/task | Option C: labels/priority |
|---|---|---|---|
| Mirrors size implementation | High | Medium | Low |
| Value contract clarity | High | Medium | Low |
| Backward compatibility | High | Low | High |
| Queue behavior risk | Low | Medium | Low |
| Query/filter usefulness | High | High | Low |
| Implementation risk | Medium | Medium | Low |

## Decision

Recommend **Option A**: add an optional task-only `complexity` field with
allowed values `trivial`, `low`, `medium`, and `high`.

The field should mean **implementation difficulty and uncertainty**, not amount
of work or urgency:

* `size` answers "how much implementation volume is this?"
* `complexity` answers "how hard, uncertain, cross-cutting, or cognitively risky
  is this?"
* `priority` answers "how important or time-sensitive is this?"

For estimation, size and complexity should be read together. A task can be
small but high complexity when it touches a risky invariant, and large but low
complexity when it is repetitive or mechanical. For queue ordering, the initial
feature should **not** change default queue ordering. Priority remains the
default scheduling signal. Complexity should be available for filters, SQL, and
operator-defined ordering such as:

* priority first, then low-complexity quick wins
* priority first, then high-complexity risk burn-down
* capacity planning that combines size and complexity

This preserves current queue semantics while making the new planning signal
machine-readable.

## Rejected Alternatives

Option B is rejected because defaulting complexity to `medium` would encode
unknown legacy work as known medium-complexity work. It also risks competing
feature-level and task-level complexity values. If aggregate rollups become
important, backlogit should add a computed-on-read `complexity_composition`
later, analogous to `size_composition`.

Option C is rejected because labels and priority cannot provide a fail-closed,
discoverable, validated metadata contract. It would also encourage operators and
agents to overload priority with implementation risk.

## Unresolved Questions

The recommendation intentionally leaves these implementation-level questions for
plan review:

* Should complexity get provenance fields like `complexity_source` and
  `complexity_ruleset_version` now, or stay simple until an automated estimator
  exists? Recommendation: stay simple now.
* Should a `complexity_composition` rollup ship in the first slice?
  Recommendation: defer unless a consumer needs aggregate complexity immediately.
* Should existing size index projection be generalized while adding complexity?
  Recommendation: yes, but keep the task scoped to query/index support and avoid
  changing default queue order.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Complexity becomes a synonym for size | Document the distinction in CLI/MCP descriptions and WIT metadata; use lower-case difficulty terms rather than T-shirt sizes. |
| Generic updates drop the field | Route writes through a body-preserving core seam and preserve reserved complexity keys across generic updates. |
| CLI/MCP/index drift | Add parity tests for update parameters, list/query filters, and WIT metadata. |
| Queue behavior changes unexpectedly | Expose filter/query support first; do not alter default queue ordering in this release. |
| SQLite column projection diverges from `custom_fields` | Populate the `items.complexity` extension column from the same source-of-truth frontmatter decode and test sync/upsert behavior. |
