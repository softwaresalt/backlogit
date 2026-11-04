---
chunk_strategy: h1-h2-h3
description: 'Architecture deliberation for stash ECF33C79 (082-DL): compare multi-session MCP runtime topologies, session adapters, Go channel operation gating, and workspace-local tool installation/configuration/version pinning across all workspace tools on top of the 204-F/205-S singleton lock.'
doc_type: decision
docline:
  decision_id: '082-DL'
  decision_status: decided
  depth: deep
  linked_artifacts:
    - docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md
    - docs/exec-plans/2026-11-04-multi-session-mcp-runtime-plan.md
    - .backlogit/archive/082-DL.md
    - .backlogit/queue/211-F.md
    - .backlogit/queue/212-F.md
    - .backlogit/queue/213-F.md
    - .backlogit/queue/212-S.md
    - .backlogit/queue/213-S.md
    - .backlogit/queue/214-S.md
    - docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md
  promoted_to: both
  source_stash_id: ECF33C79
  tags:
    - mcp
    - daemon
    - multi-session
    - channels
    - concurrency
    - stdio-shim
    - ECF33C79
    - 082-DL
  topic: 'Support multiple agents through one singleton MCP runtime with session isolation and channel-gated mutations (ECF33C79)'
schema_version: "1.0"
source: docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md
title: 'Deliberation — Multi-Session Singleton MCP Runtime, Channel Operation Gating, and Workspace-Pinned All-Tools Environment (ECF33C79 / 082-DL)'
---

## Problem Frame

Operator directive: prioritize **"Support multiple agents through one MCP runtime"** anchored to active stash entry `ECF33C79` (`priority: high`, `kind: feature`).

### Distinction Between Shipped `204-F` (`205-S` / `077-DL`) and `ECF33C79`

In `077-DL` (`docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md`), we evaluated how to prevent duplicate `backlogit mcp` processes from corrupting workspace state and deliberately selected a **CLI-only single-instance OS advisory lock** (`204-F`, shipped in `205-S`) as the minimal first step:

* **What `204-F` / `205-S` solved**: `AcquireMCPDaemonLock` (`internal/cli/mcp_daemon.go`) pins `<storage_root>/runtime` via `*os.Root`, acquires an exclusive kernel lock on `mcp-daemon.lock`, writes process-bound metadata to `mcp-daemon.json`, and provides `backlogit mcp status`, `backlogit mcp stop`, and `backlogit mcp --replace`. It prevents two independent `backlogit mcp` servers from opening `.backlogit/backlogit.db` and maintaining divergent in-memory `s.manifest` maps at the same time.
* **What `204-F` / `205-S` did NOT solve**: It is a single-process exclusion guard, not a multi-session runtime. When a second agent (e.g., Stage running alongside Orchestrator, Ship running alongside an IDE session, or parallel subagents) invokes `backlogit mcp`, startup fails immediately with `corerrors.ErrDaemonAlreadyRunning` unless `--replace` is passed—which kills the first agent's MCP server.

Stash `ECF33C79` now requests the second architectural evolution: hosting **multiple concurrent agent MCP sessions** inside a single workspace-owned runtime, using Go channels to gate internal operations sequentially where needed, investigating asynchronous callbacks if feasible, and preserving `bin/backlogit.exe` in `.mcp.json` and `start.ps1`.

### Operator Clarification and Provenance (2026-11-04T20:15:01Z)

Following initial intake, the Orchestrator asked the operator one specific lifecycle question:

> Should that independent background daemon be the lifecycle model?

The Orchestrator noted the recommendation that the daemon survives individual clients disconnecting. At `2026-11-04T20:15:01Z`, the operator answered verbatim:

> Yes, I think makes sense, but it also means that it either needs an initial startup via the start.ps1 or that it initializes with the first instantiation for the workspace to which this local binary is pinned. It also suggests, perhaps, that available tools should be contained within the so-called sandbox of the workspace itself.

**Deliberation Scope Impact**:

* **Confirmed (`2026-11-04T20:15:01Z`)**: Independent per-workspace background daemon lifecycle (`Option A` process topology over `Option B` primary foreground client hosting), plus **dual startup entry routes**—supporting both optional/idempotent prewarm via `start.ps1` and lazy initialization on the first `backlogit mcp` instantiation for the workspace to which `bin/backlogit.exe` is pinned (`R13`–`R16`).
* **Initial Tentative Containment Scope (`2026-11-04T20:15:01Z`, `R17`–`R18`, Historical `A6`)**: Initially recorded workspace-scoped application containment of `backlogit`-managed tools as a narrow default assumption (`A6`, now superseded below at `2026-11-04T21:41:00Z`) alongside the factual threat-model boundary between application containment and OS-enforced sandboxing (`R18`) and workspace-writable config trust boundaries (`A7`, `D7`).
* **Unbundled and Preserved as Independent Conservative Proposal (`R10`, `A3`)**: Because the Orchestrator's lifecycle question did not ask about persistent async job tickets, arbitrary reverse RPC, or specific internal Go struct/channel names, synchronous `tools/call` with standard MCP progress/log notifications (`A3`) remains an independent conservative architectural proposal.

