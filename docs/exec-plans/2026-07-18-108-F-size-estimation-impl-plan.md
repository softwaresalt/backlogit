---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for extending optional size estimation to feature and shipment artifacts: canonical custom_fields.size at all levels, an artifact-codec bridge proven by a round-trip test, event-before-write fail-closed exactly-once provenance, computed-on-read composition rollups, CLI/MCP mutation and read parity, and two-layer containment hardening of the size seam.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md
title: '108-F: Size estimation for feature and shipment (custom_fields.size + provenance + composition)'
---

## Source

- **PROCEED decision (AUTHORITATIVE):**
  `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
  (096-S / 109-F size-extension architecture spike, `conclusion: proceed`,
  confidence high on placement, medium on provenance/aggregation). This plan
  **implements** the decisions that spike recorded; it does not re-decide them.
- **Model A decision:**
  `docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md`
  (deliberation `052-DL`, feature `110-F`, archived). Model A scopes
  `docline.backlogit.*` to **documents** and explicitly delegated the
  `.backlogit`-artifact bridge selection to the size spike. The spike selected
  `custom_fields` (no carrier bridge).
- **Feature `108-F`** — restaged `blocked → active` (2026-07-18). The top
  `restage-2026-07-18` and `reconciliation-model-a` blocks in its body are
  authoritative; the older Model-B narrative is VOID. The four existing child
  placeholders (`108.001-T`..`108.004-T`) are unauthorized and are re-scoped or
  superseded by this plan.
- **Spike charter (context, not a plan):**
  `docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md`
  (`:183-193` exactly-once mandate; `:652-664` proceed authorization).

### Authoritative decisions carried in from the spike (do not re-open)

| # | Decision | Spike anchor |
|---|---|---|
| D1 | **Canonical size-location = `custom_fields.size` at all levels.** Task already there; extend to `feature` and `shipment`. Coexist; **no migration**. | §9(e), Recommendation 1 |
| D2 | **Inheritance bridge = no `models.Artifact` carrier; store under `custom_fields`.** The docline-carrier bridge is rejected for now (documented, reversible). | §9(d), Recommendation 2 |
| D3 | **Provenance fields = `size_source` {human, agent, derived} and `size_ruleset_version` (null until a ruleset is owned), under `custom_fields`, validated at the size seam.** Absent `size_source` reads as `unknown`/legacy and is **never rewritten as `human`**; unknown values are **rejected** with the same category as an invalid size. | §7, §9(f), Provenance |
| D4 | **Durability policy = event-before-write, fail-closed, exactly-once.** The estimate-history event appends **first** (gate-evidence precedent); an append failure **refuses** the size write, so no persisted size/provenance change ever lacks its event, and no orphan event lacks a persisted change. Crash-atomicity across append+write is implemented here. | §9(a), Provenance |
| D5 | **Composition = computed-on-read, never persisted.** XS–XL count histogram + `unsized` count + de-duplicated canonical members array; `ruleset_version = null`. Feature membership = children by `parent_id`; shipment membership = `NormalizeShipmentItems`. Missing member → `unsized`; skip `ErrNotFound`. Comparator XS<S<M<L<XL mirrors the priority `CASE` ordered-enum. | §8, §9(b) |
| D6 | **Containment = two-layer fix.** (1) reject lexical `..`/absolute escape in `QueueLayout.RootDir` and every configured search root at config-load time; (2) realpath/`EvalSymlinks` re-containment at lookup time before `parseFile` reads a candidate. Mirror `internal/core/doctor_target.go:230-280`. `SafeResolve` alone is insufficient (lexical only). | §6, Recommendation 3 |

## Problem Frame

backlogit's **generic artifact codec** captures only `custom_fields` and re-emits
only struct fields plus `custom_fields`:

- `models.ArtifactFromFrontmatter` (`internal/models/frontmatter.go:45-123`) maps
  the enumerated `Artifact` struct fields plus `custom_fields`; `models.Artifact`
  (`internal/models/artifact.go:33-55`) has **no** docline/extension carrier.
- `core.WriteArtifactFile` (`internal/core/artifacts.go:682-735`) rebuilds
  frontmatter from struct fields only, appending `custom_fields` if non-nil.
- The inline map builder in `createArtifact` (`internal/core/artifacts.go` ~`:289`)
  duplicates that emission logic — a **drift risk** the spike flags as a
  consolidation opportunity (`models.Artifact.ToFrontmatterMap()` shared by both
  write paths).

Consequently any **top-level `docline` map is DROPPED** on `.backlogit`
artifacts, while `custom_fields` survives every currently-wired mutation path.
The size mutation **seam** already exists —
`core.SetArtifactSize` (`internal/core/artifact_size.go:16-134`) — and writes
`custom_fields.size` via the body-preserving `mdfront` codec, with a **seam-only**
enum check (`validateSizeValue`, `:97-122`) that is *intentionally not*
retrofitted into `ValidateArtifactFields`. Today `size` is defined **only** on
`task` (`.backlogit/header-def.yaml:63-71`, `internal/config/defaults.go:53-69`);
`feature` and `shipment` define no `size`, and no `size_source`,
`size_ruleset_version`, or estimate-history field exists anywhere.

The work is therefore: (1) define the fields on `feature`/`shipment` (and
provenance fields at all levels); (2) prove `custom_fields.size` durability on
those levels with an executable round-trip test and remove the two-emitter
drift; (3) persist provenance with an exactly-once estimate-history event; (4)
compute (never persist) composition rollups; (5) reach CLI/MCP mutation and read
parity; (6) close the containment gap in the seam's lookup/write path; (7)
document the contract.

## Requirements Trace

| Requirement (from spike) | Implementation action | Unit |
|---|---|---|
| D1 canonical `custom_fields.size` on feature/shipment | Add `size` enum to `feature`/`shipment` header-def types + defaults; enable seam validation for those types | SE-1 |
| D3 provenance field definitions | Add `size_source` (enum) and `size_ruleset_version` (**bounded**, not free-text) to task/feature/shipment header-def | SE-1 |
| D2 no carrier bridge; durability of `custom_fields.size` | Consolidate the two frontmatter emitters into `models.Artifact.ToFrontmatterMap()` (preserving status-gated archive keys/`links`); keep the size seam the sole `custom_fields.size` writer (merge-not-replace, reserved keys); prove with an executable round-trip test | SE-2 |
| D4 exactly-once provenance | Typed `SizeMutation` seam that persists `size_source`/`size_ruleset_version`; append estimate-history event **before** the write, fail-closed | SE-3a |
| D3 actor-context stamping | New-authored size with no explicit `size_source` stamped from actor context (CLI ⇒ `human`, agent/MCP ⇒ `agent`); absent-on-read stays `unknown`, never rewritten | SE-3a / SE-5 |
| D4 crash-safety | Op-id-tagged, idempotent, reconciliation-not-truncation crash policy (no eventless write; orphan events doctor-reconciled) | SE-3b |
| D5 composition rollups | Add a computed-on-read, never-persisted histogram+unsized+members function for feature and shipment membership, with the XS<S<M<L<XL comparator | SE-4 |
| D3 mutation parity | Add `--size-source`/`--size-ruleset-version` (CLI) and `size_source`/`size_ruleset_version` (MCP) via `SizeMutation`; transport-aware error parity; reject agent human-masquerade | SE-5 |
| §7 read parity + D5 exposure | Project `custom_fields.size`/provenance identically on `get`/`get_item`/`get_queue`/`shipment get`; expose the derived composition on the MCP read surfaces (CLI human columns are a separate P2 follow-up) | SE-6 |
| D6 containment | Two-layer lexical + realpath containment on `RootDir`/search roots (config-load) and at lookup time (before `parseFile`) | SE-7 |
| Document the contract | Author/refresh the sizing contract doc (levels, provenance, durability, composition, parity, caveats) | SE-8 |

## Implementation Units

Each unit obeys the 2-hour rule (< 3 files, < 5 functions, < 4 test scenarios),
width isolation (single domain), and an atomic verifiable milestone.

### SE-1 — Size + provenance schema definition (config)

- **Changes:** In `.backlogit/header-def.yaml` and `internal/config/defaults.go`,
  add `size` (enum `XS/S/M/L/XL`, `optional: true`) to the `feature` and
  `shipment` types; add `size_source` (enum `human/agent/derived`,
  `optional: true`) and `size_ruleset_version` (**bounded** — enum with a
  currently-empty/`null`-only accepted set until a canonical ruleset is owned, or
  a bounded-string validation with max-length; **not** unbounded free text) to
  `task`, `feature`, and `shipment`. Bounding `size_ruleset_version` at the schema
  removes the free-text injection vector the review flagged.
- **Files:** `.backlogit/header-def.yaml`, `internal/config/defaults.go`.
- **Tests (unit, config/core):** header-def loads; `ResolveFieldSchema("feature")`
  and `("shipment")` return a `size` schema; `validateSizeValue(feature, "M")`
  passes and `(feature, "bogus")` fails with `ErrValidation`.
- **Execution posture:** test-first (add failing schema assertions, then extend
  the config).
- **Milestone:** `feature`/`shipment` accept a size enum at the seam; provenance
  fields are schema-known.

### SE-2 — Artifact-codec bridge + round-trip durability (core codec)

- **Changes:** Extract a single `models.Artifact.ToFrontmatterMap()` and route
  **both** `core.WriteArtifactFile` and the inline `createArtifact` emitter
  through it, eliminating the two-emitter drift (spike consolidation
  opportunity). **Preserve the two emitters' current field-set differences:**
  `WriteArtifactFile` emits `links` and **status-gated** `archived_from`/
  `archived_status` (only when `Status == Archived`), while the inline
  `createArtifact` emitter omits them — the shared emitter MUST keep archive
  provenance status-gated (protecting the "archive provenance ⇔ archived status"
  invariant) and must not silently start emitting `links` at create time.
  **Enforce the single-writer invariant at BOTH generic boundaries:** the size
  seam stays the **sole** writer of
  `custom_fields.size`/`size_source`/`size_ruleset_version`/`size_op_id` via
  **merge-not-replace**. A generic **update** carrying a `custom_fields` key MUST
  merge-preserve (not replace or delete) these **reserved** keys, or be rejected —
  closing the `updateArtifactUngated` whole-map-replacement hazard
  (`internal/core/artifacts.go:542-544`). A generic **create**
  (`WithFields`/`createArtifact`) MUST **reject or strip** the reserved sizing keys
  so an initial size can only be authored through `SizeMutation` (which emits the
  provenance event) — otherwise a create could persist a sizing change with **no**
  history event, violating Protected invariant #1. **Canonical projection rule
  (resolves the create-vs-write field-set question):** `ToFrontmatterMap()` emits
  **model state** with status-gated `archived_from`/`archived_status` and `links`;
  which model fields are *populated* is controlled by the caller (create leaves
  archive fields empty and `links` unset because the model has none yet), so the
  **single shared emitter** is field-set-consistent — there is no create-specific
  emitter branch. The reserved sizing keys are never sourced from the caller-passed
  `custom_fields` on either path.
- **Files:** `internal/models/artifact.go` (or `internal/core/artifacts.go`),
  `internal/core/artifacts.go`.
- **Tests (test-tier, extend `internal/core/docline_codec_roundtrip_test.go`;
  table-driven `t.Run` subtests count as one scenario for the 2-hour rule):**
  **MANDATORY executable round-trip** — create a `feature` and a `shipment` with
  `custom_fields.size` + `size_source`, read back, run an ordinary non-size field
  update, read back again, and assert `custom_fields.size` and `size_source`
  **survive** on both (assert on the frontmatter **map**, not body byte-identity —
  see the CRLF caveat); assert a top-level `docline` map is still dropped (guard
  unchanged); assert a generic update carrying a `custom_fields` key preserves the
  reserved size keys **and** a generic create carrying reserved sizing keys is
  rejected/stripped (create-path single-writer enforcement). **Red-phase gate:** a
  **post-refactor target** assertion that the two emitters produce
  field-set-equivalent frontmatter under the canonical projection rule (red before
  consolidation, green after). If the round-trip + create-guard + equivalence gate
  exceed a single 2-hour envelope, split the **characterization lock** into a
  sibling follow-up and keep SE-2 focused on the consolidation proof.
- **Execution posture:** characterization-first (lock the pre-refactor round-trip),
  then observe the two-emitter equivalence assertion **red**, then refactor to the
  shared emitter to turn it **green**.
- **Milestone:** `custom_fields.size`/provenance provably round-trips on
  feature/shipment through create + generic update; single emitter with preserved
  field-set semantics and enforced single-writer reserved keys.

### SE-3a — Provenance persistence + estimate-history event (core persistence)

- **Changes:** Replace the positional `SetArtifactSize(ctx, ws, id, size string)`
  signature with a **presence-aware typed command** —
  `SizeMutation{ Size, Source *string, RulesetVersion *string, Actor ActorContext }`
  — so callers can distinguish *omitted* from *cleared* from *set*, and so
  defaulting/validation is centralized at one boundary (resolves the four-adjacent-
  strings hazard and the omitted-vs-null ambiguity). The seam validates
  `size_source` against `{human, agent, derived}` and `size_ruleset_version`
  against a **bounded** set (accept only `null`/empty until a canonical ruleset is
  owned; unknown → `ErrValidation`) — never an unbounded free-text write.
  `ActorContext` is **derived at the transport boundary, not caller-supplied**:
  the CLI command layer constructs it as `human`; the MCP tool layer constructs it
  as `agent` (and rejects/overrides any caller-supplied `size_source: human`, per
  SE-5). It maps to the event's plain-string `Actor` field (`stream.go` `Event.Actor
  string`) via a single documented projection (`human`/`agent`/`derived` →
  identical string), so the seam needs no ad-hoc mapping.
  Persist all three under `custom_fields` (merge-not-replace). **Provenance
  defaulting has two distinct rules:** (a) a **new authored** size with no explicit
  `size_source` is stamped from the **actor context** (CLI human actor ⇒ `human`;
  agent/MCP transport ⇒ `agent`); (b) an **absent** `size_source` on **read** is
  `unknown`/legacy and is **never rewritten as `human`**. Append an
  **estimate-history** event (a new `EventType` constant + `Delta` payload
  convention on the existing generic `events.Event`; **no `stream.go` signature
  change**) under **event-before-write, fail-closed**: append the event **first**
  (gate-evidence precedent `internal/core/gate_evidence.go:34-58`); if the append
  fails, **refuse** the write — so no persisted size/provenance change ever lacks
  its event.
- **Files:** `internal/core/artifact_size.go`, `internal/events/stream.go`
  (new `EventType` constant only).
- **Tests (unit):** event appended before write; forced append failure ⇒ no
  persisted change (fail-closed); a `size_ruleset_version`-only change still emits
  **exactly one** event; absent `size_source` reads as `unknown`/legacy, never
  rewritten as `human`.
- **Execution posture:** test-first.
- **Milestone:** every persisted size/provenance change carries exactly one
  history event on the non-crash path; no eventless writes.

### SE-3b — Crash-safe append+write reconciliation (core persistence)

- **Changes:** Make the append+write pair crash-safe **without** the unsafe
  compensating-truncation the review rejected. The per-item JSONL is **shared**
  with non-size writers (comments, status hooks) that do **not** hold the per-task
  lock, so truncating its last line to "roll back" an orphan event can delete a
  legitimately-appended concurrent event (data loss) — **forbidden**. **Pinned
  protocol (all steps under the already-held per-task lock, on existing
  `atomicfile`/JSONL/`doctor` rails — no new dependency):**
  1. **op-id ingress:** `SizeMutation` carries an optional `OpID *string`; when nil
     the seam generates one. A retry reuses the same `OpID`, making the whole
     operation idempotent.
  2. **applied-check + predecessor capture (before append):** read the target
     artifact's current `custom_fields.size_op_id` — call it `PrevOpID` (empty for a
     never-sized artifact). If `PrevOpID` already equals **this** `OpID`, the write
     already landed — return success **without** appending a second event (satisfies
     "no duplicate event on retry").
  3. **append:** append the estimate-history event tagged with **both** `OpID` and
     the captured `PrevOpID` (an explicit predecessor link, so the event stream
     forms a per-artifact version chain) — fail-closed; append failure refuses the
     write.
  4. **atomic write:** persist `size`/provenance **and** set
     `custom_fields.size_op_id = OpID` (a reserved, system-managed key) in the same
     `mdfront` atomic write, so the artifact durably records which op last applied.
  Recovery is **reconciliation, not truncation**, and ordering is **decidable from
  the predecessor chain, not opaque ID equality**. For an event with `OpID`/`PrevOpID`,
  a `doctor` check (under the per-task lock) compares the artifact's **current**
  `size_op_id` `C`:
  - `C == OpID` ⇒ **already applied** (idempotent no-op);
  - `C == PrevOpID` ⇒ a **fresh orphan** whose write did not land — safe to
    **compare-and-swap apply** the orphan's intended state (the artifact is exactly
    at the event's declared predecessor, so nothing newer is overwritten);
  - any **other** `C` ⇒ the artifact has advanced past this op — the orphan is
    **stale/conflicting** and is left as benign, op-id-tagged audit residue, **never
    replayed**.
  This **refines** the spike's §9(a) exactly-once mandate into
  a concrete, safe form: the hard invariant is **"no persisted change without an
  event"**; orphan events are op-id-tagged, doctor-detectable, and never resolved by
  mutating the shared log. (See the revised Decisions and Invariant #1.)
- **Files:** `internal/core/artifact_size.go`, the existing `doctor` target
  (`internal/core/doctor_target.go` / `doctor` check) for reconciliation.
- **Tests (unit; table-driven `t.Run` subtests count as one scenario):** a write
  failure **after** a successful append leaves an orphan event carrying `OpID`+
  `PrevOpID` and **no** persisted change (fail-closed, no truncation of the shared
  JSONL); doctor **applies** a fresh orphan whose `PrevOpID` equals the artifact's
  current `size_op_id`, and **skips** a stale orphan whose `PrevOpID` does not
  (artifact already advanced); a retried op-id is idempotent — the applied-check
  short-circuits **before** the append, so no duplicate event and no double write.
- **Execution posture:** test-first (crash-injection at the post-append boundary).
- **Milestone:** the crash window is closed to "orphan-tolerant, op-id-reconciled";
  no eventless persisted change and no shared-log data loss.

### SE-4 — Computed-on-read composition rollups (core aggregation)

- **Changes:** Add a `SizeComposition` function that, given a feature or
  shipment, returns a **never-persisted** structure: an XS–XL count histogram, an
  `unsized` count, and a de-duplicated canonical-members array. Feature
  membership = direct children by `parent_id` (`internal/core/hierarchy.go`);
  shipment membership = `NormalizeShipmentItems`
  (`internal/core/shipment.go:525-571`); de-dup via `uniqueNonEmptyStrings` so a
  manifest listing a feature and its child tasks counts each item once. Missing
  member → `unsized`; skip `ErrNotFound`, warn+skip others
  (`DeriveCoveringFeature` precedent). Comparator XS<S<M<L<XL mirrors the priority
  `CASE` ordered-enum (`internal/core/queue.go:183-191`). `ruleset_version = null`.
- **Files:** new `internal/core/size_composition.go`.
- **Tests (unit):** (1) histogram + `unsized` counts for a feature with
  mixed/absent sizes **including a missing/`ErrNotFound` member** (skipped, not
  fatal); (2) feature+children de-dup counts each item once; (3) the function
  never writes to disk (no persistence side effects). Three scenarios keep SE-4
  inside the 2-hour envelope.
- **Execution posture:** test-first.
- **Milestone:** a pure, deterministic composition read exists; nothing persists.

### SE-5 — CLI/MCP mutation parity for provenance (interface)

- **Changes:** Add CLI flags `--size-source` and `--size-ruleset-version`
  (`internal/cli/update.go`) and MCP fields `size_source` /
  `size_ruleset_version` (`internal/mcp/tools.go`), both constructing the typed
  `SizeMutation` command (SE-3a) and routing through the single seam. **Trust
  boundary (transport-aware actor stamping):** the MCP handler MUST NOT let an
  agent transport claim `size_source: human` — an MCP-origin request that omits
  `size_source` is stamped `agent`, and an explicit `size_source: human` from the
  agent transport is **rejected** (or overridden to `agent`), so an agent cannot
  masquerade as a human author. The CLI stamps `human` from its actor context when
  the flag is omitted. Enforce transport-aware **error-category parity**: invalid
  enum (or human-masquerade rejection) ⇒ CLI validation error → **exit 1** (not
  `ExitError{Code:4}`, which is reserved for busy-lock conflicts) / MCP
  `validation_failed`.
- **Files:** `internal/cli/update.go`, `internal/mcp/tools.go`.
- **Tests (unit):** CLI flag and MCP field both reach the seam; an invalid
  `size_source` is rejected on **both** surfaces with the matching category; an
  MCP request asserting `size_source: human` is rejected/overridden; a CLI author
  with no flag is stamped `human`, an MCP agent with no field is stamped `agent`.
- **Execution posture:** test-first.
- **Milestone:** provenance is writable from both surfaces with parity errors and
  no cross-transport human-masquerade.

### SE-6 — CLI/MCP read projection parity (interface)

- **Changes:** Ensure `custom_fields.size`/provenance projects **identically** on
  the read pairs `get`↔`get_item`, `queue view`↔`get_queue`,
  `shipment get`↔`get_shipment`, `shipment list`↔`list_shipments` (MCP/JSON
  already carry `custom_fields`; ride `size` on `custom_fields`, **not** on any
  surface-specific context block). Expose the SE-4 composition on the MCP read
  surfaces (`get_item`/`get_shipment`/`get_queue`) as a derived, clearly
  non-persisted field. The **cosmetic CLI human-column** gap (`list`/`shipment
  list`/`queue view` columns omit size) is **out of scope for SE-6** and is filed
  as a **separate P2 backlog follow-up** so SE-6 stays two-file (MCP projection +
  CLI `--json`) within the 2-hour envelope.
- **Files:** `internal/mcp/tools.go`, `internal/cli/list.go`.
- **Tests (unit):** `get_item` JSON exposes `custom_fields.size` and the derived
  composition on a sized feature; CLI `--json` exposes `custom_fields.size`.
- **Execution posture:** test-first.
- **Milestone:** size + composition are visible with CLI/MCP JSON read parity.

### SE-7 — Containment hardening of the size seam (config / security)

- **Changes:** (1) At config-load, reject lexical `..`/absolute escape in
  `QueueLayout.RootDir` and every configured search root — today
  `internal/config/loader.go:77-85` guards only `reg.Directories`, leaving a
  symlink-independent lexical escape via `RootDir`
  (`internal/config/schema.go:99-104`). (2) At **lookup time**, realpath-resolve
  and re-contain each candidate **before** `parseFile` reads it (a leaf-file
  symlink is otherwise read out-of-workspace during the `FindArtifactPath` walk),
  then revalidate the resolved target before lock/read/write — mirroring
  `internal/core/doctor_target.go:230-280` (`EvalSymlinks` + realpath
  containment), **not** `SafeResolve` alone (lexical only) and not a write-only
  check (the read already happened during lookup). Also assert that `RootDir` and
  the configured search roots are **not** environment-variable-expanded (or reject
  expansion-based escape) so env expansion cannot reintroduce a traversal vector.
  **Deliberate width exception:** SE-7 spans config-load validation and
  lookup-time re-containment because D6 is a single atomic two-layer security
  invariant — splitting it would leave a half-closed traversal hole in an
  intermediate state. This is a documented exception, not width drift; SE-7 is
  targeted seam hardening (a broader workspace-wide containment audit, if wanted,
  is a separate backlog item).
- **Files:** `internal/config/loader.go` (or `schema.go` validation),
  `internal/core/artifacts.go` (lookup-time re-containment helper).
- **Tests (unit):** a `RootDir: "..\\..\\outside"` config is rejected at load; an
  env-expansion escape in a search root is rejected; a leaf-file symlink under a
  search dir is refused at lookup; an in-workspace path still resolves.
- **Execution posture:** test-first (add failing containment tests, then harden).
- **Milestone:** the size-lookup/write path cannot read or write outside the
  workspace via lexical `..` or symlink escape.

### SE-8 — Sizing contract documentation (docs)

- **Changes:** Author/refresh a durable sizing-contract document (levels and
  canonical `custom_fields.size`; provenance fields, values, defaulting,
  rejection; the event-before-write fail-closed exactly-once durability policy;
  the computed-on-read composition contract and comparator; CLI/MCP parity
  matrices; the map-replacement and validated-once caveats). Update
  `docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md`
  cross-references per spike Next Steps #1. Docs-only; no code.
- **Files:** `docs/` sizing-contract doc (+ cross-reference updates).
- **Tests:** docline frontmatter lints clean (`backlogit docs lint`); no code
  tests.
- **Execution posture:** documentation.
- **Milestone:** the contract is discoverable and consistent with the shipped
  behavior.

## Dependency Graph

Acyclic. Edges are `blocks` (target depends on source). SE-1 and SE-7 are the
two roots; SE-3a/SE-3b/SE-4 can be built in parallel once their inputs exist:

```text
SE-1 ─┬▶ SE-2 ─▶ SE-3a ─▶ SE-3b ─▶ SE-5 ─▶ SE-8
      │            └────────────┐
      └▶ SE-4 ──────────────────┼▶ SE-6 ─▶ (into SE-8)
