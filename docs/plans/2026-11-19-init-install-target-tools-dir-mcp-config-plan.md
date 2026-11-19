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

1. **`internal/cli/self_install.go` (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`)**: `init` and `install` do not create a target tool directory (`.bin` default, `bin`, or custom workspace-relative path) via pinned `*os.Root` directory handles, do not install `backlogit[.exe]` into that target tool directory, and do not add `/<tools_dir>/backlogit` and `/<tools_dir>/backlogit.exe` ignore entries to `.gitignore` and `.git/info/exclude` (which is required so installing `backlogit[.exe]` into a newly created target tool directory does not dirty the operator's git working tree). Moreover, `installWorkspaceRuntimeExecutable` currently returns early when `os.SameFile(srcStat, dstInfo)` is true, which would skip downstream `.mcp.json` and ignore-file provisioning when an already-installed workspace binary is re-invoked.
2. **Missing `.mcp.json` Provisioning and `toolchain.yaml` Reconciliation (`internal/cli/self_install.go`)**: After creating the target tool directory and installing `backlogit[.exe]` into it, `init` and `install` do not create or merge `.mcp.json` in the target workspace (preserving unrelated top-level keys and non-`backlogit` servers, detecting `backlogit` server aliases across `mcpServers` and `servers`, and reconciling `<storage_root>/toolchain.yaml` if present).
3. **Missing Operator Target Tool Directory Prompt and `--tools-dir` Flag (`internal/cli/root.go`)**: `newInitCommand` (`backlogit init` and `backlogit install`) does not accept `--tools-dir` (or `BACKLOGIT_TOOLS_DIR`), does not prompt the operator on interactive TTY or piped subprocess stdin with a default target install directory of `.bin`, `bin`, or allow entering an alternate workspace-relative directory option, and `docs/cli-reference/backlogit_init.md` must be regenerated via `cmd/gen-docs` to reflect `--tools-dir`.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Reuse canonical `validateDaemonToolsDirRel`, create missing target tool directory segments via step-by-step pinned `*os.Root` directory handles (`ensureAndOpenTargetToolsDir`), install `backlogit[.exe]` there atomically without letting `os.SameFile` short-circuit downstream steps, and record idempotent `/<tools_dir>/backlogit[.exe]` ignore rules in `.gitignore` and `.git/info/exclude` | `269D054D` / `103-DL` | Unit U1 (`218.001-T`) | `TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries` |
| R2: Verify `<tools_dir>/backlogit[.exe]` exists prior to configuring `.mcp.json`, atomically create or merge `.mcp.json` via `map[string]json.RawMessage` (preserving unrelated top-level/server values, detecting aliases across `mcpServers` and `servers`, and refusing ambiguous multi-entry or wrong-type configs), and reconcile existing `<storage_root>/toolchain.yaml` when present (without mutating `config.yaml`) | `269D054D` / `103-DL` | Unit U2 (`218.002-T`) | `TestU2_ConfigureWorkspaceMCPJSONAndReconcileToolchainAfterInstall` |
| R3: Add `--tools-dir` flag, `BACKLOGIT_TOOLS_DIR` override, fail-closed `defaultTargetToolsDir(root, storageDir) (string, error)` (handling nonexistent first-init roots safely), and `isTerminalReader` / piped-subprocess-aware prompting on `backlogit init` and `backlogit install` (`internal/cli/root.go`) | `269D054D` / `103-DL` | Unit U3 (`218.003-T`) | `TestU3_InitAndInstallPromptAndToolsDirFlag` |
| R4: Enforce `validateDaemonYAMLKeys` on `config.yaml`, inspect both `mcpServers` and `servers` up to `maxMCPConfigFileBytes` (`1 MiB`) in `resolveMCPJSONDaemonBinaryRel` (`internal/cli/mcp_daemon.go`), align `TestU12` missing-binary fixture after `u12InitBacklogitWorkspace`, and verify end-to-end `PrewarmMCPDaemon` startup from `init`/`install`-provisioned workspaces | `269D054D` / `103-DL` | Unit U4 (`218.004-T`) | `TestU4_InitAndInstallProvisionedWorkspaceStartsDaemonAndRejectsAmbiguousConfigs` |
| R5: Regenerate `docs/cli-reference/backlogit_init.md` via `go run ./cmd/gen-docs docs/cli-reference` for `--tools-dir` and verify zero reference drift in `cmd/gen-docs/main_test.go` | `269D054D` / `103-DL` | Unit U5 (`218.005-T`) | `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` |

## Implementation Units

### Unit U1: Validate and create target tool directory, install `backlogit[.exe]`, and update ignore rules (`218.001-T`)

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `M` / `medium` (2 files, 3 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU1_*` in `package cli` directly calls pre-existing modified functions `installWorkspaceRuntimeExecutable` and `ensureWorkspaceRuntimeIgnored`)
* **Affected files (2)**:
  * `internal/cli/self_install.go`
  * `internal/cli/self_install_test.go`
