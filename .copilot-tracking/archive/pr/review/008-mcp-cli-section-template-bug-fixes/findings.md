<!-- markdownlint-disable-file -->
# Code Review Findings: 008-mcp-cli-section-template-bug-fixes

Generated: 2026-03-31  
Personas: Go Quality, Constitution, MCP Protocol, SQLite  
Raw findings: 28 | After dedup: 25 | P1: 11 | P2: 9 | P3: 5

---

## P1 — Defects (Block Merge)

### F-001: CreateArtifact panics on nil Config
- **Source:** GQ-001
- **File:** `internal/core/artifacts.go`
- **Issue:** `ws.Config` is dereferenced without a nil check. Tests construct `core.Workspace{RootPath, DB}` with no Config, causing a panic rather than a returned error.
- **Fix:** Add `if ws == nil || ws.Config == nil { return nil, fmt.Errorf("workspace config is required") }` before reading any Config fields.

### F-002: MoveInQueue is a non-functional no-op [DUPLICATE: SQL-006, CO-007]
- **Source:** GQ-002 + CO-007 + SQL-006
- **File:** `internal/core/queue.go` ~line 138
- **Issue:** `MoveInQueue` discards `newPosition` and executes `UPDATE items SET updated_at = updated_at`, a tautological assignment that moves nothing. Returns success unconditionally. Violates Constitution Principle 10 (no dead code in shipped features).
- **Fix:** Either implement sibling reordering with a persisted `ordinal` column, or return `errors.New("queue reordering not yet implemented")` so callers aren't silently misled.

### F-003: BulkUpdateStatus leaves Markdown source files stale
- **Source:** GQ-003
- **File:** `internal/core/queue.go` ~line 145
- **Issue:** Only updates SQLite. Markdown files (the CQRS source of truth) are never rewritten, so status changes are lost on next `backlogit_sync_index`. The unused `ws *Workspace` parameter confirms the file-write path was intended but omitted.
- **Fix:** For each item, load artifact → update status/UpdatedAt → `WriteArtifactFile` → `db.UpsertItem`. Fail fast on file-write errors rather than committing DB-only changes.

### F-004: Archive/unarchive suppress index update errors
- **Source:** GQ-004
- **File:** `internal/core/archive.go` ~line 71
- **Issue:** Both `ArchiveItem` and `UnarchiveItem` discard `db.UpsertItem` errors (`_ = db.UpsertItem(...)`). A failed index update is invisible to callers, leaving filesystem and SQLite permanently desynchronised.
- **Fix:** Propagate: `if err := db.UpsertItem(...); err != nil { return nil, fmt.Errorf("sync archive state: %w", err) }`. Consider rolling back the file move on failure.

### F-005: Queue CLI handlers are shipped as no-ops [DUPLICATE: CO-006]
- **Source:** GQ-005 + CO-006
- **File:** `internal/cli/queue_cmd.go`
- **Issue:** `queue view`, `queue move`, and `queue bulk-status` all return `nil` from `RunE` without opening a workspace or calling any core function. The CLI advertises working operations but silently does nothing. Violates Constitution Principle 10.
- **Fix:** Wire each `RunE` to the corresponding core function (`core.QueryQueue`, `core.MoveInQueue`, `core.BulkUpdateStatus`), or remove commands from the binary until implemented.

### F-006: ⚠️ SECURITY — Path traversal via `archived_from` in UnarchiveItem
- **Source:** CO-001
- **File:** `internal/core/archive.go` ~line 99
- **Issue:** `UnarchiveItem` reads `archived_from` from Markdown frontmatter and restores directly to that path without any containment check. A crafted archive file can restore content to an arbitrary path outside `.backlogit`. Violates Constitution Principle 4.
- **Fix:** Validate the restored path with a `SafeResolve`-style containment check. Reject any `archived_from` value that escapes `ws.RootPath/.backlogit`.

### F-007: findFileAnywhere walks entire repository, not just `.backlogit`
- **Source:** CO-002
- **File:** `internal/core/archive.go` ~line 161
- **Issue:** Archive discovery walks `ws.RootPath` (the project root), matching any Markdown file in the repo with a matching `id`. Violates Constitution Principle 4 workspace containment and can produce false matches from non-artifact Markdown files.
- **Fix:** Limit the walk root to `filepath.Join(ws.RootPath, ".backlogit")`.

