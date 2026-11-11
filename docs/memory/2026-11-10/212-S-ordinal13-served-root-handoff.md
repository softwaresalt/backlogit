---
title: "212-S Ordinal 13 recovery and served-root handoff"
shipment_id: "212-S"
feature_id: "211-F"
phase: ordinal13-preparation
---

## Authority and recovery

* Operator authority: "Proceed." at `2026-11-09T23:24:15Z`, relayed by
  Orchestrator with explicit selection and restore confirmation for
  `checkpoint-20261110-033633.json`
* Selected checkpoint validated conforming and restored through MCP, then
  resolved after successful bounded read-select-summarize with reachable,
  workspace-bound Engram
* Protected `checkpoint-20261106-210317.json` remains active and untouched;
  verified SHA-256:
  `235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`
* Full checkpoint enumeration found no validation/quarantine anomalies
* Hook queue empty; no acknowledgment issued
* Live configuration validated by routing resolver: Ship
  `gpt-6.1-sol / openai / medium / long_context`

## Served-root attestation

* Served workspace: `.`
* Served storage: `.backlogit`
* MCP catalog roots independently matched the handoff; MCP database query
  returned exactly `.backlogit/backlogit.db` as the sole main database
* Shipment `212-S` active, ordered 23-member manifest matches the handoff
* Lifecycle topology gate PASS; shipment branch retained
* Index synchronized before semantic backlog reads
* All 22 task members remain terminal; open RED empty; no reclaim or restart

## Stage custody

Stage disposition SHA-256 verified:
`cbb30c206198b99ec482b7c3f3ce7971ca079ce8b1a0cc1e3db3a67c246598f6`.
All six Stage-listed artifact digests matched before any commit tracking.
Ordinal 12 is consumed FAILED; historical failed count is 11, never reset.
Ordinal 13 alone is authorized: 2700 seconds, 46 first-party packages,
`unfiltered-convergence`, scope `212-S`, exact clean HEAD binding.
Runner SHA-256 verified:
`9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e`.

## Scoped report-only review

Reviewed the preserved working-tree diff for these six files directly,
without implementation delegation or source edits:

* `internal/core/shipment_ops_security_windows_test.go`
* `internal/core/gate/probe.go`
* `internal/core/gate/probe_test.go`
* `internal/core/barrier_seam_ast_test.go`
* `internal/core/shipment_reconcile_evidence_test.go`
* `internal/core/custody_exec_test.go`

Scoped outcome: READY; P0=0, P1=0, P2=0, P3=0 new findings.
Checked bounded transient retry, absolute-binary cross-workspace cache key,
walker exclusions, latched seed initialization errors, independent byte-copy
fixtures, and isolation assertions against the authorized contract.
Existing reported RED/green scoped evidence remains in
`212-S-ordinal12-authority-incident-halt.md`.
Pinned Go 1.24 vet and six-file formatting passed. Lint's default timeout
expired; `golangci-lint run --timeout=10m` then passed with no findings.

Engram was reachable and reported no stale files, but its cache-symbol
inventory/call graph was incomplete. Direct exact-source inspection supplied
the missing context; graph coverage remains degraded, not completeness proof.
Existing residuals `0D073C21`, `D2FCA0FB` and native POSIX validation remain
unresolved. No new follow-up or whole-branch readiness claim is made here.

## Next steps and safety

Commit custody, recovery and preserved changes; associate the source commit
with `211.021-T`, `211.022-T` and `211-F`; commit resulting traceability.
Only after a completely clean tree, bind external coordinator receipts to
that HEAD and run Ordinal 13 once. Failure halts without an automatic retry.
Full build, current-HEAD branch-wide standard/adversarial review and all PR
readiness gates remain pending. Merge is not authorized.

ProposedAction: commit reviewed local changes and run the authorized suite.
ActionRisk: moderate; no source repair, branch switch, history rewrite,
shipment disposition or release closure is authorized. ActionResult:
approved for this bounded execution by the relayed operator directive.
