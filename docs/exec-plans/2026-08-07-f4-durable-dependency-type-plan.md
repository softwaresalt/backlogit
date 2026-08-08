---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for formal-gate unit F4: promote dependency edge type into markdown frontmatter as typed DependencyEdge objects so dep_type survives rehydration, with a fixed serialization contract and CLI/MCP read parity.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-07-f4-durable-dependency-type-plan.md
title: 'F4 — durable dependency type persistence'
---

# F4 — durable dependency type persistence

Source: `docs/decisions/2026-07-17-formal-gate-architecture-spike-findings.md`
(Q5, follow-up F4: "typed dependency objects in frontmatter so `dep_type`
survives rehydration").

<!-- plan-review-attempt: 2 -->

## Problem Frame

`item_deps(item_id, depends_on, dep_type)` exists in SQLite
(`internal/db/schema.go:262-269`) and every read and write carries `dep_type`
(`internal/db/dependencies.go:29-35,51-70,120-127,210-219`). But that table is a
**disposable projection**. `Rehydrate` deletes every row
(`internal/db/rehydration.go:168`) and rebuilds edges from
`artifact.Dependencies` (`:217-220`), a bare `[]string`
(`internal/models/artifact.go:46`). The rebuild has no type to restore, so it
writes the `blocks` default.

Consequence: **after any `sync`/rehydrate or out-of-band branch change, every
`relates_to` and `parent_of` edge silently collapses to `blocks`.**
`core.RemoveDependency` compounds this by recovering the type only from the
SQLite cache (`internal/core/dependencies.go:165-176`).

### Success Criteria

* A `relates_to` or `parent_of` edge survives `backlogit sync` unchanged.
* Frontmatter is the source of truth for edge type; SQLite is repopulated from it.
* Artifacts whose `dependencies` is a plain ID list keep loading, keep meaning
  `blocks`, and re-serialize **byte-identically**.
* No frontmatter key is dropped by the typed write path.
* An invalid edge type is rejected at the **load edge**, not merely at persist.
* CLI and MCP present **one canonical dependency representation**, and both
  expose the edge type.

### Scope Boundaries

**In scope:** the `DependencyEdge` model type and its fixed YAML shape; the
normalization and serialization seams; load-edge validation; rehydration reading
the durable type; `AddDependency`/`RemoveDependency`; the canonical read
representation across `get_item`/`list_items`/`get_dependencies` and CLI
`dep list`; registry and MCP schema alignment; documentation.

**Out of scope:** new `dep_type` values (the set stays `blocks`, `relates_to`,
`parent_of`). Queue resolution semantics. `links`, which are already objects.
Any change to `item_links`. Back-migrating existing artifacts — the contract is
forward-only and accepts both shapes permanently.

## Requirements Trace

| Requirement | Source | Unit |
|---|---|---|
| `dep_type` durable in markdown | Spike Q5 | U2, U3, U4 |
| Explicit Go type and fixed YAML key names | Review F4-01/F4-02 (Go) | U2 |
| Validation at the load edge, not only at persist | Review F4 (Architecture) | U2 |
| No frontmatter key dropped | `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` | U3 |
| Every `.Dependencies` call site migrated | Review F4-01 (Go) | U3 |
| Rehydration restores the durable type in one transaction | `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md` | U4 |
| One canonical read representation across surfaces | Review F4-1 (parity) | U6 |
| Registry row, MCP schema, and dependency-vs-link disambiguation | Review F4-2 (parity) | U6, U7 |
| Prove the defect red-first | `docs/decisions/2026-07-14-docline-soft-key-regression-decision.md` | U1 |

## Implementation Units

### U1 — Failing characterization: `dep_type` collapses across rehydrate (tests)

Red integration test: create a `relates_to` edge, run `Rehydrate`, assert the
type is still `relates_to`. Must be **observed failing at HEAD**. Also capture a
golden fixture of a current artifact's serialized frontmatter so U3 can assert no
key is dropped.

Files: `internal/db/rehydration_deptype_test.go`.
Scenarios: `relates_to` survives sync; `parent_of` survives sync; bare-string
dependencies still load.
Posture: characterization-first (RED).

### U2 — `DependencyEdge` model type, normalizer, and load-edge validation (code)

Declare `type DependencyEdge struct { ID string; Type string }` in
`internal/models`. Change `Artifact.Dependencies` from `[]string` to
`[]DependencyEdge`. Add `toDependencyEdges(v any) ([]DependencyEdge, error)` in
`internal/models/frontmatter.go`, mirroring the existing `toArtifactLinks`
manual-normalization precedent (`:161-195`) — **no custom `UnmarshalYAML`**,
because the repository has no such precedent and frontmatter is decoded as
`map[string]any`.

The normalizer accepts a list whose entries are either a plain string (meaning
`{ID: s, Type: "blocks"}`) or a map. **Fixed YAML shape** (this is a
machine-consumed contract and is fixed here, not at implementation time):

```yaml
dependencies:
  - 106.001-T                       # legacy / default: blocks
  - {id: 106.002-T, type: relates_to}
```

Validate `Type` against `blocks | relates_to | parent_of` **at the load edge**
and return `errors.ErrInvalidDependencyType` (in
`internal/errors/dependency_errors.go`) so an invalid value can never enter the
system even if later units are delayed.

Files: `internal/models/artifact.go`, `internal/models/frontmatter.go`,
`internal/errors/dependency_errors.go`.
Scenarios: string entry normalizes to `blocks`; object entry preserves type;
mixed list normalizes; unknown type rejected at load.
Posture: test-first.

### U3 — Serialization and call-site migration (code)

Add `dependencies` to `WriteArtifactFile`'s explicit key enumeration
(`internal/core/artifacts.go:681-725`) — an unenumerated key is silently dropped
on every typed round-trip. Emit the object shape **only when at least one edge
has a non-default type**, so untyped artifacts stay byte-stable.

Migrate every call site that treats `Dependencies` as `[]string`. The enumerated
set is: `internal/models/artifact.go:89-91,107-112`;
`internal/models/frontmatter.go:110-112`; `internal/core/artifacts.go:87,503`;
`internal/core/dependencies.go:39-46,98-111`; `internal/core/shipment.go:633`;
`internal/core/artifact_references.go:119`; `internal/cli/migrate.go:321-329`;
`internal/db/rehydration.go:217-220`.

Files: `internal/core/artifacts.go`, `internal/core/artifact_references.go`,
`internal/core/shipment.go`.
Scenarios: untyped round-trip byte-identical to the U1 golden fixture; typed
round-trip preserves type; empty dependency list serializes consistently
(assert the **empty** case explicitly); no golden key dropped.
Posture: test-first.

### U4 — Rehydration and core add/remove read the durable type (code)

`Rehydrate` upserts each edge with the type parsed from frontmatter instead of
the hardcoded default (`internal/db/rehydration.go:217-220`), staying inside the
single existing `*sql.Tx`; no `BestEffort` `*sql.DB` variant is introduced.
`AddDependency` writes the type into frontmatter; `RemoveDependency` reads it
from frontmatter and no longer depends on `lookupDependencyType`'s cache read.
Both rollback branches, including the `ErrWriteIndeterminate` special case, keep
their current structure.

Files: `internal/db/rehydration.go`, `internal/core/dependencies.go`.
Scenarios: U1's red tests turn green; add persists type to frontmatter; remove
recovers type from frontmatter; indeterminate-write path unchanged.
Posture: test-first.

### U5 — CLI dependency surface (code)

`internal/cli/dep.go` (`NewDepAddCmd`, `NewDepRemoveCmd`, `NewDepListCmd`)
presents the canonical representation: `dep list` emits `{id, type}` per edge
rather than a bare ID list.

Files: `internal/cli/dep.go`.
Scenarios: output snapshots for `blocks`, `relates_to`, `parent_of`, empty, and
mixed edge sets.
Posture: test-first.

### U6 — MCP read contract and canonical representation parity (code)

Fix the canonical dependency representation for agent reads and make it
consistent across `get_item`, `list_items`, `get_dependencies`, and
`add_dependency`/`remove_dependency`. Update the MCP tool schemas so `dep_type`
is expressed and returned, and preserve or update the existing registry row so
the CLI fallback stays honest. Add a parity contract test asserting CLI and MCP
return the same edge set with the same types.

Files: `internal/mcp/tools.go`, `.autoharness/backlog-registry.yaml`.
Scenarios: MCP response schema includes the type; CLI/MCP parity across all
three types; empty set is `[]`, never null, on both surfaces.
Posture: test-first.

### U7 — Document the dependency-type contract and the dependency/link distinction (docs)

Document the accepted frontmatter shapes, the exact YAML keys, the forward-only
compatibility rule, the default (`blocks`), and — explicitly, for agent consumers
— the distinction between a **dependency** type `relates_to` (execution-blocking
family) and a **link** type `related_to` (informational). Update the backlogit
SQL-schema and YAML-header instruction references and the MCP tool descriptions
so an agent reading the tool surface alone can tell them apart.

Files: `docs/design-docs/dependency-type-durability.md`,
`.github/instructions/backlogit-yaml-header-tooling.instructions.md`,
`.github/instructions/backlogit-sql-schema.instructions.md`.
Posture: documentation.

## Dependency Graph

```text
U1 ──> U2 ──> U3 ──> U4 ──> U5 ──> U6 ──> U7
```

Strictly sequential. `U2` deliberately carries load-edge validation so invalid
state cannot enter even if later units are delayed. `U7` last.

## Decisions and Rationale

* **Polymorphic list over a parallel type map** — a parallel map drifts out of
  sync with the ID list and doubles the merge-conflict surface.
* **Manual normalizer over a custom `UnmarshalYAML`** — frontmatter is decoded as
  `map[string]any`; the repository's precedent (`toArtifactLinks`) is manual
  normalization, and introducing the first custom unmarshaller here would be an
  unnecessary new pattern.
* **Fixed `{id, type}` keys** — machine-consumed field names are a contract and
  are fixed at plan time, consistent with `links`' `{target_id, link_type}`.
* **Emit the typed shape only when needed** — the vast majority of edges are
  `blocks`; unconditional object emission would rewrite the whole corpus for no
  semantic gain.
* **Validate at the load edge** — a projection guard controls only the `items`
  table; the frontmatter file is the source of truth.
* **Forward-only, no back-migration** — both shapes accepted permanently; a bare
  string always means `blocks`.

## Risks and Caveats

| Risk | Severity | Mitigation |
|---|---|---|
| Typed field dropped by the `WriteArtifactFile` enumeration | **high** | Enumeration updated in U3 with a golden-fixture assertion |
| A missed `[]string` call site fails to compile or silently narrows | **high** | U3 enumerates every call site explicitly by file and line |
| SQLite JSON round-trip returns `[]any` of `map[string]any` | medium | Normalize at the read edge; explicit decode test |
| `omitempty` elides an empty typed list | medium | Assert the empty case on both frontmatter and JSON surfaces |
| Agents receive different shapes from different surfaces | **high** | U6 fixes one canonical representation with a parity contract test |
| `relates_to` (dependency) confused with `related_to` (link) | medium | Disambiguated in U7 docs, in the validation error, and in MCP tool descriptions |
| Rehydration transaction weakened | high | No `BestEffort` `*sql.DB` variant; transaction boundary asserted |

## Constitution Check

| Principle | Assessment |
|---|---|
| I. Safety-First Go | No `unsafe`. `ErrInvalidDependencyType` in `internal/errors`; wrapped errors. |
| II. Test-First | U1 is an explicit RED characterization observed failing at HEAD. |
| III. Workspace Isolation | No new paths. |
| IV. CLI Containment | No writes outside the workspace. |
| V. Structured Observability | Invalid-type rejection returns a typed, actionable error naming the allowed set and the dependency-vs-link distinction. |
| VI. Single Responsibility | No new dependencies; reuses the existing normalization pattern. |
| IX. Git-Friendly Persistence | Sorted-key YAML preserved; typed shape emitted only when needed. |
| X. Context Efficiency | No change to query shape. |

No violations.

## Plan Hardening Signals

* schema or contract change: frontmatter shape for `dependencies` — **yes**
* migration or compatibility concern across the live corpus — **yes**
* agent-facing read contract change — **yes**
* security-sensitive or destructive — no

Requires plan hardening: yes

## Runtime Verification and Closure

* **Verification surface:** `backlogit sync`, `dep add/remove/list`, the MCP
  dependency handlers, and `get_item`/`list_items` dependency projection.
* **Scenarios:** typed edge survives sync; bare-string corpus unchanged; invalid
  type refused at load; empty list consistent; CLI/MCP parity.
* **Rollback:** the shape is additive and both forms are accepted, so rollback is
  a plain revert with no data migration to reverse.
* **Closure artifact:** must record that the contract is forward-only, that bare
  strings mean `blocks` permanently, and the exact YAML keys.

## Plan Hardening

Hardening was required (schema/contract change, live-corpus compatibility, and an
agent-facing read contract change).

### Protected Invariants (must not regress)

1. An artifact whose `dependencies` is a bare ID list round-trips
   **byte-identically**.
2. No frontmatter key is dropped by the typed write path.
3. `Rehydrate` remains a single transaction with clear-and-rebuild inside it.
4. Both `RemoveDependency` rollback branches, including the
   `ErrWriteIndeterminate` case, keep their current structure.
5. Frontmatter remains the source of truth; SQLite remains a disposable
   projection.
6. CLI and MCP return the **same** canonical edge representation.
7. Empty collections serialize as `[]`, never `null`, on both surfaces.

### Learnings and Instructions Consulted

* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`
* `docs/decisions/2026-07-14-docline-soft-key-regression-decision.md`
* `docs/compound/2026-06-26-docline-frontmatter-contract.md`
* `docs/compound/go-patterns/f015-shipment-stash-patterns.md`
* `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
* `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`
* `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`
* `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`
* `.github/instructions/go.instructions.md`,
  `.github/instructions/strict-safety.instructions.md`

### Risky Actions (carry forward to Ship)

| # | ProposedAction | Targets | change_kind | ActionRisk | rollback | approval_required |
|---|---|---|---|---|---|---|
| A1 | Change the frontmatter shape of a governed field across the live corpus | `internal/models/*`, `internal/core/artifacts.go` | contract + data shape | **high** | Plain revert; both shapes remain readable | **yes** |
| A2 | Change the model field type, rippling to every call site | `internal/models/artifact.go` | breaking internal API change | **high** | Plain revert | **yes** |
| A3 | Change the rehydration edge-rebuild source | `internal/db/rehydration.go` | data-model change | moderate | Plain revert; index is disposable | no |
| A4 | Change the agent-facing dependency read representation | `internal/mcp/tools.go`, registry | contract | moderate | Plain revert | no |

`ActionResult` for every entry starts `planned`.

### Deepened Verification and Rollback (for Ship)

* **Idempotency proof:** re-serialize an already-compliant artifact and assert a
  byte-identical no-op with no body-byte change.
* **Corpus inventory before declaring forward-only:** count artifacts currently
  carrying non-`blocks` edges and state the number in closure.
* **Compile-time call-site sweep:** the model type change is deliberately
  breaking so the compiler enumerates the call sites; do not add a compatibility
  shim that hides a missed site.
* **Empty-case assertion** on frontmatter, CLI, and MCP surfaces.
* **No `t.Parallel()`** in any package overriding a package-global write seam.
* **Rollback trigger:** any artifact observed losing a frontmatter key, or any
  bare-string artifact rewritten, in the first validation window → revert.
* **Validation window:** one full `sync` over the live corpus with a clean
  `git status` afterwards, owned by the operator.

### Unresolved Operator Decisions

None.

## Plan Review

* **dispatch_mode: multi-agent-dispatch** (Constitution Reviewer, Scope Boundary
  Auditor, Architecture Strategist, Go Reviewer, Agent-Native Parity Reviewer,
  Learnings Researcher — cross-model).
* **Cycle 1 decision: FAIL.** P1: the model type change was implied but never
  declared, and the call sites were not enumerated (F4-01); the YAML key shape —
  a machine-consumed contract — was left to implementation time (F4-02); the MCP
  read contract and `dep list` output shape were undefined, so agents could
  receive different shapes from different surfaces (parity F4-1). P2: validation
  sequenced after persistence, so invalid state could enter if the plan were
  abandoned mid-way (Architecture); existing registry row and MCP schemas treated
  as closure verification rather than implementation contracts (parity F4-2).
* **Resolutions:** `DependencyEdge` declared explicitly with
  `Artifact.Dependencies` changing to `[]DependencyEdge`; a `toDependencyEdges`
  normalizer specified against the `toArtifactLinks` precedent with an explicit
  decision **not** to introduce a custom `UnmarshalYAML`; the YAML shape fixed to
  `{id, type}`; every `.Dependencies` call site enumerated by file and line in
  U3; validation moved to the load edge in U2; new units U5 and U6 added for the
  CLI surface and the canonical MCP read contract with a parity test; U7 extended
  to disambiguate `relates_to` from `related_to` in tool descriptions; A1
  upgraded to `approval_required: yes` and A2 added.

### Cycle 2 Decision

decision: PASS

* dispatch_mode: multi-agent-dispatch
* P0: 0 — P1: 0 — remaining P2/P3 accepted as advisory follow-ups.
