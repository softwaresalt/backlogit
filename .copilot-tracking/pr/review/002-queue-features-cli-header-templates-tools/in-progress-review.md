<!-- markdownlint-disable-file -->
# PR Review Status: 002-queue-features-cli-header-templates-tools

## Review Status

* Phase: 2 — Analyzing Changes
* Last Updated: 2026-03-31T00:15:36Z
* Summary: Feature 002 — Queue Features: CLI Commands, Header Definitions, Templates, and Section-Aware MCP Tools

## Branch and Metadata

* Normalized Branch: `002-queue-features-cli-header-templates-tools`
* Source Branch: `002-queue-features-cli-header-templates-tools`
* Base Branch: `main`
* Commits: 4
  * `5d65566` feat(cli,mcp): implement CLI commands and MCP tool expansion for TASK-002
  * `c206708` test(mcp): revise harness for section-aware fixed MCP tools
  * `7a5a54e` test: add TASK-002 TDD harnesses for all 21 subtasks
  * `d5ba5c2` refactor(models): rename status enums todo→queued, in_progress→active
* Linked Work Items: TASK-002 (all subtasks 002.01–002.06)
* Total Files Changed: 83 (+7093 / -63)
* PR Reference Generated: `.copilot-tracking/pr/review/002-queue-features-cli-header-templates-tools/pr-reference.xml` (279 KB)

## Phase 1 Action Log

* ✅ Tracking directory created: `.copilot-tracking/pr/review/002-queue-features-cli-header-templates-tools/`
* ✅ `pr-ref-gen.sh` not present; generated `pr-reference.xml` via `git diff main...HEAD`
* ✅ Diff parsed: 83 files changed, 7093 insertions, 63 deletions
* ✅ Commits enumerated (4 commits since `main`)

## Diff Mapping

| File | Type | New Lines | Notes |
|------|------|-----------|-------|
| `internal/models/artifact.go` | modified | ~1-60 | Status enum rename + new fields (queue, sprint, assignedTo, acceptanceCriteria) |
| `internal/models/frontmatter.go` | modified | 1-41 | New frontmatter serialization helpers |
| `internal/db/schema.go` | modified | ~1-80 | New columns for queue fields |
| `internal/db/queries.go` | modified | ~1-200 | New query functions for list/search/delete/upsert |
| `internal/config/headerdef.go` | added | 1-95 | header-def.yaml schema + loader |
| `internal/config/templates.go` | added | 1-148 | Template schema + loader + GetTemplateForType |
| `internal/config/defaults.go` | modified | ~1-350 | WriteDefaults generates header-def.yaml + 8 default templates |
| `internal/core/artifacts.go` | modified | ~1-340 | FindArtifactPath, WriteArtifactFile, ID-immutability |
| `internal/core/templates/service.go` | added | 1-185 | Template service: Resolve, Create, Update, GetSection, ListTemplates |
| `internal/core/workspace.go` | modified | ~1-10 | Minor update for new defaults |
| `internal/parser/sections.go` | added | 1-78 | ParseSections + WriteSections for BEGIN/END tag markers |
| `internal/cli/add.go` | added | 1-70 | `backlogit add` command |
| `internal/cli/list.go` | added | 1-68 | `backlogit list` command |
| `internal/cli/get.go` | added | 1-77 | `backlogit get` command |
| `internal/cli/update.go` | added | 1-130 | `backlogit update` command |
| `internal/cli/move.go` | added | 1-57 | `backlogit move` command |
| `internal/cli/delete.go` | added | 1-55 | `backlogit delete` command |
| `internal/cli/search.go` | added | 1-46 | `backlogit search` command |
| `internal/cli/query.go` | added | 1-39 | `backlogit query` command |
| `internal/cli/status_cmd.go` | added | 1-47 | `backlogit status` command |
| `internal/cli/root.go` | modified | 1-9 | Register all new commands |
| `internal/mcp/server.go` | modified | ~1-80 | toolNames field + addTool helper + RegisterSectionAwareTools |
| `internal/mcp/tools.go` | modified | ~1-320 | 4 new handlers + section params on existing tools |
| `internal/mcp/dynamic.go` | added | 1-84 | RegisterSectionAwareTools, handleListTemplates, ListTools, ParseSectionsParam |
| `.backlog/tasks/task-002.*` | added/modified | various | All 21 TASK-002 subtask files |
| `.backlog/plans/2026-03-30-queue-features-plan.md` | added | 1-712 | Implementation plan |
| `.backlog/reviews/2026-03-30-queue-features-plan-review.md` | added | 1-232 | Plan review doc |
| `.copilot-tracking/harness-manifest.md` | added | 1-88 | TDD harness manifest |
| `tests/contract/tools_expansion_test.go` | added | 1-225 | Contract tests for new tools |
| `tests/integration/workflow_test.go` | added | 1-292 | Full workflow integration tests |
| *All `*_test.go` files* | added | various | Unit tests per package |

## Instruction Files Reviewed

* `.github/instructions/go.instructions.md`: Applies to all `**/*.go` — enforces GoDoc, golangci-lint, error handling, table-driven tests, path safety
* `.github/instructions/mcp-go.instructions.md`: Applies to `**/*.go` — 5-step handler pattern, read-only SQL gate, error sentinels
* `.github/instructions/markdown.instructions.md`: Applies to `**/*.md` — frontmatter required, heading levels, no em dashes
* `.github/instructions/backlogit-constitution.instructions.md`: Applies to `**` — CQRS, TDD, workspace containment, single binary, no panics

