---
chunk_strategy: h1-h2-h3
description: "Backlogit complexity attribution and provenance direction aligned to autoharness cost-per-unit-of-work-reduction routing: the task-only complexity value, a new complexity_source/complexity_ruleset_version provenance pair, audit durability, provenance-completeness for fail-safe-upward routing, query surfaces, Stage/harvest stamping, backfill posture, deferred feature/shipment composition, and the input-attribution vs observed-outcome boundary"
doc_type: design
status: draft
created: 2026-09-10
schema_version: "1.0"
source: docs/design-docs/2026-09-10-complexity-attribution-provenance.md
title: "Complexity attribution and provenance for work-attribute model routing"
---

## Summary

Backlogit stores `complexity` as a bare, task-only enum today. Autoharness now
intends to route model tier as a function of a task's `complexity` and `size`,
and its routing policy requires that both dimensions be *provenance-complete*
before it will route a task down to a cheaper tier. Backlogit's `complexity`
field cannot satisfy that precondition: it has no source, no ruleset version, no
audit event, and no automatic derivation. This document sets backlogit's
complexity direction to close that gap while preserving the non-conflation of
`complexity` and `size`, and while keeping predicted difficulty (input
attribution) strictly separate from measured cost (observed outcome).

The concrete deliverable backlogit owes autoharness is a `complexity_source` /
`complexity_ruleset_version` provenance pair that mirrors the shipped `size`
contract, plus the audit, validation, storage, and query surfaces that make the
pair trustworthy. This is a design-and-stash artifact only: it recommends work
units for later Stage curation and does not create a task hierarchy or a
shipment.

## External intent: what autoharness needs

The authoritative external source is
`C:\Source\GitHub\autoharness\docs\design-docs\cost-per-unit-of-work-reduction.md`
(status draft, created 2026-09-10). Its objective is to reduce AIC cost per unit
of work while holding outcome quality constant, and one of its five components
depends directly on backlogit metadata.

### Work-attribute routing consumes complexity (Component C, §6)

Section 6.2 defines a two-axis routing table where `complexity` maps to model
capability tier (reasoning requirement) and `size` maps to context budget and
turn allowance (volume). The two axes are orthogonal and, per §3 and §6.6, must
never be combined into a scalar score. The diagonal cases are the justification:
an `L` / `trivial` mechanical rename should route cheap, and an `XS` / `high`
concurrency fix should route to the frontier tier.

### Downward routing requires provenance-completeness (§6.1)

Section 6.1 introduces sub-policy **P-013.7 (Work-Attribute Task Routing)** to
resolve a blocking conflict with P-013.1. Its precondition is explicit:

> Provenance-completeness is a precondition. Missing or invalid `size`,
> `complexity`, `size_source`, or `size_ruleset_version` ⇒ **fail safe upward**
> to the invoking agent's role tier. Never fail downward.

Autoharness lists `size_source` and `size_ruleset_version` because those exist
today. The symmetric complexity provenance does not yet exist, which is the gap
this document addresses. Until backlogit can express complexity provenance, a
complexity-derived downward route can never be provenance-complete on the
complexity axis, so the economically valuable diagonal cases cannot route down
safely.

### The explicit route-OUT to backlogit (§5.2)

Section 5.2 ("Gap 2 — complexity has no provenance pair") names the dependency
directly and assigns ownership:

> Ownership boundary: backlogit owns field definition, storage, and validation;
> autoharness owns application.
>
> **External dependency (route OUT)**: a backlogit-side `complexity_source` /
> `complexity_ruleset_version` storage pair. Track as a deferred external item;
> do not block this work on it.

Section 10.7 and §13 reinforce the split: autoharness explicitly places the
backlogit-side storage pair out of its own scope, and §13 also defers
feature- and shipment-level complexity composition. This document is backlogit's
acceptance of that route-OUT.

### The metric context (§5.1, §5.3)

Section 5.1 bumps the autoharness `execution-epoch` telemetry schema to 1.2.0 to
carry `task_complexity_label`, `complexity_sources`, and
`complexity_ruleset_versions`, mirroring the existing sizing snapshot fields. It
notes that feature- and shipment-level complexity labels are "null by contract"
because "backlogit stores `complexity` on the `task` artifact type only." Section
5.3 defines a named, versioned `size`-label-to-point mapping so cost-per-point
becomes measurable. Complexity does not get a point mapping; it selects tier, not
budget. These autoharness-owned telemetry structures are the join target for the
provenance backlogit will emit.

