---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for extending optional size estimation to feature and shipment artifacts: canonical custom_fields.size at all levels (the spike-selected carrier; no codec bridge), a round-trip regression guard, event-before-write fail-closed exactly-once provenance, computed-on-read composition rollups, CLI/MCP mutation and read parity, and two-layer containment hardening of the size seam.'
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
| D4 | **Durability policy = event-before-write, fail-closed, exactly-once with crash-safe reconcile.** The estimate-history event appends **first** (gate-evidence precedent) and is the **source of truth**; an append failure **refuses** the size write, so the **hard invariant** is "**no persisted size/provenance change without its event**." This is **not** a two-way atomicity guarantee: a crash between a successful append and the artifact write may leave an **op-id-tagged orphan event** whose write did not land. Such an orphan is **reconciled/deduped on retry** (a pre-append lookup by `OpID` short-circuits, so no duplicate event) and **doctor-reconciled** (CAS on the predecessor op-id), never resolved by mutating the shared log. This ordering is **process-crash safe only** (Copilot G3): both shared writers are sync-free (`atomicfile.WriteFileAtomic`, `events.AppendEvent`), so OS-crash / power-loss durability is **out of scope for 099-S** and tracked as stash `131CEAE4`. | §9(a), Provenance |
| D5 | **Composition = computed-on-read, never persisted.** XS–XL count histogram + `unsized` count + de-duplicated canonical members array; `ruleset_version = null`. Feature membership = children by `parent_id`; shipment membership = `NormalizeShipmentItems`. Missing member → `unsized`; skip `ErrNotFound`. Comparator XS<S<M<L<XL mirrors the priority `CASE` ordered-enum. | §8, §9(b) |
| D6 | **Containment = two-layer fix.** (1) reject lexical `..`/absolute escape in `QueueLayout.RootDir` and every configured search root at config-load time; (2) realpath/`EvalSymlinks` re-containment at lookup time before `parseFile` reads a candidate. Mirror `internal/core/doctor_target.go:230-280`. `SafeResolve` alone is insufficient (lexical only). | §6, Recommendation 3 |

> **Reconciliation addendum (2026-07-18).** An earlier framing of this plan
> treated SE-2 as an **artifact-codec bridge** to be *built* and proven by a
> round-trip test. That framing was incorrect. Per D2 (§9(d), Recommendation 2)
> the spike **rejected** any `models.Artifact` docline carrier and **selected
> `custom_fields`**, which already round-trips — a fact **already proven** by the
> committed guard `internal/core/docline_codec_roundtrip_test.go`
> (`TestGenericArtifactCodec_DropsTopLevelDocline`,
> `TestSetArtifactSize_PreservesTopLevelDocline`,
> `TestUpdateArtifact_DropsTopLevelDocline_PreservesCustomFields`, all passing).
> There is therefore **no bridge to build**. SE-2 is now a bounded **test-only
> regression guard** that *extends* those tests to the new feature/shipment
> `size` + `size_source` + `size_ruleset_version` keys. The write-path
> single-writer/merge-not-replace integrity that the earlier SE-2 carried moves to
> **SE-3a** (the size seam / sole `custom_fields.size` writer). The optional
> `ToFrontmatterMap()` two-emitter consolidation is **descoped** (not required by
> the spike) and left as a documented, reversible future drift-reduction.

## Problem Frame

backlogit's **generic artifact codec** captures only `custom_fields` and re-emits
only struct fields plus `custom_fields`:

- `models.ArtifactFromFrontmatter` (`internal/models/frontmatter.go:45-123`) maps
  the enumerated `Artifact` struct fields plus `custom_fields`; `models.Artifact`
  (`internal/models/artifact.go:33-55`) has **no** docline/extension carrier.
- `core.WriteArtifactFile` (`internal/core/artifacts.go:682-735`) rebuilds
  frontmatter from struct fields only, appending `custom_fields` if non-nil.
- The inline map builder in `createArtifact` (`internal/core/artifacts.go` ~`:289`)
  duplicates that emission logic — a latent **drift risk** the spike notes as an
  *optional* consolidation opportunity (`models.Artifact.ToFrontmatterMap()` shared
  by both write paths). This consolidation is **not required** by the size
  contract and is **descoped** here; it remains a documented future drift-reduction.

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
those levels by **extending the existing round-trip regression guard** (no codec
bridge is built — `custom_fields` is the spike-selected, already-durable carrier);
(3) persist provenance with an exactly-once estimate-history event, keeping the
size seam the sole writer of the reserved sizing keys (merge-not-replace on
generic update, reject/strip on generic create); (4)
compute (never persist) composition rollups; (5) reach CLI/MCP mutation and read
parity; (6) close the containment gap in the seam's lookup/write path; (7)
document the contract.

## Requirements Trace

| Requirement (from spike) | Implementation action | Unit |
|---|---|---|
| D1 canonical `custom_fields.size` on feature/shipment | Add `size` enum to `feature`/`shipment` header-def types + defaults; enable seam validation for those types | SE-1 |
| D3 provenance field definitions | Add `size_source` (enum) and `size_ruleset_version` (**bounded**, not free-text) to task/feature/shipment header-def | SE-1 |
| D2 no carrier bridge; durability of `custom_fields.size` | **Extend the committed round-trip guard** (`docline_codec_roundtrip_test.go`) to assert feature/shipment `size` + `size_source` + `size_ruleset_version` survive the generic codec under `custom_fields` (docline-drop guard unchanged). No codec bridge is built. | SE-2 |
| D2 sole-writer integrity of reserved sizing keys | Keep the size seam the **sole writer** of `custom_fields.size`/`size_source`/`size_ruleset_version`/`size_op_id`: merge-not-replace on generic update (closes the `updateArtifactUngated` whole-map-replace hazard); **migration-safe** generic create (**Copilot G7**) — preserve a **provenanced** imported size (record its event) and reject/strip only an **unprovenanced** reserved size, so an initial size is never eventless yet `cli/migrate.go` import neither loses nor fails on an already-sized item | SE-3a |
| D4 exactly-once provenance | Typed `SizeMutation` seam that persists `size_source`/`size_ruleset_version`; append estimate-history event **before** the write, fail-closed | SE-3a |
| D3 actor-context stamping | New-authored size with no explicit `size_source` stamped from actor context (CLI ⇒ `human`, agent/MCP ⇒ `agent`); absent-on-read stays `unknown`, never rewritten | SE-3a / SE-5 |
| D4 crash-safety (online seam) | Op-id-tagged, idempotent, reconciliation-not-truncation crash policy on the mutation seam (no eventless write; pre-append `OpID` dedup; applied-check path re-upserts the index — **Copilot G2**); **process-crash scope only** (sync-free writers; power-loss out of scope, stash `131CEAE4` — **Copilot G3**) | SE-3b |
| D4 crash-safety (offline doctor) | Doctor CLEAR-recovery reconcile — CAS on predecessor op-id, presence-aware desired state incl. field-removal (CLEAR) semantics; split from SE-3b to keep both ≤2h (**Copilot G6**) | SE-3c |
| D5 composition rollups | Add a computed-on-read, never-persisted histogram+unsized+members function for feature and shipment membership, with **explicit feature→children expansion** for feature-typed shipment members (`NormalizeShipmentItems` returns explicit `custom_fields.items` only — **Copilot G4**) and the XS<S<M<L<XL comparator | SE-4 |
| D3 mutation parity | Add `--size-source`/`--size-ruleset-version` (CLI) and `size_source`/`size_ruleset_version` (MCP) via `SizeMutation`; transport-aware error parity; reject agent human-masquerade | SE-5 |
| §7 read parity + D5 exposure | Project `custom_fields.size`/provenance identically on `get`/`get_item`/`get_queue`/`shipment get` (**both transports**); expose the derived composition on the **MCP read surfaces only** — CLI-JSON composition parity is **not** claimed (separate shapers `buildDetailMap`/`QueryQueue`) and is filed as stash `387DE4BF`; CLI human columns filed as stash `D5FA1EE9` (**Copilot G5**) | SE-6 |
| D6 containment (config-load layer) | Reject lexical `..`/absolute + env-expansion escape on `RootDir`/search roots at config-load | SE-7a |
| D6 containment (lookup-time layer) | Realpath/`EvalSymlinks` re-containment at lookup time (before `parseFile`) in the seam path | SE-7b |
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

