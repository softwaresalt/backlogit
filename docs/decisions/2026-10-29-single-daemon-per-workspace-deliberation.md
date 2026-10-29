---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash 760670A3: enforce a single backlogit MCP daemon per workspace using a CLI-only OS-handle advisory lock, JSON metadata sidecar, and CLI lifecycle subcommands (status, stop, --replace).'
doc_type: decision
docline:
  decision_status: decided
  depth: standard
  linked_artifacts:
    - docs/exec-plans/2026-10-29-single-daemon-per-workspace-plan.md
    - .backlogit/archive/077-DL.md
    - .backlogit/queue/204-F.md
    - .backlogit/queue/205-S.md
  promoted_to: plan
  stash_id: 760670A3
  tags:
    - mcp
    - daemon
    - file-lock
    - cli
    - workspace-isolation
    - 760670A3
    - 205-S
  topic: 'Enforce single backlogit MCP daemon per workspace via CLI-only OS advisory lock and lifecycle commands'
schema_version: "1.0"
source: docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md
title: 'Deliberation: Single backlogit MCP daemon per workspace (760670A3)'
---

## Problem Frame

* `backlogit mcp` (`internal/cli/root.go`: `newMCPCommand` -> `openMCPServer` -> `mcpinternal.RunStdio`) starts a long-lived Model Context Protocol stdio server bound to a workspace root (`--cwd`). During its lifetime, the server holds an open SQLite `*sql.DB` connection pool against `<storage_root>/backlogit.db`, an in-memory file manifest snapshot (`s.manifest`) used by `backlogit_merge_sync`, and workspace event, telemetry, and hook writers.
* Currently, nothing prevents multiple `backlogit mcp` processes from running simultaneously against the same workspace. When multiple editor windows, orphaned IDE sessions, or concurrent agent launches spawn duplicate `backlogit mcp` processes for the same workspace:
  * Both processes contend on `.backlogit/backlogit.db` (or `.backlog/backlogit.db`) and workspace lock files during writes and rehydration.
  * Each process maintains an independent in-memory `s.manifest` snapshot, causing stale incremental diffs when mutations happen in the sibling server.
  * Orphaned `backlogit mcp` processes accumulate in the background with no built-in CLI inspection or termination command.
* Short-lived one-shot CLI invocations (`backlogit list`, `backlogit sync`, `backlogit shipment ...`) open and close the workspace per command and exit immediately. They are not daemons and must remain runnable alongside the active `backlogit mcp` server.
* Stash entry `760670A3` (`high` / `feature`) requests a mechanism to ensure that only one `backlogit` daemon is ever running per workspace, explicitly preferring a **CLI-only mechanism** over a separate GUI window or detached background IPC daemon.

## Research Findings

* **Existing OS-level advisory lock primitive (`internal/cli/self_update_lock_*.go`)**:
  * `internal/cli` already contains a cross-platform non-blocking OS file lock primitive (`openSelfUpdateLockFile` in `self_update_lock_windows.go`, `self_update_lock_unix.go`, and `self_update_lock_other.go`).
  * On Windows, `syscall.CreateFile` with `dwShareMode = 0` acquires an exclusive kernel file handle where contention deterministically returns `ERROR_SHARING_VIOLATION` (`errno 32`) or `ERROR_LOCK_VIOLATION` (`errno 33`).
  * On Unix (`linux`, `darwin`, `freebsd`, `netbsd`, `openbsd`, `dragonfly`), `syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)` acquires a non-blocking exclusive kernel flock where contention deterministically returns `EWOULDBLOCK` or `EAGAIN`.
  * In both OS families, the kernel automatically releases the lock handle the instant the owning process terminates—including `SIGKILL`, `TerminateProcess`, or power loss—eliminating stale lock files after a crash.
* **Windows exclusive-handle read constraint (`dwShareMode = 0`)**:
  * On Windows, a file opened with `dwShareMode = 0` cannot be opened for `os.ReadFile` by a second process while the holder is alive. Storing holder metadata (`pid`, `started_at`, `workspace_root`, `storage_root`, `version`) in a companion JSON sidecar (`<storage_root>/runtime/mcp-daemon.json`) written while holding `<storage_root>/runtime/mcp-daemon.lock` allows any second process on both Windows and Unix to read the holder's metadata without a sharing violation, while the OS lock on `mcp-daemon.lock` remains the sole source of truth for whether a live holder exists.
* **Storage-root and `backlogit init` compatibility (`internal/core/workspace.go`, `internal/core/workspace_literal_guard_test.go`)**:
  * `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` forbids hardcoded legacy storage-root literals outside `internal/core/workspace.go`.
  * When a workspace is already initialized, `core.ResolveStorageRoot(rootPath)` returns the active storage root (`.backlog` or `.backlogit`), and `<storage_root>/runtime/` is already gitignored.
  * When `backlogit mcp` runs in an uninitialized directory (`errors.Is(err, os.ErrNotExist)`), `newInitCommand` rejects initialization if a legacy `.backlogit` directory exists on disk. Resolving the uninitialized runtime directory under `core.WorkspaceRootCandidates()[0]` (`<abs_root>/.backlog/runtime`) avoids creating a legacy directory and preserves `backlogit init` compatibility.
