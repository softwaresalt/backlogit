---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for settable shipment priority and a first-class shipment-to-shipment blocking-order affordance that make dark-factory multi-shipment queue ordering clean (spike 001-SP Option B, stash 0B5FA82B).'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-01-shipment-sequencing-primitives-plan.md
title: 'Shipment sequencing primitives for dark-factory queue ordering'
---

# Shipment sequencing primitives for dark-factory queue ordering

Source deliberation:
`docs/decisions/2026-08-01-shipment-sequencing-primitives-deliberation.md`
(decision: select candidates (b)+(c), defer (a)+(d)). Authoritative research
input: spike `001-SP` /
`docs/decisions/2026-07-29-ship-sequence-manifest-spike.md` (Option B).

## Problem Frame

Long-running dark-factory (P-017) runs order queued shipments through the
existing queue surface `queue view --type shipment --status queued`
(`internal/core/queue.go`). Two backlogit product gaps make that ordering
uneven:

* **(c) Shipment priority is not a first-class create-time input.** The queue
  secondary sort already understands `priority`
  (`buildQueueOrderClause`, `queue.go:196-207,237`), and `UpdateArtifact`
  already applies priority to any artifact including a shipment
  (`artifacts.go:441,491`, with `WithPriority` at `:57`). But `CreateShipment`
  (`internal/core/shipment.go:54`) takes no priority, the CLI `shipment create`
  command exposes only `--title`/`--items`
  (`internal/cli/shipment.go:88-118`), and the MCP `create_shipment` tool
  exposes only `title`/`items`. A shipment therefore enters the queue with an
  empty priority, leaving the Orchestrator "highest-priority queued shipment"
  selection rule (spike-named gap) under-specified.
* **(b) Shipment-to-shipment blocking order has no first-class affordance.**
  `item_deps` already supports generic edges via `AddDependency`, and
  `filterByResolvedDependencies` (`queue.go:438-495`) already suppresses a
  queued item until its blocker reaches a no-longer-blocking status. A
  shipment-to-shipment `blocks` edge therefore already works end-to-end. But
  there is no validated, documented affordance for "shipment A must ship before
  shipment B", no guard that both endpoints are shipments, and no regression
  test proving the edge survives `sync_index` (Rehydrate rebuilds every edge as
  `blocks` — `dependencies.go:15-27`).

This plan closes both gaps as one coherent, single-domain-per-task release unit
without touching the shared global ordering SQL (candidate (a), deferred) and
without adding a new authoritative scheduler (candidate (d), deferred).

### Success Criteria

* A shipment can be created with a priority through core, CLI, and MCP, and that
  priority drives the secondary sort in `queue view --type shipment --status queued`.
* A validated shipment-to-shipment blocking edge suppresses the dependent
  shipment in the queued-shipment view until the blocker reaches a terminal
  release status, and the edge survives `sync_index`.
* CLI and MCP surfaces stay at parity and are locked by a parity test.
* No regression to task/feature queue ordering; no schema migration.

### Scope Boundaries

* **In scope**: create-time shipment `priority` (core `CreateShipment` + CLI
  `--priority` + MCP `create_shipment` param, at parity, with the backlog
  registry mapping updated for agent discovery); a **new additive** core
  `AddShipmentBlock` affordance that validates both endpoints are shipments and
  delegates to the existing generic `item_deps` `blocks` mechanism; the
  priority-ordered read surface is the existing CLI `queue view --sort priority`
  (default); documentation.
* **Priority is lenient, not validated** — matching the existing lenient
  `UpdateArtifact` priority path. An empty or unrecognized priority sorts last
  (`buildQueueOrderClause` `ELSE 4`) with a deterministic `id ASC` tie-break. No
  new priority taxonomy or validator is introduced, and no create-vs-update
  asymmetry is created.
* **Out of scope** (deferred, captured in the deliberation/stash): candidate (a)
  shipment-scoped `queue_position` (global-ordering-SQL refactor, highest blast
  radius); candidate (d) `ship_sequence.jsonl` audit surface (spike Option D +
  YAGNI); any Orchestrator/policy/template hardening (external autoharness).
* **Deferred within this release unit** (new follow-up, captured in Decisions):
  MCP `get_queue` priority-sort **read** parity. The CLI `queue view` already
  defaults to the priority sort and is the spike-named Option B consumption
  surface; the MCP `handleGetQueue` handler currently ignores `sort_by`
  (defaults to `created_at`), and changing its default would touch a shared
  all-types read surface. This release unit keeps the priority read surface on
  the CLI and defers MCP read-sort parity as an explicit follow-up rather than
  silently expanding blast radius.

