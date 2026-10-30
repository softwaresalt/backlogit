---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for feature 207-F (shipment 208-S, Phase 3, stash A441A74C): add task-only complexity_source and complexity_ruleset_version provenance, fail-closed estimate_history audit events, SQLite projection and bound-parameter filters, two-axis route-eligibility predicate IsTaskRouteEligible, and CLI/MCP transport-aware actor stamping.'
doc_type: plan
docline:
  feature_id: 207-F
  shipment_id: 208-S
  source_decision: docs/decisions/2026-10-30-complexity-attribution-provenance-deliberation.md
  stash_id: A441A74C
schema_version: "1.0"
source: docs/exec-plans/2026-10-30-complexity-attribution-provenance-plan.md
title: 'Implementation Plan: Complexity attribution and provenance for work-attribute model routing (207-F / 208-S)'
---

## Objective

Deliver trustworthy task-level complexity attribution and provenance for work-attribute model routing (`A441A74C`, `docs/design-docs/2026-09-10-complexity-attribution-provenance.md`) under feature `207-F` and shipment `208-S`:

1. **Header-Def & Core Complexity Provenance Seam (`U1` / `207.001-T`)**: Add task-only optional `complexity_source` (`enum: [human, agent, derived]`) and `complexity_ruleset_version` (`string`) to default header-def (`internal/config/defaults.go`), `.backlogit/header-def.yaml`, and legacy header-def upgrade (`internal/config/headerdef.go`); implement `ComplexityMutation` and `SetArtifactComplexityWithProvenance` in `internal/core/artifact_complexity.go` (keeping `SetArtifactComplexity` as a compatibility wrapper); enforce source-requires-ruleset and ruleset-requires-source/value completeness plus control-character rejection; atomically clear `complexity`, `complexity_source`, and `complexity_ruleset_version` when `complexity == ""` (while rejecting provenance fields when clearing); append a fail-closed `estimate_history` (`dimension: "complexity"`) audit event before the durable frontmatter write; and include `complexity_source` and `complexity_ruleset_version` in `reservedSizingKeys` (`internal/core/artifact_size.go`) so generic `CreateArtifact` refuses all three complexity keys and generic `UpdateArtifact` preserves all three keys across `custom_fields` updates.
2. **SQLite Projection, Bound-Parameter Query Filters, and Two-Axis Route-Eligibility Predicate (`U2` / `207.002-T`)**: Project `complexity_source` and `complexity_ruleset_version` into the SQLite `items` table for `task` artifacts only (`internal/db/upsert_projection.go`, `internal/db/schema_gen.go`), add bound-parameter filter support (`ComplexitySource`, `ComplexityRulesetVersion`) in `db.QueryFilters` and `db.QueryItems` (`internal/db/queries.go`) preserving all existing null-complexity and value-only tasks without error, and implement `IsTaskRouteEligible(item *models.Artifact) bool` (`internal/db/queries.go` and `internal/core/artifact_complexity.go`) requiring all six size + complexity provenance fields (`size`, `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, `complexity_ruleset_version`) to be present and valid.
3. **CLI & MCP Transport-Aware Parity (`U3` / `207.003-T`)**: Add `--complexity-source` and `--complexity-ruleset-version` flags to `backlogit update` (`internal/cli/update.go`, defaulting `complexity_source` to `human` when `--complexity-ruleset-version` is provided without `--complexity-source`), add `complexity_source` and `complexity_ruleset_version` parameters to `backlogit_update_item` (`internal/mcp/tools.go`, defaulting `complexity_source` to `agent` when `complexity_ruleset_version` is provided without `complexity_source`, and rejecting explicit `complexity_source: "human"` over MCP regardless of ruleset), update `.autoharness/backlog-registry.yaml` (`update_task.params`), and regenerate `docs/cli-reference/backlogit_update.md` via `go run ./cmd/gen-docs docs/cli-reference`.

Source decision: `docs/decisions/2026-10-30-complexity-attribution-provenance-deliberation.md` (Option A, stash `A441A74C`).

## Activation Contract & Routing Context

* `DARK_MODE_ACTIVE`: `true`
* `activation_utc`: `2026-10-30T05:55:05Z`
* `operator_request_verbatim`: `'Phase 1: Consistent CLI and MCP errors; Phase 2: Replan shipments without hand-editing manifests; Phase 3: Trustworthy metadata for cheaper model routing. Run pipeline in Dark Factory mode.'`
* `bounded_ordered_capability_scope`: `["Phase1 existing feature 166-F (completed in 206-S)", "Phase2 active stash DE3E7B67 (completed in 207-S)", "Phase3 active stash A441A74C (207-F / 208-S)"]`
* `cursor`: `last_completed=207-S (Phase 2 — 206-F), next=Phase 3 (stash A441A74C -> 207-F / 208-S)`
* `merge_approval_pre_authorized`: `true` (within this exact scope after current-HEAD local readiness + CI + review gates; merge commits only per P-009)
* `admin_fallback_pre_authorized`: `false`
* `routing_note`: `TOOL_OK: routing-resolver-cli` (`gemini-4-argon/google/high/long_context`).

## Problem Frame

Autoharness work-attribute model routing (`P-013.7`) routes tasks across model capability tiers using `complexity` (reasoning requirement) and `size` (volume/budget), subject to a strict fail-safe-upward precondition: both axes must be provenance-complete (`size`, `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, `complexity_ruleset_version` all present and valid). Backlogit already implements the full provenance, audit, and transport-boundary contract for `size` (`internal/core/artifact_size.go`), but `complexity` (`internal/core/artifact_complexity.go`) stores only the bare enum value with no provenance fields, no fail-closed audit event, no SQLite provenance projection or filters, no route-eligibility helper, and no CLI/MCP provenance flags.

