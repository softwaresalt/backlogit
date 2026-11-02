---
schema_version: "1.0"
doc_type: plan
title: "Decided plan: AdoptItem Cross-Reference Rewrite"
description: "Decided-plan consolidation of 2026-04-20-adoptitem-cross-reference-rewrite-plan.md: final decisions, implementation units, risks, and review verdict."
chunk_strategy: h1-h2-h3
source: docs/exec-plans/2026-10-18-adoptitem-cross-reference-rewrite-decided-plan.md
---

# Decided plan: AdoptItem Cross-Reference Rewrite

Consolidated from `docs/archive/plans/2026-04-20-adoptitem-cross-reference-rewrite-plan.md` (P-020 compaction, 2026-10-18). The owning release units are archived. Full deliberation history stays in the archived original.

## Problem Frame

When `AdoptItem` reassigns an orphaned or unparented artifact to a new parent
feature, it rewrites the artifact's hierarchical ID (e.g., `012-T` becomes
`025.012-T`). The function correctly updates:

* The artifact's own frontmatter (ID, parent_id)
* DB edge tables: `item_deps`, `item_links`, `commit_links`, `stash_links`,
  `item_logs`, `item_log_entries`
* File system: renames `.md` and `.jsonl` files

However, **other artifacts** whose Markdown frontmatter references the old ID
are not updated. Three frontmatter fields can contain cross-references:

* `parent_id` — child artifacts whose parent was the adopted item
* `dependencies` — artifacts listing the old ID as a dependency
* `links` — artifacts with semantic links targeting the old ID

Because Markdown is the source of truth (Constitution VII), the next
`backlogit sync` (rehydration) rebuilds the DB from stale Markdown, reverting
the corrected DB edges to their pre-adoption state. The fix is ephemeral; the
bug is permanent.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | After AdoptItem, no Markdown file in `.backlogit/` contains the old artifact ID in `parent_id`, `dependencies`, or `links` fields | Stash C00AA592 / 035-F |
| R2 | Cross-reference rewrites survive rehydration: `backlogit sync` preserves corrected edges | Stash C00AA592 (rehydration staleness) |
| R3 | Cross-reference rewrites are atomic with the AdoptItem operation (all-or-nothing) | Constitution VII (CQRS), VIII (atomic writes) |

## Implementation units

* **Implementation Units**: Each unit MUST be scoped to roughly 2 hours of human-equivalent effort.
* **Unit 1: Cross-Artifact Reference Scanner and Rewriter**: **Files:** `internal/core/artifact_references.go` (new file) **Test files:** `internal/core/artifact_references_test.go` (new file) **Effort size:** medium **Skill domain:** code **Execution note:** test-first **Patterns to follow:** `artifactSearchDirs` ...
* **Unit 2: AdoptItem Integration**: **Files:** `internal/core/shipment_lifecycle.go` **Test files:** `internal/core/shipment_test.go` **Effort size:** small **Skill domain:** code **Execution note:** characterization-first (existing tests must still pass) **Patterns to follow:** existing ...
* **Unit 3: Comprehensive Test Suite**: **Files:** `internal/core/artifact_references_test.go`, `internal/core/shipment_test.go` **Test files:** (same files) **Effort size:** medium **Skill domain:** tests **Execution note:** test-first **Patterns to follow:** `TestShipment_RehydrationConsistency` ...

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Filesystem scan (walk `.md` files) rather than DB query for finding references | Markdown is the source of truth. A DB query would miss references in artifacts not yet indexed. Walking the filesystem is authoritative. | DB `SELECT WHERE parent_id = ? OR dependencies LIKE ?` — fast but not authoritative; could miss unindexed artifacts |
| D2 | New file `artifact_references.go` rather than extending `shipment_lifecycle.go` | Cross-reference rewriting is a workspace-level concern, not shipment-specific. Separate file improves cohesion and testability (per review F-009). | Inline in `shipment_lifecycle.go` — grows an already large file with unrelated responsibility |
| D3 | Two-phase approach: collect outside tx, apply inside short tx | Follows Rehydrate()'s phased design (per review F-006). Collect phase holds no DB lock. Apply phase is short and targeted. | Single-phase scan+write inside tx — holds DB lock during filesystem walk |
| D4 | Defer `custom_fields.items` to follow-up | Not proven to participate in the rehydration corruption path. Introduces `[]any` vs `[]string` JSON round-trip complexity (per review F-004, LR-006). | Include in V1 — increases scope and risk without clear rehydration benefit |
| D5 | Extend `AdoptItemResult` with `RewrittenArtifactIDs` (omitempty) | Gives callers and event logging visibility into the cascade without a separate query. `omitempty` keeps the response clean when no rewrites occur. | Separate function to query rewritten refs — unnecessary indirection |
| D6 | File snapshot/restore rollback for cross-artifact writes | Follows existing AdoptItem rollback pattern (lines 652-658). Ensures all-or-nothing file writes on failure (per review F-001). | No rollback — would leave workspace in inconsistent state on partial failure |

## Risks and Caveats

1. **Performance on large workspaces.** The collect phase scans all `.md`
   files in `.backlogit/`. For workspaces with hundreds of artifacts, this adds
   latency to AdoptItem. Mitigation: the scan is read-only and runs outside the
   transaction, adding no DB lock time. Only artifacts that actually reference
   the old ID are written. For the typical backlogit workspace (dozens of
   artifacts), this is negligible.

2. **Partial file write failure.** If writing a cross-referenced artifact's file
   fails mid-apply, some files will be updated and others won't. Mitigation:
   each file write is atomic (tmp+rename). The apply function tracks which files
   succeeded and restores from snapshots on failure. The DB transaction rolls
   back automatically via `defer tx.Rollback()`.

3. **Compound learning: orphaned tasks.** The learning at
   `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
   documents that AdoptItem previously didn't rewrite IDs at all. That was fixed.
   This plan addresses the next layer: cross-artifact references. The learning's
   "systemic prevention" section (item 2) explicitly calls for this fix.

4. **`custom_fields.items` deferred.** Shipment manifests store item IDs in this
   field. V1 does not rewrite them. If a shipped shipment's manifest still
   references old IDs, the shipment display may show stale items. This is
   tracked as a follow-up stash item. Rehydration correctness is not affected
   because ...

## Plan Review

**Gate Decision: FAIL → Revised → ADVISORY**

Six P1 findings required plan revision. All P1 findings have been addressed
through in-place plan revisions. Remaining P2/P3 findings are advisory and
do not block harvest.

## Reviewers

| Persona | Model | Findings |
|---|---|---|
| Constitution Reviewer | claude-opus-4.6 | 0 |
| Go Quality Reviewer | claude-opus-4.6 | 7 (1→0 P0, 3 P1, 2 P2, 1 P3) |
| Scope Boundary Auditor | claude-opus-4.6 | 5 (2 P1, 2 P2, 1 P3) |
| Learnings Researcher | claude-haiku-4.5 | 8 relevant learnings |
| Architecture Strategist | gpt-5.4 | 6 (1→0 P0, 2 P1, 3 P2) |
| Agent-Native Parity Reviewer | gpt-5.4 | 3 (2 P2, 1 P3) |

## Traceability

| Original path | Archive path | Disposition |
|---|---|---|
| `docs/exec-plans/2026-04-20-adoptitem-cross-reference-rewrite-plan.md` | `docs/archive/plans/2026-04-20-adoptitem-cross-reference-rewrite-plan.md` | archived (decided-plan) |
