---
chunk_strategy: h1-h2-h3
description: 'Deliberation for Phase 1A of the Dark Factory C1+C3 directive (stash 7CE12EE6 and 40CC5C6B items 2, 3, 4; feature 208-F / shipment 209-S): propagate AdoptItem renames to shipment manifests and archived stash provenance, add adopt --keep-id, make ID allocation history-safe across logs and archived stash, add --force operator-reasoned repoint to CorrectStashProvenance with CLI/MCP parity, and add doctor shipment manifest integrity checks.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md
    - docs/decisions/2026-10-30-shipment-manifest-reshuffle-deliberation.md
    - docs/exec-plans/2026-10-30-adopt-propagation-id-safety-doctor-manifest-plan.md
    - .backlogit/queue/208-F.md
    - .backlogit/queue/209-S.md
  promoted_to: plan
  tags:
    - adopt
    - id-allocation
    - stash-provenance
    - doctor
    - manifest
    - 7CE12EE6
    - 40CC5C6B
    - 208-F
    - 209-S
  topic: 'Adopt rename propagation, history-safe ID allocation, stash correct --force, and doctor shipment manifest integrity checks'
schema_version: "1.0"
source: docs/decisions/2026-10-30-adopt-propagation-id-safety-doctor-manifest-deliberation.md
title: 'Deliberation: Adopt rename propagation, history-safe ID allocation, and doctor manifest checks (7CE12EE6 + 40CC5C6B / 208-F)'
---

## Problem Frame

* Active stash entries `7CE12EE6` (*backlogit ID allocator has no reservation or explicit-ID create option...*) and `40CC5C6B` (*Phase 2b follow-up from DE3E7B67: (1) declarative `shipment reconstitute --plan`; (2) `adopt` rename propagation for remaining shipment-manifest and archived-stash-provenance gaps + `--keep-id`; (3) `doctor` manifest integrity checks; (4) operator-reasoned `stash correct --reason` / `stash repoint` and non-recycling of IDs with history/provenance*) form the C1 target of the operator's Dark Factory directive (`Option 1: Run the pipeline in dark factory mode`).
* To keep every shipment bounded to 3 atomic single-domain (`code`) tasks under the 2-Hour Rule and provide clean foundational primitives before declarative multi-shipment `shipment reconstitute --plan` lands in Phase 1B (`210-S`), Phase 1A (`208-F` / `209-S`) consolidates `7CE12EE6` and `40CC5C6B` items (2), (3), and (4):
  1. **`AdoptItem` rename propagation (`custom_fields.items` in shipment manifests + `harvested_artifact_id` in `.backlogit/archive/stash.jsonl`) and `adopt --keep-id`** (`docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` §3).
  2. **History-safe non-recycling ID allocation** across SQLite history (`item_logs`, `item_log_entries`, `stash_links`) and workspace disk (`queue/`, `archive/`, `logs/`, `archive/stash.jsonl`, `archive/provenance_corrections.jsonl`) (`7CE12EE6` + `40CC5C6B` §5).
  3. **Operator-reasoned `CorrectStashProvenance` (`--force`) and CLI/MCP registry parity** (`40CC5C6B` §5).
  4. **`Doctor` shipment manifest integrity checks** (`dangling_shipment_member`, `duplicate_shipment_membership`, `split_feature_across_shipments`, `forward_shipment_dependency`, `unsequenced_queued_shipment`) (`40CC5C6B` §4).
* Item (1) of `40CC5C6B` — declarative multi-shipment `backlogit shipment reconstitute --plan <yaml> [--dry-run] [--json]` — remains active on `40CC5C6B` for **Phase 1B** (`209-F` / `210-S`), where it directly composes the hardened `AdoptItemWithOptions`, history-safe ID allocator, and `Doctor` manifest checks delivered in `209-S`.

## Research Findings

