---
chunk_strategy: h1-h2-h3
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-09-30-ship-154s-pr-ready.md
title: "Ship 154-S: all waves complete, PR #466 open, stopped before merge"
---

# Ship 154-S: PR ready, stopped before merge

## State

- Shipment `154-S` (feature `173-F`) is on branch
  `feat/154-s-shipment-claim-scheduler-baseline-marker`, with a single worktree.
- PR: <https://github.com/softwaresalt/backlogit/pull/466> targets `main`.
  Merge commit strategy (P-009). **Not merged**: merge requires separate
  operator approval (P-014), handled by the Orchestrator.
- All seven tasks in `M` are `done` and archived:
  - W1: `173.006-T`, `173.007-T`, `173.008-T`
  - W2: `173.009-T`
  - W3: `173.001-T`
  - W4: `173.003-T`
  - W5: `173.005-T`
- The resumed checkpoint `checkpoint-20260930-060618.json` is resolved. The
  operator did not authorize resolving five other active Ship 154-S
  checkpoints (`045011`, `044842`, `043858`, `042027`, `030939`), so they were
  left untouched and reported.

## This session

- W4: committed the `173.003-T` rollback harness. Its scoped `^TestU3_` tests
  passed. The amended green-regression command
  (`-race -timeout=30m`, Stage `f8885c11`) passed on its first run
  (internal/core 813.4s). The unfiltered full suite passed at convergence.
- W5: `173.005-T` had neither `harness-ready` nor a `harness-exempt` contract,
  so P-002 routed it to harness-architect. A content-probe harness
  (`tests/integration/scheduler_baseline_marker_contract_doc_test.go`) went red,
  then green, against `docs/design-docs/scheduler-baseline-marker-contract.md`.
- The final full suite `go test -count=1 -timeout=30m ./...` passed uncached.
  An earlier cached run exposed `TestDoclineSoftKeys_LiveTrackedCorpus`
  failing on `docs/closure/154-S-residual-risk-prepr.md`, which had no
  frontmatter. That was fixed.
- Local review: `READY_WITH_FOLLOWUPS` with P0/P1/P2 = 0 and P3 = 2. The doc's
  verification-status row was fixed. The committed terminal `.backlogit/ops`
  journals are inert (informational).
- CI was green on the first push (all 7 checks).
- Copilot review raised two threads:
  - Checkpoint `060618` was left active. Fixed by resolving the checkpoint.
  - The claim cascade can activate a non-queued member-parent while leaving it
    unmarked, and U1b rejects that mid-claim state. Out of scope under P-021
    C1: captured deferred stash `E52607F5` (requires deliberation), and the
    contract doc now states the gap.

## Residual risks

- `AD5AECAF`: the PATH golangci-lint v2 does not match the CI-pinned v1.64.8.
- `4DB1DFF1`: pre-existing repo-wide errcheck, staticcheck and gofmt debt.
- `5A1C4D3F`: race-suite duration.
- `D116AF58`: per-member ship validation performance.
- `67F17B6B`: lock-sidecar naming collision.
- `E52607F5`: non-queued member-parent cascade gap (new this session).

## Next steps

1. The Orchestrator obtains operator merge approval (P-014) and re-runs the
   §1.9 and P-018 gates at the last mile.
2. After the merge is confirmed, Ship runs Step 6 post-merge closure: sync
   `main`, create the `post-merge/` branch, ship `154-S` with the merge SHA,
   run P-020 compact-context, and resync the index.
