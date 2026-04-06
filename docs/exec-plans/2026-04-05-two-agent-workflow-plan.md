---
title: Two-agent workflow refactor plan
description: Implementation plan for refactoring backlogit to a groomer and shipper workflow with shipment support and stash JSONL storage
author: GitHub Copilot
ms.date: 2026-04-05
ms.topic: reference
keywords:
  - backlogit
  - workflow
  - agents
  - shipment
  - stash
estimated_reading_time: 14
origin: ".backlogit/queue/DL001.md"
status: reviewed
---

## Problem Frame

`DL001` defines a simpler operating model for backlogit: replace the current
multi-agent handoff chain with two orchestrators. The groomer owns stash to
ready backlog, and the shipper owns ready backlog to reviewed pull request.

The current repository already supports stash-linked deliberation artifacts, but
it still stores stash entries in `.backlogit/queue/.stash.md`, exposes no
shipment artifact type, and documents a workflow centered on
`backlog-harvester`, `harness-architect`, `build-orchestrator`, and `pr-review`.
The refactor therefore spans three coupled surfaces: backlogit product code,
the checked-in self-hosted workspace under `.backlogit/`, and the agent harness
under `.github/`.

This plan keeps the workflow refactor intentionally narrow. It introduces the
new two-agent pipeline and shipment artifact, migrates stash storage to JSONL,
and updates the harness and durable docs to match. It does not bundle the
separate hierarchical file-naming redesign.

## Requirements Trace

| #  | Requirement                                                                 | Origin |
|----|-----------------------------------------------------------------------------|--------|
| R1 | Replace the current many-agent workflow with two orchestrators: groomer and shipper | `DL001` Chosen Direction |
| R2 | Model the lifecycle as STASH -> BACKLOG -> SHIPMENT -> SHIPPED              | `DL001` Chosen Direction |
| R3 | Store stash as JSONL and keep backlog and shipment artifacts as Markdown with YAML frontmatter | `DL001` Chosen Direction |
| R4 | Add shipment as a first-class artifact with a 1:1 relationship to branch and pull request scope | `DL001` Notes |
| R5 | When work blocks mid-shipment, remove it from the shipment and return it to backlog with blocked state and reason | design session memory |
| R6 | Prefer a sidecar bootstrap so the new harness can coexist with the current one until validated | `DL001` Chosen Direction |
| R7 | Avoid external-user migration complexity because the project is still alpha  | design session memory |
| R8 | Keep existing leaf skills such as `deliberate`, `impl-plan`, `build-feature`, `review`, `fix-ci`, and `compound` | `DL001` Notes |
| R9 | Keep merge approval user-confirmed rather than enabling auto-merge in this refactor | design session memory |
| R10 | Keep durable docs in `docs/` and short-lived queue artifacts in `.backlogit/queue/` | repository workflow convention |

## Scope Boundaries

### In Scope

* Add shipment as a first-class artifact type in product defaults, workspace
  metadata, CLI, MCP, and queue/query logic.
* Migrate stash storage from `.backlogit/queue/.stash.md` to
  `.backlogit/stash.jsonl`, including rehydration and compatibility handling.
* Create sidecar `groomer` and `shipper` agents plus the extracted harness
  skills they need.
* Update the repository's durable instructions, workflow docs, and policy files
  to document the new pipeline.
* Add contract and integration coverage for shipment and stash migration flows.

### Non-Goals

* Do not combine the separate hierarchical file-naming redesign with this plan.
  The current prefix-based naming remains in place for this refactor.
* Do not remove legacy agents on day one. Mark them deprecated only after the
  new sidecar pipeline passes verification.
* Do not add auto-merge behavior. User approval remains the merge gate.

### Deferred to Implementation

* Decide whether the extracted skill folder should be named `harvest` or
  `backlog-harvest`, provided the final name is used consistently.
* Decide whether the shipper should invoke `doc-ops` directly or treat docs-only
  follow-up as a deferred optional branch.

## Shipment Schema Contract

The following contract applies across Units 1-3 and 8. Define it upfront so
all units share a single agreement.

**Prefix:** `S` (e.g., `S001`, `S002`). No collision with existing prefixes
(F, T, B, R, D, DL). Unit 1 must add a prefix uniqueness test.

**Frontmatter fields:**

