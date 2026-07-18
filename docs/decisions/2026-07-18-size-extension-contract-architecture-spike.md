---
chunk_strategy: h1-h2-h3
description: "Architecture spike (096-S / 109-F) selecting the durable home for feature/shipment size on .backlogit artifacts. An executable round-trip test proves the generic artifact codec drops a top-level docline map while custom_fields (and the mdfront seam) survive; the spike selects custom_fields.size — the Model-A-delegated option — as the canonical artifact-size location."
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md
title: "Spike: durable size-extension contract for .backlogit artifacts (custom_fields.size selection)"
docline:
    type: spike
    date: 2026-07-18
    time_box: "8h"
    conclusion: "pivot"
    confidence: "high"
    linked_parent_work_item: "109-F"
    promoted_to:
        - "plan"
        - "queue"
    tags:
        - "architecture"
        - "docline"
        - "frontmatter-codec"
        - "size-estimation"
---

## Goal

Does the backlogit **artifact codec** round-trip a nested `docline.backlogit`
subtree on `.backlogit` artifacts, and — given the answer — which of the two
Model-A-sanctioned bridge options (a `Docline` carrier on `models.Artifact`
vs. `custom_fields`) is the canonical, durable location for feature/shipment
`size` and its future provenance?

## Success Criteria

* An **executable** round-trip test settles, empirically, whether a top-level
  `docline` map carrying `docline.backlogit.size` survives the generic artifact
  codec, the generic update/move path, and the `SetArtifactSize` (mdfront) seam.
* The three proceed-gates from 109.004-T are resolved: (1) the 109.002-T
  containment boundary, (2) the canonical size-location, and (3) the
  inheritance-bridge selection that Model A explicitly delegated here.
* A single `proceed | pivot | defer | abandon` recommendation with a
  scoped confidence rating and a concrete, low-risk implementation path.

## Scope Constraints

* **Read-only investigation** — no production code changed. The round-trip
  behavior is codified as a committed **test-tier** regression guard
  (`internal/core/docline_codec_roundtrip_test.go`); tests are not production
  code.
* The **bridge selection** (custom_fields vs. carrier) and the **containment
  boundary** are in-scope decisions for this spike (Model A delegated the former
  to 109.004-T; the size plan's SE3 mandates a durability-policy decision).
* The exact **provenance schema** (`size_source`, `size_ruleset_version`,
  estimate-history) and the size **aggregation ruleset** are inventoried as
  feasibility evidence only; their final design is medium-confidence input to the
  108-F impl-plan, not settled here.

## Investigation Approach

1. Confirm the authoritative Model-A reconciliation (110-F / deliberation
   052-DL) and the two artifact-bridge options it explicitly delegated to this
   spike.
2. Inventory the typed size surface and every generic rewrite path and classify
   each **preserve** or **drop** for an unknown top-level `docline` key and for
   `custom_fields`.
3. Execute a round-trip test proving the drop/preserve behavior on both codec
   routes (the ground truth Model A required).
4. Inventory durability precedents (JSONL policies), the containment boundary,
   CLI/MCP parity, and structured-composition rails a future aggregation reuses.
5. Synthesize the proceed-gate resolutions and a recommendation, then
   adversarially review the decision across independent models.

## Findings

### What Was Discovered

#### 1. Model A delegated this exact selection to this spike

The Model-A decision
(`docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md:114-152`)
ratifies `docline.backlogit.*` as the owner-scoped extension namespace **for
documents**, where the docline codec preserves the whole `docline` map. It
**explicitly records** that `.backlogit` artifacts use a *distinct* codec and
that a top-level `docline` map is **dropped** on a normal artifact update
(`:121-135`), and it names **two** valid artifact-bridge options —
"either add a `Docline` carrier to `models.Artifact` (proven by an executable
round-trip test) **or** route artifact-owned metadata through `custom_fields`"
— stating that "bridge selection is a required, still-open prerequisite and is
deferred to the 109.x size spike (109.004-T)" (`:147-152`). Model A also notes
the docline ext-schema "**cannot** serve as the contract for `.backlogit`
artifact frontmatter, because `allOf` on base v1 would reject artifact fields
such as `id`, `artifact_type`, and `status`" (`:143-146`). This spike executes
the round-trip test and makes the delegated selection.

