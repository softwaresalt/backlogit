---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash ECF33C79 and deliberation 082-DL: deliver the multi-session singleton MCP runtime with stdio-to-IPC session shims and FIFO Go channel operation gating (Track B: 211-F / 212-S), the workspace tool-environment Level 1 inventory and version-pinning contract (Track A: 212-F / 213-S), and cross-tool launcher/companion MCP alignment and negative verification (Track C: 213-F / 214-S).'
doc_type: plan
docline:
  deliberation_id: '082-DL'
  features:
    - 211-F
    - 212-F
    - 213-F
  plan_hardening_status: completed
  requires_plan_hardening: 'yes'
  review_status: passed_plan_review
  shipments:
    - 212-S
    - 213-S
    - 214-S
  source_decision: docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md
  source_spec: docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md
  stash_id: ECF33C79
  status: approved
schema_version: "1.0"
source: docs/exec-plans/2026-11-04-multi-session-mcp-runtime-plan.md
title: 'Implementation Plan — Multi-Session Singleton MCP Runtime and Workspace-Pinned All-Tools Environment (ECF33C79 / 082-DL)'
---

## Objective

Deliver the approved multi-session singleton MCP runtime and workspace-pinned all-tools environment specified in `docs/product-specs/2026-11-04-multi-session-mcp-runtime-requirements.md` and decided in `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` (`082-DL`, anchor stash `ECF33C79`), structured across three outcome-oriented covering features and three queued shipments:

* **Track B — Feature `211-F` / Shipment `212-S` (`Backlogit Multi-Session Singleton MCP Runtime, Stdio Session Shim, and Channel Operation Coordinator`)**:
  * Evolve `backlogit mcp` from a single-process exclusion guard (`204-F` / `205-S`) into an **independent per-workspace background daemon** (`backlogit mcp daemon`) paired with drop-in `stdio`-to-IPC session shims (`backlogit mcp`) and idempotent startup prewarm (`backlogit mcp prewarm`, `R1`–`R6`, `R11`–`R17`).
  * Gate all `backlogit` runtime-managed MCP mutations sequentially in deterministic FIFO order via an internal Go channel `OperationCoordinator` (`OpMutating`, `OpReadOnly`, `OpExclusiveBarrier`) with synchronous-frame turn/lock ownership (`R7`–`R9`, `R12`) and capability-gated `notifications/progress` emission (`R10`).
  * Replace exclusive lock probing in `InspectMCPDaemonStatus` and session connect handshakes with non-exclusive sidecar + process-identity + IPC readiness probing (`R3`, `D5`, `A045DBA4`).
* **Track A — Feature `212-F` / Shipment `213-S` (`Workspace Tool-Environment Inventory, Level 1 Version-Pinning Schema, and Threat-Model Contract`)**:
  * Define the cross-tool workspace toolchain inventory schema (`schemas/workspace-toolchain.schema.json`) and Go verification package (`internal/toolchain`) enforcing **Level 1 containment strength** (workspace-local installation, configuration, and version pinning across all workspace tools with zero silent global `PATH` or cross-workspace fallback, explicit `host-prerequisite` and `remote-declared` declarations, and honest `built-in-host`, `unsupported`, and `non-conformant` reporting, `R18`–`R23`, `D7`, `D9`).
* **Track C — Feature `213-F` / Shipment `214-S` (`Cross-Tool Workspace Pinning Verifier, Launcher/Companion MCP Alignment, and Multi-Workspace Negative Verification`)**:
  * Integrate Track A and Track B into `backlogit mcp status --toolchain [--strict]` and `internal/toolchain/launcher.go` to audit `.mcp.json`, `start.ps1`, `start.sh`, and companion workspace MCP servers (`engram`, `graphtor-docs`, `context7`, `tavily`, `github`), backed by end-to-end multi-workspace isolation and local-pinning negative test suites (`R19`–`R23`, `SC7`–`SC14`).

## Problem Frame

### Codebase and SDK Grounding

1. **Why `204-F` (`205-S`) and `mcp-go v0.27.0` `StdioServer` Cannot Host Multiple Concurrent Agents**:
   * `internal/cli/root.go:467` (`newMCPCommand`) currently calls `AcquireMCPDaemonLock(cmd.Context(), *cwd, replace)` (`internal/cli/mcp_daemon.go:265`) and then `serveMCPStdio(s)` (`internal/mcp/server.go:132`, `mcpserver.ServeStdio(s.mcp)`).
   * When a second agent (e.g., Stage running alongside Orchestrator, Ship running alongside an IDE session, or parallel review subagents) invokes `./bin/backlogit.exe mcp`, `AcquireMCPDaemonLock` fails immediately with `corerrors.ErrDaemonAlreadyRunning` unless `--replace` is passed—which terminates the first agent's active MCP server.
   * Direct source inspection of `github.com/mark3labs/mcp-go v0.27.0` (`server/stdio.go:53-77, 197-222`) confirms two SDK constraints in `StdioServer`:
     * `StdioServer.Listen` registers a package-global static session (`&stdioSessionInstance`, `SessionID() == "stdio"`). Calling `Listen` a second time on the same `*mcpserver.MCPServer` returns `ErrSessionExists`.
     * `StdioServer.processInputStream` invokes `s.server.HandleMessage(ctx, rawMessage)` synchronously in its line-reading loop (`stdio.go:139, 236`), and `MCPServer.HandleMessage` (`server/request_handler.go:45-56`) does not maintain a per-session in-flight request cancellation table for `notifications/cancelled`.
   * However, `mcpserver.MCPServer` itself (`server/session.go:40-185`) stores sessions in a `sync.Map` keyed by `session.SessionID()`, marks the session initialized inside `handleInitialize` (`server/server.go:530`), and routes `SendNotificationToClient(ctx, method, params)` to `ClientSessionFromContext(ctx).NotificationChannel()`. Constructing a per-connection `ClientSession` implementation (`MultiSessionConn`) with a unique `sessionID`, a mutex-serialized line writer, and a per-session `inFlight` request map (`requestId -> context.CancelFunc`) provides complete per-session request, notification, and cancellation isolation over a single `*mcpserver.MCPServer`.
2. **In-Memory State and Concurrency Surfaces in `internal/mcp/Server`**:
   * `Server` (`internal/mcp/server.go:27-62`) holds `RootPath`, `Workspace *core.Workspace` (with a 4-connection WAL-mode `*sql.DB` pool configured in `internal/db/connection.go:75-76` with `busy_timeout(30000)` and `_txlock=immediate`), `Events *events.EventWriter`, `Telemetry *events.TelemetryWriter`, `HookEvents *events.HookEventWriter`, `templateSvc *templates.Service`, `workspaceMu sync.Mutex`, and `manifest map[string]db.FileEntry` guarded by `manifestMu sync.RWMutex`.
   * Two subtle concurrency hazards exist in `internal/mcp` today if multiple sessions call handlers concurrently:
     * `handleListTemplates` (`internal/mcp/dynamic.go:31`) reads `s.templateSvc` without holding `s.workspaceMu`, racing with `ensureWorkspace` (`internal/mcp/server.go:167`) which calls `s.refreshTemplateService(ctx)` and mutates `s.templateSvc` under `s.workspaceMu`.
     * `handlePollHookEvents` and `handleAckHookEvents` (`internal/mcp/hook_tools.go:71-118`) read `s.HookEvents` and `s.backlogitDir()` without calling `s.ensureWorkspace(ctx)` or locking `s.workspaceMu`, racing with `ensureWorkspace` (`server.go:180`) which reassigns `s.HookEvents` and `s.Telemetry` when an initially uninitialized workspace is later initialized on disk.
   * Furthermore, mutating tool handlers in `internal/mcp/tools.go` perform multi-step file, manifest, event-log, and SQLite writes without an internal FIFO admission queue, while `backlogit_sync_index` (`handleSyncIndex`, `tools.go:920`) runs `core.MigrateDBOnlyLinksBeforeRehydrate` + `db.Rehydrate` across the entire SQLite database without draining concurrent in-process readers.
3. **Exclusive Lock-Probe Contention in `InspectMCPDaemonStatus` (`A045DBA4` / `R3` / `D5`)**:
   * `InspectMCPDaemonStatus` (`internal/cli/mcp_daemon.go:339-374`) currently probes liveness by calling `openMCPDaemonLockFile(runtimeRoot)` (`tryLockFile`), which takes a non-blocking **exclusive kernel lock** on `mcp-daemon.lock`. If `backlogit mcp status` or a connecting shim probes `mcp-daemon.lock` at the same instant a daemon is starting up and calling `AcquireMCPDaemonLock`, the status probe's transient exclusive lock causes `AcquireMCPDaemonLock` to fail spuriously with `ErrDaemonAlreadyRunning`.
4. **All-Tools Workspace Environment and Dirty Worktree Baseline (`R19`–`R23`, `D9`)**:
   * Inspection of `.mcp.json`, `start.ps1`, `start.sh`, `bin/`, `.copilot/graphtor-mcp-shim.cjs`, and `tools/mcp/` shows:
     * `.mcp.json` pins `backlogit` to `"./bin/backlogit.exe"`, but uses bare command names for `"engram"` (`"command": "engram"`), `"graphtor-docs"` (`"command": "bun"`, `"args": [".copilot\\graphtor-mcp-shim.cjs", "graphtor-docs", ...]`), `"context7"`, `"tavily"`, and `"github"` (`"command": "github-mcp-server.exe"`), even though `bin/engram.exe`, `bin/bun.exe`, `bin/bunx.exe`, `bin/github-mcp-server.exe`, and `bin/kbdb.exe` exist in `./bin/`.
     * Pre-existing uncommitted edits exist in `.autoharness/config.yaml`, `.gitignore`, `start.ps1`, `tools/`, and unrelated stash entry `D895887C` in `.backlogit/stash.jsonl`. Stage MUST NOT modify or commit those unrelated dirty files, and the plan's toolchain verifier (`internal/toolchain`) must inspect and report workspace tool compliance honestly without mutating dirty operator files or claiming control over upstream external repositories.

## Requirements Trace (`R1`–`R23` and `SC1`–`SC14`)

| Requirement / SC | Summary | Implementation Unit(s) |
|---|---|---|
| **R1**, **SC1** | Concurrent multi-agent session hosting (`>= 2` simultaneous active MCP sessions against one workspace runtime without `ErrDaemonAlreadyRunning` or peer termination) | `U3` (`211.003-T`), `U5` (`211.005-T`), `U6` (`211.006-T`) |
| **R2**, **SC1**, **SC2** | Drop-in `"type": "stdio"` client compatibility via thin stdio-to-local-IPC session shim (`backlogit mcp`) | `U4` (`211.004-T`), `U5` (`211.005-T`), `U6` (`211.006-T`) |
| **R3**, **SC5**, **SC6c** | Handle-pinned `<storage_root>/runtime` `*os.Root` lock (`mcp-daemon.lock`), backward-compatible `mcp-daemon.json` evolution (`ipc_transport`, `endpoint`, `active_sessions`), and non-exclusive status/connect probing (`D5`, `A045DBA4`) | `U4` (`211.004-T`), `U5` (`211.005-T`) |
| **R4**, **SC2** | Per-session `ClientSession` registration (`RegisterSession`, `UnregisterSession`, `WithContext`) and independent disconnect survival | `U3` (`211.003-T`), `U5` (`211.005-T`), `U6` (`211.006-T`) |
| **R5**, **SC2** | Per-session JSON-RPC request ID (`id`) and response stream routing isolation | `U3` (`211.003-T`), `U6` (`211.006-T`) |
| **R6**, **SC2** | Per-session `notifications/cancelled` context cancellation and `NotificationChannel` isolation | `U2` (`211.002-T`), `U3` (`211.003-T`) |
| **R7**, **SC3** | Go channel-gated FIFO mutation serialization (`OperationCoordinator`, `OpMutating`) for `backlogit` runtime-managed operations; explicit external writer boundary | `U1` (`211.001-T`), `U2` (`211.002-T`), `U10` (`212.003-T`) |
| **R8**, **SC3** | Bounded concurrent read execution (`OpReadOnly`, capacity 4 matching SQLite WAL pool) + exclusive consistency barrier (`OpExclusiveBarrier`) for `backlogit_sync_index` | `U1` (`211.001-T`), `U2` (`211.002-T`) |
| **R9**, **SC4** | Bounded pending mutation queue (`cap = 32`), admission timeout (`30s`), structured `BusyErrorResponse` (`retryable: true`), and synchronous-frame turn/lock ownership (`timeout-goroutine-lock-ownership-go-2026-07-01.md`) | `U1` (`211.001-T`), `U2` (`211.002-T`) |
| **R10** | Synchronous `tools/call` responses paired with capability-gated `notifications/progress` emission on `ClientSession.NotificationChannel()` when `_meta.progressToken` is present (`D8`, `A3`) | `U1` (`211.001-T`), `U2` (`211.002-T`) |
| **R11**, **SC5** | Multi-session `backlogit mcp status`, graceful drain + retained-handle termination on `backlogit mcp stop` / `--replace`, and deterministic idle auto-shutdown (`15m` default when `active_sessions == 0`, `D3`) | `U5` (`211.005-T`), `U6` (`211.006-T`), `U7` (`211.007-T`) |
| **R12**, **SC5** | One-shot CLI coexistence via `.backlogit/.locks/` + SQLite WAL `BEGIN IMMEDIATE` + `busy_timeout(30000)`, and preservation of `bin/backlogit.exe` in `.mcp.json` and `start.ps1` without blocking stash `17214531` | `U2` (`211.002-T`), `U5` (`211.005-T`), `U11` (`213.001-T`) |
| **R13**, **SC2**, **SC6** | Confirmed Option A independent per-workspace background daemon lifecycle surviving any client exit (including the first client) | `U5` (`211.005-T`), `U6` (`211.006-T`) |
| **R14**, **SC6a**, **SC6b**, **SC6c** | Dual startup entry routes: idempotent `backlogit mcp prewarm` (for `start.ps1`) and lazy first-instantiation auto-start on `backlogit mcp`, converging concurrent cold starts to one daemon | `U5` (`211.005-T`), `U6` (`211.006-T`), `U7` (`211.007-T`) |
| **R15**, **SC7a**, **SC7b**, **SC8** | Trusted workspace resolution, pre-attach workspace/storage/binary/version handshake verification (`VerifyMCPDaemonAttachment`), explicit mismatch refusal, and zero silent global `PATH` fallback | `U1` (`211.001-T`), `U5` (`211.005-T`), `U12` (`213.002-T`) |
| **R16**, **SC7a** | Workspace-local daemon state confinement (`<storage_root>/runtime/`) and platform IPC namespace distinction (POSIX `0600` UDS with `sun_path` fallback vs Windows Named Pipe `\\.\pipe\backlogit-mcp-<hexHash16>` with current-user SID `DACL`, `D2`) | `U4` (`211.004-T`), `U5` (`211.005-T`) |
| **R17**, **SC9** | Workspace-scoped application containment of `backlogit`-managed tools, resources, and helpers (`*os.Root`, traversal/symlink/reparse refusal, zero implicit `PATH` helper lookup) | `U3` (`211.003-T`), `U5` (`211.005-T`), `U9` (`212.002-T`), `U12` (`213.002-T`) |
| **R18**, **SC14** | Explicit security threat-model boundary distinguishing Level 1 workspace-local pinning and `backlogit` `*os.Root` path containment from deferred Level 2 OS/container sandboxing and external policy authorities (`D7`) | `U8` (`212.001-T`), `U10` (`212.003-T`), `U12` (`213.002-T`) |
| **R19**, **SC10**, **SC11**, **SC12** | Workspace-owned agent/MCP tool environment across **all** workspace tools (`backlogit`, `engram`, `graphtor-docs`, `context7`, `tavily`, `github`, `bun`/`bunx`, `.mcp.json`, `start.ps1`, `start.sh`) with explicit inventory and `host-prerequisite` declarations (`D9`) | `U8` (`212.001-T`), `U9` (`212.002-T`), `U11` (`213.001-T`), `U12` (`213.002-T`) |
| **R20**, **SC12**, **SC13** | Separation of local installation/pinning from runtime process/network confinement; honest structured reporting (`workspace-local-pinned`, `host-prerequisite`, `remote-declared`, `built-in-host`, `unsupported`, `non-conformant`) | `U8` (`212.001-T`), `U9` (`212.002-T`), `U11` (`213.001-T`), `U12` (`213.002-T`) |
| **R21**, **SC13** | Remote service access policy separation (`remote-declared` services bound to workspace config without false local-binary claims or building new cloud integrations) | `U8` (`212.001-T`), `U9` (`212.002-T`), `U11` (`213.001-T`) |
| **R22** | Multi-track capability structure across Track A (`212-F` / `213-S`), Track B (`211-F` / `212-S`), and Track C (`213-F` / `214-S`) with explicit dependency edges | `U1`–`U13` (`212-S`, `213-S`, `214-S`) |
| **R23**, **SC10**–**SC14** | Confirmed Level 1 initial-release containment strength (`workspace-local installation, configuration, and version pinning`) with local identity/pinning negative tests and OS/container sandboxing deferred | `U8`–`U13` (`213-S`, `214-S`) |