| Field          | Type       | Validator                          | Notes |
|----------------|------------|------------------------------------|-------|
| `id`           | string     | `required`                         | Auto-generated with S prefix |
| `title`        | string     | `required,max=200`                 | Human-readable shipment name |
| `artifact_type`| string     | `required,eq=shipment`             | Always `shipment` |
| `status`       | string     | `required,oneof=queued active shipped abandoned` | Lifecycle state |
| `branch`       | string     | `omitempty`                        | Associated Git branch |
| `items`        | []string   | `omitempty,dive,required`          | IDs of artifacts in this shipment |
| `created_at`   | time.Time  | `required`                         | |
| `updated_at`   | time.Time  | `required`                         | |

**Item association model:** Shipment is an aggregate root holding an `items`
list of artifact IDs in its frontmatter. Items do not back-reference the
shipment; membership is owned by the shipment artifact.

**Template sections:** `## Description`, `## Items`, `## Blocked Returns`.

**Blocked-item return:** Removing an item from `items`, setting the item's
status to `blocked`, and appending a blocked-reason field to the item's
frontmatter. This is an explicit core operation, not a side effect.

## Implementation Units

### Unit 1: Add shipment defaults and metadata plumbing

**Files:** `internal/config/defaults.go`, `internal/config/config_test.go`, `internal/config/defaults_headerdef_test.go`, `internal/config/defaults_templates_test.go`, `internal/core/metadata_catalog.go`, `internal/core/metadata_catalog_test.go`
**Test files:** `internal/config/config_test.go`, `internal/config/defaults_headerdef_test.go`, `internal/config/defaults_templates_test.go`, `internal/core/metadata_catalog_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/config/defaults.go`, `internal/core/metadata_catalog.go`
**Dependencies:** none

**Approach:**

Add a new `shipment` artifact type to the default workspace model, including its
prefix, queue placement, default template, and agent-facing metadata. Keep the
existing prefix-based ID scheme for this refactor so shipment can land without
waiting on the separate naming redesign. Update the metadata catalog so agents
can discover shipment support and the new stash path once later units migrate
stash storage.

**Verification:**

A freshly initialized workspace includes shipment defaults and exposes shipment
through metadata catalog output. Existing metadata tests continue to pass with
shipment added to the type list.

### Unit 2: Implement shipment domain behavior in core and read model

**Files:** `internal/core/artifacts.go`, `internal/core/queue.go`, `internal/core/shipment.go`, `internal/db/schema.go`, `internal/db/queries.go`, `internal/db/rehydration.go`, `internal/models/artifact.go`
**Test files:** `internal/core/shipment_test.go`, `internal/core/queue_test.go`, `internal/db/queries_expansion_test.go`, `internal/db/rehydration_expansion_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/core/templates/deliberation.go`, `internal/core/stash.go`, `internal/db/rehydration.go`
**Dependencies:** Unit 1

**Approach:**

Introduce a shipment service that can create a shipment artifact, associate
queued items with it, move shipment status forward, and return blocked items to
backlog state with a reason. Keep the implementation CQRS-friendly: Markdown
remains the source of truth, while the database indexes shipment membership and
status for agent queries. Avoid hidden side effects by making blocked-item
return an explicit core operation with a clear verification path.

Define shipment sentinel errors in `internal/errors/errors.go` before
implementation: `ErrShipmentNotFound`, `ErrItemAlreadyAssigned`,
`ErrShipmentConflict`, `ErrCannotReturnItem`. All errors must support
`errors.Is` wrapping.

Emit structured `slog` entries for shipment creation (Info), item association
(Debug), status transitions (Info), and blocked-item returns (Warn on failure,
Info on success). Emit `events.jsonl` records for `shipment_created`,
`shipment_status_changed`, `item_added_to_shipment`, and
`item_returned_from_shipment`.

**Verification:**

Creating a shipment records its membership in both the queue artifact and the
read model. Returning an item from shipment marks the item blocked, removes it
from shipment membership, and keeps the remaining shipment queryable. Sentinel
error tests verify `errors.Is` compatibility for all shipment error types.

### Unit 3: Expose shipment through CLI and MCP

**Files:** `internal/cli/root.go`, `internal/cli/shipment.go`, `internal/cli/shipment_test.go`, `internal/mcp/tools.go`, `tests/contract/shipment_tools_test.go`
**Test files:** `internal/cli/shipment_test.go`, `tests/contract/shipment_tools_test.go`, `tests/contract/tools_expansion_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/cli/deliberate.go`, `internal/cli/queue_cmd.go`, `internal/mcp/tools.go`
**Dependencies:** Unit 2

