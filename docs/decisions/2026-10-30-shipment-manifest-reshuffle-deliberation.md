---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash DE3E7B67 (Phase 2, feature 206-F / shipment 207-S): stage primitive shipment manifest replanning operations (remove, move, reorder with --auto topological ordering, --dry-run, --force+--reason guard, audited events, and CLI/MCP parity) now, and capture declarative reconstitute --plan, adopt manifest propagation, doctor checks, and stash repoint as a linked Phase 2b follow-up stash.'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md
    - docs/exec-plans/2026-10-30-shipment-manifest-reshuffle-plan.md
    - .backlogit/queue/206-F.md
    - .backlogit/queue/207-S.md
  promoted_to: plan
  tags:
    - shipment
    - manifest
    - replanning
    - cli
    - mcp
    - parity
    - DE3E7B67
    - 206-F
    - 207-S
  topic: 'Replan queued shipment manifests without hand edits (remove, move, reorder) and stage Phase 2b reconstitution follow-up'
schema_version: "1.0"
source: docs/decisions/2026-10-30-shipment-manifest-reshuffle-deliberation.md
title: 'Deliberation: Queued-shipment manifest replanning operations (DE3E7B67 / 206-F)'
---

## Problem Frame

* Active stash entry `DE3E7B67` (*FEATURE REQUEST: Shipment manifest reshuffle and reconstitution*, full specification in `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md`, folded with `8CCB29CF` per `docs/decisions/2026-10-08-stage-foldins-and-dispositions.md` F4) is the Phase 2 target of the operator's Dark Factory directive (`2026-10-30T05:55:05Z`: *"Phase 1: Consistent CLI and MCP errors; Phase 2: Replan shipments without hand-editing manifests; Phase 3: Trustworthy metadata for cheaper model routing. Run pipeline in Dark Factory mode."*).
* In backlogit today (`internal/core/shipment.go`, `internal/cli/shipment.go`, `internal/mcp/tools.go`), shipment manifest membership only supports `AddItemToShipment` (`backlogit shipment add` / `backlogit_add_to_shipment`) and active-shipment `ReturnBlockedItem` (`backlogit shipment return-blocked` / `backlogit_return_blocked`).
* When a queued shipment needs replanning after review (splitting oversized shipments, removing descoped tasks, moving tasks to a sibling shipment, or restoring covering-feature-first and `blocks`-topological manifest order), operators and agents have no governed primitive to remove, move, or reorder `custom_fields.items`. Hand-editing `.backlogit/queue/<id>-S.md` bypasses core validation (`validateShipmentItemIDs`, one-shipment-per-item invariants, status guards), leaves no JSONL audit trail in `.backlogit/logs/`, and risks SQLite index drift.
* The full design document (`docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md`) spans five capabilities:
  1. **Primitive manifest operations (`remove`, `move`, `reorder`)** on queued shipments by default, with `--auto` topological ordering, `--dry-run`, `--force` + `--reason` override on non-queued shipments, audited JSONL events, and full CLI/MCP parity.
  2. **Declarative multi-shipment reconstitution (`shipment reconstitute --plan <yaml>`)** composing primitive manifest operations, feature creation, and `adopt`.
  3. **`adopt` rename propagation** for remaining shipment-manifest and archived-stash-provenance gaps.
  4. **`doctor` manifest integrity checks** (dangling/renamed manifest IDs, multi-manifest items, feature split across shipments, forward `blocks` edges, unsequenced queued shipments).
  5. **Stash provenance correction (`stash correct --reason` / `stash repoint`)** and ID-reuse prevention.

## Research Findings

* **Core shipment concurrency and persistence (`internal/core/shipment.go`)**:
  * `AddItemToShipment` (`internal/core/shipment.go:1553-1648`) establishes the canonical lock and persistence pattern: acquire `lockShipmentLifecycleGlobal(ctx, ws)`, run `recoverPendingShipmentOperations(ctx, ws)`, acquire `lockShipmentMembership(ctx, ws, shipmentID)`, normalize manifest items with `NormalizeShipmentItems(shipment)` (which never returns `nil`, preserving the `[]` JSON wire-shape invariant from `docs/compound/go-patterns/f015-shipment-stash-patterns.md` and `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`), and execute `MutationEnvelope` with `"frontmatter-update"` (`persistArtifact(ctx, ws, shipment, false)` + rollback compensation via `cloneArtifact`) and `"jsonl-append"` (`NewWorkspaceEventWriter` + `bldb.IndexEvent`).
  * For cross-shipment `MoveShipmentItem(ctx, ws, itemID, fromShipmentID, toShipmentID, opts)`, holding `lockShipmentLifecycleGlobal` and acquiring `lockShipmentMembership` on `fromShipmentID` and `toShipmentID` in canonical lexicographical order prevents lock-order deadlocks (`docs/compound/2026-10-19-lock-held-side-effect-ordering.md`).