* **Changes needed**:
  1. In `internal/cli/self_install.go` (3 functions added/updated):
     * Add `ensureAndOpenTargetToolsDir(wsRoot *os.Root, absRoot, targetDir string) (*os.Root, string, bool, error)`:
       - Check whether `targetDir` is a storage root candidate (`slices.Contains(core.WorkspaceRootCandidates(), targetDir)`). If not a storage root, validate `targetDir` via the canonical `validateDaemonToolsDirRel(targetDir)` helper from `internal/cli/mcp_daemon.go` (rejecting blank/whitespace-padded strings, NUL bytes, volume/UNC/absolute prefixes, `..` traversal, `.`, and reserved top-level segments `.git`, `.backlog`, `.backlogit`).
       - Walk each segment `part` of `cleanRel` on pinned `*os.Root` handles (`current := wsRoot`, `currentAbs := absRoot`, closing intermediate handles via `defer`): set `nextAbs := filepath.Join(currentAbs, part)`; inspect `info, err := current.Lstat(part)`; if `errors.Is(err, os.ErrNotExist)` (for non-storage target tool directories), call `current.Mkdir(part, 0o755)` and re-run `info, err = current.Lstat(part)`; verify `isLink, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)` is false, `info.Mode()&os.ModeIrregular == 0`, and `info.IsDir()`; open `next, err := current.OpenRoot(part)` and verify `opened, statErr := next.Stat(".")` with `os.SameFile(info, opened)`. Return `(innermostHandle, currentAbs, isStorageRoot, nil)`.
     * Update `installWorkspaceRuntimeExecutable(root, targetDir string) (string, error)`:
       - Use `ensureAndOpenTargetToolsDir(wsRoot, absRoot, targetDir)` so `targetDir` can be either a storage root (`.backlog` / `.backlogit`) or a single/multi-segment target tool directory (`.bin`, `bin`, `.tools/bin`, `./custom-bin`).
       - When `dstInfo` already exists and `os.SameFile(srcStat, dstInfo)` is true, skip **only** the temp-file copy/rename block rather than returning early before post-copy provisioning hooks (resolving Review Fix 4).
     * Update `ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName string) error`:
       - Keep the exact `(string, string, string) error` signature; when `toolsDirRel != ""` and `!slices.Contains(core.WorkspaceRootCandidates(), toolsDirRel)`, validate `cleanToolsDir, err := validateDaemonToolsDirRel(toolsDirRel)` and append `"/" + filepath.ToSlash(cleanToolsDir) + "/backlogit"` and `"/" + filepath.ToSlash(cleanToolsDir) + "/backlogit.exe"` to `workspaceRuntimeIgnoreEntries()` before updating `.gitignore` and `.git/info/exclude`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries` (in `package cli`, calling only pre-existing symbols `installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, and `selfInstallBeforeBinaryReplaceHook`; never naming result variables `want*`/`expect*`/`exp*`/`golden*`) testing 3 scenarios: (a) calling `installWorkspaceRuntimeExecutable(root, ".bin")`, `installWorkspaceRuntimeExecutable(root, "bin")`, and `installWorkspaceRuntimeExecutable(root, ".tools/custom")` creates the missing single/multi-segment directory on `wsRoot`, installs `backlogit[.exe]` with executable permissions (`mode & 0o111 != 0` on POSIX), and calling `ensureWorkspaceRuntimeIgnored(root, ".tools/custom", binName)` records `/.tools/custom/backlogit` and `/.tools/custom/backlogit.exe` once in `.gitignore` and `.git/info/exclude`; (b) invoking `installWorkspaceRuntimeExecutable` when `selfInstallExecutablePathFn` points to the already-installed `<targetDir>/backlogit[.exe]` (`os.SameFile`) succeeds without error or truncation; and (c) invalid target tool directories (`"../escape"`, `".git"`, `"."`, `"   "`, symlink/junction segments, or mid-flight swaps via `selfInstallBeforeBinaryReplaceHook`) fail closed with `corerrors.ErrValidation` without writing outside the workspace root.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U2: Configure `.mcp.json` and reconcile `toolchain.yaml` after tool binary install (`218.002-T`)

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `M` / `medium` (2 files, 3 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU2_*` in `package cli` directly calls pre-existing modified function `installWorkspaceRuntimeExecutable` and `selfInstallBeforeIgnoreReplaceHook`)
* **Dependencies**: `blocks` on Unit U1 (`218.001-T`)
* **Affected files (2)**:
  * `internal/cli/self_install.go`
  * `internal/cli/self_install_test.go`
* **Changes needed**:
  1. In `internal/cli/self_install.go` (3 functions added/updated):
     * Add `configureWorkspaceMCPJSON(wsRoot *os.Root, absRoot, toolsDirRel, binName string) error`:
       - Verify via `openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))` that the target tool directory and `<toolsDirRel>/<binName>` already exist as a regular, non-redirected file prior to configuring `.mcp.json`.
       - Read existing `.mcp.json` (if present) via `readLocalFileInRoot(wsRoot, absRoot, ".mcp.json", maxMCPConfigFileBytes)` where `maxMCPConfigFileBytes = 1 << 20` (`1 MiB`), and validate with `scanDaemonJSON(existingBytes)` (rejecting duplicate keys and malformed JSON fail-closed with `corerrors.ErrValidation`).
       - Unmarshal top-level JSON into `map[string]json.RawMessage` (rejecting non-object top-level JSON such as arrays, primitives, or `null` with `corerrors.ErrValidation`) so all unrelated top-level keys survive verbatim.
       - Inspect both `"mcpServers"` and `"servers"` in the top-level map: if either is present with a non-object JSON value (e.g. array/string/number/`null`), fail closed with `corerrors.ErrValidation`. Across all server entries in `"mcpServers"` and `"servers"`, unmarshal each server entry into `map[string]json.RawMessage` (rejecting non-object server entries or non-string `"command"`/`"type"`/`"url"` or non-string-array `"args"` with `corerrors.ErrValidation`) and identify entries matching `strings.EqualFold(strings.TrimSpace(name), "backlogit") || isBacklogitDaemonCommand(cmdStr)`.
       - If `len(matched) > 1` across `"mcpServers"` and `"servers"`, fail closed with `corerrors.ErrValidation` (`"ambiguous multiple backlogit entries in .mcp.json"`).
       - If `len(matched) == 1`, update that matched entry in its owning map (`"mcpServers"` or `"servers"`, preserving custom alias keys such as `"backlog-alias"` and preserving any unrelated fields on that server object such as `"tools"` or `"env"` while setting `"type": "stdio"`, `"command": "./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))`, and `"args": ["mcp", "--cwd", absRoot]`). If `len(matched) == 0`, insert `"backlogit"` into `"mcpServers"`.
       - Write formatted JSON (`json.MarshalIndent` + `"\n"`) to a handle-relative temp file on `wsRoot`, `Stat`/`Lstat`/`SameFile`, `Sync()`, `Chmod(mode)`, `Close()`, invoke `selfInstallBeforeIgnoreReplaceHook(".mcp.json")` if set, re-verify `wsRoot.Lstat(".mcp.json")` is not a symlink/reparse point or directory, and atomically publish via `atomicfile.ReplaceTempFileInRoot(wsRoot, absRoot, tmpName, ".mcp.json", false)`.
     * Add `reconcileWorkspaceToolchainYAML(wsRoot *os.Root, absRoot, toolsDirRel, binName string) error`:
       - Resolve existing storage root (`resolveDaemonStorageDirName(wsRoot, absRoot)`). If `<storageDir>/toolchain.yaml` exists on disk, read and validate it (`validateDaemonYAMLKeys`), update `tools_dir` (if present) and any `backlogit` tool entry's `executable_path` (and `sha256` if non-empty, computed from the installed `<toolsDirRel>/<binName>` file) to `"./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))`, and atomically publish `<storageDir>/toolchain.yaml` on the pinned storage handle via `atomicfile.ReplaceTempFileInRoot`. Do **not** create `toolchain.yaml` if absent and do **not** mutate `config.yaml`.
     * Update `installWorkspaceRuntimeExecutable(root, targetDir string) (string, error)` so whenever `targetDir` is a target tool directory (`!isStorageRoot`), after ensuring `<targetDir>/<binName>` is in place (even when `os.SameFile` skipped copying!), it invokes `configureWorkspaceMCPJSON(wsRoot, absRoot, cleanToolsDir, binName)` and `reconcileWorkspaceToolchainYAML(wsRoot, absRoot, cleanToolsDir, binName)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU2_ConfigureWorkspaceMCPJSONAndReconcileToolchainAfterInstall` (in `package cli`, calling only pre-existing symbols `installWorkspaceRuntimeExecutable` and `selfInstallBeforeIgnoreReplaceHook`) testing 3 scenarios: (a) calling `installWorkspaceRuntimeExecutable(root, ".bin")` and `installWorkspaceRuntimeExecutable(root, ".tools/bin")` (including when `selfInstallExecutablePathFn` already points to the installed target binary so `os.SameFile` is true) creates `.mcp.json` with `mcpServers.backlogit` (`type: "stdio"`, `command: "./<tools_dir>/<binName>"`, `args: ["mcp", "--cwd", absRoot]`) and leaves `<storage_root>/config.yaml` unmutated; (b) when `.mcp.json` already contains large (`>64 KiB`) unrelated top-level/server JSON values, an alias entry `"backlog-alias"` with `command: "./bin/backlogit.exe"`, or a `"servers"` map, `installWorkspaceRuntimeExecutable` preserves all unrelated JSON values and updates the alias entry in place, and when `<storage_root>/toolchain.yaml` exists, reconciles its `backlogit` `executable_path`/`tools_dir` and `sha256` so `toolchain.AuditRepositoryToolchain` passes; and (c) ambiguous multiple `backlogit` entries (`"backlogit"` + `"backlog-alias"`), wrong-typed JSON fields (`"mcpServers": []` or `"command": 123`), duplicate JSON keys, or symlinked/swapped `.mcp.json` fail closed with `corerrors.ErrValidation`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU2_ConfigureWorkspaceMCPJSON|TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U3: Add target tool directory prompt, `--tools-dir` flag, and fail-closed default discovery to `init` and `install` (`218.003-T`)

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `M` / `medium` (2 files, 4 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU3_*` in `package cli` directly calls pre-existing modified function `newInitCommand` and `NewRootCommand`)
* **Dependencies**: `blocks` on Unit U1 (`218.001-T`) and Unit U2 (`218.002-T`)
* **Affected files (2)**:
  * `internal/cli/root.go`
  * `internal/cli/self_install_test.go`
* **Changes needed**:
  1. In `internal/cli/root.go` (4 functions added/updated):
     * Add `isTerminalReader(r io.Reader) bool` and `stdinShouldPrompt(cmd *cobra.Command) (bool, error)`:
       - `isTerminalReader(r)` checks `f, ok := r.(*os.File); return ok && term.IsTerminal(int(f.Fd()))` (compiling cleanly with `golang.org/x/term` and `io.Reader`).
       - `stdinShouldPrompt(cmd)` returns `true` when: (1) `isTerminalReader(in) && isTerminal(out)` (interactive TTY), OR (2) `in != os.Stdin` (`cmd.SetIn` reader in tests), OR (3) `in == os.Stdin` and `fi, err := os.Stdin.Stat()` succeeds with `fi.Mode()&os.ModeNamedPipe != 0 || (fi.Mode().IsRegular() && fi.Size() > 0)` (real piped or file-redirected subprocess stdin, while `os.DevNull` in `go test` has `ModeCharDevice != 0` and `!term.IsTerminal`, returning `false`).
     * Add `defaultTargetToolsDir(cleanRoot string) (string, error)`:
       - Verify the nearest existing ancestor of `cleanRoot` is not a symlink or reparse point (`core.IsSymlinkOrReparsePoint`). If `cleanRoot` itself does not yet exist on disk (`os.IsNotExist` on `os.Lstat(cleanRoot)`), return `(".bin", nil)` for safe first-init on a nonexistent workspace directory.
       - If `cleanRoot` exists, verify `!core.IsSymlinkOrReparsePoint` and `IsDir()`, open `wsRoot, err := os.OpenRoot(cleanRoot)` + `os.SameFile`, and call `rel, found, err := resolveConfiguredDaemonBinaryRel(wsRoot, cleanRoot)`. If `err != nil`, return `("", err)` fail-closed with **no silent fallback**. If `found`, return `(filepath.ToSlash(filepath.Dir(rel)), nil)`.
       - Inspect `.bin` on `wsRoot`: if `info, err := wsRoot.Lstat(".bin")` succeeds, verify `!core.IsSymlinkOrReparsePoint(info, filepath.Join(cleanRoot, ".bin"))` and `info.IsDir()` (returning `corerrors.ErrValidation` if `.bin` is a symlink/junction or regular file), and return `(".bin", nil)`.
       - Inspect `bin` on `wsRoot`: if `info, err := wsRoot.Lstat("bin")` succeeds, verify `!core.IsSymlinkOrReparsePoint(info, filepath.Join(cleanRoot, "bin"))` and `info.IsDir()` (returning `corerrors.ErrValidation` if `bin` is a symlink/junction or regular file), and return `("bin", nil)`.
       - Otherwise return `(".bin", nil)`.
     * Add `resolveInitTargetToolsDir(cmd *cobra.Command, cleanRoot, toolsDirFlag string) (string, error)`:
       - If `cmd != nil && cmd.Flags().Changed("tools-dir")`, validate `toolsDirFlag` via `validateDaemonToolsDirRel(strings.TrimPrefix(strings.TrimPrefix(toolsDirFlag, "./"), ".\\"))` (rejecting blank or whitespace-padded `toolsDirFlag` first) and return it without prompting.
       - If `envDir, ok := os.LookupEnv("BACKLOGIT_TOOLS_DIR"); ok`, validate `envDir` via `validateDaemonToolsDirRel` (failing closed if blank or invalid) and return it without prompting.
       - Compute `defaultDir, err := defaultTargetToolsDir(cleanRoot)`. If `err != nil`, return `("", err)` immediately.
       - Check `shouldPrompt, err := stdinShouldPrompt(cmd)`. If `shouldPrompt`: write `"Target tool install directory [.bin, bin, or custom workspace-relative path] (default: %s): "` to `cmd.OutOrStdout()`, read a line via `bufio.NewReader(cmd.InOrStdin()).ReadString('\n')` (treating `io.EOF` with empty line as `""`), map `""` -> `defaultDir`, `"1"` or `".bin"` -> `".bin"`, `"2"` or `"bin"` -> `"bin"`, `"3"` -> prompt `"Enter custom workspace-relative tool directory: "` and read next line (failing with `corerrors.ErrValidation` if empty), or any other non-empty string (stripping optional leading `./` or `.\` when followed by a relative path) -> custom directory, and validate via `validateDaemonToolsDirRel`.
       - Return `validateDaemonToolsDirRel(defaultDir)`.
     * Update `newInitCommand(cwd *string) *cobra.Command` in `internal/cli/root.go`:
       - Add `--tools-dir` flag (`cmd.Flags().StringVar(&toolsDirFlag, "tools-dir", "", "workspace-relative target tool directory for backlogit binary and .mcp.json (default .bin or bin)")`).
       - Call `toolsDirRel, err := resolveInitTargetToolsDir(cmd, cleanRoot, toolsDirFlag)` **before** `os.MkdirAll(dir, 0o755)` so any invalid flag, env var, prompt input, or corrupt existing config fails closed before creating or modifying any workspace directory or file.
       - After `installWorkspaceRuntimeExecutable(root, storageDirName)`, call `if _, err := installWorkspaceRuntimeExecutable(root, toolsDirRel); err != nil { return fmt.Errorf("install workspace tool executable: %w", err) }` and pass `toolsDirRel` to `ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU3_InitAndInstallPromptAndToolsDirFlag` (in `package cli`, calling pre-existing symbols `newInitCommand` and `NewRootCommand`) testing 3 scenarios: (a) interactive/piped prompt selections (`"\n"` default `.bin` and default `bin` when `bin/` exists, `"1\n"`, `"2\n"`, `"3\n.custom/tools\n"`, direct `"custom-tools/sub\n"`, and a brand-new nonexistent workspace root path `<tempDir>/new-ws` plus a relative `--cwd` path) and real piped subprocess execution (`exec.CommandContext` with `Stdin: strings.NewReader(".piped-tools/bin\n")` on a built binary) create the target tool directory, install `backlogit[.exe]` there prior to `.mcp.json`, configure `.mcp.json`, and update `.gitignore`/`.git/info/exclude`; (b) `--tools-dir` flag (`--tools-dir .bin`, `--tools-dir bin`, `--tools-dir .flag-tools/bin`) and `BACKLOGIT_TOOLS_DIR` provision the specified directory without prompting; and (c) invalid flag/env/prompt inputs (`--tools-dir ""`, `--tools-dir "../escape"`, `".backlog\n"`, `"3\n\n"`) and corrupt/wrong-type/redirected existing `.mcp.json`, `toolchain.yaml`, `config.yaml`, or `.bin` file fail closed with `corerrors.ErrValidation` and zero workspace mutations.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3_InitAndInstallPrompt|TestU2_ConfigureWorkspaceMCPJSON|TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U4: Harden daemon config resolution, verify end-to-end daemon startup after `init`/`install`, and align `TestU12` fixture (`218.004-T`)

* **Domain**: `code` (width-isolated to `internal/cli` and `tests/integration`)
* **Size / Complexity**: `S` / `low` (2 files, 3 functions, 2 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU4_*` in `tests/integration/multi_session_workspace_containment_test.go` calls `cli.PrewarmMCPDaemon`, which is modified in `internal/cli/mcp_daemon.go` so `"PrewarmMCPDaemon"` is in `in.Symbols.Funcs` and `r.idents`)
* **Dependencies**: `blocks` on Unit U3 (`218.003-T`)
* **Affected files (2)**:
  * `internal/cli/mcp_daemon.go`
  * `tests/integration/multi_session_workspace_containment_test.go`