#### 2. Executable round-trip test — ground truth (committed regression guard)

The ground truth is codified as a committed, test-tier regression guard —
`internal/core/docline_codec_roundtrip_test.go`
(`TestGenericArtifactCodec_DropsTopLevelDocline`,
`TestSetArtifactSize_PreservesTopLevelDocline`) — so a future codec change that
alters this behavior fails a test and forces this decision to be revisited. No
production code was changed. Both tests exercise the two codec routes with an
artifact carrying a top-level `docline: { backlogit: { size: L } }` plus
`custom_fields: { size: M }`.

* **Generic codec route** (`models.ParseFrontmatter` →
  `models.ArtifactFromFrontmatter` → `core.WriteArtifactFile`): the raw parse
  retains `docline`, but after the struct round-trip and re-serialization the
  readback frontmatter contains **no `docline` key**; `custom_fields` survives.
  Observed: `docline survives = false ; custom_fields survives = true`.

* **mdfront seam** (`core.SetArtifactSize`): the nested `docline.backlogit.size:
  L` is preserved and the mutation lands in `custom_fields.size: S`, body bytes
  unchanged. Observed:
  `docline survives = true ; docline.backlogit.size = L ; custom_fields.size = S`.

This empirically confirms Model A's recorded loss point: **the generic artifact
codec drops a top-level `docline` map; `custom_fields` (a recognized carrier)
and the mdfront seam preserve their contents.**

#### 3. Generic rewrite-path preserve/drop classification (109.007-T)

`models.ArtifactFromFrontmatter` maps only the enumerated `Artifact` struct
fields plus `custom_fields` (`internal/models/frontmatter.go:45-123`);
`models.Artifact` has **no** docline/extension/raw-frontmatter carrier
(`internal/models/artifact.go:33-55`). `core.WriteArtifactFile` reconstructs the
frontmatter from struct fields only (`internal/core/artifacts.go:682-725`).

| Path | Unknown top-level `docline` | `custom_fields` | Notes |
|---|---|---|---|
| `models.ParseFrontmatter` (raw) | PRESERVE | PRESERVE | raw `map[string]any` |
| `models.ArtifactFromFrontmatter` | **DROP** | PRESERVE (into struct) | no docline carrier |
| `core.WriteArtifactFile` | **DROP** | PRESERVE (if non-nil) | struct-only rebuild, `:721-723` |
| Generic field update / move (`updateArtifactUngated`→`persistArtifact`→`WriteArtifactFile`) | **DROP** | PRESERVE unless caller passes a replacement `custom_fields` (see below) | CLI/MCP `update`/`move` |
| CLI section-only update | PRESERVE | PRESERVE | reserializes the raw fm map, `internal/cli/update.go:187-242` |
| MCP section writer (`writeSectionsToFile`) | PRESERVE by itself | PRESERVE | but MCP `update_item` DROPs docline because it runs the generic field update first, `internal/mcp/tools.go:758-770,1047-1087` |
| `core.SetArtifactSize` | PRESERVE | PRESERVE (merged) | mdfront decode/encode |

The **dominant** mutation path for ordinary edits (`WriteArtifactFile`, taken by
CLI/MCP `update` and `move`) drops a top-level `docline`. Some paths (CLI
section-only, the raw section writers, and mdfront) preserve it — so "only
mdfront preserves it" is too strong — but no ordinary *field* edit preserves a
top-level `docline`, which is what matters for a size value that must survive
routine updates. CRLF handling also differs: the generic `ParseFrontmatter`
normalizes CRLF→LF across the whole document (`internal/models/frontmatter.go:21-36`);
mdfront normalizes CRLF only inside the frontmatter block and preserves body
bytes exactly, while the frontmatter map is YAML re-marshaled (so key ordering,
comments, and quoting style are **semantically** preserved, not byte-preserved —
`internal/mdfront/codec.go:14-17,51-85`).

#### 4. Typed size surface, absent provenance, and the intentionally event-free seam (109.001-T)

