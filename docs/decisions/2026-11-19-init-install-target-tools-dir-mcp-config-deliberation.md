---
chunk_strategy: h1-h2-h3
decision_status: decided
depth: standard
description: 'Deliberation for stash 269D054D (103-DL): prompt for target tool directory (.bin, bin, or custom workspace-relative directory) on backlogit init and install, create the directory, install backlogit[.exe] there, and configure .mcp.json to use it.'
doc_type: decision
linked_artifacts:
  - .backlogit/archive/103-DL.md
  - .backlogit/queue/218-F.md
  - .backlogit/queue/218.001-T.md
  - .backlogit/queue/218.002-T.md
  - .backlogit/queue/218.003-T.md
  - .backlogit/queue/218.004-T.md
  - .backlogit/queue/218.005-T.md
  - .backlogit/queue/219-S.md
  - docs/plans/2026-11-19-init-install-target-tools-dir-mcp-config-plan.md
promoted_to: both
schema_version: "1.0"
source: docs/decisions/2026-11-19-init-install-target-tools-dir-mcp-config-deliberation.md
tags:
  - init
  - install
  - mcp-json
  - tools-dir
  - self-install
title: 'Deliberation: Target Tool Directory Prompt, Binary Installation, and .mcp.json Configuration on init and install (269D054D)'
topic: 'Prompt for target tool directory, install backlogit binary, and configure .mcp.json on init and install (269D054D / 103-DL)'
---

## Problem Frame

Currently, `backlogit init` and `backlogit install` (`internal/cli/root.go` and `internal/cli/self_install.go`) copy the running executable only into the workspace storage directory (`.backlog/backlogit[.exe]` or `.backlogit/backlogit[.exe]`). Following `217-F` (`218-S`), `pinnedMCPDaemonBinary` (`internal/cli/mcp_daemon.go`) and static toolchain verification (`internal/toolchain/verify.go`, `internal/toolchain/launcher.go`) treat `.backlog` and `.backlogit` as reserved storage roots and require the workspace-pinned `backlogit` binary to reside in a workspace-relative target tool directory (such as `.bin`, `bin`, or a custom `tools_dir`) and be wired into `.mcp.json`.

As reported in high-priority feature stash `269D054D`:

1. **Missing Target Tool Directory Selection on `init` / `install`**: `backlogit init` and `backlogit install` do not prompt the operator for a target tool install directory (offering `.bin`, `bin`, or an alternate user-entered workspace-relative directory) or accept an explicit `--tools-dir` flag for non-interactive parity.
2. **Missing Target Tool Directory Creation and Binary Installation**: If the chosen target tool directory (e.g., `.bin`, `bin`, or a custom subdirectory like `.tools/bin`) does not yet exist in the target workspace, `init` / `install` does not create it or install `backlogit.exe` (`backlogit` on POSIX) into that target tool directory.
3. **Missing `.mcp.json` and Ignore Configuration**: `init` / `install` does not configure `.mcp.json` in the target workspace (or tool-binary ignore rules in `.gitignore` / `.git/info/exclude` so the newly installed workspace binary does not dirty the git working tree) after creating the target tool directory and installing `backlogit[.exe]` into it.

## Stash Triage and Duplicate Audit

| Stash ID | Kind / Priority | Source Refs | Duplicate Scan Outcome | Session Disposition |
| --- | --- | --- | --- | --- |
| `269D054D` | `feature` / `high` | Operator intake (`269D054D`) | **CLEAN DUPLICATE SCAN** across `.backlogit/stash.jsonl` (`0` duplicates) | **Selected (`103-DL`)** for planning, harvest, and shipment assembly |

## Research Findings

### Prior Institutional Learnings (`docs/compound/`)

