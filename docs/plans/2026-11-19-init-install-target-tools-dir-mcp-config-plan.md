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
3. **Missing Operator Target Tool Directory Prompt and `--tools-dir` Flag (`internal/cli/root.go`)**: `newInitCommand` (`backlogit init` and `backlogit install`) does not accept `--tools-dir`, does not prompt the operator on interactive TTY or piped subprocess stdin with a default target install directory of `.bin`, `bin`, or allow entering an alternate workspace-relative directory option, and `docs/cli-reference/backlogit_init.md` must be regenerated via `cmd/gen-docs` to reflect `--tools-dir`.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Reuse canonical `validateDaemonToolsDirRel`, create missing target tool directory segments via step-by-step pinned `*os.Root` directory handles (`ensureAndOpenTargetToolsDir`), install `backlogit[.exe]` there atomically without letting `os.SameFile` short-circuit downstream steps, and record idempotent `/<tools_dir>/backlogit[.exe]` ignore rules in `.gitignore` and `.git/info/exclude` | `269D054D` / `103-DL` | Unit U1 (`218.001-T`) | `TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries` |
| R2: Verify `<tools_dir>/backlogit[.exe]` exists prior to configuring `.mcp.json`, atomically create or merge `.mcp.json` via `map[string]json.RawMessage` (preserving unrelated top-level/server values, detecting aliases across `mcpServers` and `servers`, and refusing ambiguous multi-entry or wrong-type configs), and reconcile existing `<storage_root>/toolchain.yaml` via canonical typed `ToolchainManifest` load-mutate-validate-write when present (without mutating `config.yaml`) | `269D054D` / `103-DL` | Unit U2 (`218.002-T`) | `TestU2_ConfigureWorkspaceMCPJSONAndReconcileToolchainAfterInstall` |
| R3: Parameterize `validateDaemonYAMLKeys(node, fileName)` in `internal/cli/mcp_daemon.go`, enforce full-ancestor walk (`validateWorkspaceAndTargetAncestors(cleanRoot, targetRel)`), and implement non-short-circuiting `defaultTargetToolsDir(cleanRoot string) (string, error)` (validating every present `.mcp.json`, `toolchain.yaml`, `config.yaml`, `.bin`, and `bin` source before selecting the precedence winner, and handling nonexistent first-init roots safely) | `269D054D` / `103-DL` | Unit U3 (`218.003-T`) | `TestU3_DefaultTargetToolsDirAndNonShortCircuitingPreflight` |
| R4: Add `--tools-dir` flag and `stdinShouldPrompt` / `resolveInitTargetToolsDir` to `newInitCommand` (`internal/cli/root.go`) running unconditional `defaultTargetToolsDir(cleanRoot)` and `validateWorkspaceAndTargetAncestors(cleanRoot, cleanRel)` preflight on every path (including `--tools-dir` overrides), align `TestU12` missing-binary fixture after `u12InitBacklogitWorkspace`, and verify end-to-end `PrewarmMCPDaemon` startup | `269D054D` / `103-DL` | Unit U4 (`218.004-T`) | `TestU4_InitAndInstallPromptToolsDirAndDaemonStartup` |
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
       - Use `ensureAndOpenTargetToolsDir(wsRoot, absRoot, targetDir)` so `targetDir` can be either a storage root (`.backlog` / `.backlogit`) or a single/multi-segment target tool directory (`.bin`, `bin`, `.tools/bin`).
       - When `dstInfo` already exists and `os.SameFile(srcStat, dstInfo)` is true, skip **only** the temp-file copy/rename block rather than returning early before post-copy provisioning hooks (`configureWorkspaceMCPJSON` / `reconcileWorkspaceToolchainYAML` in Unit U2).
     * Update `ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName string) error`:
       - Keep the exact `(string, string, string) error` signature; when `toolsDirRel != ""` and `!slices.Contains(core.WorkspaceRootCandidates(), toolsDirRel)`, validate `cleanToolsDir, err := validateDaemonToolsDirRel(toolsDirRel)` and append `"/" + filepath.ToSlash(cleanToolsDir) + "/backlogit"` and `"/" + filepath.ToSlash(cleanToolsDir) + "/backlogit.exe"` to `workspaceRuntimeIgnoreEntries()` before updating `.gitignore` and `.git/info/exclude`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries` (in `package cli`, calling only pre-existing symbols `installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, and `selfInstallBeforeBinaryReplaceHook`; never naming result variables `want*`/`expect*`/`exp*`/`golden*`) with 3 `t.Run` scenarios:
    1. `creates_target_dir_installs_binary_and_records_ignore_rules`: Calling `installWorkspaceRuntimeExecutable(root, ".tools/custom")` creates the missing directory segments on `wsRoot`, installs `backlogit[.exe]` with executable permissions (`mode & 0o111 != 0` on POSIX), and calling `ensureWorkspaceRuntimeIgnored(root, ".tools/custom", binName)` records `/.tools/custom/backlogit` and `/.tools/custom/backlogit.exe` once in `.gitignore` and `.git/info/exclude`.
    2. `samefile_installed_binary_skips_self_overwrite_without_error`: Invoking `installWorkspaceRuntimeExecutable` when `selfInstallExecutablePathFn` points to the already-installed `<targetDir>/backlogit[.exe]` (`os.SameFile`) succeeds without error or truncation.
    3. `rejects_invalid_and_redirected_target_tool_directories`: Invalid or redirected target tool directories (`"../escape"`, `".git"`, symlink/junction segments, or mid-flight swaps via `selfInstallBeforeBinaryReplaceHook`) fail closed with `corerrors.ErrValidation` without writing outside the workspace root.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U2: Configure `.mcp.json` and reconcile canonical `ToolchainManifest` after tool binary install (`218.002-T`)

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
       - Read existing `.mcp.json` (if present) via `readLocalFileInRoot(wsRoot, absRoot, ".mcp.json", maxMCPDaemonInfoBytes)` and validate with `scanDaemonJSON(existingBytes)` (rejecting duplicate keys and malformed JSON fail-closed with `corerrors.ErrValidation`).
       - Unmarshal top-level JSON into `map[string]json.RawMessage` (rejecting non-object top-level JSON such as arrays, primitives, or `null` with `corerrors.ErrValidation`) so all unrelated top-level keys survive verbatim.
       - Inspect both `"mcpServers"` and `"servers"` in the top-level map: if either is present with a non-object JSON value (e.g. array/string/number/`null`), fail closed with `corerrors.ErrValidation`. Across all server entries in `"mcpServers"` and `"servers"`, unmarshal each server entry into `map[string]json.RawMessage` (rejecting non-object server entries or non-string `"command"`/`"type"`/`"url"` or non-string-array `"args"` with `corerrors.ErrValidation`) and identify entries matching `strings.EqualFold(strings.TrimSpace(name), "backlogit") || isBacklogitDaemonCommand(cmdStr)`.
       - If `len(matched) > 1` across `"mcpServers"` and `"servers"`, fail closed with `corerrors.ErrValidation` (`"ambiguous multiple backlogit entries in .mcp.json"`).
       - If `len(matched) == 1`, update that matched entry in its owning map (`"mcpServers"` or `"servers"`, preserving custom alias keys such as `"backlog-alias"` and preserving any unrelated fields on that server object such as `"tools"` or `"env"` while setting `"type": "stdio"`, `"command": "./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))`, and `"args": ["mcp", "--cwd", absRoot]`). If `len(matched) == 0`, insert `"backlogit"` into `"mcpServers"`.
       - Write formatted JSON (`json.MarshalIndent` + `"\n"`) to a handle-relative temp file on `wsRoot`, `Stat`/`Lstat`/`SameFile`, `Sync()`, `Chmod(mode)`, `Close()`, invoke `selfInstallBeforeIgnoreReplaceHook(".mcp.json")` if set, re-verify `wsRoot.Lstat(".mcp.json")` is not a symlink/reparse point or directory, and atomically publish via `atomicfile.ReplaceTempFileInRoot(wsRoot, absRoot, tmpName, ".mcp.json", false)`.
     * Add `reconcileWorkspaceToolchainYAML(wsRoot *os.Root, absRoot, toolsDirRel, binName string) error` (canonical typed `ToolchainManifest` load-mutate-validate-write):
       - Resolve existing storage root (`resolveDaemonStorageDirName(wsRoot, absRoot)`). If `!hasStorage`, or if `<storageDir>/toolchain.yaml` does not exist (`os.ErrNotExist`), return `nil` (never creating an unrequested `toolchain.yaml` and never mutating `config.yaml`).
       - When `<storageDir>/toolchain.yaml` exists on disk, load and strictly validate it via `toolchain.LoadToolchainManifest(absRoot)` (which enforces `os.OpenRoot`, duplicate-key rejection via `validateYAMLKeys`, required-field validation via `validateManifestFields`, and strict `decoder.KnownFields(true)` decoding into `*toolchain.ToolchainManifest`, rejecting unknown fields, extra YAML documents, and workspace mismatches with `corerrors.ErrValidation`).
       - Scan `manifest.Tools` for entries matching `strings.EqualFold(strings.TrimSpace(t.ID), "backlogit") || isBacklogitDaemonCommand(t.ExecutablePath)`. If `len(matched) > 1`, or if a matched entry has `ContainmentClass != toolchain.WorkspaceLocalPinned` or non-empty `EntrypointPath`, return `corerrors.ErrValidation`.
       - Compute the SHA-256 hex digest of the installed `<toolsDirRel>/<binName>` file via `openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))`.
       - If `manifest.ToolsDir != ""`, update `manifest.ToolsDir = filepath.ToSlash(toolsDirRel)`. If `len(matched) == 1`, update that entry's `ExecutablePath = "./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))` and (when `SHA256 != ""` or `LockfilePath == ""`) update `SHA256` to the installed binary's SHA-256 hex digest while preserving all other fields on the entry (`ID`, `Kind`, `PinnedVersion`, `LockfilePath`, `WorkspaceBindingArgs`, `Reason`) and all other entries in `manifest.Tools` and `manifest.HostPrerequisites`.
       - Validate and atomically write the updated manifest via `toolchain.WriteToolchainManifest(absRoot, manifest)` (which executes `validateManifestForProvisioning` and publishes `<storageDir>/toolchain.yaml` via `storageHandle`).
     * Update `installWorkspaceRuntimeExecutable(root, targetDir string) (string, error)` so whenever `targetDir` is a target tool directory (`!isStorageRoot`), after ensuring `<targetDir>/<binName>` is in place (including when `os.SameFile` skipped copying!), it invokes `configureWorkspaceMCPJSON(wsRoot, absRoot, cleanToolsDir, binName)` and `reconcileWorkspaceToolchainYAML(wsRoot, absRoot, cleanToolsDir, binName)`.
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU2_ConfigureWorkspaceMCPJSONAndReconcileToolchainAfterInstall` (in `package cli`, calling only pre-existing symbols `installWorkspaceRuntimeExecutable` and `selfInstallBeforeIgnoreReplaceHook`) with 3 `t.Run` scenarios:
    1. `provisions_mcp_json_after_binary_install_including_samefile_invocation`: Calling `installWorkspaceRuntimeExecutable(root, ".tools/bin")` (including when `selfInstallExecutablePathFn` already points to the installed target binary so `os.SameFile` is true) writes `.mcp.json` with `mcpServers.backlogit` (`type: "stdio"`, `command: "./.tools/bin/<binName>"`, `args: ["mcp", "--cwd", absRoot]`) and leaves `<storage_root>/config.yaml` unmutated.
    2. `preserves_unrelated_json_merges_aliases_and_reconciles_typed_toolchain_manifest`: Preserves unrelated top-level/server `json.RawMessage` values, updates an alias entry `"backlog-alias"` or `"servers"` entry in place, and reconciles an existing `<storage_root>/toolchain.yaml` via `toolchain.LoadToolchainManifest` + `toolchain.WriteToolchainManifest` so `toolchain.AuditRepositoryToolchain` passes.
    3. `rejects_ambiguous_malformed_unknown_field_or_redirected_configs`: Fails closed with `corerrors.ErrValidation` on multiple `backlogit` entries in `.mcp.json` or `toolchain.yaml`, unknown fields or duplicate keys in existing `toolchain.yaml`, wrong-typed JSON fields (`"mcpServers": []` or `"command": 123`) in `.mcp.json`, and symlinked/swapped `.mcp.json`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU2_ConfigureWorkspaceMCPJSON|TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U3: Implement non-short-circuiting `defaultTargetToolsDir(cleanRoot)` and ancestor preflight validation (`218.003-T`)

* **Domain**: `code` (width-isolated to `internal/cli`)
* **Size / Complexity**: `M` / `medium` (2 files, 4 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU3_*` in `package cli` directly calls pre-existing modified functions `resolveConfiguredDaemonBinaryRel` and `validateDaemonYAMLKeys`)
* **Dependencies**: `blocks` on Unit U1 (`218.001-T`) and Unit U2 (`218.002-T`)
* **Affected files (2)**:
  * `internal/cli/mcp_daemon.go`
  * `internal/cli/self_install_test.go`