### SE-2 — Codec round-trip guard for feature/shipment size + provenance (test)

- **Reconciliation note:** the spike **selected `custom_fields`** and **rejected**
  any `models.Artifact` docline carrier (D2). `custom_fields` already round-trips,
  proven by the committed `internal/core/docline_codec_roundtrip_test.go`. **No
  codec bridge is built.** This unit is a **test-only** regression guard that
  extends that proof to the newly-defined feature/shipment fields; the write-path
  single-writer integrity that an earlier draft placed here now lives in **SE-3a**,
  and the optional two-emitter `ToFrontmatterMap()` consolidation is **descoped**.
- **Changes:** Extend `internal/core/docline_codec_roundtrip_test.go` with
  feature/shipment coverage for the new `custom_fields` sizing keys. No production
  code changes.
- **Files:** `internal/core/docline_codec_roundtrip_test.go` (test-tier only).
- **Tests (test-tier; table-driven `t.Run` subtests count as one scenario for the
  2-hour rule):** for a `feature` and a `shipment` artifact carrying
  `custom_fields.size` + `custom_fields.size_source` + `custom_fields.size_ruleset_version`:
  (1) the generic codec (`ParseFrontmatter` → `ArtifactFromFrontmatter` →
  `WriteArtifactFile`) round-trips **all three** keys (assert on the frontmatter
  **map**, not body byte-identity — the generic parser normalizes CRLF→LF; see the
  CRLF caveat); (2) an unmodeled top-level `docline` map is still **dropped**
  (premise-protection guard, unchanged); (3) the ordinary `UpdateArtifact` path,
  driven by a non-size field update, **preserves** all three provenance keys. These
  guards fail if a future codec change ever adds a docline carrier or breaks
  `custom_fields` preservation, forcing the spike selection to be revisited.
- **Execution posture:** test-first authoring, but **expected-green
  characterization** — **not a red harness**. `ArtifactFromFrontmatter` copies the
  whole `custom_fields` map with **no schema consultation**
  (`internal/models/frontmatter.go:74-75`), and the committed guard already shows an
  ordinary non-size `UpdateArtifact` preserves untouched custom fields
  (`docline_codec_roundtrip_test.go:121-160`). The new feature/shipment
  `size`/`size_source`/`size_ruleset_version` keys therefore survive the generic
  codec **from the outset**, so these assertions **pass against the current code**.
  This unit locks that behavior as a committed regression guard; it does **not**
  depend on SE-3a (no persistence is needed for the codec to preserve the map) and
  it needs no red-first step. (If a genuinely-red assertion about persistence is
  wanted, it belongs in the implementation tasks SE-1/SE-3a, not here.)
- **Milestone:** the durability premise for feature/shipment `custom_fields.size`
  and provenance is codified as an executable, committed regression guard.

### SE-3a — Provenance persistence + estimate-history event (core persistence)

