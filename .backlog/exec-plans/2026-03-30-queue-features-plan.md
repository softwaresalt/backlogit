---
title: "Queue Features: CLI Commands, Header Definitions, Templates, and Section-Aware Tools"
date: 2026-03-30
origin: ".backlog/queue.md"
status: revision-3
review: ".backlog/reviews/2026-03-30-queue-features-plan-review.md"
---

# Queue Features: CLI Commands, Header Definitions, Templates, and Section-Aware Tools

## Problem Frame

The backlogit core implementation (TASK-001) established the foundational architecture: config loading, models, SQLite cache, rehydration, event streams, and a minimal CLI with `init`, `sync`, and `mcp` commands. The queue defines the next evolution: a complete CLI command suite, a per-type header definition system, a template engine with section-based markdown management, and section-aware MCP tool integration with template discovery.

The current gaps prevent backlogit from functioning as a standalone workspace management tool. Users cannot create, list, search, or manage artifacts from the command line beyond `init` and `sync`. The artifact model lacks per-type field schemas with immutable defaults. There is no template system for standardizing artifact body content, and MCP tools lack section-aware operations for template-driven artifact management.

### Scope Boundary

This plan covers the four feature areas defined in `.backlog/queue.md`. It builds on the completed TASK-001 foundation without modifying the CQRS architecture, SQLite cache strategy, or MCP transport layer.

## Requirements Trace

| #   | Requirement                                                                                                                                                                                              | Origin               |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| R1  | Full CLI command suite: add, list, get, update, move, delete, search, query, status                                                                                                                      | queue.md line 3-6    |
| R2  | `add` command with `--type` flag for artifact type selection                                                                                                                                             | queue.md line 4      |
| R3  | `init` already exists; research and suggest additional commands                                                                                                                                          | queue.md line 6      |
| R4  | YAML header definition via `header-def.yaml` with immutable defaults by type                                                                                                                             | queue.md line 8      |
| R5  | Header fields: type (enums), created_date, updated_date, id (OP prefix + 3 digits), title, status (enums), assigned-to, owner, labels, dependencies[], references[], priority (enums), parent-id, commit | queue.md lines 9-23  |
| R6  | Templates for common operation types (tasks, bugs, features) in `.backlog/templates/`                                                                                                                    | queue.md lines 24-25 |
| R7  | `registry.yaml` defines which templates are in use                                                                                                                                                       | queue.md line 26     |
| R8  | IDs are immutable by hand; only modifiable via backlogit tool                                                                                                                                            | queue.md line 27     |
| R9  | Multi-line markdown input via input buffer for each section                                                                                                                                              | queue.md line 28     |
| R10 | Section-based updates via section flags defined in templates                                                                                                                                             | queue.md line 29     |
| R11 | Fixed MCP tools with section-aware parameters and template discovery for agent workflows                                                                                                                 | queue.md lines 30-31 |
| R12 | Custom task sections by type with BEGIN/END section tags                                                                                                                                                 | queue.md lines 31-32 |

## Scope Boundaries

### In Scope

- CLI commands: `add`, `list`, `get`, `update`, `move`, `delete`, `search`, `query`, `status`
- Header definition schema (`header-def.yaml`) with per-type immutable field defaults
- Artifact model expansion: `assigned_to`, `owner`, `labels`, `dependencies`, `references`, `commit` fields
- Template system: `.backlogit/templates/` with section-tagged markdown bodies
- Template registry integration in `registry.yaml`
- Section-based artifact updates (read/write individual sections by tag)
- Section-aware parameters on existing MCP CRUD tools (`sections` on create/update, `section` on get)
- Template discovery MCP tool (`backlogit_list_templates`) for agent section discovery
- Multi-line markdown input buffer for CLI section writes via repeatable `--section` flag
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
- `migrate` command for legacy workspace format conversion (removed from this release per F9)
- Additional default templates beyond task, bug, epic (deferred per F19 until loader and section writer are stable)

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
Add columns `assigned_to TEXT`, `owner TEXT`, `labels TEXT` (JSON array), `dependencies TEXT` (JSON array), `references TEXT` (JSON array), and `commit TEXT` to the `items` table. Update `selectCols` to include all 17 columns and update `scanArtifactRow` to scan all 17 fields, ensuring `GetItem`, `QueryItems`, and `SearchItems` all route through the updated constant (F3). Update `UpsertItem` to serialize slice fields as JSON. Update `scanArtifactRow` to deserialize them. Add `labels` and `dependencies` to FTS5 content for searchability; update all three FTS5 sync triggers (`items_ai`, `items_ad`, `items_au`) and the `CREATE VIRTUAL TABLE items_fts` statement — since `CREATE TRIGGER IF NOT EXISTS` is a no-op for existing triggers, the migration must `DROP TRIGGER` old versions first (F7). Add WHERE clause conditions for `filters.AssignedTo` and `filters.Owner` in `QueryItems` (F4). Change `err == sql.ErrNoRows` to `errors.Is(err, sql.ErrNoRows)` (F20).

