---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for Group 1B Part 1 (39F868B1 / 105-DL + EF372865 / 106-DL -> 220-F / 221-S): normalize forward and backslashes in MCP validateSessionPath on Windows and document optional tools_dir, .bin default, and init/install --tools-dir provisioning.'
doc_type: plan
ingested_at: "2026-11-21T00:55:00Z"
schema_version: "1.0"
source: docs/plans/2026-11-20-mcp-session-path-and-tools-dir-docs-plan.md
title: 'Plan: MCP validateSessionPath Windows Separator Normalization and Toolchain tools_dir Documentation (220-F / 221-S)'
---

## Problem Frame

As established in [the post-219-S stabilization deliberation](../decisions/2026-11-20-post-219s-stabilization-deliberation.md) (`105-DL` for `39F868B1` and `106-DL` for `EF372865`), two self-contained post-`219-S` product and documentation stabilization items are ready to ship immediately after `220-S`:

1. **Windows Forward-Slash Differential in `(*Server).validateSessionPath` (`internal/mcp/session_mux.go:384`, `39F868B1` / `105-DL`)**: `validateSessionPath` currently normalizes only backslashes (`strings.ReplaceAll(strings.TrimSpace(path), "\\", string(filepath.Separator))`). On Windows (`filepath.Separator == '\\'`), forward slashes `'/'` remain unnormalized when `strings.Split(path, string(filepath.Separator))` splits segments:
   * `"sub/../existing.md"` and `"junction/../existing.md"` pass `filepath.IsLocal` (`true` on Go 1.24), stay unsplit as a single element (bypassing `if component == ".."`), and collapse lexically inside `filepath.Join(".", path)` to `"existing.md"` without rejecting parent traversal or inspecting `"sub"` / `"junction"` for symlink/reparse status (returning `nil` from `validateSessionPath` and `IsError: false` over `ServeSessionStream`!).
   * `"junction/missing.md"` (where `junction` is a Windows NTFS directory junction pointing outside the workspace) stays unsplit, skips explicit per-ancestor `root.Lstat("junction")` + `core.IsSymlinkOrReparsePoint`, and causes `root.Lstat("junction\\missing.md")` on Go 1.24 Windows to return an unwrapped `errPathEscapes` error rather than `corerrors.ErrValidation` (`"MCP tool path traverses a symlink or reparse point"`).
2. **Undocumented `tools_dir`, `.bin` Default, and `init`/`install --tools-dir` Provisioning (`EF372865` / `106-DL`)**: Following `218-S` (`217-F`) and `219-S` (`218-F`, PR `#665`), `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` still document only manual `./bin/*` setup and omit optional `tools_dir` in `config.yaml` and `toolchain.yaml`, the 5-source discovery precedence and `.bin` default (with `bin` fallback), `VerifyProposedLauncherScripts`, and `backlogit init` / `install` `[--tools-dir <dir>]` automated provisioning.

## Requirements Trace

| Requirement | Stash / Deliberation | Implementation Unit | Verification Target |
| --- | --- | --- | --- |
| R1: Normalize both `'/'` and `'\\'` in `(*Server).validateSessionPath` (`internal/mcp/session_mux.go`) and extract/expose package-internal OS-parameterized `(*Server).validateSessionPathForOS(path, goos string) error` before OS-aware locality, component splitting, `'..'` traversal rejection, and per-component `root.Lstat` + `core.IsSymlinkOrReparsePoint` inspection, verified on both Ubuntu CI (`test`) and Windows runs without `.github/workflows/ci.yml` edits | `39F868B1` / `105-DL` | Unit U1 (`220.001-T`, Wave 1) | `TestU1_ValidateSessionPathMixedAndForwardSeparators` (`go test -timeout=30m -count=1 -race -v -run '^TestU1_ValidateSessionPathMixedAndForwardSeparators$' ./internal/mcp`) |
| R2: Document optional `tools_dir` in `.backlogit/config.yaml` and `.backlogit/toolchain.yaml`, 5-source discovery precedence and `.bin` default (`bin` fallback), `VerifyProposedLauncherScripts`, and `backlogit init`/`install` `--tools-dir` provisioning (updating superseded `bin`-only prose) in `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` without touching generated `docs/cli-reference/*.md` | `EF372865` / `106-DL` | Unit U2 (`220.002-T`, Wave 1) | P-002.3 `EXEMPT_VERIFY_OK:220.002-T` + `go run ./cmd/backlogit docs lint` |

