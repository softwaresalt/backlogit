---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash 269D054D (103-DL): prompt for target tool directory (.bin, bin, or custom workspace-relative directory) on backlogit init and install, create the directory, install backlogit[.exe] there, and configure .mcp.json to use it.'
doc_type: plan
schema_version: "1.0"
source: docs/plans/2026-11-19-init-install-target-tools-dir-mcp-config-plan.md
title: 'Plan: Target Tool Directory Prompt, Binary Installation, and .mcp.json Configuration on init and install'
---

## Problem Frame

Currently, `backlogit init` and `backlogit install` (`internal/cli/root.go`, lines 291-368, and `internal/cli/self_install.go`) copy the running executable only into the workspace storage directory (`.backlog/backlogit[.exe]` or `.backlogit/backlogit[.exe]`), which `pinnedMCPDaemonBinary` (`internal/cli/mcp_daemon.go`) and static toolchain verification (`internal/toolchain/verify.go`, `internal/toolchain/launcher.go`) treat as a reserved storage root rather than a valid workspace tool directory. As documented in [the 269D054D deliberation](../decisions/2026-11-19-init-install-target-tools-dir-mcp-config-deliberation.md) (`103-DL`):

1. **`internal/cli/self_install.go` (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`)**: `init` and `install` do not create a target tool directory (defaulting to `.bin`, or `bin` when `bin/` already exists and `.bin/` does not), do not install `backlogit[.exe]` into that target tool directory, and do not add `/<tools_dir>/backlogit` and `/<tools_dir>/backlogit.exe` ignore entries to `.gitignore` and `.git/info/exclude`.
2. **Missing `.mcp.json` and `config.yaml` (`tools_dir`) Provisioning (`internal/cli/self_install.go`)**: After creating the target tool directory and installing `backlogit[.exe]` into it, `init` and `install` do not create or update `.mcp.json` in the target workspace (or record `tools_dir` in `<storage_root>/config.yaml`) so that `mcpServers.backlogit` is configured to launch `./<tools_dir>/backlogit[.exe]` with `["mcp", "--cwd", "<absRoot>"]`.
3. **Missing Operator Target Tool Directory Prompt (`internal/cli/root.go`, `internal/cli/self_install.go`)**: `newInitCommand` (`backlogit init` and `backlogit install`) does not prompt the operator with a default target install directory of `.bin`, `bin`, or allow entering an alternate workspace-relative directory option prior to creating the directory, installing `backlogit[.exe]`, and configuring `.mcp.json`.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Validate target tool directory paths, create the target tool directory (defaulting to `.bin`, or `bin` when `bin/` exists and `.bin/` does not) via step-by-step pinned `*os.Root` directory handles if it does not yet exist, install `backlogit[.exe]` there atomically, and record idempotent `/<tools_dir>/backlogit[.exe]` ignore rules in `.gitignore` and `.git/info/exclude` | `269D054D` / `103-DL` | Unit U1 | `TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries` |
| R2: Verify `<tools_dir>/backlogit[.exe]` exists in the target tool directory prior to configuring `.mcp.json`, atomically create or update `.mcp.json` (preserving existing non-`backlogit` `mcpServers` entries) to reference `./<tools_dir>/backlogit[.exe]` with `["mcp", "--cwd", "<absRoot>"]`, and persist `tools_dir` in `<storage_root>/config.yaml` | `269D054D` / `103-DL` | Unit U2 | `TestU2_ConfigureWorkspaceMCPJSONAndConfigToolsDirAfterBinaryInstall` |
| R3: Prompt the operator on `backlogit init` and `backlogit install` for the target tool install directory (offering `.bin`, `bin`, or a user-entered alternate workspace-relative directory, plus `BACKLOGIT_TOOLS_DIR` non-interactive override), create the chosen directory, install `backlogit[.exe]` there, configure `.mcp.json` and `config.yaml` `tools_dir` to use it, and verify end-to-end MCP daemon startup | `269D054D` / `103-DL` | Unit U3 | `TestU3_InitAndInstallPromptForTargetToolsDirAndStartMCPDaemon` |

## Implementation Units

### Unit U1: Create target tool directory, install `backlogit[.exe]` there, and update ignore rules in `internal/cli/self_install.go`

* **Domain**: `code` (width-isolated to `internal/cli` and `tests/integration` fixture alignment)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU1_*` in `package cli` directly references modified existing functions `installWorkspaceRuntimeExecutable` and `ensureWorkspaceRuntimeIgnored` to satisfy `redhonesty` `NoChangedBranch` and `Baseline == ResultFail`)
* **Affected files (3)**:
  * `internal/cli/self_install.go`
  * `internal/cli/self_install_test.go`
  * `tests/integration/multi_session_workspace_containment_test.go`