### Prior shared history

The autoharness backlog already carries the shared lineage: `107-F`
(size-complexity-first-class-staging, archived), `107.005-T` (Enable native
complexity field in backlogit workspace header-def, archived), `108-F`
(backlogit-telemetry-evidence-mapping, archived), and `108.002-T` (Add
non-conflated complexity dimension to event schema, archived). The current work
extends that lineage from "complexity exists as a label" to "complexity is a
provenance-complete, routable attribution."

## Backlogit baseline: what exists today

Verified against the working tree at commit `1.10.1` (dirty) on 2026-09-10.

* `complexity` is a task-only optional enum with values `trivial | low | medium
  | high`, declared in `.backlogit/header-def.yaml` under the `task` type.
* It is persisted as `custom_fields.complexity` and projected to the SQLite
  `items.complexity` column by `db.UpsertItem`.
* `core.SetArtifactComplexity` (`internal/core/artifact_complexity.go`) sets or
  clears it through a body-preserving frontmatter seam, validating the value
  against the header-def enum and rejecting any non-`task` artifact type.
* CLI (`backlogit update --complexity`, `backlogit list --complexity`) and MCP
  surfaces support set, clear, display, and filter. Filtering binds the value as
  a SQL parameter (`queries.go`).
* Complexity has **no queue-order effect**, no feature or shipment roll-up, and
  no interaction with model routing.

The gaps, measured against the shipped `size` contract in
`docs/design-docs/2026-07-19-size-estimation-contract.md`:

| Concern | `size` today | `complexity` today |
|---|---|---|
| Value enum, task-only | Yes | Yes |
| `*_source` provenance field | Yes (`human/agent/derived`) | Absent |
| `*_ruleset_version` field | Yes | Absent |
| Provenance-completeness rule | Yes (source requires ruleset) | Absent |
| Audit event before write | Yes (`estimate_history`, fail-closed) | Absent (no event emitted) |
| Generic-create reserved-key refusal | Yes | Not enforced for provenance keys |
| Transport-aware actor stamping | Yes (CLI `human`, MCP `agent`) | Absent |
| Computed-on-read composition rollup | Yes (histogram) | Absent |
| SQLite projection of provenance | Not applicable beyond value | Value only |

Live attribution in the backlogit workspace confirms the field is used but
sparsely: of 83 queued tasks, 20 carry a complexity value (8 medium, 7 high, 5
low) and 63 are null. None carry provenance, because provenance cannot be
expressed.

## Canonical semantics

Complexity and size remain orthogonal and must never be combined into a derived
scalar, matching §3 of the autoharness design and the shipped size contract.

* **`complexity`** — the a priori difficulty and uncertainty of a task: how much
  reasoning, design judgment, and unknown-resolution the work demands. It is an
  **input attribution**, an estimate made before or during staging.
* **`size`** — implementation volume: how much mechanical change and context the
  work spans. Unchanged by this document.
* **`priority`** — urgency and delivery order. Unchanged and never derived from
  or overridden by complexity.
* **observed cost / outcome** — measured spend, escalation, and dispersion.
  These live in autoharness telemetry, not in backlogit's complexity field.

Complexity stays **task-only**. Features and shipments never store a complexity
value; any aggregate view is a computed-on-read rollup (deferred, see
Aggregation).

### Input attribution versus observed outcome

This is the load-bearing semantic decision. Backlogit's `complexity` field is
**input attribution only** — a prediction of difficulty. It is never overwritten
with a measured outcome. Conflating the two would corrupt the calibration loop:
autoharness recalibrates the *ruleset* that produces estimates by comparing
predicted complexity against observed escalation and cost dispersion (§5.4, §6.4,
Risk R2). That comparison is only meaningful if the stored prediction is stable
and never silently mutated into a post-hoc measurement.

Consequences for backlogit:

* Backlogit stores and versions the prediction and its provenance.
* Backlogit never writes measured cost, escalation counts, or outcome quality
  into the complexity field or its provenance.
