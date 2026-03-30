---
title: "Queue Features: CLI Commands, Header Definitions, Templates, and Dynamic Tools"
date: 2026-03-30
origin: ".backlog/queue.md"
status: draft
---

# Queue Features: CLI Commands, Header Definitions, Templates, and Dynamic Tools

## Problem Frame

The backlogit core implementation (TASK-001) established the foundational architecture: config loading, models, SQLite cache, rehydration, event streams, and a minimal CLI with `init`, `sync`, and `mcp` commands. The queue defines the next evolution: a complete CLI command suite, a per-type header definition system, a template engine with section-based markdown management, and dynamic MCP tool generation from registered templates.

The current gaps prevent backlogit from functioning as a standalone workspace management tool. Users cannot create, list, search, or manage artifacts from the command line beyond `init` and `sync`. The artifact model lacks per-type field schemas with immutable defaults. There is no template system for standardizing artifact body content, and MCP tools are statically defined rather than generated from workspace configuration.

### Scope Boundary

This plan covers the four feature areas defined in `.backlog/queue.md`. It builds on the completed TASK-001 foundation without modifying the CQRS architecture, SQLite cache strategy, or MCP transport layer.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Full CLI command suite: add, list, get, update, move, delete, search, query, status, migrate | queue.md line 3-6 |
| R2 | `add` command with `--type` flag for artifact type selection | queue.md line 4 |
| R3 | `init` already exists; research and suggest additional commands | queue.md line 6 |
| R4 | YAML header definition via `header-def.yaml` with immutable defaults by type | queue.md line 8 |
| R5 | Header fields: type (enums), created_date, updated_date, id (OP prefix + 3 digits), title, status (enums), assigned-to, owner, labels, dependencies[], references[], priority (enums), parent-id, commit | queue.md lines 9-23 |
| R6 | Templates for common operation types (tasks, bugs, features) in `.backlog/templates/` | queue.md lines 24-25 |
| R7 | `registry.yaml` defines which templates are in use | queue.md line 26 |
| R8 | IDs are immutable by hand; only modifiable via backlogit tool | queue.md line 27 |
| R9 | Multi-line markdown input via input buffer for each section | queue.md line 28 |
| R10 | Section-based updates via section flags defined in templates | queue.md line 29 |
| R11 | Dynamic MCP tool call generation from registered templates | queue.md lines 30-31 |
| R12 | Custom task sections by type with BEGIN/END section tags | queue.md lines 31-32 |

## Scope Boundaries

### In Scope

- CLI commands: `add`, `list`, `get`, `update`, `move`, `delete`, `search`, `query`, `status`, `migrate`
- Header definition schema (`header-def.yaml`) with per-type immutable field defaults
- Artifact model expansion: `assigned_to`, `owner`, `labels`, `dependencies`, `references`, `commit` fields
- Template system: `.backlogit/templates/` with section-tagged markdown bodies
- Template registry integration in `registry.yaml`
- Section-based artifact updates (read/write individual sections by tag)
- Dynamic MCP tool generation from template definitions
- Multi-line markdown input buffer for CLI section writes
- ID immutability enforcement in update paths

### Non-Goals

- TUI (Bubble Tea) implementation
- External system sync (Jira, Azure DevOps hooks)
- Sprint management commands
- OKR/milestone artifact types
- `backlogit watch` file watcher

### Deferred to Implementation

- Exact stdin buffering strategy for multi-line input (pipe vs heredoc vs interactive)
- Template inheritance chain depth (single-level vs multi-level)
- Whether dynamic MCP tools should hot-reload on config change or require server restart

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort, targets a single skill domain, and specifies a verifiable exit state.

### Unit 1: Expand Artifact Model with Queue Fields

