---
chunk_strategy: h1-h2-h3
description: 'Product requirements specification for supporting multiple concurrent agent MCP sessions through one workspace-owned backlogit singleton runtime and defining the workspace-owned agent/MCP tool environment with workspace-local installation, configuration, and version pinning across all workspace tools (stash ECF33C79, deliberation 082-DL).'
doc_type: spec
docline:
  author: Stage
  dark_factory_ready: false
  date: '2026-11-04'
  deliberation_id: '082-DL'
  handoff_status: ready_for_plan
  requirement_ids:
    - R1
    - R2
    - R3
    - R4
    - R5
    - R6
    - R7
    - R8
    - R9
    - R10
    - R11
    - R12
    - R13
    - R14
    - R15
    - R16
    - R17
    - R18
    - R19
    - R20
    - R21
    - R22
    - R23
  scope: deep
  source_research:
    - docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md
    - docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md
    - docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md
    - docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md
    - docs/compound/2026-10-27-pin-the-handle-not-the-path.md
    - docs/compound/best-practices/timeout-goroutine-lock-ownership-go-2026-07-01.md
    - docs/compound/build-errors/match-ci-go-and-linter-before-source-repair-2026-11-06.md
    - docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md
    - docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md
    - docs/compound/best-practices/git-aware-backlog-artifact-archival-preserves-follow-history-2026-07-10.md
    - docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md
    - docs/compound/2026-10-19-lock-held-side-effect-ordering.md
    - docs/compound/2026-08-29-windows-preremove-rename-pattern.md
    - docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md
  source_stash_ids:
    - ECF33C79
    - C2CFE06E
    - 60D799D9
    - 19D55778
    - 96F1DFFD
  status: approved
schema_version: "1.0"
source: docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md
title: 'Product Spec — Multi-Session Singleton MCP Runtime and Workspace-Pinned All-Tools Environment (ECF33C79)'
---

## Problem Frame

### Current State and Precedent

Feature `204-F` (shipped in `205-S`, source decision `077-DL` at `docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md`) introduced a CLI-only OS-handle advisory lock (`<storage_root>/runtime/mcp-daemon.lock`) and metadata sidecar (`<storage_root>/runtime/mcp-daemon.json`) around `backlogit mcp`. That delivery guarantees that at most **one** `backlogit mcp` process runs per workspace at any time: a second `backlogit mcp` invocation without `--replace` fails immediately with `corerrors.ErrDaemonAlreadyRunning`, while `--replace` terminates the verified existing holder before acquiring the lock.

Preventing duplicate servers solved split-brain SQLite contention and stale in-memory `s.manifest` diffs (`backlogit_merge_sync`), but it **does not allow multiple concurrent agents to share one workspace runtime**:

* In multi-agent workflows (`_orchestrator`, `_stage`, `_ship`, background review/research agents, or multiple IDE windows), each agent host spawns its own MCP client connection configured via `.mcp.json` (`"type": "stdio", "command": "./bin/backlogit.exe", "args": ["mcp"]`).
* Under `204-F`, the second agent either fails at startup with `ErrDaemonAlreadyRunning` or passes `--replace` and kills the first agent's active MCP session.
* Inside `internal/mcp/server.go`, `RunStdio` calls `mcpserver.ServeStdio(s.mcp)` in `github.com/mark3labs/mcp-go v0.27.0`. Inspection of `mcp-go v0.27.0` (`StdioServer.Listen`) confirms that `StdioServer` registers a package-global static session (`&stdioSessionInstance`) via `s.server.RegisterSession(ctx, &stdioSessionInstance)`. Calling `StdioServer.Listen` more than once on the same `*mcpserver.MCPServer` returns `ErrSessionExists` and shares a single notification channel (`stdioSessionInstance.notifications`).
* Conversely, `mcpserver.MCPServer` itself (`RegisterSession`, `UnregisterSession`, `WithContext`, `HandleMessage`, `SendNotificationToClient`, `SendNotificationToSpecificClient`) natively supports multiple concurrent `ClientSession` instances when each client connection is assigned its own `ClientSession` implementation and stream dispatcher.

### Pressure-Testing the Framing

Anchor stash `ECF33C79` (`priority: high`, `kind: feature`) states:

> As part of singleton design, we should leverage the Golang channel capability to implement a singleton runtime with the ability to host multiple MCP sessions with internal operation gating and async callbacks if possible to enable sequential processing internally while managing multiple concurrent sessions. Also ensure that both .mcp.json and start.ps1 reference bin/backlogit.exe as the workspace-local tool executable.

Pressure-testing this request against the codebase and MCP protocol surfaces three essential architectural distinctions:

1. **Go channels are an internal coordination mechanism, not a substitute for concurrency design**: Routing every MCP call—including read-only queries (`backlogit_get_item`, `backlogit_list_items`, `backlogit_query_sql`, `backlogit_get_queue`, `backlogit_get_shipment`)—through a single unbuffered sequential channel would stall all reading agents behind any long-running mutation (such as `backlogit_ship_shipment` or `backlogit_sync_index`, which take tens of seconds and previously triggered `McpError -32001: Request timed out` in `195-S`, `196-S`, and `211-S`). Channel-based operation gating must serialize **workspace mutations** in deterministic FIFO order while permitting bounded concurrent **read-only** operations.
2. **Stdio client compatibility cannot be dropped for a network-only transport**: Standard MCP hosts launch `backlogit mcp` as a child process speaking newline-delimited JSON-RPC 2.0 over `stdin`/`stdout`. Furthermore, `mcpserver.StreamableHTTPServer` in `mcp-go v0.27.0` is an unimplemented stub (`type StreamableHTTPServer struct{}`). To let multiple stdio-spawned `backlogit mcp` processes share one singleton runtime without changing `.mcp.json` away from `stdio`, `backlogit mcp` needs a session adapter (stdio-to-local-IPC shim over a Windows Named Pipe or Unix Domain Socket, analogous to `bin/engram.exe shim` + `engram.exe daemon`).
3. **Async callbacks are conditional on MCP protocol and client support**: Standard MCP tool callers (`tools/call`) block waiting for a single JSON-RPC response matching the request `id`. While `mcp-go v0.27.0` supports asynchronous server-to-client notifications (`notifications/progress`, `notifications/message`, `notifications/resources/updated`) per `ClientSession.NotificationChannel()`, replacing synchronous `tools/call` responses with detached callback tickets would break existing agent tool callers unless introduced as an explicit opt-in capability or progress-notification stream.

### Operator Clarification and Provenance (2026-11-04T20:15:01Z)

During initial intake (`2026-11-04T03:21:23Z`), the Orchestrator asked the operator a single focused question:

> Should that independent background daemon be the lifecycle model?

The Orchestrator noted that the recommended daemon survives individual clients disconnecting. At `2026-11-04T20:15:01Z`, the operator responded verbatim:

> Yes, I think makes sense, but it also means that it either needs an initial startup via the start.ps1 or that it initializes with the first instantiation for the workspace to which this local binary is pinned. It also suggests, perhaps, that available tools should be contained within the so-called sandbox of the workspace itself.

This clarification establishes three distinct provenance boundaries:

1. **Confirmed Lifecycle Model (`R13`, `A1`)**: The independent per-workspace background daemon lifecycle (`Option A`) is confirmed by the operator. Multiple thin `stdio` MCP client sessions connect to the workspace daemon, and one client's disconnect never terminates peer sessions.
2. **Confirmed Dual Startup Entry Routes and Workspace-Pinned Binary Identity (`R14`–`R16`)**: The runtime must support both optional/idempotent prewarm via `start.ps1` and lazy initialization on the first `backlogit mcp` instantiation for the workspace to which the local `bin/backlogit.exe` binary is pinned, using identical singleton admission and workspace/version verification.
3. **Initial Tentative Workspace Tool Containment (`R17`–`R18`, Historical `A6`, `A7`) vs Unbundled Async Scope (`R10`, `A3`)**: Because the operator initially qualified tool containment with `"perhaps"` at `2026-11-04T20:15:01Z` and the Orchestrator's first question did not ask about async job-ticket APIs or internal Go struct layouts, the initial continuation recorded application-level workspace containment of `backlogit`-managed tools as a narrow default assumption (`A6`, now superseded at `2026-11-04T21:41:00Z` below) while keeping conservative synchronous `tools/call` + progress notifications (`A3`) separate from the confirmed lifecycle decision.

### Second Operator Clarification and Provenance — All-Tools Workspace Breadth (2026-11-04T21:41:00Z)

Following the `2026-11-04T20:15:01Z` continuation, the operator was asked to clarify tool-containment breadth:

> Should available tools mean backlogit-managed tooling only, or all agent/MCP tools used in the workspace?

At `2026-11-04T21:41:00Z`, the operator answered verbatim:

> I mean ALL agent/MCP tools used in a workspace.

This second clarification establishes clear provenance boundaries for what is now resolved versus what remains open:

1. **Breadth Question (`Q2`) Resolved as `ALL` — Earlier Narrow Assumption `A6` Superseded**: Available tools in scope of the workspace containment outcome mean **ALL agent/MCP tools used in a workspace**—encompassing `backlogit`, other workspace MCP servers (such as `engram`, `graphtor-docs`, and any additional configured MCP servers), agent-invoked CLI/helper executables, and the workspace launcher/adapter configurations (`.mcp.json`, `start.ps1`, `start.sh`, and host tool definitions) that expose and run them (`R19`). The earlier narrower assumption `A6` (`backlogit`-managed tooling only) is explicitly **superseded** while its history is preserved for auditability, and the breadth question (`Q2`) MUST NOT be asked again. Existing `R17` remains a required component for `backlogit` itself, not the entire outcome.
2. **What This Breadth Answer Established vs What Required `Q3` Resolution**:
   * **Breadth vs containment strength (`Q3`, subsequently resolved at `2026-11-04T22:02:03Z` below)**: Answering `"ALL agent/MCP tools used in a workspace"` defined **breadth** across tools, whereas **containment strength**—workspace-local tool installation, configuration, and version pinning vs OS/container-enforced filesystem, process, and network isolation—was isolated as `Q3` and resolved at `2026-11-04T22:02:03Z` (`R18`, `R20`, `R23`).
   * **Remote API policy is separate (`R21`)**: The breadth answer does **not** automatically forbid every remote API or decide external network/authentication policy (such as Azure DevOps synchronization in stash `D895887C` or approved remote endpoints).
   * **No implied package-manager or blanket tool approval**: The breadth answer does **not** select a specific package manager, manifest/lockfile schema, or installer directory (`D9`, keeping stash `17214531` separate), nor does it automatically approve every tool currently present on the host without inventory and conformance checks (`R19`, `R20`).