## Requirements Trace

| Requirement (source) | Implementation action | Unit |
|---|---|---|
| (c) Settable shipment priority at create time | Thread `priority` into `CreateShipment` via existing `WithPriority` (lenient; no new validator) | U1, U2 |
| (c) Priority reachable from CLI and MCP at parity | Add CLI `--priority` flag and MCP `create_shipment` priority param; denylist parity test | U1, U2 |
| (c) Priority is discoverable by agents | Update `.autoharness/backlog-registry.yaml` `create_shipment` mapping (param + `--priority` template) | U5 |
| (c) Priority drives shipment queue secondary sort | Regression test over CLI `queue view --type shipment --status queued --sort priority`, incl. empty-priority determinism | U1 |
| (b) First-class shipment-to-shipment blocking order | New additive `AddShipmentBlock` delegating to generic `item_deps` `blocks`; both-are-shipments guard on the new path only | U3, U4 |
| (b) Blocking edge suppresses dependent, correct direction | Queue-suppression test with pinned direction (dependent `depends_on` prerequisite) | U3, U4 |
| (b) Blocking edge survives sync | Round-trip regression test through frontmatter + Rehydrate | U3, U4 |
| Documentation of (b)+(c) and deferrals | Docs update incl. CLI read surface, pinned direction, and deferred MCP read-sort | U6 |
| Deferred (a),(d),MCP read-sort | Explicit non-goals; captured in deliberation, stash, and Decisions | — |

## Implementation Units