* **Covering-feature and topological ordering (`internal/core/shipment_covering.go`, `internal/db/dependencies.go`)**:
  * `isRootCoveringFeature(item)` (`internal/core/shipment_covering.go:114-122`) identifies a root covering feature (`item.ArtifactType == "feature" && !strings.Contains(item.ID, ".")`).
  * `ReorderShipment` with `--auto` can deterministically place root covering feature(s) first, then tasks topologically ordered by intra-shipment `blocks` dependencies (`item.Dependencies` where `dep.Type == "" || dep.Type == "blocks"`), with each task immediately followed by its child subtasks (`subtask.ParentID == task.ID`) in deterministic tie-breaking order.
* **CLI/MCP parity and CI gates (`internal/cli/registry_parity_test.go`, `.github/workflows/ci.yml`)**:
  * `.autoharness/backlog-registry.yaml` is checksum-governed by `.autoharness/harness-manifest.yaml` and must not be modified in application tasks. Instead, `loadRegistryOperations` in `internal/cli/registry_parity_test.go` can merge an in-repo pending-registry map for `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment` so `TestRegistryParity_EveryMCPToolMappedOrDeferred`, `TestRegistryParity_EveryCLICommandResolves`, `TestRegistryParity_NoOrphanMCPTool`, `TestRegistryParity_DiscoverabilityConsistency`, and `TestRegistryParity_FlagAndPositionalParity` enforce full CLI/MCP parity without touching `.autoharness/`.
  * `.github/workflows/ci.yml` runs `go run ./cmd/gen-docs docs/cli-reference` and `git diff --exit-code docs/cli-reference/` in the `CLI Reference Drift` job (`docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`). Adding Cobra subcommands `shipment remove`, `shipment move`, and `shipment reorder` requires regenerating `docs/cli-reference/` in the same unit.

## Options Evaluated

### Option A: Stage Primitive Manifest Replanning Operations (`remove`, `move`, `reorder`) in `206-F` / `207-S` and Capture Phase 2b Extensions as Follow-Up Stash — Chosen