For schema migration (F5): add a `PRAGMA user_version` guard. When `user_version < 2`, drop and recreate the `items` table (acceptable since `index.db` is ephemeral and gitignored), set `user_version = 2`, then trigger rehydration. This avoids `ALTER TABLE ADD COLUMN` limitations.

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
Create a `HeaderDefConfig` struct that defines per-type field schemas with immutable defaults. Type identifiers use lowercase snake_case (`task`, `bug`, `feature`, `epic`, `sub_epic`, `user_story`, `sub_task`, `decision`) with an optional `display_name` field for human-readable labels (F6). To avoid dual type identity sources (F11), `header-def.yaml` owns field validation and section schemas only; type naming/prefix configuration remains in `config.yaml`'s `ArtifactTypeConfig`. Structure:

```yaml
# header-def.yaml
defaults:
  created_date: auto     # always set by system
  updated_date: auto     # always set by system
  id: immutable          # cannot be changed after creation
  assigned-to: {type: string, optional: true}
  owner: {type: string, optional: true}
  labels: {type: list, optional: true}
  parent-id: {type: string, optional: true}
  commit: {type: string, optional: true}
  status:
    type: enum
    values: [queued, active, blocked, review, done, accepted, rejected]
    default: queued

types:
  task:
    display_name: Task
    fields:
      dependencies: {type: list, optional: true}
      references: {type: list, optional: true}
      priority: {type: enum, values: [low, medium, high], default: medium}
  bug:
    display_name: Bug
    fields:
      dependencies: {type: list, optional: true}
      references: {type: list, optional: true}
      severity: {type: enum, values: [critical, high, medium, low]}
      # inherits all default fields
  feature:
    display_name: Feature
    fields:
      dependencies: {type: list, optional: true}
      references: {type: list, optional: true}
      priority: {type: enum, values: [low, medium, high], default: medium}
      # inherits all default fields

  epic:
    display_name: Epic
    fields:
      assigned-to: {type: string, optional: true}
      owner: {type: string, optional: true}
      labels: {type: list, optional: true}
      # epics have fewer required fields
```

Add `LoadHeaderDef(ctx context.Context, path string)` function (F21). Integrate with `Workspace` initialization. Use `go-playground/validator` for schema validation. Mark `id`, `created_date`, `updated_date` as system-managed immutable fields that are rejected from manual updates. Unify `FieldConfig` and `FieldDef` into a single `FieldDef` struct with all fields marked `omitempty` (F13). Add sentinel errors `ErrSectionNotFound`, `ErrMalformedDoc`, `ErrTypeNotFound` to `internal/errors/errors.go` (F23).

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
Extend `WriteDefaults` to also generate a default `header-def.yaml` with the three initial artifact types (`task`, `bug`, `epic`) and fields using lowercase identifiers (F6). Use the OP prefix with 3-digit ID format as specified in the queue. Additional types (`feature`, `sub_epic`, `user_story`, `sub_task`, `decision`) are deferred until the loader and section writer are stable (F19). Emit the correct 7-value status enum: `[queued, active, blocked, review, done, accepted, rejected]` with default `queued` (F1).

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
type: task
sections:
  - name: description
    required: true
  - name: acceptance-criteria
    required: false
  - name: implementation-notes
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

Create `LoadTemplates(ctx context.Context, templatesDir string)` to discover and parse template files (F21). Validate section names are unique within each template. Integrate template discovery into the registry so `registry.yaml` can declare which templates are active. Import `internal/config` directly for `TemplateConfig` and `SectionDef` — no mirror types (F14).

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
Generate default template files for the three initial artifact types (`task`, `bug`, `epic`) using lowercase identifiers (F19). Each template uses section tags appropriate for that type. Update `WriteDefaults` and `backlogit init` to create `.backlogit/templates/` with these files and register them in `registry.yaml`. Additional templates are deferred until the loader and section writer are stable.

