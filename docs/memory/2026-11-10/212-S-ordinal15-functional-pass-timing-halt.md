---
title: "212-S Ordinal 15 functional PASS and timing hold"
doc_type: reference
source: docs/memory/2026-11-10/212-S-ordinal15-functional-pass-timing-halt.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: "212-S"
feature_id: "211-F"
phase: ordinal15-preflight-timing-disposition
---

## Outcome

The U2j exact-path snapshot fix passes all four requested integration tests
and all 34 real-Git mutation subtests. The requested under-three-minute
acceptance target is not met. The implementation leaf reported 233.702s;
one caller recheck reported 294.496s. Both exited 0 with the unchanged
five-minute timeout. Neither is evidence of a sub-three-minute PASS.

Ordinal 15 remains authorized and unconsumed: ordinal 15, failed_count 12,
budget_seconds 2700, operation unfiltered-convergence, scope 212-S.
No token or authorization/counter receipts were issued. No unfiltered
full suite was launched. No counter was reset. No PR was created or
updated, and no push or merge was performed.

Return to the Orchestrator for the timing requirement's disposition before
issuing convergence receipts. Do not silently relax the requirement or
claim the performance target passed. Further optimization must stay within
the authorized contract; no test assertion, mutation, or timeout was weakened.

## Recovery and served-root handoff

* The active selected checkpoint was checkpoint-20261110-062748.json.
  Its initial SHA-256 matched
  9354a3d58da18f0aaae62bb14f141c98578c06e24825f192580e836f46607a42.
* The full checkpoint enumeration contained no reported validation anomaly.
  Both Ship-owned active checkpoints were surfaced; only the explicitly
  selected checkpoint was restored and resolved through MCP.
* Bounded Engram retrieval was available and workspace-bound. Recovery
  retained the shipment cursor, checkpoint pointer, and gate verdicts;
  superseded narrative was not replayed.
* Protected checkpoint-20261106-210317.json remains untouched. Its SHA-256
  is 235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70.
* The Stage amendment note and all 19 listed file digests matched, including
  all 12 compound files. The pinned runner digest matched
  9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e.
* Fresh schema-validated Ship routing resolution passed without degradation.
  Backlog MCP was reachable and INDEX_SYNC_OK preceded semantic reads.
* No concrete hook events were returned; no acknowledgment was needed.
* Served workspace is `.`, served storage is `.backlogit`, and the main
  database is `.backlogit/backlogit.db`. Independent MCP metadata and
  pragma_database_list checks matched the supplied roots.
* BRANCH_OK: the existing shipment feature branch remains active.
  The lifecycle topology gate passed: sole active shipment 212-S, matching
  branch, one implementation worktree, and no predecessor.
* The live ordered manifest matched the supplied S23 handoff.
  Frozen M22 remains unchanged; all 22 tasks were already done, and open
  RED is empty. No task was reclaimed, restarted, or re-completed.

## Bounded implementation and review

Freeze-scope applied to the authorized test follow-up. ProposedAction:
replace invalid seed-root snapshot substitution with four exact-path,
exclusive reusable worker slots. ActionRisk: moderate. ActionResult:
implemented and functionally verified; timing acceptance unmet.

The sole implementation leaf changed only
tests/integration/claim_start_wave_sim_contract_test.go. Master Git seeds
and metadata are computed once; each slot captures its baseline at its
actual future comparison path. All 34 parallel mutation subtests retain
exclusive slot ownership, eight live-capture exceptions, and the original
assertions. Fixture-only removal/recreation was explicitly requested by
the operator; no source or planning artifact was deleted.

Existing U1k edits and Ship admission sentence restoration were preserved.
The linter cache retains the additional Linux scan on non-Linux hosts.
No production changes were added during this invocation.

Scoped report-only review returned P0=0, P1=0, P2=0, P3=0 with
READY_WITH_FOLLOWUPS for the timing gap. The reviewed source delta was based
on HEAD 12d7e018ed17c591d891bf67223263bd8ab6f081, scoped diff SHA-256
bb48644eae3f678f9402c123f97f559d7efb0761ae0e4547067d4803d9f63c6d.
No obvious in-scope performance defect was identified. Machine dependence
was not established. This is not a branch-wide current-HEAD PR review.

Existing residuals 0D073C21, D2FCA0FB, native POSIX validation, and incomplete
graph coverage remain unchanged. Timing is a same-contract unmet acceptance
item, not an out-of-scope deferred expansion.

## Verification

All Go commands used the pinned bin/tmp/212-go-env.ps1 environment.

| Check | Result |
|---|---|
| Exact four-test integration selector, -timeout=5m -count=1 | PASS twice; 233.702s and 294.496s; under-three-minute target NOT MET |
| All 34 real-Git mutation subtests | PASS, no weakened assertions |
| Eleven U175 linter contracts with installed v2 linter first on PATH | PASS, 58.039s, no skips |
| Repository compile, go test -run=^$ -count=1 ./... | PASS, 46 packages, no suite executed |
| go vet ./... | PASS |
| Default golangci-lint run --timeout=10m | PASS |
| Tracked-source gofmt in Windows-safe batches | PASS, 1400 files |
| TestU175_001_BaselineEol in ./tests | PASS, 1751 files, zero non-LF/mixed results |

The caller integration recheck ran alongside vet and lint. Its slower
timing does not establish a regression or machine-independent cause.
Evidence is in the tool-generated ignored output
bin/tmp/1794293426851-copilot-tool-output-12284-1469e425-5fa7-4ed2-b3e5-9605ad5f5b1f.txt.
The parent did not duplicate the leaf's source investigation.

Raw gofmt traversal of ignored fixtures is not substituted for the
tracked-source format check. The Stage decision file's existing trailing
blank line was preserved; git diff --check was not represented as a
required gate.

## Next steps

Commit the verified functional changes and Stage custody files, associate
the implementation commit with 211.016-T, 211.022-T, and 211-F through MCP,
then commit those traceability updates and the final recovery checkpoint.
Verify the protected checkpoint and runner again and leave the worktree
clean on the shipment branch.

Only after explicit timing disposition and clean-HEAD prerequisites:
issue fresh HEAD-bound Ordinal 15 coordinator receipts and execute the
pinned unfiltered runner. Require all 46 package receipts and valid evidence.
Full build, branch-wide standard and adversarial reviews, PR creation,
current-HEAD CI/Copilot verification, runtime verification, operational
closure, and explicit merge approval remain pending.
