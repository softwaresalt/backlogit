---
title: "Shipment sequencing primitives for dark-factory queue ordering"
description: "Deliberation on the smallest high-value coherent backlogit product scope for clean multi-shipment ordering in long-running dark-factory runs (stash 0B5FA82B), reusing spike 001-SP Option B."
source: docs/decisions/2026-08-01-shipment-sequencing-primitives-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Which of the shipment-sequencing product gaps (a/b/c/d) should ship as one coherent backlogit release unit for dark-factory multi-shipment ordering (0B5FA82B)"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-08-01-shipment-sequencing-primitives-plan.md"
  - "docs/decisions/2026-07-29-ship-sequence-manifest-spike.md"
tags:
  - "feature"
  - "shipment"
  - "queue"
  - "dependencies"
  - "dark-factory"
  - "stage"
stash_ids:
  - "0B5FA82B"
---

## Problem Frame

Stash `0B5FA82B` (low priority, kind `feature`) captures the **backlogit
product-side** shipment-sequencing gaps that make multi-shipment ordering
uneven for long-running dark-factory (P-017) runs. The prior spike `001-SP`
(`docs/decisions/2026-07-29-ship-sequence-manifest-spike.md`) already answered
the adjacent question — a standalone `.backlogit/ship_sequence.jsonl` scheduler
is **deferred** — and recommended **Option B**: make the Orchestrator consume
the existing queue surface (`queue view --type shipment --status queued`), use
`item_deps` for true blocking order, and record ordered scope in dark-mode
activation evidence.

The stash lists four candidate enhancements:

* **(a) shipment-scoped `queue_position`** so ordering does not renumber across
  the combined default-status queue view. `queue_position` is a single global
  integer across all item types today
  (`internal/core/queue.go:47,174-207` `buildQueueOrderClause`; `:245` `MoveInQueue`),
  and `custom_fields.queue_position` is not settable via `update_item`.
* **(b) first-class shipment&rarr;shipment blocking-order affordance** for
  `queue view --type shipment --status queued`. `item_deps` already supports
  generic edges via `AddDependency`, but `dep_type` collapses to `blocks` on
  sync (`internal/core/dependencies.go:22-27`).
* **(c) meaningful shipment priority** so the queue secondary sort is
  well-defined. Queued shipments commonly carry no priority, which leaves the
  Orchestrator "highest-priority queued shipment" selection rule
  under-specified.
* **(d) optional non-authoritative `ship_sequence` audit surface** (spike
  Option D) recording a dark-mode activation plan; the stash itself constrains
  this to "build only after a real multi-shipment run shows activation evidence
  is insufficient."

The pure Orchestrator/policy/playbook hardening is explicitly **out of scope**
for this backlogit repository — it belongs in the external autoharness
templates. This deliberation decides only the backlogit code-side scope.

**Decision question**: what is the *smallest high-value coherent* backlogit
release unit, supported by spike `001-SP`, that makes dark-factory
multi-shipment ordering clean — and which candidates must be deferred under
YAGNI and scope-boundary review?

### Success Criteria

* The selected scope is directly supported by the spike `001-SP` recommendation
  (Option B), not a new authoritative scheduler.
* The scope forms one coherent, single-domain release unit that ships as one
  pull request and one shipment.
* Excluded candidates are deferred **explicitly** with rationale; scope does not
  silently expand.
* Every task is 2-hour-sized, single-domain, test-first, with explicit
  dependencies.

### Scope Boundaries (explicit)

* **In scope**: backlogit code-side primitives (b) and (c) that make
  `queue view --type shipment --status queued` a clean sequencing surface.
* **Out of scope this release unit**: candidate (a) global-`queue_position`
  refactor; candidate (d) `ship_sequence.jsonl` audit surface; any
  Orchestrator/policy/template hardening (external autoharness); any Stage-side
  source/test/config edits (Ship executes implementation).

## Research Findings

### Spike 001-SP (authoritative prior research)