SE-7 ─────────────▶ SE-3a       │
SE-7 ─────────────────────────────▶ SE-8
```

(SE-6 depends on SE-3a and SE-4; SE-8 depends on SE-5, SE-6, and SE-7. There is
**no** SE-2→SE-4 edge — SE-4 depends only on SE-1.)

Explicit edges to wire with `backlogit dep add <task> <depends_on> --type blocks`:

- SE-2 depends on SE-1
- SE-4 depends on SE-1 (composition consumes the schema contract; it does not
  require the emitter consolidation — de-serialized per review)
- SE-3a depends on SE-2 and SE-7 (durable custom_fields + a hardened seam before
  routing more provenance writes through it)
- SE-3b depends on SE-3a (crash-safety builds on the append+write path)
- SE-5 depends on SE-3b (surfaces provenance mutation on the crash-safe seam)
- SE-6 depends on SE-3a and SE-4 (read projection surfaces persisted size +
  composition)
- SE-8 depends on SE-5, SE-6, and SE-7 (documents the final contract)

Rationale for `SE-3a depends on SE-7`: harden the seam's containment **before**
routing additional provenance writes through it.

## Decisions and Rationale

- **`custom_fields.size`, not a top-level docline map (D1/D2).** The generic
  codec drops a top-level `docline` map on artifacts (empirically proven by the
  committed `docline_codec_roundtrip_test.go`); `custom_fields` is a recognized,
  preserved carrier and task size already lives there. The `models.Artifact`
  carrier bridge is rejected for now: the docline ext-schema structurally cannot
  validate artifact frontmatter (`allOf` base-v1 rejects
  `id`/`artifact_type`/`status`), documents carry no `size`, and doc-scoped
  ingestion does not read artifact size — so no validation or interoperability is
  forfeited. The bridge stays a documented, reversible future option.
- **Event-before-write, fail-closed, orphan-tolerant exactly-once (D4, refined
  by review).** The charter mandates exactly one history event per persisted
  provenance change, including `size_ruleset_version`-only changes
  (`plan:183-193`). Event-after-write / warn-continue could persist a size with
  zero events (an under-count) and is rejected. The gate-evidence precedent
  (append-before-durable-write, refuse on append failure) is the exact analogue.
  **Refinement (SE-3b):** true "no orphan event *and* no eventless write" via a
  compensating truncation of the **shared** per-item JSONL is **unsafe** — that
  log is appended by non-size writers that do not hold the per-task lock, so
  truncation can delete a concurrent legitimate event. The concrete, safe policy
  is therefore: the **hard** invariant is "**no persisted change without an
  event**"; the crash-window residue (an event whose write did not land) is an
  **op-id-tagged orphan** that is idempotently reconciled by a `doctor` check,
  never resolved by mutating the shared log. This preserves the charter's intent
  (no under-count, deterministic audit) without introducing data-loss.
- **Composition computed-on-read, never persisted (D5).** Persisting a derived
  rollup would re-introduce the "derived value masquerading as a human estimate"
  hazard the feature was raised to avoid. A histogram + `unsized` (not a single
  collapsed bucket) keeps the contract lossless; a single bucket is an explicit
  non-goal unless a later work item requests it.
- **Two-layer containment (D6).** `RootDir` admits a purely lexical `..` escape
  independent of symlinks, and a leaf-file symlink is read during lookup before
  any write-path check — so a write-only or lexical-only fix is insufficient. The
  realpath pattern already exists in `doctor_target.go` and is reused.
- **Single seam + single emitter.** Keeping `SetArtifactSize` the sole
  `custom_fields.size` writer and collapsing the two frontmatter emitters into
  `ToFrontmatterMap()` removes the drift that the map-replacement caveat would
  otherwise weaponize.

## Risks and Caveats

- **Map-replacement caveat:** `updateArtifactUngated` *replaces* the whole
  `custom_fields` map when an update carries a `custom_fields` key
  (`internal/core/artifacts.go:542-544`). Harmless today (no surface passes
  arbitrary `custom_fields`), but SE-2 must add a guard so this stays true, and
  SE-5 must never route provenance through a full-map passthrough.
- **Validated-once asymmetry:** `custom_fields.size` is durable on every wired
  path but validated **only** at the size seam (`validateSizeValue`), not in
  `ValidateArtifactFields`. SE-5 must keep both surfaces on the seam so no path
  writes an unvalidated size/provenance value.
- **Atomicity is the real work, split into SE-3a/SE-3b:** the exactly-once
  mandate is decided; the crash-atomic append+write mechanism is the open
  engineering detail and is isolated in **SE-3b**. The review **rejected**
  compensating truncation of the shared per-item JSONL (concurrent non-size
  writers do not hold the per-task lock — truncation can delete a legitimate
  event). The landed mechanism is therefore op-id-tagged, idempotent,
  reconciliation-not-truncation: the hard invariant is "no persisted change
  without an event"; orphan events are doctor-detectable and benign. Any residual
  crash-window is explicitly flagged for runtime verification.
- **CRLF/normalization drift:** the generic `ParseFrontmatter` normalizes
  CRLF→LF document-wide while `mdfront` preserves body bytes; round-trip tests
  (SE-2) must assert on the frontmatter map, not byte-identity of the body.
- **Cosmetic read gap is non-blocking and out of SE-6:** the CLI human-column
  size omission is filed as a **separate P2 backlog follow-up**, not part of SE-6,
  so SE-6 stays two-file (MCP projection + CLI `--json`) inside the 2-hour
  envelope; JSON/MCP parity is the SE-6 deliverable.

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change — **present** (header-def schema on
  feature/shipment; new provenance fields; new MCP/CLI surface; composition read
  contract).
- security, auth, permission, or compliance-sensitive behavior — **present**
  (SE-7 workspace-containment hardening; path traversal / symlink escape).
- migration, backfill, destructive data/config action, or irreversible step —
  **present-adjacent** (no migration by decision — coexist — but a durable
  event-stream append with a fail-closed refusal path and crash-atomicity is an
  irreversible-write concern; legacy absent-provenance interpretation must not be
  rewritten).
- external integration, operator checkpoint, or external dependency — absent.
- high runtime, rollout, or rollback risk — **present** (the size seam is a live
  mutation path; the atomic append+write and containment change its failure
  semantics).

**Requires plan hardening: yes**

## Runtime Verification and Closure

| Unit | Runtime surface? | Verify before absorbed | Closure artifact |
|---|---|---|---|
| SE-1 | No (config) | header-def loads; seam validates feature/shipment size | schema note in the contract doc |
| SE-2 | No (codec) | round-trip test green on feature+shipment; single emitter | round-trip regression guard is the closure evidence |
| SE-3a | Yes (mutation seam) | event-before-write proven; forced-append-failure refuses write; exactly-once on ruleset-only change; actor-context stamping (CLI human / agent) proven | rollback trigger: any persisted size without an event ⇒ revert; owner + validation window |
| SE-3b | Yes (crash recovery) | post-append crash leaves op-id orphan + no persisted change; doctor reconciles; retried op-id idempotent; **no** shared-JSONL truncation | rollback trigger: any shared-log truncation or duplicate event ⇒ revert |
| SE-4 | No (pure read) | composition deterministic; never persists | n/a |
| SE-5 | Yes (CLI + MCP) | both surfaces reach the seam; parity error categories; agent human-masquerade rejected | parity matrix in contract doc |
| SE-6 | Yes (CLI + MCP read) | size + composition visible; JSON/MCP parity (CLI human columns deferred to P2) | read-parity matrix in contract doc |
| SE-7 | Yes (lookup/write path) | lexical `..` config rejected; env-expansion escape rejected; symlink lookup refused; in-workspace still resolves | containment regression tests; rollback trigger: any out-of-workspace read/write |
| SE-8 | No (docs) | docline lint clean | the doc itself |

When SE-3a/SE-3b and SE-7 are hardened (below), the downstream Ship
runtime-verification and operational-closure steps should carry the fail-closed
refusal path, the op-id crash-reconciliation note, and the containment invariants
as explicit checks.

## Constitution Check

Mapping this plan against `.github/instructions/constitution.instructions.md`.
(This section closes the known governance gap that no prior artifact recorded a
constitution mapping for 108-F.)

| Principle | Compliance |
|---|---|
| **I. Safety-First Go** | All units wrap errors with `fmt.Errorf("...: %w", err)`; provenance rejection reuses the `ErrValidation` sentinel; no `unsafe`. Gates (`go vet`, `golangci-lint`, `gofmt`) run in Ship, not Stage. |
| **II. Test-First (NON-NEGOTIABLE)** | Every code unit is test-first / characterization-first; SE-2 mandates an **executable round-trip test** as the durability proof; the harness must be red before implementation. |
| **III. Workspace Isolation** | SE-7 *strengthens* isolation (two-layer lexical + realpath containment). No secrets are added. |
| **IV. CLI Containment (NON-NEGOTIABLE)** | SE-7 enforces cwd/workspace containment at config-load and lookup time; no unit writes outside the workspace root. |
| **V. Structured Observability** | SE-3a adds a durable estimate-history event stream (traceable provenance) — an observability *gain*; SE-3b keeps it consistent under crash via op-id reconciliation. |
| **VI. Single Responsibility** | No new external dependency (SE-3b reuses existing `atomicfile`/JSONL/`doctor` rails); SE-2 *reduces* surface by collapsing two emitters into one. |
| **VII. Destructive Command Approval** | No destructive terminal commands. The durable append is fail-closed, not force-overwrite; SE-3b explicitly forbids truncating the shared log. |
| **VIII. Safety Modes** | Elevated blast radius (mutation seam + containment) is flagged; `Requires plan hardening: yes`; Ship should operate in careful/investigate-first posture for SE-3a/SE-3b/SE-7. |
| **IX. Git-Friendly Persistence** | `custom_fields.*` stays human-readable YAML frontmatter; `mdfront` preserves body bytes and semantic ordering. |
| **X. Context Efficiency** | Composition is computed-on-read via targeted membership queries, not bulk scans; MCP read projection returns structured `custom_fields`. |
| **XI. Merge Commit History** | Not applicable to planning; Ship enforces merge-commit strategy at PR time. |

**Justified deviations:** none. No principle requires a documented violation.
The map-replacement and validated-once **asymmetries** are pre-existing codebase
constraints carried as caveats (Risks), not new violations introduced by this
plan.

## Plan Hardening

**Hardening required: yes.** The plan shows four active hardening signals
(schema/contract change, security-sensitive containment, irreversible durable
append, high runtime/rollback risk). This section deepens verification,
rollback, and guardrail detail beyond the base plan and applies the
`strict-safety` `ProposedAction` / `ActionRisk` / `ActionResult` vocabulary to
the risky units.

### Context consulted

- `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
  (§6 containment, §9(a) durability) — the authoritative source of the hardened
  invariants.
