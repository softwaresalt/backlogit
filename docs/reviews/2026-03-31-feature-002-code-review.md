---
chunk_strategy: h1-h2-h3
description: Structured code review of branch 002-queue-features-cli-header-templates-tools via 5 review personas
doc_type: review
docline:
    author: review-skill
    ms.date: 2026-03-31T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:33:53Z"
schema_version: "1.0"
source: docs/reviews/2026-03-31-feature-002-code-review.md
title: 'Feature 002 Code Review: Queue Features CLI, Templates, and MCP Tools'
---

# Feature 002 Code Review

**Branch**: `002-queue-features-cli-header-templates-tools` vs `main`
**Date**: 2026-03-31
**Personas**: Go Quality Reviewer, Constitution Reviewer, MCP Protocol Reviewer, SQLite Reviewer, Learnings Researcher
**Scope**: 83 files changed, 7093 insertions, 63 deletions
**Raw Findings**: 32 (before merge/dedup)
**Merged Findings**: 23

---

## Summary

The feature delivers a complete CLI command suite, header-def + template config system, section parser, and
MCP tool expansion. Core test coverage is solid and all 12 packages pass `go test ./...`. However, the review
identified **2 P0 data-loss bugs** and **9 P1 contract violations** that must be resolved before merge.

### Severity Distribution

| Severity | Count | Description |
|----------|-------|-------------|
| P0 | 2 | Data-loss bugs — silent persistence failures |
| P1 | 9 | Contract violations — documented params are no-ops, schema gaps |
| P2 | 8 | Quality issues — duplicate code, missing errors, test gaps |
| P3 | 3 | Minor — lint, conventions |

---

## P0 — Must Fix Before Merge

### F-001: `handleUpdateItem` never persists changes to disk or SQLite

**File**: `internal/mcp/tools.go` lines 288–308
**Confirmed by**: Go Quality Reviewer, Constitution Reviewer, MCP Protocol Reviewer (3/3 personas)

`core.UpdateArtifact` mutates an in-memory `*models.Artifact` and returns it, but `handleUpdateItem` never
calls `core.WriteArtifactFile` or `db.UpsertItem`. The Markdown source-of-truth file and SQLite cache are
both untouched. Every `backlogit_update_item` call silently succeeds while discarding all mutations.

Compare `handleMoveItem` (lines 187–215) which correctly executes `UpdateArtifact → FindArtifactPath →
WriteArtifactFile → UpsertItem`.

**Fix**:

```go
filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
if err != nil {
    return InternalError(fmt.Sprintf("find artifact: %v", err)), nil
}
if err := core.WriteArtifactFile(artifact, filePath); err != nil {
    return InternalError(fmt.Sprintf("write artifact: %v", err)), nil
}
if err := db.UpsertItem(ctx, s.Workspace.DB, artifact); err != nil {
    return InternalError(fmt.Sprintf("upsert item: %v", err)), nil
}
```

---

### F-002: `handleCreateItem` never indexes the new artifact in SQLite

**File**: `internal/mcp/tools.go` lines 255–286
**Confirmed by**: MCP Protocol Reviewer

`core.CreateArtifact` persists the Markdown file to disk, but `handleCreateItem` never calls `db.UpsertItem`.
Newly created items are invisible to `backlogit_get_item`, `backlogit_list_items`, `backlogit_search_items`,
and `backlogit_query_sql` until an explicit `backlogit_sync_index` call.

**Fix**: After `core.CreateArtifact` succeeds, add:

```go
if err := db.UpsertItem(ctx, s.Workspace.DB, artifact); err != nil {
    return InternalError(fmt.Sprintf("index artifact: %v", err)), nil
}
```

---

## P1 — Contract Violations

### F-003: `handleUpdateItem` silently drops 4 declared parameters

**File**: `internal/mcp/tools.go` lines 47–59, 297–302

The `backlogit_update_item` schema declares `assigned_to`, `owner`, `labels`, and `commit`, but the handler
only copies `title`, `status`, `description`, `sprint`, and `priority`. `core.UpdateArtifact` already supports
all four fields. Compound with F-001: even if wired, the data would not be persisted.

