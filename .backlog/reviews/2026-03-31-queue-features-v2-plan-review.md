---
title: "Plan Review: Queue Features V2"
date: 2026-03-31
plan: ".backlog/exec-plans/2026-03-31-queue-features-v2-plan.md"
gate: ADVISORY
reviewers:
  - Constitution Reviewer (claude-opus-4.6)
  - Go Quality Reviewer (claude-opus-4.6)
  - Architecture Strategist (gpt-5.4)
  - Scope Boundary Auditor (gpt-5.4-mini)
---

## Gate Decision: ADVISORY

The plan is structurally sound with no P0 blockers. Eight P2 advisory findings identified below. The plan may proceed to harvesting with these findings recorded in task descriptions.

## Findings

### F1 — Dynamic DDL Generation Needs Injection Protection

**Unit:** 3 (Dynamic Schema)
**Severity:** P1
**Category:** Security / SQL Safety
**Finding:** Unit 3 proposes generating SQLite DDL from YAML field definitions at runtime. Column names derived from user-controlled YAML could introduce SQL injection if not sanitized. The current `EnsureSchema` uses static SQL strings which are inherently safe.
**Recommendation:** Validate column names against a strict `^[a-z_][a-z0-9_]*$` regex before interpolating into DDL. Consider allowlisting column names from the Go struct tags rather than generating from arbitrary YAML keys.

### F2 — Hierarchical ID Generation Complexity

**Unit:** 1 (Hierarchical Naming)
**Severity:** P2
**Category:** Implementation Complexity
**Finding:** Generating hierarchical IDs (001.001.001) requires querying existing siblings within a parent scope to determine the next segment. The current `NextID` function uses a simple `COUNT(*)` by type, which won't work for scoped numbering. The plan doesn't specify the exact query for scoped ID generation.
**Recommendation:** Add explicit implementation detail: `SELECT MAX(CAST(substr(id, ?, ?) AS INTEGER)) FROM items WHERE parent_id = ?` or similar scoped counter query. Consider a dedicated `id_counters` table for atomic counter management.

### F3 — Junction Table Rehydration Not Fully Specified

**Unit:** 7 (Dependency Graph)
**Severity:** P2
**Category:** CQRS Compliance
**Finding:** The plan mentions updating the rehydration engine to populate `item_deps` from frontmatter `dependencies` fields, but doesn't specify the implementation. Since `item_deps` is ephemeral (like all SQLite data), it must be fully rebuildable from Markdown files. The current `dependencies` field is a flat `[]string` of IDs — the `dep_type` column (blocks/relates_to/child_of) has no source in the frontmatter.
**Recommendation:** Either (a) extend the frontmatter format to include typed dependencies like `dependencies: [{id: "001", type: "blocks"}]`, or (b) default all rehydrated dependencies to `blocks` type and only use other types when explicitly set via MCP/CLI.

### F4 — Commit Tracking Shells Out to Git

**Unit:** 10 (Commit Tracking)
**Severity:** P2
**Category:** Dependency / Complexity
**Finding:** The `check-merged` command requires shelling out to `git log --merges` for branch merge detection. This adds an implicit runtime dependency on git being installed and accessible. The plan lists "no new external dependencies" in the constitution check, but git is an external binary dependency.
**Recommendation:** Document git as a runtime prerequisite for this specific command. Make the command fail gracefully with a descriptive error if git is not found. Consider using go-git for pure-Go git operations (though this adds a module dependency).

### F5 — Migration Atomicity at Scale

**Unit:** 2 (Migration Tool)
**Severity:** P2
**Category:** Migration Risk
**Finding:** The plan says migration operates "atomically per-file" but if the process crashes mid-migration (after renaming 50 of 100 files), the workspace is in an inconsistent state. Some files have old IDs, some have new IDs, and the index is stale.
**Recommendation:** Add a migration state file (`.backlogit/.migration-state`) that tracks progress. On resume, skip already-migrated files. Alternatively, use a two-phase approach: copy-then-delete rather than rename, so old files survive until the entire batch succeeds.

### F6 — Breaking Change Surface Underspecified

**Unit:** 1, 2 (Hierarchical Naming + Migration)
**Severity:** P2
**Category:** Breaking Change
**Finding:** The shift from per-type directories to `.backlogit/queue/` breaks any external tooling that references `tasks/`, `bugs/`, `epics/` paths. The registry.yaml routing rules also change meaning. The plan identifies this as a risk but doesn't specify how `registry.yaml` evolves.
**Recommendation:** Add explicit migration steps for `registry.yaml`. Specify backward-compatibility: should the old directory rules still work as aliases during a transition period? Or is it a hard cut-over?

### F7 — `describe_type` Response Size

**Unit:** 6 (WIT Metadata API)
**Severity:** P3
**Category:** Agent Context Efficiency
**Finding:** The `backlogit_describe_type` response includes fields, sections, relationships, and directory mappings. For types with many fields and sections, this could be a large response. The constitution principle IX emphasizes minimal targeted data.
**Recommendation:** Consider adding a `fields_only` or `sections_only` parameter to allow agents to request just the subset they need.

### F8 — Harness Status Scope is Very Small

**Unit:** 12 (Harness Status)
**Severity:** P3
**Category:** Effort Estimate
**Finding:** Unit 12 is marked as "small" effort but it's essentially just adding an enum field to the default header-def.yaml configuration. This could be a sub-unit of Unit 4 (Required/Optional Fields) rather than a standalone unit.
**Recommendation:** Consider merging Unit 12 into Unit 4 to reduce unit count and harvesting overhead. If kept separate, acknowledge it's a configuration change rather than a code change.

## Summary

| Severity | Count | Blocking? |
|----------|-------|-----------|
| P0       | 0     | —         |
| P1       | 1     | No (mitigable) |
| P2       | 5     | No (advisory) |
| P3       | 2     | No        |

The single P1 finding (F1: DDL injection protection) is mitigable with a column name validation regex. All other findings are advisory improvements. The plan is approved for harvesting with these findings recorded in relevant task descriptions.
