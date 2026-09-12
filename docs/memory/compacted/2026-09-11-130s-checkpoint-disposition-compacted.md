---
chunk_strategy: h1-h2-h3
compacted_at: 2026-09-12T02:40:00Z
compacted_from:
  - docs/archive/memory/2026-08-24/circuit-break-pr377-copilot-review-request.md
  - docs/archive/memory/2026-08-24/pr377-review-cycle-limit-memory.md
  - docs/archive/memory/2026-08-24/stage-d3ce9e81-checkpoint-toplevel-keys-memory.md
  - docs/archive/memory/2026-08-24/stage-pr377-plan-review-cycle-16-gate-memory.md
  - docs/archive/memory/2026-08-24/stage-pr377-remediation-cycle-15-memory.md
  - docs/archive/memory/2026-08-24/stage-pr377-remediation-cycle-16-memory.md
  - docs/archive/memory/2026-08-28/130-s-closure-session-scope.md
doc_type: learning
schema_version: "1.0"
source: docs/memory/compacted/2026-09-11-130s-checkpoint-disposition-compacted.md
source_shipment: 130-S
title: "Compacted Session: 130-S Checkpoint Disposition"
---

## Outcome

Shipment `130-S` and feature `147-F` established fail-closed administrative
disposition for stored checkpoints carrying unmodeled or duplicate keys.
Staging PR #377 merged as `d125565b`; the implementation reached `main` at
`856e9819`. The newest completion checkpoint remains at
`docs/memory/2026-08-28/130-s-dark-mode-complete.md`.

## Final Decisions

* Stored non-conforming checkpoints are quarantine-only; resolve and abandon
  refuse them
* No spelling-based or case-fold-based implicit survivor is selected for
  duplicate members
* Diagnostic projection is bounded and isolated from mutation
* Destructive checkpoint disposition requires explicit operator identity and
  reason
* `147.018-T` remained a hard same-merge gate with `147.007-T`,
  `147.008-T`, and `147.009-T`
* The implementation stayed in one release unit and preserved merge-commit
  history

## Review and Failure History

* Copilot review request polling tripped the three-attempt circuit breaker
  before a review completed
* Multiple plan-review cycles failed until quarantine-only semantics,
  duplicate handling, task width, and executable RED evidence converged
* A proposed repair runbook was withdrawn because this release authorized
  disposition, not automated restoration
* Claims that a linked research worktree alone violated P-016 were rejected;
  the single implementation worktree invariant remained authoritative

## Durable Learnings

* Administrative read and rewrite surfaces must preserve raw-token ambiguity
  rather than silently collapse duplicate JSON members
* Quarantine is an operator disposition, not an automatic session-start repair
* Review findings about contract ambiguity require contract consolidation
  before implementation resumes

## Traceability IDs

Primary work IDs:

`130-S`, `147-F`, `147.001-T`, `147.003-T`, `147.004-T`, `147.005-T`,
`147.007-T`, `147.008-T`, `147.009-T`, `147.010-T`, `147.011-T`,
`147.012-T`, `147.013-T`, `147.014-T`, `147.015-T`, `147.016-T`,
`147.017-T`, `147.018-T`, `147.019-T`, `147.020-T`, `147.021-T`,
`147.022-T`, `147.025-T`, `147.026-T`, `147.027-T`, `147.028-T`,
`147.029-T`, `147.030-T`, `147.032-T`, `147.036-T`, `147.038-T`,
`147.041-T`, `147.042-T`, `147.043-T`, `147.044-T`.

Operational, stash, and commit tokens retained from the originals:

`540930D6`, `E4F0DB5E`, `D3CE9E81`, `337F2436`, `E429A031`, `35A27CD0`,
`6FA45E69`, `DBBA62AA`, `D125565B`, `856E9819`, `ECEBE820`, `5803CBD0`,
`CD2AD50B`, `F0716B53`, `E37A8C5A`, `1517AE94`, `901DCA66`, `BC357816`.

## Archived Originals

The seven verbose originals are preserved byte-for-byte under
`docs/archive/memory/2026-08-24/` and `docs/archive/memory/2026-08-28/`.
