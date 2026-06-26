---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-10T00:00:00Z
    origin: .backlogit/queue/025-F.md
    review: .copilot-tracking/plan-review/2026-04-10-workspace-governance-integrity-plan-review.md
    status: revised
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-10-workspace-governance-integrity-plan.md
title: Workspace Governance and Integrity
---

## Problem Frame

backlogit's workspace integrity depends on several invariants that are currently
unenforced or only partially enforced:

1. **Hierarchy violations**: Level-2 tasks can be created with null `parent_id`
   despite `hierarchy_level: 2` in the WIT configuration. `validateArtifactParent`
   exists (artifacts.go:265-294) but only checks allowed-children configuration,
   not hierarchy-level requirements. Nine orphaned tasks required manual
   remediation (see origin: docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md).

2. **Archive/queue duplication**: `ArchiveItem` removes the original file
   (archive.go:73-76) and `ShipShipment` delegates to it, but no test asserts
   queue-path absence after archiving. The 021-F/009-S scope was found in both
   locations (see origin: .backlogit/queue/006-DL.md).

3. **Agent write discipline**: The constitution (Section IV) states all
   operations must resolve within `.backlogit/`, but no instruction explicitly
   prohibits agents from writing directly to `.backlogit/` files, bypassing
   backlogit tooling.

4. **Archival lifecycle**: No policy governs when completed items should be
   archived. Manual-only archival leads to stale artifacts accumulating in the
   queue directory.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Reject level-2+ items with null parent_id at creation time | 025.013-T, compound learning |
| R2 | Reject level-2+ items with null parent_id at harvest time | 025.013-T, compound learning |
| R3 | Doctor command to detect orphaned items | 025.013-T, 006-DL |
| R4 | Test queue-path absence after archive and ship operations | 006-DL chosen direction |
| R5 | Post-ship consistency verification in ShipShipment | 006-DL chosen direction |
| R6 | Agent instructions enforcing .backlogit write-only via tooling | 025.011-T |
| R7 | Archival policy design document with rationale | 025.012-T |
| R8 | Doctor exposed as MCP tool for agent and CI use | 006-DL option C |

## Scope Boundaries

### In Scope

* Hierarchy enforcement at creation and harvest boundaries
* Archive/queue consistency test hardening
* Post-ship verification in ShipShipment
* Doctor command (CLI + MCP) for orphan and duplicate detection
* Agent instruction updates for write-only .backlogit discipline
* Archival policy design document

### Non-Goals

* Circular reference detection in dependency graphs (separate feature)
* Concurrency/locking governance (separate feature)
* Adopt-item ID rewriting (related but larger scope, tracked separately)
* CI integration for doctor checks (can be added after doctor exists)
* Age-based auto-archival implementation (policy design only in this plan)

### Deferred to Implementation

* Exact error message wording for hierarchy rejection
* Doctor dual-format presentation and richer CLI flag design (first cut: one canonical JSON report)
* Stale-done-item detection in doctor (future follow-up; not referenced by archival policy)

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units target
a single skill domain and specify a verifiable exit state.

### Unit 1: Harden archive and shipment tests for queue-path absence

**Files:** `internal/core/archive_test.go`, `internal/core/shipment_test.go`
**Test files:** same (test-only unit)
**Effort size:** small
**Skill domain:** tests
**Execution note:** test-first (write failing tests against current code; they should pass since ArchiveItem already removes the file, but the assertions are missing)
**Patterns to follow:** existing table-driven test style in `archive_test.go` (TestArchiveItem_MovesToArchive)
**Dependencies:** none

**Approach:**
Add assertions to `TestArchiveItem_MovesToArchive` verifying the original queue
file no longer exists after archiving. Add a new test
`TestShipShipment_QueuePathAbsentAfterShip` that ships a shipment and verifies
no item ID exists as a file in `.backlogit/queue/`. Add a test
`TestArchiveItem_NoDuplicateAcrossQueueArchive` that archives an item and
confirms the ID appears in exactly one location.

**Verification:**
`go test ./internal/core/... -run "QueuePath|NoDuplicate"` passes. No item file
exists in both queue and archive directories after any archive or ship operation.

### Unit 2: Enforce hierarchy level constraints in CreateArtifact