`size` is defined **only** on `task` (enum `XS/S/M/L/XL`, `optional`) under
`custom_fields.size` (`.backlogit/header-def.yaml:51-71`,
`internal/config/defaults.go:53-69`). Feature and shipment types define **no**
`size` field. There is **no** `size_source`, `size_ruleset_version`, or
estimate-history field anywhere in `internal/`, `schemas/`, or `header-def.yaml`
(absence confirmed by grep). `core.SetArtifactSize(ctx, ws, id, size)` takes
**no provenance input**, and its doc comment (`internal/core/artifact_size.go:32-34`)
documents that it **intentionally** emits no mutation *hook* event (a size-only
change is a no-op for the status-change pre-hook). That deliberate omission is
about the lifecycle **hook chain**, not about an audit/provenance **event-stream
append** — a distinction the provenance recommendation below must respect.

#### 5. Durability precedents (109.002-T)

Three distinct, precedented JSONL policies exist — a future size-provenance
append must pick one deliberately:

* **LinkCommit — warn-continue on the durable append**: the
  `INSERT OR REPLACE INTO commit_links` write targets the **disposable** SQLite
  index (fail-surfaced) and is *not* rebuilt on a fresh index rehydration — only
  `items`/`item_deps`/`item_links`/`gate_evidence` are cleared and repopulated
  (`internal/db/rehydration.go:165-171,473`), and no path re-inserts `commit_links`
  from JSONL. The **durable** record is the subsequent best-effort `commit_tracked`
  JSONL history append (which rehydrates into `item_log_entries`), whose failure is
  `slog.Warn`-logged and swallowed (`internal/core/commits.go:27-56`). Warn-continue
  therefore governs the *durable* audit append — the precise analogue for a
  size-provenance append, and the reason this precedent is weaker than it first
  appears.
* **AppendComment — fail-surface**: event append failure is returned to the
  caller (`internal/core/commits.go:72-97`); events carry a zero `Timestamp`,
  stamped `time.Now()` independently by `AppendEvent`
  (`internal/events/stream.go:39-46`) and `db.IndexEvent`
  (`internal/db/logs.go:32-40`).
* **Gate evidence — fail-closed**: evidence is appended **before** the durable
  status write; under `evidence_required`, append failure **refuses** the
  completion (`internal/core/gate_evidence.go:34-58`,
  `internal/core/gate_transition.go:231-279`,
  `internal/core/shipment_gate.go:490-499`).

#### 6. Containment boundary — a realpath gap, not a SafeResolve gap (109.002-T)

`SetArtifactSize` resolves via `FindArtifactPath`/`WalkDir` within
`artifactSearchDirs` (built under `WorkspaceStorageRoot`; registry paths reject
absolute and leading-`..`, `internal/config/loader.go:77-85`) and writes via
`atomicfile.WriteFileAtomic`, without calling `SafeResolve`
(`internal/core/artifact_size.go:35-80`). `SafeResolve` **is** used by
`MoveArtifactFile` and `WriteCommandMap` (`internal/core/routing.go:30-42`,
`internal/core/metadata_catalog.go:308-320`). **However, `SafeResolve` is
lexical only** (`internal/core/workspace.go:271-290`) — it joins and prefix-checks
but never calls `Lstat`/`EvalSymlinks`, so adding it would **not** close a
symlink escape. The repo's real symlink-containment pattern uses `EvalSymlinks`
+ realpath containment (`internal/core/doctor_target.go:256-279`). The residual
risk is nuanced: a **leaf-file** symlink under a search dir can cause an
out-of-workspace *read*; an **intermediate** symlink in a configured search path
can redirect a *write*. `QueueLayout.RootDir` is only `required`, with no
containment validation (`internal/config/schema.go:99-104`). The correct
hardening is realpath/`EvalSymlinks` containment on the size write path, not
`SafeResolve` alone.

#### 7. CLI/MCP parity matrices (109.005-T read, 109.006-T mutation)

