---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for extending optional size estimation to feature and shipment artifacts: canonical custom_fields.size at all levels (the spike-selected carrier; no codec bridge), a round-trip regression guard, event-before-write best-effort-audit provenance (durable size is source of truth; orphan events ignored on read), computed-on-read composition rollups, CLI/MCP mutation and read parity, and two-layer containment hardening of the size seam.'
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
  (`:183-193` provenance-event mandate; `:652-664` proceed authorization).

### Authoritative decisions carried in from the spike (do not re-open)

| # | Decision | Spike anchor |
|---|---|---|
| D1 | **Canonical size-location = `custom_fields.size` at all levels.** Task already there; extend to `feature` and `shipment`. Coexist; **no migration**. | §9(e), Recommendation 1 |
| D2 | **Inheritance bridge = no `models.Artifact` carrier; store under `custom_fields`.** The docline-carrier bridge is rejected for now (documented, reversible). | §9(d), Recommendation 2 |
| D3 | **Provenance fields = `size_source` {human, agent, derived} and `size_ruleset_version` (null until a ruleset is owned), under `custom_fields`, validated at the size seam.** Absent `size_source` reads as `unknown`/legacy and is **never rewritten as `human`**; unknown values are **rejected** with the same category as an invalid size. | §7, §9(f), Provenance |
| D4 | **Durability policy = event-before-write, best-effort audit ordering; durable size is the source of truth.** The estimate-history event appends **first** (gate-evidence precedent) as a **best-effort observability/audit trail**; an append failure **refuses** the size write (fail-closed on the write path), so a normal completion always carries its audit event. The durable `custom_fields.size` (written `WriteFileAtomic` then `db.UpsertItem`) is the **sole source of truth**: reads are **fail-closed on the event stream** — any orphan appended event left by a crash (append succeeded, artifact write did not land) is simply **IGNORED on read** and never replayed. **Descoped (Copilot cycle-3 H1/H5, Option B2, stash `9D5BB492`):** crash-window exactly-once, OpID-based dedup, the `size_op_id` reserved key, the `PrevOpID` predecessor chain, and doctor reconciliation are **removed** — no OpID transport ingress exists on CLI/MCP, so exactly-once could not be honored across a client retry. This ordering is **process-crash safe only** (Copilot G3): both shared writers are sync-free (`atomicfile.WriteFileAtomic`, `events.AppendEvent`), so OS-crash / power-loss durability is **out of scope for 099-S** and tracked as stash `131CEAE4`. | §9(a), Provenance |
| D5 | **Composition = computed-on-read, never persisted.** XS–XL count histogram + `unsized` count + de-duplicated canonical members array; `ruleset_version = null`. Feature membership = children by `parent_id`; shipment membership = `NormalizeShipmentItems`. An **existing artifact with no size** → `unsized`; an **unresolved manifest id** (`ErrNotFound`) → **warn+skip** (not counted). Comparator XS<S<M<L<XL mirrors the priority `CASE` ordered-enum. | §8, §9(b) |
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
(3) persist provenance with a **best-effort append-before-write** estimate-history
audit event (durable size is the source of truth; orphan events are ignored on
read), keeping the
size seam the sole writer of the reserved sizing keys (merge-not-replace on
generic update, reject/strip only an **unprovenanced** size on generic create —
a **provenanced** migration import is preserved, Copilot G7); (4)
compute (never persist) composition rollups; (5) reach CLI/MCP mutation and read
parity; (6) close the containment gap in the seam's lookup/write path; (7)
document the contract.

## Requirements Trace

