---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CAP-2 (215-F / 216-S): self-installing workspace runtime executable, cross-branch ID sequence counter reservation, server-owned MCP coordinator drain API, executable-aware static toolchain validation, atomicfile manifest replacement, and U1k TEMP Git ceiling repair.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-15-cap2-sandbox-multisession-runtime-hardening-plan.md
title: 'CAP-2 Plan: Self-Contained Sandbox and Multi-Session Runtime Hardening'
---

## Problem Frame

Workspaces operating in isolated sandboxes (GitHub Copilot CLI sandboxes, Docker containers, GitHub workspaces) and multi-session development workflows require six targeted runtime, toolchain, ID-allocation, and Git-boundary hardenings resolved in [the CAP-2 deliberation](../decisions/2026-11-15-cap2-sandbox-multisession-runtime-hardening-deliberation.md) (`090-DL` through `095-DL`):

1. **`C5189EAF` (`094-DL`)**: `WriteToolchainManifest` (`internal/toolchain/launcher.go:875`) currently replaces `.backlogit/toolchain.yaml` via raw `os.Rename(tmpAbs, dstAbs)`, lacking `internal/atomicfile`'s Windows `MoveFileExW` sharing-violation retry semantics, permanent-denial detection, `MOVEFILE_WRITE_THROUGH` durability, and `ErrWriteNotApplied` error classification.
2. **`D1455316` (`093-DL`)**: Static workspace-toolchain validation (`internal/toolchain/verify.go`, `internal/toolchain/launcher.go`) identifies `backlogit` only when `strings.EqualFold(strings.TrimSpace(tool.ID), "backlogit")`, allowing an aliased inventory or `.mcp.json` entry pointing to `backlogit.exe` to omit the mandatory workspace-root `--cwd` binding.
3. **`0D073C21` (`092-DL`)**: `(*Server).CloseAllSessions()` (`internal/mcp/session_mux.go:549-582`) currently closes `s.coordinator`, violating session-only closure semantics for in-process `InvokeTool` callers, while `RunMCPDaemon` (`internal/cli/mcp_daemon.go:108,167`) needs a dedicated server-owned full-shutdown/coordinator-drain API on `*mcp.Server`.
4. **`1CED4AD6` (`095-DL`)**: `withTempGitCeiling` (`internal/core/gate/baseref.go:154-184`) synthesizes `GIT_CEILING_DIRECTORIES=os.TempDir()` whenever `g.Dir` is nested under `os.TempDir()`. When a valid Git repository is rooted directly at `os.TempDir()` (`TMPDIR=/work/repo`, `/work/repo/.git`, workspace `/work/repo/subproject`), Git refuses to inspect `/work/repo/.git`.
5. **`3083FC1A` (`091-DL`)**: `CreateArtifact` and `AdoptItem` (`internal/core/artifacts.go`, `internal/core/id_history.go`) allocate sequential `NNN-F` and `NNN.MMM-T` IDs solely from the current working tree and SQLite cache without cross-process allocation locking or cross-branch high-water-mark reservation, producing duplicate IDs when switching branches or running concurrent sessions.
6. **`17214531` (`090-DL`)**: `backlogit init` (and `backlogit install`) does not self-install the running `backlogit` executable into the workspace storage directory root (`.backlog/` or `.backlogit/`) or wire an ignore rule in `.gitignore` / `.git/info/exclude`.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Reuse `internal/atomicfile` Windows `MoveFileEx` retry and durable replacement in `WriteToolchainManifest` | `C5189EAF` / `094-DL` | Unit U1 | `TestU1_WriteToolchainManifestUsesAtomicFileReplacementAndRetrySemantics` |
| R2: Identify `backlogit` by executable identity as well as inventory ID and require `--cwd` binding on aliased entries | `D1455316` / `093-DL` | Unit U2 | `TestU2_AliasedBacklogitExecutableRequiresWorkspaceLocalPinAndCwdBinding` |
| R3: Preserve `CloseAllSessions()` session-only semantics and expose `CloseCoordinator()` / `Shutdown()` on `*mcp.Server` | `0D073C21` / `092-DL` | Unit U3a | `TestU3a_CloseAllSessionsPreservesCoordinatorAndCloseCoordinatorDrains` |
| R4: Wire `RunMCPDaemon` graceful stop and teardown to drain both sessions and coordinator via `Shutdown()` / `CloseCoordinator()` | `0D073C21` / `092-DL` | Unit U3b | `TestU3b_RunMCPDaemonShutdownDrainsCoordinatorAndSessions` |
| R5: Allow `ExecGitRunner.Verify` to discover a valid repository rooted at `os.TempDir()` from nested workspaces while preserving ancestor isolation | `1CED4AD6` / `095-DL` | Unit U4 | `TestU4_ExecGitRunnerVerifyAllowsRepoRootedAtTempDirAndIsolatesOuterCheckout` |
| R6: Prevent cross-branch and multi-session `NNN-F` / `NNN.MMM-T` ID collisions via `.locks/id_allocator.lock`, `.locks/id-sequences.json`, and local branch tree scan | `3083FC1A` / `091-DL` | Unit U5 | `TestU5_CrossBranchAndMultiSessionArtifactIDSequenceReservation` |
| R7: Self-install runtime executable into workspace storage root during `backlogit init`/`install` and add ignore entries to `.gitignore` / `.git/info/exclude` | `17214531` / `090-DL` | Unit U6 | `TestU6_InitAndInstallSelfInstallRuntimeExecutableAndIgnoreEntries` |

