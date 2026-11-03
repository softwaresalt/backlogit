---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure and compound refresh for 162-S (baseline convergence RS-W05 lint remediation wave 5)."
doc_type: closure
docline:
  date: 2026-10-23T23:30:00Z
  status: final
  tags:
    - closure
    - compound-refresh
    - 162-S
schema_version: "1.0"
shipment_id: 162-S
source: docs/closure/162-S-175-F-post-merge-closure.md
title: "162-S closure and compound refresh"
---

# 162-S closure and compound refresh

Context: feature 175-F (baseline convergence), PR #573, merge `caa245db4920c4ea7dfbdb652a261a18a34a2bd6`. Members: 175.041, 175.042, 175.045, 175.046, 175.047, 175.048, 175.049, 175.050, 175.051 (9 tasks, task-only manifest by decomposition decision H6). Changes: checked `fmt.Fprint*` and `Close` error returns in `internal/cli` (migrate, telemetry slice 1/3, docs, delete, link, root, shipment_reconcile_shipped and their tests), 14 QF1012 rewrites in `internal/telemetry/reporter.go` (slice 2/2), nine AST harnesses, plus a review-driven fix so `migrate` still rehydrates after a failed summary write (regression test added).

## Releasability

`closure_status: READY`. There is no deployed runtime surface (library and CLI only). Behaviour change: a failed write of a success message after a committed side effect now returns an error. The one path that could skip follow-on work (`migrate`) was fixed in review and is behaviourally tested. No unmet condition exists, so no `conditions:` block is required. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #573. Owner: operator. Validation window: next PR to main.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` stays under the 40-file / 500 KB thresholds, so there were no candidates.

## Evidence

* Local review: three personas (Correctness, Go, Scope Boundary) on `5a8e6769`: P0=0, one P1 (harness false-pass via `_ =` and `go` statements, and silently missing owned functions) fixed in `9ac957bf` and re-reviewed clean; P2 and P3 advisory. A scope over-fix (telemetry.go cleared 9 occurrences owned by slices 175.043 and 175.044) was caught before review and restored in `5a8e6769` per P-021.
* Copilot: one finding (migrate summary write skipped rehydration) fixed in `8cc487b0`, replied and resolved; re-review at `8cc487b0` clean; P-018 gate SATISFIED, re-run at last mile.
* CI: all checks green at `8cc487b0`.
* Lint: golangci-lint v2.13.2 shows zero findings in owned files except the contract residual in `telemetry.go` (Fprintf 9, Fprintln 8, Fprint 6, Close 1) owned by 175.043 and 175.044. `task_lint_cmd` remains a recorded non-gating TOOL_DEGRADED (stash `C007D571`).
* Full suite: build, vet, gofmt clean; all packages ok with `-count=1`. `internal/core` hit the 30 minute timeout in the concurrent full run (machine load) and passed in isolation (2095s); it does not depend on `internal/cli` or `internal/telemetry`.
* Transient: `backlogit_claim_shipment`, nine `backlogit_move_item` calls, and `backlogit_ship_shipment` timed out client-side under load. Each completed server-side and was verified by quiescence polling; ship was not retried.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

No entry in `docs/compound/` is superseded. A new note records the closure-gate lesson: `docs/compound/2026-10-23-closure-gate-requires-compaction-and-conditions.md`.

## Follow-ups

* Previously captured stash entries unchanged (`558861F9`, `C007D571`, `3C2B15C4`, `8E8CD750`, `69AF7B15`, `D8391636`, `3FF72EB8`).
* `autoharness gate closure-evidence` fails only at `close_path` (known posture, stash `3FF72EB8`); no `close_path` is fabricated.
* Residual P2: the wave-5 AST harnesses match only literal `ws` and `fmt` receivers and duplicate one walker nine times; consolidate in a later harness task.