* **Changes needed**:
  1. In `internal/cli/mcp_daemon.go` (3 functions updated):
     * Update `resolveMCPJSONDaemonBinaryRel(root *os.Root, absWorkspace string) (string, bool, error)`: read `.mcp.json` up to `maxMCPConfigFileBytes` (`1 << 20`, 1 MiB) so large unrelated server entries in `.mcp.json` do not fail `PrewarmMCPDaemon`, and inspect **both** `cfg.MCPServers` and `cfg.Servers` (rather than ignoring `cfg.Servers` when `cfg.MCPServers` is non-empty) so a `backlogit` entry in `servers` is discovered and duplicate `backlogit` entries split across `mcpServers` and `servers` fail closed as ambiguous (`ErrValidation`).
     * Update `resolveConfigDaemonBinaryRel(root *os.Root, absWorkspace, storageDir string) (string, bool, error)`: call `if err := validateDaemonYAMLKeys(&doc); err != nil { return "", false, err }` on `config.yaml` (matching `resolveToolchainDaemonBinaryRel`) so duplicate folded keys in `config.yaml` fail closed with `corerrors.ErrValidation`.
     * Update `PrewarmMCPDaemon` to normalize `rootPath = strings.TrimSpace(rootPath)` before the empty check so `PrewarmMCPDaemon` is in `ChangedSymbols` (`in.Symbols.Funcs`) for `redhonesty` attribution and rejects whitespace-only `rootPath` with `ErrValidation`.
  2. In `tests/integration/multi_session_workspace_containment_test.go`:
     * In `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`missing_local_binary_refuses_global_path_fallback`), remove the `.bin` binary installed by `u12InitBacklogitWorkspace(t, root)` (`require.NoError(t, os.RemoveAll(filepath.Join(root, ".bin")))`) so the subtest continues to verify that when the workspace's configured local binary is missing on disk, `PrewarmMCPDaemon` fails closed with `corerrors.ErrDaemonBinaryMissing` without falling back to `PATH`.
* **RED-first acceptance criteria**:
  * **RED harness (`tests/integration/multi_session_workspace_containment_test.go`)**: Add `TestU4_InitAndInstallProvisionedWorkspaceStartsDaemonAndRejectsAmbiguousConfigs` (calling `cli.PrewarmMCPDaemon`, `cli.NewRootCommand`, and `cli.StopMCPDaemon`) testing 2 scenarios: (a) initializing a workspace via `backlogit init --tools-dir .custom/bin` (with a large `>64 KiB` unrelated server entry in `.mcp.json`) and copying the built CLI binary into `.custom/bin/backlogit[.exe]` starts `cli.PrewarmMCPDaemon` cleanly from `.custom/bin/backlogit[.exe]` and stops via `cli.StopMCPDaemon`; and (b) `cli.PrewarmMCPDaemon` fails closed with `corerrors.ErrValidation` on duplicate folded keys in `config.yaml` (`tools_dir` + `Tools_Dir`), duplicate `backlogit` entries split across `mcpServers` and `servers` in `.mcp.json`, and whitespace-only `rootPath`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU4_InitAndInstallProvisionedWorkspace|TestU12_)' ./tests/integration`

