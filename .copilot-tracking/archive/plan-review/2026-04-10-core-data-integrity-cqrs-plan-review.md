---
title: "Plan Review: Core Data Integrity & CQRS Compliance"
date: 2026-04-10
plan: "docs/exec-plans/2026-04-10-core-data-integrity-cqrs-plan.md"
gate: advisory
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
---

# Plan Review: Core Data Integrity & CQRS Compliance

## Gate Decision: ADVISORY

The plan is architecturally sound and well-structured. Four P0 findings were
identified but all are correctible specification gaps, not fundamental design
flaws. The corrections have been applied to the plan as amendments. P1 findings
are carried as implementation guidance. The plan is approved to proceed to
harvest with the amendments below.

## Summary

- **P0 (Critical)**: 4 findings → all corrected via plan amendment
- **P1 (High)**: 8 findings → carried as implementation guidance
- **P2 (Moderate)**: 6 findings → advisory
- **P3 (Low)**: 4 findings → advisory

## Findings

### P0 — Critical (corrected in plan amendment)

**F-1 (merged: A-3, S-1, C-1)** — Unit 4 write ordering is DB-first, not Markdown-first.
The plan describes `AddLinkDurable` as calling `db.AddLink` first, then writing
Markdown. This recreates the exact CQRS break the plan is fixing. **Correction**:
reverse the ordering to Markdown-first, then SQLite cache upsert. Add sentinel
errors `ErrLinkNotFound`, `ErrLinkInvalid` for proper domain error mapping.

**F-2 (merged: G-1)** — DSN pragma syntax needs spike verification.
The `_pragma=journal_mode(WAL)` format may not be supported by the exact driver
version. **Correction**: Unit 1 must begin with a spike test of the DSN format.
Fallback to `SetMaxOpenConns(1)` + post-open `Exec` is already documented.

**F-3 (merged: A-2, G-2)** — Links field on shared Artifact model causes
inconsistent hydration across DB vs file callers. `db.scanArtifactRow` /
`UpsertItem` have no `links` storage. **Correction**: `Links` field is populated
only from Markdown file reads, never from DB queries. `upsertItemTx` ignores it.
The `item_links` table remains the SQLite projection, populated during
rehydration from Markdown.

**F-4 (merged: A-6, S-3, G-9)** — `sync.Once` is the wrong primitive for
`ensureWorkspace`. It caches errors permanently, so a server started before
`.backlogit` exists can never recover. **Correction**: Unit 13 should use
mutex/double-check pattern that retries on failure but caches success.

### P1 — High (implementation guidance)

**F-5 (A-4)** — Units 5 and 14 both touch `DeleteAllItemLogs` / rehydration
transaction. These are not cleanly separated. *Guidance*: merge rehydration
transactional work into Unit 5 and have Unit 14 focus only on cascade-delete
for item deletion.

**F-6 (A-5)** — Unit 9 duplicates existing relocation primitives in
`internal/core/relocate.go` and `internal/core/routing.go`. *Guidance*: reuse
the existing `RelocateArtifact` / routing functions rather than creating new ones.

**F-7 (S-5, G-4)** — Existing SQLite-only links will be lost when rehydration
carve-out is removed. *Guidance*: Unit 5 must include a pre-deployment migration
step or startup guard that detects DB-only links and writes them to Markdown
before the carve-out is removed.

**F-8 (S-2, G-7)** — Move handler doesn't rollback on relocation failure.
*Guidance*: Unit 9 must revert status in Markdown if `os.Rename` fails, or
document that rehydration will recover consistency on next run.

**F-9 (G-6, C-6)** — `BulkUpdateStatus` partial-success return type is
unspecified. *Guidance*: define `BulkUpdateResult{Succeeded int, Failed []string,
Err error}` as the return type.

**F-10 (G-10)** — `DeleteAllItemLogs` is intentionally pre-transactional in
current design. Moving it inside the transaction changes semantics. *Guidance*:
keep log clearing pre-transactional unless there's a specific atomicity need.

**F-11 (A-1)** — Unit 12 may over-engineer the shipment response shape fix.
*Guidance*: verify the actual wire-shape difference before creating a new
`core.ListShipments` API. A handler-level normalizer may suffice.

**F-12 (G-8, C-3)** — Unit 7 and Unit 11 must confirm sentinel error definitions
exist in `internal/errors/errors.go`. *Guidance*: verify `ErrNotFound` is defined
as a `var` sentinel before relying on `errors.Is` chains.

### P2 — Moderate (advisory)

**F-13 (C-7)** — Unit 9 should validate path containment before `os.Rename`.
**F-14 (C-8)** — Unit 11 should create comprehensive error mapping table.
**F-15 (C-9, C-2)** — Links YAML schema should be resolved in plan, not deferred.
Recommendation: use `[]ArtifactLink` (list of objects) matching `Dependencies`
pattern.
**F-16 (G-11, C-10)** — Unit 14 cascade DELETE statements need per-statement
error checking.
**F-17 (G-12, S-4)** — Unit 12 `ShipmentDetail` struct is undefined. Keep
minimal if new core API is needed.
**F-18 (A-7)** — Unit 1 → Unit 14 dependency may be false since `item_links`
has no FK constraints.

### P3 — Low (advisory)

**F-19 (C-11)** — `SetMaxOpenConns(4)` choice should be documented with rationale.
**F-20 (C-12)** — Unit 6 tests should clarify how rehydration is triggered.
**F-21 (C-13)** — Document that workspace must exist before first tool call.
**F-22 (C-14, S-6)** — Some "medium" effort units may be undersized.

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F-1 | architecture-strategist, scope-boundary-auditor, constitution-reviewer | gpt-5.4, gpt-5.4, claude-haiku-4.5 |
| F-2 | go-quality-reviewer | claude-haiku-4.5 |
| F-3 | architecture-strategist, go-quality-reviewer | gpt-5.4, claude-haiku-4.5 |
| F-4 | architecture-strategist, scope-boundary-auditor, go-quality-reviewer | gpt-5.4, gpt-5.4, claude-haiku-4.5 |
| F-5 | architecture-strategist | gpt-5.4 |
| F-6 | architecture-strategist | gpt-5.4 |
| F-7 | scope-boundary-auditor, go-quality-reviewer | gpt-5.4, claude-haiku-4.5 |
| F-8 | scope-boundary-auditor, go-quality-reviewer | gpt-5.4, claude-haiku-4.5 |
| F-9 | go-quality-reviewer, constitution-reviewer | claude-haiku-4.5, claude-haiku-4.5 |
| F-10 | go-quality-reviewer | claude-haiku-4.5 |
| F-11 | architecture-strategist | gpt-5.4 |
| F-12 | go-quality-reviewer, constitution-reviewer | claude-haiku-4.5, claude-haiku-4.5 |

## Next Steps

Plan proceeds to `harvest` with amendments applied. Implementation units should
incorporate P1 guidance as implementation constraints. P0 corrections are
non-negotiable and must be reflected in the harvested backlog descriptions.