* **Conclusion**: defer a standalone `ship_sequence.jsonl`; it would duplicate
  data already owned by shipment manifests, `item_deps`, queue ordering, and
  lifecycle status, creating split-brain state.
* **Recommended path (Option B)**: the existing `queue_position` and
  `item_deps` primitives can already express shipment ordering because
  shipments are ordinary artifacts in the `items` table; the missing pieces are
  a well-defined **priority** secondary sort and a usable **blocking-order**
  affordance for shipments.
* **Named gap**: Orchestrator's "highest-priority queued shipment" rule is
  under-specified when queued shipments carry no priority.
* **Option D** (`ship_sequence.jsonl` as non-authoritative audit plan) is the
  only safe manifest variant, and is gated on a real multi-shipment run proving
  activation/checkpoint evidence insufficient.

### Codebase grounding (current v1.7.0)

* **Priority (c)**: `buildQueueOrderClause` already supports a `priority`
  secondary sort with the canonical critical/high/medium/low mapping
  (`queue.go:196-207,237`). `UpdateArtifact` already applies `priority` to any
  artifact including a shipment (`artifacts.go:441,491`), and `WithPriority`
  is an existing create option (`artifacts.go:57`). The gaps: `CreateShipment`
  (`shipment.go:54`) takes no priority, and the CLI `shipment create` command
  exposes only `--title`/`--items` (`internal/cli/shipment.go:88-118`); the MCP
  `create_shipment` tool exposes only `title`/`items`. Priority filter/param
  plumbing already exists for `list`/`list_items`.
* **Blocking order (b)**: `filterByResolvedDependencies` treats `blocks`,
  `parent_of`, and `relates_to` as execution-blocking and suppresses a queue
  item until its blocker reaches a no-longer-blocking status
  (`queue.go:438-495`). Because `AddDependency` writes only the target ID to
  frontmatter and Rehydrate rebuilds every edge as `blocks`
  (`dependencies.go:15-27`), a shipment&rarr;shipment edge **already** blocks
  the dependent shipment in `queue view --type shipment --status queued` and
  survives `sync_index`. The gap is a first-class, validated, documented
  affordance plus a regression guard — not new core ordering logic.
* **Queue position (a)**: `MoveInQueue` renumbers within the caller's filter,
  but writes to a single global `custom_fields.queue_position` consumed by
  `buildQueueOrderClause` for every item type. Positions set in a
  shipment-only view therefore interleave with task/feature positions in the
  combined view. Making positions shipment-scoped changes shared ordering SQL
  and the move logic used by all item types — the highest blast radius of the
  four candidates.

### Prior learnings (compound library, HIGH confidence)

The learnings-researcher surfaced directly-applicable precedent:

* **Reload from Markdown at re-persist seams** — DB-first loaders omit
  non-projected fields (`item_links`, provenance); stamping a new scalar such
  as `priority` or a dep edge at a **mutation** (re-persist) seam must reload via `findArtifact` (the `MoveInQueue`
  precedent). Note: this applies to mutation paths (AddDependency, MoveInQueue)
  where a DB-loaded artifact is re-persisted, NOT to creation paths
  (CreateArtifact) which build the in-memory artifact directly. Source: `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`.
* **SQLite JSON round-trips are type-lossy** — shipment membership and
  `queue_position` deserialize as `[]interface{}`/`float64`; normalize at every
  read edge. Source: `docs/compound/go-patterns/f015-shipment-stash-patterns.md`.
* **Enforce typed metadata in the core setter before schema resolution**, not
  only in the DB projection. Source:
  `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`.
* **`dep_type` collapses to `blocks` on Rehydrate**; the edge must round-trip
  through frontmatter (transactional `item_deps` rebuild). Sources:
  `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`,
  `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`.
* **CLI/MCP parity** — a new priority create surface must be wired to both CLI
  and MCP and locked with a parity test; every mutating write must re-upsert
  into SQLite+FTS. Sources:
  `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md`,
  `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`.