### F-008: events.jsonl and telemetry.jsonl written outside `.backlogit`
- **Source:** CO-005
- **File:** `internal/mcp/server.go` ~line 33
- **Issue:** Both JSONL streams are created at `ws.RootPath` directly (project root), not under `.backlogit`. Violates Constitution Principle 4.
- **Fix:** Change paths to `filepath.Join(ws.RootPath, ".backlogit", "events.jsonl")` and `filepath.Join(ws.RootPath, ".backlogit", "telemetry.jsonl")`.

### F-009: handleUpdateItem silently discards `sections` parameter
- **Source:** MCP-001
- **File:** `internal/mcp/tools.go` ~line 62
- **Issue:** The `backlogit_update_item` tool schema registers a `sections` parameter, but `handleUpdateItem` never calls `ParseSectionsParam` or `writeSectionsToFile`. Agents receive success with no indication that their section updates were dropped.
- **Fix:** After labels parsing and before calling `core.UpdateArtifact`, parse and apply sections using the same pattern as `handleCreateItem`.

### F-010: handleCreateItem drops sections when `templateSvc` is nil
- **Source:** MCP-002
- **File:** `internal/mcp/tools.go` ~line 338
- **Issue:** `writeSectionsToFile` does not use `templateSvc` internally, but the guard `if sections != nil && s.templateSvc != nil` silently skips the write when `templateSvc` fails to initialize. Data loss with HTTP 200 response.
- **Fix:** Remove `&& s.templateSvc != nil` from the guard. Section writes are independent of template metadata.

### F-011: LinkCommit discards git author — silent data loss
- **Source:** SQL-001
- **File:** `internal/core/commits.go` ~line 23
- **Issue:** `LinkCommit` hardcodes `''` for the `author` column. `AutoLinkCommits` correctly extracts the author from git-log output into `CommitLinkInfo.Author` but never passes it to `LinkCommit`. Every `commit_links` row has an empty author.
- **Fix:** Add `author string` parameter to `LinkCommit`, pass `info.Author` from `AutoLinkCommits`, and bind it to the fourth `?` placeholder.

---

## P2 — Should Fix Before Merge

### F-012: Archive/unarchive file writes are non-atomic
- **Source:** CO-003
- **File:** `internal/core/archive.go` ~line 60
- **Issue:** Uses `os.WriteFile` directly — violates Constitution Principle 8 (temp-file-then-rename). A crash between the old-file delete and new-file write leaves the workspace in a corrupted half-moved state.
- **Fix:** Write to a sibling temp file, then `os.Rename` into place. Only delete the source after the rename succeeds.

### F-013: Migration state file written non-atomically
- **Source:** CO-004
- **File:** `internal/core/migrate_queue.go` ~line 76
- **Issue:** `.backlogit/.migration-state` uses bare `os.WriteFile` — violates Constitution Principle 8. A crash during write produces a corrupt/empty migration state, breaking rollback.
- **Fix:** Write to `.backlogit/.migration-state.tmp` then rename to `.migration-state`.

### F-014: ⚠️ SECURITY — Section names embedded in HTML comments without sanitization
- **Source:** MCP-003
- **File:** `internal/mcp/tools.go` ~line 519
- **Issue:** `writeSectionsToFile` builds markers as `"<!-- BEGIN:" + name + " -->"` with no validation. A name containing `-->` terminates the comment early and injects raw text into the artifact body. A newline in the name splits the marker, breaking subsequent reads.
- **Fix:** Add a `validateSectionName` function rejecting names with `-->`, newlines, or empty string. Return a validation error before writing any markers.

### F-015: CallToolForTest leaks goroutines — missing `c.Close()`
- **Source:** MCP-004
- **File:** `internal/mcp/call_tool.go` ~line 13
- **Issue:** The in-process client is started but never closed. Each call leaks the goroutine pair backing the transport. Race detector may surface data races under parallel test execution.
- **Fix:** Add `defer c.Close()` immediately after the `c.Start` success check.

### F-016: toolResultJSON returns raw Go error on marshal failure
- **Source:** MCP-005
- **File:** `internal/mcp/server.go` ~line 81
- **Issue:** Marshal failure returns `(nil, fmt.Errorf(...))` — a Go error, not a structured `InternalError()` response. Inconsistent with all other error paths; agents pattern-matching the structured error field will fail to classify this error.
- **Fix:** Change to `return InternalError(fmt.Sprintf("marshal result: %v", err)), nil`.