## Closed P-002.1 Harness-Exempt Set

The closed `harness-exempt` (`docs-only`) set for Feature `220-F` / Shipment `221-S` is:

* `220.002-T` (`U2`).

`220.001-T` (`U1`) is `harness-ready` (`test-first`).

## Implementation Units

### Unit U1: Normalize `'/'` and `'\\'` in `validateSessionPath` and expose OS-parameterized `validateSessionPathForOS` (`220.001-T`, Wave 1)

* **Domain**: `code` (width-isolated to `internal/mcp`)
* **Size / Complexity**: `S` / `low` (2 files, 3 production functions/helpers + 1 test function, 3 test scenarios)
* **Execution posture**: `test-first` (`P-002.1` compile PASS + runtime assertion RED at `baseSHA` on **both** Windows and Linux; `TestU1_ValidateSessionPathMixedAndForwardSeparators` in `package mcp` directly calls `(*Server).validateSessionPath`, `ServeSessionStream`, and `validateSessionPathForOS` via generic interface assertion so Ubuntu CI and Windows runs both execute the Windows `'/'` + `'..'` separator-normalization branch without `.github/workflows/ci.yml` edits)
* **Affected files (`## File Budget`: 2)**:
  * `internal/mcp/session_mux.go`
  * `internal/mcp/session_mux_test.go`
