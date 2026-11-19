---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash 2CCDFDED (102-DL): honor .mcp.json command paths, support configurable tools_dir, default the workspace tool directory to .bin with bin fallback in MCP daemon startup and toolchain verification, and bound cross-branch ID history git scans.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-19-mcp-daemon-configurable-tools-dir-plan.md
title: 'Plan: Honor .mcp.json, Configurable tools_dir, and .bin Default in MCP Daemon Startup and Toolchain Verification'
---

## Problem Frame

Released `backlogit` binaries fail to start `backlogit mcp` and fail static toolchain/MCP verification in workspaces that use `.bin/` (the GSD default tool directory) or custom `.mcp.json` / tool-directory paths, as analyzed in [the 2CCDFDED deliberation](../decisions/2026-11-19-mcp-daemon-configurable-tools-dir-deliberation.md) (`102-DL`):

1. **`internal/cli/mcp_daemon.go` (`pinnedMCPDaemonBinary`, lines 384-418)**: `PrewarmMCPDaemon`, `RunMCPSessionShim`, and `VerifyMCPDaemonAttachment` call `pinnedMCPDaemonBinary(workspace)`, which hard-codes `filepath.Join(workspace, "bin", name)` and checks only `root.Lstat("bin")` and `root.Lstat(filepath.Join("bin", name))`. Workspaces with `.bin/backlogit[.exe]` or a custom `.mcp.json` / `toolchain.yaml` / `config.yaml` path fail with `corerrors.ErrDaemonBinaryMissing` (or mismatch a running `.bin/backlogit[.exe]` daemon against a stale `bin/backlogit[.exe]` binary with `corerrors.ErrDaemonVersionMismatch`).
2. **`internal/toolchain/verify.go` (`isPinnedBacklogitRelPath`, `isBacklogitExecutable`, `validBacklogitTool`, `validateMCPConfigPinning`, lines 304-335, 692, 725)**: Static toolchain and `.mcp.json` verification hard-codes `bin/backlogit.exe` and `bin/backlogit`, rejecting `.bin/backlogit[.exe]` and configurable tool directories.
3. **`internal/toolchain/launcher.go` (`launcherFinding`, `toolRemediation`, `inspectLauncher`, `validateManifestForProvisioning`, lines 86-120, 228, 611)**: Launcher inspection and remediation messages hard-code `bin/` prefix checks and `./bin/backlogit.exe` guidance, and `ToolchainManifest` / `schemas/workspace-toolchain.schema.json` lack an optional `tools_dir` setting.
4. **`internal/core/id_history.go` (`scanLocalBranchArtifactIDsContext`, lines 1148-1188)**: `scanLocalBranchArtifactIDsContext` invokes `git for-each-ref refs/heads` across all local branches and spawns a separate `git ls-tree` subprocess per branch under a 5-second `ws.boundedHelperTimeout()`, timing out in repositories with many merged historical local branches.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Support `.bin/backlogit[.exe]`, `bin/backlogit[.exe]`, and configurable `tools_dir` in `ToolchainManifest` (`types.go`) and static toolchain/MCP verification (`verify.go`) while preserving fail-closed containment | `2CCDFDED` / `102-DL` | Unit U1 | `TestU1_ToolchainVerifyAcceptsDotBinAndConfigurableToolsDir` |
| R2: Support `.bin` default and configurable `tools_dir` in `schemas/workspace-toolchain.schema.json` and `internal/toolchain/launcher.go` (`validateManifestForProvisioning`, `toolRemediation`, `launcherFinding`, `inspectLauncher`) | `2CCDFDED` / `102-DL` | Unit U2 | `TestU2_LauncherAndProvisioningSupportDotBinAndConfigurableToolsDir` |
| R3: Honor `.mcp.json`, `toolchain.yaml`, `config.yaml` `tools_dir`, `.bin` default, and `bin` fallback in `pinnedMCPDaemonBinary`, `PrewarmMCPDaemon`, and `VerifyMCPDaemonAttachment` under `os.OpenRoot(absWorkspace)` | `2CCDFDED` / `102-DL` | Unit U3 | `TestU3_PinnedMCPDaemonBinaryHonorsMCPJsonToolsDirAndDotBinDefault` |
| R4: Batch and deduplicate branch-tree inspection in `scanLocalBranchArtifactIDsContext` (`internal/core/id_history.go`) across all `refs/heads` using `git cat-file --batch` and subtree OID deduplication so all merged and unmerged branch-tip reservations (including merged-then-reverted artifacts) are preserved while `CreateArtifact` never times out on repositories with hundreds of local branches | `2CCDFDED` / `102-DL` | Unit U4 | `TestU4_ScanLocalBranchArtifactIDsBatchesTreesAndPreservesMergedRevertedReservations` |

## Implementation Units

### Unit U1: Support `.bin` default and configurable `tools_dir` in `internal/toolchain/types.go` and `internal/toolchain/verify.go`

* **Domain**: `code` (width-isolated to `internal/toolchain`)
* **Size / Complexity**: `S` / `low` (3 files, <= 4 functions, <= 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit)
* **Affected files (3)**:
  * `internal/toolchain/types.go`
  * `internal/toolchain/verify.go`
  * `internal/toolchain/verify_test.go`