**Mutation seam — GOOD**: both CLI `update --size`
(`internal/cli/update.go:91-114,281-293`) and MCP `update_item` size
(`internal/mcp/tools.go:56-72,743-755`) route through the single
`core.SetArtifactSize` seam. Validation is **seam-only**: `validateSizeValue`
is **intentionally not** retrofitted into `ValidateArtifactFields`
(`internal/core/artifact_size.go:66-69,97-122`) — so `custom_fields.size` is
*durable on every currently-wired path* but *validated only on the size seam*, a
durable-everywhere / validated-once asymmetry the impl-plan must carry.

**Read-surface parity (109.005-T)** — the five true command→tool read pairs a
future size projection must populate identically:

| CLI ↔ MCP pair | Request-contract parity | Response-shape parity for `size` |
|---|---|---|
| `queue view` ↔ `get_queue` | CLI flags vs MCP params; both default to the queue ordering (`internal/core/queue.go:127-191`) | size not currently projected on either; a future projection must appear on both |
| `shipment get` ↔ `get_shipment` | single-ID lookup, symmetric | shipment `custom_fields` returned by both |
| `shipment list` ↔ `list_shipments` | list filters symmetric | structured `custom_fields` on MCP; CLI human columns omit size |
| `get` ↔ `get_item` | single-ID; **pinned** `get --format json` ↔ `get_item` | both emit `custom_fields.size` field-for-field in JSON |
| `list` ↔ `list_items` | list filters symmetric | MCP structured `custom_fields`; CLI `--json` exposes `custom_fields` (`internal/cli/list.go:132-135`), CLI human columns omit size (`internal/cli/list.go:20-41`) |

**Pinned `get --format json` ↔ `get_item` context asymmetry — decision-input**:
the two surfaces differ on the *context* fields they attach (`body`,
`dependencies_detail`, `commit_links`). **Decision**: a future size projection
MUST ride on `custom_fields.size` (present identically on both surfaces),
**not** on any surface-specific context block, so read parity for size is
independent of the `body`/`dependencies_detail`/`commit_links` asymmetry. The
only current read gap is cosmetic — the CLI `list`/`shipment list` **human
columns** omit size — and is a non-blocking follow-up.

**Mutation-surface parity (109.006-T)** — current write pair
`CLI update --size` ↔ `MCP update_item size`, error parity by category
(equivalent category + message; transport carriers mapped, not byte-identical):

| Failure | CLI carrier | MCP carrier |
|---|---|---|
| invalid enum value | `ErrValidation` → exit 1 | `validation_failed` |
| unsupported artifact type | validation error → exit 1 | `validation_failed` |
| busy task lock (`ErrTaskBusy`) | `ExitError{Code:4}` (`internal/cli/exit_error.go:5-32`) | `conflict` (`internal/mcp/errors.go:14-98`) |
| not-found | exit 1 | `not_found` |
| workspace-open failure | exit 1 (config/setup) | structured `{error,message}` |

**Future provenance flags/fields — SELECTED (109.006-T evidence, 109.004-T (f)
selection)**: `size_source` (CLI `--size-source`, MCP field `size_source`),
accepted values `{human, agent, derived}`; **defaulting** — an authored size
with no explicit source is stamped from the actor context, an **absent**
`size_source` reads as `unknown`/legacy and is **never rewritten as `human`**
(per 109.003-T); unknown values are **rejected** with the same `validation_failed`
/ exit-1 category as an invalid size. `size_ruleset_version` (CLI
`--size-ruleset-version`, MCP field `size_ruleset_version`) is `null` until a
canonical XS–XL aggregation ruleset is owned; an unknown/unsupported version is
rejected with the same category. Both fields live under `custom_fields`
(durable by the same mechanism as size) and are validated at the size seam, and
both surfaces map the same error category with transport-specific carriers
(CLI Cobra error / `ExitError{Code:4}`; MCP `{error,message}` via
`domainError`/`makeErrorResult`).

#### 8. Ratified structured-composition contract (109.003-T, ratified with 109.004-T)

A future feature/shipment size derived from member task sizes is feasible on
existing rails, and the **structured-composition contract is ratified here**:

* **Response shape**: a computed-on-read, **never-persisted** structure of an
  `XS–XL` **count histogram** + an **`unsized`** count + a **de-duplicated
  canonical members** array. `ruleset_version` is **`null`** until a canonical
  XS–XL aggregation ruleset is owned (none exists in current code).
* **Membership**: feature membership = direct children by `parent_id`
  (`internal/core/hierarchy.go`); shipment membership = manifest expansion via
  `NormalizeShipmentItems` (`internal/core/shipment.go:525-571`).
* **De-duplication**: the `{feature + its child tasks}` ID set is de-duplicated
  (via `uniqueNonEmptyStrings`) so a manifest listing both a feature and its
  child tasks counts each canonical work item **exactly once**.
* **Missing/legacy handling**: a member without an authored size increments
  **`unsized`**, never a default bucket; an absent `size_source` reads as
  `unknown`/legacy and is **never rewritten as `human`**; missing members are
  skipped (`DeriveCoveringFeature` precedent: skip `ErrNotFound`, warn+skip
  others).
* **Ordering**: the XS<S<M<L<XL comparator mirrors the priority `CASE`
  ordered-enum precedent (`internal/core/queue.go:183-191`).

#### 9. Synthesis exit decisions (109.004-T (a)–(g))

The 2h synthesis task records these first-class decisions (it performs no new
inventory; it consumes the evidence above):

* **(a) Durability/ordering policy — SELECTED: fail-surface, event-after-write,
  no rollback.** The size value is the primary durable state (lands first via
  the mdfront seam into `custom_fields.size`); a provenance event append follows
  and, on failure, is **surfaced** to the caller (AppendComment precedent,
  `internal/core/commits.go:72-97`) rather than silently warn-continued
  (LinkCommit, the weakest option for user-visible state). Fail-closed
  (gate-evidence, `internal/core/gate_evidence.go:34-58`) is **rejected** for
  the size mutation itself: size is user-visible primary state, not a completion
  gate, so refusing a valid size write because an audit note could not be
  recorded would harm usability more than the audit gap; fail-closed remains the
  right model only for gate-evidence, not for size provenance. The size write is
  **not rolled back** on a provenance-append failure because it is valid durable
  state on its own.
* **(b) Composition — RATIFIED** per section 8 (jointly with 109.003-T).
* **(c) Parity — CONSUMED** per section 7 (read + mutation matrices).
* **(d) Inheritance bridge — SELECTED: no carrier bridge; store under
  `custom_fields`.** Closes the DROP loss points (`ArtifactFromFrontmatter`
  enumerated-key mapping; `WriteArtifactFile` struct-only re-emit) by using the
  already-preserved `custom_fields` carrier rather than adding a `Docline`
  field to `models.Artifact`. Model A sanctions exactly this option (`:147-152`).
* **(e) Canonical size-location — DECIDED: `custom_fields.size` at all levels**
  (task already there; extend to feature/shipment; coexist, no migration
  needed).
* **(f) Future provenance flags/fields — SELECTED** per section 7
  (`size_source`, `size_ruleset_version`, values, defaulting, rejection,
  transport-aware error mapping).
* **(g) Exit decision — PIVOT (confidence per the Recommendation).** All three
  proceed-gates (containment boundary, canonical size-location, inheritance
  bridge) are resolved, so a later separately-planned, harvested, and
  re-reviewed implementation is authorized.

### What Was Tried and Failed

* The naive expectation — reuse `docline.backlogit.size` (the Model-A *document*
  namespace) for feature/shipment size **on artifacts** — was tested and
  **falsified** for durability: the generic artifact codec drops it, and no
  ordinary field edit preserves it. It would work only with the
  `models.Artifact` carrier bridge or by routing every writer through mdfront.

### Remaining Unknowns

* The durability policy, provenance flags/fields, and composition contract are
  now **decided** (sections 7–9); what remains for the 108-F impl-plan is
  **implementation detail**, not decision: the concrete enum wiring for
  `size_source`, the seam that emits the provenance event, and the aggregation
  computation itself.
* Aggregation **rounding** semantics for a future single-bucket rollup (if one
  is ever wanted in addition to the histogram) — the ratified contract returns a
  histogram + `unsized`, so a single collapsed bucket is an explicit non-goal
  unless a later work item requests it.