- `internal/core/gate_evidence.go:34-58`, `internal/core/gate_transition.go:231-279`,
  `internal/core/shipment_gate.go:490-499` — the append-before-durable-write,
  fail-closed precedent SE-3a reuses.
- `internal/core/doctor_target.go:230-280` — the realpath/`EvalSymlinks`
  containment pattern SE-7 mirrors.
- `internal/core/commits.go:27-97`, `internal/events/stream.go:39-46`,
  `internal/db/logs.go:32-40` — event-append durability + independent timestamp
  stamping, relevant to SE-3a exactly-once.
- Compound library: no existing `docs/compound/` learning contradicts this plan;
  the closest prior art is the fail-closed / "absence is not a pass" family
  (e.g. `exported-cache-zero-value-bypass`) — SE-3a's fail-closed refusal and
  SE-7's "unknown path ⇒ refuse" both apply that governing rule.

### Protected invariants

1. **Exactly-once provenance (orphan-tolerant):** no persisted
   `custom_fields.size` / `size_source` / `size_ruleset_version` change may exist
   without an estimate-history event (the **hard** invariant). The reciprocal
   ("no event without a persisted change") is enforced as **op-id reconciliation,
   not shared-log truncation**: a crash-residue orphan event is op-id-tagged and
   doctor-reconciled, never resolved by mutating the shared JSONL. (SE-3a/SE-3b)
