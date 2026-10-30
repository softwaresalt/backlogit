---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for feature 208-F (shipment 209-S, Phase 1A, stash 7CE12EE6 + 40CC5C6B Slice A): propagate AdoptItem ID rewrites to shipment manifests and archived stash provenance, add AdoptItemWithOptions (KeepID), make ID allocation history-safe across logs and archive, add --force operator-reasoned repoint to CorrectStashProvenance, add doctor shipment manifest integrity checks, and wire CLI/MCP/registry parity.'
doc_type: plan
docline:
  feature_id: 208-F
  shipment_id: 209-S
  source_decision: docs/decisions/2026-10-30-adopt-propagation-id-safety-doctor-manifest-deliberation.md
  stash_id: 7CE12EE6
schema_version: "1.0"
source: docs/exec-plans/2026-10-30-adopt-propagation-id-safety-doctor-manifest-plan.md
title: 'Implementation Plan: Adopt rename propagation, history-safe ID allocation, and doctor manifest checks (208-F / 209-S)'
---

## Objective

Deliver the Phase 1A integrity, provenance, and local ID-safety primitives from `40CC5C6B` (items 2, 3, and 4; `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §§3–5) under covering feature `208-F` and shipment `209-S` (while tracking the cross-branch/open-PR ID reservation / explicit `--id` / remote-aware allocator requirement from `7CE12EE6` in follow-up stash entry `7BE3AF92`):

1. **`AdoptItem` Rename Propagation, `KeepID` Option, and Local History-Safe ID Allocation (`U1` / `208.001-T`)**: Extend `findCrossArtifactReferences` and `applyCrossArtifactRewrites` (`internal/core/artifact_references.go`) to rewrite `oldID -> newID` inside shipment artifacts' `custom_fields.items` (preserving slice order, deduplicating if `newID` is already present, and maintaining a non-nil `[]string`); add atomic `.backlogit/archive/stash.jsonl` `harvested_artifact_id` rewrite with snapshot/restore rollback in `AdoptItemWithOptions` (`internal/core/shipment_lifecycle.go`); add `AdoptItemOptions{KeepID bool}` and `AdoptItemWithOptions` so `KeepID: true` updates `parent_id` and `origin_feature` without renumbering IDs or renaming files; and make `NextTypedHierarchicalID` / `NextID` / `MaxDiskArtifactSequence` (`internal/core/hierarchy.go`, `internal/core/naming.go`, `internal/core/artifacts.go`) local-history-safe by advancing past the maximum sequence ordinal found across SQLite (`items`, `item_logs`, `item_log_entries` including `delta_json` `old_id`, `stash_links`) and workspace disk (`queue/`, `archive/`, `logs/*.jsonl` filenames and event records' `item_id` + `delta.old_id` on `adopted` events, `archive/stash.jsonl`, and `archive/provenance_corrections.jsonl`). Note: U1 (`208.001-T`) addresses **local workspace history-based ID reuse** (`40CC5C6B` §5), whereas cross-branch reservation/explicit-ID/remote-aware allocation from `7CE12EE6` is tracked in follow-up stash entry `7BE3AF92`.
2. **Operator-Reasoned `CorrectStashProvenance` (`--force`) and `Doctor` Shipment Manifest Integrity Checks (`U2` / `208.002-T`)**: Add `Force bool` to `StashProvenanceCorrectionRequest` and `Forced bool` to `ProvenanceCorrection` / `StashProvenanceCorrectionResult` (`internal/core/stash_provenance.go`) so `Force: true` with non-empty `Reason` and `Actor` looks up the archived stash entry for `req.StashID` (from `.backlogit/archive/stash.jsonl` / `stash_entries`) and atomically stamps the **complete provenance snapshot** onto `target.CustomFields` — `source_stash_id`, `source_stash_priority`, `source_stash_kind`, `source_stash_text`, `source_stash_path` (defaulting to `"stash.jsonl"` when empty), and setting `source_deliberation_id` from the archived entry's `deliberation_id` when non-empty or deleting any pre-existing `source_deliberation_id` key when empty — on a target artifact lacking or carrying a different `source_stash_id` (updating Markdown, SQLite `items`, `stash_entries`, and `stash_links` so rehydration via `internal/db/rehydration.go:621-650` preserves full metadata), allows superseding a prior conflicting correction in `provenance_corrections.jsonl`, and records `"forced": true`; and extend `Doctor` (`internal/core/doctor.go`) with `detectShipmentManifestFindings` checking non-terminal shipments (`queued`, `active`, `blocked`) for `FindingDanglingShipmentMember`, `FindingDuplicateShipmentMembership`, `FindingSplitFeatureAcrossShipments`, `FindingForwardShipmentDependency` (error severity), and `FindingUnsequencedQueuedShipment` (advisory severity when more than 1 `queued` shipment exists without a `blocks` dependency or `dag-root` label).
3. **CLI, MCP, Registry, and CLI-Reference Parity (`U3` / `208.003-T`)**: Wire `--keep-id` on `backlogit adopt` (`internal/cli/adopt.go`) and `keep_id` on `backlogit_adopt_item` (`internal/mcp/tools.go`); wire `--force`, optional positional `[stash-id]`, and `--artifact` (alias for `--canonical-delivery`) on `backlogit stash correct` (`internal/cli/stash_correct.go`) and `force` on `backlogit_correct_stash_provenance` (`internal/mcp/tools_reconcile.go`); update `.autoharness/backlog-registry.yaml` (`adopt_item` and `correct_stash_provenance` CLI mapping); add CLI/MCP parity tests; and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference`.

Source decision: `docs/decisions/2026-10-30-adopt-propagation-id-safety-doctor-manifest-deliberation.md` (Option A, stash `7CE12EE6` + `40CC5C6B`; cross-branch ID reservation follow-up tracked in `7BE3AF92`).

## Activation Contract & Routing Context

* `DARK_MODE_ACTIVE`: `true` (`2026-10-30T22:02:00Z`)
* `operator_request_verbatim`: `'Option 1: Run the pipeline in dark factory mode'`
* `bounded_ordered_capability_scope`: `["Phase 1A: 7CE12EE6 + 40CC5C6B Slice A (208-F / 209-S)", "Phase 1B: 40CC5C6B Slice B shipment reconstitute --plan (209-F / 210-S)", "Phase 2: 21BA4886 ComplexityComposition rollup and harvest auto-stamping (210-F / 211-S)"]`
* `cursor`: `last_completed=208-S, next=Phase 1A (7CE12EE6 + 40CC5C6B Slice A -> 208-F / 209-S)`
* `merge_approval_pre_authorized`: `true`
* `admin_fallback_pre_authorized`: `false`
* `routing_note`: `gemini-4-argon/google/high/long_context` (`tier3`).

## Problem Frame

Re-parenting tasks via `AdoptItem` currently rewrites `parent_id`, `dependencies`, and `links` in other artifacts' frontmatter, but leaves `oldID` stale inside shipment manifests (`custom_fields.items`) and archived stash provenance (`.backlogit/archive/stash.jsonl` `harvested_artifact_id`), and offers no `--keep-id` option. Meanwhile, `NextTypedHierarchicalID` and `NextID` inspect only SQLite `items`, allowing sequence numbers of deleted or unindexed items that still have `.backlogit/logs/<id>.jsonl` or archived stash provenance to be recycled. Furthermore, `CorrectStashProvenance` cannot repair provenance when a target artifact lacks `source_stash_id` (`--force`), and `Doctor` lacks shipment manifest integrity checks for dangling members, duplicate membership, split features, forward `blocks` dependencies across shipments, and unsequenced queued shipments.

## Requirements Trace

| ID | Requirement | Source | Implementation Unit |
|---|---|---|---|
| R1 | Propagate `AdoptItem` `oldID -> newID` rewrites to shipment `custom_fields.items` (`internal/core/artifact_references.go`) and `.backlogit/archive/stash.jsonl` `harvested_artifact_id` (`internal/core/shipment_lifecycle.go`) with rollback safety; add `AdoptItemOptions{KeepID bool}` and `AdoptItemWithOptions`; and make `NextTypedHierarchicalID` / `NextID` / `MaxDiskArtifactSequence` (`internal/core/hierarchy.go`, `internal/core/naming.go`, `internal/core/artifacts.go`) advance past ordinals present in SQLite (`items`, `item_logs`, `item_log_entries` including `delta_json` `old_id`, `stash_links`) and workspace disk (`queue/`, `archive/`, `logs/*.jsonl` filenames and event records' `item_id` + `delta.old_id` on `adopted` events, `archive/stash.jsonl`, `archive/provenance_corrections.jsonl`) to prevent local workspace history-based ID reuse (cross-branch ID reservation from `7CE12EE6` is tracked in follow-up stash `7BE3AF92`) | `40CC5C6B` (§3, §5) & `7CE12EE6` (local scope; cross-branch in `7BE3AF92`) | U1 (`208.001-T`) |
| R2 | Add `Force bool` to `StashProvenanceCorrectionRequest` and `Forced bool` to `ProvenanceCorrection` / `StashProvenanceCorrectionResult` (`internal/core/stash_provenance.go`) to look up the archived stash entry for `req.StashID` and stamp the complete `source_stash_*` provenance snapshot (`source_stash_id`, `source_stash_priority`, `source_stash_kind`, `source_stash_text`, `source_stash_path` defaulting to `"stash.jsonl"`, and `source_deliberation_id` set or deleted) onto `target.CustomFields` (persisting Markdown + SQLite `items` + `stash_entries` + `stash_links` so rehydration preserves full metadata) and allow superseding prior conflicting corrections when `Force: true` with non-empty `Reason` and `Actor`; and add `detectShipmentManifestFindings` to `Doctor` (`internal/core/doctor.go`) emitting `FindingDanglingShipmentMember`, `FindingDuplicateShipmentMembership`, `FindingSplitFeatureAcrossShipments`, `FindingForwardShipmentDependency`, and `FindingUnsequencedQueuedShipment` | `40CC5C6B` (§4, §5) | U2 (`208.002-T`) |
| R3 | Expose `--keep-id` on `backlogit adopt` (`internal/cli/adopt.go`) and `keep_id` on `backlogit_adopt_item` (`internal/mcp/tools.go`), expose `--force`, optional positional `[stash-id]`, and `--artifact` on `backlogit stash correct` (`internal/cli/stash_correct.go`) and `force` on `backlogit_correct_stash_provenance` (`internal/mcp/tools_reconcile.go`), update `.autoharness/backlog-registry.yaml`, add CLI/MCP parity tests, and regenerate `docs/cli-reference/` | `40CC5C6B` (§3, §5) | U3 (`208.003-T`) |

## Constraints

* **2-Hour Rule and Width Isolation**: Every unit stays in the `code` domain, touches 2–4 tightly coupled source/test files (plus `.autoharness/backlog-registry.yaml` and generated `docs/cli-reference/` in U3), and keeps `go test ./...` green after every wave.
* **Non-nil slice wire-shape & symlink containment**: Rewritten shipment `custom_fields.items` slices must always be non-nil `[]string`; all `.backlogit/archive/stash.jsonl` and `provenance_corrections.jsonl` reads/writes must refuse symlinks via `os.Lstat` and verify workspace containment via `filepath.EvalSymlinks`.
* **Genuine tests-first RED -> GREEN**: Every unit specifies the exact failing test selector (`^TestU1_`, `^TestU2_`, `^TestU3_`) that compiles cleanly and is observed RED before production implementation turns it GREEN (`closed_exempt_set: []`).

## Implementation Units

### U1: Propagate `AdoptItem` ID rewrites, add `AdoptItemWithOptions` (`KeepID`), and make ID allocation history-safe (`code`) — `208.001-T`

* **Domain**: `code`
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/core/shipment_lifecycle.go`, declare exported struct `AdoptItemOptions{KeepID bool}` and stub `AdoptItemWithOptions(ctx context.Context, ws *Workspace, itemID, newParentID string, opts AdoptItemOptions) (*AdoptItemResult, error)` delegating to `AdoptItem(ctx, ws, itemID, newParentID)`.
    2. In `internal/core/adopt_id_safety_test.go`, add `TestU1_AdoptPropagationKeepIDAndHistorySafeAllocation`:
       * `adopt_rewrites_shipment_manifest_items_and_archived_stash_provenance`: adopting task `001.001-T` under feature `002-F` (`001.001-T -> 002.001-T`) atomically rewrites `001.001-T` to `002.001-T` in a referencing shipment's `custom_fields.items` (both Markdown and SQLite `items`, preserving slice order and deduplicating if `002.001-T` was already present) AND rewrites `"harvested_artifact_id":"001.001-T"` to `"002.001-T"` in `.backlogit/archive/stash.jsonl`.
       * `adopt_with_keep_id_preserves_id_and_updates_parent_and_origin_feature`: `AdoptItemWithOptions(ctx, ws, "001.001-T", "002-F", AdoptItemOptions{KeepID: true})` returns `OldID == "001.001-T"`, `NewID == "001.001-T"`, `NewParentID == "002-F"`, updates `parent_id: 002-F` and `custom_fields.origin_feature: 001-F` in Markdown and SQLite without renaming `.md` or `.jsonl` files, and appends an `item_adopted` event.
       * `id_allocation_never_recycles_deleted_adopted_or_unindexed_ids_with_logs_or_stash_history` (and `TestU1_NextTypedHierarchicalID_SkipsDeletedLoggedAndArchivedIDs`): (a) when `001.001-T` is created and then deleted from SQLite `items` and `.backlogit/queue/` (leaving `.backlogit/logs/001.001-T.jsonl`), and `001.002-T` is recorded as `"harvested_artifact_id":"001.002-T"` in `.backlogit/archive/stash.jsonl` (with no row in `items`), creating the next task under `001-F` allocates `001.003-T` rather than recycling `001.001-T` or `001.002-T`; (b) explicit adoption-retirement regression: `AdoptItem(001.001-T -> 002-F)` (which renames `001.001-T.md` / `001.001-T.jsonl` to `002.001-T` and rewrites `harvested_artifact_id` in `archive/stash.jsonl`) followed by `SyncIndex` and `CreateItem(parent: 001-F)` allocates `001.002-T` under `001-F` (never reusing the adopted-away `001.001-T` recorded in `002.001-T.jsonl` event records' `item_id` and `delta.old_id`); and (c) root feature/shipment allocation skips ordinals present in `.backlogit/logs/`, `.backlogit/archive/`, or `.backlogit/archive/stash.jsonl`.
    3. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU1_AdoptPropagationKeepIDAndHistorySafeAllocation$' ./internal/core` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/core/artifact_references.go`, update `findCrossArtifactReferences` and the under-lock refresh loop in `applyCrossArtifactRewrites` to inspect `NormalizeShipmentItems(a)` when `a.ArtifactType == "shipment"`; when `oldID` is present, replace `oldID` with `newID` in slice order (deduplicating if `newID` already exists in the slice and ensuring a non-nil `[]string`) and set `updated.CustomFields["items"]`.
    2. In `internal/core/shipment_lifecycle.go`, implement `AdoptItemWithOptions(ctx, ws, itemID, newParentID, opts)` (with `AdoptItem` calling `AdoptItemWithOptions(..., AdoptItemOptions{})`): when `opts.KeepID` is `true`, keep `newID = oldID`; when `newID != oldID`, use `NextTypedHierarchicalIDForWorkspace` and atomically rewrite any matching `harvested_artifact_id == oldID` to `newID` in `.backlogit/archive/stash.jsonl` (under `lockStashFile` with `os.Lstat` symlink refusal and byte-snapshot rollback if `tx.Commit()` fails).
    3. In `internal/core/hierarchy.go`, `internal/core/naming.go`, and `internal/core/artifacts.go`, extend `NextTypedHierarchicalID` and `NextID` to also query `item_logs`, `item_log_entries` (including `delta_json` `old_id` on `adopted` events), and `stash_links` in SQLite for historical IDs matching the parent/type pattern, and add `NextTypedHierarchicalIDForWorkspace` / `NextIDForWorkspace` / `MaxDiskArtifactSequence` (used by `CreateArtifact` and `AdoptItemWithOptions`) that additionally scan filenames in `WorkspaceQueueRoot`, `WorkspaceArchiveRoot`, and `WorkspaceLogsRoot`, event records inside `.backlogit/logs/*.jsonl` (specifically each event record's `item_id` and `delta.old_id` on `adopted` events so adopted-away IDs remain durably reserved even after `.md`/`.jsonl` renames and `SyncIndex`), `harvested_artifact_id` in `.backlogit/archive/stash.jsonl`, and `historical_artifact_id` / `canonical_delivery_artifact_id` in `.backlogit/archive/provenance_corrections.jsonl`.
* **Affected files**:
  * `internal/core/artifact_references.go`
  * `internal/core/shipment_lifecycle.go`
  * `internal/core/hierarchy.go`
  * `internal/core/naming.go`
  * `internal/core/artifacts.go`
  * `internal/core/adopt_id_safety_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU1_AdoptPropagationKeepIDAndHistorySafeAllocation$' ./internal/core`.
  * GREEN phase passes `go test -count=1 -race ./internal/core/...`.
* **Execution posture**: test-first.

### U2: Add `--force` to `CorrectStashProvenance` and shipment manifest integrity checks to `Doctor` (`code`) — `208.002-T`

* **Domain**: `code`
* **Depends on**: U1 (`208.001-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/core/stash_provenance.go`, add `Force bool` to `StashProvenanceCorrectionRequest` and `Forced bool` (`json:"forced,omitempty"`) to `ProvenanceCorrection` and `StashProvenanceCorrectionResult`.
    2. In `internal/core/doctor.go`, declare exported `DoctorFindingType` constants `FindingDanglingShipmentMember = "dangling_shipment_member"`, `FindingDuplicateShipmentMembership = "duplicate_shipment_membership"`, `FindingSplitFeatureAcrossShipments = "split_feature_across_shipments"`, `FindingForwardShipmentDependency = "forward_shipment_dependency"`, and `FindingUnsequencedQueuedShipment = "unsequenced_queued_shipment"`.
    3. In `internal/core/doctor_manifest_and_provenance_test.go`, add `TestU2_StashProvenanceForceAndDoctorManifestChecks` (and `TestU2_CorrectStashProvenance_ForceRepointsMissingSourceStashID`):
       * `stash_provenance_force_stamps_complete_provenance_snapshot_and_supersedes_conflict`: calling `CorrectStashProvenance` on a target artifact that has no `source_stash_id` (or a different `source_stash_id`) fails with `blerrors.ErrValidation` when `Force: false`, and succeeds when `Force: true` (with non-empty `Reason` and `Actor`), looking up the archived stash entry for `req.StashID` from `.backlogit/archive/stash.jsonl` / `stash_entries` and stamping the **complete provenance snapshot** onto `target.CustomFields` — `source_stash_id`, `source_stash_priority`, `source_stash_kind`, `source_stash_text`, `source_stash_path` (defaulting to `"stash.jsonl"` when empty), and setting `source_deliberation_id` from the archived entry's `deliberation_id` when non-empty or deleting any pre-existing `source_deliberation_id` key when empty — in Markdown and SQLite (`items`, `stash_entries`, and `stash_links`), recording `"forced": true` in `provenance_corrections.jsonl` and the `stash_provenance_corrected` item-log event, allowing a forced correction to supersede an earlier conflicting correction in `provenance_corrections.jsonl`, and passing an explicit `CorrectStashProvenance(Force: true) -> SyncIndex (Rehydrate)` regression assertion verifying that both the rebuilt `stash_entries` metadata and the canonical `stash_links` row survive rehydration (`internal/db/rehydration.go:621-650`).
       * `doctor_detects_shipment_manifest_integrity_anomalies`: `Doctor` over non-terminal shipments (`queued`, `active`, `blocked`) detects: (1) `FindingDanglingShipmentMember` (error severity) when a manifest item ID is missing from `queue/` and `archive/`; (2) `FindingDuplicateShipmentMembership` (error severity) when an item ID appears in two non-terminal shipment manifests; (3) `FindingSplitFeatureAcrossShipments` (error severity) when tasks sharing the same `parent_id` feature are in different non-terminal shipments; (4) `FindingForwardShipmentDependency` (error severity) when an item in shipment `S1` has a `blocks` dependency on an item in shipment `S2` while `S2` depends on `S1`; and (5) `FindingUnsequencedQueuedShipment` (advisory severity, `report.HasErrors() == false`) when 2+ `queued` shipments exist and a `queued` shipment has no `blocks` dependency on another shipment and lacks the `dag-root` label (and emits no finding when `dag-root` label or a `blocks` dependency is present).
    4. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU2_StashProvenanceForceAndDoctorManifestChecks$' ./internal/core` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/core/stash_provenance.go`: validate `strings.TrimSpace(req.Reason) != ""` and `strings.TrimSpace(req.Actor) != ""`; when `sourceStashID != req.StashID`, if `!req.Force` return `blerrors.ErrValidation`, else load the target artifact and stamp the complete provenance snapshot from the archived stash entry `entry` (already loaded via `readStashArchiveEntry`, with fallback lookup in `stash_entries` if needed) onto `artifact.CustomFields`: set `source_stash_id = req.StashID`, `source_stash_priority = entry.Priority` (defaulting to `stash.DefaultPriority` when empty), `source_stash_kind = entry.Kind`, `source_stash_text = entry.Text`, `source_stash_path` (preserving existing non-empty `source_stash_path` or defaulting to `"stash.jsonl"`), and if `strings.TrimSpace(entry.DeliberationID) != ""` set `source_deliberation_id = entry.DeliberationID` else `delete(artifact.CustomFields, "source_deliberation_id")`; durably write via `WriteArtifactFileWithOptions(artifact, artifactPath, WorkspaceDurableWrites(ws))`, and upsert into SQLite `items`, `stash_entries`, and `stash_links` (when `ws.DB != nil`); in `readProvenanceCorrections`, scan all matching lines and return the last `CanonicalDeliveryArtifactID`, and when `existingCanonical != "" && existingCanonical != req.CanonicalDeliveryArtifactID`, allow proceeding when `req.Force == true`; set `Forced: req.Force` on `ProvenanceCorrection`, `StashProvenanceCorrectionResult`, and the `stash_provenance_corrected` event delta.
    2. In `internal/core/doctor.go`, implement `detectShipmentManifestFindings(ws, refs, ids)` and invoke it from `Doctor` after `detectShipmentLifecycleFindings`: inspect non-terminal shipments (`status` in `queued`, `active`, `blocked` and path not under `archive/`) to emit `FindingDanglingShipmentMember`, `FindingDuplicateShipmentMembership`, `FindingSplitFeatureAcrossShipments`, and `FindingForwardShipmentDependency` via `newDoctorErrorFinding`, and `FindingUnsequencedQueuedShipment` as an advisory `DoctorFinding` (no `DoctorSeverityError`) when `len(queuedShipments) > 1` and a `queued` shipment has no `blocks` dependency on another shipment and lacks label `"dag-root"`.
* **Affected files**:
  * `internal/core/stash_provenance.go`
  * `internal/core/doctor.go`
  * `internal/core/doctor_manifest_and_provenance_test.go`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU2_StashProvenanceForceAndDoctorManifestChecks$' ./internal/core`.
  * GREEN phase passes `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/...`.
* **Execution posture**: test-first.

### U3: Wire CLI/MCP surfaces (`adopt --keep-id`, `stash correct --force`), update registry, and regenerate CLI docs (`code`) — `208.003-T`

* **Domain**: `code`
* **Depends on**: U2 (`208.002-T`)
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` in `*_test.go`)**:
    1. In `internal/cli/stash_correct_test.go` (or `internal/cli/adopt_stash_parity_test.go`), add `TestU3_AdoptKeepIDAndStashCorrectCLIMCPParity`:
       * CLI `backlogit adopt <item-id> --parent <feature-id> --keep-id` and MCP `backlogit_adopt_item` with `keep_id: true` re-parent the item while preserving its ID (`old_id == new_id`), and `.autoharness/backlog-registry.yaml` maps `keep_id` under `adopt_item.params`.
       * CLI `backlogit stash correct` supports `--force` as well as both `--stash-id <id> --canonical-delivery <id>` and positional `<stash-id> --artifact <id>` forms; MCP `backlogit_correct_stash_provenance` supports `force: true`; and `.autoharness/backlog-registry.yaml` maps `correct_stash_provenance` to `cli_command: "backlogit stash correct --stash-id {{stash_id}} --canonical-delivery {{canonical_delivery_artifact_id}} --reason {{reason}} --actor {{actor}}"` with `force: "force"` in `params` while passing all `TestRegistryParity_*` tests.
    2. Observe RED: `go test -run=^$ -count=1 ./...` exits 0 and `go test -count=1 -run '^TestU3_AdoptKeepIDAndStashCorrectCLIMCPParity$' ./internal/cli` fails.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. In `internal/cli/adopt.go`, add `--keep-id` boolean flag and call `core.AdoptItemWithOptions(ctx, ws, args[0], parentID, core.AdoptItemOptions{KeepID: keepID})`.
    2. In `internal/mcp/tools.go`, add `mcplib.WithBoolean("keep_id", mcplib.Description("Re-parent without renumbering the artifact ID or renaming files"))` to `backlogit_adopt_item` and pass `core.AdoptItemOptions{KeepID: keepID}` in `handleAdoptItem`.
    3. In `internal/cli/stash_correct.go`, add `--force` (`bool`) and `--artifact` (`string`, alias for `--canonical-delivery`), set `Args: cobra.MaximumNArgs(1)` so `<stash-id>` may be supplied positionally or via `--stash-id`, and pass `Force: force` in `core.StashProvenanceCorrectionRequest`.
    4. In `internal/mcp/tools_reconcile.go`, add `mcplib.WithBoolean("force", mcplib.Description("Stamp source_stash_id on the target artifact and override conflicting provenance when operator reason is supplied"))` to `backlogit_correct_stash_provenance` and pass `Force: force` in `handleCorrectStashProvenance`.
    5. In `.autoharness/backlog-registry.yaml`, add `keep_id: "keep_id"` under `adopt_item.params`, and update `correct_stash_provenance` to remove `mcp_only: true`, set `cli_command: "backlogit stash correct --stash-id {{stash_id}} --canonical-delivery {{canonical_delivery_artifact_id}} --reason {{reason}} --actor {{actor}}"`, and add `force: "force"` to `params`.
    6. Run `go run ./cmd/gen-docs docs/cli-reference` to regenerate `docs/cli-reference/backlogit_adopt.md` and `docs/cli-reference/backlogit_stash_correct.md`.
* **Affected files**:
  * `internal/cli/adopt.go`
  * `internal/cli/stash_correct.go`
  * `internal/cli/stash_correct_test.go`
  * `internal/mcp/tools.go`
  * `internal/mcp/tools_reconcile.go`
  * `.autoharness/backlog-registry.yaml`
  * `docs/cli-reference/backlogit_adopt.md`
  * `docs/cli-reference/backlogit_stash_correct.md`
* **Acceptance criteria**:
  * RED phase is observed failing in `go test -count=1 -run '^TestU3_AdoptKeepIDAndStashCorrectCLIMCPParity$' ./internal/cli`.
  * GREEN phase passes `go test -count=1 -race ./internal/cli/... ./internal/mcp/...`.
  * `go run ./cmd/gen-docs docs/cli-reference` followed by `git diff --exit-code docs/cli-reference/` exits 0, and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Execution posture**: test-first.

## Dependency Graph

* **Shipment sequencing**: `208-S` (shipped) -> `209-S` (`209-S` carries an explicit `blocks` dependency on `208-S`).
* **Wave 1**: `208.001-T` (U1 — `AdoptItem` manifest/stash propagation, `AdoptItemWithOptions`, and history-safe ID allocation)
* **Wave 2**: `208.002-T` (U2 — blocked by `208.001-T`; `CorrectStashProvenance` `--force` and `Doctor` shipment manifest integrity checks)
* **Wave 3**: `208.003-T` (U3 — blocked by `208.002-T`; CLI/MCP/registry wiring for `adopt --keep-id` and `stash correct --force` + `docs/cli-reference/` regeneration)

## Constitution Check

* `Constitution Check`: `pass`
* **CQRS & Durable Markdown/JSONL Authority**: All mutations (`AdoptItemWithOptions`, `CorrectStashProvenance --force`) write Markdown and JSONL atomically first and synchronize SQLite (`items`, `stash_links`) within the same governed flow.
* **CLI/MCP Parity**: Both `adopt --keep-id` and `stash correct --force` are exposed symmetrically across CLI, MCP, and `.autoharness/backlog-registry.yaml`.

## Plan Hardening Signals

* `Requires plan hardening`: `yes`

## Plan Hardening

### System-Wide Invariants

* **I1 (Atomic cross-reference & archived-stash rewrite in `AdoptItem`)**: When `AdoptItemWithOptions` renames `oldID -> newID`, any shipment manifest containing `oldID` in `custom_fields.items` and any `.backlogit/archive/stash.jsonl` record with `harvested_artifact_id == oldID` MUST be rewritten to `newID` with byte-snapshot rollback if transaction commit fails, and `custom_fields.items` MUST remain a deduplicated non-nil `[]string`.
* **I2 (Non-recycling of IDs with history, adoption lineage, or provenance)**: `NextTypedHierarchicalID` / `NextID` and their workspace-aware counterparts (`MaxDiskArtifactSequence`) MUST advance past the maximum ordinal present across SQLite (`items`, `item_logs`, `item_log_entries` including `delta_json` `old_id`, `stash_links`) and disk (`queue/`, `archive/`, `logs/*.jsonl` filenames and event records' `item_id` + `delta.old_id` on `adopted` events, `archive/stash.jsonl`, `archive/provenance_corrections.jsonl`).
* **I3 (Audited `--force` provenance repoint with full snapshot)**: `CorrectStashProvenance` with `Force: false` MUST continue to reject `source_stash_id != req.StashID` and conflicting corrections with `blerrors.ErrValidation`; `Force: true` requires non-empty `Reason` and `Actor`, stamps the complete `source_stash_*` provenance snapshot (`source_stash_id`, `source_stash_priority`, `source_stash_kind`, `source_stash_text`, `source_stash_path` defaulting to `"stash.jsonl"`, and `source_deliberation_id` set or deleted) onto `target.CustomFields` in Markdown and SQLite so `SyncIndex` / `Rehydrate` preserves both `stash_entries` metadata and `stash_links`, and records `"forced": true` in both the item log and `provenance_corrections.jsonl`.
* **I4 (`Doctor` manifest check severity split)**: `dangling_shipment_member`, `duplicate_shipment_membership`, `split_feature_across_shipments`, and `forward_shipment_dependency` are emitted with `DoctorSeverityError` (`HasErrors() == true`), whereas `unsequenced_queued_shipment` (emitted only when `len(queuedShipments) > 1` and the shipment has neither a shipment `blocks` dependency nor the `dag-root` label) is advisory (`Severity == ""`) so single-shipment or legacy test fixtures never fail unexpectedly.

### Learnings Consulted

* `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md` — Combine SQLite and canonical filesystem/log/archive scans so stale or deleted index rows never cause ID collisions or recycling.
* `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` — Keep `.autoharness/backlog-registry.yaml` flag templates aligned with `TestRegistryParity_FlagAndPositionalParity` and preserve non-nil `[]string` JSON slices.
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — Always run `go run ./cmd/gen-docs docs/cli-reference` when CLI flags change.

### Strict-Safety Action Classifications (`strict-safety`)

| Unit | ProposedAction | ActionRisk | Safeguard |
|---|---|---|---|
| U1 (`208.001-T`) | Update `internal/core/artifact_references.go`, `internal/core/shipment_lifecycle.go`, `internal/core/hierarchy.go`, `internal/core/naming.go`, `internal/core/artifacts.go`, and `internal/core/adopt_id_safety_test.go` | `safe_write` | Snapshot/restore rollback on `AdoptItem` cross-ref and stash archive rewrites; tested in `t.TempDir()` |
| U2 (`208.002-T`) | Update `internal/core/stash_provenance.go`, `internal/core/doctor.go`, and `internal/core/doctor_manifest_and_provenance_test.go` | `safe_write` | Symlink refusal (`os.Lstat`) + `EvalSymlinks` containment; read-only `Doctor` manifest checks; tested in `t.TempDir()` |
| U3 (`208.003-T`) | Update `internal/cli/adopt.go`, `internal/cli/stash_correct.go`, `internal/cli/stash_correct_test.go`, `internal/mcp/tools.go`, `internal/mcp/tools_reconcile.go`, `.autoharness/backlog-registry.yaml`, and regenerate `docs/cli-reference/` | `safe_write` | Verified by `TestRegistryParity_*` and `git diff --exit-code docs/cli-reference/` |

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * Core: `core.AdoptItemOptions`, `core.AdoptItemWithOptions`, `core.NextTypedHierarchicalIDForWorkspace`, `core.NextIDForWorkspace`, `core.CorrectStashProvenance` (`Force`), and `core.Doctor` (`detectShipmentManifestFindings`).
  * CLI & MCP: `backlogit adopt --keep-id` / `backlogit_adopt_item(keep_id)`, `backlogit stash correct [--force] [--artifact]` / `backlogit_correct_stash_provenance(force)`.
* **Runtime verification expectations**:
  * Run `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/...` and `go vet ./...`.
  * Verify `go run ./cmd/gen-docs docs/cli-reference` produces zero `git diff` on `docs/cli-reference/` and `backlogit docs lint --path docs/cli-reference` reports 0 violations.
* **Operational closure expectations**:
  * Produce `docs/closure/209-S-208-F-post-merge-closure.md` with top-level `closure_status` and `compaction_status`.
  * **Rollback trigger and procedure**: If `AdoptItem`, ID allocation, `stash correct`, or `Doctor` checks regress in CI, revert the `209-S` merge commit via `git revert -m 1 <merge_sha>` on a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Concurrency Reviewer, Security Lens Reviewer, SQLite Reviewer, Schema-CLI-Docs Coupling Reviewer.
* plan_hardening: `Requires plan hardening: yes` satisfied by `## Plan Hardening` (Invariants I1–I4, learnings consulted, and `strict-safety` classifications).
* constitution_check: `Constitution Check: pass` confirmed with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 2 (applied in place).
  * P3-1 (`Schema-CLI-Docs Coupling Reviewer`, U3 `internal/cli/stash_correct.go` & `.autoharness/backlog-registry.yaml`): Ensure `newStashCorrectCommand` supports both `--stash-id` / `--canonical-delivery` (used by `.autoharness/backlog-registry.yaml` and existing `TestStashCorrectCommand_*`) and optional positional `<stash-id>` / `--artifact` alias without breaking `TestRegistryParity_FlagAndPositionalParity`. Applied in U3.
  * P3-2 (`Go Reviewer`, U2 `internal/core/doctor.go`): Keep `FindingUnsequencedQueuedShipment` advisory (`Severity: ""`) and gated on `len(queuedShipments) > 1` so existing single-shipment and lifecycle doctor tests remain clean. Applied in U2.