* **Changes needed**:
  1. In `internal/cli/mcp_daemon.go` (4 functions added/updated):
     * Update `validateDaemonYAMLKeys(node *yaml.Node, fileName string) error` (or keep a 1-arg wrapper if needed and parameterize `fileName` across `toolchain.yaml` and `config.yaml`) so duplicate-key errors report `mcpDaemonValidationError("duplicate YAML key %q in %s", rawKey, fileName)` rather than hard-coding `"in toolchain.yaml"`.
     * Update `resolveMCPJSONDaemonBinaryRel` and `resolveConfigDaemonBinaryRel` (called by `resolveConfiguredDaemonBinaryRel`):
       - In `resolveMCPJSONDaemonBinaryRel`, inspect **both** `cfg.MCPServers` and `cfg.Servers` so a `backlogit` entry in `servers` is discovered and duplicate `backlogit` entries split across `mcpServers` and `servers` fail closed as ambiguous (`ErrValidation`).
       - In `resolveConfigDaemonBinaryRel`, call `validateDaemonYAMLKeys(&doc, "config.yaml")` and verify that any `tools_dir` mapping value node is a string scalar (`yaml.ScalarNode` with `Tag == "!!str"`), failing closed on sequence/map/int/bool/null `tools_dir` values.
     * Add `validateWorkspaceAndTargetAncestors(cleanRoot, targetRel string) error`:
       - Walk **every ancestor directory component** of `cleanRoot` from `filepath.VolumeName(cleanRoot)+string(filepath.Separator)` down to `cleanRoot`: for every existing component along the path, call `os.Lstat(prefix)` and verify `!core.IsSymlinkOrReparsePoint(info, prefix)` and `info.IsDir()`.
       - When `cleanRoot` exists on disk and `targetRel != ""`, open `wsRoot` (`os.OpenRoot(cleanRoot)`) + `os.SameFile` and walk every existing segment of `targetRel` on pinned child `*os.Root` handles verifying `!core.IsSymlinkOrReparsePoint` and `info.IsDir()`.
     * Add `defaultTargetToolsDir(cleanRoot string) (string, error)` (canonical 1-arg signature):
       - Call `validateWorkspaceAndTargetAncestors(cleanRoot, "")`. If `cleanRoot` does not yet exist on disk (`os.IsNotExist` on `os.Lstat(cleanRoot)`), return `(".bin", nil)` for safe first-init on a nonexistent workspace root.
       - When `cleanRoot` exists, open `wsRoot` (`os.OpenRoot(cleanRoot)`) + `os.SameFile` and validate **every present source and candidate directory without short-circuiting**:
         1. `.mcp.json` via `resolveMCPJSONDaemonBinaryRel(wsRoot, cleanRoot)`
         2. `<storageDir>/toolchain.yaml` via `resolveToolchainDaemonBinaryRel(wsRoot, cleanRoot, storageDir)` (when a storage root exists)
         3. `<storageDir>/config.yaml` via `resolveConfigDaemonBinaryRel(wsRoot, cleanRoot, storageDir)` (when a storage root exists)
         4. `.bin` on `wsRoot` (`wsRoot.Lstat(".bin")`: if present, must satisfy `!core.IsSymlinkOrReparsePoint` and `info.IsDir()`)
         5. `bin` on `wsRoot` (`wsRoot.Lstat("bin")`: if present, must satisfy `!core.IsSymlinkOrReparsePoint` and `info.IsDir()`)
       - If **any** present source (1)–(5) returns an error, return `("", err)` fail-closed with **no silent fallback** (even when a higher-priority source was valid).
       - After all present sources (1)–(5) validate cleanly, return the precedence winner: (1) `.mcp.json` dir -> (2) `toolchain.yaml` dir -> (3) `config.yaml` dir -> (4) `".bin"` (if `.bin` exists or neither `.bin` nor `bin` exists) -> (5) `"bin"` (if `bin` exists and `.bin` does not).