- **Changes:** Introduce a **new** presence-aware typed entry point —
  `SetArtifactSizeWithProvenance(ctx, ws, id, SizeMutation{ Size, Source *string,
  RulesetVersion *string, Actor ActorContext, OpID *string })` — and **keep the
  existing positional `SetArtifactSize(ctx, ws, id, size string)` as a thin
  compatibility wrapper** that constructs a `SizeMutation` (no provenance, actor
  from context) and delegates to the new entry point. This keeps the repo
  **buildable at every commit**: the current callers
  (`internal/cli/update.go:103`, `internal/mcp/tools.go:751`, and the existing core
  tests) keep compiling against the wrapper; **SE-5** migrates them to the typed
  path and then removes the wrapper. The typed command lets callers distinguish
  *omitted* from *cleared* from *set*, and centralizes defaulting/validation at one
  boundary (resolving the four-adjacent-strings hazard and the omitted-vs-null
  ambiguity). The seam validates
  `size_source` against `{human, agent, derived}` and `size_ruleset_version`
  against a **bounded** set (accept only `null`/empty until a canonical ruleset is
  owned; unknown → `ErrValidation`) — never an unbounded free-text write.
  `ActorContext` is **derived at the transport boundary, not caller-supplied**:
  the CLI command layer constructs it as `human`; the MCP tool layer constructs it
  as `agent` (and rejects/overrides any caller-supplied `size_source: human`, per
  SE-5). It maps to the event's plain-string `Actor` field (`stream.go` `Event.Actor
  string`) via a single documented projection (`human`/`agent`/`derived` →
  identical string), so the seam needs no ad-hoc mapping.
  Persist all three under `custom_fields` (**merge-not-replace**). **Sole-writer
  integrity (moved here from the earlier SE-2):** keep the size seam the only
  writer of the reserved sizing keys
  (`size`/`size_source`/`size_ruleset_version`/`size_op_id`) — a generic **update**
  carrying a `custom_fields` key must merge-preserve (not replace/delete) them
  (closes the `updateArtifactUngated` whole-map-replace hazard,
  `internal/core/artifacts.go:542-544`), and a generic **create** is
  **migration-safe (Copilot G7):** it **preserves** an imported size **when
  accompanied by provenance** — routing it through the size seam so the
  estimate-history event is recorded — and rejects/strips only an **unprovenanced**
  reserved size, so an initial size is never eventless (protects Protected invariant
  #1) **while** migration/import via `cli/migrate.go` (`WithFields` → `CreateArtifact`,
  `:273`/`:310`) neither loses nor fails on an already-sized item. This is a **minimal
  targeted guard**, not an emitter refactor. **Provenance
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
  (new `EventType` constant only), and a minimal reserved-key guard in
  `internal/core/artifacts.go` (generic create reject/strip + update merge-preserve).
- **Tests (unit; table-driven `t.Run` subtests count as one scenario):** event
  appended before write; forced append failure ⇒ no persisted change (fail-closed);
  a `size_ruleset_version`-only change still emits **exactly one** event; absent
  `size_source` reads as `unknown`/legacy, never rewritten as `human`; a generic
  create carrying an **unprovenanced** reserved sizing key is rejected/stripped, a
  generic create carrying a **provenanced** size **preserves** it and records the event
  (**migration-safe, Copilot G7**), and a generic update carrying a `custom_fields` key
  merge-preserves the reserved keys (sole-writer integrity); **importing an
  already-sized item via the migration path preserves size + provenance**.
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
  2. **applied-check + predecessor capture + index repair (before append):** read the
     target artifact's current `custom_fields.size_op_id` — call it `C` (empty for a
     never-sized artifact) and capture the intended predecessor `PrevOpID = C`. If
     `C` already equals **this** `OpID`, the artifact write already landed — **but**
     because `SetArtifactSize` writes the file (`artifact_size.go:79`) **before**
     `db.UpsertItem` (`:90`), a prior **write-succeeded / index-failed** crash can leave
     the **SQLite row stale**. So the applied-check path must **re-verify and
     re-`UpsertItem`** the artifact into the index **before** returning success
     (**Copilot G2**) — it must **repair the index, not merely skip** because the op-id
     matches. Then return the fully-applied idempotent result **without** appending a
     duplicate event.
  3. **orphan dedup by `OpID` (before append — closes the crash-retry gap):** scan
     the estimate-history event stream for an event **already tagged with this
     `OpID`**. If one exists, a prior attempt appended the event but its artifact
     write did **not** land (so `C == PrevOpID`, **not** `OpID`, and step 2 did not
     short-circuit). Do **not** append a second event; instead **reconcile** by
     completing the artifact write from that event's pinned desired-state payload
     (CAS on `PrevOpID`), then return. This makes a retried `OpID` idempotent **even
     across the documented append-succeeded / write-failed crash window**, so the
     retry never appends a **duplicate** event.
  4. **append (only when no event for this `OpID` already exists):** append the
     estimate-history event tagged with **both** `OpID` and the captured `PrevOpID`
     (an explicit predecessor link, so the event stream forms a per-artifact version
     chain) **and carrying the complete presence-aware desired state** — for **each**
     of `size`, `size_source`, `size_ruleset_version`, an explicit **SET(value)** or
     **CLEAR(remove)** marker (not merely a set). Recovery therefore reconstructs the
     exact intended mutation — **including field removal (CLEAR)** — from the event
     alone. Fail-closed; append failure refuses the write.
  5. **atomic write:** apply the pinned desired state (`size`/provenance SET/CLEAR)
     **and** set `custom_fields.size_op_id = OpID` (a reserved, system-managed key)
     in the same `mdfront` atomic write, so the artifact durably records which op
     last applied.
  The hard invariant is **"no persisted change without an event"**; the crash-window
  residue (an event whose write did not land) is an **op-id-tagged orphan** that is
  reconciled — **never** by mutating the shared log. Two reconciliation paths exist:
  the **online** path (this unit) is the **seam-side retry** in step 3 above, which
  completes the write when the same `OpID` is re-submitted in-process; the **offline**
  path (a `doctor` check that scans and reconciles orphans left by a crash, **including
  CLEAR-a-field recovery**) is owned by **SE-3c (`108.011-T`)** to keep this unit
  single-file (`artifact_size.go`) and inside the 2-hour envelope (**Copilot G6**).
  This **refines** the spike's §9(a) exactly-once mandate into a concrete, safe form;
  orphan events are op-id-tagged, doctor-detectable, and never resolved by mutating the
  shared log. (See the revised Decisions and Invariant #1.)
- **Durability scope (process-crash only — Copilot G3):** the event-before-write
  ordering guarantees **process-crash** safety, **not** OS-crash / power-loss. Both
  shared writers are **sync-free**: `atomicfile.WriteFileAtomic`
  (`internal/atomicfile/atomicfile.go:15-63`) does temp-write → `Chmod` → `Close` →
  `Rename` with **no `fsync`** before the rename, and `events.EventWriter.AppendEvent`
  (`internal/events/stream.go:40-64`) does an `O_APPEND` write with **no `f.Sync`**.
  Closing the power-loss gap would need a separate `fsync` protocol coordinated across
  **both** shared writers (event file + atomic temp file + parent dir) — that is
  **cross-cutting, changes the whole repo write path, and is OUT OF SCOPE for 099-S**;
  it is filed as stash **`131CEAE4`**. This unit must **not** add `fsync` to the shared
  writers.
- **Files:** `internal/core/artifact_size.go`.
- **Tests (unit; table-driven `t.Run` subtests count as one scenario):** a write
  failure **after** a successful append leaves an orphan event carrying `OpID`+
  `PrevOpID` and **no** persisted change (fail-closed, no truncation of the shared
  JSONL); a **write-succeeded / index-failed** crash followed by a retry with the
  matching `OpID` **re-upserts the artifact into the index** before returning success
  (**Copilot G2**); **retry from the orphan state** — the same `OpID` is re-submitted
  while an event for it already exists and the artifact still shows `PrevOpID`: the seam
  **detects the existing event, appends no duplicate**, and completes the write
  (idempotent across the crash window); a retried op-id that already fully applied
  short-circuits at the applied-check (index-repaired).
- **Execution posture:** test-first (crash-injection at the post-append boundary).
- **Milestone:** the seam-side crash window is closed to "orphan-tolerant,
  op-id-reconciled, index-repaired"; no eventless persisted change, no stale index on
  the applied path, and no shared-log data loss. Offline doctor reconciliation (incl.
  CLEAR recovery) lands in SE-3c.

### SE-3c — Doctor CLEAR-recovery reconciliation (core doctor)

- **Split rationale (Copilot G6).** After the F4/F5 additions SE-3b sat at/over the
  2-hour envelope. The **offline** doctor-side reconciliation is a distinct domain
  (`doctor_target.go`, not the seam), so it becomes its own single-domain unit
  (`108.011-T`) depending on SE-3b. SE-3b now genuinely fits one 2-hour envelope.
- **Changes:** Add a `doctor` check that, **under the per-task lock**, scans
  op-id-tagged orphan estimate-history events and reconciles each by comparing the
  artifact's **current** `custom_fields.size_op_id` `C` against the event's
  `OpID`/`PrevOpID`:
  - `C == OpID` ⇒ **already applied** (idempotent no-op);
  - `C == PrevOpID` ⇒ a **fresh orphan** whose write did not land — **compare-and-swap
    apply** the orphan's pinned presence-aware desired state (the artifact is exactly
    at the event's declared predecessor, so nothing newer is overwritten);
  - any **other** `C` ⇒ the artifact has advanced past this op — the orphan is
    **stale/conflicting**, left as benign op-id-tagged audit residue, **never replayed**.
  Ordering is **decidable from the predecessor chain**, not opaque ID equality. The
  intended mutation — **including field removal (CLEAR)** — is reconstructed from the
  event's pinned **SET/CLEAR-per-field** payload alone, so recovery of a
  *CLEAR-a-provenance-field* op **removes** the field rather than leaving a stale value.
  Reconciliation **never** mutates the shared JSONL.
- **Files:** `internal/core/doctor_target.go` (the existing `doctor` target).
- **Tests (unit; table-driven `t.Run` subtests count as one scenario):** doctor
  **applies** a fresh orphan whose `PrevOpID` equals the artifact's current
  `size_op_id`; doctor **skips** a stale orphan whose `PrevOpID` does not match
  (artifact already advanced); doctor **reconciles a CLEAR-a-field mutation exactly** —
  an orphan whose payload marks a provenance field CLEAR is recovered by *removing* that
  field (not leaving a stale value).
- **Execution posture:** test-first.
- **Milestone:** offline crash recovery is complete and CLEAR-aware; a crash that leaves
  a fresh orphan is doctor-reconciled to the exact intended state.

### SE-4 — Computed-on-read composition rollups (core aggregation)

- **Changes:** Add a `SizeComposition` function that, given a feature or
  shipment, returns a **never-persisted** structure: an XS–XL count histogram, an
  `unsized` count, and a de-duplicated canonical-members array. Feature
  membership = direct children by `parent_id` (`internal/core/hierarchy.go`).
  Shipment membership = `NormalizeShipmentItems`
  (`internal/core/shipment.go:545-571`), which returns **only** the explicit
  `custom_fields.items` verbatim and does **not** expand a feature into its child
  tasks (**Copilot G4**). This unit therefore adds **explicit feature→children
  expansion**: when a normalized shipment member is a **feature**, walk its child
  tasks (`parent_id`) and include their sizes, so a shipment that lists only a feature
  still reflects its members' sizes. De-dup via `uniqueNonEmptyStrings` so a manifest
  listing a feature **and** its explicit child tasks counts each item **once**. Missing
  member → `unsized`; skip `ErrNotFound`, warn+skip others
  (`DeriveCoveringFeature` precedent). Comparator XS<S<M<L<XL mirrors the priority
  `CASE` ordered-enum (`internal/core/queue.go:183-191`). `ruleset_version = null`.
- **Files:** new `internal/core/size_composition.go`.
- **Tests (unit):** (1) histogram + `unsized` counts for a feature with
  mixed/absent sizes **including a missing/`ErrNotFound` member** (skipped, not
  fatal); (2) a shipment whose manifest lists **only a feature** expands to that
  feature's children and counts their sizes (**Copilot G4**); (3) a shipment listing a
  feature **plus** its explicit child tasks **de-dups** so each item is counted once
  (**Copilot G4**); (4) the function never writes to disk (no persistence side
  effects). Table-driven `t.Run` subtests keep SE-4 inside the 2-hour envelope.
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
  `validation_failed`. **Wrapper retirement (single migration operation):** this
  unit also **removes** the SE-3a `SetArtifactSize` compat wrapper
  (`internal/core/artifact_size.go`) and, **in the same commit**, migrates every
  remaining caller and the existing **core tests** that reference the positional
  signature to `SetArtifactSizeWithProvenance`, so the tree stays **buildable at
  that commit** (`go test ./...` compiles). Retiring the wrapper together with its
  callers/tests is one cohesive migration step (the migration axis), not a mix of
  unrelated concerns; see the Constitution Check for the documented, bounded
  cross-file deviation this entails.
- **Files:** `internal/cli/update.go`, `internal/mcp/tools.go`,
  `internal/core/artifact_size.go` (wrapper removal), and the affected
  `internal/core/*_test.go` files that call the positional `SetArtifactSize` (migrated
  in the same commit for buildable-commit safety — **Copilot G1/Constitution lens**).
- **Tests (unit):** CLI flag and MCP field both reach the seam; an invalid
  `size_source` is rejected on **both** surfaces with the matching category; an
  MCP request asserting `size_source: human` is rejected/overridden; a CLI author
  with no flag is stamped `human`, an MCP agent with no field is stamped `agent`;
  the migrated core tests compile and pass against the typed entry point with the
  wrapper removed.
- **Execution posture:** test-first.
- **Milestone:** provenance is writable from both surfaces with parity errors and
  no cross-transport human-masquerade; the compat wrapper is retired with the tree
  buildable at the migrating commit.

### SE-6 — CLI/MCP read projection parity (interface)

- **Changes:** Ensure `custom_fields.size`/provenance projects **identically** on
  the read pairs `get`↔`get_item`, `queue view`↔`get_queue`,
  `shipment get`↔`get_shipment`, `shipment list`↔`list_shipments` (MCP/JSON
  already carry `custom_fields`; ride `size` on `custom_fields`, **not** on any
  surface-specific context block) — this size/provenance parity holds on **both**
  transports. Expose the SE-4 composition on the **MCP read surfaces**
  (`get_item`/`get_shipment`/`get_queue`) as a derived, clearly non-persisted field.
  **Parity-scope correction (Copilot G5):** the derived **composition** is exposed on
  **MCP only** in 099-S. CLI `get` JSON (`internal/cli/get.go` `buildDetailMap`) and
  queue JSON (`internal/cli/queue_cmd.go` / `QueryQueue`) are **separate shapers** that
  do **not** carry the derived composition, so SE-6 does **not** claim CLI-JSON
  composition parity; that gap is filed as stash **`387DE4BF`** (add the composition to
  the CLI shapers, or introduce a shared read-shaper, and test every CLI/MCP pair). The
  **cosmetic CLI human-column** gap (`list`/`shipment list`/`queue view` columns omit
  size) is separately filed as stash **`D5FA1EE9`**. Both deferrals keep SE-6 two-file
  (MCP projection + CLI `--json` size/provenance) within the 2-hour envelope.
- **Files:** `internal/mcp/tools.go`, `internal/cli/list.go`.
- **Tests (unit):** size/provenance projects **identically** on each named CLI/MCP
  read pair; `get_item` JSON exposes `custom_fields.size` **and** the derived
  composition on a sized feature; CLI `--json` exposes `custom_fields.size`
  (size/provenance parity) — composition on CLI JSON is **out of scope** (stash
  `387DE4BF`).
- **Execution posture:** test-first.
- **Milestone:** size/provenance reach CLI/MCP JSON read parity; the derived
  composition is visible on the MCP read surfaces (CLI-JSON composition tracked as
  stash `387DE4BF`).

> **Two-layer containment, split for width isolation (Copilot F1).** D6 is a
> two-layer security invariant, but the two layers live in **different domains**
> (config-load validation vs. core lookup-time re-containment), so combining them in
> one task violated Width Isolation. The work is split into **SE-7a** (config) and
> **SE-7b** (core), wired as **dependent single-domain tasks** (`SE-7a → SE-7b`) so
> the two-layer invariant still ships as one release unit — no half-closed traversal
> hole is ever released — **without** a cross-domain task. Both are members of
> `099-S`.

### SE-7a — Config-load containment (config / security)

- **Changes:** At config-load, reject lexical `..`/absolute escape in
  `QueueLayout.RootDir` and every configured search root — today
  `internal/config/loader.go:77-85` guards only `reg.Directories`, leaving a
  symlink-independent lexical escape via `RootDir`
  (`internal/config/schema.go:99-104`). Also assert that `RootDir` and the
  configured search roots are **not** environment-variable-expanded (or reject
  expansion-based escape) so env expansion cannot reintroduce a traversal vector.
  **Single domain: config.**
- **Files:** `internal/config/loader.go` (or `schema.go` validation).
- **Tests (unit):** a `RootDir: "..\\..\\outside"` config is rejected at load; an
  env-expansion escape in a search root is rejected; a legitimate in-workspace
  config still loads.
- **Execution posture:** test-first (add failing config-containment tests, then
  harden).
- **Milestone:** no configured root can lexically or via env-expansion point
  outside the workspace.

### SE-7b — Lookup-time containment (core / security)

- **Changes:** At **lookup time**, realpath-resolve and re-contain each candidate
  **before** `parseFile` reads it (a leaf-file symlink is otherwise read
  out-of-workspace during the `FindArtifactPath` walk), then revalidate the resolved
  target before lock/read/write — mirroring `internal/core/doctor_target.go:230-280`
  (`EvalSymlinks` + realpath containment), **not** `SafeResolve` alone (lexical
  only) and not a write-only check (the read already happened during lookup).
  **Single domain: core.**
- **Files:** `internal/core/artifacts.go` (lookup-time re-containment helper).
- **Tests (unit):** a leaf-file symlink under a search dir is refused at lookup; an
  in-workspace path still resolves and reads.
- **Execution posture:** test-first (add failing lookup-containment tests, then
  harden).
- **Dependency:** depends on **SE-7a** — the two layers release together as the D6
  invariant; the seam-path guard (this unit) is the layer SE-3a routes writes
  through.
- **Milestone:** the size-lookup/write path cannot read or write outside the
  workspace via symlink escape.

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

Acyclic. Edges are `blocks` (target depends on source). SE-1 and SE-7a are the
two roots; SE-3a/SE-3b/SE-4 can be built in parallel once their inputs exist:

```text
SE-1 ─┬▶ SE-2 (test guard; leaf)
      ├▶ SE-3a ─▶ SE-3b ─┬▶ SE-3c ─────────────▶ SE-8
      │    └─────────────┼▶ SE-5 ─▶ SE-8
      └▶ SE-4 ───────────┼▶ SE-6 ─▶ SE-8
SE-7a ─▶ SE-7b ──┬▶ SE-3a │
                 └▶ SE-8 ─┘
```

(SE-2 is a **test-only, expected-green** regression guard depending solely on SE-1;
**nothing depends on SE-2**, and it has **no** dependency on SE-3a — the codec
preserves `custom_fields` with no schema/persistence step, so the guard passes
against current code. SE-7a and SE-7b are the two width-isolated containment layers,
wired `SE-7a → SE-7b` so the D6 invariant ships together. SE-3a depends on SE-1 and
**SE-7b** — the schema fields plus the hardened lookup-time seam (transitively SE-7a)
— no longer on any codec bridge. **SE-3c** (doctor CLEAR-recovery, split from SE-3b
per **Copilot G6**) depends on **SE-3b**; SE-8 additionally depends on SE-3c so the
docs reflect the final offline-recovery behavior. SE-6 depends on SE-3a and SE-4;
SE-8 depends on SE-5, SE-6, SE-7b, and **SE-3c**. There is **no** SE-2→SE-4 and **no**
SE-2→SE-3a edge.)

Explicit edges to wire with `backlogit dep add <task> <depends_on> --type blocks`
(14 edges total):

- SE-2 depends on SE-1 (green characterization guard; references the schema-defined
  feature/shipment keys — it does **not** depend on SE-3a)
- SE-4 depends on SE-1 (composition consumes the schema contract; it does not
  require the emitter consolidation — de-serialized per review)
- SE-7b depends on SE-7a (the two containment layers release as one D6 invariant)
- SE-3a depends on SE-1 and SE-7b (schema fields + the hardened lookup-time seam
  before routing provenance writes through it)
- SE-3b depends on SE-3a (crash-safety builds on the append+write path)
- **SE-3c depends on SE-3b** (offline doctor CLEAR-recovery builds on the crash-safe
  seam; split per **Copilot G6** to keep both ≤2h)
- SE-5 depends on SE-3b (surfaces provenance mutation on the crash-safe seam)
- SE-6 depends on SE-3a and SE-4 (read projection surfaces persisted size +
  composition)
- SE-8 depends on SE-5, SE-6, SE-7b, and **SE-3c** (documents the final contract incl.
  offline doctor recovery)

Rationale for `SE-3a depends on SE-7b`: harden the seam's **lookup-time**
containment (the layer in the seam's read/write path) before routing additional
provenance writes through it; SE-7b already carries SE-7a transitively, so both
containment layers land before the seam is extended.

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
- **Two-layer containment, width-isolated (D6).** `RootDir` admits a purely lexical
  `..` escape independent of symlinks (a **config** concern), and a leaf-file symlink
  is read during lookup before any write-path check (a **core** concern) — so a
  write-only or lexical-only fix is insufficient. Because the two layers live in
  different domains, they are **split into SE-7a (config-load) and SE-7b
  (lookup-time)**, wired `SE-7a → SE-7b` so the invariant still releases as one unit
  (no half-closed hole) while each task stays single-domain. The realpath pattern
  already exists in `doctor_target.go` and is reused by SE-7b.
- **Single seam, sole writer (no emitter consolidation).** `SetArtifactSize`
  stays the only `custom_fields.size` writer; SE-3a adds a minimal reserved-key
  guard (create reject/strip, update merge-not-replace) so the map-replacement
  caveat cannot be weaponized. The `ToFrontmatterMap()` two-emitter consolidation
  is **descoped** — it is an optional drift-reduction the size contract does not
  require, left as a documented, reversible future option.

## Risks and Caveats

- **Map-replacement caveat:** `updateArtifactUngated` *replaces* the whole
  `custom_fields` map when an update carries a `custom_fields` key
  (`internal/core/artifacts.go:542-544`). Harmless today (no surface passes
  arbitrary `custom_fields`), but **SE-3a** adds a reserved-key merge-preserve
  guard so this stays true, and SE-5 must never route provenance through a
  full-map passthrough.
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
  size omission is filed as stash **`D5FA1EE9`** (P2 backlog follow-up), not part of SE-6,
  so SE-6 stays two-file (MCP projection + CLI `--json`) inside the 2-hour
  envelope; JSON/MCP parity is the SE-6 deliverable.

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change — **present** (header-def schema on
  feature/shipment; new provenance fields; new MCP/CLI surface; composition read
  contract).
- security, auth, permission, or compliance-sensitive behavior — **present**
  (SE-7a/SE-7b workspace-containment hardening; path traversal / symlink escape).
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
| SE-2 | No (test) | round-trip guard green on feature+shipment; docline-drop guard unchanged | the extended round-trip regression guard is the closure evidence |
| SE-3a | Yes (mutation seam) | event-before-write proven; forced-append-failure refuses write; exactly-once on ruleset-only change; actor-context stamping (CLI human / agent) proven | rollback trigger: any persisted size without an event ⇒ revert; owner + validation window |
| SE-3b | Yes (crash recovery) | post-append crash leaves op-id orphan + no persisted change; retried op-id **pre-append dedup** short-circuits (no duplicate event); **applied-check path re-upserts the index** (write-succeeded/index-failed retry repairs the stale row — **Copilot G2**); **no** shared-JSONL truncation; **process-crash scope only** (**Copilot G3**) | rollback trigger: any shared-log truncation or duplicate event ⇒ revert |
| SE-3c | Yes (offline doctor) | doctor reconciles an orphan from the predecessor op-id (CAS); **presence-aware CLEAR recovery** — a field-removal (CLEAR) mutation is reconciled exactly from the pinned desired state (**Copilot F5/G6**) | rollback trigger: any doctor mutation that diverges from the pinned desired state ⇒ revert |
| SE-4 | No (pure read) | composition deterministic; never persists | n/a |
| SE-5 | Yes (CLI + MCP) | both surfaces reach the seam; parity error categories; agent human-masquerade rejected | parity matrix in contract doc |
| SE-6 | Yes (CLI + MCP read) | size/provenance visible with JSON/MCP parity on **both** transports; derived composition on **MCP read surfaces only** (CLI-JSON composition deferred to stash `387DE4BF`; CLI human columns to stash `D5FA1EE9` — **Copilot G5**) | read-parity matrix in contract doc |
| SE-7a | Yes (config-load) | lexical `..` config rejected; env-expansion escape rejected; in-workspace config still loads | containment regression tests; rollback trigger: any accepted escaping config |
| SE-7b | Yes (lookup/write path) | symlink lookup refused before `parseFile`; in-workspace still resolves | containment regression tests; rollback trigger: any out-of-workspace read/write |
| SE-8 | No (docs) | docline lint clean | the doc itself |

When SE-3a/SE-3b and SE-7a/SE-7b are hardened (below), the downstream Ship
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
| **II. Test-First (NON-NEGOTIABLE)** | Every code unit is red-first (write the failing test, then implement). SE-2 is the one deliberate exception: it is an **expected-green characterization guard** (the generic codec already preserves `custom_fields`), so it locks current behavior as an executable committed regression guard rather than driving new code — this is characterization, not a skipped red phase. |
| **II. Task Granularity / Width Isolation (NON-NEGOTIABLE)** | Every task targets a single skill domain and ~2h. **Two bounded deviations are documented below** (SE-3a file-count boundary; SE-5 cross-file wrapper retirement) per the Governance Conflict-resolution clause — neither mixes unrelated skill domains. |
| **III. Workspace Isolation** | SE-7a/SE-7b *strengthen* isolation (two-layer lexical + realpath containment, split by domain). No secrets are added. |
| **IV. CLI Containment (NON-NEGOTIABLE)** | SE-7a enforces containment at config-load and SE-7b at lookup time; no unit writes outside the workspace root. |
| **V. Structured Observability** | SE-3a adds a durable estimate-history event stream (traceable provenance) — an observability *gain*; SE-3b keeps it consistent under crash via op-id reconciliation. |
| **VI. Single Responsibility** | No new external dependency (SE-3b reuses existing `atomicfile`/JSONL/`doctor` rails); no new carrier is added to `models.Artifact` — the spike-selected `custom_fields` carrier is reused, so the codec surface is unchanged. The optional two-emitter consolidation is descoped to avoid speculative refactoring. |
| **VII. Destructive Command Approval** | No destructive terminal commands. The durable append is fail-closed, not force-overwrite; SE-3b explicitly forbids truncating the shared log. |
| **VIII. Safety Modes** | Elevated blast radius (mutation seam + containment) is flagged; `Requires plan hardening: yes`; Ship should operate in careful/investigate-first posture for SE-3a/SE-3b/SE-7a/SE-7b. |
| **IX. Git-Friendly Persistence** | `custom_fields.*` stays human-readable YAML frontmatter; `mdfront` preserves body bytes and semantic ordering. |
| **X. Context Efficiency** | Composition is computed-on-read via targeted membership queries, not bulk scans; MCP read projection returns structured `custom_fields`. |
| **XI. Merge Commit History** | Not applicable to planning; Ship enforces merge-commit strategy at PR time. |

**Justified deviations (documented per Governance Conflict-resolution clause):**
The former SE-7 "width exception" (a single task spanning config + core) is
**removed** — SE-7 is split into the single-domain SE-7a/SE-7b, wired
`SE-7a → SE-7b`. The SE-3b/SE-3c split (**Copilot G6**) likewise keeps each
crash-safety unit single-domain (SE-3b = online mutation seam `artifact_size.go`;
SE-3c = offline doctor `doctor_target.go`). No task **mixes skill domains**. Two
bounded, single-domain deviations remain and are explicitly documented rather than
asserted away (**Copilot G1 multi-persona re-review, cycle 2**):

1. **SE-3a sits at the file-count boundary (Scope-Boundary lens, contested P1 →
   documented).** It touches three files — `artifact_size.go` (the seam),
   `stream.go` (**a single new `EventType` constant**), and `artifacts.go` (**a
   minimal reserved-key sole-writer guard**) — where the file-budget heuristic is
   "< 3". Two of the three touches are minimal, and all are the **same skill
   domain** (Go core persistence / write-path integrity). The plan carries an
   explicit **Ship-time split fallback**: if implementation exceeds one 2-hour
   envelope, carve the `artifacts.go` reserved-key guard into a sibling task (the
   G6 precedent). This is a documented near-boundary case, **not** a domain-mixing
   violation.
2. **SE-5 retires the compat wrapper across files (Constitution lens, single-reviewer
   P1 → documented + buildable-commit safeguard added).** SE-5 is interface-domain,
   but retiring the SE-3a `SetArtifactSize` wrapper necessarily touches the wrapper's
   core file and the core tests that call it. This is **one cohesive migration
   operation on a single axis** (retire the transitional API), performed **in one
   commit** so the tree stays buildable (`go test ./...` compiles). It is documented
   as a bounded cross-file migration, not a mix of unrelated concerns.

SE-2's expected-green characterization posture is a documented test-authoring choice,
not a Test-First violation (it still ships an executable committed guard). The
map-replacement and validated-once **asymmetries** are pre-existing codebase
constraints carried as caveats (Risks), not new violations introduced by this plan.

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
  containment pattern SE-7b mirrors.
- `internal/core/commits.go:27-97`, `internal/events/stream.go:39-46`,
  `internal/db/logs.go:32-40` — event-append durability + independent timestamp
  stamping, relevant to SE-3a exactly-once.
- Compound library: no existing `docs/compound/` learning contradicts this plan;
  the closest prior art is the fail-closed / "absence is not a pass" family
  (e.g. `exported-cache-zero-value-bypass`) — SE-3a's fail-closed refusal and
  SE-7a/SE-7b's "unknown path ⇒ refuse" both apply that governing rule.

### Protected invariants

1. **Exactly-once provenance (orphan-tolerant):** no persisted
   `custom_fields.size` / `size_source` / `size_ruleset_version` change may exist
   without an estimate-history event (the **hard** invariant). The reciprocal
   ("no event without a persisted change") is enforced as **op-id reconciliation,
   not shared-log truncation**: a crash-residue orphan event is op-id-tagged and
   doctor-reconciled, never resolved by mutating the shared JSONL. Scope:
   **process-crash** safety only; power-loss durability (sync-free writers) is out of
   scope (stash `131CEAE4`). Online reconcile lives in SE-3b; offline doctor reconcile
   (incl. CLEAR recovery) in **SE-3c**. (SE-3a/SE-3b/SE-3c)
2. **Sole writer (create + update):** `SetArtifactSize` remains the only writer of
   `custom_fields.size`/`size_source`/`size_ruleset_version`/`size_op_id`; no
   generic **create** or **update** path may inject or replace these reserved keys
   (create rejects/strips them; update merge-preserves them). Enforced by a minimal
   reserved-key guard in **SE-3a** (no emitter consolidation). The round-trip guard
   (SE-2) proves the keys survive (expected-green characterization); the CRLF-safe
   map assertion covers the codec. (SE-3a, SE-2, SE-5)
3. **Legacy non-rewrite:** an absent `size_source` reads as `unknown`/legacy and
   is never rewritten as `human`; agent/MCP transports may not stamp `human`.
   (SE-3a, SE-4, SE-5)
4. **Containment:** no size lookup or write may read or write outside the
   workspace root via lexical `..`, absolute path, or symlink escape. (SE-7a, SE-7b)
5. **No-persist derived:** composition rollups are computed-on-read and never
   written to disk. (SE-4)

### Risky actions (ProposedAction / ActionRisk / ActionResult)

| # | ProposedAction | targets | change_kind | ActionRisk | rollback | approval | ActionResult |
|---|---|---|---|---|---|---|---|
| PA-1 | Route provenance writes through an event-before-write, fail-closed, **op-id-reconciled** append+write in the live size seam (SE-3a persists+events; SE-3b crash-safety) | `internal/core/artifact_size.go`, `internal/events/stream.go`, `internal/core/doctor_target.go` | durable append + mutation (contract) | **high** | revert the seam commit; the append is additive to the JSONL event stream and index-rehydratable; a failed write leaves the prior size intact (fail-closed); orphan events are op-id-tagged and doctor-reconciled, **never** resolved by truncating the shared log | prefer approval (production mutation-path + failure-semantics change) | planned |
| PA-2 | Add two-layer lexical + realpath containment, split by domain: SE-7a at config-load, SE-7b at lookup time | `internal/config/loader.go` (SE-7a), `internal/core/artifacts.go` (SE-7b) | config validation + security guard | **high** | revert; new rejections are fail-closed (worst case: a previously-accepted escaping config is now refused — surface a clear diagnostic) | prefer approval (changes accepted-config surface) | planned |
| PA-3 | Extend header-def with size on feature/shipment + provenance fields | `.backlogit/header-def.yaml`, `internal/config/defaults.go` | schema/contract | **moderate** | revert; fields are `optional`, additive, backward-compatible (coexist, no migration) | standard review | planned |
| PA-4 | Add a minimal reserved-key write-path guard (create reject/strip + update merge-not-replace) so the size seam stays the sole writer of the reserved sizing keys | `internal/core/artifacts.go` | write-path integrity guard | **moderate** | revert; the guard is additive and fail-closed (a create/update that tries to inject reserved sizing keys is rejected/stripped), guarded by the SE-3a sole-writer tests and the SE-2 round-trip map assertion | standard review | planned |

No `ActionRisk: destructive` step exists — there is no deletion, force-overwrite,
or history rewrite. The durable append is fail-closed, not destructive.

### Deepened runtime verification (SE-3a/SE-3b, SE-7a/SE-7b)

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
- **SE-7a target scenarios:** (a) `RootDir: "..\\..\\outside"` rejected at load;
  (b) env-expansion escape in a search root rejected; (c) legitimate in-workspace
  config still loads.
- **SE-7b target scenarios:** (a) leaf-file symlink under a search dir refused at
  lookup (before `parseFile`); (b) legitimate in-workspace artifact still resolves
  and writes.

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
  revert SE-7a/SE-7b and treat as a security incident; (4) round-trip test regression on
  feature/shipment ⇒ the SE-2 guard is the **detector**; investigate and revert
  the codec or write-path change (SE-3a) that broke preservation.
- **Rollback procedure:** each unit is a coherent, revertible commit; the schema
  and provenance additions are optional/backward-compatible, so a revert cannot
  strand data (existing `custom_fields.size` values remain valid).
- **Owner / validation window:** the Ship agent owns runtime verification and
  closure; validate SE-3a/SE-3b and SE-7a/SE-7b across a full create→update→read cycle on
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
- **SE-6 CLI human-column parity** is deferred and tracked as filed stash
  **`D5FA1EE9`** (kind: task, priority: low, outside 099-S scope); JSON/MCP
  parity is the blocking SE-6 requirement.

## Plan Review

This plan passed a **genuine multi-persona, cross-model** review gate (NOT a
single-agent self-assessment). Reviewer persona subagents were dispatched in
parallel on different model tiers. Full findings are archived at
`docs/reviews/2026-07-18-108-F-size-estimation-plan-review.md`. **The FINAL gate is
the Copilot cycle-2 multi-persona re-review** (Architecture, Scope-Boundary,
Constitution, Security personas over the fully reconciled plan) recorded in the
"Copilot cycle-2 reconciliation" subsection below — it supersedes the earlier
per-cycle verdicts as the authoritative PASS covering the final plan state.

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
2. **Single-writer of reserved sizing keys (Architecture).** A generic create
   could inject reserved sizing keys, bypassing the provenance event; a generic
   update could whole-map-replace them. **Resolved:** the sole-writer guard
   (reject/strip on create, merge-preserve on update) is enforced by a minimal
   reserved-key guard in **SE-3a**. *(Reconciliation 2026-07-18: this guard was
   originally drafted inside SE-2 alongside a `ToFrontmatterMap()` emitter
   consolidation. Because the spike selected `custom_fields` with **no** codec
   bridge, SE-2 collapsed to a test-only round-trip guard and the write-path
   integrity moved to SE-3a; the emitter consolidation was descoped. The safety
   intent is unchanged — the reserved-key guard still ships in SE-3a.)*
3. **size_source human-masquerade via MCP (Security + Constitution).** **Resolved:**
   transport-aware actor stamping — MCP/agent rejects/overrides `human`; CLI stamps
   `human`; absent-on-read stays `unknown`, never rewritten.
4. **Positional-string seam signature (Architecture + Go).** **Resolved:** typed
   presence-aware `SizeMutation{Size, Source *string, RulesetVersion *string,
   Actor ActorContext, OpID *string}`. *(Reconciliation 2026-07-18, Copilot F2: to
   keep every commit buildable, SE-3a introduces the typed entry point as
   `SetArtifactSizeWithProvenance` and retains the existing `SetArtifactSize` as a
   thin compatibility wrapper; SE-5 migrates the `internal/cli/update.go` and
   `internal/mcp/tools.go` callers and removes the wrapper. No intermediate
   non-compiling state.)*

### Residual / deferred (non-blocking)

- **P2 (Go):** per-unit test-scenario counts sit at the 2-hour boundary; the plan
  states table-driven `t.Run` subtests count as one scenario. Post-reconciliation
  the unit closest to the envelope is **SE-3a** (typed seam + provenance event +
  reserved-key sole-writer guard across `artifact_size.go`/`stream.go`/`artifacts.go`);
  if it exceeds a single 2-hour envelope, split the reserved-key write-path guard
  into a sibling task.
- **P2 (Go), added by Copilot F4/F5, RESOLVED by Copilot G6:** SE-3b had grown two
  extra scenarios (retry-from-orphan idempotency and doctor CLEAR-a-field recovery)
  plus the presence-aware desired-state payload, pushing it at/over the 2-hour
  envelope. **Resolved:** SE-3b (`108.006-T`) is **split** — it keeps the online seam
  crash-safety (append-before-write, pre-append OpID dedup, and the G2 applied-check
  index re-upsert) and is now genuinely **≤2h**; the doctor **CLEAR-recovery
  reconcile** (incl. the presence-aware CLEAR recovery test) moves to the new sibling
  **SE-3c (`108.011-T`)**, which depends on SE-3b. No unit now sits over the envelope.
- **P3 (Security):** transport-aware stamping cannot stop masquerade if an agent
  is granted unrestricted local shell (it can invoke the CLI directly) — to be
  noted in the SE-8 contract doc.
- **P3 (Architecture):** exact `size_op_id`/`PrevOpID` JSON key names and doctor
  reporting verbosity are Ship-level implementation detail.
- **Deferred item:** SE-6 CLI human-column parity → filed stash **`D5FA1EE9`**
  (kind: task, priority: low), tracked outside 099-S.

### Copilot PR #259 reconciliation (2026-07-18)

External Copilot review on staging PR #259 raised 7 valid findings against the
099-S planning artifacts. All were accepted and reconciled in-plan and in the
backlog (no source/test/config code changed). Dispositions:

| # | Finding | Disposition |
|---|---|---|
| F1 (P1) | SE-7 combined config-load + core containment as a declared "width exception" — violates Width Isolation | **Split** SE-7 into `108.009-T` (SE-7a, config-load) and new `108.010-T` (SE-7b, lookup-time), wired `SE-7a → SE-7b`; 099-S manifest, dep graph, task table, Constitution Check, and memory updated. No task mixes skill domains (see Constitution Check for two documented bounded cross-file deviations surfaced in the cycle-2 re-review). |
| F2 (P1) | SE-3a signature change left callers uncompiled until SE-5 (non-buildable intermediate) | New entry point `SetArtifactSizeWithProvenance`; existing `SetArtifactSize` kept as a thin compat wrapper until SE-5 migrates callers and removes it. |
| F3 (P1) | SE-2 was framed as a red harness but its assertions are already green | Reframed SE-2 (`108.005-T`) as an **expected-green characterization/round-trip guard**; removed its SE-3a dependency (depends only on SE-1). |
| F4 (P1) | SE-3b retry not idempotent (append-then-write crash appends a duplicate event) | Pre-append OpID orphan-check: if an event with this OpID already exists, reconcile the pending write instead of re-appending; added a retry-from-orphan test. |
| F5 (P1) | Doctor recovery underspecified (cannot reconstruct mutation from OpID/PrevOpID alone) | Event payload now pins the full presence-aware desired state (size / size_source / size_ruleset_version) with SET/CLEAR semantics; added a CLEAR-a-field recovery test. |
| F6 (P2) | SE-6 referenced a "separate P2 follow-up" that did not exist | Filed concrete stash **`D5FA1EE9`**; plan §328 and `108.008-T` now name it. |
| F7 (P2) | Summary overstated two-way atomicity | Restated the actual hard invariant: append-then-write with crash-safe reconcile/dedup on retry; the event log is source of truth (no two-way atomicity claim). |

**Post-reconciliation gate:** PASS holds. The changes narrow scope, split a
cross-domain task into single-domain units, and add guardrail/idempotency detail;
no P0/P1 was reopened. New residual: SE-3b now sits at/over the 2-hour envelope
(see Residual/deferred P2 above).

### Copilot cycle-2 reconciliation (2026-07-18, G1–G8)

A second external Copilot pass on the pushed HEAD raised 10 findings (G1–G8; G9 is
the PR description, out of scope for Stage). All technical findings were verified
against the actual code (ground-truth facts supplied by the Orchestrator) and
reconciled in the planning/backlog artifacts only (no source/test/config changed).
Dispositions:

| # | Finding | Disposition |
|---|---|---|
| G2 (P1) | Idempotency must not return success solely on matching `size_op_id`: `SetArtifactSize` writes the file (`artifact_size.go:79`) before `db.UpsertItem` (`:90`), so a write-succeeded/index-failed crash leaves the SQLite row stale | On the applied-check path (op_id matches), **re-verify + re-`UpsertItem`** the artifact into the index before returning success. Added a "write-succeeded/index-failed → retry re-upserts the index" test. Impl-plan SE-3b + `108.006-T`. |
| G3 (P1) | Durability overstated: both `atomicfile.WriteFileAtomic` and `events.AppendEvent` are **sync-free** (no fsync), so ordering holds for **process** crashes only, not OS crash / power loss | **Narrowed** the invariant and all durability claims to **process-crash** scope (D4 summary, invariant #1, Decisions D4, SE-3b tail). Power-loss durability (a cross-cutting fsync protocol across the shared writers) is **out of scope for 099-S** and filed as stash **`131CEAE4`**. **No fsync added.** |
| G4 (P1) | `NormalizeShipmentItems` returns only explicit `custom_fields.items` verbatim; no feature→children expansion, so a feature-only manifest omits member sizes | SE-4 now specifies **explicit feature→children expansion**: when a shipment member is a feature, walk its child tasks and include their sizes. Tests for a **feature-only** manifest and a **feature-plus-explicit-child** manifest (dedup). Impl-plan SE-4 + `108.003-T`. |
| G5 (P1) | CLI/MCP JSON parity overclaimed: composition lands on MCP, but CLI `get` JSON (`buildDetailMap`) and queue JSON (`QueryQueue`) are separate shapers | **Composition scoped to MCP read surfaces only**; size/provenance parity still holds on both transports. The CLI-JSON composition gap is filed as stash **`387DE4BF`** (add composition to CLI shapers or a shared read-shaper) — distinct from the CLI human-column gap (`D5FA1EE9`). Impl-plan SE-6 + `108.008-T`. |
| G6 (P1) | SE-3b (`108.006-T`) admits at/over 2h after F4/F5 | **Split**: created new sibling **`108.011-T` (SE-3c)** "Doctor CLEAR-recovery reconcile" depending on `108.006-T`. SE-3b keeps append/write + orphan-dedup + G2 index re-upsert (now genuinely ≤2h; dropped the ">=2h" admission); SE-3c owns doctor CLEAR-recovery + the presence-aware CLEAR recovery test. 099-S → **12 members**; graph re-verified acyclic (**14 edges**). |
| G7 (P1) | `cli/migrate.go` passes imported metadata through `WithFields` → `CreateArtifact`; a blanket create reject/strip of size would break importing an already-sized item | **Migration-safe create**: preserve an imported size **when accompanied by provenance** (route through the seam, record the event); reject/strip only an **unprovenanced** reserved size. Added an "import an already-sized item preserves size + provenance" test. Impl-plan SE-3a + `108.001-T`. |
| G8 (P2) | `108.004-T` body still said "SE-7" | Updated `108.004-T` body to name **SE-7b (`108.010-T`)** explicitly. |

**G1 (P1, done last) — genuine multi-persona re-review of the FINAL plan.** After
applying G2–G8, four independent persona subagents reviewed the final plan
end-to-end (not a single-agent addendum):

| Persona (agent) | Model tier | Focus | Verdict |
|---|---|---|---|
| Architecture Strategist | Tier 2 | cohesion, coupling, dependency-graph acyclicity, seam/split design | **PASS** (P2/P3 advisories only) |
| Scope Boundary Auditor | Tier 2 | 2-hour rule, width isolation, YAGNI, deferral tracking | **PASS after fix** (raised SE-3a file-boundary P1 → documented) |
| Constitution Reviewer | Tier 2 | constitution mapping, Test-First honesty, buildable-commit | **PASS after fix** (raised SE-5 wrapper-removal P1 → resolved) |
| Security Lens Reviewer | Tier 2 | masquerade defense, path containment, crash-safety robustness | **PASS** (P2/P3 provenance-integrity items tracked) |

**Two P1s surfaced and were resolved in-plan (no source touched):**

1. **Scope P1 — SE-3a at the 3-file budget boundary.** Contested (Architecture P2,
   Constitution P3, Scope P1). Resolved by **documenting** it as a bounded,
   single-domain near-boundary case with an explicit **Ship-time split fallback**
   (carve the `artifacts.go` reserved-key guard into a sibling if implementation
   exceeds 2h), per the Governance Conflict-resolution clause. See Constitution Check.
2. **Constitution P1 — SE-5 wrapper retirement crossed into core + risked a
   non-buildable commit.** Resolved by making SE-5's scope explicit: it removes the
   `SetArtifactSize` wrapper **and migrates the core tests in the same commit**, so
   the tree stays buildable; documented as a bounded single-axis migration deviation.
   See SE-5 and Constitution Check.

**Cycle-2 gate: PASS** — genuine multi-persona coverage of the FINAL plan; the two
P1s are resolved (documented deviation + buildable-commit safeguard), leaving only
P2/P3 advisories carried as tracked residuals below. No unresolved P0/P1 remains.

**New tracked residuals from the multi-persona pass (P2/P3, non-blocking; Ship-level or filed):**

- **P2 (Security), migration provenance forgery (G7 path):** the import path preserves
  a file-claimed `size_source` (incl. `human`) verbatim rather than re-deriving it from
  the migrate actor context. SE-8 must document import as an equal-trust boundary to
  direct file authoring, or the migration event should record the migrate actor + an
  `imported` marker. Tracked for Ship/SE-8.
- **P2 (Security), OpID uniqueness:** OpID generation semantics are correctness-critical
  (dedup/CAS depend on it) and must be a collision-resistant source with a uniqueness
  test — promote from "Ship-level detail" to a pinned SE-3b requirement at implementation.
- **P2 (Security), competing same-`PrevOpID` orphans:** SE-3c must specify deterministic
  resolution (e.g., total-order by event seq/timestamp) so two orphans sharing a
  predecessor cannot nondeterministically drop a mutation; add a two-orphan test.
- **P2 (Security), containment choke point:** confirm SE-7b's realpath re-containment
  sits at the shared `FindArtifactPath` resolver (not only the size path) so all reads
  are contained; and that the sole-writer guard sits at the single create/update choke
  point; strip/regenerate any imported `size_op_id`.
- **P2 (Architecture), shared read-shaper:** prefer introducing a shared read-projection
  layer (stash `387DE4BF`) over per-shaper composition to prevent future parity drift.
- **P3:** label the `SE-3a → SE-7b` and `SE-5 → SE-3b` edges as release-ordering (not
  compile) dependencies; add `// Deprecated:` to the interim `SetArtifactSize` wrapper;
  confirm SE-6's CLI shapers already emit `custom_fields`; re-label `doctor_target.go`
  under SE-3c in the PA-1 row; keep `size_op_id` in a stable frontmatter position.
- **P3 (Scope), `size_ruleset_version` mutation surface:** the write/CLI/MCP surface for a
  field bounded to `null` today is near-YAGNI; Ship may keep only the schema field until a
  ruleset exists. Non-blocking.