## Decisions and Rationale (`D1`–`D9` Resolved)

1. **`D1` — FIFO Go Channel Coordinator Parameters, Synchronous-Frame Turn Ownership, and External Writer Boundary (`R7`–`R9`, `R12`)**:
   * *Decision*: Implement `OperationCoordinator` in `internal/mcp/coordinator.go` with `DefaultMutationQueueCapacity = 32`, `DefaultMutationAdmissionTimeout = 30 * time.Second` (matching SQLite `busy_timeout(30000)`), `DefaultMaxConcurrentReads = 4` (matching `db.Open` `SetMaxOpenConns(4)` in `internal/db/connection.go:75`), and `DefaultReadAdmissionTimeout = 15 * time.Second`.
   * *Synchronous-frame turn handoff (`R9`, `timeout-goroutine-lock-ownership-go-2026-07-01.md`)*: Each `OpMutating` call enqueues a `*mutationWaiter` carrying an unbuffered rendezvous channel `grant chan chan struct{}` and cancellation signal `abandoned chan struct{}` into the bounded `mutationQueue` (`cap = 32`). When the sequential coordinator loop reaches the waiter, it selects between `waiter.grant <- done` and `<-waiter.abandoned`. The caller waits in its **synchronous execution frame** for `releaseTurn := <-waiter.grant` vs `<-ctx.Done()` vs `<-admissionTimer.C`, registers `defer close(releaseTurn)` in that synchronous frame, and executes the tool handler directly in the caller goroutine (never spawning a timeout goroutine that could strand locks). `(*OperationCoordinator).Close()` signals shutdown by closing an internal `closed chan struct{}` (never closing `mutationQueue` directly, avoiding `send on closed channel` panics).
    * *Exclusive consistency barrier (`R8`)*: `OpExclusiveBarrier` (`backlogit_sync_index`) first acquires the FIFO mutation turn and then acquires all `4` tokens from `readTokens` (tracking `acquiredCount` and releasing all `acquiredCount` tokens via synchronous-frame `defer` even if `ctx.Done()` or admission timeout aborts after acquiring 1–3 tokens) before running `Rehydrate`. Similarly, when `s.Workspace == nil` on entry to a tool call in `internal/mcp/server.go`, lazy workspace initialization (`s.ensureWorkspace`) is gated under `s.workspaceMu` (and `s.backlogitDir()` locks `s.workspaceMu`) before handler dispatch so `s.Workspace`, `s.templateSvc`, `s.Events`, `s.Telemetry`, and `s.HookEvents` never race with concurrent readers. Because only the active mutation holder ever acquires all 4 read tokens and readers never acquire the mutation turn, lock ordering is strictly `MutationTurn -> ReadTokens` (deadlock-free).
   * *External writer boundary (`R7`, `R12`)*: `OperationCoordinator` schedules `backlogit` runtime-managed MCP operations only. One-shot CLI commands and external editors continue to coordinate through `.backlogit/.locks/` advisory file locks (`shipment-lifecycle-global -> A -> C -> B`) and SQLite WAL `BEGIN IMMEDIATE` + `busy_timeout(30000)`.
2. **`D2` — Deterministic Platform IPC Endpoint Naming and POSIX `sun_path` Length Fallback (`R16`, `A4`)**:
   * *Decision*: Compute `hexHash16 = hex(sha256(canonicalStorageRoot)[:8])` (where `canonicalStorageRoot` is `EvalSymlinks`-resolved, cleaned absolute `storageRoot`, lowercased on Windows).
   * *Windows (`go:build windows`)*: Named Pipe endpoint `\\.\pipe\backlogit-mcp-<hexHash16>`, created via `golang.org/x/sys/windows` `CreateNamedPipe` with `PIPE_ACCESS_DUPLEX | FILE_FLAG_OVERLAPPED`, `PIPE_REJECT_REMOTE_CLIENTS`, and a security descriptor (`SECURITY_ATTRIBUTES`) whose DACL grants access exclusively to the current process owner SID (`TOKEN_USER`, SDDL `D:P(A;;GA;;;<UserSID>)`).
   * *POSIX (`go:build !windows`)*: Primary Unix Domain Socket path `<storage_root>/runtime/mcp.sock` with mode `0600`. When `len(primaryPath) > 100` (approaching the 104/108-byte `sockaddr_un.sun_path` limit), fall back to `<tempOrXdgRuntimeDir>/backlogit-<uid>/mcp-<hexHash16>.sock` inside a `0700` directory verified via `Lstat` to be a real directory owned by `os.Getuid()`, while keeping `mcp-daemon.lock` and `mcp-daemon.json` inside `<storage_root>/runtime/`.
3. **`D3` — Idle Auto-Shutdown Grace Period and Prewarm Idle Window (`R11`, `R13`, `R14`)**:
   * *Decision*: Set `DefaultDaemonIdleTimeout = 15 * time.Minute` (overridable via `--idle-timeout` or `BACKLOGIT_MCP_IDLE_TIMEOUT`). The timer starts at daemon boot (covering unattached `backlogit mcp prewarm` runs) and stops whenever `active_sessions > 0`. When the last client session disconnects (`active_sessions` drops to `0`), the timer resets to `idleTimeout`; when it expires with `active_sessions == 0`, the daemon closes the IPC listener, removes `mcp-daemon.json`, releases `mcp-daemon.lock`, closes `Workspace`, and exits `0`.
4. **`D4` — Binary Version-Skew and Wrong-Workspace Refusal Recovery UX (`R15`, `SC7`, `SC8`)**:
   * *Decision*: `VerifyMCPDaemonAttachment` checks both `mcp-daemon.json` and the IPC handshake (`backlogit-mcp-ipc/v1`). Wrong-workspace/storage mismatches return `corerrors.ErrDaemonWorkspaceMismatch` (`retryable: false`) and never replace a foreign daemon. Binary path or version/commit mismatches return `corerrors.ErrDaemonVersionMismatch` with the daemon PID, running vs expected version/binary path, `active_sessions`, and explicit CLI remediation (`backlogit mcp stop` or `backlogit mcp --replace`), replacing only when `--replace` is explicitly passed. Missing workspace-pinned binary returns `corerrors.ErrDaemonBinaryMissing` with zero global `PATH` fallback.
5. **`D5` — Non-Exclusive Status and Handshake Probing (`R3`, `A045DBA4`)**:
   * *Decision*: `InspectMCPDaemonStatus`, `StopMCPDaemon` (when no daemon is running), and shim connect probes first perform a lock-free `runtimeRoot.Lstat(mcpDaemonLockName)` + `rejectLink` check (preserving `TestU2_RejectsSymlinkOrReparseRuntimeDir/lock_leaf_symlink`), then read `"mcp-daemon.json"` via `readMCPDaemonInfo(runtimeRoot)` (no-follow read handle, **zero lock on `mcp-daemon.lock`**) and check `isVerifiedProcessAlive(info.PID, info.ProcessStartTime, info.ExecutablePath)` plus a non-disruptive `500ms` IPC `"status"` ping (when `info.Endpoint != ""`). If `"mcp-daemon.json"` is absent (`os.ErrNotExist`), `InspectMCPDaemonStatus` returns `Running: false` without locking `mcp-daemon.lock`. Only when `"mcp-daemon.json"` exists AND `isVerifiedProcessAlive` proves the recorded process is dead does `InspectMCPDaemonStatus` acquire `openMCPDaemonLockFile` to remove the stale sidecar under lock.
