---
title: "Artifact Identity, Hierarchy & Relationships"
date: 2026-04-07
origin: ".backlogit/queue/DL003.md"
status: reviewed
---

## Problem Frame

Six high-priority stash entries converge on a single design surface: how backlogit
artifacts are named, parented, linked, and reconciled across their lifecycle.

The current prefix-based ID system (`F015.T009.ST001`) couples artifact identity
to type classification. A type reclassification (features → epics, for example)
would require renaming every artifact ID, all log references, and all DB rows. The
operator requires a numeric hierarchy system where structure is decoupled from
classification, so type changes become configuration changes rather than ID
migrations.

Additional gaps include: bugs have no valid parent path in config, relationships
are limited to `item_deps` and ad-hoc custom fields, status cascade between parent
and child is advisory rather than enforced, and orphaned items from partial
shipment releases lack adoption operations.

### Scope Boundary

This plan covers the five streams defined in DL003's chosen direction. It does NOT
cover: stash harvest race condition hardening (stash 44E3C9D4), auto-archiving
policy (stash 93A77D46), or operational improvements (stash 834CCDB7, 60EF697D).

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Replace prefix IDs with numeric hierarchy + type suffix | Stash 3C7BCC11, Operator direction |
| R2 | File names match artifact IDs; JSONL logs share root filename | Stash 3C7BCC11 |
| R3 | Migrate all existing artifacts, DB refs, and JSONL files | Operator: "migration scripts acceptable" |
| R4 | Bug parenting: default level 3, configurable to 2 or 3 | Stash BA3DB37B, Operator: "mirrors Azure DevOps" |
| R5 | Typed relationship links via `item_links` table | Stash AA10AF37, 6A545842 |
| R6 | Migrate custom_fields (spike_ref, source_stash_id) to item_links | Operator: "durable domain metadata" |
| R7 | Blocking bidirectional status cascade | Stash CE39AE5D, Operator: "advisory swallows work" |
| R8 | Orphan lifecycle: null parent_id, adopt/reparent, queue indicator | Stash 51B11D29, DL002 |
| R9 | Preserve provenance IDs on adoption | Operator direction |

## Scope Boundaries

### In Scope

* Numeric hierarchy ID generation (`001-F`, `001.001-T`, `001.001.001-ST`)
* Config schema changes for type suffix, bug level placement
* `item_links` DB table and MCP/CLI surface
* Blocking status cascade in both directions
* Orphan detection, adoption/reparent operations
* Migration scripts for existing 193 artifacts and 50 JSONL logs
* Rehydration updates for new ID format

### Non-Goals

* External system integration (Jira, ADO sync)
* Stash harvest race condition hardening (separate stash entry 44E3C9D4)
* Auto-archive policy changes
* UI or dashboard rendering

### Deferred to Implementation

* Exact zero-padding width (3 digits assumed; implementer may adjust if workspace exceeds 999 items at any level)
* Migration script idempotency strategy (implementer decides: checksums, marker files, or dry-run flags)
* Whether `isHierarchicalID` in rehydration.go needs to recognize the new type suffix format

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units that modify
fewer than 3 files, touch fewer than 5 functions, and cover fewer than 4 test scenarios
stay within the "small" effort size. Larger units are split.

### Stream 1: Naming Overhaul

#### Unit 1A: Config Schema for Numeric IDs

**Files:** `internal/config/schema.go`, `.backlogit/config.yaml`, `.backlogit/header-def.yaml`
**Test files:** `internal/config/config_test.go`, `internal/config/defaults_headerdef_test.go`
**Effort size:** small
**Skill domain:** config
**Execution note:** test-first — write tests for new config fields before modifying structs

**Approach:**

Add a `Suffix` field to `ArtifactTypeConfig` alongside the existing `Prefix` field.
The suffix is the type indicator appended after the numeric segment (e.g., `-F`,
`-T`, `-ST`, `-B`). Update `config.yaml` to replace `name_format: "{prefix}{NNN}"`
with a numeric format using suffixes. Update `header-def.yaml` `id_format` fields.
Keep the `Prefix` field for backward compatibility during migration; mark it
deprecated with a config migration path.