* **RED-first acceptance criteria**:
  * **RED harness (`internal/cli/self_install_test.go`)**: Add `TestU3_DefaultTargetToolsDirAndNonShortCircuitingPreflight` (in `package cli`, calling pre-existing modified symbol `resolveConfiguredDaemonBinaryRel`) with 3 `t.Run` scenarios:
    1. `precedence_and_split_servers_resolution`: Verifies precedence across `.mcp.json` (`mcpServers` and `servers`), `toolchain.yaml`, `config.yaml`, `.bin`, and `bin`, and refusal of duplicate `backlogit` entries split across `mcpServers` and `servers`.
    2. `config_yaml_duplicate_key_and_wrong_type_refusal`: Verifies `resolveConfiguredDaemonBinaryRel` fails closed with `corerrors.ErrValidation` and an error message naming `"config.yaml"` (never `"toolchain.yaml"`) when `config.yaml` contains duplicate folded keys (`tools_dir` + `Tools_Dir`) or a non-string `tools_dir` value.
    3. `non_short_circuiting_lower_priority_and_redirected_dir_refusal`: Verifies that malformed/redirected lower-priority sources or symlinked/non-directory `.bin` / `bin` entries are never masked by higher-priority sources.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU3_DefaultTargetToolsDir|TestU2_ConfigureWorkspaceMCPJSON|TestU1_TargetToolDirectory|TestU6_)' ./internal/cli`

### Unit U4: Wire target tool directory prompt and `--tools-dir` flag into `init` and `install` (`218.004-T`)

* **Domain**: `code` (width-isolated to `internal/cli` and `tests/integration`)
* **Size / Complexity**: `M` / `medium` (2 files, 4 functions, 3 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before production edit; `TestU4_*` in `tests/integration/multi_session_workspace_containment_test.go` calls `cli.NewRootCommand`, which is touched in `internal/cli/root.go` so `"NewRootCommand"` is in `in.Symbols.Funcs` and `r.idents`)
* **Dependencies**: `blocks` on Unit U3 (`218.003-T`)
* **Affected files (2)**:
  * `internal/cli/root.go`
  * `tests/integration/multi_session_workspace_containment_test.go`