* **Go map-iteration nondeterminism** — test ordering with N independent pairs
  or a deterministic tie-breaker; priority ordering and dependency resolution
  are named in-scope. Source:
  `docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md`.

## Options Evaluated

### Option 1: Full sequencing suite (a) + (b) + (c)

Deliver shipment-scoped `queue_position`, the shipment&rarr;shipment blocking
affordance, and shipment priority together.

* **Pros**: Covers every listed gap in one pass.
* **Cons**: Candidate (a) rewrites shared, global ordering SQL and the move
  logic used by all item types, risking regression of task/feature queue
  ordering; the spike treats existing `queue_position` as already-sufficient
  manual order and does not ask for this change. High blast radius for a
  low-priority, optional enhancement.
* **Effort**: high. **Fit**: weak (over-scoped, exceeds spike recommendation).

### Option 2: Spike Option B primitives (b) + (c) — SELECTED

Deliver meaningful/settable shipment **priority** (secondary sort) and a
first-class, validated, documented shipment&rarr;shipment **blocking-order**
affordance over the existing `item_deps` mechanism, with regression tests
proving `queue view --type shipment --status queued` honors both and survives
`sync_index`.

* **Pros**: Exactly the two primitives spike `001-SP` names for Option B;
  together they form a *complete* sequencing surface (soft priority tie-break +
  hard blocking order); reuses existing plumbing (`WithPriority`, priority
  filter, `AddDependency`, `filterByResolvedDependencies`); low-moderate,
  single-domain, bounded blast radius; excludes the two risky/premature
  candidates.
* **Cons**: Does not add shipment-scoped manual positions; operators relying on
  `queue move` in a combined view still see the global-namespace behavior
  (acceptable — deferred as (a)).
* **Effort**: medium. **Fit**: strong (matches spike Option B precisely).

### Option 3: Priority only (c)

Deliver only shipment priority.

* **Pros**: Absolute minimum; fixes the single named Orchestrator selection
  gap.
* **Cons**: Leaves the spike's "use `item_deps` for true blocking order" half of
  Option B without a first-class affordance or regression guard, so the
  sequencing surface is incomplete and likely to require immediate follow-up
  rework. Priority alone gives a tie-break but no "A must ship before B"
  guarantee.
* **Effort**: low. **Fit**: partial (incomplete for the stated dark-factory
  ordering goal).

### Option 4: Add the (d) ship_sequence audit surface

Any option, plus building the non-authoritative `ship_sequence.jsonl` audit
plan now.

