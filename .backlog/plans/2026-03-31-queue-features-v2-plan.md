---
title: "Queue Features V2: WIT Metadata, Archive Lifecycle, Dependency Queue, and Hierarchical File Organization"
date: 2026-03-31
origin: ".backlog/queue.md"
status: draft
predecessor: ".backlog/plans/2026-03-30-queue-features-plan.md"
---

## Problem Statement

The first queue features plan (TASK-002) established CLI commands, header definitions, templates, and section-aware MCP tools. That foundation is largely implemented. The queue.md has since evolved with thirteen additional feature requirements that address the next layer of backlogit's maturity: making the WIT type system self-describing and agent-queryable, adding lifecycle management through archiving and commit tracking, enabling cross-level dependency-aware work queues, integrating workflow policy enforcement, and restructuring the file system around a hierarchical `.backlogit/queue` folder with configurable WIT-to-level mappings.

These features transform backlogit from a structured file store into an agent-native operating system where agents can autonomously discover work item types, navigate their relationships, determine what to work on next, and track work through its full lifecycle without human guidance.

## Requirements Trace

| #    | Requirement                                                           | Origin            |
|------|-----------------------------------------------------------------------|-------------------|
| R1   | DB schema auto-maps from YAML template/header-def definitions         | queue.md line 5   |
| R2   | WIT templates define required/optional fields per attribute           | queue.md line 7   |
| R3   | MCP tool returns WIT metadata including field requirements            | queue.md line 9   |
| R4   | Templates include self-descriptions: WIT, relationships, attributes   | queue.md line 11  |
| R5   | Agent-queryable WIT system: types, relationships, fields, enums       | queue.md line 13  |
| R6   | Archive command for completed work after branch merge                 | queue.md line 15  |
| R7   | Commit tracking on work items for auto-archive detection              | queue.md line 17  |
| R8   | WIT templates include directory mapping for active/archived locations  | queue.md line 19  |
| R9   | CLI tabular listing by status + full text/YAML view by ID             | queue.md line 21  |
| R10  | Cross-level dependency tracking (feature→epic→task→sub-task→bug→decision) | queue.md line 23 |
| R11  | Streaming work queue: "what to work on next" with parallel awareness  | queue.md line 32  |
| R12  | Harness status attribute for feature-level workflow policy enforcement | queue.md line 34  |
| R13  | Hierarchical `.backlogit/queue` folder with level-based naming        | queue.md line 40  |

## Scope Boundaries

### In Scope

- Dynamic DB schema generation from header-def.yaml field definitions
- Required/optional field enforcement in validation and MCP responses
- `backlogit_describe_type` MCP tool for agent WIT discovery
- Template description metadata: type description, relationship descriptions, attribute descriptions
- `backlogit archive` CLI command and `backlogit_archive_item` MCP tool
- Commit field tracking with `backlogit_check_merged` automation hook
- Directory mapping in WIT template frontmatter (`active_dir`, `archive_dir`)
- Enhanced CLI `list` with status grouping and `status` summary view
- Dependency graph model with cross-level edge tracking
- `backlogit_next_work` MCP tool for dependency-aware queue consumption
- `harness` status attribute with `ready`/`building`/`testing`/`passing`/`failing` enum
- `.backlogit/queue/` hierarchical folder with `NNN.NNN.NNN` file naming
- Configurable WIT-to-hierarchy-level mapping in config.yaml
- Migration tooling for existing `.backlogit/tasks/` → `.backlogit/queue/`

### Non-Goals

- TUI (Bubble Tea) implementation
- External system sync (Jira, Azure DevOps hooks)
- Sprint management or OKR tracking
- Git hook installation (commit tracking is read-only)
- Branch merge detection via CI (auto-archive is a manual trigger, not a webhook)

### Deferred to Implementation