* `docs/compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md`: `PrewarmMCPDaemon` and `RunMCPSessionShim` require a regular workspace-pinned binary at the path declared in `.mcp.json` (or `toolchain.yaml` / `config.yaml` `tools_dir` / `.bin` / `bin`).
* `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md` and `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`: All target tool directory creation, binary installation, and `.mcp.json` / `config.yaml` writes must descend through pinned `*os.Root` directory handles while threading the explicit canonical absolute path for `core.IsSymlinkOrReparsePoint` checks (`Lstat` -> `IsSymlinkOrReparsePoint` -> `OpenRoot` -> `Stat(".")` + `os.SameFile`) and publishing via `atomicfile.ReplaceTempFileInRoot`.
* `docs/compound/2026-10-16-root-confined-access-and-graphql-reviewer-union.md`: Custom user-entered tool directories must be validated as workspace-local relative paths (`filepath.IsLocal`, not `.`, no `..` traversal, no volume/UNC prefix, and top-level segment not `.git`, `.backlog`, or `.backlogit`) and opened through `os.OpenRoot`.
* `docs/compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`: Normalize workspace roots with `filepath.Abs` and exercise CLI `init` / `install` with relative `--cwd` paths in tests.
* `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`: Use atomic temp-file write + `Sync()` + `atomicfile.ReplaceTempFileInRoot` without a pre-`Remove`.
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`: Adding `--tools-dir` to `backlogit init` requires regenerating `docs/cli-reference/backlogit_init.md` via `go run ./cmd/gen-docs docs/cli-reference` so the CI `CLI Reference Drift` gate passes.

### Codebase Analysis

* `internal/cli/root.go` (`newInitCommand`, lines 291-368) defines `backlogit init [path]` with `Aliases: []string{"install"}`.
* `internal/cli/self_install.go` implements `resolveInstallStorageDir`, `openValidatedWorkspaceAndStorageRoots`, `installWorkspaceRuntimeExecutable`, and `ensureWorkspaceRuntimeIgnored`.
* `internal/toolchain/verify.go` (`validateMCPConfigPinning`, lines 725-835) and `internal/toolchain/launcher.go` (`RenderPinnedMCPConfig`, lines 1020-1045) require `.mcp.json`'s `mcpServers.backlogit` entry to specify `"type": "stdio"`, `"command": "./<tools_dir>/<binName>"` (POSIX slash format), and `"args": ["mcp", "--cwd", "<absRoot>"]`.
* `internal/cli/init_workspace_dir_test.go` (`TestU6_InitAndInstallSelfInstallRuntimeExecutableAndIgnoreEntries`) asserts that non-interactive `backlogit init --cwd <root>` and `backlogit install --cwd <root>` install `<storage_root>/backlogit[.exe]`, keep `bin/backlogit.exe` absent when default `.bin` is used, and write idempotent ignore rules.
* `tests/integration/multi_session_workspace_containment_test.go` (`TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` / `missing_local_binary_refuses_global_path_fallback`) calls `u12InitBacklogitWorkspace(t, root)` and tests that when a workspace's local binary is missing on disk, `PrewarmMCPDaemon` fails closed with `corerrors.ErrDaemonBinaryMissing` rather than falling back to `PATH`.

## Options Evaluated

### Option A: Hard-code `.bin/` directory creation and `.mcp.json` generation without operator prompt or custom directory support

* **Description**: Always create `.bin/`, copy `backlogit[.exe]` to `.bin/`, and write `.mcp.json` pointing to `./.bin/backlogit[.exe]` with no prompt or custom option.
* **Pros**: Minimal implementation surface.
* **Cons**: Fails the explicit requirement in `269D054D` to prompt the operator with a default target install directory of `.bin`, `bin`, or allow the user to type in an alternate directory option.
* **Effort estimate**: Low.
* **Fit**: Poor — does not satisfy the operator requirement.

### Option B: Interactive/piped prompt + `--tools-dir` flag for `.bin`, `bin`, or custom workspace-relative tool directory, `*os.Root` directory creation and binary install prior to `.mcp.json` configuration (Recommended)

* **Description**:
  1. **Target Tool Directory Resolution, Handle-Relative Root Walk, Non-Short-Circuiting Discovery, & Prompt (`internal/cli/mcp_daemon.go` + `internal/cli/root.go`)**:
     * Support `--tools-dir` on `backlogit init` / `backlogit install` for non-interactive CLI/agent parity (without introducing unrequested environment variables such as `BACKLOGIT_TOOLS_DIR`).
     * **Component validation from a trusted anchor root and handle-relative creation of missing suffixes (`openOrCreateValidatedWorkspaceRoot(cleanRoot string, createIfMissing bool) (string, *os.Root, error)`)**:
       - Resolve `absRoot, err := filepath.Abs(filepath.Clean(cleanRoot))` and open a pinned `*os.Root` handle at the volume/filesystem root anchor (`anchorDir := filepath.VolumeName(absRoot) + string(filepath.Separator)` on Windows, `"/"` on POSIX).
       - Walk **every path component** of `relFromAnchor` step-by-step from `current := anchorRoot`: for each component `part`, inspect `info, err := current.Lstat(part)`. If `errors.Is(err, os.ErrNotExist)`: when `!createIfMissing` (read-only preflight), return `("", nil, os.ErrNotExist)` after having verified every existing ancestor component via pinned handles; when `createIfMissing`, create the missing suffix component **handle-relatively on the pinned parent handle** via `current.Mkdir(part, 0o755)` (never via path-based `os.MkdirAll` after an `Lstat` check, which would follow an ancestor symlink/junction swap on `link/existing/new`) and re-run `info, err = current.Lstat(part)`. At every component, verify `!core.IsSymlinkOrReparsePoint(info, nextAbs)` and `info.IsDir()`, open `next, err := current.OpenRoot(part)`, and verify `opened, statErr := next.Stat(".")` with `os.SameFile(info, opened)`.
     * **Unconditional read-only preflight of every recognized config and directory source (`defaultTargetToolsDir(cleanRoot string) (string, error)` / `resolveConfiguredDaemonBinaryRel`)**:
       - Run `defaultTargetToolsDir(cleanRoot)` unconditionally on **every** execution path (including when `--tools-dir` is explicitly passed) before any workspace directory creation or file mutation.
       - When `cleanRoot` does not yet exist on disk (`os.ErrNotExist` from `openOrCreateValidatedWorkspaceRoot(cleanRoot, false)` after validating all existing ancestor components), return `(".bin", nil)`.
       - When `cleanRoot` exists, validate **every present source and candidate directory without short-circuiting**:
         (a) `.mcp.json` (`resolveMCPJSONDaemonBinaryRel`, inspecting both `mcpServers` and `servers`, enforcing `scanDaemonJSON` duplicate-key rejection, rejecting non-stdio transport conflicts such as non-empty `url` or `type != "stdio"`, and validating any matched `backlogit` command),
         (b) `<storage_root>/toolchain.yaml` (override-aware via `resolveDaemonStorageDirName` so `BACKLOGIT_WORKSPACE_DIR=.backlog` or `.backlogit` is honored when both roots exist while dual roots without an override are refused; enforcing full typed `*toolchain.ToolchainManifest` validation with `validateDaemonYAMLKeys(&doc, "toolchain.yaml")`, `decoder.KnownFields(true)`, and all manifest publication constraints from `validateManifestForProvisioning` during read-only preflight before any write),
         (c) `<storage_root>/config.yaml` (`resolveConfigDaemonBinaryRel` using parameterized `validateDaemonYAMLKeys(&doc, "config.yaml")` and strict `!!str` scalar type checking on `tools_dir`),
         (d) `.bin` on `wsRoot` (`wsRoot.Lstat(".bin")`: if present, must be a real non-redirected directory), and
         (e) `bin` on `wsRoot` (`wsRoot.Lstat("bin")`: if present, must be a real non-redirected directory).
       - If **any** present source (a)–(e) is invalid, malformed, unknown-fielded, wrong-typed, duplicate-keyed, escaping, or a symlink/reparse point/regular file, return `corerrors.ErrValidation` fail-closed with **zero filesystem mutations and no silent fallback** (even if a higher-priority source or `--tools-dir` override is valid).
       - Only after all present sources (a)–(e) validate cleanly, select the default directory in precedence order: (a) `.mcp.json` -> (b) `toolchain.yaml` -> (c) `config.yaml` -> (d) `".bin"` (when `.bin` exists or neither `.bin` nor `bin` exists) -> (e) `"bin"` (when `bin` exists and `.bin` does not).
     * When `--tools-dir` is not supplied and input should prompt (`stdinShouldPrompt(cmd)`), prompt the operator on `cmd.OutOrStdout()` offering `.bin`, `bin`, or an alternate workspace-relative directory.
     * Validate the selected target directory (whether from `--tools-dir`, prompt, or `defaultDir`) via `validateDaemonToolsDirRel` (rejecting `\r`, `\n`, and ASCII control characters in addition to traversal, volume/UNC, `.`, and reserved `.git`/`.backlog`/`.backlogit` segments) and verify every existing segment of `cleanRel` on `wsRoot` before any directory creation or file mutation.
  2. **Root-Confined Target Tool Directory Creation, Binary Installation, and Escaped Gitignore Rules (Prior to `.mcp.json`)**:
     * On the pinned `wsRoot *os.Root` handle (and creating `<storageDirName>` and `<storageDirName>/logs` handle-relatively via `wsRoot.Mkdir` / `storageHandle.Mkdir` rather than `os.MkdirAll`), walk each segment of `toolsDirRel` step-by-step via pinned child `*os.Root` directory handles (`ensureAndOpenTargetToolsDir`): create missing directory segments via `current.Mkdir(part, 0o755)`, verify `Lstat` + `core.IsSymlinkOrReparsePoint(info, nextAbs)` + `IsDir()` + `OpenRoot` + `Stat(".")` + `os.SameFile` at every segment, and install `backlogit[.exe]` into the innermost pinned `toolsDirHandle` via temp file + `Chmod(0o755)` + `Sync()` + `atomicfile.ReplaceTempFileInRoot`.
     * When the running executable is already identical (`os.SameFile(srcStat, dstInfo)`) to `<storage_root>/backlogit[.exe]` or `<toolsDirRel>/backlogit[.exe]`, skip only the file-copy/rename for that destination and **never short-circuit** target tool directory provisioning, `.mcp.json` configuration, `toolchain.yaml` reconciliation, or ignore-file updates.
     * In `ensureWorkspaceRuntimeIgnored`, escape Gitignore glob/comment metacharacters (`\`, `*`, `?`, `[`, `]`, `#`, `!`, trailing spaces) via `escapeGitignoreLiteralPath` when constructing `/<escapedToolsDir>/backlogit` and `/<escapedToolsDir>/backlogit.exe` so literal directories such as `tools[1]` are matched accurately by Git.
  3. **Canonical `.mcp.json` `mcpServers.backlogit` Provisioning, Override-Aware Typed `ToolchainManifest` Reconciliation, and CLI Reference Regeneration**:
     * Only after verifying via `openLocalFileInRoot(wsRoot, absRoot, filepath.Join(toolsDirRel, binName))` that `<toolsDirRel>/backlogit[.exe]` exists as a regular non-redirected file in the target workspace, create or update `.mcp.json` atomically via `wsRoot` (`atomicfile.ReplaceTempFileInRoot`): validate with `scanDaemonJSON`, unmarshal top-level and server maps as `map[string]json.RawMessage` to preserve unrelated top-level keys and sibling server entries verbatim, inspect both `mcpServers` and `servers` using `foldDaemonKey` and `isBacklogitDaemonCommand` (failing closed on `>1` matched `backlogit` entries, conflicting non-stdio `url`/`type` fields, or wrong-typed JSON fields), and **canonicalize the single matched entry into `mcpServers.backlogit`** (removing any alias key such as `"backlog-alias"` or `"servers"`-only `backlogit` entry and removing `"servers"` if it becomes empty, while setting `"type": "stdio"`, `"command": "./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))`, and `"args": ["mcp", "--cwd", absRoot]`) so `ValidateMCPConfigPinning` and `AuditRepositoryToolchain` pass with zero `uninventoried_shadow_tool` violations.
     * **Explicit scope justification for persistent side effects**:
       - **`config.yaml` and environment variables**: `<storage_root>/config.yaml` is never mutated by `init`/`install` (read-only during `defaultTargetToolsDir`), and no unrequested environment variable (`BACKLOGIT_TOOLS_DIR`) is added.
       - **Override-aware `<storage_root>/toolchain.yaml` reconciliation (when already present)**: Using the override-aware resolved `storageDir` (`resolveDaemonStorageDirName`, honoring `BACKLOGIT_WORKSPACE_DIR=.backlog` or `.backlogit` when both roots exist while refusing dual roots without an override), if `<storageDir>/toolchain.yaml` exists on disk, `reconcileWorkspaceToolchainYAML` loads and validates the typed `*toolchain.ToolchainManifest` on the pinned `storageHandle` (`KnownFields(true)` + `validateDaemonYAMLKeys` + manifest publication constraints), updates `manifest.ToolsDir = filepath.ToSlash(toolsDirRel)` (if non-empty) and the single `backlogit` `ToolEntry.ExecutablePath = "./" + filepath.ToSlash(filepath.Join(toolsDirRel, binName))` and **unconditionally populates `SHA256`** with the hex SHA-256 digest of the installed `<toolsDirRel>/<binName>` binary, re-validates all provisioning constraints, and atomically writes `<storageDir>/toolchain.yaml` on `storageHandle` via `atomicfile.ReplaceTempFileInRoot`.
       - **`.gitignore` and `.git/info/exclude` ignore rules**: Extending `ensureWorkspaceRuntimeIgnored` with escaped literal `/<escapedToolsDir>/backlogit` and `/<escapedToolsDir>/backlogit.exe` patterns prevents the newly installed `<toolsDirRel>/backlogit[.exe]` binary from dirtying `git status`.
     * Regenerate `docs/cli-reference/backlogit_init.md` via `go run ./cmd/gen-docs docs/cli-reference` in Unit U5 (`218.005-T`), seeding `outDir/backlogit_init.md` from the committed page in `TestGenerateDocs_InitToolsDirReferenceMatchesCommittedPage` before calling `generateDocs(outDir)` so `ingested_at` is preserved and byte-for-byte comparison is exact.
