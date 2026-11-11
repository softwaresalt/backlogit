---
title: "Stage 212-S Wave 7 U4 Process-Identity and Same-Contract Review-Fix Clarification"
shipment_id: "212-S"
feature_id: "211-F"
phase: "stage-wave7-u4-contract-amendment"
status: "ready-for-ship"
---

## Amendment Summary

Under Operator Option 5 authorization, Stage completed the surgical contract clarification for `211.004-T` (`U4`, Wave 7) in Shipment `212-S` (`211-F`), resolving the Wave 7 review-fix scope question reported in [212-S M19 Wave 7 IPC execution](212-S-M19-wave7-u4-resume.md) without expanding `M19` or modifying `internal/cli/mcp_daemon_lock_unix.go`:

* **Cross-platform `unsafe`-free OS process-identity contract in `isVerifiedProcessAlive` (`U4` / `211.004-T`)**:
  * In `205-S` (`internal/cli/mcp_daemon_terminate_linux.go` and `internal/cli/mcp_daemon_terminate_other.go`), the repository already established the `unsafe`-free (Constitution Principle I) cross-platform OS process-identity contract:
    * On Windows (`internal/cli/mcp_ipc_windows.go`): `isVerifiedProcessAlive` uses `windows.OpenProcess`, `windows.GetProcessTimes` (`CreationTime.Nanoseconds() == startTime`), `windows.QueryFullProcessImageName`, and zero-timeout `windows.WaitForSingleObject(handle, 0) == windows.WAIT_TIMEOUT`.
    * On POSIX (`internal/cli/mcp_ipc_unix.go`): `isVerifiedProcessAlive` reads `/proc/<pid>/stat` field 22 directly (requiring `starttime > 0` and never using `processStartTime`'s `1` fallback when `/proc` is absent) and `/proc/<pid>/exe` (stripping any `" (deleted)"` suffix as in `mcp_daemon_terminate_linux.go` and comparing canonical `filepath.Clean` paths before re-verifying `/proc/<pid>/stat` field 22 == `start`), and **intentionally fails closed (`return false`) on non-procfs POSIX hosts** (`!windows && !linux`), matching `mcp_daemon_terminate_other.go` without modifying `internal/cli/mcp_daemon_lock_unix.go`. (In `U5`, live daemon status/attachment uses the authenticated IPC `"status"` control probe over the workspace-contained socket when `info.Endpoint != ""`.)
* **Three same-contract `U4` refinements inside `internal/cli/mcp_ipc_unix.go` and `internal/cli/mcp_ipc_windows.go`** (`Affected files (3): internal/cli/mcp_ipc_windows.go, internal/cli/mcp_ipc_unix.go, internal/cli/mcp_ipc_test.go`):
  1. In `ListenMCPDaemonIPC` (`mcp_ipc_unix.go`), require `filepath.Clean(endpoint) == endpoint` and `filepath.Dir(endpoint) == filepath.Clean(runtimeDir)` before bind (rejecting unclean `x/..` segments before `net.Listen`), and use identity-checked `runtimeRoot.Remove(leafName)` cleanup if post-bind verification fails.
  2. In `(*mcpPipeConn).Read` (`mcp_ipc_windows.go`), return `0, io.EOF` directly when `errors.Is(err, io.EOF)` so callers receive unwrapped `io.EOF` on peer disconnect.
  3. In `(*mcpPipeConn).await` (`mcp_ipc_windows.go`), when `operationErr != nil` and `cancelErr == nil && drainErr == nil`, return `operationErr` directly (and unwrap single-error `os.ErrDeadlineExceeded` in `Read`/`Write` or preserve `net.Error` `Timeout() == true` compliance).
* **Preserved scheduler, task, draft, and log state**:
  * `211.004-T` retains `status: active`, `labels: [code, track-b, cli, ipc, u4, harness-ready]`, `custom_fields.scheduler_baseline_claim: 212-S`, and its untouched `.backlogit/logs/211.004-T.jsonl` with its single `WORK_STARTED: 212-S` event (`d90b35ca4f948ad524893c2c75487aa3b34aca6f86b3032d1ab6941b6258211a`).
  * Checkpoints `checkpoint-20261109-052455.json` (`d931f05962417e7ba59323730f5fd632df268907f705bfba51176ba1f881ec36`) and `checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`), Go source/test files (`internal/cli/mcp_ipc_windows.go`, `internal/cli/mcp_ipc_unix.go`, `internal/cli/mcp_ipc_test.go`), and git history (`NOFEATURECOMMIT`) are untouched.

## Updated Stage-Owned File Digests (SHA-256)

| File | SHA-256 Digest |
|---|---|
| `.backlogit/queue/211.004-T.md` | `49dd7001f936c19a95bbbd4558241779c21ee2b5bfeb912bb82e2ec927400a70` |
| `.backlogit/archive/089-DL.md` | `a4573258d203424d4c725a69f309b067c4f086c6b77802f13235598d213ba8cc` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `2b9a550f3e1328c3cd30f7d59b7abd42632308ffed49f6ad27a94881e9fd72a4` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `c8c6a8829ae09ec843dd35e46e744f9dab92ba21134b3b51ee8116dfee097387` |

## Verification Gate Results

* `backlogit_sync_index`: PASS (`indexed: 2262`)
* `backlogit_docs_lint` (`docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md`): PASS (`valid: true`, `violation_count: 0`)
* `backlogit_docs_lint` (`docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md`): PASS (`valid: true`, `violation_count: 0`)
* `backlogit_docs_lint` (`docs/memory/2026-11-08/stage-212-s-u4-procfs-contract-amendment.md`): PASS (`valid: true`, `violation_count: 0`)
