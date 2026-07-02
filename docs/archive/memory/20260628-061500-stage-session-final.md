# Stage session final memory — 2026-06-28

- **Session type:** stash-to-backlog stage pass (triage + grooming + plan-and-harvest)
- **Repo:** softwaresalt/backlogit @ main (base merge commit fd7b68f0)
- **Outcome:** 6 stash entries archived; feature 068-F + 4 tasks harvested; shipment 068-S (queued) assembled.

## Stash grooming results (6 archived total)

### Operator-flagged stale (category A) — verified consumed/discharged
- `98C4F063` (deliberation, high) — docline policy for generated CLI docs. RESOLVED via Option A
  in 065-S RUN 2 (PR #137): cmd/gen-docs emits docline frontmatter via shared codec; docs lint clean.
- `E4B7767C` (task, high) — gen-docs vs docline-migration conflict on docs/cli-reference/. Same
  resolution (Option A, PR #137). Verified, archived consumed.
- `71A2CB10` (task, low) — compact-context on docs/memory/. DISCHARGED by 067-S closure compaction;
  docs/memory/ confirmed = 18 files (< 40 threshold). Archived discharged.

### Bonus stale finds (category C docline cluster) — discovered via diligence
- `0615F487` (docline ApplyMigration zero-write preflight) — already RESOLVED on main by commit
  a366bd3d (preflight present in service.go:162-192). Archived.
- `A2436E1E` (docline docs migrate --dry-run flag) — already RESOLVED on main by commit 887522ad
  (the --dry-run flag was removed entirely; default is already dry-run). Archived.

### Consumed by this harvest
- `8863C6C8` (task, medium) — shared frontmatter codec extraction. Promoted to feature 068-F.

### Category C entries KEPT active (still valid on main)
- `AE53BC5C` (docline ApplyMigration TOCTOU re-read) — VALID, kept.
- `B349CBED` (docline ValidateFields JSON-schema) — VALID, kept.

## Selected work + rationale
Selected `8863C6C8` (medium) over the docline cluster (2 of 4 already resolved, remaining 2 are
small/low) and over `C55C5158` (design-gated, persistence model unresolved — left for its own
deliberation/spike). Rationale: highest-priority no-design-gate well-bounded refactor; removes
active divergence risk (067-S inlined a codec mirroring docline's into core/doctor.go). Grouping
heuristic: solo unit — the codec extraction has no contextually-consistent peer in the active stash
(the remaining medium item C55C5158 is design-gated and in a different domain).

## Pipeline artifacts
- Deliberation: docs/decisions/2026-06-27-shared-frontmatter-codec-extraction-deliberation.md (Option B)
- Plan: docs/exec-plans/2026-06-27-shared-frontmatter-codec-extraction-plan.md
- Plan-review: cycle 1 FAIL (3 P1) -> revised -> cycle 2 PASS (attempt 2)

## Harvested backlog (feature 068-F)
- 068-F — Shared frontmatter codec extraction (feature, queued, medium)
- 068.001-T (U1) — Create internal/mdfront body-preserving codec [no deps]
- 068.002-T (U2) — Add hardened WriteFileAtomic to internal/atomicfile [no deps]
- 068.003-T (U3) — Migrate internal/docline onto leaf packages [deps: U1, U2]
- 068.004-T (U4) — Migrate internal/core/doctor.go onto leaf packages [deps: U1, U2]
- Dependency graph: {U1 ∥ U2} -> {U3 ∥ U4}. Verified via dep list.

## Shipment (handoff token to Ship)
- **068-S** — "Shared frontmatter codec extraction" — status queued.
- Items (parent-first): 068-F, 068.001-T, 068.002-T, 068.003-T, 068.004-T. Manifest verified.

## Active stash after this pass (8 entries — all deferred)
`21E17BFC` (singleton MCP server — contingency, low), `C55C5158` (durable counter — design-gated,
medium), `D6B44FF6` (CreateArtifact scan optimization — low), `2797E9F8` (db logger DI — low),
`D070FD3C` (surface covering feature ID in shipment views — low), `B349CBED` (docline ValidateFields
JSON-schema — low), `AE53BC5C` (docline ApplyMigration TOCTOU — low), `9685B1AA` (malformed
archived_from disposition — low).

## Open questions for operator
- `C55C5158` persistence model (Git-committed vs local-only high-water-mark counter) remains an open
  design gate — needs its own deliberation/spike before it can be planned.
- `9685B1AA` malformed archived_from disposition (clear field / stamp canonical path / keep flag-only)
  is an operator decision; could ride a future doctor-audit follow-up.

## Next (landing — pending)
Branch chore/stage-068-S; commit .backlogit/ + docs/ (conventional msg + Copilot co-author trailer);
PR to main; drive CI green (test (1.24) + Docline frontmatter gate); §1.9 Copilot readiness gate;
HALT for operator merge (merge commit, no self-merge).