## Implementation Units

### Unit U1: Harden `WriteToolchainManifest` replacement with `internal/atomicfile` (`C5189EAF`)

* **Domain**: `code` (width-isolated to `internal/atomicfile` and `internal/toolchain`)
* **Size / Complexity**: `S` / `low` (3 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit)
* **Affected files (3)**:
  * `internal/atomicfile/atomicfile.go`
  * `internal/toolchain/launcher.go`
  * `internal/toolchain/launcher_test.go`
* **Changes needed**:
  1. In `internal/atomicfile/atomicfile.go`, export `ReplaceTempFile(tmpName, dstPath string, durable bool) error`, which delegates to platform `atomicReplace(tmpName, dstPath, durable)` (`MoveFileExW` with `MOVEFILE_REPLACE_EXISTING` and `MOVEFILE_WRITE_THROUGH` when `durable` is true, 6-attempt transient `ERROR_SHARING_VIOLATION` / `ERROR_ACCESS_DENIED` exponential backoff, permanent-denial check on `FILE_ATTRIBUTE_READONLY | FILE_ATTRIBUTE_DIRECTORY`, and `blerrors.ErrWriteNotApplied` wrapping) and, when `durable` is true, runs `SyncParentDir(dstPath)` on POSIX (returning `ErrWriteIndeterminate` if post-rename directory fsync fails).
  2. In `internal/toolchain/launcher.go` (`WriteToolchainManifest`, lines 830-885), retain `*os.Root`-confined temp file creation (`storageHandle.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)` using `toolchainManifestTempSeq`), `os.SameFile(openedStat, lstatAfterOpen)`, `file.Sync()`, and `file.Close()`. Immediately before final replacement, re-verify `storageHandle.Lstat("toolchain.yaml")` + `core.IsSymlinkOrReparsePoint` so a destination swapped for a symlink, reparse point, or directory during temp write is rejected with `corerrors.ErrValidation`. Replace raw `os.Rename(tmpAbs, dstAbs)` at line 875 with an injectable package seam `var replaceToolchainManifestFile = func(tmpAbs, dstAbs string) error { return atomicfile.ReplaceTempFile(tmpAbs, dstAbs, true) }`, and re-verify `storageHandle.Lstat("toolchain.yaml")` + `core.IsSymlinkOrReparsePoint` after replacement.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/toolchain/launcher_test.go`)**: Add `TestU1_WriteToolchainManifestUsesAtomicFileReplacementAndRetrySemantics` (using only existing `toolchain.WriteToolchainManifest` and `go/parser` so the RED harness compiles cleanly before `atomicfile.go` or `launcher.go` is edited) which: (a) verifies that when `toolchain.yaml` replacement fails (tested via a read-only `0o400` destination file on Windows and a read-only parent storage directory / seam on POSIX), `WriteToolchainManifest` returns an error wrapping `corerrors.ErrWriteNotApplied` while leaving the existing `toolchain.yaml` bytes intact (failing deterministically today because raw `os.Rename` does not wrap `corerrors.ErrWriteNotApplied`); (b) parses `launcher.go` via `go/parser` and asserts `WriteToolchainManifest` has zero raw `os.Rename` calls and delegates replacement through `replaceToolchainManifestFile` / `atomicfile.ReplaceTempFile`; and (c) verifies normal `WriteToolchainManifest` atomic creation and overwrite produce a regular `toolchain.yaml` file.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU1_WriteToolchainManifest|TestU11|TestU12)' ./internal/toolchain ./internal/atomicfile`

### Unit U2: Extend static workspace-toolchain validation for aliased `backlogit` executables (`D1455316`)