* **Changes needed**:
  1. In `internal/cli/root.go` (4 functions added/updated):
     * Add `stdinShouldPrompt(cmd *cobra.Command) (bool, error)`:
       - Single function combining terminal and pipe/redirected-file detection: checks `in := cmd.InOrStdin()`, `out := cmd.OutOrStdout()`, `inFile, ok := in.(*os.File)`, `isTTY := ok && term.IsTerminal(int(inFile.Fd())) && isTerminal(out)`. Returns `true` when `isTTY` is true (interactive terminal), OR when `in != os.Stdin` (`cmd.SetIn` reader in tests), OR when `in == os.Stdin` and `fi, err := os.Stdin.Stat()` succeeds with `fi.Mode()&os.ModeNamedPipe != 0 || (fi.Mode().IsRegular() && fi.Size() > 0)` (real piped or file-redirected subprocess stdin).
     * Add `resolveInitTargetToolsDir(cmd *cobra.Command, cleanRoot, toolsDirFlag string) (string, error)`:
       - **Unconditional preflight on EVERY path (including `--tools-dir`)**: Always call `defaultDir, err := defaultTargetToolsDir(cleanRoot)` FIRST so every execution path runs full ancestor-chain validation (`validateWorkspaceAndTargetAncestors(cleanRoot, "")`) and non-short-circuiting validation of all present `.mcp.json`, `toolchain.yaml`, `config.yaml`, `.bin`, and `bin` sources before any mutation.
       - If `cmd != nil && cmd.Flags().Changed("tools-dir")`: reject blank or whitespace-padded `toolsDirFlag`, strip optional leading `./` or `.\`, validate `cleanRel, err := validateDaemonToolsDirRel(candidate)` and `validateWorkspaceAndTargetAncestors(cleanRoot, cleanRel)`, and return `cleanRel` without prompting.
       - Otherwise check `shouldPrompt, err := stdinShouldPrompt(cmd)`. If `shouldPrompt`: write `"Target tool install directory [.bin, bin, or custom workspace-relative path] (default: %s): "` to `cmd.OutOrStdout()`, read a line from `bufio.NewReader(cmd.InOrStdin())`, map `""` (or `io.EOF` with empty line) -> `defaultDir`, `"1"` or `".bin"` -> `".bin"`, `"2"` or `"bin"` -> `"bin"`, `"3"` -> prompt `"Enter custom workspace-relative tool directory: "` and read next line (failing with `corerrors.ErrValidation` if empty), or any other non-empty string -> custom directory; validate `cleanRel, err := validateDaemonToolsDirRel(candidate)` and `validateWorkspaceAndTargetAncestors(cleanRoot, cleanRel)`, and return `cleanRel`.
       - When `!shouldPrompt`: validate `validateWorkspaceAndTargetAncestors(cleanRoot, defaultDir)` and return `defaultDir`.
     * Update `newInitCommand(cwd *string) *cobra.Command` and `NewRootCommand() *cobra.Command` in `internal/cli/root.go`:
       - Register `--tools-dir` flag (`cmd.Flags().StringVar(&toolsDirFlag, "tools-dir", "", "workspace-relative target tool directory for backlogit binary and .mcp.json (default .bin or bin)")`).
       - Call `toolsDirRel, err := resolveInitTargetToolsDir(cmd, cleanRoot, toolsDirFlag)` **before** `os.MkdirAll(dir, 0o755)` so any invalid `--tools-dir`, invalid prompt input, redirected ancestor/target segment, or corrupt existing config fails closed with zero filesystem mutations.
       - After `installWorkspaceRuntimeExecutable(root, storageDirName)`, call `if _, err := installWorkspaceRuntimeExecutable(root, toolsDirRel); err != nil { return fmt.Errorf("install workspace tool executable: %w", err) }` and `ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName)`.
  2. In `tests/integration/multi_session_workspace_containment_test.go`:
     * In `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`missing_local_binary_refuses_global_path_fallback`), remove the `.bin` binary installed by `u12InitBacklogitWorkspace(t, root)` (`require.NoError(t, os.RemoveAll(filepath.Join(root, ".bin")))`) so the subtest continues to verify that when the workspace's configured local binary is missing on disk, `PrewarmMCPDaemon` fails closed with `corerrors.ErrDaemonBinaryMissing` without falling back to `PATH`.
* **RED-first acceptance criteria**:
  * **RED harness (`tests/integration/multi_session_workspace_containment_test.go`)**: Add `TestU4_InitAndInstallPromptToolsDirAndDaemonStartup` (calling `cli.NewRootCommand`, `cli.PrewarmMCPDaemon`, and `cli.StopMCPDaemon`) with 3 `t.Run` scenarios:
    1. `interactive_and_piped_subprocess_prompt_options`: Verifies `"\n"` (default `.bin` and default `bin` when `bin/` exists), `"1\n"`, `"2\n"`, `"3\n.custom/tools\n"`, direct `"custom-tools/sub\n"` (including nonexistent first-init workspace root `<tempDir>/new-ws` and relative `--cwd`), and real piped subprocess execution (`exec.CommandContext` with `Stdin: strings.NewReader(".piped-tools/bin\n")`).
    2. `tools_dir_flag_and_end_to_end_daemon_startup`: Verifies `backlogit init --tools-dir .custom/bin` and `backlogit install --tools-dir .custom/bin` followed by `cli.PrewarmMCPDaemon` starting and stopping the workspace daemon from `.custom/bin/backlogit[.exe]`.
    3. `preflight_refuses_invalid_flag_prompt_ancestor_or_corrupt_config_before_mutation`: Verifies that `--tools-dir`, prompted, and default paths ALL run full ancestor-chain (`validateWorkspaceAndTargetAncestors`) and non-short-circuiting config/directory preflight (`defaultTargetToolsDir(cleanRoot)`) and fail closed with `corerrors.ErrValidation` (creating zero directories or files) when `--tools-dir` or prompt input is invalid (`""`, `"../escape"`, `".backlog"`, `"3\n\n"`), when any ancestor or target segment is a symlink/junction, or when an existing `.mcp.json`, `toolchain.yaml`, `config.yaml`, or `.bin`/`bin` entry is malformed or redirected — even when a valid `--tools-dir .bin` override is passed.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^(TestU4_InitAndInstallPromptToolsDir|TestU12_)' ./tests/integration`