**Files:** `internal/core/artifacts.go`, `internal/core/hierarchy.go`
**Test files:** `internal/core/artifacts_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first (red tests before implementation)
**Patterns to follow:** existing `validateArtifactParent` function (artifacts.go:265-294), `LevelForType` in hierarchy.go
**Dependencies:** none

**Red tests (write first):**

1. `TestCreateArtifact_RejectsLevel2WithoutParent`: creating a task (level 2)
   without parent_id returns `ErrValidation`-wrapped error containing "requires
   parent_id".
2. `TestCreateArtifact_AcceptsLevel1WithoutParent`: creating a feature (level 1)
   without parent_id succeeds.
3. `TestCreateArtifact_AcceptsLevel2WithParent`: creating a task with valid
   parent_id succeeds.
4. `TestLevelForType_NilLayout`: calling `LevelForType` with nil QueueLayout
   returns 0 and no panic.

**Approach:**

1. Harden `LevelForType()` to return 0 when `layout` or `layout.Levels` is nil
   instead of panicking. (Addresses F-007.)
2. Extend `validateArtifactParent` (or add a new `validateHierarchyLevel` check
   called from `CreateArtifact`) to query the WIT config for the artifact type's
   `hierarchy_level`. If level >= 2 and `parentID` is empty, return a descriptive
   error wrapped with `internal/errors.ErrValidation` via `%w`. (Addresses F-008.)
3. Scope enforcement to creation-time only. Items that become orphans through
   shipment returns (where `ShipShipment` intentionally clears `parent_id`) are
   not affected by this check. (Addresses F-009.)

**Verification:**
Table-driven tests: all 4 red tests pass. MCP handler surfaces
`validation_failed` error code, not `internal`. `go test ./internal/core/...
-run "Hierarchy|LevelForType"` passes.

### Unit 3: Enforce hierarchy level constraints in HarvestStashEntry

**Files:** `internal/core/stash.go`
**Test files:** `internal/core/stash_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first (red tests before implementation)
**Patterns to follow:** existing `HarvestStashEntry` function (stash.go:247-320) delegates to `CreateArtifact`
**Dependencies:** Unit 2 (hierarchy enforcement must exist in CreateArtifact first)

**Red tests (write first):**

1. `TestHarvestStashEntry_RejectsTaskWithoutParent`: harvesting a task-kind stash
   entry without parent_id returns error and the stash entry remains intact.
2. `TestHarvestStashEntry_SucceedsWithParent`: harvesting with valid parent_id
   succeeds and removes the stash entry.
3. `TestHarvestStashEntry_PreservesStashOnCreateFailure`: if CreateArtifact fails
   for any reason, the stash JSONL file still contains the original entry.

**Approach:**

`HarvestStashEntry` currently removes and rewrites the stash file *before*
`CreateArtifact` runs. With Unit 2 adding hierarchy validation, a failed
harvest would return an error after the stash entry has already been deleted,
causing data loss. (Addresses F-003, P0.)

Fix the ordering:

1. Validate hierarchy constraints *before* modifying the stash file. Call
   `LevelForType()` and check parent_id requirements as a pre-flight check
   before any stash mutation.
2. Only remove the stash entry after `CreateArtifact` succeeds.
3. If `CreateArtifact` fails, the stash entry must remain intact. No partial
   mutations.

**Verification:**
`go test ./internal/core/... -run "HarvestHierarchy|HarvestStash"` passes. Red
test 3 explicitly verifies that the stash JSONL file is unchanged after a
failed harvest attempt.

### Unit 4: Implement doctor command with orphan and duplicate checks