* **Domain**: `code` (width-isolated to `internal/toolchain`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, <= 4 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U1 (shares `internal/toolchain/launcher.go`)
* **Affected files (3)**:
  * `internal/toolchain/verify.go`
  * `internal/toolchain/launcher.go`
  * `internal/toolchain/verify_test.go`
* **Changes needed**:
  1. In `internal/toolchain/verify.go`, define `isBacklogitExecutable(abs, execPath string) bool` (returning `true` when `execPath` resolves relative to `abs` to `bin/backlogit.exe` or `bin/backlogit`, or when the lowercase base name of `execPath` is `"backlogit"`, `"backlogit.exe"`, or starts with `"backlogit."`, `"backlogit-"`, or `"backlogit_"`) and `isBacklogitTool(abs string, tool ToolEntry) bool` (returning `strings.EqualFold(strings.TrimSpace(tool.ID), "backlogit") || isBacklogitExecutable(abs, tool.ExecutablePath)`).
  2. Update `validBacklogitTool` (`verify.go:324`), `verifyWorkspaceToolchain` (`verify.go:373`), `verifyLocalTool` (`verify.go:477`), and `validateMCPConfigPinning` (`verify.go:692,765-808`) to use `isBacklogitTool` and `isBacklogitExecutable` so any aliased tool or `.mcp.json` server entry pointing to a `backlogit` executable must use `WorkspaceLocalPinned` `./bin/backlogit.exe` AND declare the workspace-root `--cwd` binding (`actualBindings["--cwd"] != ""` and `samePath(actualBindings["--cwd"], abs)`).
  3. Update all `backlogit` identity checks in `internal/toolchain/launcher.go` — lines `450` (`launcherProfile.invoke`), `611`, `634`, `669` (`validateManifestForProvisioning`), `935`, and **`951`** (`if cwdVal, hasCWD := bindings["--cwd"]; hasCWD && !isBacklogitTool(abs, tool)` in `RenderPinnedMCPConfig`) — so aliased `backlogit` entries require `--cwd` during manifest validation and receive `Args: []string{"mcp", "--cwd", abs}` exactly once (never duplicating `--cwd`).
  4. In `internal/toolchain/verify_test.go` (`TestU9_ReviewFixMCPArgumentsAndBindings`, line `418`/`439`), update the default generic `"launcher"` test binary from `"./bin/backlogit.exe"` to `"./bin/launcher.exe"` (writing `"bin/launcher.exe"` in the test setup) so `TestU9`'s generic non-`backlogit` subcases (`entrypoint-present`, `script-flag-matches`, `no-entry-subcommands`, `casefold-env-binding`) remain non-`backlogit` tools while `TestU2` tests aliased `backlogit.exe` entries.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/toolchain/verify_test.go`)**: Add `TestU2_AliasedBacklogitExecutableRequiresWorkspaceLocalPinAndCwdBinding` testing: (a) a manifest tool with `ID: "backlog-alias"` and `ExecutablePath: "./bin/backlogit.exe"` omitting `--cwd` in `WorkspaceBindingArgs` is flagged with `local_tool_invalid` (`Violations > 0`) by `VerifyWorkspaceToolchain` and rejected with `ErrValidation` by `WriteToolchainManifest`; (b) a `.mcp.json` server named `"backlog-alias"` invoking `"./bin/backlogit.exe"` without `"--cwd", <root>` in `args` is flagged as a violation by `ValidateMCPConfigPinning`; and (c) an aliased `backlogit` entry with valid `./bin/backlogit.exe` and `--cwd` binding passes verification and renders `["mcp", "--cwd", <abs>]` (without duplicate `--cwd` flags) in `RenderPinnedMCPConfig`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU2_AliasedBacklogit|TestU9_|TestU11)' ./internal/toolchain`

### Unit U3a: Expose server-owned coordinator drain API on `*mcp.Server` and preserve `CloseAllSessions` session-only semantics (`0D073C21` Part 1)

* **Domain**: `code` (width-isolated to `internal/mcp`)
* **Size / Complexity**: `S` / `low` (2 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Affected files (2)**:
  * `internal/mcp/session_mux.go`
  * `internal/mcp/session_mux_test.go`
* **Changes needed**:
  1. In `internal/mcp/session_mux.go`, remove `if s.coordinator != nil { result = errors.Join(result, s.coordinator.Close()) }` from `(*Server).CloseAllSessions()` so `CloseAllSessions()` stops session admission, cancels active sessions, and joins their drains without closing `s.coordinator`.
  2. Add `func (s *Server) CloseCoordinator() error` (which calls `s.coordinator.Close()` when `s.coordinator != nil`, idempotently) and `func (s *Server) Shutdown() error` (which calls `errors.Join(s.CloseAllSessions(), s.CloseCoordinator())`) to `*mcp.Server` in `internal/mcp/session_mux.go`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/mcp/session_mux_test.go`)**: Add `TestU3a_CloseAllSessionsPreservesCoordinatorAndCloseCoordinatorDrains` (using compile-safe interface assertion `any(s).(interface{ CloseCoordinator() error; Shutdown() error })` so the harness compiles before `session_mux.go` is edited) verifying: (a) after `s.CloseAllSessions()`, in-process `s.InvokeTool(ctx, "backlogit_get_version", ...)` still succeeds (`isError == false`) because `s.coordinator` remains open; (b) `s.CloseCoordinator()` (and `s.Shutdown()`) is exposed on `*Server`, is idempotent, and closes the server-owned `OperationCoordinator` so subsequent `s.InvokeTool` calls return `server_busy` (`mutation_queue_full`).
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3a_|TestU3_|TestU1_|TestU2_)' ./internal/mcp`