### Unit U5: Regenerate CLI reference for `backlogit init --tools-dir` and enforce reference drift check (`218.005-T`)

* **Domain**: `docs` (width-isolated to `docs/cli-reference` and `cmd/gen-docs`)
* **Size / Complexity**: `S` / `low` (2 files, 1 function, 2 test scenarios)
* **Execution posture**: `test-first` (P-002.1 compile PASS + runtime assertion RED before `docs/cli-reference/backlogit_init.md` regeneration; because no production `.go` symbol is modified in U5, `in.Symbols.Empty()` is true in `redhonesty` and `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` fails at `baseSHA` before `docs/cli-reference/backlogit_init.md` is regenerated and passes at `headSHA`)
* **Dependencies**: `blocks` on Unit U4 (`218.004-T`) only
* **Affected files (2)**:
  * `docs/cli-reference/backlogit_init.md`
  * `cmd/gen-docs/main_test.go`
* **Changes needed**:
  1. Run `go run ./cmd/gen-docs docs/cli-reference` so `docs/cli-reference/backlogit_init.md` documents `--tools-dir` with its preserved `ingested_at` frontmatter.
  2. In `cmd/gen-docs/main_test.go`, add `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` verifying that `generateDocs(outDir)` emits `--tools-dir` in `backlogit_init.md` and that the committed `../../docs/cli-reference/backlogit_init.md` matches the generated `backlogit_init.md` byte-for-byte.
* **RED-first acceptance criteria**:
  * **RED harness (`cmd/gen-docs/main_test.go`)**: Add `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` with 2 `t.Run` scenarios:
    1. `generated_init_page_documents_tools_dir_flag`: `generateDocs(outDir)` produces `backlogit_init.md` documenting `--tools-dir`.
    2. `committed_init_page_matches_generated_output`: The committed `../../docs/cli-reference/backlogit_init.md` contains `--tools-dir` and equals the generated `backlogit_init.md` byte-for-byte (RED at `baseSHA` before `docs/cli-reference/backlogit_init.md` is regenerated, and GREEN at `headSHA`).
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^TestGenerateDocs_' ./cmd/gen-docs`

## Dependency Graph

* **Unit U1 (`218.001-T`)** (`internal/cli/self_install.go`, `internal/cli/self_install_test.go`): no upstream dependencies (Wave 1)
* **Unit U2 (`218.002-T`)** (`internal/cli/self_install.go`, `internal/cli/self_install_test.go`): `blocks` on Unit U1 (`218.001-T`) (Wave 2)
* **Unit U3 (`218.003-T`)** (`internal/cli/mcp_daemon.go`, `internal/cli/self_install_test.go`): `blocks` on Unit U1 (`218.001-T`) and Unit U2 (`218.002-T`) (Wave 3)
* **Unit U4 (`218.004-T`)** (`internal/cli/root.go`, `tests/integration/multi_session_workspace_containment_test.go`): `blocks` on Unit U3 (`218.003-T`) (Wave 4)
* **Unit U5 (`218.005-T`)** (`docs/cli-reference/backlogit_init.md`, `cmd/gen-docs/main_test.go`): `blocks` on Unit U4 (`218.004-T`) (Wave 5)

## Decisions and Rationale

1. **Reconciled CLI Contract (`--tools-dir` flag + TTY/piped subprocess stdin prompt + `cmd/gen-docs` regeneration, with zero unrequested env vars)**:
   * Honoring `--tools-dir` alongside interactive TTY and piped subprocess stdin prompting satisfies `103-DL` and `269D054D` with full human/agent parity without introducing unrequested environment variables (`BACKLOGIT_TOOLS_DIR`), while isolating `docs/cli-reference/backlogit_init.md` regeneration in Unit U5 (`218.005-T`) keeps every unit at strictly `2 files` (`<3 files`), `1`–`4 functions` (`<5 functions`), and `2`–`3 test scenarios` (`<4 scenarios`).
2. **Single `stdinShouldPrompt(cmd)` Helper for TTY and Real Piped Subprocess Stdin**:
   * `stdinShouldPrompt(cmd *cobra.Command) (bool, error)` checks `inFile, ok := in.(*os.File); isTTY := ok && term.IsTerminal(int(inFile.Fd())) && isTerminal(out)` for interactive terminals, `in != os.Stdin` for `cmd.SetIn` tests, and `os.Stdin.Stat()` (`fi.Mode()&os.ModeNamedPipe != 0 || (fi.Mode().IsRegular() && fi.Size() > 0)`) for real piped/redirected subprocesses (`echo "bin" | backlogit init`), while `os.DevNull` in `go test` (`ModeCharDevice != 0` and `!term.IsTerminal`) uses `defaultDir` without blocking.
3. **Canonical `defaultTargetToolsDir(cleanRoot string) (string, error)` with Full Ancestor Walk and Non-Short-Circuiting Source Validation on Every Path**:
   * `resolveInitTargetToolsDir` calls `defaultTargetToolsDir(cleanRoot)` unconditionally on **every** path (even when `--tools-dir` is explicitly passed) and calls `validateWorkspaceAndTargetAncestors(cleanRoot, cleanRel)` on the selected target directory. `validateWorkspaceAndTargetAncestors` walks every ancestor component from the filesystem/volume root down to `cleanRoot` and every existing segment of `cleanRel`, rejecting symlinks/reparse points. When `cleanRoot` exists, `defaultTargetToolsDir` validates all 5 sources (`.mcp.json`, `toolchain.yaml`, `config.yaml` with `validateDaemonYAMLKeys(&doc, "config.yaml")`, `.bin`, and `bin`) without short-circuiting so a malformed or redirected lower-priority source is never masked by a higher-priority source.
4. **Non-Short-Circuiting `os.SameFile` Self-Install and Single Mutation Ownership**:
   * When `os.SameFile(srcStat, dstInfo)` detects that the running executable is already the file at `<storage_dir>/backlogit[.exe]` or `<tools_dir>/backlogit[.exe]`, only the file-copy/rename step for that path is skipped; `.mcp.json` configuration, `toolchain.yaml` reconciliation, and `.gitignore`/`.git/info/exclude` updates still run to completion.