Add a `BugLevel` field (int, default 3, valid 2 or 3) to `WorkspaceConfig` or
`QueueLayoutConfig` for Stream 2 consumption. This field controls which hierarchy
level bugs occupy and which parent types accept bugs as children.

Config changes:

```yaml
artifact_types:
  feature:
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
  task:
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: [subtask]
  subtask:
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
  bug:
    suffix: "-B"
    name_format: "{NNN}{suffix}"
    # allowed_children empty; bug_level config drives parenting
  review:
    suffix: "-R"
    name_format: "{NNN}{suffix}"
    file_name_format: "{id}-{title_slug}"
  deliberation:
    suffix: "-DL"
    name_format: "{NNN}{suffix}"
  shipment:
    suffix: "-S"
    name_format: "{NNN}{suffix}"

bug_level: 3  # 2 = child of feature, 3 = child of task
```

**Patterns to follow:** Existing `ArtifactTypeConfig` struct with validator tags
(`internal/config/schema.go:21-26`)

**Dependencies:** None — this is the foundation unit.

**Verification:**

* Config loads with new suffix fields; validator accepts valid suffix values
* Config rejects missing suffix when name_format contains `{suffix}`
* Bug level validation: accepts 2 or 3, rejects other values
* Existing tests continue passing with updated config fixtures

#### Unit 1B: ID Generation Rewrite

**Files:** `internal/core/hierarchy.go`, `internal/core/naming.go`
**Test files:** `internal/core/hierarchy_test.go`, `internal/core/naming_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write tests for new ID format before changing generation logic

**Approach:**

Rewrite `NextTypedHierarchicalID` to produce numeric-suffix IDs: the numeric
segment is zero-padded (3 digits default), the suffix comes from
`ArtifactTypeConfig.Suffix`. For a root-level feature: `001-F`. For a task under
feature 001: `001.001-T`. For a subtask under that task: `001.001.001-ST`.

The ordinal counter queries siblings by parent_id and extracts the numeric prefix
from the last segment (strip the suffix to get the ordinal). Update `formatTypedSegment`
to use the suffix instead of the prefix. Update `typedSegmentOrdinal` to parse
the numeric portion before the suffix dash.

Update `ResolveName` to support the new `{NNN}{suffix}` template pattern. Update
`ResolveFileName` since file names match artifact IDs in the new scheme.

Key ID format: `{parent_numeric}.{NNN}{suffix}` where NNN is the zero-padded
sibling ordinal and suffix is the type indicator.

Update `ParseHierarchicalID` to handle the suffix: strip the suffix from each
segment before parsing the numeric value. Update `isHierarchicalID` in
`internal/db/rehydration.go` to recognize the new format (segments may end with
a dash-alpha suffix).

Update `FormatHierarchicalID` to accept the suffix parameter.

**Patterns to follow:** Existing `NextTypedHierarchicalID` at `hierarchy.go:64-119`;
existing `ResolveName` at `naming.go:28-35`

**Dependencies:** Unit 1A (config schema)

**Verification:**

* `NextTypedHierarchicalID` generates `001-F` for root feature, `001.001-T` for child task
* Sibling ordinals increment correctly: `001.001-T`, `001.002-T`, `001.003-T`
* `ParseHierarchicalID("001.002-T")` returns `[1, 2]`
* `ResolveFileName` returns the artifact ID as filename for standard types

#### Unit 1C: CreateArtifact Integration

**Files:** `internal/core/artifacts.go`
**Test files:** `internal/core/artifacts_expansion_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

Update `CreateArtifact` (line 94) to use the rewritten `NextTypedHierarchicalID`
with suffix-based config. The `artifactID` assignment at line 125 already uses the
return value; the change is in the arguments passed (suffix instead of prefix).

Update `validateArtifactParent` (line 253) to enforce parenting for ALL types with
configured parent relationships, not just reviews. The current guard (`if
artifactType != "review" { return nil }`) is too narrow. Replace with a lookup
into config: if the type has no allowed parents at its level, skip validation;
otherwise, verify the parent type is allowed to have this child type.