**Files:** `internal/models/artifact.go`
**Test files:** `internal/models/artifact_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `Artifact` struct with validator tags (`internal/models/artifact.go` lines 25-37)
**Dependencies:** none

**Approach:**
Add the missing fields to the `Artifact` struct that the queue requires: `AssignedTo`, `Owner`, `Labels` (string slice), `Dependencies` (string slice), `References` (string slice), and `Commit`. Add corresponding JSON/YAML struct tags and validator constraints. Update `Validate()` to cover new fields. The `Labels`, `Dependencies`, and `References` fields use `[]string` since they are multi-value. `AssignedTo` and `Owner` are plain strings. `Commit` is an optional string for linking to a specific git commit.

**Verification:**
- `go test ./internal/models/...` passes with new field validation tests
- Table-driven tests cover: valid artifact with all new fields, empty optional fields, labels with duplicates

### Unit 2: Update DB Schema and Query Functions for New Fields

**Files:** `internal/db/schema.go`, `internal/db/queries.go`
**Test files:** `internal/db/queries_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing schema bootstrap (`internal/db/schema.go`), `UpsertItem` / `scanArtifactRow` patterns (`internal/db/queries.go`)
**Dependencies:** Unit 1

**Approach:**
Add columns `assigned_to TEXT`, `owner TEXT`, `labels TEXT` (JSON array), `dependencies TEXT` (JSON array), `references TEXT` (JSON array), and `commit TEXT` to the `items` table. Update `UpsertItem` to serialize slice fields as JSON. Update `scanArtifactRow` to deserialize them. Add `labels` and `dependencies` to FTS5 content for searchability. Update `QueryFilters` with `AssignedTo` and `Owner` filter options.

**Verification:**
- `go test ./internal/db/...` passes with upsert/scan round-trip tests for new fields
- FTS5 search returns results matching label content

### Unit 3: Update Frontmatter Parser for New Fields

**Files:** `internal/models/frontmatter.go`
**Test files:** `internal/models/frontmatter_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `ArtifactFromFrontmatter` and `SerializeFrontmatter` in `internal/models/frontmatter.go`
**Dependencies:** Unit 1

**Approach:**
Update `ArtifactFromFrontmatter` to extract `assigned_to`, `owner`, `labels`, `dependencies`, `references`, and `commit` from the frontmatter map. Update `SerializeFrontmatter` to emit these fields. Slice fields serialize as YAML sequences. Ensure round-trip fidelity: parse → serialize → parse produces identical results.

**Verification:**
- `go test ./internal/models/...` passes with round-trip frontmatter tests for all new fields
- YAML output for slice fields uses flow-style sequences

### Unit 4: Update Rehydration for New Fields

**Files:** `internal/db/rehydration.go`
**Test files:** `internal/db/rehydration_test.go` (new or expand existing)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `Rehydrate` function in `internal/db/rehydration.go`
**Dependencies:** Unit 2, Unit 3

**Approach:**
Ensure the rehydration engine passes the new fields through from parsed frontmatter to `UpsertItem`. Since rehydration uses `models.ParseFrontmatter` → `models.ArtifactFromFrontmatter` → `db.UpsertItem`, the changes in Units 1-3 should flow through, but integration testing is required.

**Verification:**
- Integration test: write a Markdown file with all new fields → rehydrate → query SQLite → verify all fields round-trip

### Unit 5: Update Core CreateArtifact and UpdateArtifact for New Fields

**Files:** `internal/core/artifacts.go`
**Test files:** `internal/core/artifacts_test.go` (new or expand)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Functional options pattern in `internal/core/artifacts.go` lines 14-47
**Dependencies:** Unit 1, Unit 2, Unit 3

**Approach:**
Add new `Option` functions: `WithAssignedTo`, `WithOwner`, `WithLabels`, `WithDependencies`, `WithReferences`, `WithCommit`. Update `CreateArtifact` to pass these to the `Artifact` struct and include them in frontmatter serialization. Update `UpdateArtifact` to handle the new fields in the updates map. Enforce ID immutability: reject any update that attempts to change the `id` field.

**Verification:**
- `go test ./internal/core/...` passes with creation tests using new options
- Update test confirms ID change is rejected with descriptive error

### Unit 6: Header Definition Schema (header-def.yaml)

**Files:** `internal/config/headerdef.go` (new), `internal/config/schema.go`
**Test files:** `internal/config/headerdef_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `WorkspaceConfig` loading pattern in `internal/config/loader.go`
**Dependencies:** Unit 1