**Approach:**

Add the minimum shipment surface needed by the two-agent harness: create,
get/list, claim, and return-blocked. Keep the tool surface unconditional, with
workspace-state errors returned descriptively, in line with the repository's MCP
rules. The CLI and MCP contracts should share the same underlying core service
so behavior does not fork.

Emit `slog.Info` on each CLI command invocation and MCP tool call with the
operation name and shipment ID. MCP tools must return descriptive errors
(not hidden tools) when workspace is uninitialized.

**Verification:**

CLI shipment commands succeed against a configured workspace and fail
predictably when inputs are invalid. Contract tests cover the MCP schemas and
key success and error responses. Pre-init tool calls return descriptive errors.

### Unit 4: Migrate stash from Markdown to JSONL

**Files:** `internal/stash/stash.go`, `internal/stash/stash_test.go`, `internal/core/stash.go`, `internal/db/stash.go`, `internal/db/rehydration.go`, `internal/core/metadata_catalog.go`, `internal/config/defaults.go`, `tests/integration/migration_test.go`
**Test files:** `internal/stash/stash_test.go`, `internal/core/stash_test.go`, `internal/db/rehydration_expansion_test.go`, `tests/integration/migration_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/db/stash.go`, `internal/core/stash.go`, `internal/db/rehydration.go`
**Dependencies:** Unit 1, Unit 2

**Approach:**

Change the stash storage to `.backlogit/stash.jsonl`, treating it as a JSONL
queue per Constitution v2.1.0 layer 4: Git-tracked, append-friendly, and
machine-parseable transient data. Stash entries are not sources of truth; they
graduate into Markdown artifacts through deliberation. Update parsing, writing,
metadata, and rehydration logic together. Since the only workspace to migrate is
this repository's own checked-in `.backlogit/queue/.stash.md`, the migration
path is a one-time conversion with no external compatibility burden.

Emit `slog.Info` for migration start and completion with entry counts, and
`slog.Warn` for entries that require fixup during conversion.

**Verification:**

Tests written before implementation must cover: (a) round-trip JSONL
serialization and deserialization of stash entries preserving IDs, priorities,
kinds, and linked deliberation IDs, (b) migration from `.stash.md` format to
JSONL with entry-count validation before and after, (c) partial-write recovery
via atomic temp-file-then-rename, (d) rehydration indexes the JSONL stash
correctly, and (e) metadata surfaces report the new path.

### Unit 5: Create the groomer agent and extract harvest as a skill

**Files:** `.github/agents/groomer.agent.md`, `.github/skills/harvest/SKILL.md`, `.github/agents/backlog-harvester.agent.md`, `.github/agents/deliberator.agent.md`
**Test files:** none, documentation-only unit
**Effort size:** medium
**Skill domain:** docs
**Execution note:** characterization-first
**Patterns to follow:** `.github/agents/deliberator.agent.md`, `.github/skills/impl-plan/SKILL.md`, `.github/skills/deliberate/SKILL.md`
**Dependencies:** Unit 3, Unit 4

**Approach:**

Create a sidecar `groomer` agent that owns stash triage, deliberation handoff,
planning, review gating, and harvest orchestration. Extract the current harvest
phase from `backlog-harvester` into a reusable skill so grooming logic is no
longer locked inside a full agent. Update the current harvester and deliberator
instructions to point at the new control flow without removing them yet.

**Verification:**

The new groomer instructions define an end-to-end stash-to-backlog workflow,
reference the shipment-aware backlog model, and leave the current agents in a
clearly deprecated but still understandable state.

### Unit 6: Create the shipper agent and extract shipper leaf skills

**Files:** `.github/agents/shipper.agent.md`, `.github/skills/harness-architect/SKILL.md`, `.github/skills/pr-lifecycle/SKILL.md`, `.github/agents/harness-architect.agent.md`, `.github/agents/build-orchestrator.agent.md`, `.github/agents/pr-review.agent.md`
**Test files:** none, documentation-only unit
**Effort size:** medium
**Skill domain:** docs
**Execution note:** characterization-first
**Patterns to follow:** `.github/agents/build-orchestrator.agent.md`, `.github/agents/pr-review.agent.md`, `.github/skills/build-feature/SKILL.md`, `.github/skills/review/SKILL.md`
**Dependencies:** Unit 3