**Files:** `internal/core/doctor.go` (new), `internal/cli/doctor_cmd.go` (new)
**Test files:** `internal/core/doctor_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first (red tests before implementation)
**Patterns to follow:** existing CLI command pattern in `internal/cli/` (Cobra subcommands), `LevelForType` for hierarchy lookups, `*core.Workspace` aggregate
**Dependencies:** none (works against current workspace state)

**Red tests (write first):**

1. `TestDoctor_DetectsOrphanedTask`: seed workspace with a level-2 task with no
   parent_id, assert finding type "orphan" with the task ID.
2. `TestDoctor_IgnoresIntentionalOrphans`: seed workspace with a level-2 task
   whose status is "queued" and has no parent_id but was returned from a
   shipment (has `archived_from` or shipment provenance metadata), assert no
   false-positive orphan finding. (Addresses F-009.)
3. `TestDoctor_DetectsDuplicateAcrossQueueArchive`: seed workspace with the same
   artifact ID file in both queue and archive, assert finding type "duplicate".
4. `TestDoctor_CleanWorkspaceNoFindings`: seed a valid workspace, assert empty
   findings.
5. `TestDoctor_NilLayoutDoesNotPanic`: workspace with no QueueLayout config
   returns empty report, no panic.

**Approach:**

Create a `Doctor` function in `internal/core/doctor.go` that accepts a
`*core.Workspace` (not raw DB + path) and returns a `DoctorReport` struct.
(Addresses F-017.) Implement two checks:

1. **orphans**: Read markdown artifacts from the workspace's active directory
   (resolved through workspace/registry abstractions, not literal paths —
   addresses F-016). Parse frontmatter for `id`, `artifact_type`, and
   `parent_id`. Flag any item where `LevelForType(artifact_type) >= 2` and
   `parent_id` is null or references a non-existent item. Distinguish
   intentional orphans (items returned from shipment with cleared parent_id)
   from corruption by checking for shipment provenance in event logs.
   (Addresses F-009.)
2. **duplicates**: Walk the workspace's active and archive roots (resolved
   through registry, not hardcoded paths — addresses F-016). Parse frontmatter
   for `id` (not filename extraction — addresses F-010). Flag any ID present in
   both active and archive locations.

Doctor reads markdown/JSONL artifacts directly as the authoritative source, not
the ephemeral SQLite cache. SQLite may assist performance for large workspaces
as a rebuildable index, but all findings must be verifiable against the
filesystem. (Addresses F-005.)

Wire into CLI as `backlogit doctor` with `--check=orphans`,
`--check=duplicates`, or `--check=all` (default). Single canonical output
format: structured JSON `DoctorReport` to stdout. (Addresses F-013.)

Add `slog.Info` for scan start/end and `slog.Warn` for each detected issue.
Emit a telemetry event to `telemetry.jsonl` with scan duration, check types
run, and finding counts. (Addresses F-014.)

**Verification:**
All 5 red tests pass. `go test ./internal/core/... -run "Doctor"` passes.
`backlogit doctor` on a clean workspace returns `{"findings": []}`.
`backlogit doctor` on a seeded workspace returns expected orphan/duplicate
findings with correct IDs.

### Unit 5: Expose doctor as MCP tool

**Files:** `internal/mcp/tools.go`
**Test files:** `tests/contract/doctor_contract_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first (red contract tests before implementation)
**Patterns to follow:** existing MCP tool registration pattern in tools.go (five-step handler pattern)
**Dependencies:** Unit 4 (doctor core logic must exist)

**Red tests (write first):**

1. `TestDoctorTool_SchemaValid`: tool schema has required fields and valid enum.
2. `TestDoctorTool_AlwaysVisible`: tool appears in tool list regardless of
   workspace state. (Addresses F-006.)
3. `TestDoctorTool_DescriptivePreInitError`: calling doctor before workspace init
   returns structured `workspace_not_initialized` error, not a hidden tool.
   (Addresses F-006.)
4. `TestDoctorTool_ReturnsCompactJSON`: successful invocation returns compact
   `DoctorReport` JSON with findings array (issue type, artifact IDs, severity,
   remediation hints — no raw markdown bodies). (Addresses F-006.)
5. `TestDoctorTool_CleanWorkspaceEmptyFindings`: invocation on clean workspace
   returns `{"findings": []}`.

**Approach:**
Register `backlogit_doctor` MCP tool with optional `check` parameter (enum:
"orphans", "duplicates", "all"; default "all"). Handler follows the five-step
pattern: validate workspace, parse params, get Workspace, call
`core.Doctor(ws)`, return JSON result. Tool is unconditionally visible and
returns descriptive errors before init. JSON Schema is derived from Go struct
tags. (Addresses F-006.)

Add `slog.Info` for tool invocation and result summary. (Addresses F-014.)

**Verification:**
All 5 contract tests pass. `go test ./tests/contract/... -run "Doctor"` passes.
Tool is visible in `tools/list` response before and after workspace init.

### Unit 6: Add post-ship archive consistency verification

