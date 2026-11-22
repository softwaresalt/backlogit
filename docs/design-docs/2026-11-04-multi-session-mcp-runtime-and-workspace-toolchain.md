---
chunk_strategy: h1-h2-h3
description: Multi-session MCP runtime and Level 1 workspace toolchain design and provisioning boundaries
doc_type: design
ingested_at: "2026-11-21T22:20:00Z"
schema_version: "1.0"
source: docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md
title: Multi-Session MCP Runtime and Workspace Toolchain
---

## Status and scope

The Option A multi-session runtime (`212-S`), the Level 1 schema and read-only
inventory verifier (`213-S`), the Track C launcher/companion MCP auditor,
manifest writer, configuration renderer, strict status integration
(`backlogit mcp status --toolchain [--strict]`), and repository manifest
(`.backlogit/toolchain.yaml`) (`214-S`), plus configurable `tools_dir`, the
`.bin` default with `bin` fallback, `VerifyProposedLauncherScripts`, and
automated `backlogit init` / `backlogit install --tools-dir` provisioning
(`218-S` / `219-S`) are implemented. Operator-owned local binaries (in `.bin/`,
`bin/`, or a declared `tools_dir`) and gitignored `.mcp.json` remain under
operator custody; the operator procedure below describes how operators
provision and verify local runtime files against the shipped contract.

See the [requirements](../product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md)
and [decision](../decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md)
for the approved design and the [implementation plan](../plans/2026-11-04-multi-session-mcp-runtime-plan.md)
for sequencing.

## Multi-session runtime

Option A runs one independent daemon per workspace. `backlogit mcp` is a
stdio session shim connecting an agent to that daemon; `backlogit mcp prewarm`
starts or attaches to it. One shared MCP server maintains distinct sessions,
request identities, notifications, and cancellation contexts. Disconnecting one
client does not terminate the other sessions. Daemon status uses sidecar,
process identity, and IPC readiness rather than taking the daemon's exclusive
lock as a liveness probe.

### IPC boundary

On POSIX, the daemon listens on a workspace storage-root runtime Unix-domain
socket, normally `runtime/mcp.sock`, with a short `runtime/s` fallback and
`0600` permissions. An endpoint exceeding the supported length is rejected.
On Windows, the endpoint is a named pipe derived from the workspace identity,
`\\.\pipe\backlogit-mcp-<hexHash16>`, with a current-user SID DACL. A named
pipe is an OS namespace object, not a file inside the workspace; its identity is
bound to the workspace rather than claiming filesystem containment.

### Operation coordination

`OperationCoordinator` gates runtime-managed MCP operations. `OpMutating`
operations enter a bounded FIFO mutation queue, while `OpReadOnly` operations
use bounded read admission. `OpExclusiveBarrier` drains readers as well as
serializing mutations for index synchronization. Turns remain held through the
synchronous handler frame, including cancellation cleanup; queue saturation
and admission timeout return explicit busy results. Progress notifications are
sent only when the client supports them. The coordinator does not serialize
external CLI processes or arbitrary filesystem writers.

## Level 1 workspace toolchain

The `schemas/workspace-toolchain.schema.json` contract declares
`schema_version: "1.0"`, `containment_level` set to
`level-1-workspace-local-pinned`, `os_container_sandbox_deferred: true`,
`workspace_root`, optional `tools_dir`, `tools`, and `host_prerequisites`.
`config.yaml` and `toolchain.yaml` both support optional `tools_dir` as a
validated workspace-relative subdirectory (such as `.bin`, `bin`, or
`.tools/bin`). Tool entries identify their class, executable and entrypoint
paths, version, SHA-256 or lockfile provenance, workspace binding arguments,
remote policy, and reason where applicable. Classes distinguish
`workspace-local-pinned`, `host-prerequisite`, `remote-declared`,
`built-in-host`, `unsupported`, and `non-conformant`. These declarations must
not disguise unsupported or unverified tools as pinned.

When resolving the workspace-pinned `backlogit` executable or default
`--tools-dir` target, backlogit evaluates five sources in priority order without
short-circuiting invalid entries:

1. `.mcp.json` (`backlogit` stdio server `command`)
2. `<storage>/toolchain.yaml` (`backlogit` `executable_path` and/or `tools_dir`)
3. `<storage>/config.yaml` (`tools_dir`)
4. `.bin` default (`.bin/backlogit[.exe]`)
5. `bin` fallback:
   * `--tools-dir` default target selection (`defaultTargetToolsDir` in
     `internal/cli/mcp_daemon.go`): selects `bin` when `bin/` exists and
     `.bin/` is absent
   * MCP daemon executable attachment (`pinnedMCPDaemonBinary` in
     `internal/cli/mcp_daemon.go`): probes `.bin/backlogit[.exe]` and falls
     back to `bin/backlogit[.exe]` on `os.ErrNotExist` (even if an empty `.bin/`
     directory exists)