### Third Operator Clarification and Provenance — Containment Strength Resolved for Initial Release (2026-11-04T22:02:03Z)

Following the `2026-11-04T21:41:00Z` breadth confirmation (`Q2 = ALL`), the operator was asked the final product-scope question (`Q3`) on containment strength across all workspace agent/MCP tools (workspace-local installation, configuration, and version pinning vs OS/container-enforced process, filesystem, and network sandboxing).

At `2026-11-04T22:02:03Z`, the operator answered verbatim:

> For now, workspace-local installation, configuration, and version pinning is sufficient.

This third clarification closes all blocking product-scope intake questions and establishes four explicit requirements boundaries:

1. **Initial-Release Containment Strength Resolved as Level 1 (`Q3` Resolved, `R23`, `A8`)**: All workspace agent/MCP tools and helpers in scope (`Q2 = ALL`) use **workspace-local installation, configuration, and version pinning** with explicit tool inventory, startup/provenance/compatibility contracts, and zero silent global `PATH` or cross-workspace executable fallback (`R19`, `R23`, `SC10`–`SC12`). Any unavoidable host bootstrap prerequisites or exceptions (such as the OS shell `pwsh`/`powershell`/`sh` required to invoke `start.ps1`/`start.sh`) must be declared explicitly rather than disguised as fully local tool compliance (`R19`, `R20`, `SC12`, `SC13`).
2. **OS/Container Sandboxing and Kernel-Level Escape Validation Deferred Out of Initial Release (`R18`, `R20`, `R23`, `SC14`)**: OS/container-enforced process, filesystem, and network sandboxing (`Level 2`) and hard host-isolation escape validation are **deferred / out of scope** for this initial release. Workspace-local binary placement, configuration, and version pinning MUST NOT be claimed to provide OS, container, process, network, or host-filesystem isolation (`R18`, `R20`, `R23`).
3. **`backlogit` Path Containment Retained Without Overclaiming External Tool Confinement (`R17`, `R20`, `R23`)**: Existing `backlogit` canonical workspace path containment checks (`*os.Root` and traversal/symlink/reparse refusal in `R17`, `SC9`) remain required product behavior for `backlogit` itself, but this specification does **not** claim that every external agent/MCP tool obeys `backlogit`'s path checks or that `backlogit` can confine unrelated host tools. Unsupported, built-in host, or remote tooling limitations must be reported honestly rather than silently counted as compliant (`R20`, `SC13`).
4. **Remote API Policy and Conservative Synchronous + Progress-Notification Scope Unbundled (`R10`, `R21`, `A3`, `D8`)**: Remote API authorization policy (`R21`) is neither banned nor settled by `Q3` (and external integration construction remains out of scope). Synchronous `tools/call` responses paired with supported MCP progress/log notifications (`R10`, `A3`) remain a separate conservative implementation proposal/assumption whose exact SDK/client details are deferred to planning (`D8`); no persistent async job-ticket API or arbitrary reverse RPC is authorized.

## Requirements

### Multi-Session Singleton Runtime and Transport Adaptation

* **R1 (Concurrent Multi-Agent Session Hosting)**: At least two independent agents (such as Orchestrator, Stage, Ship, or concurrent IDE sessions) MUST be able to establish and maintain simultaneous, active MCP sessions against a single workspace-owned `backlogit` runtime instance without receiving `corerrors.ErrDaemonAlreadyRunning` or terminating peer sessions.
* **R2 (Drop-In Stdio Client Compatibility via Session Adapter)**: Because standard MCP hosts spawn per-session child processes configured with `"type": "stdio"`, `backlogit mcp` MUST support concurrent stdio callers by bridging each caller's `stdin`/`stdout` stream as an isolated session over a workspace-local IPC transport (Windows Named Pipe / Unix Domain Socket) to the singleton workspace runtime, rather than assuming all clients can configure an HTTP/SSE network transport.
* **R3 (Handle-Pinned Runtime Lock, Verified Absolute Leaf Reparse Validation, Sidecar Schema Evolution, and Non-Disruptive Probing)**: The singleton runtime MUST retain the `<storage_root>/runtime` `*os.Root` handle-pinned lock (`mcp-daemon.lock`) and sidecar (`mcp-daemon.json`) security invariants from `204-F` (`docs/compound/2026-10-27-pin-the-handle-not-the-path.md`). Per `085-DL` (`C2CFE06E`, `docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`), while all file opens, reads, writes, and removals execute relative to the pinned `runtimeRoot` (`*os.Root`) and `os.SameFile(opened, linked)` verifies opened-handle to handle-relative-`Lstat` object identity, path-based symlink/reparse validation (`rejectLink` -> `core.IsSymlinkOrReparsePoint`) across `AcquireMCPDaemonLock`, `InspectMCPDaemonStatus`, `StopMCPDaemon`, and holder metadata read/reverification seams MUST resolve leaf paths against the verified absolute `runtimeDir` (never against a relative `runtimeRoot.Name()` string from `storageHandle.OpenRoot("runtime")` that resolves against process CWD on Windows Go 1.24; scoped to Wave-1 prerequisite unit `U1b` / `211.008-T`). Separately, in `U4` and `U5`, the sidecar schema MUST be extended backward-compatibly to advertise the local session IPC endpoint and active session metadata, and status/handshake probing (`backlogit mcp status` and new session connects) MUST NOT take an exclusive lock probe that causes concurrent startups to fail spuriously with `ErrDaemonAlreadyRunning` (addressing constraint `A045DBA4`).

### Per-Session Isolation