* **`AdoptItem` & cross-reference rewrites (`internal/core/shipment_lifecycle.go:1185-1450`, `internal/core/artifact_references.go`)**:
  * `findCrossArtifactReferences` and `applyCrossArtifactRewrites` (`internal/core/artifact_references.go`) currently inspect only `a.ParentID`, `a.Dependencies`, and `a.Links`. When an item inside a shipment's `custom_fields.items` manifest is adopted under a new parent feature and receives a new ID (`oldID -> newID`), shipment manifests are not rewritten, leaving a dangling `oldID` in `custom_fields.items`. Extending `findCrossArtifactReferences` and the under-lock refresh loop in `applyCrossArtifactRewrites` to inspect `NormalizeShipmentItems(a)` when `a.ArtifactType == "shipment"` and replace `oldID` with `newID` in slice order (deduplicating if `newID` is already present and preserving a non-nil `[]string`) makes shipment manifest rewrites atomic and rollback-safe under `applyCrossArtifactRewrites`.
  * While `AdoptItem` updates `stash_links.item_id` in SQLite via `bldb.RewriteAncillaryReferences`, `.backlogit/archive/stash.jsonl` stores `"harvested_artifact_id": "<oldID>"`. Adding an atomic helper in `AdoptItemWithOptions` that locks `.backlogit/archive/stash.jsonl` (`lockStashFile`, refusing symlinks via `os.Lstat`), snapshots the file bytes for rollback on transaction failure, and rewrites any `harvested_artifact_id == oldID` to `newID` keeps the durable JSONL archive consistent with SQLite.
  * Adding `AdoptItemOptions{KeepID bool}` and `AdoptItemWithOptions(ctx, ws, itemID, newParentID, opts)` (with `AdoptItem` delegating with `AdoptItemOptions{}`) allows callers to re-parent an item (`parent_id = newParentID`, `origin_feature` preserved) while keeping `newID == oldID` when `KeepID: true`.
* **History-safe ID allocation (`internal/core/hierarchy.go`, `internal/core/naming.go`, `internal/core/artifacts.go`)**:
  * `NextTypedHierarchicalID` (`internal/core/hierarchy.go:81-147`) and `NextID` (`internal/core/naming.go:47-74`) currently query only `SELECT id FROM items`. When an artifact is created and deleted before merge or unindexed on disk, its ordinal can be recycled even though `.backlogit/logs/<id>.jsonl` or `.backlogit/archive/stash.jsonl` (`harvested_artifact_id`) still references `<id>`.
  * Extending `NextTypedHierarchicalID` and `NextID` to also advance past ordinals recorded in SQLite `item_logs`, `item_log_entries`, and `stash_links`, and adding workspace-aware `NextTypedHierarchicalIDForWorkspace` / `NextIDForWorkspace` (invoked by `CreateArtifact` and `AdoptItemWithOptions`) that also scan `WorkspaceQueueRoot`, `WorkspaceArchiveRoot`, `WorkspaceLogsRoot` (`<id>.md`, `<id>.jsonl`), `.backlogit/archive/stash.jsonl` (`harvested_artifact_id`), and `.backlogit/archive/provenance_corrections.jsonl`, guarantees that no ID with history or provenance is ever recycled.
* **Operator-reasoned stash provenance correction (`internal/core/stash_provenance.go`, `internal/cli/stash_correct.go`, `internal/mcp/tools_reconcile.go`, `.autoharness/backlog-registry.yaml`)**:
  * `CorrectStashProvenance` (`internal/core/stash_provenance.go:63-224`) currently rejects any target artifact whose `custom_fields.source_stash_id` does not already equal `req.StashID`, and rejects a second correction for the same stash ID if it names a different canonical artifact.
  * Adding `Force bool` to `StashProvenanceCorrectionRequest` (requiring non-empty `Reason` and `Actor`) allows an operator-reasoned correction when `Force: true` to (a) atomically stamp `custom_fields["source_stash_id"] = req.StashID` on the target artifact (persisting Markdown + SQLite `items` + `stash_links`), (b) supersede an earlier conflicting correction in `provenance_corrections.jsonl` (`readProvenanceCorrections` returns the last matching `CanonicalDeliveryArtifactID`), and (c) record `"forced": true` on the `ProvenanceCorrection` JSONL entry, the `stash_provenance_corrected` item-log event, and `StashProvenanceCorrectionResult`.
  * `internal/cli/stash_correct.go` already defines `backlogit stash correct --stash-id <id> --canonical-delivery <id> --reason <r> --actor <a>`. Extending `newStashCorrectCommand` to add `--force`, accept optional positional `[stash-id]` and `--artifact` (alias for `--canonical-delivery`), wiring `force` on MCP `backlogit_correct_stash_provenance`, and replacing `mcp_only: true` on `correct_stash_provenance` in `.autoharness/backlog-registry.yaml` with `cli_command: "backlogit stash correct --stash-id {{stash_id}} --canonical-delivery {{canonical_delivery_artifact_id}} --reason {{reason}} --actor {{actor}}"` achieves full CLI/MCP registry parity.