* **Changes needed**:
  1. In `internal/mcp/session_mux.go` (`(*Server).validateSessionPath`, line 384):
     * Delegate `(*Server).validateSessionPath(path string) error` to a package-internal OS-parameterized method `(*Server).validateSessionPathForOS(path, goos string) (result error)` called with `runtime.GOOS`.
     * In `(*Server).validateSessionPathForOS(path, goos string)`:
       - Select target separator `sep := "/"` (or `sep = "\\"` when `goos == "windows"`) and normalize **both** `'/'` and `'\\'` to `sep` using package-level `strings.NewReplacer("/", "\\", "\\", "\\")` (when `goos == "windows"`) or `strings.NewReplacer("/", "/", "\\", "/")` (when `goos != "windows"`) on `strings.TrimSpace(path)`.
       - Validate OS-aware locality (`isLocalSessionPathForOS(normalized, goos)`):
         1. **Unconditionally enforce base POSIX locality for every `goos`** (regardless of `runtime.GOOS`, so `validateSessionPathForOS("/abs/escape.md", "linux")` and `validateSessionPathForOS("\\abs\\escape.md", "linux")` return `corerrors.ErrValidation` on Windows hosts too!): reject empty string `""`, NUL bytes `\x00`, leading `'/'` or `'\\'`, and lexical `..` escape;
         2. **Layer Windows locality rules whenever `goos == "windows"`** (even when executed on an Ubuntu Linux CI host where `filepath.IsLocal` uses POSIX rules!): reject Windows drive/volume/UNC prefixes (`len >= 2 && normalized[1] == ':'`) and Windows reserved device segment stems (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, case-insensitive before optional `.ext`); and
         3. **Check `filepath.IsLocal(normalized)` when `goos == runtime.GOOS`**.
       - Split `components := strings.Split(normalized, sep)` and check every component for `component == ".."` (rejecting `"sub/../existing.md"`, `"junction/../existing.md"`, and `"sub\\../existing.md"` with `corerrors.ErrValidation` under **both** `goos == "windows"` and `goos == "linux"` before `filepath.Join` can collapse `..`). Keep explicit `absRoot, err := filepath.Abs(s.RootPath)` normalization so relative `Server.RootPath` configurations remain safe.
       - In the `root.Lstat` component loop, skip **only** empty components from redundant/trailing separators (`if component == "" { continue }`) after locality and the `..` check have run (never skipping `component == "."`, so `validateSessionPath(".")` continues to inspect `root.Lstat(".")` + `core.IsSymlinkOrReparsePoint(info, absRoot)`), joining each split segment via host `filepath.Join(relative, component)` so `root.Lstat(relative)` and `core.IsSymlinkOrReparsePoint(info, filepath.Join(absRoot, relative))` work identically on both Linux and Windows hosts for `"junction/missing.md"`, `"junction\\missing.md"`, `"sub/junction/missing.md"`, and `"junction/"`.
  2. In `internal/mcp/session_mux_test.go` (`package mcp`):
     * Add `TestU1_ValidateSessionPathMixedAndForwardSeparators` (allocating short `ws` and `ext` directories inside parent `t.TempDir()` with explicit `t.Cleanup` teardown so Windows `cmd /c mklink /J` stays well below Win32 `MAX_PATH`, never binding changed-function results to `want*`/`expect*`/`exp*`/`golden*` variables, and resolving `validateSessionPathForOS(path, goos string) error` via a justified generic interface helper `asOSPathValidator[T any](v T)` so `TestU1` compiles cleanly at `baseSHA` under `redhonesty` while failing runtime assertions on **both** Linux and Windows baselines!) with 3 `t.Run` scenarios:
       1. `dotdot_traversal_cross_os`: Verifies `s.validateSessionPath` **and** `osValidator.validateSessionPathForOS(p, "windows")` + `osValidator.validateSessionPathForOS(p, "linux")` (requiring `osValidator` to be implemented on `*Server`!) reject `""`, `"docs/\x00bad.md"`, `"sub/../existing.md"`, `"sub/inner\\..\\existing.md"`, `"sub\\../existing.md"`, `"../escape.md"`, `"/abs/escape.md"`, `"\\abs\\escape.md"`, and (for `"windows"`) `"C:relative.md"`, `"C:/abs.md"`, and `"docs/NUL.txt"` with `errors.Is(err, corerrors.ErrValidation)` on **both** Ubuntu CI (`GOOS=linux`) and Windows (`GOOS=windows`) hosts (ensuring `validateSessionPathForOS("/abs/escape.md", "linux")` fails closed on Windows hosts and `goos == "windows"` `'/'` + `'..'` fails closed on Linux CI without `.github/workflows/ci.yml` changes); and verifies end-to-end `ServeSessionStream` invocation of `"path"`-bearing tool `backlogit_docs_lint` (`{"path": "sub/../docs/seed.md"}` against a seeded `docs/seed.md` in `ws`) returns `u3ToolResult(t, msg).IsError == true` with `"validation_failed"`.
       2. `reparse_ancestors`: Creates an external target directory `ext` and an in-workspace directory link (`cmd /c mklink /J` junction on Windows — zero elevation required, never skipped on Windows — and `os.Symlink` on POSIX) and verifies direct `s.validateSessionPath` **and** `osValidator.validateSessionPathForOS(..., "windows")` on `"junction/missing.md"`, `"junction/"`, `"nested/junction/missing.md"`, and `"nested\\junction/missing.md"` all fail closed with `errors.Is(err, corerrors.ErrValidation)` (`"MCP tool path traverses a symlink or reparse point"`), AND verifies end-to-end `ServeSessionStream` `"path"`-bearing calls (`backlogit_docs_lint` with `{"path": "junction/../docs/seed.md"}` and `backlogit_export_command_map` with `{"path": "junction/../cmdmap.md"}` and `{"path": "junction/missing.md"}`) all fail closed with `u3ToolResult(t, msg).IsError == true` and write zero files outside `ws`.
       3. `clean_relative_paths`: Verifies clean workspace-relative paths (`"."`, `".backlogit/queue/001-F.md"`, `".backlogit\\queue\\001-F.md"`, `"docs/nonexistent.md"`, `"docs//seed.md"`, and a `Server` constructed with a relative `RootPath` via `t.Chdir`) succeed (`err == nil`) on both `s.validateSessionPath` and `osValidator.validateSessionPathForOS(..., "windows")` / `"linux"`.
  * **Scoped verification command**: `go test -timeout=30m -count=1 -race -v -run '^TestU1_ValidateSessionPathMixedAndForwardSeparators$' ./internal/mcp` (must exit `0` with `1` top-level `--- PASS: TestU1_ValidateSessionPathMixedAndForwardSeparators` line and zero `[no tests to run]`, `no test files`, `--- FAIL:`, or `--- SKIP:` lines per `P-002.6` Item 5).

