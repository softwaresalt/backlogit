---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
conditions:
  - id: feature-pr-518-merged
    description: "PR #518 (143-S implementation) is merged as a merge commit into main."
    satisfied: true
    evidence: "gh pr view 518: state MERGED, merge commit 6235ab737c18b784a0b33d69a6e13b4076463c45; final PR head 29a51277."
  - id: governed-closure-archived
    description: "Shipment 143-S and its explicit members are archived."
    satisfied: true
    evidence: "ship_shipment with sha 6235ab737c18b784a0b33d69a6e13b4076463c45 archived 143-S and 161-F; tasks 161.001-T..161.003-T were pre-archived and stamped."
description: "143-S (161-F red-test honesty gate) post-merge closure for PR #518, merge 6235ab737c18b784a0b33d69a6e13b4076463c45. READY: the gate is a library and has no deployed runtime surface."
doc_type: closure
docline:
  date: 2026-10-17T09:00:00Z
  status: draft
  tags:
    - operational-closure
    - post-merge
    - 143-S
    - 161-F
feature_id: 161-F
schema_version: "1.0"
shipment_id: 143-S
source: docs/closure/143-S-161-F-post-merge-closure.md
title: "143-S post-merge closure"
---

# 143-S post-merge closure

## Release record

| Field | Value |
|---|---|
| Shipment | `143-S` |
| Feature | `161-F` |
| Implementation PR | [#518](https://github.com/softwaresalt/backlogit/pull/518) |
| Merge commit | `6235ab737c18b784a0b33d69a6e13b4076463c45` (merge commit, P-009) |
| Final reviewed HEAD | `29a512776efff52f6813377b42c44823b53bdb62` (cycle 10 `5f18c5bf`, cycle 11 `29a51277`) |
| Closure branch | `post-merge/143-s9-red-test-honesty-gate` |
| Closure status | `READY` |
| compaction status | `done` |

## Summary

A new package `internal/redhonesty` verifies that red-test evidence is honest: it exports the base commit, overlays head test files, runs the tests at the baseline and requires named assertion failures. `internal/faultline` gains schema v2 (`{parity, red_green}`) with `RedGreenEvidence`. Documented in `docs/design-docs/2026-10-17-s9-red-test-honesty-gate.md`.

## Review record

* Copilot ran 11 fix cycles. The prior sessions stopped at the cycle limits; the operator authorized extensions for this PR only, with stricter P-021 C1 triage, and no scope was widened.
* Cycle 10 (`5f18c5bf`) fixed a cycle-9 regression: overlay eligibility is now separate from discovery exclusion. Cycle 11 (`29a51277`) made overlay use `isTestFile`, so Go-ignored test-suffixed assets stay at baseline. Both have red-first regressions and a design-doc note.
* Copilot reviewed `29a51277` with no new findings and the copilot-review gate returned SATISFIED. All required CI checks passed. Merge was a normal merge commit; no admin fallback was used.
* The unfiltered `go test -timeout=30m -count=1 ./...` passed on the cycle-10 HEAD. On the cycle-11 HEAD, the `internal/redhonesty` package passed and a full run showed three load-induced timing flakes in `internal/core` (`TestUR10_SubprocessCrashReopenRecovery`, `TestIsAncestor`, `TestValidateMemberGateEvidence_StaleRefused`); all three passed on rerun and CI `test` passed. Local golangci-lint is unusable here (built with go1.24 against newer modules); CI lint passed.

## Runtime verification and releasability

Runtime-verification is not applicable: the change is an offline library with no deployed runtime surface. Releasability: `READY`.

* Owner: Ship / operator.
* Rollback: revert merge `6235ab737c18b784a0b33d69a6e13b4076463c45` through a PR that preserves `.backlogit/` state.
* Validation window: S10 and S11 consumers exercise the gate; no monitoring needed.
* The consumer wiring (S10/S11) is planned work (stash `19E87B10`) and does not gate the release.

## Residual risk

* Go-toolchain shapes beyond the documented discovery rules are not guessed at; the gate reports unverifiable (fail closed), as documented in the design doc.
* The gate runs repository test code as CI does; it is not a sandbox.
* The `ship_shipment` MCP call timed out client-side and completed in the server minutes later; state was verified by re-reading, never by a retry. Both archive writes (161-F, then 143-S) landed.

## Governed closure

* Pre-closure baseline was clean apart from `.backlogit/hooks_queue.jsonl`, which is outside the predicted write set. The stale 153-S checkpoint was left alone.
* All three tasks were pre-archived; feature `161-F` was pending governed completion and `143-S` was `active`. The ship archived `143-S` and `161-F` and stamped the tasks with the merge commit.
* Only the shipment, the feature and three tasks were touched, and no archive file was deleted (P-007).
* Provenance: no source stash or deliberation was archived by Ship.

## Follow-ups

Deferred stash entries, all requiring Stage triage: `0ACF635E`, `3A2C710D`, `E6EE4ACD`, `19E87B10`, `95DF7CE9`.

Next shipment `144-S` is pending operator direction; it was not started.

## P-020 compaction

compact-context ran with `target: all`. The two 143-S checkpoints were compacted into `docs/memory/compacted/2026-10-17-143s-ship-compacted.md` and moved, not deleted, to `docs/archive/memory/2026-10-17/`. Outcome: `done`.

## Provenance

Compound refresh: `docs/closure/2026-10-17-143-s-compound-refresh.md`.