**Approach:**

Create a sidecar `shipper` agent that takes a shipment from queued work through
harness generation, build execution, review, CI remediation, and PR lifecycle.
Extract the current harness-generation and PR-lifecycle responsibilities into
leaf skills so shipper composes them rather than nesting more agents. Preserve
explicit user merge approval in the instructions.

**Verification:**

The shipper documentation defines shipment entry and exit criteria, references
the new shipment commands, and leaves the current `build-orchestrator`,
`harness-architect`, and `pr-review` surfaces deprecated but still navigable.

### Unit 7: Refresh durable workflow and policy documentation

**Files:** `AGENTS.md`, `.github/copilot-instructions.md`, `.github/policies/workflow-policies.md`, `docs/workflow.md`, `docs/rationale.md`
**Test files:** none, documentation-only unit
**Effort size:** small
**Skill domain:** docs
**Execution note:** characterization-first
**Patterns to follow:** `AGENTS.md`, `docs/workflow.md`
**Dependencies:** Unit 5, Unit 6

**Approach:**

Update the durable workflow map so repository instructions, policies, and user
facing docs stop describing the retired multi-agent path as the primary model.
Use progressive disclosure: keep `AGENTS.md` brief, move durable rationale into
`docs/`, and document the sidecar migration path clearly so future sessions do
not revive stale agent topology by accident.

**Verification:**

The top-level workflow documentation consistently describes the groomer and
shipper path, and policy references no longer assume `build-orchestrator` is the
primary implementation orchestrator.

### Unit 8: Align the checked-in self-hosted workspace and add end-to-end coverage

**Files:** `.backlogit/config.yaml`, `.backlogit/header-def.yaml`, `.backlogit/templates/shipment.md`, `.backlogit/stash.jsonl`, `tests/contract/queue_tools_test.go`, `tests/integration/workflow_test.go`, `tests/integration/migration_test.go`
**Test files:** `tests/contract/queue_tools_test.go`, `tests/integration/workflow_test.go`, `tests/integration/migration_test.go`
**Effort size:** medium
**Skill domain:** config
**Execution note:** test-first
**Patterns to follow:** `.backlogit/templates/deliberation.md`, `.backlogit/config.yaml`, `tests/integration/workflow_test.go`
**Dependencies:** Unit 1, Unit 3, Unit 4, Unit 7

**Approach:**

Dogfood the new workflow in the repository's own checked-in workspace after the
product code and docs are ready. Add the shipment template and config to the
repo workspace, migrate the checked-in stash data to JSONL, and extend the
integration tests to cover the core story: stash entry, linked deliberation,
shipment creation, and blocked-item return. Prefer CLI- or migration-driven
workspace updates where tooling exists, because the repository has already
identified direct `.backlogit/` writes as a hygiene risk.

**Verification:**

Tests written before workspace changes must cover: (a) stash entry round-trip
through JSONL in the live workspace, (b) shipment creation from queued backlog
items using CLI, (c) blocked-item return restores item to backlog state,
(d) rehydration produces consistent index from the updated workspace. The
repository's checked-in workspace matches the new product defaults, and the
integration suite exercises the new two-agent data path with the real workspace
fixtures.

## Dependency Graph

```text
Unit 1 -> Unit 2 -> Unit 3
Unit 1 -> Unit 2 -> Unit 4
Unit 3 + Unit 4 -> Unit 5
Unit 3 -> Unit 6
Unit 5 + Unit 6 -> Unit 7
Unit 1 + Unit 3 + Unit 4 + Unit 7 -> Unit 8
```

### Suggested Delivery Slices

* Slice 1: Units 1 and 2. Establish shipment as a product concept and the core
  service that Units 3 and 4 both depend on.
* Slice 2: Units 3 and 4. Make shipment and stash JSONL externally usable.
  Unit 4 coordinates its rehydration changes against Unit 2's schema.
* Slice 3: Units 5, 6, and 7. Add sidecar agents and update durable workflow
  documentation. Gate: Unit 3 contract tests must pass before agent authoring.
* Slice 4: Unit 8. Dogfood the repository workspace and prove the flow with
  contract and integration tests.

## Decisions