6. **`D6` — `start.ps1` Prewarm Command Contract and CLI Surface (`R11`, `R14`, `A5`)**:
   * *Decision*: Add `backlogit mcp daemon [--replace] [--idle-timeout <dur>]` and `backlogit mcp prewarm [--replace] [--idle-timeout <dur>] [--format text|json]` under `backlogit mcp`, plus `--toolchain` and `--strict` flags on `backlogit mcp status` (preserving `newMCPCommand.Short` and `newMCPStatusCommand.Short` so `cmd/gen-docs` updates only the target CLI reference files). Register `backlogit mcp daemon` and `backlogit mcp prewarm` in `cliOnlyIntentional` (`internal/cli/registry_parity_test.go`) and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference`.
7. **`D7` — External Policy Authority Boundary Note for Future Hostile-Agent / OS-Sandbox Phase (`R18`, `R23`, `A7`)**:
   * *Decision*: Document in `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` and `ToolchainVerificationReport` (`containment_level: "level-1-workspace-local-pinned"`, `os_container_sandbox_deferred: true`) that workspace-writable files (`.mcp.json`, `start.ps1`, `.backlogit/toolchain.yaml`) guard against accidental `PATH` drift and cross-workspace bleed, whereas defending against a hostile local process running under the same OS user requires an out-of-workspace policy authority and Level 2 OS/container sandboxing (deferred per `Q3`).
8. **`D8` — SDK (`mcp-go v0.27.0`) Progress-Notification Verification (`R10`, `A3`)**:
   * *Decision*: Verified in `github.com/mark3labs/mcp-go v0.27.0` (`mcp.CallToolRequest.Params.Meta.ProgressToken` and `MCPServer.SendNotificationToClient`). When `req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil`, `OperationCoordinator` emits standard `notifications/progress` messages (`progressToken`, `progress`, `total`, `message`) to the calling session's `NotificationChannel()` on queue wait, execution start, and completion without changing synchronous `tools/call` response semantics.
9. **`D9` — Workspace Tool-Environment Manifest Schema, Pinning Mechanics, and Honest Reporting (`R19`–`R23`, `SC10`–`SC14`)**:
   * *Decision*: Define `schemas/workspace-toolchain.schema.json` and `internal/toolchain` supporting optional `.backlogit/toolchain.yaml` (or `.backlog/toolchain.yaml`) and automatic discovery over `.mcp.json`, `start.ps1`, `start.sh`, `bin/`, and `tools/mcp/`. Every tool is classified into a closed enum (`workspace-local-pinned`, `host-prerequisite`, `remote-declared`, `built-in-host`, `unsupported`, `non-conformant`) and audited without mutating pre-existing dirty workspace files (`start.ps1`, `tools/`) or external upstream tool repositories.

## Constraints and Harness-Exempt Contract

* **2-Hour Rule and Width Isolation**: Every implementation unit (`U1`–`U13`) touches **at most 3 files**, **at most 4 functions**, **at most 3 test scenarios**, and a **single skill domain** (`code`, `tests`, or `docs`).
* **No hardcoded `.backlogit` literals in Go packages**: Per `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` (`internal/core/workspace_literal_guard_test.go`), `internal/cli`, `internal/mcp`, and `internal/toolchain` MUST resolve workspace storage roots via `core.ResolveStorageRoot`, `core.WorkspaceStorageRoot`, and `core.WorkspaceRootCandidates()`.
* **P-002.6 Intra-Shipment Task Dependency Rule (`deps(t) ⊆ M`)**: Every task-to-task `blocks` dependency edge (`item_deps` on a task `t`) targets a task inside the **same** shipment's task set `M`. Cross-shipment dependencies are recorded as shipment-to-shipment `blocks` edges (`214-S` blocked by `212-S` and `213-S` via `AddShipmentBlock`) and feature-to-feature `blocks` edges (`213-F` blocked by `211-F` and `212-F`).
* **Closed P-002.1 `harness-exempt` set across this plan**:
  * Shipment `212-S` (`211-F`, Track B): `["211.007-T"]` (`U7`, `docs-only` CLI reference regeneration). All other tasks (`211.001-T`..`211.006-T`) are `harness-ready`.
  * Shipment `213-S` (`212-F`, Track A): `["212.003-T"]` (`U10`, `docs-only` architecture & design documentation). All other tasks (`212.001-T`..`212.002-T`) are `harness-ready`.
  * Shipment `214-S` (`213-F`, Track C): `["213.003-T"]` (`U13`, `docs-only` CLI reference + configuration documentation). All other tasks (`213.001-T`..`213.002-T`) are `harness-ready`.

## Implementation Units

### Track B — Feature `211-F` / Shipment `212-S`: `Backlogit Multi-Session Singleton MCP Runtime, Stdio Session Shim, and Channel Operation Coordinator`

#### U1: Declare Multi-Session MCP Coordinator Types, Daemon Sentinels, and Source-Shape Harness (`code`) — `211.001-T`

* **Domain**: `code` (declaration + `go/ast` source-shape harness per P-002.1 cycle-31: harness FIRST, declarations SECOND).
* **Depends on**: None (Wave 1 of `212-S`).
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` with zero production files modified)**: Create `internal/mcp/multi_session_shape_test.go` with `TestU1_MultiSessionCoordinatorSourceShape` using `go/ast`, `go/parser`, and `go/token`. Assert:
    1. `internal/errors/errors.go` declares exported sentinels `ErrMCPMutationQueueFull`, `ErrMCPAdmissionTimeout`, `ErrDaemonWorkspaceMismatch`, `ErrDaemonVersionMismatch`, and `ErrDaemonBinaryMissing`.
    2. `internal/mcp/coordinator.go` declares exported types `OperationKind`, `CoordinatorConfig`, `BusyErrorResponse` (with JSON tags `error`, `code`, `message`, `retryable`, `remediation`), and `OperationCoordinator`.
    3. `internal/mcp/coordinator.go` declares exported functions/methods with exact signatures `NewOperationCoordinator(cfg CoordinatorConfig) *OperationCoordinator`, `ClassifyToolOperation(toolName string, args map[string]any) OperationKind`, `(*OperationCoordinator).Execute(ctx context.Context, srv *mcpserver.MCPServer, toolName string, req mcplib.CallToolRequest, next mcpserver.ToolHandlerFunc) (*mcplib.CallToolResult, error)`, and `(*OperationCoordinator).Close() error` without freezing temporary stub bodies (`docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`).
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Add the 5 exported sentinel errors to `internal/errors/errors.go`.
    2. Create `internal/mcp/coordinator.go` declaring `OperationKind` (`OpReadOnly`, `OpMutating`, `OpExclusiveBarrier`), `DefaultMutationQueueCapacity = 32`, `DefaultMutationAdmissionTimeout = 30 * time.Second`, `DefaultMaxConcurrentReads = 4`, `DefaultReadAdmissionTimeout = 15 * time.Second`, `CoordinatorConfig`, `BusyErrorResponse` (canonical `Error: "server_busy"` and closed `Code` values `"mutation_queue_full"`, `"mutation_admission_timeout"`, `"read_admission_timeout"`, `"request_cancelled"`), `OperationCoordinator`, and the 4 functions returning `corerrors.ErrNotImplemented` / initial defaults.
* **Affected files (`3` files)**:
  * `internal/mcp/multi_session_shape_test.go` (new)
  * `internal/errors/errors.go`
  * `internal/mcp/coordinator.go` (new)
* **Acceptance criteria**:
  * `TestU1_MultiSessionCoordinatorSourceShape` compiles and is observed RED before `internal/errors/errors.go` and `internal/mcp/coordinator.go` are updated, then turns GREEN in the implementation phase.
  * `go test -timeout=30m -count=1 -run '^TestU1_' ./internal/mcp` passes with `0` failures.
* **Execution posture**: `test-first` (P-002.1 `go/ast` source-shape harness).

#### U2: Implement FIFO Go Channel Mutation Coordinator, Bounded Read Limiter, Exclusive Barrier, and Progress Notifications (`code`) — `211.002-T`

* **Domain**: `code`.
* **Depends on**: `U1` (`211.001-T`).
* **Changes**:
  * In `internal/mcp/coordinator.go`:
    1. Implement `ClassifyToolOperation(toolName string, args map[string]any) OperationKind`:
       * Returns `OpExclusiveBarrier` for `backlogit_sync_index`.
       * Returns `OpReadOnly` for the 18 always-read-only tools in `R8` plus `backlogit_list_templates`, `backlogit_get_wit_metadata`, `backlogit_list_types`, `backlogit_poll_hook_events`, `backlogit_custody_preview`, and `backlogit_custody_verify`, as well as `backlogit_doctor` when `fix_orphans != true`, `backlogit_docs_migrate` when `apply != true`, and `backlogit_merge_sync` / `backlogit_remove_from_shipment` / `backlogit_move_shipment_item` / `backlogit_reorder_shipment` / `backlogit_reconstitute_shipment` / `backlogit_custody_offer` / `backlogit_custody_accept` when `dry_run == true`.
       * Returns `OpMutating` for all other tools (and when `fix_orphans == true`, `apply == true`, or `dry_run == false` on the conditional tools above).
    2. Implement `NewOperationCoordinator`, `(*OperationCoordinator).Execute`, and `(*OperationCoordinator).Close`:
       * `(*OperationCoordinator).Close()` signals shutdown by closing an internal `closed chan struct{}` (never closing `mutationQueue` directly, preventing `send on closed channel` panics on concurrent `Execute` calls).
       * `OpMutating` enqueues a `*mutationWaiter` (`grant chan chan struct{}`, `abandoned chan struct{}`) onto bounded `mutationQueue` (`cap = 32`). If `mutationQueue` is full (or `<-c.closed` is signaled), immediately returns a structured `BusyErrorResponse` (`Error: "server_busy"`, `Code: "mutation_queue_full"`, `Retryable: true`) wrapped via `mcplib.NewToolResultError` without mutating state.
       * Waits in the caller's synchronous frame for `releaseTurn := <-waiter.grant` vs `<-ctx.Done()` vs `<-admissionTimer.C` vs `<-c.closed`. On timeout or context cancel while queued, closes `waiter.abandoned`, drains any racing grant (`select { case r := <-waiter.grant: close(r) default: }`), and returns `BusyErrorResponse` (`Error: "server_busy"`, `Code: "mutation_admission_timeout"` or `"request_cancelled"`, `Retryable: true`).
       * On turn grant, binds `defer close(releaseTurn)` to the caller's synchronous frame (`timeout-goroutine-lock-ownership-go-2026-07-01.md`). When `kind == OpExclusiveBarrier`, acquires all `MaxConcurrentReads` (`4`) tokens from `readTokens`, tracking `acquiredCount` and registering a synchronous-frame `defer` that returns all `acquiredCount` tokens to `readTokens` even if `ctx.Done()` or `ReadAdmissionTimeout` aborts after acquiring 1–3 tokens, before invoking `next(ctx, req)`.
       * When `req.Params.Meta != nil && req.Params.Meta.ProgressToken != nil && srv != nil`, emits capability-gated `notifications/progress` updates (`queued`, `started`, `completed`) to the calling session via `srv.SendNotificationToClient(ctx, "notifications/progress", ...)` (`R10`, `D8`).
       * `OpReadOnly` acquires 1 token from `readTokens` in the caller's synchronous frame (racing `ctx.Done()` and `ReadAdmissionTimeout`, returning `BusyErrorResponse{Error: "server_busy", Code: "read_admission_timeout" | "request_cancelled", Retryable: true}` on timeout/cancel, and releasing via synchronous `defer`) and executes `next(ctx, req)` concurrently.
  * In `internal/mcp/server.go`:
    1. Add `coordinator *OperationCoordinator` to `Server`, initialize in `newServer`, and route `addTool` handlers and `InvokeTool` through `s.coordinator.Execute`.
    2. Enforce the lazy-workspace-initialization consistency barrier (`R8`) inside `internal/mcp/server.go` without touching `dynamic.go`, `hook_tools.go`, or `metadata.go`: at the `addTool` / `InvokeTool` dispatch wrapper (before `OpReadOnly` handlers run), if `s.Workspace == nil` and `dirExists(s.backlogitDir())`, invoke `s.ensureWorkspace(ctx)` (which holds `s.workspaceMu` while initializing `s.Workspace` and rebinding `s.templateSvc`, `s.Events`, `s.Telemetry`, and `s.HookEvents`), and guard `s.backlogitDir()` with `s.workspaceMu` so no reader ever races with lazy workspace initialization.
* **Affected files (`3` files)**:
  * `internal/mcp/coordinator.go`
  * `internal/mcp/server.go`
  * `internal/mcp/coordinator_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU2_CoordinatorFIFOMutationOrderAndConcurrentReads`): 8 concurrent mutating tool calls execute strictly in FIFO admission order with zero lost writes or interleaved JSONL events (`SC3`), 4 concurrent read-only calls run in parallel without blocking behind slow mutations, and `backlogit_sync_index` (`OpExclusiveBarrier`) waits for in-flight readers to drain before rehydrating.
  * Scenario 2 (`TestU2_CoordinatorQueueFullAndCallerCancellationSafety`): saturating `mutationQueue` or cancelling a queued caller's `ctx` returns structured `BusyErrorResponse` (`retryable: true`) with zero partial mutations and immediately hands the turn to the next queued caller (`SC4`).
  * Scenario 3 (`TestU2_CoordinatorProgressNotificationsIsolatedToCallerSession`): a gated mutation with `_meta.progressToken` delivers `notifications/progress` only to the calling session's `NotificationChannel()` (`R10`, `D8`).
  * `go test -timeout=30m -count=1 -race -run '^TestU2_' ./internal/mcp` passes.
* **Execution posture**: `test-first`.

#### U3: Implement Multi-Session Stream Dispatcher, Per-Session `ClientSession` Lifecycle, Request ID Isolation, and Per-Session Cancellation (`code`) — `211.003-T`

* **Domain**: `code`.
* **Depends on**: `U2` (`211.002-T`).
* **Changes**:
  * Create `internal/mcp/session_mux.go` implementing:
    1. `MultiSessionConn` (implementing `mcpserver.ClientSession` with unique `sessionID`, `initialized atomic.Bool`, buffered `notifications chan mcplib.JSONRPCNotification`, mutex-guarded JSON-RPC line writer, and per-session `inFlight map[string]context.CancelFunc` keyed via a typed canonical request-ID helper `canonicalRequestIDKey(any) string` using explicit type switches / `json.Number` / `string` / `int64` rather than `fmt.Sprintf("%v", id)`).
    2. `(s *Server) ServeSessionStream(ctx context.Context, sessionID string, r io.Reader, w io.Writer) error`: registers `MultiSessionConn` via `s.mcp.RegisterSession(ctx, sess)`, runs the per-session notification writer goroutine draining `sess.NotificationChannel()`, reads newline-delimited JSON-RPC frames from `r` via `bufio.Reader` (`ReadBytes('\n')` with a `4 MiB` max frame guard, avoiding `bufio.Scanner`'s 64 KiB `ErrTooLong` limit), intercepts `notifications/cancelled` (`params.requestId`) to cancel only that session's matching in-flight request context, dispatches JSON-RPC requests concurrently within the session using `s.mcp.WithContext(reqCtx, sess)` -> `s.mcp.HandleMessage` tracked by a per-session `sync.WaitGroup`, and on stream EOF or context cancel unregisters only `sessionID` (`s.mcp.UnregisterSession`), cancels that session's remaining in-flight contexts, waits on the session `sync.WaitGroup` for all in-flight request goroutines and the notification writer goroutine to drain before returning (ensuring zero writes to `w` after return), and decrements `activeSessions`.
    3. `(s *Server) ActiveSessionCount() int` and `(s *Server) CloseAllSessions() error` to support `backlogit mcp status` and graceful daemon drain on `stop` / `--replace`.
* **Affected files (`2` files)**:
  * `internal/mcp/session_mux.go` (new)
  * `internal/mcp/session_mux_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU3_ConcurrentSessionsOverlappingRequestIDsAndDisconnectSurvival`): 3 concurrent client streams connected to a single `*Server` send overlapping JSON-RPC `"id": 1` and `"id": 2` (`initialize`, `tools/list`, `tools/call`); every response arrives on the exact originating client stream with zero cross-session interleaving (`R4`, `R5`, `SC1`, `SC2`); closing Client 1's stream leaves `ActiveSessionCount() == 2` and Clients 2 & 3 executing read and write tool calls without error (`R13`, `SC2`).
  * Scenario 2 (`TestU3_PerSessionNotificationAndCancellationIsolation`): Client 1 and Client 2 both have an in-flight `"id": 1` request; Client 1 sends `notifications/cancelled` for `requestId: 1`; only Client 1's request context is cancelled while Client 2's `"id": 1` request succeeds (`R6`, `SC2`).
  * Scenario 3 (`TestU3_WorkspaceScopedToolAndResourceContainment`): verifies that sessions bound to `s.RootPath` reject `..` traversal, external paths, and symlink/reparse escapes across MCP tool and resource calls (`R17`, `SC9`).
  * `go test -timeout=30m -count=1 -race -run '^TestU3_' ./internal/mcp` passes.
