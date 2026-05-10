---
type: compacted-summary
date: 2026-05-09
source_count: 3
source_date_range: "2026-03-31"
---

# Compacted Summary: PR Review --- 008 MCP/CLI Bug Fixes

## Sources

All archived to `archive/pr/review/008-mcp-cli-section-template-bug-fixes/`.

- `findings.md`
- `in-progress-review.md`
- `pr-reference.xml` (XML PR diff reference)

## Key Decisions

- Branch: `008-mcp-cli-section-template-bug-fixes` (3 commits, 43 files changed)
- Feature 008 fixed MCP tools, CLI commands, and domain stubs surfaced in the 002 code review
- Review reached Phase 2 with findings document generated

## Outcomes

### Findings Summary (28 raw, 25 after dedup)

**P1 --- Block Merge (11):**

- F-001: CreateArtifact panics on nil Config
- F-002: MoveInQueue is a non-functional no-op
- F-003: BulkUpdateStatus leaves Markdown stale (SQLite-only update)
- F-004: Archive/unarchive suppress index update errors
- F-005: Queue CLI handlers are shipped as no-ops
- F-006: SECURITY --- Path traversal via `archived_from` in UnarchiveItem
- F-007: findFileAnywhere walks entire repository (containment violation)
- F-008: events.jsonl written outside `.backlogit/`
- F-009: handleUpdateItem silently discards `sections` parameter
- F-010: handleCreateItem drops sections when templateSvc is nil
- F-011: LinkCommit discards git author (silent data loss)

**P2 --- Should Fix (9):**

- Non-atomic file writes in archive and migration
- SECURITY --- Section name injection via HTML comment markers
- CallToolForTest leaks goroutines
- toolResultJSON returns raw Go error
- handleDeleteItem removes DB before file
- ALTER TABLE uses unquoted column names
- ApplySchemaExtensions outside transaction
- TOCTOU race between DetectCycle and UpsertDependency

**P3 --- Advisory (5):**

- handleListTemplates missing workspace check
- Double-registration guard brittleness
- commit_links missing SHA index
- QueueFilter.SortBy silently ignored
- QueueFilter.AssignedTo/Labels silently dropped

### Security Findings

- F-006: Path traversal via `archived_from` --- crafted archive can restore outside `.backlogit/`
- F-014: Section name injection --- `-->` in name terminates HTML comment early

## Preserved Context

- Review identified that 008 branch fixed some 002 issues but introduced new ones
- Constitution violations: workspace containment (Principle 4), CQRS write ordering (Principle 7), no dead code (Principle 10)
- Review did not proceed to Phase 3 (user decisions) or Phase 4 (PR/handoff)
