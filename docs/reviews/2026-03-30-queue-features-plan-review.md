---
chunk_strategy: h1-h2-h3
description: ""
doc_type: review
docline:
    date: 2026-03-30T00:00:00Z
    gate: fail
    plan: .backlog/exec-plans/2026-03-30-queue-features-plan.md
    reviewers:
        - constitution-reviewer
        - go-quality-reviewer
        - architecture-strategist
        - scope-boundary-auditor
    revision: 2
ingested_at: "2026-06-26T02:33:53Z"
schema_version: "1.0"
source: docs/reviews/2026-03-30-queue-features-plan-review.md
title: 'Plan Review: Queue Features — CLI Commands, Header Definitions, Templates, and Dynamic Tools'
---

# Plan Review: Queue Features (Revision 2)

## Gate Decision: FAIL

The plan contains 2 P0 critical findings and 8 P1 high-priority findings that must be resolved before implementation proceeds. The status enum conflict (F1) would produce artifacts that fail validation immediately. The dynamic MCP tool visibility gap (F2) violates a constitutional mandate.

## Summary

| Severity              | Count | Action Required                    |
|-----------------------|-------|------------------------------------|
| P0 (Critical)         | 2     | Must fix before any implementation |
| P1 (High)             | 8     | Should fix before implementation   |
| P2 (Moderate)         | 10    | User discretion                    |
| P3 (Low / Advisory)   | 3     | Informational                      |
| **Total (deduplicated)** | **23** | From 49 raw findings across 4 reviewers |

## Findings

### P0: Critical (must fix before proceeding)

#### F1: Status Enum Conflict Between Plan and Codebase

* **Unit(s):** Unit 6, Unit 7, Unit 21
* **Reviewers:** Constitution Reviewer (C-01), Go Quality Reviewer (GQ-001), Scope Boundary Auditor (SB-03)
* **Issue:** The `header-def.yaml` example in Unit 6 declares status values `[To-Do, In-Progress, Blocked, Done]` with default `To-Do`. The codebase already uses `queued` and `active` (renamed in commit `d5ba5c2`). The validator tag enforces `oneof=queued active blocked review done accepted rejected`. Any artifact created from the plan's example fails `Validate()` immediately. Unit 21's integration test also uses `--status in_progress` which is not a valid value.
* **Recommendation:** Update the plan's `header-def.yaml` example to use `[queued, active, blocked, review, done, accepted, rejected]` with default `queued`. Update Unit 21 step 5 to use `--status active`. Update Unit 7's `WriteDefaults` to emit the correct enum values. The plan also silently drops `review`, `accepted`, and `rejected` from its 4-value set, which would orphan existing artifacts.

#### F2: Dynamic MCP Tools Violate Unconditional Visibility Mandate

* **Unit(s):** Unit 20
* **Reviewer:** Constitution Reviewer (C-05)
* **Issue:** Unit 20 generates dynamic MCP tools from templates at server startup. If the workspace is not initialized (no `.backlogit/`, no templates), dynamic tools are absent, violating the constitution: "all MCP tools MUST be unconditionally visible to every connected agent regardless of workspace state." An agent connecting before `init` sees fewer tools than after.
* **Recommendation:** Register a fixed set of dynamic tool stubs (e.g., `backlogit_create_task`, `backlogit_create_bug`) unconditionally using default template definitions. When the workspace is uninitialized, these tools return a descriptive error rather than being absent. Alternatively, amend the constitution to exempt dynamic tools with documented rationale.

### P1: High (should fix before proceeding)

#### F3: `selectCols` and `scanArtifactRow` Not Updated for New Fields

* **Unit(s):** Unit 2
* **Reviewer:** Go Quality Reviewer (GQ-002)
* **Issue:** `selectCols` in `queries.go` selects 11 columns. Adding 6 new columns to the table without updating `selectCols` means `scanArtifactRow` will never receive the new data. The expansion tests are confirmed failing because new fields return empty strings.
* **Recommendation:** Unit 2 must explicitly state: update `selectCols` to include all 17 columns, update `scanArtifactRow` to scan all 17, verify `GetItem`/`QueryItems`/`SearchItems` all route through the updated constant.