- Exact `backlogit_check_merged` branch detection strategy (git log parsing vs GitHub API)
- Dependency cycle detection algorithm selection (DFS vs Kahn's)
- Whether archived items retain their SQLite index rows or are purged on archive

## Approach

Six implementation epics, ordered by dependency:

1. **Epic A: Hierarchical File Organization** — Restructure the filesystem first since every other feature depends on how files are stored and named
2. **Epic B: WIT Type System Enhancement** — Make the type system self-describing with metadata, required/optional enforcement, and agent-queryable discovery
3. **Epic C: Dependency Graph Model** — Add the relational model for cross-level dependency tracking
4. **Epic D: Archive & Lifecycle Management** — Archive commands, commit tracking, directory mapping
5. **Epic E: Work Queue System** — Dependency-aware "what to work on next" queue
6. **Epic F: Workflow Policy Integration** — Harness status attribute and policy enforcement primitives

## Key Decisions

| #  | Decision                                                                | Rationale                                                                      | Alternatives Rejected                                        |
|----|-------------------------------------------------------------------------|--------------------------------------------------------------------------------|--------------------------------------------------------------|
| D1 | Hierarchical naming uses dot-separated numerals (001.001.001)           | Filesystem-sortable, deterministic, avoids path-depth issues                  | Nested subdirectories (deep nesting), UUID-based (unsortable) |
| D2 | WIT-to-level mapping stored in config.yaml as `hierarchy_levels`        | Central config already defines type behavior; avoids a new config file        | Separate hierarchy.yaml (too many config files)              |
| D3 | Single `.backlogit/queue/` folder for all active work items             | Flat folder with hierarchical filenames simplifies glob patterns and routing  | Per-type subdirectories (current approach, doesn't scale)    |
| D4 | Dependency edges stored as a SQLite junction table `item_deps`          | Enables efficient graph queries (ancestors, descendants, ready-to-work)       | JSON arrays only (no graph queries), separate JSONL (slow)   |
| D5 | `backlogit_describe_type` returns merged header-def + template metadata | Single tool call gives agents everything they need for a WIT type             | Separate tools for fields vs sections (more round-trips)     |
| D6 | Archive moves files to `.backlogit/archive/{type}/` preserving naming   | Keeps archived items browsable and greppable; simple undo via file move       | Delete files (data loss), separate archive DB (complexity)   |
| D7 | `backlogit_next_work` uses topological sort on dependency DAG           | Returns work items whose dependencies are all `done`/`accepted`               | Simple priority sort (ignores dependencies)                  |
| D8 | Folder named `.backlogit/queue` per user revision                       | User explicitly requested `queue` instead of `work`                           | `.backlogit/work` (original queue.md)                        |

## Implementation Units

### Unit 1: Hierarchical File Naming and Queue Folder

**Files:** `internal/core/naming.go`, `internal/core/routing.go`, `internal/config/schema.go`
**Test files:** `internal/core/naming_test.go`, `internal/core/routing_test.go`
**Effort:** medium
**Skill domain:** code
**Dependencies:** none

**Approach:**

Restructure how artifact files are named and where they live.

1. Add `HierarchyLevels` to `WorkspaceConfig`:

```go
type HierarchyLevel struct {
    Level int      `yaml:"level" validate:"required,gte=1,lte=5"`
    Types []string `yaml:"types" validate:"required,min=1"`
}
```

Default config maps: level 1 → feature, level 2 → task/bug, level 3 → subtask.

2. Update `ResolveName` to generate hierarchical IDs. Level 1 items get `001`, level 2 get `001.001`, level 3 get `001.001.001`. The numeric segment at each level auto-increments within the parent scope.

3. Update `ResolveTargetDir` to always return `queue` for active items (replacing per-type directories like `tasks/`, `bugs/`, `epics/`).

4. Update `CreateArtifact` to resolve the hierarchy level from the artifact type, then generate the hierarchical ID based on the parent's ID prefix.

**Verification:**
- Feature-type artifact gets ID like `001`
- Task-type child of `001` gets ID like `001.001`
- Subtask-type child of `001.001` gets ID like `001.001.001`
- All files written to `.backlogit/queue/`

### Unit 2: Migration Tool for Existing Files

**Files:** `internal/cli/migrate_queue.go` (new), `internal/core/migration.go` (new)
**Test files:** `internal/core/migration_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** Unit 1

**Approach:**

Create `backlogit migrate-to-queue` command that:
1. Scans all existing type-specific directories (`tasks/`, `bugs/`, `epics/`, `stories/`)
2. Reads each artifact's frontmatter to determine type and parent relationships
3. Assigns hierarchical IDs based on the WIT-to-level mapping
4. Moves files to `.backlogit/queue/` with the new naming scheme
5. Updates frontmatter `id` fields in place
6. Triggers a full rehydration to rebuild the index

The command operates atomically per-file (temp-file-then-rename) and is idempotent (running twice produces the same result).

**Verification:**
- Existing `tasks/T001-my-task.md` moves to `queue/001.001-my-task.md`
- Frontmatter ID updated to match new scheme
- Rehydration succeeds after migration
- Running migration twice is a no-op

### Unit 3: Dynamic DB Schema from YAML Definitions

**Files:** `internal/db/schema.go`, `internal/db/schema_gen.go` (new)
**Test files:** `internal/db/schema_gen_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** none

**Approach:**

Create a schema generation function that reads `header-def.yaml` and produces the appropriate SQLite columns. The `EnsureSchema` function already uses `CREATE TABLE IF NOT EXISTS`. Extend it to:

1. Read the `HeaderDefConfig` to enumerate all fields across all types
2. Generate a unified column set (union of all type-specific fields plus system defaults)
3. Map `FieldDef.Type` to SQLite column types: `string` → `TEXT`, `int` → `INTEGER`, `enum` → `TEXT`, `list` → `TEXT` (JSON), `datetime` → `DATETIME`
4. Use `PRAGMA user_version` to detect when the schema needs rebuilding (increment version when field definitions change)
5. Drop and recreate the `items` table when the version changes (safe because index.db is ephemeral)

This means customizing `header-def.yaml` with new fields automatically produces matching DB columns on the next `sync`.

**Verification:**
- Adding a custom field to header-def.yaml → `sync` → column appears in items table
- Removing a field → `sync` → column disappears, data rehydrated from Markdown
- `PRAGMA user_version` increments correctly

### Unit 4: Required/Optional Field Enforcement

**Files:** `internal/core/fields.go`, `internal/config/headerdef.go`
**Test files:** `internal/core/fields_test.go`
**Effort:** small
**Skill domain:** code
**Dependencies:** Unit 3

**Approach:**

The `FieldDef` struct already has an `Optional bool` field. Wire this into validation:

1. Add `ValidateArtifactFields(artifact *models.Artifact, headerDef *config.HeaderDefConfig) error` to `internal/core/fields.go`
2. For each field in the type's schema where `Optional == false`, verify the artifact has a non-zero value
3. Call this validation from `CreateArtifact` and `UpdateArtifact` (after applying updates but before writing)
4. Return structured errors listing all missing required fields

The MCP tools and CLI commands inherit this validation through the core functions.

**Verification:**
- Creating a task without a required field returns a descriptive error naming the field
- Creating a task with all required fields succeeds
- Optional fields can be omitted without error

### Unit 5: Template Self-Descriptions

**Files:** `internal/config/templates.go`, `internal/config/defaults.go`
**Test files:** `internal/config/templates_test.go`
**Effort:** small
**Skill domain:** code
**Dependencies:** none

**Approach:**

Extend `TemplateConfig` and its YAML frontmatter with description metadata:

```go
type TemplateConfig struct {
    Name         string       `yaml:"name" validate:"required"`
    ArtifactType string       `yaml:"type" validate:"required"`
    Description  string       `yaml:"description"`
    Relationships []WITRelationship `yaml:"relationships,omitempty"`
    Sections     []SectionDef `yaml:"sections" validate:"required,min=1,dive"`
    Body         string       `yaml:"-"`
}

type WITRelationship struct {
    RelatedType string `yaml:"related_type" validate:"required"`
    Relation    string `yaml:"relation" validate:"required"` // "parent_of", "child_of", "depends_on"
    Description string `yaml:"description"`
}

type SectionDef struct {
    Name        string `yaml:"name" validate:"required"`
    Required    bool   `yaml:"required"`
    Description string `yaml:"description"`
}
```

Update default templates to include descriptions:

```yaml
---
name: task-template
type: task
description: "A discrete unit of work that can be completed in a single sprint"
relationships:
  - related_type: feature
    relation: child_of
    description: "Tasks are children of features"
  - related_type: subtask
    relation: parent_of
    description: "Tasks can have subtasks for granular breakdown"
sections:
  - name: description
    required: true
    description: "Detailed description of the work to be done"
  - name: acceptance-criteria
    required: false
    description: "Conditions that must be met for the task to be considered complete"
---
```

**Verification:**
- `LoadTemplates` parses description fields without error
- Templates without descriptions still load (backward-compatible)
- `ListTemplates` includes descriptions in output

### Unit 6: WIT Metadata API — `backlogit_describe_type` MCP Tool

**Files:** `internal/mcp/tools.go`, `internal/core/wit_metadata.go` (new)
**Test files:** `internal/core/wit_metadata_test.go` (new), `tests/contract/wit_metadata_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** Unit 4, Unit 5

**Approach:**

Create a new MCP tool that returns everything an agent needs to work with a WIT type:

```go
// RegisterTools — add to existing registration:
s.addTool(
    mcplib.NewTool("backlogit_describe_type",
        mcplib.WithDescription("Get complete metadata for a work item type"),
        mcplib.WithString("type", mcplib.Required(),
            mcplib.Description("Artifact type name")),
    ),
    s.handleDescribeType,
)
```

The handler merges data from three sources:
1. `header-def.yaml` — field schemas with required/optional, types, enums, defaults
2. Template config — section definitions with descriptions, relationships
3. `config.yaml` — hierarchy level, naming format, ID prefix

Response structure:

```json
{
    "type": "task",
    "description": "A discrete unit of work...",
    "hierarchy_level": 2,
    "id_format": "{prefix}{NNN}",
    "fields": {
        "status": {"type": "enum", "values": [...], "required": true, "default": "queued"},
        "priority": {"type": "enum", "values": [...], "required": false, "default": "medium"}
    },
    "sections": [
        {"name": "description", "required": true, "description": "..."},
        {"name": "acceptance-criteria", "required": false, "description": "..."}
    ],
    "relationships": [
        {"related_type": "feature", "relation": "child_of", "description": "..."}
    ],
    "directory": {"active": "queue", "archive": "archive/tasks"}
}
```

Also add `backlogit_list_types` MCP tool that returns a summary of all WIT types with their hierarchy levels and descriptions (lightweight discovery, no field details).

**Verification:**
- `backlogit_describe_type` with `type=task` returns complete merged metadata
- `backlogit_list_types` returns all configured types with hierarchy levels
- Unknown type returns descriptive error
- Contract tests validate response schema

### Unit 7: Dependency Graph — Junction Table and Query Functions

**Files:** `internal/db/schema.go`, `internal/db/dependencies.go` (new)
**Test files:** `internal/db/dependencies_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** none

**Approach:**

Add a dedicated dependency junction table to the SQLite schema:

```sql
CREATE TABLE IF NOT EXISTS item_deps (
    item_id    TEXT NOT NULL,
    depends_on TEXT NOT NULL,
    dep_type   TEXT NOT NULL DEFAULT 'blocks',
    PRIMARY KEY (item_id, depends_on),
    FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on) REFERENCES items(id) ON DELETE CASCADE
)
```

The `dep_type` field supports: `blocks` (hard dependency), `relates_to` (soft link), `child_of` (hierarchy).

Query functions:

```go
func UpsertDependency(ctx, db, itemID, dependsOn, depType string) error
func DeleteDependency(ctx, db, itemID, dependsOn string) error
func GetDependencies(ctx, db, itemID string) ([]DependencyEdge, error)
func GetDependents(ctx, db, dependsOn string) ([]DependencyEdge, error)
func GetReadyItems(ctx, db, artifactType string) ([]*models.Artifact, error)
```

`GetReadyItems` returns items whose status is `queued` AND all `blocks`-type dependencies have status `done` or `accepted`. This is the core query for the work queue.

Update the rehydration engine to populate `item_deps` from the `dependencies` field in each artifact's frontmatter.

**Verification:**
- Insert dependency → query returns it
- Delete dependency → query no longer returns it
- `GetReadyItems` returns only items with all blockers resolved
- Circular dependency data does not cause infinite loops in queries

### Unit 8: Enhanced Dependency Tracking in Core and MCP

**Files:** `internal/core/artifacts.go`, `internal/mcp/tools.go`
**Test files:** `internal/core/artifacts_test.go`, `tests/contract/dependency_test.go` (new)
**Effort:** small
**Skill domain:** code
**Dependencies:** Unit 7

**Approach:**

Wire the dependency junction table into the existing artifact lifecycle:

1. When `CreateArtifact` is called with `WithDependencies(...)`, also call `UpsertDependency` for each dep
2. When `UpdateArtifact` modifies dependencies, reconcile the junction table (delete removed, add new)
3. Add a `backlogit_add_dependency` MCP tool for explicit dependency wiring
4. Add a `backlogit_get_dependencies` MCP tool that returns both upstream (blocks) and downstream (blocked-by) edges
5. Update `backlogit_get_item` response to include resolved dependency information

**Verification:**
- Creating an item with dependencies populates `item_deps`
- Updating dependencies reconciles the junction table
- MCP tools return correct dependency information

### Unit 9: Archive Command and MCP Tool

**Files:** `internal/cli/archive.go` (new), `internal/core/archive.go` (new), `internal/mcp/tools.go`
**Test files:** `internal/core/archive_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** Unit 1

**Approach:**

Implement `backlogit archive <id>` CLI command and `backlogit_archive_item` MCP tool:

1. Look up the artifact by ID
2. Resolve the archive directory from the template's `archive_dir` field (default: `.backlogit/archive/{type}/`)
3. Move the file from `.backlogit/queue/` to the archive directory using atomic rename
4. Update the artifact status to `accepted` (or preserve current if already terminal)
5. Delete the item from the SQLite index (or mark as archived)
6. Append an `archived` event to `events.jsonl`

For bulk archiving: `backlogit archive --status done` archives all items with status `done`.

Add template frontmatter fields for directory mapping:

```yaml
directories:
  active: queue
  archive: archive/tasks
```

**Verification:**
- `backlogit archive OP001` moves file to archive directory
- Archived item no longer appears in `list` or `query` results
- Event stream records the archive action
- Bulk archive processes all matching items

### Unit 10: Commit Tracking and Merge Detection

**Files:** `internal/core/commits.go` (new), `internal/cli/check_merged.go` (new)
**Test files:** `internal/core/commits_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** Unit 9

**Approach:**

The `commit` field already exists on Artifact. Extend it to support multiple commits and branch association:

1. Add `Branch` field to Artifact: `Branch string "json:\"branch,omitempty\" yaml:\"branch,omitempty\""`
2. Create `backlogit check-merged [--auto-archive]` CLI command that:
   - Scans all items with a `branch` field
   - Runs `git log --merges main` to check if the branch has been merged
   - Reports which items are eligible for archiving
   - With `--auto-archive`, archives eligible items
3. Add `backlogit_check_merged` MCP tool with the same logic

This keeps the implementation simple (read-only git log parsing) without requiring git hooks or CI integration.

**Verification:**
- Item with `branch: feat/my-feature` and merged branch → reported as merge-ready
- Item with unmerged branch → not reported
- `--auto-archive` flag archives merge-ready items

### Unit 11: Work Queue — `backlogit_next_work` MCP Tool

**Files:** `internal/mcp/tools.go`, `internal/core/queue.go` (new), `internal/db/dependencies.go`
**Test files:** `internal/core/queue_test.go` (new), `tests/contract/queue_test.go` (new)
**Effort:** medium
**Skill domain:** code
**Dependencies:** Unit 7, Unit 8

**Approach:**

Create the "what to work on next" query interface:

```go
s.addTool(
    mcplib.NewTool("backlogit_next_work",
        mcplib.WithDescription("Get the next work item(s) ready to be worked on"),
        mcplib.WithString("type", mcplib.Description("Filter by artifact type")),
        mcplib.WithString("level", mcplib.Description("Filter by hierarchy level (1=feature, 2=task, 3=subtask)")),
        mcplib.WithNumber("limit", mcplib.Description("Max items to return (default 5)")),
    ),
    s.handleNextWork,
)
```

The handler:
1. Calls `GetReadyItems` to find items with all dependencies resolved
2. Sorts by priority (high first), then by creation date (oldest first)
3. Optionally filters by type or hierarchy level
4. Returns the items with their dependency context (what they unblock)

Also add `backlogit next` CLI command for console use, with the same logic.

**Verification:**
- Item with unresolved dependencies not returned
- Item with all deps done appears in results
- Priority ordering respected
- Level filtering works

### Unit 12: Harness Workflow Status Attribute

**Files:** `internal/config/defaults.go`, `internal/core/fields.go`
**Test files:** `internal/core/fields_test.go`
**Effort:** small
**Skill domain:** code
**Dependencies:** Unit 4

**Approach:**

Add `harness` as a new field in header-def.yaml for feature-level WITs:

```yaml
types:
  feature:
    fields:
      harness:
        type: enum
        values: [ready, building, testing, passing, failing]
        default: ready
        optional: true
```

The harness field enables the build orchestrator's workflow policy to enforce state transitions. Policies in `.github/policies/` can reference this field to gate work progression.

Wire the field through the standard validation pipeline. No special logic needed beyond the existing enum validation — the policy enforcement is external to backlogit.

**Verification:**
- Feature-type item can have `harness: ready` set via CLI and MCP
- Invalid harness value rejected with descriptive error
- Other WIT types do not have the harness field (type-scoped)

### Unit 13: CLI Enhancements — Status Grouping and View Rendering

**Files:** `internal/cli/list.go`, `internal/cli/get.go`, `internal/cli/status_cmd.go`
**Test files:** `internal/cli/list_test.go`, `internal/cli/get_test.go`
**Effort:** small
**Skill domain:** code
**Dependencies:** Unit 1

**Approach:**

Enhance the existing CLI commands:

1. **`list` with status grouping**: Add `--group-by-status` flag that groups output by status level with headers:
```
=== QUEUED (3) ===
ID          TITLE                          TYPE    PRIORITY
001.001     Implement parser               task    high
001.002     Fix routing bug                bug     high
001.003     Add validation                 task    medium

=== ACTIVE (1) ===
ID          TITLE                          TYPE    PRIORITY
001.001.001 Write unit tests               subtask medium
```

2. **`get` with enhanced rendering**: The existing `get` already prints full text. Add `--header-only` flag to print just the YAML frontmatter in a formatted table.

3. **`status` summary**: Enhance to show counts per type per status as a cross-tabulation.

**Verification:**
- `backlogit list --group-by-status` produces grouped tabular output
- `backlogit get OP001 --header-only` produces formatted frontmatter
- `backlogit status` shows type×status cross-tab

## Dependency Graph

```text
Unit 1 (Hierarchical Naming)
├── Unit 2 (Migration Tool)
├── Unit 9 (Archive Command)
└── Unit 13 (CLI Enhancements)

Unit 3 (Dynamic Schema)
└── Unit 4 (Required/Optional Fields)
    ├── Unit 6 (WIT Metadata API)
    └── Unit 12 (Harness Status)

Unit 5 (Template Descriptions)
└── Unit 6 (WIT Metadata API)

Unit 7 (Dependency Junction Table)
├── Unit 8 (Dependency Tracking in Core/MCP)
│   └── Unit 11 (Work Queue)
└── Unit 11 (Work Queue)

Unit 10 (Commit Tracking) ← Unit 9
```

Sequencing: {1, 3, 5, 7} → {2, 4, 8, 9, 13} → {6, 10, 11, 12}

### Phased Delivery

**Phase 1: Foundation** (Units 1, 2, 3, 5, 7)
Delivers: Hierarchical file organization, migration, dynamic schema, template descriptions, dependency table. User-visible: files reorganized into `.backlogit/queue/`, schema auto-maps from YAML.

**Phase 2: Type System + Dependencies** (Units 4, 6, 8, 12, 13)
Delivers: Required/optional enforcement, WIT metadata API, dependency wiring, harness status, CLI enhancements. User-visible: agents can query type metadata, dependencies tracked in DB.

**Phase 3: Lifecycle + Queue** (Units 9, 10, 11)
Delivers: Archive command, commit tracking, work queue. User-visible: full lifecycle management, agents can ask "what's next?"

## Constitution Check

| Principle                      | Compliance | Notes                                                                     |
|--------------------------------|------------|---------------------------------------------------------------------------|
| I. Type-Safe Go                | ✅          | All new structs use validator tags; no `any` without justification       |
| II. MCP Protocol Fidelity      | ✅          | New tools registered unconditionally; return errors when workspace missing |
| III. Test-First Development    | ✅          | Every unit specifies test files and verification criteria                 |
| IV. Workspace Containment      | ✅          | All file ops through SafeResolve; archive paths validated                |
| V. Structured Observability    | ✅          | All new functions use slog with package context fields                   |
| VI. Single-Binary Simplicity   | ✅          | No new external dependencies                                            |
| VII. CQRS Architecture         | ✅          | Markdown remains source of truth; junction table populated via rehydration |
| VIII. Git-Friendly Persistence | ✅          | Hierarchical naming is deterministic; archive preserves files            |
| IX. Agent Context Efficiency   | ✅          | `describe_type` and `next_work` return targeted metadata                 |

## Risks and Caveats

1. **Migration complexity**: Renaming all existing files and updating IDs is a disruptive operation. The migration tool must be idempotent and provide a dry-run mode.

2. **Hierarchical ID generation**: When a parent has many children, the 3-digit segment (`NNN`) limits to 999 items per level. This is acceptable for the vast majority of projects. Support for wider segments can be added later via config.

3. **Dependency cycle detection**: The `GetReadyItems` query assumes a DAG. Circular dependencies would cause items to never appear as "ready." Cycle detection should be added as a validation step when dependencies are wired.

4. **Archive and index consistency**: Archived files are moved out of the active directory. The rehydration engine must be updated to skip the archive directory. If an archived item is referenced as a dependency, the junction table must handle the missing FK gracefully.

5. **Backward compatibility**: The shift from per-type directories to a single `queue/` folder is a breaking change. The migration tool provides the upgrade path, but users must run it explicitly.

## Learnings Applied

1. **Atomic file writes** (from feature-001): All file moves use temp-file-then-rename pattern.
2. **Pure string transformers** (from queue-features-plan review F16): Section operations stay pure; file I/O in core.
3. **SafeResolve for all paths** (from feature-001): Archive and migration paths validated through SafeResolve.
4. **PRAGMA user_version for schema migration** (from queue-features-plan Unit 2): Extended for dynamic schema versioning.
5. **Fixed MCP tool surface** (from queue-features-plan D4): New tools are fixed, not dynamic.
