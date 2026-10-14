---
title: "Compacted memory: 2026-09-19 baseline-convergence restage and decomposition (PR 449)"
date_range: "2026-09-19 to 2026-09-19"
doc_type: compacted-memory
source_count: 1
---

# Compacted memory: 2026-09-19

Compacts 1 source file. The original was moved to docs/archive/memory/ (not deleted).

## Decisions made

- A clean staging branch, stage/baseline-convergence-decomposed, was cut from origin/main at 37a5cba. There was no second worktree (P-016).
- Only Stage-owned backlog (feature 175-F and 98 remediation tasks) and eight planning docs were ported. All Ship-authored implementation was excluded. No Go source was touched.
- The oversized single shipment 156-S was decomposed into 13 wave-aligned replacement shipments, 157-S through 169-S (RS-W00 to RS-W12). They are chained by blocks dependencies.
- 156-S was superseded non-destructively. It stayed queued with items cleared, a SUPERSEDED DO NOT CLAIM banner, and a guard edge 156-S depends_on 169-S. 149-S was rewired to 169-S as advisory ordering.
- Runner-bootstrap prerequisites: tasks 175.099-T (task-lint), 175.100-T (baseline-lint), and 175.101-T (terminal-lint) were added in a new prerequisite shipment 176-S. The sequence is gated by 157-S depends_on 176-S. The feature totals 101 tasks in 14 shipments.
- Each bootstrap harness is behavioral and fixture-driven (success and fail-closed scenarios with divergent exit codes). The owned Go harness is the gate, not the created runner.
- The terminal runner covers both windows and linux surfaces and emits TERMINAL-LINT-OK:175-F only after both pass. U12 (175.012-T) delegates its terminal lint gate to it.

## Files modified or created

- Backlog: 175-F, tasks 175.001-T through 175.101-T, shipments 157-S through 169-S, prerequisite shipment 176-S, and 156-S (superseded).
- Eight planning documents under docs/.
- Scripts named by the plan: scripts/verify-task-lint.ps1, scripts/verify-baseline-lint.ps1, scripts/verify-terminal-lint.ps1 (runner bootstrap targets).

## Key learnings

- Invariants INV1 to INV9 all passed. Examples: 101 unique executable tasks, no 175-F in any shipment, all queued, acyclic chain including the 176 to 157 gate.
- Each bootstrap gate is buildable before its runner exists (RED before, GREEN after).
- The restage was driven by an operator directive to abandon PR 448 and restage.

## Failed or rejected approaches

- PR 448 was abandoned and closed unmerged.
- One oversized bootstrap task was split into three runner tasks (cycle 9 of PR 449).
- Carrying the 168.001-T archive move onto this branch. It was later reverted on 2026-09-26, because it carried Ship-owned state from an unmerged branch.

## Outcomes and open items

- PR 449 tracks this staging work. No shipment is active. The Orchestrator owns the staging merge gate.
- Governance follow-ups remain in stash: 6434A4D7 (tool-level prerequisite and claim enforcement) and B633E9B9 (machine-authenticated reauthorization).
- Four unrelated deferred stash entries stayed active: 3F06493B, 7B71AD77, 156F65EB, FF6D467A.
- Provenance: old branch stage/baseline-convergence-main at d4d394e1 was left untouched as read-only evidence.

## Sources

- docs/memory/2026-09-19-baseline-convergence-decomposition-session.md