2. **Sole writer (create + update):** `SetArtifactSize` remains the only writer of
   `custom_fields.size`/`size_source`/`size_ruleset_version`/`size_op_id`; no
   generic **create** or **update** path may inject or replace these reserved keys
   (create rejects/strips them; update merge-preserves them), and
   `ToFrontmatterMap()` preserves status-gated `archived_from`/`archived_status`
   and `links`. (SE-2, SE-5)
3. **Legacy non-rewrite:** an absent `size_source` reads as `unknown`/legacy and
   is never rewritten as `human`; agent/MCP transports may not stamp `human`.
   (SE-3a, SE-4, SE-5)
4. **Containment:** no size lookup or write may read or write outside the
   workspace root via lexical `..`, absolute path, or symlink escape. (SE-7)
5. **No-persist derived:** composition rollups are computed-on-read and never
   written to disk. (SE-4)

### Risky actions (ProposedAction / ActionRisk / ActionResult)

| # | ProposedAction | targets | change_kind | ActionRisk | rollback | approval | ActionResult |
|---|---|---|---|---|---|---|---|
| PA-1 | Route provenance writes through an event-before-write, fail-closed, **op-id-reconciled** append+write in the live size seam (SE-3a persists+events; SE-3b crash-safety) | `internal/core/artifact_size.go`, `internal/events/stream.go`, `internal/core/doctor_target.go` | durable append + mutation (contract) | **high** | revert the seam commit; the append is additive to the JSONL event stream and index-rehydratable; a failed write leaves the prior size intact (fail-closed); orphan events are op-id-tagged and doctor-reconciled, **never** resolved by truncating the shared log | prefer approval (production mutation-path + failure-semantics change) | planned |
| PA-2 | Add two-layer lexical + realpath containment at config-load and lookup time | `internal/config/loader.go`, `internal/core/artifacts.go` | config validation + security guard | **high** | revert; new rejections are fail-closed (worst case: a previously-accepted escaping config is now refused — surface a clear diagnostic) | prefer approval (changes accepted-config surface) | planned |
| PA-3 | Extend header-def with size on feature/shipment + provenance fields | `.backlogit/header-def.yaml`, `internal/config/defaults.go` | schema/contract | **moderate** | revert; fields are `optional`, additive, backward-compatible (coexist, no migration) | standard review | planned |
| PA-4 | Collapse two frontmatter emitters into `ToFrontmatterMap()` | `internal/models/artifact.go`, `internal/core/artifacts.go` | shared-code refactor | **moderate** | revert; guarded by the SE-2 round-trip test asserting on the frontmatter **map** (not body byte-identity — the generic parser normalizes CRLF→LF) plus the two-emitter field-set-equivalence gate | standard review | planned |