### Unit U3b: Wire `RunMCPDaemon` graceful stop and teardown to `(*mcp.Server).Shutdown()` (`0D073C21` Part 2)

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `S` / `low` (2 files, <= 2 functions, <= 2 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U3a
* **Affected files (2)**:
  * `internal/cli/mcp_daemon.go`
  * `internal/cli/mcp_daemon_multisession_test.go`
* **Changes needed**:
  1. In `internal/cli/mcp_daemon.go` (`RunMCPDaemon`), call `s.CloseAllSessions()` before `workers.Wait()` (so session admission stops and active session streams cancel and unblock `workers.Wait()`), then call `s.Shutdown()` (or `s.CloseCoordinator()`) after `workers.Wait()` and in the `defer` at line 108 before `s.Workspace.Close()`, ensuring all `runtime.serve` worker goroutines have exited before the server-owned `OperationCoordinator` and workspace close.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/mcp_daemon_multisession_test.go`)**: Add `TestU3b_RunMCPDaemonShutdownDrainsCoordinatorAndSessions` which parses `internal/cli/mcp_daemon.go` via `go/parser` to assert `RunMCPDaemon` invokes `s.Shutdown` (or `s.CloseCoordinator`) during daemon shutdown and verifies live multi-session daemon graceful stop drains both active sessions and the coordinator cleanly.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3b_|TestU5_)' ./internal/cli`

### Unit U4: Repair `ExecGitRunner.Verify` ambient `TEMP`/`TMPDIR` Git discovery ceiling for repositories rooted at `TEMP` (`1CED4AD6`)

* **Domain**: `code` (width-isolated to `internal/core/gate`)
* **Size / Complexity**: `S` / `low` (2 files, <= 2 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Affected files (2)**:
  * `internal/core/gate/baseref.go`
  * `internal/core/gate/probe_test.go`
* **Changes needed**:
  1. In `internal/core/gate/baseref.go` (`withTempGitCeiling`, lines 154-184), resolve the effective temp directory from `TMPDIR`/`TEMP`/`TMP` in `env` when present (falling back to `os.TempDir()`), and check whether `tempDir` / `evalTemp` itself contains a valid Git repository marker (`os.Lstat(filepath.Join(evalTemp, ".git", "HEAD")) == nil`, or `.git` is a regular gitfile at `filepath.Join(evalTemp, ".git")`).
  2. When `tempDir` / `evalTemp` is itself a Git repository root, shift both `tempDir` -> `filepath.Dir(tempDir)` and `evalTemp` -> `filepath.Dir(evalTemp)` (when `filepath.Dir(p) != p`) when building `GIT_CEILING_DIRECTORIES` so Git is permitted to inspect `tempDir/.git` from nested workspaces (`/work/repo/subproject`) while still blocking upward traversal into any ancestor checkout above `tempDir`. When `tempDir` / `evalTemp` has no valid `.git` marker, retain `tempDir` / `evalTemp` as the ceiling so temporary non-repositories under `TEMP` cannot discover an outer checkout at `filepath.Dir(tempDir)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/gate/probe_test.go`)**: Add `TestU4_ExecGitRunnerVerifyAllowsRepoRootedAtTempDirAndIsolatesOuterCheckout` testing: (a) when an outer repo exists at `outer`, `tempRepo := filepath.Join(outer, "temp-repo")` is initialized as a valid committed Git repository with `TEMP`/`TMP`/`TMPDIR` set to `tempRepo`, and `subdir := filepath.Join(tempRepo, "subproject")` is a nested workspace directory without its own `.git`, `(ExecGitRunner{Dir: subdir, Env: os.Environ()}).Verify(ctx, "HEAD")` returns `true, nil` (discovering `tempRepo/.git`); (b) when `tempRepo/.git` is removed (making `tempRepo` a non-repository inside `outer`), `Verify(ctx, "HEAD")` from `subdir` returns `false, nil` (never leaking `outer/.git`); and (c) explicit caller `GIT_CEILING_DIRECTORIES` remains authoritative.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU4_ExecGitRunnerVerify|TestU1k_)' ./internal/core/gate`

### Unit U5: Cross-branch and multi-session artifact ID sequence counter reservation and isolation (`3083FC1A`)

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 5 functions, <= 4 test scenarios)
* **Execution posture**: `test-first`
* **Affected files (3)**:
  * `internal/core/id_history.go`
  * `internal/core/artifacts.go`
  * `internal/core/id_history_test.go`
* **Changes needed**:
  1. In `internal/core/id_history.go`, implement a workspace-scoped sequence reservation store backed by `<storage_root>/.locks/id-sequences.json` (validating `<storage_root>/.locks` containment via `EvalSymlinks` + `pathContained` + `core.IsSymlinkOrReparsePoint` before creating `.locks/` or accessing files, and protecting mutations with an in-process mutex plus cross-process advisory file lock `<storage_root>/.locks/id_allocator.lock` via `lockTaskFileWithHeartbeat` with a dedicated `idAllocatorLockBoundedWait = 30 * time.Second` budget). Keep `historicalIDsFromDisk(ws)` and `allocateTypedHierarchicalID` strictly read-only (zero-write when `.locks/id-sequences.json` is absent so `ReconstituteShipments` `DryRun: true` writes nothing), and include all reserved IDs from `<storage_root>/.locks/id-sequences.json` (plus a fingerprint-cached scan of `.backlog/` and `.backlogit/` artifact filenames across local `refs/heads/*` branches when `.git` exists at `ws.RootPath` with multiple branches, executed with `ws.boundedHelperTimeout()`, `gate.MinimalEnv()`, `--no-replace-objects`, and `--no-optional-locks`) inside `historicalIDsFromDisk(ws)`.
  2. In `internal/core/artifacts.go` (`CreateArtifact`), place `id_allocator.lock` at the innermost leaf of the canonical lock hierarchy (`stash -> lifecycle-global -> A -> C -> B -> id_allocator.lock`): acquire `id_allocator.lock` strictly *after* `validateArtifactParent` and `ws.HookRunner.FirePre(ctx, hooks.HookCreateArtifact, hookCtx)`, hold it across ID computation (`allocateTypedHierarchicalID` / `NextID`) and `MutationEnvelope`, commit the newly allocated `artifactID` to `<storage_root>/.locks/id-sequences.json` via `atomicfile.WriteFileAtomic` (with deterministic sorted JSON keys) only *after* `MutationEnvelope` succeeds (skipping reservation commit when `ctx.Value(shipmentOperationContextKey{}) != nil` so a rolled-back `ReconstituteShipments` never leaves unapplied created IDs in `id-sequences.json`), and release `id_allocator.lock` *before* `ws.HookRunner.FirePost(ctx, hooks.HookCreateArtifact, hookCtx)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/id_history_test.go`)**: Add `TestU5_CrossBranchAndMultiSessionArtifactIDSequenceReservation` testing: (a) Session 1 on `branch-a` creates `001-F` and `001.001-T`; simulating a `git checkout branch-b` (removing `001-F.md` and `001.001-T.md` from `queue/` and rehydrating `ws.DB` so neither the working tree nor SQLite contains `001-F`), Session 2 on `branch-b` creates a feature and task and receives `002-F` and `002.001-T` (never colliding with `001-F` / `001.001-T`), and creating another child under `001-F` after restoring `001-F.md` allocates `001.002-T`; (b) 10 concurrent goroutines calling `CreateArtifact` for features and child tasks on the same workspace root allocate 10 unique sequential IDs with zero `ErrIDCollision` or `ErrGateInProgress` failures; and (c) neither a vetoed `CreateArtifact` (via pre-create hook error) nor a failed `MutationEnvelope` (`db-upsert` failure) burns the unapplied sequence number.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU5_CrossBranch|TestNextTypedHierarchicalID|TestNextID)' ./internal/core`