| Requirement (from spike) | Implementation action | Unit |
|---|---|---|
| D1 canonical `custom_fields.size` on feature/shipment | Add `size` enum to `feature`/`shipment` header-def types + defaults; enable seam validation for those types | SE-1 |
| D3 provenance field definitions | Add `size_source` (enum) and `size_ruleset_version` (**bounded**, not free-text) to task/feature/shipment header-def | SE-1 |
| D2 no carrier bridge; durability of `custom_fields.size` | **Extend the committed round-trip guard** (`docline_codec_roundtrip_test.go`) to assert feature/shipment `size` + `size_source` + `size_ruleset_version` survive the generic codec under `custom_fields` (docline-drop guard unchanged). No codec bridge is built. | SE-2 |
| D2 sole-writer integrity of reserved sizing keys | Keep the size seam the **sole writer** of `custom_fields.size`/`size_source`/`size_ruleset_version`: merge-not-replace on generic update (closes the `updateArtifactUngated` whole-map-replace hazard); **migration-safe** generic create (**Copilot G7**) — preserve a **provenanced** imported size (record its event) and reject/strip only an **unprovenanced** reserved size, so an initial size is never eventless yet `cli/migrate.go` import neither loses nor fails on an already-sized item | SE-3a |
| D4 best-effort audit provenance | Typed `SizeMutation` seam that persists `size_source`/`size_ruleset_version`; append estimate-history event **before** the write (best-effort audit), fail-closed on the write path | SE-3a |
| D3 actor-context stamping | New-authored size with no explicit `size_source` stamped from actor context (CLI ⇒ `human`, agent/MCP ⇒ `agent`); absent-on-read stays `unknown`, never rewritten | SE-3a / SE-5 |
| D4 crash posture (best-effort audit) | Append-before-write ordering with the **durable size as source of truth**; the write path re-upserts the SQLite index so a normal set is index-consistent; **fail-closed on read** (orphan crash-residue events are ignored, never replayed); **process-crash scope only** (sync-free writers; power-loss out of scope, stash `131CEAE4` — **Copilot G3**). Exactly-once / OpID dedup / doctor reconciliation **descoped** (Option B2, stash `9D5BB492`). **Implemented in SE-3a; verified by SE-3b** (test-only crash-residue characterization). | SE-3a (impl) / SE-3b (tests) |
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
  RulesetVersion *string, Actor ActorContext })` — and **keep the
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
  (`size`/`size_source`/`size_ruleset_version`) — a generic **update**
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
  `internal/core/artifacts.go` (generic create reject/strip of an **unprovenanced**
  reserved size — a **provenanced** import is preserved, G7 — plus update merge-preserve).
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

### SE-3b — Crash-audit robustness verification (test-only; verifies SE-3a)

- **Changes (test-only recast — Copilot cycle-5 J1):** **SE-3b adds NO production
  code.** After the Option B2 descope (cycle-3 H1/H5), all production behavior —
  the append-before-write ordering, the fail-closed refuse-on-append-failure, the
  `WriteFileAtomic` → `db.UpsertItem` file-then-index re-upsert, and the "ignore
  orphan events on read" posture (which needs no new read-path code: reads already
  consult only the durable `custom_fields.size`) — lives entirely in **SE-3a** on
  `internal/core/artifact_size.go`. SE-3b is therefore an explicitly **test-only
  robustness/characterization unit** that VERIFIES SE-3a's crash-audit semantics; it
  introduces no new seam, signature, or read path. It exists to lock the crash-window
  behavior as an executable regression guard, so a future change to the seam cannot
  silently regress the "durable size is sole source of truth" contract.
- **Test-ownership line (non-overlapping with SE-3a):** SE-3a owns the *happy-path
  ordering* tests — "event appended **before** the write" and "forced **append**
  failure ⇒ no persisted change (fail-closed write)". SE-3b owns the *crash-residue*
  tests: (a) **fail-closed READ ignores an orphan event** — an event that landed while
  the subsequent artifact write did **not**, so the durable `custom_fields.size` stays
  the sole source of truth; (b) the orphan event is **never replayed** and the
  **shared per-item JSONL is never truncated** to "roll it back" (truncation could
  delete a concurrent legitimate event from a non-size writer that does not hold the
  per-task lock — **forbidden**); (c) a **normal size set is index-consistent** (the
  write-path `db.UpsertItem` re-upserts the row after the file write).
- **Rationale carried from the descope:** the estimate-history event is a
  **best-effort observability/audit trail**, not a source of truth; the hard read
  invariant is **"the durable size is authoritative; the audit stream is never read
  back as truth."** OpID-based dedup, exactly-once, the `size_op_id` reserved key, the
  `PrevOpID` predecessor chain, and doctor reconciliation remain **dropped** (deferred
  to stash **`9D5BB492`**). No `OpID` transport ingress exists. (See the revised
  Decisions and Invariant #1.)
- **Durability scope (process-crash only — Copilot G3):** these characterization tests
  assert **process-crash** semantics, **not** OS-crash / power-loss. Both shared writers
  are **sync-free**: `atomicfile.WriteFileAtomic`
  (`internal/atomicfile/atomicfile.go:15-63`) does temp-write → `Chmod` → `Close` →
  `Rename` with **no `fsync`** before the rename, and `events.EventWriter.AppendEvent`
  (`internal/events/stream.go:40-64`) does an `O_APPEND` write with **no `f.Sync`**.
  Closing the power-loss gap would need a separate `fsync` protocol coordinated across
  **both** shared writers — **cross-cutting, out of scope for 099-S**, filed as stash
  **`131CEAE4`**. This unit must **not** add `fsync` (it adds no production code at all).
- **Files:** test-only — `internal/core/artifact_size_test.go` (crash-residue /
  fail-closed-read characterization). **No production file.**
- **Tests (unit; table-driven `t.Run` subtests count as one scenario):** (a) a **forced
  write failure after a successful append** leaves an **orphan audit event that a
  subsequent read IGNORES** (durable `custom_fields.size` unchanged); (b) the orphan is
  **never replayed** and the **shared JSONL is never truncated**; (c) a normal size set
  **re-upserts the SQLite index** so the row is consistent (file-then-index). No
  exactly-once / OpID / doctor assertions remain.
- **Execution posture:** test-first (crash-injection at the post-append boundary), but
  the code-under-test is SE-3a's seam — SE-3b writes only the tests.
- **Milestone:** tests green; **no new production code** — SE-3b verifies SE-3a's
  crash-audit semantics. Single-domain (test), ≤2h. Depends on SE-3a (`108.002-T`).

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
  listing a feature **and** its explicit child tasks counts each item **once**.
  **Membership resolution disambiguation (Copilot cycle-3 H6):** an **existing
  artifact that has no size** increments the `unsized` count; an **unresolved manifest
  id** (`ErrNotFound` — the id resolves to **no artifact**) is **warn+skipped** and is
  **not** counted in the histogram or in `unsized` (optionally surfaced as a separate
  `skipped`/`unresolved` count); other resolution errors are warn+skipped likewise
  (`DeriveCoveringFeature` precedent). Comparator XS<S<M<L<XL mirrors the priority
  `CASE` ordered-enum (`internal/core/queue.go:183-191`). `ruleset_version = null`.
- **Files:** new `internal/core/size_composition.go`.
- **Tests (unit):** (1) an **existing-but-unsized** child increments `unsized` (+1)
  **and** a **dangling manifest id** (`ErrNotFound`) is **skipped** — not counted,
  `unsized` unchanged by the dangling id, warn emitted; (2) a shipment whose manifest
  lists **only a feature** expands to that feature's children and counts their sizes
  (**Copilot G4**); (3) a shipment listing a feature **plus** its explicit child tasks
  **de-dups** so each item is counted once (**Copilot G4**); (4) the function never
  writes to disk (no persistence side effects). Table-driven `t.Run` subtests keep
  SE-4 inside the 2-hour envelope.
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
  size) is separately filed as stash **`D5FA1EE9`**. **File-count reconciliation
  (Copilot cycle-3 H4):** size/provenance parity requires **no new CLI production
  code** — CLI `get` JSON already projects `custom_fields.size` verbatim because
  `internal/cli/get.go` `buildDetailMap` copies the **entire frontmatter map** (which
  includes `custom_fields`) into the detail map, so `size`/provenance rides free on
  `get --json`. The **single production change** in SE-6 is therefore **MCP-only**:
  expose the SE-4 composition on the MCP read surfaces in `internal/mcp/tools.go`. This
  keeps SE-6 to **one production file** within the 2-hour envelope.
- **Files:** `internal/mcp/tools.go` (single production file) + parity tests.
- **Tests (unit):** size/provenance projects **identically** on each named CLI/MCP
  read pair (proving the no-code-change CLI parity); `get_item` JSON exposes
  `custom_fields.size` **and** the derived composition on a sized feature; CLI `--json`
  exposes `custom_fields.size` (size/provenance parity) — composition on CLI JSON is
  **out of scope** (stash `387DE4BF`).
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
  rejection; the event-before-write best-effort-audit durability policy (the durable
  `custom_fields.size` is the source of truth; orphan crash-residue events are ignored
  on read);
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
two roots; SE-3a and SE-4 can be built in parallel once their inputs exist (SE-3b
is a **test-only** unit whose characterization tests follow SE-3a):

```text
SE-1 ─┬▶ SE-2 (test guard; leaf)
      ├▶ SE-3a ─▶ SE-3b ─▶ SE-5 ─▶ SE-8
      │    └───────────────▶ SE-6 ─▶ SE-8
      └▶ SE-4 ──────────────▶ SE-6