* **Changes needed**:
  1. In `internal/cli/self_install.go`:
     * Add `validateTargetToolsDirRel(raw string) (string, error)` to validate any target tool directory path: reject empty/whitespace-only or whitespace-padded strings, NUL bytes, absolute paths (`filepath.IsAbs`), volume names (`filepath.VolumeName`), drive/UNC prefixes (`hasDriveOrUNCPrefix`), `..` traversal segments, and `.` (workspace root); normalize optional leading `./` or `.\` via `filepath.Clean(filepath.FromSlash(strings.ReplaceAll(trimmed, "\\", "/")))`; require `filepath.IsLocal(clean) && clean != "."`; and reject reserved top-level segments `.git`, `.backlog`, and `.backlogit` case-insensitively (`isReservedDaemonDirSegment`).
     * Add `defaultTargetToolsDir(root, storageDir string) string`: open `wsRoot` on `absRoot`; if an existing valid `.mcp.json`, `<storageDir>/toolchain.yaml`, or `<storageDir>/config.yaml` already declares a valid tool directory, return it; otherwise if `.bin` exists as a non-redirected directory (`wsRoot.Lstat(".bin")`), return `".bin"`; else if `bin` exists as a non-redirected directory (`wsRoot.Lstat("bin")`), return `"bin"`; otherwise return `".bin"`.
     * Add `ensureAndOpenTargetToolsDir(wsRoot *os.Root, absRoot, toolsDirRel string) (*os.Root, string, error)`: split `toolsDirRel` into segments and walk `current := wsRoot`, `currentAbs := absRoot` (closing intermediate `*os.Root` handles via `defer`). For each segment `part`: set `nextAbs := filepath.Join(currentAbs, part)`; inspect `info, err := current.Lstat(part)`; if `errors.Is(err, os.ErrNotExist)`, call `current.Mkdir(part, 0o755)` and re-run `info, err = current.Lstat(part)`; verify `isLink, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)` is false, `info.Mode()&os.ModeIrregular == 0`, and `info.IsDir()`; open `next, err := current.OpenRoot(part)` and verify `opened, statErr := next.Stat(".")` with `os.SameFile(info, opened)`. Return the innermost pinned `toolsDirHandle` and `toolsDirAbs`.
     * Update `installWorkspaceRuntimeExecutable(root, storageDir string) (string, error)` so that in addition to installing `<storageDir>/backlogit[.exe]`, it resolves the active target tool directory (honoring a package-scoped or env-supplied target tool directory, defaulting to `defaultTargetToolsDir(root, storageDir)`), creates the target tool directory via `ensureAndOpenTargetToolsDir`, and installs `backlogit[.exe]` into `toolsDirHandle` using handle-relative temp file creation (`toolsDirHandle.OpenFile`), `Stat`/`Lstat`/`SameFile`, `io.Copy`, `Chmod(0o755)`, `Sync()`, `Close()`, `selfInstallBeforeBinaryReplaceHook`, destination `Lstat` re-verification, and `atomicfile.ReplaceTempFileInRoot(toolsDirHandle, toolsDirAbs, tmpName, binName, true)` (skipping self-overwrite when `os.SameFile(srcStat, dstInfo)`).
     * Update `ensureWorkspaceRuntimeIgnored(root, storageDir, binName string) error` to include `/<toolsDirSlash>/backlogit` and `/<toolsDirSlash>/backlogit.exe` alongside `workspaceRuntimeIgnoreEntries()` in `.gitignore` and `.git/info/exclude` idempotently.
  2. In `tests/integration/multi_session_workspace_containment_test.go`:
     * In `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`missing_local_binary_refuses_global_path_fallback`), remove the newly self-installed `.bin` directory (`require.NoError(t, os.RemoveAll(filepath.Join(root, ".bin")))`) immediately after `u12InitBacklogitWorkspace(t, root)` so the subtest continues to verify that a workspace whose local pinned binary is missing on disk fails `PrewarmMCPDaemon` with `corerrors.ErrDaemonBinaryMissing` without falling back to `PATH`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries` (in `package cli`, calling existing symbols `installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, `NewRootCommand`, and `selfInstallBeforeBinaryReplaceHook`; never naming result variables `want*`/`expect*`/`exp*`/`golden*`) testing: (a) on a fresh workspace where `.bin` does not exist, `installWorkspaceRuntimeExecutable` and `ensureWorkspaceRuntimeIgnored` (and `backlogit init` / `backlogit install`) create `.bin`, install `.bin/backlogit[.exe]` with executable permissions, keep `bin/backlogit[.exe]` absent, and write `/.bin/backlogit` and `/.bin/backlogit.exe` once to `.gitignore` and `.git/info/exclude`; (b) when `bin/` already exists in the workspace (and `.bin/` does not), default resolution selects `bin/`, installs `bin/backlogit[.exe]`, and records `/bin/backlogit` and `/bin/backlogit.exe` in `.gitignore` and `.git/info/exclude`; and (c) if the target tool directory (e.g., `.bin`) is a symlink/junction to an outside directory or swapped mid-flight via `selfInstallBeforeBinaryReplaceHook`, `installWorkspaceRuntimeExecutable` fails closed with `corerrors.ErrValidation` and never writes `backlogit[.exe]` outside the workspace root.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U2: Configure `.mcp.json` and `<storage_root>/config.yaml` (`tools_dir`) after installing `backlogit[.exe]` into the target tool directory

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `S` / `low` (2 files, <= 3 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU2_*` in `package cli` directly calls modified existing function `installWorkspaceRuntimeExecutable` and `selfInstallBeforeIgnoreReplaceHook`)
* **Dependencies**: `blocks` on Unit U1
* **Affected files (2)**:
  * `internal/cli/self_install.go`
  * `internal/cli/self_install_test.go`
* **Changes needed**:
  1. In `internal/cli/self_install.go`:
     * Add `configureWorkspaceMCPJSON(wsRoot *os.Root, absRoot, toolsDirRel, binName string) error`:
       - First, verify via pinned child `*os.Root` handles under `wsRoot` (`openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))`) that the target tool directory and `<toolsDirRel>/<binName>` already exist as a regular, non-redirected file before configuring `.mcp.json` (enforcing the `269D054D` ordering invariant that target tool directory creation and binary installation happen prior to `.mcp.json` configuration).
       - Inspect `.mcp.json` on `wsRoot`. If it exists, verify `!core.IsSymlinkOrReparsePoint(info, filepath.Join(absRoot, ".mcp.json"))` and `info.Mode().IsRegular()`, open via `wsRoot.OpenFile(".mcp.json", os.O_RDONLY, 0)`, verify `os.SameFile`, read with `io.LimitReader(rf, maxMCPDaemonInfoBytes+1)`, validate with `scanDaemonJSON(existingBytes)` (failing closed with `corerrors.ErrValidation` on malformed JSON or duplicate keys), and unmarshal into `map[string]any` to preserve all other top-level keys and non-`backlogit` `mcpServers` entries (`engram`, `kbdb`, `context7`, `tavily`, `github`, etc.).
       - Set `mcpServers["backlogit"]` with `"type": "stdio"`, `"command": "./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))`, and `"args": []string{"mcp", "--cwd", absRoot}` (preserving any additional fields on an existing `backlogit` server map such as `"tools"` while removing any case-folded duplicate `"backlogit"` key).
       - Marshal with `json.MarshalIndent(doc, "", "  ")` + `"\n"`, write to a handle-relative temp file on `wsRoot`, `Stat`/`Lstat`/`SameFile`, `Sync()`, `Chmod(mode)`, `Close()`, invoke `selfInstallBeforeIgnoreReplaceHook(".mcp.json")` if set, re-verify `wsRoot.Lstat(".mcp.json")` is not a symlink/reparse point or directory, and atomically publish via `atomicfile.ReplaceTempFileInRoot(wsRoot, absRoot, tmpName, ".mcp.json", false)`.
     * Add `persistWorkspaceConfigToolsDir(storageHandle *os.Root, storageDirAbs, toolsDirRel string) error`:
       - If `config.yaml` exists in `storageHandle`, verify `!core.IsSymlinkOrReparsePoint` and `IsRegular()`, read via `storageHandle.OpenFile("config.yaml", os.O_RDONLY, 0)` + `os.SameFile` + `io.LimitReader`, parse with `yaml.Node` (`validateDaemonYAMLKeys`), set or insert `tools_dir: <filepath.ToSlash(toolsDirRel)>` on the root mapping node while preserving all existing fields, and atomically publish via `atomicfile.ReplaceTempFileInRoot(storageHandle, storageDirAbs, tmpName, "config.yaml", false)`.
     * Invoke `configureWorkspaceMCPJSON` and `persistWorkspaceConfigToolsDir` at the end of `installWorkspaceRuntimeExecutable` after `<toolsDirRel>/<binName>` has been installed.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU2_ConfigureWorkspaceMCPJSONAndConfigToolsDirAfterBinaryInstall` (in `package cli`, calling existing symbols `installWorkspaceRuntimeExecutable`, `NewRootCommand`, and `selfInstallBeforeIgnoreReplaceHook`) testing: (a) `installWorkspaceRuntimeExecutable` (and `backlogit init` / `backlogit install`) creates `.mcp.json` with `mcpServers.backlogit.type == "stdio"`, `mcpServers.backlogit.command == "./.bin/backlogit[.exe]"` (or `"./bin/backlogit[.exe]"` when `bin/` is the resolved tool directory), and `mcpServers.backlogit.args == ["mcp", "--cwd", absRoot]`, and persists `tools_dir` in `<storage_root>/config.yaml` (verified via `config.Load`); (b) when `.mcp.json` already exists with other servers (`engram`, `github`) and a legacy `"./bin/backlogit.exe"` command, running `install` updates `mcpServers.backlogit.command` to the active tool directory while preserving `engram`, `github`, and any existing `tools` array on `backlogit`; and (c) a symlinked `.mcp.json`, a mid-flight `.mcp.json` symlink swap via `selfInstallBeforeIgnoreReplaceHook(".mcp.json")`, or an existing `.mcp.json` with duplicate JSON keys fails closed with `corerrors.ErrValidation`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU2_ConfigureWorkspaceMCPJSON|TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U3: Prompt for target tool directory (`.bin`, `bin`, or custom option) on `backlogit init` and `backlogit install` and verify end-to-end MCP daemon startup

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `M` / `medium` (3 files, <= 4 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU3_*` in `package cli` directly calls modified existing functions `newInitCommand` and `installWorkspaceRuntimeExecutable`)
* **Dependencies**: `blocks` on Unit U1 and Unit U2
* **Affected files (3)**:
  * `internal/cli/root.go`
  * `internal/cli/self_install.go`
  * `internal/cli/self_install_test.go`
