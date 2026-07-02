---
description: "Forward-UX display convention surfacing each shipment's covering feature ID and title in CLI and MCP shipment views — read-only presentation layer, no manifest mutation"
doc_type: exec-plan
docline:
    plan_status: proposed
    depth: standard
    source_stash: D070FD3C
    linked_artifacts:
        - docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md
    tags:
        - shipment
        - cli
        - mcp
        - forward-ux
        - read-only
        - presentation-layer
schema_version: "1.0"
---

# Implementation Plan: Surface Covering Feature in Shipment Views

**Source stash:** `D070FD3C` (kind=feature, priority=low)
**Governing decision:** `docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md` (stash `B8FF7590`)

## Deliberation Basis (why no separate deliberation artifact)

The hard, contestable decision this feature depends on is **already settled and
durable**: the `B8FF7590` determination (Option 1, decided) concluded that the
shipment#-vs-feature# numbering offset is a **benign, cosmetic artifact of two
independent monotonic counters** and that the *only* safe remedy is a **forward
display convention** — explicitly: *"if operators find the shipment#≠feature#
offset confusing in future, the durable fix is a forward convention/UX change
(e.g. surfacing the covering feature ID in shipment listings), not a retroactive
manifest rewrite."* (Unresolved Question 1.)

This plan implements exactly that forward convention. There is no fresh options
trade-off to relitigate; the remaining choices (what to display, field-shape,
multiple/zero-feature rule) are design-level and are recorded in **Decisions and
Rationale** below. Per the lean-plan directive, no redundant deliberation
artifact is created — the determination doc is the deliberation of record.

## Problem Frame

Shipment IDs (`NNN-S`) and feature IDs (`NNN-F`) are drawn from **independent
monotonic counters**, so a queued shipment `NNN-S` commonly carries covering
feature `(NNN+1)-F` (e.g. `060-S`→`061-F`, `061-S`→`062-F`; precedent shipped
`057-S`→`058-F`). The binding is authoritative via the shipment's
`custom_fields.items` array — not the numeric label — but the offset is not
visible in shipment listings, so a reader must manually open the manifest and
cross-reference item IDs to learn which feature a shipment actually ships.

The fix is a **read-only presentation enhancement**: shipment views (CLI
`shipment list` / `shipment get`; MCP `backlogit_list_shipments` /
`backlogit_get_shipment`) should surface each shipment's **covering feature ID
and title** alongside the shipment record, derived at render/response time from
the manifest. **No manifest, title, or `custom_fields` storage is mutated.**

### Grounded code paths

| Surface | Symbol | File | Current behavior |
|---|---|---|---|
| CLI list | `newShipmentListCmd` | `internal/cli/shipment.go` | `db.QueryItems(type=shipment)` → table/tile via `artifactsToRows` OR `--json` raw-encodes `[]*models.Artifact`. Flags: `--status`, `--format`. |
| CLI get | `newShipmentGetCmd` | `internal/cli/shipment.go` | `core.GetShipment` → JSON-only encode (no human-readable/`--format` split). |
| MCP list | `handleListShipments` | `internal/mcp/tools.go` | `db.QueryItems` → `normalizeShipmentItems` each → `toolResultJSON(shipments)`. |
| MCP get | `handleGetShipment` | `internal/mcp/tools.go` | `core.GetShipment` → `toolResultJSON(shipment)`. |
| Core | `GetShipment`, `shipmentItems`, `loadArtifact` | `internal/core/shipment.go` | `shipmentItems()` normalizes `CustomFields["items"]` (`[]string`/`[]any`) at read edge; `loadArtifact`→`bldb.GetItem` resolves an ID to a `*models.Artifact`. |
| Model | `models.Artifact` | `internal/models/artifact.go` | Fields incl. `ID`, `Title`, `ArtifactType`, `Level`, `CustomFields`. |
| Shared render | `artifactColumns`, `artifactsToRows` | `internal/cli/list.go` | **Shared** by list/queue/shipment table output. Must NOT be globally mutated. |

**Covering-feature derivation rule (verified against the indexed DB, not raw
frontmatter):** among the shipment's `custom_fields.items`, the covering feature
is the resolvable item that is (a) `ArtifactType == "feature"` and (b) a **root
feature** — identified structurally by a **dotless root ID** (`id` contains no
`.`), corroborated by `Level == 1`. On the (unexpected) tie of multiple root
features, pick the first in `items` order (manifests are parent-first). Items
are resolved with a **pure read** (`bldb.GetItem`), not `loadArtifact` (which
upserts the index on a cache miss).

