<!-- markdownlint-disable-file -->
# PR Review Status: 008-mcp-cli-section-template-bug-fixes

## Review Status

* Phase: 2 — Analyze Changes
* Last Updated: 2026-03-31T22:50:37Z
* Summary: MCP & CLI Section/Template Bug Fixes — implements TASK-008 across MCP tools, CLI commands, and domain stubs

## Branch and Metadata

* Normalized Branch: `008-mcp-cli-section-template-bug-fixes`
* Source Branch: `008-mcp-cli-section-template-bug-fixes`
* Base Branch: `main`
* Commits Ahead of Main: 3 (f9da3a5, e837d27, 2f57251)
* Linked Work Items: TASK-008 and subtasks 008.01–008.06

## Phase 1 Log

* Tracking directory created: `.copilot-tracking/pr/review/008-mcp-cli-section-template-bug-fixes/`
* `pr-ref-gen.sh` not found — diff captured via `git diff main...HEAD -- internal/` (127 KB, ~3985 lines)
* 43 files changed across `internal/`, `.backlog/`, and test harnesses

## Changed Files — internal/ only

| File | Type | Notes |
|------|------|-------|
| `internal/cli/dep.go` | new | Dependency CLI — add/remove/list subcommands |
| `internal/cli/dep_test.go` | new | Tests for dep subcommands |
| `internal/cli/list_enhanced_test.go` | new | Tests for enhanced list flags |
| `internal/cli/list_grouping.go` | new | NewListEnhancedCmd, FormatTreeView, FormatGroupedView |
| `internal/cli/move.go` | modified | Calls RelocateArtifactFile + WriteArtifactFile |
| `internal/cli/move_relocate_test.go` | new | Tests for move relocation |
| `internal/cli/queue_cmd.go` | new | Queue CLI — view/move/bulk-status subcommands |
| `internal/cli/queue_cmd_test.go` | new | Tests for queue subcommands |
| `internal/cli/update.go` | modified | Bumps updated_at + calls db.UpsertItem after section writes |
| `internal/cli/update_sync_test.go` | new | Tests for update DB sync |
| `internal/config/defaults.go` | modified | Added status-based routing rules (archive/review) |
| `internal/config/loader.go` | modified | Added LoadRegistry function |
| `internal/core/archive.go` | new | ArchiveItem, UnarchiveItem, AutoArchive |
| `internal/core/archive_test.go` | new | Tests for archive operations |
| `internal/core/artifacts.go` | modified | FindArtifactPath now walks all non-hidden subdirs |
| `internal/core/commits.go` | new | LinkCommit, GetCommitLinks, AutoLinkCommits |
| `internal/core/commits_test.go` | new | Tests for commit linking |
| `internal/core/field_validation.go` | new | ValidateArtifactFields, ApplyFieldDefaults |
| `internal/core/field_validation_test.go` | new | Tests for field validation |
| `internal/core/harness_status.go` | new | ValidateTransition, ComputeParentStatus, CascadeStatusUpdate |
| `internal/core/harness_status_test.go` | new | Tests for status transitions |
| `internal/core/hierarchy.go` | new | ParseHierarchicalID, LevelForType, FormatHierarchicalID, ResolveHierarchicalPath, NextHierarchicalID |
| `internal/core/hierarchy_test.go` | new | Tests for hierarchy operations |
| `internal/core/migrate_queue.go` | new | MigrateFlatToHierarchical, RollbackMigration |
| `internal/core/migrate_queue_test.go` | new | Tests for migration |
| `internal/core/queue.go` | new | QueryQueue, MoveInQueue, BulkUpdateStatus |
| `internal/core/queue_test.go` | new | Tests for queue operations |
| `internal/core/relocate.go` | new | RelocateArtifactFile implementation |
| `internal/core/templates/service.go` | modified | Create calls db.UpsertItem; Update sets updated_at |
| `internal/core/templates/service_sync_test.go` | new | Tests for template DB sync |
| `internal/core/wit_metadata.go` | new | DescribeType, ListTypes |
| `internal/core/wit_metadata_test.go` | new | Tests for WIT metadata |
| `internal/db/dependencies.go` | new | UpsertDependency, DeleteDependency, GetDependencies, GetDependents, DetectCycle |
| `internal/db/dependencies_test.go` | new | Tests for dependency graph |
| `internal/db/queries.go` | modified | RFC3339Nano timestamps; exports ScanArtifactRow, SelectCols |
| `internal/db/schema.go` | modified | Added item_deps + commit_links tables |
| `internal/db/schema_gen.go` | new | ValidateColumnName, MapFieldTypeToSQLite, GenerateSchemaExtensions, ApplySchemaExtensions |
| `internal/db/schema_gen_test.go` | new | Tests for schema generation |
| `internal/mcp/call_tool.go` | new | CallToolForTest via InProcessClient |
| `internal/mcp/dynamic.go` | modified | Double-registration guard; templateSvc closure |
| `internal/mcp/section_bugs_test.go` | new | Contract tests for section/template MCP tools |
| `internal/mcp/server.go` | modified | templateSvc field; NewServer wires template service |
| `internal/mcp/tools.go` | modified | writeSectionsToFile helper; handleGetItem section extraction |

## Instruction Files Matched

| Instruction | Applies To | Reason |
|-------------|-----------|--------|
| `**/*.go` (go-coding-conventions) | All Go files | Primary language conventions |
| `**/*.go` (mcp-go-best-practices) | `internal/mcp/` | MCP server patterns |
| `**` (backlogit-constitution) | All files | Architecture principles |

## Review Items

### 🔍 In Review

* Pending review skill analysis

### ✅ Approved for PR Comment

* (none yet)

### ❌ Rejected / No Action

* (none yet)

## Next Steps

* [ ] Phase 2: Invoke review skill with diff scope
* [ ] Phase 3: Present findings to user
* [ ] Phase 4: Generate handoff.md and create PR
