---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for two archive/shipment re-persist data-loss bugs (D04D63D0 + 7A965F8A). The SQLite index is a rebuildable cache that carries neither item_links (stored in the separate item_links table) nor archive provenance (not indexed), so the shipment lifecycle re-persist path (loadArtifact fast-path -> persistArtifact) drops them. Fix at the core re-persist seam by reloading from Markdown (source of truth) per the MoveInQueue precedent, plus a self-contained guard skipping already-archived linked deliberations in the shipment archive flow.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-28-archive-repersist-projection-drop-plan.md
title: 'Archive/shipment re-persist field-drop fix (links + provenance)'
---

## Source

- Deliberation: `docs/decisions/2026-07-28-archive-repersist-projection-drop-deliberation.md`.
- Stash entries: `D04D63D0` (bug, medium) and `7A965F8A` (bug, medium), triaged
  together as Group A. Surfaced during shipment closure `108-S`/`109-S`
  (PR #305 `ef9218d2`, PR #306 `c5892c7e`, PR #309).
- Root-cause seam (corrected after plan-review, attempt 1): the SQLite index is a
  **rebuildable query cache**; Markdown files are the source of truth. The
  `items` table (`internal/db/schema.go:285-289`) carries `dependencies`,
  `"references"`, `commit`, ... but has **no** `links`, `archived_from`, or
  `archived_status` columns. Links are stored in the **separate normalized
  `item_links` table** (`internal/db/schema.go:330`); archive provenance is not
  indexed at all. So `internal/core/shipment.go:337` `loadArtifact` — which
  prefers the fast-path `bldb.GetItem` and only falls back to Markdown
  `findArtifact` on `ErrNotFound` — returns an `Artifact` with empty `Links`,
  `ArchivedFrom`, and `ArchivedStatus`; the subsequent `persistArtifact` rewrite
  drops them.
- Precedent (the correct, in-repo fix pattern): `MoveInQueue` reloads each item
  from Markdown before rewriting rather than trusting the DB projection —
  documented in `internal/core/serializer_provenance_hardening_test.go`. Related
  prior learnings: `docs/compound/2026-07-17-backlogit-update-drops-archive-provenance.md`,
  `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`
  (ship-gate reads `archived_status` from Markdown and fails closed —
  **must be preserved**),
  `docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`
  (empty-vs-populated round-trip must both be asserted).

## Problem Frame

`attachCommitToItems` (`internal/core/shipment_lifecycle.go:341`) iterates every
archive/commit candidate, `loadArtifact`s it (DB fast-path), sets `Commit`, and
`persistArtifact`s it. Because the fast-path load carries neither `item_links`
nor archive provenance (they are not columns on `items`), the rewrite via
`WriteArtifactFileWithOptions` (`internal/core/artifacts.go`) emits neither:
`ToFrontmatterMap` (`internal/models/artifact.go:74+`) emits `links` only when
non-empty and archive provenance only while `Status == archived`.

This single seam produces both bugs:

1. **D04D63D0** — `collectArchiveCandidateIDs`
   (`internal/core/shipment_lifecycle.go:292`) filters archived features and
   descendants but appends `linkedDeliberationIDs` **without** an archived filter
   (lines ~331-335). When a candidate is an already-archived linked deliberation
   (e.g. `054-DL`), the fast-path load yields `Status == archived` with empty
   provenance, so the write-boundary guard (`internal/core/artifacts.go:761`,
   `"refusing to write archived artifact ... without provenance"`) refuses the
   write and `ship_shipment` aborts **after** stamping members but **before**
   archiving the manifest. Recovery today: `backlogit archive <shipment-id>`.
2. **7A965F8A** — the same fast-path re-persist drops modeled `item_links` for
   **every** commit-stamped candidate (not only deliberations); observed as the
   `123-F -> 120-F` `spike_ref` dropped during `109-S` archive (restored by hand
   in PR #309). `ArchiveItem`'s own raw-frontmatter path
   (`internal/core/archive.go:97`) preserves links; the drop originates in the
   DB-fast-path re-persist flow.

## Requirements Trace

| Requirement (source) | Implementation action |
|---|---|
| Shipping a feature that links an already-archived deliberation must succeed and archive the manifest (D04D63D0) | Unit 1: skip/guard already-archived linked deliberations in the shipment archive flow (`shipment_lifecycle.go`); core regression test |
| Modeled `item_links` (`spike_ref`) + archive provenance survive the shipment/archive re-persist path for every stamped candidate (7A965F8A) | Unit 2: reload each candidate from Markdown (source of truth) before `persistArtifact` in the shipment re-persist seam, per the `MoveInQueue` precedent; core regression test |

## Implementation Units

### Unit 1 (Task) — Skip already-archived linked deliberations in shipment archive flow (D04D63D0)

- **Changes**: In `internal/core/shipment_lifecycle.go`, stop routing a
  pre-existing archived linked deliberation into the commit-stamp/re-persist set.
  Preferred seam: skip already-archived items inside `attachCommitToItems`,
  where the artifact is already loaded — a single `Status == StatusArchived`
  check at the one re-persist seam that avoids a redundant per-deliberation
  load (`linkedDeliberationIDs` returns bare IDs, so filtering in
  `collectArchiveCandidateIDs` would force an extra `loadArtifact` each). The
  check mirrors the existing `Status == StatusArchived` skip already applied to
  features and descendants. (Alternative, equivalent: drop archived deliberations
  in `collectArchiveCandidateIDs` before appending `linkedDeliberationIDs`.)
  Stamping a
  shipment merge commit onto a pre-existing linked deliberation is semantically
  wrong, so the skip is a correctness fix, not merely a guard against the write
  error. This unit is **independent** of Unit 2: it fixes the abort regardless of
  the field-fidelity seam.
- **Files**: `internal/core/shipment_lifecycle.go` (1 file, 1 function).
- **Tests (test-first)**: new core regression test — build a shipment whose
  feature links an already-archived deliberation, run the ship/archive flow,
  assert (a) ship completes without the provenance-guard error and (b) the
  shipment manifest is archived. Write RED first (reproduce the abort), then
  implement.
- **Posture**: test-first. **Depends on**: none.

### Unit 2 (Task) — Reload from Markdown before re-persist so links + provenance survive (7A965F8A)

- **Changes**: In `internal/core/shipment_lifecycle.go`, make the re-persist path
  full-fidelity by **replacing** the lossy DB fast-path `loadArtifact` result
  with a Markdown (source-of-truth) load for the mutate-then-persist candidate —
  reload via the `findArtifact` Markdown path, set `Commit`/`UpdatedAt` on the
  **reloaded** artifact, then `persistArtifact` that artifact. Do NOT merely add a
  supplemental read alongside the fast-path load (both loads coexisting would
  re-introduce the drop — a false-green). Wrap the reload error with
  `fmt.Errorf("reload item %s from markdown: %w", id, err)` and branch on
  `errors.Is(err, blerrors.ErrNotFound)`, consistent with existing `loadArtifact`
  error handling (all shipment candidates are real on-disk artifacts, so the
  not-found branch is expected inert). This mirrors the established
  `MoveInQueue` / `serializer_provenance_hardening` precedent (reload each item
  from Markdown before rewriting). Because `attachCommitToItems` re-persists
  every stamped candidate, the reload seam preserves `item_links` and archive
  provenance uniformly for all members, not just deliberations. Keep the index a
  rebuildable cache — do **not** widen the `GetItem` projection (that would add a
  schema migration, create a links dual-source-of-truth against `item_links`, and
  leave the same class of bug latent for the next modeled field). Preserve the
  ship-gate's Markdown fail-closed provenance read
  (`docs/compound/2026-07-20-...`) — do not simplify it on the basis of any
  newly-trusted field.
- **Files**: `internal/core/shipment_lifecycle.go` (1 file; a small re-persist
  helper plus its call sites — target < 3 functions).
- **Tests (test-first)**: new core regression test — an item carrying a
  `spike_ref` link (and, for a second assertion, an archived item carrying
  provenance) goes through the shipment/archive re-persist path; assert the
  frontmatter `links` block still contains the `spike_ref` and archive
  provenance survives after re-persist. Assert BOTH the populated and the
  empty/nil-links cases to avoid the omitempty false-green
  (`docs/compound/2026-07-21-...`). Write RED first, then implement.
- **Posture**: test-first. **Depends on**: none (independent of Unit 1;
  different concern in the same file — Ship should sequence them to avoid a merge
  conflict, but there is no logical dependency).

## Dependency Graph

```
Unit 1 (D04D63D0 guard)      [independent]
Unit 2 (reload-from-Markdown) [independent]
```

Both units are independent core fixes in the same file. There is no logical
dependency; the prior draft's Unit-1-foundational ordering was removed after
plan-review (the DB-projection seam it depended on was rejected). Ship should
land them in either order and resolve the shared-file merge locally.

## Decisions and Rationale

- **Reload-from-Markdown at the core seam, not a widened DB projection**
  (revised after plan-review attempt 1). The index is an explicitly rebuildable
  cache; Markdown is the source of truth. `item_links` is a separate normalized
  table and provenance is unindexed, so widening `selectCols` would require a
  schema migration, create a links dual-source-of-truth, and only patch the three
  enumerated fields — any future modeled field would silently re-break. Reloading
  from Markdown before re-persist (the `MoveInQueue` precedent) fixes the whole
  class of dropped-field defects and needs no migration.
- **Two independent units, decoupled**. Unit 1 (skip archived deliberations) is a
  self-contained semantic-correctness fix for the abort; Unit 2 (reload seam) is
  the field-fidelity fix covering all stamped members. Neither blocks the other.

## Risks and Caveats

- **Reload cost**: reloading from Markdown adds a file read per re-persisted
  candidate. Shipment archive sets are small (feature + tasks + linked
  deliberations), so the cost is negligible and matches the accepted
  `MoveInQueue` precedent.
- **Empty-vs-populated links round-trip**: the regression test MUST assert both
  the populated (`spike_ref` survives) and the empty/nil case to avoid the
  documented omitempty false-green (`docs/compound/2026-07-21-...`).
- **Provenance fail-closed invariant**: the ship-gate's Markdown-based
  `archived_status` read must remain fail-closed
  (`docs/compound/2026-07-20-...`); this plan does not relax it.
- **Partial-ship recovery** (`backlogit archive <shipment-id>`) remains valid and
  is not regressed.

## Constitution Check (REQUIRED)

- **I. Safety-First Go** — pass. Changes stay in Go (`internal/core`); errors wrap
  with `%w`; no `unsafe`.
- **II. Test-First Development (NON-NEGOTIABLE)** — pass. Each unit lands a
  failing (RED) harness before production code: ship-with-archived-linked-
  deliberation test (Unit 1); links-and-provenance-survive-re-persist test,
  populated + empty cases (Unit 2).
- **III. Workspace Isolation / Security Boundaries** — N/A. No new file-system
  surface; reloads read existing in-workspace artifacts.
- **IV. CLI Workspace Containment (NON-NEGOTIABLE)** — N/A. No out-of-tree writes.
- **V. Structured Observability** — pass. Behavior traces through existing
  shipment lifecycle logging; no reduction.
- **VI. Single Responsibility** — pass. No new dependencies.
- **VII. Destructive Command Approval (NON-NEGOTIABLE)** — N/A. No destructive
  commands; archival flows through the existing guarded write boundary.
- **VIII. Explicit Safety Modes** — pass. Blast radius is small and bounded to
  one core file (`shipment_lifecycle.go`); freeze-scope to `internal/core` is the
  posture for Ship. No production-config or large-blast-radius action.
- **IX. Git-Friendly Persistence** — pass. Reload-from-Markdown preserves the
  existing deterministic frontmatter serialization (stable key ordering via
  `ToFrontmatterMap`), keeping re-persisted Markdown Git-mergeable; no index
  schema change is introduced.
- **X. Agent Context Efficiency** — pass. The index stays the query-first,
  token-efficient access mechanism; this fix does not push callers toward bulk
  Markdown reads — only the small re-persist path (already touching disk) reloads
  its own candidates.
- **XI. Merge Commit History Preservation (NON-NEGOTIABLE)** — pass. Ships via a
  merge commit; no squash/rebase.

Constitution Check: pass

## Plan Hardening Signals (REQUIRED)

- public API, schema, or contract change — **absent** (no schema/DDL change; the
  reload seam is internal core logic).
- security, auth, permission, or compliance-sensitive behavior — **absent**.
- migration, backfill, destructive data/config action, or irreversible step —
  **absent** (no schema migration; the reload-from-Markdown seam needs none).
- external integration, operator checkpoint, or external dependency — **absent**.
- high runtime, rollout, or rollback risk — **absent** (bounded core fix with
  regression tests; rollback is a code revert).

Requires plan hardening: no

## Runtime Verification and Closure

- **Runtime surface**: `backlogit shipment ship` / archive flow (CLI + MCP).
- **Runtime verification (Ship)**: reproduce D04D63D0 — ship a shipment whose
  feature links an already-archived deliberation — and confirm it completes and
  archives the manifest; confirm a `spike_ref` link and archive provenance
  survive an archive/shipment round-trip (7A965F8A). Covered by the regression
  tests plus a `backlogit` shipment/archive smoke check.
- **Closure**: no monitoring/rollback infrastructure required (local CLI tool);
  rollback trigger is a failing regression test → revert commit. At Ship closure,
  graduate the shared finding — *shipment re-persist must reload from Markdown
  (source of truth) because the index carries neither `item_links` nor archive
  provenance* — into `docs/compound/`.

<!-- plan-review-attempt: 1 (FAIL: DB-projection premise factually wrong — items table has no links/archived_from/archived_status columns; item_links is a separate table. Revised to reload-from-Markdown core seam and decoupled the two units.) -->

## Plan Review

dispatch_mode: multi-agent-dispatch

decision: PASS

**Attempt**: 2 (attempt 1 FAILed on the DB-projection premise; see the
`plan-review-attempt: 1` note above). Reviewer sub-agent dispatch was available,
so all selected personas were dispatched as independent sub-agents
(`TOOL_OK: reviewer-subagent-dispatch`). Every selected persona completed and
returned findings; no partial coverage.

**Personas dispatched (both attempts)**: Constitution Reviewer, Go Reviewer,
Scope Boundary Auditor (always-on), Architecture Strategist (cross-model,
always-triggered), Learnings Researcher (always-on). Agent-Native Parity Reviewer
and Security Lens Reviewer were **not triggered**: the plan changes internal core
re-persist logic only — it exposes no new MCP tool or agent-facing action, and
touches no auth/authz, sensitive data store, external integration, or secrets.

**Gate rationale**: All attempt-1 P1 findings are RESOLVED in attempt 2. The
architecture, Go, and learnings personas independently confirmed the corrected
seam (reload-from-Markdown at `internal/core/shipment_lifecycle.go`, per the
`MoveInQueue` / `serializer_provenance_hardening` precedent; the DB `GetItem`
projection is explicitly NOT widened), the corrected schema premise (the `items`
table has no `links`/`archived_from`/`archived_status` columns; `item_links` is a
separate normalized table), and the now-consistent "no migration" hardening
verdict. Constitution Reviewer returned a clean PASS with VIII/IX/X now mapped.
No P0 or P1 findings remain.

**Plan hardening**: `Requires plan hardening: no`. No hardening signals are
present (no schema/contract change, no migration, no security/external surface),
so no `## Plan Hardening` section is required. Gate satisfied.

### Findings by severity

**P0 / P1**: none.

**P2 (addressed in this revision — folded into the plan)**:
- Go Reviewer: Unit 1's primary seam should be the `attachCommitToItems`
  `Status == StatusArchived` skip (artifact already loaded; avoids a redundant
  per-deliberation `loadArtifact`). → Plan Unit 1 updated to make this the
  preferred seam.
- Go Reviewer: Unit 2 must **replace** `loadArtifact` with the Markdown
  (`findArtifact`) load for the mutate-then-persist candidate — not add a
  supplemental read (coexisting loads would re-introduce the drop / false-green).
  → Plan Unit 2 updated to state the replacement and the error-wrapping
  (`fmt.Errorf(... %w ...)` + `errors.Is(ErrNotFound)`) explicitly.

**P3 (advisory — for Ship awareness, no plan change required)**:
- Architecture Strategist: keep the reload seam confined to the mutate-then-persist
  path; retain the DB fast-path for read-only status checks (e.g. the Unit 1
  archived-skip) to keep the cache-vs-source-of-truth boundary crisp.
- Go Reviewer: prefer table-driven `t.Run` subtests for the populated/empty link
  cases and use `t.Helper()` in provenance-assertion helpers, matching
  `serializer_provenance_hardening_test.go`.
- Learnings Researcher: at Ship, confirm no gate/decision code is refactored to
  trust a now-reloaded `Artifact` field in lieu of the Markdown fail-closed read
  (`docs/compound/2026-07-20-...`); type-check the empty-links frontmatter shape
  so a missing-vs-empty regression fails.
- Scope Boundary Auditor: stale rejected-option text in the deliberation's Risks
  section → pruned in this revision.

**Runtime verification / closure**: the plan's Runtime Verification and Closure
section covers the affected runtime surface (`backlogit shipment ship` / archive)
with concrete verification steps and a compound-learning graduation at closure.
No gaps.

<!-- plan-review-attempt: 2 (PASS) -->
