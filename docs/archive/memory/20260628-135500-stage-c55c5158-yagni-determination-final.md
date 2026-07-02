# Stage Session — C55C5158 durable high-water-mark counter (YAGNI determination)

Date: 2026-06-28
Agent: Stage
Entry point: operator-targeted stash `C55C5158` (design-gated enhancement, medium, kind=task)

## Outcome (terminal): DO NOT BUILD — archived as YAGNI

Single-entry session (Step 1.5 solo-group fallback). Deliberation-first per the
design gate. Terminal decision: the durable per-artifact-type high-water-mark
counter is **not genuinely additive** over the shipped 066-S canonical pre-write
uniqueness guard. Archived `C55C5158` (consumed/won't-do). No plan, no harvest,
no shipment.

## Gate 1 (PRIMARY) — NOT additive (code-grounded)

Out-of-view window analysis:
- Window A (same-ws stale SQLite index — the window the stash names): ALREADY
  CLOSED. `CreateArtifact` runs `scanCanonicalArtifacts` (force-includes the fixed
  `.backlogit/archive` dir, `internal/core/canonical_scan.go:44-65`) and fails loud
  with `ErrIDCollision` (`internal/core/artifacts.go:159-172`); pinned by
  `066_create_guard_test.go` and `canonical_scan_test.go:144`. A counter would only
  convert fail-loud into silent auto-advance (UX preference, not integrity).
- Window B (cross-branch / not-yet-merged — the only window beyond the scan):
  closed by NEITHER persistence model pre-merge. Git-committed counter is
  branch-local until merge (no better than scanning that branch); local-only is
  blind to other branches and must be re-seeded from canonical max on clone. At
  merge, existing detect (doctor `FindingRootIDCollision`) + refuse (`ErrIDCollision`,
  `ErrArchiveDestinationOccupied`) + warn (rehydrate dup-source) trio handles it.

## Gate 2 (persistence model) — MOOT

Because Gate 1 = do not build. Documented that no persistence model is additive,
so the Git-committed vs local-only decision does not need to be made.

## Artifacts
- Decision doc: `docs/decisions/2026-06-28-durable-highwater-counter-yagni-determination-deliberation.md`
  (docline-compliant; `docs lint` = 0 violations; `promoted_to: none`).
- Stash `C55C5158`: active -> archived (`.backlogit/stash.jsonl` -> `.backlogit/archive/stash.jsonl`).

## Residual / follow-ups (already tracked elsewhere)
- Scan hot-path cost: stash `D6B44FF6` (066-S review P2). A counter would ADD writes, not remove the scan.
- If a future *product* requirement for audit-stable never-reuse ordinals (distinct
  from integrity) emerges, re-open under that explicit framing — not via this stash.

## Next step
Land on main via staging branch `chore/stage-c55c5158-decision` -> PR -> CI green
(test 1.24 + Docline gate) -> Copilot readiness gate (unresolved threads = 0) ->
HALT for operator merge approval (merge commit, P-009). No self-merge.