* **Changes needed**:
  1. In `internal/cli/self_install.go`:
     * Add `resolveOrPromptTargetToolsDir(cmd *cobra.Command, root, storageDir string) (string, error)`:
       - Compute `defaultDir := defaultTargetToolsDir(root, storageDir)`.
       - If environment override `BACKLOGIT_TOOLS_DIR` is set (`os.LookupEnv("BACKLOGIT_TOOLS_DIR")`), validate it via `validateTargetToolsDirRel` (failing closed with `corerrors.ErrValidation` if blank or invalid) and return it without prompting.
       - When `cmd != nil` and input should prompt (`cmd.InOrStdin() != os.Stdin` or `isTerminal(cmd.InOrStdin()) && isTerminal(cmd.OutOrStdout())`):
         * Write the prompt to `cmd.OutOrStdout()`:
           `fmt.Fprintf(cmd.OutOrStdout(), "Target tool install directory [.bin, bin, or custom workspace-relative path] (default: %s): ", defaultDir)`
         * Read a line from `bufio.NewReader(cmd.InOrStdin())`.
         * Map operator input `choice := strings.TrimSpace(line)`:
           - `""` (empty / Enter) -> `defaultDir`
           - `"1"` or `".bin"` -> `".bin"`
           - `"2"` or `"bin"` -> `"bin"`
           - `"3"` -> write `"Enter custom workspace-relative tool directory: "` to `cmd.OutOrStdout()`, read the next line from the same `bufio.Reader`, and require non-empty input
           - Any other non-empty string -> use `choice` directly as the custom workspace-relative directory
         * Validate and normalize via `validateTargetToolsDirRel`.
       - Otherwise (non-interactive default `os.Stdin` when not a terminal), validate and return `defaultDir`.
     * Add `installWorkspaceRuntimeAndToolsWithDir(root, storageDir, toolsDirRel string) (string, error)` (and route `installWorkspaceRuntimeExecutable(root, storageDir)` through it with `defaultTargetToolsDir(root, storageDir)`) so `newInitCommand` can pass the prompted/resolved `toolsDirRel` directly into the target tool directory creation, binary installation, `.mcp.json` configuration, `config.yaml` `tools_dir` persistence, and `ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName)`.
  2. In `internal/cli/root.go` (`newInitCommand`):
     * Call `toolsDirRel, err := resolveOrPromptTargetToolsDir(cmd, root, storageDirName)` before creating/modifying target workspace files (so invalid prompted or `BACKLOGIT_TOOLS_DIR` input fails closed before mutating the workspace).
     * Call `binName, err := installWorkspaceRuntimeAndToolsWithDir(root, storageDirName, toolsDirRel)` and `ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU3_InitAndInstallPromptForTargetToolsDirAndStartMCPDaemon` (in `package cli`, calling existing symbols `newInitCommand`, `NewRootCommand`, `installWorkspaceRuntimeExecutable`, `PrewarmMCPDaemon`, `RunMCPSessionShim`, and `StopMCPDaemon`) testing: (a) interactive prompt selections on `backlogit init` and `backlogit install` via `cmd.SetIn`: pressing Enter (`"\n"`, selecting `.bin` by default or `bin` when `bin/` already exists), typing `"bin\n"` or `"2\n"`, typing `".bin\n"` or `"1\n"`, typing `"3\n.custom/tools\n"`, and typing `"custom-tools/sub\n"` directly (including with a relative `--cwd` path), verifying the prompt text is emitted to stdout, the chosen directory is created in the workspace, `backlogit[.exe]` is installed there prior to `.mcp.json` configuration, and `.mcp.json` + `<storage_root>/config.yaml` `tools_dir` + `.gitignore` / `.git/info/exclude` all record the chosen directory; (b) invalid prompted directories (`"../escape\n"`, `".backlog\n"`, `".backlogit\n"`, `".git\n"`, `"3\n\n"`) and invalid `BACKLOGIT_TOOLS_DIR` values fail closed with `corerrors.ErrValidation` before creating directories or writing `.mcp.json`; and (c) end-to-end daemon verification: initializing a workspace via `backlogit init` with a custom prompted tool directory (`".tools/mcp-bin\n"`) and `selfInstallExecutablePathFn` returning the built `u6BuildBinary(t)` binary allows `PrewarmMCPDaemon` and `RunMCPSessionShim` to start and serve MCP requests directly from the initialized `.tools/mcp-bin/backlogit[.exe]` binary.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3_InitAndInstallPrompt|TestU2_ConfigureWorkspaceMCPJSON|TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

