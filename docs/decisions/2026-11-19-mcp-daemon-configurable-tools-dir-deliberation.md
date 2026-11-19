---
chunk_strategy: h1-h2-h3
decision_status: decided
depth: standard
description: 'Deliberation for stash 2CCDFDED (102-DL): honor .mcp.json command paths, support configurable tools_dir, default workspace tool directory to .bin with bin fallback in MCP daemon startup and toolchain verification, and bound cross-branch ID history git scans.'
doc_type: decision
linked_artifacts:
  - .backlogit/archive/102-DL.md
  - .backlogit/queue/217-F.md
  - .backlogit/queue/218-S.md
  - docs/plans/2026-11-19-mcp-daemon-configurable-tools-dir-plan.md
promoted_to: both
schema_version: "1.0"
source: docs/decisions/2026-11-19-mcp-daemon-configurable-tools-dir-deliberation.md
tags:
  - mcp-daemon
  - toolchain
  - tools-dir
  - dot-bin
  - id-history
title: 'Deliberation: Honor .mcp.json, Configurable tools_dir, and .bin Default in MCP Daemon Startup and Toolchain Verification (2CCDFDED)'
topic: 'Honor .mcp.json, configurable tools_dir, and .bin default in MCP daemon startup and toolchain verification (2CCDFDED / 102-DL)'
---

## Problem Frame

In released `backlogit` executables, workspace MCP daemon startup and static toolchain verification hard-code the `bin/` directory, breaking MCP startup and toolchain verification in GSD workspaces (which default to `.bin/`) and workspaces with custom `.mcp.json` or tool-directory configurations:

