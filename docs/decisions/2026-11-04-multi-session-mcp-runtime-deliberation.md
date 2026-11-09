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
    - docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md
    - .backlogit/archive/082-DL.md
    - .backlogit/archive/085-DL.md
    - .backlogit/archive/086-DL.md
    - .backlogit/archive/087-DL.md
    - .backlogit/archive/088-DL.md
    - .backlogit/archive/089-DL.md
    - .backlogit/queue/211-F.md
    - .backlogit/archive/211.008-T.md
    - .backlogit/archive/211.009-T.md
    - .backlogit/archive/211.010-T.md
    - .backlogit/archive/211.011-T.md
    - .backlogit/archive/211.012-T.md
    - .backlogit/archive/211.013-T.md
    - .backlogit/archive/211.014-T.md
    - .backlogit/archive/211.015-T.md
    - .backlogit/archive/211.016-T.md
    - .backlogit/queue/211.017-T.md
    - .backlogit/queue/211.018-T.md
    - .backlogit/queue/211.019-T.md
    - .backlogit/queue/212-F.md
    - .backlogit/queue/213-F.md
    - .backlogit/queue/212-S.md
    - .backlogit/queue/213-S.md
    - .backlogit/queue/214-S.md
    - docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md
    - docs/memory/2026-11-08/212-S-longpaths-wave34-convergence.md
    - docs/memory/2026-11-06/212-S-wave1-convergence-blocked.md
    - docs/memory/2026-11-06/212-S-U1b-green-unrelated-convergence-blockers.md
    - docs/memory/2026-11-06/212-S-M11-fullsuite-circuit-handoff.md
    - docs/memory/2026-11-06/212-S-wave-convergence-circuit-checkpoint.md
    - docs/memory/2026-11-06/212-S-fourth-cycle-authorization-and-diagnosis.md
    - docs/compound/build-errors/match-ci-go-and-linter-before-source-repair-2026-11-06.md
    - docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md
    - docs/compound/2026-07-07-empty-head-fail-closed-repo-presence-probe.md
    - docs/compound/best-practices/git-aware-backlog-artifact-archival-preserves-follow-history-2026-07-10.md
    - docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md
    - docs/compound/2026-10-19-lock-held-side-effect-ordering.md
    - docs/compound/2026-08-29-windows-preremove-rename-pattern.md
    - docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md
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
    - C2CFE06E
    - 085-DL
    - 60D799D9
    - 19D55778
    - 96F1DFFD
    - 086-DL
    - 087-DL
    - 088-DL
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

## P-021 C6 Forward Deliberation Addendum — Folding `C2CFE06E` into `211-F` / `212-S` as Wave-1 Prerequisite `U1b` (`085-DL`, 2026-11-06)

### Problem Frame and Operator Forward-Planning Authorization

During Wave 1 convergence of active Shipment `212-S` (`211-F`) on branch `feat/212-s-backlogit-multi-session-singleton-mcp-runtime-stdio-session-shim-and-channel-operation-coordinator` at `HEAD` `bcc2809027077752449d8bf38792e2f9f93504a6`—after `U1` (`211.001-T`) completed its 3-file declaration-only contract (`e8fbefb71fa277d387348d45f12377cd7fda2f8d` harness, `bcc2809027077752449d8bf38792e2f9f93504a6` implementation) and was officially archived as `done`—the required unfiltered Go 1.24.0 test gate (`go test -timeout=30m ./...`) failed 16 existing daemon-related tests in `./internal/cli` (`TestU3_StopMCPDaemon_NotRunning`, `TestU2_AcquireLockAndContention`, `TestU3_StopMCPDaemon_TerminatesVerifiedHolder`, `TestU4_CrossProcessMCPDaemonLockAndStop`, and related subtests; see `docs/memory/2026-11-06/212-S-wave1-convergence-blocked.md`).

Verified root cause (`docs/compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md`):

* At `internal/cli/mcp_daemon.go:177`, `storageHandle.OpenRoot("runtime")` opens a nested pinned `*os.Root`. In Go 1.24 on Windows, `runtimeRoot.Name()` returns the relative string `"runtime"`, not the canonical absolute `runtimeDir` (`filepath.Join(storageRoot, "runtime")`) already verified at line 138.
* `openMCPDaemonLockFile` (lines 220, 222, 243), `readMCPDaemonInfo` (lines 270, 275), and `writeMCPDaemonSidecar` (line 314) construct `filepath.Join(runtimeRoot.Name(), ...)` (`"runtime\mcp-daemon.lock"` and `"runtime\mcp-daemon.json"`) and pass that relative path to `rejectLink` -> `core.IsSymlinkOrReparsePoint` -> `windows.GetFileAttributes` (`internal/core/workspace_reparse_windows.go:19`), which resolves against process CWD instead of `runtimeDir` and fails with `ERROR_PATH_NOT_FOUND` whenever process CWD lacks a `runtime\` directory.
* Existing deferred-scope-expansion stash entry `C2CFE06E` (`priority: medium`, `kind: bug`) tracks this exact defect surface.
* Under **P-021 C4 & C6**, the operator's explicit acceptance of the named Ship recovery plan followed by the directive (`"when I ask pipeline ... proactively identify issues and resolve/fix without stopping every time ... compoundlearn"`) activates the conditional inclusion clause already recorded in `docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md` (`## Scope Boundaries` -> `### Out of Scope`: *"unless explicitly folded in by the operator during planning"*). Stage therefore deliberated on `C2CFE06E` via `backlogit_deliberate` (`085-DL`, `status: done`, `.backlogit/archive/085-DL.md`) before amending the plan.

### P-021 C5/C6 Triage and Reconciliation Audit Record for `C2CFE06E`

1. **Originating Source References Carried Forward**:
   * `stash_id`: `C2CFE06E`
   * `deliberation_id`: `085-DL` (`.backlogit/archive/085-DL.md`, linked `informs -> 211-F`, `informs -> 212-S`, `related_to -> 082-DL`)
   * Originating capture context: `PR #638`, `shipment ID: 210-S`, `feature ID: 209-F`, `task ID: 209.003-T`, `review-thread ID: N/A (threadless local full-test finding)` (`docs/memory/2026-10-31/ship-210-s-pr638-review-halt.md`, `docs/memory/2026-11-06/212-S-wave1-convergence-blocked.md`).
2. **(A) Unconditional Duplicate Scan Outcome (`CLEAN DUPLICATE SCAN`)**:
   * Full inspection of active `.backlogit/stash.jsonl` and `.backlogit/archive/` confirmed `C2CFE06E` (`created_at: 2026-11-01T02:27:08.1478359Z`) is the sole stash entry for the Windows Go 1.24 `runtime\mcp-daemon.lock` missing-path and helper-process EOF defect. Zero duplicate entries exist to merge or archive.
   * Earlier host-toolchain lint captures `C6E224B8`, `46BD6AEE`, and `259A4F71` remain untouched and are not current blockers under the supported CI-equivalent Go 1.24.0 + `golangci-lint` v1.64.8 toolchain (`docs/compound/build-errors/match-ci-go-and-linter-before-source-repair-2026-11-06.md`).
3. **(B) Late-Identifier Reconciliation Outcome (`No late identifier found — terminal review-thread ID: N/A stands`)**:
   * Inspection of all `210-S` and `212-S` residual-risk, runtime-verification, pre-merge, post-merge closure, and session memory records confirmed that the finding originated from a local unfiltered `go test` run and never acquired a GitHub review thread. `review-thread ID: N/A` stands as a truthful terminal record.

### Options Evaluated in `085-DL`

* **Route A (Chosen — Bounded Wave-1 Prerequisite Task `U1b` / `211.008-T` + Stage Manifest Amendment `M7 -> M8`)**:
  * Create a bounded 2-file Wave-1 prerequisite task `U1b` (`211.008-T`, `status: queued`, `priority: high`, `size: S`, `complexity: medium`, `dependencies: []`) under `211-F` owning `internal/cli/mcp_daemon.go` and `internal/cli/mcp_daemon_lock_test.go`.
  * Bind `U2` (`211.002-T`) `depends_on` `U1b` (`211.008-T`) and `U5` (`211.005-T`) `depends_on` `U1b` (`211.008-T`) (`dep_type: "blocks"`), preserving all existing chain edges (`U1 -> U2 -> U3 -> U4 -> U5 -> U6 -> U7`) and keeping whole `U5` after `U3`/`U4`.
  * Append `211.008-T` to `212-S` via `backlogit_add_to_shipment` without force reorder (preserving all 8 existing members -> 9 total items `S` and 8 tasks `M8`, parent-first preserved, deterministic `ready_1 = {211.008-T}`).
  * Correct `U4` (`211.004-T`) POSIX `ListenMCPDaemonIPC` socket verification from `filepath.Dir(endpoint) == runtimeRoot.Name()` to canonical absolute containment and `os.SameFile` identity between `runtimeRoot.Stat/Lstat` and `os.Stat/Lstat`.