Each unit is test-first at the release-unit level: a tests-only unit lands a
failing (red) harness before the paired code-only unit implements to green
(width isolation — tests and production code are separate tasks per the
constitution's Task Granularity rule). Each unit is single-domain and scoped to
roughly two hours; the file/scenario counts below are the actual counts.

### U1 — Failing tests: settable shipment priority create surface + queue sort (tests)

* **Domain**: tests. **Posture**: test-first (red).
* **Changes**: Add failing tests covering (1) `CreateShipment` persists a
  supplied priority to Markdown frontmatter and the index; (2) CLI
  `shipment create --priority high` and MCP `create_shipment` priority param set
  it, and the two surfaces stay at parity (denylist parity assertion); (3) CLI
  `queue view --type shipment --status queued --sort priority` orders queued
  shipments by priority, **including at least one empty-priority shipment** that
  sorts last deterministically (empty and unknown priorities tie-break by
  `id ASC`).
* **Test-seam notes**: exercise the create/persist path through the package-level
  `persistArtifactWriteFn` seam and do **not** use `t.Parallel` in tests that
  override package-global seams; assert queue order with N independent pairs so
  Go map-iteration nondeterminism cannot mask a fixture bug (production ordering
  is deterministic SQL with an `id ASC` tie-break).
* **Files**: `internal/core/shipment_test.go`, `internal/cli/shipment_test.go`,
  `internal/mcp/tools_test.go` (parity).
* **Scenarios**: core-persist, CLI+MCP+parity, queue-sort (incl. empty-priority).
* **Milestone**: `go test ./...` compiles and the new tests fail for the expected
  (unimplemented) reasons.

### U2 — Implement settable shipment priority create surface (code)

* **Domain**: code. **Posture**: implement to green. **Depends on**: U1.
* **Changes**: Add a `priority` parameter to `CreateShipment`, threaded through
  the **existing** `WithPriority` create option into `CreateArtifact` (which
  builds the in-memory source-of-truth artifact and writes Markdown + index
  directly — **no** post-create Markdown reload is needed, because nothing is
  DB-loaded at create time). Priority is **lenient** (accepted as-is, matching
  `UpdateArtifact`; no new validator, no `ErrValidation` rejection). Wire the CLI
  `--priority` flag (`internal/cli/shipment.go`) and the MCP `create_shipment`
  priority param (`internal/mcp/tools.go`); re-upsert into SQLite+FTS via the
  existing create path.
* **Files**: `internal/core/shipment.go`, `internal/cli/shipment.go`,
  `internal/mcp/tools.go` (a cohesive create-surface parity change; reuses
  existing `WithPriority`, no new taxonomy; `internal/core/artifacts.go` is
  **read-only** here — `WithPriority` is invoked, not modified).
* **Functions**: `CreateShipment`, CLI create `RunE`, MCP `handleCreateShipment`.
* **Milestone**: U1 tests pass green; `queue view --sort priority` orders queued
  shipments by priority.

### U3 — Failing tests: shipment-to-shipment blocking affordance + sync round-trip (tests)

* **Domain**: tests. **Posture**: test-first (red). **Depends on**: U2 (serialize
  the shared CLI/MCP surface for a single active branch).
* **Changes**: Add failing tests covering (1) `AddShipmentBlock` creates a
  `blocks` edge between two shipments; (2) it rejects an edge whose endpoints are
  not **both** shipments, while the generic `AddDependency` path is unchanged and
  still accepts existing edge shapes (additive guard, not a restriction of the
  shared path); (3) `queue view --type shipment --status queued` suppresses the
  **dependent** shipment (the one whose `depends_on` names the prerequisite)
  until the **prerequisite** reaches a terminal release status, and the
  prerequisite itself is **not** suppressed — pinning direction "A must ship
  before B" ⇒ B `depends_on` A; (4) the edge round-trips through frontmatter and
  survives `sync_index` (Rehydrate rebuilds it as `blocks`).
* **Test-seam notes**: drive the dependency write through `persistArtifactWriteFn`
  (the `AddDependency` relocate=false path) and the `ErrWriteIndeterminate`
  two-class contract; no `t.Parallel` when overriding package-global seams.
* **Files**: `internal/core/dependencies_test.go`, `internal/core/queue_test.go`.
* **Scenarios**: add-edge, additive-endpoint-validation, directional
  queue-suppression, sync-survival.
* **Milestone**: tests compile and fail for the expected reasons.

### U4 — Implement shipment-to-shipment blocking affordance (code)

* **Domain**: code. **Posture**: implement to green. **Depends on**: U3.
* **Changes**: Add a **new additive** core operation
  `AddShipmentBlock(dependentID, prerequisiteID)` that validates both endpoints
  resolve to `artifact_type: shipment` and delegates to the existing generic
  dependency mechanism with `blocks` (reusing `AddDependency`'s `findArtifact`
  reload + `ErrWriteIndeterminate` two-class contract, which already avoid
  dropping non-projected fields). The generic `AddDependency` path and
  `filterByResolvedDependencies` ordering semantics are **unchanged** — no
  previously-valid edge (e.g. shipment→task) is newly rejected. Suppression is
  **read-time** via `filterByResolvedDependencies`, which sets no
  `blocked_reason`, so **no derived/blocked-metadata clearing is performed**.
  Surface the affordance through the existing dependency path on both CLI and MCP,
  and update the MCP `backlogit_add_dependency` tool description to advertise
  shipment-to-shipment sequencing.
* **Files**: `internal/core/dependencies.go` (new `AddShipmentBlock` guard),
  `internal/mcp/tools.go` (surface + tool description). CLI reuses the existing
  `dep add` command.
* **Functions**: `AddShipmentBlock`, its MCP surface hook.
* **Milestone**: U3 tests pass green.

### U5 — Registry mapping for agent discovery (config)

* **Domain**: config. **Posture**: config-only. **Depends on**: U2.
* **Changes**: Update `.autoharness/backlog-registry.yaml` so the
  `create_shipment` operation advertises the new `priority` param and its CLI
  `--priority {{priority}}` template, so `export_command_map` and agent
  discovery reflect the surface added in U2.
* **Files**: `.autoharness/backlog-registry.yaml`.
* **Milestone**: `export_command_map` includes the `priority` param for
  `create_shipment`.

### U6 — Document shipment priority and blocking order (docs)

* **Domain**: docs. **Posture**: docs-only. **Depends on**: U2, U4, U5.
* **Changes**: Document settable shipment priority (create-time via core/CLI/MCP)
  and that the priority-ordered read surface is CLI `queue view --sort priority`
  (default); document the shipment-to-shipment blocking-order affordance with the
  **pinned direction** (dependent `depends_on` prerequisite; "A before B" ⇒ B
  `depends_on` A); record the **deferred** MCP `get_queue` read-sort parity and
  candidates (a)/(d) as explicit follow-ups. Update any command/tool reference
  that enumerates `shipment create` flags or `create_shipment` params.
* **Files**: the relevant `docs/**` reference pages (single docs domain).
* **Milestone**: `backlogit docs lint` passes; docs describe both behaviors and
  the deferrals.

## Dependency Graph

```text
U1 (tests: priority) ─▶ U2 (code: priority) ─┬─▶ U3 (tests: blocking) ─▶ U4 (code: blocking) ─┐
                                             └─▶ U5 (config: registry) ──────────────────────┴─▶ U6 (docs)
```

* U2 depends on U1 (code needs the failing harness).
* U3 depends on U2 (serialize the shared CLI/MCP `shipment create` surface edits;
  keep the release unit a clean build order for a single active branch).
* U4 depends on U3 (code needs the failing harness).
* U5 depends on U2 (the registry map reflects the create surface added in U2).
* U6 depends on U2, U4, and U5 (documents both implemented behaviors and the
  registry/read surfaces).
* Acyclic. Ships as one shipment, parent (feature) first, then U1→U6 in order.

## Decisions and Rationale

* **Reuse existing priority plumbing; keep priority lenient** — `WithPriority`,
  the priority sort in `buildQueueOrderClause`, and the `list --priority` filter
  already exist; the only true gap is the create-time surface. Priority is
  accepted leniently (matching `UpdateArtifact`); no priority-taxonomy validator
  exists today (`buildQueueOrderClause` maps unknown→`ELSE 4`, never rejects), so
  introducing create-time rejection would add a new taxonomy **and** a
  create-vs-update asymmetry. An empty/unknown priority sorts last with an
  `id ASC` tie-break. Minimizes blast radius and honors Single Responsibility.
* **Model (b) as a new additive `AddShipmentBlock`, not a restriction of the
  generic path** — a "both-endpoints-are-shipments" guard placed on the shared
  `AddDependency` path would newly reject previously-valid shipment→non-shipment
  edges (backward-incompatible). Instead, a narrow new core operation validates
  the shipment-to-shipment case and delegates to the generic `blocks` mechanism;
  the generic path and `filterByResolvedDependencies` stay byte-for-byte
  unchanged. This is genuinely additive.
* **Scope (b) to blocking order only** — because `dep_type` collapses to
  `blocks` on Rehydrate (`dependencies.go:15-27`), a non-`blocks` sequencing
  semantic would not survive sync. Blocking order is exactly what dark-factory
  needs and already round-trips.
* **Pin blocking direction** — `filterByResolvedDependencies` suppresses the edge
  **source** (`item_id`) while its **target** (`depends_on`) is non-terminal. So
  "shipment A must ship before shipment B" is the data edge **B `depends_on` A**.
  The affordance, tests, and docs all state dependent→prerequisite explicitly to
  prevent inversion.
* **No Markdown reload at create time for (c)** — `CreateShipment`→
  `CreateArtifact` writes the in-memory source-of-truth artifact directly and
  never DB-loads it, so the `MoveInQueue` reload precedent does not apply to
  create-time field injection (it applies to mutating a DB-loaded artifact).
  Threading `WithPriority` is sufficient; the reload precedent is retained for
  (b), where `AddDependency` already performs it.
* **Priority read surface is the CLI `queue view`; defer MCP read-sort parity** —
  the CLI already defaults to the priority sort and is the spike-named Option B
  consumption surface. `handleGetQueue` ignores `sort_by` (defaults to
  `created_at`); changing its default touches a shared all-types read surface, so
  MCP read-sort parity is deferred as an explicit follow-up rather than expanding
  blast radius here.
* **Defer (a) and (d)** — (a) rewrites shared global ordering SQL (highest blast
  radius, not spike-required); (d) contradicts spike Option D and lacks
  multi-shipment evidence of insufficient activation evidence (YAGNI). Both stay
  captured in the deliberation and stash for a future Stage session.

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| DB-first re-persist drops `item_links`/provenance | Applies to (b): `AddShipmentBlock` reuses `AddDependency`'s `findArtifact` reload. (c) create-time needs no reload (nothing is DB-loaded). |
| SQLite JSON round-trip is type-lossy (`[]interface{}`/`float64`) | Reuse existing normalization at read edges; add no un-normalized `custom_fields` reads |
| `dep_type` collapses to `blocks` on Rehydrate | Scope (b) to blocking order; round-trip the edge through frontmatter; assert survival in U3 |
| Backward-incompatible dependency restriction | Guard lives only on the new `AddShipmentBlock` path; generic `AddDependency` and `filterByResolvedDependencies` stay byte-for-byte unchanged; assert in U3 |
| Blocking direction inverted in code/docs | Pin dependent `depends_on` prerequisite; assert directional suppression (dependent suppressed, prerequisite not) in U3 |
| CLI/MCP surface drift | Wire both surfaces in U2/U4; lock create parity with a denylist parity test; update the registry map (U5); re-upsert on every write |
| MCP `get_queue` does not sort by priority | Read surface is CLI `queue view --sort priority`; MCP read-sort parity is an explicit deferred follow-up (documented in U6) |
| Dependency suppression runs after SQL `LIMIT`/`OFFSET` | The sequencing contract is defined over the **unpaginated** queued-shipment view (default: no limit); U3 tests the unpaginated view and the docs note the `LIMIT` interaction |
| Go map-iteration nondeterminism in ordering tests | Use N independent pairs / deterministic tie-breaker in U1/U3 |
| Default-template hardcoded-count regressions | Grep count assertions before adding any default; priority stays empty-until-set |
| Pinned-binary version skew | The separately installed release CLI operating `.backlogit/` will not reflect merged behavior until rebuilt/reinstalled; verify against the rebuilt binary (see `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`, `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`) |
| Changing empty-priority queue behavior for non-shipment items | Keep priority empty-until-set; add no implicit default that reorders existing task/feature queues |

## Constitution Check

* **I. Safety-First Go** — pass. Production code stays in Go; errors wrap with
  `%w`; sentinel/typed errors for validation; no `unsafe`.
* **II. Test-First Development (NON-NEGOTIABLE)** — pass. U1 and U3 land failing
  harnesses before U2 and U4 implement; tests and production code are separate
  tasks (width isolation).
* **III. Workspace Isolation** — pass. All file operations resolve within the
  workspace root; no secrets committed.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)** — pass. No writes outside
  the working tree.