### Second Operator Clarification and Provenance — All-Tools Workspace Breadth (2026-11-04T21:41:00Z)

In response to the follow-up scope question:

> Should available tools mean backlogit-managed tooling only, or all agent/MCP tools used in the workspace?

At `2026-11-04T21:41:00Z`, the operator answered verbatim:

> I mean ALL agent/MCP tools used in a workspace.

**Deliberation Scope Impact (`2026-11-04T21:41:00Z`)**:

* **Breadth Question (`Q2`) Resolved as `ALL`; Narrow Assumption `A6` Superseded (`R19`–`R22`, `A8`)**: The target outcome is a **workspace-owned agent/MCP tool environment** encompassing `backlogit`, other workspace MCP servers (such as `engram`, `graphtor-docs`, and any additional workspace MCP servers), agent-invoked helper/CLI binaries and scripts, and the launcher/adapter configurations (`.mcp.json`, `start.ps1`, `start.sh`, and host tool definitions) used to expose and run them (`R19`). Earlier narrow assumption `A6` (`backlogit`-managed tools only) is explicitly superseded (`A8`), and `Q2` MUST NOT be asked again. `backlogit`'s multi-session runtime (`R1`–`R17`) is one participant/consumer in this environment, not the entire outcome and not an automatic broker for unrelated MCP servers or built-in editor tools.
* **Location/Availability vs Runtime Data/Process/Network Confinement (`R18`, `R20`)**: Workspace-local executable pinning and manifest accounting prevent global `PATH` drift and cross-workspace binary resolution, but do **not** by themselves sandbox a process from reading/writing outside-workspace host paths, spawning processes, or accessing networks. Built-in host/editor tools and remote/hosted MCP services cannot be made local by `backlogit`; any unsupported, remote, or host-inherited tool must produce explicit policy reporting (`SC13`).
* **Remote Service Access Policy Kept Separate (`R21`)**: The `ALL` breadth answer does not automatically forbid every remote API or decide external service auth/network policy (e.g., Azure DevOps sync in stash `D895887C`), nor does it select a specific package manager, manifest schema, or installer directory (`D9`, keeping stash `17214531` separate).
* **Containment Strength Subsequently Resolved in `Q3` (`2026-11-04T22:02:03Z` below)**: While breadth (`Q2 = ALL`) was resolved at `2026-11-04T21:41:00Z`, **containment strength** (`Q3`) was isolated as the final product question and resolved by the operator at `2026-11-04T22:02:03Z`.

### Third Operator Clarification and Provenance — Containment Strength Resolved as Workspace-Local Installation, Configuration, and Version Pinning (2026-11-04T22:02:03Z)

In response to the final product-scope question (`Q3`) asking whether containment strength across all workspace agent/MCP tools requires workspace-local installation, configuration, and version pinning (`Level 1`) vs OS/container-enforced process, filesystem, and network sandboxing (`Level 2`), at `2026-11-04T22:02:03Z` the operator answered verbatim:

> For now, workspace-local installation, configuration, and version pinning is sufficient.

**Deliberation Scope Impact (`2026-11-04T22:02:03Z`)**:

* **`Q3` Resolved as Level 1 (`R23`, `A8`) — Deliberation Finalized (`decision_status: decided`)**: All workspace agent/MCP tools and helpers in scope (`Q2 = ALL`) use workspace-local installation, configuration, and version pinning with explicit tool inventory, startup/provenance/compatibility contracts, and zero silent global `PATH` or cross-workspace executable fallback (`R19`, `R23`, `SC10`–`SC12`). Any host bootstrap prerequisites or exceptions must be explicit rather than disguised as fully local tool compliance (`R19`, `R20`, `SC12`, `SC13`).
* **OS/Container Sandboxing and OS-Level Escape Validation Deferred Out of Initial Release (`R18`, `R20`, `R23`, `SC14`)**: OS/container-enforced process, filesystem, and network sandboxing (`Level 2`) and OS-level adversarial escape validation are deferred out of this initial release; local placement/pinning is never claimed to provide OS/container isolation.
* **`backlogit` Path Containment Retained as Component Behavior (`R17`, `R20`, `R23`)**: `backlogit`'s canonical workspace path containment checks (`*os.Root` and traversal/symlink/reparse refusal in `R17`) remain mandatory product behavior for `backlogit`, without claiming that external agent tools obey `backlogit`'s path rules or that `backlogit` confines unrelated host tools. Unsupported, built-in host, or remote tooling limitations must be reported honestly (`SC13`).
* **`backlogit` Channel Gate Scoped to `backlogit`-Managed Mutations (`R7`, `R12`) and Async Scope Kept Separate (`R10`, `A3`)**: The Go channel coordinator serializes `backlogit` runtime-managed MCP mutations while preserving cross-process file lock and SQLite WAL/`busy_timeout` duties (`R12`) and treating external writer coordination as a separate concern rather than an approved universal cross-tool broker. Synchronous `tools/call` + standard MCP progress/log notifications (`R10`, `A3`, `D8`) remain an unbundled conservative implementation proposal, with no persistent async job-ticket API or arbitrary reverse RPC authorized.