* **Route B (Rejected — In-Place Rescoping of `U4` and `U5` Within Fixed 7-Task `M`)**:
  * Rejected because removing `U3 -> U4` to move `U4` into Wave 1 and splitting Windows vs POSIX IPC across `U4` and `U5` fractures `U4`'s cross-platform IPC cohesion.
* **Option C (Rejected — Unwritten/Read-Only Clarification, Retroactive `U1` Expansion, or Expected-Red Waiver)**:
  * Rejected because editing `internal/cli/mcp_daemon.go` without an admitted task violates P-002/P-004, reopening `U1` (`211.001-T`) violates its 3-file ceiling and single `WORK_STARTED` epoch rule, and reclassifying pre-existing failures as expected RED violates P-002.6 Step 4.6.

### Chosen Direction and Governance Safeguards (`M1`, `M2`, `M3`)

1. **`M1` — Serialized Stage -> Ship Branch Custody and Recorded Residual Interpretive Risk**:
   * Stage authors the planning and backlog amendment (`085-DL`, `211.008-T`, updated `211-F`, `211.002-T`, `211.004-T`, `211.005-T`, `212-S`, deliberation/spec/plan docs, and Stage memory note) directly on the single active `212-S` feature branch worktree while Ship (`c5793081-4c91-4fa8-b9df-6693ec537a3c`) is IDLE with zero concurrent writers.
   * Stage does **not** exercise its default/admin branch `git commit` permission on the feature branch (`NOFEATURECOMMIT`), does not create or switch branches/worktrees, and does not push or create a PR. After Stage yields custody, Ship validates and commits the **unchanged Stage-authored bytes** with explicit provenance.
   * **Residual Interpretive Risk (Recorded, Not a Waiver)**: Supplemental independent Anchor review (`gpt-6.1-sol` `high`) inspected the exact Stage/Ship role Git/Planning boundaries, Ship Step 3, Orchestrator Concurrency/Step 1.5, and `.github/instructions/concurrency.instructions.md` and attested `READY_WITH_FOLLOWUPS`. `_orchestrator` Concurrency Control's statement that *"Stage and Ship operate on separate branches"* sits adjacent to the ban on simultaneous mutation and pipelined execution and is purpose-scoped to concurrent/pipelined work; for an in-flight wave-convergence blocker on an already-claimed shipment, **P-016** (single implementation branch/worktree per active shipment) and **P-002.6** (*"Only a Stage manifest amendment may change `M`"*) govern this serialized amendment path. Orchestrator Step 1.5's initial remote-`main` manifest gate was already satisfied prior to `212-S` claim.
2. **`M2` — Explicit Step 3 Freeze Replay (`M7 -> M8`) and Superseded Checkpoint Handling**:
   * `checkpoint-20261106-011453.json` (`M7`) is preserved unchanged on disk during Stage's turn (`frozen_task_ids` and failure history byte-preserved; never hand-edited in place). Its `M7` schedule cursor is designated **SUPERSEDED** in the amended plan (`D10` `M2`) and Stage memory note.
   * Ship replays full Step 3 freeze validation, scheduler fixtures/simulation, and fresh Step 4.0 wave census on `M8` (`8` tasks): `terminal_success = {211.001-T}` (`done`/archived, never re-run), `ready_1 = {211.008-T}` (`queued`, `0` starts), and `claim-assigned = {211.002-T .. 211.007-T}` (`6` active tasks with `scheduler_baseline_claim: "212-S"`, `0` starts, and `211.002-T` blocked by non-terminal `211.008-T`). After confirmed resume and writing a new official `backlogit_create_checkpoint` carrying `M8` (referencing `checkpoint-20261106-011453.json` as superseded predecessor), Ship calls `backlogit_resolve_checkpoint` on `checkpoint-20261106-011453.json` (the sole permitted governed lifecycle transition).
3. **`M3` — Technical Hardening of `U1b` (`211.008-T`) and `U4` (`211.004-T`)**:
   * `TestU1b_NestedRuntimeRootUsesCanonicalAbsolutePathForReparseValidation` in `internal/cli/mcp_daemon_lock_test.go` is sequential (no `t.Parallel()`), compiles cleanly against existing `internal/cli/mcp_daemon.go`, uses `t.Chdir` to an unrelated directory with no `runtime/` subdirectory, exercises existing production entrypoints (`AcquireMCPDaemonLock`, `InspectMCPDaemonStatus`, `StopMCPDaemon`) with default options to produce a genuine assertion RED on Windows (and cross-platform regression guard on POSIX), and executes both malicious lock-leaf and malicious sidecar-leaf (with live lock holder) reparse/symlink refusal assertions (`errors.Is(err, corerrors.ErrValidation)`) without `t.Skipf` (failing explicitly if fixture creation is unavailable).
   * `internal/cli/mcp_daemon.go` propagates the verified absolute `runtimeDir` across all production default options (`defaultMCPDaemonOptions().OpenLock` taking `(runtimeRoot *os.Root, runtimeDir string)`) and internal lock/sidecar/holder-read/reverification seams (`resolveMCPDaemonLeafPath`, `openMCPDaemonLockFile`, `readMCPDaemonInfoAt`, `writeMCPDaemonSidecar`) while validating `filepath.IsAbs(runtimeRoot.Name())` and `os.SameFile` on the test-only single-argument `readMCPDaemonInfo(runtimeRoot)` compatibility wrapper.
   * After `U1b` turns GREEN, Ship runs the unfiltered `go test -timeout=30m ./...` to genuine GREEN before admitting `U2` (`211.002-T`). Any later unrelated full-suite failure discovered after `U1b` routes through P-021 C1/C2 Stage intake rather than widening `U1b`.

## P-021 C6 Second Forward Deliberation Addendum — Folding `60D799D9` (`086-DL`, `U1c`/`U1d`) and `19D55778` (`087-DL`, `U1e`) into `211-F` / `212-S` and Dispositioning `96F1DFFD` (2026-11-06)

### Problem Frame and Lawful Separate Stage C6 Cycle While Dark Execution Is Paused

After `U1b` (`211.008-T`, commit `834972ab9108321c79c155022c5daaa9a9857d7f`) repaired Windows Go 1.24 nested `os.Root` runtime path validation and cleared all `./internal/cli` tests (`324.828s` unfiltered PASS), the mandatory Wave 1 Step 4.6 unfiltered full-suite gate (`go test -timeout=30m ./...`, captured in `bin/tmp/1793937249660-copilot-tool-output-7128-71ef1bbe-f07c-4954-a037-f7d919acbe22.txt` and diagnosed in `docs/memory/2026-11-06/212-S-U1b-green-unrelated-convergence-blockers.md`) under workspace-contained `TEMP` (`C:\Source\GitHub\backlogit\bin\_authoringtmp`) surfaced 9 top-level test failures across three independent surfaces, which Ship captured once under P-021 C2/C5 (`DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` fail-safe) and halted without triaging or modifying:

1. **`60D799D9` (`086-DL`, Git Discovery & Sanitized Runner Boundary Contracts)**:
   * **Family 1A (Narrow Production Local-`.git`-Directory Boundary Bug)**: `TestPlanArtifactMove_StrategyDetection/empty_git_directory_fails_closed` (`internal/core/archive_git_test.go:127`) and `TestInGitWorktreeBounded` subcase `(e)` (`internal/core/shipment_gate_ancestry_test.go:152`) create an empty `.git` directory at the workspace root (`os.Mkdir(filepath.Join(root, ".git"), 0o755)`) and assert fail-closed error return. Both `planArtifactMove` (`internal/core/archive.go:843-855`) and `(*Workspace).inGitWorktreeBounded` (`internal/core/shipment_gate.go:180-214`) check for a present-but-broken `.git` directory **only on the `err != nil` (exit 128) branch**. Because Git's `is_git_directory` ignores an empty `.git` directory (missing `HEAD`/`refs`/`objects`) and continues walking parent directories, whenever an ancestor Git repository exists above `workspaceRoot` (nested workspaces in production or contained `TEMP` in tests), `git rev-parse` exits `0` with the ancestor's worktree root, bypassing the `err != nil` guard and failing open.
   * **Family 1B (Non-Hermetic "No-Usable-Repository" Test Fixtures)**: `TestInGitWorktreeBounded` subcase `(b)` (`shipment_gate_ancestry_test.go:117`), `TestHeadSHABounded` (`shipment_gate_headdrift_test.go:76`), `TestRepairShipmentMemberEvidence_NoRepoHead_Errors` (`shipment_repair_test.go:248`), and `TestU163_002_ExecGitRequiresARepositoryDirectory` (`internal/policyengine/u163_002_record_test.go:215`) use plain `t.TempDir()` directories with no local `.git` boundary, so ordinary Git discovers the outer repository (`HEAD` `834972a`) while production runners (`gate.MinimalEnv`, `isGitOverrideEnv`, `policyengine.gitEnv`) rightly strip ambient `GIT_CEILING_DIRECTORIES`.
2. **`19D55778` (`087-DL`, Shipment Lifecycle / Doctor Fixture Member-Completion Sequencing)**:
   * `TestDoctor_IgnoresIntentionalOrphans` (`internal/core/025_doctor_harness_test.go:98`) and `TestShipmentWorkflow_ShipmentReleaseCleanup` (`tests/integration/shipment_workflow_test.go:199`) create `releasedTask` (`001.001-T`), call `ClaimShipment` (activating `001.001-T`), and call `ShipShipment` without completing `001.001-T`. Under contained `TEMP`, the default `ws.GateBroker` (`enabled: auto`) resolves the outer checkout's `HEAD`/`origin/HEAD` and enforces `validateMemberGateEvidence` (`internal/core/shipment_gate.go:750-754`), refusing the active member.
3. **`96F1DFFD` (`086-DL`, Intermittent Windows File-Replacement `Access is denied` & `internal/core` 30m Alarm — Pending Post-Fix Unfiltered Verification)**:
   * `TestNormalizeBlockedShipmentForRecovery_UnrelatedPoisonRemainsDiagnosable` and `TestUCYA1_ShipmentLifecycleStepErrorsKeepSentinelChain/U20C5_commit_journal` encountered transient Windows `Access is denied` during the parallel full run, but both passed `1/1` in Ship's isolated Windows probe (`3.668s` and `2.661s`). The `internal/core` 30m0s alarm fired `1s` into `TestCheckChildrenTerminal_AllDone_NoError` after steady progress across `1,184` top-level tests under parallel multi-package load (not an infinite gate-subprocess recursion).

**Why Separate Stage C6 Cycle Is Lawful Under P-017 and P-021 C4/C5/C6**:
* Ship did not auto-triage or implement `60D799D9`, `19D55778`, or `96F1DFFD` inside `U1b` or treat `DARK_MODE_ACTIVE` as a P-021 waiver. Execution is paused at the Wave 1 Step 4.6 convergence halt (`checkpoint-20261106-040526.json`), and Stage is invoked in a separate C6 cycle under the operator's explicit proactive-remediation mandate (`P-021 C4`).
* Amending `212-S` in-place (`M8 -> M11` per `P-002.6` and `P-016`) preserves the `212-S -> 213-S -> 214-S -> 215-S` cursor without claiming a new shipment or opening a parallel branch/worktree.

### P-021 C5/C6 Four-Case Triage and Reconciliation Audit Record for `60D799D9`, `19D55778`, and `96F1DFFD`

1. **Why Ship Recorded `DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` at Capture vs. Stage's Direct Active + Archived JSONL Scan**:
   * At capture time during Wave 1 convergence, Ship used only the SQLite-backed `backlogit` MCP search/stash tools (which index active `.backlogit/stash.jsonl` and harvested items, not `.backlogit/archive/stash.jsonl`), so Ship lawfully applied the P-021 fail-safe token `DISCOVERY-STATUS: LOOKUP-UNAVAILABLE`.
   * During this separate Stage C6 cycle, Stage directly scanned both `.backlogit/stash.jsonl` (`290` lines) and `.backlogit/archive/stash.jsonl` (`185` lines) and evaluated all near-surface archived candidates:
     * `885263D2` (archived): Windows `FindFirstFile`/`EvalSymlinks` stall tied to recovery-enabled `core.NewWorkspace` -> distinct surface from `60D799D9` (Git worktree discovery) and `96F1DFFD` (`NtSetInformationFile`/`MoveFileEx` journal replacement `AccessDenied`).
     * `D8EF5443` (archived): Full-suite log-capture buffer truncation (~9m of diagnostics lost) -> distinct harness-output surface, cited as an observability constraint on `96F1DFFD`.
     * `8F41D60D` (archived): Historical `197-S` full-suite load flake -> distinct surface.
     * `CCAEF17B` (archived): Historical Doctor orphan false-positive rule -> distinct from `19D55778` fixture member-completion sequencing.
2. **Standalone Four-Case Record for `60D799D9` (`086-DL`, `.backlogit/archive/086-DL.md`)**:
   * **Source refs carried forward**: `task ID: 211.008-T`, `feature ID: 211-F`, `shipment ID: 212-S`, `PR number: N/A (genuinely pre-PR)`, `review-thread ID: N/A (threadless local full-suite finding)`.
   * **C1 -> C6 Reconciliation**: Inside `U1b` (`211.008-T`), changing `archive.go`, `shipment_gate.go`, or `policyengine` tests failed P-021 C1 because `U1b` authorized only `internal/cli/mcp_daemon.go` and `internal/cli/mcp_daemon_lock_test.go`. Under P-021 C4/C6, `086-DL` forward-authorizes separate work units `U1c` (`211.009-T`) and `U1d` (`211.010-T`) under `211-F` / `212-S`.
   * **(A) Unconditional Duplicate Scan**: `CLEAN DUPLICATE SCAN` (`0` duplicates across `.backlogit/stash.jsonl` and `.backlogit/archive/stash.jsonl`; near-surface candidates `885263D2`, `D8EF5443`, `8F41D60D` adjudicated as distinct).
   * **(B) Late-Identifier Reconciliation**: `No late identifier found — terminal PR: N/A, review-thread ID: N/A stand`. Ship-owned residual-risk records searched by join key `60D799D9` / `211.008-T`: `docs/memory/2026-11-06/212-S-U1b-green-unrelated-convergence-blockers.md`, `docs/memory/2026-11-06/212-S-wave1-convergence-blocked.md`, `docs/memory/2026-11-06/212-S-M8-amended-cursor.md`, `docs/memory/2026-11-06/212-S-confirmed-resume.md`, `.backlogit/reconcile/212-S-pre-20261106T000529Z.md`, and `docs/closure/211-S-210-F-post-merge-closure.md`.
3. **Standalone Four-Case Record for `19D55778` (`087-DL`, `.backlogit/archive/087-DL.md`)**:
   * **Source refs carried forward**: `task ID: 211.008-T`, `feature ID: 211-F`, `shipment ID: 212-S`, `PR number: N/A (genuinely pre-PR)`, `review-thread ID: N/A (threadless local full-suite finding)`.
   * **C1 -> C6 Reconciliation**: Inside `U1b` (`211.008-T`), editing `internal/core/025_doctor_harness_test.go` or `tests/integration/shipment_workflow_test.go` failed P-021 C1. Under P-021 C4/C6, `087-DL` forward-authorizes separate work unit `U1e` (`211.011-T`) under `211-F` / `212-S`.
   * **(A) Unconditional Duplicate Scan**: `CLEAN DUPLICATE SCAN` (`0` duplicates across `.backlogit/stash.jsonl` and `.backlogit/archive/stash.jsonl`; near-surface candidate `CCAEF17B` adjudicated as distinct).
   * **(B) Late-Identifier Reconciliation**: `No late identifier found — terminal PR: N/A, review-thread ID: N/A stand` across all Ship-owned residual-risk records listed above.