* **V. Structured Observability** — pass. Conventional commits and test output
  trace each change.
* **VI. Single Responsibility** — pass. No new dependencies; reuses existing
  priority and `item_deps` plumbing.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)** — N/A. No destructive
  commands; no schema migration.
* **VIII. Explicit Safety Modes** — pass. The plan adopts a
  freeze-scope/careful posture: the Plan Hardening section's Protected Invariants
  bound the edits, and risky Ship-side actions are classified as
  `ProposedAction`/`ActionRisk`/`ActionResult`.
* **IX. Git-Friendly Persistence** — pass. Priority and edges round-trip through
  Markdown frontmatter.
* **X. Agent Context Efficiency** — pass. No bulk-data surfaces added.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)** — pass. Ship merges
  via merge commit.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change** — present. Adds a CLI `--priority`
  flag, an MCP `create_shipment` priority param, and a shipment-to-shipment
  blocking affordance (additive, backward-compatible; no schema migration).
* **security, auth, permission, or compliance-sensitive behavior** — absent.
* **migration, backfill, destructive data/config action, or irreversible step**
  — absent. `priority` column and `item_deps` already exist; no migration.
* **external integration, operator checkpoint, or external dependency** — absent.
* **high runtime, rollout, or rollback risk** — present (minor). Touches the core
  queue-ordering and dependency-resolution runtime surface
  (`buildQueueOrderClause`, `filterByResolvedDependencies`); bounded to shipment
  scope, backward-compatible, and revertible via ordinary Git.