**Approach:**
Create a `HeaderDefConfig` struct that defines per-type field schemas with immutable defaults. Structure:

```yaml
# header-def.yaml
defaults:
  created_date: auto     # always set by system
  updated_date: auto     # always set by system
  id: immutable          # cannot be changed after creation
  status:
    type: enum
    values: [To-Do, In-Progress, Blocked, Done]
    default: To-Do

types:
  Task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      assigned-to: {type: string, optional: true}
      owner: {type: string, optional: true}
      labels: {type: list, optional: true}
      dependencies: {type: list, optional: true}
      references: {type: list, optional: true}
      priority: {type: enum, values: [Low, Medium, High], default: Medium}
      parent-id: {type: string, optional: true}
      commit: {type: string, optional: true}
  Bug:
    prefix: OP
    fields:
      severity: {type: enum, values: [Critical, High, Medium, Low]}
      # inherits all default fields
  Epic:
    prefix: OP
    fields:
      # epics have fewer required fields
```

Add `LoadHeaderDef` function. Integrate with `Workspace` initialization. Use `go-playground/validator` for schema validation. Mark `id`, `created_date`, `updated_date` as system-managed immutable fields that are rejected from manual updates.

**Verification:**
- `go test ./internal/config/...` passes with header-def loading, validation, and per-type field resolution tests
- Invalid header-def files produce descriptive errors

### Unit 7: Header Definition Defaults Writer

**Files:** `internal/config/defaults.go`
**Test files:** `internal/config/defaults_test.go` (new or expand)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `WriteDefaults` in `internal/config/defaults.go`
**Dependencies:** Unit 6

**Approach:**
Extend `WriteDefaults` to also generate a default `header-def.yaml` with the queue-specified types (Epic, Feature, Sub-Epic, User-Story, Task, Sub-Task, Bug, Decision) and fields. Use the OP prefix with 3-digit ID format as specified in the queue.

**Verification:**
- `go test ./internal/config/...` passes with `WriteDefaults` producing valid `header-def.yaml`
- Generated file can be loaded back by `LoadHeaderDef` without errors

### Unit 8: Template System — Schema and Loader

**Files:** `internal/config/templates.go` (new)
**Test files:** `internal/config/templates_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `WorkspaceConfig` loader pattern in `internal/config/loader.go`
**Dependencies:** Unit 6

**Approach:**
Define a `TemplateConfig` struct representing a Markdown template with named sections delimited by `<!-- BEGIN:{section_name} -->` and `<!-- END:{section_name} -->` tags. Each template file lives in `.backlogit/templates/` and declares:

```yaml
# frontmatter of .backlogit/templates/task.md
---
type: Task
sections:
  - name: description
    flag: --description
    required: true
  - name: acceptance-criteria
    flag: --acceptance-criteria
    required: false
  - name: implementation-notes
    flag: --notes
    required: false
---
# {title}

<!-- BEGIN:description -->
<!-- END:description -->

## Acceptance Criteria

<!-- BEGIN:acceptance-criteria -->
<!-- END:acceptance-criteria -->

## Implementation Notes

