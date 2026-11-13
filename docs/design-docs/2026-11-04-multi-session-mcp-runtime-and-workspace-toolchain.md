---
chunk_strategy: h1-h2-h3
description: Multi-session MCP runtime and Level 1 workspace toolchain design and provisioning boundaries
doc_type: design
schema_version: "1.0"
source: docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md
title: Multi-Session MCP Runtime and Workspace Toolchain
---

## Status and scope

The Option A multi-session runtime is implemented. The Level 1 schema and
read-only inventory verifier are present; launcher auditing, manifest writing,
configuration rendering, strict status integration, and the repository's pinned
manifest are planned for the subsequent integration shipment. The operator
procedure below is a target contract, not evidence that this workspace is already
provisioned or verified.

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
`workspace_root`, `tools`, and `host_prerequisites`. Tool entries identify their
class, executable and entrypoint paths, version, SHA-256 or lockfile provenance,
workspace binding arguments, remote policy, and reason where applicable.
Classes distinguish `workspace-local-pinned`, `host-prerequisite`,
`remote-declared`, `built-in-host`, `unsupported`, and `non-conformant`.
These declarations must not disguise unsupported or unverified tools as pinned.

`internal/toolchain` currently provides `LoadToolchainManifest`,
`DefaultWorkspaceToolchainManifest`, `VerifyWorkspaceToolchain`, and
`ValidateMCPConfigPinning`. Loading checks the selected `.backlog` or legacy
`.backlogit` storage root; a missing manifest yields an unverified default
inventory, not a successful pinning claim. Verification is read-only and
reports findings and violations. It uses workspace-rooted file access and
checks paths, identity, declared pins and MCP configuration; it does not
provision tools.

The planned `WriteToolchainManifest` and `RenderPinnedMCPConfig` APIs belong to
the later integration work. They are intended to write the selected workspace
manifest and render workspace-pinned MCP configuration respectively. The
planned repository manifest is `.backlogit/toolchain.yaml`; the intended
`mcp status --toolchain --strict` integration is not yet a current verification
surface.

## Operator-owned verified provisioning

The D9 procedure is the target state after the integration surfaces exist.
Operator-owned local binaries, ignored configuration, and pre-existing
worktree changes remain under operator custody; do not overwrite them as a
side effect of verification.

1. Establish trusted sources, versions and SHA-256 digests for workspace-local
   `backlogit`, `engram`, `bun`, `bunx`, GitHub MCP, and `kbdb`. Build
   `backlogit` from repository source and match its version to
   `version.Resolve()`. Put platform-appropriate binaries under workspace
   `bin/`, not a silent global `PATH` fallback.
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
5. Once implemented, use `WriteToolchainManifest` and
   `RenderPinnedMCPConfig` to align the workspace manifest and `.mcp.json`,
   then align `start.ps1` and `start.sh` with those same local paths. Review
   custody and local changes before applying any generated configuration.
6. Once implemented, run
   `backlogit mcp status --toolchain --strict --format json` for the workspace.
   Acceptance requires exit zero, `valid: true`,
   `violations: 0`, `containment_level: level-1-workspace-local-pinned`, and
   `os_container_sandbox_deferred: true`. Investigate each finding instead of
   treating a default inventory or absent command as verification.

## Threat-model boundary

Level 1 establishes workspace-local installation, configuration and pinning
for declared tools; it is not an OS or container sandbox. Rooted file checks
and IPC permissions protect the documented backlogit path and attachment
boundaries, but do not constrain arbitrary external processes, hostile actors
with equivalent user privileges, undeclared writers, host tools, or remote
service behavior. Remote policies and host prerequisites are explicit
exceptions, not proof of confinement. Level 2 sandboxing and external policy
enforcement are deferred.