4. **Standalone Four-Case Record for `96F1DFFD` (`086-DL`, retained active in `.backlogit/stash.jsonl`)**:
   * **Source refs carried forward**: `task ID: 211.008-T`, `feature ID: 211-F`, `shipment ID: 212-S`, `PR number: N/A (genuinely pre-PR)`, `review-thread ID: N/A (threadless local full-suite finding)`.
   * **(A) Unconditional Duplicate Scan**: `CLEAN DUPLICATE SCAN` (`0` duplicates across `.backlogit/stash.jsonl` and `.backlogit/archive/stash.jsonl`; near-surface candidates `885263D2`, `D8EF5443`, `8F41D60D` adjudicated as distinct).
   * **(B) Late-Identifier Reconciliation**: `No late identifier found — terminal PR: N/A, review-thread ID: N/A stand` across all Ship-owned residual-risk records listed above.
   * **Disposition**: Pending post-fix unfiltered full-suite verification (no-code diagnostic disposition).

### Chosen Direction (`086-DL` & `087-DL`: `U1c` / `211.009-T`, `U1d` / `211.010-T`, `U1e` / `211.011-T`, and `M8 -> M11`)

1. **`U1c` (`211.009-T`, `code`, 3 files, 4 functions total [3 production + 1 test]: `internal/core/archive.go`, `internal/core/shipment_gate.go`, `internal/core/shipment_gate_ancestry_test.go`)**:
   * Fail closed on present-but-unresolved `.git` entries in the half-open interval `[workspaceRoot, workTreeRoot)` across `planArtifactMove` (`internal/core/archive.go:848`) and `(*Workspace).inGitWorktreeBounded` (`internal/core/shipment_gate.go:171`) via `gitEntryPresentBelowWorkTree(workspaceRoot, workTreeRoot)`, while keeping `headSHABounded` (`shipment_gate.go:126`), production sanitization (`gate.MinimalEnv`, `isGitOverrideEnv`, `policyengine.gitEnv`), and valid nested/unborn/bare repository semantics unchanged.
   * Isolate `TestInGitWorktreeBounded` subcase `(b)` (`no-repo skip -> (false, nil)`) via the existing cross-platform `writeFakeGit(t, fakeGitScripts{...})` helper (`internal/core/archive_git_test.go:486`, Windows `.bat` executed via `kernel32!CreateProcessInternalW` + POSIX `#!/bin/sh`) scoped inside a subtest via `t.Setenv("PATH", fakeDir)`, and add hermetic nested-parent RED/GREEN tests (direct `wsNestedEmpty/.git` and intermediate `outer/mid/.git` above `outer/mid/ws`).
2. **`U1d` (`211.010-T`, `tests`, 3 files: `internal/core/shipment_gate_headdrift_test.go`, `internal/core/shipment_repair_test.go`, `internal/policyengine/u163_002_record_test.go`)**:
   * Write owned invalid `.git` file boundaries (`gitdir: <dir>/nonexistent-gitdir`) in `TestHeadSHABounded` and `TestU163_002_ExecGitRequiresARepositoryDirectory`, and call existing `initGitRepoNoCommits(t, ws.RootPath)` (`shipment_gate_ancestry_test.go:86`) in `TestRepairShipmentMemberEvidence_NoRepoHead_Errors` (`"(no commit yet)"`, asserting `"workspace HEAD is unresolvable"`).
3. **`U1e` (`211.011-T`, `tests`, 2 files: `internal/core/025_doctor_harness_test.go`, `tests/integration/shipment_workflow_test.go`)**:
   * In `TestDoctor_IgnoresIntentionalOrphans` and `TestShipmentWorkflow_ShipmentReleaseCleanup` (the exact 2 fixtures in `19D55778`, leaving `blocking_cascade_test.go` untouched), first set `ws.GateBroker = nil; require.Nil(t, ws.GateBroker)` (disabling the unconfigured external `autoharness` subprocess broker on these non-gate fixtures, matching `025_archive_harness_test.go:118`) and second complete activated `releasedTask` to `done` via governed `core.UpdateArtifact` after `ClaimShipment` and before `ShipShipment`.
4. **`M8 -> M11` Manifest Amendment and `96F1DFFD` Disposition**:
   * Wire `211.002-T` (`U2`) to depend on `211.009-T`, `211.010-T`, and `211.011-T` (`dep_type: "blocks"`), append `211.009-T`..`211.011-T` to `212-S` (`S12` / `M11`) without `force` reorder, keep `checkpoint-20261106-040526.json` (`M8`) byte-identical on disk as **SUPERSEDED** by `M11`, and verify `96F1DFFD` in the fresh post-repair uncached unfiltered Go 1.24 full suite under quiet-host conditions with `-json`/`-v` named-failure capture (`docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md`, `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md`, `docs/compound/2026-08-29-windows-preremove-rename-pattern.md`, permitting `-p=1` package-serial scheduling if needed without filtering tests or changing the 30m per-binary timeout). `[PROVISIONAL NO-CODE DISPOSITION SUPERSEDED BY 088-DL BELOW FOR RECONSTITUTE/ASSOCIATECOMMIT PRELIMINARY-PLAN READER LIFETIME COORDINATION]`.

## P-021 C6 Third Forward Deliberation Addendum — Adjudicating `96F1DFFD` Windows `ReconstituteShipments` Preliminary-Plan Reader Lifetime Overlap With `AssociateCommit` as Wave-1 Prerequisite `U1f` (`088-DL`, 2026-11-06)

### Problem Frame, Controlled Core Proof, and Official Windows Documentation

After `U1c` (`211.009-T`), `U1d` (`211.010-T`), and `U1e` (`211.011-T`) completed at `HEAD` `2c87ae6fdf64480ab6767e9015cae51f24abc2e6`, the third Wave-1 unfiltered full-suite convergence invocation (`212-M11-wave1-full`: `go test -count=1 -p=1 -timeout=30m -json ./...`, captured in `bin/tmp/1793952023849-copilot-tool-output-7128-9c0c1662-babc-4f5a-8b00-a8285271ca6d.txt`) accounted for all `50` packages (`3,871` top-level passes, `9` top-level failures, `0` timeouts). Seven of eight selected failures were caused by an overly long private `TEMP` directory path (`bin/_authoringtmp/212-M11-wave1-full-20261105T231529817`) triggering Git ref-lock `Filename too long` errors and passed `1/1` once Ship shortened its owned private `TEMP` to `bin/_qf4781986` (`41` characters). However, `TestU2_ReconstituteShipmentsAtomicExecutionAndRollback/concurrent_associate_commit_and_post_hook_isolation` (`internal/core/shipment_reconstitute_test.go:499`) failed in both the full run and the short-`TEMP` comparison with:

```text
associate commit: mutation partial (not-applied):
failed at frontmatter-scalar with no prior steps applied:
associate commit step 1 (frontmatter scalar):
move temp over destination:
backlogit: durable write not applied: Access is denied.
```

Under the operator's verbatim authorization (`"Fourth iteration approved."` at `2026-11-06T16:57:46.238Z`, authorizing one additional current-wave convergence cycle without resetting the `3` prior failures or granting a blanket fifth attempt), Ship executed one bounded, representative public-API Core mechanism proof (`bin/_c4e7a91b2/summary.json`, `bin/_c4e7a91b2/evidence2.json`, `PID 61948`, native exit `0`; recorded in `docs/memory/2026-11-06/212-S-fourth-cycle-authorization-and-diagnosis.md`) over API-created tasks `001.001-T`, `001.002-T`, and `001.003-T` (`config.WriteDefaults`, `core.NewWorkspace`, `core.CreateArtifact` with `core.WithParent`, `core.FindArtifactPath`, `core.AssociateCommit`) with `DurableWrites = true` (move flags `9`: `MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH`):

1. **Ordinary `os.Open` reader held open on `001.001-T.md` (nominal share mode `3`: `FILE_SHARE_READ | FILE_SHARE_WRITE`)**: `AssociateCommit` failed at `frontmatter-scalar` with wrapped `syscall.Errno(5)` (`Access is denied.`) and `errors.Is(err, blerrors.ErrWriteNotApplied) == true` (zero prior steps applied; file bytes, old reader snapshot, and DB commit unchanged).
2. **Explicit `CreateFileW` reader held open on `001.002-T.md` with full share mode `7` (`FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE`, exact no-follow target verified)**: `AssociateCommit` **failed with the exact same wrapped `syscall.Errno(5)` and `ErrWriteNotApplied == true`**.
3. **Reader handle closed on `001.003-T.md` before `AssociateCommit`**: `AssociateCommit` **succeeded** (`err == nil`, frontmatter and DB commit updated to `abc123def`, `1` `commit_links` row).