**Verification:**

- `backlogit init` on a fresh directory produces `.backlogit/templates/` with 3 template files
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
Implement `ParseSections(content string) (map[string]string, error)` that extracts named sections from markdown content between `<!-- BEGIN:{name} -->` and `<!-- END:{name} -->` tags. Implement `WriteSections(content string, updates map[string]string) (string, error)` that replaces section content while preserving the rest of the document. Implement `WriteSection(content string, name string, value string) (string, error)` for single-section updates. All three functions are pure string transformers operating on in-memory content — file I/O responsibility stays in `internal/core/` via `SafeResolve` (F16).

Use a scanner/state machine for section delimiters matching only exact non-nested `BEGIN:`/`END:` markers. Do not implement a general-purpose HTML comment parser (F22). Handle edge cases: missing end tags, empty sections, sections with leading/trailing whitespace.

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
Implement `backlogit add --type <type> --title <title> [--section name="content"]`. The command:

1. Opens the workspace
2. Resolves the type from `header-def.yaml`
3. Loads the appropriate template
4. Creates the artifact via `core.CreateArtifact`
5. Populates sections from `--section` flags or stdin (multi-line input buffer)
6. Writes the artifact file with template structure

Use a single repeatable `--section` flag: `backlogit add --type task --title "Foo" --section description="content" --section acceptance-criteria="criteria"`. This avoids the irreconcilable ordering problem of registering per-template flags at Cobra command creation time before templates are loaded (F10).

For multi-line input: if a section flag value is `-`, read from stdin until EOF. Support pipe input (`echo "content" | backlogit add --type task --title "Foo" --section description=-`).

All CLI commands use package-level slog logger with operation context fields (F15).

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
Implement `backlogit update <id> [--title <title>] [--status <status>] [--priority <priority>] [--section name="content"]`. Updates frontmatter fields via `core.UpdateArtifact`. Section updates use the repeatable `--section` flag (defined in the template) to update individual sections via the section writer. Enforce ID immutability: reject `--id` flag. Support stdin for multi-line section content (`--section description=-`). Re-sync the SQLite index after update.

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
`backlogit delete <id> [--force]`: Remove the artifact file and delete from the index. Without `--force`, prompt for confirmation. Require `SafeResolve` path validation before `os.Remove` to prevent path traversal via crafted artifact IDs (F17). Add a path-traversal test case. `backlogit search <query> [--limit N]`: Full-text search via `db.SearchItems` with FTS5. Display results in table format with relevance ordering.

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

### Unit 19: Section-Aware MCP Tools and Template Discovery

**Files:** `internal/mcp/tools.go`, `internal/mcp/templates.go` (new)
**Test files:** `tests/contract/tools_contract_test.go` (new or expand)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing tool registration in `internal/mcp/tools.go`
**Dependencies:** Unit 5, Unit 8, Unit 10

**Approach:**
Add section-aware parameters to the existing fixed MCP tool surface. All tools are registered unconditionally regardless of workspace state, satisfying the constitutional mandate (F2). When called before workspace initialization, tools return a descriptive error rather than being absent.

Updates to existing tools:

- `backlogit_create_item`: Add `sections` parameter (JSON object, optional). Keys are section names defined in the template, values are markdown content strings. Example: `{"description": "Task body", "acceptance-criteria": "- [ ] Done"}`. The handler resolves the template by `artifact_type`, validates section names against the template definition, and writes content between `BEGIN`/`END` tags.
- `backlogit_update_item`: Add `sections` parameter (JSON object, optional). Same structure as create. Only specified sections are updated; omitted sections remain unchanged. The handler reads the existing file, applies section updates via `parser.WriteSections`, and writes back.
- `backlogit_get_item`: Add `section` parameter (string, optional). When provided, returns only the content of the named section instead of the full markdown body.
- `backlogit_delete_item`: No section support needed. Delete the entire artifact.

New tools:

- `backlogit_list_templates`: Returns all registered template types with their section definitions. Response includes type name, display name, and for each section: name, required/optional flag, and description. This enables agents to discover what sections are available for each artifact type without needing dynamic tool generation. Registered unconditionally; returns empty list when workspace is uninitialized.
- `backlogit_list_items`: Add filter parameters for `assigned_to`, `owner`, `labels`, `type`, `status`, `priority`.
- `backlogit_search_items`: Full-text search via FTS5 with limit parameter.
- `backlogit_move_item`: Status change with file routing per `registry.yaml`.