### Unit U5: Regenerate CLI reference for `backlogit init --tools-dir` and enforce reference drift check (`218.005-T`)

* **Domain**: `docs` (width-isolated to `docs/cli-reference` and `cmd/gen-docs`)
* **Size / Complexity**: `S` / `low` (2 files, 1 function, 2 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before `docs/cli-reference/backlogit_init.md` regeneration; because no production `.go` symbol is modified in U5, `in.Symbols.Empty()` is true in `redhonesty` and `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` fails at `baseSHA` before `docs/cli-reference/backlogit_init.md` is regenerated and passes at `headSHA`)
* **Dependencies**: `blocks` on Unit U3 (`218.003-T`) and Unit U4 (`218.004-T`)
* **Affected files (2)**:
  * `docs/cli-reference/backlogit_init.md`
  * `cmd/gen-docs/main_test.go`
* **Changes needed**:
  1. Run `go run ./cmd/gen-docs docs/cli-reference` so `docs/cli-reference/backlogit_init.md` documents `--tools-dir` with its preserved `ingested_at` frontmatter.
  2. In `cmd/gen-docs/main_test.go`, add `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` verifying that `generateDocs(outDir)` emits `--tools-dir` in `backlogit_init.md` and that the committed `../../docs/cli-reference/backlogit_init.md` matches the generated `backlogit_init.md` byte-for-byte.
