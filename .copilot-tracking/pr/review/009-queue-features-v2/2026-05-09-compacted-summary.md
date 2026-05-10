---
type: compacted-summary
date: 2026-05-09
source_count: 1
source_date_range: "2026-04-01"
---

# Compacted Summary: PR Review --- 009 Queue Features V2

## Sources

Archived to `archive/pr/review/009-queue-features-v2/`.

- `in-progress-review.md`

## Key Decisions

- Branch: `009-queue-features-v2` (4 commits, 56 files, +941/-102)
- Feature 009 covered hierarchical file org, WIT type system, dependency graph, archive lifecycle, work queue, workflow policy
- Review reached Phase 3 (delegated review in progress)

## Outcomes

### Critical Findings

**P0 (1):**

- `backlogit.exe` binary (17MB) committed to repository --- `.gitignore` missing `*.exe`

**P1 (11):**

- `harness-status` CLI flag accepted but not persisted
- `MoveInQueue` is a silent no-op stub
- Hardcoded `QueueLayoutConfig` in two MCP handlers
- `BulkUpdateStatus` makes SQLite authoritative over Markdown (Constitution violation)
- `filterByResolvedDependencies` treats DB errors as "no dependencies"
- `commit_links` is SQLite-only --- not in Markdown/JSONL (lost on sync)
- `UnarchiveItem` restore path anchors to workspace root (containment issue)
- 8 new MCP tools have no contract tests
- `CAST(id AS INTEGER)` returns 0 for prefix-based IDs (duplicate ID generation)
- Schema migration gap for existing databases
- Rehydration does not populate `level` / `hierarchy_path`

**P2 (9):**

- N+1 query in dependency filtering
- Unbounded QueryQueue without row cap
- UnarchiveItem ignores ArtifactFromFrontmatter failures
- dependencies TEXT and item_deps table can diverge
- QueueFilter missing IncludeArchived
- No slog instrumentation for MCP tool execution
- Workspace init suppresses malformed config errors
- Rehydration discards dependency upsert errors
- Missing index on level column

**P3 (10):**

- FormatHierarchicalID ignores layout parameter
- .gitignore missing `*.exe` and `backlogit.db`
- Missing unarchive MCP tool
- Multiple nolint:errcheck without justification
- GoDoc gaps, missing context.Context, FK constraints, DetectCycle thread safety

## Preserved Context

- Review identified the 17MB binary commit as the only P0
- Recurring themes: SQLite-authoritative writes violating Markdown-first CQRS, missing contract tests, workspace containment gaps
- Review was in Phase 3 (delegated review) --- user decisions pending