Insert an application service boundary in `internal/core/templates` that both CLI and MCP call for template resolution and section mutation logic. Keep `internal/mcp` as a thin registration and parameter-parsing layer (F12).

**Verification:**

- Contract tests validate tool input/output schemas including new `sections` and `section` parameters
- Each tool follows the five-step handler pattern
- `backlogit_list_templates` returns correct section metadata from registered templates
- Section write round-trip: create with sections → get with section → verify content match
- All tools present in tool list regardless of workspace initialization state

### Unit 20: Application Service Boundary for Template Operations

**Files:** `internal/core/templates/service.go` (new), `internal/core/templates/resolve.go` (new)
**Test files:** `internal/core/templates/service_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `CreateArtifact` functional options pattern in `internal/core/artifacts.go`
**Dependencies:** Unit 8, Unit 10

**Approach:**
Create `internal/core/templates` as the application service boundary that both CLI commands and MCP tool handlers call for template-aware operations (F12). This package owns:

1. **Template resolution**: Given an artifact type, resolve the active template from the registry and return its section definitions.
2. **Section-aware creation**: Accept an artifact type, title, metadata, and a `map[string]string` of section content. Validate section names against the template, construct the markdown body with `BEGIN`/`END` tags populated, and delegate to `core.CreateArtifact` for file writing.
3. **Section-aware updates**: Accept an artifact ID and a `map[string]string` of section updates. Read the existing file, apply updates via `parser.WriteSections`, validate against the template's section definitions, and delegate to `core.UpdateArtifact` for file writing.
4. **Section extraction**: Accept an artifact ID and section name, return the section content via `parser.ParseSections`.
5. **Template listing**: Return all active templates with their section metadata for discovery.

This service ensures CLI and MCP share identical template logic without either becoming an integration hub. `internal/mcp/tools.go` calls this service for section operations. `internal/cli/add.go`, `update.go`, and `get.go` call the same service.

**Verification:**

- Unit tests validate template resolution, section-aware CRUD, and section extraction
- Integration test: CLI `add` and MCP `create_item` produce identical file output for the same inputs
- Invalid section names are rejected with descriptive errors referencing the template definition

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
2. `add --type task --title "Test" --section description="Task body"` → creates artifact with template sections
3. `list` → shows the created artifact
4. `get <id>` → displays full content
5. `update <id> --status active` → updates frontmatter
6. `move <id> --status done` → relocates file
7. `search "Test"` → finds via FTS5
8. `delete <id>` → removes artifact
9. MCP `backlogit_list_templates` → returns template types with section definitions
10. MCP `backlogit_create_item` with `sections` param → produces correct artifacts with section content
11. MCP `backlogit_get_item` with `section` param → returns specific section content
12. MCP `backlogit_update_item` with `sections` param → updates section content without affecting other sections

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
│       └── Unit 20 (Tmpl Service)  │
├── Unit 12 (CLI list) ◄── Unit 2   │
├── Unit 13 (CLI get) ◄── Unit 2    │
├── Unit 16 (CLI delete/search)◄─ 2 │
├── Unit 17 (CLI query/status) ◄─ 2 │
├── Unit 18 (CLI registration)◄─ ALL│
├── Unit 19 (MCP tools) ◄── 5,8,10 │
└── Unit 21 (Integration) ◄──── ALL
```

Sequencing: Units 1 → {2, 3, 6} → {4, 5, 7, 8} → {9, 10, 11-17, 20} → {18, 19} → 21

### Phased Delivery (F18)

To avoid a monolithic 21-unit batch, split into three deliverable phases:

**Phase 1: Foundation + Minimal CRUD** (Units 1-5, 6-8, 10, 11, 12, 13)
Delivers: Model expansion, DB schema, header definitions, template loader, section parser, and `add`/`list`/`get` CLI commands. User-visible value: create and view artifacts with template-based sections.

**Phase 2: Full CLI + Section Operations** (Units 9, 14-18, 20)
Delivers: Default templates, `update`/`move`/`delete`/`search`/`query`/`status` CLI commands, CLI registration, and the template service boundary. User-visible value: complete CLI workflow for artifact lifecycle management.

**Phase 3: MCP Integration + Integration Tests** (Units 19, 21)
Delivers: Section-aware MCP tools, template discovery tool, and end-to-end integration tests. User-visible value: agents can discover templates and manage artifacts with section targeting.

## Decisions