* A recalibration that produces a *new* estimate is a new attributed mutation
  carrying a new `complexity_ruleset_version` — a fresh prediction, not a
  measurement written back. The audit trail preserves the prior estimate.
* Outcome correlation is achieved by join, not by co-location: backlogit exposes
  stable task IDs plus complexity provenance; autoharness telemetry carries the
  observed series and joins on task ID.

## Proposed backlogit contract

Backlogit adds a complexity provenance pair and the machinery that makes it
trustworthy, mirroring the `size` contract field-for-field so the two dimensions
stay symmetric and auditable.

### Attribution and provenance fields

Two new task-only frontmatter fields, declared in the header-def alongside the
existing `size_source` / `size_ruleset_version`:

* `complexity_source` — enum `human | agent | derived`. Absent means unknown or
  legacy; it is never rewritten as `human`. Setting a plain complexity value does
  not stamp a source.
* `complexity_ruleset_version` — opaque version string identifying the ruleset
  that produced the estimate. Not enum-validated; its presence is what matters.

Provenance-completeness rule (mirrors size Rule R): an explicit
`complexity_source` requires an accompanying `complexity_ruleset_version`. A
source without a ruleset is rejected and the file is left unchanged. A plain
complexity value with no source remains valid (legacy-compatible).

### Storage, audit, and validation

* **Audit event before write.** Complexity mutations append an audit event
  (proposed `estimate_history` reused with a `dimension: complexity`
  discriminator, or a dedicated `complexity_history` event) before the durable
  frontmatter write, fail-closed: if the event cannot be appended, the mutation
  is refused. This closes the current gap where `SetArtifactComplexity` writes
  with no audit trail. The durable `custom_fields.complexity` (with provenance)
  remains the single source of truth; the event stream is advisory. Orphan
  crash-residue events are ignored on read, matching the size policy.
* **Durable write protocol.** Reuse the shared atomicfile primitive and the
  opt-in `durable_writes` protocol already used by the size seam, including the
  two-class `ErrWriteNotApplied` / `ErrWriteIndeterminate` error contract and
  retry-scoped-to-the-atomic-write discipline so the audit-event count stays
  exactly one.
* **Transport-aware actor stamping.** CLI defaults `complexity_source` to
  `human`; MCP defaults to `agent`. An explicit `complexity_source: human`
  submitted over MCP is rejected regardless of ruleset. This is the same trust
  boundary the size seam enforces.
* **Generic-create refusal.** Creating an artifact through the generic create
  path that carries any reserved complexity key (`complexity`,
  `complexity_source`, `complexity_ruleset_version`) is refused, so an initial
  complexity is never recorded off-seam, unvalidated, and eventless. All initial
  complexity attribution routes through the audited complexity seam.
* **Whole-map preservation.** A generic `custom_fields` update preserves reserved
  complexity keys through an explicit merge, so a generic field edit does not
  silently drop complexity provenance.
* **Validated-once.** The seam validates value, source, and provenance
  completeness at mutation time; downstream readers trust the persisted value.

### Query and provenance-completeness surface

* Project `complexity_source` and `complexity_ruleset_version` into the SQLite
  `items` table (new columns) so provenance is queryable without reading
  Markdown.
* Extend query filters to select by `complexity_source` and by presence of
  ruleset version.
* Expose a **route-eligibility predicate**: a read helper that reports whether a
  task is provenance-complete on both axes — `size`, `size_source`,
  `size_ruleset_version`, `complexity`, `complexity_source`,
  `complexity_ruleset_version` all present and valid. This is the exact
  precondition P-013.7 fail-safe-upward evaluates (§6.1). Backlogit computes and
  exposes it; autoharness consumes it. Backlogit does not itself perform routing.

### Non-goals

* Backlogit does not implement model routing, tier selection, or the routing
  table. It supplies the attribution and provenance that routing consumes.
* Backlogit does not store observed cost, escalation counts, or the
  size-label-to-point mapping (`ah-size-points-v1`). Those are autoharness-owned.
* Backlogit does not give complexity a queue-order or priority effect. Routing is
  not scheduling; priority and dependency gates remain authoritative for order.