<!-- BEGIN:implementation-notes -->
<!-- END:implementation-notes -->
```

Create `LoadTemplates` to discover and parse template files. Validate section names are unique and flags are valid CLI flag names. Integrate template discovery into the registry so `registry.yaml` can declare which templates are active.

**Verification:**
- `go test ./internal/config/...` passes with template loading, parsing, and section extraction tests
- Invalid templates (missing END tags, duplicate section names) produce descriptive errors

### Unit 9: Template System — Default Templates

**Files:** `internal/config/defaults.go`
**Test files:** `internal/config/defaults_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `WriteDefaults` in `internal/config/defaults.go`
**Dependencies:** Unit 8

**Approach:**
Generate default template files for the eight artifact types defined in the queue (Task, Bug, Epic, Feature, Sub-Epic, User-Story, Sub-Task, Decision). Each template uses section tags appropriate for that type. Update `WriteDefaults` and `backlogit init` to create `.backlogit/templates/` with these files and register them in `registry.yaml`.

**Verification:**
- `backlogit init` on a fresh directory produces `.backlogit/templates/` with 8 template files
- Each template file loads successfully via `LoadTemplates`

### Unit 10: Section Parser and Writer

**Files:** `internal/parser/sections.go` (new)
**Test files:** `internal/parser/sections_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `ParseFrontmatter` pattern in `internal/models/frontmatter.go`
**Dependencies:** Unit 8

**Approach:**
Implement `ParseSections(content string) (map[string]string, error)` that extracts named sections from markdown content between `<!-- BEGIN:{name} -->` and `<!-- END:{name} -->` tags. Implement `WriteSections(content string, updates map[string]string) (string, error)` that replaces section content while preserving the rest of the document. Implement `WriteSection(content string, name string, value string) (string, error)` for single-section updates.

Handle edge cases: nested HTML comments, missing end tags, empty sections, sections with leading/trailing whitespace.

**Verification:**
- `go test ./internal/parser/...` passes with round-trip section parse/write tests
- Edge case tests: empty sections, sections with markdown content, missing tags produce errors

### Unit 11: CLI `add` Command

**Files:** `internal/cli/add.go` (new)
**Test files:** `internal/cli/add_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newInitCommand` pattern in `internal/cli/root.go`
**Dependencies:** Unit 5, Unit 6, Unit 10

**Approach:**
Implement `backlogit add --type <type> --title <title> [section flags]`. The command:
1. Opens the workspace
2. Resolves the type from `header-def.yaml`
3. Loads the appropriate template
4. Creates the artifact via `core.CreateArtifact`
5. Populates sections from flags or stdin (multi-line input buffer)
6. Writes the artifact file with template structure

For multi-line input: if a section flag value is `-`, read from stdin until EOF. Support pipe input (`echo "content" | backlogit add --type task --title "Foo" --description -`).

Register the command in `root.go` via `root.AddCommand(newAddCommand(&cwd))`.

**Verification:**
- `go test ./internal/cli/...` passes with add command tests using captured stdout
- Integration test: `add` creates a valid markdown file with frontmatter and section tags

### Unit 12: CLI `list` Command

**Files:** `internal/cli/list.go` (new)
**Test files:** `internal/cli/list_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newSyncCommand` pattern in `internal/cli/root.go`
**Dependencies:** Unit 2

**Approach:**
Implement `backlogit list [--type <type>] [--status <status>] [--assigned-to <user>] [--sprint <id>]`. Query the SQLite index via `db.QueryItems` and format output as a table with columns: ID, Title, Status, Type, Priority. Support `--json` flag for JSON output. Default to table format.

**Verification:**
- `go test ./internal/cli/...` passes with list output formatting tests
- JSON mode produces valid JSON array

### Unit 13: CLI `get` Command

**Files:** `internal/cli/get.go` (new)
**Test files:** `internal/cli/get_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newSyncCommand` pattern
**Dependencies:** Unit 2

**Approach:**
Implement `backlogit get <id>`. Retrieve the artifact via `db.GetItem`, then read the full Markdown file from disk. Display the full artifact content (frontmatter + body). Support `--json` for frontmatter-only JSON output and `--section <name>` to extract a specific section.

**Verification:**
- `go test ./internal/cli/...` passes with get output tests
- `--section` flag returns only the specified section content

### Unit 14: CLI `update` Command

**Files:** `internal/cli/update.go` (new)
**Test files:** `internal/cli/update_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newSyncCommand` pattern
**Dependencies:** Unit 5, Unit 10

**Approach:**
Implement `backlogit update <id> [--title <title>] [--status <status>] [--priority <priority>] [section flags]`. Updates frontmatter fields via `core.UpdateArtifact`. Section flags (defined in the template) update individual sections via the section writer. Enforce ID immutability: reject `--id` flag. Support stdin for multi-line section content. Re-sync the SQLite index after update.

**Verification:**
- `go test ./internal/cli/...` passes with update tests for both metadata and section content
- ID update attempt produces descriptive error

### Unit 15: CLI `move` Command

**Files:** `internal/cli/move.go` (new)
**Test files:** `internal/cli/move_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `MoveArtifactFile` in `internal/core/routing.go`
**Dependencies:** Unit 5