Official public Microsoft Windows documentation explains both held-reader denials:

* [FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information): Legacy Windows rename (without `FILE_RENAME_POSIX_SEMANTICS` `0x2` combined with `FILE_RENAME_REPLACE_IF_EXISTS` `0x1`) fails if the existing target file has open handles (unless only held by a breakable batch oplock). `MoveFileExW` flags `9` (`MOVEFILE_REPLACE_EXISTING` `0x1` | `MOVEFILE_WRITE_THROUGH` `0x8`) belong to a different flag domain (`0x2` on `MoveFileExW` is `MOVEFILE_COPY_ALLOWED`, not `FILE_RENAME_POSIX_SEMANTICS`).
* [MoveFileExW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw): `MOVEFILE_WRITE_THROUGH` defines the disk-flush durability contract; blindly swapping `MoveFileExW` without proving durability parity, OS compatibility, handle pinning, and old-handle snapshot semantics would risk weakening persistence guarantees.

Source trace of `internal/core/shipment_reconstitute_exec.go:319-345` proves a direct in-process overlap between `ReconstituteShipments` (`executeReconstitution`) and `AssociateCommit` (`internal/core/commits.go:95`):

* `AssociateCommit` acquires `lockShipmentLifecycleGlobal(ctx, ws)` (`commits.go:95`) before calling `findArtifact`, acquiring item-log lock `C` (`commits.go:142`), and calling `persistArtifactWithGuard` -> `atomicfile.WriteFileAtomicWithOptions` -> `MoveFileExW` (`commits.go:169`).
* However, `executeReconstitution` (`shipment_reconstitute_exec.go:321`) calls `peek, err := planReconstitution(ctx, ws, opts)` **before** acquiring `lockActiveStash(ws)`, `lockShipmentLifecycleGlobal(ctx, ws)`, and `lockShipmentMembership(ctx, ws, id)`. During that unlocked preliminary `planReconstitution` peek, `loadSources`, `buildTargets`, and `loadWorld` (`shipment_reconstitute_plan.go:293-305, 475-476`, `canonical_scan.go:93`, `artifacts.go:1156-1157`) walk `.backlogit/queue/` and `.backlogit/archive/` via `parseFile` -> `os.ReadFile`, opening `001.006-T.md` concurrently while `AssociateCommit` holds `lockShipmentLifecycleGlobal` and calls `MoveFileExW` on `001.006-T.md`.

### P-021 C5/C6 Four-Case Triage and Reconciliation Audit Record for `96F1DFFD` (`088-DL`)