### F-017: handleDeleteItem removes DB record before file deletion
- **Source:** MCP-006
- **File:** `internal/mcp/tools.go` ~line 233
- **Issue:** `db.DeleteItem` runs before `os.Remove`. If file deletion fails, the item is permanently removed from the index but the Markdown file still exists. Next `backlogit_sync_index` would re-index it, silently reversing the delete.
- **Fix:** Call `os.Remove` first. Only call `db.DeleteItem` if file removal succeeds.

### F-018: ALTER TABLE DDL uses unquoted column names — reserved words break migrations
- **Source:** SQL-002
- **File:** `internal/db/schema_gen.go` ~line 60
- **Issue:** `fmt.Sprintf("ALTER TABLE items ADD COLUMN %s %s", fieldName, sqlType)`. While `ValidateColumnName` blocks special characters, it allows lowercase SQL reserved words (`order`, `select`, `group`, `values`, etc.) that SQLite rejects as unquoted column names.
- **Fix:** Quote identifiers: `fmt.Sprintf("ALTER TABLE items ADD COLUMN \"%s\" %s", fieldName, sqlType)`.

### F-019: ApplySchemaExtensions executes DDL outside a transaction — partial migration on failure
- **Source:** SQL-003
- **File:** `internal/db/schema_gen.go` ~line 67
- **Issue:** Each `ALTER TABLE` is executed individually. A failure mid-loop leaves the schema partially extended with no rollback capability.
- **Fix:** Wrap the execution loop in an explicit `db.Begin()` / `tx.Commit()` transaction. The `isColumnExistsError` guard moves inside the loop's error handler; on any other error, `tx.Rollback()`.

### F-020: TOCTOU race between DetectCycle and UpsertDependency
- **Source:** SQL-004
- **File:** `internal/cli/dep.go` ~line 45
- **Issue:** `DetectCycle` and `UpsertDependency` are two separate non-atomic operations. Two concurrent processes can both pass the cycle check and both insert, creating a cycle.
- **Fix:** Add `AddDependencyChecked` in `internal/db/dependencies.go` that wraps both operations in a single `BEGIN IMMEDIATE` transaction.

---

## P3 — Advisory (Non-Blocking)

### F-021: handleListTemplates missing workspace initialization check
- **Source:** MCP-007
- **File:** `internal/mcp/dynamic.go` ~line 39
- **Issue:** Unlike all other handlers, no `.backlogit` existence check. Returns empty array if workspace is absent rather than the canonical `WorkspaceNotInitialized` error.

### F-022: Double-registration guard tied to specific tool name — brittle
- **Source:** MCP-008
- **File:** `internal/mcp/dynamic.go` ~line 22
- **Issue:** Guard checks for `"backlogit_list_templates"` by string. Adding a second tool to `RegisterSectionAwareTools` in future requires updating the guard manually or the new tool will double-register.
- **Fix:** Add a `sectionToolsRegistered bool` flag to `Server` and check/set it in `RegisterSectionAwareTools`.

### F-023: commit_links missing index on commit_sha
- **Source:** SQL-005
- **File:** `internal/db/schema.go`
- **Issue:** No index on `commit_sha` alone. Reverse lookups ("which tasks reference this commit?") require full table scans.
- **Fix:** Add `CREATE INDEX IF NOT EXISTS idx_commit_links_sha ON commit_links(commit_sha)`.

### F-024: QueueFilter.SortBy/SortOrder silently ignored
- **Source:** SQL-007
- **File:** `internal/core/queue.go` ~line 78
- **Issue:** `ORDER BY` is hardcoded to `created_at ASC` regardless of filter. Callers setting sort fields receive silently incorrect order. Future developer adding SortBy to the ORDER BY clause naively risks SQL injection.

### F-025: QueueFilter.AssignedTo and Labels silently dropped
- **Source:** SQL-008
- **File:** `internal/core/queue.go` ~line 64
- **Issue:** `QueryQueue` builds its own WHERE clause that never reads `filter.AssignedTo` or `filter.Labels`. Callers receive unfiltered results with no error.

---

## Summary

| Severity | Count | Merge Status |
|----------|-------|--------------|
| P0 | 0 | — |
| P1 | 11 | ❌ Block merge |
| P2 | 9 | ⚠️ Should fix |
| P3 | 5 | ℹ️ Advisory |

**Security findings requiring immediate attention:**
- F-006: Path traversal via `archived_from` (P1)
- F-014: Section name injection via HTML comment markers (P2)