Update `WriteArtifactFile` and `findArtifact` to work with the new filename =
artifact ID convention.

**Patterns to follow:** Existing `CreateArtifact` pattern at `artifacts.go:94-160`

**Dependencies:** Unit 1B (ID generation)

**Verification:**

* Creating a feature generates `001-F` file in queue directory
* Creating a task under feature `001-F` generates `001.001-T`
* Creating a bug under task `001.001-T` generates `001.001.001-B`
* `validateArtifactParent` rejects bug under feature when `bug_level: 3`

#### Unit 1D: Rehydration and DB Schema Updates

**Files:** `internal/db/rehydration.go`, `internal/db/schema.go`
**Test files:** `tests/integration/` (new rehydration test)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first — run existing rehydration tests, then update

**Approach:**

Update `isHierarchicalID` to recognize the new suffix format. A segment like
`001-F` is hierarchical (numeric prefix followed by optional dash-alpha suffix).
Update `hierarchyPathFromID` to strip suffixes before building ancestor paths.

Update `EnsureSchema` to add the `item_links` table (for Stream 3) in the same
schema migration batch. This is forward-compatible and avoids a second schema
migration.

Ensure rehydration correctly derives `level` and `hierarchy_path` from the new
ID format during index rebuild.

**Patterns to follow:** Existing `isHierarchicalID` at `rehydration.go:139-157`;
existing schema migration pattern at `schema.go:162-174`

**Dependencies:** Unit 1B (ID format)

**Verification:**

* `isHierarchicalID("001-F")` returns true
* `isHierarchicalID("001.002-T")` returns true
* `isHierarchicalID("F015")` returns false (legacy format)
* `hierarchyPathFromID("001.002.003-ST")` returns `001-F/001.002-T/001.002.003-ST`
  (or numeric-only path; implementer decides based on parent lookup feasibility)
* Rehydration rebuilds level and hierarchy_path correctly for new IDs

#### Unit 1E: MCP and CLI ID Consumers

**Files:** `internal/mcp/tools.go`, `internal/cli/` (affected commands)
**Test files:** `tests/contract/` (MCP tool contract tests)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first — run existing contract tests, then update

**Approach:**

Audit all MCP tool handlers that accept or return artifact IDs. The tool handlers
are ID-agnostic (they pass IDs through to core functions), so changes should be
minimal. Verify that:

* `handleCreateItem` correctly flows the new ID format
* `handleGetItem`, `handleMoveItem`, `handleUpdateItem` accept new format IDs
* `handleAddDependency`, `handleRemoveDependency`, `handleGetDependencies` work
  with new IDs in `item_deps` table
* `handleQuerySQL` returns new-format IDs from the index

Update CLI commands if any have hardcoded ID pattern assumptions.

**Patterns to follow:** Existing MCP tool handler pattern at `tools.go:810-835`

**Dependencies:** Units 1B, 1C, 1D

**Verification:**

* Contract tests pass with new ID format
* MCP tool round-trip: create → get → move → update produces consistent IDs
* SQL query results contain new-format IDs after rehydration

#### Unit 1F: Migration Script for Existing Artifacts