1. **`internal/cli/mcp_daemon.go` (`pinnedMCPDaemonBinary`, `PrewarmMCPDaemon`, `VerifyMCPDaemonAttachment`, `RunMCPSessionShim`)**:
   * `backlogit mcp` invokes `RunMCPSessionShim` -> `PrewarmMCPDaemon` -> `pinnedMCPDaemonBinary(workspace)`.
   * `pinnedMCPDaemonBinary(workspace)` hard-codes `filepath.Join(workspace, "bin", name)` and inspects only `root.Lstat("bin")` and `root.Lstat(filepath.Join("bin", name))`.
   * When a workspace places `backlogit[.exe]` in `.bin/backlogit[.exe]` (the GSD default, and the path configured in `.mcp.json` after PR #658) or configures another workspace-local path in `.mcp.json`, `toolchain.yaml`, or `config.yaml`, `pinnedMCPDaemonBinary` fails with `ErrDaemonBinaryMissing` (or resolves a stale `bin/backlogit[.exe]` that mismatches the running `.bin/backlogit[.exe]`, failing `VerifyMCPDaemonAttachment` with `ErrDaemonVersionMismatch`).
2. **`internal/toolchain/verify.go` & `internal/toolchain/launcher.go`**:
   * `isPinnedBacklogitRelPath(rel string)` (`internal/toolchain/verify.go:326`) and `isBacklogitExecutable(abs, execPath string)` (`internal/toolchain/verify.go:304`) hard-code `bin/backlogit.exe` and `bin/backlogit`, rejecting `.bin/backlogit[.exe]` and custom configured tool directories in `VerifyWorkspaceToolchain`, `ValidateMCPConfigPinning`, `WriteToolchainManifest`, and `RenderPinnedMCPConfig`.
   * `toolRemediation` (`internal/toolchain/launcher.go:93`), `launcherFinding` (`internal/toolchain/launcher.go:86`), and `inspectLauncher` (`internal/toolchain/launcher.go:228`) hard-code `bin/` prefix checks and remediation strings.
3. **`internal/core/id_history.go` (`scanLocalBranchArtifactIDsContext`)**:
   * Discovered during Stage deliberation on this repository: `scanLocalBranchArtifactIDsContext` (`internal/core/id_history.go:1148-1188`, introduced in `215.006-T`) runs `git for-each-ref refs/heads` across all local branches (324 in this repository) and spawns a separate `git ls-tree` subprocess per branch under a shared 5-second `ws.boundedHelperTimeout()`. When a repository has more than ~50 merged local branches on Windows, the sequential per-branch subprocess loop exceeds 5 seconds and aborts `CreateArtifact` (`backlogit add`, `backlogit deliberate`, `backlogit shipment create`, `backlogit stash harvest`).

## P-021 C5 and C6 Deferred-Expansion Triage and Reconciliation Audit

Companion stash entry `85442104` was evaluated alongside `2CCDFDED` under Stage's P-021 C5/C6 intake obligations:

| Stash ID | Marker / Kind | Originating Source Refs | (A) Unconditional Duplicate Scan Outcome | (B) Mandatory Late-Identifier Reconciliation Outcome | Session Disposition |
| --- | --- | --- | --- | --- | --- |
| `2CCDFDED` | `bug` / `high` (operator intake) | Operator report following PR `#658` | **CLEAN DUPLICATE SCAN** across all records in `.backlogit/stash.jsonl` (`0` duplicates; `368E21A2` standalone installation docs verified distinct) | N/A (direct operator bug intake) | **Selected (`102-DL`)** for immediate planning, harvest, and shipment assembly |
| `85442104` | `DEFERRED SCOPE EXPANSION` / `task` / `medium` | `PR=658`, `review_thread_id=PRRT_kwDORzozKM65RT8k` (`comment_id=4622801853`), `task_id=N/A`, `feature_id=N/A`, `shipment_id=N/A` | **CLEAN DUPLICATE SCAN** across all records in `.backlogit/stash.jsonl` (`0` duplicates) | **No late identifier found**: checked `docs/memory/2026-11-19/pr658-gsd-harness-migration-memory.md` and `docs/closure/2026-11-19-pr658-compound-refresh.md`; PR `#658` was a harness migration PR without a backlogit shipment/feature/task ID, so `N/A` stands as a truthful terminal record | **Deferred**: modifies PowerShell helper scripts (`tools/mcp/install.ps1` and `scripts/build-dev.ps1`); excluded to preserve pure Go width isolation and Step 5.5 scope guard focus on `2CCDFDED` |

## Research Findings

### Prior Institutional Learnings (`docs/compound/`)

* `docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md`: `PrewarmMCPDaemon` and `VerifyMCPDaemonAttachment` must derive expected version and executable identity from the verified workspace-pinned binary, never from ambient `PATH` or an unverified host binary.
* `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md` and `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`: All workspace-local binary and config inspections must use `os.OpenRoot(absWorkspace)` with explicit absolute base paths for `rejectLink` (`core.IsSymlinkOrReparsePoint`), segment-by-segment directory `Lstat` + `rejectLink`, and post-open `os.SameFile(opened, linked)` + `IsRegular()` verification.
* `docs/compound/2026-10-16-root-confined-access-and-graphql-reviewer-union.md` and `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md`: Any executable path read from `.mcp.json`, `toolchain.yaml`, or `config.yaml` must be validated as a canonical workspace-contained relative path inside a subdirectory before execution, rejecting bare command names, `..` traversal, volume prefixes, symlinks, and Windows junctions/reparse points fail-closed.

### Codebase Analysis

* `TestU6_MissingPinnedBinaryRefusalAndCLIWiring` (`internal/cli/mcp_session_shim_test.go:299`) and `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`tests/integration/multi_session_workspace_containment_test.go:369`) require `PrewarmMCPDaemon` and `RunMCPSessionShim` to fail closed with `corerrors.ErrDaemonBinaryMissing` when no workspace-pinned binary exists in `.mcp.json`, `toolchain.yaml`, `config.yaml`, `.bin/`, or `bin/` — even when `backlogit init` has self-installed `<storage_root>/backlogit[.exe]` (`.backlog/backlogit[.exe]` or `.backlogit/backlogit[.exe]`) and a real `backlogit` binary is on `PATH`.
* Existing test fixtures (`u6NewFixture`, `u9PinnedWorkspace`, `u11Fixture`, `u12FullManifest`) place inert or built binaries in `bin/backlogit[.exe]`. Supporting `.bin` as the default tool directory with `bin` as the fallback (and honoring `.mcp.json`, `toolchain.yaml`, and `config.yaml` `tools_dir` ahead of defaults) satisfies new `.bin` and custom-configured workspaces while keeping all existing `bin/` fixtures 100% green.

## Options Evaluated

### Option A: Hard-code `.bin/` in place of `bin/`

* **Description**: Replace `"bin"` with `".bin"` in `pinnedMCPDaemonBinary`, `isPinnedBacklogitRelPath`, and `toolRemediation`.
* **Pros**: Minimal code diff.
* **Cons**: Breaks all existing `bin/` workspaces and unit/integration test fixtures (`TestU6_*`, `TestU9_*`, `TestU11_*`, `TestU12_*`), does not allow configuring the tools directory, and ignores custom paths in `.mcp.json`.
* **Effort estimate**: Low.
* **Fit**: Poor — fails operator requirements 2 and 3 and breaks backward compatibility.

### Option B: Layered root-confined resolution (`.mcp.json` -> `toolchain.yaml` -> `config.yaml` `tools_dir` -> `.bin` default -> `bin` fallback) + configurable `tools_dir` + bounded unmerged branch ID scan (Recommended)

* **Description**:
  1. In `internal/cli/mcp_daemon.go` (`pinnedMCPDaemonBinary`), resolve the workspace-pinned `backlogit` executable under `os.OpenRoot(absWorkspace)` in strict priority order:
     * **`.mcp.json`** (`mcpServers.backlogit.command` or `servers.backlogit.command`, or any server entry whose command is a `backlogit` executable): if `.mcp.json` declares a `backlogit` command, validate and open that exact workspace-local path via `*os.Root` segment-by-segment `Lstat` + `rejectLink` + `Open` + `SameFile`, failing closed (`ErrValidation` / `ErrDaemonBinaryMissing`) without fallback if the command is bare, escaping, redirected, or missing on disk.
     * **`toolchain.yaml`** (`.backlog/toolchain.yaml` or `.backlogit/toolchain.yaml`): `backlogit` tool's `executable_path`, or `<tools_dir>/backlogit[.exe]` when `tools_dir` is set.
     * **`config.yaml`** (`.backlog/config.yaml` or `.backlogit/config.yaml`): `<tools_dir>/backlogit[.exe]` when `tools_dir` is set.
     * **Default `.bin/backlogit[.exe]`** (GSD default tool directory), falling back to **`bin/backlogit[.exe]`** (legacy tool directory) when `.bin/backlogit[.exe]` is absent (`os.ErrNotExist`).
  2. In `internal/toolchain` (`types.go`, `verify.go`, `launcher.go`), `internal/config/schema.go`, and `schemas/workspace-toolchain.schema.json`:
     * Add optional `ToolsDir string` (`tools_dir`) to `ToolchainManifest` and `WorkspaceConfig`.
     * Update `isPinnedBacklogitRelPath`, `isBacklogitExecutable`, `validBacklogitTool`, `validateManifestForProvisioning`, `toolRemediation`, `launcherFinding`, and `inspectLauncher` to accept `.bin/backlogit[.exe]`, `bin/backlogit[.exe]`, and configured workspace-local tool directories (`<tools_dir>/backlogit[.exe]`), and to emit remediation paths matching the manifest's configured tool directory (defaulting to `.bin`).
  3. In `internal/core/id_history.go` (`scanLocalBranchArtifactIDsContext`):
     * Query unmerged local branches (`git for-each-ref --no-merged=HEAD --format=%(refname:short) %(objectname) refs/heads`) plus `HEAD`, deduplicate commit OIDs before invoking `git ls-tree`, so repositories with hundreds of historical merged local branches complete ID history scanning in <200ms rather than timing out at 5s.
* **Pros**: Honors `.mcp.json` directly, defaults to `.bin` with `bin` fallback, supports configurable `tools_dir` in `toolchain.yaml` and `config.yaml`, preserves all `*os.Root` containment and `TestU6`/`TestU9`/`TestU11`/`TestU12` invariants, and eliminates the 5s `scanLocalBranchArtifactIDsContext` timeout.
* **Cons**: Touches `internal/toolchain`, `internal/config`, `internal/cli`, and `internal/core` (cleanly decomposable into 4 atomic tasks).
* **Effort estimate**: Medium.
* **Fit**: Excellent — satisfies all operator requirements and repository security/test invariants.

### Option C: Fall back to `os.Executable()` or `exec.LookPath("backlogit")`

* **Description**: Allow `pinnedMCPDaemonBinary` to use the current process executable or global `PATH` when `bin/backlogit.exe` is missing.
* **Pros**: Starts daemon without checking workspace tool directories.
* **Cons**: Directly violates Level 1 workspace-local pinning (`082-DL`, `089-DL`) and breaks `TestU6_MissingPinnedBinaryRefusalAndCLIWiring` and `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment`.
* **Effort estimate**: Low.
* **Fit**: Unacceptable — violates workspace containment policy.

## Trade-off Comparison

| Criterion | Option A (`.bin` only) | Option B (Layered `.mcp.json` + `tools_dir` + `.bin`/`bin` + branch scan fix) | Option C (`os.Executable()` / `PATH` fallback) |
| --- | --- | --- | --- |
| Honors `.mcp.json` command path | No | Yes (Priority 1, fail-closed on invalid/missing target) | No |
| Configurable tools directory | No | Yes (`tools_dir` in `toolchain.yaml` and `config.yaml`) | No |
| Defaults to `.bin` while supporting `bin` | Partial (`.bin` only) | Yes (`.bin` default, `bin` fallback) | No |
| Preserves `*os.Root` no-follow containment | Yes | Yes (every directory segment and leaf verified via `*os.Root`) | No |
| Backward compatibility with `TestU6`–`TestU12` | Fails | 100% green | Fails `TestU6` and `TestU12` |

## Decision

Adopt **Option B** (`102-DL`) and promote directly to implementation planning (`docs/plans/2026-11-19-mcp-daemon-configurable-tools-dir-plan.md`) and backlog harvest.

## Rejected Alternatives

* **Option A** was rejected because it fails to honor custom `.mcp.json` paths or configurable tool directories and breaks backward compatibility with `bin/` workspaces and tests.
* **Option C** was rejected because ambient `PATH` or unpinned host executable fallback violates Level 1 workspace containment (`082-DL`, `089-DL`, `TestU6`, `TestU12`).

## Unresolved Questions

None.

## Risks and Mitigations

* **Risk**: `backlogit init` self-installs `<storage_root>/backlogit[.exe]` (`.backlog/backlogit[.exe]` or `.backlogit/backlogit[.exe]`), and `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` expects `PrewarmMCPDaemon` on a freshly `init`-ed workspace (with no `.bin/`, `bin/`, `.mcp.json`, or `toolchain.yaml`) to return `ErrDaemonBinaryMissing`.
  * **Mitigation**: Forbid `.backlog`, `.backlogit`, and `.git` as tool directories in both `pinnedMCPDaemonBinary` and `toolchain` verification so `<storage_root>/backlogit[.exe]` is never mistaken for a workspace tool-directory binary.
* **Risk**: Symlink or Windows junction (`mklink /J`) swap on `.bin`, `bin`, a custom `tools_dir`, or `.mcp.json`.
  * **Mitigation**: Walk every path component under `os.OpenRoot(absWorkspace)` using `root.Lstat(segment)` + `rejectLink(info, filepath.Join(absWorkspace, segment))` before opening the regular file handle and verifying `os.SameFile(opened, linked)`.