* **Changes needed**:
  1. In `internal/toolchain/types.go`, add optional `ToolsDir string` (`json:"tools_dir,omitempty" yaml:"tools_dir,omitempty"`) to `ToolchainManifest`.
  2. In `internal/toolchain/verify.go`:
     * Update `isPinnedBacklogitRelPath(rel string) bool` to accept any clean workspace-local relative path (`rel = filepath.Clean(rel)`, `filepath.IsLocal(rel)`) that resides in a subdirectory (`dir := filepath.Dir(rel); dir != "." && dir != ".."`), whose first path segment is not `.git`, `.backlog`, or `.backlogit` (preventing `<storage_root>/backlogit[.exe]` self-installed copies or `.git` from being treated as a tools directory), and whose base name is `"backlogit"` or `"backlogit.exe"` (`samePath(filepath.Base(rel), "backlogit") || samePath(filepath.Base(rel), "backlogit.exe")`).
     * Add `validManifestToolsDir(root *os.Root, abs, toolsDir string) (string, error)` to validate a non-empty `m.ToolsDir`: reject whitespace-padded or whitespace-only values, require `relDir, err := relativePath(abs, toolsDir, false)` with `relDir != "."`, reject top-level segments `.git`, `.backlog`, and `.backlogit`, and require `bindingWithin(root, abs, toolsDir)`.
     * Add `validBacklogitToolInManifest(abs string, m *ToolchainManifest, tool ToolEntry) bool`: preserve `validBacklogitTool`'s non-backlogit early return (`if !isBacklogitTool(abs, tool) { return true }`) at the very top so `remote-declared`, `built-in-host` (empty `ExecutablePath`), and non-`backlogit` tools in mixed inventories are unaffected by `m.ToolsDir`; for `backlogit` tools, require `validBacklogitTool(abs, tool)` and, when `m != nil && strings.TrimSpace(m.ToolsDir) != ""`, additionally require `relExec, err := relativePath(abs, tool.ExecutablePath, true)` and `relToolsDir, dirErr := relativePath(abs, m.ToolsDir, false)` to satisfy `err == nil && dirErr == nil && samePath(filepath.Dir(relExec), relToolsDir)`.
     * Update `loadToolchainManifest` (checking `hasRawYAMLKey(&node, "tools_dir")` so an explicit blank/whitespace `tools_dir: ""` or `tools_dir: "   "` is rejected with `blerrors.ErrValidation`, and validating non-empty `m.ToolsDir` via `validManifestToolsDir`), `verifyWorkspaceToolchain`, and `validateMCPConfigPinning` to validate `m.ToolsDir` and enforce `validBacklogitToolInManifest(abs, m, tool)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/toolchain/verify_test.go`)**: Add `TestU1_ToolchainVerifyAcceptsDotBinAndConfigurableToolsDir` (using existing exported functions and `LoadToolchainManifest` YAML decoding so the test compiles cleanly before production edits) verifying: (a) a workspace with `.bin/backlogit.exe` and `.mcp.json` using `"./.bin/backlogit.exe"` passes `VerifyWorkspaceToolchain` and `ValidateMCPConfigPinning` with zero violations; (b) a mixed-inventory workspace manifest YAML declaring `tools_dir: ".tools/bin"` with `backlogit` (`workspace-local-pinned`, `executable_path: "./.tools/bin/backlogit.exe"`), a `remote-declared` tool (empty `executable_path`), and a `built-in-host` tool (empty `executable_path`) loads cleanly via `LoadToolchainManifest` and passes `VerifyWorkspaceToolchain`, `ValidateMCPConfigPinning`, and `WriteToolchainManifest` with zero violations, whereas a `backlogit` executable outside the declared `tools_dir` (`./bin/backlogit.exe` when `tools_dir: ".tools/bin"`) or an invalid `tools_dir` (`"../outside"`, `".backlogit"`, or `"   "`) is rejected; and (c) unsafe `backlogit` paths (`./.bin/other.exe`, root-level `./backlogit.exe`, `.backlogit/backlogit.exe`, and symlinked/junction `.bin`) are rejected fail-closed.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU1_ToolchainVerifyAcceptsDotBinAndConfigurableToolsDir|TestU8_|TestU9_|TestU2_)' ./internal/toolchain`

### Unit U2: Support `.bin` default and configurable `tools_dir` in `internal/toolchain/launcher.go` and `schemas/workspace-toolchain.schema.json`

