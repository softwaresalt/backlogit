---
chunk_strategy: h1-h2-h3
decision_status: decided
depth: deep
description: 'Deliberation and P-021 C5/C6 provenance adjudication for CAP-2: self-installing workspace runtime executable, cross-branch ID sequence counter reservation, server-owned MCP coordinator drain API, executable-aware static toolchain validation, atomicfile manifest replacement, and U1k TEMP Git ceiling repair.'
doc_type: decision
linked_artifacts:
  - .backlogit/archive/090-DL.md
  - .backlogit/archive/091-DL.md
  - .backlogit/archive/092-DL.md
  - .backlogit/archive/093-DL.md
  - .backlogit/archive/094-DL.md
  - .backlogit/archive/095-DL.md
  - .backlogit/queue/215-F.md
  - .backlogit/queue/216-S.md
  - docs/plans/2026-11-15-cap2-sandbox-multisession-runtime-hardening-plan.md
promoted_to: both
schema_version: "1.0"
source: docs/decisions/2026-11-15-cap2-sandbox-multisession-runtime-hardening-deliberation.md
tags:
  - cap-2
  - runtime-hardening
  - toolchain
  - mcp-daemon
  - id-allocation
  - git-ceiling
title: 'CAP-2 Deliberation: Self-Contained Sandbox and Multi-Session Runtime Hardening'
topic: 'CAP-2: Self-Contained Sandbox & Multi-Session Runtime Hardening (17214531, 3083FC1A, 0D073C21, D1455316, C5189EAF, 1CED4AD6)'
---

## Problem Frame

Workspaces running inside isolated sandboxes (GitHub Copilot CLI sandboxes, Docker containers, GitHub Codespaces/workspaces) and multi-session developer environments face six coupled runtime and toolchain hardening gaps across `internal/cli`, `internal/core`, `internal/mcp`, `internal/toolchain`, `internal/atomicfile`, and `internal/core/gate`:

1. **`17214531` (`090-DL`, `feature`, `high`) — Self-installing runtime executable in workspace storage root and ignore-file wiring**: `backlogit init` (and `backlogit install`) initializes `.backlog` configuration and directories but does not install the running `backlogit` executable into the workspace storage directory root (`.backlogit/` or `.backlog/`) or add an ignore rule in `.gitignore` or `.git/info/exclude`.
2. **`3083FC1A` (`091-DL`, `bug`, `high`) — Cross-branch and multi-session artifact ID sequence counter reservation/isolation**: `allocateTypedHierarchicalID` and `NextID` (`internal/core/id_history.go`, `internal/core/naming.go`, `internal/core/hierarchy.go`) compute `maxOrdinal + 1` solely from the currently checked-out working tree and SQLite cache without cross-process allocation locking or cross-branch high-water-mark reservation. Switching between branches/sessions (`git checkout`) or running concurrent sessions allocates duplicate `NNN-F` and `NNN.MMM-T` IDs, causing merge conflicts in `.backlogit/queue/` and `.backlogit/archive/`.
3. **`0D073C21` (`092-DL`, `task`, `high`, `DEFERRED SCOPE EXPANSION`) — Server-owned operation-coordinator / full-shutdown drain API**: `RunMCPDaemon` (`internal/cli/mcp_daemon.go:108,167`) requires draining both active sessions and the server-owned `OperationCoordinator` (`internal/mcp/server.go:53,102`) on graceful stop, whereas `(*Server).CloseAllSessions()` (`internal/mcp/session_mux.go:549-582`) must preserve session-only semantics so in-process `InvokeTool` callers can still execute operations after session closure.
4. **`D1455316` (`093-DL`, `bug`, `high`, `DEFERRED SCOPE EXPANSION`) — Executable-aware `backlogit` alias identification in static toolchain validation**: `internal/toolchain/verify.go` (`validBacklogitTool`, `verifyWorkspaceToolchain`, `verifyLocalTool`, `validateMCPConfigPinning`) and `internal/toolchain/launcher.go` (`validateManifestForProvisioning`, `RenderPinnedMCPConfig`) identify `backlogit` solely by `strings.EqualFold(strings.TrimSpace(tool.ID), "backlogit")`. An aliased entry (e.g., `id: "backlog-alias"`, `executable_path: "./bin/backlogit.exe"`) bypasses the mandatory workspace-root `--cwd` binding check.
5. **`C5189EAF` (`094-DL`, `task`, `high`, `DEFERRED SCOPE EXPANSION`) — `WriteToolchainManifest` atomic replacement hardening**: `WriteToolchainManifest` (`internal/toolchain/launcher.go:875`) uses raw `os.Rename(tmpAbs, dstAbs)` instead of reusing `internal/atomicfile`'s Windows `MoveFileEx` sharing-violation retry semantics, permanent-denial detection, and `MOVEFILE_WRITE_THROUGH` durability.
6. **`1CED4AD6` (`095-DL`, `bug`, `high`, `DEFERRED SCOPE EXPANSION`) — U1k `ExecGitRunner.Verify` ambient `TEMP`/`TMPDIR` Git ceiling repair**: `withTempGitCeiling` (`internal/core/gate/baseref.go:154-184`) synthesizes `GIT_CEILING_DIRECTORIES=os.TempDir()` whenever `g.Dir` is a subdirectory of `os.TempDir()`. Because Git refuses to inspect `.git` inside a directory listed in `GIT_CEILING_DIRECTORIES`, a legitimate repository rooted at `TMPDIR` (e.g., `TMPDIR=/work/repo`, repo at `/work/repo/.git`, nested workspace at `/work/repo/subproject`) becomes undiscoverable.

