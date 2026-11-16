---
title: "216-S pre-merge readiness"
doc_type: closure
source: docs/closure/2026-11-16-216-s-pre-merge-readiness.md
ingested_at: "2026-11-16T14:45:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-16"
  mode: pre-merge
  shipment_id: 216-S
  feature_id: 215-F
  evidence_head: c3c1eb87
  closure_status: READY
  compaction_status: pending
---

## Readiness summary

Shipment `216-S` (`215-F`: Self-Contained Sandbox and Multi-Session Runtime Hardening (CAP-2)) completed all 8 tasks (`215.001-T`..`215.008-T`) across Waves 1–3, resolved all in-scope pre-PR review remediation items (`c3c1eb87`), passed all 50 Go packages, passed `gofmt`, `go vet`, `golangci-lint`, full local build (`go build ./cmd/backlogit`), and passed pre-merge runtime verification (`PASS`).

Pre-merge readiness is `READY` (subject to final current-HEAD local review confirmation, CI checks, and P-018 Copilot review before merge execution).

## Invariants and pre-deploy audits

* **Atomic file replacement (`U1`)**: `atomicfile.ReplaceTempFileInRoot` verifies target directory handle identity (`verifyExpectedDirMatchesHandle` on Windows via volume serial/file index + final path) and applies `MoveFileExW` retry semantics without POSIX/non-Windows build regressions.
* **Toolchain alias validation (`U2`)**: Static toolchain manifest validation detects aliased `backlogit` executables and enforces explicit `--cwd` binding.
* **MCP server shutdown (`U3a`, `U3b`)**: `CloseCoordinator` and `Shutdown` stop both session and coordinator admission before waiting on handler/session drain; `CloseAllSessions` preserves session-only behavior; `RunMCPDaemon` invokes `s.Shutdown()`.
* **TEMP-rooted Git ceiling (`U4`)**: `withTempGitCeiling` resolves the deepest matching `TEMP`/`TMPDIR` candidate via absolute path and cloned environment.
* **ID sequence reservation and rollback (`U5a`, `U5b`)**: `CreateArtifact`, `AdoptItem`, and `ReconstituteShipments` serialize on `id_allocator.lock` using pinned `.locks` `*os.Root` handles, inspect local Git branches with sanitized env and symlink rejection plus pending reconstitute intent journals, and roll back/unreserve on failure without acquiring `B`/`C` inside `id_allocator.lock`.
* **Runtime self-install (`U6`)**: `backlogit init` / `install` installs the runtime binary into the active workspace storage root and wires ignore entries via handle-relative `*os.Root` operations.

## Validator evidence

See [216-S pre-merge runtime verification](2026-11-16-216-s-pre-merge-runtime-verification.md):

* Verdict: `PASS` (`cli` command adapter + isolated multi-branch probe + all 50 Go packages at `c3c1eb87`).
* `.\backlogit.exe install --cwd <temp_repo>` installed `.backlogit/backlogit.exe` and `.gitignore` rules verified by `git check-ignore -v`.
* Cross-branch `add` across `main`, `probe-branch-b`, and `probe-branch-c` produced `001-F`, `002-F`, `003-F` with `.locks/id-sequences.json` preserving all IDs.

## Risky action record

| ProposedAction | Risk | Approval and result |
| --- | --- | --- |
| Harden `atomicfile`, `id_history`, `shipment_lifecycle`, `shipment_reconstitute_exec`, `self_install`, and `mcp.Server` shutdown | High (concurrency, lock order, Windows/POSIX FS safety) | Approved in `docs/plans/2026-11-15-cap2-sandbox-multisession-runtime-hardening-plan.md` (`090-DL`..`095-DL`) and `DARK_MODE_ACTIVE`; implemented and verified across all 50 Go packages |
| Merge PR for shipment `216-S` | Moderate (release handoff) | Pending final current-HEAD local review, CI, P-018 Copilot review, and P-014 merge approval |

## Monitoring plan, owner, and validation window

* **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
* **Validation window**: Immediate post-merge verification on `main` plus 7-day observation window.
* **Healthy signals**:
  * All 50 Go packages, `golangci-lint`, `go vet`, and `go build ./cmd/backlogit` pass on `main`.
  * `backlogit install` installs the workspace runtime executable cleanly with ignore rules active.
  * Multi-session and cross-branch `CreateArtifact`, `AdoptItem`, and `ReconstituteShipments` operations show zero ID collisions and zero lock-order errors.
* **Failure signals**:
  * Any `ErrWriteNotApplied`, `id_allocator.lock` timeout, lock-order violation, or `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` failure.

## Rollback trigger and procedure

* **Rollback trigger**: Any regression in Windows/Linux atomic file replacement, MCP daemon graceful shutdown, TEMP-rooted Git gate execution, ID allocation/rollback, or `init`/`install` ignore wiring on `main`.
* **Rollback procedure**: Revert the `216-S` merge commit (`git revert -m 1 <merge_sha>`) via PR with explicit operator approval; remove workspace `.locks/id-sequences.json` only if an incompatible schema change were introduced (not expected for `schema_version: 1`).

## Source artifact provenance

Feature `215-F` declares `source_stash_id: none` and `source_deliberation_id: none`. Source decision `docs/decisions/2026-11-15-cap2-sandbox-multisession-runtime-hardening-deliberation.md` (`090-DL`..`095-DL`) is referenced as provenance only and remains untouched.

## Residual risks and deferred follow-ups

Four out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` stash entries were captured in `.backlogit/stash.jsonl` during `216-S` local review (awaiting Stage triage):

* `39F868B1` (bug, high): Normalize `/` as well as `\` in `validateSessionPath` (`internal/mcp/session_mux.go:319`) on Windows.
* `E0A6FDAF` (task, medium): Bounded backoff/retry cap in `PrewarmMCPDaemon` (`internal/cli/mcp_daemon.go:386`), `fmt.Fprintf` error checks in `writeMCPStatusToolchain` (`internal/cli/root.go:446`), unreachable nil normalization in `internal/toolchain/verify.go:175`, and `EntrypointPath` validation parity in `internal/toolchain/verify.go:407`.
* `7C26BE01` (task, low): Evaluate Windows handle-relative `SetFileInformationByHandle` (`FileRenameInfo`) in `atomicfile_windows.go:183`, modularize `internal/core/id_history.go`, and refactor `CreateArtifact` pre-create callback into an explicit `Option`.
* `DE5D1EA5` (task, medium): Require `SHA256` digest verification in `verifyLocalTool` (`internal/toolchain/verify.go:501`), replace pre-existing hardcoded `.backlogit`/`.backlog` literals in `internal/toolchain/{launcher,verify}.go` and expand `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot`, return non-retryable shutdown error from `OperationCoordinator.Execute` on `<-c.closed` (`internal/mcp/coordinator.go`), reject duplicate active session IDs in `registerStream` (`internal/mcp/session_mux.go:148`), drain responses on EOF in `proxyMCPSession` (`internal/cli/mcp_daemon.go:508`), and distinguish git exit code 1 vs >=128 in `ExecGitRunner.Verify` (`internal/core/gate/baseref.go:143`).

## Structured releasability and compaction

| Evidence | Requirement | State |
| --- | --- | --- |
| Runtime verification | Optional (`required: false`) | Satisfied (`PASS`) |
| Monitoring plan | `required: false` | Recorded |
| Rollback procedure | `required: false` | Recorded |
| Owner and validation window | `required: false` | Recorded (`_Ship`, 7 days) |
| Final releasability | `required: true` (`status_when_satisfied: READY`) | `READY` |
| P-020 compaction status | Post-merge closure duty | `pending` |
