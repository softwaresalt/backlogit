---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Size Extension Contract Architecture Spike Charter — Feature and Shipment Sizing (SPIKE; conclusion pending)'
source: docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md
doc_type: plan
description: 'Architecture spike charter (PR #241 refocus; PR #242 split of the oversized parity/synthesis task) of seven sequenced <=2h research/decision tasks for optional backlogit-owned size extensions on the docline base contract: base/extension ownership, mutation/provenance/JSONL history durability and the unresolved size-write containment boundary, structured-composition membership/dedup/ruleset, CLI/MCP read-surface parity, CLI/MCP mutation parity, inheritance-bridge and canonical size-location evidence, and an explicit proceed/pivot/defer synthesis exit decision — no implementation and no root-containment claim.'
docline:
    date: 2026-07-14T23:40:00Z
    refocused: 2026-07-15T00:00:00Z
    time_box: "14h"
    conclusion: "pending"
    confidence: "low"
    linked_stash_ids:
        - D7B1B33D
    review_state: spike-chartered
    gate: SPIKE
    review_provenance: "plan-review skill RE-RUN 2026-07-15 by Stage against final plan bytes after PR #241 refocus AND the restructure that (a) moves the spike into a NEW queued feature 109-F + four sequenced <=2h research/decision tasks 109.001-T..109.004-T (096-S rebound to the 109 tree only), (b) restores the harvested 108-F tree to a BLOCKED future-implementation placeholder outside 096-S that depends on 109.004-T AND the docline pass-through task 107.009-T (shipment 095-S) (the third edge 107.011-T was added in the 2026-07-16 cycle-2 re-run recorded below), informed by 109-F, (c) inventories the multiple established JSONL durability policies (LinkCommit warn-continue, AppendComment fail-surface, gate-evidence fail-closed) and makes the size ordering/rollback policy an explicit 109.004-T decision output, (d) makes composition ratification depend on 109.003-T + 109.004-T, and (e) uses true CLI/MCP command-to-tool parity pairs across READ surfaces AND the mutation pair (CLI update --size <-> MCP update_item size), with 109.004-T deciding future size_source/size_ruleset_version flags/fields and transport-aware validation/error parity (equivalent category/message); single-model multi-persona (cross-model unavailable per skill fallback); no implementation PASS recorded — spike exit criteria only, gate remains SPIKE. PR #242 review-fix RE-RUN 2026-07-16 split the oversized 109.004-T into width-isolated <=2h evidence tasks 109.005-T (read-surface request/response/context parity), 109.006-T (mutation validation/error parity + future provenance flag/field options), and 109.007-T (inheritance-bridge + generic rewrite-path preservation + canonical size-location evidence); 109.004-T refocused to final synthesis/exit only (depends on 109.002/109.003/109.005/109.006/109.007); 096-S now carries 109-F + seven tasks; timebox raised to 14h; preservation of top-level size extensions restated as semantic/deep-value equivalence + unchanged body bytes + idempotency, not raw frontmatter byte/lexical preservation; gate remains SPIKE. PR #242 review-fix CYCLE 2 RE-RUN 2026-07-16 by Stage against these exact final bytes with the Schema-CLI-Docs Coupling and Agent-Native Parity reviewers: read-surface parity ownership assigned to 109.005-T and mutation parity to 109.006-T with 109.004-T consuming completed evidence only (no parity investigation in 109.004-T); 109.007-T restated from preservation-confirmation to per-path preserve/drop CLASSIFICATION with identified loss points and concrete bridge options (current generic paths demonstrably drop unknown top-level fields); 109.004-T gains an explicit inheritance-bridge SELECTION exit criterion (proceed impossible until selected); current-HEAD docline behavior corrected to relocation-under-docline with a latent (not current) top-level drop; base schema restated as OPEN to producer-owned top-level extension properties without enumerating/validating size fields and without a declared-vs-legacy distinction (backlogit derived contract owns size validation later); 109-F summary names all three 108-F prerequisites (109.004-T + 107.009-T + 107.011-T, necessary-but-not-sufficient); gate remains SPIKE (conclusion pending)"
---

# Size Extension Contract Architecture Spike — Feature and Shipment Sizing (D7B1B33D)

## Status: Restructured to a queued spike (109-F) plus a blocked implementation placeholder (108-F) (PR #241)

This document was harvested as an implementation plan and previously recorded an
implementation-ready PASS. Bounded investigation for PR #241 found that
provenance/history atomicity, XS-XL aggregation semantics, and even the
workspace-containment boundary of the size-write path are **not yet resolvable as
an implementable contract** against current code. Per the Stage honesty rule
("do not manufacture implementation readiness"), the size work is now a genuine
**architecture spike**, and the topology has been restructured for a clean,
shippable release scope and consistent lifecycle history:

* **`109-F` — NEW queued spike (carried by shipment `096-S`).** Composed **only**
  of seven sequenced research/decision tasks — `109.001-T` (base-contract/extension
  ownership + typed surface inventory), `109.002-T` (mutation/provenance/JSONL
  history durability, failure ordering, containment, and the generic rewrite-path
  inventory), `109.003-T` (structured-composition membership/dedup/missing/ruleset),
  `109.005-T` (CLI/MCP read-surface request/response/context parity inventory),
  `109.006-T` (CLI/MCP mutation validation/error parity + future
  `size_source`/`size_ruleset_version` flag/field options), `109.007-T`
  (inheritance-bridge + generic rewrite-path **preserve/drop classification** + canonical size-location
  evidence), and `109.004-T` (final synthesis consuming
  `109.002`/`109.003`/`109.005`/`109.006`/`109.007` to select the durability policy,
  ratify composition, **select the inheritance bridge**, decide the canonical size-location, and record the explicit
  `proceed`/`pivot`/`defer` exit record). Each task is strictly **≤2h**,
  independently verifiable, and **queued** (14h cap total). It contains **no
  implementation tasks** and asserts **no implementation readiness**. Shipment
  `096-S` is rebound to **only** the
  `109` tree, so its recursive release scope contains **no blocked descendants**.