## Research Findings

### Codebase and SDK Evidence (`github.com/mark3labs/mcp-go v0.27.0`)

1. **`mcp-go v0.27.0` Session Architecture (`server.MCPServer` vs `server.StdioServer`)**:
   * `internal/mcp/server.go:133` (`RunStdio`) calls `mcpserver.ServeStdio(s.mcp)`, which constructs a `mcpserver.StdioServer` and calls `Listen(ctx, os.Stdin, os.Stdout)`.
   * Direct source inspection of `mcp-go v0.27.0` (`StdioServer.Listen`) shows that `StdioServer` registers a single package-global `stdioSessionInstance` (`s.server.RegisterSession(ctx, &stdioSessionInstance)`). Because `stdioSessionInstance.SessionID()` is static (`"stdio"`), calling `StdioServer.Listen` for a second stream on the same `*MCPServer` fails immediately with `ErrSessionExists`, and all notifications are read from the single `stdioSessionInstance.notifications` channel.
   * However, `mcpserver.MCPServer` itself stores sessions in a `sync.Map` keyed by `session.SessionID()`. Any transport loop that constructs a per-connection `ClientSession` implementation (with a unique UUID/counter `SessionID()` and its own `chan mcp.JSONRPCNotification`), calls `s.mcp.RegisterSession(ctx, sess)`, binds `sessCtx := s.mcp.WithContext(ctx, sess)`, and dispatches each newline-delimited JSON-RPC frame through `s.mcp.HandleMessage(sessCtx, rawLine)` gets full per-session isolation inside a single `*mcpserver.MCPServer`.
   * `mcpserver.StreamableHTTPServer` in `mcp-go v0.27.0` is an empty stub (`type StreamableHTTPServer struct{}` with a `TODO` comment), while `SSEServer` requires an HTTP client transport incompatible with `"type": "stdio"` MCP host configurations.
2. **In-Memory State and Synchronization in `internal/mcp/Server`**:
   * `Server` holds `workspaceMu sync.Mutex` (guarding lazy `ensureWorkspace` initialization and `s.Events` / `s.Telemetry` / `s.HookEvents` rebinding), `manifestMu sync.RWMutex` (guarding `s.manifest` and `s.manifestVersion` for `backlogit_merge_sync`), and `s.Workspace` (`*core.Workspace` with a 4-connection WAL-mode `*sql.DB` pool).
   * Per `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`, `s.Events` (`*events.EventWriter`) serializes JSONL event appends only when callers share that single instance.
   * Several mutating tool handlers in `internal/mcp/tools.go` perform multi-step reads and writes across Markdown files, SQLite index tables, `s.manifest`, and JSONL logs without a server-wide mutation queue, relying today on cross-process file locks (`.backlogit/.locks/`) or single-caller execution.
3. **In-Repo Multi-Client Stdio Precedent (`bin/engram.exe` and `.copilot/graphtor-mcp-shim.cjs`)**:
   * `.mcp.json` already demonstrates how stdio-only MCP hosts connect to shared workspace services: `engram` runs `engram shim` (which connects to or spawns `engram daemon` over local IPC and proxies JSON-RPC between `stdin`/`stdout` and the daemon).
   * `.mcp.json` currently configures `"backlogit"` with `"type": "stdio"`, `"command": "./bin/backlogit.exe"`, `"args": ["mcp"]`.
   * `start.ps1` currently configures `$sidecarExecutables.backlogit = (Join-Path $workspaceBinDirectory "backlogit.exe")` and verifies `bin\backlogit.exe` exists before startup.
4. **Related Active Stash Constraints (Context Only)**:
   * `17214531` (`high`, `feature`): Requests `backlogit.exe install` to place the executable inside `.backlogit/` and gitignore it for container/sandbox portability. That conflicts with `ECF33C79`'s explicit requirement to keep `bin/backlogit.exe` in `.mcp.json` and `start.ps1`. Keeping `17214531` as a separate initiative while avoiding hardcoded binary paths inside the daemon protocol preserves compatibility with both.
   * `A045DBA4` (`medium`, `bug`): Notes that `InspectMCPDaemonStatus` and `StopMCPDaemon` probe `mcp-daemon.lock` by attempting an exclusive lock acquisition, which can cause a concurrent `backlogit mcp` start to fail spuriously with `ErrDaemonAlreadyRunning`. Any multi-session connect/status handshake must avoid exclusive lock-probe contention.
   * `C2CFE06E` (`medium`, `bug`, deferred scope expansion): Tracks Windows Go 1.24 `mcp-daemon.lock` missing-path and helper-process EOF test failures from PR #638; kept separate per P-021 C6.

## Options Evaluated

### Option A: Auto-Spawned Singleton Workspace Daemon + Thin Stdio-to-IPC Session Shim + Channel-Gated Reader/Writer Coordinator Within a Workspace-Pinned All-Tools Environment (Confirmed 2026-11-04T20:15:01Z, 2026-11-04T21:41:00Z, & 2026-11-04T22:02:03Z; Internal Details Proposed)