## Dependency Graph

* **Unit U1** (`internal/cli/self_install.go`, `internal/cli/self_install_test.go`, `tests/integration/multi_session_workspace_containment_test.go`): no upstream dependencies (Wave 1)
* **Unit U2** (`internal/cli/self_install.go`, `internal/cli/self_install_test.go`): `blocks` on Unit U1 (Wave 2)
* **Unit U3** (`internal/cli/root.go`, `internal/cli/self_install.go`, `internal/cli/self_install_test.go`): `blocks` on Unit U1 and Unit U2 (Wave 3)

## Decisions and Rationale

1. **Prompt on `cmd.InOrStdin()` / `cmd.OutOrStdout()` when interactive or when a custom `io.Reader` is attached, and default to `.bin` / `bin` when non-interactive**:
   * Checking `cmd.InOrStdin() != os.Stdin || (isTerminal(cmd.InOrStdin()) && isTerminal(cmd.OutOrStdout()))` ensures human operators at a terminal are always prompted with `.bin`, `bin`, or custom directory options, and automated tests passing `cmd.SetIn(strings.NewReader(...))` exercise the exact prompt and input parser without requiring a pseudo-TTY, while existing non-interactive CLI callers (`cmd.SetArgs([]string{"init", "--cwd", root})` with default `os.Stdin`) never block on stdin and deterministically use `defaultTargetToolsDir` (`".bin"`, or `"bin"` if `bin/` already exists).
   * Supporting `BACKLOGIT_TOOLS_DIR` in `resolveOrPromptTargetToolsDir` provides non-interactive script/agent parity without altering the Cobra flag surface of `backlogit init` (avoiding `docs/cli-reference/backlogit_init.md` churn).
