---
title: "Plan Review: Artifact Identity, Hierarchy & Relationships"
date: 2026-04-07
plan: "docs/exec-plans/2026-04-07-artifact-identity-hierarchy-relationships-plan.md"
origin: ".backlogit/queue/DL003.md"
gate_decision: PASS with advisories
---

## Gate Decision: PASS (with advisories)

The plan passes the review gate. No P0 (critical) or P1 (high) findings.
Five P2 (moderate) findings and four P3 (advisory) findings are noted below
for the implementer's consideration. None block harvest.

## Findings Summary

| Severity | Count | Blocking? |
|----------|-------|-----------|
| P0       | 0     | N/A       |
| P1       | 0     | N/A       |
| P2       | 5     | No        |
| P3       | 4     | No        |

## Findings

### F-01: Migration script as standalone binary vs constitution principle VI

**Severity:** P2
**Unit:** 1F
**Reviewer:** Constitution Reviewer
**Principle:** VI — Single-Binary Simplicity

The plan places migration scripts in `scripts/migrate-ids.go` as standalone
`go run` executables. Constitution principle VI states the project must be
installable as a single static binary. Standalone scripts outside
`cmd/backlogit/` are not part of the installable binary.

**Assessment:** This is explicitly justified in Decision D7: migration is a
one-time operation that should not ship as permanent dead weight in the
production binary. The constitution permits justified deviations when
documented. The plan documents the justification. **No change required.**

### F-02: Unit 1D forward-declares item_links for Stream 3

**Severity:** P2
**Unit:** 1D
**Reviewer:** Architecture Strategist
**Principle:** Module cohesion

Unit 1D (Rehydration and DB Schema) creates the `item_links` table, which is
a Stream 3 (Typed Relationship Links) concept. This crosses stream boundaries
in the schema layer.

**Assessment:** Decision D8 justifies this as avoiding a second schema migration
round. Schema changes in `EnsureSchema` are idempotent CREATE IF NOT EXISTS
statements and the pattern already exists (the function creates all tables in
one transaction). The table is inert until Stream 3 populates it. **Acceptable;
implementer should add a comment noting the forward declaration.**

### F-03: Unit 4A shipment exemption fragility

**Severity:** P2
**Unit:** 4A
**Reviewer:** Go Quality Reviewer / Architecture Strategist

The blocking cascade exemption for shipment releases creates a conditional
code path in `setArtifactStatus`. The current function signature includes a
`reason` string parameter. Using `reason` to detect shipment context
(e.g., checking for "shipment released") would be fragile string matching.

**Recommendation:** Introduce an explicit `opts ...StatusOption` parameter or
a boolean `skipChildCheck` flag rather than parsing the reason string. The
implementer should define a clean mechanism for the exemption:

```go
type statusOption struct{ skipChildCheck bool }
type StatusOption func(*statusOption)
func SkipChildCheck() StatusOption { return func(o *statusOption) { o.skipChildCheck = true } }
```

### F-04: hierarchyPathFromID cannot resolve parent suffixes without DB lookup

**Severity:** P2
**Unit:** 1D
**Reviewer:** Go Quality Reviewer

The plan's verification example shows
`hierarchyPathFromID("001.002.003-ST")` returning
`001-F/001.002-T/001.002.003-ST`. However, the function cannot know the
parent segments' suffixes (`-F`, `-T`) from the child ID alone since the
suffix encodes type, not position. The child ID `001.002.003-ST` only
contains its own suffix.

**Recommendation:** The implementer must choose:
(a) hierarchy_path uses numeric-only segments: `001/001.002/001.002.003`
(b) hierarchy_path is built during rehydration with DB lookups for parent types.
Option (a) is simpler and sufficient for ancestor queries. The plan already
notes "implementer decides based on parent lookup feasibility." **Clarify the
plan to recommend option (a) as the default.**

### F-05: Unit 1F migration size may exceed "medium" effort

**Severity:** P2
**Unit:** 1F
**Reviewer:** Scope Boundary Auditor