* **RED-first acceptance criteria**:
  * **RED harness (`cmd/gen-docs/main_test.go`)**: Add `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` testing 2 scenarios: (a) `generateDocs(outDir)` produces `backlogit_init.md` documenting `--tools-dir`; and (b) the committed `../../docs/cli-reference/backlogit_init.md` contains `--tools-dir` and equals the generated `backlogit_init.md` byte-for-byte (which is RED at `baseSHA` before `docs/cli-reference/backlogit_init.md` is regenerated, and GREEN at `headSHA`).
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^TestGenerateDocs_' ./cmd/gen-docs`

## Dependency Graph

* **Unit U1 (`218.001-T`)** (`internal/cli/self_install.go`, `internal/cli/self_install_test.go`): no upstream dependencies (Wave 1)
* **Unit U2 (`218.002-T`)** (`internal/cli/self_install.go`, `internal/cli/self_install_test.go`): `blocks` on Unit U1 (`218.001-T`) (Wave 2)
* **Unit U3 (`218.003-T`)** (`internal/cli/root.go`, `internal/cli/self_install_test.go`): `blocks` on Unit U1 (`218.001-T`) and Unit U2 (`218.002-T`) (Wave 3)
* **Unit U4 (`218.004-T`)** (`internal/cli/mcp_daemon.go`, `tests/integration/multi_session_workspace_containment_test.go`): `blocks` on Unit U3 (`218.003-T`) (Wave 4)
* **Unit U5 (`218.005-T`)** (`docs/cli-reference/backlogit_init.md`, `cmd/gen-docs/main_test.go`): `blocks` on Unit U3 (`218.003-T`) and Unit U4 (`218.004-T`) (Wave 5)

## Decisions and Rationale

1. **Reconciled CLI Contract (`--tools-dir` flag + `BACKLOGIT_TOOLS_DIR` + TTY/piped subprocess stdin prompt + `cmd/gen-docs` regeneration)**:
   * Honoring `--tools-dir` alongside interactive TTY and piped subprocess stdin prompting satisfies `103-DL` and `269D054D` with full human/agent parity, while isolating `docs/cli-reference/backlogit_init.md` regeneration in Unit U5 (`218.005-T`) keeps every unit at strictly `<= 2 files` and a single domain.
2. **Compile-Clean `isTerminalReader` and Real Piped Subprocess Stdin Support**:
   * `isTerminal(w io.Writer) bool` (`internal/cli/list.go:137`) accepts `io.Writer`, so checking `cmd.InOrStdin()` (`io.Reader`) uses `isTerminalReader(r io.Reader) bool` (`f, ok := r.(*os.File); ok && term.IsTerminal(int(f.Fd()))`). Additionally, `stdinShouldPrompt(cmd)` checks `os.Stdin.Stat()` (`fi.Mode()&os.ModeNamedPipe != 0 || (fi.Mode().IsRegular() && fi.Size() > 0)`) so real piped subprocesses (`echo "bin" | backlogit init`) prompt and read stdin just like interactive terminals and `cmd.SetIn` tests, while `os.DevNull` in `go test` (`ModeCharDevice != 0` and `!term.IsTerminal`) uses `defaultDir` without blocking.
3. **Fail-Closed `defaultTargetToolsDir(cleanRoot) (string, error)` and Safe First-Init on Nonexistent Workspace Roots**:
   * When `cleanRoot` does not yet exist on disk, `defaultTargetToolsDir` verifies its nearest existing ancestor directory is not a symlink or reparse point and returns `(".bin", nil)` before `cleanRoot` is created. When `cleanRoot` exists, `defaultTargetToolsDir` delegates to `resolveConfiguredDaemonBinaryRel(wsRoot, cleanRoot)` and inspects `.bin` / `bin` via `wsRoot.Lstat`, returning any validation error (`corerrors.ErrValidation`) on corrupt, wrong-typed, duplicate-keyed, oversized, escaping, or redirected configs/directories with **no silent fallback**.
4. **Non-Short-Circuiting `os.SameFile` Self-Install and Single Mutation Ownership**:
   * When `os.SameFile(srcStat, dstInfo)` detects that the running executable is already the file at `<storage_dir>/backlogit[.exe]` or `<tools_dir>/backlogit[.exe]`, only the file-copy/rename step for that path is skipped; `.mcp.json` configuration, `toolchain.yaml` reconciliation, and `.gitignore`/`.git/info/exclude` updates still run to completion.
5. **Scope Discipline — No `config.yaml` Mutation; Explicit Justification for Ignore Rules and `toolchain.yaml` Reconciliation**:
   * `init` and `install` do **not** mutate `<storage_root>/config.yaml` (avoiding unrequested config overreach; `.mcp.json` is Priority 1 in `pinnedMCPDaemonBinary`). Updating `.gitignore` and `.git/info/exclude` with `/<tools_dir>/backlogit` and `/<tools_dir>/backlogit.exe` is explicitly required so installing `backlogit[.exe]` into a newly created target tool directory does not leave an untracked binary in the user's git working tree. Reconciling an already-present `<storage_root>/toolchain.yaml` prevents `AuditRepositoryToolchain` (`backlogit mcp status --toolchain --strict`) from reporting a mismatch after `backlogit install --tools-dir <dir>`.

## Risks and Caveats

* **Risk**: `redhonesty` (`internal/redhonesty/rules.go`) rejects a new test with `RejectNoChangedBranch` if the test's AST identifiers (`r.idents`) do not intersect `in.Symbols` (`ChangedSymbols`), with `RejectUnverifiableBaseline` if the test fails to compile at `baseSHA`, or with `RejectBuggyPathExpected` if a changed function's return value is assigned to a `want*`/`expect*`/`exp*`/`golden*` variable.
  * **Mitigation**: Units U1–U3 place `TestU1_*`..`TestU3_*` in `internal/cli/self_install_test.go` (`package cli`) calling pre-existing modified functions (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, `newInitCommand`); Unit U4 places `TestU4_*` in `tests/integration/multi_session_workspace_containment_test.go` calling modified `cli.PrewarmMCPDaemon`; and Unit U5 modifies only `docs/cli-reference/backlogit_init.md` and `cmd/gen-docs/main_test.go` (`in.Symbols.Empty() == true`, failing at `baseSHA` before `backlogit_init.md` is regenerated and passing at `headSHA`).