Requires plan hardening: yes

## Runtime Verification and Closure

Runtime surfaces changed: the `backlogit shipment create` CLI command, the MCP
`create_shipment` tool, and the `queue view --type shipment --status queued`
ordering/suppression behavior. Ship executes runtime verification and closure
after build; this plan seeds the expectations:

* **Runtime verification** (Ship): create two queued shipments with distinct
  priorities and confirm CLI `queue view --type shipment --status queued
  --sort priority` orders by priority; add a shipment-to-shipment blocking edge
  (dependent `depends_on` prerequisite) and confirm the dependent shipment is
  suppressed until the prerequisite reaches a terminal status; run `sync_index`
  and confirm the edge and priority survive.
* **Version-skew closure note**: verify against the freshly rebuilt binary, not
  the pinned release CLI, and record whether the operating `.backlogit/` binary
  needs reinstalling to exercise the new behavior.
* **Operational closure artifacts** (Ship): validation window and ownership for
  the queue-ordering change; rollback trigger = any regression in task/feature
  queue ordering, reverted via Git (no data migration to unwind).

Because a hardening signal is present, this plan proceeds to the `plan-harden`
step (P-006) before `plan-review`.

## Plan Hardening

**Hardening required: yes.** Triggered by the public API/contract change signal
(new CLI `--priority` flag, MCP `create_shipment` priority param, first-class
shipment-to-shipment blocking affordance) and the minor high-runtime-risk signal
(edits land on the core queue-ordering and dependency-resolution surface). This
record is produced by the Stage agent under ACTIVE DARK FACTORY MODE (P-017);
the risky actions below are **Ship-side implementation actions** classified now
so they carry forward into build, review, runtime verification, and closure.