### Unit U2: Document optional `tools_dir`, `.bin` default, and `init`/`install --tools-dir` provisioning (`220.002-T`, Wave 1)

* **Domain**: `docs` (`P-002.1` `docs-only`; 2 files, 0 Go files)
* **Size / Complexity**: `S` / `low`
* **Execution posture**: `docs-only` (`must-fail-before-deliverable`)
* **Affected files (2)**:
  * `docs/configuration.md`
  * `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`
* **Changes needed**:
  1. In `docs/configuration.md` (preserving valid Docline v1 frontmatter `doc_type: guide`, `source: docs/configuration.md`, `chunk_strategy: h1-h2-h3`, `schema_version: "1.0"`, and quoting any YAML scalar containing `#` or `:`):
     * Under `## config.yaml`: document optional top-level `tools_dir` string scalar (workspace-relative subdirectory such as `".bin"`, `"bin"`, or `".tools/bin"`; distinguishing `config.Load` / `validateWorkspaceToolsDir` rejection of blank/whitespace-padded strings, NUL bytes, rooted/volume/UNC paths, `..` traversal, `.`, `.git`, `.backlog`, and the active storage directory basename from stricter daemon/`toolchain.yaml`/`--tools-dir` `validateDaemonToolsDirRel` & `validManifestToolsDir` rejection of all ASCII control characters and reserved `.git`/`.backlog`/`.backlogit`/`.gitignore`/`.mcp.json`/`start.ps1`/`start.sh` segments) and 5-source discovery precedence (`.mcp.json` -> `toolchain.yaml` -> `config.yaml` -> `.bin` default -> `bin` fallback when `bin/` exists and `.bin/` does not).
     * Under `## Level 1 Workspace Toolchain Configuration`:
       - Document optional top-level `tools_dir` in `.backlogit/toolchain.yaml`: when explicitly non-empty, `validBacklogitToolInManifest` requires `backlogit` `executable_path` to reside in `tools_dir`; when omitted, any valid workspace-relative tool directory passes verification and `effectiveManifestToolsDir` resolves `m.ToolsDir` -> `backlogit` `executable_path` dir -> first `workspace-local-pinned` tool dir -> `".bin"`.
       - Document `VerifyProposedLauncherScripts` alongside `WriteToolchainManifest`, `RenderPinnedMCPConfig`, and `VerifyLauncherScripts`.
       - Document `backlogit init [--tools-dir <dir>]` / `backlogit install [--tools-dir <dir>]` automated target tool directory creation, `backlogit[.exe]` installation, literal-escaped `.gitignore`/`.git/info/exclude` entries, `.mcp.json` `mcpServers.backlogit` configuration, and `toolchain.yaml`/`start.ps1`/`start.sh` reconciliation, noting explicitly that `init`/`install` (with `--jsonrpc` for non-interactive/agent callers) and `mcp prewarm`/`status --toolchain` are CLI-only bootstrap/audit surfaces while connected MCP sessions inspect workspace binding via `backlogit_get_metadata_catalog`.
       - Reconcile the 4-step `### Operator-Owned Verified Workspace Toolchain Setup, Provisioning, and Custody Alignment Procedure` so `./bin/*` and `tools/mcp/install.ps1` `(not .bin)` are clearly framed as an example of an explicit `bin` workspace layout (such as this repository's committed `.backlogit/toolchain.yaml`) alongside the `.bin` default.
  2. In `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` (preserving valid Docline v1 frontmatter `doc_type: design`, `source: docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`, `chunk_strategy: h1-h2-h3`, `schema_version: "1.0"`, and adding UTC ISO-8601 `ingested_at`):
     * Update `## Status and scope`, `## Level 1 workspace toolchain`, and `## Operator-owned verified provisioning` to incorporate `218-S` (`217-F`) and `219-S` (`218-F`): optional `tools_dir` in `schemas/workspace-toolchain.schema.json` / `toolchain.yaml` and `config.yaml`, the 5-source discovery precedence order (`.mcp.json` -> `toolchain.yaml` -> `config.yaml` -> `.bin` default -> standalone `bin` fallback when `bin/` exists and `.bin/` is absent, while noting explicit `bin` pins remain supported), `VerifyProposedLauncherScripts`, and explicit `backlogit init` and `backlogit install` `--tools-dir` CLI provisioning.
  * **Verification (`P-002.3`)**: Use safe array capture `$raw=@(& git status --porcelain -- docs/cli-reference/); if ($LASTEXITCODE -ne 0) { exit 2 }; $cliDiff=($raw -join "`n").Trim(); if ($cliDiff.Length -ne 0) { Write-Output 'cli-reference unexpectedly dirty'; exit 2 }` (never `.Trim()` on a possibly `$null` scalar under `$ErrorActionPreference='Stop'`), normalize whitespace (`$an=($a -replace '\s+',' '); $bn=($b -replace '\s+',' ')`), assert substantive claims in **both** files (`tools_dir` in `config.yaml` and `toolchain.yaml`, `validateWorkspaceToolsDir` / `validateDaemonToolsDirRel` / `validBacklogitToolInManifest` / `effectiveManifestToolsDir` in `docs/configuration.md`, 5-source discovery precedence matching `(?s)\.mcp\.json.*toolchain\.yaml.*config\.yaml.*\.bin.*(?<![.\w])bin(?![.\w])` with standalone non-dot-prefixed `bin` fallback in **both** files, `VerifyProposedLauncherScripts` in **both** files, and explicit `backlogit init` + `backlogit install` + `--tools-dir` provisioning in **both** files alongside `--jsonrpc` + `backlogit_get_metadata_catalog` in `docs/configuration.md`) AND absence of the exact backticked superseded `bin`-only prose present in `HEAD` (``to `./bin/...` executables`` and ``every local MCP server `"command"` targets `./bin/...` `` in `$an`, and ``Operator-owned local binaries (`./bin/*`)`` and ``under workspace `bin/`, not a silent`` in `$bn`), run repo-wide `go run ./cmd/backlogit docs lint --format json` **plus** scoped `go run ./cmd/backlogit docs lint --profile ingestion --path docs/configuration.md --format json` and `go run ./cmd/backlogit docs lint --profile ingestion --path docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md --format json` (`valid == true`, `violation_count == 0` on all three), and emit `EXEMPT_VERIFY_OK:220.002-T`.

## Dependency Graph

* **Shipment sequencing**: Shipment `221-S` (`220-F`) `blocks` on Shipment `220-S` (`219-F`).
* **Intra-shipment wave**: `220.001-T` (`U1`) and `220.002-T` (`U2`) touch disjoint domains (`internal/mcp/*` vs `docs/*`) and run in **Wave 1 (`ready_1`)**.

## Decisions and Rationale

* **Normalizing both `'/'` and `'\\'` to `string(filepath.Separator)`**: Eliminates the Windows separator differential between `validateSessionPath` and `os.Root` / Win32 path consumers while keeping POSIX backslash-to-slash normalization intact.
* **Parameter-key boundary in `dispatchSessionFrame`**: `dispatchSessionFrame` (`internal/mcp/session_mux.go:258-271`) gates `Arguments["path"]` (`backlogit_docs_lint`, `backlogit_docs_migrate`, `backlogit_docs_classify`, `backlogit_export_command_map`), whereas `backlogit_doctor.target` is confined downstream by `core.DoctorTarget` / `confineToStorageRoot`. Unifying non-`"path"` filesystem parameter keys (`plan_path`, `snapshot_ref`, `copilot_path`) and single-session `(*Server).executeTool` ingress is out of scope for `39F868B1` (`P-021 C1`) and recorded as an optional future hardening note.
* **Width-isolated `docs-only` task for `EF372865`**: Keeps narrative guide and design-doc updates strictly separated from `internal/mcp` Go code and leaves generated `docs/cli-reference/*.md` untouched.

## Risks and Caveats

* **Windows junction cleanup and `MAX_PATH` in tests**: Allocate short `ws` and `ext` paths inside the parent `TestU1`'s `t.TempDir()` so `cmd /c mklink /J` stays well below Win32 `MAX_PATH` under workspace-contained `TEMP` directories and cleans up automatically.
* **Governed transitional formatting gate (`079-DL` / `105-DL` / `106-DL` across `220-S`, `221-S`, and `222-S` Waves 1–4)**: Deliberations `079-DL`, `104-DL`, `105-DL`, and `106-DL` explicitly extend `079-DL`'s governed scoped repository Go source-tree formatting gate (`gofmt -l cmd internal tests scripts`, covering `gofmt -l cmd internal tests` plus `scripts` while excluding gitignored/local `.copilot/`, `.bin/`, `bin/`, `.engram/`, `.kbdb/`, and `logs/` caches, **forbidding unscoped `gofmt -l .` and unscoped `gofmt -w .`**, and paired with branch-diff `.go` `gofmt` + LF-only `0 \r` + `git diff --check` verification) across **`220-S`, `221-S`, and `222-S` Waves 1–4** until `222-S` Wave 4 (`221.003-T`..`221.008-T` + `221.010-T`) updates the repository's canonical format-gate text on `main`.

## Constitution Check

* **I. Safety-First Go**: `pass` — Standard-library `strings.NewReplacer` in Go 1.24, wrapped sentinel `corerrors.ErrValidation`, zero `unsafe`.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — `U1` lands a compiling, failing unit test (`TestU1_ValidateSessionPathMixedAndForwardSeparators`) before modifying `session_mux.go`; `U2` carries a complete fail-before-deliverable `P-002.1`/`P-002.3` `docs-only` contract.
* **III. Workspace Isolation and Security Boundaries**: `pass` — Closes a Windows forward-slash traversal and junction-ancestor inspection bypass at the MCP session ingress boundary.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — All test fixtures use `t.TempDir()` and zero operations escape the workspace.
* **V. Structured Observability**: `pass` — Clear wrapped validation error messages and deterministic test/exempt markers.
* **VI. Single Responsibility**: `pass` — Zero new dependencies; 2 files per unit in isolated `code` and `docs` domains.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — Read-only `exempt_verification_command` on `U2`.
* **VIII. Explicit Safety Modes**: `pass` — Security-boundary path validation hardened with real Windows junction fixtures.
* **IX. Git-Friendly Persistence**: `pass` — Clean LF Go and Docline-compliant Markdown files.
* **X. Agent Context Efficiency**: `pass` — Minimal, targeted changes.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — Merges via `--merge` commit (`P-009`).

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `absent` — Tightens existing MCP session path containment to match its documented contract and updates operator documentation.
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Modifies `(*Server).validateSessionPath` (`internal/mcp/session_mux.go`), the security trust-boundary validator for incoming MCP session tool path arguments.
* **migration, backfill, destructive data/config action, or irreversible step**: `absent` — Additive normalization and documentation only.
* **external integration, operator checkpoint, or external dependency**: `absent` — None.
* **high runtime, rollout, or rollback risk**: `absent` — Low blast radius; clean workspace-relative `/` and `\` paths continue to pass.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surface**: Multi-session MCP server ingress path validator (`(*Server).validateSessionPath` in `internal/mcp/session_mux.go`).
* **Required runtime verification**:
  1. `go test -timeout=30m -count=1 -race -v -run '^TestU1_ValidateSessionPathMixedAndForwardSeparators$' ./internal/mcp` (including end-to-end `ServeSessionStream` JSON-RPC tool invocation of `backlogit_docs_lint` and `backlogit_export_command_map` with `"junction/../docs/seed.md"`, `"sub/../docs/seed.md"`, and `"junction/missing.md"`).
  2. Full `go test -timeout=30m ./...`, `golangci-lint run`, `go vet ./...`, `scripts/md-lint.ps1`, and `go run ./cmd/backlogit docs lint`.
* **Operational closure**: `docs/closure/221-S-220-F-post-merge-closure.md` recording MCP session containment verification, `_Ship` owner, 7-day validation window, and standard merge-commit revert path.

## Plan Hardening

* **Hardening required**: Yes — P-006 triggered by security trust-boundary path validation in `internal/mcp/session_mux.go`.
* **Protected invariants**:
  * **I1 (Zero separator differential between `validateSessionPath` and `os.Root` / Win32)**: Both `'/'` and `'\\'` MUST be normalized to `string(filepath.Separator)` before `filepath.IsLocal`, `strings.Split`, `component == ".."`, and `root.Lstat` so no mixed or forward-slash input can hide a `'..'` segment or skip `root.Lstat` / `core.IsSymlinkOrReparsePoint` on an ancestor junction/symlink.
  * **I2 (Non-skippable Windows junction test coverage)**: On `runtime.GOOS == "windows"`, the test MUST create a real NTFS directory junction (`cmd /c mklink /J`) rather than skipping on unprivileged `os.Symlink`, proving real Win32 reparse-point rejection when the leaf under the junction does not exist (`"junction/missing.md"`).
  * **I3 (Generated CLI reference preservation)**: Unit `U2` MUST NOT hand-edit `docs/cli-reference/*.md` and MUST satisfy `TestDoclineSoftKeys_LiveTrackedCorpus` (`chunk_strategy: h1-h2-h3`, `schema_version: "1.0"`) and `go run ./cmd/backlogit docs lint`.
* **Reinforced instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`, `.github/instructions/go-mcp-server.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/markdown.instructions.md`
  * `docs/compound/security-issues/mcp-validator-sdk-parser-differentials-2026-11-08.md`
  * `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`
  * `docs/compound/best-practices/red-snapshot-observation-scoped-metadata-memo-2026-11-08.md`
  * `docs/compound/2026-06-26-docline-frontmatter-contract.md`
  * `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`
* **Risky actions classification**:

| ProposedAction | Target Surface | ActionRisk | Approval / Guardrail Path | Expected ActionResult |
| --- | --- | --- | --- | --- |
| Normalize `'/'` and `'\\'` in `(*Server).validateSessionPath` (`U1`) | `internal/mcp/session_mux.go`, `internal/mcp/session_mux_test.go` | `moderate` (MCP security boundary path validator) | Test forward, back, mixed, trailing separators + real Windows junctions + clean relative paths | Traversal and junction-ancestor paths fail closed with `ErrValidation`; clean relative `/` and `'\\'` paths succeed |
| Update `docs/configuration.md` and toolchain design doc (`U2`) | `docs/configuration.md`, `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` | `low` (narrative documentation) | Keep `docs/cli-reference/` untouched; verify Docline frontmatter via `go run ./cmd/backlogit docs lint` and `md-lint` | Accurate `tools_dir` / `.bin` / `--tools-dir` docs with `0` lint violations |

* **Review-gate capability and marker requirements**:
  * `plan-review` MUST probe subagent dispatch capability, record literal `dispatch_mode:` and `decision:` markers, cover all 7 required personas (including `Agent-Native Parity Reviewer` and `Security Lens Reviewer`) plus `rubber-duck` critique, and achieve `decision: PASS` before harvest.
* **Unresolved operator decisions**: None.

## Plan Review

* `dispatch_mode: multi-agent-dispatch`
* `TOOL_OK: reviewer-subagent-dispatch`
* `decision: FAIL`
* **Gate rationale (Attempt 1 — superseded by Attempt 2)**: Multi-persona + `rubber-duck` review (`Go Reviewer`, `Agent-Native Parity Reviewer`, `Security Lens Reviewer`, `Learnings Researcher`, and `rubber-duck`) identified a `P1` blocker in Unit `U1`'s end-to-end `ServeSessionStream` scenario (`backlogit_doctor` uses `Arguments["target"]` and bypasses `dispatchSessionFrame`'s `Arguments["path"]` `validateSessionPath` gate) plus `P2` improvements (keep `"."` inspected by skipping only `component == ""`, short `t.TempDir()` paths for Windows `mklink /J`, exact `validBacklogitToolInManifest` / `effectiveManifestToolsDir` / `validateWorkspaceToolsDir` semantics in `U2`, and Docline `structural_table_shape` escaping on `'\\'`), all remediated in Attempt 2 above.

<!-- plan-review-attempt: 2 -->

## Plan Review

* `dispatch_mode: single-agent-declared-degradation`
* `TOOL_DEGRADED: reviewer-subagent-dispatch`
* `decision: PASS`
* **Gate rationale (Attempt 2)**: Dispatched all 7 required `plan-review` personas plus `rubber-duck` (`/gsd-dark-mode` Step 2.1); 7 subagents returned structured `PASS` verdicts (`0 P0, 0 P1, 0 P2`), and following an empty stream return from `Agent-Native Parity Reviewer` a complete sequential rubric pass was executed across all 7 personas + `rubber-duck` per `plan-review/SKILL.md` Terminal States. Plan hardening was required (`Requires plan hardening: yes`) and is satisfied by `## Plan Hardening` (`I1`–`I3` and `ProposedAction` / `ActionRisk` table). `## Constitution Check` concludes with literal `Constitution Check: pass`. Merged findings: `0 P0, 0 P1, 0 P2` (`3 P3` implementation advisories noted below for `build-feature` awareness: package-level `strings.NewReplacer`, explicit `CloseAllSessions`/`coordinator.Close`/`ws.Close` `t.Cleanup` on short `ws` roots, and `t.Chdir` relative-`RootPath` check).
* **Per-persona findings summary**:
  * **Constitution Reviewer**: `PASS` (`0` findings).
  * **Go Reviewer**: `PASS` (`0 P0, 0 P1, 0 P2, 2 P3` advisories on `strings.NewReplacer` hoisting and `t.Cleanup` server/workspace teardown).
  * **Scope Boundary Auditor**: `PASS` (`0` findings).
  * **Learnings Researcher**: `PASS` (`0 P0, 0 P1, 0 P2, 1 P3` advisory on `t.Chdir` relative-`RootPath` check, `confidence: high`).
  * **Architecture Strategist**: `PASS` (`0` findings).
  * **Agent-Native Parity Reviewer** (complete rubric pass): `PASS` (`0` findings). Verified `ServeSessionStream` `"path"`-bearing `backlogit_docs_lint` & `backlogit_export_command_map` coverage, CLI-vs-MCP bootstrap documentation boundary, and 4-step provisioning alignment in `docs/configuration.md`.
  * **Security Lens Reviewer**: `PASS` (`0` findings). Non-skippable Windows `mklink /J` junction containment and `P-002.5` read-only `U2` gate verified.
  * **`rubber-duck`**: `PASS` (`0` findings).