**Files:** `scripts/migrate-ids.go` (new), `scripts/migrate-ids_test.go` (new)
**Test files:** `scripts/migrate-ids_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** migration-first — build mapping table, then apply

**Approach:**

Create a standalone Go migration script (`go run scripts/migrate-ids.go`) that:

1. Scans `.backlogit/archive/` and `.backlogit/queue/` for all `.md` files
2. Builds an old-ID → new-ID mapping by reading frontmatter and computing the
   new numeric hierarchy ID from the parent-child structure
3. For each artifact:
   a. Updates the `id` field in YAML frontmatter
   b. Updates `parent_id` to the new parent ID
   c. Updates `dependencies` array entries to new IDs
   d. Renames the `.md` file to match the new ID
4. For each JSONL log in `.backlogit/logs/`:
   a. Updates `item_id` fields in each JSON line
   b. Renames the `.jsonl` file to match the new artifact ID
5. Updates references in `stash_links`, `item_deps`, `commit_links` table entries
   within the JSONL event streams
6. Produces a `migration-report.json` summarizing: items migrated, old→new ID map,
   errors encountered

The script must be **idempotent**: running it twice produces the same result.
Detect already-migrated artifacts by checking if the ID matches the new format.

**Migration ordering:** Process level-1 artifacts first (features, deliberations,
shipments), then level-2 (tasks, reviews), then level-3 (subtasks, bugs). This
ensures parent IDs are resolved before children are processed.

**Artifact count:** ~193 .md files (182 archived, 5 active, 6 templates), ~50 JSONL
log files.

**Patterns to follow:** Atomic file operations pattern from `artifacts.go:241-248`
(temp file + rename)

**Dependencies:** Units 1A, 1B (needs the new ID generation logic)

**Verification:**

* Dry-run mode lists all renames without executing
* Test with a temporary copy of `.backlogit/` workspace
* All old IDs in frontmatter are replaced with new format
* All JSONL log filenames match their artifact's new ID
* Parent-child relationships preserved across the rename
* Running twice produces no changes (idempotent)

### Stream 2: Bug Parenting

#### Unit 2A: Dynamic Bug Level Configuration

**Files:** `internal/config/schema.go`, `internal/core/artifacts.go`, `.backlogit/config.yaml`
**Test files:** `internal/core/artifacts_expansion_test.go`, `internal/config/config_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

Read the `bug_level` config field added in Unit 1A. At artifact creation time:

* If `bug_level: 3` (default): bug requires a task parent. Add `bug` to `task.allowed_children` in config.
* If `bug_level: 2`: bug requires a feature parent. Add `bug` to `feature.allowed_children` in config.

Rather than hardcoding `allowed_children` in YAML, compute it dynamically at config
load time based on `bug_level`. The `allowed_children` in YAML remains the base set;
bug placement is added programmatically.

Update `validateArtifactParent` (already generalized in Unit 1C) to check the
dynamically computed allowed children list.

Update `QueueLayoutConfig.Levels` to place bug at the correct level based on config.

**Patterns to follow:** Existing `LevelForType` at `hierarchy.go:144-153`

**Dependencies:** Units 1A, 1C (generalized parent validation)

**Verification:**

* `bug_level: 3` → creating bug under task succeeds, under feature fails
* `bug_level: 2` → creating bug under feature succeeds, under task fails
* `LevelForType` returns correct level for bug based on config
* Config validation rejects `bug_level: 1` or `bug_level: 4`

### Stream 3: Typed Relationship Links

#### Unit 3A: item_links DB Table and Core Functions

**Files:** `internal/db/schema.go`, `internal/db/queries.go` (or new `internal/db/links.go`)
**Test files:** `internal/db/` (new link query tests)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

The `item_links` table was forward-declared in Unit 1D's schema migration. This
unit adds the query functions:

```sql
CREATE TABLE IF NOT EXISTS item_links (
    source_id  TEXT NOT NULL,
    target_id  TEXT NOT NULL,
    link_type  TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_id, target_id, link_type)
);
CREATE INDEX IF NOT EXISTS idx_item_links_source ON item_links(source_id);
CREATE INDEX IF NOT EXISTS idx_item_links_target ON item_links(target_id);
CREATE INDEX IF NOT EXISTS idx_item_links_type   ON item_links(link_type);
```

Valid `link_type` values: `related_to`, `duplicate_of`, `informs`, `supersedes`,
`spike_ref`.

Core functions:

* `AddLink(ctx, db, sourceID, targetID, linkType) error` — validates link_type enum, inserts
* `RemoveLink(ctx, db, sourceID, targetID, linkType) error` — deletes
* `GetLinks(ctx, db, itemID) ([]LinkEdge, error)` — returns all links where item is source or target
* `GetLinksByType(ctx, db, itemID, linkType) ([]LinkEdge, error)` — filtered

**Patterns to follow:** Existing `AddDependencyChecked` in `internal/db/` for dep
edge management

**Dependencies:** Unit 1D (schema includes table creation)

**Verification:**