**Approach:**
Implement `backlogit move <id> --status <new_status>`. Changes the artifact status and physically relocates the file according to `registry.yaml` routing rules. Uses `core.UpdateArtifact` for status change and `core.MoveArtifactFile` for relocation. Re-syncs the index.

**Verification:**
- `go test ./internal/cli/...` passes
- Integration test: move changes both status in frontmatter and file location on disk

### Unit 16: CLI `delete` and `search` Commands

**Files:** `internal/cli/delete.go` (new), `internal/cli/search.go` (new)
**Test files:** `internal/cli/delete_test.go` (new), `internal/cli/search_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `newSyncCommand` pattern
**Dependencies:** Unit 2

**Approach:**
`backlogit delete <id> [--force]`: Remove the artifact file and delete from the index. Without `--force`, prompt for confirmation. `backlogit search <query> [--limit N]`: Full-text search via `db.SearchItems` with FTS5. Display results in table format with relevance ordering.

**Verification:**
- Delete removes both the markdown file and the SQLite row
- Search returns FTS5 results formatted as a table

### Unit 17: CLI `query` and `status` Commands

**Files:** `internal/cli/query.go` (new), `internal/cli/status.go` (new)
**Test files:** `internal/cli/query_test.go` (new), `internal/cli/status_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `db.ExecuteGatedQuery` in `internal/db/gate.go`
**Dependencies:** Unit 2

**Approach:**
`backlogit query "<sql>"`: Execute a read-only SQL query via the gate and display results as a table. `backlogit status`: Show workspace summary (artifact counts by type and status, last sync time, workspace path).

**Verification:**
- `query` rejects non-SELECT statements with descriptive error
- `status` displays correct counts matching SQLite index

### Unit 18: CLI Command Registration

**Files:** `internal/cli/root.go`
**Test files:** `internal/cli/root_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first
**Patterns to follow:** Existing `root.AddCommand` calls in `internal/cli/root.go`
**Dependencies:** Units 11-17

**Approach:**
Register all new commands in `NewRootCommand`. Verify command help text and flag definitions. Ensure no flag name collisions across commands.

**Verification:**
- `backlogit --help` lists all commands
- Each command's `--help` produces valid documentation

### Unit 19: Update MCP Tools for New Fields

**Files:** `internal/mcp/tools.go`
**Test files:** `tests/contract/tools_contract_test.go` (new or expand)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing tool registration in `internal/mcp/tools.go`
**Dependencies:** Unit 5

**Approach:**
Add the new fields (`assigned_to`, `owner`, `labels`, `dependencies`, `references`, `commit`) to `backlogit_create_item` and `backlogit_update_item` tool schemas. Add `backlogit_list_items` tool with filter parameters. Add `backlogit_search_items` tool. Add `backlogit_move_item` tool for status change with file routing. Add `backlogit_delete_item` tool.

**Verification:**
- Contract tests validate tool input/output schemas
- Each new tool follows the five-step handler pattern

### Unit 20: Dynamic MCP Tool Generation from Templates

**Files:** `internal/mcp/dynamic.go` (new)
**Test files:** `internal/mcp/dynamic_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `RegisterTools` in `internal/mcp/tools.go`, `LoadTemplates` from Unit 8
**Dependencies:** Unit 8, Unit 10, Unit 19