* **Pros**: Satisfies every requirement of `269D054D`, maintains human/agent CLI parity (`--tools-dir` + TTY/piped prompt), enforces full ancestor and non-short-circuiting config validation on every path, reuses canonical `ToolchainManifest` load-mutate-validate-write, and decomposes cleanly into 5 atomic `2-file` units (`218.001-T`..`218.005-T`).
* **Cons**: Spans `internal/cli`, `tests/integration`, `cmd/gen-docs`, and `docs/cli-reference` across 5 sequential waves.
* **Effort estimate**: Medium.
* **Fit**: Excellent.

### Option C: Delegate `init` / `install` tool-directory setup to external shell scripts (`start.ps1` / `install.ps1`)

* **Description**: Have `backlogit init` shell out to PowerShell or POSIX shell scripts to prompt and write `.mcp.json`.
* **Pros**: Avoids Go prompt code.
* **Cons**: Breaks self-contained single-binary CLI execution, introduces external shell dependencies during `backlogit init`, and loses `*os.Root` handle-confined safety guarantees.
* **Effort estimate**: Medium.
* **Fit**: Unacceptable.

## Trade-off Comparison

| Criterion | Option A (Hard-coded `.bin`) | Option B (Prompt + `--tools-dir` + `*os.Root` install + `.mcp.json`) | Option C (External shell script) |
| --- | --- | --- | --- |
| Prompts for `.bin`, `bin`, or custom directory | No | Yes (plus `--tools-dir` flag for non-interactive parity) | Partial |
| Creates target tool dir and installs binary prior to `.mcp.json` | Yes (`.bin` only) | Yes (`.bin`, `bin`, or any valid custom relative subdirectory) | Fragile |
| `*os.Root` handle-confined TOCTOU/symlink safety | Yes | Yes (every segment and leaf verified via `*os.Root`) | No |
| Backward compatibility with `TestU6` and `TestU12` | Partial | 100% green | Fails |