* AddLink creates a row; duplicate insert is idempotent
* RemoveLink deletes the specific edge
* GetLinks returns bidirectional results
* Invalid link_type is rejected with descriptive error

#### Unit 3B: MCP and CLI Link Tools

**Files:** `internal/mcp/tools.go`, `internal/cli/` (new link subcommand)
**Test files:** `tests/contract/` (new link tool contract tests)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

Register two new MCP tools:

* `backlogit_add_link` — parameters: `source_id` (required), `target_id` (required), `link_type` (required, enum)
* `backlogit_get_links` — parameters: `id` (required), `link_type` (optional filter)

Add CLI subcommand: `backlogit link add <source> <target> --type <link_type>` and
`backlogit link list <id> [--type <link_type>]`.

Both tools follow the five-step handler pattern from the MCP conventions.

**Patterns to follow:** Existing `handleAddDependency` at `tools.go:810-835`

**Dependencies:** Unit 3A (core link functions)

**Verification:**

* MCP tool: add link, then get links returns it
* CLI: `backlogit link add 001-F 002-F --type related_to` succeeds
* Contract test validates schema and error codes

#### Unit 3C: Migrate Custom Fields to Item Links

**Files:** `scripts/migrate-custom-fields.go` (new)
**Test files:** `scripts/migrate-custom-fields_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** migration-first

**Approach:**

Create a migration script that:

1. Scans all artifacts for `custom_fields` containing `spike_ref` or `source_stash_id`
2. For each match, creates an `item_links` row:
   * `spike_ref` → link_type `spike_ref`, target = the referenced deliberation/spike ID
   * `source_stash_id` → link_type `informs`, target = the stash-linked item
3. Removes the migrated fields from `custom_fields` in the artifact frontmatter
4. Produces a migration report

**Dependencies:** Units 3A, 1F (run after ID migration)

**Verification:**

* Artifacts with spike_ref custom_field gain an item_links row
* custom_fields map no longer contains migrated keys
* Idempotent: running twice produces no changes

### Stream 4: Blocking Status Reconciliation

#### Unit 4A: Blocking Cascade Logic

**Files:** `internal/core/harness_status.go`, `internal/core/shipment_lifecycle.go`
**Test files:** `internal/core/harness_status_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — write failing tests for blocking behavior

**Approach:**

The existing `ComputeParentStatus` (harness_status.go:48-90) already handles the
"all children done → parent done" direction. The missing piece is the **blocking
direction**: when a parent is moved to `done`, verify all children are in terminal
status first.

Modify `setArtifactStatus` (shipment_lifecycle.go:361-385) to check children
before allowing a parent to transition to `done`:

1. Query `SELECT status FROM items WHERE parent_id = ?`
2. If any child status is NOT in terminal set (`done`, `accepted`, `archived`,
   `shipped`, `abandoned`), return a blocking error
3. The error message lists the non-terminal children by ID and status

This makes the cascade **bidirectional and blocking**:

* **Upward (existing):** All children done → parent auto-transitions to done
* **Downward (new):** Parent to done → blocked if children are not all terminal

The `cascadePersistedParentStatuses` function continues to handle the upward
cascade. The blocking check is a new pre-condition in `setArtifactStatus`.

Exempt shipment release operations from the blocking check, since `ReleaseShipment`
explicitly marks items done as part of the release flow.

**Patterns to follow:** Existing `ComputeParentStatus` at `harness_status.go:48-90`

**Dependencies:** Units 1C (generalized parent validation), 2A (bug parenting)

**Verification:**

* Moving parent to done with active children returns blocking error
* Moving parent to done with all children done succeeds
* Child completion triggers upward cascade (existing behavior preserved)
* Shipment release bypasses the blocking check
* Error message includes IDs and statuses of blocking children

#### Unit 4B: MCP Tool Update for Blocking Errors

**Files:** `internal/mcp/tools.go`
**Test files:** `tests/contract/` (update move_item contract tests)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

Update `handleMoveItem` to surface the blocking error as a structured JSON
response rather than a generic error. The agent consuming the MCP tool needs to
understand that children are blocking the transition and which children need
attention.

Return format:

```json
{
  "error": "status_blocked_by_children",
  "message": "Cannot move to done: 2 children not in terminal status",
  "blocking_children": [
    {"id": "001.001-T", "status": "active"},
    {"id": "001.002-T", "status": "queued"}
  ]
}
```

**Dependencies:** Unit 4A (blocking logic)

**Verification:**

* MCP tool returns structured blocking error with child details
* Non-blocking moves return success as before
* Contract test validates the error schema

### Stream 5: Orphan Lifecycle

#### Unit 5A: Adopt/Reparent Operation

**Files:** `internal/core/artifacts.go` (or new `internal/core/orphan.go`), `internal/core/shipment_lifecycle.go`
**Test files:** `internal/core/` (new orphan test file)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

`clearParentID` already exists at `shipment_lifecycle.go:422-425` and is called
during `returnUnreleasedFeatureItems`. This unit adds the **adopt** operation:

* `AdoptArtifact(ctx, ws, itemID, newParentID) error`
  * Validates the new parent exists and the child type is allowed under the
    parent type (using generalized `validateArtifactParent` from Unit 1C)
  * Sets `parent_id` in frontmatter and DB
  * **Preserves the original hierarchical ID** (provenance); does NOT rename
  * Appends an `adopted` event to the item's JSONL log with `new_parent_id`
  * Triggers upward status cascade to the new parent

The provenance ID (e.g., `001.002-T` from the old feature) remains as the
artifact's immutable identifier even though its parent is now a different feature.
This is the operator's explicit decision.

**Patterns to follow:** Existing `clearParentID` at `shipment_lifecycle.go:422-440`

**Dependencies:** Units 1C, 2A, 4A (parenting validation and status cascade)

**Verification:**

* Orphan (parent_id = null) can be adopted by a new feature
* Adoption preserves original ID
* Event log records adoption with old and new parent context
* Status cascade fires after adoption

#### Unit 5B: MCP/CLI Orphan Tools and Queue Indicator

**Files:** `internal/mcp/tools.go`, `internal/core/queue.go`
**Test files:** `tests/contract/` (new adopt tool contract test), `internal/core/queue_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first

**Approach:**

Register new MCP tool:

* `backlogit_adopt_item` — parameters: `item_id` (required), `new_parent_id` (required)

Add CLI: `backlogit adopt <item_id> --parent <new_parent_id>`

Update `QueryQueue` to add an `is_orphan` indicator to queue results. An orphan
is an item with `parent_id IS NULL` and a hierarchical ID that implies it was
once parented (contains a `.` separator). Add a computed column or result field.

**Patterns to follow:** Existing `handleAddDependency` at `tools.go:810`

**Dependencies:** Unit 5A (adopt logic)

**Verification:**

* MCP tool: adopt orphan succeeds, returns updated item
* Queue view: orphans are flagged with indicator
* Contract test validates adopt tool schema

## Dependency Graph

```text
Stream 1 (Naming):
  1A (Config) → 1B (ID Gen) → 1C (CreateArtifact) → 1D (Rehydration/Schema)
                                                    → 1E (MCP/CLI)
                                                    → 1F (Migration Script)

Stream 2 (Bug Parenting):
  1A + 1C → 2A (Dynamic Bug Level)

Stream 3 (Relationships):
  1D → 3A (item_links table) → 3B (MCP/CLI tools)
  1F + 3A → 3C (Custom field migration)

Stream 4 (Status Cascade):
  1C + 2A → 4A (Blocking cascade) → 4B (MCP error format)

Stream 5 (Orphan Lifecycle):
  1C + 2A + 4A → 5A (Adopt/Reparent) → 5B (MCP/CLI + Queue indicator)