### Protected Invariants (must not regress)

* **Task/feature queue ordering is unchanged.** Only shipment ordering gains a
  well-defined priority secondary sort; `buildQueueOrderClause` global behavior
  for non-shipment types stays byte-for-byte equivalent, and priority stays
  empty-until-set (no implicit default that reorders existing queues).
* **The generic `AddDependency` path stays byte-for-byte unchanged.** The
  shipment-to-shipment guard lives only on the new additive `AddShipmentBlock`
  operation; no previously-valid edge (e.g. shipment→task) is newly rejected, and
  `filterByResolvedDependencies` ordering/suppression semantics are untouched.
* **Blocking direction is pinned dependent→prerequisite.** "A must ship before B"
  is the edge B `depends_on` A; the dependent (source) is suppressed while the
  prerequisite (target) is non-terminal, and the prerequisite is never suppressed.
* **No blocked-metadata clearing is introduced.** Suppression is read-time via
  `filterByResolvedDependencies` and sets no `blocked_reason`; the affordance
  writes no derived/blocked fields, so there is nothing to clear.
* **`item_deps` edges round-trip through frontmatter and survive Rehydrate as
  `blocks`.** The blocking affordance must not depend on a non-`blocks`
  `dep_type` surviving `sync_index` (`dependencies.go:15-27`).
* **No non-projected fields are dropped** at the (b) edge re-persist seam
  (`item_links`, provenance, membership) — `AddShipmentBlock` reuses
  `AddDependency`'s reload-from-Markdown. (c) create-time performs no DB load, so
  no reload is required there.
* **CLI and MCP stay at parity** for the new create surface (denylist parity
  lock), and every mutating write re-upserts into SQLite+FTS.
* **No schema migration**: the `priority` column and `item_deps` table already
  exist; the change is additive and backward-compatible.

### Learnings and Instructions Consulted

* `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`
  — reload from Markdown at re-persist seams (MoveInQueue precedent).
* `docs/compound/go-patterns/f015-shipment-stash-patterns.md` — SQLite JSON
  round-trips are type-lossy; normalize at read edges.
* `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`
  — governs typed `custom_fields` metadata. Priority is a **lenient scalar**
  consistent with `UpdateArtifact` (no validator exists today); a create-only
  validator would create the create-vs-update asymmetry the Go review flagged, so
  this plan deliberately does **not** add one.
* `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`
  — stale-`blocked` clearing must happen at **all** choke points; this plan sets
  no `blocked` metadata (suppression is read-time), so the clearing obligation
  does not apply — confirming the "clear blocked metadata" clause was removed.
* `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md` — drive durable
  writes through the `persistArtifactWriteFn` seam without `t.Parallel` when
  overriding package globals.
* `docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md`
  — treat empty priority as "unset" (sorts last via `ELSE 4`), not as a distinct
  sentinel bucket; assert the empty-priority determinism in U1.
* `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
  and `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
  — transactional `item_deps` rebuild; commit-then-surface two-class contract.
* `docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md` and
  `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`
  — CLI/MCP parity via DI + parity test + index consistency.
* `docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md`
  — N-independent-pair ordering tests to defeat Go map-iteration nondeterminism.
* Instructions: `technology.instructions.md` / `go.instructions.md` (error
  wrapping, no `unsafe`), `go-mcp-server.instructions.md` (MCP tool contract),
  `constitution.instructions.md` (Test-First, Merge Commit).

### Risky Actions (carry forward to Ship)

* **ProposedAction PA-1** — thread `priority` through `CreateShipment` via the
  existing `WithPriority` create option (lenient; no new validator).
  * targets: `internal/core/shipment.go`, shipment Markdown + index.
    (`internal/core/artifacts.go` `WithPriority` is **invoked, read-only**, not
    modified.)
  * change_kind: local code edit (additive contract surface).
  * rollback: revert commit; no data migration to unwind.
  * approval_required: no (non-destructive, bounded).
  * **ActionRisk: moderate** — changes a create contract on a core artifact and
    the queue secondary sort input. **ActionResult: planned.**