## Requirements Trace

| ID | Requirement | Source | Implementation Unit |
|---|---|---|---|
| R1 | Declare task-only optional `complexity_source` (`enum: [human, agent, derived]`) and `complexity_ruleset_version` (`string`) in `defaultHeaderDef()` (`internal/config/defaults.go`), `.backlogit/header-def.yaml`, and `upgradeLegacyGeneratedHeaderDef` (`internal/config/headerdef.go`); implement `ComplexityMutation` and `SetArtifactComplexityWithProvenance` in `internal/core/artifact_complexity.go` (keeping `SetArtifactComplexity` compatible); enforce task-only guard before schema lookup, source-requires-ruleset and ruleset-requires-source/value completeness, control-character rejection on ruleset version, atomic 3-field clear when `complexity == ""` (rejecting provenance when clearing), fail-closed `estimate_history` (`dimension: "complexity"`) audit event before durable frontmatter write, and `reservedSizingKeys` generic-create refusal + generic-update preservation for `complexity`, `complexity_source`, and `complexity_ruleset_version` | `A441A74C` & `2026-10-30-complexity-attribution-provenance-deliberation.md` (Q1, Q2, Q3, Q5) | U1 (`207.001-T`) |
| R2 | Project `complexity_source` and `complexity_ruleset_version` into SQLite `items` for `task` artifacts only (`internal/db/upsert_projection.go` + `ApplySchemaExtensions` in `internal/db/schema_gen.go`), add bound-parameter filters `ComplexitySource` and `ComplexityRulesetVersion` to `db.QueryFilters` and `db.QueryItems` (`internal/db/queries.go`), preserve existing null and value-only tasks without error, and expose `IsTaskRouteEligible(item *models.Artifact) bool` in `internal/db/queries.go` and `internal/core/artifact_complexity.go` | `A441A74C` & `2026-10-30-complexity-attribution-provenance-deliberation.md` (Q4, Q5) | U2 (`207.002-T`) |
| R3 | Expose `--complexity-source` and `--complexity-ruleset-version` on `backlogit update` (`internal/cli/update.go`, defaulting `complexity_source` to `human` when `--complexity-ruleset-version` is set without `--complexity-source`) and `complexity_source` / `complexity_ruleset_version` on `backlogit_update_item` (`internal/mcp/tools.go`, defaulting `complexity_source` to `agent` when `complexity_ruleset_version` is set without `complexity_source`, and rejecting explicit `complexity_source: "human"` over MCP), update `.autoharness/backlog-registry.yaml` (`update_task.params`), and regenerate `docs/cli-reference/backlogit_update.md` | `A441A74C` & `2026-10-30-complexity-attribution-provenance-deliberation.md` (Q1, Q5) | U3 (`207.003-T`) |

## Constraints