SE-7a ─▶ SE-7b ──┬▶ SE-3a
                 └▶ SE-8
```

(SE-2 is a **test-only, expected-green** regression guard depending solely on SE-1;
**nothing depends on SE-2**, and it has **no** dependency on SE-3a — the codec
preserves `custom_fields` with no schema/persistence step, so the guard passes
against current code. SE-7a and SE-7b are the two width-isolated containment layers,
wired `SE-7a → SE-7b` so the D6 invariant ships together. SE-3a depends on SE-1 and
**SE-7b** — the schema fields plus the hardened lookup-time seam (transitively SE-7a)
— no longer on any codec bridge. **SE-3c was removed** (Option B2 descope, Copilot
cycle-3 H1/H5): the offline doctor CLEAR-recovery reconcile existed only to serve the
now-descoped exactly-once ambition, so `108.011-T` is archived and the deferred
ambition is tracked as stash `9D5BB492`. SE-6 depends on SE-3a and SE-4; SE-8 depends
on SE-5, SE-6, and SE-7b. There is **no** SE-2→SE-4 and **no** SE-2→SE-3a edge.)

Explicit edges to wire with `backlogit dep add <task> <depends_on> --type blocks`
(12 edges total):

- SE-2 depends on SE-1 (green characterization guard; references the schema-defined
  feature/shipment keys — it does **not** depend on SE-3a)
- SE-4 depends on SE-1 (composition consumes the schema contract; it does not
  require the emitter consolidation — de-serialized per review)
- SE-7b depends on SE-7a (the two containment layers release as one D6 invariant)
- SE-3a depends on SE-1 and SE-7b (schema fields + the hardened lookup-time seam
  before routing provenance writes through it)
- SE-3b depends on SE-3a (SE-3b's crash-audit characterization tests verify SE-3a's
  seam behavior; SE-3b adds no production code)
- SE-5 depends on SE-3b (release-ordering, not compile: land the crash-audit
  characterization tests before SE-5 migrates callers and retires the compat wrapper)
- SE-6 depends on SE-3a and SE-4 (read projection surfaces persisted size +
  composition)
- SE-8 depends on SE-5, SE-6, and SE-7b (documents the final contract)

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
- **Event-before-write, best-effort audit; durable size is source of truth (D4,
  Option B2 descope — Copilot cycle-3 H1/H5).** The charter mandated exactly-once
  provenance events (`plan:183-193`), but no `OpID` transport ingress exists on
  CLI/MCP (SE-5 exposes none, no caller carries one), so exactly-once could not be
  honored across a client retry. Rather than ship an unreachable half-mechanism, the
  ambition is **descoped** (stash `9D5BB492`). The landed policy: the estimate-history
  event is a **best-effort observability/audit trail** appended **before** the durable
  write; an append failure **refuses** the write (fail-closed write path) so a normal
  completion always has its event. The durable `custom_fields.size` (written
  `WriteFileAtomic` then `db.UpsertItem`) is the **sole source of truth** —
  **fail-closed on read**: an orphan appended event left by a crash is **ignored on
  read** and never replayed. Compensating truncation of the **shared** per-item JSONL
  remains **forbidden** (non-size writers do not hold the per-task lock — truncation
  can delete a concurrent legitimate event). This preserves the charter's audit intent
  (a normal completion is always evented) without the unsafe truncation and without an
  unreachable exactly-once claim. Process-crash scope only (Copilot G3); power-loss is
  stash `131CEAE4`.
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
- **Single seam, sole writer (no emitter consolidation).** The typed seam
  `SetArtifactSizeWithProvenance` is the only `custom_fields.size` writer (the
  positional `SetArtifactSize` is a thin compat wrapper delegating to it, **retired by
  SE-5**); SE-3a adds a minimal reserved-key guard (generic create **rejects/strips an
  unprovenanced** reserved size but **preserves a provenanced** import, generic update
  merge-not-replace) so the map-replacement caveat cannot be weaponized and a
  migration import is never lost. The `ToFrontmatterMap()` two-emitter consolidation
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
- **Best-effort audit ordering, implemented in SE-3a and verified by SE-3b:** the
  durability policy is decided (best-effort append-before-write; durable size is
  source of truth; fail-closed on read); the append+write ordering mechanism is
  **implemented in SE-3a's seam** (`internal/core/artifact_size.go`), and the
  crash-residue robustness is **verified by SE-3b, a test-only characterization unit**
  (it adds no production code). The review **rejected**
  compensating truncation of the shared per-item JSONL (concurrent non-size
  writers do not hold the per-task lock — truncation can delete a legitimate
  event). The landed mechanism is therefore **append-before-write with the durable
  size as source of truth**: orphan crash-residue events are ignored on read, never
  replayed, and never resolved by mutating the shared log. Exactly-once / OpID dedup /
  doctor reconciliation are **descoped** (Option B2, stash `9D5BB492`).
- **CRLF/normalization drift:** the generic `ParseFrontmatter` normalizes
  CRLF→LF document-wide while `mdfront` preserves body bytes; round-trip tests
  (SE-2) must assert on the frontmatter map, not byte-identity of the body.
- **Cosmetic read gap is non-blocking and out of SE-6:** the CLI human-column
  size omission is filed as stash **`D5FA1EE9`** (P2 backlog follow-up), not part of SE-6,
  so SE-6 stays a **single production file** (MCP composition; CLI `get --json` already
  projects `custom_fields.size` with no code change) inside the 2-hour
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
| SE-3a | Yes (mutation seam) | event appended before write proven; forced-append-failure refuses write; a `size_ruleset_version`-only change still emits its event; actor-context stamping (CLI human / agent) proven | rollback trigger: any persisted size without its best-effort event ⇒ revert; owner + validation window |
| SE-3b | No (test-only) | verifies SE-3a's crash-audit semantics: a forced write failure after append leaves an **orphan audit event that a read IGNORES** (durable size unchanged, sole source of truth); the orphan is **never replayed** and the **shared JSONL is never truncated**; a normal set **re-upserts the SQLite index** (file-then-index) | the crash-residue characterization tests are the closure evidence |
| SE-4 | No (pure read) | composition deterministic; never persists | n/a |
| SE-5 | Yes (CLI + MCP) | both surfaces reach the seam; parity error categories; agent human-masquerade rejected | parity matrix in contract doc |
| SE-6 | Yes (CLI + MCP read) | size/provenance visible with JSON/MCP parity on **both** transports; derived composition on **MCP read surfaces only** (CLI-JSON composition deferred to stash `387DE4BF`; CLI human columns to stash `D5FA1EE9` — **Copilot G5**) | read-parity matrix in contract doc |
| SE-7a | Yes (config-load) | lexical `..` config rejected; env-expansion escape rejected; in-workspace config still loads | containment regression tests; rollback trigger: any accepted escaping config |
| SE-7b | Yes (lookup/write path) | symlink lookup refused before `parseFile`; in-workspace still resolves | containment regression tests; rollback trigger: any out-of-workspace read/write |
| SE-8 | No (docs) | docline lint clean | the doc itself |

When SE-3a/SE-3b and SE-7a/SE-7b are hardened (below), the downstream Ship
runtime-verification and operational-closure steps should carry the fail-closed
write-refusal path, the best-effort-audit / fail-closed-on-read note, and the
containment invariants as explicit checks.

## Constitution Check

Mapping this plan against `.github/instructions/constitution.instructions.md`.
(This section closes the known governance gap that no prior artifact recorded a
constitution mapping for 108-F.)

| Principle | Compliance |
|---|---|
| **I. Safety-First Go** | All units wrap errors with `fmt.Errorf("...: %w", err)`; provenance rejection reuses the `ErrValidation` sentinel; no `unsafe`. Gates (`go vet`, `golangci-lint`, `gofmt`) run in Ship, not Stage. |
| **II. Test-First (NON-NEGOTIABLE)** | Every production code unit is red-first (write the failing test, then implement). **SE-2 and SE-3b are deliberate test-only characterization units**, not skipped red phases: SE-2 locks the generic codec's `custom_fields` preservation, and SE-3b locks SE-3a's crash-residue / fail-closed-read semantics — each ships an executable committed regression guard against behavior implemented elsewhere. |
| **II. Task Granularity / Width Isolation (NON-NEGOTIABLE)** | Every task targets a single skill domain and ~2h. **Two bounded deviations are documented below** (SE-3a file-count boundary; SE-5 cross-file wrapper retirement) per the Governance Conflict-resolution clause — neither mixes unrelated skill domains. SE-3b is single-domain (**test**). |
| **III. Workspace Isolation** | SE-7a/SE-7b *strengthen* isolation (two-layer lexical + realpath containment, split by domain). No secrets are added. |
| **IV. CLI Containment (NON-NEGOTIABLE)** | SE-7a enforces containment at config-load and SE-7b at lookup time; no unit writes outside the workspace root. |
| **V. Structured Observability** | SE-3a adds a durable estimate-history event stream (traceable provenance) — an observability *gain*; SE-3b (test-only) verifies the durable size stays authoritative and the audit event stays best-effort (orphan crash-residue ignored on read). |
| **VI. Single Responsibility** | No new external dependency (SE-3a reuses existing `atomicfile`/JSONL rails; SE-3b adds **no** production code — test-only verification); no new carrier is added to `models.Artifact` — the spike-selected `custom_fields` carrier is reused, so the codec surface is unchanged. The optional two-emitter consolidation is descoped to avoid speculative refactoring; the exactly-once/doctor machinery is descoped (Option B2) to avoid an unreachable half-mechanism. |
| **VII. Destructive Command Approval** | No destructive terminal commands. The durable append is fail-closed, not force-overwrite; the design (SE-3a) forbids truncating the shared log, and SE-3b's tests assert it is never truncated. |
| **VIII. Safety Modes** | Elevated blast radius (mutation seam + containment) is flagged; `Requires plan hardening: yes`; Ship should operate in careful/investigate-first posture for SE-3a/SE-7a/SE-7b (SE-3b is test-only). |
| **IX. Git-Friendly Persistence** | `custom_fields.*` stays human-readable YAML frontmatter; `mdfront` preserves body bytes and semantic ordering. |
| **X. Context Efficiency** | Composition is computed-on-read via targeted membership queries, not bulk scans; MCP read projection returns structured `custom_fields`. |
| **XI. Merge Commit History** | Not applicable to planning; Ship enforces merge-commit strategy at PR time. |

**Justified deviations (documented per Governance Conflict-resolution clause):**
The former SE-7 "width exception" (a single task spanning config + core) is
**removed** — SE-7 is split into the single-domain SE-7a/SE-7b, wired
`SE-7a → SE-7b`. The former SE-3b/SE-3c split (Copilot G6) is now **moot**: SE-3c is
**removed** in the Option B2 descope (Copilot cycle-3 H1/H5), and SE-3b is now recast
(Copilot cycle-5 J1) as a single **test-only** verification unit (verifying SE-3a's
seam) with no offline-doctor sibling and no production file of its own. No task
**mixes skill domains**. Two bounded, single-domain deviations remain and are
explicitly documented rather than asserted away (**Copilot G1 multi-persona
re-review, cycle 2**):

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
  stamping, relevant to SE-3a's best-effort provenance events.
- Compound library: no existing `docs/compound/` learning contradicts this plan;
  the closest prior art is the fail-closed / "absence is not a pass" family
  (e.g. `exported-cache-zero-value-bypass`) — SE-3a's fail-closed refusal and
  SE-7a/SE-7b's "unknown path ⇒ refuse" both apply that governing rule.

### Protected invariants

1. **Best-effort provenance audit (durable size is source of truth):** every
   *normal* persisted `custom_fields.size` / `size_source` / `size_ruleset_version`
   change is preceded by a best-effort estimate-history event (append-before-write;
   append failure **refuses** the write, so a completed change is always evented).
   The reciprocal ("no event without a persisted change") is **not** guaranteed:
   a crash-residue orphan event (append landed, write did not) is **ignored on read**
   — the durable `custom_fields.size` is the **sole source of truth**. The shared
   JSONL is **never** truncated to reconcile. Scope: **process-crash** safety only;
   power-loss durability (sync-free writers) is out of scope (stash `131CEAE4`).
   Exactly-once / OpID dedup / offline doctor reconciliation are **descoped**
   (Option B2, stash `9D5BB492`). (SE-3a/SE-3b)
2. **Sole writer (create + update):** the **typed seam
   `SetArtifactSizeWithProvenance`** is the only writer of
   `custom_fields.size`/`size_source`/`size_ruleset_version` (the positional
   `SetArtifactSize` is a thin compat wrapper that delegates to it and is **retired by
   SE-5**, so it is not a second writer). No generic **update** path may inject or
   replace these reserved keys — a generic update **merge-preserves** existing reserved
   keys. A generic **create** distinguishes provenance: one carrying **unprovenanced**
   reserved sizing keys **rejects/strips** them (an initial size is never eventless,
   protecting Invariant #1), while one carrying a **provenanced** size (the
   migration-safe import path, Copilot G7) is **preserved** and routed through the seam
   so its estimate-history event is recorded. Enforced by a minimal reserved-key guard
   in **SE-3a** (no emitter consolidation). The round-trip guard (SE-2) proves the keys
   survive (expected-green characterization); the CRLF-safe map assertion covers the
   codec. (SE-3a, SE-2, SE-5)
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
| PA-1 | Route provenance writes through a best-effort event-before-write append+write in the live size seam (durable size is source of truth; orphan events ignored on read) | `internal/core/artifact_size.go`, `internal/events/stream.go` | durable append + mutation (contract) | **high** | revert the seam commit; the append is additive to the JSONL event stream and index-rehydratable; a failed write leaves the prior size intact (fail-closed write path); orphan crash-residue events are ignored on read, **never** resolved by truncating the shared log | prefer approval (production mutation-path + failure-semantics change) | planned |
| PA-2 | Add two-layer lexical + realpath containment, split by domain: SE-7a at config-load, SE-7b at lookup time | `internal/config/loader.go` (SE-7a), `internal/core/artifacts.go` (SE-7b) | config validation + security guard | **high** | revert; new rejections are fail-closed (worst case: a previously-accepted escaping config is now refused — surface a clear diagnostic) | prefer approval (changes accepted-config surface) | planned |
| PA-3 | Extend header-def with size on feature/shipment + provenance fields | `.backlogit/header-def.yaml`, `internal/config/defaults.go` | schema/contract | **moderate** | revert; fields are `optional`, additive, backward-compatible (coexist, no migration) | standard review | planned |
| PA-4 | Add a minimal reserved-key write-path guard (create reject/strip of an **unprovenanced** reserved size — a **provenanced** import is preserved, G7 — + update merge-not-replace) so the typed size seam stays the sole writer of the reserved sizing keys | `internal/core/artifacts.go` | write-path integrity guard | **moderate** | revert; the guard is additive and fail-closed (a create/update that tries to inject an unprovenanced reserved sizing key is rejected/stripped; a provenanced import is preserved and its event recorded), guarded by the SE-3a sole-writer tests and the SE-2 round-trip map assertion | standard review | planned |

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
  orphan audit event + no persisted change, and a subsequent **read ignores the
  orphan** (durable size unchanged, source of truth); (b) a normal size set
  **re-upserts the SQLite index** (file-then-index ordering holds); (c) the shared
  per-item JSONL is **never** truncated.
- **SE-7a target scenarios:** (a) `RootDir: "..\\..\\outside"` rejected at load;
  (b) env-expansion escape in a search root rejected; (c) legitimate in-workspace
  config still loads.
- **SE-7b target scenarios:** (a) leaf-file symlink under a search dir refused at
  lookup (before `parseFile`); (b) legitimate in-workspace artifact still resolves
  and writes.

### Operational closure detail

- **Monitoring signals:** count of estimate-history events vs. count of persisted
  size changes should stay consistent under the best-effort audit policy (events ≥
  persisted changes; any *persisted change with no event* is the alert
  condition); orphan crash-residue events are ignored on read, not alerts. Any
  containment-refusal log line during normal operation indicates a
  misconfiguration to investigate.
- **Rollback triggers:** (1) any persisted size change observed without a
  matching event ⇒ revert **SE-3a** (the production seam; SE-3b is test-only); (2)
  any shared-JSONL truncation, or any read that trusts an orphan event over the
  durable size ⇒ the **SE-3b crash-residue tests are the detector**, revert the
  **SE-3a** change that broke the invariant; (3) any
  out-of-workspace read/write observed ⇒
  revert SE-7a/SE-7b and treat as a security incident; (4) round-trip test regression on
  feature/shipment ⇒ the SE-2 guard is the **detector**; investigate and revert
  the codec or write-path change (SE-3a) that broke preservation.
- **Rollback procedure:** each unit is a coherent, revertible commit; the schema
  and provenance additions are optional/backward-compatible, so a revert cannot
  strand data (existing `custom_fields.size` values remain valid).
- **Owner / validation window:** the Ship agent owns runtime verification and
  closure; validate SE-3a (with SE-3b's crash-residue tests) and SE-7a/SE-7b across a full create→update→read cycle on
  a scratch feature and shipment before the observation window closes.

### Unresolved operator decisions

- **SE-3a durability policy — pinned (Option B2 descope; verified by SE-3b tests).** The estimate-history
  event is a **best-effort audit trail** appended before the durable write (append
  failure refuses the write). The durable `custom_fields.size` (written
  `WriteFileAtomic` then `db.UpsertItem`) is the **sole source of truth**; a
  crash-residue orphan event is **ignored on read**, never replayed, and the shared
  JSONL is never truncated. No `size_op_id` key, no OpID transport ingress, no
  predecessor chain, no offline doctor reconciliation. The deferred exactly-once
  ambition is filed as stash **`9D5BB492`**. Only the exact event JSON field names
  remain Ship-level detail.
- **SE-6 CLI human-column parity** is deferred and tracked as filed stash
  **`D5FA1EE9`** (kind: task, priority: low, outside 099-S scope); JSON/MCP
  parity is the blocking SE-6 requirement.

## Plan Review

This plan passed a **genuine multi-persona, cross-model** review gate (NOT a
single-agent self-assessment). Reviewer persona subagents were dispatched in
parallel on different model tiers. Full findings are archived at
`docs/reviews/2026-07-18-108-F-size-estimation-plan-review.md`. **The FINAL gate is
the Copilot cycle-5 multi-persona re-review** (Architecture, Scope-Boundary,
Constitution, Security personas over the J1/J2-reconciled plan) recorded in the
"Copilot cycle-5 reconciliation" subsection below — it supersedes the earlier
per-cycle verdicts as the authoritative PASS covering the final plan state.
Both cycle-3 (Option B2, removing task SE-3c `108.011-T`, narrowing the D4
invariant to best-effort audit, and deleting the OpID/exactly-once machinery) and
cycle-5 (J1 recasting SE-3b as a **test-only** verification unit that adds no
production code, and J2 restating the sole-writer invariant on the typed seam) are
**strictly scope-reducing / clarifying** — they remove scope and sharpen wording
without adding capability — so the earlier multi-persona PASS holds *a fortiori*
over the smaller, simpler final surface.

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
   Actor ActorContext}`. *(Reconciliation 2026-07-18, Copilot F2: to
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

### Copilot cycle-3 reconciliation (2026-07-18, H1–H6, Option B2 descope)

A third external Copilot pass (wave-3, HEAD `20ad40f`) raised 6 findings (H1–H6).
The Orchestrator selected **Option B2**: descope the crash-window exactly-once
idempotency ambition at the root rather than continue patching it (the recurring
review magnet F4 → G3 → H1). All findings were verified against actual code and
reconciled in planning/backlog artifacts only (no source/test/config changed).
Dispositions:

| # | Finding | Disposition |
|---|---|---|
| H1 + H5 (P1) | The seam generates the OpID internally and no CLI/MCP caller carries one, so a client retry submits a NEW id and the orphan is never deduplicated — the "exactly-once crash-window retry" claim is unsupported (H1). SE-3c's CAS reconcile is nondeterministic when two crash residues share a `PrevOpID` (H5). | **Option B2 — remove the ambition, not patch it.** SE-3b (`108.006-T`) narrowed to **best-effort append-before-write audit**: append an intent event before the durable write (append failure refuses the write); durable size is source of truth; **fail-closed on read** (orphan events ignored, never replayed). **Dropped entirely:** OpID dedup, exactly-once, `size_op_id`, `PrevOpID`, client-retry-reuses-OpID, OpID transport ingress. **SE-3c (`108.011-T`) removed from 099-S and archived** — its whole rationale (reconciling orphans for exactly-once) is descoped, eliminating H2 (wrong entry point) and H5 (two-orphan ordering) at the root. 099-S → **11 members**. Dependency edges `108.011←108.006` and `108.004←108.011` removed; `108.004` predecessor restored to SE-5/SE-6/SE-7b (`108.007`, `108.008`, `108.010`). Full deferred ambition filed as stash **`9D5BB492`** (requires stable/transport-visible OpID ingress + deterministic multi-orphan ordering + reachable offline doctor via the real `internal/core/doctor.go` `Doctor()`, not `doctor_target.go`). |
| H3 (P1) | `108.007-T` (SE-5) omits the compat-wrapper retirement + core-test migration, so Ship would leave a dead `SetArtifactSize` compat wrapper behind. | `108.007-T` body + Files list now explicitly include **retiring the `SetArtifactSize` compat wrapper and migrating the core tests off it in the same buildable commit**. Plan SE-5 task map confirmed to match. |
| H4 (P1) | `108.008-T` (SE-6) claimed "two-file ≤2h" but listed 3 production files (`tools.go`, `get.go`, `list.go`), violating the <3-files rule; G5-corrected text says composition is MCP-only. | Verified in code: `cli/get.go buildDetailMap` copies the full frontmatter map (incl. `custom_fields`) → `get --json` **already** projects `custom_fields.size` with **no code change**; `cli/list.go`/`queue_cmd.go` shapers omit `custom_fields` (the descoped human-column gap, stash `D5FA1EE9`). SE-6's only production change is **MCP composition in `internal/mcp/tools.go` (one file)**. `108.008-T` + plan SE-6 row reconciled to single-production-file; file count, prose, and the <3-files rule now agree. |
| H6 (P1) | `108.003-T` (SE-4) / plan self-contradicted: "missing member ⇒ unsized; skip ErrNotFound" — a missing artifact cannot both increment `unsized` AND be skipped. | Disambiguated: an **existing artifact with no size** ⇒ `unsized`+1; an **unresolved manifest id (ErrNotFound — resolves to no artifact)** ⇒ **warn + skip**, NOT counted in the histogram (optionally surfaced as a separate `skipped`/`unresolved` count). Acceptance test asserts the chosen counts (existing-but-unsized child → unsized+1; dangling manifest id → skipped, unsized unchanged). `108.003-T` + plan SE-4 reconciled. |

**Superseded residuals (from cycles 1–2, now MOOT under Option B2):** the following
earlier findings/residuals targeted the exactly-once/OpID/doctor machinery that
cycle-3 **removed**; they no longer describe the plan and require no further work:

- **F4 / F5** (retry-from-orphan idempotency; doctor CLEAR-recovery) — the mechanism they
  hardened is descoped; SE-3b no longer dedups by OpID and there is no doctor reconcile.
- **G2** (re-upsert the index on the applied-check path) — there is no OpID "applied-check"
  path any more. The surviving, always-on behavior is retained: a **normal** size set
  re-upserts the index (file-then-index ordering), so no matching-op_id short-circuit can
  skip the index write. Captured in the SE-3b runtime-verification row.
- **G6** (split SE-3b/SE-3c to keep both ≤2h) — SE-3c is removed, so the split is unwound;
  SE-3b is genuinely ≤2h on its own *(cycle-5 J1: SE-3b is later recast to a test-only unit
  that verifies SE-3a's crash-audit semantics and adds no production code — the
  append-before-write mechanism lives in SE-3a)*.
- **P2 (Security) OpID uniqueness** and **P2 (Security) competing same-`PrevOpID` orphans** —
  both concern OpID/CAS semantics that no longer exist. Closed as MOOT.
- **P3** "re-label `doctor_target.go` under SE-3c" and "keep `size_op_id` in a stable
  frontmatter position" — no SE-3c, no `size_op_id`. Closed as MOOT.

The **G3 process-crash durability narrowing** and the power-loss deferral (stash
`131CEAE4`) still stand and are unaffected by the descope.

**Cycle-3 multi-persona re-review of the FINAL (descoped) plan.** Because Option B2
is strictly scope-reducing, a single-agent addendum would be an honest but weaker
signal than the brief asks for. The four persona lenses were re-run over the final
plan end-to-end:

| Persona | Focus | What it checked on the descoped plan | Verdict |
|---|---|---|---|
| Architecture / cohesion | dependency-graph acyclicity, seam cohesion after SE-3c removal | 12 edges, roots {SE-1 `108.001`, SE-7a `108.009`}, leaves {SE-8 `108.004`, SE-2 `108.005`}, topo-sorts cleanly; SE-3b is a single cohesive unit; no dangling reference to the removed SE-3c *(cycle-5 J1: SE-3b later recast test-only — see the cycle-5 subsection)* | **PASS** |
| Scope-boundary / YAGNI | 2-hour rule, width isolation, deferral tracking | SE-3b now genuinely ≤2h; SE-6 single production file; every task single-domain; the deferred ambition is tracked (stash `9D5BB492`) not silently dropped *(cycle-5 J1: SE-3b is now test-only, still ≤2h)* | **PASS** |
| Standards / constitution | Test-First honesty, buildable-commit, "all width-isolated" claim | SE-2 still expected-green characterization (documented); SE-5 wrapper retirement in one buildable commit (H3); Constitution Check no longer references SE-3c; the two documented bounded deviations (SE-3a file-boundary, SE-5 cross-file migration) remain the only deviations | **PASS** |
| Security / robustness | crash-safety claims, masquerade defense, containment | durability claim narrowed to best-effort process-crash audit with the durable size as sole source of truth (no overclaim); masquerade/actor-stamping (SE-3a) and two-layer containment (SE-7a/SE-7b) unchanged and intact | **PASS** |

**Cycle-3 gate: PASS.** No P0/P1 remains. Option B2 removed the recurring
crash-idempotency review magnet at the root (removed SE-3c, narrowed the invariant,
deleted the OpID surface). 099-S is **11 members**, the graph is **acyclic (12
edges)**, and the width-isolation claim now matches the task set (no cross-domain
task; SE-3c no longer exists). Any wave-4 finding is banked as backlog per the
§1.8 review-fix cycle limit (=3).

### Copilot cycle-5 reconciliation (PR #259 wave-5 — J1, J2)

Wave-5 raised two self-inflicted inconsistencies introduced by the Option-B2
descope, both on this impl-plan. Neither adds capability; both sharpen the plan.

| Finding | Disposition |
|---|---|
| **J1 (P1)** — After the descope, SE-3b (`108.006-T`) restated SE-3a's SAME append-before-write / fail-closed-read protocol on the SAME file (`internal/core/artifact_size.go`), so it had **no distinct production milestone**. | **Recast SE-3b as an explicitly TEST-ONLY crash-audit characterization unit.** SE-3a owns the production append-before-write + fail-closed refuse-on-append-failure. SE-3b's Files are **test-only** (`internal/core/artifact_size_test.go`); its Milestone is "tests green; **no new production code** — verifies SE-3a's crash-audit semantics." Clean, non-overlapping test-ownership line: **SE-3a** keeps the happy-path "event appended before write / forced-append-failure ⇒ no persisted change" tests; **SE-3b** owns the crash-residue tests — (a) fail-closed READ ignores an orphan event, (b) the orphan is never replayed and the shared JSONL is never truncated, (c) a normal set is index-consistent (re-upsert). SE-3b still depends on SE-3a (`108.002-T`); the SE-5→SE-3b edge is retained as release-ordering (land the crash-audit tests before SE-5 retires the wrapper). |
| **J2 (P1)** — Protected Invariant #2 named the positional `SetArtifactSize` as the "only writer" (but SE-5 **retires** it, H3) and said "create rejects/strips them" as a blanket reject (but the migration-safe create, G7, **preserves** a provenanced import). | **Restated the invariant on the typed seam.** `SetArtifactSizeWithProvenance` is the sole writer of the reserved sizing keys; the positional `SetArtifactSize` is a thin compat wrapper delegating to it, **retired by SE-5** (not a second writer). A generic **update** merge-preserves existing reserved keys; a generic **create** carrying an **unprovenanced** reserved size rejects/strips it, but a generic create carrying a **provenanced** size (migration-safe import) **preserves** it and records the event. |

**Consistency sweep (break the spiral).** Beyond the two flagged lines, every
occurrence of the three classes was reconciled across the whole plan: the
Requirements Trace D4 crash-posture row (now "SE-3a impl / SE-3b tests"); the
dependency-graph parallel-build prose and the SE-3b / SE-5 edge bullets; the
Decision "Best-effort audit ordering … implemented in SE-3a and verified by
SE-3b"; the Runtime-Verification SE-3b row (reframed "No (test-only)"); the
Constitution Check rows II/V/VI/VII/VIII and the deviations paragraph; the
rollback triggers (production reverts point at SE-3a; SE-3b tests are the
detector); the "SE-3a durability policy … verified by SE-3b tests" unresolved-
decision heading; the overview sole-writer line (unprovenanced-reject vs
provenanced-preserve); and superseded annotations on the cycle-2/cycle-3
historical rows that described SE-3b as an online-seam production unit.

**Cycle-5 multi-persona re-review of the FINAL (J1/J2-reconciled) plan.** The four
persona lenses were re-run end-to-end over the sweep-reconciled plan:

| Persona | Focus | What it checked | Verdict |
|---|---|---|---|
| Architecture / cohesion | graph acyclicity, test-ownership boundary | 12 edges unchanged (SE-5→SE-3b kept as release-ordering); roots {SE-1 `108.001`, SE-7a `108.009`}, leaves {SE-8 `108.004`, SE-2 `108.005`}; topo-sorts cleanly; SE-3a/SE-3b test-ownership line is clean and non-overlapping | **PASS** |
| Scope-boundary / YAGNI | 2-hour rule, width isolation | SE-3b test-only single-domain (test), still ≤2h; no other task's files/≤2h claim moved; no capability added | **PASS** |
| Standards / constitution | Test-First honesty, buildable-commit | SE-3b is now a documented test-only characterization unit (like SE-2), not a skipped red phase; SE-5 wrapper retirement unchanged; no invariant misnames the retired positional wrapper | **PASS** |
| Security / robustness | crash-safety claims, sole-writer integrity | durability claim still best-effort process-crash audit (durable size sole source of truth); sole-writer invariant now correctly on the typed seam with unprovenanced-reject / provenanced-preserve distinction (no import-loss, no eventless size) | **PASS** |

**Cycle-5 gate: PASS.** No P0/P1 remains. J1/J2 are strictly clarifying /
scope-reducing (SE-3b loses its production scope; wording sharpened). 099-S stays
**11 members** (108-F + 108.001-T…108.010-T); the graph is **acyclic (12 edges)**;
SE-3b remains ≤2h. Any wave-6 finding is banked as backlog per the §1.8
review-fix cycle limit (=3).