5. **Scope Discipline — No `config.yaml` Mutation; Explicit Justification for Ignore Rules and Canonical Typed `ToolchainManifest` Reconciliation**:
   * `init` and `install` do **not** mutate `<storage_root>/config.yaml` (`config.yaml` remains read-only fallback Priority 3 in `defaultTargetToolsDir` / `pinnedMCPDaemonBinary`). Updating `.gitignore` and `.git/info/exclude` with `/<tools_dir>/backlogit` and `/<tools_dir>/backlogit.exe` is directly necessary so installing `backlogit[.exe]` into a newly created target tool directory does not leave an untracked binary in the user's git working tree. When `<storage_root>/toolchain.yaml` already exists on disk, reconciling it via `toolchain.LoadToolchainManifest(absRoot)` + mutating `ToolsDir` and the single `backlogit` `ToolEntry` (`ExecutablePath`, `SHA256`) + `toolchain.WriteToolchainManifest(absRoot, manifest)` is directly necessary so `AuditRepositoryToolchain` (`backlogit mcp status --toolchain --strict`) does not fail after `backlogit install --tools-dir <dir>`.

## Risks and Caveats

* **Risk**: `redhonesty` (`internal/redhonesty/rules.go`) rejects a new test with `RejectNoChangedBranch` if the test's AST identifiers (`r.idents`) do not intersect `in.Symbols` (`ChangedSymbols`), with `RejectUnverifiableBaseline` if the test fails to compile at `baseSHA`, or with `RejectBuggyPathExpected` if a changed function's return value is assigned to a `want*`/`expect*`/`exp*`/`golden*` variable.
  * **Mitigation**: Units U1–U3 place `TestU1_*`..`TestU3_*` in `internal/cli/self_install_test.go` (`package cli`) calling pre-existing modified functions (`installWorkspaceRuntimeExecutable`, `ensureWorkspaceRuntimeIgnored`, `resolveConfiguredDaemonBinaryRel`); Unit U4 places `TestU4_*` in `tests/integration/multi_session_workspace_containment_test.go` calling modified `cli.NewRootCommand`; and Unit U5 modifies only `docs/cli-reference/backlogit_init.md` and `cmd/gen-docs/main_test.go` (`in.Symbols.Empty() == true`, failing at `baseSHA` before `backlogit_init.md` is regenerated and passing at `headSHA`).
* **Risk**: Symlink, Windows junction (`mklink /J`), or mid-flight directory/leaf swap on any ancestor of `cleanRoot`, the target tool directory, or `.mcp.json`.
  * **Mitigation**: Walk every ancestor component of `cleanRoot` (`validateWorkspaceAndTargetAncestors`), walk and create every segment of `toolsDirRel` on pinned `*os.Root` handles (`ensureAndOpenTargetToolsDir`) passing the explicit canonical absolute path to `core.IsSymlinkOrReparsePoint`, and publish both `<toolsDirRel>/backlogit[.exe]` and `.mcp.json` via `atomicfile.ReplaceTempFileInRoot` with pre-replace `Lstat` re-verification and `selfInstallBeforeBinaryReplaceHook` / `selfInstallBeforeIgnoreReplaceHook` test hooks.

## Constitution Check

* **Principle I — Safety-First Go (`MUST`)**: `pass` — All production code is in Go 1.24, wraps errors with `fmt.Errorf("context: %w", err)` preserving `corerrors.ErrValidation`, and uses zero `unsafe` constructs.
* **Principle II — Test-First Development (`NON-NEGOTIABLE`)**: `pass` — Every unit (`U1` through `U5`) begins with a compiling, failing RED test harness verified against `redhonesty` invariants before any production edit.
* **Principle III — Workspace Isolation and Security Boundaries (`MUST`)**: `pass` — All directory creations, binary copies, `.mcp.json` merges, `toolchain.yaml` reconciliations, and ignore-file updates use `os.OpenRoot(absRoot)`, pinned child `*os.Root` handles, `core.IsSymlinkOrReparsePoint`, `os.SameFile`, and `atomicfile.ReplaceTempFileInRoot`.
* **Principle IV — CLI Workspace Containment (`NON-NEGOTIABLE`)**: `pass` — All `init` and `install` operations resolve and write strictly inside the target workspace root (`--cwd` or `[path]`).
* **Principle V — Structured Observability (`MUST`)**: `pass` — `backlogit init` and `backlogit install` emit deterministic initialization confirmation and prompt output to `cmd.OutOrStdout()`, wrap all failure causes with actionable context and sentinel errors (`corerrors.ErrValidation`, `corerrors.ErrDaemonBinaryMissing`), and `_Ship` records red/green test evidence, ldflags-injected `HEAD` commit + SHA-256 runtime binary identity, and P-020 closure artifacts.
* **Principle VI — Single Responsibility (`SHOULD`)**: `pass` — Reuses existing standard library (`os`, `bufio`, `encoding/json`) and internal packages (`internal/atomicfile`, `internal/core`, `internal/toolchain`, `golang.org/x/term`, `gopkg.in/yaml.v3`) with zero new dependencies (`go.mod` unchanged).
* **Principle VII — Destructive Command Approval (`NON-NEGOTIABLE`)**: `pass` — No destructive commands or data removals are introduced; `.mcp.json` merging preserves all unrelated top-level and server entries via `map[string]json.RawMessage`.
* **Principle VIII — Explicit Safety Modes for Elevated Risk (`MUST`)**: `pass` — Operates under **Freeze-scope mode** (strictly confined to `internal/cli`, `tests/integration/multi_session_workspace_containment_test.go`, `cmd/gen-docs/main_test.go`, and `docs/cli-reference/backlogit_init.md` for stash `269D054D`) and **Careful mode** for workspace root and `.mcp.json` atomic replacement invariants (`I1`–`I5`).
* **Principle IX — Git-Friendly Persistence (`SHOULD`)**: `pass` — `.mcp.json` is formatted with deterministic 2-space indentation and trailing newline (`json.MarshalIndent` + `"\n"`), and `.gitignore` / `.git/info/exclude` entries are appended idempotently preserving line endings.
* **Principle X — Agent Context Efficiency (`SHOULD`)**: `pass` — Uses targeted `backlogit` MCP operations and bounded file reads (`maxMCPDaemonInfoBytes`).
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
  * `backlogit init [path] [--tools-dir <dir>]` and `backlogit install [path] [--tools-dir <dir>]` (`internal/cli/root.go`, `internal/cli/self_install.go`, `internal/cli/mcp_daemon.go`)
  * Workspace `.mcp.json` generation and `toolchain.yaml` reconciliation consumed by `backlogit mcp` (`PrewarmMCPDaemon`, `RunMCPSessionShim`) and `backlogit mcp status --toolchain`