* **`108-F` — BLOCKED future-implementation placeholder (outside `096-S`).** The
  originally-harvested `D7B1B33D` feature and its `108.*` tasks are preserved as a
  blocked implementation-planning placeholder tree, mirroring the `106-F`
  formal-gate pattern. `108-F` is authored `blocked` and **unmanifested** (no
  shipment home until a later Stage restaging), and depends on **three**
  necessary-but-not-sufficient gate edges — `109.004-T` (the spike exit
  decision), `107.009-T` (the docline top-level extension pass-through
  production codec task in shipment `095-S`), **and** `107.011-T` (the docline
  base-schema opening to **accept** producer-owned top-level extension properties, shipment `095-S`) — and is
  **informed by** `109-F`. The
  `107.009-T` and `107.011-T` edges are genuine cross-shipment prerequisites: the top-level
  derived size fields (`size`, `size_source`, `size_ruleset_version`) are
  top-level backlogit-owned extension keys; at current HEAD docline **relocates**
  unknown top-level keys under the `docline` namespace (folded and preserved as a nested map,
  **not** dropped today — a top-level drop is only **latent**, occurring if the fold is
  removed without a top-level carrier), and the base schema rejects them at the top level via `additionalProperties: false`,
  so future feature/shipment size provenance cannot survive a docline
  lint/migration round-trip or validate against the base contract until both land.
  Base-contract conformance is **necessary but not sufficient**: backlogit's own
  `.backlogit` rewrite paths (`models.ArtifactFromFrontmatter`,
  `core.WriteArtifactFile`, `core.SetArtifactSize`, generic title/status/section
  updates) must independently preserve top-level extensions. **Current vs. target
  (honest):** `.backlogit` artifacts are today parsed and written by backlogit's own
  `models`/`core` codec (`models.ArtifactFromFrontmatter`, `core.WriteArtifactFile`),
  **not** automatically by `internal/docline.Scope`/`Normalize`. The base/extension
  inheritance model is the **target** contract for interoperable ingestion
  (graphtor/engram), **not** proof that current backlog writers already use the docline
  codec; `109.001-T` inventories this gap and `109.004-T` records the concrete bridge
  (which base fields/schema/codec invariants backlog artifacts must implement or reuse)
  without assuming direct reuse of `docline.Scope`. The `107.009-T`/`107.011-T` edges do
  **not** provide backlog-artifact round-trip preservation; `109.002-T`/`109.004-T`
  resolve the backlog writers separately. Dependency completion is
  outcome-agnostic and does not auto-transition `108-F`. Its child tasks (`108.001-T`..`108.004-T`) are blocked implementation
  placeholders that are **not authorized** and may be re-scoped or superseded by
  the spike outcome. No implementation activates until `109.004-T` records a
  `proceed` decision (with the containment boundary and canonical size-location
  resolved) **and** a later Stage
  restaging moves `108-F` `blocked->active`, gives it a shipment home, and re-harvests bounded ≤2h units.
  `D7B1B33D` traceability is preserved on `108-F`.

The resolvable parts (base-contract/extension ownership and the structured
composition sketch) are recorded below as spike inputs; the unresolved parts —
including the size-write containment boundary — are named explicitly as spike
questions, not decisions. Any size implementation may be planned, harvested, and
reviewed **only after** `109.004-T` records an explicit `proceed` decision.

## Problem Frame

Size estimation currently exists as an optional `size` field on **tasks only**
(enum XS-XL; `.backlogit/header-def.yaml`). Feature and shipment types define no
`size` field, and no `size_source`, `size_ruleset_version`, or estimate-history
concept exists anywhere in `internal/`, `schemas/`, or the header-def. Stash
`D7B1B33D` asks to extend optional size estimation to feature and shipment
levels **without** conflating human-authored estimates with machine-derived
composition.

