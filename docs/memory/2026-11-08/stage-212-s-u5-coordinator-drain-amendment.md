---
title: "Stage 212-S Wave 8 U5 Coordinator-Drain and 0D073C21 Contract Amendment"
shipment_id: "212-S"
feature_id: "211-F"
phase: "stage-wave8-u5-contract-amendment"
status: "ready-for-ship"
---

## Amendment Summary

Under Operator Option 5 authorization, Stage completed the surgical contract amendment for `211.005-T` (`U5`, Wave 8) in Shipment `212-S` (`211-F`), adjudicating deferred-scope-expansion stash `0D073C21` (`089-DL`) and resolving the Wave 8 coordinator-drain prerequisite halt reported in [212-S M19 Wave 8 U5 execution](212-S-M19-wave8-u5-execution.md) without expanding `M19` or adding any new exported method to `*mcp.Server`:

* **Zero-new-symbol coordinator drain in `(*Server).CloseAllSessions()` (`internal/mcp/session_mux.go`) and `RunMCPDaemon` (`internal/cli/mcp_daemon.go`)**:
  * Because `s.coordinator` is unexported in `package mcp` while `RunMCPDaemon` lives in `package cli` (`internal/cli/mcp_daemon.go`), and because `(*OperationCoordinator).Close()` (`internal/mcp/coordinator.go:288-292`) is idempotent (`c.closeOnce.Do(func() { close(c.closed) }); <-c.loopDone`) and no test in `internal/mcp/session_mux_test.go` calls `InvokeTool` after `CloseAllSessions()`, in `(*Server).CloseAllSessions()` (`internal/mcp/session_mux.go:543-574`), after waiting for all sessions to drain (`<-sess.done`), call `if s.coordinator != nil { result = errors.Join(result, s.coordinator.Close()) }` before publishing `s.sessionDrainErr` and closing `s.sessionDrainDone`.
  * Include `internal/mcp/session_mux.go` in `211.005-T` (`U5`)'s affected files (`Affected files (3): internal/mcp/session_mux.go, internal/cli/mcp_daemon.go, internal/cli/mcp_daemon_multisession_test.go`), so `RunMCPDaemon` in `internal/cli/mcp_daemon.go` drains both active sessions and the `OperationCoordinator` via a single `s.CloseAllSessions()` call (and closes `s.Workspace` when non-nil) on graceful IPC `"stop"`, idle auto-shutdown, or context exit without adding any new exported method to `*mcp.Server`.
* **P-021 C5/C6 four-case triage and reconciliation audit for `0D073C21` (`089-DL`)**:
  * **Source refs carried forward**: `stash_id: 0D073C21` (retained active and unedited in `.backlogit/stash.jsonl` per Ship's single-write capture invariant); `task_id: 211.005-T`; `feature_id: 211-F`; `shipment_id: 212-S`; `PR: N/A (genuinely pre-PR)`; `review_thread_id: N/A (threadless build prerequisite)`.
  * **(A) Unconditional Duplicate Scan Outcome**: `CLEAN DUPLICATE SCAN` across all `302` active lines of `.backlogit/stash.jsonl` and all `317` archived lines of `.backlogit/archive/stash.jsonl` (`0` duplicates).
  * **(B) Mandatory Late-Identifier Reconciliation Outcome**: `No late identifier found — terminal PR: N/A and review-thread ID: N/A stand`.
* **Preserved scheduler, task, harness, checkpoint, and log state**:
  * `211.005-T` retains `status: active`, `labels: [code, track-b, cli, daemon, shim, u5, harness-ready]`, `custom_fields.scheduler_baseline_claim: 212-S`, and its untouched `.backlogit/logs/211.005-T.jsonl` (`783534d15bb9f48e6873cefe0f1d952d38c5323b1e0b1b699c988bb97baa70c5`) with its single `WORK_STARTED: 212-S` event and `1/5` build observations.
  * Checkpoints `checkpoint-20261109-062504.json` (`fabd3f8978dcf5447065b4150d7fa39d2c68d306269504d34b847965fe513c79`) and `checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`), Go source/test files (`internal/mcp/session_mux.go` `3d43d91f07af66ea3b895468fbd5033ce15e1bfc3a0c8b4eeb5873dd569e04b9`, `internal/cli/mcp_daemon.go` `9aef6f7c133d1d604cec9dd3d9b375bf24b1aa6ab7f73d55f6a8d4fa2391f349`, `internal/cli/mcp_daemon_multisession_test.go` `cf3a85a8abe5a3ab3fc1f297a95e3e58b449afc1c7c40cd0a0eb1ea474c4492d`), and git history (`NOFEATURECOMMIT`) are untouched.

## Updated Stage-Owned File Digests (SHA-256)

| File | SHA-256 Digest |
|---|---|
| `.backlogit/queue/211.005-T.md` | `8550e3af0787ac865c2800c0d53af1f3c4889dd00ad04817d491e0466a6ab606` |
| `.backlogit/archive/089-DL.md` | `0a571d59e1b058c6a6305770f8ce39a8be9a5f114bdb4c700142a6a789304480` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `3f3b2f04c0f1b5cae6163d848c8eb90883ee112bc2f838b319abcaed480e9580` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `7feea4c5c6c8e101e0f0f8a79a0516433d86889f0ae73b850270a8687e6794ff` |

## Verification Gate Results

* `backlogit_sync_index`: PASS (`indexed: 2263`)
* `backlogit_docs_lint` (`docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md`): PASS (`valid: true`, `violation_count: 0`)
* `backlogit_docs_lint` (`docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md`): PASS (`valid: true`, `violation_count: 0`)
* `backlogit_docs_lint` (`docs/memory/2026-11-08/stage-212-s-u5-coordinator-drain-amendment.md`): PASS (`valid: true`, `violation_count: 0`)