| #   | Decision                                                                                 | Rationale                                                                                                                                           | Alternatives Rejected                                                                                               |
| --- | ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| D1  | Use `<!-- BEGIN:{name} -->` / `<!-- END:{name} -->` HTML comments for section delimiters | HTML comments are invisible in rendered markdown, widely supported, and parseable with a simple state machine scanner.                              | Custom `:::section` fences (non-standard), YAML section markers (conflicts with frontmatter)                        |
| D2  | Store `header-def.yaml` as a separate file rather than embedding in `config.yaml`        | Separation of concerns: `config.yaml` defines workspace behavior, `header-def.yaml` defines artifact field schemas. Type naming/prefix stays in `config.yaml`. | Single config file (too large), directory of per-type YAML files (too many files for initial setup)                 |
| D3  | Slice fields (labels, dependencies, references) stored as JSON arrays in SQLite          | SQLite lacks native array type. JSON arrays enable indexing and querying with `json_each()`. Consistent with existing `custom_fields` JSON pattern. | Separate junction tables (over-engineered for file-backed cache), comma-separated strings (no querying)             |
| D4  | Fixed MCP tools with section-aware parameters instead of dynamic tool generation          | Constitutional mandate requires all tools unconditionally visible regardless of workspace state. Fixed tools with `type` and `sections` parameters provide the same capability without dynamic registration. Agents discover sections via `backlogit_list_templates`. Eliminates namespace collision risk (F8) and non-deterministic tool surface between sessions. | Dynamic per-type tools generated at startup (violates unconditional visibility), hot-reload (complex, fragile) |
| D5  | Multi-line stdin input triggered by `-` flag value                                       | Unix convention. Familiar to developers. Works with pipes and heredocs.                                                                             | Interactive editor launch (heavy dependency), temporary file (extra cleanup)                                        |
| D6  | ID prefix changed from per-type single letter to configurable `OP` prefix per queue spec | Queue explicitly requests `OP` prefix with 3-digit numeral. Make it configurable in `config.yaml` so users can customize.                           | Hard-coded OP (inflexible), keep single-letter prefixes (contradicts queue requirements)                            |
| D7  | Repeatable `--section name="content"` CLI flag instead of per-template flags              | Cobra requires flags to be bound at command creation time, before any workspace is opened or templates loaded. A single repeatable flag avoids this irreconcilable ordering problem (F10). | Per-template flags (impossible with Cobra lifecycle), environment variables (poor UX)                               |
| D8  | Application service boundary in `internal/core/templates`                                | Prevents `internal/mcp` from becoming an integration hub for both content semantics and transport semantics (F12). CLI and MCP both call the service. | Direct template logic in MCP handlers (coupling), shared utility functions (insufficient abstraction)               |
| D9  | Three initial default templates (task, bug, epic) instead of eight                       | Start with proven types. Add remaining templates once the loader and section writer are stable (F19). Avoids YAGNI risk. | Eight templates on day one (exceeds stated need, fragile foundation)                                                |

## Risks and Caveats

1. **Schema migration**: Adding columns to the `items` table requires dropping and recreating the table since SQLite lacks `ALTER TABLE ADD COLUMN` for all cases. The ephemeral nature of `index.db` makes this safe (rehydration rebuilds from scratch), but existing installations must re-sync. The `PRAGMA user_version` guard handles this automatically.

2. **Template section parsing fragility**: HTML comment delimiters could appear in user content. The parser uses a state machine matching only exact non-nested `BEGIN:`/`END:` patterns to reduce false positives.

3. **Section parameter validation**: Agents may pass section names that do not exist in the template definition. The template service validates section names against the active template and returns descriptive errors listing valid sections. This replaces the prior risk of dynamic tool naming collisions.

4. **ID format change**: Moving from `{prefix}{NNN}-{title_slug}` to `OP{NNN}` is a breaking change for existing workspaces. The migration path should support both formats during a transition period.

5. **Template discovery latency**: `backlogit_list_templates` must load and parse template files on each call when no cache exists. For workspaces with many templates, consider caching template metadata in the SQLite index during rehydration.

## Learnings Applied

1. **Package-level validator instance** (from `feature-001-core-implementation.md`): Continue using cached `validator.New()` for new struct validation in `headerdef.go` and `templates.go`.

2. **Atomic file writes** (from `feature-001-core-implementation.md`): All new file writes in CLI commands follow the temp-file-then-rename pattern.