* **2-Hour Rule and Width Isolation**: Every unit is restricted to the `code` domain, touches 3–4 tightly coupled files within a single concern, and keeps `go test ./...` green at the end of every wave.
* **Task-only guard before schema resolution**: Per `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`, both `validateComplexityMutation` (`internal/core/artifact_complexity.go`) and `projectedCustomFields` (`internal/db/upsert_projection.go`) MUST enforce `artifactType == "task"` for `complexity`, `complexity_source`, and `complexity_ruleset_version` before schema resolution or SQLite column projection.
* **Fail-closed audit event before write**: Every complexity mutation (`SetArtifactComplexity` and `SetArtifactComplexityWithProvenance`, including clears) MUST append an `events.EventEstimateHistory` (`"estimate_history"`) event with `"dimension": "complexity"` and `"actor": string(m.Actor)` before the durable frontmatter write. If the event append fails, the mutation MUST fail closed without modifying the Markdown file or SQLite row.
* **Transport trust boundary**: Over MCP (`backlogit_update_item`), an explicit `complexity_source: "human"` MUST be rejected with a validation error (`ValidationFailed("complexity_source: human cannot be set from the MCP (agent) transport")`) regardless of whether `complexity_ruleset_version` is provided.
* **Genuine tests-first RED -> GREEN**: Every unit specifies the exact failing test selector (`^TestU1_`, `^TestU2_`, `^TestU3_`) that compiles cleanly and is observed RED before the production implementation turns it GREEN. No `harness-exempt` tasks exist in this plan (`closed_exempt_set: []`).

## Implementation Units

### U1: Header-def & Core Complexity Provenance Seam (`code`) — `207.001-T`

* **Domain**: `code`
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/core/artifact_complexity.go`, declare exported struct `ComplexityMutation` (`Complexity *string`, `Source *string`, `RulesetVersion *string`, `Actor ActorContext`) and stub `SetArtifactComplexityWithProvenance(ctx context.Context, ws *Workspace, id string, m ComplexityMutation) (*models.Artifact, error)` returning `nil, ErrSizeEstimationNotImplemented` (or `blerrors.ErrValidation` stub) while keeping `SetArtifactComplexity` compiling.
    2. In `internal/core/artifact_complexity_test.go`, add `TestU1_ComplexityProvenanceSeam`:
       * `header_def_declares_complexity_provenance_task_only`: `config.WriteDefaults` + `config.LoadHeaderDef` exposes optional `complexity_source` (`enum: [human, agent, derived]`) and `complexity_ruleset_version` (`string`) on `task` only (absent on `feature` and `shipment`), and `LoadHeaderDef` upgrades legacy generated `header-def.yaml` files lacking `complexity_source` / `complexity_ruleset_version`.
       * `persists_provenance_and_appends_fail_closed_audit_event_before_write`: `SetArtifactComplexityWithProvenance` with `Complexity: ptr("high"), Source: ptr("agent"), RulesetVersion: ptr("ah-complexity-v1"), Actor: ActorContextAgent` preserves body bytes, writes `complexity`, `complexity_source`, and `complexity_ruleset_version` to `custom_fields`, and appends an `estimate_history` event with `actor == "agent"` and `delta.dimension == "complexity"`; when the item log path is obstructed (e.g. a directory at `.backlogit/logs/<id>.jsonl`) so event append fails, `SetArtifactComplexityWithProvenance` returns an error and leaves the task Markdown file byte-identical.
       * `enforces_provenance_completeness_and_control_char_rejection`: rejects `Source` without `RulesetVersion` (`blerrors.ErrValidation`), rejects `RulesetVersion` without `Source` on an un-sourced task (`blerrors.ErrValidation`), rejects `Source` + `RulesetVersion` when effective `complexity` is empty (`blerrors.ErrValidation`), and rejects `RulesetVersion` containing whitespace-only or control characters (`\n`, `\x00`, `\x1f`), leaving the file byte-identical in every rejected case.
       * `clear_complexity_atomically_clears_all_three_fields_and_rejects_provenance_on_clear`: setting `Complexity: ptr("")` on a task that has `complexity`, `complexity_source`, and `complexity_ruleset_version` removes all 3 keys from `custom_fields` and appends an `estimate_history` (`dimension: "complexity"`, `complexity: ""`) event; supplying `Complexity: ptr("")` together with `Source` or `RulesetVersion` is rejected with `blerrors.ErrValidation` and leaves the file byte-identical.
       * `generic_create_refuses_and_generic_update_preserves_all_three_complexity_keys`: `CreateArtifact` with `WithFields` carrying `complexity`, `complexity_source`, or `complexity_ruleset_version` is refused with `blerrors.ErrValidation`; `UpdateArtifact` with a `custom_fields` map preserves existing `complexity`, `complexity_source`, and `complexity_ruleset_version` and drops any off-seam incoming overrides.
    3. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU1_ComplexityProvenanceSeam$' ./internal/core` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `.backlogit/header-def.yaml` and `internal/config/defaults.go` (`defaultHeaderDef`), add `complexity_source` (`type: enum, values: [human, agent, derived], optional: true`) and `complexity_ruleset_version` (`type: string, optional: true`) under `types.task.fields`, and update `upgradeLegacyGeneratedHeaderDef` in `internal/config/headerdef.go` so pre-207-F generated default `header-def.yaml` variants (missing `complexity_source` and `complexity_ruleset_version`, with or without `complexity` and `shipment.status: blocked`) upgrade cleanly to include `complexity`, `complexity_source`, and `complexity_ruleset_version`.
    2. In `internal/core/artifact_size.go`, extend `reservedSizingKeys` to `[]string{"size", "size_source", "size_ruleset_version", "complexity", "complexity_source", "complexity_ruleset_version"}`.
    3. In `internal/core/artifact_complexity.go`, implement `SetArtifactComplexityWithProvenance` (and delegate `SetArtifactComplexity(ctx, ws, id, complexity)` to `SetArtifactComplexityWithProvenance(ctx, ws, id, ComplexityMutation{Complexity: &complexity, Actor: ActorContextHuman})`):
       * Validate `artifactType == "task"` before resolving `ws.HeaderDef` schema.
       * When `m.Complexity != nil && *m.Complexity == ""`: reject if `m.Source != nil || m.RulesetVersion != nil` (`blerrors.ErrValidation`); validate `complexity` exists in task schema; append `events.EventEstimateHistory` (`dimension: "complexity"`, `actor: string(m.Actor)`, `complexity: ""`) fail-closed before write; clear `complexity`, `complexity_source`, and `complexity_ruleset_version` from `mdDoc`; encode, write durably via `writeComplexitySeamDurable`, and `db.UpsertItem`.
       * Otherwise: validate `m.Complexity` (if non-nil) against task `complexity` enum; validate `m.Source` (if non-nil) in `{human, agent, derived}` and require `m.RulesetVersion != nil && strings.TrimSpace(*m.RulesetVersion) != ""`; validate `m.RulesetVersion` (if non-nil) is non-empty after `strings.TrimSpace` and contains no control characters (`unicode.IsControl`), and require effective source (`*m.Source` or `priorSource`) and effective complexity (`*m.Complexity` or `priorComplexity`) to be non-empty; append `events.EventEstimateHistory` (`dimension: "complexity"`, `actor: string(m.Actor)`, plus supplied fields) fail-closed before write; set supplied fields on `mdDoc`; encode, write durably, and `db.UpsertItem`.