2. **Create the target tool directory and install `backlogit[.exe]` prior to configuring `.mcp.json`**:
   * Stash `269D054D` explicitly requires creating the target tool directory and installing `backlogit.exe` prior to configuring `.mcp.json`. `configureWorkspaceMCPJSON` enforces this invariant by verifying via `openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))` that the target binary already exists as a regular non-redirected file before writing `.mcp.json`.
3. **Preserve existing non-`backlogit` servers when updating `.mcp.json` and existing fields in `config.yaml`**:
   * Workspaces using GSD capability packs (`engram`, `kbdb`, `context7`, `tavily`, `github`) already contain other entries in `.mcp.json`. Validating existing `.mcp.json` with `scanDaemonJSON` and unmarshaling into `map[string]any` preserves all sibling `mcpServers` entries while updating `mcpServers.backlogit` (`type: "stdio"`, `command: "./<tools_dir>/<binName>"`, `args: ["mcp", "--cwd", absRoot]`).
4. **Preserve `<storage_root>/backlogit[.exe]` alongside `<tools_dir>/backlogit[.exe]`**:
   * Keeping `<storage_root>/backlogit[.exe]` in `installWorkspaceRuntimeExecutable` preserves `TestU6_InitAndInstallSelfInstallRuntimeExecutableAndIgnoreEntries` (`internal/cli/init_workspace_dir_test.go`) and `TestU6_SelfInstallMidFlightParentAndLeafSwapRejected` (`internal/cli/self_install_test.go`), while `<tools_dir>/backlogit[.exe]` and `.mcp.json` satisfy `269D054D` and `pinnedMCPDaemonBinary`.

## Risks and Caveats