1. **Source References and Reconciled Evidence Carried Forward**:
   * `stash_id`: `96F1DFFD` (identity reused; captured once by Ship under P-021 C2/C5; formally linked to `088-DL` at `.backlogit/archive/088-DL.md` via `backlogit_deliberate` with `"deliberation_id": "088-DL"`, superseding `086-DL`'s provisional no-code pending-verification disposition for this overlap surface).
   * Originating capture refs: `task ID: 211.008-T`, `feature ID: 211-F`, `shipment ID: 212-S`, `PR number: N/A (genuinely pre-PR)`, `review-thread ID: N/A (threadless full-suite finding)`.
   * Reconciled Ship residual-risk and diagnostic evidence records joined by `96F1DFFD` / `212-S`:
     * `bin/tmp/1793937249660-copilot-tool-output-7128-71ef1bbe-f07c-4954-a037-f7d919acbe22.txt` and `docs/memory/2026-11-06/212-S-U1b-green-unrelated-convergence-blockers.md`.
     * `bin/tmp/1793952023849-copilot-tool-output-7128-9c0c1662-babc-4f5a-8b00-a8285271ca6d.txt`, `docs/memory/2026-11-06/212-S-M11-fullsuite-circuit-handoff.md`, `docs/memory/2026-11-06/212-S-wave-convergence-circuit-checkpoint.md`, and `docs/memory/2026-11-06/212-S-wave-convergence-escalation-payload.json`.
     * `bin/_c4e7a91b2/summary.json`, `bin/_c4e7a91b2/evidence2.json` (gitignored private diagnostic artifacts under `bin/`), and tracked custody record `docs/memory/2026-11-06/212-S-fourth-cycle-authorization-and-diagnosis.md` (`"Fourth iteration approved."` at `2026-11-06T16:57:46.238Z`; disclosing both the initial preflight-only `probe2.exe` `PID 5704` exit `2` with `0` reader opens and `0` `AssociateCommit` calls caused by assuming public default `durable_writes=true`, and the corrected `probe3.exe` `PID 61948` exit `0` matrix with in-memory `ws.Config.DurableWrites=true`).
2. **(A) Unconditional Duplicate Scan Outcome (`CLEAN DUPLICATE SCAN`)**:
   * Stage directly scanned all `290` lines of active `.backlogit/stash.jsonl` and all `317` non-empty lines of archived `.backlogit/archive/stash.jsonl` (retiring the capture-time `DISCOVERY-STATUS: LOOKUP-UNAVAILABLE` token in `088-DL` while preserving `96F1DFFD`'s raw captured text unedited per Ship's single-write capture invariant):
     * Active `.backlogit/stash.jsonl` near-surface entries adjudicated as distinct: `703835DC` (`200-S` commit-writer `C -> global` lock order, resolved in `commits.go:95`), `DF533572` (`079-DL` `gofmt` cache traversal), `D3C8D13B` (`080-DL` crash-window created-ID journal ordering), `C2CFE06E` (`085-DL` `mcp_daemon.go` nested `os.Root.Name`), `7E14915F` (`078-DL` post-crash cross-writer attribution), `60D799D9` (`086-DL` Git worktree boundary), and `19D55778` (`087-DL` Doctor/integration fixture completion).
     * Archived `.backlogit/archive/stash.jsonl` near-surface entries adjudicated as distinct: `763A1152` (PR #424 `C -> B` lock order), `1E0C2251` (`archived_status` guard), `FE440C62` (`ArchiveItem` lock order), `62C6A469` (`072-DL`), `8F41D60D` (`197-S` `TestAppendEvent` flake), `D8EF5443` (`072-DL` test log truncation), `8CCB29CF` / `2F7FCA8B` (docline frontmatter), `FF1F3AC7` (`068-DL` checkpoint isolation), `885263D2` (`070-DL` `NewWorkspace` recovery scan), and `CCAEF17B` (Doctor orphan rule).
   * Zero duplicate entries exist; zero stash entries were archived or merged.
3. **(B) Mandatory Late-Identifier Reconciliation Outcome (`No late identifier found — terminal PR: N/A and review-thread ID: N/A stand`)**:
   * Stage searched all Ship-owned residual-risk and continuity records listed above by join key `96F1DFFD` / `212-S`. Because `212-S` remains pre-PR on local branch `feat/212-s-backlogit-multi-session-singleton-mcp-runtime-stdio-session-shim-and-channel-operation-coordinator` at `HEAD` `2c87ae6fdf64480ab6767e9015cae51f24abc2e6`, no PR number or GitHub review-thread ID has surfaced; `PR number: N/A` and `review-thread ID: N/A` stand as truthful terminal records.
4. **Honest Mechanism-vs-Attribution and Non-Global Writer Boundary**:
   * While `bin/_c4e7a91b2/evidence2.json` (and `docs/memory/2026-11-06/212-S-fourth-cycle-authorization-and-diagnosis.md`) proves the exact Windows legacy-rename mechanism and `shipment_reconstitute_exec.go:318-345` (`peek` at line `321`) proves the unlocked preliminary-plan read overlap with `AssociateCommit`, the exact failure-time denying handle in the historical test runs (`212-M11-wave1-full` and `212-M11-short-temp-once`) was not kernel-captured at failure time. We do not claim historical kernel-handle attribution or global serialization of all read-only queries; `U1f` (`211.012-T`) closes the source-proved `executeReconstitution` preliminary-plan reader lifetime overlap with `lifecycle-global` and source `shipment-membership` writers, while documenting that uncoordinated external readers, lock-free `DryRun: true` previews, and non-global in-process writers (such as `AssociateCommit` with `C` pre-held on a non-member artifact at `commits.go:57-95` or single-lock `B` writers like `SetArtifactSizeWithProvenance` at `artifact_size.go:110` acquiring `lockArtifactMutation` at `shipment.go:1423-1438`) remain outside `lifecycle-global` serialization.

### Options Evaluated in `088-DL` and Chosen Direction (`Route A`: `U1f` / `211.012-T` & `M11 -> M12`)

* **Route A (Chosen — Narrow Preliminary-Plan Reader Lifetime Coordination Under Existing Canonical Locks in `U1f` / `211.012-T` + Stage Manifest Amendment `M11 -> M12`)**:
  1. Keep `MoveFileExW` (`MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH`), `parseFile`, and lock-free `DryRun: true` preview semantics intact; do not reopen or modify completed `U1` (`211.001-T`), `U1b` (`211.008-T`), `U1c` (`211.009-T`), `U1d` (`211.010-T`), or `U1e` (`211.011-T`).
  2. Create **`U1f` (`211.012-T`, `code`, `status: queued`, `priority: high`, `size: S`, `complexity: medium`, `dependencies: []`, `3` files, `3` functions total [2 production + 1 test]: `internal/core/shipment_reconstitute_plan.go`, `internal/core/shipment_reconstitute_exec.go`, `internal/core/shipment_reconstitute_hardening_test.go`)** executed under **`careful` + `investigate-first` + `freeze-scope`** safety modes:
     * In `internal/core/shipment_reconstitute_plan.go` (`loadAndValidateReconstitutePlan`, lines 191–233; leaving `planReconstitution` at lines 310–359 and `loadSources` at lines 361–382 untouched so only 1 function in `shipment_reconstitute_plan.go` is modified): validate at the end of `loadAndValidateReconstitutePlan` that each entry in `plan.SourceShipments` is non-empty (`"source_shipments contains an empty ID"`), matches `validReconstituteID(id) && !strings.EqualFold(id, shipmentLifecycleGlobalLockID)` (`"source shipment ID %q is invalid"`, rejecting the reserved global lock key so it cannot self-contend in `lockShipmentMembership`), and is not duplicated under case-folding `seenSources[strings.ToLower(id)]` (`"source shipment %s is listed more than once"`, preventing Windows case-insensitive self-contention), with zero canonical `.md` artifact reads.
     * In `internal/core/shipment_reconstitute_exec.go` (`executeReconstitution`, lines 318–519): call `validateShipmentManifestOptions(ShipmentManifestOptions{Force: opts.Force, Reason: opts.Reason})` and `plan, digest, err := loadAndValidateReconstitutePlan(ws, opts)` first so invalid options, malformed/escaping plan YAML, or empty/invalid/reserved/duplicate `source_shipments` IDs fail fast before acquiring locks (accepting that `planReconstitution` re-runs option/plan validation during `peek` and `st` so `planReconstitution` and `loadSources` remain untouched within the 2-production-function budget); extract sorted `sourceIDs` from `plan.SourceShipments`; acquire `lockActiveStash(ws)` -> `lockShipmentLifecycleGlobal(ctx, ws)` -> `lockShipmentMembership(ctx, ws, id)` for each sorted `sourceID` **before** calling `peek, err := planReconstitution(ctx, ws, opts)`; check `peek.digest == digest` immediately after `peek` (before `C` and `B`); and check `st.digest == digest` after `st, err := planReconstitution(ctx, ws, opts)` under `C` and `B`.
     * In `internal/core/shipment_reconstitute_hardening_test.go`: add sequential `TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks` (inline closures only, no `t.Parallel()`) with 3 deterministic, zero-sleep subtests (`preliminary_plan_reads_wait_for_stash_global_and_membership_locks`, `concurrent_associate_commit_durable_replace_serializes_before_preliminary_plan_reads`, and `pure_input_and_plan_validation_fails_fast_before_locks`).
  3. Amend Shipment `212-S` from `M11` to `M12` (`S12 -> S13`): wire `211.002-T` (`U2`) `depends_on` `211.012-T` (`U1f`, `dep_type: "blocks"`), append `211.012-T` to `212-S` via `backlogit_add_to_shipment` without `force`, preserve `checkpoint-20261106-081656.json` byte-identical on disk as **SUPERSEDED** by `M12` upon Ship's confirmed re-freeze, and retain the `3` prior convergence failures with the `1` operator-approved fourth current-wave convergence cycle (`"Fourth iteration approved."` at `2026-11-06T16:57:46.238Z`) reserved for post-`U1f` verification.
* **Route B (Rejected — Change Global `parseFile` Share Flags to Share Mode `7`)**: Rejected because `bin/_c4e7a91b2/evidence2.json` (`001.002-T`) and Microsoft's `FILE_RENAME_INFORMATION` documentation prove that legacy `MoveFileExW` still fails with `syscall.Errno(5)` when a target handle is held open with share mode `7`, and existing `NoFollow` helpers also change containment, reparse, bounds, and error contracts.
* **Route C (Rejected for `212-S` Prerequisite Scope — Replace `MoveFileExW` Globally in `internal/atomicfile` With `FILE_RENAME_POSIX_SEMANTICS` or Serialize All Readers Under `lifecycle-global`)**: Rejected because replacing the repository-wide `atomicfile` primitive requires a dedicated cross-version Windows durability/filesystem/old-handle-snapshot proof beyond a bounded 2-hour prerequisite unit, and locking all read-only queries under `lifecycle-global` would destroy intended read concurrency.

## P-021 C6 Fourth Forward Deliberation Addendum — Wave 7 RCA Prerequisite Repairs (`089-DL`: `96F1DFFD` `U1g` / `211.013-T` and `F2509AE3` `U1h` / `211.014-T`) and Operator-Approved Option 1 `212-S` Gate-Cadence Exception (2026-11-07)

### Problem Frame, Consolidated RCA, and Verbatim Operator Authorizations

After `U1f` (`211.012-T`, `779f1200` / `d6b23ed3`) passed the sixth unfiltered 50-package convergence suite (`sixth-6-b6d58147`, `native_exit: 0`) and `U2` (`211.002-T`, `183ec2d52287a6465e4ef2e2952685f0ba0d7eb0`) completed its TDD and review gates (`docs/memory/2026-11-07/212-S-U2-done-await-wave-convergence.json`), the Wave 7 unfiltered convergence run (`bin/_rr8b31a6d2/wave7-rearm1-183ec2d/result.json`) terminated with `native_exit: 124` (`elapsed_seconds: 36266.58098970004` against `deadline_seconds: 2700`, `32/50` packages completed) and `1` assertion failure in `internal/core` (`TestNewWorkspace_RemovesWriterTempResidueAndRecoversValidJournal` at `internal/core/shipment_p1_lifecycle_remediation_test.go:88`). Consolidated Ship + Stage RCA (`docs/decisions/2026-11-07-212-s-delivery-blockers-rca.md`) and the concurrent read-only `claude-sonnet-5.5` (`medium`) actor trace established:

1. **Scope 1 (`96F1DFFD`, continuing `088-DL` — Startup Legacy Return-Blocked Recovery Redundant Task Rewrite)**:
   * In `internal/core/shipment_p1_lifecycle_remediation_test.go:68-85`, the fixture writes a legacy `return-blocked` journal (`journal.Reason == ""`) with preimages for shipment `001-S` and task `001.001-T`, mutates only shipment `001-S` membership on disk, leaves task `001.001-T` byte-unchanged, and closes `Workspace`.
   * During `NewWorkspace` auto-recovery (`internal/core/shipment.go:2963-3025`), `recoverLegacyReturnBlockedJournal` validates under held recovery locks (`lockShipmentLifecycleGlobalRaw` at `2727`, `lockShipmentMembership` at `2834`, `lockArtifactMutations` at `2840-2844`) that the on-disk task already matches `journal.Item` (`lines 2979-3004`), restores shipment `001-S` (`line 3010`), and then **unconditionally calls `persistArtifact(recoveryCtx, ws, journal.Item, true)` (`line 3011`)**, triggering an unnecessary temp-file write and `MoveFileExW` replacement (`internal/atomicfile/atomicfile_windows.go:50-53`) over the unchanged task `.md` file that returned `Access is denied` (`ErrWriteNotApplied`).
   * The redundant physical write is **proven**; the handle/filter actor denying `MoveFileExW` remains **unproven** (concurrent `claude-sonnet-5.5` `medium` read-only actor RCA confirmed with `HEAD` `183ec2d5` unchanged that the historical `TEMP` target is gone, Windows Security log had no `4656/4658/4660/4663` file-access events because `bin/` has no SACL, `Kernel-File` Analytic logging is disabled, `Sysmon`/`handle`/`procmon` are absent, and installed filter/Defender/indexer presence is not cause proof).
2. **Scope 2 (`F2509AE3`, `089-DL` — Host S4 Hibernation During Wave 7 & Private Verification Runner Hygiene)**:
   * Sanitized Windows `System` power events (`.copilot\session-state\4a60e334-a29e-4db4-9f3a-be1e5f382920\files\rca-power-events.json`, `SHA256: a546b1b7ce87d0de84ffeb57fd4501bcaa325f6f4d511352f996cc97b67aa5be`) prove the host entered **S4 Hibernation (`TargetState=5, EffectiveState=5`)** at `2026-11-07T10:59:55.045Z` (`1,828.05s` into test child `PID 10344`) and remained suspended for **`34,435.56s` (`9h 33m 55.6s`)** until `2026-11-07T20:33:46.500Z-20:33:50.601Z`, whereupon `_supervise` (`bin/tmp/212-full-suite-cycle7.py:454-460`) killed the job on wake under the `2,700s` wall budget.
   * `_supervise` (`bin/tmp/212-full-suite.py`) lacks process-scoped `SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED)` and poll-gap telemetry (`max_poll_gap_seconds`, `poll_gap_over_5s_count`); `_main` lacks a pre-flight `TestU175_001_BaselineEol` check (which would have avoided the extra `~41.66 min` Cycle 6 rerun after Cycle 5's `42.53 min` CRLF receipt failure); and `_validate_authorization` (`line 535`) hardcodes integer literals `ordinal` and `failed_count` in Python source.

### Verbatim Operator Authorizations and Chosen Direction (`089-DL`: `U1g` / `211.013-T`, `U1h` / `211.014-T`, `M12 -> M14`, and `212-S` Option 1 Cadence)

* **Verbatim operator directives**:
  * `2026-11-07T21:20:35Z`: `"If you were planning, stop planning and start implementing... make good decisions and keep working...Keep working autonomously until task truly finished"`
  * `2026-11-07T21:24:02Z`: `"1. Option 1 in primary workflow process 2. Try to run option 3 concurrently, if possible, in a separate subagent task routed claude-sonnet-5.5 medium. Return results findings and deliberate on how to address what you find."`
* **Chosen Direction (`089-DL`, `.backlogit/archive/089-DL.md`, `status: done`)**:
  1. **`U1g` (`211.013-T`, `code`, `status: queued`, `priority: high`, `size: S`, `complexity: low`, `2` files, `2` functions total [1 production + 1 test]: `internal/core/shipment.go`, `internal/core/shipment_p1_lifecycle_remediation_test.go`)**:
     * In `recoverLegacyReturnBlockedJournal` (`internal/core/shipment.go:2963-3025`), under existing recovery locks and after preimage validation (`lines 2979-3004`), check whether `currentItem` already matches `journal.Item` (`recoveryArtifactMatchesAny(currentItem, []recoveryArtifactCandidate{{artifact: journal.Item}})`) at its canonical target path (`currentItemPath == targetItemPath` via `resolveArtifactPersistPaths(recoveryCtx, ws, journal.Item, true)`). When identical at the target path, skip the physical file replacement for `journal.Item`, reconcile SQLite directly via `mergeDBOnlyLinksIntoArtifact(recoveryCtx, ws, indexedItem)` and `bldb.UpsertItem(recoveryCtx, ws.DB, indexedItem)`, and remove the journal only after both shipment restore and item reconciliation succeed. Preserve fail-closed `ErrWriteNotApplied` when shipment restore or a changed task fails replacement.
     * Verify with `TestU1g_LegacyReturnBlockedRecoverySkipsUnchangedTaskRewriteAndFailsClosedOnShipmentOrTaskDivergence` covering: (1) unchanged-task writer denial (`RED` -> idempotent `GREEN` with `0` task writer calls, SQLite reconciled, journal removed); (2) admitted changed-shipment (membership-removed) restore writer denial ($\rightarrow$ `ErrWriteNotApplied` + retained journal); and (3) non-admissible changed task with empty `journal.Reason` ($\rightarrow$ `ErrShipmentConflict` + `0` restore writer calls + retained journal).
  2. **`U1h` (`211.014-T`, `tests`, `status: queued`, `priority: high`, `size: S`, `complexity: low`, `3` files: `bin/tmp/212-full-suite.py`, `bin/tmp/test_212_full_suite.py`, `scripts/verify-u1h-harness.ps1`)**:
     * Harden `bin/tmp/212-full-suite.py` with process-scoped `SetThreadExecutionState(0x80000001)` (`ES_CONTINUOUS | ES_SYSTEM_REQUIRED`) cleared via `SetThreadExecutionState(0x80000000)` in `finally:`; `max_poll_gap_seconds` and `poll_gap_over_5s_count` telemetry in `supervision` (preserving the `2,700s` outer budget and `1,800s` package budget without subtracting poll gaps); supervised pre-flight `TestU175_001_BaselineEol` execution (`preflight-eol-result.json`) after discovery succeeds and before launching the 50-package test child; and dynamic `ordinal`/`failed_count` validation from matching coordinator receipts in `_validate_authorization` with explicit `prelaunch_only` / `test_cycle_consumed` reporting while preserving unique per-attempt evidence directories. Verify via `bin/tmp/test_212_full_suite.py` and tracked `scripts/verify-u1h-harness.ps1` (`-Phase Red | Green`) with `0` skips.
  3. **`M12 -> M16` Manifest Amendment, P-002.6 Red-Deliverable Step 0.5 Pre-Dispatch Snapshot Boundary Repair `U2j` (`211.016-T`), and Approved `212-S`-Only Option 1 Gate-Cadence Exception via `U2i` (`211.015-T`)**:
     * Under the operator's verbatim R5 implementation directive (`"If you were planning, stop planning and start implementing...make good decisions and keep working...Keep working autonomously until task truly finished."` at `2026-11-08T02:47:24Z`), add **`U2j` (`211.016-T`, `tests/harness`, `status: queued`, `priority: high`, `size: S`, `complexity: medium`, `dependencies: [211.001-T, 211.002-T, 211.013-T, 211.014-T]`)** to partition the P-002.6 `red_deliverable` Step 0.5 zero-delta gate between pre-claim `red_baseline_sha` (`30908ecb`) and an immediate pre-dispatch working-tree/index state snapshot (`red_predispatch_snapshot`) across `tests/integration/claim_start_wave_sim_contract_test.go`, `scripts/wave-scheduler-sim.ps1`, `tests/simulation/wave-scheduler-contract.json`, `.github/skills/build-feature/SKILL.md`, `.github/agents/_ship.agent.md`, and `.github/policies/workflow-policies.md` (strictly scoped to `red_deliverable` Step 0.5; zero `harness-exempt` / P-002.4 changes).
     * Wire `211.015-T` (`U2i`, preserving its `status: active`, scaffold commit `6eee17f9`, `harness-ready` label, and single `WORK_STARTED` record without reclaiming/rewinding/restarting) `depends_on` `211.016-T` (`U2j`, `dep_type: "blocks"`), update `green_maker_closes_wave: 9` on `211.015-T`, and append `211.016-T` to `212-S` (`S16 -> S17`, `M15 -> M16`).
     * Once `211.016-T` (`U2j`) is committed and `done` in Wave 3, Ship reloads the updated `.github/agents/_ship.agent.md` and `.github/skills/build-feature/SKILL.md` sources, captures a fresh post-caller `red_predispatch_snapshot`, and re-dispatches `211.015-T` (`U2i`) in Wave 4 with original `red_baseline_sha = 30908ecb8438c0b3e7fe8bd514fa3680e35d90e5` + `red_predispatch_snapshot` -> `RED_DELIVERABLE_ACCEPTED` -> `done` -> registers `^TestU2i_` in `open_red_deliverables_k` across Waves 4–8 (`FULL_SUITE_DEFERRED: wave 4..8`), then closes `^TestU2i_` GREEN at Wave 9 (after `U7` `211.007-T` completes) and executes **ONE** unfiltered 50-package full-suite convergence run (`go test -count=1 -p=1 -timeout=30m -json ./...`, `2,700s` outer / `1,800s` package budgets unchanged) before PR creation. Successor shipments `213-S`–`215-S` do not inherit this `212-S`-only exception. `[WAVE SCHEDULE UPDATED BY FIFTH FORWARD DELIBERATION ADDENDUM BELOW: M16 -> M19 / S17 -> S20; U3a-U5a AT WAVE 5, U3-U7 AT WAVES 6-10, green_maker_closes_wave: 10]`.

## P-021 C6 / P-002.1 Fifth Forward Deliberation Addendum — Wave 5 Declaration-vs-Behavior Split (`089-DL` / Option 5: `U3a` `211.017-T`, `U4a` `211.018-T`, `U5a` `211.019-T`, and `M16 -> M19` Refreeze, 2026-11-08)

### Problem Frame and Wave 5 `harness-architect` Structural Halt

After Ship completed `211.016-T` (`U2j`, commit `fd038b8feba15e43d7f02ed2742a0e91479b16b3`), accepted `211.015-T` (`U2i`) RED (`RED_DELIVERABLE_ACCEPTED`), and discharged both pending Wave 3 and Wave 4 convergence gates (`FULL_SUITE_DEFERRED`, recorded in `docs/memory/2026-11-08/212-S-longpaths-wave34-convergence.md` at `HEAD` `905d0a43`), `harness-architect` halted at Wave 5 (`ready_5 = {211.003-T}`) before any mutation with a structural plan defect:

* Under **P-002.1 cycle-31** (`.github/policies/workflow-policies.md:123-143`) and `.github/skills/harness-architect/SKILL.md` Step 1 item 7 and Step 4 items 3, 5, 6, any task whose delta introduces new exported symbols/declarations whose bodies will absorb real behavior MUST be split into:
  1. a **declaration task** gated by a `go/ast` source-shape harness (`harness first, declaration second`, with **no production stub** written by `harness-architect` during scaffolding and no temporary stub body frozen in the shape test per `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`), followed by
  2. a **behavior task** in a strictly later wave whose behavioral harness (`TestU3_*`, `TestU4_*`, `TestU5_*`, `TestU6_*`) compiles cleanly against the already-landed declarations on `HEAD` (with zero production stub edits by `harness-architect`), exactly as `U1` (`211.001-T`) declared the coordinator symbols and `U2` (`211.002-T`) implemented their behavior.
* In the `M16` plan, `U3` (`211.003-T`), `U4` (`211.004-T`), `U5` (`211.005-T`), and `U6` (`211.006-T`) combined introducing new declarations (`MultiSessionConn`, `ServeSessionStream`, `ActiveSessionCount`, `CloseAllSessions`, `DeriveMCPDaemonEndpoint`, `ListenMCPDaemonIPC`, `DialMCPDaemonIPC`, `isVerifiedProcessAlive`, `MCPDaemonInfo`/`MCPDaemonStatus` fields, `VerifyMCPDaemonAttachment`, `RunMCPDaemon`, `PrewarmMCPDaemon`, `RunMCPSessionShim`) with behavioral implementation in the same task.

### Chosen Direction under Operator Option 5 Authorization (`M16 -> M19`, `S17 -> S20`)

1. **Create Three Wave 5 Declaration Prerequisite Tasks (`U3a` / `211.017-T`, `U4a` / `211.018-T`, `U5a` / `211.019-T`) Under `211-F` / `212-S`**:
   * All three tasks depend on `[211.002-T, 211.013-T, 211.014-T, 211.015-T, 211.016-T]` (all `done`), have 100% disjoint file scopes, and enter Wave 5 together (`ready_5 = {211.017-T, 211.018-T, 211.019-T}`):
     * **`U3a` (`211.017-T`, `code`, `size: XS`, `complexity: low`, `2` files: `internal/mcp/multi_session_shape_test.go`, `internal/mcp/session_mux.go`)**: `TestU3a_MultiSessionStreamDispatcherSourceShape` `go/ast` shape assertions (`harness-architect` writes NO production stub) + `MultiSessionConn` (implementing `mcpserver.ClientSession`) and `(*Server).ServeSessionStream`, `(*Server).ActiveSessionCount`, `(*Server).CloseAllSessions` minimal stub declarations in `internal/mcp/session_mux.go`. Scoped verification: `go test -timeout=30m -count=1 -run '^TestU3a_' ./internal/mcp`.
     * **`U4a` (`211.018-T`, `code`, `size: XS`, `complexity: low`, `3` files: `internal/cli/mcp_ipc_shape_test.go`, `internal/cli/mcp_ipc_windows.go`, `internal/cli/mcp_ipc_unix.go`)**: `TestU4a_PlatformIPCSignaturesSourceShape` `go/ast` shape assertions (`harness-architect` writes NO production stub) + `DeriveMCPDaemonEndpoint`, `ListenMCPDaemonIPC`, `DialMCPDaemonIPC`, and `isVerifiedProcessAlive` minimal stub declarations across `internal/cli/mcp_ipc_windows.go` and `internal/cli/mcp_ipc_unix.go`. Scoped verification: `go test -timeout=30m -count=1 -run '^TestU4a_' ./internal/cli`.
     * **`U5a` (`211.019-T`, `code`, `size: XS`, `complexity: low`, `2` files: `internal/cli/mcp_daemon_shape_test.go`, `internal/cli/mcp_daemon.go`)**: `TestU5a_DaemonAndShimDeclarationsSourceShape` `go/ast` shape assertions (`harness-architect` writes NO production stub) + `MCPDaemonInfo` and `MCPDaemonStatus` struct field extensions (`IPCTransport`, `Endpoint`, `ActiveSessions`, `ProtocolVersion`) and `VerifyMCPDaemonAttachment`, `RunMCPDaemon`, `PrewarmMCPDaemon`, `RunMCPSessionShim` minimal stub declarations in `internal/cli/mcp_daemon.go` (**without** touching `internal/cli/root.go`, so `TestRegistryParity_*` stays green and `^TestU2i_` stays RED through Wave 9 until `U7` completes in Wave 10). Scoped verification: `go test -timeout=30m -count=1 -run '^TestU5a_' ./internal/cli`.
2. **Wire Downstream Behavioral Waves (`Waves 6–10`) and Update `green_maker_closes_wave: 10` on `211.015-T` (`U2i`)**:
   * `211.003-T` (`U3`, Wave 6) `depends_on` `211.017-T` (`U3a`); `harness-architect` scaffolds only `internal/mcp/session_mux_test.go` with zero production stub edits.
   * `211.004-T` (`U4`, Wave 7) `depends_on` `211.003-T` (`U3`) and `211.018-T` (`U4a`); `harness-architect` scaffolds only `internal/cli/mcp_ipc_test.go` with zero production stub edits.
   * `211.005-T` (`U5`, Wave 8) `depends_on` `211.004-T` (`U4`), `211.008-T` (`U1b`), and `211.019-T` (`U5a`); `harness-architect` scaffolds only `internal/cli/mcp_daemon_multisession_test.go` with zero production stub edits.
   * `211.006-T` (`U6`, Wave 9) `depends_on` `211.005-T` (`U5`) and `211.019-T` (`U5a`); `harness-architect` scaffolds only `internal/cli/mcp_session_shim_test.go` with zero production stub edits.
   * `211.007-T` (`U7`, Wave 10, `harness-exempt: docs-only`) `depends_on` `211.006-T` (`U6`), closing `211.015-T` (`U2i`, `green_maker_closes_wave: 10`) GREEN at Wave 10 and discharging the single unfiltered 50-package convergence suite before PR creation.
   * Preserve `211.003-T`..`211.007-T` `status: active`, `custom_fields.scheduler_baseline_claim: 212-S`, and `0` `WORK_STARTED` log state, and leave `checkpoint-20261108-230851.json` and `checkpoint-20261106-210317.json` completely untouched on disk (`NOFEATURECOMMIT`).
