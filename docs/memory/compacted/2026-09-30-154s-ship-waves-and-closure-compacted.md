---
schema_version: "1.0"
doc_type: memory
title: "Compacted Memory: 154-S Ship waves, PR, and Orchestrator closure (2026-09-30)"
description: "Compacted record of 154-S waves 1 through 5, the lint and race-timeout blockers, PR #466 readiness, and the Orchestrator closure session."
chunk_strategy: h1-h2-h3
compacted_at: "2026-10-17"
---

# 154-S waves, PR, and closure

Shipment `154-S` / feature `173-F` shipped via implementation PR #466 (merge
`6d233d21`) and closure PR #467 (merge `046c0130`). Originals are archived under
`docs/archive/memory/`:
`2026-09-30-ship-154s-wave1-lint-blocked.md`,
`2026-09-30-ship-154s-w2-173009-harness.md`,
`2026-09-30-ship-154s-w2-173009-complete.md`,
`2026-09-30-ship-154s-wave2-converged.md`,
`2026-09-30-ship-154s-w4-race-timeout-halt.md`,
`2026-09-30-ship-154s-pr-ready.md`, and
`2026-09-30-orchestrator-154s-closure-session.md`.

## Wave 1: lint blocker

* `173.006-T` had a compiling RED harness failing on named assertions.
* `golangci-lint run` reported 57 pre-existing findings (50 `errcheck`, 7
  `staticcheck`) outside the harness files. Fixing them was out of scope
  (P-021 C1); deferred stash `AD5AECAF` captured it. Raw `gofmt -l .` walked
  ignored `.copilot/session-state` fixtures, so an LF-normalized check of 867
  tracked Go files was used (zero unformatted).
* Later re-characterization: the local PATH golangci-lint v2.13.2 differs from
  CI's pinned v1.64.8, so the pinned command
  `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=10m`
  is authoritative (`AD5AECAF` became tooling drift).

## Wave 2

* W1 converged at `d816126a`; full suite deferred because `.006`, `.007`, and
  `.008` red deliverables stayed open until their green-makers landed.
* `173.009-T` (U1b): harness commit `181d9194` went RED (missing
  `schedulerBaselineClaimKey` constant, two candidates instead of three).
  Implementation commit `b9bd914b` touched only
  `internal/core/shipment_recovery.go` and `internal/core/shipment_lifecycle.go`;
  `memberRecoveryCandidates` returns the preimage, active with preimage marker,
  then active with the journal shipment ID marker, with the marked candidate
  last. Report-only review was READY (0/0/0/0).
* Convergence: `.009` and the newly closed `.008` selectors GREEN; `.006` and
  `.007` selectors remained RED with green-maker `173.001-T` in wave 3.
  `FULL_SUITE_DEFERRED: wave 2` recorded both selectors and owners.
* The checkpoint-create MCP tool was unavailable; the registry-declared CLI
  fallback (`backlogit checkpoint create --state-dump`) was used.

## Wave 4: race-timeout halt

* W3 converged and its unfiltered suite passed. In W4, `173.003-T` rollback
  tests passed, but the frozen race regression
  `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/... ./internal/db/...`
  timed out after 10 minutes in the pre-existing
  `TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt`.
  It was not rerun, per the no-known-flake-rerun policy.
* P-021: a capture-only deferred entry `6E37FD63` was created with
  `DISCOVERY-STATUS: AMBIGUOUS` (ten unconfirmed candidate IDs). Existing risks
  `D116AF58` and `67F17B6B` did not match.
* Resolution seen later: the amended green-regression command (`-race
  -timeout=30m`, Stage `f8885c11`) passed on its first run (internal/core
  813.4s).

## PR #466 readiness

* All seven tasks done and archived (W1 `.006/.007/.008`, W2 `.009`, W3 `.001`,
  W4 `.003`, W5 `.005`). `.005` had no `harness-ready` or exempt contract, so a
  content-probe harness
  (`tests/integration/scheduler_baseline_marker_contract_doc_test.go`) went
  red then green against `docs/design-docs/scheduler-baseline-marker-contract.md`.
* The uncached full suite passed. An earlier cached run exposed
  `TestDoclineSoftKeys_LiveTrackedCorpus` failing on
  `docs/closure/154-S-residual-risk-prepr.md` lacking frontmatter; fixed.
* Local review `READY_WITH_FOLLOWUPS` (P3=2). CI green on first push.
* Copilot threads: (1) checkpoint `060618` left active, fixed by resolving it;
  (2) claim cascade can activate a non-queued member-parent leaving it unmarked,
  which U1b rejects mid-claim. Out of scope under P-021 C1, captured as
  `E52607F5` (requires deliberation) and documented in the contract doc.
* Residual risks: `AD5AECAF` (lint toolchain drift), `4DB1DFF1` (repo-wide
  errcheck, staticcheck, gofmt debt), `5A1C4D3F` (race-suite duration),
  `D116AF58` (per-member ship validation performance), `67F17B6B` (lock
  sidecar naming collision), `E52607F5`.

## Orchestrator closure session

* PRs #466 and #467 merged; `154-S` and `173-F` archived; reconciliation
  `CLOSED`; `compaction_status` degraded (stashes `A17EE897`, `AF1E5075`).
* Decisions: the pipeline-topology `lifecycle` gate does not apply to post-merge
  closure PRs (Ship Step 6.0.4 needs only P-014 local readiness and the P-018
  Copilot gate; `--force` was not used; stash `F05661B1` tracks the gate/prompt
  defect). Stage moved `173-F` to done because Ship cannot change feature
  status. The operator's manifest-reshuffle design doc was committed in PR #467
  and stashed as `DE3E7B67`.
* Other stashes: `8CCB29CF` (AdoptItem scope correction), `8B52A5F1` (`.lock`
  sidecar collision).
* Open: five stale active 154-S build checkpoints (`045011`, `044842`,
  `043858`, `042027`, `030939`) need operator authorization; operator decision on
  `4DB1DFF1`; the operator must attest that the external scheduler consumed the
  154-S baseline marker before routing any successor shipment.