## Decision

Adopt **Option B** (`103-DL`) and promote directly to implementation planning (`docs/plans/2026-11-19-init-install-target-tools-dir-mcp-config-plan.md`) and backlog harvest.

## Rejected Alternatives

* **Option A** was rejected because it omits the operator prompt and custom target tool directory support required by `269D054D`.
* **Option C** was rejected because external shell execution breaks single-binary portability and `*os.Root` workspace confinement.

## Unresolved Questions

None.

## Risks and Mitigations

* **Risk**: Symlink, Windows junction (`mklink /J`), or mid-flight directory/leaf swap on the target tool directory or `.mcp.json`.
  * **Mitigation**: Walk and create every segment of `toolsDirRel` on pinned `*os.Root` handles (`Lstat` -> `IsSymlinkOrReparsePoint` with explicit canonical absolute path -> `Mkdir` if absent -> `OpenRoot` -> `Stat(".")` + `os.SameFile`), and publish both `<toolsDirRel>/backlogit[.exe]` and `.mcp.json` via `atomicfile.ReplaceTempFileInRoot` with pre-replace `Lstat` re-verification and deterministic swap hooks.
* **Risk**: `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`missing_local_binary_refuses_global_path_fallback`) calls `u12InitBacklogitWorkspace(t, root)` and expects `PrewarmMCPDaemon` to fail with `ErrDaemonBinaryMissing`.
  * **Mitigation**: Because `backlogit init` now installs `.bin/backlogit[.exe]` and configures `.mcp.json`, update `missing_local_binary_refuses_global_path_fallback` in `tests/integration/multi_session_workspace_containment_test.go` to remove the installed `.bin` binary after `u12InitBacklogitWorkspace(t, root)` so the test genuinely exercises a workspace whose configured local binary is missing on disk.
* **Risk**: Adding `--tools-dir` to `backlogit init` causes CI `CLI Reference Drift` if `docs/cli-reference/backlogit_init.md` is not regenerated.
  * **Mitigation**: Include a dedicated unit to regenerate `docs/cli-reference/backlogit_init.md` via `go run ./cmd/gen-docs docs/cli-reference` and verify it in `cmd/gen-docs/main_test.go`.