No `ActionRisk: destructive` step exists — there is no deletion, force-overwrite,
or history rewrite. The durable append is fail-closed, not destructive.

### Deepened runtime verification (SE-3a/SE-3b, SE-7)

- **SE-3a environment prechecks:** confirm the event stream is writable and
  index-rehydratable before enabling the fail-closed refusal; run the seam under
  an injected append-failure to prove the write is refused (no partial state).
- **SE-3a target scenarios:** (a) normal size set ⇒ one event + persisted change;
  (b) append fails ⇒ zero events + zero persisted change;
  (c) `size_ruleset_version`-only change ⇒ exactly one event; (d) actor-context
  stamping — CLI-authored new size ⇒ `size_source: human`, agent/MCP ⇒ `agent`,
  agent-supplied `human` rejected/overridden.
- **SE-3b target scenarios:** (a) crash injected after append, before write ⇒
  orphan event carrying `OpID`+`PrevOpID` + no persisted change (fail-closed);
  (b) doctor **applies** a fresh orphan (`PrevOpID == current size_op_id`) and
  **skips** a stale one (artifact advanced); (c) retried op-id is idempotent
  (applied-check short-circuits before append); (d) the shared per-item JSONL is
  **never** truncated.
- **SE-7 target scenarios:** (a) `RootDir: "..\\..\\outside"` rejected at load;
  (b) env-expansion escape in a search root rejected; (c) leaf-file symlink under
  a search dir refused at lookup (before `parseFile`); (d) legitimate in-workspace
  artifact still resolves and writes.