The migration script handles 193 .md files, 50 JSONL log files, inline
item_id rewrites in JSONL content, parent_id and dependency array updates,
file renames, idempotency logic, dry-run mode, and a migration report.
This is substantial for a "medium" (~2 hour) unit.

**Recommendation:** Consider splitting 1F into two sub-units:
- **1F-i:** Build old→new ID mapping table and dry-run report (small)
- **1F-ii:** Apply renames and content updates using the mapping (medium)

This also improves the review gate: the mapping can be verified before
any destructive changes.

### F-06: `supersedes` and `informs` link types may be YAGNI

**Severity:** P3
**Unit:** 3A
**Reviewer:** Scope Boundary Auditor

The `item_links` table defines 5 link types: `related_to`, `duplicate_of`,
`informs`, `supersedes`, `spike_ref`. Only `spike_ref` and `related_to`
have clear existing usage. `duplicate_of` is common in issue trackers.
`informs` and `supersedes` have no current consumers.

**Assessment:** The link_type is a TEXT column validated by application code,
not a DB constraint. Adding types later costs nothing. However, the
migration script (3C) maps `source_stash_id` to `informs`, which provides
a concrete consumer. `supersedes` is the only truly speculative type.
**Advisory: implementer may defer `supersedes` if it simplifies initial
scope.**

### F-07: Stream 1 dependency chain length

**Severity:** P3
**Unit:** cross-cutting
**Reviewer:** Architecture Strategist

Stream 1 has a 6-unit sequential chain: 1A→1B→1C→1D→1E→1F. This is the
longest chain in the plan. Units 1D and 1E could potentially parallelize
after 1C since they touch different packages (db/ vs mcp/).

**Assessment:** The plan's stated dependency is correct (1E needs 1D for
schema changes). However, 1D only forward-declares item_links; the actual
schema change 1E depends on is isHierarchicalID in rehydration.go. If 1D
is split into "schema DDL" and "rehydration format recognition," 1E could
start after the format recognition piece. **Advisory: implementer may
parallelize if the chain feels bottlenecked.**

### F-08: Missing verification for empty workspace edge case

**Severity:** P3
**Unit:** 1B, 1F
**Reviewer:** Scope Boundary Auditor

Unit 1B's verification covers standard ID generation but doesn't mention
behavior in an empty workspace (no existing artifacts). The first feature
should generate `001-F`. Unit 1F doesn't specify behavior when the archive
directory is empty.

**Recommendation:** Add empty workspace test case to 1B verification.
Unit 1F should handle gracefully (no-op with clean report).

### F-09: ID immutability exception is adequately justified

**Severity:** P3
**Unit:** 1F
**Reviewer:** Constitution Reviewer
**Principle:** VII — CQRS Data Architecture (ID immutability)

The Standards Check notes ID immutability as ⚠ with "one-time exception."
Constitution principle VII states IDs are immutable keys. The migration
is a one-time schema evolution, not a runtime mutation. Post-migration,
IDs return to immutable status.

**Assessment:** The justification is adequate. The plan's Risks section
(R1) addresses data integrity. The migration report provides audit trail.
**No change required.**

## Reviewer Attribution

| Persona | Model | Focus |
|---|---|---|
| Constitution Reviewer | Claude Sonnet 4 | Principle compliance |
| Go Quality Reviewer | go-engineer (Claude Haiku 4.5) | Code quality gates |
| Architecture Strategist | GPT-5.4 | Cohesion, coupling, deps |
| Scope Boundary Auditor | GPT-5.4 | Scope creep, YAGNI, sizing |

## Recommendations for Implementer

1. **F-03:** Use an explicit `StatusOption` pattern for the shipment exemption
   rather than string-based reason parsing
2. **F-04:** Default to numeric-only hierarchy_path segments (option a)
3. **F-05:** Consider splitting Unit 1F into mapping-build + apply sub-units
4. **F-08:** Add empty workspace edge case to Unit 1B and 1F verification

## Next Steps

This plan is approved for harvest. Invoke the `harvest` skill to decompose
the reviewed plan into backlogit feature, task, and subtask items.