* **R4 (Per-Session Identity and Independent Disconnect Lifecycle)**: Each connected agent stream MUST register a distinct `mcpserver.ClientSession` with a unique `sessionID` via `MCPServer.RegisterSession` and `MCPServer.WithContext` (bypassing `StdioServer.Listen`'s static `stdioSessionInstance`). When one client session disconnects (`stdin` EOF or IPC stream closure), the runtime MUST unregister only that `sessionID` (`MCPServer.UnregisterSession`) and keep serving all remaining active sessions.
* **R5 (Per-Session Request ID and Response Routing Isolation)**: JSON-RPC 2.0 request IDs (`id`) MUST be isolated per session connection. Two concurrent sessions issuing requests with identical JSON-RPC IDs (for example, `"id": 1`) MUST receive their respective responses on their own transport streams with zero cross-session framing interleaving or response misrouting.
* **R6 (Per-Session Cancellation and Notification Isolation)**: Cancelling a request (`notifications/cancelled`) or closing a client session MUST cancel only the `context.Context` associated with that session's in-flight or queued request(s), never aborting operations belonging to other sessions. Server-initiated notifications (`SendNotificationToClient` / `SendNotificationToSpecificClient`) MUST be delivered only to the target session's `NotificationChannel` (with workspace-wide resource notifications delivered only to initialized sessions).

### Internal Operation Gating, Concurrency, and Backpressure

* **R7 (Go Channel-Gated FIFO Mutation Serialization for `backlogit` Runtime-Managed Operations)**: All **`backlogit` runtime-managed** workspace-mutating MCP operations—including daemon-managed writes to `.backlogit/queue/`, `.backlogit/archive/`, `.backlogit/stash.jsonl`, `.backlogit/hooks_queue.jsonl`, `.backlogit/checkpoints/`, `.backlogit/ops/`, shared event logs, in-memory manifest state, and `backlogit.db` write transactions—MUST be gated through an internal Go channel-based operation coordinator that executes `backlogit` runtime mutations sequentially in deterministic FIFO admission order. This Go channel coordinator is an internal `backlogit` runtime component, **not** a universal mutation scheduler or broker for every external editor, one-shot CLI process, or independent agent/MCP process in the workspace: existing cross-process advisory file locks (`.backlogit/.locks/`) and SQLite `BEGIN IMMEDIATE` + `busy_timeout(30000)` consistency duties (`R12`) remain required, and external/unmanaged writer coordination is recognized as a separate boundary concern rather than an implied cross-tool channel broker. (Specific internal Go struct and channel names are deferred to implementation planning `D1`.)
* **R8 (Bounded Concurrent Read Execution Without Forced Serialization)**: Read-only MCP operations (`backlogit_get_item`, `backlogit_list_items`, `backlogit_search_items`, `backlogit_query_sql`, `backlogit_get_queue`, `backlogit_get_shipment`, `backlogit_list_shipments`, `backlogit_get_dependencies`, `backlogit_get_links`, `backlogit_fetch_stash`, `backlogit_stash_get`, `backlogit_list_checkpoints`, `backlogit_get_checkpoint`, `backlogit_docs_scope`, `backlogit_docs_classify`, `backlogit_docs_lint`, `backlogit_get_metadata_catalog`, `backlogit_get_version`) MUST NOT be forced to wait single-file behind unrelated read calls; they SHOULD run concurrently under a bounded read-admission limiter backed by SQLite WAL concurrent readers (currently configured with a 4-connection pool in `internal/db`), subject only to explicit consistency barriers when a full index rehydration or lazy workspace initialization is in progress.
* **R9 (Bounded Admission Queue, Backpressure Errors, and Synchronous Lock Ownership)**: The channel operation coordinator MUST enforce a bounded pending-operation queue capacity and respect caller context deadlines while queued:
  * If the admission channel is full or the caller's context deadline expires before admission, the call MUST abort before mutating state and return a structured, machine-readable busy/timeout error envelope (`retryable: true`) with remediation guidance.
  * Per `docs/compound/best-practices/timeout-goroutine-lock-ownership-go-2026-07-01.md`, resource locks and admission slots MUST be owned in the synchronous execution frame (never stranded inside an abandoned timeout goroutine).

### Async Callbacks, CLI/Launcher Compatibility, and Lifecycle Ownership

* **R10 (Capability-Gated Progress Notifications and Async Callback Investigation)**: Standard MCP `tools/call` requests MUST continue to return synchronous `CallToolResult` responses so existing agent harnesses work without modification. As a conservative first-release proposal/assumption (`A3`), where supported by the protocol and client session, long-running gated operations (such as `backlogit_ship_shipment`, `backlogit_sync_index`, or `backlogit_merge_sync`) SHOULD emit standard MCP server-to-client progress or diagnostic notifications (`notifications/progress` when `_meta.progressToken` is present, or `notifications/message`) over the calling session's `NotificationChannel()`. Exact client/SDK support is investigated during planning (`D8`); persistent async job tickets, arbitrary reverse RPC, or unverified SDK capability guarantees are NOT assumed or approved for v1.
* **R11 (Deterministic Daemon Startup, Idle Shutdown, and Multi-Session `status`/`stop`/`--replace`)**: The runtime MUST define clear lifecycle ownership across sessions:
  * `backlogit mcp status [--format text|json]` MUST report the singleton daemon PID, start time, version, lock path, IPC endpoint status, and count of currently attached MCP sessions.
  * `backlogit mcp stop` and `backlogit mcp --replace` MUST safely drain or terminate the verified daemon holder using the retained OS process-handle identity check from `204-F`, cleanly closing all attached session streams.
  * When all attached client sessions disconnect (`active_sessions == 0`), the independent background daemon (`R13`) MUST follow a deterministic idle shutdown policy (releasing `mcp-daemon.lock` and `mcp-daemon.json` after a bounded idle grace period configured during planning `D3`, or immediately on explicit `backlogit mcp stop`).
* **R12 (One-Shot CLI Coexistence and Workspace-Local `bin/backlogit.exe` Reference)**: One-shot CLI invocations (`backlogit list`, `backlogit sync`, `backlogit shipment ...`) MUST remain runnable alongside the active multi-session MCP runtime using existing cross-process advisory locks (`.backlogit/.locks/`) and SQLite `BEGIN IMMEDIATE` + `busy_timeout(30000)`. Both `.mcp.json` (`"command": "./bin/backlogit.exe"`) and `start.ps1` (`$sidecarExecutables.backlogit = (Join-Path $workspaceBinDirectory "backlogit.exe")`) MUST reference `bin/backlogit.exe` as the workspace-local executable, while keeping the runtime IPC/sidecar contract path-agnostic so the separate `.backlogit/` self-install initiative (`17214531`) is not coupled or blocked.

### Confirmed Independent Daemon Lifecycle, Startup Discovery, and Workspace Containment (2026-11-04T20:15:01Z)

* **R13 (Confirmed Independent Per-Workspace Daemon Lifecycle)**: Per operator confirmation (`2026-11-04T20:15:01Z`), the runtime lifecycle model MUST be an **independent per-workspace background daemon** paired with thin `stdio`-compatible MCP client session shims (`Option A`). Multiple concurrent client sessions MUST remain drop-in `stdio`-compatible, and the exit, EOF, or termination of any individual client session (including the first client that triggered daemon startup) MUST NOT terminate the daemon or disrupt other active sessions.
* **R14 (Dual Startup Entry Routes: Idempotent `start.ps1` Prewarm and Lazy First-Instantiation Auto-Start)**: The runtime MUST support **both** startup entry routes as an explicit product requirement without requiring MCP clients to be launched from `start.ps1`:
  * **Optional/idempotent prewarm via `start.ps1`**: `start.ps1` MAY prewarm or verify the workspace daemon idempotently using the workspace-pinned `bin/backlogit.exe` binary; if a compatible daemon is already running for the workspace, prewarm MUST succeed cleanly without spawning a duplicate daemon or disrupting attached sessions.
  * **Lazy auto-start/connect on first `backlogit mcp` invocation**: When no workspace daemon is running (for example, when an IDE or agent host launches `./bin/backlogit.exe mcp` directly without running `start.ps1`), the first `backlogit mcp` invocation MUST automatically initialize the background daemon for the workspace and attach its `stdio` session.
  * **Unified singleton admission and race convergence**: Both entry routes MUST share the exact same singleton admission authority (`mcp-daemon.lock` + `mcp-daemon.json`) and workspace-pinned binary identity. Concurrent first-client invocations and/or a simultaneous `start.ps1` prewarm MUST converge deterministically to **one** active daemon instance, recovering cleanly from stale sidecar files without false-positive readiness or split-brain execution.
* **R15 (Trusted Workspace Resolution and Pre-Attach Identity/Compatibility Verification)**: Workspace resolution and daemon attachment MUST derive from explicit, trusted workspace context and the workspace-pinned executable association (`bin/backlogit.exe`), never from an arbitrary untrusted ambient working directory alone or whichever `backlogit` binary happens to appear first on the system `PATH`:
  * Before attaching a client session or completing prewarm against an existing daemon, the caller MUST verify the canonical workspace root and storage root identity, the workspace-pinned binary path, and a compatible daemon version/build identity.
  * The caller MUST explicitly reject wrong-workspace or version/build mismatches with a structured error. It MUST NOT silently adopt a foreign workspace's daemon, silently kill/replace an active daemon serving peer sessions without explicit operator replacement (`--replace`), or fall back to a global `PATH` binary when the workspace-pinned binary is missing or mismatched. (Bounded recovery UX and exact CLI flags are deferred to planning `D4`, `D6`.)
* **R16 (Workspace-Local Daemon State Confinement vs Platform IPC Namespace Distinction)**: All daemon-managed filesystem state (sidecar metadata, lock files, logs, caches, checkpoints, and SQLite database/WAL files where used or introduced by this feature) MUST remain strictly under configured workspace storage/runtime paths (such as `.backlogit/` and `<storage_root>/runtime/`), with zero system-wide installation directories or outside-workspace state writes. Transport endpoint specifications MUST distinguish platform namespaces accurately:
  * **POSIX Unix Domain Sockets** use a filesystem socket path bound to the workspace runtime directory (with bounded path-length fallback deferred to `D2`) and `0600` owner-only permissions.
  * **Windows Named Pipes** reside in the Windows kernel named-pipe object namespace (`\\.\pipe\...`) and cannot literally be filesystem files inside the workspace tree; they MUST instead use a deterministic workspace-bound endpoint identity paired with current-user SID access control (`DACL`) and mandatory pre-attach workspace/version handshake verification (`R15`). Exact pipe/socket naming formulas are deferred to planning (`D2`).
* **R17 (Workspace-Scoped Application Containment of `backlogit`-Managed Tools and Helpers — `backlogit` Component Within All-Tools Scope)**: As the `backlogit`-specific component of the broader workspace tool environment (`R19`–`R22`), `backlogit`'s own runtime, MCP tools/resources, and any `backlogit`-managed helpers MUST be workspace-scoped:
  * **Canonical workspace file confinement**: All `backlogit` MCP tool and resource file operations MUST stay within the canonical authorized workspace and storage root, deterministically rejecting `..` path traversal, drive-relative escapes, and symlink/junction/reparse-point escapes.
  * **Session-to-workspace binding and no cross-workspace leakage**: Startup handshake and tool/resource discovery (`initialize`, `tools/list`, `resources/list`) MUST bind each session to the verified canonical workspace and compatible daemon capabilities, preventing cross-workspace session rebinding or tool/state leakage across workspaces.
  * **No implicit global `PATH` discovery for managed helpers**: The `backlogit` runtime and any `backlogit`-managed helper executables MUST resolve exclusively through explicit workspace-local/pinned configuration (preserving `bin/backlogit.exe` for this initiative while keeping portable `.backlogit/` binary installation separate in stash `17214531`), with zero implicit global `PATH` discovery.
  * **Component role**: Satisfying `R17` is necessary for `backlogit` as one participant in the workspace, but does NOT by itself satisfy the all-tools workspace outcome (`R19`–`R22`).
* **R18 (Explicit Threat-Model Boundary: Workspace Pinning and `backlogit` Path Containment vs Deferred OS/Container Sandboxing)**: The product specification and runtime documentation MUST state the factual security threat-model boundary across the workspace tool environment:
  * Per operator confirmation (`2026-11-04T22:02:03Z`, `Q3`, `R23`), the initial release enforces **workspace-local tool installation, configuration, and version pinning** across workspace tools (`R19`, `R23`), pre-attach workspace/version handshake validation (`R15`), and `backlogit`'s own application-level filesystem path containment (`*os.Root`, `R17`) against accidental cross-workspace attachment, path traversal, and global `PATH` shadowing.
  * Placing an executable inside a workspace directory, pinning versions in workspace configuration, or listing tools in a workspace manifest does **not** create an OS-enforced sandbox: a normal user-space process running with standard host account permissions can still read/write host paths outside the workspace, spawn child processes, or open network sockets unless constrained by an OS/container isolation boundary (`R20`, `R23`). OS/container sandboxing and hard process/network/host-filesystem isolation are **deferred / out of scope** for this initial release (`R23`, `SC14`).
  * `backlogit`'s own path containment checks (`R17`) govern `backlogit` operations only and MUST NOT be claimed to confine unrelated external agent/MCP tools or host utilities (`R20`, `R23`).
  * Workspace-writable configuration files (such as `.mcp.json`, `start.ps1`, or workspace config files) MUST NOT be treated as a hostile-agent security boundary unless backed by an independent out-of-workspace policy authority (`A7`, `D7`); no new authentication or policy-authority infrastructure is implemented in this initial release.

### Workspace-Owned All-Tools Environment, Containment Boundaries, and Phased Dependency Structure (2026-11-04T21:41:00Z & 2026-11-04T22:02:03Z)

* **R19 (Workspace-Owned Agent and MCP Tool Environment Across All Workspace Tools)**: Per explicit operator breadth confirmation (`2026-11-04T21:41:00Z`: `'I mean ALL agent/MCP tools used in a workspace.'`) and containment-strength confirmation (`2026-11-04T22:02:03Z`: `'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), the product outcome encompasses a **workspace-owned agent/MCP tool environment** covering `backlogit`, other workspace MCP servers (such as `engram`, `graphtor-docs`, and any additional workspace-configured MCP servers), agent-invoked CLI/helper executables and scripts, and the launcher/adapter configurations (`.mcp.json`, `start.ps1`, `start.sh`, and host/harness tool definitions) used to expose and run them:
  * **Shared `backlogit` runtime as one participant, not universal broker**: The multi-session `backlogit` daemon (`R1`–`R17`) is one participant/consumer within this workspace tool environment. It is not a runtime proxy, universal mutation scheduler (`R7`), process supervisor, or security enforcer for independent MCP servers or built-in editor/agent host tools that execute outside `backlogit`.
  * **Core workspace tool-environment contract subjects**:
    1. **Inventory and discovery**: Every MCP server, CLI helper, and host/adapter tool exposed to agents in a workspace MUST be explicitly inventoried and discoverable as part of the workspace's declared tool surface.
    2. **Workspace-local pinned executable, configuration, and dependency resolution**: Local MCP server binaries, shims, and CLI helpers MUST resolve from workspace-local installations, workspace-scoped configurations, and workspace-pinned versions/paths with **zero silent fallback to the global system `PATH`** or another workspace's binaries (`R23`). Any unavoidable host/bootstrap prerequisite or exception (for example, the OS shell `pwsh`/`powershell`/`sh` or host runtime needed to invoke `start.ps1`) MUST be declared explicitly as a host prerequisite rather than disguised as fully local tool compliance.
    3. **Workspace identity and allowed capabilities**: Every workspace-managed tool/server launch and startup handshake MUST bind to the canonical workspace root and declared capability policy so two workspaces on the same host never share mutable tool state or cross-attach.
    4. **Provenance and compatible version checks**: Workspace startup/prewarm and tool adapters MUST verify executable existence, workspace provenance, and version/build compatibility before launch or attachment.
    5. **Startup and launcher/adapter contracts**: Workspace startup entry points (`start.ps1`, `start.sh`, `.mcp.json`, and supported host adapters) MUST apply consistent workspace-local installation, configuration, and version-pinning rules across all configured workspace tools. (Exact manifest/lockfile schema, package manager, or installer layout is not pre-selected here and is deferred to `D9` while keeping stash `17214531` separate.)
* **R20 (Distinction Between Tool Location/Availability and Runtime Data/Process/Network Confinement; Honest Unsupported and Exception Tool Reporting)**: The workspace tool environment MUST separate **workspace-local tool installation, configuration, and version pinning** from **runtime data, process, and network confinement**:
  * **Location/pinning vs confinement**: Ensuring that a tool binary is installed locally in the workspace, configured per workspace, and version-pinned (`R19`, `R23`) guarantees reproducible local tool resolution and prevents global `PATH` drift, but does **not** by itself prevent that process from accessing outside-workspace host files, environment variables, processes, or networks (`R18`).
  * **`backlogit` path containment vs external tools and built-in/remote surfaces**: While `backlogit` enforces its own canonical workspace path containment (`R17`), this specification does **not** claim that every external agent/MCP tool obeys `backlogit`'s path rules or that `backlogit` can confine unrelated host tools. Built-in editor/agent host tools (for example, host-native file, shell, or search tools) and hosted/remote MCP services cannot be made workspace-local or confined by `backlogit`, and OS/container-level runtime sandboxing is deferred out of this initial release (`R23`). Nothing in this specification claims control over the current agent runtime's external tool surface.
  * **Honest, explicit unsupported/non-conformant/prerequisite reporting**: Any configured or inherited tool, built-in host tool, host bootstrap prerequisite, or remote MCP server that cannot satisfy workspace-local installation, configuration, and version pinning MUST produce **explicit, structured reporting** (`unsupported`, `non-conformant`, `host-prerequisite`, or `remote-declared`, naming the tool, category, and policy reason) rather than being disguised or silently counted as compliant (`SC12`, `SC13`). Unpinned local tools where a workspace-pinned binary is required MUST fail closed without silent global `PATH` fallback (`SC8`, `SC12`).
* **R21 (Authorized Remote Service Access Policy Separation vs Workspace Tool Binding)**: The all-tools breadth scope (`2026-11-04T21:41:00Z`) and workspace-local pinning scope (`2026-11-04T22:02:03Z`) govern workspace tool accounting, binding, and local-pinning policy, and MUST NOT be conflated with a blanket ban on all remote network APIs or an expansion into building cloud integrations:
  * Whether a workspace permits specific authorized remote service access (such as Azure DevOps work-item synchronization in stash `D895887C`, GitHub API calls, or remote documentation/model endpoints) is a **separate workspace policy decision**, neither banned nor settled by `Q2` or `Q3`.
  * Tools MUST execute where declared and approved with explicit workspace binding: locally executed tools must use workspace-installed, configured, and version-pinned executables (`R19`, `R23`), whereas any policy-approved remote/hosted MCP or API tool MUST bind to workspace-scoped configuration/credentials and be reported transparently (`R20`, `SC13`) rather than subjected to an impossible literal local-binary path requirement or expanded into implementing new external auth/integration providers in this initiative.
* **R22 (Multi-Track Capability and Dependency Structure Across Workspace Tool Environment, Multi-Session Runtime, and Adapters)**: Planning and delivery for `ECF33C79` / `082-DL` MUST structure work across three distinct capability tracks without conflating internal `backlogit` daemon mechanics with completion of the whole workspace tool environment:
  * **Track A — Workspace Tool-Environment Contract (`R15`–`R21`, `R23`; Settled at Requirements Level by `Q3` `2026-11-04T22:02:03Z`)**: Defines the cross-tool workspace inventory/manifest contract, workspace-local installation/configuration/version-pinning rules, explicit host bootstrap prerequisite and exception declarations, workspace identity/version compatibility checks, and honest unsupported/remote tool reporting for the approved Level 1 initial release.
  * **Track B — `backlogit` Multi-Session Singleton Daemon & Channel-Gated Runtime (`R1`–`R17`)**: Delivers `backlogit`'s independent per-workspace background daemon (`Option A`), `stdio`-to-IPC session shims, dual startup routes (`start.ps1` prewarm + lazy first-instantiation auto-start), per-session `ClientSession` isolation, FIFO Go channel mutation coordinator for `backlogit`-managed operations + concurrent reads, and `backlogit`'s own workspace-pinned `bin/backlogit.exe` and `*os.Root` path containment.
  * **Track C — Cross-Tool Launcher/Host Adapters, Companion MCP Alignment, & Local Pinning Validation (`R19`–`R23`, `SC10`–`SC14`)**: Applies the Track A workspace-local installation, configuration, and version-pinning contract across `.mcp.json`, `start.ps1`/`start.sh`, companion workspace MCP servers (`engram`, `graphtor-docs`, etc.), CLI helpers, and supported host/launcher adapters, together with explicit non-conformant/prerequisite/remote tool reporting and multi-workspace/local-pinning negative verification suites (`SC10`–`SC14`, with OS/container sandboxing deferred).
  * **Completion invariant**: While the Track B `backlogit` runtime slice can be developed independently of external adapters, the overall all-tools product outcome is **not** complete without Track A and Track C all-tools evidence fulfilling the `2026-11-04T21:41:00Z` and `2026-11-04T22:02:03Z` scope.
* **R23 (Confirmed Initial-Release Containment Strength — Workspace-Local Installation, Configuration, and Version Pinning; OS/Container Sandboxing Deferred)**: Per explicit operator confirmation at `2026-11-04T22:02:03Z` (`'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), the initial release of the workspace-owned agent/MCP tool environment MUST implement **Level 1 containment strength**:
  * **Workspace-local installation, configuration, and version pinning**: All workspace-managed agent/MCP tools, companion MCP servers (`backlogit`, `engram`, `graphtor-docs`, etc.), shims, and CLI helpers MUST be locally installed or provisioned for the workspace, configured via workspace-scoped config, and pinned to explicit compatible versions/paths with startup provenance and compatibility verification and **no silent global `PATH` or cross-workspace executable fallback** (`R15`, `R19`, `SC10`–`SC12`).
  * **Explicit bootstrap prerequisites and honest limitation reporting**: Host bootstrap prerequisites or exceptions (such as the OS shell needed to invoke `start.ps1`/`start.sh`) and unsupported/built-in/remote tooling limitations MUST be declared and reported explicitly (`R19`, `R20`, `SC12`, `SC13`), never disguised as fully local tool compliance.
  * **`backlogit` path containment retained as component behavior**: `backlogit`'s own canonical workspace path containment (`*os.Root` and path/symlink/reparse traversal refusal in `R17`, `SC9`) remains required product behavior for `backlogit`, without claiming that external agent tools obey `backlogit`'s path checks or that `backlogit` confines unrelated host tools.
  * **OS/container sandboxing deferred**: OS- or container-enforced process, filesystem, and network sandboxing (`Level 2`) and kernel/container adversarial escape validation are **deferred / out of scope** for this initial release (`R18`, `SC14`), and local placement/pinning MUST NOT be claimed to provide OS/container isolation.

### Capability and Dependency Matrix (Conceptual DAG)

```text
[Q3 Resolved (2026-11-04T22:02:03Z): Workspace-Local Installation, Configuration, & Version Pinning (R23)]
                 │
                 ▼ (Requirements-level containment strength settled: Level 1 local setup/pinning; OS sandbox deferred)
┌───────────────────────────────────────────────────────────────────────────────┐
│ Track A: Workspace Tool-Environment Contract (R15-R21, R23)                   │
│ - Tool inventory & classification (local-pinned, remote-declared, host/prereq)│
│ - Local installation/config/version pinning & no silent global PATH fallback  │
│ - Workspace identity/version handshake & honest unsupported/exception reports │
└──────────────────────┬───────────────────────────────────┬────────────────────┘
                       │ (shared workspace/startup         │ (TRUE BLOCKER for cross-tool
                       │  identity conventions;            │  adapter alignment & all-tools
                       │  Track B runtime slice can be     │  compliance verification)
                       │  developed independently)         ▼
                       ▼                   ┌────────────────────────────────────────────────────┐
┌────────────────────────────────────────┐ │ Track C: Companion MCP & Adapter Integration       │
│ Track B: backlogit Multi-Session       │ │ (R19-R23, SC10-SC14)                               │
│ Singleton Runtime (R1-R17)             │ │ - .mcp.json / start.ps1 / start.sh local pinning   │
│ - Independent daemon + stdio IPC shims │─┼─► Companion MCP servers (engram, graphtor-docs)    │
│ - start.ps1 prewarm + lazy auto-start  │ │ - Host/launcher adapter & prerequisite checks      │
│ - Per-session isolation + FIFO channel │ │ - Honest unsupported reports & local negative tests│
│   (backlogit-managed mutations only)   │ │   (OS/container sandboxing deferred per R23)       │
└────────────────────────────────────────┘ └────────────────────────────────────────────────────┘
```

| Capability Track | Core Deliverables | True Blocker Relationships | Proposed Phased Delivery Relationships |
|---|---|---|---|
| **Track A: Workspace Tool-Environment Contract (`R15`–`R21`, `R23`; Settled at Requirements Level via `Q3`)** | Cross-tool inventory contract, workspace-local installation/configuration/version-pinning rules, explicit bootstrap prerequisite/exception policy, workspace identity/compatibility checks, and honest unsupported/remote tool reporting | **Settled at requirements level (`2026-11-04T22:02:03Z`)**: `Q3` resolved to workspace-local installation, configuration, and version pinning (`R23`). **True blocker for Track C** cross-tool adapter compliance and verification | Concrete manifest/schema and adapter mechanics (`D9`) are designed in `impl-plan` alongside Track B |
| **Track B: `backlogit` Multi-Session Singleton Runtime (`R1`–`R17`)** | Independent per-workspace `backlogit` daemon, `stdio`-to-IPC shims, `start.ps1` prewarm + lazy first-use startup, `ClientSession` isolation, Go channel FIFO write gate for `backlogit`-managed mutations + concurrent reads, `bin/backlogit.exe` & `*os.Root` confinement | **Not a universal broker or scheduler for non-`backlogit` tools (`R7`, `R19`)**; internal Go channel coordinator and `stdio` shim mechanics (`R1`–`R16`) do not require external Track C adapters to exist first | **Can be developed independently as a runtime slice**, and participates in Track C end-to-end workspace verification; **broader outcome still requires Track A + Track C all-tools evidence** |
| **Track C: Cross-Tool Adapters, Companion MCP Alignment, & Local Pinning Validation (`R19`–`R23`, `SC10`–`SC14`)** | Launcher/config verification (`.mcp.json`, `start.ps1`, `start.sh`), companion MCP server local installation/config/version pinning & workspace binding (`engram`, `graphtor-docs`), explicit prerequisite/unsupported/remote reporting, and multi-workspace/local-pinning negative test suites | **True blocker dependency on Track A (`R19`–`R23`)** and integrates **Track B** for `backlogit` to satisfy the all-tools workspace outcome | Delivered after/alongside Track A and Track B (with OS/container sandboxing and OS-level adversarial escape validation deferred to a future phase per `R23`/`SC14`). No recorded backlog `blocks` edges or shipment IDs (`NNN-S`) are invented at intake |

## Success Criteria

* **SC1 (Multi-Agent Session Concurrency)**: At least two independent `backlogit mcp` client sessions against the same workspace simultaneously complete `initialize`, `notifications/initialized`, `tools/list`, and concurrent `tools/call` requests without `ErrDaemonAlreadyRunning` or transport errors.
* **SC2 (Session Isolation and First-Client Disconnect Survival)**: With two or more active sessions using overlapping JSON-RPC request IDs (e.g., both sending `"id": 1`, `"id": 2`), every response and session-targeted notification arrives on the exact originating session stream; disconnecting or terminating Client 1 (including the client that triggered lazy daemon startup) leaves the background daemon and Client 2 healthy and able to execute subsequent read and write tool calls.
* **SC3 (Deterministic FIFO Mutation Gating and Concurrent Reads)**: Concurrent mutating tool calls (`backlogit_create_item`, `backlogit_update_item`, `backlogit_append_comment`, `backlogit_stash_edit`) from multiple sessions execute sequentially in FIFO admission order with zero lost updates, zero interleaved JSONL event lines, and consistent manifest state, while concurrent read-only tool calls execute in parallel without serializing behind one another.
* **SC4 (Admission Backpressure and Timeout Safety)**: When the mutation admission channel is saturated or a queued request's `context.Context` is cancelled while waiting for the gate, the caller receives a deterministic structured cancellation/busy error, zero partial mutations occur, and the gate token is immediately released to the next queued request with no leaked goroutines or stranded locks.
* **SC5 (Lifecycle and Path Contract Verification)**: `backlogit mcp status` accurately reports active session metadata, `backlogit mcp stop` and `--replace` cleanly shut down the runtime, and `.mcp.json` and `start.ps1` both resolve `bin/backlogit.exe` cleanly.
* **SC6 (Startup Entry Routes and Cold-Start Race Convergence — Positive Cases)**:
  * **SC6a (`start.ps1` Prewarm Then Multiple Clients)**: Running `start.ps1` prewarms (or idempotently verifies) the workspace daemon using `bin/backlogit.exe`; subsequent multiple `backlogit mcp` stdio clients attach to that prewarmed daemon without starting a second daemon.
  * **SC6b (No-Prewarm Lazy First-Use Startup)**: When `start.ps1` has not been run, the first `backlogit mcp` invocation lazily initializes the workspace daemon pinned to `bin/backlogit.exe` and serves its session; later clients attach to that same daemon.
  * **SC6c (Simultaneous Cold Startup Converges to One Daemon)**: When two or more `backlogit mcp` clients (and/or `start.ps1` prewarm) launch concurrently against a workspace with no live daemon (including when a stale `mcp-daemon.json` from a dead process exists), all callers converge to **exactly one** live daemon instance with zero split-brain daemons and zero stale-sidecar false successes.
* **SC7 (Cross-Workspace Isolation and Mismatch Refusal — Negative Cases)**:
  * **SC7a (Distinct Workspaces Never Attach or Cross-Read)**: Two distinct workspaces (`Workspace A` and `Workspace B`) on the same machine never attach to each other's daemon, share session state, or cross-read/write each other's `.backlogit/` storage, even when sessions run concurrently under the same OS user.
  * **SC7b (Incompatible Version/Build or Wrong-Root Daemon Explicit Refusal)**: If a client encounters a daemon or sidecar advertising a different canonical workspace/storage root or an incompatible binary path/version/build, the client explicitly refuses attachment with a structured error and neither adopts the mismatched daemon nor silently kills active peer sessions.
* **SC8 (Missing Workspace Binary No Global `PATH` Fallback — Negative Case)**: If the workspace-pinned `bin/backlogit.exe` binary is missing, startup (`start.ps1` prewarm and `backlogit mcp` daemon launch) fails closed with an explicit missing-binary error and never falls back to a `backlogit` binary discovered on the global system `PATH`.
* **SC9 (Managed Helper and Path/Reparse Escape Refused — Negative Case)**: Any `backlogit` MCP tool/resource call that attempts to read or write outside the canonical workspace root (via `..` segments, absolute external paths, drive-relative paths, or symlinks/junctions/reparse points targeting outside the workspace) or any attempt to resolve a managed helper via implicit global `PATH` lookup is deterministically refused.
* **SC10 (Multi-Workspace All-Tools Identity, Config, and State Separation)**: Two distinct workspaces (`Workspace A` and `Workspace B`) on the same machine maintain separate tool identities, launcher/MCP configurations, runtime endpoints, and workspace-local state across all workspace-managed agent/MCP tools (including `backlogit` and companion workspace MCP servers/helpers), with zero cross-workspace attachment, config bleed, or state contamination.
* **SC11 (Complete Per-Host Workspace Tool Inventory and Accounting)**: Every agent/MCP tool, companion MCP server, and CLI helper exposed by each supported host/launcher configuration in a workspace is explicitly accounted for in the workspace tool inventory with its declared source, workspace binding, and containment classification (zero uninventoried shadow tools).
* **SC12 (No Silent Cross-Workspace or Global `PATH` Executable Resolution Across Workspace Launch/Setup — Negative Case)**: Workspace launch, configuration, and setup flows (`.mcp.json`, `start.ps1`, `start.sh`, and workspace tool adapters) never silently resolve an executable or helper from another workspace or from the global system `PATH` when a workspace-pinned binary is required; any unavoidable host/bootstrap prerequisite (such as the OS shell) is explicitly declared and validated rather than hidden via ambient `PATH` fallback.
* **SC13 (Explicit Policy Reporting for Unsupported, Remote, or Host-Inherited Tools)**: Any built-in editor/agent host tool, remote/hosted MCP service, or host-inherited utility that cannot be pinned as a workspace-local binary or confined by workspace-local mechanisms is explicitly reported with its declared policy classification (`unsupported`, `non-conformant`, `remote-declared`, or `host-prerequisite`) rather than silently treated as workspace-contained.
* **SC14 (Local Identity, Pinning, and `backlogit` Path-Containment Negative Verification in Initial Release; OS/Container Adversarial Escape Validation Reclassified as Later Work)**: Per the `2026-11-04T22:02:03Z` `Q3` resolution (`R23`), required verification for this initial release covers concrete negative tests for the approved workspace-local installation, configuration, and version-pinning phase—unpinned global `PATH` shadowing refusal and cross-workspace binary/config resolution refusal (`SC8`, `SC12`), cross-workspace IPC attach and version/root mismatch refusal (`SC7`, `SC10`), `backlogit`'s own canonical workspace path/symlink/reparse traversal refusal (`SC9`), and honest explicit reporting for host prerequisites and unsupported/built-in/remote tools (`SC13`). OS/container-level adversarial escape validation (outside-workspace kernel filesystem, process-spawn, and network escape probes across external tools) is explicitly **reclassified as deferred later-phase work** alongside OS/container sandboxing (`Level 2`) and is **not** a launch blocker for this initial release.

## Scope Boundaries

### In Scope

* Designing and specifying the workspace-owned agent/MCP tool environment contract across **all** agent/MCP tools used in a workspace (`R19`–`R23`), using **workspace-local installation, configuration, and version pinning** (`Q3` resolved `2026-11-04T22:02:03Z`), explicit tool inventory/discovery, zero silent global `PATH` or cross-workspace executable fallback, explicit host/bootstrap prerequisite and exception declarations, workspace identity/version compatibility verification, launcher/adapter startup contracts (`.mcp.json`, `start.ps1`, `start.sh`), and honest unsupported/built-in/remote tool reporting (`SC10`–`SC14`).
* Designing and specifying the `backlogit` singleton multi-session MCP runtime architecture (`internal/mcp`, `internal/cli/mcp_daemon.go`, and session transport adapter) as a core participant in the workspace tool environment (`R1`–`R17`).
* Supporting multiple simultaneous stdio-initiated `backlogit mcp` client sessions per workspace via an independent per-workspace background daemon (`R13`), a local IPC session bridge, and per-session `mcpserver.ClientSession` registration (`R2`, `R4`).
* Supporting both idempotent `start.ps1` prewarm and lazy first-instantiation `backlogit mcp` startup using the same singleton admission and workspace-pinned `bin/backlogit.exe` binary (`R14`, `R15`).
* Enforcing workspace-local daemon state placement and platform-accurate IPC endpoint rules (POSIX Unix Domain Socket filesystem paths vs Windows Named Pipe OS namespace endpoints bound to workspace identity, `R16`).
* Retaining `backlogit`'s canonical workspace path containment checks for `backlogit`-managed tools, file operations, session binding, and any `backlogit`-managed helpers (`R17`), together with an explicit threat-model boundary stating that local installation/pinning does not provide OS/container sandboxing and `backlogit` path checks do not confine unrelated external host tools (`R18`, `R20`, `R23`).
* Implementing a Go channel-based internal operation coordinator in `backlogit` that gates and serializes **`backlogit` runtime-managed** workspace mutations FIFO while allowing bounded concurrent read-only operations and preserving cross-process lock/SQLite coexistence duties (`R7`–`R9`, `R12`).
* Defining per-session request ID, cancellation, and notification channel isolation (`R4`–`R6`).
* Specifying conservative synchronous `tools/call` + MCP progress/notification callbacks during long-running operations (`R10`, `A3`, `D8`).
* Extending `backlogit mcp status`, `stop`, and `--replace` for multi-session visibility and lifecycle control (`R11`).
* Preserving `bin/backlogit.exe` in `.mcp.json` and `start.ps1` as the workspace-local `backlogit` executable (`R12`).
* **Wave-1 prerequisite repair for `C2CFE06E` (`085-DL`, folded in `2026-11-06` per P-021 C4/C6)**: Propagating verified absolute `runtimeDir` across `internal/cli/mcp_daemon.go` lock and sidecar symlink/reparse validation seams (`U1b` / `211.008-T`, strictly bounded to `internal/cli/mcp_daemon.go` and `internal/cli/mcp_daemon_lock_test.go`) while preserving `*os.Root` handle-relative operations and `os.SameFile(opened, linked)` handle-to-handle-relative-`Lstat` identity binding (`R3`, `SC5`, `SC9`).
* **Wave-1 convergence prerequisite repairs for `60D799D9` (`086-DL`, `U1c` / `211.009-T` and `U1d` / `211.010-T`) and `19D55778` (`087-DL`, `U1e` / `211.011-T`, folded in `2026-11-06` per P-021 C4/C6 while dark execution is paused at Wave 1 Step 4.6)**:
  1. Closing the exit-0 ancestor-fallback gap on present-but-unresolved local `.git` directories in `planArtifactMove` (`internal/core/archive.go`) and `(*Workspace).inGitWorktreeBounded` (`internal/core/shipment_gate.go`), while preserving production environment sanitization (`gate.MinimalEnv`, `isGitOverrideEnv`, `policyengine.gitEnv`) and valid nested-workspace/unborn/bare repository semantics, and isolating `TestInGitWorktreeBounded` subcase `(b)` via the existing cross-platform `writeFakeGit` helper (`U1c` / `211.009-T`).
  2. Isolating no-usable-repository test fixtures in `TestHeadSHABounded` (`internal/core/shipment_gate_headdrift_test.go`), `TestRepairShipmentMemberEvidence_NoRepoHead_Errors` (`internal/core/shipment_repair_test.go`), and `TestU163_002_ExecGitRequiresARepositoryDirectory` (`internal/policyengine/u163_002_record_test.go`) using owned invalid `.git` file boundaries and `initGitRepoNoCommits` (`U1d` / `211.010-T`).
3. **Completing activated `releasedTask` members via governed `core.UpdateArtifact` before `core.ShipShipment` and disabling unconfigured external `autoharness` subprocess gate brokers in non-gate Doctor and integration release fixtures (`internal/core/025_doctor_harness_test.go` and `tests/integration/shipment_workflow_test.go`, `U1e` / `211.011-T`)**.
  4. **Pending-verification no-code diagnostic disposition for `96F1DFFD` (`086-DL`, subsequently superseded for the `ReconstituteShipments`/`AssociateCommit` overlap by `088-DL` / `U1f` `211.012-T` below)**: initially retained `96F1DFFD` active in `.backlogit/stash.jsonl` pending post-`U1c`–`U1e` unfiltered verification.
* **Wave-1 convergence prerequisite repair for `96F1DFFD` (`088-DL`, `U1f` / `211.012-T`, folded in `2026-11-06` per P-021 C4/C6 after the `M11` full-suite/short-`TEMP` reproduction and Ship's controlled public-API Core proof `bin/_c4e7a91b2/evidence2.json`)**: Coordinating live `executeReconstitution` (`internal/core/shipment_reconstitute_exec.go`) preliminary `planReconstitution` workspace `.md` reads under the existing canonical `lockActiveStash -> lockShipmentLifecycleGlobal -> lockShipmentMembership` lock order and lifetime while preserving pure option/YAML/source-ID fail-fast validation (`loadAndValidateReconstitutePlan` in `internal/core/shipment_reconstitute_plan.go`), post-`C`/`B` plan re-validation, `MoveFileExW` durability (`MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH`), and zero-write `DryRun: true` preview semantics (`R12`, `R17`).

### Out of Scope

* **OS/container-enforced process, filesystem, and network sandboxing (`Level 2`) and OS-level adversarial escape validation in this initial release**: Per the operator's `2026-11-04T22:02:03Z` `Q3` decision (`'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), kernel/OS/container sandboxing (Windows Job/AppContainer/restricted tokens, Linux namespaces/Landlock/seccomp, container wrappers) and OS-sandbox adversarial escape probes are deferred out of this initial release (`R18`, `R20`, `R23`, `SC14`).
* **Universal mutation scheduling across unmanaged external processes (`R7`)**: The Go channel coordinator serializes `backlogit` runtime-managed MCP mutations only; building a universal cross-process write broker for external editors or unrelated MCP servers is out of scope.
* **Persistent async job-ticket APIs or arbitrary reverse RPC (`R10`, `A3`)**: Replacing synchronous `tools/call` responses with detached job-ticket polling or arbitrary server-to-client RPC callbacks is not authorized for this release.
* **Unapproved selection of a specific third-party package manager during intake**: Exact workspace tool-manifest/lockfile schema and launcher adapter mechanics are deferred to implementation planning (`D9`) within the approved workspace-local installation, configuration, and version-pinning scope (`R23`).
* **Claiming direct control over the current agent host's built-in tool surface or claiming local placement equals OS sandboxing**: `backlogit` cannot unilaterally rewrite or sandbox built-in editor/agent host tools or unrelated external tools; non-local/built-in/remote tools require honest explicit reporting (`R18`, `R20`, `R23`, `SC13`).
* **Building new remote/cloud integrations or authentication providers in this initiative**: Remote API authorization policy is not banned or settled by `Q3` (`R21`), and implementing Azure DevOps work-item synchronization (stash `D895887C`) or new cloud auth/network providers is out of scope.
* **Hostile-agent security boundary over workspace-writable config without external policy authority**: Protecting against a hostile local process that already has OS user permissions to rewrite workspace files or `.mcp.json` requires an external policy authority or OS isolation boundary (`R18`, `A7`, `D7`), which is deferred out of this initial release.
* **Cloud sync or remote/unauthenticated network MCP hosting for `backlogit`**: Exposing the workspace `backlogit` MCP server over non-loopback TCP/HTTP is out of scope.
* **Portable binary installation inside `.backlogit/` (Stash `17214531`)**: Changing `backlogit install` to copy `backlogit.exe` into `.backlogit/` and mutate `.gitignore` or `.git/info/exclude` remains a separate feature track (`17214531`). This specification preserves `bin/backlogit.exe` for `.mcp.json` and `start.ps1` and requires only that daemon sidecar/IPC identity checks do not hardcode a single install directory.
* **Unrelated host-toolchain lint captures (`C6E224B8`, `46BD6AEE`, `259A4F71`) and any later unrelated full-suite failures**: Host Go 1.27 / `golangci-lint` v2 diagnostic captures (`C6E224B8`, `46BD6AEE`, `259A4F71`) are not blockers under the supported Go 1.24.0 + `golangci-lint` v1.64.8 CI toolchain and remain out of scope. Any later unrelated full-suite failure after `U1b` (`211.008-T`) must follow P-021 C1/C2 Stage intake rather than widening `U1b`.

## Key Decisions

1. **Distinguish `204-F` (Single-Process Lock Guard) from Multi-Session Runtime (`ECF33C79`)**: `204-F` / `205-S` (`077-DL`) is recognized as the foundational OS-lock and process-identity layer (`mcp-daemon.lock` + `mcp-daemon.json`), not a multi-session runtime. `ECF33C79` builds on `204-F`'s pinned `runtimeRoot` handle discipline rather than replacing it.
2. **Confirmed Independent Per-Workspace Background Daemon (`Option A`, `2026-11-04T20:15:01Z`)**: Per explicit operator confirmation (`Q1`), the `backlogit` runtime uses an independent per-workspace background daemon with thin `stdio`-to-local-IPC session shims so individual client exits never terminate peer sessions (`R13`).
3. **Dual Entry Routes with Unified Singleton Admission (`2026-11-04T20:15:01Z`)**: Both optional/idempotent `start.ps1` prewarm and lazy first-instantiation `backlogit mcp` startup are supported using the same singleton lock/sidecar admission and workspace-pinned `bin/backlogit.exe` binary (`R14`, `R15`).
4. **Confirmed All-Tools Workspace Breadth (`Q2 = ALL`, `2026-11-04T21:41:00Z`) Superseding Narrow Assumption `A6`**: Per operator confirmation (`'I mean ALL agent/MCP tools used in a workspace.'`), the workspace tool-environment scope encompasses **all** agent/MCP tools used in a workspace (`R19`–`R23`, `A8`), superseding the earlier narrower `A6` assumption (`backlogit`-managed tools only). `backlogit`'s multi-session runtime (`R1`–`R17`) is one participant/component within this workspace tool environment, not the entire outcome and not an automatic broker for unrelated host/MCP tools.
5. **Confirmed Initial-Release Containment Strength (`Q3` = Workspace-Local Installation, Configuration, and Version Pinning, `2026-11-04T22:02:03Z`)**: Per operator confirmation (`'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), initial-release containment across all workspace agent/MCP tools is scoped to workspace-local installation, configuration, and version pinning with explicit inventory, startup/provenance/compatibility verification, zero silent global `PATH` or cross-workspace fallback, explicit host bootstrap prerequisite declarations, and honest unsupported/built-in/remote tool reporting (`R18`–`R23`, `SC10`–`SC14`). OS/container sandboxing and OS-level adversarial escape validation are deferred out of this initial release; `backlogit` retains its own `*os.Root` path containment (`R17`) without claiming external tools obey it; and authorized remote service access remains a separate policy question (`R21`).
6. **Use `mcp-go` Per-Connection `ClientSession` Dispatch Rather Than Single-Session `StdioServer.Listen` (Proposed SDK Mechanism)**: Because `mcpserver.StdioServer.Listen` in `mcp-go v0.27.0` hardcodes a single static `&stdioSessionInstance`, the multi-session `backlogit` runtime will register a dedicated `ClientSession` per client stream (`RegisterSession` / `UnregisterSession` / `WithContext`) and dispatch framed JSON-RPC messages per session.
7. **Reader/Writer Split in the `backlogit` Go Channel Coordinator (`R7`, `R8`, `R12`)**: `backlogit` runtime-managed mutations are serialized sequentially through a bounded Go channel queue (`R7`) while read-only tools execute concurrently (`R8`) and one-shot CLI / external writer coexistence continues to use cross-process file locks and SQLite WAL/`busy_timeout` (`R12`) rather than pretending the Go channel schedules unmanaged external processes.
8. **Preserve `bin/backlogit.exe` in `.mcp.json` and `start.ps1` Without Preempting `17214531`**: Both `.mcp.json` and `start.ps1` continue to point to `bin/backlogit.exe` in this workspace; portable `.backlogit/` binary installation (`17214531`) remains a separate backlog outcome.
9. **Fold `C2CFE06E` into `211-F` / `212-S` as Bounded Wave-1 Prerequisite `U1b` (`211.008-T`, `085-DL`, `2026-11-06`)**: Originally excluded unless explicitly folded in by the operator during planning (`082-DL`). Following the Wave 1 unfiltered full-suite blocker on `212-S` (`docs/memory/2026-11-06/212-S-wave1-convergence-blocked.md`) and the operator's proactive resolution directive (`P-021 C4 & C6`), Stage deliberated on `C2CFE06E` in `085-DL` and folded the bounded 2-file prerequisite repair (`U1b` / `211.008-T`) into `211-F` / `212-S` (`R3`), while keeping earlier host-toolchain lint captures (`C6E224B8`, `46BD6AEE`, `259A4F71`) and any later unrelated full-suite failures out of scope.
10. **Fold `60D799D9` (`086-DL`, `U1c` / `211.009-T` & `U1d` / `211.010-T`) and `19D55778` (`087-DL`, `U1e` / `211.011-T`) into `211-F` / `212-S` as Bounded Wave-1 Convergence Prerequisites, and Record Initial No-Code Diagnostic Disposition for `96F1DFFD` (`2026-11-06`)**: After `U1b` (`211.008-T`, commit `834972a`) cleared all `./internal/cli` tests, the post-`U1b` Wave 1 Step 4.6 unfiltered full suite under workspace-contained `TEMP` surfaced `60D799D9`, `19D55778`, and `96F1DFFD` (`docs/memory/2026-11-06/212-S-U1b-green-unrelated-convergence-blockers.md`). With dark execution paused at Wave 1 convergence, Stage deliberated in `086-DL` and `087-DL` under the operator's proactive-remediation mandate (`P-021 C4 & C6`), authorizing three separate Wave-1 prerequisite units (`U1c` / `211.009-T`, `U1d` / `211.010-T`, `U1e` / `211.011-T`, amending `212-S` from `M8` to `M11` without touching completed `U1`/`U1b`) and recording `96F1DFFD` as a provisional no-code diagnostic disposition pending the post-`U1c`–`U1e` full-suite run.
11. **Supersede `96F1DFFD` Provisional No-Code Disposition With Bounded Wave-1 Prerequisite `U1f` (`211.012-T`, `088-DL`, `M11 -> M12`, `2026-11-06`)**: After `U1c`–`U1e` completed (`HEAD` `2c87ae6f`), the `M11` 50-package full suite (`3,871` passes, `9` failures, `0` timeouts) and short-`TEMP` (`bin/_qf4781986`, 41 chars) comparison cleared `7` of `8` selected Git path-length failures but reproduced `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation` (`internal/core/shipment_reconstitute_test.go:499`, `Access is denied` / `ErrWriteNotApplied`). Under the operator's `2026-11-06T16:57:46.238Z` `"Fourth iteration approved."` authorization (authorizing 1 additional current-wave convergence cycle without resetting the 3 prior failures), Ship's public-API Core matrix (`bin/_c4e7a91b2/evidence2.json`, `PID 61948`) and official Microsoft `FILE_RENAME_INFORMATION` / `MoveFileExW` documentation proved that legacy `MoveFileExW` (`MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH`) denies replacement with `syscall.Errno(5)` whenever any target handle is held open (both share mode `3` and share mode `7`), and succeeds when the reader handle is closed before `AssociateCommit`. Stage deliberated in `088-DL` (`96F1DFFD`) and authorized bounded 3-file Wave-1 prerequisite `U1f` (`211.012-T`, amending `212-S` from `M11` to `M12`) to move live `executeReconstitution`'s preliminary `planReconstitution` artifact reads under `lockActiveStash -> lockShipmentLifecycleGlobal -> lockShipmentMembership` while preserving pure input fail-fast validation, post-`C`/`B` re-validation, `MoveFileExW` durability, and lock-free `DryRun: true` preview semantics (documenting that uncoordinated external readers remain outside legacy `MoveFileExW` open-handle replacement guarantees and that historical failure-time kernel-handle attribution was not captured).

## Assumptions

* **A1 ([CONFIRMED 2026-11-04T20:15:01Z] Session Topology — Independent Per-Workspace Daemon + Thin Stdio Shim + Dual Entry Routes)**: Confirmed by operator (`2026-11-04T20:15:01Z`) and codified in `R13`–`R15`. `backlogit mcp` runs as a lightweight `stdio` session shim that connects over a workspace-bound local IPC endpoint to an independent per-workspace background daemon holding `mcp-daemon.lock`, supporting both optional/idempotent `start.ps1` prewarm and lazy initialization on first `backlogit mcp` instantiation for the workspace to which `bin/backlogit.exe` is pinned.
* **A2 (Concurrent Read Safety via SQLite WAL Pool and `backlogit`-Managed Write Gate — Proposed Architecture Baseline)**: Read-only `backlogit` MCP handlers using the workspace's WAL-mode SQLite connection pool and read-locked manifest state can safely execute concurrently across sessions while `backlogit` runtime-managed mutations are serialized by the internal Go channel coordinator (`R7`, `R8`) and cross-process CLI coexistence uses `.backlogit/.locks/` and `busy_timeout` (`R12`). Exact internal lock/barrier wiring is deferred to planning (`D1`).
* **A3 (Conservative First-Release Async Callback Scope — Independent Architectural Proposal / Default Assumption)**: Keeping `tools/call` synchronous while emitting standard MCP notifications (`notifications/progress` / `notifications/message`) on the session's `NotificationChannel()` where supported (`R10`, `D8`) remains an independent conservative architectural proposal. No persistent async job-ticket API or arbitrary reverse RPC is authorized, and this proposal is kept separate from confirmed `Q1`, `Q2`, and `Q3`.
* **A4 (Same-User Local IPC Access Control and Platform Namespace Distinction)**: Local IPC endpoints are restricted to the current OS user (`0600` on POSIX Unix Domain Sockets under workspace runtime storage; current-user SID `DACL` on Windows Named Pipes in the `\\.\pipe\...` kernel namespace) and paired with pre-attach workspace/version verification (`R15`, `R16`). Exact endpoint naming is deferred to planning (`D2`).
* **A5 (CLI Surface and Reference Sync Deferred to Planning)**: Exact CLI subcommand/flag spellings for daemon prewarm/hosting and any accompanying `docs/cli-reference/` regeneration are deferred to `impl-plan` (`D4`, `D6`).
* **A6 ([SUPERSEDED 2026-11-04T21:41:00Z by Operator Breadth Clarification Q2] Historical Narrow Default Breadth of Workspace Tool Containment)**: Initially recorded at `2026-11-04T20:15:01Z` as a tentative default limiting workspace tool containment to `backlogit`-managed MCP tools, resources, state, and helpers (`R15`–`R17`). **Explicitly superseded at `2026-11-04T21:41:00Z`** when the operator answered `'I mean ALL agent/MCP tools used in a workspace.'` (`Q2`). Retained here solely to preserve decision history; see `A8` and `R19`–`R23` for active scope.
* **A7 (Workspace-Writable Config Trust Boundary — Deferred Security Assumption)**: Workspace-writable configuration files are not treated as a hostile-agent security boundary without an independent out-of-workspace policy authority (`R18`); per `Q3` (`2026-11-04T22:02:03Z`), OS/container isolation and external policy-authority enforcement are deferred out of this initial release (`R23`, `D7`).
* **A8 ([CONFIRMED BREADTH 2026-11-04T21:41:00Z AND CONFIRMED LEVEL 1 CONTAINMENT STRENGTH 2026-11-04T22:02:03Z] Workspace-Owned All-Tools Environment Scope)**: Per operator confirmations on breadth (`2026-11-04T21:41:00Z`, `Q2 = ALL`) and containment strength (`2026-11-04T22:02:03Z`, `Q3`: `'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), workspace tool governance covers **all** agent/MCP tools used in a workspace (`backlogit`, companion MCP servers, CLI helpers, and `.mcp.json`/`start.ps1`/`start.sh`/adapter configs) at **Level 1 containment strength** (`R19`–`R23`, `SC10`–`SC14`)—enforcing workspace-local installation, configuration, and version pinning, explicit tool inventory, startup provenance/compatibility checks, zero silent global `PATH` or cross-workspace fallback, explicit bootstrap prerequisite declarations, `backlogit`'s own path containment (`R17`), and honest unsupported/remote/built-in tool reporting, while deferring OS/container sandboxing (`Level 2`) out of this initial release.

## Outstanding Questions

### Resolved Before Planning (Operator Confirmation Log)

* **Q1 (Independent Per-Workspace Daemon Lifecycle Topology — CONFIRMED `2026-11-04T20:15:01Z`)**:
  * **Question asked by Orchestrator**: `'Should that independent background daemon be the lifecycle model?'` (Recommending that the daemon survives individual clients disconnecting.)
  * **Verbatim operator confirmation (`2026-11-04T20:15:01Z`)**: `'Yes, I think makes sense, but it also means that it either needs an initial startup via the start.ps1 or that it initializes with the first instantiation for the workspace to which this local binary is pinned. It also suggests, perhaps, that available tools should be contained within the so-called sandbox of the workspace itself.'`
  * **Outcome**: Independent per-workspace daemon lifecycle (`Option A`) is confirmed (`R13`), along with supporting both optional/idempotent `start.ps1` prewarm and lazy first-instantiation startup pinned to the workspace binary (`R14`–`R16`).
* **Q2 (Workspace Tool Containment Breadth — CONFIRMED AS `ALL` `2026-11-04T21:41:00Z`)**:
  * **Question asked**: `'Should available tools mean backlogit-managed tooling only, or all agent/MCP tools used in the workspace?'`
  * **Verbatim operator confirmation (`2026-11-04T21:41:00Z`)**: `'I mean ALL agent/MCP tools used in a workspace.'`
  * **Outcome**: Tool breadth is **resolved as `ALL`**, superseding the earlier narrower assumption `A6` and establishing `R19`–`R22`, `SC10`–`SC14`, and `A8`.
* **Q3 (Containment Strength Across All Workspace Agent/MCP Tools — CONFIRMED AS WORKSPACE-LOCAL INSTALLATION, CONFIGURATION, AND VERSION PINNING `2026-11-04T22:02:03Z`)**:
  * **Question asked**: Whether containment strength across all workspace agent/MCP tools requires workspace-local installation, configuration, and version pinning (`Level 1`) vs OS/container-enforced process, filesystem, and network sandboxing (`Level 2`).
  * **Verbatim operator confirmation (`2026-11-04T22:02:03Z`)**: `'For now, workspace-local installation, configuration, and version pinning is sufficient.'`
  * **Outcome**: Initial-release containment strength is **resolved as Level 1** (`R23`, `A8`, updated `R18`, `R20`, `R22`, `SC14`): all workspace agent/MCP tools and helpers use workspace-local installation, configuration, and version pinning with explicit inventory, startup/provenance/compatibility contracts, zero silent global `PATH` or cross-workspace executable fallback, explicit host bootstrap prerequisite declarations, `backlogit`'s own path containment (`R17`), and honest unsupported/built-in/remote tool reporting (`SC10`–`SC14`). OS/container-enforced process, filesystem, and network sandboxing (`Level 2`) and OS-level adversarial escape validation are deferred out of this initial release.

### Resolve Before Planning

* None. All blocking product-scope questions (`Q1`, `Q2`, `Q3`) have been resolved by the operator; remaining engineering and protocol-detail questions belong to `impl-plan` (`D1`–`D9` below).

### Deferred to Planning

* **D1 (Channel Admission Queue Capacity, Wait Timeout, Internal Coordinator Wiring, and External Writer Boundary)**: Select exact defaults for the `backlogit` mutation channel buffer size (e.g., `32` vs `64`), default admission wait timeout before returning a structured busy error, internal Go coordinator struct layout, and interaction order with `.backlogit/.locks/` and SQLite `BEGIN IMMEDIATE` for external CLI/editor coexistence (`R7`, `R12`).
* **D2 (Platform IPC Endpoint Naming and UDS Path Length Fallback)**: Finalize deterministic Windows Named Pipe naming (`\\.\pipe\...` bound to canonical workspace identity) and POSIX Unix Domain Socket path placement / `sun_path` length fallback (104/108 bytes).
* **D3 (Idle Auto-Shutdown Grace Period and Prewarm Idle Window)**: Choose the default idle timeout after the last session disconnects (or after an unattached `start.ps1` prewarm) before the background daemon releases `mcp-daemon.lock` and exits.
* **D4 (Binary Version-Skew and Wrong-Workspace Refusal Recovery UX)**: Define exact diagnostic error codes, CLI flags, and recovery guidance when a client or prewarm detects an incompatible `version`/build or wrong-workspace daemon (e.g., safe auto-restart only when `active_sessions == 0` vs explicit `--replace` requirement when sessions are active).
* **D5 (Non-Exclusive Status and Handshake Probing)**: Coordinate the `mcp-daemon.json` + IPC ping readiness check so `backlogit mcp status` and new session connects do not take an exclusive probe lock on `mcp-daemon.lock` (`A045DBA4`).
* **D6 (`start.ps1` Prewarm Command Contract)**: Define the exact idempotent CLI invocation used by `start.ps1` to prewarm/verify the workspace daemon alongside `backlogit sync` and update CLI reference docs accordingly (`A5`).
* **D7 (External Policy Authority for Future Hostile-Agent / OS-Sandbox Phase)**: Record the boundary note for any future phase that introduces an out-of-workspace policy authority or OS/container sandbox wrapper beyond the initial local-pinning release (`R18`, `R23`, `A7`).
* **D8 (Client and SDK Progress-Notification Verification)**: Verify exact `github.com/mark3labs/mcp-go v0.27.0` and MCP host behavior for `notifications/progress` and `notifications/message` emission during queued/long-running gated operations (`R10`, `A3`).
* **D9 (Workspace Tool Inventory Manifest Schema, Version-Pinning/Package Mechanics, Bootstrap Prerequisite Catalog, and Host Adapter Design)**: Define the concrete workspace tool-environment manifest/configuration schema, local installation/version-pinning verification mechanics, supported host adapter checks, and explicit bootstrap prerequisite/exception declarations for Track A and Track C under the approved `Q3` Level 1 scope (`R19`–`R23`, keeping portable `.backlogit/` installation in stash `17214531` separate).

## Handoff

* **Current `handoff_status`**: `ready_for_plan` (All three product-scope clarifications are resolved and recorded with UTC provenance: `Q1` lifecycle and dual startup routes at `2026-11-04T20:15:01Z` in `R13`–`R16`/`A1`; `Q2` all-tools workspace breadth at `2026-11-04T21:41:00Z` in `R19`–`R22`/`A8` superseding `A6`; and `Q3` initial-release containment strength as workspace-local installation, configuration, and version pinning at `2026-11-04T22:02:03Z` in `R23`/`A8`/`SC14`).
* **Resolved Handoff Path**: `both` — durable requirements specification linked to existing deliberation item `082-DL` (`source_stash_id: ECF33C79`, originated at `.backlogit/queue/082-DL.md` and relocated to `.backlogit/archive/082-DL.md` with `status: done` [not `archived`] by `backlogit_move_item` per `.backlogit/registry.yaml`; `decision_status: decided`), with `impl-plan` established as the next pipeline milestone (readiness event only; `impl-plan` has NOT been executed and no new feature/task/shipment items have been created).
* **Document Self-Review Status**: Completed (`2026-11-04T22:02:03Z`) — verified all five `brainstorm` handoff contract gates:
  1. `Resolve Before Planning` is empty (`unresolved_blocking_questions: []`); engineering parameters `D1`–`D9` are cleanly separated under `Deferred to Planning`.
  2. Every requirement has a stable ID (`R1`–`R23`) and every success criterion has a stable ID (`SC1`–`SC14`) with zero renumbering.
  3. Scope boundaries (`In Scope` and `Out of Scope`), `R7` (`backlogit`-managed mutation gating vs external writers), `R10`/`A3` (synchronous `tools/call` + progress notifications unbundled; no async job-ticket API authorized), `R18`/`R20`/`R22`/`R23` (workspace-local installation/configuration/version pinning approved for initial release; OS/container sandboxing deferred; `backlogit` path containment preserved without overclaiming external tool confinement; honest unsupported/prerequisite/remote reporting required; remote API policy kept separate in `R21`), and `SC14` (local identity/pinning negative tests retained; OS adversarial escape validation reclassified to future phase) are explicit and internally consistent.
  4. Direct document review pass completed across this specification, `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md`, and `.backlogit/queue/082-DL.md`.
  5. `handoff_status` is set to `ready_for_plan`.
* **Dark Factory Status**: `dark_factory_ready: false` (dark mode was not activated by the operator; no implementation, branch/worktree creation, or shipment claim is authorized).
* **Readiness Event (`BRAINSTORM_HANDOFF_READY`)**:

```yaml
event: BRAINSTORM_HANDOFF_READY
timestamp: "2026-11-04T22:02:03Z"
artifact_path: "docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md"
deliberation_artifact_path: "docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md"
handoff_status: "ready_for_plan"
resolved_handoff_path: "both"
dark_factory_ready: false
unresolved_blocking_questions: []
resolved_questions:
  - id: "Q1"
    resolved_at: "2026-11-04T20:15:01Z"
    summary: "Independent per-workspace background daemon lifecycle (Option A) with idempotent start.ps1 prewarm and lazy first-instantiation startup pinned to bin/backlogit.exe (R13-R16, A1)."
  - id: "Q2"
    resolved_at: "2026-11-04T21:41:00Z"
    summary: "Workspace tool containment breadth covers ALL agent/MCP tools used in a workspace, superseding narrow assumption A6 (R19-R22, SC10-SC14, A8)."
  - id: "Q3"
    resolved_at: "2026-11-04T22:02:03Z"
    verbatim_operator_statement: "For now, workspace-local installation, configuration, and version pinning is sufficient."
    summary: "Initial-release containment strength is Level 1 (workspace-local installation, configuration, and version pinning across all workspace tools with explicit inventory, startup/provenance/compatibility checks, no silent global PATH/cross-workspace fallback, explicit bootstrap prerequisites, backlogit path containment R17, and honest unsupported/remote reporting; OS/container sandboxing and OS-level adversarial escape validation deferred out of initial release, R23, SC14)."
requirement_ids:
  - R1
  - R2
  - R3
  - R4
  - R5
  - R6
  - R7
  - R8
  - R9
  - R10
  - R11
  - R12
  - R13
  - R14
  - R15
  - R16
  - R17
  - R18
  - R19
  - R20
  - R21
  - R22
  - R23
success_criteria_ids:
  - SC1
  - SC2
  - SC3
  - SC4
  - SC5
  - SC6
  - SC7
  - SC8
  - SC9
  - SC10
  - SC11
  - SC12
  - SC13
  - SC14
deferred_to_planning:
  - "D1: Channel admission queue capacity, wait timeout, internal coordinator wiring, and external writer boundary"
  - "D2: Platform IPC endpoint naming and UDS path length fallback"
  - "D3: Idle auto-shutdown grace period and prewarm idle window"
  - "D4: Binary version-skew and wrong-workspace refusal recovery UX"
  - "D5: Non-exclusive status and handshake probing (A045DBA4)"
  - "D6: start.ps1 prewarm CLI command contract"
  - "D7: External policy authority boundary note for future hostile-agent / OS-sandbox phase"
  - "D8: Client and SDK progress-notification verification (mcp-go v0.27.0)"
  - "D9: Workspace tool inventory manifest schema, version-pinning/package mechanics, bootstrap prerequisite catalog, and host adapter design"
target:
  queue_item_id: "082-DL"
  queue_item_path: ".backlogit/queue/082-DL.md"
  resolved_item_path: ".backlogit/archive/082-DL.md"
  item_status: "done"
  source_stash_id: "ECF33C79"
  next_milestone: "impl-plan"
```

* **Next Milestone**: Invoke `impl-plan` using `docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md` and `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` (`082-DL`) as the approved source documents when the operator or Orchestrator authorizes the planning phase.