* **ProposedAction PA-2** — wire CLI `--priority` and MCP `create_shipment`
  priority param at parity.
  * targets: `internal/cli/shipment.go`, `internal/mcp/tools.go`, parity test.
  * change_kind: local code edit (public CLI/MCP surface addition).
  * rollback: revert commit.
  * approval_required: no.
  * **ActionRisk: moderate** — public surface addition; parity risk.
    **ActionResult: planned.**
* **ProposedAction PA-3** — add the **new additive** `AddShipmentBlock` core
  operation over `item_deps` `blocks` (both-are-shipments guard on the new path
  only), surfaced through the existing CLI/MCP dependency path, with a
  sync-surviving round-trip.
  * targets: `internal/core/dependencies.go` (new `AddShipmentBlock`),
    `internal/mcp/tools.go` (surface + `add_dependency` tool-description update).
    `internal/core/queue.go` is **read-only** (no ordering-semantic change); the
    generic `AddDependency` path is unchanged.
  * change_kind: local code edit (additive affordance; no restriction of the
    shared dependency path; no blocked-metadata clearing).
  * rollback: revert commit; edges are re-derived from frontmatter on sync.
  * approval_required: no.
  * **ActionRisk: moderate** — touches dependency resolution and Rehydrate
    round-trip. **ActionResult: planned.**

No `ActionRisk: destructive` or `high` action exists in this release unit; no
operator approval is required for the implementation actions. Stage takes none
of these actions (Role Boundary: Stage does not write source/tests/config or run
builds).

### Deepened Verification and Rollback (for Ship)

* **Environment precheck**: build the binary from source before runtime
  verification; do **not** verify against the pinned release CLI (version-skew
  caveat) — the operating `.backlogit/` binary must be rebuilt/reinstalled to
  exercise the new behavior.
* **Target scenarios**: (1) two queued shipments with distinct priorities →
  assert CLI `queue view --type shipment --status queued --sort priority` order
  (incl. an empty-priority shipment sorting last); (2) blocking edge B
  `depends_on` A between two shipments → assert the **dependent** B is suppressed
  until the **prerequisite** A is terminal, and A is never suppressed; (3)
  `sync_index` → assert priority and the `blocks` edge survive; (4) create a
  queued feature/task alongside → assert non-shipment queue order is unchanged
  (regression guard for the protected invariant); (5) generic `AddDependency`
  with a non-shipment endpoint still succeeds (additive-guard regression).
* **Blocked-path handling**: priority is **lenient** — an empty/unrecognized
  value is not rejected; it sorts last (`ELSE 4`) with an `id ASC` tie-break. The
  commit-then-surface two-class contract (`ErrWriteIndeterminate`) applies to the
  (b) dependency write: on an indeterminate write the surface reports the
  indeterminate error and the index is reconciled on the next `sync_index`.
* **Rollback trigger**: any observed change to task/feature queue ordering, or
  an edge/priority not surviving `sync_index`. **Rollback procedure**: revert
  the release-unit merge commit (Git-only; no migration to reverse). **Owner**:
  Ship agent during the post-merge validation window.

### Unresolved Operator Decisions

None block safe execution. Two planning-level questions are resolved here for
Ship: (1) shipment priority stays **empty-until-set** (no create-time default)
and **lenient** (no create-time validator), preserving existing queue behavior
and avoiding a create-vs-update asymmetry; (2) the blocking affordance is a
narrow new additive core operation (`AddShipmentBlock`) that delegates to the
generic `blocks` mechanism and is surfaced through the **existing** CLI/MCP
dependency path (no new user-facing verb), keeping the change additive and the
shared path unchanged.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** (task-tool subagents available).
* **review-fix cycle: 1 of 3** (circuit-breaker limit 3). One revision cycle was
  applied in-band to resolve every P0/P1 and the load-bearing P2 findings; the
  plan body above reflects the resolved state.
* **decision: PASS.**

### Persona Trigger Evaluation

Six personas were triggered; one was evaluated and deliberately not triggered:

* **Constitution Reviewer** — triggered (every plan).
* **Go Reviewer** — triggered (Go production-code contract change).
* **Scope Boundary Auditor** — triggered (candidate-set scoping, YAGNI on (a)/(d)).
* **Learnings Researcher** — triggered (durable-write/parity/seam learnings apply).
* **Architecture Strategist** — triggered (queue-ordering + dependency-resolution
  runtime surface).