**Fix**: Extend the extraction loop; add `assigned_to`, `owner`, `commit` as string fields; split `labels` as `[]string`.

---

### F-004: `handleCreateItem` silently drops 7 declared parameters

**File**: `internal/mcp/tools.go` lines 28–43, 268–281

The `backlogit_create_item` schema declares `assigned_to`, `owner`, `labels`, `dependencies`, `references`,
`commit`, and `sections`. The handler only converts `status`, `description`, `parent_id`, and `sprint` into
`core.Option` values. Five declared parameters are ghost params.

**Fix**: Add `WithAssignedTo`, `WithOwner`, `WithLabels`, `WithDependencies`, `WithReferences`, `WithCommit`
option conversions, then fix `sections` (see F-005).

---

### F-005: `handleCreateItem` sections parameter is a documented no-op

**File**: `internal/mcp/tools.go` lines 255–286 and `internal/mcp/dynamic.go` lines 41–51
**Confirmed by**: Go Quality Reviewer, Constitution Reviewer, MCP Protocol Reviewer (3/3 personas)

`ParseSectionsParam` is implemented in `dynamic.go` but `handleCreateItem` never calls it.
`handleCreateItemSections` always returns `("", nil)` via `_ = tmpl`. Agents passing section content
to `backlogit_create_item` receive an artifact with no section content.

**Fix**: In `handleCreateItem`, parse sections and apply via `templateSvc.Create` or `parser.WriteSections`.
Delete or implement `handleCreateItemSections`.

---

### F-006: `handleGetItem` section parameter is a documented no-op

**File**: `internal/mcp/tools.go` lines 239–252
**Confirmed by**: Go Quality Reviewer, Constitution Reviewer, MCP Protocol Reviewer (3/3 personas)

The `backlogit_get_item` schema declares `section` as "Extract a named section from the body". The handler
reads only `id`, then returns `db.GetItem(...)` — a cache row without body content. The `section` parameter
is never read.

**Fix**: If `section` is provided, resolve the file path, read the Markdown, call `parser.ParseSections`,
and return the named section content. Otherwise, fall back to the current DB-based response.

---

### F-007: `sections` declared as `string` in schema but expected as JSON object by handler

**File**: `internal/mcp/tools.go` lines 42–43, 59–60
**Confirmed by**: MCP Protocol Reviewer

Both `backlogit_create_item` and `backlogit_update_item` register `sections` via `mcplib.WithString`.
`ParseSectionsParam` accepts both string (JSON) and `map[string]any`, but the MCP schema tells clients
this is a string. This ambiguity creates a contract mismatch.

**Fix**: Use a consistent wire format. Either document sections as a JSON-encoded string (and update
all tests and docs) or switch to an object-type schema parameter if the SDK supports it.

---

### F-008: `handleDeleteItem` deletes source file before updating SQLite index

**File**: `internal/mcp/tools.go` lines 217–236

`os.Remove(filePath)` executes before `db.DeleteItem`. If `db.DeleteItem` fails, the Markdown source-of-truth
file is permanently gone but the SQLite cache still advertises the artifact.

**Fix**: Attempt `db.DeleteItem` first. Only call `os.Remove` after the DB delete succeeds. If atomicity
is required, use a rename-to-tombstone approach and clean up after DB confirmation.

---

### F-009: `queue` and `acceptance_criteria` columns missing from schema and query layer

**File**: `internal/db/schema.go` lines 18–65 and `internal/db/queries.go` lines 25–155
**Confirmed by**: SQLite Reviewer (both SQL-001 and SQL-002)

The schema adds `assigned_to`, `owner`, `labels`, etc., but never adds `queue` or `acceptance_criteria`
columns. `selectCols`, `scanArtifactRow`, and `UpsertItem` also omit them. These fields exist on the
`models.Artifact` struct but are silently dropped by the projection layer.

**Fix**: Add `queue TEXT` and `acceptance_criteria TEXT` columns to the `items` table DDL and include
them in `selectCols`, `scanArtifactRow`, and `UpsertItem`. Add to `items_fts` if full-text search is desired.

---