### Unit U6: Self-installing runtime executable in workspace storage root and ignore-file wiring during `backlogit init`/`install` (`17214531`)

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, <= 4 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U3b (serializes `internal/cli` modifications)
* **Affected files (3)**:
  * `internal/cli/root.go`
  * `internal/cli/self_install.go`
  * `internal/cli/init_workspace_dir_test.go`
* **Changes needed**:
  1. In `internal/cli/root.go` (`newInitCommand`), add `Aliases: []string{"install"}` (keeping `Use`, `Short`, `Long`, `Example`, and flags unchanged so `cmd/gen-docs` output in `docs/cli-reference/` has zero drift, and checking all `Close`/`fmt.Fprint*` errors per `TestU175_051_LintFix`). Keep `cmd.CalledAs() != "install"` (`backlogit init`) target-directory resolution **100% unchanged** — `init` always refuses an existing `.backlogit` directory and always initializes `<root>/.backlog` (even if `BACKLOGIT_WORKSPACE_DIR=.backlogit` is set in the environment, preserving `u12InitBacklogitWorkspace` in `TestU12_*`). When `cmd.CalledAs() == "install"`, resolve the target storage root to an existing `.backlog` or `.backlogit` directory (or honor `BACKLOGIT_WORKSPACE_DIR`, defaulting to `.backlogit` when neither exists) without failing the legacy-root check.
  2. Create `internal/cli/self_install.go` implementing `installWorkspaceRuntimeExecutable(root, storageDir string) (string, error)` and `ensureWorkspaceRuntimeIgnored(root, storageDir, binName string) error` (closing all `*os.Root` and `*os.File` handles before returning so Windows directory renames immediately after `init` never hit `ERROR_ACCESS_DENIED`):
     * Resolve the running binary via `os.Executable()` (with an injectable seam), verify `os.Lstat` + `core.IsSymlinkOrReparsePoint` and `os.SameFile` on the opened source handle; if `<storageDir>/backlogit[.exe]` already exists and `os.SameFile(srcInfo, dstInfo)` is true, return early as a no-op (avoiding Windows self-overwrite `ERROR_SHARING_VIOLATION`); otherwise write `<storageDir>/backlogit.exe` (Windows) or `<storageDir>/backlogit` (POSIX) using `*os.Root` handle confinement and `atomicfile.WriteFileAtomicWithOptions(..., atomicfile.Options{DurableWrites: false})` followed by `os.Chmod(dstPath, 0o755)` so POSIX execute permissions (`0o755`) are set.
     * Idempotently ensure ignore entries (`/.backlog/backlogit`, `/.backlog/backlogit.exe`, `/.backlogit/backlogit`, `/.backlogit/backlogit.exe`) are present in `<root>/.gitignore` AND, when `<root>/.git/info` exists as a non-reparse directory, in `<root>/.git/info/exclude` (using `*os.Root` no-follow checks and `atomicfile.WriteFileAtomic`), while leaving `<root>/bin/backlogit.exe` untouched so `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` remains green.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/init_workspace_dir_test.go`)**: Add `TestU6_InitAndInstallSelfInstallRuntimeExecutableAndIgnoreEntries` (in `package cli_test`, calling only `cli.NewRootCommand()` so the RED harness compiles cleanly) testing: (a) `backlogit init --cwd <root>` initializes `.backlog`, installs the runtime binary at `.backlog/backlogit[.exe]` with executable permissions (`mode & 0o111 != 0` on POSIX), and writes idempotent ignore rules to `.gitignore` (and `.git/info/exclude` when `.git/info` exists); (b) `backlogit install --cwd <root>` works both on a fresh directory (installing `.backlogit` and `.backlogit/backlogit[.exe]`) and on an existing `.backlogit` workspace without failing the legacy-root check; and (c) running `init` or `install` twice does not duplicate ignore lines in `.gitignore` or `.git/info/exclude` and rejects symlinked `.gitignore` / `.git/info/exclude` targets.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU6_InitAndInstall|TestInitCommand_|TestRootCommand_|TestU175_051_LintFix)' ./internal/cli`

## Dependency Graph

```text
Unit U1 (C5189EAF: internal/toolchain/launcher.go atomicfile replacement)
  └──> Unit U2 (D1455316: internal/toolchain verify.go + launcher.go aliased backlogit --cwd validation)

Unit U3a (0D073C21 Part 1: internal/mcp/session_mux.go CloseAllSessions vs CloseCoordinator/Shutdown)
  └──> Unit U3b (0D073C21 Part 2: internal/cli/mcp_daemon.go RunMCPDaemon Shutdown drain)
         └──> Unit U6 (17214531: internal/cli/root.go + self_install.go init/install binary & ignore wiring)

Unit U4 (1CED4AD6: internal/core/gate/baseref.go withTempGitCeiling repo-at-TEMP fix) [independent]

Unit U5 (3083FC1A: internal/core/id_history.go + artifacts.go cross-branch ID reservation) [independent]
```