## Phase 2 Findings

### 🔴 P0 — `handleUpdateItem` loses all updates (data loss bug)

**File**: `internal/mcp/tools.go` lines 288–308

`handleUpdateItem` calls `core.UpdateArtifact` (which reads from disk, mutates in-memory, validates) but **never calls `WriteArtifactFile` or `UpsertItem`**. The result: the updated struct is returned to the caller as JSON, but the file on disk and the SQLite index are never changed. Every `backlogit_update_item` MCP call silently drops all updates. The CLI `update` command does not have this bug (it calls both helpers).

Compare with `handleMoveItem` (lines 200–214) which correctly calls `UpdateArtifact → FindArtifactPath → WriteArtifactFile → UpsertItem`.

**Fix**: After `UpdateArtifact` succeeds, add `FindArtifactPath → WriteArtifactFile → UpsertItem` calls, matching the `handleMoveItem` pattern.

---

### 🟠 P1 — `backlogit_create_item` `sections` param is a no-op

**File**: `internal/mcp/tools.go` lines 255–286 and `internal/mcp/dynamic.go` lines 41–51

The `sections` parameter is declared in the tool schema (line 42 of tools.go) but `handleCreateItem` never reads or applies it. `handleCreateItemSections` in dynamic.go contains `_ = tmpl` and always returns `("", nil)`. An agent passing `sections: {"description": "..."}` to `backlogit_create_item` gets back an artifact with no section content.

**Fix**: In `handleCreateItem`, after parsing standard opts, call `ParseSectionsParam(request.Params.Arguments)`. If sections are non-empty and a `templateSvc` is available on the server, delegate to `templateSvc.Create(...)`. Otherwise, build the section body via `parser.WriteSections(tmpl.Body, sections)` and prepend it as `WithDescription`.

---

### 🟠 P1 — `backlogit_get_item` `section` param is unused; returns DB row not file body

**File**: `internal/mcp/tools.go` lines 239–252

The `section` parameter is declared in the schema but `handleGetItem` returns `db.GetItem(...)` which reads the SQLite cache row. The Description field in the cache is not populated by `UpsertItem` (it only stores frontmatter fields). An agent calling `backlogit_get_item` with `section: "acceptance-criteria"` gets no section data.

**Fix**: When `section` is specified, fall back to reading the file from disk, parse sections, and return the named section. When no `section` is specified, the current DB-based behavior is acceptable.

---

### 🟡 P2 — `handleMoveItem` triple-walk (performance)

**File**: `internal/mcp/tools.go` lines 187–214

`core.UpdateArtifact` → `findArtifact` (walk #1) then `core.FindArtifactPath` (walk #2) are called sequentially. Both walk the entire workspace. This is a known design choice documented in the checkpoint, but it's worth a follow-up task to introduce a `FindArtifactWithPath` helper that returns both the struct and path in one walk.

---

### 🟡 P2 — CLI `update` silently swallows `WriteSections` error

**File**: `internal/cli/update.go` lines 100–107

When `parser.WriteSections` returns an error (section not found in body), the code catches the error and falls back to appending new section markers rather than propagating the error. While the integration test expects this behavior, it means a typo in a section name silently creates a new (orphan) section instead of returning an actionable error message. The CLI `add` command has no such fallback — only `update` silently swallows it.

---

## Review Items

### 🔍 In Review — All Findings (27 merged, sorted by severity)

**P0 (2):**
* F-001: `handleUpdateItem` never persists to disk or DB
* F-002: `handleCreateItem` never indexes to SQLite

**P1 (10):**
* F-003: `handleUpdateItem` drops 4 declared params
* F-004: `handleCreateItem` drops 7 declared params
* F-005: `sections` param no-op on create
* F-006: `section` param no-op on get
* F-007: `sections` schema type mismatch (string vs object)
* F-008: delete file before DB delete — partial failure risk
* F-009: `queue`/`acceptance_criteria` missing from schema + queries
* F-010: `templateSvc` always nil → `list_templates` returns `[]`
* F-011: No schema migration for existing `index.db`
* F-023: Frontmatter CRLF silently drops all metadata

**P2 (11):**
* F-012: Triple filesystem walk on move/update
* F-013: CLI `update` swallows `WriteSections` error
* F-014: CLI `add` drops all `--section` flags except `description`
* F-015: CLI `get --section` silent empty on missing section
* F-016: Frontmatter map construction duplicated
* F-017: `RootPath` not absolute in `NewWorkspace`
* F-018: Section contract tests are assertion-free stubs
* F-019: FTS5 triggers don't index new columns
* F-024: Section parser silent overwrite on duplicate names
* F-025: Mixed CRLF/LF from section parser/writer on Windows
* F-026: Template loader rejects CRLF template files

**P3 (4):**
* F-020: `validator.New()` per template parse
* F-021: Missing `//nolint:errcheck` on `os.Remove`
* F-022: Non-atomic write in `writeFileIfNotExists`
* F-027: `SectionDef.Required` never enforced

### ✅ Approved for PR Comment

*(pending user decisions in Phase 3)*

### ❌ Rejected / No Action

*(pending user decisions in Phase 3)*

## Review Artifact

[`.backlog/reviews/2026-03-31-feature-002-code-review.md`](.backlog/reviews/2026-03-31-feature-002-code-review.md)

## Next Steps

* [ ] Phase 3: Present P0+P1 findings to user for decisions (fix/reject/defer)
* [ ] Phase 4: Generate `handoff.md` and create PR
