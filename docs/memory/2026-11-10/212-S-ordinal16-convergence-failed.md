---
title: "212-S Ordinal 16 consumed convergence failure"
doc_type: reference
source: docs/memory/2026-11-10/212-S-ordinal16-convergence-failed.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
---

# Wave 12 convergence: HALTED

## Completed execution

- Restored and resolved only the selected, valid and conforming `checkpoint-20261110-082120.json` under explicit invocation authority. Bounded recovery summary preserved cursor and gate verdicts; workspace-bound Engram was reachable.
- Protected `checkpoint-20261106-210317.json` remains active and byte-identical: SHA-256 `235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
- Verified all Stage custody digests and independent served-root attestation. Recorded handoff using workspace-relative paths in `212-S-ordinal16-served-root-handoff.md`.
- Delegated only the authorized six-file U1k test-fixture follow-up, retaining assertions and production source. Exact scoped verification, all-46-package compile-cache prewarm, vet, lint, tracked Go formatting, EOL baseline, and scoped report-only review passed.
- Implementation commit: `e460e4361c22d368f8223657c9a630264e1536cd`. Tracked on `211.022-T` and `211-F`; tracking commit: `318775695ab7faf3bccb1954f0208a8e209fb56e`.
- Frozen scope remains S23 / M22, all 22 tasks done, no active/queued/blocked/unsupported task members and no open RED. No task reclaimed, started, or re-completed.

## Single authorized Ordinal 16

- Clean execution HEAD before and after: `318775695ab7faf3bccb1954f0208a8e209fb56e`; worktree remained completely clean through suite completion.
- Runner SHA-256 unchanged: `9aead967085d5a9f45a2705ba4d2035e15b4cf71e160f8dacd4ecf1b500b957e`.
- Command: `& bin/tmp/212-go-env.ps1 python bin/tmp/212-full-suite.py --authorization-token-file bin/tmp/212-ordinal16-35c89494.token --authorization-receipt bin/tmp/212-ordinal16-35c89494-authorization.json --counter-receipt bin/tmp/212-ordinal16-35c89494-counter.json --output-dir bin/_rr35c89494/wave12-ordinal16`.
- Native suite: `go test -count=1 -p=1 -timeout=30m -json ./...`. Ordinal 16, cumulative failed count before 13, external budget 2700 seconds, 46 first-party packages.
- Started `2026-11-10T09:16:04Z`; finished `2026-11-10T09:56:09Z`; elapsed `2404.6069334996864` seconds. No timeout.
- `native_exit=1`, `suite_native_exit=1`, `evidence_valid=true`, `reported_failure_count=2`. All 46 packages reported terminal results: 45 PASS, 1 FAIL (`internal/core`), no missing or extra packages. 3921 top-level tests passed.
- Discovery and LF preflight passed. All five critical receipts passed exactly once. No supervisor/cleanup errors, no poll gap over five seconds; maximum gap approximately 0.036 seconds.
- Result: `bin/_rr35c89494/wave12-ordinal16/result.json`, SHA-256 `f328095cab87edd6c19c374da6da95e26d73f7aef78c73413b005a6e8d14c51a`. Raw output retention is bounded, with omission counters in the result.

## Observed failure, not a proven root cause

- Named failing subtest: `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery/preview_and_verify_zero_write_and_default_branch_refusal`, assertion at `internal/core/custody_exec_test.go:99`, message `read-only preview and verify must not write any file`.
- Its parent `TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery` is the second reported failure; this is one failing assertion, not two independent defects.
- Expected and actual `.backlogit/backlogit.db-wal` content hash is identical: `e51551b6d6770bd72819608b97e18e739b4c95aa251a09073af7317df07df199`.
- Expected WAL modification time: `2026-11-10T09:20:59.9436609Z`; actual: `2026-11-10T09:21:01.8289073Z`. The reported map diff identifies only this timestamp change.
- This observation does not establish whether the PRAGMA change, deferred OS metadata reporting, SQLite activity, or another cause produced the timestamp change. No assertion was weakened, no cause invented, and no diagnostic rerun or fix attempted after the failure.

## Consumption and next decision

- Ordinal 16 is **consumed FAILED**: cumulative `failed_count: 13 -> 14`, zero reset, zero retry.
- Original issuance receipts remain historical records. Authoritative consumption: `bin/tmp/212-ordinal16-35c89494-consumption.json` plus this persisted memory and the final recovery checkpoint. Never replay the original receipts.
- Wave 12 is not converged. The conditional PASS path is not entered: no full local build, branch-wide standard/adversarial current-HEAD review, push, PR creation, CI/Copilot confirmation, runtime verification, operational closure, or merge.
- Return to Orchestrator/operator for Stage-owned failure disposition and any new contract or execution authority. No Ordinal 17 is authorized.
- Remain on `feat/212-s-backlogit-multi-session-singleton-mcp-runtime-stdio-session-shim-and-channel-operation-coordinator`. Preserve all completed task state. No merge authorization exists.