`internal/toolchain` provides `LoadToolchainManifest`,
`DefaultWorkspaceToolchainManifest`, `VerifyWorkspaceToolchain`, and
`ValidateMCPConfigPinning` (`213-S`), plus `VerifyLauncherScripts`,
`VerifyProposedLauncherScripts`, `AuditRepositoryToolchain`,
`WriteToolchainManifest`, and `RenderPinnedMCPConfig` (`214-S`, `218-S`, `219-S`).
Loading checks the selected `.backlog` or legacy `.backlogit` storage root; a
missing manifest yields an unverified default inventory, not a successful
pinning claim. Verification is read-only and reports findings and violations. It
uses workspace-rooted file access and checks paths, identity, declared pins,
launcher scripts (`start.ps1`, `start.sh`), and MCP configuration; it does not
overwrite operator-owned tools.

`WriteToolchainManifest` writes the selected workspace manifest (such as the
committed `.backlogit/toolchain.yaml`), `RenderPinnedMCPConfig` renders
workspace-pinned MCP configuration, `VerifyProposedLauncherScripts` validates
in-memory launcher script changes before disk replacement, `backlogit init
[--tools-dir <dir>]` and `backlogit install [--tools-dir <dir>]` automate
`backlogit` binary installation, `.gitignore`/`.git/info/exclude` wiring,
`.mcp.json` registration, and `toolchain.yaml`/launcher reconciliation, and
`backlogit mcp status --toolchain [--strict]` surfaces live daemon status
alongside the Level 1 repository toolchain audit.

## Operator-owned verified provisioning

The D9 procedure governs operator-owned local runtime provisioning against the
shipped Level 1 contract. Operator-owned local binaries, ignored configuration,
and pre-existing worktree changes remain under operator custody; do not
overwrite them as a side effect of verification.

1. Establish trusted sources, versions and SHA-256 digests for workspace-local
   `backlogit`, `engram`, `bun`, `bunx`, GitHub MCP, and `kbdb`. Build
   `backlogit` from repository source (or provision it with `backlogit init` /
   `backlogit install --tools-dir <dir>`) and match its version to
   `version.Resolve()`. Put platform-appropriate binaries under the workspace
   tools directory (`.bin/` by default, `bin/` fallback when `bin/` exists and
   `.bin/` is absent for `--tools-dir` or on `os.ErrNotExist` for MCP daemon
   attachment, or an explicit `tools_dir` in `config.yaml` /
   `toolchain.yaml` such as committed `bin/` pins), not a silent global `PATH`
   fallback.
2. Pin Context7 and Tavily package versions and lockfile provenance in
   `tools/mcp/package.json` and `tools/mcp/bun.lock`. Provision via the
   operator-owned installer using the local Bun binary, a frozen lockfile and
   disabled install scripts. Invoke local package entrypoints directly.
3. Bind Engram data to the workspace `.engram` directory. Bind Bun installation
   and cache paths to the workspace. Configure the Graphtor shim and local
   `kbdb` with `serve --read-only`; do not index this workspace.
4. Declare host-provided `pwsh`, `bash`, `git`, `gh`, `go`, and `copilot`
   as host prerequisites, and host-native tools as `built-in-host`. Declare
   remote services and their policies rather than representing them as local
   isolation.
5. Use `backlogit init --tools-dir <dir>` or `backlogit install --tools-dir <dir>`
   to provision `backlogit`, `.mcp.json`, and ignore rules (reconciling
   `toolchain.yaml` and verifying proposed launcher updates with
   `VerifyProposedLauncherScripts` when `toolchain.yaml` exists), and use
   `WriteToolchainManifest` and `RenderPinnedMCPConfig` to align full
   multi-tool manifests and `.mcp.json`, then align `start.ps1` and `start.sh`
   with those same local paths. Review custody and local changes before applying
   any generated configuration.
6. Run `backlogit mcp status --toolchain --strict --format json` for the
   workspace. Acceptance requires exit zero, `valid: true`,
   `violations: 0`, `containment_level: level-1-workspace-local-pinned`, and
   `os_container_sandbox_deferred: true`. Investigate each finding instead of
   treating a default inventory or an error exit as verification.

## Threat-model boundary

Level 1 establishes workspace-local installation, configuration and pinning
for declared tools; it is not an OS or container sandbox. Rooted file checks
and IPC permissions protect the documented backlogit path and attachment
boundaries, but do not constrain arbitrary external processes, hostile actors
with equivalent user privileges, undeclared writers, host tools, or remote
service behavior. Remote policies and host prerequisites are explicit
exceptions, not proof of confinement. Level 2 sandboxing and external policy
enforcement are deferred.