* The single-namespace *unification* enhancement (make artifacts also use
  `docline.backlogit.size` via the carrier bridge) is an **optional** future
  enhancement the spike recommends against for now; it is not a gate on the size
  feature.

## Recommendation

**Conclusion**: pivot
**Confidence**: high — for the durability finding, the artifact-size placement
(`custom_fields.size`), the inheritance-bridge selection, and the containment
fix (empirically proven, code-confirmed, and within the spike's
Model-A-delegated authority). medium — for the provenance flag/field selection
and the composition ruleset, which are **decided here** (sections 7–9) but whose
concrete wiring is validated during 108-F implementation.

Store feature/shipment `size` at **`custom_fields.size`** — the option Model A
pre-named and delegated to this spike — reserving top-level `docline.backlogit.*`
for **documents** (its round-trip-durable home). Concretely: extend `size` to
the `feature` and `shipment` header-def types and reuse the existing
`SetArtifactSize` mdfront seam. This resolves all three proceed-gates:

1. **Canonical size-location — DECIDED**: `custom_fields.size` for all artifact
   levels. `custom_fields` is a recognized carrier preserved by every
   currently-wired mutation path; task size already lives there. **Caveat to
   carry into impl-plan**: `updateArtifactUngated` *replaces* the whole
   `custom_fields` map when an update carries a `custom_fields` key
   (`internal/core/artifacts.go:542-544`) — harmless today because no CLI/MCP
   surface passes arbitrary `custom_fields`, but the size seam must remain the
   sole `custom_fields.size` writer (or merge-not-replace must be enforced)
   before any passthrough is added.
2. **Inheritance-bridge selection — DECIDED (no carrier bridge)**: the
   `models.Artifact` docline-carrier bridge is **rejected** for now. Its only
   benefit would be a single uniform namespace across docs and artifacts, but
   (a) the docline ext-schema structurally cannot validate artifact frontmatter
   (`allOf` base-v1 rejects `id`/`artifact_type`/`status`), so no validation is
   forfeited by choosing `custom_fields`; (b) documents carry no `size`, so
   there is no doc/artifact size value to diverge; and (c) engram/graphtor
   ingestion of the closed docline base contract is scoped to **documents** in
   Model A — no evidence indicates they read `.backlogit` artifact size. The
   bridge remains a documented, reversible future option if uniform-namespace
   tooling ever justifies it.
3. **Containment boundary — RESOLVED with the correct fix**: add
   realpath/`EvalSymlinks` containment to the size write path, mirroring
   `internal/core/doctor_target.go:256-279` (not `SafeResolve`, which is lexical
   only and would not close the symlink risk).

**Provenance (SELECTED; see section 9(a),(f))**: `size_source` /
`size_ruleset_version` live under `custom_fields` (durable by the same
mechanism), validated at the size seam. The provenance audit record is emitted
as an **event-stream append** (distinct from the intentionally-omitted lifecycle
hook event at `artifact_size.go:32-34`) under a **fail-surface, event-after-write,
no-rollback** policy: the size write lands first as valid durable state, then the
provenance append is surfaced (not silently swallowed) on failure. Fail-closed is
deliberately **not** chosen for the size mutation itself (size is user-visible
primary state, not a completion gate); warn-continue is the weakest option and is
rejected for user-visible state. The impl-plan implements this selection; it does
not re-decide it.

This selection is consistent with Model A, which scoped `docline.backlogit.*` to
documents and delegated the artifact-bridge choice to this spike.

## Adversarial Review (operator-required)

The decision was reviewed by three independent reviewers on different model
tiers (Claude Opus, GPT-5.6, Gemini 3.1 Pro) before shipping:

* **Consensus (HIGH confidence)**: the core chain — generic codec drops
  top-level `docline`; `custom_fields` and mdfront preserve; therefore
  `custom_fields.size` — is code-confirmed and sound. No P0 blockers.