Verified against live indexed data (`backlogit query`): 72 features are indexed
at `level = 1`; nested features `013.001-F`/`013.002-F` are `level = 2`; the real
covering feature `058-F` of `057-S` is indexed at `level = 1`. The dotless-root-ID
predicate is the primary, `omitempty`-immune signal (a nested feature always has a
dotted ID); `Level == 1` is corroborating. This resolves a plan-review concern
that archived markdown frontmatter omits `level` — the derivation reads the
**index**, where `level` is populated, and never depends on `level` alone.

### Prior art (compound library — mandatory pre-planning retrieval)

* `docs/compound/go-patterns/f015-shipment-stash-patterns.md` — **SQLite JSON
  round-trips are lossy**; always normalize `CustomFields["items"]`
  (`[]any`→`[]string`) at the read edge. The derivation MUST reuse the existing
  `shipmentItems()` normalizer rather than re-parsing raw `custom_fields`.
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`
  — any change to a Cobra command's `Short`/`Long`/flags regenerates
  `docs/cli-reference/*.md` via `make docs`; CI's **CLI Reference Drift Check**
  fails if generated docs are not committed. Applies to Task 2 if command
  help/flags change.
* `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`
  and `2026-05-07-mcp-cli-config-parity.md` — CLI and MCP surfaces must stay at
  parity; the covering-feature field must appear on both, with the same key
  names and semantics. Per the first doc's Rule 1, **lock the contract with a
  dedicated CLI==MCP parity test** rather than relying on separate per-surface
  assertions.
* `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
  — the shipment manifest contract is **one root covering feature + subordinate
  tasks, parent-first**. This grounds the derivation rule and R5, and reframes
  the **zero-feature case as a contract exception** (an orphan condition
  `backlogit doctor --check-orphans` can detect), not a routine state — the
  derivation surfaces it as an absent covering feature rather than silently
  masking it.
* `docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md`
  — keep display sentinels at the render boundary only; the data layer returns
  `("", "", false)` for "no covering feature", and the response **omits** the
  covering-feature object (via `omitempty`) rather than emitting empty strings.
  This resolves the omit-vs-empty ambiguity deterministically.

Learnings retrieval confidence: **high** (directly-applicable normalization and
CLI-drift patterns).

## Requirements Trace

| # | Requirement (from stash + determination) | Implementation action |
|---|---|---|
| R1 | Surface covering feature ID **and** title in shipment listings/views | Derive `(id, title)` and expose a top-level `covering_feature` object in CLI list/get + MCP list/get |
| R2 | Cover CLI *and* MCP shipment views at parity | Task 2 (CLI) + Task 3 (MCP), both consuming the shared Task 1 derivation + shaping helper with identical field names; parity locked by a dedicated CLI==MCP test |
| R3 | **Read-only** — never mutate manifest/title/`custom_fields` storage | Derive at render/response time into a **top-level** field (never inside the persisted `custom_fields` map); resolve items with pure `bldb.GetItem` (no cache-miss upsert); symmetric regression tests assert stored file + DB row unchanged on all four surfaces |
| R4 | Handle zero-feature manifest defensively | Helper returns `ok=false`; views **omit** the `covering_feature` object (`omitempty`) — a contract exception surfaced as absent, not masked |
| R5 | Handle multiple features — pick the root covering feature | Helper selects `ArtifactType=="feature"` && dotless-root ID (corroborated by `Level==1`), parent-first first match |
| R6 | Work for queued and archived/done shipments alike | Derivation is status-agnostic (pure read of `items`) |

## Implementation Units

### Unit 1 — Core covering-feature derivation + shaping helper (FOUNDATION)

* **What:** Add to `internal/core` (in `internal/core/shipment.go` or a new
  `internal/core/shipment_covering.go`):
  1. A value type `type CoveringFeature struct { ID, Title string }` (extensible;
     avoids swappable positional strings).
  2. An exported, read-only derivation helper
     `DeriveCoveringFeature(ctx context.Context, ws *Workspace, shipment *models.Artifact) (CoveringFeature, bool)`.
     It calls `shipmentItems(shipment)` to get member IDs (reusing the read-edge
     normalizer per f015), resolves each with **`bldb.GetItem`** (a pure read —
     **not** `loadArtifact`, which upserts the index on a cache miss), and returns
     the first item that is `ArtifactType == "feature"` **and** a root feature
     (dotless ID; corroborated by `Level == 1`), in parent-first `items` order.
     Error handling: on `errors.Is(err, blerrors.ErrNotFound)` skip the item and
     `slog.Debug("covering-feature: item not found", "shipment_id", …, "item_id", …)`;
     on any **other** error, `slog.Warn` and skip (never silently swallow —
     Principle I/V). Zero root features → `(CoveringFeature{}, false)`. Pure read:
     no persist, no upsert, no mutation of the input artifact.
  3. A shared shaping helper in `internal/core` that returns a typed envelope
     which **embeds `*models.Artifact`** (so JSON field promotion preserves the
     full existing shipment shape — id/title/status/artifact_type/custom_fields/
     level/hierarchy_path/timestamps — with zero risk of dropping fields) and adds
     a **pointer** field `CoveringFeature *CoveringFeature `json:"covering_feature,omitempty"``.
     The pointer is `nil` when `ok == false`, so `omitempty` **actually omits** the
     object (note: `omitempty` on a non-pointer struct is a no-op and would emit
     `{"id":"","title":""}` — a pointer or conditional construction is required).
     Both CLI and MCP use this one shaper so field name, `omitempty` policy, and
     semantics have a single source of truth. Use `slog.DebugContext`/`WarnContext`
     (context-aware, matching the `internal/core` convention) for the skip logs.
* **Files:** `internal/core/shipment_covering.go` (new) + `internal/core/shipment_covering_test.go` (new).
* **Tests (test-first, ≤4 scenarios):**
  1. Single-feature manifest → returns that feature's ID + title, `ok=true`.
  2. Manifest with a root feature **and** a nested (dotted-ID) feature → returns the root covering feature (dotless ID), not the nested one.
  3. Zero features (tasks-only manifest) → `(CoveringFeature{}, false)`.
  4. Item ID that resolves to `ErrNotFound` (missing/archived) → skipped defensively via `GetItem`; still resolves the valid covering feature (or `ok=false` if none). Assert **no upsert** occurred (pure read).
* **Posture:** test-first.
* **Milestone:** `go test ./internal/core/...` green; the resolution path performs **no DB write** (uses `GetItem`, not the upserting `loadArtifact`) and does not mutate the input shipment.

### Unit 2 — CLI shipment views surface covering feature

* **What:** In `internal/cli/shipment.go`, surface the covering feature in
  `shipment list` and `shipment get` using `core.DeriveCoveringFeature` +
  the shared shaper:
  * **`shipment list` human-readable (table/tile):** add a `COVERING FEATURE`
    column (rendered as `<id> — <title>`, empty when `ok=false`). Build the
    shipment column set by **appending** the column to a **copy** of the shared
    `artifactColumns` (composing over `artifactsToRows`) so the shared base cannot
    drift — do **not** mutate the shared `artifactColumns`/`artifactsToRows` in
    `list.go` (they also drive `list` and `queue`).
  * **`shipment list --json` and `shipment get` (JSON):** emit a **top-level**
    `covering_feature: { "id": …, "title": … }` object alongside the shipment
    record via the shared shaper. The object is **omitted** (`omitempty`) when
    `ok=false`. The derived data is **never** written into `custom_fields` — it is
    a sibling, clearly-derived field, so it cannot round-trip through any
    `custom_fields` write path.
  * If any Cobra `Short`/`Long`/flag text is added to explain the column, that
    help content must live in the command's **`Long:` field** (never edited into
    the generated docs) so `make docs` reproduces it (CLI Reference Drift rule).
* **Files:** `internal/cli/shipment.go` + `internal/cli/shipment_test.go`.
* **Tests (test-first, ≤4 scenarios):**
  1. `shipment list --format table` includes a `COVERING FEATURE` column showing `<feature-id> — <title>`; shared `list`/`queue` columns are unchanged.
  2. `shipment list --format json` and `shipment get --json` include a top-level `covering_feature` object with `id`/`title`; `custom_fields` is byte-for-byte unchanged (no derived keys leak in).
  3. Zero-feature shipment → column empty and `covering_feature` **omitted** from JSON (no panic).
  4. **Read-only regression:** after `shipment get`/`list`, the shipment's stored file + DB row are unchanged.
* **Posture:** test-first.
* **Milestone:** `go test ./internal/cli/...` green; **CLI Reference Drift**: if any Cobra `Short`/`Long`/flag changed, run `make docs` and commit regenerated `docs/cli-reference/*.md` (see Quality Gates).
* **Depends on:** Unit 1.

### Unit 3 — MCP shipment tools surface covering feature

* **What:** In `internal/mcp/tools.go`, surface the covering feature in
  `handleListShipments` and `handleGetShipment` using `core.DeriveCoveringFeature`
  + the shared shaper: after the existing normalization
  (`normalizeShipmentItems` for list), emit a **top-level** `covering_feature`
  object (`id`,`title`) alongside each shipment in the response — **not** inside
  `custom_fields` — via the same shared shaper the CLI uses (identical field
  names/semantics). Omitted (`omitempty`) when `ok=false`. Preserves the existing
  raw-Artifact fields and the list==get same-shape contract. Update the
  `backlogit_get_shipment` / `backlogit_list_shipments` tool **descriptions** to
  note `covering_feature` is a **read-only, render-time derivation** (not editable
  stored state).
* **Files:** `internal/mcp/tools.go` + `internal/mcp/shipment_covering_test.go` (new) or extend `shipment_response_test.go`.
* **Tests (test-first, ≤4 scenarios):**
  1. `handleGetShipment` and `handleListShipments` responses include a top-level `covering_feature` object for a shipment with a covering feature; `custom_fields.items` is still never null (preserves `TestListShipments_EmptyItems_NeverNull`).
  2. list==get **same-shape** for a shipment **with** a covering feature (extends `TestListShipments_SameShapeAsGetShipment`).
  3. list==get **same-shape** for a **zero-feature** shipment — both **omit** `covering_feature` (closes the `ok=false` parity gap).
  4. **Read-only regression:** after `handleGetShipment` and `handleListShipments`, the shipment's stored file + DB row are byte-for-byte unchanged (no `covering_feature` persisted anywhere) and no upsert occurs.
* **Posture:** test-first.
* **Milestone:** `go test ./internal/mcp/...` green; contract-consistency + same-shape tests pass; a dedicated **CLI==MCP parity test** asserts both surfaces emit identical `covering_feature` field names/semantics.
* **Depends on:** Unit 1.

## Dependency Graph

```
Unit 1 (core helper)
   ├── Unit 2 (CLI views)
   └── Unit 3 (MCP tools)
```

Unit 1 is the foundation. Units 2 and 3 depend on Unit 1 and are independent of
each other (parallelizable). No cycles.

## Decisions and Rationale

1. **Display ID + title (not ID alone).** The stash asks for "feature ID/title";
   title gives the reader immediate semantic context (e.g. `061-F — Shipment
   State Integrity`) that resolves the offset confusion the feature targets.
2. **Expose a segregated top-level `covering_feature` object — NOT keys inside
   `custom_fields`.** Placing derived display data inside the persisted-looking
   `custom_fields` map (which also holds the authoritative `items` binding) risks
   an **agent write-path bypass**: an agent that reads a shipment and echoes
   `custom_fields` back through a write/update tool would silently persist the
   derived keys into the manifest — the *exact* retroactive manifest mutation the
   `B8FF7590` determination forbids. It also conflates derived vs. stored state.
   The revised design emits `covering_feature: { id, title }` as a **sibling
   top-level field** produced by a response-shaping layer. Both CLI and MCP use
   the **same shared shaper**, so list==get same-shape holds. Explicit,
   provenance-clear named fields satisfy **Principle X (Agent Context Efficiency)**
   better than overloading `custom_fields`. (Revised after plan-review P1: model-
   boundary leak / write-path bypass.)
3. **Read-only is guaranteed structurally, not just by convention.** Because the
   derived data lives in a top-level field and **never** enters the persisted
   `custom_fields` map, it cannot round-trip through any `custom_fields` write
   path. Combined with pure `GetItem` reads (Decision 4), no code path in this
   feature writes anything. Symmetric regression tests on all four surfaces assert
   the stored file + DB row are unchanged and no upsert occurs — machine-verifying
   R3, the invariant that ties this feature to the `B8FF7590` determination.
4. **Resolve items with pure `bldb.GetItem`, not `loadArtifact`.** `loadArtifact`
   upserts the index on a cache miss (a DB write), which would falsify the
   read-only claim and couple display rendering to index-write semantics. The
   derivation uses `GetItem` directly and skips items that return `ErrNotFound`.
   (Revised after plan-review P2.)
5. **Do not touch shared CLI columns.** `artifactColumns`/`artifactsToRows` are
   shared with `list`/`queue`; the shipment column set is built by **appending**
   the covering-feature column to a **copy** of `artifactColumns` (Single
   Responsibility / Principle VI) so the shared base stays authoritative and
   cannot drift.
6. **Root-feature rule = dotless root ID (corroborated by `Level==1`),
   parent-first.** Verified against the indexed DB: root features are dotless and
   `level==1`; nested features are dotted and `level==2`. The dotless-ID predicate
   is `omitempty`-immune (independent of frontmatter `level` population). Zero root
   features → omitted `covering_feature` — surfaced as an absent covering feature
   (a manifest contract exception per
   `orphaned-tasks-without-parent-features-2026-04-10.md`), not silently masked.
7. **CLI `shipment get` stays JSON-only.** It has no human-readable/`--format`
   split today; adding one is out of scope (YAGNI). The top-level `covering_feature`
   field delivers the requirement on that surface. `shipment list` carries the
   human-readable column.
8. **Single shared derivation + shaping helper.** The derivation and the
   response-shaping are both in `internal/core`, consumed by CLI and MCP, so field
   names, `omitempty` policy, and semantics have exactly one source of truth. The
   envelope is a **typed struct that embeds `*models.Artifact`** and adds a
   `*CoveringFeature` pointer field (`covering_feature,omitempty`) — struct
   embedding preserves the full existing response shape and pointer-omit honors the
   pinned zero-feature contract; avoid marshal-to-`map[string]any`-and-inject (the
   top-level analogue of the `custom_fields` anti-pattern this plan removes). The
   core shaper stays minimal (a derivation + one envelope type); if presentation
   logic in core later grows beyond this one field, revisit extracting a dedicated
   shared presentation helper. Locked by a dedicated CLI==MCP parity test. (Revised
   after plan-review P2: duplicated boundary logic; P2/P3: envelope typing.)
9. **Return a named value object.** `DeriveCoveringFeature` returns
   `(CoveringFeature{ID,Title}, ok bool)` rather than two positional strings —
   self-documenting call sites and room to grow the shape without signature churn.

## Risks and Caveats

| Risk | Mitigation |
|---|---|
| Agent write-path bypass: derived data echoed back through a `custom_fields` write tool would persist into the manifest (violates R3 / B8FF7590) | **Eliminated structurally**: `covering_feature` is a top-level field and never enters `custom_fields`, so there is nothing in the persisted map to echo back. No write-path guard needed. |
| Enrichment accidentally persisted → silent manifest mutation | Derived data lives outside the persisted model; resolution uses pure `GetItem` (no upsert); symmetric read-only regression tests on all four surfaces assert stored file + DB row unchanged and no upsert |
| `loadArtifact` upserts the index on cache miss (a DB write) | Use `bldb.GetItem` directly for item resolution; skip on `ErrNotFound` |
| Swallowing real DB/IO errors as "no covering feature" | Skip only on `errors.Is(err, ErrNotFound)`; `slog.Warn` (and skip) on any other error — never silently swallow (Principle I/V) |
| `ok=false` shape ambiguity → parity/shape drift | Pin one behavior: **omit** the `covering_feature` object (`omitempty`) identically on all four surfaces; zero-feature list==get same-shape asserted by test |
| Derivation rule depends on `omitempty` `level` | Primary predicate is the **dotless root ID** (structural, `level`-independent); `Level==1` only corroborates |
| Extra `GetItem` lookups per shipment item slow down `list` on large workspaces | Resolve lazily, stop at the first root feature (parent-first → typically the first item); scope is small. Follow-up noted to batch-resolve if lists grow |
| CLI Reference Drift Check CI failure if command help/flags change without doc regen | New help text lives in the Cobra `Long:` field; quality gate runs `make docs && git diff --exit-code docs/cli-reference/` |
| CLI/MCP field-name divergence breaks parity | Single shared core shaper; a dedicated CLI==MCP parity test asserts identical field names/semantics |
| `items` contains an unresolvable/archived ID | Helper skips defensively and continues; never errors the whole list/get |

## Plan Hardening Signals (REQUIRED)

* **public API, schema, or contract change** — *present-but-low-risk / treated as
  ABSENT for hardening.* The CLI/MCP responses gain a single **additive, optional**
  top-level field (`covering_feature`, `omitempty`); no field is removed, renamed,
  or retyped; no persisted schema changes; existing consumers ignoring unknown
  fields are unaffected. This is a backward-compatible additive presentation change,
  not a breaking contract change. Parity + list==get same-shape are preserved by a
  dedicated parity test. Notably, the derived field is **outside** `custom_fields`,
  so it introduces no new persistable state and no write-path exposure.
* **security, auth, permission, or compliance-sensitive behavior** — ABSENT.
  Read-only display of already-visible backlog data; no auth/permission surface.
* **migration, backfill, destructive data/config action, or irreversible step** —
  ABSENT. No data mutation of any kind; fully reversible (revert = remove display).
* **external integration, operator checkpoint, or external dependency** — ABSENT.
  No new dependency; derivation uses existing in-workspace core helpers.
* **high runtime, rollout, or rollback risk** — ABSENT. Trivial rollback; the
  only runtime cost is bounded per-shipment item lookups noted under Risks.

**Requires plan hardening: no.** No destructive, migration, security, irreversible,
or high-risk-rollout signal is present. The single additive-contract nuance is
backward-compatible and is addressed by explicit parity + read-only regression
tests rather than by a hardening pass.

## Runtime Verification and Closure

* **Changed runtime surfaces:** CLI (`shipment list`, `shipment get`) and MCP
  (`backlogit_list_shipments`, `backlogit_get_shipment`).
* **Runtime verification (for Ship to prove before absorption):**
  * `go run ./cmd/backlogit shipment list --format table` shows a `COVERING
      FEATURE` column; `--format json` and `shipment get <id>` include a top-level
      `covering_feature` object — verified against `057-S` (→ `058-F`) and the
    new session shipment.
  * `git status`/`git diff` on `.backlogit/**` shipment manifests is **clean**
      after running the views, `backlogit doctor` reports no issues, and no index
      upsert occurs — demonstrating the read-only invariant end to end.
* **Operational closure:** regenerate `docs/cli-reference/*.md` via `make docs`
  and commit alongside the Go change if any command metadata changed; ownership =
  Ship agent for execution; validation window = the shipment's CI run
  (build + unit tests + CLI Reference Drift Check green).

## Quality Gates (for Ship execution)

* `go build ./...` and `go test ./...` green.
* `make docs && git diff --exit-code docs/cli-reference/` clean (CLI Reference
  Drift Check) — **required** if any Cobra `Short`/`Long`/flag on shipment
  commands changed. New help text MUST live in the command's `Long:` field, never
  edited directly into the generated `docs/cli-reference/*.md`.
* Read-only regression tests present and passing on **all four surfaces** (CLI
  list/get, MCP list/get): no `covering_feature` written to any manifest or DB
  row, and no index upsert during derivation.
* A dedicated CLI==MCP parity test asserts both surfaces emit an identical
  top-level `covering_feature` field (same name, same `id`/`title` semantics,
  same `omitempty` behavior on zero-feature shipments).

<!-- plan-review-attempt: 2 -->

## Plan Review

### Attempt 1 — Gate: FAIL

Multi-persona review (Constitution, Go, Scope Boundary, Architecture Strategist,
Agent-Native Parity, Learnings Researcher). Merged findings, conservative
severity on disagreement.

**Blocking (drove FAIL):**

* **P1 — Derived data in `custom_fields` → agent write-path bypass + model-boundary
  leak.** (Agent-Native Parity P1; Architecture Strategist P2; Learnings Researcher
  P1.) Injecting `covering_feature_*` into the persisted-looking `custom_fields`
  map (which also holds the authoritative `items` binding) lets an agent echo it
  back through a write tool and persist it — the retroactive manifest mutation
  B8FF7590 forbids. **Resolved:** emit a segregated **top-level** `covering_feature`
  object via a shared shaper; never touch `custom_fields` (Decisions 2–3, 8).
* **P1 — `ok=false` shape ambiguity ("omit or leave empty").** (Parity P1; Go P2.)
  Divergence risks CLI/MCP shape drift and heterogeneous list shape. **Resolved:**
  pin a single behavior — **omit** the object (`omitempty`) identically on all four
  surfaces; added zero-feature list==get same-shape test (Decision 6; Unit 3 test 3).

**Non-blocking, folded into the revision:**

* **P0 (Constitution) → FALSE POSITIVE.** The reviewer claimed `feature && Level==1`
  fails because archived `058-F.md` frontmatter omits `level`. **Verified against
  the indexed DB** (`backlogit query`): `058-F` and 72 root features are `level==1`;
  nested features are `level==2`. The code path reads the **index**, not raw
  frontmatter. Rule is correct; **hardened** anyway with a `level`-immune dotless-
  root-ID predicate (Decision 6).
* **P2 — `loadArtifact` upserts on cache miss (not pure).** (Go, Architecture,
  Constitution.) **Resolved:** resolve via `bldb.GetItem`; milestone reworded
  (Decision 4; Unit 1).
* **P2 — Error-swallowing hides real failures.** (Go, Constitution.) **Resolved:**
  skip only on `ErrNotFound`; `slog.Warn` others (Unit 1; Risks).
* **P2 — Read-only regression asymmetry (MCP-get only).** (Constitution, Parity,
  Scope.) **Resolved:** symmetric read-only + no-upsert regression on all four
  surfaces (Units 2–3; Quality Gates).
* **P2 — Duplicated injection/shaping logic across CLI+MCP.** (Parity, Architecture.)
  **Resolved:** single shared core shaper + CLI==MCP parity test (Decision 8).
* **P1/P2 (Learnings) — Missing citations + parity test + drift rule.** **Resolved:**
  cited `orphaned-tasks-without-parent-features` and `empty-string-vs-sentinel`;
  added parity test and "help text lives in Cobra `Long`" rule.
* **P3 — Observability on skip; shipment-column drift; MCP tool-description update;
  value-object return.** Adopted (Units 1–3; Decisions 5, 9).
* **P3 (deferred, out of scope) — Consolidate `mcp.normalizeShipmentItems` into
  `core`.** Recorded as a follow-up; not expanded into this feature to avoid scope
  creep.

**Positive confirmations:** `internal/core` is the right home (no import cycles);
T1→T2/T3 sequencing sound; 3-task decomposition right-sized; shipment-local column
decision correct; YAGNI deferrals (`shipment get` JSON-only, no wrapper churn, no
premature batch-lookup) correct; scope tight; `f015` normalization citation apt.


### Attempt 2 — Gate: PASS (advisory)

Focused re-review of the revised plan by the three personas whose attempt-1
findings drove the FAIL (Agent-Native Parity, Architecture Strategist, Go
Reviewer). All returned **PASS (advisory)**.

* **Both attempt-1 P1s structurally resolved:** derived data is now a segregated
  top-level `covering_feature` object (a shared core envelope embedding
  `*models.Artifact` + a `*CoveringFeature` pointer), never inside `custom_fields`
  — the write-path bypass is eliminated (cross-checked: `handleUpdateItem` does not
  re-persist arbitrary `custom_fields`). Zero-feature behavior is pinned to
  **omit** and test-locked identically across all four surfaces.
* **All attempt-1 P2s resolved and source-verified:** pure `bldb.GetItem`
  (no upsert), `ErrNotFound`-only skip with `slog.WarnContext` on other errors,
  symmetric read-only regression on all four surfaces, single shared shaper +
  CLI==MCP parity test. list==get same-shape and items-never-null preserved.

**Advisory items folded into the plan before proceeding:**

* **P2 (Go/Parity) — `omitempty` on a value struct is a no-op.** Folded: Unit 1
  and Decision 8 now mandate a `*CoveringFeature` **pointer** (nil on `ok=false`)
  so the object is truly omitted.
* **P3 — Preserve full response shape.** Folded: the envelope **embeds
  `*models.Artifact`** rather than hand-listing fields.
* **P3 — Context-aware logging.** Folded: `slog.DebugContext`/`WarnContext`.

**Remaining P3 advisories (non-blocking, for Ship execution):**

* Deferred `mcp.normalizeShipmentItems` → `core` consolidation should be tracked
  as a low-priority follow-up stash (not lost as a mere plan note). Stage will
  record it.
* `covering_feature` is intentionally a **read-view-only** enrichment (get/list on
  both surfaces); create/claim/ship action confirmations do not carry it by design
  (Decision 7 scope).

**No residual P0/P1/P2 blocks the harvest gate.** Gate: **PASS**. Proceed to harvest.