* **Wave 1 (Parallel DAG roots)**: `Unit U1`, `Unit U3a`, `Unit U4`, `Unit U5` (4 completely disjoint packages: `internal/toolchain`, `internal/mcp`, `internal/core/gate`, `internal/core`).
* **Wave 2**: `Unit U2` (depends on `U1` in `internal/toolchain`), `Unit U3b` (depends on `U3a`, in `internal/cli`).
* **Wave 3**: `Unit U6` (depends on `U3b`, in `internal/cli`).

## Decisions and Rationale

1. **Reuse `atomicfile.WriteFileAtomicWithOptions` in `WriteToolchainManifest` (`C5189EAF`)**: Avoids adding new exported symbols to `internal/atomicfile` while gaining its full Windows `MoveFileExW` sharing-violation retry loop, permanent-denial check, `MOVEFILE_WRITE_THROUGH` durability, and `ErrWriteNotApplied` error chain.
2. **Add `Aliases: []string{"install"}` on `newInitCommand` (`17214531`)**: Cobra's `doc.GenMarkdownTreeCustom` (`cmd/gen-docs`) does not emit command aliases, so adding `install` as an alias on `init` keeps `docs/cli-reference/` 100% drift-free in CI while letting `cmd.CalledAs() == "install"` support existing `.backlogit` workspaces.
3. **Store sequence counter reservations in `<storage_root>/.locks/id-sequences.json` (`3083FC1A`)**: Because `.locks/` is git-ignored and untracked, `<storage_root>/.locks/id-sequences.json` automatically survives `git checkout` branch switches across sessions in the same workspace without violating CLI Workspace Containment (Principle IV). Combining it with `<storage_root>/.locks/id_allocator.lock` and a fingerprint-cached local `refs/heads/*` tree scan covers concurrent sessions, branch switches, and multi-branch repositories.
4. **Compile-safe interface assertion in `TestU3a` (`0D073C21`)**: Using `any(s).(interface{ CloseCoordinator() error; Shutdown() error })` in `session_mux_test.go` allows `harness-architect` to land a compiling, assertion-RED test in `internal/mcp/session_mux_test.go` without needing a separate production stub task.

## Risks and Caveats