#### F4: `QueryItems` Ignores `AssignedTo` and `Owner` Filters

* **Unit(s):** Unit 2
* **Reviewer:** Go Quality Reviewer (GQ-003)
* **Issue:** `QueryFilters` has `AssignedTo` and `Owner` fields (added in harness prep) but `QueryItems` never reads them in the WHERE clause builder. Filter calls silently return all items.
* **Recommendation:** Add WHERE clause conditions for `filters.AssignedTo` and `filters.Owner` in `QueryItems`. The fields exist; the query logic is missing.

#### F5: Schema Migration Strategy Missing

* **Unit(s):** Unit 2
* **Reviewer:** Go Quality Reviewer (GQ-004)
* **Issue:** `EnsureSchema` uses `CREATE TABLE IF NOT EXISTS`, which is a no-op on existing databases. New columns are never added to existing `index.db` files. Running `sync` will fail with "table items has no column named assigned_to."
* **Recommendation:** Add a `PRAGMA user_version` guard. When `user_version < 2`, drop and recreate the items table (acceptable since `index.db` is ephemeral), then rehydrate. Or use `ALTER TABLE ADD COLUMN` for nullable columns.

#### F6: Artifact Type Identifier Casing Inconsistency

* **Unit(s):** Unit 6, Unit 7
* **Reviewer:** Constitution Reviewer (C-03)
* **Issue:** The plan uses PascalCase/hyphenated type names (Epic, Feature, Sub-Epic, User-Story). The existing codebase uses lowercase: `task`, `story`, `bug`, `epic`. Case-sensitive map lookups and validator constraints will fail.
* **Recommendation:** Normalize type identifiers to lowercase snake_case (`user_story`, `sub_task`, `sub_epic`). Use PascalCase as a separate `display_name` field in `header-def.yaml`.

#### F7: FTS5 Triggers Not Updated for New Columns

* **Unit(s):** Unit 2
* **Reviewer:** Go Quality Reviewer (GQ-008)
* **Issue:** FTS5 sync triggers (`items_ai`, `items_ad`, `items_au`) only index `id`, `title`, `description`. Adding `labels` and `dependencies` to FTS5 requires updating all three triggers. Without this, `TestSearchItems_MatchesLabels` fails.
* **Recommendation:** Unit 2 must update all three FTS5 triggers and the `CREATE VIRTUAL TABLE items_fts` statement. Since `CREATE TRIGGER IF NOT EXISTS` is a no-op for existing triggers, the migration must `DROP TRIGGER` old versions first.

#### F8: Dynamic Tool Namespace Collision Risk

* **Unit(s):** Unit 20
* **Reviewer:** Constitution Reviewer (C-06)
* **Issue:** Dynamic tools use names like `backlogit_create_{type}`. User-defined template types could collide with static tool names. The tool surface becomes non-deterministic across sessions if templates change between restarts.
* **Recommendation:** Add a namespace prefix (e.g., `backlogit_tmpl_create_{type}`) to prevent collisions with static tools.

#### F9: `migrate` Command Listed but No Implementation Unit

* **Unit(s):** R1, Scope section
* **Reviewer:** Scope Boundary Auditor (SB-04)
* **Issue:** The requirements trace and in-scope list include a `migrate` command, but no implementation unit, dependency, or verification step exists. The plan is incomplete against its own stated scope.
* **Recommendation:** Either add a concrete unit for `migrate` with pass/fail verification, or remove it from R1 and the in-scope list for this release.

#### F10: Cobra Section Flags Cannot Be Registered Before Template Load

* **Unit(s):** Unit 11
* **Reviewer:** Go Quality Reviewer (GQ-010)
* **Issue:** Unit 11 says `add` exposes "section flags from templates." Cobra requires flags to be bound at command creation time, before any workspace is opened or templates loaded. This is an irreconcilable ordering problem.
* **Recommendation:** Use a single repeatable `--section` flag: `backlogit add --type task --title "Foo" --section description="content" --section acceptance-criteria="criteria"`. No template loading needed at flag registration time.