**Approach:**
Implement `RegisterDynamicTools(s *MCPServer, templates []*TemplateConfig)`. For each registered template, generate MCP tools:
- `backlogit_create_{type}`: Pre-fills the artifact type, exposes section-specific string parameters from the template's section definitions
- `backlogit_update_{type}_section`: Accepts `id` and `section_name` and `content` to update a specific section

Each dynamic tool handler delegates to the core create/update paths with template-aware section writing. Tool descriptions include the section names and their required/optional status.

**Verification:**
- Dynamic tools appear in the MCP tool list based on registered templates
- Creating an artifact via a dynamic tool produces a file with correct template structure

### Unit 21: Integration Tests for Full Workflow

**Files:** `tests/integration/workflow_test.go` (new)
**Test files:** (self-contained)
**Effort size:** medium
**Skill domain:** tests
**Execution note:** test-first
**Patterns to follow:** `t.TempDir()` workspace pattern from compound learnings
**Dependencies:** Units 1-20

**Approach:**
End-to-end tests covering:
1. `init` → creates workspace with `header-def.yaml`, templates, config, registry
2. `add --type task --title "Test"` → creates artifact with template sections
3. `list` → shows the created artifact
4. `get <id>` → displays full content
5. `update <id> --status in_progress` → updates frontmatter
6. `move <id> --status done` → relocates file
7. `search "Test"` → finds via FTS5
8. `delete <id>` → removes artifact
9. MCP dynamic tool creation → produces correct artifacts

**Verification:**
- `go test ./tests/integration/...` passes with the complete workflow

## Dependency Graph

```text
Unit 1 (Model)
├── Unit 2 (DB Schema) ──────┐
├── Unit 3 (Frontmatter)     │
│   └── Unit 4 (Rehydration) ◄─┘
├── Unit 5 (Core CRUD) ─────────────┐
│   ├── Unit 11 (CLI add)           │
│   ├── Unit 14 (CLI update)        │
│   └── Unit 15 (CLI move)          │
├── Unit 6 (Header Def) ────────────┤
│   ├── Unit 7 (Defaults Writer)    │
│   └── Unit 8 (Template Schema)    │
│       ├── Unit 9 (Default Tmpls)  │
│       ├── Unit 10 (Section Parse) │
│       └── Unit 20 (Dynamic MCP)   │
├── Unit 12 (CLI list) ◄── Unit 2   │
├── Unit 13 (CLI get) ◄── Unit 2    │
├── Unit 16 (CLI delete/search)◄─ 2 │
├── Unit 17 (CLI query/status) ◄─ 2 │
├── Unit 18 (CLI registration)◄─ ALL│
├── Unit 19 (MCP new fields) ◄── 5  │
└── Unit 21 (Integration) ◄──── ALL
```