* **Description**:
  * **Transport, Process Lifecycle, and Dual Entry Routes (`R13`, `R14` — Confirmed `2026-11-04T20:15:01Z`)**: `backlogit mcp` acts as a transparent `stdio` session adapter (shim) backed by an independent per-workspace background daemon holding `<storage_root>/runtime/mcp-daemon.lock` and advertising its endpoint in `mcp-daemon.json`. It supports **both** startup entry routes without requiring clients to launch via `start.ps1`:
    1. **Optional/idempotent prewarm via `start.ps1`**: `start.ps1` prewarms or verifies the workspace daemon using the workspace-pinned `bin/backlogit.exe` binary.
    2. **Lazy auto-start/connect on first `backlogit mcp` invocation**: When no daemon holds `mcp-daemon.lock`, the first `backlogit mcp` invocation spawns the detached background singleton using the workspace-pinned binary and connects its `stdio` session. Concurrent first clients and/or `start.ps1` prewarm converge deterministically to a single daemon instance without stale-sidecar false success or split brain.
  * **Workspace Resolution, Pre-Attach Verification, and IPC Namespace Boundary (`R15`, `R16`)**: Workspace identity is resolved from explicit/trusted workspace context and the local `bin/backlogit.exe` executable association, never from arbitrary ambient `cwd` alone or global `PATH` lookup. Before attaching, callers verify canonical workspace root/storage identity, pinned binary path, and compatible daemon version/build—explicitly refusing wrong-workspace or version-mismatched daemons without silent replacement or global `PATH` fallback. Daemon-managed state stays under configured workspace storage/runtime paths; POSIX Unix Domain Sockets reside in the workspace runtime filesystem path, whereas Windows Named Pipes reside in the Windows kernel pipe namespace (`\\.\pipe\...`) bound to the canonical workspace identity with current-user SID `DACL` controls (exact endpoint naming deferred to `D2`).
  * **Workspace-Owned All-Tools Environment, `backlogit` Component Containment, and Confirmed Level 1 Containment Strength (`R17`–`R23`, `A8`, Superseding `A6`)**:
    * **`backlogit` Component (`R17`)**: Available `backlogit`-managed tools, resources, and any managed helpers are workspace-scoped, confining `backlogit` file operations to the canonical workspace root (rejecting traversal and symlink/reparse escapes), binding startup handshake/tool discovery to the verified workspace, and forbidding implicit global `PATH` helper discovery.
    * **All-Tools Workspace Environment (`R19`–`R23`)**: Beyond `backlogit` as one participant, the workspace tool environment covers all workspace MCP servers (`engram`, `graphtor-docs`, etc.), CLI helpers, and launcher/adapter configurations (`.mcp.json`, `start.ps1`, `start.sh`). Per `Q3` (`2026-11-04T22:02:03Z`: `'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), initial-release containment enforces **Level 1**: workspace-local installation, configuration, and version pinning with explicit tool inventory/discovery, startup provenance/compatibility verification, zero silent global `PATH` or cross-workspace executable fallback, explicit host/bootstrap prerequisite declarations, and honest reporting of unsupported, host-inherited, or declared remote tools (`R20`, `R21`, `R23`, `SC10`–`SC14`).
    * **Containment Strength Boundary (`R18`, `R20`, `R23`, `SC14`)**: Distinguishes workspace-local tool installation, configuration, and version pinning (**Level 1**, approved for initial release) from OS/container-enforced process, filesystem, and network sandboxing (**Level 2**, deferred out of initial release), without claiming that local placement provides OS/container isolation or that `backlogit`'s path checks confine unrelated external tools.
  * **Session Isolation (Proposed SDK Mechanism)**: For each accepted IPC connection, the daemon creates a dedicated `ClientSession` (`sessionID = uuid`), registers it with `s.mcp.RegisterSession(ctx, sess)`, runs a per-session notification writer goroutine draining `sess.NotificationChannel()`, and dispatches incoming JSON-RPC lines via `s.mcp.HandleMessage(sessCtx, line)` with per-request child contexts so `notifications/cancelled` or client EOF cancels only that session's requests.
  * **Channel-Gated Internal Coordinator for `backlogit`-Managed Operations (`R7`–`R9`, `R12`, Deferred Parameters `D1`)**: `backlogit` MCP tool handlers are classified into **Mutating** and **Read-Only**:
    * **Mutating `backlogit` calls** submit an operation envelope to a bounded Go channel serviced by a single sequential coordinator loop (FIFO). If `ctx.Done()` fires before admission or the bounded queue is full, the caller immediately receives a structured busy/cancelled error without executing the mutation. This coordinator serializes `backlogit` runtime-managed MCP mutations only and is not a universal cross-process scheduler for external editors or other MCP servers (`R7`, `R12`).
    * **Read-only `backlogit` calls** acquire a token from a bounded read-concurrency limiter and execute concurrently against the WAL-mode SQLite pool and read-locked manifest, coordinating with full-rehydration barriers only when needed.
  * **Async Callbacks (Conservative First-Release Proposal `A3`, `R10`, `D8`)**: Standard `tools/call` remains synchronous per MCP specification, while the coordinator may emit standard MCP `notifications/progress` (when `_meta.progressToken` is supplied) or `notifications/message` to the calling session's `NotificationChannel` during long-running operations (`backlogit_ship_shipment`, `backlogit_sync_index`). Exact client/SDK support is verified during planning (`D8`); no persistent async job-ticket or reverse-RPC API is authorized.
* **Pros**:
  * Zero configuration changes required for MCP hosts: `.mcp.json` keeps `"type": "stdio", "command": "./bin/backlogit.exe", "args": ["mcp"]`.
  * **Decoupled session lifetimes**: Disconnecting Agent 1 (closing its `stdin`) exits only Agent 1's thin shim; the background singleton daemon and Agent 2's session continue uninterrupted.
  * Eliminates head-of-line read blocking while strictly serializing workspace writes via idiomatic Go channels.
  * Matches the proven `engram shim` + `engram daemon` architecture already operating in this repository.
* **Cons**:
  * Requires cross-platform local IPC listener (`net.Listen("unix", ...)` on POSIX and Windows Named Pipe via `golang.org/x/sys/windows` in `internal/cli` / `internal/mcp`), idle-timeout management when `active_sessions == 0`, and binary version-skew checks when `bin/backlogit.exe` is rebuilt.
* **Effort estimate**: Medium-High.

### Option B: First-Started Foreground `backlogit mcp` Process as Primary Host + Secondary Stdio-to-IPC Shims with Reference-Counted Drain

* **Description**:
  * Instead of spawning a separate background daemon process, the first `backlogit mcp` invocation that acquires `mcp-daemon.lock` becomes the primary runtime host: it serves its own `os.Stdin`/`os.Stdout` as Session 1 **and** opens the local IPC listener (`mcp-daemon.json`) for subsequent `backlogit mcp` invocations (which automatically fall back to thin stdio-to-IPC shims when `AcquireMCPDaemonLock` sees an active holder with an IPC endpoint).
  * To prevent Session 1's exit from killing Session 2, the primary process does **not** exit immediately when its own `os.Stdin` reaches EOF if secondary IPC sessions are still attached; instead, it closes Session 1 and stays alive until `active_sessions` drops to zero (or `backlogit mcp stop` / `--replace` is called).
* **Pros**:
  * No child-process detachment (`cmd.Start` background daemon) required on initial startup; single process tree when only one agent is active.
  * Preserves drop-in `"type": "stdio"` compatibility in `.mcp.json`.
* **Cons**:
  * **Parent process-group / job-object termination hazard**: Many MCP hosts (VS Code, Copilot CLI, Claude Code, test runners) place their spawned `stdio` child process into a Windows Job Object (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) or POSIX process group and send `SIGKILL` / `TerminateJobObject` when the first agent's session ends—which forcibly kills the primary process and severs all secondary agents' sessions even if reference-counted in Go.
* **Effort estimate**: Medium.

### Option C: Network-Only (Loopback HTTP / SSE) Singleton Server Without Stdio Session Shim

* **Description**:
  * Run `backlogit mcp` exclusively as a long-lived HTTP/SSE server (`mcpserver.NewSSEServer`) on `127.0.0.1:<port>` and require all agents to connect over HTTP/SSE.
* **Pros**:
  * Uses `mcp-go v0.27.0`'s built-in `SSEServer` without custom Windows Named Pipe / Unix Domain Socket IPC framing.
* **Cons**:
  * Breaks drop-in compatibility with stdio-only MCP client configurations and scripts that invoke `backlogit mcp` over `stdin`/`stdout`.
  * `StreamableHTTPServer` in `mcp-go v0.27.0` is an unimplemented stub, and fixed ports across multiple workspaces on the same machine collide unless dynamic port discovery is mediated by a stdio shim anyway.
  * Loopback TCP ports on multi-user or containerized hosts widen the local attack surface compared to filesystem/SID-scoped pipes/sockets unless paired with per-launch bearer tokens.
* **Effort estimate**: Medium (but fails stdio compatibility constraints).

### Option D: Single Unbuffered Channel Serializing Every MCP Request (Reads and Writes Alike)

* **Description**:
  * Funnel every `HandleMessage` call (including `tools/list`, `backlogit_get_item`, `backlogit_query_sql`, `backlogit_list_items`, and mutations) through a single sequential worker goroutine.
* **Pros**:
  * Trivial concurrency model inside `internal/mcp`.
* **Cons**:
  * Causes severe head-of-line blocking: a single slow mutation (`backlogit_ship_shipment` taking 30–120s) or index sync stalls every concurrent agent's read queries and `ping` health checks, causing cascading `-32001: Request timed out` failures across all sessions.
* **Effort estimate**: Low (rejected as concurrency anti-pattern).

## Trade-off Comparison

| Criterion | Option A (Auto-Spawned Daemon + Stdio Shim + R/W Channel Gate) | Option B (Primary Foreground Host + Secondary Shims) | Option C (Loopback HTTP/SSE Only) | Option D (Serialize All Reads + Writes) |
|---|---|---|---|---|
| Drop-in `.mcp.json` `stdio` compatibility | Yes | Yes | No (requires SSE client config or shim) | N/A (internal scheduler only) |
| Survives first agent host closing/killing its child process | High (independent daemon process) | Low/Moderate (vulnerable to host Job Object / `SIGKILL`) | High | N/A |
| Prevents head-of-line read starvation | Yes (concurrent reads, FIFO channel-gated writes) | Yes | Yes | No (reads block behind slow writes) |
| Alignment with `204-F` `mcp-daemon.lock` + `runtimeRoot` | High (daemon holds lock; shims read sidecar) | High | Partial | High |
| Alignment with in-repo `engram shim` pattern | Exact match | Partial | Low | N/A |

## Decision

* **Current Status**: `decided` (`082-DL` moved via `backlogit_move_item` `queued` -> `active` -> `done`, located at `.backlogit/archive/082-DL.md` from `.backlogit/queue/082-DL.md` per `.backlogit/registry.yaml` status-directory routing with `status: done` [not `archived`]; requirements spec `handoff_status: ready_for_plan` with `BRAINSTORM_HANDOFF_READY` recorded; `promoted_to: both` linking `082-DL` with `impl-plan` as the next milestone). Operator confirmations have locked in:
  1. Independent per-workspace background daemon lifecycle and dual startup routes (`Q1`, `2026-11-04T20:15:01Z`),
  2. All-tools workspace breadth (`Q2 = ALL`, `2026-11-04T21:41:00Z`, superseding narrow assumption `A6`), and
  3. Initial-release containment strength (`Q3`, `2026-11-04T22:02:03Z`: `'For now, workspace-local installation, configuration, and version pinning is sufficient.'`), deferring OS/container sandboxing (`Level 2`) out of this initial release.
* **Chosen Direction (`2026-11-04T20:15:01Z`, `2026-11-04T21:41:00Z`, & `2026-11-04T22:02:03Z`) and Bounded Proposals (`A1`–`A8`)**:
  1. **Confirmed Lifecycle & Startup Model — Option A Independent Per-Workspace Daemon with Dual Entry Routes (`R13`–`R16`, `A1`)**:
     * Keep `204-F`'s handle-pinned `<storage_root>/runtime/mcp-daemon.lock` and `mcp-daemon.json` as the singleton workspace authority.
     * Support **both** optional/idempotent prewarm via `start.ps1` and lazy initialization on the first `backlogit mcp` invocation for the workspace to which `bin/backlogit.exe` is pinned, converging concurrent startups to a single daemon without stale-sidecar false success or split brain.
     * Verify canonical workspace root/storage identity, workspace-pinned binary path, and compatible daemon version/build before attach; explicitly refuse wrong-workspace or version-mismatched daemons without silent active-daemon replacement or global `PATH` fallback.
     * Keep all daemon-managed filesystem state under configured workspace storage/runtime paths, while distinguishing POSIX Unix Domain Socket filesystem paths from the Windows Named Pipe OS namespace (`\\.\pipe\...` bound to workspace identity with current-user SID `DACL`).
  2. **Confirmed All-Tools Workspace Environment Breadth & Level 1 Containment Strength (`R17`–`R23`, `SC10`–`SC14`, `A8` Superseding `A6`)**:
     * Define the product outcome as a **workspace-owned agent/MCP tool environment** covering `backlogit`, other workspace MCP servers (`engram`, `graphtor-docs`, etc.), agent-invoked CLI helpers/scripts, and launcher/adapter configurations (`.mcp.json`, `start.ps1`, `start.sh`).
     * Treat the shared `backlogit` multi-session runtime (`R1`–`R17`) as **one participant/consumer** inside that environment—not the entire outcome and not a universal runtime broker, process supervisor, or mutation scheduler for independent MCP servers or built-in host tools.
     * Per `Q3` (`2026-11-04T22:02:03Z`), enforce **workspace-local installation, configuration, and version pinning** across all workspace tools (`R19`, `R23`): explicit tool inventory/discovery, workspace-pinned binary/config/version resolution with zero silent global `PATH` or cross-workspace fallback, explicit declarations for unavoidable host/bootstrap prerequisites, workspace identity and version compatibility verification, `backlogit`'s own canonical workspace path containment (`R17`), and honest explicit policy reporting for unsupported, built-in host, or remote tools (`R19`–`R23`, `SC10`–`SC14`).
     * Separate **workspace-local tool installation/pinning** from **runtime data/process/network confinement** (`R18`, `R20`, `R23`), explicitly defer OS/container-enforced process, filesystem, and network sandboxing (`Level 2`) and OS-level adversarial escape validation out of this initial release (`R23`, `SC14`), keep authorized remote service access as a separate policy question (`R21`), and structure delivery into **Track A** (workspace tool-environment contract, now settled at requirements level via `Q3`), **Track B** (`backlogit` multi-session daemon + Go channel coordinator for `backlogit`-managed mutations), and **Track C** (cross-tool launcher/host adapters, companion MCP alignment, honest unsupported/prerequisite reporting, and local identity/pinning negative tests, `R22`, `R23`, `SC10`–`SC14`).
  3. **Proposed Internal Session & Channel Architecture (`R4`–`R9`, `R12`, `A2`)**:
     * Register a distinct `mcpserver.ClientSession` per IPC connection on the shared `*mcpserver.MCPServer` and dispatch messages per session.
     * Implement a Go channel-based operation coordinator in `internal/mcp` that serializes all **`backlogit` runtime-managed** workspace-mutating tools FIFO with bounded admission and synchronous lock ownership, while allowing read-only tools to execute concurrently under a bounded read limiter and preserving cross-process file lock (`.backlogit/.locks/`) and SQLite WAL/`busy_timeout` coexistence (`R7`, `R12`). (Exact channel buffer sizes, timeouts, and struct names are deferred to planning `D1`.)
  4. **Independent Conservative Scope of Async Callbacks (`R10`, `A3`, `D8`)**:
     * Keep `tools/call` responses synchronous (preserving compatibility with all existing agent callers) and use each session's `ClientSession.NotificationChannel()` to emit standard MCP `notifications/progress` / `notifications/message` updates during long-running gated operations, verifying exact client/SDK support during planning (`D8`). No persistent async job-ticket API or arbitrary reverse RPC is authorized, and this proposal remains unbundled from `Q3`.
  5. **Executable Path Boundary (`R12`)**:
     * Preserve `bin/backlogit.exe` in `.mcp.json` and `start.ps1` as requested in `ECF33C79`, while keeping portable `.backlogit/` binary installation (`17214531`), Windows Go 1.24 lock-test EOF investigation (`C2CFE06E`), and Azure DevOps sync (`D895887C`) in their separate stash tracks.

### Capability and Dependency Structure (True Blockers vs Proposed Phased Delivery)

| Track | Scope | Dependency Classification |
|---|---|---|
| **Track A: Workspace Tool-Environment Contract (`R15`–`R21`, `R23`; Settled at Requirements Level via `Q3` `2026-11-04T22:02:03Z`)** | Cross-tool inventory/discovery contract, workspace-local installation/configuration/version-pinning rules, explicit bootstrap prerequisite/exception policy, workspace identity/version verification, and honest unsupported/remote tool reporting | **Settled at requirements level (`2026-11-04T22:02:03Z`)** as Level 1 local installation, configuration, and version pinning (`R23`). **True Blocker** for Track C cross-tool adapter compliance and verification |
| **Track B: `backlogit` Multi-Session Singleton Daemon & Channel Gate (`R1`–`R17`)** | Independent per-workspace `backlogit` daemon, `stdio`-to-IPC shims, `start.ps1` prewarm + lazy first-use startup, `ClientSession` isolation, FIFO Go channel mutation gate for `backlogit`-managed operations + concurrent reads, `bin/backlogit.exe` & `*os.Root` confinement | **Can be developed independently as a runtime slice** (does not wait on external Track C adapters to implement internal channel/IPC mechanics, and is not a universal broker for external writers per `R7`, but **broader outcome still requires Track A + Track C all-tools evidence**) |
| **Track C: Cross-Tool Adapters, Companion MCP Alignment, & Local Pinning Validation (`R19`–`R23`, `SC10`–`SC14`)** | `.mcp.json` / `start.ps1` / `start.sh` local pinning & verification, companion MCP servers (`engram`, `graphtor-docs`), host/launcher adapter & prerequisite checks, honest non-conformant/remote tool reporting, and multi-workspace/local-pinning negative test suites | **True Blocker Dependency on Track A (`R19`–`R23`)** and integrates **Track B** to deliver and verify the complete workspace-owned tool environment (OS/container sandboxing and OS-level adversarial escape validation deferred to a later phase per `R23`/`SC14`; no fake backlog `blocks` edges or shipment IDs pre-allocated at intake) |

## Rejected Alternatives

* **Option B (First-Started Foreground `backlogit mcp` Process as Primary Host + Secondary Shims)** is set aside following the operator's `2026-11-04T20:15:01Z` confirmation of the independent background daemon lifecycle (`Option A`), avoiding parent process-group / Windows Job Object termination hazards when the first client exits.
* **Option C (Network-Only HTTP/SSE Without Stdio Shim)** is rejected because stdio MCP hosts cannot connect without a stdio adapter and `StreamableHTTPServer` in `mcp-go v0.27.0` is an unimplemented stub.
* **Option D (Serializing All Reads and Writes on One Channel)** is rejected because long-running mutations (`backlogit_ship_shipment`, `backlogit_sync_index`) would block concurrent read-only queries across all connected agents and trigger `-32001` timeouts.
* **Narrow `backlogit`-Only Tool Containment Assumption (`A6`)** is superseded as of `2026-11-04T21:41:00Z` by the operator's explicit `Q2 = ALL` breadth confirmation (`'I mean ALL agent/MCP tools used in a workspace.'`).
* **Level 2 OS / Container-Enforced Process, Filesystem, and Network Sandboxing in Initial Release** is **deferred out of this initial release** as of `2026-11-04T22:02:03Z` per the operator's `Q3` decision (`'For now, workspace-local installation, configuration, and version pinning is sufficient.'`). Local binary placement and version pinning are not claimed to provide OS/container isolation (`R18`, `R20`, `R23`, `SC14`).

## Unresolved Questions

* **Resolved Question `Q1` (Confirmed `2026-11-04T20:15:01Z`)**:
  * **Question asked by Orchestrator**: `'Should that independent background daemon be the lifecycle model?'`
  * **Operator confirmation (`2026-11-04T20:15:01Z`)**: `'Yes, I think makes sense, but it also means that it either needs an initial startup via the start.ps1 or that it initializes with the first instantiation for the workspace to which this local binary is pinned. It also suggests, perhaps, that available tools should be contained within the so-called sandbox of the workspace itself.'`
  * **Resolution**: Independent per-workspace background daemon lifecycle (`Option A`) and dual startup entry routes (`start.ps1` prewarm + lazy first-instantiation auto-start pinned to `bin/backlogit.exe`) are confirmed (`R13`–`R16`).
* **Resolved Question `Q2` (Breadth Confirmed as `ALL` `2026-11-04T21:41:00Z`)**:
  * **Question asked**: `'Should available tools mean backlogit-managed tooling only, or all agent/MCP tools used in the workspace?'`
  * **Operator confirmation (`2026-11-04T21:41:00Z`)**: `'I mean ALL agent/MCP tools used in a workspace.'`
  * **Resolution**: Tool breadth is resolved as **ALL agent/MCP tools used in a workspace** (`R19`–`R22`, `SC10`–`SC14`, `A8`), superseding narrow assumption `A6`.
* **Resolved Question `Q3` (Containment Strength Confirmed as Workspace-Local Installation, Configuration, and Version Pinning `2026-11-04T22:02:03Z`)**:
  * **Question asked**: Whether containment strength across all workspace agent/MCP tools requires workspace-local installation, configuration, and version pinning (`Level 1`) vs OS/container-enforced process, filesystem, and network sandboxing (`Level 2`).
  * **Operator confirmation (`2026-11-04T22:02:03Z`)**: `'For now, workspace-local installation, configuration, and version pinning is sufficient.'`
  * **Resolution**: Initial-release containment strength is resolved as **Level 1 (workspace-local installation, configuration, and version pinning)** across all workspace agent/MCP tools (`R23`, `A8`, updated `R18`, `R20`, `R22`, `SC14`). OS/container sandboxing (`Level 2`) and OS-level adversarial escape validation are deferred out of this initial release.
* **Blocking Product Questions Before `impl-plan`**: `None.` All product-scope questions (`Q1`, `Q2`, `Q3`) are resolved.
* **Deferred to Planning (`D1`–`D9`)**:
  * `D1`: Exact channel buffer size (e.g., 32), default admission wait timeout, internal coordinator struct layout, and interaction order with `.backlogit/.locks/` and SQLite `BEGIN IMMEDIATE` for external CLI/editor coexistence (`R7`, `R12`).
  * `D2`: Deterministic Windows Named Pipe name derivation (`\\.\pipe\...`) and Unix Domain Socket path-length fallback.
  * `D3`: Default idle auto-shutdown timeout when `active_sessions == 0` and prewarm idle window after `start.ps1`.
  * `D4`: Version-skew and wrong-workspace refusal recovery UX/flags when active sessions are present vs idle.
  * `D5`: Non-exclusive status and connect readiness probing (`A045DBA4`).
  * `D6`: Exact idempotent `start.ps1` prewarm CLI invocation syntax and CLI reference doc regeneration (`A5`).
  * `D7`: External policy-authority boundary note for any future hostile-agent / OS-sandbox phase (`R18`, `R23`, `A7`).
  * `D8`: Verification of exact client/SDK (`mcp-go v0.27.0` and target MCP hosts) handling of `notifications/progress` and `notifications/message` (`R10`, `A3`).
  * `D9`: Concrete workspace tool-environment manifest/inventory schema, local installation/version-pinning verification mechanics, bootstrap prerequisite catalog, and launcher/host adapter contract under the approved `Q3` Level 1 scope (`R19`–`R23`).

## Risks and Mitigations

* **Risk**: A slow or hung mutation monopolizes the sequential mutation channel and causes subsequent mutating calls from other sessions to wait indefinitely.
  * **Mitigation**: Enforce bounded queue depth, per-request context cancellation while waiting in the admission channel, and synchronous frame lock/slot ownership (`timeout-goroutine-lock-ownership-go-2026-07-01.md`) so cancelled callers fail fast before mutation and never strand the coordinator.
* **Risk**: When the first agent session closes in an IDE or CLI host, the host terminates the entire child process tree or Windows Job Object, killing a shared in-process server.
  * **Mitigation**: Option A decouples the background workspace daemon process from any single stdio shim's process group / job object, and uses reference counting + an idle grace timer for clean shutdown.
* **Risk**: Concurrent `backlogit mcp` startups race to spawn the daemon when no daemon is running.
  * **Mitigation**: Reuse `204-F`'s atomic `AcquireMCPDaemonLock` on `<storage_root>/runtime/mcp-daemon.lock`: only the winner becomes/spawns the daemon and writes `mcp-daemon.json`; losing racers detect `ErrDaemonAlreadyRunning`, poll `readHolderInfo` with bounded backoff until the IPC endpoint is ready, and connect as client shims instead of failing.