* **CLI Reference Drift Gate (`cmd/gen-docs/main.go`)**:
  * Per `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`, adding flags (`--replace`) or subcommands (`backlogit mcp status`, `backlogit mcp stop`) to `newMCPCommand` requires regenerating `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` so CI's `CLI Reference Drift Check` and `Docline frontmatter gate` stay clean.

## Options Evaluated

### Option A: CLI-Only OS-Lock Single-Instance Guard + Lifecycle CLI (`status`, `stop`, `--replace`) — Chosen

* **Description**:
  * Pin a validated runtime directory handle (`openMCPDaemonRuntimeRoot` via `os.OpenRoot` on validated `storageRoot` -> `runtimeRoot`), execute all `<storage_root>/runtime` leaf operations (`mcp-daemon.lock`, `mcp-daemon.json`, and temporary sidecars) as directory-relative operations through `runtimeRoot`, and acquire an exclusive OS-handle lock on `"mcp-daemon.lock"` relative to `runtimeRoot` (using a directory-relative no-follow/reparse-safe open and handle identity verification) when `backlogit mcp` runs, before opening the MCP server or starting the stdio loop, holding that lock handle for the lifetime of `mcpinternal.RunStdio`.
  * Immediately upon lock acquisition, remove any pre-existing stale `"mcp-daemon.json"` through `runtimeRoot`, then write an atomic JSON metadata sidecar at `"mcp-daemon.json"` relative to `runtimeRoot` containing `pid`, `process_start_time`, `executable_path`, `started_at` (RFC3339 UTC), `workspace_root`, `storage_root`, and `version`, and remove `"mcp-daemon.json"` through `runtimeRoot` on clean exit before releasing `mcp-daemon.lock`. Decode `"mcp-daemon.json"` directly from a no-follow validated `*os.File` handle opened on `runtimeRoot` (`json.NewDecoder(file)`), never via a separate path-based `os.ReadFile`.
  * When a second `backlogit mcp` starts for the same workspace without `--replace`, fail fast with sentinel error `corerrors.ErrDaemonAlreadyRunning`, reporting the active holder's PID and start time from `mcp-daemon.json` and remediation guidance (`backlogit mcp status`, `backlogit mcp stop`, or `backlogit mcp --replace`).
  * Provide `backlogit mcp status`, `backlogit mcp stop` (opening a single retained OS process handle for `pid`, verifying `process_start_time` and `executable_path` and re-verifying the sidecar on that retained handle, and terminating the holder **through that same retained handle** without a second PID lookup, failing closed on any mismatch, missing startup metadata, or unavailable identity-bound signaling), and `backlogit mcp --replace` so operators and automation can inspect, stop, or atomically replace an active workspace MCP daemon purely via the CLI.
* **Pros**:
  * Honors the stash preference for a pure CLI-only mechanism with zero background-daemon IPC or detached-process complexity.
  * Crash-safe by construction: OS kernel closes `mcp-daemon.lock` immediately on process death, so crashed instances never wedge subsequent `backlogit mcp` starts.
  * Works identically across Windows and Unix by separating the exclusive lock file (`mcp-daemon.lock`) from the readable metadata sidecar (`mcp-daemon.json`).
  * Small, self-contained blast radius in `internal/errors` and `internal/cli` plus regenerated CLI reference docs.
* **Cons**:
  * Does not multiplex multiple simultaneous stdio MCP clients into a single shared server process; instead, it enforces the single-daemon invariant and lets `--replace` or `stop` reclaim the workspace cleanly when an older client session is orphaned.
* **Effort estimate**: Low (5 atomic tasks across error sentinel, lock/lifecycle engine, CLI wiring, tests, and generated CLI reference docs).

### Option B: Detached Background Daemon with Named Pipe / Unix Domain Socket + Stdio Proxy

* **Description**:
  * Spawn a detached background daemon per workspace listening on a Windows named pipe (`\\.\pipe\backlogit-...`) or Unix domain socket (`.backlogit/runtime/mcp.sock`), and turn `backlogit mcp` into a thin stdio-to-IPC bridge multiplexing multiple stdio clients over a single background server.
* **Pros**:
  * Allows multiple simultaneous MCP stdio clients to share one in-memory server instance.
* **Cons**:
  * High complexity and blast radius: requires concurrent session multiplexing inside `internal/mcp/server.go`, platform-specific named-pipe/UDS framing and ACL hardening, version-skew detection when the binary upgrades while a background daemon is running, and detached-process lifecycle management.
  * Contradicts the stash preference ("preferably a CLI only mechanism") and violates YAGNI for single-workspace agent workflows.