* **Execution posture**: `test-first`.

#### U4: Implement Cross-Platform Local IPC Listener and Dialer (Windows Named Pipe Current-User DACL and POSIX `0600` UDS with `sun_path` Fallback) (`code`) — `211.004-T`

* **Domain**: `code`.
* **Depends on**: `U3` (`211.003-T`).
* **Changes**:
  * Create `internal/cli/mcp_ipc_windows.go` (`//go:build windows`) and `internal/cli/mcp_ipc_unix.go` (`//go:build !windows`) implementing:
    1. `DeriveMCPDaemonEndpoint(workspaceRoot, storageRoot, runtimeDir string) (transport, endpoint string, err error)`:
       * Computes `hexHash16` from `sha256` of the canonical `storageRoot` path (lowercased on Windows).
       * On Windows: returns `("named_pipe", `\\.\pipe\backlogit-mcp-<hexHash16>`, nil)`.
       * On POSIX: returns `("unix", filepath.Join(runtimeDir, "mcp.sock"), nil)` when `len <= 100`, or falls back to `<tempOrXdgDir>/backlogit-<uid>/mcp-<hexHash16>.sock` inside a verified `0700` user-owned directory when `len > 100` (`R16`, `D2`).
    2. `ListenMCPDaemonIPC(ctx context.Context, runtimeRoot *os.Root, transport, endpoint string) (net.Listener, error)`:
       * On Windows: creates a multi-instance Windows Named Pipe listener via `golang.org/x/sys/windows` `CreateNamedPipe` (`PIPE_ACCESS_DUPLEX`, `PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT`, `PIPE_UNLIMITED_INSTANCES`, `PIPE_REJECT_REMOTE_CLIENTS`) with a current-user SID DACL (`TOKEN_USER` SDDL `D:P(A;;GA;;;<UserSID>)`), using either explicit `windows.Overlapped` event handles on `ConnectNamedPipe`/`ReadFile`/`WriteFile` or synchronous per-instance pipe handles woken on `Listener.Close()` via a loopback dial so `os.NewFile` reads/writes never fail with `ERROR_INVALID_PARAMETER`.
       * On POSIX: removes any stale socket file via `runtimeRoot.Remove("mcp.sock")` (or verified fallback dir), binds `net.Listen("unix", endpoint)`, and enforces `0600` owner-only socket permissions.
    3. `DialMCPDaemonIPC(ctx context.Context, transport, endpoint string, timeout time.Duration) (net.Conn, error)`: connects a client shim or status/stop probe to the local IPC endpoint within `timeout` (retrying `windows.ERROR_PIPE_BUSY` via `windows.WaitNamedPipe` up to `timeout` on Windows).
    4. `isVerifiedProcessAlive(pid int, startTime int64, exePath string) bool`: performs a non-destructive read-only OS process identity check (`processStartTime(pid) == startTime` and executable path match) without opening or locking `mcp-daemon.lock` (`R3`, `D5`).
* **Affected files (`3` files)**:
  * `internal/cli/mcp_ipc_windows.go` (new)
  * `internal/cli/mcp_ipc_unix.go` (new)
  * `internal/cli/mcp_ipc_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU4_DeriveMCPDaemonEndpointDeterministicAndWorkspaceIsolated`): verifies deterministic per-workspace endpoint derivation, distinct endpoints for distinct workspaces (`SC7a`), and POSIX `sun_path` >100-byte fallback behavior (`R16`, `D2`).
  * Scenario 2 (`TestU4_ListenAndDialMCPDaemonIPCMultiClient`): opens `ListenMCPDaemonIPC`, connects 4 concurrent clients via `DialMCPDaemonIPC`, exchanges framed JSON lines concurrently, and closes cleanly.
  * Scenario 3 (`TestU4_IsVerifiedProcessAliveChecksIdentity`): verifies `isVerifiedProcessAlive` returns `true` for the current process with matching `ProcessStartTime` and `ExecutablePath`, and `false` on mismatched `ProcessStartTime`, mismatched `ExecutablePath`, or dead PID (`D5`).
  * `go test -timeout=30m -count=1 -race -run '^TestU4_' ./internal/cli` passes.
* **Execution posture**: `test-first`.

#### U5: Implement Background Daemon Lifecycle, Non-Exclusive Probing, Dual Startup Entry Routes (`prewarm` + Lazy Auto-Start), Pre-Attach Handshake, and Stdio Shim (`code`) — `211.005-T`

* **Domain**: `code`.
* **Depends on**: `U4` (`211.004-T`).
* **Changes**:
  * In `internal/cli/mcp_daemon.go`:
    1. Extend `MCPDaemonInfo` and `MCPDaemonStatus` with `IPCTransport string` (`json:"ipc_transport,omitempty"`), `Endpoint string` (`json:"endpoint,omitempty"`), `ActiveSessions int` (`json:"active_sessions"`), and `ProtocolVersion string` (`json:"protocol_version,omitempty"`).
    2. Update `InspectMCPDaemonStatus` (and `StopMCPDaemon`'s initial not-running check) to perform **non-exclusive probing first** (`R3`, `D5`, `A045DBA4`): first perform a lock-free `runtimeRoot.Lstat(mcpDaemonLockName)` + `rejectLink` check (preserving `TestU2_RejectsSymlinkOrReparseRuntimeDir/lock_leaf_symlink`), then read `"mcp-daemon.json"` via `readMCPDaemonInfo(runtimeRoot)` without locking `mcp-daemon.lock`. If `"mcp-daemon.json"` is absent (`os.ErrNotExist`), return `Running: false` immediately without locking `mcp-daemon.lock`. If present and `isVerifiedProcessAlive` is `true`, query live `ActiveSessions` over a `500ms` IPC `"status"` control frame when `info.Endpoint != ""` (tolerating empty `info.Endpoint` from lock-only test holders in `TestU3_`/`TestU4_`) and return `Running: true` without touching `mcp-daemon.lock`. Only when `"mcp-daemon.json"` exists on disk and `isVerifiedProcessAlive` is `false` (crashed holder), acquire `openMCPDaemonLockFile` to remove the stale sidecar under lock. In `StopMCPDaemon` (and `--replace`), when a verified daemon with a non-empty `Endpoint` is running, send a bounded (`500ms`) IPC `"stop"` control frame to trigger graceful `s.CloseAllSessions()` + `s.coordinator.Close()` drain before falling back to `TerminateVerifiedProcess` (`R11`).
    3. Implement `RunMCPDaemon(ctx context.Context, rootPath string, replace bool, idleTimeout time.Duration) error`, `PrewarmMCPDaemon(ctx context.Context, rootPath string, replace bool, idleTimeout time.Duration) (MCPDaemonStatus, error)`, and `RunMCPSessionShim(ctx context.Context, rootPath string, replace bool, stdin io.Reader, stdout io.Writer) error`:
       * In `RunMCPDaemon`: on each accepted `net.Conn`, read the initial `backlogit-mcp-ipc/v1` control header line via a `*bufio.Reader`. Answer `"status"` and `"stop"` control probes directly in `mcp_daemon.go` without calling `s.ServeSessionStream` or incrementing `ActiveSessionCount()`. For verified `"session"` connections, pass that same `*bufio.Reader` (`r`) and `conn` (`w`) into `s.ServeSessionStream(ctx, sessionID, reader, conn)` so pipelined `initialize` bytes buffered after the handshake newline are never lost.
       * Enforces trusted workspace resolution and verifies the workspace-pinned `bin/backlogit.exe` executable (failing closed with `corerrors.ErrDaemonBinaryMissing` and zero global `PATH` fallback if a configured workspace-local binary is missing, `R15`, `SC8`).
       * `VerifyMCPDaemonAttachment` checks canonical `WorkspaceRoot`, `StorageRoot`, `ExecutablePath`, and `Version` before attaching, returning `corerrors.ErrDaemonWorkspaceMismatch` or `corerrors.ErrDaemonVersionMismatch` on mismatch without killing active peer sessions (`R15`, `SC7b`, `D4`).
       * Converges concurrent cold-start callers (`RunMCPSessionShim` and `PrewarmMCPDaemon`) to a single background daemon instance via `AcquireMCPDaemonLock` + non-exclusive readiness polling (`R14`, `SC6a`–`SC6c`).
       * Implements deterministic idle auto-shutdown (`DefaultDaemonIdleTimeout = 15 * time.Minute` when `active_sessions == 0`, `R11`, `D3`).
  * In `internal/cli/root.go`:
    * Add `newMCPDaemonCommand(cwd *string)` (`backlogit mcp daemon [--replace] [--idle-timeout <dur>]`) and `newMCPPrewarmCommand(cwd *string)` (`backlogit mcp prewarm [--replace] [--idle-timeout <dur>] [--format text|json]`) under `newMCPCommand` (keeping `newMCPCommand.Short` and `newMCPStatusCommand.Short` unchanged so `cmd/gen-docs` only touches the expected CLI reference files in `U7` and `U13`, and updating `newMCPStatusCommand` `--format text` to include `active_sessions` and `endpoint` per `R11`).
    * Default `var serveMCPStdio func(*mcpinternal.Server) error` to `nil` so `newMCPCommand` runs `RunMCPSessionShim` when `serveMCPStdio == nil`, and runs the legacy `AcquireMCPDaemonLock` + `serveMCPStdio(s)` path when `u3StubServe` sets `serveMCPStdio != nil` in existing `TestU3_*` unit tests.
* **Affected files (`3` files)**:
  * `internal/cli/mcp_daemon.go`
  * `internal/cli/root.go`
  * `internal/cli/mcp_daemon_multisession_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU5_PrewarmAndLazyAutoStartColdRaceConvergence`): verifies `PrewarmMCPDaemon` idempotency + multi-client attach (`SC6a`), lazy auto-start on first `RunMCPSessionShim` + first-client disconnect survival (`SC6b`, `R13`), and 4 simultaneous cold starts (with a pre-existing stale `mcp-daemon.json`) converging to 1 daemon PID with zero `ErrDaemonAlreadyRunning` errors (`SC6c`, `R14`).
  * Scenario 2 (`TestU5_PreAttachRefusesWrongWorkspaceVersionSkewAndMissingPinnedBinary`): verifies cross-workspace refusal (`ErrDaemonWorkspaceMismatch`, `SC7a`), version/binary-skew refusal without killing active sessions (`ErrDaemonVersionMismatch`, `SC7b`), and missing `bin/backlogit.exe` fail-closed refusal (`ErrDaemonBinaryMissing`, `SC8`).
  * Scenario 3 (`TestU5_NonExclusiveStatusProbeAndIdleAutoShutdown`): verifies `InspectMCPDaemonStatus` never acquires `mcp-daemon.lock` during live or cold status checks (`R3`, `D5`), reports `active_sessions: 2 -> 1 -> 0` (`SC5`), and auto-exits after `idleTimeout` when `active_sessions == 0` (`R11`, `D3`).
  * `go test -timeout=30m -count=1 -race -run '^(TestU5_|TestU1_|TestU2_|TestU3_|TestOpenMCPServer_)' ./internal/cli` passes.
* **Execution posture**: `test-first`.

#### U6: Update CLI-Only Registry Parity Allowlist and Cross-Process Multi-Session Subprocess Integration Suite (`tests`) — `211.006-T`

* **Domain**: `tests`.
* **Depends on**: `U5` (`211.005-T`).
* **Changes**:
  * In `internal/cli/registry_parity_test.go`, add `"backlogit mcp daemon"` and `"backlogit mcp prewarm"` to `cliOnlyIntentional`, and add an assertion in `TestRegistryParity_DiscoverabilityConsistency` that every live `cliPaths` command with prefix `"backlogit mcp"` is present in `cliOnlyIntentional`.
  * In `internal/cli/mcp_daemon_subprocess_test.go`, add `TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival`: spawns two real `backlogit mcp` child subprocesses against a shared temp workspace, executes concurrent `initialize`, `tools/list`, and mutating + read-only `tools/call` requests with overlapping `"id": 1`, terminates Client 1's subprocess, verifies via `InspectMCPDaemonStatus` and Client 2's subsequent `tools/call` that the background daemon and Client 2 remain healthy, and stops the daemon via `StopMCPDaemon`.
* **Affected files (`2` files)**:
  * `internal/cli/registry_parity_test.go`
  * `internal/cli/mcp_daemon_subprocess_test.go`
* **Acceptance criteria**:
  * `TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival`, `TestU4_CrossProcessMCPDaemonLockAndStop`, and `TestRegistryParity_DiscoverabilityConsistency` pass.
  * `go test -timeout=30m -count=1 -race -run '^(TestU6_|TestU4_CrossProcess|TestRegistryParity_)' ./internal/cli` passes.
* **Execution posture**: `test-first`.

#### U7: Regenerate CLI Reference Documentation for `backlogit mcp`, `backlogit mcp daemon`, and `backlogit mcp prewarm` (`docs`) — `211.007-T`

* **Domain**: `docs` (`harness-exempt: docs-only`).
* **Depends on**: `U6` (`211.006-T`).
* **Changes**:
  * Run `go run ./cmd/gen-docs docs/cli-reference` from the repository root to update `docs/cli-reference/backlogit_mcp.md` and generate `docs/cli-reference/backlogit_mcp_daemon.md` and `docs/cli-reference/backlogit_mcp_prewarm.md`.
  * Verify `go run ./cmd/backlogit docs lint --path docs/cli-reference` reports `valid: true` with `0` violations.
* **Affected files (`3` files)**:
  * `docs/cli-reference/backlogit_mcp.md`
  * `docs/cli-reference/backlogit_mcp_daemon.md` (new generated file)
  * `docs/cli-reference/backlogit_mcp_prewarm.md` (new generated file)
* **Harness Exemption Contract**:

<!-- BEGIN:harness-exemption-contract -->
```text
harness_exemption_class: docs-only
harness_exemption_reason: Regenerates born-compliant Cobra CLI reference markdown files under docs/cli-reference/ with zero Go production or test code changes
harness_owner: none
exempt_verification_command: if (-not (Test-Path -LiteralPath 'docs/cli-reference/backlogit_mcp_daemon.md') -or -not (Test-Path -LiteralPath 'docs/cli-reference/backlogit_mcp_prewarm.md')) { exit 1 }; if (-not (Select-String -Path 'docs/cli-reference/backlogit_mcp.md' -SimpleMatch 'backlogit_mcp_daemon.md' -Quiet) -or -not (Select-String -Path 'docs/cli-reference/backlogit_mcp.md' -SimpleMatch 'backlogit_mcp_prewarm.md' -Quiet) -or -not (Select-String -Path 'docs/cli-reference/backlogit_mcp_daemon.md' -SimpleMatch '## backlogit mcp daemon' -Quiet) -or -not (Select-String -Path 'docs/cli-reference/backlogit_mcp_prewarm.md' -SimpleMatch '## backlogit mcp prewarm' -Quiet)) { exit 1 }; go run ./cmd/backlogit docs lint --path docs/cli-reference; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'EXEMPT_VERIFY_OK:211.007-T'; exit 0
exempt_precondition: must-fail-before-deliverable
```
<!-- END:harness-exemption-contract -->

* **Acceptance criteria**:
  * `docs/cli-reference/backlogit_mcp_daemon.md` and `docs/cli-reference/backlogit_mcp_prewarm.md` exist, are linked from `docs/cli-reference/backlogit_mcp.md`, and conform to P-008 (`make md-lint` / MD001, MD025, MD041).
  * Running `go run ./cmd/gen-docs docs/cli-reference` followed by `git diff --exit-code docs/cli-reference/` exits `0` (zero CLI reference drift).
  * `exempt_verification_command` fails before `U7` runs and exits `0` printing `EXEMPT_VERIFY_OK:211.007-T` after `U7` completes.
* **Execution posture**: `characterization-first` (`harness-exempt: docs-only`).

---

### Track A — Feature `212-F` / Shipment `213-S`: `Workspace Tool-Environment Inventory, Level 1 Version-Pinning Schema, and Threat-Model Contract`

#### U8: Declare Workspace Toolchain Inventory Schema, Level 1 Containment Types, and Source-Shape Harness (`code`) — `212.001-T`

* **Domain**: `code` (declaration + `go/ast` source-shape harness per P-002.1 cycle-31: harness FIRST, declarations SECOND).
* **Depends on**: None (Wave 1 of `213-S`).
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect` with zero production files modified)**: Create `internal/toolchain/toolchain_shape_test.go` declaring `package toolchain` (so `go test -run=^$ -count=1 ./...` compiles cleanly in the RED phase before `internal/toolchain/types.go` exists) with `TestU8_ToolchainDeclarationsSourceShape` using `go/ast`, `go/parser`, and `go/token`. Assert:
    1. `schemas/workspace-toolchain.schema.json` exists, parses as valid JSON Schema Draft 2020-12, and defines closed enum values for `containment_class` (`workspace-local-pinned`, `host-prerequisite`, `remote-declared`, `built-in-host`, `unsupported`, `non-conformant`) and `containment_level` (`level-1-workspace-local-pinned`).
    2. `internal/toolchain/types.go` declares structs with exact JSON/YAML tags:
       * `ToolchainManifest` (`schema_version`, `containment_level`, `os_container_sandbox_deferred`, `workspace_root`, `tools`, `host_prerequisites`),
       * `ToolEntry` (`id`, `kind`, `containment_class`, `executable_path`, `entrypoint_path`, `pinned_version`, `sha256`, `lockfile_path`, `workspace_binding_args`, `remote_policy`, `reason`),
       * `HostPrerequisiteEntry` (`id`, `command`, `purpose`, `required`, `reason`),
       * `ToolVerificationFinding` (`tool_id`, `containment_class`, `status`, `code`, `message`, `remediation`),
       * `ToolchainVerificationReport` (`valid`, `containment_level`, `os_container_sandbox_deferred`, `workspace_root`, `tools`, `host_prerequisites`, `violations`).
    3. `internal/toolchain/types.go` declares exported functions with exact signatures `LoadToolchainManifest(rootPath string) (*ToolchainManifest, error)`, `DefaultWorkspaceToolchainManifest(rootPath string) *ToolchainManifest`, `VerifyWorkspaceToolchain(ctx context.Context, rootPath string, manifest *ToolchainManifest) (*ToolchainVerificationReport, error)`, and `ValidateMCPConfigPinning(rootPath string, mcpJSONPath string, manifest *ToolchainManifest) ([]ToolVerificationFinding, error)` without freezing stub bodies.
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Create `schemas/workspace-toolchain.schema.json`.
    2. Create `internal/toolchain/types.go` declaring the `ToolContainmentClass` constants, the 5 structs (initializing slice fields as non-nil `[]` in constructors/builders so JSON output never emits `null`), and the 4 exported functions returning `corerrors.ErrNotImplemented` / initial defaults.
* **Affected files (`3` files)**:
  * `internal/toolchain/toolchain_shape_test.go` (new)
  * `schemas/workspace-toolchain.schema.json` (new)
  * `internal/toolchain/types.go` (new)
* **Acceptance criteria**:
  * `TestU8_ToolchainDeclarationsSourceShape` compiles and is observed RED before `schemas/workspace-toolchain.schema.json` and `internal/toolchain/types.go` exist, then turns GREEN once they are created.
  * `go test -timeout=30m -count=1 -run '^TestU8_' ./internal/toolchain` passes.
* **Execution posture**: `test-first` (P-002.1 `go/ast` source-shape harness).

#### U9: Implement Level 1 Toolchain Manifest Loader, Local Binary/Config/Version-Pinning Validator, and Honest Prerequisite/Remote/Unsupported Reporter (`code`) — `212.002-T`

* **Domain**: `code`.
* **Depends on**: `U8` (`212.001-T`).
* **Changes**:
  * In `internal/toolchain/types.go` and `internal/toolchain/verify.go`:
    1. Implement `DefaultWorkspaceToolchainManifest(rootPath string)` and `LoadToolchainManifest(rootPath string)`: reads optional `<storage_root>/toolchain.yaml` via `*os.Root` no-follow handle with duplicate-YAML-key rejection (`docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md`), or synthesizes the default workspace tool inventory covering `backlogit`, companion MCP servers (`engram`, `graphtor-docs`, `context7`, `tavily`, `github`), CLI helpers (`bun`, `bunx`, `.copilot/graphtor-mcp-shim.cjs`), `host-prerequisite` entries (`pwsh`, `bash`, `git`, `gh`, `go`, `copilot`), and `built-in-host` tools.
    2. Implement `VerifyWorkspaceToolchain(ctx context.Context, rootPath string, manifest *ToolchainManifest) (*ToolchainVerificationReport, error)`:
       * Sets `ContainmentLevel: "level-1-workspace-local-pinned"` and `OSContainerSandboxDeferred: true` (`R18`, `R23`).
       * For `workspace-local-pinned` entries: verifies workspace-relative path confinement inside `rootPath` via `os.Lstat` + `!core.IsSymlinkOrReparsePoint` (rejecting bare global `PATH` names, `..` traversal, absolute external paths, cross-workspace paths, and symlinks/junctions to external files), file existence, non-empty `PinnedVersion` or lockfile provenance, optional SHA-256 match, and workspace-scoped binding (`--cwd`, `COPILOT_HOME`, `ENGRAM_DATA_DIR`, `SC8`–`SC12`).
       * Emits explicit, structured `ToolVerificationFinding` records for `host-prerequisite`, `remote-declared`, `built-in-host`, `unsupported`, and `non-conformant` entries rather than silently counting them as workspace-contained (`R20`, `R21`, `SC13`).
    3. Implement `ValidateMCPConfigPinning(rootPath string, mcpJSONPath string, manifest *ToolchainManifest) ([]ToolVerificationFinding, error)`: parses `.mcp.json`, verifies 100% inventory accounting (`SC11`, flagging any uninventoried shadow server with `Code: "uninventoried_shadow_tool"`), verifies `backlogit` points to `./bin/backlogit.exe` (`R12`), and flags bare global `PATH` commands or cross-workspace paths (`SC12`, `SC13`).
* **Affected files (`3` files)**:
  * `internal/toolchain/types.go`
  * `internal/toolchain/verify.go` (new)
  * `internal/toolchain/verify_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU9_VerifyWorkspaceToolchainAcceptsPinnedLocalToolsAndReportsPrereqsAndRemote`): verifies a conformant Level 1 workspace passes with `Violations == 0` while explicitly reporting `host-prerequisite`, `remote-declared`, and `built-in-host` entries (`R19`–`R21`, `R23`, `SC11`, `SC13`).
  * Scenario 2 (`TestU9_VerifyWorkspaceToolchainRejectsGlobalPATHShadowToolsAndCrossWorkspacePaths`): verifies missing `./bin/backlogit.exe`, bare global `PATH` commands, cross-workspace binary/config references (`Workspace A` -> `Workspace B`), and uninventoried `.mcp.json` shadow tools fail closed (`SC8`, `SC10`, `SC12`, `SC14`).
  * Scenario 3 (`TestU9_VerifyWorkspaceToolchainRejectsSymlinkAndReparseEscapes`): verifies symlinks and Windows junctions inside `bin/` or `tools/` targeting outside the workspace are deterministically refused (`R17`, `SC9`, `SC14`).
  * `go test -timeout=30m -count=1 -race -run '^TestU9_' ./internal/toolchain` passes.
* **Execution posture**: `test-first`.

#### U10: Author Design Documentation for Multi-Session Singleton MCP Runtime and Level 1 Workspace Tool-Environment Contract (`docs`) — `212.003-T`

* **Domain**: `docs` (`harness-exempt: docs-only`).
* **Depends on**: `U9` (`212.002-T`).
* **Changes**:
  * Create `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` (`doc_type: design`, `schema_version: "1.0"`) documenting the Option A multi-session daemon + stdio shim topology, `OperationCoordinator` FIFO channel gating and `.backlogit/.locks/` + SQLite WAL coexistence boundary (`R7`, `R12`), Level 1 workspace tool-environment contract (`schemas/workspace-toolchain.schema.json`), and the explicit Threat-Model Boundary (`R18`, `R20`, `R23`, `D7`, `D9`).
  * Update `docs/ARCHITECTURE.md` to document `internal/toolchain` and the multi-session `internal/mcp` coordinator/session-mux architecture, including the explicit `## Dependency Direction` rules (`cli -> toolchain`, `toolchain -> core, errors`, and the cross-cutting rule that `core`, `mcp`, `db`, `models`, and `errors` must never import `internal/toolchain`), ensuring P-008 (`make md-lint` / MD001, MD025, MD041) heading conformance.
* **Affected files (`2` files)**:
  * `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` (new)
  * `docs/ARCHITECTURE.md`
* **Harness Exemption Contract**:

<!-- BEGIN:harness-exemption-contract -->
```text
harness_exemption_class: docs-only
harness_exemption_reason: Authors architecture and design documentation under docs/design-docs/ and docs/ARCHITECTURE.md with zero Go production or test code changes
harness_owner: none
exempt_verification_command: if (-not (Test-Path -LiteralPath 'docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md')) { exit 1 }; if (-not (Select-String -Path 'docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md' -SimpleMatch 'level-1-workspace-local-pinned' -Quiet) -or -not (Select-String -Path 'docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md' -SimpleMatch 'OperationCoordinator' -Quiet) -or -not (Select-String -Path 'docs/ARCHITECTURE.md' -SimpleMatch 'internal/toolchain' -Quiet)) { exit 1 }; go run ./cmd/backlogit docs lint --path docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; go run ./cmd/backlogit docs lint --path docs/ARCHITECTURE.md; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'EXEMPT_VERIFY_OK:212.003-T'; exit 0
exempt_precondition: must-fail-before-deliverable
```
<!-- END:harness-exemption-contract -->

* **Acceptance criteria**:
  * Both documentation files pass `go run ./cmd/backlogit docs lint` with `0` violations, and `exempt_verification_command` fails before `U10` and exits `0` printing `EXEMPT_VERIFY_OK:212.003-T` after `U10`.
* **Execution posture**: `characterization-first` (`harness-exempt: docs-only`).

---

### Track C — Feature `213-F` / Shipment `214-S` (Blocked by `212-S` and `213-S`): `Cross-Tool Workspace Pinning Verifier, Launcher/Companion MCP Alignment, and Multi-Workspace Negative Verification`

#### U11: Implement Launcher and Companion MCP Alignment Auditor and Wire `backlogit mcp status --toolchain [--strict]` (`code`) — `213.001-T`