* Backlogit does not add feature- or shipment-level *stored* complexity. Any
  aggregate is computed-on-read and is deferred (see Aggregation), matching
  autoharness §13.
* Backlogit does not auto-derive complexity from task text in this pass; a
  `derived` source is a declared provenance value, not an inference engine.

## Stage and harvest assignment workflow

Complexity attribution is a staging responsibility. When Stage harvests a plan
into tasks:

* Each task receives a complexity value and, where a ruleset is in force, a
  `complexity_source` and `complexity_ruleset_version` stamped through the
  audited seam. Agent-authored stamps carry `complexity_source: agent`.
* Harvest emits a complexity-homogeneity assessment (aligned with autoharness
  §7): a task that bundles a hard design core with mechanical edge work is
  flagged for re-split, because complexity heterogeneity within a task forces the
  whole task onto the hard tier. This is a staging-quality signal, not a routing
  action.
* The two-axis 2-hour reliability gate is untouched. It governs reliability, not
  cost; complexity provenance does not repurpose it.
* Stage may leave complexity unstamped when uncertain. Unset complexity is a
  valid state (see Unknown handling) and routes fail-safe-upward rather than
  blocking harvest.

Backlogit's role here is to make the stamp cheap, audited, and provenance-checked
at the seam. The staging *policy* (when to stamp, which ruleset) is expressed in
the autoharness Stage and harvest templates.

## Handling of unknown or unset complexity

Unset complexity is a first-class, safe state and must never be coerced.

* A task with no complexity value is valid. It is not defaulted to `medium` or
  any other value at the storage seam.
* A task with a complexity value but no source is valid (legacy-compatible) and
  is treated as provenance-incomplete on the complexity axis.
* The route-eligibility predicate reports provenance-incomplete for any of these
  states, and autoharness fail-safe-upward routes such tasks to the invoking
  agent's role tier (§6.1). Missing attribution costs a conservative route, never
  an unsafe cheap one.
* Backlogit never fails a read or a listing because complexity is absent; the
  63 currently-null queued tasks continue to list and query normally.

## Historical backfill posture

* **No bulk auto-backfill.** Existing tasks are not machine-stamped with a
  guessed complexity or a synthesized provenance. A fabricated estimate would
  poison the calibration loop it is meant to feed.
* **Backfill is opt-in and attributed.** When an operator or agent backfills a
  historical task, the mutation routes through the audited seam and carries an
  explicit `complexity_source` and `complexity_ruleset_version`, so a backfilled
  estimate is distinguishable from an original one only by its event timestamp,
  never by missing provenance.
* **Legacy values are respected.** The existing 20 attributed tasks keep their
  values; they become provenance-incomplete (source absent) and therefore
  fail-safe-upward until deliberately re-stamped. No silent rewrite.
* **Migration is additive.** Adding the provenance columns and header-def fields
  does not require rewriting any existing artifact; absent provenance reads as
  null.

## Feature and shipment aggregation

Aligned with autoharness §5.1 and §13, feature- and shipment-level complexity is
**deferred and non-stored** in the first phases.

* No stored complexity on features or shipments; the field stays task-only.
* When aggregation is wanted, provide a **computed-on-read complexity
  composition** mirroring the size histogram: a `trivial | low | medium | high`
  count map plus an `unattributed` count over a feature's direct task children or
  a shipment's resolved task manifest, de-duplicated, with unresolved members
  warn-skipped. It never persists and never blocks a read.
* Composition is a distribution, not a rolled-up scalar. There is no "feature
  complexity = max(children)" reduction, because that would re-introduce the
  scalar conflation §3 forbids and would misrepresent a mixed-complexity feature.
* This is explicitly a later phase; the routing use case needs only task-level
  attribution.

## Work-selection and routing use without overriding safety gates

Complexity provenance informs *which model tier executes a task*, never *whether
or when a task is eligible to run*.

* Dependency edges (`blocks`, parent) and priority remain the sole authorities
  for eligibility and order. A `trivial` task blocked by an unfinished dependency
  stays blocked; a `high` task does not jump the queue because of its tier.
* Routing consumes attribution read-only. Backlogit exposes the values and the
  route-eligibility predicate; it does not select, downgrade, or upgrade a model.