### F-010: `templateSvc` always nil — `backlogit_list_templates` always returns `[]`

**File**: `internal/mcp/server.go` line 46
**Confirmed by**: Constitution Reviewer

`NewServer` calls `RegisterSectionAwareTools(s, nil)`. `handleListTemplates` short-circuits on nil
and returns `"[]"`, making `backlogit_list_templates` permanently non-functional.

**Fix**: Construct a live `*templates.Service` from the workspace templates directory and pass it to
`RegisterSectionAwareTools`. Add a nil-guard returning a no-op service when the templates dir is absent.

---

### F-011: No schema migration for existing `index.db` files

**File**: `internal/db/schema.go`
**Confirmed by**: SQLite Reviewer (SQL-003)

`EnsureSchema` uses only `CREATE TABLE IF NOT EXISTS`. Existing databases receive no `ALTER TABLE ADD COLUMN`
for new fields. Workspaces upgraded from Feature 001 silently keep the old schema.

**Fix**: Add `ALTER TABLE items ADD COLUMN IF NOT EXISTS` for each new column, and version the FTS table
definition to force a rebuild when triggers change. Or add a schema version check that drops and recreates
the table + triggers on version mismatch.

---

## P2 — Quality Issues

### F-012: Triple filesystem walk per move/update operation