* **Runtime verification probes (for `_Ship` Step 6 — MUST verify clean tree, build with ldflags injecting `HEAD` commit, verify JSON commit equality, and verify SHA-256 of installed binaries)**:
  1. **Verify clean tree and build `HEAD`-stamped verification binary (never use an unverified ambient `PATH` binary)**:
     * Assert `git status --porcelain` is empty.
     * Record `headSHA=$(git rev-parse HEAD)` and `buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)`.
     * Build `<tempBin>/backlogit[.exe]` injecting exact `HEAD` commit and build date into `internal/version`:
       `go build -ldflags "-X github.com/softwaresalt/backlogit/internal/version.Commit=$headSHA -X github.com/softwaresalt/backlogit/internal/version.BuildDate=$buildDate" -o <tempBin>/backlogit[.exe] ./cmd/backlogit`.
     * Run `<tempBin>/backlogit[.exe] version --no-update-check --format json` and assert the parsed JSON field `.commit == $headSHA`.
     * Compute and record `sourceSHA256` (`Get-FileHash <tempBin>/backlogit.exe -Algorithm SHA256` / `sha256sum`).
  2. **Verify interactive/piped prompt, `--tools-dir` provisioning, and installed binary SHA-256 equality**:
     * In isolated `--cwd <tempDir>` workspaces (including a brand-new nonexistent `<tempDir>/first-init` path), execute `<tempBin>/backlogit[.exe] init --cwd <tempDir> --tools-dir .custom-tools/bin` and pipe `"bin\n"` into `<tempBin>/backlogit[.exe] install --cwd <tempDir2>`.
     * Compute the SHA-256 digest of the installed `<tempDir>/.custom-tools/bin/backlogit[.exe]`, `<tempDir>/.backlog/backlogit[.exe]`, and `<tempDir2>/bin/backlogit[.exe]`, asserting each equals `sourceSHA256`, and verify `.mcp.json` and `.gitignore` are configured accurately.
  3. **Verify MCP daemon startup and commit identity from the provisioned workspace**:
     * Run `<tempBin>/backlogit[.exe] --cwd <tempDir> mcp prewarm --format json`, `<tempBin>/backlogit[.exe] --cwd <tempDir> mcp status --format json`, and `<tempBin>/backlogit[.exe] --cwd <tempDir> mcp stop --format json`, confirming the daemon starts from `<tempDir>/.custom-tools/bin/backlogit[.exe]` and reports the expected version.
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
  * **I3 (Lossless `.mcp.json` raw-JSON merge, alias/multi-entry detection, and canonical typed `ToolchainManifest` reconciliation)**: When `.mcp.json` already exists, `configureWorkspaceMCPJSON` MUST reject symlinks/reparse points and duplicate keys (`scanDaemonJSON`), unmarshal top-level and server maps as `map[string]json.RawMessage` to preserve unrelated values verbatim, inspect both `mcpServers` and `servers` using `isBacklogitDaemonCommand` (failing closed on `>1` matched `backlogit` entries or wrong-typed JSON fields), and update the single matched entry in place (or insert `mcpServers.backlogit`). When `<storage_root>/toolchain.yaml` exists, `reconcileWorkspaceToolchainYAML` MUST load it via `toolchain.LoadToolchainManifest(absRoot)` (enforcing `KnownFields(true)` and duplicate-key validation), update `ToolsDir` (if set) and the single `backlogit` `ToolEntry` (`ExecutablePath`, `SHA256`), and write via `toolchain.WriteToolchainManifest(absRoot, manifest)` without mutating `config.yaml`.
  * **I4 (Unconditional full-ancestor walk and non-short-circuiting `defaultTargetToolsDir(cleanRoot string) (string, error)` preflight on every path)**: `resolveInitTargetToolsDir` MUST call `defaultTargetToolsDir(cleanRoot)` and `validateWorkspaceAndTargetAncestors(cleanRoot, cleanRel)` on every path (`--tools-dir`, prompted, or default). `validateWorkspaceAndTargetAncestors` walks every ancestor component from the volume root down to `cleanRoot` and every existing segment of `cleanRel`. When `cleanRoot` exists, `defaultTargetToolsDir(cleanRoot)` validates all 5 sources (`.mcp.json`, `toolchain.yaml`, `config.yaml` with `validateDaemonYAMLKeys(&doc, "config.yaml")`, `.bin`, and `bin`) without short-circuiting, returning `corerrors.ErrValidation` on any malformed, wrong-typed, duplicate-keyed, or redirected source with no silent fallback.
  * **I5 (`redhonesty` P-002.1 compliance and `<3 files` / `<5 functions` / `<4 scenarios` granularity across `U1`–`U5`)**: Every unit (`U1`–`U5`) modifies strictly `2 files`, `1`–`4 functions`, and `2`–`3 test scenarios`. New tests in `U1`–`U4` call only symbols that already exist at that unit's `baseSHA` and directly reference at least one modified function in `in.Symbols.Funcs` (`installWorkspaceRuntimeExecutable` / `ensureWorkspaceRuntimeIgnored` in U1 and U2; `resolveConfiguredDaemonBinaryRel` in U3; `NewRootCommand` in U4), while `U5` has `in.Symbols.Empty() == true` and fails at `baseSHA` on the unregenerated `docs/cli-reference/backlogit_init.md`.
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
| Create or merge `.mcp.json` via `json.RawMessage` and reconcile canonical `ToolchainManifest` after binary install (`U2`) | `internal/cli/self_install.go`, `internal/cli/self_install_test.go` | `moderate` (MCP launch configuration write) | Verify `<tools_dir>/backlogit[.exe]` exists first; validate JSON/YAML for duplicate keys and types; detect aliases across `mcpServers` and `servers`; use `LoadToolchainManifest` + `WriteToolchainManifest` | `.mcp.json` points to `./<tools_dir>/backlogit[.exe]` with `--cwd <absRoot>` while preserving unrelated entries and reconciling `toolchain.yaml` |
| Implement non-short-circuiting `defaultTargetToolsDir(cleanRoot)` and full-ancestor preflight (`U3`) | `internal/cli/mcp_daemon.go`, `internal/cli/self_install_test.go` | `low` (fail-closed config and ancestor validation) | Parameterize `validateDaemonYAMLKeys(node, fileName)`; walk all ancestor components; validate all 5 sources before selecting winner | Corrupt/redirected configs or ancestors fail closed with accurate filename attribution and zero silent fallback |
| Wire `stdinShouldPrompt`, `resolveInitTargetToolsDir`, and `--tools-dir` into `newInitCommand` (`U4`) | `internal/cli/root.go`, `tests/integration/multi_session_workspace_containment_test.go` | `low` (CLI prompt, flag, & unconditional preflight) | Run `defaultTargetToolsDir(cleanRoot)` and `validateWorkspaceAndTargetAncestors` on every path before mutating workspace; align `TestU12` fixture | Operator and automated callers can select `.bin`, `bin`, or custom directory via prompt, pipe, or `--tools-dir` flag |
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
* **Gate rationale (Attempt 2 — superseded by Attempt 3 multi-agent-dispatch below)**: Post-cycle-1 single-agent rubric pass superseded by full 7-subagent `multi-agent-dispatch` in Attempt 3 below.