* **Domain**: `code`.
* **Depends on**: None within `214-S` (`214-S` is blocked at the shipment level by `212-S` and `213-S`, and `213-F` is blocked by `211-F` and `212-F`).
* **Changes**:
  * Create `internal/toolchain/launcher.go` implementing:
    1. `VerifyLauncherScripts(rootPath string) ([]ToolVerificationFinding, error)`: inspects `.mcp.json`, `start.ps1`, and `start.sh` (read-only, never mutating pre-existing dirty files) to verify:
       * `.mcp.json` (`"command": "./bin/backlogit.exe"`) and `start.ps1` (`Join-Path $workspaceBinDirectory "backlogit.exe"`) both reference `bin/backlogit.exe` (`R12`, `SC5`);
       * Startup scripts enforce fail-closed local binary checks (`Test-Path` in `start.ps1`) and workspace-scoped state directories (`COPILOT_HOME`, `ENGRAM_DATA_DIR`, `SC10`, `SC12`);
       * Any launcher or `.mcp.json` entry that resolves a workspace tool via bare `command -v` / ambient `PATH` or references a binary missing from `./bin/` (e.g., `engram`, `bun`, `github-mcp-server.exe` bare names in `.mcp.json`, or `graphtor-docs` vs `bin/kbdb.exe`) is reported with an explicit `non-conformant` or `unsupported` finding and exact `./bin/...` remediation (`R20`, `SC11`–`SC13`).
    2. `AuditRepositoryToolchain(ctx context.Context, rootPath string) (*ToolchainVerificationReport, error)`: combines `LoadToolchainManifest`, `VerifyWorkspaceToolchain`, `ValidateMCPConfigPinning`, and `VerifyLauncherScripts` into a unified workspace toolchain report.
  * In `internal/cli/root.go`:
    * Add `--toolchain` (`"include workspace all-tools Level 1 pinning and containment audit"`) and `--strict` (`"return non-zero exit status when toolchain violations or non-conformant local tools are present"`; `--strict` implicitly enables `--toolchain`) flags to `newMCPStatusCommand` (`backlogit mcp status [--toolchain] [--strict] [--format text|json]`).
    * To keep `U11` strictly within its 3 declared files without modifying `internal/cli/mcp_daemon.go`, serialize an output composition struct `mcpStatusToolchainOutput` in `internal/cli/root.go` (embedding `MCPDaemonStatus` plus `Toolchain *toolchain.ToolchainVerificationReport `json:"toolchain,omitempty"``) and write the formatted JSON/text output to `cmd.OutOrStdout()` **before** returning a wrapped `corerrors.ErrValidation` when `--strict` is set and `report.Violations > 0`.
* **Affected files (`3` files)**:
  * `internal/toolchain/launcher.go` (new)
  * `internal/cli/root.go`
  * `internal/cli/mcp_toolchain_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU11_MCPStatusToolchainFlagReportsPinnedAndNonConformantCompanionTools`): `backlogit --cwd <tempRoot> mcp status --toolchain --format json` returns both daemon status and `toolchain` verification findings classifying `backlogit` (`workspace-local-pinned`), `pwsh`/`sh`/`git` (`host-prerequisite`), remote APIs (`remote-declared`), `built-in-host` tools, and any bare/missing companion MCP tool (`non-conformant` / `unsupported`, `SC11`, `SC13`).
  * Scenario 2 (`TestU11_VerifyLauncherScriptsChecksStartPs1AndMcpJsonBacklogitPinning`): verifies `VerifyLauncherScripts` confirms `bin/backlogit.exe` pinning in `.mcp.json` and `start.ps1` (`R12`, `SC5`) and flags missing-binary or global `PATH` fallbacks (`SC8`, `SC12`).
  * Scenario 3 (`TestU11_MCPStatusToolchainStrictExitsNonZeroOnViolations`): verifies `--toolchain --strict` returns a wrapped validation error when `Violations > 0` and exits `0` when all workspace-local tools are conformant.
  * `go test -timeout=30m -count=1 -race -run '^TestU11_' ./internal/cli ./internal/toolchain` passes.
* **Execution posture**: `test-first`.

#### U12: End-to-End Multi-Workspace All-Tools Isolation, Repository Inventory Audit, and Negative Pinning Verification Suite (`tests`) — `213.002-T`

* **Domain**: `tests`.
* **Depends on**: `U11` (`213.001-T`).
* **Changes**:
  * Create `internal/toolchain/repo_inventory_test.go` and `tests/integration/multi_session_workspace_containment_test.go`:
    1. `TestU12_RepositoryToolchainInventoryCompleteAndHonest` (`internal/toolchain/repo_inventory_test.go`, `SC11`, `SC12`, `SC13`): runs `AuditRepositoryToolchain` read-only against the repository root (without modifying dirty `start.ps1` or `tools/`), verifying every server in `.mcp.json` (`backlogit`, `engram`, `graphtor-docs`, `context7`, `tavily`, `github`) and every helper in `start.ps1`/`start.sh` is inventoried with zero shadow tools, `bin/backlogit.exe` is verified in both `.mcp.json` and `start.ps1`, and unpinned bare commands / missing binaries / host prerequisites / remote APIs are reported honestly with `containment_level: "level-1-workspace-local-pinned"` and `os_container_sandbox_deferred: true`.
    2. `TestU12_MultiWorkspaceAllToolsIsolationAndCrossAttachRefusal` (`tests/integration/multi_session_workspace_containment_test.go`, `SC7`, `SC10`, `SC14`): provisions two distinct workspaces (`Workspace A` and `Workspace B`) on the same host under the same user, runs simultaneous multi-session `backlogit` daemons and client sessions in both workspaces, verifies distinct IPC endpoints, distinct lock/sidecar files, and zero cross-workspace state bleed, and proves that cross-workspace daemon attach (`ErrDaemonWorkspaceMismatch`) and cross-workspace tool/config paths (`cross_workspace_path_refused`) are deterministically rejected.
    3. `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment` (`tests/integration/multi_session_workspace_containment_test.go`, `SC8`, `SC9`, `SC12`, `SC14`): proves missing `./bin/backlogit.exe` fails closed even when a fake `backlogit` binary is planted on `PATH`, and proves `..` traversal, absolute external path, and symlink/junction escape attempts across MCP tools and toolchain verification are deterministically refused.
* **Affected files (`2` files)**:
  * `internal/toolchain/repo_inventory_test.go` (new)
  * `tests/integration/multi_session_workspace_containment_test.go` (new)
* **Acceptance criteria**:
  * All 3 test scenarios (`TestU12_RepositoryToolchainInventoryCompleteAndHonest`, `TestU12_MultiWorkspaceAllToolsIsolationAndCrossAttachRefusal`, `TestU12_NegativePinningGlobalPATHRefusalAndPathTraversalContainment`) pass under `-race`.
  * `go test -timeout=30m -count=1 -race -run '^TestU12_' ./internal/toolchain ./tests/integration` passes.
* **Execution posture**: `test-first`.

#### U13: Regenerate `backlogit mcp status` CLI Reference and Document Level 1 Workspace Toolchain Configuration (`docs`) — `213.003-T`

* **Domain**: `docs` (`harness-exempt: docs-only`).
* **Depends on**: `U12` (`213.002-T`).
* **Changes**:
  * Run `go run ./cmd/gen-docs docs/cli-reference` to update `docs/cli-reference/backlogit_mcp_status.md` with `--toolchain` and `--strict`.
  * Update `docs/configuration.md` to document `.backlogit/toolchain.yaml` (`schemas/workspace-toolchain.schema.json`), Level 1 containment classes (`workspace-local-pinned`, `host-prerequisite`, `remote-declared`, `built-in-host`, `unsupported`, `non-conformant`), `backlogit mcp prewarm`, and `backlogit mcp status --toolchain [--strict]`.
* **Affected files (`2` files)**:
  * `docs/cli-reference/backlogit_mcp_status.md`
  * `docs/configuration.md`
* **Harness Exemption Contract**:

<!-- BEGIN:harness-exemption-contract -->
```text
harness_exemption_class: docs-only
harness_exemption_reason: Updates docs/cli-reference/backlogit_mcp_status.md via cmd/gen-docs and documents Level 1 toolchain configuration in docs/configuration.md with zero Go code changes
harness_owner: none
exempt_verification_command: if (-not (Select-String -Path 'docs/cli-reference/backlogit_mcp_status.md' -SimpleMatch -- '--toolchain' -Quiet) -or -not (Select-String -Path 'docs/cli-reference/backlogit_mcp_status.md' -SimpleMatch -- '--strict' -Quiet) -or -not (Select-String -Path 'docs/configuration.md' -SimpleMatch 'level-1-workspace-local-pinned' -Quiet)) { exit 1 }; go run ./cmd/backlogit docs lint --path docs/cli-reference/backlogit_mcp_status.md; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; go run ./cmd/backlogit docs lint --path docs/configuration.md; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'EXEMPT_VERIFY_OK:213.003-T'; exit 0
exempt_precondition: must-fail-before-deliverable
```
<!-- END:harness-exemption-contract -->

* **Acceptance criteria**:
  * `docs/cli-reference/backlogit_mcp_status.md` and `docs/configuration.md` pass `go run ./cmd/backlogit docs lint` with `0` violations, `git diff --exit-code docs/cli-reference/` after `go run ./cmd/gen-docs docs/cli-reference` is clean, and `exempt_verification_command` fails before `U13` and exits `0` printing `EXEMPT_VERIFY_OK:213.003-T` after `U13`.
* **Execution posture**: `characterization-first` (`harness-exempt: docs-only`).

## Dependency Graph (Recorded True-Prerequisite `blocks` Edges vs Proposed Execution Order)

### 1. Recorded True-Prerequisite `blocks` Edges (Persisted via `backlogit_add_dependency`)

* **Intra-Shipment Task DAG for Shipment `212-S` (`211-F`, Track B — 7 waves, 1 task per wave, `deps(t) ⊆ M_{212-S}`)**:
  * `211.001-T` (`U1`) -> `211.002-T` (`U2`) -> `211.003-T` (`U3`) -> `211.004-T` (`U4`) -> `211.005-T` (`U5`) -> `211.006-T` (`U6`) -> `211.007-T` (`U7`)
* **Intra-Shipment Task DAG for Shipment `213-S` (`212-F`, Track A — 3 waves, 1 task per wave, `deps(t) ⊆ M_{213-S}`)**:
  * `212.001-T` (`U8`) -> `212.002-T` (`U9`) -> `212.003-T` (`U10`)
* **Intra-Shipment Task DAG for Shipment `214-S` (`213-F`, Track C — 3 waves, 1 task per wave, `deps(t) ⊆ M_{214-S}`)**:
  * `213.001-T` (`U11`) -> `213.002-T` (`U12`) -> `213.003-T` (`U13`)
* **Cross-Track Feature and Shipment Prerequisite `blocks` Edges**:
  * Feature edges:
    * `213-F` (Track C) `depends_on` `211-F` (Track B) (`dep_type: "blocks"`)
    * `213-F` (Track C) `depends_on` `212-F` (Track A) (`dep_type: "blocks"`)
  * Shipment edges (via `AddShipmentBlock`):
    * `214-S` (Track C) `depends_on` `212-S` (Track B) (`dep_type: "blocks"`)
    * `214-S` (Track C) `depends_on` `213-S` (Track A) (`dep_type: "blocks"`)
  * Note on `212-S` (Track B) and `213-S` (Track A): Per `R22`, Track B and Track A have no true technical prerequisite edge between each other, so no artificial `blocks` edge is recorded between `212-S` and `213-S`.

### 2. Proposed P-001 Single-Active-Shipment Execution Order

```text
Shipment 212-S (211-F: Track B Multi-Session Runtime, U1..U7)   [Ready for claim once staged on main]
        │
        ├──► (Proposed serial P-001 order: ship 212-S first, then 213-S)
        ▼
Shipment 213-S (212-F: Track A Toolchain Contract, U8..U10)     [No blocks predecessor; ships after 212-S under P-001]
        │
        ├──► (Recorded true-prerequisite blocks edges: BOTH 212-S and 213-S must be shipped before 214-S can be claimed)
        ▼
Shipment 214-S (213-F: Track C Adapters & Negative Suite, U11..U13) [Blocked by 212-S and 213-S]
```

## Risks and Caveats

* **Risk 1: Live Workspace MCP Server Disruption During Testing or Runtime Verification**:
  * *Caveat*: The agent executing Ship for `212-S` / `213-S` / `214-S` communicates with the repository's own live `backlogit` MCP server. Calling `backlogit mcp stop` or `backlogit mcp --replace` against the repository root (`--cwd .`) would terminate the agent's active MCP session.
  * *Mitigation*: Every unit/integration test (`U4`–`U6`, `U11`–`U12`) and every Ship runtime verification probe MUST pass an isolated temporary workspace (`--cwd <tempDir>`).
* **Risk 2: Pre-Existing Dirty Worktree Files (`start.ps1`, `tools/`, `.autoharness/config.yaml`, `.gitignore`, `.backlogit/stash.jsonl` `D895887C`)**:
  * *Caveat*: `start.ps1` and `tools/` are pre-existing uncommitted files in the working tree.
  * *Mitigation*: None of `U1`–`U13` modifies `start.ps1`, `tools/`, `.autoharness/config.yaml`, or `.gitignore`. `U11` and `U12` inspect launcher and `.mcp.json` files strictly read-only and test modifications inside `t.TempDir()` fixtures. Any future operator adoption of dirty `start.ps1` or `tools/` is governed separately by `210-F` (`workflow-custody/v1` `Profile A`).
* **Risk 3: Known Release Caveat on Predecessor Shipment `211-S` (`210-F`)**:
  * *Caveat*: While `211-S` / `210-F` is shipped and its post-merge closure PR #642 merged to `main` (`5d47e8d4`), `docs/closure/211-S-210-F-post-merge-closure.md` had a shipment-specific operator exception for the closure-evidence gate. That exception does NOT transfer to `212-S`, `213-S`, or `214-S`. Furthermore, staged artifacts committed on a planning branch are NOT on remote `main` until an approved planning PR merges to `main` and Ship's pre-claim gate passes.

## Constitution Check

