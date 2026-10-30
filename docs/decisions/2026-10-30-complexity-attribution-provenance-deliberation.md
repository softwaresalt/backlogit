---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash A441A74C (Phase 3, feature 207-F / shipment 208-S): stage task-only complexity_source and complexity_ruleset_version provenance, fail-closed estimate_history audit events, SQLite projection and query filters, two-axis route-eligibility predicate IsTaskRouteEligible, and CLI/MCP transport-aware actor stamping in 208-S, while capturing Stage harvest stamping and computed-on-read complexity composition rollup as a linked follow-up stash.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/design-docs/2026-09-10-complexity-attribution-provenance.md
    - docs/design-docs/2026-07-19-size-estimation-contract.md
    - docs/exec-plans/2026-10-30-complexity-attribution-provenance-plan.md
    - .backlogit/queue/207-F.md
    - .backlogit/queue/208-S.md
  promoted_to: plan
  tags:
    - complexity
    - provenance
    - model-routing
    - custom-fields
    - sqlite-projection
    - cli-mcp-parity
    - A441A74C
    - 207-F
    - 208-S
  topic: 'Complexity attribution and provenance for work-attribute model routing (A441A74C / 207-F / 208-S)'
schema_version: "1.0"
source: docs/decisions/2026-10-30-complexity-attribution-provenance-deliberation.md
title: 'Deliberation: Complexity attribution and provenance for work-attribute model routing (A441A74C / 207-F)'
---

## Problem Frame