## P-021 C5 and C6 Deferred-Expansion Triage and Reconciliation Audit

All four `DEFERRED SCOPE EXPANSION` entries in CAP-2 were audited under Stage's P-021 C5/C6 intake obligations before entering grouping and planning:

| Stash ID | Deliberation | Originating Source Refs | (A) Unconditional Duplicate Scan Outcome | (B) Mandatory Late-Identifier Reconciliation Outcome |
| --- | --- | --- | --- | --- |
| `0D073C21` | `092-DL` | `task_id=211.005-T` (`U5`), `feature_id=211-F`, `shipment_id=212-S`, `PR=N/A` (pre-PR; `212-S` shipped via PR `#645`), `review_thread_id=N/A` (threadless build prerequisite; `docs/memory/2026-11-08/212-S-M19-wave8-u5-execution.md`, commit `15ac1682`) | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl` and closure/memory records (`0` duplicates) | **No late review-thread identifier found** (threadless pre-PR build prerequisite; shipment `212-S` PR `#645` recorded in place) |
| `D1455316` | `093-DL` | `task_id=212.002-T` (`U9`), `feature_id=212-F`, `shipment_id=213-S`, `PR=N/A` (pre-PR; `213-S` shipped via PR `#647`), `review_thread_id=N/A` (report-only review at HEAD `3b2d937573f3327f331309c30bddc15b69b361cf`) | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl`; candidates `58AA0592` (runtime launcher `cmd.Dir` / shebang) and `EC589C00` (Level 2 binary-header inspection) verified distinct | **No late review-thread identifier found** (pre-PR local report-only review finding; shipment `213-S` PR `#647` recorded in place) |
| `C5189EAF` | `094-DL` | `task_id=213.002-T` (`U12`), `feature_id=213-F`, `shipment_id=214-S`, `PR=649` (late-reconciled), `review_thread_id=PRRT_kwDORzozKM63bqzH` (late-reconciled) | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl` (`0` duplicates) | **RECOVERED late identifiers `PR=649` and `review_thread_id=PRRT_kwDORzozKM63bqzH`** from `docs/closure/2026-11-14-214-s-pre-merge-readiness.md` and updated `C5189EAF` in place via `backlogit_stash_edit` |
| `1CED4AD6` | `095-DL` | `task_id=211.022-T` (`U1k`), `feature_id=211-F`, `shipment_id=212-S`, `PR=645`, `review_thread_id=PRRT_kwDORzozKM615Onq`, `comment_id=4530351422`, `review_id=5849123381`, `head=62064e89cf3b00d1daa6f5a1b0bffe040806e6bf` | **CLEAN DUPLICATE SCAN** across all 349 records in `.backlogit/stash.jsonl`; candidate `60D799D9` (`211.008-T` U1b no-repo fixture / archive / policyengine boundary) adjudicated distinct | **No `N/A` fields at capture** (`PR=645` and `review_thread_id=PRRT_kwDORzozKM615Onq` already recorded; no-op) |

## Research Findings

### Prior Institutional Learnings (`docs/compound/`)

* `docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md` and `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`: Workspace-local binary pinning and ignore rules prevent global `PATH` version skew while keeping git status clean.
* `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md` and `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`: Handle-confined operations (`*os.Root`) must be paired with verified absolute paths for Win32 syscalls (`MoveFileExW`, `GetFileAttributes`), and `os.SameFile` must verify opened handles against `Lstat`.
* `docs/compound/test-failures/windows-retry-error-boundaries-and-handle-aware-observers-2026-11-09.md` and `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`: `internal/atomicfile` implements `MoveFileExW` with `MOVEFILE_REPLACE_EXISTING` (`| MOVEFILE_WRITE_THROUGH` when durable), 6-attempt exponential backoff on transient `ERROR_SHARING_VIOLATION` / `ERROR_ACCESS_DENIED`, immediate exit on `FILE_ATTRIBUTE_READONLY | FILE_ATTRIBUTE_DIRECTORY`, and `ErrWriteNotApplied` wrapping without a pre-remove data-loss window.
* `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md`, `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`, and `docs/compound/concurrency-issues/2026-08-09-stable-lock-keys-and-heartbeat-refresh-for-unbounded-holds.md`: Sequence allocation must protect both root ordinals (`NNN-F`, `NNN-S`, `NNN-DL`) and parent-scoped child ordinals (`NNN.MMM-T`), serialize concurrent callers under an advisory lock, and account for IDs allocated across branches/sessions.
* `docs/compound/concurrency-issues/unregister-completed-request-session-cancellation-2026-11-09.md`: Session closure (`CloseAllSessions`) and server-wide operation draining (`OperationCoordinator.Close`) are distinct lifecycle boundaries.
* `docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md` and `docs/compound/test-failures/short-private-temp-for-windows-git-fixtures-2026-11-06.md`: `GIT_CEILING_DIRECTORIES` stops Git *before* inspecting `.git` in the ceiling directory itself; when `os.TempDir()` is itself a valid repository root, the ceiling must be `filepath.Dir(tempDir)` so nested workspaces under `tempDir` can still resolve `tempDir/.git`.

### Codebase Inspection

* `internal/cli/root.go:315-365` (`newInitCommand`): Currently initializes `.backlog` (refusing if `.backlogit` exists), creates `logs/`, and writes default configs. Adding `Aliases: []string{"install"}` on `newInitCommand` enables `backlogit install` without changing Cobra's generated markdown reference (`cmd/gen-docs/main.go`), keeping CI's `CLI Reference Drift` check clean.
* `internal/core/id_history.go` & `internal/core/artifacts.go:196-256`: `CreateArtifact` and `AdoptItem` call `allocateTypedHierarchicalID` and `NextID` without holding a sequence-allocation lock and without persisting a cross-branch high-water-mark reservation file under `<storage_root>/.locks/`.
* `internal/mcp/session_mux.go:549-582` & `internal/cli/mcp_daemon.go:108,167`: `CloseAllSessions()` currently calls `s.coordinator.Close()` on line 575, breaking session-only semantics for in-process `InvokeTool` callers after `CloseAllSessions()`.
* `internal/toolchain/verify.go:324,373,477,692` & `internal/toolchain/launcher.go:669,935`: Only `strings.EqualFold(strings.TrimSpace(tool.ID), "backlogit")` is checked for `--cwd` workspace binding and `./bin/backlogit.exe` pinning.
* `internal/toolchain/launcher.go:873-878`: `WriteToolchainManifest` calls raw `os.Rename(tmpAbs, dstAbs)` instead of `internal/atomicfile`.
* `internal/core/gate/baseref.go:154-184`: `withTempGitCeiling` unconditionally sets `ceiling = tempDir` whenever `dir` is inside `tempDir`, even when `filepath.Join(tempDir, ".git")` exists.

## Options Evaluated

### Option A: Minimal Patching Without Shared Primitives

* **Description**: Inline a Windows retry loop in `launcher.go`, keep `CloseAllSessions()` closing the coordinator, and rely on manual `backlogit sync` for cross-branch IDs.
* **Pros**: Fewer files touched.
* **Cons**: Fails `0D073C21` (`CloseAllSessions` session-only semantics), fails `3083FC1A` (branch switches still reuse `NNN-F` / `NNN.MMM-T` ordinals), and duplicates `internal/atomicfile` Windows `MoveFileEx` logic.
* **Effort**: Low | **Fit**: Poor.

### Option B: Cohesive Architectural Hardening Across All Six CAP-2 Surfaces (Recommended)

* **Description**:
  1. **`C5189EAF` (`internal/atomicfile` + `internal/toolchain/launcher.go`)**: Export `ReplaceTempFile(tmpName, dstPath string, durable bool) error` in `internal/atomicfile/atomicfile.go` and delegate `WriteToolchainManifest`'s final replacement (after re-verifying destination regular-file identity under `storageHandle`) to `atomicfile.ReplaceTempFile` via an injectable seam `replaceToolchainManifestFile`.
  2. **`D1455316` (`internal/toolchain/verify.go` + `internal/toolchain/launcher.go`)**: Add `isBacklogitExecutable(abs, execPath string) bool` and `isBacklogitTool(abs string, tool ToolEntry) bool` so both `id == "backlogit"` and aliased entries pointing to a `backlogit` executable require `WorkspaceLocalPinned` (`./bin/backlogit.exe`) and workspace-root `--cwd` bindings across `VerifyWorkspaceToolchain`, `ValidateMCPConfigPinning`, `WriteToolchainManifest`, and `RenderPinnedMCPConfig`.
  3. **`0D073C21` (`internal/mcp/session_mux.go` + `internal/cli/mcp_daemon.go`)**: Restore `(*Server).CloseAllSessions()` to session-only drain semantics, expose `(*Server).CloseCoordinator() error` and `(*Server).Shutdown() error` on `*mcp.Server`, and update `RunMCPDaemon` to call `s.Shutdown()`.
  4. **`1CED4AD6` (`internal/core/gate/baseref.go`)**: Update `withTempGitCeiling` so that when `tempDir` / `evalTemp` itself contains `.git` (`os.Lstat(filepath.Join(evalTemp, ".git")) == nil`), the synthesized ceiling is `filepath.Dir(tempDir)` / `filepath.Dir(evalTemp)` (allowing nested workspaces under `tempDir` to discover `tempDir/.git` while blocking any ancestor checkout above `tempDir`), and when `tempDir` has no `.git`, `tempDir` remains the ceiling.
  5. **`3083FC1A` (`internal/core/id_history.go` + `internal/core/artifacts.go`)**: Serialize artifact ID allocation and creation under an in-process mutex + cross-process advisory lock (`<storage_root>/.locks/id_allocator.lock`), persist monotonic sequence reservations in `<storage_root>/.locks/id-sequences.json` (which survives `git checkout` branch switches under git-ignored `.locks/` and rolls back on compensated/failed creates), and include a fingerprint-cached local Git branch tree scan (`refs/heads/*`) when `.git` is present at `ws.RootPath`.
  6. **`17214531` (`internal/cli/root.go` + `internal/cli/self_install.go`)**: Add `Aliases: []string{"install"}` to `newInitCommand` and implement handle-confined runtime executable self-installation into `<storage_root>/backlogit[.exe]` (with explicit `0o755` chmod on POSIX and `os.SameFile` self-overwrite skip on Windows) plus idempotent ignore-entry wiring in `.gitignore` and `.git/info/exclude` (when `.git/info` exists). Keep `cmd.CalledAs() == "init"` unconditionally initializing `.backlog` (and refusing `.backlogit`) so `u12InitBacklogitWorkspace` in `TestU12_*` is preserved, while `cmd.CalledAs() == "install"` accepts an existing `.backlog` or `.backlogit` root (or honors `BACKLOGIT_WORKSPACE_DIR`, defaulting to `.backlogit`).
* **Pros**: Resolves all 6 CAP-2 items cleanly, preserves every existing test and CI gate (`CLI Reference Drift`, `Docline`, `P-008`), and decomposes into width-isolated 2-hour tasks.
* **Cons**: Requires careful dependency ordering (e.g., `atomicfile.ReplaceTempFile` before `WriteToolchainManifest` and CLI self-install; `mcp.Server.Shutdown` before `cli.RunMCPDaemon`).
* **Effort**: Medium | **Fit**: Excellent.

## Trade-off Comparison

| Criterion | Option A | Option B (Selected) |
| --- | --- | --- |
| Satisfies all 6 CAP-2 stash contracts | No | Yes (100%) |
| Preserves `CloseAllSessions` session-only semantics (`0D073C21`) | No | Yes (`CloseAllSessions` vs `CloseCoordinator` / `Shutdown`) |
| Prevents cross-branch / multi-session ID collisions (`3083FC1A`) | No | Yes (`.locks/id_allocator.lock` + `.locks/id-sequences.json` + branch ref scan) |
| Reuses `internal/atomicfile` Windows `MoveFileEx` retry (`C5189EAF`) | No | Yes (`atomicfile.ReplaceTempFile`) |
| Preserves CI `CLI Reference Drift` & existing tests | Partial | Yes |

## Decision

Adopt **Option B** and promote directly to the CAP-2 implementation plan (`docs/plans/2026-11-15-cap2-sandbox-multisession-runtime-hardening-plan.md`) and covering feature `215-F` / shipment `216-S`.

## Rejected Alternatives

* **Option A** is rejected because it leaves `0D073C21` and `3083FC1A` unfixed and duplicates Windows retry logic instead of reusing `internal/atomicfile`.
* **Option C (Installing runtime binary only in `./bin/backlogit.exe` during `init`)** is rejected because `17214531` explicitly mandates installing in the `.backlogit` directory root and `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` requires `./bin/backlogit.exe` to remain absent after `backlogit init` unless explicitly provisioned.

## Unresolved Questions

* None. All six items have verified root causes and exact file/symbol boundaries.

## Risks and Mitigations

* **Risk**: `backlogit init` creating files that break existing `TestInitCommand_CreatesBacklogDirectory` or `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment`.
  * **Mitigation**: `backlogit init` (`cmd.CalledAs() == "init"`) unconditionally initializes `.backlog` (never creating `.backlogit` even when `BACKLOGIT_WORKSPACE_DIR=.backlogit` is set in `TestU12_*`, where `u12InitBacklogitWorkspace` runs `init` and then `os.Rename(".backlog", ".backlogit")`), closes all handles before returning, installs the runtime executable inside `.backlog/backlogit[.exe]`, and leaves `./bin/backlogit.exe` untouched; `backlogit install` (`cmd.CalledAs() == "install"`) accepts an existing `.backlogit` or `.backlog` root (or honors `BACKLOGIT_WORKSPACE_DIR`, defaulting to `.backlogit` when neither exists) without failing the legacy-root guard.
* **Risk**: Sequence reservation advancing on a vetoed or rolled-back `CreateArtifact` / `ReconstituteShipments`.
  * **Mitigation**: Commit the reserved ID to `<storage_root>/.locks/id-sequences.json` only after `MutationEnvelope` succeeds, and provide a rollback helper so compensated/rolled-back creates remove their unapplied reservation.