**Base-contract / extension ownership (authoritative, PR #241).** Docline owns
only the base Markdown/frontmatter ingestion contract and its compatibility
rules. That base contract is open/extensible — like a base class that can be
extended with additional optional properties. `size`, `size_source`, and
`size_ruleset_version` are optional **backlogit-owned** extension properties.
Docline does **not** calculate, default, aggregate, validate domain semantics,
or synthesize size (it re-emits/serializes caller-provided size unchanged at the
top level when present but attaches no semantics); consumers (graphtor, engram) tolerate/preserve or safely ignore
these extension keys per existing codec behavior. Any derived feature/shipment
composition is a backlogit runtime/query projection and is **never** persisted as
if human-authored.

## Bounded Investigation Findings (read-only, current HEAD)

| # | Finding | Evidence |
|---|---|---|
| F1 | The size mutation seam accepts only `(id, size)` and has no provenance inputs. | `internal/core/artifact_size.go:35` — `SetArtifactSize(ctx, ws, id, size string)`. |
| F2 | Both adapters route only `size` through that seam. | CLI `internal/cli/update.go:97-115`; MCP `internal/mcp/tools.go:747-755`. |
| F3 | `SetArtifactSize` intentionally emits **no** event (size-only changes bypass the hook chain). | `internal/core/artifact_size.go:32-34`. |
| F4 | Item history is append-only **JSONL** events (not YAML frontmatter), and JSONL durability is **not uniform** — the codebase already uses at least three distinct, precedented policies. | `internal/core/commits.go`: `LinkCommit` appends `commit_tracked` best-effort and only `slog.Warn`/continues on append failure; `AppendComment` **returns** the append failure (fail-surfacing). `internal/core/shipment_gate.go`: gate evidence **fails closed** on unverifiable state. |
| F5 | The size-write path is **not proven root-contained**: `SetArtifactSize` resolves the path via `FindArtifactPath` (`artifactSearchDirs` + `filepath.WalkDir`) and writes via `atomicfile.WriteFileAtomic` **without ever calling `SafeResolve`**. `SafeResolve` itself is lexical-only (no realpath/rollback) and is not on this path. | `internal/core/artifact_size.go:35-81` (`FindArtifactPath` → `atomicfile.WriteFileAtomic`, no `SafeResolve`); `internal/core/artifacts.go:646-679` (`FindArtifactPath` WalkDir); `internal/core/workspace.go:271-290` `SafeResolve` (lexical, off-path). |
| F6 | No canonical XS-XL aggregation ruleset exists anywhere in the codebase. | grep of `internal/`, `schemas/`, header-def: no `size_source`/`size_ruleset`/size-rollup/histogram code. |

## Requirements Trace (stash requirement → spike investigation; no implementation authorized)

| ID | Requirement | Disposition |
|---|---|---|
| SE1 | Level-specific optional `size` semantics for feature vs shipment (schema/contract) | Ownership/surface researched in `109.001-T`; implementable contract deferred to a future plan authored only after a `proceed` decision. |
| SE2 | Typed provenance inputs `size_source`/`size_ruleset_version` and defaulting across the core seam and both adapters | **Unresolved** (F1–F3): the seam has no provenance input path today. Investigated in `109.002-T` as a named spike question. |
| SE3 | Estimate-history behavior covering **every** provenance-field change, with a defined event/write failure ordering | **Unresolved** (F3–F5): `109.002-T` inventories the established durability policies (LinkCommit warn-continue, AppendComment fail-surface, gate-evidence fail-closed) as evidence; `109.004-T` records the explicit ordering + rollback/fail-closed **decision** for size. Overlaps the partial-core-mutation-rollback and containment-boundary questions. |
| SE4 | Explicit **structured** derived composition (not a synthetic categorical aggregate) exposed at render with CLI/MCP parity | Composition shape sketched (see below); membership/dedup/ruleset ownership investigated in `109.003-T`, and **ratified jointly by `109.003-T` and the `109.004-T` synthesis/exit decision** — never by the ownership inventory alone. |
| SE5 | CLI/MCP parity documentation and verification | **REQUIRED/PENDING — not yet performed.** Request-contract and response-shape parity across the true command-to-tool pairs is split by surface into two width-isolated **evidence** tasks: **read parity is owned by `109.005-T`** (CLI `queue view`↔MCP `get_queue`; CLI `shipment get`↔MCP `get_shipment`; CLI `shipment list`↔MCP `list_shipments`; CLI `get`↔MCP `get_item`; CLI `list`↔MCP `list_items`) and **mutation parity is owned by `109.006-T`** (the current true write pair CLI `update --size`↔MCP `update_item` size, plus the paired **future** CLI flags / MCP fields and transport-aware validation/error parity (equivalent category/message) for `size_source`/`size_ruleset_version`). `109.004-T` performs **no** parity investigation or verification of its own — it **only consumes** the completed `109.005-T`/`109.006-T` evidence and records the final decisions (future flag/field selection, canonical size-location, inheritance-bridge selection, and the proceed/pivot/defer exit). **No parity verification is complete and no exit decision is recorded** — `docline.conclusion` is still `pending`. Documentation and implementation follow a future implementation plan, only after `109.004-T` records a `proceed` decision. |

## Structured Composition Contract Sketch (spike input, must be ratified)

No canonical XS-XL aggregation ruleset exists (F6), so summing categorical
values into a single synthetic size would invent arithmetic. The spike's
preferred direction is an **explicit structured composition** response, computed
on read and never persisted:

```yaml
composition:
  histogram: { XS: n, S: n, M: n, L: n, XL: n }   # counts per authored size
  unsized: n                                        # members with no size
  members: [ canonical member IDs counted ]         # exact, de-duplicated
  ruleset_version: <string|null>                    # null until a ruleset is owned
```

**Membership and de-duplication (resolves the double-count finding):**

* **Feature composition** counts a feature's **direct children by `parent_id`**
  (tasks/reviews), each canonical ID once.
* **Shipment composition** expands the shipment manifest, then **de-duplicates the
  union of `{feature, its child tasks}`** so a manifest listing both a feature and
  its child tasks counts each canonical work item exactly once. The `members`
  array makes the counted set auditable.
* **Missing/legacy handling:** members without an authored `size` increment
  `unsized`, never a default bucket. Absent `size_source` reads as unknown/legacy
  and is never rewritten as `human`.

This shape is implementable and avoids both invented categorical arithmetic and
feature+child double counting. It remains a **sketch** until the `109.003-T`
composition investigation and the `109.004-T` synthesis/exit decision jointly
ratify membership rules, the `unsized` contract, and ruleset-version ownership —
**not** the `109.001-T` ownership inventory, which only records the ownership
boundary and surface.

## Named Spike Questions (unresolved — do not manufacture readiness)

1. **Typed mutation/defaulting seam.** `SetArtifactSize` is `(id, size)`-only
   (F1–F2). What is the typed signature that carries `size_source`/
   `size_ruleset_version`, and what are the defaulting rules, without duplicating
   logic across the CLI and MCP adapters? Both adapters must gain source/version
   inputs or a defined default; a `size_ruleset_version`-only change must be
   representable.
2. **Provenance/history atomicity and durability-policy selection.**
   `SetArtifactSize` emits no event today (F3), and JSONL durability in this
   codebase is **not uniform** (F4): `LinkCommit` warns/continues on append
   failure, `AppendComment` returns the append failure, and shipment gate evidence
   fails closed. `109.002-T` must **inventory** these established policies as
   evidence; `109.004-T` must then record an **explicit decision** selecting the
   size mutation/event write-append ordering and the rollback-vs-fail-closed
   policy that guarantees **exactly one** history event per persisted provenance
   change (including `size_ruleset_version`-only changes). This overlaps the
   formal-gate spike's open partial-core-mutation-rollback question and must be
   resolved coherently, not assumed.
3. **Ruleset-version ownership.** Who owns and versions any future aggregation
   ruleset, and until one exists, is `ruleset_version` simply `null` with the
   structured histogram as the only composition output?
4. **Provenance vs. history storage split.** Provenance (`size_source`,
   `size_ruleset_version`) belongs in YAML frontmatter; estimate **history**
   belongs in append-only JSONL item-event logs. These are separate durability
   surfaces and the plan must state each guarantee honestly — and, per the
   durability inventory in question 2, the JSONL guarantee is **policy-dependent**
   (warn-continue vs. fail-surface vs. fail-closed), not a single "best-effort"
   default.
5. **Workspace-containment boundary of the size-write path (must be resolved
   before `proceed`).** `SetArtifactSize` reaches `atomicfile.WriteFileAtomic`
   via `FindArtifactPath` (`filepath.WalkDir` over the artifact search dirs) and
   **does not invoke `SafeResolve`** (F5). Size writes are therefore **not
   proven root-contained** by this seam. The spike MUST determine the correct
   containment seam (where/whether `SafeResolve` or a realpath boundary belongs
   on the write path) and record the evidence. A `proceed` outcome is **not
   permitted** until this containment boundary is resolved. This is a named open
   question and evidence item — not a decision, and not something implemented in
   this Stage turn.
6. **Generic backlog rewrite paths — preserve/drop classification (do NOT assume preservation).**
   The `.backlogit` artifact mutation paths use **generic backlog artifact
   readers/writers**, not `internal/docline.Scope()` alone, and the current generic
   paths **demonstrably drop** unknown top-level extension fields, so a future
   top-level `size`/`size_source`/`size_ruleset_version` extension could be
   silently lost by a later write. `109.002-T` MUST inventory each generic
   rewrite path and `109.007-T` MUST **classify each as preserve or drop**, identify
   the exact **loss points**, and propose **concrete bridge options** — it must NOT
   assert a preservation success condition: `models.ArtifactFromFrontmatter` (`internal/models/frontmatter.go`,
   frontmatter→Artifact parse — maps only an enumerated set of known keys, so unknown
   **top-level** keys have no carrier and are dropped on read), `core.WriteArtifactFile`
   (`internal/core/artifacts.go`, generic write — re-emits only struct-backed fields; a
   top-level extension is not carried), `core.SetArtifactSize`
   (`internal/core/artifact_size.go`, which today stores size under the **nested**
   `custom_fields.size`, which survives because `custom_fields` is captured), and the generic title/status/section
   update rewrites. This is the evidence — classification, loss points, and bridge
   options — that a future top-level size extension can be made to survive backlogit's own read/modify/write cycle **independent of docline**; the bridge **selection** is made in `109.004-T`.
7. **Size placement consistency (must be decided before `proceed`).** Existing
   task size is physically stored under `custom_fields.size` (a **nested**
   location) via `core.SetArtifactSize`, while this spike proposes feature/shipment
   size as a **top-level** backlogit-owned extension. `109.004-T` MUST record this
   inconsistency and make a **canonical-location decision** (reconcile task
   `custom_fields.size` vs feature/shipment top-level size — whether task size
   migrates to top-level or the two coexist with a documented rationale) as a
   first-class exit criterion before any `proceed`.

## Task Map (seven sequenced research/decision tasks under 109-F, all QUEUED)

This spike (`109-F`, carried by shipment `096-S`) contains **no implementation
tasks**. All seven tasks are strictly ≤2h, investigation/decision only, and
independently verifiable (14h cap total).

### `109.001-T` — Base-contract/extension ownership + typed surface inventory (2h max, QUEUED)

Research/decision. Inventory the typed size surface (task-only `size` enum under
`custom_fields.size`; no feature/shipment `size`; no `size_source`/
`size_ruleset_version`/history anywhere; `SetArtifactSize(ctx, ws, id, size)`
`(id, size)`-only seam; CLI `update --size` and MCP `update_item size` route only
`size`). Record the base-contract/extension ownership boundary: docline owns only
the base ingestion contract; the size keys are optional backlogit-owned extensions
(top-level in the derived backlogit contract) docline never calculates/defaults/
aggregates/validates/synthesizes (it re-emits caller-provided extension keys unchanged
at the top level but attaches no semantics). Deliverable: written ownership boundary + surface
inventory feeding `109.002-T`/`109.003-T`. No code, schema, or CLI changes.

### `109.002-T` — Mutation/provenance/JSONL history durability, ordering, and containment (2h max, QUEUED)

Research/decision (depends on `109.001-T`). Investigate the durability questions a
future typed provenance seam must answer: the missing provenance-input path and
absent mutation event (F1–F3); **inventory the established, non-uniform JSONL
durability policies** (F4) — `LinkCommit` warn-continue, `AppendComment`
fail-surface, and shipment gate-evidence fail-closed — as distinct precedents for
the size ordering/rollback choice; the write/append ordering options for exactly
one history event per persisted provenance change; and the **unresolved
workspace-containment boundary** (F5 — size writes reach `atomicfile` without
`SafeResolve`); and the **generic backlog rewrite-path inventory** (spike question
6) — `models.ArtifactFromFrontmatter`, `core.WriteArtifactFile`,
`core.SetArtifactSize` (today nested `custom_fields.size`), and the generic
title/status/section update rewrites — recording whether each round-trips unknown
top-level extension keys so no later write drops a future top-level size
extension. Deliverable: durability-policy inventory, ordering options, the
containment open question, and the generic rewrite-path preserve/drop inventory as
**evidence** (the ordering/rollback selection is made in `109.004-T`, not here).
No implementation.

### `109.003-T` — Structured-composition membership/dedup/missing/ruleset (2h max, QUEUED)

Research/decision (depends on `109.001-T`). Investigate the candidate structured
composition (histogram + `unsized` + de-duplicated `members`), feature membership
(direct children by `parent_id`) vs. shipment manifest expansion with
`{feature, its child tasks}` dedup, missing/legacy handling (`unsized`; absent
`size_source` reads unknown/legacy), and the ruleset-version ownership question
(`null` until owned). Deliverable: membership/dedup/missing/ruleset findings that,
together with the `109.004-T` synthesis, **ratify** the composition contract;
never persisted as authored. No implementation.

### `109.005-T` — CLI/MCP read-surface request/response/context parity inventory (2h max, QUEUED)

Research/decision (depends on `109.001-T`; extracted from the former oversized
`109.004-T` in the PR #242 split, width-isolated to read/projection surfaces).
Inventory READ parity across **true command-to-tool pairs** a future size
projection must populate identically: CLI `queue view` ↔ MCP `get_queue`; CLI
`shipment get` ↔ MCP `get_shipment`; CLI `shipment list` ↔ MCP `list_shipments`;
CLI `get` ↔ MCP `get_item`; CLI `list` ↔ MCP `list_items`. For each pair record
**request-contract parity** (exact CLI flags vs MCP params, default filters,
default sort, default grouping) and the **shared-subset vs capability gaps** for
the queue and list surfaces, **and** response-shape parity (field-for-field).
**Pin** the CLI `get --format json` ↔ MCP `get_item` comparison and record an
explicit decision-input on the `body` / `dependencies_detail` / `commit_links`
context asymmetry (which context fields each surface returns and whether a future
size projection must appear identically on both). Deliverable: read
request/response/context parity inventory as **evidence** feeding the `109.004-T`
synthesis; the proceed/pivot/defer decision and canonical size-location decision
are made in `109.004-T`, not here. No implementation.

### `109.006-T` — CLI/MCP mutation validation/error parity + future provenance flag/field options (2h max, QUEUED)

Research/decision (depends on `109.001-T`; extracted from the former oversized
`109.004-T` in the PR #242 split, width-isolated to write/mutation surfaces).
Inventory the current true write pair CLI `update --size` ↔ MCP `update_item`
size (both route only `size` through `core.SetArtifactSize` today) and record its
write-shape parity **and mandatory validation/error parity** across invalid size
value, unsupported artifact type, busy-lock/conflict (`ErrTaskBusy`), and
workspace-open failures. Then, as **decision-input evidence (not implementation)**,
inventory the paired **future** CLI flags / MCP fields and the **validation/error
semantics** for `size_source`/`size_ruleset_version` across both surfaces (flag and
field names, accepted values, defaulting, rejection of unknown source/version, and
transport-aware error parity across CLI and MCP — equivalent category/message with transport carriers mapped, not a byte-identical wire shape, per `109.006-T`). Deliverable: mutation
validation/error parity inventory plus the future source/version flag/field
options as **evidence** feeding the `109.004-T` synthesis; the final selection is
made in `109.004-T`. No implementation.

### `109.007-T` — Inheritance-bridge, generic rewrite-path preserve/drop classification, and canonical size-location evidence (2h max, QUEUED)

Research/decision (depends on `109.001-T` and `109.002-T`; extracted from the
former oversized `109.004-T` in the PR #242 split, width-isolated to the
backlog-writer bridge and size-location questions). (a) **Inheritance-bridge
evidence:** record which docline base-contract invariants (which base fields,
schema constraints, and codec preservation behaviors) the `.backlogit` backlog
writers must **implement or reuse** to honor the target inheritance model, WITHOUT
assuming direct reuse of `internal/docline.Scope` or `internal/docline.Normalize`;
current `.backlogit` artifacts are parsed/written by backlogit's own models/core
codec, so the `107.009-T`/`107.011-T` base-contract edges are necessary but do not
themselves provide backlog-artifact round-trip preservation. (b) **Generic
rewrite-path preserve/drop classification:** consume the `109.002-T` generic
rewrite-path inventory (`models.ArtifactFromFrontmatter`, `core.WriteArtifactFile`,
`core.SetArtifactSize`, generic title/status/section updates) and **classify each path
as preserve or drop** for unknown top-level extension keys, identifying the exact
**loss points** and proposing **concrete bridge options** — do NOT assert a preservation
success condition. The current generic paths demonstrably **drop** unknown top-level
fields (`models.ArtifactFromFrontmatter` maps only enumerated known keys, so a top-level
extension has no carrier and is dropped on read; `core.WriteArtifactFile` re-emits only
struct-backed fields; only the nested `custom_fields` survives). Where a path already
preserves, that is stated honestly as the extension key/value graph staying **semantically/deep-value
equivalent and top-level**, with **unchanged document body bytes** and **idempotent
normalization** — **not** raw frontmatter byte/lexical preservation (out of scope
unless a future YAML-node/raw-byte design is explicitly chosen). (c) **Canonical
size-location evidence:** record that task size is physically stored under the
nested `custom_fields.size` (via `core.SetArtifactSize`) while feature/shipment
size is proposed top-level, and gather the reconciliation options (migrate vs
coexist with rationale) as **evidence**. Deliverable: inheritance-bridge
decision-input, per-path rewrite-path **preserve/drop classification** (loss points +
concrete bridge options), and canonical
size-location evidence feeding the `109.004-T` synthesis; the final decisions are
made in `109.004-T`. No implementation.

### `109.004-T` — Final synthesis and explicit proceed/pivot/defer exit decision (2h max, QUEUED)

Research/decision — the **final synthesis/exit-decision** task (refocused in the
PR #242 split; **no broad inventory work remains here**). Depends on `109.002-T`,
`109.003-T`, `109.005-T`, `109.006-T`, and `109.007-T`; it **consumes** the
parity/bridge/size-location evidence and records the decisions. (a) **Durability
policy:** select the size mutation/event ordering and the rollback-vs-fail-closed
policy, choosing among the `109.002-T` precedents (`LinkCommit` warn-continue,
`AppendComment` fail-surface, gate-evidence fail-closed), as a first-class decision
output. (b) **Composition ratification:** ratify the structured-composition
contract jointly with `109.003-T`. (c) **Consume parity/bridge/size-location
evidence:** consume the completed `109.005-T` read request/response/context parity, the
completed `109.006-T` mutation validation/error parity (including the future
`size_source`/`size_ruleset_version` flag/field options), and the `109.007-T`
inheritance-bridge + generic rewrite-path preserve/drop classification + canonical size-location
evidence (this task performs no parity investigation of its own); where a path preserves,
that is honored as **semantic/deep-value
equivalence** of the top-level extension key/value graph plus unchanged document
body bytes and idempotent normalization (the codec reserializes YAML and may
canonicalize ordering/quotes/scalar spelling/comments/anchors), **not** raw
frontmatter byte/lexical preservation. (d) **Inheritance-bridge selection:**
informed by the `109.007-T` bridge decision-input and preserve/drop classification,
**SELECT** the concrete inheritance bridge — which base fields/schema/codec invariants
the `.backlogit` backlog writers implement or reuse, and how the identified DROP loss
points (`models.ArtifactFromFrontmatter` enumerated-key mapping; `core.WriteArtifactFile`
struct-only re-emit) are closed so a future top-level size survives backlogit's own
read/modify/write cycle — as a first-class deliverable; a `proceed` decision is impossible
until the bridge is selected. (e) **Canonical size-location decision:**
make the canonical-location decision (nested `custom_fields.size` vs
feature/shipment top-level — migrate or coexist with rationale) as a first-class
exit criterion, consuming the `109.007-T` evidence and coherent with the selected bridge. (f) **Future provenance
flags/fields:** select the future `size_source`/`size_ruleset_version` CLI flags /
MCP fields and their transport-aware validation/error parity (equivalent category/message across CLI and MCP, transport carriers mapped, not a byte-identical wire shape; per `109.006-T`) from the `109.006-T`
options. (g) **Exit decision:** record an explicit `proceed`/`pivot`/`defer`
decision with confidence in this plan's `docline.conclusion`. A `proceed` outcome
is permitted ONLY if **all** of the containment-boundary question (`109.002-T`),
the canonical size-location question, **and** the inheritance-bridge selection are resolved. A `proceed` decision is the ONLY
authorization for a later, separately planned, harvested, and reviewed
implementation (with `plan-harden` evaluated). Deliverable: the recorded
durability/ordering policy, ratified composition, **selected inheritance bridge**, canonical size-location decision,
future source/version flag/field selection, and explicit proceed/pivot/defer exit
record. This remains a ≤2h synthesis-only task: no broad inventory work and no implementation.

## Sequencing

`109.001-T` runs first and grounds the ownership/surface inventory. `109.002-T`,
`109.003-T`, `109.005-T`, and `109.006-T` then investigate durability/containment,
composition, and CLI/MCP read/mutation parity in parallel (each depends on
`109.001-T`; `109.005-T` and `109.006-T` are the width-isolated parity halves);
`109.007-T` gathers the inheritance-bridge, generic rewrite-path **preserve/drop
classification**, and canonical size-location evidence (depends on `109.001-T` + `109.002-T`).
`109.004-T` then synthesizes `109.002`/`109.003`/`109.005`/`109.006`/`109.007`,
**consumes** the completed CLI/MCP parity evidence (it runs no parity investigation of
its own), and records the exit decision (including the durability ordering/rollback
selection, the **inheritance-bridge selection**, and the canonical
size-location decision). All seven are queued research tasks — none carries a
`blocked` status or implementation readiness, so shipment `096-S` completes under
recursive release-scope semantics. The blocked `108-F` implementation placeholder
sits **outside** `096-S` (unmanifested), depends on `109.004-T`, the docline
pass-through codec task `107.009-T`, **and** the docline base-schema task `107.011-T` (both shipment `095-S`), and is informed by `109-F`; a
future implementation plan is authored only after `109.004-T` records a `proceed`
decision and a later Stage restaging moves `108-F` `blocked->active`. The
`107.009-T` and `107.011-T` edges exist because top-level derived size fields cannot stay
top-level through docline normalization or validate against the base contract until docline preserves unknown top-level extension keys in
place **and** the base schema is opened to **accept** producer-owned top-level extension properties (without validating their semantics, which remain backlogit-owned and derived).

## Non-Goals

* No coupling to the formal-gate architecture spike or the docline open
  extension-key guard staged in the same PR.
* No mandatory sizing; the field stays optional at every level.
* No persisting of derived composition as human-authored estimates.
* No invented categorical arithmetic and no manufactured implementation readiness
  while atomicity/aggregation questions are open.

## Constitution Check

This is a read-only research spike; the checks below describe the **spike's**
conduct, not an authorized implementation.

- **I (Safety-First Go):** No production Go is written by this spike. Any future
  implementation (authored only after a `proceed` decision) must keep wrapped
  errors and pass `go vet ./...` and `golangci-lint run` with zero warnings; no
  `unsafe` usage. Not exercised here.
- **II (Test-First, NON-NEGOTIABLE):** No implementation and therefore no
  test-first obligation in this spike. Each of the seven tasks is
  investigation/decision only. Test-first applies to the future implementation
  plan, not to these research tasks.
- **III/IV (Workspace isolation / CLI containment):** The size-write path is
  **not proven root-contained** — `SetArtifactSize` reaches
  `atomicfile.WriteFileAtomic` via `FindArtifactPath`/`filepath.WalkDir` and does
  **not** invoke `SafeResolve` (F5). This spike therefore makes **no** claim that
  size writes are root-contained; instead it records the containment boundary as
  named spike question 5, which MUST be resolved before any `proceed` outcome.
- **V (Structured Observability):** The desired guarantee — exactly one
  append-only JSONL estimate-history event per persisted provenance change — is a
  named spike question (`109.002-T` inventories the established durability policies;
  `109.004-T` selects the size ordering/rollback policy), not an assumed guarantee.
  `SetArtifactSize` emits no event today.
- **VI (Single Responsibility):** The spike adds no dependencies and writes no
  code; it inventories the existing size seam and schema/registry only.
- **VII (Destructive Approval, NON-NEGOTIABLE):** No destructive operations —
  the spike is read-only investigation plus this plan's recorded findings.
- **VIII (Safety Modes):** Investigate-first + freeze-scope posture — read-only
  investigation confined to the size subsystem, explicitly decoupled from the
  formal-gate spike and docline guard.
- **IX (Git-Friendly Persistence):** Provenance fields (`size_source`,
  `size_ruleset_version`) would serialize to human-readable YAML frontmatter;
  estimate **history** is separate append-only **JSONL** item-event logs
  (`events` package). These are distinct durability surfaces — the earlier
  "history as Markdown/YAML" wording is corrected here — and JSONL append
  durability is **policy-dependent, not uniformly best-effort**: `LinkCommit`
  warn-continues, `AppendComment` fail-surfaces, and gate evidence fails closed.
  The size ordering/rollback selection (`109.004-T`) chooses among these
  precedents before any implementation.
- **X (Context Efficiency):** Any future rollups are computed on read through
  existing query/render surfaces; no bulk duplication. Not implemented here.
- **XI (Merge Commit Preservation):** Not applicable at spike stage; Stage does
  not merge or ship.

Task Granularity: each of the seven tasks is a single **research/decision**
concern (ownership + surface inventory / durability + ordering + containment /
composition membership + dedup + ruleset / read-surface parity / mutation parity /
inheritance-bridge + rewrite-path + size-location evidence / final synthesis +
exit decision) and is strictly ≤2h and independently verifiable, capped at 14h
total. These are **not**
implementation tasks, so the plan makes **no** claim that they fit "under three
files and five functions" — that granularity heuristic applies to a future
implementation plan, not to investigation. Width isolation is preserved (the
spike is decoupled from CLI, schema, and template work). No constitutional
violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: SPIKE (no implementation PASS)

**Formal review provenance:** RE-RUN on 2026-07-15 by the Stage agent following
the `plan-review` skill against these exact final bytes after the PR #241 refocus
**and** the subsequent restructure that (a) moves the spike into a NEW queued
feature `109-F` + seven sequenced ≤2h research/decision tasks (`109.001-T`–`109.007-T`,
including the PR #242 split `109.005-T`/`109.006-T`/`109.007-T`) (with `096-S` rebound to the `109` tree only), (b) restores the
harvested `108-F` tree to a BLOCKED implementation placeholder outside `096-S`
(depends on `109.004-T`, the docline pass-through codec task `107.009-T`, **and** the docline base-schema task `107.011-T`,
informed by `109-F`), (c) inventories the non-uniform
JSONL durability policies and makes the size ordering/rollback selection an
explicit `109.004-T` decision output, (d) makes composition ratification depend on
`109.003-T` + `109.004-T`, and (e) uses true CLI/MCP command-to-tool parity pairs
across list/get surfaces. **PR #242 re-run (2026-07-16)** additionally split the
oversized `109.004-T` into the width-isolated ≤2h evidence tasks `109.005-T`
(read-surface request/response/context parity), `109.006-T` (mutation
validation/error parity + future `size_source`/`size_ruleset_version` flags/fields),
and `109.007-T` (inheritance-bridge + generic rewrite-path preserve/drop classification + canonical
size-location evidence), refocusing `109.004-T` to final synthesis/exit only
(depends on `109.002`/`109.003`/`109.005`/`109.006`/`109.007`), raised the timebox
to 14h, and restated preservation of top-level size extensions as semantic/deep-value
equivalence + unchanged body bytes + idempotency rather than raw frontmatter
byte/lexical preservation. **PR #242 review-fix CYCLE 2 re-run (2026-07-16)** — with the
Schema-CLI-Docs Coupling and Agent-Native Parity reviewers triggered — additionally:
assigned SE5 read-surface parity ownership to `109.005-T` and mutation parity to
`109.006-T`, with `109.004-T` consuming completed evidence only and performing no parity
investigation of its own; restated `109.007-T` from a preservation *confirmation* to a
per-path **preserve/drop classification** (identified loss points +
concrete bridge options; the current generic backlog rewrite paths demonstrably **drop**
unknown top-level fields via the enumerated-key `models.ArtifactFromFrontmatter` parse and
struct-only `core.WriteArtifactFile` re-emit, only nested `custom_fields` surviving);
added an explicit **inheritance-bridge SELECTION** exit criterion to `109.004-T` (a
`proceed` is impossible until the bridge is selected); corrected the current-HEAD docline
behavior to **relocation** under the `docline` namespace with a **latent** (not current)
top-level drop; restated the base schema as **open** to producer-owned top-level extension
properties (docline accepts and preserves; it does not enumerate or validate size fields,
and there is no declared-vs-legacy base-schema distinction — backlogit's future **derived**
contract owns size validation); and named all three `108-F` prerequisites in the `109-F`
summary (`109.004-T` + `107.009-T` + `107.011-T`, necessary-but-not-sufficient). No
implementation PASS is recorded and the gate remains **SPIKE**. The prior implementation PASS is **withdrawn** and
replaced with a **spike-charter review** with explicit exit criteria — not an
implementation PASS — because bounded investigation (F1–F6) showed the
provenance/history atomicity, XS-XL aggregation semantics, and workspace-
containment boundary are not yet an implementable contract. Recording an
implementation PASS now would manufacture readiness. Cross-model invocation was
unavailable; per the skill's fallback, all personas ran with the caller's model
(single-model multi-persona, disclosed). **PR #242 review-fix CYCLE 3 re-run (FINAL, 2026-07-16):** re-run against these exact final bytes as the final automated fix cycle. The docline current-HEAD behavior is restated as **relocation** (folded/nested, preserved) with only a **latent** top-level drop; contradictory *never emitted* wording is replaced with the generation-vs-serialization contract (docline **re-emits/serializes** caller-provided extension keys at the top level when present but never synthesizes/defaults/validates/interprets their semantics); the generic backlog rewrite-path **drop** evidence (`models.ArtifactFromFrontmatter` enumerated-key parse and struct-only `core.WriteArtifactFile` re-emit) is preserved as-is because it is a distinct backlogit-writer subsystem, not docline; and the size spike remains a charter-only **SPIKE** with **P0 = 0, P1 = 0** and no manufactured implementation readiness. **PR #242 review-fix CYCLE 4 re-run (operator-authorized beyond the 3-cycle limit, 2026-07-16):** the prior *CYCLE 3 FINAL* marker is superseded by this operator-authorized cycle; re-run against these exact final bytes resolving the remaining Copilot findings without broadening scope — (1) CLI↔MCP **error-shape parity** is reframed as **transport-aware** parity (equivalent error category/message; CLI Cobra `ExitError{Code:4}` on `core.ErrTaskBusy` vs MCP `{error,message}` via `domainError`/`makeErrorResult`), **not** a byte-identical wire shape; and (2) the size-shipment prerequisite provenance records the full **three-edge** set for `108-F` (`109.004-T`, `107.009-T`, `107.011-T`), correcting the stale two-edge wording. The size spike stays a charter-only **SPIKE** with **P0 = 0, P1 = 0** and no manufactured implementation readiness.

**Reviewer personas executed (against the restructured spike charter):**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | The restructure resolves the honesty violation (Principle II / no manufactured readiness): unresolved atomicity/aggregation/containment are named as spike questions, not asserted contracts. The `109` tree is seven **queued research/decision** tasks (none `blocked`), so `096-S` completes under recursive release-scope semantics; the blocked `108-F` placeholder sits outside `096-S` (no blocked descendants in scope) and depends on all **three** necessary-but-not-sufficient edges — `109.004-T` (the spike exit decision), the docline pass-through `107.009-T`, and the base-schema opening `107.011-T`. The III/IV containment overreach is corrected (no root-containment claim). No P0/P1. |
| Go Reviewer | always-on | Corrected factual claims: `SetArtifactSize` is `(id, size)`-only and emits no event (F1–F3); JSONL durability is **policy-dependent** — `LinkCommit` warn-continues, `AppendComment` fail-surfaces, gate evidence fails closed (F4), not uniformly best-effort; the size-write path reaches `atomicfile` via `FindArtifactPath`/`WalkDir` and never calls `SafeResolve` (F5). The charter no longer asserts seam capabilities or containment guarantees that do not exist. No P0/P1. |
| Scope Boundary Auditor | always-on | Width isolation intact; the spike is read-only investigation, decoupled from formal-gate (105-F/106-F) and docline (107-F). `096-S` carries only the `109` tree; `108-F` is a separate blocked placeholder. The charter no longer claims implementation granularity ("three files/five functions") for research tasks. No P0/P1. |
| Learnings Researcher | always-on | The atomicity question is correctly linked to the formal-gate spike's open partial-core-mutation-rollback question rather than re-deciding it in isolation, and the durability inventory reuses established codebase precedents rather than inventing a policy. No P0/P1. |
| Architecture Strategist | always-on | Structured composition (histogram + `unsized` + de-duplicated `members`) is a sound direction that avoids invented categorical arithmetic and feature+child double counting; correctly held as an investigation input ratified jointly by `109.003-T` and the `109.004-T` decision. No P0/P1. |
| Agent-Native Parity Reviewer | triggered (sizing touches MCP and CLI surfaces) | Parity is a REQUIRED/PENDING investigation split across `109.005-T` (read-surface request/response/context parity) and `109.006-T` (mutation validation/error parity + future `size_source`/`size_ruleset_version` flags/fields), with the final flag/field selection and canonical size-location decision made in the `109.004-T` synthesis (all queued; not yet performed), using true command-to-tool pairs across read surfaces (CLI `queue view`↔MCP `get_queue`; CLI `shipment get`↔MCP `get_shipment`; CLI `shipment list`↔MCP `list_shipments`; CLI `get`↔MCP `get_item`; CLI `list`↔MCP `list_items`) **and the mutation pair** (CLI `update --size`↔MCP `update_item` size). Scope now covers **request-contract** parity (exact CLI flags vs MCP params, default filters/sort/grouping, shared-subset vs capability gaps), the pinned CLI `get --format json`↔MCP `get_item` decision on `body`/`dependencies_detail`/`commit_links` asymmetry, **mandatory mutation validation/error parity** (invalid size, unsupported type, busy-lock/conflict, workspace-open), the future `size_source`/`size_ruleset_version` flags/fields decision, and the canonical size-location decision (nested `custom_fields.size` vs top-level) — all as exit criteria; no implementation parity is claimed. No P0/P1. |
| Security Lens Reviewer | triggered (F5 raises a workspace-containment/trust-boundary question) | The unresolved containment boundary is recorded honestly as spike question 5 and is a hard precondition for any `proceed`. No claim of proven containment remains. No P0/P1. |

**Findings disposition:** The prior latent P1s raised by Copilot — undefined
aggregation/membership (3585440713, 3585440748), missing provenance-input seam
(3585440776, 3585440806), and the YAML-vs-JSONL history contradiction
(3585440828) — plus the current review's containment-boundary finding (F5, size
writes not proven root-contained) are addressed structurally: the composition
sketch is an investigation input ratified by `109.003-T` + `109.004-T`, the seam
gap, the **non-uniform** durability inventory, and the containment boundary are
named spike questions with an explicit ordering/rollback decision output in
`109.004-T`, and the Constitution Check III/IV, V, and IX claims are corrected.
Additional LOCAL review findings for PR #241 (and its follow-up local-review
fixes) are incorporated without adding
implementation: (1) **cross-shipment prerequisites** — `108-F` now depends on the
docline pass-through codec task `107.009-T` **and** the docline base-schema opening
task `107.011-T` (both shipment `095-S`) in addition to
`109.004-T`, because top-level derived size fields cannot stay top-level through docline
normalization until unknown top-level extension keys are preserved in place and the
base schema is opened to **accept** producer-owned top-level extension properties (without
validating their semantics — those remain backlogit-owned and derived); all edges stay
necessary-but-not-sufficient and `108-F` stays authored `blocked` and unmanifested.
(2) **mutation parity** — `109.006-T` now inventories the current true write pair
(CLI `update --size` ↔ MCP `update_item` size), records **mandatory
validation/error parity** (invalid size, unsupported artifact type,
busy-lock/conflict, workspace-open failures), and inventories the paired future
CLI flags / MCP fields and transport-aware validation/error parity (equivalent category/message) for
`size_source`/`size_ruleset_version` as evidence, with `109.004-T` making the final
selection — closing a gap where only read list/get pairs
were inventoried. (3) **request-contract parity** — `109.005-T` read pairs now require exact
CLI flags vs MCP params, default filters/sort/grouping, shared-subset vs capability
gaps, and a pinned CLI `get --format json` ↔ MCP `get_item` decision on the
`body`/`dependencies_detail`/`commit_links` context asymmetry. (4) **generic
rewrite-path preserve/drop classification** — `109.002-T` inventories `models.ArtifactFromFrontmatter`,
`core.WriteArtifactFile`, `core.SetArtifactSize`, and the generic
title/status/section rewrites, and `109.007-T` **classifies each path preserve or drop,
identifies the loss points, and proposes concrete bridge options** — the current generic
paths demonstrably **drop** unknown top-level fields (enumerated-key parse with no top-level
carrier; struct-only re-emit; only nested `custom_fields` survives). Where a path preserves,
equivalence is semantic/deep-value (unchanged body bytes + idempotent
normalization, not raw frontmatter byte/lexical preservation); the bridge **selection**
is made in `109.004-T`. (5) **size placement consistency** — task size is stored under the nested
`custom_fields.size` while feature/shipment size is proposed top-level; `109.007-T`
gathers the reconciliation evidence and `109.004-T`
must make a canonical-location decision before any `proceed`. (6) **oversized-task
split** — the former `109.004-T` combined broad read/mutation parity, inheritance-bridge,
rewrite-path, and size-location inventory with the synthesis, exceeding a single ≤2h
width-isolated concern; it is split into `109.005-T`/`109.006-T`/`109.007-T` evidence
tasks and refocused to synthesis/exit only, keeping every research task ≤2h and
width-isolated (14h cap). None are resolved
into an implementation contract here; that is the spike's job.

**Plan hardening:** N/A for a read-only spike charter. If `109.004-T` records a
`proceed` decision, the resulting implementation plan (touching the core mutation
seam, event durability, and the containment seam) MUST be evaluated for
`plan-harden` before any implementation is harvested.

### Spike Exit Criteria (in lieu of an implementation PASS)

Each research/decision task has an explicit, independently verifiable exit:

* **`109.001-T`** — exits when the base-contract/extension ownership boundary and
  the typed size-surface inventory are recorded.
* **`109.002-T`** — exits when the mutation/provenance path, the **non-uniform
  JSONL durability-policy inventory** (LinkCommit warn-continue, AppendComment
  fail-surface, gate-evidence fail-closed), the write/append ordering options, the
  **containment boundary** (F5), and the **generic backlog rewrite-path
  preserve/drop inventory** (`models.ArtifactFromFrontmatter`,
  `core.WriteArtifactFile`, `core.SetArtifactSize`, generic title/status/section
  rewrites) are documented as findings and open questions (no policy selected here).
* **`109.003-T`** — exits when feature/shipment structured-composition membership,
  dedup, missing/legacy handling, and ruleset-version ownership are documented as
  findings feeding ratification.
* **`109.005-T`** — exits when CLI/MCP **read-surface request/response/context
  parity** is inventoried across the true READ pairs (CLI `queue view`↔MCP
  `get_queue`; CLI `shipment get`↔MCP `get_shipment`; CLI `shipment list`↔MCP
  `list_shipments`; CLI `get`↔MCP `get_item`; CLI `list`↔MCP `list_items`),
  including exact CLI flags vs MCP params, default filters/sort/grouping,
  shared-subset vs capability gaps, and the pinned CLI `get --format json`↔MCP
  `get_item` decision-input on `body`/`dependencies_detail`/`commit_links`
  asymmetry — recorded as evidence for `109.004-T`.
* **`109.006-T`** — exits when CLI/MCP **mutation validation/error parity** is
  inventoried for the current true write pair (CLI `update --size`↔MCP
  `update_item` size) across invalid size, unsupported artifact type,
  busy-lock/conflict, and workspace-open failures, and the paired **future**
  `size_source`/`size_ruleset_version` CLI flags / MCP fields plus
  transport-aware validation/error parity options are inventoried — recorded as evidence for the
  `109.004-T` selection.
* **`109.007-T`** — exits when the **inheritance-bridge** evidence (which
  base-contract invariants backlog writers must implement/reuse without assuming
  direct `internal/docline` reuse), the **generic rewrite-path preserve/drop
  classification** (each path classified preserve or drop, with identified loss
  points and concrete bridge options; the current generic paths drop unknown
  top-level fields; where preserved, semantic/deep-value equivalence + unchanged body bytes +
  idempotency, **not** raw frontmatter byte/lexical preservation), and the
  **canonical size-location** reconciliation options are documented as evidence
  for `109.004-T` — with **no** preservation success condition asserted here.
* **`109.004-T`** — exits when: the size mutation/event **ordering and
  rollback-vs-fail-closed policy** is **explicitly selected** (among the
  `109.002-T` precedents); the composition contract is **ratified jointly** with
  `109.003-T`; the `109.005-T` read request/response/context parity, the
  `109.006-T` mutation validation/error parity (with the future
  `size_source`/`size_ruleset_version` flags/fields options), and the `109.007-T`
  inheritance-bridge + generic rewrite-path preserve/drop classification + canonical
  size-location evidence are **consumed** (no parity investigation is performed here);
  the **inheritance bridge is explicitly SELECTED** (which base fields/schema/codec
  invariants backlog writers implement or reuse, and how the identified drop loss points
  are closed) as a first-class deliverable; the **future**
  `size_source`/`size_ruleset_version` CLI flags / MCP fields and their transport-aware
  validation/error parity (equivalent category/message, not a byte-identical wire shape; per `109.006-T`) are **selected**; the **canonical size-location
  decision** (nested `custom_fields.size` vs top-level) is **recorded**; and an
  explicit `proceed`/`pivot`/`defer` decision with confidence is recorded in
  `docline.conclusion`. A `proceed` outcome is **only** permitted if **all** of the
  containment boundary from `109.002-T`, the canonical size-location
  question, **and** the inheritance-bridge selection are resolved. A `proceed` decision is
  the sole authorization for a later, separately planned and re-reviewed (with
  `plan-harden` evaluated) implementation, and for a Stage restaging that moves
  `108-F` `blocked->active`; `pivot`/`defer` keeps size implementation out of scope
  and re-enters staging.

**Disposition:** SPIKE chartered. Shipment `096-S` now carries the size extension
contract architecture spike (`109-F` + seven **queued research/decision** tasks
`109.001-T`/`109.002-T`/`109.003-T`/`109.005-T`/`109.006-T`/`109.007-T`/`109.004-T`, none `blocked`). The originally
harvested `108-F` tree is preserved as a BLOCKED future-implementation placeholder
**outside** `096-S`, is **unmanifested** (no shipment home until a later Stage restaging), depends on `109.004-T`, the docline pass-through codec
task `107.009-T`, **and** the docline base-schema task `107.011-T` (both shipment `095-S`), and is informed by `109-F`. No
implementation is authorized. This work remains decoupled from the formal-gate
governance work and the docline open extension-key guard staged in the same PR.