* **Effort estimate**: High.

### Option C: External Shell Wrapper Script Only (`start.ps1` / `start.sh`)

* **Description**:
  * Check for existing `backlogit mcp` processes in external startup scripts using PID files or process-table scans.
* **Pros**:
  * Zero Go code changes.
* **Cons**:
  * Bypassed whenever MCP hosts (`.mcp.json`, Copilot CLI, Claude Code, Cursor) invoke `backlogit mcp` directly.
  * PID files without kernel-held file locks are vulnerable to stale-lock wedging after crashes and PID reuse on Windows.
* **Effort estimate**: Low, but fails the core enforcement requirement.

## Trade-off Comparison

| Criterion | Option A (CLI-Only OS Lock + Lifecycle CLI) | Option B (Detached IPC Daemon + Proxy) | Option C (External Script Only) |
|---|---|---|---|
| Enforced by `backlogit mcp` directly | Yes | Yes | No |
| Honors CLI-only preference | Yes | No (detached background IPC daemon) | Partial |
| Crash recovery | Automatic (OS kernel releases lock handle) | Complex (stale socket/pipe + orphan daemon) | Brittle (stale PID files) |
| Cross-platform safety (Windows + Unix) | High (reuses proven `CreateFile` / `Flock` pattern) | Moderate/Low (named pipe vs UDS divergence) | Low |
| Implementation blast radius | Minimal (`internal/errors`, `internal/cli`, `docs/cli-reference`) | Large (`internal/mcp`, IPC transport, daemon supervisor) | External only |

## Decision

Adopt **Option A (CLI-Only Single-Instance Workspace Guard + Lifecycle CLI)** per the operator's in-session directive (`"Let's start with Option 1, then go directly to Option 2: Single-Daemon-Per-Workspace"`):

1. Define sentinel error `ErrDaemonAlreadyRunning = errors.New("backlogit: mcp daemon already running for workspace")` in `internal/errors/errors.go`.
2. Implement the single-instance workspace daemon lock and lifecycle manager in `internal/cli/mcp_daemon.go`:
   * Resolve and pin the workspace runtime directory handle (`openMCPDaemonRuntimeRoot` via `core.ResolveStorageRoot(rootPath)` -> `os.OpenRoot(storageRoot)` -> `storageRootHandle.OpenRoot("runtime")`, or `<abs_root>/<core.WorkspaceRootCandidates()[0]>/runtime` when uninitialized), verifying `Lstat` + `core.IsSymlinkOrReparsePoint` and handle-derived final path equality on `storageRoot` and `runtime`, and executing all leaf operations (`mcp-daemon.lock`, `mcp-daemon.json`, and temporary sidecar create/write/rename/remove) as **directory-relative operations through the pinned `runtimeRoot` handle**.
   * Acquire an exclusive non-blocking OS lock on `"mcp-daemon.lock"` relative to `runtimeRoot` using a directory-relative no-follow/reparse-safe lock open (`Openat` with `O_NOFOLLOW` on Unix; directory-relative open with `FILE_FLAG_OPEN_REPARSE_POINT` + `FILE_ATTRIBUTE_REPARSE_POINT` rejection on Windows) and handle identity verification. Immediately after acquiring `mcp-daemon.lock` and before writing the new sidecar, remove any pre-existing stale `"mcp-daemon.json"` through `runtimeRoot`, then persist holder metadata (`MCPDaemonInfo`: `PID`, `ProcessStartTime`, `ExecutablePath`, `StartedAt`, `WorkspaceRoot`, `StorageRoot`, `Version`, `LockPath`) atomically via a directory-relative temp file and rename on `runtimeRoot`. Read `"mcp-daemon.json"` by opening a no-follow validated file handle relative to `runtimeRoot` and decoding directly from that handle (`json.NewDecoder(file)`), never via a separate path-based `os.ReadFile`.
   * On contention without `--replace`, read `mcp-daemon.json` via `runtimeRoot` (best-effort fallback when metadata is unreadable or still being written, failing closed on symlink/reparse validation errors) and return a wrapped `corerrors.ErrDaemonAlreadyRunning` with actionable CLI remediation.
   * With `--replace`, stop the existing holder via the bounded stop flow and retry lock acquisition.