### P2: Moderate (user discretion)

#### F11: Dual Type Identity Sources

* **Unit(s):** Unit 6
* **Reviewers:** Architecture Strategist (ARCH-01), Constitution Reviewer (C-04)
* **Issue:** `header-def.yaml` introduces a second source for type identity (prefix, id_format) while `config.yaml` already owns `ArtifactTypeConfig` with `Prefix` and `NameFormat`. This creates an architectural fork with divergence risk.
* **Recommendation:** Pick one canonical owner for type identity and naming. Either keep naming in `WorkspaceConfig` and use `header-def.yaml` only for field validation, or consolidate into a single schema package.

#### F12: MCP Dynamic Layer as Integration Hub

* **Unit(s):** Unit 20
* **Reviewer:** Architecture Strategist (ARCH-02)
* **Issue:** `internal/mcp/dynamic.go` depends on template loading from `config`, section mutation from `parser`, core CRUD, and tool registration. This makes the dynamic tool layer the integration hub for both content semantics and transport semantics.
* **Recommendation:** Insert an application service boundary (e.g., `internal/core/templates` or `internal/workflows`). Let CLI and MCP both call that service, keeping `internal/mcp` as a thin registration layer.

#### F13: `FieldConfig` vs `FieldDef` Duplication

* **Unit(s):** Unit 6
* **Reviewer:** Go Quality Reviewer (GQ-011)
* **Issue:** Two structurally similar field definition types exist in `config`: `FieldConfig` (schema.go) and `FieldDef` (headerdef.go). `golangci-lint` with `dupl` will flag these.
* **Recommendation:** Unify into a single `FieldDef` struct with all fields marked `omitempty`. Remove `FieldConfig`.

#### F14: `DynamicTemplateInput` Mirror Types

* **Unit(s):** Unit 20
* **Reviewer:** Go Quality Reviewer (GQ-009)
* **Issue:** `dynamic.go` defines mirror types duplicating `config.TemplateConfig` and `config.SectionDef`. No import cycle prevents using the originals directly.
* **Recommendation:** Remove mirror types. Import `internal/config` directly and use `[]*config.TemplateConfig`.

#### F15: Missing slog Instrumentation in CLI Units

* **Unit(s):** Units 11-17
* **Reviewers:** Constitution Reviewer (C-07)
* **Issue:** None of the 9 CLI command unit descriptions explicitly mention slog usage. The constitution requires structured logging for all significant operations.
* **Recommendation:** Add cross-cutting note: all CLI commands use package-level slog logger with operation context fields.

#### F16: Section Parser Safety and File I/O Boundary

* **Unit(s):** Unit 10
* **Reviewers:** Constitution Reviewer (C-08), Architecture Strategist (ARCH-05)
* **Issue:** `WriteSections`/`WriteSection` must operate on string content (in-memory transformation), not direct file I/O. File writing responsibility stays in `internal/core/` via `SafeResolve`. The plan should also use a scanner/state machine for section delimiters rather than loose regex to avoid false positives.
* **Recommendation:** Clarify that section functions are pure string transformers. File writing stays in `core`.

#### F17: Delete Command Path Traversal Risk

* **Unit(s):** Unit 16
* **Reviewer:** Constitution Reviewer (C-14)
* **Issue:** Delete removes artifact files without specifying SafeResolve path validation. A crafted artifact ID could delete files outside `.backlogit/`.
* **Recommendation:** Require SafeResolve before `os.Remove`. Add a path-traversal test case.

#### F18: 21-Unit Monolithic Batch

* **Unit(s):** All
* **Reviewer:** Scope Boundary Auditor (SB-08)
* **Issue:** 21 units is too monolithic for a safe first increment. User-visible value doesn't appear until after a long chain of foundational units.
* **Recommendation:** Split into smaller deliverables: (1) header definitions + minimal templates + `add/list/get`, (2) section-aware `update/move/delete/search/query/status`, (3) dynamic MCP generation.

#### F19: 8 Default Templates May Be YAGNI