**Files:** `internal/core/shipment_lifecycle.go`
**Test files:** `internal/core/shipment_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first (red tests before implementation)
**Patterns to follow:** existing `archiveItems` helper (shipment_lifecycle.go:248-265)
**Dependencies:** Unit 1 (tests for queue-path absence should exist first)

**Red tests (write first):**

1. `TestShipShipment_FailsOnStaleQueueFile`: inject a scenario where a queue
   file persists after archival (mock or skip removal). Verify ShipShipment
   returns an error indicating the stale file.
2. `TestShipShipment_SucceedsWhenQueueClean`: normal ship completes without
   error when all queue files are properly removed.

**Approach:**

After `archiveItems()` completes in `ShipShipment`, add a verification pass
that checks each archived item ID does not have a corresponding file in the
queue directory. Use explicit queue-root path check (`os.Stat` on the expected
queue path), not `FindArtifactPath` or `loadArtifact` which search across
registered directories and could find the archived file. (Addresses F-015.)

Resolve the queue root through workspace/registry abstractions, not literal
`.backlogit/queue/` paths. (Addresses F-016.)

If any queue file persists, return an error that blocks the shipment. Source-of-
truth consistency failures are too important to allow through as warnings.
(Addresses F-004. Reverses Decision D3.)

Log with `slog.Error` for consistency failures and `slog.Info` for verification
pass/fail. Emit telemetry event. (Addresses F-014.)

Verification result is logged only; the `ShipShipmentResult` struct is not
changed. If a typed warnings field is needed later, it can be added in a
follow-up. (Addresses F-018.)

**Verification:**
Unit 1 tests continue to pass. Both red tests pass. `go test
./internal/core/... -run "Ship"` passes.

### Unit 7: Update agent instructions and skill protocols for governance

**Files:** `.github/instructions/backlogit.instructions.md`, `.github/instructions/constitution.instructions.md`, `.github/skills/harvest/SKILL.md`, `.github/agents/groomer.agent.md`, `.github/agents/shipper.agent.md`
**Test files:** none (documentation unit)
**Effort size:** small
**Skill domain:** docs
**Execution note:** characterization-first (read current instructions, identify gaps, patch)
**Patterns to follow:** existing instruction file structure and tone
**Dependencies:** none

**Approach:**

The originating task 025.011-T requires constraints in constitution, skills,
and agent files — not just two instruction documents. This unit covers all
affected surfaces. (Addresses F-012.)

1. Add explicit rules to `backlogit.instructions.md`:
   * "Agents MUST NOT write directly to `.backlogit/` files. All mutations must
     go through backlogit CLI commands or MCP tools."
   * "When creating tasks, you MUST provide a `parent_id` referencing an existing
     feature. Create the parent feature first if one does not exist."
   * "When adding items to a shipment, always add the parent feature before child
     tasks."

2. Clarify in the constitution's Section IV (Workspace Containment) that
   `.backlogit/` write discipline has two enforcement layers:
   * **Tool-level**: `SafeResolve` and path validation in core code enforce
     containment at the file-operation boundary.
   * **Agent-level**: Instructions and skill protocols enforce that agents use
     the tool surface rather than writing files directly.
   This two-layer model satisfies F-002 without implying a gap in tool-level
   enforcement. (Addresses F-002.)

3. Update `harvest/SKILL.md` to require parent_id for task-kind harvests and
   document the parent-first ordering requirement.

4. Update `groomer.agent.md` and `shipper.agent.md` to reference the write-only
   discipline rule.

**Verification:**
Grep for the new rules in all 5 files confirms they exist. No build or test
impact (docs-only change).

### Unit 8: Design and document archival policy

**Files:** `docs/decisions/archival-policy.md` (new)
**Test files:** none (documentation unit)
**Effort size:** small
**Skill domain:** docs
**Execution note:** characterization-first (evaluate options from 025.012-T and 006-DL)
**Patterns to follow:** existing decision documents in `docs/decisions/`
**Dependencies:** none

**Approach:**
Write a decision document evaluating four archival policy options:

1. **Shipment-based** (archive on shipment close): items archived when their
   shipment ships. Strongest traceability, already partially implemented.
2. **Time-based** (archive N days after done): automatic cleanup for items that
   complete outside shipment flow.
3. **Manual-only**: operator archives explicitly. Maximum control, no automation.
4. **Hybrid** (shipment-close primary + time-based fallback): recommended.
   Shipment close handles the normal path; items marked done for >30 days
   without a shipment are flagged by doctor for manual archival.

Recommend the hybrid approach with rationale. Note that implementation of
time-based detection is deferred to a future feature; the archival policy
document describes the target state without referencing tooling that does not
yet exist. (Addresses F-011.)

**Verification:**
File exists at `docs/decisions/archival-policy.md` with frontmatter and all four
options evaluated. No reference to in-scope doctor tooling for stale-done
detection.

## Dependency Graph

```text
Unit 1 (test hardening)      ──→ Unit 6 (post-ship verification)
Unit 2 (hierarchy in create)  ──→ Unit 3 (hierarchy in harvest)
Unit 4 (doctor core)          ──→ Unit 5 (doctor MCP tool)
Unit 7 (instructions/skills)  ──→ Units 2/3 safe to ship
Unit 8 (archival policy)      ── (independent)
```

Parallel tracks:
- **Track A** (archive consistency): Unit 1 → Unit 6
- **Track B** (hierarchy enforcement): Unit 2 → Unit 3
- **Track C** (doctor tooling): Unit 4 → Unit 5
- **Track D** (documentation): Unit 7 + Unit 8 (parallel, no code dependencies)

All tracks can proceed in parallel for code-complete. However, Units 2 and 3
cannot be shipped (merged to main) until Unit 7 is also complete, because
hierarchy enforcement will break agent workflows that harvest tasks without
parent-first ordering. Unit 7 updates the harvest skill protocol and agent
instructions to prevent this. (Addresses F-020.)

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Enforce hierarchy via level check, not config-per-type | `LevelForType()` already maps types to levels; adding per-type `requires_parent` flags duplicates the hierarchy concept | Per-type boolean flag in config.yaml |
| D2 | Doctor returns one canonical structured JSON format | Agents consume JSON efficiently. Defer dual-format presentation and richer CLI flags to follow-up | Dual stdout/stderr format (premature complexity) |
| D3 | Post-ship verification fails, does not merely warn | Source-of-truth consistency (Principle VII) is too important for advisory-only. Blocking prevents corrupt state from persisting. Revised from original warn-only per review F-004 | Warn-only (risks silent corruption) |
| D4 | Hybrid archival policy (shipment-close + time fallback, policy-only) | Covers the normal path and the edge case. Time-based detection is deferred to future work; policy document describes target state only | Pure time-based (ignores shipment lifecycle), manual-only (no automation) |
| D5 | Write-only enforcement via two layers: tool-level containment + agent-level instructions | SafeResolve enforces path containment at the code boundary. Instructions enforce that agents use the tool surface rather than writing files directly. Neither layer alone is sufficient | Filesystem ACLs (blocks backlogit itself), instructions-only (no tool-level guarantee) |
| D6 | Doctor reads markdown/JSONL as authority, not SQLite cache | Principle VII (CQRS) treats index.db as disposable. Integrity checks must verify the source of truth | SQLite-only checks (validates stale derived data) |
| D7 | Doctor distinguishes intentional orphans from corruption | ShipShipment legitimately clears parent_id on returned items. Flagging those as corruption produces false positives | Flag all nil-parent level-2+ items (breaks shipment flow) |

## Risks and Caveats

1. **Harvest backward compatibility**: Enforcing parent_id on task harvest will
   break any agent workflow that harvests tasks without first creating a parent
   feature. Mitigation: Unit 7 updates harvest skill protocol and agent
   instructions to require parent-first ordering. Unit 7 must ship alongside
   or before Units 2/3.

2. **Harvest data loss (P0 — addressed)**: `HarvestStashEntry` previously
   removed the stash entry before `CreateArtifact`. With new hierarchy
   validation, this would cause data loss on failure. Unit 3 fixes the ordering:
   validate first, mutate only on success.

3. **Existing orphaned items**: If any orphaned items remain in the workspace
   when hierarchy enforcement lands, they will not retroactively fail. Doctor
   (Unit 4) provides detection. Manual remediation via `backlogit_adopt_item`
   remains the fix path.

4. **Intentional orphans from shipment returns**: `ShipShipment` legitimately
   clears `parent_id` on unreleased descendants. Doctor (Unit 4) distinguishes
   these from corruption by checking shipment provenance. Creation-time
   enforcement (Unit 2) does not affect items that become orphans post-creation.

5. **Doctor performance on large workspaces**: Filesystem walk for duplicate
   detection could be slow with thousands of artifacts. Mitigation: doctor
   reads markdown directly for authority but may fall back to the SQLite index
   for performance on orphan queries, cross-checking a sample against markdown.

6. **Post-ship verification blocks shipment**: Decision D3 (revised) makes
   consistency failures block shipping. If filesystem caching (Windows) causes
   a false positive, the operator must retry or investigate. This is
   intentionally conservative for source-of-truth integrity.

## Learnings Applied

* Orphaned tasks compound learning (docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md): informed R1, R2, R3, Units 2-5
* Atomic rehydration learning (docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md): informed defensive approach in Unit 6
* Advisory file lock learning (docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md): informed crash-safety considerations for doctor
* Stash staleness learning (docs/compound/workflow-issues/stash-staleness-requires-custom-scripting-2026-04-09.md): informed archival policy options in Unit 8
* Queue view filter learning (docs/compound/config-issues/queue-view-empty-filter-values-2026-04-05.md): informed duplicate detection approach in Unit 4

## Standards Check

* **Constitution I (Type-Safe Go)**: All new code uses typed structs, GoDoc comments, sentinel errors. Doctor returns `DoctorReport` struct, not raw maps. Hierarchy errors wrap `ErrValidation` via `%w`.
* **Constitution II (MCP Protocol Fidelity)**: Doctor MCP tool is unconditionally visible, returns descriptive pre-init errors, derives JSON Schema from Go structs.
* **Constitution III (Test-First)**: Every code unit specifies explicit red tests written before implementation. Contract tests for MCP tool. Red test lists are enumerated in each unit.
* **Constitution IV (Workspace Containment)**: Two-layer enforcement: SafeResolve at tool boundary + instructions at agent boundary. Doctor reads only within `.backlogit/`. No path traversal.
* **Constitution V (Structured Observability)**: All new operations emit `slog.Info`/`slog.Warn`/`slog.Error` events. Doctor and post-ship verification emit telemetry to `telemetry.jsonl`.
* **Constitution VII (CQRS)**: Doctor reads markdown/JSONL as authoritative source. SQLite assists performance only. Post-ship verification blocks on source-of-truth inconsistency.
* **Constitution IX (Agent Context Efficiency)**: Doctor MCP tool returns compact structured JSON: issue type, artifact IDs, severity, remediation hints. No raw markdown bodies.

## Review Response

This revision addresses all findings from the plan review gate
(`.copilot-tracking/plan-review/2026-04-10-workspace-governance-integrity-plan-review.md`).

### P0 findings addressed

| Finding | Resolution |
|---|---|
| F-001 (test-first missing) | Added explicit "Red tests (write first)" subsections to Units 2, 3, 4, 5, 6 |
| F-002 (write-only enforcement) | Clarified two-layer model in Unit 7 and Decision D5 |
| F-003 (harvest data loss) | Rewrote Unit 3 to validate before stash mutation; added Risk 2 |

### P1 findings addressed

| Finding | Resolution |
|---|---|
| F-004 (warn too weak) | Reversed D3: post-ship fails, not warns. Unit 6 rewritten |
| F-005 (markdown not SQLite) | Added D6: doctor reads markdown as authority. Unit 4 rewritten |
| F-006 (MCP protocol) | Added 5 red contract tests to Unit 5 covering visibility, pre-init errors, schema |
| F-007 (LevelForType nil) | Added red test 4 in Unit 2 and nil guard in approach |
| F-008 (ErrValidation wrap) | Unit 2 approach specifies `ErrValidation` wrapping |
| F-009 (intentional orphans) | Added D7: doctor distinguishes provenance. Units 2, 4 updated. Risk 4 added |
| F-010 (filename coupling) | Unit 4 approach: parse frontmatter for ID, not derive from basename |
| F-011 (policy references missing tooling) | Unit 8 revised: no reference to in-scope doctor stale-done detection |
| F-012 (Unit 7 scope gap) | Expanded to 5 files including skills and agent files |

### P2 findings addressed

| Finding | Resolution |
|---|---|
| F-013 (output simplify) | D2 revised: one canonical JSON format. Dual-format deferred |
| F-014 (observability) | Added slog + telemetry requirements to Units 4, 5, 6 |
| F-015 (queue absence method) | Unit 6: explicit os.Stat on queue-root path |
| F-016 (registry abstractions) | Units 4, 6: resolve paths through workspace/registry |
| F-017 (Workspace aggregate) | Unit 4: accepts `*core.Workspace`, not raw DB + path |
| F-018 (result contract) | Unit 6: log-only, no ShipShipmentResult change |
| F-019 (CLI verification) | Unit 4: CLI verification included in test approach |
| F-020 (Unit 7 prerequisite) | Dependency graph: Unit 7 required before Units 2/3 ship |