* Active stash entry `A441A74C` (*Complexity attribution and provenance for work-attribute model routing*, full design in `docs/design-docs/2026-09-10-complexity-attribution-provenance.md`) is the Phase 3 target of the operator's Dark Factory directive (`2026-10-30T05:55:05Z`: *"Phase 1: Consistent CLI and MCP errors; Phase 2: Replan shipments without hand-editing manifests; Phase 3: Trustworthy metadata for cheaper model routing. Run pipeline in Dark Factory mode."*).
* In backlogit today (`internal/core/artifact_complexity.go`, `internal/config/defaults.go`, `.backlogit/header-def.yaml`, `internal/db/upsert_projection.go`, `internal/db/queries.go`, `internal/cli/update.go`, `internal/mcp/tools.go`), `complexity` is stored as a bare, task-only enum (`trivial | low | medium | high`) with no provenance source, no ruleset version, no fail-closed audit event before write, no transport-aware actor stamping, and no two-axis route-eligibility check.
* Autoharness work-attribute model routing (`P-013.7`) requires both `size` and `complexity` axes to be provenance-complete (`size`, `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, `complexity_ruleset_version` all present and valid) before routing a task down to a cheaper model tier; any missing or invalid field must fail safe upward to the invoking agent's role tier.
* While `size` already satisfies this contract (`docs/design-docs/2026-07-19-size-estimation-contract.md`, `internal/core/artifact_size.go`), `complexity` lacks the symmetric provenance pair, audit trail, SQLite projection of provenance columns, bound-parameter provenance filters, route-eligibility predicate, and CLI/MCP transport-aware actor stamping.

## Research Findings

* **Shipped `size` provenance precedent (`internal/core/artifact_size.go`)**:
  * `SetArtifactSizeWithProvenance(ctx, ws, id, SizeMutation)` acquires the per-task advisory lock (`lockArtifactMutation`), resolves the symlink-contained path (`resolveContainedArtifactPath`), decodes the Markdown file via `mdfront.Decode`, validates the mutation against the `task` schema and provenance completeness rules (`validateSizeMutation`), appends an `events.EventEstimateHistory` (`"estimate_history"`) event via `appendItemEventWithActorErr` BEFORE the durable frontmatter write (fail-closed, with injectable test hooks `sizeSeamWriteFailureHook` and `sizeSeamAtomicWrite`), merges only the supplied reserved keys into `custom_fields`, writes atomically via `writeSizeSeamDurable` (retrying `ErrWriteNotApplied` once without re-appending the audit event), and updates SQLite via `db.UpsertItem`.
  * `reservedSizingKeys` in `internal/core/artifact_size.go` (`[]string{"size", "size_source", "size_ruleset_version", "complexity"}`) backs both `rejectReservedSizingKeysOnCreate` (`internal/core/artifacts.go:147`) and `mergePreserveReservedSizingKeys` (`internal/core/artifacts.go:663`). Expanding `reservedSizingKeys` to include `"complexity_source"` and `"complexity_ruleset_version"` enforces sole-writer integrity on generic create and whole-map preservation on generic `custom_fields` updates across all six keys.
* **Current `complexity` seam and header-def upgrade (`internal/core/artifact_complexity.go`, `internal/config/defaults.go`, `internal/config/headerdef.go`)**:
  * `SetArtifactComplexity(ctx, ws, id, complexity)` sets or clears `custom_fields.complexity` (when `complexity == ""`, it calls `clearDecodedCustomField(mdDoc, "complexity")`), enforces `artifactType == "task"` before resolving the header-def schema (`docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`), and writes via `writeComplexitySeamDurable`, but emits no audit event and accepts no provenance fields.
  * `upgradeLegacyGeneratedHeaderDef` (`internal/config/headerdef.go:62-84`) upgrades recognized legacy generated `header-def.yaml` files when `defaultHeaderDef()` in `internal/config/defaults.go` gains new task fields (`TestLoadHeaderDef_LegacyGeneratedDefaultAddsComplexity` in `internal/config/defaults_complexity_test.go`). Updating `defaultHeaderDef()`, `.backlogit/header-def.yaml`, and `upgradeLegacyGeneratedHeaderDef` together ensures both fresh and legacy generated workspaces expose `complexity_source` and `complexity_ruleset_version` while operator-customized task schemas remain untouched.
* **SQLite schema extension, projection, and query filters (`internal/db/schema_gen.go`, `internal/db/upsert_projection.go`, `internal/db/queries.go`)**:
  * `ApplySchemaExtensions` (`internal/db/schema_gen.go`) automatically generates `ALTER TABLE items ADD COLUMN "complexity_source" TEXT` and `ALTER TABLE items ADD COLUMN "complexity_ruleset_version" TEXT` from `HeaderDefConfig.Types["task"].Fields`.
  * `projectedCustomFields` (`internal/db/upsert_projection.go:118-120`) currently guards `if name == "complexity" && artifact.ArtifactType != "task" { continue }`. Extending that task-only guard to `complexity`, `complexity_source`, and `complexity_ruleset_version` prevents non-task artifacts from projecting complexity or complexity provenance into SQLite even under a customized header-def.
  * `db.QueryFilters` and `db.QueryItems` (`internal/db/queries.go:17-29, 369-374`) support bound-parameter filtering for `Complexity` (`artifact_type = 'task' AND complexity = ?`). Adding `ComplexitySource` and `ComplexityRulesetVersion` to `db.QueryFilters` and `db.QueryItems` enables injection-safe bound-parameter queries without breaking the 63 null-complexity and 20 value-only queued tasks.
* **CLI/MCP transport-aware actor stamping and CI drift gates (`internal/cli/update.go`, `internal/mcp/tools.go`, `.autoharness/backlog-registry.yaml`, `docs/cli-reference/`)**:
  * On CLI (`internal/cli/update.go`), `--complexity`, `--complexity-source`, and `--complexity-ruleset-version` must be mutually exclusive with all other field/section flags and `--size*` flags (by adding `"complexity-source"` and `"complexity-ruleset-version"` to `artifactUpdateFlagsChanged` and `conflictingSizeFlags` while keeping them out of `conflictingComplexityFlags` so all three complexity flags can be combined without self-conflict, mirroring how `size-source` and `size-ruleset-version` are excluded from `conflictingSizeFlags`), and default `complexity_source` to `human` when `--complexity-ruleset-version` is provided without `--complexity-source`.
  * On MCP (`internal/mcp/tools.go`), `backlogit_update_item` accepts `complexity`, `complexity_source`, and `complexity_ruleset_version`, rejects explicit `complexity_source: "human"` with a validation error regardless of ruleset, and defaults `complexity_source` to `agent` when `complexity_ruleset_version` is provided without `complexity_source`.
  * Updating `.autoharness/backlog-registry.yaml` (`update_task.params`) and regenerating `docs/cli-reference/backlogit_update.md` via `go run ./cmd/gen-docs docs/cli-reference` keeps `TestRegistryParity_*` and CI's `CLI Reference Drift` check green (`docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`).

## Resolution of the 5 Open Questions from the Design Document

1. **Audit-event shape (Open Question 1)**:
   * **Decision**: Reuse `events.EventEstimateHistory` (`"estimate_history"`) with `"dimension": "complexity"` in `delta` (alongside `"actor": string(m.Actor)` and the mutated `"complexity"`, `"complexity_source"`, and/or `"complexity_ruleset_version"` keys).
   * **Rationale**: Reusing `estimate_history` keeps a single unified estimation audit stream in `.backlogit/logs/<id>.jsonl` and `item_log_entries` while the `"dimension": "complexity"` discriminator cleanly distinguishes complexity mutations from size mutations without affecting any existing `size` readers or tests. Every complexity set or clear appends this event fail-closed BEFORE the durable frontmatter write; if the event append fails, the mutation is refused and the Markdown file remains byte-identical.
2. **Ruleset ownership and naming (Open Question 2)**:
   * **Decision**: `complexity_ruleset_version` is an opaque non-empty string (`type: string, optional: true` in `header-def.yaml`), owned by autoharness/callers and never enum-validated by backlogit.
   * **Rationale**: Backlogit enforces provenance completeness at the complexity seam: whenever `complexity_source` is set, `complexity_ruleset_version` must be present and non-empty after `strings.TrimSpace`; any supplied `complexity_ruleset_version` must be non-empty after trimming whitespace, must not contain control characters (`unicode.IsControl`), and requires an accompanying non-empty `complexity_source` (defaulted by CLI/MCP transport when omitted at the CLI/MCP boundary) and an effective non-empty `complexity` value.
3. **Clear and value-only update provenance-reset semantics (Open Question 3)**:
   * **Decision**:
     * Clearing complexity (`complexity == ""`) atomically clears `complexity`, `complexity_source`, and `complexity_ruleset_version` from `custom_fields` (and nulls all three columns in SQLite via `db.UpsertItem`), and rejects any call that supplies `complexity_source` or `complexity_ruleset_version` alongside `complexity == ""` with `blerrors.ErrValidation`.
     * Updating or re-setting complexity with a non-empty `complexity` value and empty provenance (`m.Source == nil && m.RulesetVersion == nil`, i.e., `source == ""` and `rulesetVersion == ""`) sets `custom_fields["complexity"] = value` AND deletes any pre-existing `custom_fields["complexity_source"]` and `custom_fields["complexity_ruleset_version"]` keys (and nulls both provenance columns in SQLite).
   * **Rationale**: Unlike `size`, `complexity` supports explicit clearing via empty string (`--complexity ""` or MCP `complexity: ""`) as well as legacy value-only writes. Clearing `complexity_source` and `complexity_ruleset_version` on both `complexity == ""` and value-only complexity updates ensures orphan or stale provenance (such as a prior `human`/`ah-v1` attribution after an un-provenanced MCP update from `high` to `trivial`) never survives to make `IsTaskRouteEligible` falsely return `true`.
4. **Route-eligibility predicate (Open Question 4)**:
   * **Decision**: Expose `core.IsTaskRouteEligible(item *models.Artifact) bool` in `internal/core/artifact_complexity.go` (and `db.IsTaskRouteEligible(item *models.Artifact) bool` in `internal/db/queries.go`), returning `true` if and only if `item != nil && item.ArtifactType == "task"` and all six two-axis fields in `item.CustomFields` are present and valid:
     * `size` in `{"XS", "S", "M", "L", "XL"}`
     * `size_source` in `{"human", "agent", "derived"}`
     * `size_ruleset_version` non-empty (after `strings.TrimSpace`) and free of control characters
     * `complexity` in `{"trivial", "low", "medium", "high"}`
     * `complexity_source` in `{"human", "agent", "derived"}`
     * `complexity_ruleset_version` non-empty (after `strings.TrimSpace`) and free of control characters
   * **Rationale**: Directly implements the fail-safe-upward precondition required by autoharness `P-013.7` (§6.1) without coupling backlogit to tier-routing tables. Any unset, partial, legacy value-only, or non-task state returns `false`.
5. **Scope, phasing, and backfill governance (Open Question 5)**:
   * **Decision**: Deliver Phase 1 (header-def, core `ComplexityMutation` & `SetArtifactComplexityWithProvenance`, provenance completeness, fail-closed `estimate_history` audit event before write, generic-create refusal of reserved complexity keys, whole-map preservation on generic updates) + Phase 2 (SQLite projection of `complexity_source` and `complexity_ruleset_version`, bound-parameter query filters `ComplexitySource` and `ComplexityRulesetVersion`, and `IsTaskRouteEligible` predicate) + CLI/MCP transport-aware parity (`--complexity-source`, `--complexity-ruleset-version`, MCP `complexity_source`, `complexity_ruleset_version`, `.autoharness/backlog-registry.yaml` `update_task.params`, and `docs/cli-reference/backlogit_update.md` regeneration) in shipment `208-S` across 3 atomic `code` tasks (`207.001-T`, `207.002-T`, `207.003-T`). Capture Phase 3/4 (Stage harvest complexity-homogeneity assessment and deferred computed-on-read feature/shipment complexity composition rollup) in a linked follow-up stash entry (`kind: "feature"`, `priority: "low"`). Historical backfill remains opt-in through the standard audited complexity seam (no bulk auto-stamp; existing 63 null and 20 value-only queued tasks read and query unchanged).

## Options Evaluated

### Option A: Deliver Phase 1 + Phase 2 + CLI/MCP Transport-Aware Parity in `207-F` / `208-S` (3 Atomic Tasks) and Defer Phase 3/4 to Follow-Up Stash — Chosen

* **Description**:
  1. **U1 (`207.001-T`, `code`)**: Header-def & Core Complexity Provenance Seam (`internal/config/defaults.go`, `internal/config/headerdef.go`, `.backlogit/header-def.yaml`, `internal/core/artifact_complexity.go`, `internal/core/artifact_size.go`, `internal/core/artifact_complexity_test.go`). Add task-only optional `complexity_source` (`enum: [human, agent, derived]`) and `complexity_ruleset_version` (`string`) to default header-def, `.backlogit/header-def.yaml`, and legacy header-def upgrade; implement `ComplexityMutation` and `SetArtifactComplexityWithProvenance` (keeping `SetArtifactComplexity` as a compatibility wrapper); enforce provenance completeness, control-character rejection, 3-field atomic clear when `complexity == ""`, and stale-provenance deletion (`clearDecodedCustomField` on `complexity_source` and `complexity_ruleset_version`) when a non-empty `complexity` value is written without provenance (`m.Source == nil && m.RulesetVersion == nil`); append fail-closed `estimate_history` (`dimension: "complexity"`) event before durable frontmatter write; and add `complexity_source` and `complexity_ruleset_version` to `reservedSizingKeys` for generic-create refusal and whole-map update preservation.
  2. **U2 (`207.002-T`, `code`, blocked by `207.001-T`)**: SQLite Projection, Bound-Parameter Query Filters, and Two-Axis Route-Eligibility Predicate (`internal/db/upsert_projection.go`, `internal/db/queries.go`, `internal/core/artifact_complexity.go`, `internal/db/queries_complexity_test.go`). Enforce task-only projection guard for `complexity`, `complexity_source`, and `complexity_ruleset_version` in `projectedCustomFields`; add bound-parameter filters `ComplexitySource` and `ComplexityRulesetVersion` to `db.QueryFilters` and `db.QueryItems`; and implement `IsTaskRouteEligible(item *models.Artifact) bool` in `internal/db` and `internal/core`.
  3. **U3 (`207.003-T`, `code`, blocked by `207.002-T`)**: CLI & MCP Transport-Aware Parity (`internal/cli/update.go`, `internal/mcp/tools.go`, `.autoharness/backlog-registry.yaml`, `internal/mcp/complexity_metadata_test.go`, `docs/cli-reference/backlogit_update.md`). Add `--complexity-source` and `--complexity-ruleset-version` to `backlogit update` (adding both flags to `artifactUpdateFlagsChanged` and `conflictingSizeFlags` while keeping them out of `conflictingComplexityFlags`, and defaulting `complexity_source` to `human` when `--complexity-ruleset-version` is set without `--complexity-source`); add `complexity_source` and `complexity_ruleset_version` to `backlogit_update_item` (defaulting `complexity_source` to `agent` when `complexity_ruleset_version` is set without `complexity_source`, rejecting explicit `complexity_source: "human"` over MCP, and clearing pre-existing provenance on value-only updates); update `.autoharness/backlog-registry.yaml` (`update_task.params`); and regenerate `docs/cli-reference/backlogit_update.md`.
  4. **Follow-up stash (Phase 3/4)**: Capture Stage harvest complexity-homogeneity assessment and computed-on-read feature/shipment complexity composition rollup in a linked low-priority feature stash entry.
* **Pros**:
  * Completes the entire external route-OUT that autoharness `cost-per-unit-of-work-reduction.md` (§5.2, §6.1, §6.2) requires from backlogit in a single cohesive shipment `208-S`.
  * Obeys the 2-Hour Rule and width isolation across 3 sequential, test-first `code` units (`207.001-T -> 207.002-T -> 207.003-T`).
  * Preserves full backward compatibility with all 63 null-complexity and 20 value-only tasks in the workspace.
* **Cons**:
  * Computed-on-read feature/shipment complexity composition rollup (Phase 4) and Stage harvest homogeneity assessment (Phase 3) are deferred to a follow-up stash entry, which is explicitly aligned with `docs/design-docs/2026-09-10-complexity-attribution-provenance.md` §13 ("only if a consumer materializes").
* **Effort estimate**: Medium (3 atomic tasks, ~6 hours human-equivalent).

### Option B: Bundle Phase 4 Feature/Shipment Complexity Composition Rollup into `208-S`

* **Description**:
  * Include computed-on-read `ComplexityComposition` across `internal/core`, `internal/cli/list.go`, `internal/cli/queue.go`, and `internal/mcp/tools.go` in the same shipment.
* **Pros**:
  * Implements Phase 4 alongside Phases 1 and 2.
* **Cons**:
  * Violates YAGNI and the design document's explicit guidance that feature- and shipment-level complexity labels are "null by contract" in autoharness `execution-epoch` 1.2.0 and that Phase 4 is deferred until a consumer materializes.
  * Expands blast radius across CLI table/tile renderers and MCP `list_items` / `get_shipment` schemas unnecessarily.
* **Effort estimate**: Large (5–6 tasks, >10 hours human-equivalent).

## Chosen Direction and Rationale

Select **Option A**. It delivers 100% of the storage, validation, fail-closed audit durability, SQLite projection, bound-parameter filtering, two-axis route-eligibility predicate (`IsTaskRouteEligible`), and CLI/MCP transport-aware actor stamping required by autoharness `P-013.7` in 3 tightly scoped, test-first units under feature `207-F` and shipment `208-S`, while cleanly deferring Phase 3/4 to a linked follow-up stash entry.