* **Pros**: Machine-queryable dark-mode activation rationale.
* **Cons**: Directly contradicts spike Option D and the stash's own gating
  ("build only after a real multi-shipment run shows activation evidence is
  insufficient"). No such evidence exists: the completed multi-shipment runs
  (113-S, 114-S, 115-S) closed successfully using activation/checkpoint
  evidence. Adds a new file, schema, and sync/rehydration surface. Fails YAGNI
  and scope-boundary review.
* **Effort**: high. **Fit**: rejected.

## Trade-off Comparison

| Criterion | Opt 1 (a+b+c) | Opt 2 (b+c) | Opt 3 (c) | Opt 4 (+d) |
|---|---|---|---|---|
| Spike 001-SP support | partial (adds unasked (a)) | strong (Option B) | partial | contradicts Option D |
| Blast radius | high (shared ordering SQL) | low-moderate | low | high (new file/schema) |
| Coherence / completeness | complete but over-scoped | complete for Option B | incomplete | over-scoped |
| Regression risk to task/feature queue | real | none | none | none |
| YAGNI / scope discipline | weak | strong | strong | fails |
| Effort | high | medium | low | high |

## Decision

**Selected: Option 2 — deliver (b) + (c) as one coherent backlogit release
unit; defer (a) and (d) explicitly.**

Rationale: Option 2 is the smallest scope that fully realizes spike `001-SP`'s
recommended Option B path. Priority (c) resolves the spike's named Orchestrator
"highest-priority queued shipment" gap and reuses existing priority plumbing.
The blocking affordance (b) makes the already-working `item_deps` mechanism a
first-class, validated, documented, and regression-guarded surface for
shipment&rarr;shipment hard ordering. Together they give dark-factory a
complete, low-risk sequencing surface via `queue view --type shipment --status
queued` without touching the shared global ordering SQL or introducing a new
authoritative scheduler.

The covering feature: **"Shipment sequencing primitives for dark-factory queue
ordering (spike 001-SP Option B)"**.

## Rejected Alternatives

* **(a) shipment-scoped `queue_position`** — deferred. Highest blast radius of
  the four; rewrites global ordering SQL (`buildQueueOrderClause`) and the move
  logic (`MoveInQueue`) shared by all item types, risking regression of
  task/feature ordering. The spike treats the existing `queue_position` as the
  durable manual order and does not require scoping it. Revisit as a dedicated
  release unit only if a real dark-factory run shows the global-namespace
  interleave materially harms shipment ordering.
* **(d) `ship_sequence.jsonl` audit surface** — deferred per spike Option D,
  the stash's own gating clause, and YAGNI. No completed multi-shipment
  dark-mode run has demonstrated that activation/checkpoint evidence is
  insufficient for resume or audit; 113-S/114-S/115-S closed successfully on
  activation evidence. Building it now would add a file/schema/sync surface with
  no demonstrated need.
* **Options 1, 3, 4** — rejected per the trade-off table above (over-scoped,
  incomplete, and scope-violating respectively).

Both deferred candidates remain captured: stash `0B5FA82B` records them, and
this deliberation plus the linked plan preserve the deferral rationale for a
future Stage session.

## Unresolved Questions

* Will dark-factory runs frequently hold more than one queued shipment at once?
  (Spike open question; still unquantified — argues for keeping scope minimal.)
* Should shipment priority default to a value at creation, or remain empty until
  set? (Planning question — resolved in the impl-plan; default should avoid
  changing existing empty-priority queue behavior for non-shipment items.)
* Should the blocking affordance expose a distinct shipment-sequencing verb, or
  document the existing `dep add <shipA> <shipB>` path with shipment-aware
  validation? (Planning question — resolved in the impl-plan.)

## Risks and Mitigations

* **DB-first re-persist drops non-projected fields** &rarr; at mutation
  (re-persist) seams such as `AddDependency`, reload from Markdown via
  `findArtifact` (the `MoveInQueue` precedent); never persist a DB-loaded
  artifact. This does NOT apply to the create-time priority surface: `CreateShipment` →
  `CreateArtifact` writes the in-memory artifact directly without any DB load.
* **SQLite JSON type-lossy round-trip** &rarr; reuse existing normalization at
  read edges; do not introduce new `custom_fields` reads without normalizing.
* **`dep_type` collapses to `blocks` on sync** &rarr; scope (b) to blocking
  order only; the edge must round-trip through frontmatter; do not rely on a
  non-`blocks` dep_type surviving sync.
* **CLI/MCP surface drift** &rarr; add the priority create surface to both CLI
  (`shipment create --priority`) and MCP (`create_shipment` priority param) and
  lock with a parity test; re-upsert on every mutating write.
* **Go map-iteration nondeterminism in ordering tests** &rarr; use N
  independent pairs or a deterministic tie-breaker for priority/blocking
  ordering tests.
* **Default-template / hardcoded-count regressions** &rarr; grep count
  assertions (`defaults_templates_test.go`, etc.) before adding any default.
* **Setter-vs-projection divergence** &rarr; enforce any priority validation in
  the core setter before schema resolution, not only in the DB projection.
* **Pinned-binary version skew** &rarr; the separately-installed release CLI
  used to operate `.backlogit/` will not reflect merged behavior until
  rebuilt/reinstalled; note this in closure so the new sequencing behavior is
  verified against the correct binary.