**Files**: `internal/mcp/tools.go` lines 200–207, `internal/cli/update.go` line 71–78
`core.UpdateArtifact` calls `findArtifact` (walk #1), then callers call `FindArtifactPath` (walk #2).
`cli/update.go` calls `FindArtifactPath` before `UpdateArtifact`, adding a third walk.

**Fix**: Refactor `UpdateArtifact` to return `(artifact, filePath, error)` so callers avoid the redundant walk.

---

### F-013: CLI `update` silently converts `WriteSections` error to append behavior

**File**: `internal/cli/update.go` lines 100–107

When `parser.WriteSections` errors (section not found), the code appends new markers rather than propagating
the error. A typo in a section name silently creates a new orphan section.

**Fix**: Remove the fallback and propagate the error. Add explicit `--create-section` flag if append behavior is desired.

---

### F-014: CLI `add` drops all `--section` flags except `description`

**File**: `internal/cli/add.go` lines 45–54

The add command only processes `--section description=...`. All other section names are silently discarded.

**Fix**: Collect all section flags into a map and delegate to `templates.Service.Create`.

---

### F-015: CLI `get --section <name>` returns empty output when section is absent

**File**: `internal/cli/get.go` lines 56–60

Missing section produces silent empty output with exit code 0. Should return an error.

**Fix**: Return `fmt.Errorf("section %q not found in artifact %s", section, id)` when the lookup fails.

---

### F-016: Frontmatter map construction duplicated between `CreateArtifact` and `WriteArtifactFile`

**File**: `internal/core/artifacts.go` lines 139–178, 278–315

Identical field-by-field map construction exists in both functions. Adding a new `Artifact` field requires
updating both sites.

**Fix**: Extract `artifactToFrontmatter(a *models.Artifact) map[string]any` helper.

---

### F-017: `RootPath` not normalized to absolute in `NewWorkspace`

**File**: `internal/core/workspace.go` lines 22–45
**Confirmed by**: Constitution Reviewer (CR-007), Learnings Researcher (LR-001)

`RootPath` is stored without calling `filepath.Abs`. Compound learning #4 from Feature 001 explicitly flags
this as a path traversal risk when `workspaceRoot` is relative.

**Fix**: `absRoot, err := filepath.Abs(rootPath)` in `NewWorkspace` before storing.

---

### F-018: Section-aware contract tests are assertion-free stubs

**File**: `tests/contract/tools_expansion_test.go` lines 179–225
**Confirmed by**: Constitution Reviewer (CR-008)

`TestCreateItem_ContractAcceptsSectionsParam`, `TestGetItem_ContractAcceptsSectionParam`, and
`TestUpdateItem_ContractAcceptsSectionsParam` build request structs and assert only that the argument
map contains the key they just inserted — no handler is ever invoked. These tests would pass regardless
of whether the handlers work.

**Fix**: Rewrite to call handlers via a live test server and assert on returned JSON content.

---

### F-019: FTS5 triggers do not index new searchable columns

**File**: `internal/db/schema.go`
**Confirmed by**: Learnings Researcher (LR-002)

`items_ai`, `items_au`, `items_ad` triggers only sync `id`, `title`, `description`, and `labels` to FTS.
Fields like `sprint`, `assigned_to`, and `priority` are not searchable via FTS.

**Fix** (advisory): Expand FTS5 column list and update all three triggers to include desired searchable fields.

---

## P3 — Minor / Conventions

### F-020: `validator.New()` allocated per template parse, not cached

**File**: `internal/config/templates.go` line 142, `internal/config/headerdef.go` line 56

Should be a package-level `var validate = validator.New()` per project convention (see `models/artifact.go:9`).

---

### F-021: `os.Remove(tmpPath)` missing `//nolint:errcheck` in `CreateArtifact`

**File**: `internal/core/artifacts.go` line 186

Inconsistent with every other intentional discard in the codebase; will trigger `golangci-lint errcheck`.

---

### F-022: `writeFileIfNotExists` in `defaults.go` uses non-atomic write

**File**: `internal/config/defaults.go` lines 237–242

Lower risk (init-only code path), but inconsistent with the atomic temp-file pattern used everywhere else.
Advisory only.

---

## Markdown/YAML Reviewer Findings

### F-023: Frontmatter parser silently drops all metadata on CRLF files — P1

**File**: `internal/models/frontmatter.go` lines 13–25

`ParseFrontmatter` recognizes only `"---\n"` as the opener. A file saved on Windows with CRLF line endings
starts with `"---\r\n"` and the prefix check fails, returning nil frontmatter with the entire file as body.
All metadata is silently dropped. Since this repository runs on Windows, any `.md` file edited outside the
repo (e.g., by an agent or editor writing CRLF) will be misread with zero error indication.

**Fix**: Accept both delimiters:
`strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n")` and make the closing
delimiter regex `\r?\n---` tolerant.

---

### F-024: Section parser silently overwrites duplicate section names — P2

**File**: `internal/parser/sections.go` lines 17–37

`ParseSections` stores results in a map without checking for duplicates. If the same section name appears
twice, the later occurrence silently overwrites the earlier one, discarding content without error.

**Fix**: Track seen names and return an error on the second occurrence.

---

### F-025: Section parser/writer produces mixed CRLF/LF on Windows documents — P2

**File**: `internal/parser/sections.go` lines 35–36, 48–68

`ParseSections` trims only `"\n"` leaving stray `"\r"` at boundaries in CRLF files.
`WriteSections` always inserts `"\n"` delimiters, creating mixed line endings when rewriting a CRLF document.

**Fix**: Use `strings.Trim(extracted, "\r\n")` and detect/preserve the document's native line ending style.

---

### F-026: Template loader rejects CRLF-authored template files — P2

**File**: `internal/config/templates.go` lines 92–108

`parseTemplateFile` hardcodes `"---\n"` as the frontmatter opener. CRLF template files fail to load
with a misleading "missing YAML frontmatter opener" error.

**Fix**: Apply the same CRLF-tolerant delimiter fix as F-023 to template parsing.

---

### F-027: `SectionDef.Required` is loaded but never enforced — P3

**File**: `internal/config/templates.go` lines 23–26

`Required` is metadata only — no validation enforces it at create or update time. Either enforce it or
document it as informational.

---

---

## Merge Recommendation

**Do not merge as-is.** Resolve P0 (F-001, F-002) and at minimum P1 (F-003 through F-011) before merge.
P2/P3 items can be tracked as follow-up tasks.

**Minimum merge bar:**

* [ ] F-001 — `handleUpdateItem` persistence fixed
* [ ] F-002 — `handleCreateItem` indexes to SQLite
* [ ] F-003/F-004 — Ghost params wired or explicitly removed from schema
* [ ] F-005 — Sections applied on create
* [ ] F-008 — Delete order corrected
* [ ] F-009 — `queue`/`acceptance_criteria` in schema and queries
* [ ] F-011 — Schema migration strategy documented or implemented