* **Unit(s):** Unit 9
* **Reviewer:** Scope Boundary Auditor (SB-06)
* **Issue:** The queue asks for "common operation types, such as tasks, bugs, features." Requiring 8 templates on day one exceeds the stated need.
* **Recommendation:** Start with Task, Bug, and Epic. Add remaining templates once the loader and section writer are stable.

#### F20: `err == sql.ErrNoRows` Should Use `errors.Is`

* **Unit(s):** Unit 2
* **Reviewer:** Go Quality Reviewer (GQ-012)
* **Issue:** `queries.go` line 105 uses `==` for sentinel comparison instead of `errors.Is`. Flagged by `staticcheck`.
* **Recommendation:** Change to `errors.Is(err, sql.ErrNoRows)` as part of Unit 2 work.

### P3: Low (advisory)

#### F21: `LoadHeaderDef` and `LoadTemplates` Missing `context.Context`

* **Unit(s):** Unit 6, Unit 8
* **Reviewer:** Go Quality Reviewer (GQ-005)
* **Issue:** Existing `Load` in `loader.go` accepts `context.Context`. The new stubs omit it, breaking the established pattern.
* **Recommendation:** Add `context.Context` as first parameter to both functions.

#### F22: Nested HTML Comment Handling Over-Engineered

* **Unit(s):** Unit 10
* **Reviewer:** Scope Boundary Auditor (SB-11)
* **Issue:** The queue only needs deterministic BEGIN/END markers, not a general-purpose HTML comment parser.
* **Recommendation:** Limit to exact non-nested markers. Defer broader tolerance until a real example demands it.

#### F23: Missing Sentinel Errors for New Domains

* **Unit(s):** Unit 6, Unit 10
* **Reviewer:** Go Quality Reviewer (GQ-014)
* **Issue:** Parser and extended config domains lack typed errors. Callers cannot distinguish "section not found" from "malformed document."
* **Recommendation:** Add `ErrSectionNotFound`, `ErrMalformedDoc`, `ErrTypeNotFound` to `internal/errors/errors.go`.

## Reviewer Attribution

| Finding   | Reviewer(s)                                                     | Model(s)                   |
|-----------|-----------------------------------------------------------------|----------------------------|
| F1        | Constitution Reviewer, Go Quality Reviewer, Scope Boundary Auditor | Claude Opus, Claude Haiku, GPT-5.4 |
| F2        | Constitution Reviewer                                           | Claude Opus                |
| F3        | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F4        | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F5        | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F6        | Constitution Reviewer                                           | Claude Opus                |
| F7        | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F8        | Constitution Reviewer                                           | Claude Opus                |
| F9        | Scope Boundary Auditor                                          | GPT-5.4                    |
| F10       | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F11       | Architecture Strategist, Constitution Reviewer                  | GPT-5.4, Claude Opus       |
| F12       | Architecture Strategist                                         | GPT-5.4                    |
| F13       | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F14       | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F15       | Constitution Reviewer                                           | Claude Opus                |
| F16       | Constitution Reviewer, Architecture Strategist                  | Claude Opus, GPT-5.4       |
| F17       | Constitution Reviewer                                           | Claude Opus                |
| F18       | Scope Boundary Auditor                                          | GPT-5.4                    |
| F19       | Scope Boundary Auditor                                          | GPT-5.4                    |
| F20       | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F21       | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |
| F22       | Scope Boundary Auditor                                          | GPT-5.4                    |
| F23       | Go Quality Reviewer                                             | Claude Haiku (go-engineer) |

## Next Steps

Gate decision is **FAIL** (2 P0 + 8 P1 findings). The plan must be revised before implementation proceeds:

1. **Fix F1 (P0):** Update all status enum references to match `queued`/`active` and the full 7-value set
2. **Fix F2 (P0):** Decide on unconditional dynamic tool registration or amend the constitution
3. **Fix F3-F10 (P1):** Address DB schema gaps, type casing, FTS5 triggers, namespace collisions, missing `migrate` unit, and Cobra flag architecture
4. After revisions, re-run the plan review gate to confirm PASS/ADVISORY before decomposition