* **Risk**: Symlink, Windows junction (`mklink /J`), or mid-flight directory/leaf swap on the target tool directory or `.mcp.json`.
  * **Mitigation**: Walk and create every segment of `toolsDirRel` on pinned `*os.Root` handles (`ensureAndOpenTargetToolsDir`) passing the explicit canonical absolute path to `core.IsSymlinkOrReparsePoint`, and publish both `<toolsDirRel>/backlogit[.exe]` and `.mcp.json` via `atomicfile.ReplaceTempFileInRoot` with pre-replace `Lstat` re-verification and `selfInstallBeforeBinaryReplaceHook` / `selfInstallBeforeIgnoreReplaceHook` test hooks.

## Constitution Check

* **Principle I — Safety-First Go (`MUST`)**: `pass` — All production code is in Go 1.24, wraps errors with `fmt.Errorf("context: %w", err)` preserving `corerrors.ErrValidation`, and uses zero `unsafe` constructs.
* **Principle II — Test-First Development (`NON-NEGOTIABLE`)**: `pass` — Every unit (`U1` through `U5`) begins with a compiling, failing RED test harness verified against `redhonesty` invariants before any production edit.
* **Principle III — Workspace Isolation and Security Boundaries (`MUST`)**: `pass` — All directory creations, binary copies, `.mcp.json` merges, `toolchain.yaml` reconciliations, and ignore-file updates use `os.OpenRoot(absRoot)`, pinned child `*os.Root` handles, `core.IsSymlinkOrReparsePoint`, `os.SameFile`, and `atomicfile.ReplaceTempFileInRoot`.
* **Principle IV — CLI Workspace Containment (`NON-NEGOTIABLE`)**: `pass` — All `init` and `install` operations resolve and write strictly inside the target workspace root (`--cwd` or `[path]`).
* **Principle V — Structured Observability (`MUST`)**: `pass` — `backlogit init` and `backlogit install` emit deterministic initialization confirmation and prompt output to `cmd.OutOrStdout()`, wrap all failure causes with actionable context and sentinel errors (`corerrors.ErrValidation`, `corerrors.ErrDaemonBinaryMissing`), and `_Ship` records red/green test evidence, runtime verification binary identity (`HEAD` SHA + SHA-256), and P-020 closure artifacts.
* **Principle VI — Single Responsibility (`SHOULD`)**: `pass` — Reuses existing standard library (`os`, `bufio`, `encoding/json`) and internal packages (`internal/atomicfile`, `internal/core`, `golang.org/x/term`, `gopkg.in/yaml.v3`) with zero new dependencies (`go.mod` unchanged).
* **Principle VII — Destructive Command Approval (`NON-NEGOTIABLE`)**: `pass` — No destructive commands or data removals are introduced; `.mcp.json` merging preserves all unrelated top-level and server entries via `map[string]json.RawMessage`.
* **Principle VIII — Explicit Safety Modes for Elevated Risk (`MUST`)**: `pass` — Operates under **Freeze-scope mode** (strictly confined to `internal/cli`, `tests/integration/multi_session_workspace_containment_test.go`, `cmd/gen-docs/main_test.go`, and `docs/cli-reference/backlogit_init.md` for stash `269D054D`) and **Careful mode** for workspace root and `.mcp.json` atomic replacement invariants (`I1`–`I5`).
* **Principle IX — Git-Friendly Persistence (`SHOULD`)**: `pass` — `.mcp.json` is formatted with deterministic 2-space indentation and trailing newline (`json.MarshalIndent` + `"\n"`), and `.gitignore` / `.git/info/exclude` entries are appended idempotently preserving line endings.
* **Principle X — Agent Context Efficiency (`SHOULD`)**: `pass` — Uses targeted `backlogit` MCP operations and bounded file reads (`maxMCPConfigFileBytes`).
* **Principle XI — Merge Commit History Preservation (`NON-NEGOTIABLE`)**: `pass` — Shipment `219-S` will merge via a merge commit (`--merge`) under P-009/P-014.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Adds `--tools-dir` flag and interactive/piped target tool directory prompt to `backlogit init` and `backlogit install` (plus `docs/cli-reference/backlogit_init.md` regeneration), creating the target tool directory, installing `backlogit[.exe]` there, and configuring `.mcp.json`.
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Creates workspace directories, installs an executable binary (`0o755`), and writes `.mcp.json` (which controls MCP server command execution) inside the target workspace.
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — Additive, idempotent provisioning that preserves existing non-`backlogit` `.mcp.json` entries via `map[string]json.RawMessage` and does not mutate `config.yaml`.
* **external integration, operator checkpoint, or external dependency**: `absent` — Uses existing standard library and internal packages (`go.mod` unchanged).
* **high runtime, rollout, or rollback risk**: `present` — Modifies `backlogit init` and `backlogit install` workspace bootstrap and `.mcp.json` generation consumed by `backlogit mcp` daemon startup and `ValidateMCPConfigPinning`.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * `backlogit init [path] [--tools-dir <dir>]` and `backlogit install [path] [--tools-dir <dir>]` (`internal/cli/root.go`, `internal/cli/self_install.go`)
  * Workspace `.mcp.json` generation and `toolchain.yaml` reconciliation consumed by `backlogit mcp` (`PrewarmMCPDaemon`, `RunMCPSessionShim`) and `backlogit mcp status --toolchain`