3. **Import cycle prevention** (from `feature-001-core-implementation.md`): The section parser lives in `internal/parser/` and only imports `internal/models/` for frontmatter types, not `internal/db/`.

4. **modernc.org/sqlite driver name** (from `feature-001-core-implementation.md`): Use `"sqlite"` not `"sqlite3"` in all new test files.

5. **SafeResolve absolute path comparison** (from `feature-001-core-implementation.md`): All new file operations in CLI commands go through `SafeResolve` before disk writes.

6. **errors.Is for sentinel comparison** (from review F20): Use `errors.Is(err, sql.ErrNoRows)` instead of `==` for all sentinel error checks.

7. **Pure string transformers for section operations** (from review F16): Section parse/write functions operate on in-memory content only; file I/O stays in `internal/core/`.

## Review Findings Addressed

This revision addresses all findings from the plan review (`.backlog/reviews/2026-03-30-queue-features-plan-review.md`):

| Finding | Severity | Resolution |
| ------- | -------- | ---------- |
| F1      | P0       | Status values updated to `queued`/`active` + full 7-value enum throughout |
| F2      | P0       | Dynamic MCP tools replaced with fixed tool surface + `backlogit_list_templates` discovery |
| F3      | P1       | Unit 2 explicitly updates `selectCols`/`scanArtifactRow` for all 17 columns |
| F4      | P1       | Unit 2 adds `AssignedTo`/`Owner` WHERE clause logic to `QueryItems` |
| F5      | P1       | Unit 2 adds `PRAGMA user_version` migration with drop-and-recreate strategy |
| F6      | P1       | All type identifiers normalized to lowercase snake_case with `display_name` field |
| F7      | P1       | Unit 2 drops and recreates FTS5 triggers for new columns |
| F8      | P1       | No dynamic tool names; no collision risk (fixed tool surface) |
| F9      | P1       | `migrate` removed from in-scope; deferred to future release |
| F10     | P1       | CLI uses single repeatable `--section name="content"` flag (D7) |
| F11     | P2       | `config.yaml` owns type identity; `header-def.yaml` owns field validation only (D2 updated) |
| F12     | P2       | Application service boundary at `internal/core/templates` (D8, Unit 20) |
| F13     | P2       | `FieldConfig` and `FieldDef` unified into single `FieldDef` struct |
| F14     | P2       | No mirror types; `internal/mcp` imports `internal/config` directly |
| F15     | P2       | Cross-cutting note: all CLI commands use package-level slog with context fields |
| F16     | P2       | Section functions are pure string transformers; file I/O stays in core |
| F17     | P2       | Delete command requires `SafeResolve` before `os.Remove` with path-traversal test |
| F18     | P2       | Three-phase delivery plan added to dependency graph section |
| F19     | P2       | Default templates reduced from 8 to 3 (task, bug, epic) |
| F20     | P2       | `errors.Is` used for all sentinel comparisons |
| F21     | P3       | `context.Context` added as first parameter to `LoadHeaderDef` and `LoadTemplates` |
| F22     | P3       | Section parser uses exact non-nested markers only |
| F23     | P3       | `ErrSectionNotFound`, `ErrMalformedDoc`, `ErrTypeNotFound` added to sentinel errors |

## Standards Check

| Principle                      | Compliance | Notes                                                                            |
| ------------------------------ | ---------- | -------------------------------------------------------------------------------- |
| I. Type-Safe Go                | ✅          | All new structs use validator tags; `FieldDef` unified (F13); no `any` without justification |
| II. MCP Protocol Fidelity      | ✅          | All tools registered unconditionally as fixed surface; `backlogit_list_templates` enables discovery; uninitialized workspace returns descriptive errors |
| III. Test-First Development    | ✅          | Every unit specifies test files and verification criteria first                  |
| IV. Workspace Containment      | ✅          | All file ops route through SafeResolve; section writes validate paths; delete uses SafeResolve (F17) |
| V. Structured Observability    | ✅          | All CLI commands and MCP tools use slog with package context fields (F15)        |
| VI. Single-Binary Simplicity   | ✅          | No new external dependencies; templates are embedded in the binary for defaults  |
| VII. CQRS Architecture         | ✅          | Markdown remains source of truth; SQLite columns added for query efficiency      |
| VIII. Git-Friendly Persistence | ✅          | Section delimiters are HTML comments; slice fields use stable YAML serialization |
| IX. Agent Context Efficiency   | ✅          | Section-aware get returns targeted content; `backlogit_list_templates` provides metadata without file reads |