* **I. Safety-First Go**: `pass` — All production code in `internal/errors`, `internal/mcp`, `internal/cli`, and `internal/toolchain` targets Go 1.24.0, wraps errors with `fmt.Errorf("context: %w", err)`, uses typed sentinel errors, avoids `unsafe` and `panic()`, and passes `go vet ./...`, `golangci-lint run`, and `gofmt -l .`.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — `U1` (`211.001-T`) and `U8` (`212.001-T`, with `package toolchain` in `toolchain_shape_test.go` so the new package compiles in RED before `types.go` exists) use `go/ast` source-shape harnesses landed before declarations (`P-002.1` cycle-31); `U2`–`U6`, `U9`, and `U11`–`U12` use red-first unit and cross-process integration harnesses; `U7` (`211.007-T`), `U10` (`212.003-T`), and `U13` (`213.003-T`) are P-002.1-valid `docs-only` tasks with fail-before-deliverable read-only `exempt_verification_command` scripts emitting `EXEMPT_VERIFY_OK:<task_id>` and conforming to P-008 (`make md-lint`). Each TDD unit touches at most 3 files (1–2 tightly coupled implementation/schema files + 1 colocated `*_test.go` file, or at most 3 generated CLI reference files in `U7`).
* **III. Workspace Isolation and Security Boundaries**: `pass` — All `<storage_root>/runtime` lock/sidecar operations remain pinned to `*os.Root` (`openMCPDaemonRuntimeRoot`), `VerifyWorkspaceToolchain` rejects `..` traversal and symlinks/junctions via `os.Lstat` + `core.IsSymlinkOrReparsePoint`, Windows Named Pipes enforce current-user SID DACLs, POSIX sockets enforce `0600` permissions, and zero secrets or tokens are written to manifests or docs.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — All daemon state, locks, sidecars, and toolchain manifests reside inside the target workspace (`<storage_root>/runtime/` and `<storage_root>/toolchain.yaml`), and all tests execute inside `t.TempDir()`. On POSIX, `<storage_root>/runtime/mcp.sock` is the primary socket path, with the `>100`-byte `sockaddr_un.sun_path` kernel-limit fallback (`D2`) confined to a `0700` `os.Getuid()`-owned temp/XDG socket while `mcp-daemon.lock` and `mcp-daemon.json` remain strictly inside `<storage_root>/runtime/`.
* **V. Structured Observability**: `pass` — `backlogit mcp status [--toolchain] --format json`, `backlogit mcp prewarm --format json`, and `backlogit mcp stop --format json` emit structured JSON envelopes and `log/slog` diagnostics.
* **VI. Single Responsibility**: `pass` — Zero new third-party module dependencies are added to `go.mod`; Windows Named Pipes and POSIX UDS use existing `golang.org/x/sys` and standard library `net`.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — `StopMCPDaemon` and `--replace` only terminate a process handle verified to match the sidecar's `PID`, `ProcessStartTime`, `ExecutablePath`, and `WorkspaceRoot`. All `exempt_verification_command` scripts are strictly read-only (`P-002.5`).
* **VIII. Explicit Safety Modes for Elevated Risk**: `pass` — `Requires plan hardening: yes` is declared and satisfied by `## Plan Hardening` with explicit `strict-safety` `ProposedAction` / `ActionRisk` / `ActionResult` records.
* **IX. Git-Friendly Persistence**: `pass` — `<storage_root>/runtime/` remains ephemeral and gitignored; `.backlogit/toolchain.yaml` and `schemas/workspace-toolchain.schema.json` are human-readable YAML/JSON with deterministic key ordering.
* **X. Agent Context Efficiency**: `pass` — `OperationCoordinator` prevents head-of-line read blocking behind slow mutations and emits bounded JSON responses (`< 10 KiB`) with non-nil slices (`[]`).
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — Shipments `212-S`, `213-S`, and `214-S` will merge exclusively via merge commits (`--merge`, `P-009`).
* **Capability Overlays (`agent-engram`, `backlogit`, `graphtor-docs`, `strict-safety`)**: `pass` — Engram CLI was probed twice and degraded fallback declared per protocol (`TOOL_DEGRADED: engram-cli`); `graphtor-docs` was queried read-only (`get_status`, `list_sources`) without indexing the workspace; `backlogit` query/sync/dependency/shipment tools and `strict-safety` action classifications are used throughout.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — Adds `backlogit mcp daemon` and `backlogit mcp prewarm` CLI subcommands, adds `--toolchain` and `--strict` flags to `backlogit mcp status`, extends `MCPDaemonInfo` / `MCPDaemonStatus` JSON schema, adds `schemas/workspace-toolchain.schema.json`, and introduces structured `BusyErrorResponse` MCP error codes.
* **security, auth, permission, or compliance-sensitive behavior**: `present` — Enforces Windows Named Pipe current-user SID DACLs (`TOKEN_USER`), POSIX `0600` socket permissions, pre-attach workspace/binary/version handshake verification (`VerifyMCPDaemonAttachment`), and Level 1 workspace-local tool pinning + `*os.Root` path/reparse checks.
* **migration, backfill, destructive data/config action, or irreversible step**: `present` — `backlogit mcp stop` and `--replace` drain and terminate the verified workspace daemon process holding `mcp-daemon.lock`.
* **external integration, operator checkpoint, or external dependency**: `present` — Defines the cross-tool workspace inventory and Level 1 pinning verification contract across `backlogit`, `engram`, `graphtor-docs`, `context7`, `tavily`, `github-mcp-server`, `bun`, `.mcp.json`, `start.ps1`, and `start.sh`.
* **high runtime, rollout, or rollback risk**: `present` — Modifies the `backlogit mcp` startup and request-dispatch path used by all connected MCP agent sessions.

Requires plan hardening: yes

## Plan Hardening

### 1. Risk Triggers and Protected System-Wide Invariants

* **Invariant I1 (Synchronous-Frame Turn and Resource Ownership — `R9`)**: `OperationCoordinator.Execute` MUST never acquire a mutation turn, read token, or file lock inside a background timeout goroutine (`docs/compound/best-practices/timeout-goroutine-lock-ownership-go-2026-07-01.md`). Every granted mutation turn (`releaseTurn`) and read token MUST be owned in the caller's synchronous stack frame and released via `defer`. If `ctx.Done()` or `admissionTimer.C` fires while queued, the waiter closes `abandoned`, drains any racing grant (`select { case r := <-waiter.grant: close(r) default: }`), and returns `BusyErrorResponse` without executing the handler.
* **Invariant I2 (Deadlock-Free Reader/Writer and Exclusive Barrier Ordering — `R7`, `R8`)**: Normal mutations (`OpMutating`) acquire only the single FIFO mutation turn and allow up to `MaxConcurrentReads` (`4`) concurrent `OpReadOnly` calls against the WAL-mode SQLite pool. Exclusive barrier operations (`OpExclusiveBarrier`: `backlogit_sync_index`) acquire the FIFO mutation turn **first** and then drain all `4` tokens from `readTokens` before calling `db.Rehydrate`. Readers never acquire the mutation turn, so `MutationTurn -> ReadTokens` cannot cycle.
* **Invariant I3 (Non-Exclusive Status and Handshake Probing — `R3`, `D5`, `A045DBA4`)**: `InspectMCPDaemonStatus`, `PrewarmMCPDaemon`, and `RunMCPSessionShim` readiness checks MUST read `"mcp-daemon.json"` via `readMCPDaemonInfo(runtimeRoot)` and verify `isVerifiedProcessAlive` + IPC `"status"` ping **without** calling `tryLockFile` on `mcp-daemon.lock`. `openMCPDaemonLockFile` is called by `InspectMCPDaemonStatus` only when `"mcp-daemon.json"` exists on disk and `isVerifiedProcessAlive` confirms the recorded process is dead.
* **Invariant I4 (Per-Session Isolation and First-Client Disconnect Survival — `R4`–`R6`, `R13`)**: Each IPC connection registers a unique `MultiSessionConn` (`sessionID`) on `s.mcp`. Request IDs (`id`), `notifications/cancelled` cancellation contexts, and `NotificationChannel()` streams are scoped strictly per `sessionID`. Disconnecting any client session (including the first client that triggered lazy daemon auto-start) unregisters only that `sessionID` and never terminates the daemon while other sessions remain active or within `idleTimeout`.
* **Invariant I5 (Pre-Attach Identity Verification and Zero Silent Global `PATH` Fallback — `R15`, `SC7`, `SC8`, `SC12`)**: Before attaching a session or completing prewarm, the caller MUST verify canonical `WorkspaceRoot`, `StorageRoot`, `ExecutablePath`, and `Version`. Any workspace mismatch (`ErrDaemonWorkspaceMismatch`), version/binary mismatch without `--replace` (`ErrDaemonVersionMismatch`), or missing workspace-pinned binary (`ErrDaemonBinaryMissing`) MUST fail closed without adopting a foreign daemon, without killing active peer sessions, and without falling back to global `PATH`.
* **Invariant I6 (Handle-Pinned Runtime Directory and Same-User IPC Access Control — `R3`, `R16`)**: All `<storage_root>/runtime` file operations (`mcp-daemon.lock`, `mcp-daemon.json`, and POSIX primary `mcp.sock`) execute relative to the pinned `*os.Root` handle (`openMCPDaemonRuntimeRoot`). Windows Named Pipes (`\\.\pipe\backlogit-mcp-<hexHash16>`) enforce a current-user SID `DACL` and `PIPE_REJECT_REMOTE_CLIENTS`; POSIX sockets enforce `0600` mode (and a `0700` user-owned directory when the `>100`-byte `sun_path` fallback is used).
* **Invariant I7 (Honest Level 1 All-Tools Reporting and Threat-Model Boundary — `R18`–`R23`, `SC11`–`SC14`)**: `VerifyWorkspaceToolchain` and `AuditRepositoryToolchain` MUST account for every tool in `.mcp.json`, `start.ps1`, and `start.sh` (`SC11`), classify unavoidable bootstrap tools as `host-prerequisite`, classify approved network-backed MCP tools as `remote-declared`, flag bare-command or missing local binaries as `non-conformant` or `unsupported` (`SC13`), and record `containment_level: "level-1-workspace-local-pinned"` with `os_container_sandbox_deferred: true` (`R18`, `R23`).
* **Invariant I8 (Live Workspace MCP Server and Pre-Existing Dirty File Preservation)**: No test or verification command may run `StopMCPDaemon` or `--replace` against the live repository root (`C:\Source\GitHub\backlogit`), and no task in `212-S`, `213-S`, or `214-S` may overwrite or discard pre-existing dirty operator files (`start.ps1`, `tools/`, `.autoharness/config.yaml`, `.gitignore`, or stash `D895887C`).

### 2. Reinforced Instructions and Institutional Learnings Consulted

* `docs/compound/best-practices/timeout-goroutine-lock-ownership-go-2026-07-01.md` — Own admission turns, read tokens, and locks in the synchronous caller frame via `defer`, never inside an abandoned timeout goroutine (`U1`, `U2`).
* `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` — Keep `s.Events` as the single shared `*events.EventWriter` instance across all sessions on `Server` (`U2`, `U3`).
* `docs/compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` — Prevent head-of-line read starvation behind long-running mutations (`backlogit_ship_shipment`) by allowing bounded concurrent reads (`OpReadOnly`) alongside FIFO-gated writes (`OpMutating`) (`U2`).
* `docs/compound/2026-10-27-pin-the-handle-not-the-path.md` — Retain `*os.Root` handle-pinned operations on `<storage_root>/runtime` and verify `os.Lstat` + `!core.IsSymlinkOrReparsePoint` on all toolchain paths (`U4`, `U5`, `U9`).
* `docs/compound/2026-10-19-lock-held-side-effect-ordering.md` — Place `OperationCoordinator` outside `.backlogit/.locks/` (`MutationTurn -> ReadTokens` and `OperationCoordinator -> .backlogit/.locks/ -> SQLite BEGIN IMMEDIATE`) so lock acquisition orders never invert (`U2`).
* `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md` and `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md` — Use argv-array-only `exec.CommandContext` (never shell interpolation) and hard-capped handshake/status timeouts (`500ms`–`10s`) when spawning or probing `backlogit mcp daemon` (`U5`, `U9`).
* `docs/compound/2026-08-31-duplicate-yaml-keys-survive-safe-load-validation.md` — Reject duplicate YAML/JSON keys when loading `.backlogit/toolchain.yaml` and `.mcp.json` (`U9`, `U11`).
* `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md` — `U1` and `U8` `go/ast` source-shape harnesses assert permanent type, tag, and signature contracts rather than temporary `ErrNotImplemented` bodies.
* `docs/compound/2026-06-26-docline-frontmatter-contract.md` and `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — Regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` in the same shipment that modifies CLI commands/flags (`U7` in `212-S` and `U13` in `214-S`) and self-lint all docs via `go run ./cmd/backlogit docs lint`.
* `docs/compound/2026-07-20-manual-feature-harvest-provenance-backfill.md` — Harvest `ECF33C79` once via `backlogit_harvest_stash` into `211-F` so canonical `source_stash_*`, `source_deliberation_id: 082-DL`, and `.backlogit/archive/stash.jsonl` (`reason: "harvested"`, `harvested_artifact_id: "211-F"`) are populated without duplicate harvests.

### 3. Strict-Safety Risky Action Classifications (`strict-safety`)