### Operational closure detail

- **Monitoring signals:** count of estimate-history events vs. count of persisted
  size changes should stay consistent under the orphan-tolerant policy (events ≥
  persisted changes; any *persisted change with no event* is the alert
  condition); orphan events are op-id-tagged and reconcilable, not alerts. Any
  containment-refusal log line during normal operation indicates a
  misconfiguration to investigate.
- **Rollback triggers:** (1) any persisted size change observed without a
  matching event ⇒ revert SE-3a/SE-3b; (2) any shared-JSONL truncation or
  duplicate event ⇒ revert SE-3b; (3) any out-of-workspace read/write observed ⇒
  revert SE-7 and treat as a security incident; (4) round-trip test regression on
  feature/shipment ⇒ revert SE-2.
- **Rollback procedure:** each unit is a coherent, revertible commit; the schema
  and provenance additions are optional/backward-compatible, so a revert cannot
  strand data (existing `custom_fields.size` values remain valid).
- **Owner / validation window:** the Ship agent owns runtime verification and
  closure; validate SE-3a/SE-3b and SE-7 across a full create→update→read cycle on
  a scratch feature and shipment before the observation window closes.

### Unresolved operator decisions

- **SE-3b op-id + predecessor chain — pinned.** `size_op_id` is a reserved,
  system-managed `custom_fields` key written in the same atomic `mdfront` write as
  the size/provenance change; the estimate-history event carries both the new
  `OpID` and the captured predecessor `PrevOpID`, forming a per-artifact version
  chain. Doctor reconciliation is compare-and-swap keyed on `PrevOpID` (apply only
  when the artifact's current `size_op_id == PrevOpID`; `== OpID` is already-applied;
  anything else is stale/conflict — never replayed). No sidecar file, no new
  external dependency. Only the exact JSON key names and the doctor check's
  reporting verbosity remain Ship-level detail.