* **Affected files**:
  * `.backlogit/header-def.yaml`
  * `internal/config/defaults.go`
  * `internal/config/headerdef.go`
  * `internal/core/artifact_complexity.go`
  * `internal/core/artifact_size.go`
  * `internal/core/artifact_complexity_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU1_ComplexityProvenanceSeam$' ./internal/core`.
  * GREEN phase passes `go test -count=1 -race ./internal/config/... ./internal/core/...` with all existing size and complexity tests green.
* **Execution posture**: test-first.

### U2: SQLite Projection, Bound-Parameter Query Filters, and Two-Axis Route-Eligibility Predicate (`code`) — `207.002-T`

* **Domain**: `code`
* **Depends on**: U1 (`207.001-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/db/queries.go`, add `ComplexitySource string` and `ComplexityRulesetVersion string` fields to `QueryFilters` and declare stub `IsTaskRouteEligible(item *models.Artifact) bool` returning `false` (and `core.IsTaskRouteEligible(item *models.Artifact) bool` in `internal/core/artifact_complexity.go` delegating to `db.IsTaskRouteEligible`).
    2. In `internal/db/queries_complexity_test.go`, add `TestU2_ComplexityProvenanceProjectionAndRouteEligibility`:
       * `projects_complexity_provenance_for_tasks_only_and_nulls_on_clear`: `db.UpsertItem` and `db.Rehydrate` project `complexity_source` and `complexity_ruleset_version` into SQLite `items` columns for `task` artifacts, refuse to project `complexity_source` or `complexity_ruleset_version` for non-task artifacts (e.g. `feature`), and null all three columns (`complexity`, `complexity_source`, `complexity_ruleset_version`) when cleared.
       * `bound_parameter_filters_and_legacy_null_and_value_only_compatibility`: `db.QueryItems` filters by `ComplexitySource` and `ComplexityRulesetVersion` using bound SQL parameters (rejecting SQL injection payloads like `"agent' OR '1'='1"` by returning 0 rows), while un-filtered or value-filtered queries over null-complexity tasks and value-only (no-provenance) tasks succeed without error.
       * `two_axis_route_eligibility_predicate`: `db.IsTaskRouteEligible` and `core.IsTaskRouteEligible` return `true` if and only if the artifact is a `task` and all 6 fields (`size` in `XS|S|M|L|XL`, `size_source` in `human|agent|derived`, non-empty control-char-free `size_ruleset_version`, `complexity` in `trivial|low|medium|high`, `complexity_source` in `human|agent|derived`, non-empty control-char-free `complexity_ruleset_version`) are present and valid; returns `false` for nil items, non-task artifacts, tasks missing any of the 6 fields, or tasks with any invalid enum or whitespace/control-char ruleset version.
    3. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU2_ComplexityProvenanceProjectionAndRouteEligibility$' ./internal/db` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/db/upsert_projection.go` (`projectedCustomFields`), extend the task-only guard so `"complexity"`, `"complexity_source"`, and `"complexity_ruleset_version"` are skipped whenever `artifact.ArtifactType != "task"`.
    2. In `internal/db/queries.go` (`QueryItems`), add bound-parameter filter conditions for `filters.ComplexitySource != ""` (`artifact_type = 'task' AND complexity_source = ?`) and `filters.ComplexityRulesetVersion != ""` (`artifact_type = 'task' AND complexity_ruleset_version = ?`).
    3. In `internal/db/queries.go` and `internal/core/artifact_complexity.go`, implement `IsTaskRouteEligible(item *models.Artifact) bool` verifying `item != nil && item.ArtifactType == "task" && item.CustomFields != nil` and validating all 6 size + complexity provenance fields against their allowed sets (`XS|S|M|L|XL`, `human|agent|derived`, non-empty control-char-free ruleset, `trivial|low|medium|high`, `human|agent|derived`, non-empty control-char-free ruleset).
* **Affected files**:
  * `internal/db/upsert_projection.go`
  * `internal/db/queries.go`
  * `internal/core/artifact_complexity.go`
  * `internal/db/queries_complexity_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU2_ComplexityProvenanceProjectionAndRouteEligibility$' ./internal/db`.
  * GREEN phase passes `go test -count=1 -race ./internal/db/... ./internal/core/...`.
* **Execution posture**: test-first.

### U3: CLI & MCP Transport-Aware Parity (`code`) — `207.003-T`

* **Domain**: `code`
* **Depends on**: U2 (`207.002-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/mcp/complexity_metadata_test.go`, add `TestU3_ComplexityProvenanceMCPAndCLIParity`:
       * Over MCP (`backlogit_update_item`), setting `complexity: "high"` with `complexity_ruleset_version: "ah-v1"` (and omitted `complexity_source`) defaults `complexity_source` to `"agent"`, persists all 3 fields in Markdown and SQLite, and records `actor == "agent"` in `estimate_history`; setting `complexity_source: "derived"` with `complexity_ruleset_version: "ah-v1"` persists `complexity_source == "derived"`; setting `complexity_source: "human"` over MCP is rejected with `validation_failed` regardless of whether `complexity_ruleset_version` is supplied; setting `complexity_source: "agent"` without `complexity_ruleset_version` is rejected with `validation_failed`; clearing with `complexity: ""` clears `complexity`, `complexity_source`, and `complexity_ruleset_version` in Markdown and SQLite; and combining `complexity_source` or `complexity_ruleset_version` with `title`, `sections`, or `size` is rejected as mutually exclusive.
       * Over CLI (`backlogit update`), add a test in `internal/cli/update_test.go` (or `internal/mcp/complexity_metadata_test.go` / `internal/cli/registry_parity_test.go`) verifying `backlogit update <id> --complexity medium --complexity-ruleset-version ah-v1` defaults `complexity_source` to `"human"`, `--complexity-source derived --complexity-ruleset-version ah-v1` stamps `"derived"`, `--complexity-source human` without `--complexity-ruleset-version` fails validation, `--complexity ""` clears all 3 fields, and `--complexity-source` / `--complexity-ruleset-version` are mutually exclusive with `--title`, `--size`, and `--section`.
    2. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU3_ComplexityProvenanceMCPAndCLIParity$' ./internal/mcp` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/cli/update.go`:
       * Add `--complexity-source` (`"complexity provenance source (human, agent, derived)"`) and `--complexity-ruleset-version` (`"complexity ruleset version"`) flags to `newUpdateCommand`.
       * Add `complexityMutationFlagsChanged(cmd)` checking `cmd.Flags().Changed("complexity") || cmd.Flags().Changed("complexity-source") || cmd.Flags().Changed("complexity-ruleset-version")`, update `artifactUpdateFlagsChanged`, `conflictingSizeFlags`, and `conflictingComplexityFlags` to include `complexity-source` and `complexity-ruleset-version`, and route complexity mutations through `core.SetArtifactComplexityWithProvenance` with `Actor: core.ActorContextHuman` (defaulting `mutation.Source` to `"human"` when `--complexity-ruleset-version` is set without `--complexity-source` and complexity is not being cleared).
    2. In `internal/mcp/tools.go`:
       * Register `complexity_source` and `complexity_ruleset_version` string parameters on `backlogit_update_item` (and update the `complexity` parameter description to reflect optional provenance and audit history).
       * Update `hasComplexityMutationArguments` to check for presence of `"complexity"`, `"complexity_source"`, or `"complexity_ruleset_version"`.
       * In `handleUpdateItem`, validate that any present key among `complexity`, `complexity_source`, and `complexity_ruleset_version` is a string; reject `complexity_source == "human"` with `ValidationFailed("complexity_source: human cannot be set from the MCP (agent) transport")`; default `mutation.Source` to `"agent"` when `complexity_ruleset_version` is provided without `complexity_source` (and complexity is not being cleared); and call `core.SetArtifactComplexityWithProvenance(ctx, s.Workspace, id, mutation)` with `Actor: core.ActorContextAgent`.
    3. In `.autoharness/backlog-registry.yaml`, update `update_task.params` to include `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, and `complexity_ruleset_version`.
    4. Run `go run ./cmd/gen-docs docs/cli-reference` to regenerate `docs/cli-reference/backlogit_update.md`.
* **Affected files**:
  * `internal/cli/update.go`
  * `internal/mcp/tools.go`
  * `internal/mcp/complexity_metadata_test.go`
  * `.autoharness/backlog-registry.yaml`
  * `docs/cli-reference/backlogit_update.md`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU3_ComplexityProvenanceMCPAndCLIParity$' ./internal/mcp`.
  * GREEN phase passes `go test -count=1 -race ./internal/cli/... ./internal/mcp/...`.
  * `go run ./cmd/gen-docs docs/cli-reference` followed by `git diff --exit-code docs/cli-reference/` exits 0, and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Execution posture**: test-first.

## Dependency Graph

* **Shipment sequencing**: `207-S` (shipped) -> `208-S` (`208-S` carries an explicit `blocks` dependency on `207-S`).
* **Wave 1**: `207.001-T` (U1 — Header-def & Core Complexity Provenance Seam + `TestU1_ComplexityProvenanceSeam`)
* **Wave 2**: `207.002-T` (U2 — blocked by `207.001-T`; SQLite Projection, Bound-Parameter Query Filters, and `IsTaskRouteEligible` + `TestU2_ComplexityProvenanceProjectionAndRouteEligibility`)
* **Wave 3**: `207.003-T` (U3 — blocked by `207.002-T`; CLI & MCP Transport-Aware Parity, `.autoharness/backlog-registry.yaml`, `docs/cli-reference/backlogit_update.md` + `TestU3_ComplexityProvenanceMCPAndCLIParity`)

No cycles exist in the task or shipment dependency graph (`207.001-T -> 207.002-T -> 207.003-T`).

## Decisions and Rationale

* **Unified `estimate_history` event with `dimension: "complexity"`**: Reusing `events.EventEstimateHistory` (`"estimate_history"`) with `"dimension": "complexity"` in `delta` keeps all sizing and complexity estimation audit records in a single event family while preserving full compatibility with existing `size` readers and tests.
* **Atomic 3-field clear on `complexity == ""`**: Clearing a task's complexity removes `complexity`, `complexity_source`, and `complexity_ruleset_version` together and rejects any simultaneous provenance arguments, preventing orphan provenance on cleared tasks.
* **Two-axis `IsTaskRouteEligible` predicate**: Implementing `IsTaskRouteEligible(item *models.Artifact) bool` in `internal/db` and `internal/core` gives autoharness `P-013.7` a deterministic single-call check across all 6 required fields (`size`, `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, `complexity_ruleset_version`) that fails safe upward (`false`) on any missing or malformed field.

## Risks and Mitigations

* **Risk 1 (Legacy generated `header-def.yaml` compatibility in `LoadHeaderDef`)**: Adding `complexity_source` and `complexity_ruleset_version` to `defaultHeaderDef()` without updating `upgradeLegacyGeneratedHeaderDef` in `internal/config/headerdef.go` would break `TestLoadHeaderDef_LegacyGeneratedDefaultAddsComplexity` or leave pre-207-F workspaces without the new task fields.
  * **Mitigation**: Update `upgradeLegacyGeneratedHeaderDef` in U1 to recognize pre-207-F generated default `header-def.yaml` variants (missing `complexity_source`/`complexity_ruleset_version` and/or `complexity` and/or `shipment.status: blocked`) and upgrade them to include `complexity`, `complexity_source`, and `complexity_ruleset_version`, while preserving operator-customized schemas.
* **Risk 2 (Non-task projection leakage or SQL injection on provenance filters)**: A customized `header-def.yaml` or unguarded custom field map on a non-task artifact could project `complexity_source` or `complexity_ruleset_version` into SQLite, or string-interpolated filters could allow SQL injection.
  * **Mitigation**: Guard `complexity`, `complexity_source`, and `complexity_ruleset_version` with `artifact.ArtifactType == "task"` in both `validateComplexityMutation` and `projectedCustomFields`, and bind `ComplexitySource` and `ComplexityRulesetVersion` as `?` parameters in `db.QueryItems`.
* **Risk 3 (CLI Reference Drift in CI)**: Adding `--complexity-source` and `--complexity-ruleset-version` to `internal/cli/update.go` without regenerating `docs/cli-reference/` would fail CI's `CLI Reference Drift` job.
  * **Mitigation**: Regenerate `docs/cli-reference/backlogit_update.md` via `go run ./cmd/gen-docs docs/cli-reference` in U3 and verify `git diff --exit-code docs/cli-reference/` exits 0.

## Plan Hardening

* `Requires plan hardening`: `yes`
* `Constitution Check`: `pass`

### System-Wide Invariants

* **I1 (Task-only complexity & provenance boundary)**: Only `task` artifacts may store or project `complexity`, `complexity_source`, and `complexity_ruleset_version`. Both `validateComplexityMutation` (`internal/core/artifact_complexity.go`) and `projectedCustomFields` (`internal/db/upsert_projection.go`) MUST enforce `artifactType == "task"` before schema lookup or SQLite projection.
* **I2 (Provenance completeness & atomic clear)**: At the core seam (`SetArtifactComplexityWithProvenance`), setting `complexity_source` requires a non-empty, control-character-free `complexity_ruleset_version`; setting `complexity_ruleset_version` requires an effective non-empty `complexity_source` and `complexity`; and setting `complexity == ""` atomically clears `complexity`, `complexity_source`, and `complexity_ruleset_version` (while rejecting any accompanying `complexity_source` or `complexity_ruleset_version` argument).
* **I3 (Fail-closed event-before-write durability)**: Every complexity mutation MUST append an `estimate_history` (`dimension: "complexity"`) event to `.backlogit/logs/<id>.jsonl` BEFORE writing the Markdown file. If the audit append fails, the Markdown file and SQLite row MUST remain untouched.
* **I4 (Sole-writer seam & transport trust boundary)**: Generic `CreateArtifact` MUST refuse `complexity`, `complexity_source`, and `complexity_ruleset_version` (`reservedSizingKeys`), generic `UpdateArtifact` MUST preserve existing values of all three keys across `custom_fields` updates, and MCP `backlogit_update_item` MUST reject explicit `complexity_source: "human"` regardless of ruleset.
* **I5 (Fail-safe-upward route eligibility)**: `IsTaskRouteEligible(item)` MUST return `true` if and only if `item != nil && item.ArtifactType == "task"` and all 6 size + complexity provenance fields (`size`, `size_source`, `size_ruleset_version`, `complexity`, `complexity_source`, `complexity_ruleset_version`) are present and valid against their enums and ruleset constraints.

### Learnings Consulted

* `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md` — Enforce `artifactType == "task"` in the core setter before resolving `HeaderDef` schema and in `internal/db/upsert_projection.go`.
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — Regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` whenever CLI flags change.
* `docs/compound/2026-06-26-docline-frontmatter-contract.md` — Preserve Markdown body bytes via `internal/mdfront` and keep generated CLI reference docs compliant with `backlogit docs lint`.
* `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md` — Merge-preserve reserved provenance keys across generic updates so typed round-trips never strip provenance.
* `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` — Re-upsert into SQLite on every mutating write and lock CLI/MCP parity with cross-surface tests.

### Strict-Safety Action Classifications (`strict-safety`)

| Unit | ProposedAction | ActionRisk | Safeguard |
|---|---|---|---|
| U1 (`207.001-T`) | Update `.backlogit/header-def.yaml`, `internal/config/defaults.go`, `internal/config/headerdef.go`, `internal/core/artifact_complexity.go`, `internal/core/artifact_size.go`, and `internal/core/artifact_complexity_test.go` | `safe_write` | Confined to workspace config and core complexity/size seams; fail-closed audit append before atomic write; tested in `t.TempDir()` |
| U2 (`207.002-T`) | Update `internal/db/upsert_projection.go`, `internal/db/queries.go`, `internal/core/artifact_complexity.go`, and `internal/db/queries_complexity_test.go` | `safe_write` | Additive SQLite schema extension via existing `ApplySchemaExtensions`; bound SQL parameters (`?`); tested in `t.TempDir()` |
| U3 (`207.003-T`) | Update `internal/cli/update.go`, `internal/mcp/tools.go`, `internal/mcp/complexity_metadata_test.go`, `.autoharness/backlog-registry.yaml`, and regenerate `docs/cli-reference/backlogit_update.md` | `safe_write` | Enforces MCP `human` rejection trust boundary; verified by `TestRegistryParity_*` and `git diff --exit-code docs/cli-reference/` |

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * Core: `core.ComplexityMutation`, `core.SetArtifactComplexityWithProvenance`, `core.SetArtifactComplexity`, `core.IsTaskRouteEligible` (`internal/core/artifact_complexity.go`, `internal/core/artifact_size.go`).
  * Config & Schema: `.backlogit/header-def.yaml`, `internal/config/defaults.go`, `internal/config/headerdef.go`, `internal/db/upsert_projection.go`, `internal/db/queries.go` (`db.QueryFilters`, `db.QueryItems`, `db.IsTaskRouteEligible`).
  * CLI & MCP: `backlogit update --complexity-source --complexity-ruleset-version` (`internal/cli/update.go`) and `backlogit_update_item` (`internal/mcp/tools.go`).
* **Runtime verification expectations**:
  * Run `go test -count=1 -race ./internal/config/... ./internal/core/... ./internal/db/... ./internal/cli/... ./internal/mcp/...` and `go vet ./...`.
  * Verify `go run ./cmd/gen-docs docs/cli-reference` produces zero `git diff` on `docs/cli-reference/` and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Operational closure expectations**:
  * Produce `docs/closure/208-S-207-F-post-merge-closure.md` conforming to the `{shipment_id}-{feature_id}-post-merge-closure.md` filename contract, with top-level `closure_status` and `compaction_status`.
  * Invoke `compact-context` (`target: all`) per P-020 before finalizing `compaction_status` and completing Phase 3 of the Dark Factory run.
  * **Rollback trigger and procedure**: If complexity provenance mutations, SQLite projections, or CLI reference drift checks regress in CI, revert the `208-S` merge commit via `git revert -m 1 <merge_sha>` on a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Concurrency Reviewer, Security Lens Reviewer, SQLite Reviewer, Schema-CLI-Docs Coupling Reviewer (complete sequential rubric pass across all 10 applicable personas).
* plan_hardening: `Requires plan hardening: yes` confirmed and satisfied by `## Plan Hardening` (Invariants I1–I5, learnings trace, and `strict-safety` `ProposedAction` / `ActionRisk` classifications).
* constitution_check: `Constitution Check: pass` confirmed; all constitutional principles mapped with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 3 (applied in place).
  * P3-1 (`Go Reviewer` / `Schema-CLI-Docs Coupling Reviewer`, U1 `internal/config/headerdef.go`): Ensure `upgradeLegacyGeneratedHeaderDef` upgrades pre-207-F generated `header-def.yaml` files lacking `complexity_source` and `complexity_ruleset_version` (in addition to pre-complexity variants lacking `complexity`) so `TestLoadHeaderDef_LegacyGeneratedDefaultAddsComplexity` and existing workspaces continue to load seamlessly. Applied in place in U1.
  * P3-2 (`SQLite Reviewer`, U2 `internal/db/upsert_projection.go` & `internal/db/queries.go`): Extend the `artifact.ArtifactType != "task"` projection guard in `projectedCustomFields` to cover `complexity_source` and `complexity_ruleset_version` alongside `complexity`, and use bound `?` parameters for `ComplexitySource` and `ComplexityRulesetVersion` in `db.QueryItems`. Applied in place in U2.
  * P3-3 (`Security Lens Reviewer` / `Agent-Native Parity Reviewer`, U3 `internal/mcp/tools.go` & `internal/cli/update.go`): Reject explicit `complexity_source: "human"` over MCP regardless of whether `complexity_ruleset_version` is supplied, and default `complexity_source` to `"human"` on CLI and `"agent"` on MCP when `complexity_ruleset_version` is supplied without `complexity_source` (and complexity is not being cleared). Applied in place in U3.
* runtime_verification_and_closure: Targeted `-race` verification across `./internal/config/...`, `./internal/core/...`, `./internal/db/...`, `./internal/cli/...`, and `./internal/mcp/...`, CLI reference drift verification, and post-merge closure artifact `docs/closure/208-S-207-F-post-merge-closure.md` with P-020 `compact-context` invocation are explicitly specified.