* **Risk**: `redhonesty` (`internal/redhonesty/rules.go`) rejects a new test with `RejectNoChangedBranch` if the test's AST identifiers (`r.idents`, walked only across same-file helpers) do not intersect `in.Symbols` (`ChangedSymbols`), or with `RejectUnverifiableBaseline` if the test calls a symbol that does not exist at `baseSHA`, or with `RejectBuggyPathExpected` if a changed function's return value is assigned to a `want*`/`expect*`/`exp*`/`golden*` variable.
  * **Mitigation**: All three units place their new tests (`TestU1_*`, `TestU2_*`, `TestU3_*`) in `internal/cli/self_install_test.go` (`package cli`) and directly call pre-existing functions (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, `newInitCommand`) that are modified in that respective unit, without calling any newly introduced function identifier from the test file and without naming any result variable `want*`/`expect*`/`exp*`/`golden*`.
* **Risk**: Symlink, Windows junction (`mklink /J`), or mid-flight directory/leaf swap on the target tool directory or `.mcp.json`.
  * **Mitigation**: Walk and create every segment of `toolsDirRel` on pinned `*os.Root` handles (`ensureAndOpenTargetToolsDir`) passing the explicit canonical absolute path to `core.IsSymlinkOrReparsePoint`, and publish both `<toolsDirRel>/backlogit[.exe]` and `.mcp.json` via `atomicfile.ReplaceTempFileInRoot` with pre-replace `Lstat` re-verification and `selfInstallBeforeBinaryReplaceHook` / `selfInstallBeforeIgnoreReplaceHook` test hooks.