<!-- plan-review-attempt: 3 -->

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `TOOL_OK: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale (Attempt 3 — full 7-persona `multi-agent-dispatch` post-remediation of PR `#664` local review cycle 2)**: Dispatched all 7 required reviewer persona subagents (`Constitution Reviewer` [`plan-review-constitution`], `Go Reviewer` [`plan-review-go`], `Scope Boundary Auditor` [`plan-review-scope`], `Learnings Researcher` [`plan-review-learnings`], `Architecture Strategist` [`plan-review-architecture`], `Agent-Native Parity Reviewer` [`plan-review-parity`], and `Security Lens Reviewer` [`plan-review-security`]) via the `task` tool. All 7 subagents completed and returned `[]` (`0` findings). Plan hardening was required (`Requires plan hardening: yes`) and is satisfied by `## Plan Hardening` (Invariants `I1`–`I5` and `ProposedAction` / `ActionRisk` table). `## Constitution Check` maps Principles I–XI explicitly and concludes with literal `Constitution Check: pass`. Total merged findings across all 7 dispatched personas: `0` (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Per-persona `multi-agent-dispatch` findings & verification evidence**:
  * **Constitution Reviewer (`plan-review-constitution`)**: `[]` (`0` findings, `PASS`). Verified Principles I–XI (including Principle V Structured Observability and Principle VIII Freeze-scope / Careful safety modes) and Task Granularity across `U1` (`2` files, `3` funcs, `3` scenarios), `U2` (`2` files, `3` funcs, `3` scenarios), `U3` (`2` files, `4` funcs, `3` scenarios), `U4` (`2` files, `4` funcs, `3` scenarios), and `U5` (`2` files, `1` func, `2` scenarios), concluding with `Constitution Check: pass`.
  * **Go Reviewer (`plan-review-go`)**: `[]` (`0` findings, `PASS`). Verified canonical 1-arg `defaultTargetToolsDir(cleanRoot string) (string, error)`, combined `stdinShouldPrompt(cmd *cobra.Command) (bool, error)` (`term.IsTerminal(int(inFile.Fd())) && isTerminal(out)` + `cmd.SetIn` + `os.Stdin.Stat()` `ModeNamedPipe`/non-empty `IsRegular()`), unconditional preflight in `resolveInitTargetToolsDir`, parameterized `validateDaemonYAMLKeys(node, fileName)`, deterministic `*os.Root` handle closure, and non-short-circuiting `os.SameFile` handling.
  * **Scope Boundary Auditor (`plan-review-scope`)**: `[]` (`0` findings, `PASS`). Verified strict alignment with stash `269D054D` and deliberation `103-DL`, reconciled `--tools-dir` + prompt + `cmd/gen-docs` contract across all artifacts, confirmed zero `config.yaml` mutations and zero unrequested environment variables, and verified explicit necessity justifications for `.gitignore`/`.git/info/exclude` and canonical typed `ToolchainManifest` reconciliation.
  * **Learnings Researcher (`plan-review-learnings`)**: `[]` (`0` findings, `PASS`, `confidence: high`, `137` files in `docs/compound/` searched). Verified incorporation of `2026-10-27-pin-the-handle-not-the-path.md`, `nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`, `2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`, `2026-08-29-windows-preremove-rename-pattern.md`, `derive-daemon-version-from-pinned-image-2026-11-09.md`, `cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`, and `2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md`.
  * **Architecture Strategist (`plan-review-architecture`)**: `[]` (`0` findings, `PASS`). Verified single ownership of mutations (`installWorkspaceRuntimeExecutable`, `configureWorkspaceMCPJSON`, `reconcileWorkspaceToolchainYAML`, `ensureWorkspaceRuntimeIgnored`), canonical `toolchain.LoadToolchainManifest` + `toolchain.WriteToolchainManifest` pipeline, and linear 5-wave sequence (`U1 -> U2 -> U3 -> U4 -> U5`).
  * **Agent-Native Parity Reviewer (`plan-review-parity`)**: `[]` (`0` findings, `PASS`). Verified symmetric human TTY prompt, real piped subprocess stdin, `--tools-dir` flag, and non-interactive default discovery, with identical preflight safety boundaries and lossless `map[string]json.RawMessage` `.mcp.json` merging across `mcpServers` and `servers`.
  * **Security Lens Reviewer (`plan-review-security`)**: `[]` (`0` findings, `PASS`). Verified 3-threat model: (1) traversal/reserved/malformed/duplicate-key/split-server inputs fail closed via `validateDaemonToolsDirRel`, `scanDaemonJSON`, parameterized `validateDaemonYAMLKeys(node, fileName)`, and non-short-circuiting `defaultTargetToolsDir(cleanRoot)`; (2) symlink/junction swaps on any ancestor or target segment are blocked by `validateWorkspaceAndTargetAncestors`, pinned `*os.Root` handles + `os.SameFile`, and `atomicfile.ReplaceTempFileInRoot`; (3) `os.SameFile` never skips `.mcp.json`/`toolchain.yaml` provisioning, and runtime verification enforces clean-tree `-ldflags` `HEAD` commit injection, JSON `.commit` equality, and installed binary SHA-256 verification.
* **Runtime verification & operational closure check**: Verified complete in `## Runtime Verification and Closure` with clean-tree `-ldflags` `HEAD` commit injection, JSON `.commit` equality, and installed binary SHA-256 verification.