* **Runtime verification probes (for `_Ship` Step 6 — MUST build and identify binary from reviewed/merged `HEAD`)**:
  1. **Build and identify the `HEAD` binary (never use an unverified ambient `PATH` binary)**:
     * Record `headSHA=$(git rev-parse HEAD)`.
     * Build a fresh verification binary from `HEAD`: `go build -o <tempBin>/backlogit[.exe] ./cmd/backlogit`.
     * Record the binary's SHA-256 hash (`Get-FileHash <tempBin>/backlogit.exe -Algorithm SHA256` / `sha256sum`) and output of `<tempBin>/backlogit[.exe] version --no-update-check --format json` as identity evidence in the closure artifact.
  2. **Verify interactive/piped prompt and `--tools-dir` provisioning with `<tempBin>/backlogit[.exe]`**:
     * In isolated `--cwd <tempDir>` workspaces (including a brand-new nonexistent `<tempDir>/first-init` path), execute `<tempBin>/backlogit[.exe] init --cwd <tempDir> --tools-dir .custom-tools/bin` and pipe `"bin\n"` into `<tempBin>/backlogit[.exe] install --cwd <tempDir2>`, verifying that the target tool directory is created, `backlogit[.exe]` is installed into that directory (matching the `HEAD` binary's SHA-256) prior to `.mcp.json` configuration, and `.mcp.json` + `.gitignore` are configured accurately.
  3. **Verify MCP daemon startup from the provisioned workspace**:
     * Run `<tempBin>/backlogit[.exe] --cwd <tempDir> mcp prewarm --format json`, `<tempBin>/backlogit[.exe] --cwd <tempDir> mcp status --format json`, and `<tempBin>/backlogit[.exe] --cwd <tempDir> mcp stop --format json`, confirming `ExecutablePath` in the daemon status matches `<tempDir>/.custom-tools/bin/backlogit[.exe]`.
* **Operational closure expectations**:
  * **Owner**: `_Ship` agent (supervised by `_Orchestrator` and operator).
  * **Validation window**: Immediate post-merge CLI/MCP verification on `main`.
  * **Healthy signals**: `backlogit init` and `backlogit install` provision `<tools_dir>/backlogit[.exe]` and `.mcp.json` cleanly; `PrewarmMCPDaemon` starts without `ErrDaemonBinaryMissing`; `go test -timeout=30m ./...` and `make docs-lint` pass with zero violations.
  * **Rollback trigger & procedure**: Any regression in `backlogit init`/`install` or `.mcp.json` provisioning triggers a reviewed `git revert -m 1 <merge_sha>` PR.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by CLI contract, filesystem executable/config write containment, and MCP bootstrap runtime signals across `internal/cli/root.go`, `internal/cli/self_install.go`, and `internal/cli/mcp_daemon.go`.
* **Protected invariants**:
  * **I1 (Handle-pinned segment-by-segment directory creation and non-short-circuiting binary installation)**: `ensureAndOpenTargetToolsDir` and `installWorkspaceRuntimeExecutable` MUST operate relative to `os.OpenRoot(absRoot)` by walking each segment of `toolsDirRel` on pinned child `*os.Root` directory handles (`Lstat` -> `Mkdir(part, 0o755)` if `os.ErrNotExist` -> `core.IsSymlinkOrReparsePoint(info, nextAbs)` using the explicit canonical `nextAbs` path -> `OpenRoot(part)` -> `Stat(".")` + `os.SameFile`), creating the temp executable on the innermost `toolsDirHandle`, applying `tmpFile.Chmod(0o755)` and `tmpFile.Sync()` on the open file handle, and publishing via `atomicfile.ReplaceTempFileInRoot(toolsDirHandle, toolsDirAbs, tmpName, binName, true)`. When `os.SameFile(srcStat, dstInfo)` is true, skip only the file-copy/rename block and still execute `.mcp.json` configuration, `toolchain.yaml` reconciliation, and ignore-file updates.
  * **I2 (Strict ordering — target tool directory creation and binary install prior to `.mcp.json` configuration)**: `configureWorkspaceMCPJSON` MUST verify via `openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))` that the target tool directory and `<toolsDirRel>/<binName>` already exist as a regular, non-redirected file before writing or replacing `.mcp.json`.
  * **I3 (Lossless `.mcp.json` raw-JSON merge, alias/multi-entry detection, and `toolchain.yaml` reconciliation)**: When `.mcp.json` already exists, `configureWorkspaceMCPJSON` MUST read up to `maxMCPConfigFileBytes` (`1 MiB`), reject symlinks/reparse points and duplicate keys (`scanDaemonJSON`), unmarshal top-level and server maps as `map[string]json.RawMessage` to preserve unrelated values verbatim, inspect both `mcpServers` and `servers` using `isBacklogitDaemonCommand` (failing closed on `>1` matched `backlogit` entries or wrong-typed JSON fields), update the single matched entry in place (or insert `mcpServers.backlogit`), and reconcile `<storage_root>/toolchain.yaml` if present without mutating `config.yaml`.
  * **I4 (Safe first-init on nonexistent workspace roots and fail-closed `defaultTargetToolsDir`)**: `defaultTargetToolsDir(cleanRoot) (string, error)` MUST check the nearest existing ancestor directory for symlinks/reparse points and return `(".bin", nil)` when `cleanRoot` does not yet exist; when `cleanRoot` exists, any malformed, wrong-typed, duplicate-keyed, oversized, or redirected `.mcp.json`, `toolchain.yaml`, `config.yaml`, `.bin`, or `bin` MUST return `corerrors.ErrValidation` fail-closed with no silent fallback.
  * **I5 (`redhonesty` P-002.1 compliance and `<3 files` task granularity across `U1`–`U5`)**: Every unit (`U1`–`U5`) modifies strictly `2 files`, `<= 4 functions`, and `<= 3 test scenarios`. New tests in `U1`–`U4` call only symbols that already exist at that unit's `baseSHA` and directly reference at least one modified function in `in.Symbols.Funcs` (`installWorkspaceRuntimeExecutable` / `ensureWorkspaceRuntimeIgnored` in U1 and U2; `newInitCommand` in U3; `PrewarmMCPDaemon` in U4), while `U5` has `in.Symbols.Empty() == true` and fails at `baseSHA` on the unregenerated `docs/cli-reference/backlogit_init.md`.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/go-mcp-server.instructions.md`
  * `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`
  * `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`
  * `docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md`
  * `docs/compound/2026-10-16-root-confined-access-and-graphql-reviewer-union.md`
  * `docs/compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`
  * `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`
  * `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`
* **Risky actions classification (`strict-safety`)**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Create target tool directory (`.bin`, `bin`, or custom relative path), self-install `backlogit[.exe]`, and update ignore rules (`U1`) | `internal/cli/self_install.go`, `internal/cli/self_install_test.go` | `moderate` (filesystem directory & executable write) | Reuse `validateDaemonToolsDirRel`; walk pinned `*os.Root` handles; publish via `atomicfile.ReplaceTempFileInRoot`; do not short-circuit on `os.SameFile` | Target tool directory and `backlogit[.exe]` are created safely inside the workspace root; symlink/junction escapes fail closed |
| Create or merge `.mcp.json` via `json.RawMessage` and reconcile `toolchain.yaml` after binary install (`U2`) | `internal/cli/self_install.go`, `internal/cli/self_install_test.go` | `moderate` (MCP launch configuration write) | Verify `<tools_dir>/backlogit[.exe]` exists first; validate JSON/YAML for duplicate keys and types; detect aliases across `mcpServers` and `servers`; publish via `ReplaceTempFileInRoot` | `.mcp.json` points to `./<tools_dir>/backlogit[.exe]` with `--cwd <absRoot>` while preserving unrelated entries and reconciling `toolchain.yaml` |
| Prompt operator (TTY and piped stdin) and support `--tools-dir` / `BACKLOGIT_TOOLS_DIR` on `init` and `install` (`U3`) | `internal/cli/root.go`, `internal/cli/self_install_test.go` | `low` (CLI prompt, flag, & fail-closed default discovery) | Use `isTerminalReader` + `os.Stdin.Stat()` pipe detection; handle nonexistent first-init roots safely; fail closed on invalid inputs or corrupt configs before mutating workspace | Operator and automated callers can select `.bin`, `bin`, or custom directory via prompt, pipe, flag, or env var |
| Harden daemon config resolution (`1 MiB` `.mcp.json`, `servers` inspection, `config.yaml` duplicate-key check) and align `TestU12` fixture (`U4`) | `internal/cli/mcp_daemon.go`, `tests/integration/multi_session_workspace_containment_test.go` | `low` (daemon config validation & test fixture alignment) | Enforce `validateDaemonYAMLKeys` on `config.yaml`; inspect `mcpServers` + `servers` in `resolveMCPJSONDaemonBinaryRel` | End-to-end `init`/`install` workspaces start `PrewarmMCPDaemon` cleanly and malformed/ambiguous configs fail closed |
| Regenerate `docs/cli-reference/backlogit_init.md` via `cmd/gen-docs` (`U5`) | `docs/cli-reference/backlogit_init.md`, `cmd/gen-docs/main_test.go` | `low` (generated CLI reference documentation) | Run `go run ./cmd/gen-docs docs/cli-reference` and assert byte-identity in `main_test.go` | Zero CI `CLI Reference Drift` |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode: multi-agent-dispatch` (or `single-agent-declared-degradation` with `TOOL_DEGRADED: reviewer-subagent-dispatch` if degraded), cover all 7 required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`), and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) before harvest.
* **Unresolved operator decisions**: None.

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale (Attempt 1 — superseded by Attempt 2 below)**: Initial 3-unit plan review prior to local review cycle 1 remediation.