* **Risk**: `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`missing_local_binary_refuses_global_path_fallback` in `tests/integration/multi_session_workspace_containment_test.go`) calls `u12InitBacklogitWorkspace(t, root)` and expects `PrewarmMCPDaemon` to return `ErrDaemonBinaryMissing`.
  * **Mitigation**: In Unit U1, remove the newly self-installed `.bin` directory (`require.NoError(t, os.RemoveAll(filepath.Join(root, ".bin")))`) right after `u12InitBacklogitWorkspace(t, root)` in `missing_local_binary_refuses_global_path_fallback` so the local binary is genuinely missing on disk as the subtest name specifies.

## Constitution Check

* **Safety-First Go (`NON-NEGOTIABLE`)**: `pass` — All changes are in Go 1.24, wrap errors with `%w` preserving `corerrors.ErrValidation`, and use zero `unsafe` constructs.
* **Test-First Development (`NON-NEGOTIABLE`)**: `pass` — Every unit (`U1`, `U2`, `U3`) begins with a compiling, failing RED test harness (`TestU1_*`, `TestU2_*`, `TestU3_*`) verified against `redhonesty` invariants before any production edit.
* **Workspace Isolation and Security Boundaries (`NON-NEGOTIABLE`)**: `pass` — All directory creations, binary copies, `.mcp.json` writes, `config.yaml` updates, and ignore-file updates use `os.OpenRoot(absRoot)`, pinned child `*os.Root` handles, `core.IsSymlinkOrReparsePoint`, `os.SameFile`, and `atomicfile.ReplaceTempFileInRoot`.
* **CLI Workspace Containment (`NON-NEGOTIABLE`)**: `pass` — All `init` and `install` operations resolve and write strictly inside the target workspace root (`--cwd` or `[path]`).
* **Destructive Command Approval (`NON-NEGOTIABLE`)**: `pass` — No destructive commands or schema removals are introduced; existing `.mcp.json` and `config.yaml` entries are preserved.
* **Merge Commit History Preservation (`NON-NEGOTIABLE`)**: `pass` — The shipment will merge via a merge commit (`--merge`) under P-009/P-014.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Extends `backlogit init` and `backlogit install` to prompt for a target tool directory (`.bin`, `bin`, or custom), create the directory, install `backlogit[.exe]` there, and configure `.mcp.json` and `<storage_root>/config.yaml` (`tools_dir`).
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Creates workspace directories, installs an executable binary (`0o755`), and writes `.mcp.json` (which controls MCP server command execution) inside the target workspace.
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — Additive, idempotent provisioning that preserves existing non-`backlogit` `.mcp.json` servers and `config.yaml` settings.
* **external integration, operator checkpoint, or external dependency**: `absent` — Uses standard library `bufio`, `os.Root`, and existing `internal/atomicfile` / `gopkg.in/yaml.v3` packages (`go.mod` unchanged).
* **high runtime, rollout, or rollback risk**: `present` — Modifies `backlogit init` and `backlogit install` workspace bootstrap and `.mcp.json` generation consumed by `backlogit mcp` daemon startup and `ValidateMCPConfigPinning`.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * `backlogit init [path]` and `backlogit install [path]` (`internal/cli/root.go`, `internal/cli/self_install.go`)
  * Workspace `.mcp.json` and `<storage_root>/config.yaml` (`tools_dir`) generation consumed by `backlogit mcp` (`PrewarmMCPDaemon`, `RunMCPSessionShim`) and `backlogit mcp status --toolchain`
* **Runtime verification probes (for `_Ship` Step 6)**:
  1. In an isolated `--cwd <tempDir>` workspace, pipe `"\n"` (default `.bin`), `"bin\n"`, and `".custom-tools/bin\n"` into `backlogit init --cwd <tempDir>` / `backlogit install --cwd <tempDir>`, verifying that the prompt is displayed, the target tool directory is created, `backlogit[.exe]` is installed into that directory prior to `.mcp.json` configuration, and `.mcp.json` + `<storage_root>/config.yaml` + `.gitignore` are configured accurately.
  2. Run `backlogit --cwd <tempDir> mcp prewarm --format json` and `backlogit --cwd <tempDir> mcp stop --format json` against the initialized workspace, confirming the MCP daemon launches from the newly installed `<tools_dir>/backlogit[.exe]` configured in `.mcp.json`.
* **Operational closure expectations**:
  * **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
  * **Validation window**: Immediate post-merge CLI/MCP verification on `main`.
  * **Healthy signals**: `backlogit init` and `backlogit install` provision `<tools_dir>/backlogit[.exe]` and `.mcp.json` cleanly; `PrewarmMCPDaemon` starts without `ErrDaemonBinaryMissing`; `go test -timeout=30m ./...` and `make docs-lint` pass with zero violations.
  * **Rollback trigger & procedure**: Any regression in `backlogit init`/`install` or `.mcp.json` provisioning triggers a reviewed `git revert -m 1 <merge_sha>` PR.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by CLI contract, filesystem executable/config write containment, and MCP bootstrap runtime signals across `internal/cli/root.go` and `internal/cli/self_install.go`.
* **Protected invariants**:
  * **I1 (Handle-pinned segment-by-segment directory creation and binary installation)**: `ensureAndOpenTargetToolsDir` and `installWorkspaceToolDirectoryExecutable` MUST operate relative to `os.OpenRoot(absRoot)` by walking each segment of `toolsDirRel` on pinned child `*os.Root` directory handles (`Lstat` -> `Mkdir(part, 0o755)` if `os.ErrNotExist` -> `core.IsSymlinkOrReparsePoint(info, nextAbs)` using the explicit canonical `nextAbs` path -> `OpenRoot(part)` -> `Stat(".")` + `os.SameFile`), creating the temp executable on the innermost `toolsDirHandle`, applying `tmpFile.Chmod(0o755)` and `tmpFile.Sync()` on the open file handle, and publishing via `atomicfile.ReplaceTempFileInRoot(toolsDirHandle, toolsDirAbs, tmpName, binName, true)`.
  * **I2 (Strict ordering — target tool directory creation and binary install prior to `.mcp.json` configuration)**: `configureWorkspaceMCPJSON` MUST verify via `openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))` that the target tool directory and `<toolsDirRel>/<binName>` already exist as a regular, non-redirected file before writing or replacing `.mcp.json`.
  * **I3 (Non-destructive `.mcp.json` and `config.yaml` merge with duplicate-key rejection)**: When `.mcp.json` or `<storage_root>/config.yaml` already exists in the target workspace, `configureWorkspaceMCPJSON` and `persistWorkspaceConfigToolsDir` MUST reject symlinks/reparse points and duplicate keys (`scanDaemonJSON` / `validateDaemonYAMLKeys`), preserve all non-`backlogit` `mcpServers` entries and existing `config.yaml` settings, and publish updates via `atomicfile.ReplaceTempFileInRoot`.
  * **I4 (`redhonesty` P-002.1 compliance across all three units)**: Every new test function (`TestU1_*`, `TestU2_*`, `TestU3_*`) in `internal/cli/self_install_test.go` (`package cli`) MUST call only symbols that already exist at that unit's `baseSHA` (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, `newInitCommand`, `NewRootCommand`, `selfInstallBeforeBinaryReplaceHook`, `selfInstallBeforeIgnoreReplaceHook`) and MUST directly reference at least one existing function modified in that unit (`installWorkspaceRuntimeExecutable` / `ensureWorkspaceRuntimeIgnored` in U1 and U2; `newInitCommand` / `installWorkspaceRuntimeExecutable` in U3), without assigning any changed function's output to a `want*`/`expect*`/`exp*`/`golden*` variable.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/go-mcp-server.instructions.md`
  * `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`
  * `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`
  * `docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md`
  * `docs/compound/2026-10-16-root-confined-access-and-graphql-reviewer-union.md`
  * `docs/compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`
  * `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`