* Fail-safe-upward is the only asymmetry backlogit encodes into the predicate:
  incomplete provenance can only make routing *more* conservative, never cheaper.
* P-013.7's determinism and auditability requirements are met by backlogit at the
  data layer: every stamp is validated, versioned, and audited, so a downward
  route is always traceable to a concrete, provenance-complete attribution.

## Telemetry and outcome correlation

* Backlogit remains the attribution and provenance authority; autoharness remains
  the telemetry and measurement authority.
* Correlation is by join on stable task ID. Backlogit exposes `complexity`,
  `complexity_source`, and `complexity_ruleset_version`; autoharness's
  `execution-epoch` 1.2.0 snapshot (§5.1) carries `task_complexity_label`,
  `complexity_sources`, and `complexity_ruleset_versions` captured once at
  `pre_execution` and frozen through close. The two agree by construction when
  the snapshot is captured from the backlogit record.
* Backlogit does not compute cost-per-point, escalation rate, or dispersion.
  Those reports (§5.4) belong to autoharness telemetry.
* The freeze semantics matter: autoharness captures the estimate at
  pre-execution and never re-reads it at close. Backlogit's audit trail lets a
  later analyst reconstruct what the estimate was at capture time even if the task
  was re-stamped afterward.

## Calibration and versioning

* `complexity_ruleset_version` is the calibration hinge. A recalibrated ruleset
  produces new estimates under a new version string; history is not invalidated
  because each estimate carries the version that produced it.
* Backlogit treats the ruleset version as opaque; it validates presence, not
  content. Ownership of the ruleset itself and of the point/tier mappings is
  autoharness's.
* Re-stamping a task under a new ruleset version is an ordinary audited mutation.
  The prior estimate is preserved in the event stream, enabling before/after
  calibration analysis.

## Compatibility and migration

* **Additive schema.** New header-def fields and new SQLite columns; existing
  artifacts read unchanged with null provenance.
* **Backward-compatible reads.** All current CLI/MCP/query surfaces keep working;
  provenance is opt-in on write and null-tolerant on read.
* **Existence-gated behavior.** Provenance-completeness checks treat absent
  fields as incomplete rather than erroring, so a workspace that never stamps
  complexity behaves exactly as today.
* **Index migration.** The SQLite projection gains columns via the existing index
  migration path; a sync rebuilds the projection from the canonical Markdown.
* **Reserved-key handling on import.** Migration adapters that prefix imported
  frontmatter keys (as the backlog.md adapter does for `size`) keep reserved
  complexity keys from arriving off-seam.

## Privacy and security

* Provenance carries no secrets: `complexity_source` is a three-value enum and
  `complexity_ruleset_version` is an opaque, operator-chosen version string.
  Neither should encode PII or credentials; document this as a constraint on
  ruleset naming.
* The transport trust boundary is preserved: MCP (agent) cannot claim `human`
  provenance, matching the size seam.
* Audit events record actor and dimension but not raw payloads, consistent with
  the size audit policy.
* Path containment and validated-once guarantees carry over from the shared seam;
  no new file-lookup surface is introduced.

## Rollout phases

Phased so the external route-OUT autoharness needs lands first and independently.

* **Phase 0 — decision.** Ratify this design; confirm ruleset ownership and the
  audit-event shape (reuse `estimate_history` with a dimension discriminator vs a
  dedicated `complexity_history`) with autoharness. Resolve the open questions.
* **Phase 1 — storage and validation (the route-OUT).** Header-def fields,
  core-seam provenance acceptance, provenance-completeness rule, audit event
  before write, transport-aware actor stamping, generic-create refusal,
  whole-map preservation. This alone unblocks autoharness P-013.7 on the
  complexity axis.
* **Phase 2 — query and route-eligibility.** SQLite projection of provenance
  columns, provenance filters, and the route-eligibility predicate consumed by
  fail-safe-upward.
* **Phase 3 — Stage and harvest stamping.** Harvest emits complexity with
  provenance and a homogeneity assessment; backfill posture documented and
  operator-driven.
* **Phase 4 — deferred aggregation.** Computed-on-read complexity composition for
  features and shipments, only if a consumer materializes.

## Recommended work units (for later Stage curation)