<!-- plan-review-attempt: 2 -->

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale (Attempt 2 — post-remediation of PR #664 local review cycle 1)**: Executed a complete sequential rubric pass across all 7 required personas (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`) under `single-agent-declared-degradation` (`[PERSONA-PASS]` / `[PERSONA-DONE]` for all 7 personas) after remediating all 13 review themes from PR #664 local review cycle 1. Plan hardening was required (`Requires plan hardening: yes`) and is satisfied by the updated `## Plan Hardening` section (Invariants `I1`–`I5` and `ProposedAction` / `ActionRisk` table). `## Constitution Check` maps Principles I–XI explicitly (including Principle V Structured Observability and Principle VIII Explicit Safety Modes) and concludes with literal `Constitution Check: pass`. Total merged findings across all 7 personas: `0` (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Persona evaluation summary**:
  * **Constitution Reviewer**: `0` findings (`PASS`). All Principles I–XI (including Principle V Structured Observability and Principle VIII Freeze-scope / Careful safety modes) are explicitly mapped in `## Constitution Check` concluding with `Constitution Check: pass`. Every unit (`U1`–`U5`) modifies strictly `2 files` (`<3 files`), `1`–`4 functions` (`<5 functions`), and `2`–`3 test scenarios` (`<4 scenarios`), in a single domain (`code` for `U1`–`U4`, `docs` for `U5`).
  * **Go Reviewer**: `0` findings (`PASS`). Terminal detection on `cmd.InOrStdin()` uses `isTerminalReader(r io.Reader) bool` (`f, ok := r.(*os.File); ok && term.IsTerminal(int(f.Fd()))`) and `os.Stdin.Stat()` pipe/regular-file inspection; `defaultTargetToolsDir(cleanRoot) (string, error)` returns errors fail-closed and handles nonexistent first-init roots safely; `os.SameFile` skips only the binary file copy without short-circuiting `.mcp.json` or ignore provisioning; and canonical validators (`validateDaemonToolsDirRel`, `isBacklogitDaemonCommand`, `scanDaemonJSON`, `validateDaemonYAMLKeys`, `resolveConfiguredDaemonBinaryRel`) are reused.
  * **Scope Boundary Auditor**: `0` findings (`PASS`). Reconciled the CLI contract (`--tools-dir` + prompt + `cmd/gen-docs` in `U5` `218.005-T`) across `103-DL`, deliberation doc, plan, `218-F`, `218.001-T`..`218.005-T`, and `219-S`; removed unrequested `config.yaml` `tools_dir` write overreach; and explicitly justified `.gitignore` / `.git/info/exclude` ignore rules and existing `toolchain.yaml` reconciliation.
  * **Learnings Researcher**: `0` findings (`PASS`, `confidence: high`). Plan incorporates `2026-10-27-pin-the-handle-not-the-path.md`, `nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`, `derive-daemon-version-from-pinned-image-2026-11-09.md`, `2026-10-16-root-confined-access-and-graphql-reviewer-union.md`, `2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`, `2026-08-29-windows-preremove-rename-pattern.md`, and `cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`.
  * **Architecture Strategist**: `0` findings (`PASS`). Single ownership/API in `internal/cli/self_install.go` and `internal/cli/root.go` eliminates duplicate calls, reuses `internal/cli/mcp_daemon.go` validators, reconciles `toolchain.yaml` for static audit parity, and sequences Waves 1–5 (`U1 -> U2 -> U3 -> U4 -> U5`) with zero cross-wave file conflicts.
  * **Agent-Native Parity Reviewer**: `0` findings (`PASS`). Supports interactive TTY prompt, real piped subprocess stdin (`echo "bin" | backlogit init`), `--tools-dir` flag, and `BACKLOGIT_TOOLS_DIR` env var, while `.mcp.json` merging via `map[string]json.RawMessage` preserves unrelated top-level and server entries and detects `backlogit` aliases across `mcpServers` and `servers`.
  * **Security Lens Reviewer**: `0` findings (`PASS`). 3-threat model verified: (1) traversal/reserved/absolute `tools_dir` inputs and corrupt/wrong-typed/duplicate-keyed existing configs fail closed with `corerrors.ErrValidation` and no silent fallback; (2) symlink/junction swaps on nonexistent or existing workspace roots, target tool directories, or `.mcp.json` are blocked by ancestor `Lstat` checks, pinned child `*os.Root` handles + `core.IsSymlinkOrReparsePoint` + `os.SameFile`, and `atomicfile.ReplaceTempFileInRoot`; (3) runtime verification explicitly builds and verifies the `HEAD` binary by commit SHA and SHA-256 digest rather than relying on ambient `PATH`.
* **Runtime verification & operational closure check**: Verified complete in `## Runtime Verification and Closure` with explicit `HEAD`-built binary identity verification.