Sequencing: Units 1 → {2, 3, 6} → {4, 5, 7, 8} → {9, 10, 11-17, 19} → {18, 20} → 21

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Use `<!-- BEGIN:{name} -->` / `<!-- END:{name} -->` HTML comments for section delimiters | HTML comments are invisible in rendered markdown, widely supported, and parseable with simple regex. | Custom `:::section` fences (non-standard), YAML section markers (conflicts with frontmatter) |
| D2 | Store `header-def.yaml` as a separate file rather than embedding in `config.yaml` | Separation of concerns: `config.yaml` defines workspace behavior, `header-def.yaml` defines artifact schemas. Keeps both files manageable. | Single config file (too large), directory of per-type YAML files (too many files for initial setup) |
| D3 | Slice fields (labels, dependencies, references) stored as JSON arrays in SQLite | SQLite lacks native array type. JSON arrays enable indexing and querying with `json_each()`. Consistent with existing `custom_fields` JSON pattern. | Separate junction tables (over-engineered for file-backed cache), comma-separated strings (no querying) |
| D4 | Dynamic MCP tools generated at server startup, not hot-reloaded | MCP-go SDK registers tools at server creation. Hot-reload would require server restart anyway. Keep it simple. | File watcher with tool re-registration (complex, fragile), lazy registration on first call (violates MCP discovery) |
| D5 | Multi-line stdin input triggered by `-` flag value | Unix convention. Familiar to developers. Works with pipes and heredocs. | Interactive editor launch (heavy dependency), temporary file (extra cleanup) |
| D6 | ID prefix changed from per-type single letter to configurable `OP` prefix per queue spec | Queue explicitly requests `OP` prefix with 3-digit numeral. Make it configurable in `header-def.yaml` so users can customize. | Hard-coded OP (inflexible), keep single-letter prefixes (contradicts queue requirements) |

## Risks and Caveats

1. **Schema migration**: Adding columns to the `items` table requires dropping and recreating the table since SQLite lacks `ALTER TABLE ADD COLUMN` for all cases. The ephemeral nature of `index.db` makes this safe (rehydration rebuilds from scratch), but existing installations must re-sync.

2. **Template section parsing fragility**: HTML comment delimiters could appear in user content. The parser must handle escaped comments and only match the exact `BEGIN:`/`END:` pattern.

3. **Dynamic tool naming collisions**: If a user defines a template type that conflicts with a static tool name (e.g., "query_sql"), the dynamic tool registration must detect and reject the collision.

4. **ID format change**: Moving from `{prefix}{NNN}-{title_slug}` to `OP{NNN}` is a breaking change for existing workspaces. The migration path should support both formats during a transition period.

## Learnings Applied

1. **Package-level validator instance** (from `feature-001-core-implementation.md`): Continue using cached `validator.New()` for new struct validation in `headerdef.go` and `templates.go`.

2. **Atomic file writes** (from `feature-001-core-implementation.md`): All new file writes in CLI commands follow the temp-file-then-rename pattern.

3. **Import cycle prevention** (from `feature-001-core-implementation.md`): The section parser lives in `internal/parser/` and only imports `internal/models/` for frontmatter types, not `internal/db/`.

4. **modernc.org/sqlite driver name** (from `feature-001-core-implementation.md`): Use `"sqlite"` not `"sqlite3"` in all new test files.

5. **SafeResolve absolute path comparison** (from `feature-001-core-implementation.md`): All new file operations in CLI commands go through `SafeResolve` before disk writes.

## Standards Check

| Principle | Compliance | Notes |
|---|---|---|
| I. Type-Safe Go | ✅ | All new structs use validator tags; no `any` without justification |
| II. MCP Protocol Fidelity | ✅ | Dynamic tools are visible unconditionally; error on uninitialized workspace |
| III. Test-First Development | ✅ | Every unit specifies test files and verification criteria first |
| IV. Workspace Containment | ✅ | All file ops route through SafeResolve; section writes validate paths |
| V. Structured Observability | ✅ | New CLI commands and MCP tools use slog with package context |
| VI. Single-Binary Simplicity | ✅ | No new external dependencies; templates are embedded in the binary for defaults |
| VII. CQRS Architecture | ✅ | Markdown remains source of truth; SQLite columns added for query efficiency |
| VIII. Git-Friendly Persistence | ✅ | Section delimiters are HTML comments; slice fields use stable YAML serialization |
| IX. Agent Context Efficiency | ✅ | Dynamic tools return structured JSON; section extraction avoids full-file reads |