* **Agent-Native Parity Reviewer** — triggered (CLI/MCP/registry agent surface).
* **Security Lens Reviewer** — **not triggered.** The change adds no auth/authz,
  touches no secrets or credentials, processes no untrusted external input, and
  crosses no new trust boundary (all edits are local `.backlogit/` state and Go
  surfaces). No security-sensitive attack surface exists to review.

### Findings and Resolutions

**P0** — none.

**P1 (blocking; all validated against the code and resolved in this cycle):**

* **Go F1 — phantom priority validator / create-vs-update asymmetry.** The prior
  plan validated priority and rejected with `ErrValidation`, but no priority
  validator exists (`ValidateArtifactFields` checks presence only;
  `buildQueueOrderClause` maps unknown→`ELSE 4`) and `UpdateArtifact` is lenient.
  **Resolved**: priority is now lenient end-to-end (U2, Decisions, PA-1); empty/
  unknown sorts last with an `id ASC` tie-break (U1 asserts determinism).
* **Go F2 — backward-incompatible dependency guard mislabeled "additive."** A
  both-endpoints-are-shipments guard on the shared `AddDependency` path would
  reject existing shipment→task edges. **Resolved**: reframed as a new additive
  `AddShipmentBlock` core operation; the generic path and
  `filterByResolvedDependencies` stay byte-for-byte unchanged (U3/U4, Decisions,
  PA-3, Protected Invariants; U3 adds an additive-guard regression scenario).
* **Learnings P1 — unnecessary/under-specified "clear blocked metadata" clause.**
  Suppression is read-time via `filterByResolvedDependencies` and sets no
  `blocked_reason`. **Resolved**: the blocked-metadata-clearing clause was
  removed; a Protected Invariant now states no blocked metadata is written, and
  the stale-blocked-clearing learning is cited to confirm non-applicability.
* **Agent-Native Parity P1 — missing registry mapping for `create_shipment`
  priority.** Agents discover surfaces via `.autoharness/backlog-registry.yaml`.
  **Resolved**: added U5 (config) to update the `create_shipment` mapping with
  the `priority` param + `--priority` template, wired into the dependency graph.

**P2 (resolved or explicitly accepted):**

* **Go F3 / Architecture — reload-from-Markdown misapplied to (c) create.**
  `CreateArtifact` never DB-loads. **Resolved**: dropped the reload for (c);
  retained it for (b) where `AddDependency` already performs it.
* **Go F4 — unpinned edge direction.** **Resolved**: direction pinned dependent
  `depends_on` prerequisite across Decisions, U3/U4, Protected Invariants, and
  verification scenarios.
* **Go F5 — no `ErrValidation` negative test.** **Resolved** by going lenient (no
  such path to test).
* **Architecture — MCP `get_queue` read-parity gap.** Confirmed `handleGetQueue`
  ignores `sort_by`. **Resolved by explicit deferral**: the read surface is CLI
  `queue view --sort priority`; MCP read-sort parity is a documented follow-up
  (Scope Boundaries, Decisions, U6, Risks) rather than a silent expansion.
* **Architecture — pagination/dependency-filter interaction.** Confirmed
  suppression runs after SQL `LIMIT`/`OFFSET`. **Accepted with caveat**: the
  sequencing contract is defined over the unpaginated view (default no limit);
  Risks + U3 note it.
* **Learnings P2 — durable-write test seam + empty-priority determinism.**
  **Resolved**: U1/U3 test-seam notes (`persistArtifactWriteFn`, no `t.Parallel`)
  and empty-priority determinism added; two additional learnings cited.

**P3 (accepted; addressed where cheap):**

* Constitution VIII relabeled N/A→pass (freeze-scope/careful posture).
* Requirements Trace gained a docs (U6) row; granularity annotations replaced
  count-guesses with actual counts.
* `artifacts.go` clarified as read-only in PA-1 targets.
* Version-skew citations added to Risks; MCP `add_dependency` tool-description
  update folded into U4.

### Remaining Follow-ups (accepted, deferred as backlog/documentation)

* MCP `get_queue` priority-sort read parity (documented deferral, candidate for a
  future release unit).
* Deferred candidates (a) shipment-scoped `queue_position` and (d)
  `ship_sequence.jsonl` audit surface remain captured in the deliberation and the
  stash for a future Stage session.

decision: PASS