* **Description**:
  1. **U1 (`206.001-T`, `code`)**: Implement `ShipmentManifestOptions`, `RemoveFromShipmentResult`, `MoveShipmentItemResult`, `RemoveFromShipment`, and `MoveShipmentItem` in `internal/core/shipment_manifest.go` with unit tests `TestU1_RemoveFromShipment_*` and `TestU1_MoveShipmentItem_*` in `internal/core/shipment_manifest_test.go`. Enforce `queued`-only default with `--force` + non-empty `--reason` override for non-`queued` shipments, `--dry-run` preview without persistence or event emission, atomic source+target `MutationEnvelope` rollback on `MoveShipmentItem`, and `shipment_item_removed` / `shipment_item_moved` JSONL + SQLite-indexed events.
  2. **U2 (`206.002-T`, `code`, blocked by `206.001-T`)**: Implement `ReorderShipmentResult` and `ReorderShipment` in `internal/core/shipment_manifest.go` supporting both explicit `--items` permutation validation (exact member set equality, root covering feature first, subtasks not preceding their parent task) and `--auto` topological sorting (covering feature first, tasks in `blocks` topological order with cycle detection and deterministic tie-breaking, subtasks immediately after their parent task), `--dry-run`, `--force` + `--reason` guard on non-`queued` shipments, and `shipment_reordered` events, with unit tests `TestU2_ReorderShipment_*`.
  3. **U3 (`206.003-T`, `code`, blocked by `206.002-T`)**: Wire CLI subcommands `backlogit shipment remove`, `backlogit shipment move`, and `backlogit shipment reorder` in `internal/cli/shipment.go` and MCP tools `backlogit_remove_from_shipment`, `backlogit_move_shipment_item`, and `backlogit_reorder_shipment` in `internal/mcp/tools.go`; register pending in-repo parity mappings and `TestU3_ShipmentManifestCLIAndMCP_*` tests in `internal/cli/registry_parity_test.go`; and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference`.
  4. **Follow-up stash (Phase 2b)**: Capture declarative `shipment reconstitute --plan`, `adopt` manifest/stash-provenance rename propagation, `doctor` manifest integrity checks, and `stash correct --reason` / `stash repoint` in a linked follow-up stash entry referencing `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` and `206-F` / `207-S`.
* **Pros**:
  * Delivers the complete foundational capability to replan queued shipment manifests without hand-editing Markdown files (`remove`, `move`, `reorder --items|--auto`, `--dry-run`, `--force --reason`) in 3 right-sized, test-first tasks that strictly obey the 2-Hour Rule and width isolation.
  * Provides the exact core building blocks that declarative `reconstitute --plan` will compose in Phase 2b.
  * Keeps `.autoharness/backlog-registry.yaml` untouched while satisfying both `registry_parity_test.go` and CI's `CLI Reference Drift` check.
* **Cons**:
  * Multi-shipment YAML plan execution (`reconstitute --plan`), `adopt` manifest propagation, `doctor` manifest checks, and `stash repoint` are staged for a subsequent shipment rather than bundled into `207-S`.
* **Effort estimate**: Medium (3 atomic tasks, ~6 hours human-equivalent).

### Option B: Monolithic 5-Subsystem Shipment (`remove`/`move`/`reorder` + `reconstitute --plan` + `adopt` propagation + `doctor` + `stash repoint`)

* **Description**:
  * Stage all five sections of `docs/design-docs/2026-09-29-backlogit-shipment-manifest-reshuffle-feature-request.md` across `internal/core`, `internal/stash`, `internal/db`, `internal/cli`, and `internal/mcp` in a single shipment `207-S`.
* **Pros**:
  * Implements every section of the design doc in one PR.
* **Cons**:
  * Violates the 2-Hour Rule and width isolation (requires 12–15 tasks across 5 distinct subsystems, recreating the exact oversized-shipment problem described in the `201-S` scenario of the design doc itself).
  * High blast radius across shipment lifecycle, `adopt` ID renumbering, `doctor` diagnostics, and stash provenance logs.
* **Effort estimate**: Very High (12+ tasks).

## Trade-off Comparison

| Criterion | Option A (Foundational Primitive Ops in `207-S` + Phase 2b Follow-Up Stash) | Option B (Monolithic 5-Subsystem Shipment) |
|---|---|---|
| Solves hand-editing of shipment manifests | Yes (`remove`, `move`, `reorder --items\|--auto` via CLI and MCP) | Yes |
| 2-Hour Rule & Width Isolation | Pass (3 focused `code` tasks) | Fail (12+ tasks across 5 subsystems) |
| Atomic rollback & audit trail | Full `MutationEnvelope` + JSONL/SQLite event parity | Complex multi-subsystem transaction surface |
| CI & harness safety | Preserves `.autoharness/` checksums and `docs/cli-reference/` drift gate | High risk of cross-package conflict |

## Decision

Adopt **Option A**: promote primitive shipment manifest replanning operations (`remove`, `move`, `reorder`) to implementation plan `docs/exec-plans/2026-10-30-shipment-manifest-reshuffle-plan.md` under feature `206-F` and shipment `207-S`, and record a linked follow-up stash entry for Phase 2b (`shipment reconstitute --plan`, `adopt` manifest/stash rename propagation, `doctor` manifest integrity checks, and `stash correct --reason` / `stash repoint`).

## Rejected Alternatives

* **Option B (Monolithic 5-Subsystem Shipment)**: Rejected because bundling all five subsystems into a single shipment violates the 2-Hour Rule and width isolation, whereas landing the primitive manifest operations first delivers immediate replanning value and establishes a clean foundation for Phase 2b.

## Unresolved Questions

* None. All core lock helpers, sentinel errors, event schemas, CLI flags, MCP parameters, and CI drift gates are verified against the current codebase.

## Risks and Mitigations

* **Risk 1 (Cross-shipment lock ordering in `MoveShipmentItem`)**: Acquiring `lockShipmentMembership` in caller-supplied `(from, to)` order could deadlock if two concurrent moves run in opposite directions (`S1 -> S2` and `S2 -> S1`).
  * **Mitigation**: Always acquire `lockShipmentLifecycleGlobal` first and then acquire `lockShipmentMembership` on the two shipment IDs in canonical lexicographical order (`min(from, to)` then `max(from, to)`).
* **Risk 2 (`TestRegistryParity_EveryMCPToolMappedOrDeferred` vs `.autoharness/backlog-registry.yaml` checksum guard)**: Registering new MCP tools in `internal/mcp/tools.go` without updating `internal/cli/registry_parity_test.go` would fail `TestRegistryParity_EveryMCPToolMappedOrDeferred`, while editing `.autoharness/backlog-registry.yaml` would break `.autoharness/harness-manifest.yaml` checksum verification.
  * **Mitigation**: Merge `pendingInRepoRegistryOperations` inside `loadRegistryOperations` in `internal/cli/registry_parity_test.go` for `remove_from_shipment`, `move_shipment_item`, and `reorder_shipment` so all five registry parity assertions validate the new tools and CLI commands without modifying `.autoharness/`.
* **Risk 3 (`CLI Reference Drift` CI gate)**: Adding `remove`, `move`, and `reorder` under `backlogit shipment` changes the Cobra command tree checked by `go run ./cmd/gen-docs docs/cli-reference`.
  * **Mitigation**: Include `go run ./cmd/gen-docs docs/cli-reference` in U3 (`206.003-T`) so `docs/cli-reference/backlogit_shipment*.md` stays byte-identical to the generator output.