* **Risky actions classification (`strict-safety`)**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Create target tool directory (`.bin`, `bin`, or custom relative path) and self-install `backlogit[.exe]` | `internal/cli/self_install.go`, `tests/integration/multi_session_workspace_containment_test.go` | `moderate` (filesystem directory & executable write during `init`/`install`) | Validate relative path (`filepath.IsLocal`, forbid `.git`/`.backlog`/`.backlogit`); walk pinned `*os.Root` handles; publish via `atomicfile.ReplaceTempFileInRoot` | Target tool directory and `backlogit[.exe]` are created safely inside the workspace root; symlink/junction escapes fail closed |
| Create or update `.mcp.json` and `<storage_root>/config.yaml` (`tools_dir`) after binary install | `internal/cli/self_install.go` | `moderate` (MCP launch configuration write) | Verify `<tools_dir>/backlogit[.exe]` exists first; validate existing JSON/YAML for duplicate keys; preserve sibling `mcpServers` entries; publish via `ReplaceTempFileInRoot` | `.mcp.json` and `config.yaml` point to `./<tools_dir>/backlogit[.exe]` with `--cwd <absRoot>` while preserving existing servers |
| Prompt operator for target tool directory on `backlogit init` and `backlogit install` | `internal/cli/root.go`, `internal/cli/self_install.go` | `low` (CLI interactive prompt & non-interactive fallback) | Prompt when `cmd.InOrStdin() != os.Stdin` or terminal; default to `.bin` (or existing `bin`) when non-interactive; validate before mutating workspace | Operator can choose `.bin`, `bin`, or custom directory interactively, while non-interactive scripts and tests run deterministically without hanging |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode: multi-agent-dispatch` (or `single-agent-declared-degradation` with `TOOL_DEGRADED: reviewer-subagent-dispatch` if any subagent fails to return), cover all 7 required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`), and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) before harvest.
* **Unresolved operator decisions**: None.

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale**: Executed a complete sequential rubric pass across all 7 required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) under `single-agent-declared-degradation` (`[PERSONA-PASS]` / `[PERSONA-DONE]` for all 7 personas). Plan hardening was required (`Requires plan hardening: yes`) and is satisfied by the `## Plan Hardening` section (Invariants I1–I4 and `ProposedAction` / `ActionRisk` table). `## Constitution Check` is present and concludes with literal `Constitution Check: pass`. Total merged findings across all 7 personas: `0` (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Persona evaluation summary**:
  * **Constitution Reviewer**: `0` findings (`PASS`). Units U1–U3 comply with Principles I–XI and Task Granularity (<= 3 files, <= 4 functions, <= 3 test scenarios per unit; pure Go width isolation). `## Constitution Check` concludes with literal `Constitution Check: pass`.
  * **Go Reviewer**: `0` findings (`PASS`). Existing function signatures (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, `newInitCommand`) remain backward-compatible, all `*os.Root` and `*os.File` handles are closed before return, and error paths wrap `corerrors.ErrValidation` with `%w`.
  * **Scope Boundary Auditor**: `0` findings (`PASS`). Units U1–U3 map 1:1 to `R1`–`R3` (`269D054D` / `103-DL`), and unrelated stash entries remain untouched.
  * **Learnings Researcher**: `0` findings (`PASS`, `confidence: high`). Plan incorporates `2026-10-27-pin-the-handle-not-the-path.md`, `nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`, `derive-daemon-version-from-pinned-image-2026-11-09.md`, `2026-10-16-root-confined-access-and-graphql-reviewer-union.md`, `2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`, and `2026-08-29-windows-preremove-rename-pattern.md`.
  * **Architecture Strategist**: `0` findings (`PASS`). Cohesion within `internal/cli` (`root.go`, `self_install.go`, `self_install_test.go`) and sequential wave ordering (`U1 -> U2 -> U3`) eliminate cross-wave file conflicts and preserve package boundaries.
  * **Agent-Native Parity Reviewer**: `0` findings (`PASS`). Interactive operators are prompted for `.bin`, `bin`, or a custom directory on `cmd.InOrStdin()` / `cmd.OutOrStdout()`, while non-interactive scripts, tests, and agents can supply piped stdin, `BACKLOGIT_TOOLS_DIR`, or use the deterministic `.bin`/`bin` default.
  * **Security Lens Reviewer**: `0` findings (`PASS`). 3-threat model verified: (1) traversal/reserved/absolute `tools_dir` inputs are rejected by `validateTargetToolsDirRel` before any filesystem mutation; (2) symlink/junction swaps on the target tool directory or `.mcp.json` are blocked by pinned child `*os.Root` handles + `core.IsSymlinkOrReparsePoint` + `os.SameFile` + `atomicfile.ReplaceTempFileInRoot`; (3) duplicate JSON/YAML keys in existing `.mcp.json` or `config.yaml` fail closed via `scanDaemonJSON` and `validateDaemonYAMLKeys`.
* **Runtime verification & operational closure check**: Verified complete in `## Runtime Verification and Closure`.

