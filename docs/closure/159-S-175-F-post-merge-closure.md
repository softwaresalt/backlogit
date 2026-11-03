---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure and compound refresh for 159-S (baseline convergence RS-W02 errcheck remediation + U14 line-endings workflow)."
doc_type: closure
docline:
  date: 2026-10-18T23:00:00Z
  status: final
  tags:
    - closure
    - compound-refresh
    - 159-S
schema_version: "1.0"
shipment_id: 159-S
source: docs/closure/159-S-175-F-post-merge-closure.md
title: "159-S closure and compound refresh"
---

# 159-S closure and compound refresh

Context: feature 175-F (baseline convergence), PR #536, merge `28060960737a26ae653be79f5e3ba2019d2b1799`. Members: 175.011, 175.013-175.021 (10 tasks). Changes: handled previously-unchecked `Close`/`fmt.Fprint*` error returns in `internal/cli` and `internal/core`, added the always-run `.github/workflows/line-endings.yml` (U14), and added task-scoped harnesses.

## Releasability

`closure_status: READY`. No runtime surface, so runtime-verification is not applicable. Monitoring: the new `Line endings guard` workflow reports on every PR and push to main. Rollback: revert PR #536. Owner: operator. Validation window: next PR to main (guard must report).

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; `docs/memory/` holds 12 files / 63 KB, under the 40-file / 500 KB thresholds, so there were no candidates and nothing was archived (no incoming-reference risk, cf. the BDDB9F9C lesson).

## Evidence

* Local review: 3-reviewer adversarial review of HEAD `e2b75e44`: READY_WITH_FOLLOWUPS (P0=0, P1=0, P2=2 out-of-scope, P3=5 advisory).
* Copilot: two review iterations; 3 threads fixed, replied and resolved (streaming reads with checked close in `doctor.go`; harness pin validation) plus one review-body finding fixed (exact guard invocation). Final review "Approval recommended", P-018 gate SATISFIED at `905383c6`.
* CI: all required checks green at `905383c6` (including the non-required pipeline-topology check).
* migrate.go slice 1/2 retired exactly 10 of 19 flagged calls; 9 remain for the later slice.
* Transient: backlogit MCP claim, five task moves and ship timed out client-side (~60s) and converged server-side; verified by polling, no CLI retry.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` is kept (this run reproduced it: claim, five task moves and ship all timed out client-side and converged server-side; ship took roughly 20 minutes).

## Follow-ups

* Stash `3C2B15C4` (DEFERRED SCOPE EXPANSION): pre-existing `_ =` discards in `registry_parity_test.go` and `shipment_reconcile_fs_windows.go` plus extending the lintfix AST harnesses to `AssignStmt`.