```

**Suggested shipment grouping:**

* **Shipment 1:** Units 1A → 1B → 1C → 1D → 1E → 2A → 1F (naming + parenting + migration)
* **Shipment 2:** Units 3A → 3B → 3C → 4A → 4B → 5A → 5B (relationships + cascade + orphan)

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Numeric hierarchy with type suffix (`001-F`, `001.001-T`) | Decouples structure from classification; type reclassification becomes config change not ID migration | Prefix-based IDs (operator rejected); pure numeric without suffix (loses type visibility) |
| D2 | Suffix on ArtifactTypeConfig, not separate naming config | Keeps type identity and naming co-located; single struct change | Separate NamingConfig struct (over-engineering for current scope) |
| D3 | Bug level as workspace config field, not per-artifact | Workspace-wide consistency mirrors Azure DevOps behavior; per-artifact would create hierarchy ambiguity | Per-artifact level override (too complex, inconsistent tree) |
| D4 | `item_links` as separate table, not extending `item_deps` | Clean semantic separation: deps are workflow constraints, links are informational cross-references | Extending item_deps with more dep_type values (conflates blocking deps with informational links) |
| D5 | Blocking status cascade, not advisory | Operator directive: advisory silently swallows unhandled work | Advisory with warnings (operator rejected) |
| D6 | Preserve provenance IDs on adoption | Operator directive: easier to maintain than cascading renames; immutable IDs are a constitution principle | Cascade rename on adoption (violates ID immutability principle VII) |
| D7 | Migration as standalone Go script, not built-in CLI command | One-time operation; no value in shipping migration logic in the production binary | Built-in `backlogit migrate` command (carries dead weight after migration) |
| D8 | Forward-declare item_links table in Unit 1D | Avoids second schema migration round; schema changes are idempotent | Separate schema migration in Stream 3 (two-step migration, more complex) |

## Risks and Caveats

**R1: Migration data integrity.** The migration script (Unit 1F) touches 193
artifacts and 50 JSONL files. A bug in ID mapping could corrupt parent-child
relationships. Mitigation: dry-run mode, backup before execution, migration
report with old→new mapping for manual verification.

**R2: Rehydration compatibility during transition.** Between migration and full
deployment, some artifacts may have old-format IDs while new ones use the new
format. The `isHierarchicalID` function must recognize both formats during the
transition window. Mitigation: Unit 1D explicitly handles both formats.

**R3: Event log reference integrity.** JSONL event logs reference artifact IDs
inline. The migration script updates filenames but must also update `item_id`
fields within each JSON line. Mitigation: line-by-line JSONL rewrite with
validation.

**R4: Blocking cascade could surface latent data issues.** Enabling blocking
status cascade may reveal existing artifacts where parents are "done" but children
are still active. These must be resolved before the cascade is enforced.
Mitigation: run a diagnostic query before enabling, fix any violations.

**R5: Concurrent Groomer/Shipper during migration.** The migration script modifies
files that may be read by MCP tools during an active session. Mitigation: run
migration when no agent sessions are active; document this requirement.

## Learnings Applied

No existing compound learnings in `docs/compound/` are directly applicable (the
directory is currently empty). The following institutional knowledge from the
session informs this plan:

* **Orphan identity** (repository memory): `ShipShipment` returns unreleased items
  with cleared `parent_id`; hierarchical ID prefix preserved as provenance.
  Applied in Stream 5 design.
* **CQRS four-layer architecture** (repository memory): Markdown is source of
  truth, SQLite is ephemeral cache. Migration must update both layers. Applied
  in Unit 1F design.
* **Custom fields as property bag** (operator direction): custom_fields are
  untyped; item_links provide durable domain metadata. Applied in Stream 3
  migration design.

## Standards Check

| Standard | Compliance | Notes |
|---|---|---|
| Go 1.22+ with GoDoc | ✓ | All new exported functions will have GoDoc comments |
| golangci-lint zero errors | ✓ | All units include lint verification |
| Test-first development | ✓ | Every unit specifies test-first or characterization-first execution |
| Workspace containment | ✓ | All file operations resolve within `.backlogit/` |
| CQRS layer separation | ✓ | Markdown updated first, then rehydration rebuilds cache |
| Atomic file writes | ✓ | Migration script uses temp-file-then-rename pattern |
| ID immutability | ⚠ | Stream 1 migration is a one-time exception; post-migration IDs are immutable |
| Structured logging (slog) | ✓ | Migration script and new functions use slog |
| Parameterized SQL | ✓ | All new queries use parameterized statements |
