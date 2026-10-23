---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: done
description: "Post-merge closure and compound refresh for 161-S (baseline convergence RS-W04 lint remediation wave 4)."
doc_type: closure
docline:
  date: 2026-10-23T09:10:00Z
  status: final
  tags:
    - closure
    - compound-refresh
    - 161-S
schema_version: "1.0"
shipment_id: 161-S
source: docs/closure/161-S-175-F-post-merge-closure.md
title: "161-S closure and compound refresh"
---

# 161-S closure and compound refresh

Context: feature 175-F (baseline convergence), PR #570, merge `c639db6d137702101ba7a8cd19aa9611709aaad4`. Members: 175.031-T through 175.039-T (9 tasks, task-only manifest by decomposition decision H6). Changes: checked `Close` error returns, one `errors.Join` temp-file cleanup, and 12 QF1012 `fmt.Fprintf` rewrites (reporter.go slice 1/2) across `internal/mcp`, `internal/stash`, and `internal/telemetry`, with nine task-scoped AST harnesses.

## Releasability

`closure_status: READY_WITH_CONDITIONS`. There is no deployed runtime surface (library/CLI only), but PR #570 changed production error paths: `ReadSessionStore` and `HarvestEventsFacts` now propagate close failures, and `internal/stash/jsonl.go` temp-file cleanup now joins removal errors. Validation evidence: existing behavioural tests for those paths pass with uncached per-package `go test -count=1` (`internal/stash`, `internal/telemetry`) and in CI `test`; the nine scoped AST harnesses pass. Gap: no test forces a close/remove failure, so the new error-joining branches are shape-verified, not behaviourally exercised. Condition: stash `D8391636` adds the behavioural test; until then those branches are covered only by review. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #570. Owner: operator. Validation window: next PR to main.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds 27 files / ~100 KB, under the 40-file / 500 KB thresholds, so there were no candidates and nothing was archived.

## Evidence

* Local review: 3-persona adversarial review of `277f4a25` READY_WITH_FOLLOWUPS (P0=0, P1=0, P2=2, P3 advisory). Later head `cab34cb2` differed only by a 2-line doc comment, backlog state, and a memory note.
* Lint: golangci-lint v2.13.2 shows 49 owned findings removed (87 to 38 across the three packages); remaining findings are owned by later slices/tasks per `docs/decisions/baseline-lint-inventory.json` or are the pre-existing non-inventoried set (stash `558861F9`). `task_lint_cmd` is a recorded non-gating TOOL_DEGRADED (stash `C007D571`).
* Copilot: one review iteration, zero comments/threads; P-018 gate SATISFIED at `cab34cb2` (re-run at last mile).
* CI: all checks green at `cab34cb2`.
* Full suite: `go build`, `go vet`, gofmt clean; all 46 packages ok with uncached per-package `-count=1`. `internal/core` `TestNextGateEvidenceCounter_ConcurrentAllocationsAreUnique` and `..._LockSurvivesPastStaleTTLWhileHeld` failed once under machine load (concurrent MCP and lint activity) in an untouched package and passed on isolated rerun.
* Transient: `backlogit_claim_shipment` (twice) and `backlogit_ship_shipment` timed out client-side. Claim completed server-side (verified, post_claim gate OK). Ship was not retried; the item log records `shipment_status_changed` (shipped) 01:43:01, `commit_tracked` (c639db6d) 01:43:13 and the server's `archived` event 01:59:41; `161-S` is archived with `archived_status: shipped` and `commit: c639db6d`; `backlogit doctor --check-shipped-event-completeness` reports no `161-S` finding. No operator reconciliation required.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` is kept (reproduced again; the archival lag was about 16 minutes this time).

## Follow-ups

* Stash `558861F9`, `C007D571`, `3C2B15C4`, `8E8CD750`, `69AF7B15` (previously captured; unchanged).
* Stash `D8391636` (advisory): harness 175.038 hardcodes residual count 14 (update when slice 2/2 lands); optional behavioural test for `internal/stash/jsonl.go` temp-file cleanup.