- **SE-6 CLI human-column parity** is deferred to a P2 backlog item; JSON/MCP
  parity is the blocking SE-6 requirement.

## Plan Review

This plan passed a **genuine multi-persona, cross-model** review gate (NOT a
single-agent self-assessment). Reviewer persona subagents were dispatched in
parallel on different model tiers. Full findings are archived at
`docs/reviews/2026-07-18-108-F-size-estimation-plan-review.md`.

### Gate outcome: PASS (after 3 review-fix cycles)

| Cycle | Reviewers | Result |
|---|---|---|
| 0 | Constitution, Scope Boundary, Architecture (gpt-5.6-sol), Security Lens (gemini-3.1-pro-preview), Go | **FAIL** — 3 personas gated FAIL, 2 ADVISORY; 4 converging P1s |
| 1 | Architecture (gpt-5.6-sol), Security Lens (gemini-3.1-pro-preview), Go | Security **PASS**, Go **ADVISORY** (all 4 prior RESOLVED), Architecture **FAIL** (3 new P1s) |
| 2 | Architecture (gpt-5.6-sol) | **FAIL** — prior P1 #2/#3 RESOLVED, 1 blocker on op-id CAS decidability |
| 3 | Architecture (gpt-5.6-sol) | **PASS** — blocker RESOLVED; only a P3 advisory remains |