| Unit | ProposedAction | ActionRisk | Approval Required | ActionResult | Safeguard & Rollback |
|---|---|---|---|---|---|
| `U1` (`211.001-T`) | Declare multi-session coordinator types and 5 daemon/queue error sentinels in `internal/errors/errors.go` and `internal/mcp/coordinator.go` (`change_kind: local edit`) | `low` | `false` | `planned` | Pure declarations + `go/ast` shape test; revert commit to roll back |
| `U2` (`211.002-T`) | Gate all `Server` MCP tool handlers through `OperationCoordinator` (`OpMutating` FIFO channel `cap=32`, `OpReadOnly` semaphore `cap=4`, `OpExclusiveBarrier` for `sync_index`, `change_kind: contract`) | `high` | `false` (pre-authorized by `082-DL` / `ECF33C79`) | `planned` | Synchronous-frame `defer` release, bounded 30s admission timeout, and race-detector unit suite (`TestU2_`); revert commit to roll back |
| `U3` (`211.003-T`) | Add `MultiSessionConn` and `ServeSessionStream` per-connection `ClientSession` multiplexer in `internal/mcp/session_mux.go` (`change_kind: local edit`) | `moderate` | `false` | `planned` | Existing `RunStdio` preserved; per-session `RegisterSession` / `UnregisterSession` tested with 3 concurrent streams in `TestU3_` |
| `U4` (`211.004-T`) | Create Windows Named Pipe (`\\.\pipe\backlogit-mcp-<hexHash16>`, current-user SID DACL) and POSIX `0600` UDS IPC listener/dialer in `internal/cli/mcp_ipc_*.go` (`change_kind: local edit`) | `high` | `false` (pre-authorized by `082-DL` `Q1`/`R16`) | `planned` | Current-user SID DACL + `PIPE_REJECT_REMOTE_CLIENTS` on Windows, `0600` socket inside `0700` dir on POSIX; tested in `t.TempDir()` |
| `U5` (`211.005-T`) | Evolve `backlogit mcp` into stdio-to-IPC session shim with auto-spawned `backlogit mcp daemon`, `backlogit mcp prewarm`, non-exclusive `InspectMCPDaemonStatus`, and `StopMCPDaemon` drain (`change_kind: rollout / destructive process stop on --replace`) | `high` | `false` (pre-authorized by `082-DL` `Q1`; runtime `--replace`/`stop` guarded by retained-handle identity verification) | `planned` | Retained-handle process identity check (`PID`, `ProcessStartTime`, `ExecutablePath`, `WorkspaceRoot`) prevents signaling unrelated processes; all tests use isolated `t.TempDir()` |
| `U6` (`211.006-T`) | Update `cliOnlyIntentional` in `internal/cli/registry_parity_test.go` and run cross-process multi-session subprocess tests in `internal/cli/mcp_daemon_subprocess_test.go` (`change_kind: local edit`) | `moderate` | `false` | `planned` | Child subprocesses bound to `t.TempDir()` with `t.Cleanup` termination |
| `U7` (`211.007-T`) | Regenerate `docs/cli-reference/backlogit_mcp*.md` via `go run ./cmd/gen-docs docs/cli-reference` (`change_kind: local edit`) | `low` | `false` | `planned` | Deterministic generator output verified by `backlogit docs lint` |
| `U8` (`212.001-T`) | Create `schemas/workspace-toolchain.schema.json` and `internal/toolchain/types.go` (`change_kind: local edit`) | `low` | `false` | `planned` | Schema + type declarations only; gated by `TestU8_` |
| `U9` (`212.002-T`) | Implement `VerifyWorkspaceToolchain` and `ValidateMCPConfigPinning` in `internal/toolchain/verify.go` (`change_kind: local edit`) | `moderate` | `false` | `planned` | Strictly read-only inspection using `*os.Root` and `os.Lstat` |
| `U10` (`212.003-T`) | Author `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` and update `docs/ARCHITECTURE.md` (`change_kind: local edit`) | `low` | `false` | `planned` | Docline-linted documentation only |
| `U11` (`213.001-T`) | Implement `VerifyLauncherScripts` / `AuditRepositoryToolchain` in `internal/toolchain/launcher.go` and add `--toolchain` / `--strict` flags to `backlogit mcp status` (`change_kind: contract`) | `moderate` | `false` | `planned` | Read-only inspection; preserves existing `backlogit mcp status` output when `--toolchain` is omitted |
| `U12` (`213.002-T`) | Add multi-workspace isolation and negative pinning integration tests in `internal/toolchain/repo_inventory_test.go` and `tests/integration/multi_session_workspace_containment_test.go` (`change_kind: local edit`) | `moderate` | `false` | `planned` | Runs in isolated `t.TempDir()` workspaces `Workspace A` and `Workspace B` |
| `U13` (`213.003-T`) | Regenerate `docs/cli-reference/backlogit_mcp_status.md` and update `docs/configuration.md` (`change_kind: local edit`) | `low` | `false` | `planned` | Verified by `cmd/gen-docs` diff check and `backlogit docs lint` |

### 4. Review-Gate Capability Risks and Fallback Declarations

* `plan-review` MUST probe subagent dispatch capability before spawning reviewers, record literal `dispatch_mode: multi-agent-dispatch` (or `dispatch_mode: single-agent-declared-degradation` with `TOOL_DEGRADED: reviewer-subagent-dispatch` if subagent dispatch is unavailable or any persona fails to return), cover all 7 applicable personas (Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Security Lens Reviewer), and record literal `decision: PASS` (or `ADVISORY` with `operator_authorization: approved`) in the final `## Plan Review` section before `harvest` is invoked.

## Runtime Verification and Closure

* **Changed Runtime Surfaces**:
  * **Shipment `212-S` (`211-F`, Track B)**: `internal/mcp` (`OperationCoordinator`, `MultiSessionConn`, `ServeSessionStream`) and `internal/cli` (`backlogit mcp` session shim, `backlogit mcp daemon`, `backlogit mcp prewarm`, non-exclusive `backlogit mcp status`, `backlogit mcp stop`, Windows Named Pipe & POSIX UDS IPC transport).
  * **Shipment `213-S` (`212-F`, Track A)**: `schemas/workspace-toolchain.schema.json` and `internal/toolchain` (`LoadToolchainManifest`, `VerifyWorkspaceToolchain`, `ValidateMCPConfigPinning`).
  * **Shipment `214-S` (`213-F`, Track C)**: `internal/toolchain/launcher.go` (`VerifyLauncherScripts`, `AuditRepositoryToolchain`) and `internal/cli` (`backlogit mcp status --toolchain [--strict]`).
* **Environment Prechecks and Runtime Verification Protocol (Per Shipment)**:
  1. **Mandatory isolated workspace rule**: Never invoke `backlogit mcp stop` or `backlogit mcp --replace` against `C:\Source\GitHub\backlogit` during Ship execution; always pass `--cwd <isolatedTempDir>`.
  2. **Shipment `212-S` (`211-F`) verification**:
     * Build `./bin/backlogit.exe` (in an isolated temp output path during verification) and run `backlogit --cwd <tempDir> mcp prewarm --format json`, two concurrent `backlogit --cwd <tempDir> mcp` stdio client sessions issuing overlapping `"id": 1` read/write tool calls, `backlogit --cwd <tempDir> mcp status --format json` (confirming `active_sessions: 2`), disconnect Client 1 (confirming `active_sessions: 1` and Client 2 health), and `backlogit --cwd <tempDir> mcp stop --format json` (confirming `stopped: true, running: false`).
     * Run `go test -timeout=30m -count=1 -race ./internal/mcp/... ./internal/cli/... ./internal/errors/...` and the full repository suite `go test -timeout=30m ./...`.
  3. **Shipment `213-S` (`212-F`) verification**:
     * Run `go test -timeout=30m -count=1 -race ./internal/toolchain/...`, `go run ./cmd/backlogit docs lint`, and `go test -timeout=30m ./...`.
  4. **Shipment `214-S` (`213-F`) verification**:
     * Run `backlogit --cwd <tempDir> mcp status --toolchain --format json` against both a fully pinned temp workspace (`Violations == 0`) and an unpinned/shadow-tool temp workspace (`Violations > 0`), plus `go test -timeout=30m -count=1 -race ./internal/toolchain/... ./internal/cli/... ./tests/integration/...` and `go test -timeout=30m ./...`.
* **Operational Closure Expectations**:
  * **Owner**: Ship agent (`_Ship`) for each shipment (`212-S`, `213-S`, `214-S`), supervised by Orchestrator (`_Orchestrator`).
  * **Validation window**: Immediate post-merge verification on `main` plus 7-day observation window across multi-agent sessions (`_Orchestrator`, `_Stage`, `_Ship`).
  * **Monitoring signals**:
    * `backlogit mcp status --format json` (`running`, `pid`, `active_sessions`, `ipc_transport`, `endpoint`);
    * Structured `BusyErrorResponse` telemetry (`mutation_queue_full`, `mutation_admission_timeout`) and zero `-32001` read timeouts during concurrent `backlogit_ship_shipment` / `backlogit_sync_index` execution;
    * `backlogit mcp status --toolchain --format json` (`violations`, per-tool `containment_class` and `status`);
    * `backlogit doctor` reporting `0` errors.
  * **Rollback triggers and procedure**:
    * *Trigger*: Spurious `ErrDaemonAlreadyRunning` on multi-agent startup, cross-session response/notification misrouting, deadlocked `OperationCoordinator` queue, or `CLI Reference Drift Check` / `Docline frontmatter gate` failure on `main`.
    * *Procedure*: Stop any affected temp/workspace daemon (`backlogit mcp stop`), revert the offending shipment merge commit on a reviewed PR via `git revert -m 1 <merge_sha>`, rebuild `bin/backlogit.exe`, and run `backlogit sync`. Because `mcp-daemon.json` and IPC endpoints are ephemeral under `<storage_root>/runtime/` and `.backlogit/toolchain.yaml` is additive, rollback requires zero database or Markdown migration.
  * **Closure artifacts**:
    * `docs/closure/212-S-211-F-post-merge-closure.md`
    * `docs/closure/213-S-212-F-post-merge-closure.md`
    * `docs/closure/214-S-213-F-post-merge-closure.md`
    * Each closure must invoke `compact-context` (`target: all`) per P-020 and satisfy the closure-evidence gate on its own merits (the shipment-specific operator exception on `211-S` does NOT transfer to `212-S`, `213-S`, or `214-S`).

## Plan Review

* dispatch_mode: multi-agent-dispatch
* decision: PASS
* attempt: 2
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer, Security Lens Reviewer (all 7 leaf personas dispatched and completed in Round 1; all 4 personas with Round 1 P2/P3 findings re-dispatched in Round 2 confirming 0 remaining findings).
* plan_hardening: `Requires plan hardening: yes` confirmed and satisfied by `## Plan Hardening` (Invariants `I1`–`I8`, 11 institutional learnings traces, and `strict-safety` `ProposedAction` / `ActionRisk` / `ActionResult` table across `U1`–`U13`).
* constitution_check: `Constitution Check: pass` confirmed; all Principles I–XI, capability overlays, and Task Granularity rules mapped and verified.
* findings:
  * Round 1 merged totals (before in-place remediation): `P0: 0`, `P1: 0`, `P2: 7` (deduplicated across Go Reviewer, Architecture Strategist, Agent-Native Parity Reviewer), `P3: 8` (across Constitution Reviewer, Go Reviewer, Architecture Strategist, Agent-Native Parity Reviewer).
  * Round 2 merged totals (after in-place remediation): `P0: 0`, `P1: 0`, `P2: 0`, `P3: 0`.
* round_1_remediation_log:
  * **P2-1 (`Architecture Strategist` + `Agent-Native Parity Reviewer`, `U11` `213.001-T`)**: Replaced editing `MCPDaemonStatus` in `internal/cli/mcp_daemon.go` with an output wrapper struct `mcpStatusToolchainOutput` in `internal/cli/root.go` (preserving `U11`'s 3-file cap and decoupling `mcp_daemon.go` from `internal/toolchain`), and specified writing the JSON/text report before returning the wrapped validation error under `--strict`.
  * **P2-2 (`Architecture Strategist`, `U2` `211.002-T`)**: Enforced the lazy-workspace-initialization barrier (`R8`) inside `internal/mcp/server.go` at the `addTool` / `InvokeTool` wrapper boundary and `s.backlogitDir()` under `s.workspaceMu`, avoiding edits to `dynamic.go`, `hook_tools.go`, or `metadata.go` while keeping `U2` within its 3-file cap.
  * **P2-3 (`Go Reviewer` + `Agent-Native Parity Reviewer`, `U1`/`U2` `211.001-T`/`211.002-T`)**: Pinned `BusyErrorResponse.Error` to `"server_busy"`, added `"read_admission_timeout"` to the closed `BusyErrorResponse.Code` set, specified a `closed chan struct{}` shutdown signal in `(*OperationCoordinator).Close()`, and required `OpExclusiveBarrier` to release any partially acquired `readTokens` via its synchronous-frame `defer` on timeout or cancellation.
  * **P2-4 (`Go Reviewer`, `U3` `211.003-T`)**: Specified `bufio.Reader` (`ReadBytes('\n')` with `4 MiB` max frame guard), a per-session `sync.WaitGroup` joining all request and notification goroutines before `ServeSessionStream` returns, and typed `canonicalRequestIDKey` normalization instead of `fmt.Sprintf("%v", id)`.
  * **P2-5 (`Go Reviewer`, `U4` `211.004-T`)**: Specified `CreateNamedPipe` handle/overlapped semantics and `windows.WaitNamedPipe` retry on `ERROR_PIPE_BUSY` in `DialMCPDaemonIPC`.
  * **P2-6 (`Go Reviewer` + `Architecture Strategist`, `U5` `211.005-T`)**: Retained lock-free `runtimeRoot.Lstat(mcpDaemonLockName)` + `rejectLink` in `InspectMCPDaemonStatus` (preserving `TestU2_RejectsSymlinkOrReparseRuntimeDir/lock_leaf_symlink`), defaulted `serveMCPStdio` to `nil` in `internal/cli/root.go` for `TestU3_*` compatibility, passed the handshake `*bufio.Reader` into `ServeSessionStream` so pipelined `initialize` bytes are preserved, and added non-exclusive initial check + bounded (`500ms`) IPC `"stop"` graceful drain to `StopMCPDaemon`.
  * **P3-1 through P3-8 (`Constitution Reviewer`, `Go Reviewer`, `Architecture Strategist`, `Agent-Native Parity Reviewer`)**: Added exact function signatures in `U1`/`U8`, exact struct JSON/YAML tag lists and `package toolchain` RED-phase declaration in `U8`, `"backlogit mcp"` prefix parity assertion in `U6`, `git diff --exit-code docs/cli-reference/` in `U7`, `## Dependency Direction` rules in `U10`, and P-008 (`make md-lint`) + 3-file TDD bound + POSIX `sun_path` fallback notes in `## Constitution Check`.
* runtime_verification_and_closure: Verified per-shipment runtime verification protocols (`--cwd <tempDir>` isolation preserving the live workspace MCP server), monitoring signals, rollback procedures, and post-merge closure expectations (`212-S`, `213-S`, `214-S` with P-020 `compact-context`).