* **Domain**: `code` (width-isolated to `internal/toolchain` and `schemas/workspace-toolchain.schema.json`)
* **Size / Complexity**: `S` / `low` (3 files, <= 4 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U1 (`ToolchainManifest.ToolsDir`, `validManifestToolsDir`, and `validBacklogitToolInManifest`)
* **Affected files (3)**:
  * `internal/toolchain/launcher.go`
  * `schemas/workspace-toolchain.schema.json`
  * `internal/toolchain/launcher_test.go`
* **Changes needed**:
  1. In `schemas/workspace-toolchain.schema.json`, add optional `"tools_dir": { "type": "string", "minLength": 1 }` to top-level `properties`.
  2. In `internal/toolchain/launcher.go`:
     * Update `validateManifestForProvisioning(root *os.Root, abs string, manifest *ToolchainManifest) error` to validate `manifest.ToolsDir` via `validManifestToolsDir(root, abs, manifest.ToolsDir)` when `manifest.ToolsDir != ""` and enforce `validBacklogitToolInManifest(abs, manifest, tool)`.
     * Add `effectiveManifestToolsDir(m *ToolchainManifest) string` and `backlogitRemediationPath(m *ToolchainManifest) string`: resolve the tool directory from `m.ToolsDir` (if valid), else from the `backlogit` (or first valid `WorkspaceLocalPinned`) entry's `ExecutablePath` parent directory in `m.Tools` (returning `"bin"` for legacy `./bin/...` manifests so all `TestU11_*` assertions stay green, `".bin"` for `./.bin/...` manifests, or custom `<dir>`), and defaulting to `".bin"` when `m` is nil or has no valid local tool paths.
     * Update `toolRemediation`, `launcherFinding`, `verifyLaunchers`, and `inspectLauncher` to use `effectiveManifestToolsDir(m)` and `backlogitRemediationPath(m)` instead of hard-coding `"bin/"` and `"./bin/backlogit.exe"`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/toolchain/launcher_test.go`)**: Add `TestU2_LauncherAndProvisioningSupportDotBinAndConfigurableToolsDir` testing: (a) `WriteToolchainManifest`, `RenderPinnedMCPConfig`, `VerifyLauncherScripts`, and `AuditRepositoryToolchain` pass with zero violations when tools and `start.ps1`/`start.sh` use `.bin/backlogit.exe` or a custom `tools_dir: "custom-bin"`; (b) violation remediation strings in `AuditRepositoryToolchain` reference `./.bin/...` for `.bin` (or default) manifests, `./custom-bin/...` for `tools_dir: "custom-bin"` manifests, and `./bin/...` for legacy `bin` manifests; and (c) `WriteToolchainManifest` and `RenderPinnedMCPConfig` reject invalid `tools_dir` values (`"../escape"`, `".backlogit"`, or mismatched `backlogit` executable directory) with `corerrors.ErrValidation`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU2_LauncherAndProvisioning|TestU1_WriteToolchainManifest|TestU11)' ./internal/toolchain ./internal/cli`

### Unit U3: Honor `.mcp.json`, `toolchain.yaml`, configurable `tools_dir`, and `.bin` default in `pinnedMCPDaemonBinary`

* **Domain**: `code` (width-isolated to `internal/config` and `internal/cli`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: `blocks` on Unit U1 and Unit U2
* **Affected files (3)**:
  * `internal/config/schema.go`
  * `internal/cli/mcp_daemon.go`
  * `internal/cli/mcp_session_shim_test.go`
* **Changes needed**:
  1. In `internal/config/schema.go`, add optional `ToolsDir string` (`yaml:"tools_dir,omitempty"`) to `WorkspaceConfig` and validate it in `(c *WorkspaceConfig) Validate() error`: when `c.ToolsDir != ""`, reject leading/trailing whitespace or whitespace-only values, require a clean local relative subdirectory path (`filepath.IsLocal(clean)`, `clean != "."`, no volume or absolute prefix), and reject top-level segments `.git`, `.backlog`, and `.backlogit`.
  2. In `internal/cli/mcp_daemon.go`, update `pinnedMCPDaemonBinary(workspace string) (binary string, result error)` (with helpers `resolveConfiguredDaemonBinaryRel` and `verifyPinnedDaemonBinaryRel`) to resolve the workspace-pinned `backlogit` binary under `os.OpenRoot(absWorkspace)` in strict priority order:
     * **Pinned child-handle traversal (`verifyPinnedDaemonBinaryRel`, following `openLocal` in `internal/toolchain/verify.go:102-153`)**: Split `rel` into segments (`parts := strings.Split(rel, string(filepath.Separator))`), tracking `current := root`, `path := absWorkspace` (never `current.Name()`), and `owned []*os.Root` (closed in reverse order via `defer`). For each segment `part`: advance `path = filepath.Join(path, part)`, call `info, err := current.Lstat(part)`, and verify `rejectLink(info, path)` (`core.IsSymlinkOrReparsePoint(info, path)` and `info.Mode()&os.ModeIrregular == 0`). For each intermediate directory segment (`i < len(parts)-1`), require `info.IsDir()`, open a pinned child directory handle `next, err := current.OpenRoot(part)`, verify directory handle identity via `opened, err := next.Stat(".")` and `os.SameFile(info, opened)`, and advance `current = next`. For the final leaf (`i == len(parts)-1`), require `info.Mode().IsRegular()`, open the file on the innermost pinned parent handle via `file, err := current.Open(part)` (never resolving `rel` from the top-level workspace `root`), and verify `opened, err := file.Stat()` with `opened.Mode().IsRegular() && os.SameFile(info, opened)`. Apply the same pinned child-handle descent when reading `.backlog/toolchain.yaml`, `.backlogit/toolchain.yaml`, `.backlog/config.yaml`, or `.backlogit/config.yaml`.
     * **Priority 1 (`.mcp.json`)**: Inspect `.mcp.json` via `root.Lstat(".mcp.json")`. If present, verify `rejectLink(info, filepath.Join(absWorkspace, ".mcp.json"))`, `info.Mode().IsRegular()`, and `os.SameFile`, then decode `mcpServers` (and fallback `servers`) with a bounded reader (`maxMCPDaemonInfoBytes`). If a `"backlogit"` entry (case-insensitive) or any server entry whose `command` base name identifies `backlogit` is declared, validate `command` as a workspace-contained relative subdirectory path (`!filepath.IsAbs` unless inside `absWorkspace`, contains `/` or `\`, no `..` traversal, `filepath.IsLocal(rel)`, `filepath.Dir(rel) != "."`, top-level segment not `.git`/`.backlog`/`.backlogit`, base name matching `backlogit[.exe]`) and verify every directory handle and the regular file leaf via `verifyPinnedDaemonBinaryRel(root, absWorkspace, rel)`. If `.mcp.json` is redirected/invalid JSON or its `backlogit` command is bare, escaping, redirected, non-regular, or missing on disk, fail closed immediately (`corerrors.ErrValidation` / `corerrors.ErrDaemonBinaryMissing`) without falling back to any other directory.
     * **Priority 2 (`toolchain.yaml`)**: If `.mcp.json` is absent or declares no `backlogit` server, inspect the workspace storage root (`.backlog` or `.backlogit`, honoring `BACKLOGIT_WORKSPACE_DIR`) via pinned child `*os.Root` handle (`root.Lstat(storageDir)` -> `rejectLink` -> `root.OpenRoot(storageDir)` -> `Stat(".")` + `os.SameFile`) for `toolchain.yaml`. If present, verify `rejectLink` + `IsRegular` + `os.SameFile` on the pinned storage handle and parse `toolchain.yaml`: if a `backlogit` tool entry (`executable_path`) or top-level `tools_dir` is declared, validate and verify that target via `verifyPinnedDaemonBinaryRel`, failing closed if invalid or missing.
     * **Priority 3 (`config.yaml` `tools_dir`)**: If `config.yaml` in the workspace storage root (read via the pinned storage `*os.Root` handle) declares a non-empty `tools_dir`, validate `<tools_dir>/backlogit[.exe]` and verify via `verifyPinnedDaemonBinaryRel`, failing closed if invalid or missing.
     * **Priority 4 (Default `.bin` then fallback `bin`)**: Check `.bin/<name>` first via `verifyPinnedDaemonBinaryRel` (if `.bin` or `.bin/<name>` is a symlink/reparse point, swapped directory, or non-regular file, fail closed immediately with `corerrors.ErrValidation`; if absent with `os.ErrNotExist`, continue to fallback). Next, check `bin/<name>` via `verifyPinnedDaemonBinaryRel`. If neither exists, return `corerrors.ErrDaemonBinaryMissing`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/mcp_session_shim_test.go`)**: Add `TestU3_PinnedMCPDaemonBinaryHonorsMCPJsonToolsDirAndDotBinDefault` testing: (a) a workspace with `.bin/backlogit[.exe]` (and no `bin/backlogit[.exe]`) succeeds in `PrewarmMCPDaemon`, `RunMCPSessionShim`, and `VerifyMCPDaemonAttachment`, and when both `.bin/backlogit[.exe]` and `bin/backlogit[.exe]` exist without `.mcp.json`, `.bin/backlogit[.exe]` is selected; (b) a workspace with `.mcp.json` pointing to `./custom-tools/backlogit[.exe]` (or `./.bin/backlogit[.exe]`) or `config.yaml`/`toolchain.yaml` setting `tools_dir: "custom-tools"` resolves `./custom-tools/backlogit[.exe]` even when `bin/backlogit[.exe]` also exists, and fails closed (`ErrDaemonBinaryMissing` / `ErrValidation`) without falling back to `bin/` or `PATH` when `.mcp.json` points to a missing binary, bare `"backlogit"`, traversal `"../outside/backlogit.exe"`, a symlink/junction directory, or an intermediate directory replaced with an in-workspace symlink during traversal (intermediate-directory swap regression); and (c) a workspace initialized with `backlogit init` (containing `<storage_root>/backlogit[.exe]` and no `.bin/`, `bin/`, `.mcp.json`, or `toolchain.yaml`) still fails `PrewarmMCPDaemon` with `ErrDaemonBinaryMissing`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3_PinnedMCPDaemonBinary|TestU5_|TestU6_)' ./internal/cli`

### Unit U4: Batch and deduplicate branch-tree inspection in `scanLocalBranchArtifactIDsContext` while preserving all branch-tip reservations

* **Domain**: `code` (width-isolated to `internal/core`)
* **Size / Complexity**: `S` / `low` (2 files, <= 3 functions, <= 3 test scenarios)
* **Execution posture**: `test-first`
* **Dependencies**: Independent of Units U1–U3
* **Affected files (2)**:
  * `internal/core/id_history.go`
  * `internal/core/id_history_test.go`
* **Changes needed**:
  1. In `internal/core/id_history.go` (`scanLocalBranchArtifactIDsContext`), replace the per-branch `git ls-tree` subprocess loop with batched tree inspection across **all** local branches in `refs/heads` (without `--no-merged=HEAD`, which would lose reservations on a merged branch whose artifact was later reverted or deleted from `HEAD`):
     * Query all local branches with their root tree OIDs via `git --no-replace-objects --no-optional-locks for-each-ref --format=%(refname:short) %(objectname) %(tree) refs/heads` (1 subprocess) and retain the `refname=objectname` fingerprint for `branchIDScanCache`.
     * Deduplicate root `%(tree)` OIDs across all branch tips, and inspect `<treeOID>:.backlog` and `<treeOID>:.backlogit` across all unique branch trees using level-by-level batched `git --no-replace-objects --no-optional-locks cat-file --batch` calls (`seenTrees := make(map[string]struct{})`): at level 0, feed all unique `<treeOID>:.backlog` and `<treeOID>:.backlogit` specs on stdin; parse each returned `tree` payload (`<mode> <name>\x00<raw-oid>`), recording `.md` entry stems and queueing any unseen `"40000"` directory entry hex OIDs for the next `git cat-file --batch` level until no unseen directory subtrees remain.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/core/id_history_test.go`)**: Add `TestU4_ScanLocalBranchArtifactIDsBatchesTreesAndPreservesMergedRevertedReservations` testing: (a) a git repository with 120 merged local branches plus 2 unmerged branches containing branch-only `.backlogit/queue/098-F.md` artifacts completes `scanLocalBranchArtifactIDsContext` and `CreateArtifact` within 2 seconds (well under the 5s `boundedHelperTimeout()`); and (b) a **merged-then-reverted branch reservation regression** where a feature branch creates `.backlogit/queue/099-F.md`, is merged into `HEAD`, and a subsequent commit on `HEAD` deletes/reverts `.backlogit/queue/099-F.md` (with no local `id-sequences.json` reservation ledger), verifying that `scanLocalBranchArtifactIDsContext` still preserves the `099-F` reservation from the merged branch tip and `CreateArtifact` allocates `100-F` rather than reusing `099-F`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU4_ScanLocalBranchArtifactIDs|TestU5_CrossBranch|TestU5b_)' ./internal/core`

## Dependency Graph

* **Unit U1** (`internal/toolchain/types.go`, `internal/toolchain/verify.go`, `internal/toolchain/verify_test.go`): no upstream dependencies (Wave 1)
* **Unit U4** (`internal/core/id_history.go`, `internal/core/id_history_test.go`): no upstream dependencies (Wave 1, parallel with Unit U1)
* **Unit U2** (`internal/toolchain/launcher.go`, `schemas/workspace-toolchain.schema.json`, `internal/toolchain/launcher_test.go`): `blocks` on Unit U1 (Wave 2)
* **Unit U3** (`internal/config/schema.go`, `internal/cli/mcp_daemon.go`, `internal/cli/mcp_session_shim_test.go`): `blocks` on Unit U1 and Unit U2 (Wave 3)

## Decisions and Rationale

1. **Strict priority order in `pinnedMCPDaemonBinary` (`.mcp.json` -> `toolchain.yaml` -> `config.yaml` `tools_dir` -> `.bin` default -> `bin` fallback)**:
   * Honoring `.mcp.json` first ensures that whatever workspace-local binary command is configured in `.mcp.json` (the file MCP hosts actually launch) is authoritative for `PrewarmMCPDaemon` and `VerifyMCPDaemonAttachment`, even if a legacy `toolchain.yaml` or `bin/` directory is also present.
   * Checking `.bin` before `bin` when no explicit config is present aligns with GSD's `.bin` default while preserving backward compatibility with legacy `bin` workspaces and all existing test fixtures (`u6NewFixture`, `u9PinnedWorkspace`, `u11Fixture`, `u12FullManifest`).
2. **Fail-closed refusal when `.mcp.json` (or `toolchain.yaml` / `config.yaml`) configures an invalid or missing binary**:
   * If `.mcp.json` declares a `backlogit` server whose `command` is a bare command (`"backlogit"`), traverses outside the workspace (`"../outside/backlogit.exe"`), points through a symlink/junction, or points to a missing file, `pinnedMCPDaemonBinary` immediately returns `ErrValidation` / `ErrDaemonBinaryMissing` and never falls back to `.bin/`, `bin/`, or `PATH`.
3. **Exclude `.backlog`, `.backlogit`, and `.git` from valid tool directories**:
   * `backlogit init` and `backlogit install` self-install `<storage_root>/backlogit[.exe]` (`.backlog/backlogit[.exe]` or `.backlogit/backlogit[.exe]`). Forbidding `.backlog`, `.backlogit`, and `.git` as tool directories ensures `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`missing_local_binary_refuses_global_path_fallback`) continues to fail closed with `ErrDaemonBinaryMissing` when no tool-directory binary exists.
4. **Preserve all `refs/heads` branch-tip reservations via batched `git cat-file --batch` tree traversal and OID deduplication**:
   * Filtering `refs/heads` with `--no-merged=HEAD` is unsafe because a merged branch can still retain `.backlogit/queue/099-F.md` after a later deletion or revert removes every trace of `099-F` from `HEAD`'s current tree; excluding that merged branch would lose its reservation when the local `id-sequences.json` ledger is absent. Instead, `scanLocalBranchArtifactIDsContext` inspects all `refs/heads` via `for-each-ref --format=%(refname:short) %(objectname) %(tree) refs/heads`, deduplicates root tree OIDs and visited `.backlog`/`.backlogit` subtree OIDs (`seenTrees`), and reads tree entries in batch via `git cat-file --batch` alongside `branchIDScanCache`, preserving 100% of branch-tip reservations while reducing 325 per-branch subprocesses to 2–3 subprocesses total.

## Risks and Caveats

* **Risk**: `TestU11_LauncherProfileRefusesAmbiguousSyntaxAndNamesCompanionFallback` (`internal/cli/mcp_toolchain_test.go:467`) and `TestU11_VerifyLauncherScriptsChecksStartPs1AndMcpJsonBacklogitPinning` (`mcp_toolchain_test.go:250`) assert that remediation strings in `u11Fixture` (whose manifest declares `./bin/backlogit.exe`) contain `"./bin/engram.exe"` and `"./bin/backlogit.exe"`.
  * **Mitigation**: `effectiveManifestToolsDir(m)` and `backlogitRemediationPath(m)` inspect `m.ToolsDir` first, then the directory of `backlogit` / local tools in `m.Tools` (which is `"bin"` in `u11Fixture`), and default to `".bin"` only when `m` has no configured tool directory or `m.Tools` uses `.bin`. Thus `TestU11_*` receives `"./bin/..."` for its `./bin` fixture while `.bin` and default manifests receive `"./.bin/..."`.
* **Risk**: Symlink, Windows junction (`mklink /J`), or TOCTOU directory swap on an intermediate directory segment of `.bin`, `bin`, or a multi-segment `tools_dir` (e.g., `.tools/bin`).
  * **Mitigation**: `verifyPinnedDaemonBinaryRel` descends step-by-step through pinned child `*os.Root` directory handles following `openLocal` (`internal/toolchain/verify.go:102-153`): for each directory segment `part`, it calls `current.Lstat(part)` + `rejectLink(info, path)` -> `next, err := current.OpenRoot(part)` -> `opened, err := next.Stat(".")` + `os.SameFile(info, opened)` before advancing `current = next`, and opens the final leaf via `current.Open(part)` on the innermost pinned parent handle with `os.SameFile(info, opened)` verification. Replacing a parent directory with an in-workspace symlink after inspection cannot redirect the pinned child handle or pass `os.SameFile`.

## Constitution Check

* **Safety-First Go (`NON-NEGOTIABLE`)**: `pass` — All changes are written in Go 1.24, wrap errors with `%w` preserving `ErrValidation` and `ErrDaemonBinaryMissing`, and use zero `unsafe` constructs.
* **Test-First Development (`NON-NEGOTIABLE`)**: `pass` — Every unit (`U1` through `U4`) begins with a compiling, failing RED test harness (`TestU1_*` through `TestU4_*`) before any production code edit.
* **Workspace Isolation and Security Boundaries (`NON-NEGOTIABLE`)**: `pass` — All binary and config path checks in `pinnedMCPDaemonBinary` and `internal/toolchain` use `os.OpenRoot(absWorkspace)` handle-confined traversal, `core.IsSymlinkOrReparsePoint`, and `os.SameFile`.
* **CLI Workspace Containment (`NON-NEGOTIABLE`)**: `pass` — All operations read and validate strictly within the target workspace root.
* **Destructive Command Approval (`NON-NEGOTIABLE`)**: `pass` — No destructive commands or migrations are introduced.
* **Merge Commit History Preservation (`NON-NEGOTIABLE`)**: `pass` — The shipment will merge via a merge commit (`--merge`) under P-009/P-014.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Adds optional `tools_dir` to `schemas/workspace-toolchain.schema.json`, `ToolchainManifest` (`internal/toolchain/types.go`), and `WorkspaceConfig` (`internal/config/schema.go`), and widens valid pinned `backlogit` paths from hard-coded `bin/` to `.bin/` (default), `bin/`, and configured tool directories.
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Modifies workspace-local executable pinning and containment verification in `internal/cli/mcp_daemon.go` and `internal/toolchain/verify.go` / `launcher.go`.
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — `tools_dir` is optional (`omitempty`) and backward-compatible with all existing `toolchain.yaml` and `config.yaml` files.
* **external integration, operator checkpoint, or external dependency**: `absent` — No external dependencies added (`go.mod` unchanged).
* **high runtime, rollout, or rollback risk**: `present` — Modifies `backlogit mcp` daemon startup/attachment (`PrewarmMCPDaemon`, `VerifyMCPDaemonAttachment`, `RunMCPSessionShim`) and `CreateArtifact` git branch ID scanning (`scanLocalBranchArtifactIDsContext`).

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * `backlogit mcp`, `backlogit mcp prewarm`, and `backlogit mcp status` (`internal/cli/mcp_daemon.go`)
  * `backlogit mcp status --toolchain --strict` (`internal/toolchain/verify.go`, `internal/toolchain/launcher.go`)
  * `backlogit add` / `backlogit deliberate` / `backlogit shipment create` ID allocator branch history scan (`internal/core/id_history.go`)
* **Runtime verification probes (for `_Ship` Step 6)**:
  1. In an isolated `--cwd <tempDir>` workspace containing `.bin/backlogit[.exe]` and `.mcp.json` with `"command": "./.bin/backlogit[.exe]"`, run `PrewarmMCPDaemon` / `backlogit --cwd <tempDir> mcp prewarm --format json` and `backlogit --cwd <tempDir> mcp status --format json`, verifying the daemon starts from `.bin/backlogit[.exe]`, attaches cleanly, and stops via `backlogit --cwd <tempDir> mcp stop`.
  2. In an isolated `--cwd <tempDir>` workspace configured with a custom `tools_dir: "custom-bin"` in `.backlogit/toolchain.yaml` and `./custom-bin/backlogit[.exe]` in `.mcp.json`, run `AuditRepositoryToolchain` / `backlogit --cwd <tempDir> mcp status --toolchain --strict --format json` and `PrewarmMCPDaemon`, verifying `valid: true, violations: 0` and clean daemon startup.
  3. Run `backlogit add` / `CreateArtifact` in a repository with >100 merged local branches and verify ID allocation completes in <2s without timing out.
* **Operational closure expectations**:
  * **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
  * **Validation window**: Immediate post-merge CLI/MCP verification on `main` and external GSD workspaces using `.bin/backlogit[.exe]`.
  * **Healthy signals**: Zero `ErrDaemonBinaryMissing` or `ErrDaemonVersionMismatch` errors on `.bin/` and `.mcp.json`-configured workspaces; zero violations on `make docs-lint` and `go test -timeout=30m ./...`.
  * **Rollback trigger & procedure**: Any containment regression or daemon startup failure triggers a reviewed `git revert -m 1 <merge_sha>` PR; optional `tools_dir` fields are `omitempty` and require no state migration on rollback.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by contract, security-containment, and runtime-startup signals across `internal/cli`, `internal/toolchain`, `internal/config`, and `internal/core`.
* **Protected invariants**:
  * **I1 (Pinned child-handle step-by-step verification in `pinnedMCPDaemonBinary`)**: Every candidate binary path (from `.mcp.json`, `toolchain.yaml`, `config.yaml` `tools_dir`, `.bin/`, or `bin/`) MUST be verified relative to `os.OpenRoot(absWorkspace)` by descending through pinned child `*os.Root` directory handles following `openLocal` (`internal/toolchain/verify.go:102-153`): for each directory segment `part`, inspect `info, err := current.Lstat(part)` + `rejectLink(info, path)` (`core.IsSymlinkOrReparsePoint`), open the child directory handle `next, err := current.OpenRoot(part)`, verify `opened, err := next.Stat(".")` and `os.SameFile(info, opened)`, and advance `current = next`; then open the final leaf on the innermost pinned parent handle via `current.Open(part)` (never `root.Open(rel)` from the top-level workspace root) and confirm `opened.Mode().IsRegular() && os.SameFile(info, opened)`. Never pass `current.Name()` into path-based Win32 helpers when a nested root is used; always thread the verified `absWorkspace`.
  * **I2 (Fail-closed refusal on invalid or missing configured paths — no silent fallback)**: When `.mcp.json` (or `toolchain.yaml` / `config.yaml` `tools_dir`) explicitly configures a `backlogit` command or tool directory, any validation failure (bare command name, `..` traversal, path outside workspace, root-level `./backlogit.exe`, `.backlog`/`.backlogit`/`.git` directory, symlink/reparse point, or missing file on disk) MUST fail closed immediately (`ErrValidation` / `ErrDaemonBinaryMissing`) and MUST NOT fall back to `.bin/`, `bin/`, `os.Executable()`, or `PATH`.
  * **I3 (Storage-root binary exclusion preserving `TestU12` and `TestU6`)**: `<storage_root>/backlogit[.exe]` (`.backlog/backlogit[.exe]` and `.backlogit/backlogit[.exe]`, created by `backlogit init` / `install`) and `.git` MUST never be accepted as a tool directory in `pinnedMCPDaemonBinary` or `isPinnedBacklogitRelPath` / `validManifestToolsDir`, preserving `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` and `TestU6_MissingPinnedBinaryRefusalAndCLIWiring`.
  * **I4 (Manifest-aware remediation preserving `TestU11` compatibility)**: `effectiveManifestToolsDir(m)` and `backlogitRemediationPath(m)` in `internal/toolchain/launcher.go` MUST derive the remediation directory from `m.ToolsDir` or `m.Tools` (`ExecutablePath` directory) when present and default to `".bin"` only when `m` has no valid local tool directory, keeping all existing `./bin/...` remediation assertions in `TestU11_*` green while emitting `./.bin/...` or `./<tools_dir>/...` for `.bin` and custom-configured workspaces.
  * **I5 (Cross-branch ID reservation completeness across all `refs/heads`)**: `scanLocalBranchArtifactIDsContext` MUST scan all branches in `refs/heads` (never filtering with `--no-merged=HEAD`, so merged branches whose artifacts were later deleted or reverted on `HEAD` retain their reservations even when `id-sequences.json` is absent) by deduplicating branch root `%(tree)` OIDs and `.backlog`/`.backlogit` subtree OIDs and reading tree entries in batch via `git cat-file --batch` alongside `branchIDScanCache`.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/go-mcp-server.instructions.md`
  * `docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md`
  * `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`
  * `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`
  * `docs/compound/2026-10-16-root-confined-access-and-graphql-reviewer-union.md`
  * `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md`
  * `docs/compound/test-failures/windows-retry-error-boundaries-and-handle-aware-observers-2026-11-09.md`
* **Risky actions classification (`strict-safety`)**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Expand `isPinnedBacklogitRelPath` and add `ToolsDir` validation in `internal/toolchain/verify.go` | `internal/toolchain/types.go`, `verify.go` | `moderate` (containment contract) | Preserve `!isBacklogitTool` early return for mixed inventories; require subdirectory path, forbid `.git`/`.backlog`/`.backlogit`, require `backlogit[.exe]` base name, enforce `m.ToolsDir` match on `backlogit` tools when set | `.bin`, `bin`, and custom `tools_dir` (including mixed inventories) pass verification; unsafe/mismatched paths fail closed |
| Update `launcher.go` provisioning validation and manifest-aware remediation paths | `internal/toolchain/launcher.go`, `schemas/workspace-toolchain.schema.json` | `low` (static audit & schema) | Derive remediation path from manifest (`m.ToolsDir` / `m.Tools`), defaulting to `.bin` | Accurate `.bin`/`<tools_dir>`/`bin` remediation and launcher verification with `TestU11`/`TestU12` green |
| Resolve `pinnedMCPDaemonBinary` from `.mcp.json` -> `toolchain.yaml` -> `config.yaml` -> `.bin` -> `bin` | `internal/cli/mcp_daemon.go`, `internal/config/schema.go` | `moderate` (daemon execution boundary) | Descend through pinned child `*os.Root` directory handles (`Lstat` + `rejectLink` + `OpenRoot` + `Stat(".")` + `SameFile`) and open leaf on innermost handle; fail closed on invalid/missing configured command | `.bin` and custom `.mcp.json` workspaces start `backlogit mcp` cleanly; unpinned/redirected/swapped paths are refused |
| Batch and deduplicate branch-tree inspection in `scanLocalBranchArtifactIDsContext` across all `refs/heads` | `internal/core/id_history.go` | `low` (ID allocator performance & reliability) | Query `%(tree)` for all `refs/heads`, deduplicate root and `.backlog`/`.backlogit` subtree OIDs, and read tree entries via batched `git cat-file --batch` | Eliminates 5s timeout on repos with 300+ branches while preserving all branch-tip reservations (including merged-then-reverted artifacts) |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode: multi-agent-dispatch` (or `single-agent-declared-degradation` if degraded), cover all required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`), and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) before harvest.
* **Unresolved operator decisions**: None (autopilot mode active).

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale**: Dispatched all 7 reviewer subagents (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`). Six subagents (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`) completed and returned `0` findings (`PASS`). Because `Security Lens Reviewer` returned an empty subagent response mid-gate, per the `plan-review` terminal-states rule ("do not keep the `multi-agent-dispatch` label... run a complete sequential rubric pass over all selected personas and emit `single-agent-declared-degradation`"), a complete sequential rubric pass was executed across all 7 selected personas (`[PERSONA-PASS]` / `[PERSONA-DONE]` for `Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, and `Security Lens Reviewer`). Total merged findings across all 7 personas: `0` (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Persona evaluation summary**:
  * **Constitution Reviewer**: `0` findings (`PASS`). Units U1–U4 comply with Principles I–XI and Task Granularity (<= 3 files, <= 4 functions, <= 3 test scenarios per unit; pure Go width isolation). `## Constitution Check` concludes with literal `Constitution Check: pass`.
  * **Go Reviewer**: `0` findings (`PASS`). `ToolsDir` struct tags use `omitempty`, error paths wrap `corerrors.ErrValidation` and `corerrors.ErrDaemonBinaryMissing` with `%w`, and `os.OpenRoot(absWorkspace)` handles are closed via `defer` with `os.SameFile` verification.
  * **Scope Boundary Auditor**: `0` findings (`PASS`). Units U1–U4 map 1:1 to `R1`–`R4` (`2CCDFDED` / `102-DL`), and companion stash `85442104` (PowerShell helper scripts) is deferred to maintain width isolation.
  * **Learnings Researcher**: `0` findings (`PASS`, `confidence: high`). Plan incorporates `2026-10-27-pin-the-handle-not-the-path.md`, `nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`, `derive-daemon-version-from-pinned-image-2026-11-09.md`, `2026-07-06-exec-binary-config-must-be-bare-path-validated.md`, and `2026-07-06-bounded-helper-timeout-hard-cap.md`.
  * **Architecture Strategist**: `0` findings (`PASS`). Package dependency direction (`cli -> {toolchain, config, core}`, `toolchain -> {core, errors}`) and wave ordering (`U1` + `U4` -> `U2` -> `U3`) are acyclic and cohesive.
  * **Agent-Native Parity Reviewer**: `0` findings (`PASS`). MCP daemon startup (`PrewarmMCPDaemon` / `RunMCPSessionShim` / `VerifyMCPDaemonAttachment`) and CLI toolchain verification (`backlogit mcp status --toolchain`) consistently honor `.mcp.json`, `toolchain.yaml`, `config.yaml` `tools_dir`, `.bin` default, and `bin` fallback.
  * **Security Lens Reviewer**: `0` findings (`PASS`). Lightweight 3-threat model verified: (1) bare/escaping/non-`backlogit` commands in `.mcp.json` / `toolchain.yaml` / `config.yaml` fail closed (`ErrValidation` / `ErrDaemonBinaryMissing`) without fallback (Invariant I2); (2) symlink/junction redirects or TOCTOU intermediate-directory swaps on `.bin`, `bin`, or multi-segment `tools_dir` paths are refused by descending through pinned child `*os.Root` directory handles (`Lstat` + `rejectLink` + `OpenRoot` + `Stat(".")` + `SameFile`) and opening the leaf on the innermost handle (Invariant I1); and (3) `.git`, `.backlog`, and `.backlogit` are excluded from valid tool directories (Invariant I3).
* **Plan hardening verification**: `Requires plan hardening: yes` is satisfied by the complete `## Plan Hardening` section with protected invariants `I1`–`I5`, `ProposedAction` / `ActionRisk` table, and `## Runtime Verification and Closure`.