| #  | Decision                                                           | Rationale                                                                 | Alternatives Rejected |
|----|--------------------------------------------------------------------|---------------------------------------------------------------------------|-----------------------|
| D1 | Use a sidecar bootstrap                                            | It lets backlogit self-host the migration gradually and keeps rollback simple | Big-bang replacement, fully manual ad hoc rewrite |
| D2 | Keep the filename redesign out of scope for this plan              | It is substantial on its own and would blur failure causes during rollout | Bundling both refactors in one shipment |
| D3 | Introduce `shipment` within the current prefix-based naming model  | The agent workflow can land now without waiting for a second storage redesign | Blocking shipment on hierarchical IDs |
| D4 | Store stash as JSONL queue (CQRS layer 4)                          | Constitution v2.1.0 defines JSONL queues as the format for transient intake data; stash entries are transient ideas that graduate into Markdown artifacts through deliberation | Keeping `.stash.md` indefinitely |
| D5 | Preserve user-confirmed merge approval                             | The new workflow is still earning trust and should not auto-merge by default | Immediate auto-merge support |

## Risks and Caveats

* Shipment touches multiple layers at once: defaults, core behavior, CLI, MCP,
  and integration fixtures. Test-first sequencing matters.
* Stash migration can create split-brain behavior if `.stash.md` and
  `stash.jsonl` are both writable for too long. The migration bridge should be
  short-lived.
* The harness docs currently contain many direct references to legacy agent
  names. Missing even one of those leaves future sessions with stale routing
  guidance.
* The repository has an open desire to prevent direct agent writes into
  `.backlogit/`. Implementation should prefer CLI- or migration-mediated writes
  whenever practical.

## Learnings Applied

* Deliberation is already a first-class backlogit workflow. The refactor should
  build from `DL001` and the existing `backlogit deliberate` flow rather than
  inventing a new planning entry point.
* Archived artifacts already disappear from active queue queries. Shipment and
  blocked-item flows should reuse that visibility model instead of inventing a
  second archive concept.
* Durable knowledge belongs in `docs/`, while review artifacts stay short-lived
  in `.backlogit/queue/`. The workflow refresh should preserve that split.
* Repository validation expects `go test ./...`, `go vet ./...`, and
  `golangci-lint run`.

## Standards Check

* Go implementation units stay in typed structs and existing core packages.
  Public-facing additions require GoDoc comments and typed validation where
  fields cross package boundaries.
* CLI and MCP additions must be paired so the agent and developer surfaces stay
  aligned.
* Markdown artifacts must include frontmatter and follow the repository's
  progressive-disclosure documentation rules.
* Repository workspace changes under `.backlogit/` should prefer tool-mediated
  updates over ad hoc direct editing when the product surface can support it.

## Constitution Check (v2.1.0)

| Principle                        | Compliance | Notes |
|----------------------------------|------------|-------|
| I. Type-Safe Go                  | Yes        | Shipment struct defined with validator tags and JSON/YAML annotations in the schema contract above; sentinel errors in `internal/errors/` |
| II. MCP Protocol Fidelity        | Yes        | Shipment tools unconditionally discoverable; pre-init calls return descriptive errors per Unit 3 spec |
| III. Test-First Development      | Yes        | All code-bearing units specify test-first execution with named test files and measurable verification criteria |
| IV. Workspace Containment        | Yes        | All new storage remains inside `.backlogit/`; no external migration scenarios |
| V. Structured Observability      | Yes        | Units 2, 3, and 4 specify `slog` entries and `events.jsonl` schemas for significant operations |
| VI. Single-Binary Simplicity     | Yes        | No new runtime services or databases |
| VII. CQRS Data Architecture      | Yes        | Markdown (layer 1) for durable artifacts; SQLite (layer 2) as ephemeral cache rebuildable from Markdown + JSONL queues; JSONL streams (layer 3) for event history; stash.jsonl (layer 4) as transient intake queue graduating into artifacts through deliberation |
| VIII. Git-Friendly Persistence   | Yes        | Shipment manifests as Markdown with YAML frontmatter; stash as JSONL (line-oriented, Git-mergeable); atomic temp-file-then-rename writes |
| IX. Agent Context Efficiency     | Yes        | Shipment and stash metadata give agents narrow, queryable workflow boundaries via MCP tools |

## Recommended Next Step

Harvest the approved slices into backlogit work items and start with Slice 1.