These are candidate work units, not a task hierarchy. Stage owns decomposition,
sizing, and sequencing. They are listed in dependency-friendly order.

1. Header-def and schema: add task-only `complexity_source` (enum) and
   `complexity_ruleset_version` (string) fields; keep optional and existence-gated.
2. Core seam: extend `SetArtifactComplexity` to accept and validate provenance,
   enforce the source-requires-ruleset completeness rule, and reject invalid
   source values before any write.
3. Audit durability: append a fail-closed complexity audit event before the
   frontmatter write; ignore orphan crash-residue events on read.
4. Transport-aware actor stamping: CLI defaults `human`, MCP defaults `agent`,
   reject explicit `human` over MCP.
5. Generic-create refusal and whole-map preservation for reserved complexity keys.
6. SQLite projection: add `complexity_source` and `complexity_ruleset_version`
   columns and the index migration.
7. Query surface: filter by complexity provenance and expose the two-axis
   route-eligibility predicate for fail-safe-upward.
8. CLI/MCP parity: `--complexity-source` / `--complexity-ruleset-version` flags
   and the matching MCP params, with size-style mutual-exclusivity rules.
9. Docs graduation: update the size-complexity semantics reference and telemetry
   correlation notes; this design doc is the durable anchor.
10. Deferred: computed-on-read complexity composition rollup for features and
    shipments (Phase 4, gated on a real consumer).

## Measurable acceptance criteria

* A complexity mutation with `complexity_source` set and
  `complexity_ruleset_version` absent is rejected, and the artifact file is
  byte-identical afterward.
* A complexity mutation appends exactly one audit event before the durable write;
  a simulated append failure refuses the mutation and leaves the file unchanged.
* An explicit `complexity_source: human` over the MCP transport is rejected; the
  CLI transport stamps `human` by default and MCP stamps `agent`.
* `complexity_source` and `complexity_ruleset_version` are queryable via SQL
  without reading Markdown, and a bound-parameter filter on `complexity_source`
  is injection-safe.
* The route-eligibility predicate returns provenance-complete only when all six
  fields across both axes are present and valid, and provenance-incomplete for
  every unset or partial state.
* Existing artifacts (including the 63 null-complexity and 20
  value-only-no-provenance queued tasks) read, list, and query without error
  after the additive migration.
* Generic create refuses any reserved complexity key; generic `custom_fields`
  update preserves existing complexity provenance.
* No feature or shipment stores a complexity value; any aggregate view is
  computed-on-read and never persisted.

## Open questions and decisions

1. **Audit-event shape.** Reuse `estimate_history` with a `dimension` field
   spanning size and complexity, or introduce a dedicated `complexity_history`
   event. The former keeps one audit stream; the latter avoids overloading the
   size event. Decide in Phase 0.
2. **Ruleset ownership and naming.** Autoharness owns the ruleset that produces
   estimates; confirm the version-string convention (mirrors autoharness open
   question 1 on `ah-size-points-v1`). Backlogit only validates presence.
3. **Aggregation demand.** Confirm no near-term consumer needs feature/shipment
   complexity composition, keeping Phase 4 deferred (aligns with autoharness §13).
4. **Route-eligibility predicate location.** Whether the predicate ships as a
   query helper, a dedicated CLI/MCP surface, or both. Recommendation: a query
   helper first (Phase 2), a surface only if autoharness needs it directly.
5. **Backfill governance.** Whether backfill stamping needs an operator-only
   guard beyond the standard audited seam, given calibration sensitivity.

## References

* `C:\Source\GitHub\autoharness\docs\design-docs\cost-per-unit-of-work-reduction.md`
  (§3 non-conflation, §5.1 telemetry complexity fields, §5.2 route-OUT provenance
  pair, §5.3 label-to-point mapping, §6.1 P-013.7, §6.2 routing table, §7
  complexity seams, §13 out of scope, §14 open questions).
* `docs/design-docs/2026-07-19-size-estimation-contract.md` — the shipped `size`
  provenance, audit, and composition precedent this design mirrors.
* `internal/core/artifact_complexity.go`, `internal/cli/list.go`,
  `internal/cli/update.go`, `internal/db/queries.go`,
  `.backlogit/header-def.yaml` — the current complexity implementation.