* **Risk**: `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`tests/integration/multi_session_workspace_containment_test.go:497`) calls `backlogit init` and then asserts `PrewarmMCPDaemon` fails with `ErrDaemonBinaryMissing` because `./bin/backlogit.exe` does not exist.
  * **Mitigation**: Unit U6 installs the self-installed binary into `<storage_root>/backlogit[.exe]` (`.backlog/backlogit[.exe]` or `.backlogit/backlogit[.exe]`) as specified by `17214531`, never into `./bin/backlogit.exe`, so `TestU12_*` remains 100% green.
* **Risk**: Symlink or reparse-point swap on `.gitignore`, `.git/info/exclude`, or `toolchain.yaml` during write.
  * **Mitigation**: Open the workspace root via `os.OpenRoot(root)` and verify `Lstat` + `core.IsSymlinkOrReparsePoint` on `.gitignore`, `.git`, `.git/info`, `.git/info/exclude`, and `<storage_root>/backlogit[.exe]` before writing via `atomicfile.WriteFileAtomicWithOptions`.

## Constitution Check

* **Safety-First Go (`NON-NEGOTIABLE`)**: `pass` — All changes are pure Go, wrap errors with `%w` (preserving `ErrValidation` and `ErrWriteNotApplied`), and introduce zero `unsafe` usage.
* **Test-First Development (`NON-NEGOTIABLE`)**: `pass` — Every unit (`U1` through `U6`) defines a concrete RED harness that compiles cleanly and fails on a behavioral/AST assertion before production code is modified.
* **Workspace Isolation and Security Boundaries (`NON-NEGOTIABLE`)**: `pass` — All file writes in `WriteToolchainManifest`, `self_install.go`, and `id_history.go` are confined within the validated workspace root via `*os.Root` and `core.IsSymlinkOrReparsePoint`, and `withTempGitCeiling` preserves ancestor-checkout isolation.
* **CLI Workspace Containment (`NON-NEGOTIABLE`)**: `pass` — All sequence-reservation and self-install writes occur strictly inside `<workspace_root>` (`<storage_root>/.locks/`, `<storage_root>/`, `.gitignore`, `.git/info/exclude`).
* **Destructive Command Approval (`NON-NEGOTIABLE`)**: `pass` — No destructive operations are introduced.
* **Merge Commit History Preservation (`NON-NEGOTIABLE`)**: `pass` — Shipment `216-S` will merge via a merge commit (`--merge`) under P-009/P-014.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Adds `(*mcp.Server).CloseCoordinator()` / `Shutdown()` in `internal/mcp`, `install` command alias in `internal/cli`, and extends `internal/toolchain` aliased `backlogit` validation (`0D073C21`, `17214531`, `D1455316`).
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Modifies static workspace-toolchain containment validation (`D1455316`), handle-confined manifest replacement (`C5189EAF`), self-installed binary/ignore writes (`17214531`), and `ExecGitRunner.Verify` `GIT_CEILING_DIRECTORIES` synthesis (`1CED4AD6`).
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — `.locks/id-sequences.json` and `<storage_root>/backlogit[.exe]` are additive, git-ignored runtime state requiring no schema migration.
* **external integration, operator checkpoint, or external dependency**: `absent` — No new external dependencies (`go.mod` unchanged).
* **high runtime, rollout, or rollback risk**: `present` — Touches MCP daemon shutdown lifecycle (`RunMCPDaemon`) and `CreateArtifact` ID allocation critical path.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * `backlogit init` and `backlogit install` CLI commands (`internal/cli/root.go`, `internal/cli/self_install.go`)
  * `backlogit mcp daemon` / `backlogit mcp stop` graceful shutdown (`internal/cli/mcp_daemon.go`, `internal/mcp/session_mux.go`)
  * `backlogit mcp status --toolchain --strict` static validator (`internal/toolchain/verify.go`, `internal/toolchain/launcher.go`)
  * `backlogit add` / `backlogit_create_item` artifact ID sequence allocator (`internal/core/id_history.go`, `internal/core/artifacts.go`)
* **Runtime verification probes (for `_Ship` Step 6)**:
  1. In an isolated `--cwd <tempDir>` git repository, run `backlogit install --cwd <tempDir>` and verify `.backlogit/backlogit[.exe]` is installed, `.gitignore` and `.git/info/exclude` contain the ignore entries, and `git status --porcelain` does not list the installed binary as untracked.
  2. Run `backlogit add --type feature --title "Branch A"` on `branch-a`, switch to `branch-b`, run `backlogit add --type feature --title "Branch B"`, and verify the two features receive distinct sequential IDs (`001-F` and `002-F`).
  3. Start and stop an isolated workspace MCP daemon (`PrewarmMCPDaemon` -> `StopMCPDaemon`) and verify clean coordinator + session drain (`active_sessions: 0`).
* **Operational closure expectations**:
  * **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
  * **Validation window**: Immediate post-merge CLI/test verification on `main` plus 7-day multi-session observation window.
  * **Healthy signals**: Zero `ErrIDCollision` across multi-branch sessions, clean `backlogit mcp stop` drain, zero `CLI Reference Drift` or `Docline` violations in CI.
  * **Rollback trigger & procedure**: Any regression in `backlogit init`, `CreateArtifact`, `mcp stop`, or `AuditRepositoryToolchain` triggers a reviewed `git revert -m 1 <merge_sha>` PR; `.locks/id-sequences.json` and `<storage_root>/backlogit[.exe]` are additive and require no database migration on rollback.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by contract, security-boundary, and runtime-lifecycle signals across `internal/toolchain`, `internal/mcp`, `internal/cli`, `internal/core/gate`, and `internal/core`.
* **Protected invariants**:
  * **I1 (Toolchain containment & symlink defense)**: `WriteToolchainManifest` and `installWorkspaceRuntimeExecutable` must verify directory and file identities via `*os.Root`, `Lstat`, `os.SameFile`, and `core.IsSymlinkOrReparsePoint` before and after `atomicfile.WriteFileAtomicWithOptions`, never following symlinks or reparse points outside the workspace root.
  * **I2 (Session vs. coordinator lifecycle separation)**: `(*mcp.Server).CloseAllSessions()` must drain sessions only and leave `s.coordinator` active for in-process `InvokeTool` callers; only `(*mcp.Server).CloseCoordinator()` / `(*mcp.Server).Shutdown()` (called by `RunMCPDaemon`) closes `s.coordinator`.
  * **I3 (Git discovery boundary)**: `withTempGitCeiling` must never override an explicit caller-supplied `GIT_CEILING_DIRECTORIES`, must block temporary non-repositories under `os.TempDir()` from discovering an outer checkout above `os.TempDir()`, and must shift the synthesized ceiling to `filepath.Dir(tempDir)` only when `tempDir` itself contains a `.git` entry.
  * **I4 (Transactional sequence reservation)**: `CreateArtifact` must commit an allocated ID to `<storage_root>/.locks/id-sequences.json` only after `MutationEnvelope` succeeds (rolling back the reservation if creation fails or is compensated), so failed creates never create gaps or dirty state.
  * **I5 (Legacy `init` vs `install` compatibility)**: `backlogit init` must preserve `TestInitCommand_CreatesBacklogDirectory`, `TestInitCommand_RefusesLegacyWorkspaceRoot`, and `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (never creating `./bin/backlogit.exe`).
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/go-mcp-server.instructions.md`
  * `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`
  * `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`
  * `docs/compound/test-failures/windows-retry-error-boundaries-and-handle-aware-observers-2026-11-09.md`
  * `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`
  * `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md`
  * `docs/compound/concurrency-issues/unregister-completed-request-session-cancellation-2026-11-09.md`
* **Risky actions classification (`strict-safety`)**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Replace `os.Rename` in `WriteToolchainManifest` with `atomicfile.WriteFileAtomicWithOptions` | `internal/toolchain/launcher.go` | `moderate` (security boundary) | Preserve `*os.Root` + `IsSymlinkOrReparsePoint` + `os.SameFile` pre/post checks | Atomic `MoveFileExW` replacement with 6-attempt sharing-violation retry and `ErrWriteNotApplied` classification |
| Require `--cwd` and `./bin/backlogit.exe` on aliased `backlogit` entries | `internal/toolchain/verify.go`, `launcher.go` | `moderate` (validation contract) | Scoped to entries whose ID is `backlogit` or whose executable path identifies `backlogit` | Closes static alias bypass (`D1455316`) while keeping `TestU9_*`/`TestU11_*`/`TestU12_*` green |
| Separate `CloseAllSessions` from `CloseCoordinator` / `Shutdown` | `internal/mcp/session_mux.go`, `internal/cli/mcp_daemon.go` | `moderate` (daemon lifecycle) | `RunMCPDaemon` calls `s.Shutdown()`; `CloseAllSessions()` remains session-only | In-process `InvokeTool` survives `CloseAllSessions()`; daemon `stop` drains both sessions and coordinator |
| Shift synthesized `GIT_CEILING_DIRECTORIES` to `filepath.Dir(tempDir)` when `tempDir/.git` exists | `internal/core/gate/baseref.go` | `moderate` (Git security boundary) | Only when `os.Lstat(filepath.Join(evalTemp, ".git")) == nil`; caller ceiling still wins | Nested workspaces under a repo rooted at `TMPDIR` resolve `HEAD` while non-repo temp dirs remain isolated |
| Serialize ID allocation under `.locks/id_allocator.lock` and `.locks/id-sequences.json` | `internal/core/id_history.go`, `artifacts.go` | `moderate` (concurrency & ID allocation) | Bounded lock wait (`defaultGateLockBoundedWait`); commit reservation only after `MutationEnvelope` succeeds | Zero cross-branch or multi-session `NNN-F` / `NNN.MMM-T` collisions |
| Self-install runtime binary into `<storage_root>/backlogit[.exe]` and update `.gitignore` / `.git/info/exclude` | `internal/cli/root.go`, `self_install.go` | `moderate` (filesystem writes during `init`/`install`) | Confined via `os.OpenRoot(root)` + `IsSymlinkOrReparsePoint` + `atomicfile`; never writes `./bin/backlogit.exe` | Sandbox-ready workspace binary with idempotent git-ignore protection |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode: multi-agent-dispatch` (or `single-agent-declared-degradation` if degraded), cover all required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) plus `Adversarial Review` and `rubber-duck` review, and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) before harvest.
* **Unresolved operator decisions**: None (`DARK_MODE_ACTIVE: true` pre-authorizes CAP-2 scope and staging into `215-F` / `216-S`).

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `decision: PASS`
* **Gate rationale**: Dispatched all 7 `plan-review` personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) plus independent `Adversarial Review` (`mode: report-only`) and `rubber-duck` (`Correctness Reviewer`) review. All P0/P1/P2/P3 findings identified during review (`CAP2-F1` through `CAP2-F6`, `CAP2-LR-001` through `CAP2-LR-003`) were incorporated directly into Units U1–U6 above before harvest:
  1. **`CAP2-F1` (P0 — `u12InitBacklogitWorkspace` compatibility under `BACKLOGIT_WORKSPACE_DIR=.backlogit`)**: Scoped `.backlogit` / `BACKLOGIT_WORKSPACE_DIR` resolution in Unit U6 strictly to `cmd.CalledAs() == "install"` while keeping `cmd.CalledAs() != "install"` (`backlogit init`) unconditionally initializing `<root>/.backlog` and closing all handles before returning so `u12InitBacklogitWorkspace`'s `os.Rename(".backlog", ".backlogit")` in `TestU12_*` succeeds on Windows.
  2. **`CAP2-F2` (P0 — `launcher.go:951` duplicate `--cwd` prevention & `TestU9_ReviewFixMCPArgumentsAndBindings` fixture binary)**: Updated Unit U2 to include `launcher.go:450,611,634,669,935,951` (`!isBacklogitTool(abs, tool)`), updated `TestU9_ReviewFixMCPArgumentsAndBindings` to use `"./bin/launcher.exe"` for its generic `"launcher"` cases, and enforced `actualBindings["--cwd"] != ""` in `validateMCPConfigPinning` for aliased `backlogit` executables.
  3. **`CAP2-F3` (P1 — `WriteToolchainManifest` `*os.Root` temp handle confinement & cross-platform `ErrWriteNotApplied` test)**: Preserved `storageHandle.OpenFile(tmpName, ...)` + `os.SameFile` in `WriteToolchainManifest`, exported `atomicfile.ReplaceTempFile(tmpName, dstPath, durable)` in `internal/atomicfile/atomicfile.go`, and made `TestU1`'s `ErrWriteNotApplied` verification cross-platform across Windows and POSIX.
  4. **`CAP2-F4` (P1 — POSIX `0o755` permissions and Windows self-overwrite `os.SameFile` guard)**: Added explicit `os.Chmod(dstPath, 0o755)` after `WriteFileAtomicWithOptions` and an `os.SameFile(srcInfo, dstInfo)` early-return guard in `installWorkspaceRuntimeExecutable`.
  5. **`CAP2-F5` / `CAP2-LR-001` (P1/P2 — `id_allocator.lock` 30s wait budget, leaf lock order, and zero-write `DryRun`)**: Set `idAllocatorLockBoundedWait = 30 * time.Second`, placed `id_allocator.lock` at the leaf of the lock hierarchy (`after FirePre`, `before FirePost`), kept `allocateTypedHierarchicalID` and `historicalIDsFromDisk` read-only for `DryRun: true`, and removed the extraneous `shipment_lifecycle.go` mention from Unit U5.
  6. **`CAP2-F6` (P2 — `withTempGitCeiling` `.git/HEAD` check and dual `tempDir`/`evalTemp` parent shift)**: Updated Unit U4 to inspect `TMPDIR`/`TEMP`/`TMP` in `env`, verify `.git/HEAD` (or regular gitfile) at `tempDir`/`evalTemp`, and shift both `tempDir` and `evalTemp` to `filepath.Dir(...)`.
* **Plan hardening verification**: `Requires plan hardening: yes` is satisfied by the complete `## Plan Hardening` section with `ProposedAction` / `ActionRisk` classifications and `## Runtime Verification and Closure`.