* **Cross-model disagreement resolved**: one reviewer argued for the carrier
  bridge and a confidence downgrade, citing validation loss and model fracture.
  Those costs were rebutted with Model-A evidence: the ext-schema cannot validate
  artifacts (`:143-146`), documents carry no size, and Model A itself sanctions
  `custom_fields` and **delegates the selection to this spike** (`:147-152`) —
  so deciding `custom_fields` is within ratified authority, not overreach. The
  reviewers' shared, valid finding was that the draft's *framing* was incoherent
  (RESOLVED + high + an "operator-ratification gate" + pre-enqueued work); this
  final version removes the ratification-gate framing and scopes confidence
  explicitly (high on placement, medium on provenance/aggregation).
* **Second review cycle (Copilot, task-completion contracts)**: a follow-up
  review flagged that marking 109.003–006-T and 109.004-T/109-F done required the
  artifact to *record* the concrete parity matrices, ratified composition
  contract, durability-policy selection, and provenance flag/field selection that
  those tasks' completion contracts mandate — rather than defer them. Resolved by
  recording all of them here (sections 7–9); no task was left with unmet exit
  criteria, so no task was reopened.

## Next Steps

1. Promote to `impl-plan` (108-F size feature): update
   `docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md` to
   **implement** the decisions recorded here — `custom_fields.size` for
   feature/shipment, the realpath containment hardening, provenance under
   `custom_fields` with the selected fail-surface event policy (section 9(a)),
   and the `size_source`/`size_ruleset_version` flags/fields (section 7); carry
   the map-replacement and validated-once caveats as constraints.
2. Plan the size-aggregation ruleset as a separate work item, reusing the
   membership/dedup/missing-handling rails and the priority `CASE` ordered-enum
   comparator precedent.
3. Optionally close the CLI `list` human-column read-parity gap.

## References

* Authoritative reconciliation: `docs/decisions/2026-07-16-docline-backlogit-owner-profile-model-a-decision.md:114-152` (two artifact-bridge options; selection delegated to 109.004-T; ext-schema inapplicable to artifacts).
* Deliberation `052-DL`; feature `110-F` (archived, done, commit `b9bae62`).
* Existing plan: `docs/exec-plans/2026-07-14-size-estimation-feature-shipment-plan.md`.
* Codec: `internal/models/frontmatter.go:21-123`, `internal/models/artifact.go:33-55`, `internal/core/artifacts.go:466-570,646-735`.
* Size seam: `internal/core/artifact_size.go:16-134`; header-def `.backlogit/header-def.yaml:51-71`; defaults `internal/config/defaults.go:53-69`.
* Body/semantic-preserving codec: `internal/mdfront/codec.go:10-85`.
* Durability precedents: `internal/core/commits.go:27-97`, `internal/core/gate_evidence.go:34-58`, `internal/core/gate_transition.go:231-279`, `internal/core/shipment_gate.go:490-499`; event stamping `internal/events/stream.go:39-46`, `internal/db/logs.go:32-40`.
* Containment: `internal/core/workspace.go:271-290` (lexical SafeResolve), `internal/core/doctor_target.go:256-279` (realpath pattern), `internal/core/routing.go:30-42`, `internal/core/artifacts.go:597-678`, `internal/config/loader.go:77-85`, `internal/config/schema.go:99-104`, `internal/atomicfile/atomicfile.go`.
* Parity: `internal/cli/update.go:91-114,187-242,281-293`, `internal/cli/exit_error.go:5-32`, `internal/cli/list.go:20-41,132-135`, `internal/mcp/tools.go:56-72,743-755,1047-1087`, `internal/mcp/errors.go:14-98`.
* Composition rails: `internal/core/hierarchy.go`, `internal/core/shipment.go:466-571`, `internal/core/shipment_covering.go:61-77`, `internal/core/queue.go:127-191`.
* Ext schema: `schemas/docline/ext/backlogit-v1.schema.json:5-30`, `schemas/docline/base-frontmatter-v1.schema.json:5-40`.
* Ground-truth round-trip test (committed regression guard): `internal/core/docline_codec_roundtrip_test.go` (`TestGenericArtifactCodec_DropsTopLevelDocline`, `TestSetArtifactSize_PreservesTopLevelDocline`).