### P1 findings and resolutions

1. **SE-3 append+write atomicity was unsafe (Architecture + Go).** Compensating
   truncation of the shared per-item JSONL could delete concurrent legitimate
   events. **Resolved:** split into SE-3a (persist + event-before-write,
   fail-closed) and SE-3b (op-id + `PrevOpID` predecessor chain; doctor
   compare-and-swap; no truncation; orphan-tolerant).
2. **SE-2 single-writer was update-only (Architecture).** A generic create could
   inject reserved sizing keys, bypassing the provenance event. **Resolved:** SE-2
   enforces reject/strip on **create** and merge-preserve on **update**; a
   canonical `ToFrontmatterMap()` projection rule removes the emitter-equivalence
   contradiction.
3. **size_source human-masquerade via MCP (Security + Constitution).** **Resolved:**
   transport-aware actor stamping — MCP/agent rejects/overrides `human`; CLI stamps
   `human`; absent-on-read stays `unknown`, never rewritten.
4. **Positional-string seam signature (Architecture + Go).** **Resolved:** typed
   presence-aware `SizeMutation{Size, Source *string, RulesetVersion *string,
   Actor ActorContext, OpID *string}`.

### Residual / deferred (non-blocking)

- **P2 (Go):** per-unit test-scenario counts sit at the 2-hour boundary; the plan
  states table-driven `t.Run` subtests count as one scenario, and flags SE-2's
  characterization lock as a splittable sibling if the envelope is exceeded.
- **P3 (Security):** transport-aware stamping cannot stop masquerade if an agent
  is granted unrestricted local shell (it can invoke the CLI directly) — to be
  noted in the SE-8 contract doc.
- **P3 (Architecture):** exact `size_op_id`/`PrevOpID` JSON key names and doctor
  reporting verbosity are Ship-level implementation detail.
- **Deferred item:** SE-6 CLI human-column parity → separate P2 backlog follow-up.