* **`Doctor` shipment manifest integrity checks (`internal/core/doctor.go`)**:
  * Extending `Doctor` (`internal/core/doctor.go`) with `detectShipmentManifestFindings(refs, ids)` over non-terminal shipments (`status` in `queued`, `active`, `blocked` and located outside `archive/`) detects all 5 manifest anomalies from `40CC5C6B` §4:
    1. `FindingDanglingShipmentMember` (`"dangling_shipment_member"`, error severity): manifest item ID not present in `refs` (`queue/` or `archive/`).
    2. `FindingDuplicateShipmentMembership` (`"duplicate_shipment_membership"`, error severity): item ID present in `custom_fields.items` of more than one non-terminal shipment.
    3. `FindingSplitFeatureAcrossShipments` (`"split_feature_across_shipments"`, error severity): child tasks/subtasks sharing the same `parent_id` feature distributed across more than one non-terminal shipment.
    4. `FindingForwardShipmentDependency` (`"forward_shipment_dependency"`, error severity): an item in non-terminal shipment `S1` has a `blocks` dependency on an item in non-terminal shipment `S2` where `S2` depends on `S1` (directly or transitively in the shipment `blocks` DAG, meaning `S1` is scheduled before `S2`).
    5. `FindingUnsequencedQueuedShipment` (`"unsequenced_queued_shipment"`, advisory severity — does not set `DoctorSeverityError`): when more than 1 `queued` shipment exists, a `queued` shipment has no `blocks` dependency on another shipment and does not carry the `dag-root` label.

## Options Evaluated

### Option A: Stage Slice A (`AdoptItem` propagation + `--keep-id`, history-safe ID allocation, `stash correct --force`, and `Doctor` manifest checks) in `208-F` / `209-S`, and Stage Declarative `shipment reconstitute --plan` in Phase 1B (`210-S`) — Chosen

* **Description**: Deliver the four foundational integrity and provenance capabilities (`7CE12EE6` + `40CC5C6B` items 2, 3, 4) in feature `208-F` and shipment `209-S` across 3 sequential single-domain (`code`) tasks (`208.001-T` -> `208.002-T` -> `208.003-T`), while keeping `40CC5C6B` active for Phase 1B (`209-F` / `210-S`: declarative `backlogit shipment reconstitute --plan`).
* **Pros**:
  * Respects the 2-Hour Rule (3 focused `code` tasks touching 2–4 files each).
  * Ensures `AdoptItem` manifest/stash propagation, history-safe ID allocation, and `Doctor` manifest checks are already in place when `shipment reconstitute --plan` is built in Phase 1B (`210-S`).
  * Fully resolves `7CE12EE6` (preventing ID recycling across deleted/logged/archived items) and items (2), (3), and (4) of `40CC5C6B`.
* **Cons**:
  * Splits `40CC5C6B` across two consecutive shipments (`209-S` and `210-S`), mitigated by immediate sequential execution in the same Dark Factory run.

### Option B: Bundle Declarative `shipment reconstitute --plan` into `208-F` / `209-S` — Rejected

* **Description**: Combine all 4 items of `40CC5C6B` plus `7CE12EE6` into a single 5–6 task shipment.
* **Cons**:
  * Exceeds the 3-task single-concern scope for `209-S` and couples YAML plan parsing/transaction orchestration with foundational `AdoptItem`, ID-allocator, `stash correct`, and `Doctor` primitives.

## Decision

Adopt **Option A**: promote `7CE12EE6` and `40CC5C6B` items (2), (3), and (4) to implementation plan `docs/exec-plans/2026-10-30-adopt-propagation-id-safety-doctor-manifest-plan.md` under covering feature `208-F` and shipment `209-S`, and update `40CC5C6B` in place so item (1) (`backlogit shipment reconstitute --plan`) proceeds immediately in Phase 1B (`210-S`).