3. Expose CLI subcommands and flags under `backlogit mcp`:
   * `backlogit mcp [--replace]`: acquires and holds the single-instance lock across `mcpinternal.RunStdio`, cleaning up `mcp-daemon.json` and releasing `mcp-daemon.lock` on exit.
   * `backlogit mcp status [--format text|json]`: probes `mcp-daemon.lock` and reads `mcp-daemon.json` through the pinned `runtimeRoot` to report whether a workspace MCP daemon is running, cleaning up any stale `mcp-daemon.json` if the OS lock is free.
   * `backlogit mcp stop [--format text|json]`: probes `mcp-daemon.lock` through the pinned `runtimeRoot`; if running, reads `mcp-daemon.json` from the validated no-follow handle, validates positive/non-self PID, opens a **single retained OS process handle** for `info.PID` (`windows.OpenProcess` on Windows / `unix.PidfdOpen` + `unix.PidfdSendSignal` or retained verified process handle on Unix), verifies that the retained handle matches the sidecar's bound process-instance identity (`ProcessStartTime` and `ExecutablePath`) and re-verifies the sidecar while holding that handle, and terminates the holder **through that same retained process handle** without a second PID lookup (failing closed with `corerrors.ErrValidation` without signaling if the process identity mismatches, PID ownership changes before termination, metadata is stale/missing during startup, or identity-bound signaling is unavailable), polls `mcp-daemon.lock` until the OS releases the handle (bounded timeout), removes `mcp-daemon.json` via `runtimeRoot`, and reports the stop result (idempotent no-op when no daemon is running).
4. Update `cliOnlyIntentional` in `internal/cli/registry_parity_test.go` for `backlogit mcp status` and `backlogit mcp stop`, and regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference`.

## Rejected Alternatives

* **Option B** is set aside because detached background IPC daemons and stdio-to-pipe multiplexing introduce disproportionate transport, security, and lifecycle complexity when single-instance OS-lock enforcement + `--replace`/`stop`/`status` satisfies the workspace isolation requirement cleanly.
* **Option C** is set aside because MCP hosts launch `backlogit mcp` directly rather than through wrapper scripts, and PID files without kernel file locks leave stale locks after crashes.

## Unresolved Questions

* None for single-instance enforcement. If multi-client multiplexing over a shared daemon is ever needed in the future, `mcp-daemon.json` can be extended with an endpoint descriptor in a separate feature without breaking the CLI lifecycle commands established here.

## Risks and Mitigations

* **Risk**: On Windows, exclusive lock handles (`dwShareMode = 0`) prevent other processes from reading the locked file directly.
  * **Mitigation**: Keep `mcp-daemon.lock` as a zero-byte OS lock file and write holder metadata to the sibling `mcp-daemon.json` file, which remains readable by all processes on both Windows and Unix.
* **Risk**: After an abrupt crash, a new daemon acquires `mcp-daemon.lock` before replacing the old `mcp-daemon.json` (or the holder exits and its PID is reused between identity check and termination), and `stop` or `--replace` could signal an unrelated process if the crashed daemon's PID was reused by the OS.
  * **Mitigation**: Remove any pre-existing stale `mcp-daemon.json` through the pinned `runtimeRoot` immediately after acquiring `mcp-daemon.lock` before writing the new sidecar, record the holder's OS process-instance identity (`ProcessStartTime` and `ExecutablePath`) in `MCPDaemonInfo`, and require `StopMCPDaemon` to open a single retained OS process handle for `info.PID`, verify `ProcessStartTime` and `ExecutablePath` and re-verify the sidecar on that retained handle, and terminate the process **through that same retained handle** without a second PID lookup (failing closed without signaling on any mismatch, PID ownership change, stale startup metadata, or unavailable identity-bound handle signaling).
* **Risk**: A symlink or Windows reparse point planted at `<storage_root>/runtime`, `mcp-daemon.lock`, or `mcp-daemon.json` (or swapped in after path validation) redirects lock creation or metadata reads/writes outside the workspace.
  * **Mitigation**: Pin a validated `*os.Root` runtime directory handle (`openMCPDaemonRuntimeRoot` via `os.OpenRoot` on validated `storageRoot` -> `runtimeRoot`), execute all lock, sidecar, and temp-file operations as directory-relative operations through `runtimeRoot` with no-follow/reparse-safe opens (`O_NOFOLLOW` on Unix / `FILE_FLAG_OPEN_REPARSE_POINT` + reparse-point attribute rejection on Windows), and decode `mcp-daemon.json` directly from the validated no-follow `*os.File` handle (`json.NewDecoder(file)`), covered by parent-swap and leaf-swap regression tests.
* **Risk**: Adding subcommands (`status`, `stop`) and flags (`--replace`) to `backlogit mcp` causes `TestRegistryParity_DiscoverabilityConsistency` or CI's `CLI Reference Drift Check` to fail if the CLI-only allowlist and generated reference docs are not updated in lockstep.
  * **Mitigation**: Register `backlogit mcp status` and `backlogit mcp stop` in `cliOnlyIntentional` (`internal/cli/registry_parity_test.go`) and run `go run ./cmd/gen-docs docs/cli-reference` + `go run ./cmd/backlogit docs lint` in the final documentation unit.
