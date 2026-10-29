---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for stash 760670A3 (Option A): enforce a single backlogit MCP daemon per workspace via an OS advisory lock, JSON metadata sidecar, and CLI lifecycle commands (status, stop, --replace).'
doc_type: plan
docline:
  feature_id: 204-F
  shipment_id: 205-S
  source_decision: docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md
  stash_id: 760670A3
schema_version: "1.0"
source: docs/exec-plans/2026-10-29-single-daemon-per-workspace-plan.md
title: 'Implementation Plan: Single backlogit MCP daemon per workspace (760670A3)'
---

## Objective

Enforce that at most one `backlogit mcp` stdio server daemon runs per workspace at any time using a pure CLI-and-OS-lock mechanism (`<runtime_dir>/mcp-daemon.lock` + `<runtime_dir>/mcp-daemon.json`), fail fast with sentinel error `corerrors.ErrDaemonAlreadyRunning` when a duplicate instance starts without `--replace`, and expose `backlogit mcp status`, `backlogit mcp stop`, and `backlogit mcp --replace` CLI controls.
Source decision: `docs/decisions/2026-10-29-single-daemon-per-workspace-deliberation.md` (Option A, stash `760670A3`, deliberation `077-DL`).

## Problem Frame

`backlogit mcp` (`internal/cli/root.go`: `newMCPCommand` -> `openMCPServer` -> `mcpinternal.RunStdio`) starts a long-lived MCP server holding an open SQLite `*sql.DB` pool on `<storage_root>/backlogit.db`, an in-memory file manifest snapshot (`s.manifest`), and event/telemetry/hook writers. When multiple editor windows, orphaned IDE sessions, or parallel agent launches start duplicate `backlogit mcp` processes in the same workspace:

* Multiple long-lived processes contend on `<storage_root>/backlogit.db` and workspace lock sidecars.
* Each server maintains an isolated `s.manifest` snapshot, producing stale diffs on `backlogit_merge_sync`.
* Orphaned `backlogit mcp` instances linger in the background with no CLI status or stop mechanism.

Short-lived one-shot CLI invocations (`backlogit list`, `backlogit sync`, `backlogit shipment ...`) open and close the workspace per command and must remain unblocked while `backlogit mcp` holds the workspace daemon lock.

## Requirements Trace

| ID | Requirement | Source | Implementation Unit |
|---|---|---|---|
| R1 | Declare sentinel `ErrDaemonAlreadyRunning` in `internal/errors` and daemon lock/status/stop types and function signatures in `internal/cli/mcp_daemon.go`, gated by a `go/ast` source-shape harness | `2026-10-29-single-daemon-per-workspace-deliberation.md` (Option A, item 1) | U1 (`204.001-T`) |
| R2 | Resolve the workspace runtime directory safely (`.backlog/runtime` or `.backlogit/runtime` via `core.ResolveStorageRoot`, falling back to `core.WorkspaceRootCandidates()[0]` when uninitialized, with `core.IsSymlinkOrReparsePoint` rejection), acquire an exclusive OS lock on `mcp-daemon.lock` via `openSelfUpdateLockFile`, persist `mcp-daemon.json`, fail fast with `ErrDaemonAlreadyRunning` on contention, and clean up stale `mcp-daemon.json` on crash-recovery status probes | `2026-10-29-single-daemon-per-workspace-deliberation.md` (Option A, item 2) | U2 (`204.002-T`) |
| R3 | Implement `StopMCPDaemon` (with PID safety validation and bounded wait for OS lock release) and `--replace` takeover, and wire `backlogit mcp [--replace]`, `backlogit mcp status [--format text\|json]`, and `backlogit mcp stop [--format text\|json]` into `newMCPCommand` | `2026-10-29-single-daemon-per-workspace-deliberation.md` (Option A, item 3) | U3 (`204.003-T`) |
| R4 | Register `backlogit mcp status` and `backlogit mcp stop` in `cliOnlyIntentional` (`internal/cli/registry_parity_test.go`) and verify cross-process OS lock contention and stop/replace behavior using a real child subprocess | `2026-10-29-single-daemon-per-workspace-deliberation.md` (Option A, item 4) | U4 (`204.004-T`) |
| R5 | Regenerate `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` and verify zero docline or markdownlint violations | `2026-10-29-single-daemon-per-workspace-deliberation.md` (Option A, item 4) | U5 (`204.005-T`) |

## Constraints

* **2-Hour Rule and Width Isolation**: Every unit touches at most 3 files, at most 4 functions, at most 3 test scenarios, and a single skill domain (`code`, `tests`, or `docs`).
* **No hardcoded `.backlogit` literals**: `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` (`internal/core/workspace_literal_guard_test.go`) forbids hardcoded `".backlogit"` string literals in `internal/cli`. Always resolve storage roots via `core.ResolveStorageRoot` and `core.WorkspaceRootCandidates()[0]`.
* **Windows exclusive lock compatibility**: Because `openSelfUpdateLockFile` on Windows opens `mcp-daemon.lock` with `dwShareMode = 0`, no second process on Windows can read `mcp-daemon.lock` while it is held. Holder metadata MUST be written to the companion `<runtime_dir>/mcp-daemon.json` sidecar, while `mcp-daemon.lock` serves as the exclusive OS lock handle.
* **Preserve `openMCPServer` unit-test contract**: `openMCPServer` (`internal/cli/root.go`) is called directly by in-process unit tests (`TestOpenMCPServer_AllowsUninitializedWorkspace`). Acquire the workspace daemon lock in `newMCPCommand`'s `RunE` before calling `openMCPServer`, leaving `openMCPServer` itself lock-free.
* **Closed P-002.1 `harness-exempt` set**: Exactly `["204.005-T"]` (`docs-only`, CLI reference regeneration in U5). All other tasks (`204.001-T`..`204.004-T`) are normal `harness-ready` tasks.

## Implementation Units

### U1: Declare `ErrDaemonAlreadyRunning` sentinel and MCP daemon lock types + source-shape harness (`code`) — `204.001-T`

* **Domain**: `code` (declaration + `go/ast` source-shape harness per P-002.1).
* **Changes**:
  * **First (RED phase, scaffolded by `harness-architect`)**: Create `internal/cli/mcp_daemon_shape_test.go` with `TestU1_MCPDaemonDeclarationsSourceShape` using `go/ast`, `go/parser`, and `go/token` (no direct symbol references so it compiles before declarations exist and fails on assertions). Assert:
    1. `internal/errors/errors.go` declares exported sentinel `ErrDaemonAlreadyRunning`.
    2. `internal/cli/mcp_daemon.go` declares exported structs `MCPDaemonInfo` (with json-tagged fields `pid`, `started_at`, `workspace_root`, `storage_root`, `version`, `lock_path`), `MCPDaemonStatus` (with json-tagged fields `running`, `pid`, `started_at`, `workspace_root`, `storage_root`, `version`, `lock_path`), and `MCPDaemonStopResult` (with json-tagged fields `stopped`, `running`, `pid`, `workspace_root`, `storage_root`, `lock_path`), plus functions `AcquireMCPDaemonLock`, `InspectMCPDaemonStatus`, and `StopMCPDaemon`. Do not freeze temporary function bodies so successor units U2 and U3 keep `TestU1_` green (`docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`).
  * **Second (GREEN phase, implemented by `build-feature`)**:
    1. Add `ErrDaemonAlreadyRunning = errors.New("backlogit: mcp daemon already running for workspace")` to `internal/errors/errors.go`.
    2. Create `internal/cli/mcp_daemon.go` declaring `MCPDaemonInfo`, `MCPDaemonStatus`, `MCPDaemonStopResult`, internal `mcpDaemonOptions`, and functions `AcquireMCPDaemonLock(ctx context.Context, rootPath string, replace bool) (func() error, error)`, `InspectMCPDaemonStatus(rootPath string) (MCPDaemonStatus, error)`, and `StopMCPDaemon(ctx context.Context, rootPath string) (MCPDaemonStopResult, error)` returning `corerrors.ErrNotImplemented` (no unverified behavior in U1).
* **Affected files**:
  * `internal/errors/errors.go`
  * `internal/cli/mcp_daemon.go` (new)
  * `internal/cli/mcp_daemon_shape_test.go` (new)
* **Acceptance criteria**:
  * `TestU1_MCPDaemonDeclarationsSourceShape` compiles and is observed RED before the declarations land, then turns GREEN once `internal/errors/errors.go` and `internal/cli/mcp_daemon.go` are written.
  * `internal/cli/mcp_daemon.go` contains no hardcoded `".backlogit"` string literal (`TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` stays green).
  * `go test -count=1 -run '^TestU1_' ./internal/cli` passes.
* **Execution posture**: test-first (P-002.1 source-shape harness first, declaration second).

### U2: Implement single-instance OS lock acquisition, metadata sidecar persistence, and crash-recovery status probe (`code`) — `204.002-T`

* **Domain**: `code`.
* **Depends on**: U1 (`204.001-T`).
* **Changes**:
  * In `internal/cli/mcp_daemon.go`, implement:
    1. `resolveMCPDaemonRuntimeDir(rootPath string) (workspaceRoot, storageRoot, runtimeDir string, err error)`:
       * Resolve `absRoot` via `filepath.Abs(filepath.Clean(rootPath))`.
       * Call `core.ResolveStorageRoot(absRoot)`: when `err == nil`, set `storageRoot` and `workspaceRoot = filepath.Dir(storageRoot)`; when `errors.Is(err, os.ErrNotExist)`, set `workspaceRoot = absRoot` and `storageRoot = filepath.Join(absRoot, core.WorkspaceRootCandidates()[0])`; otherwise return the error wrapped with `%w`.
       * Check `os.Lstat` + `core.IsSymlinkOrReparsePoint` on `storageRoot` (if present) and `runtimeDir = filepath.Join(storageRoot, "runtime")` (if present), rejecting any symlink or reparse point with `fmt.Errorf("%w: ...", corerrors.ErrValidation)` before calling `os.MkdirAll(runtimeDir, 0o755)` and re-verifying `core.IsSymlinkOrReparsePoint` after `MkdirAll`.
    2. `AcquireMCPDaemonLock` (and testable `acquireMCPDaemonLockWithOptions(ctx context.Context, rootPath string, replace bool, opts mcpDaemonOptions) (func() error, error)`):
       * Acquire exclusive non-blocking OS lock on `<runtimeDir>/mcp-daemon.lock` via `opts.OpenLock` (defaulting to `openSelfUpdateLockFile`).
       * When `acquired == true`: write `MCPDaemonInfo` (`PID: opts.PID()`, `StartedAt: opts.Now().UTC().Format(time.RFC3339)`, `WorkspaceRoot: workspaceRoot`, `StorageRoot: storageRoot`, `Version: version.Resolve()`, `LockPath: lockPath`) atomically via `os.CreateTemp(runtimeDir, ".mcp-daemon-*.tmp")` + `os.Rename` to `<runtimeDir>/mcp-daemon.json`. Return an idempotent `sync.Once` release closure that removes `<runtimeDir>/mcp-daemon.json` (ignoring `os.ErrNotExist`) and closes the `mcp-daemon.lock` file handle.
       * When reading `<runtimeDir>/mcp-daemon.json` (`readMCPDaemonInfo`), verify via `os.Lstat` + `core.IsSymlinkOrReparsePoint` that `mcp-daemon.json` is not a symlink or reparse point before reading it.
       * When `acquired == false` and `replace == false`: read `<runtimeDir>/mcp-daemon.json` (best-effort) and return an error wrapping `corerrors.ErrDaemonAlreadyRunning` naming the active holder's PID and `started_at` (when available) and actionable CLI remediation (`backlogit mcp status`, `backlogit mcp stop`, `backlogit mcp --replace`).
    3. `InspectMCPDaemonStatus` (and `inspectMCPDaemonStatusWithOptions`):
       * Probe `<runtimeDir>/mcp-daemon.lock` via `opts.OpenLock`.
       * If `acquired == true` (no live process holds the OS lock): immediately close the probe handle, remove any crash-leftover `<runtimeDir>/mcp-daemon.json` (`os.Remove`, ignoring `os.ErrNotExist`), and return `MCPDaemonStatus{Running: false, WorkspaceRoot: workspaceRoot, StorageRoot: storageRoot, LockPath: lockPath}`.
       * If `acquired == false` (live process holds the OS lock): read `<runtimeDir>/mcp-daemon.json` (tolerating missing/partial metadata if inspected mid-start) and return `MCPDaemonStatus{Running: true, PID: info.PID, StartedAt: info.StartedAt, WorkspaceRoot: workspaceRoot, StorageRoot: storageRoot, Version: info.Version, LockPath: lockPath}`.
* **Affected files**:
  * `internal/cli/mcp_daemon.go`
  * `internal/cli/mcp_daemon_lock_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU2_AcquireLockAndContention`): first `AcquireMCPDaemonLock` succeeds, writes `mcp-daemon.json`, and `InspectMCPDaemonStatus` reports `Running: true` with matching `PID`, `WorkspaceRoot`, `StorageRoot`, and `Version`; a second `AcquireMCPDaemonLock` with `replace: false` fails with `errors.Is(err, corerrors.ErrDaemonAlreadyRunning)` naming the holder PID; calling `release()` removes `mcp-daemon.json` and closes the lock handle.
  * Scenario 2 (`TestU2_CrashRecoveryCleansStaleMetadata`): if the lock handle is closed directly without removing `mcp-daemon.json` (simulating an abrupt crash / `SIGKILL`), `InspectMCPDaemonStatus` detects the uncontended OS lock, removes the stale `mcp-daemon.json`, and reports `Running: false`, and a subsequent `AcquireMCPDaemonLock` succeeds.
  * Scenario 3 (`TestU2_RejectsSymlinkOrReparseRuntimeDir`): if `runtime` is a symlink or junction pointing outside the workspace, `AcquireMCPDaemonLock` and `InspectMCPDaemonStatus` fail closed with `errors.Is(err, corerrors.ErrValidation)`.
  * `go test -count=1 -run '^TestU2_' ./internal/cli` passes.
* **Execution posture**: test-first.

### U3: Implement `StopMCPDaemon` and `--replace` takeover + wire `backlogit mcp`, `backlogit mcp status`, and `backlogit mcp stop` CLI commands (`code`) — `204.003-T`

* **Domain**: `code`.
* **Depends on**: U2 (`204.002-T`).
* **Changes**:
  * In `internal/cli/mcp_daemon.go`:
    1. Implement `StopMCPDaemon(ctx context.Context, rootPath string) (MCPDaemonStopResult, error)` (and `stopMCPDaemonWithOptions`):
       * Probe `<runtimeDir>/mcp-daemon.lock` via `opts.OpenLock`. If `acquired == true` (no daemon running), close the probe handle, remove any stale `mcp-daemon.json`, and return `MCPDaemonStopResult{Stopped: false, Running: false, WorkspaceRoot: workspaceRoot, StorageRoot: storageRoot, LockPath: lockPath}`, `nil`.
       * If `acquired == false`: read `<runtimeDir>/mcp-daemon.json`. Validate `info.PID > 0` and `info.PID != opts.PID()` (refuse invalid or self PID with a descriptive error). Call `opts.KillProcess(info.PID)` (default: `os.FindProcess(pid)` -> `proc.Kill()`, treating `os.ErrProcessDone` as already exited). Poll `opts.OpenLock(lockPath)` at 25ms intervals up to `opts.StopTimeout` (default `5 * time.Second`, respecting `ctx.Done()`) until `acquired == true`. Close the probe handle, remove `mcp-daemon.json`, and return `MCPDaemonStopResult{Stopped: true, Running: false, PID: info.PID, WorkspaceRoot: workspaceRoot, StorageRoot: storageRoot, LockPath: lockPath}`, `nil`.
    2. Complete the `replace == true` branch in `acquireMCPDaemonLockWithOptions`: when the initial `OpenLock` returns `acquired == false` and `replace == true`, invoke `stopMCPDaemonWithOptions(ctx, rootPath, opts)` and then re-acquire `OpenLock` and write the new holder's `mcp-daemon.json`.
  * In `internal/cli/root.go`:
    1. Add package-level test seam `var serveMCPStdio = mcpinternal.RunStdio`.
    2. Add `--replace` boolean flag (`"stop any running workspace MCP daemon before starting"`) to `newMCPCommand`.
    3. In `newMCPCommand`'s `RunE`, call `release, err := AcquireMCPDaemonLock(cmd.Context(), *cwd, replaceFlag)` before `openMCPServer`; if `err != nil`, return `err`; immediately register `defer func() { _ = release() }()` (so the lock and `mcp-daemon.json` are released even if `openMCPServer` returns an error), then call `serveMCPStdio(s)`.
    4. Add subcommands `newMCPStatusCommand(cwd *string)` (`backlogit mcp status [--format text|json]`) and `newMCPStopCommand(cwd *string)` (`backlogit mcp stop [--format text|json]`) under `newMCPCommand`.
* **Affected files**:
  * `internal/cli/mcp_daemon.go`
  * `internal/cli/root.go`
  * `internal/cli/mcp_daemon_cli_test.go` (new)
* **Acceptance criteria**:
  * Scenario 1 (`TestU3_MCPCommandSingleInstanceAndStatus`): while one `backlogit --cwd <root> mcp` execution is active in `serveMCPStdio`, `backlogit --cwd <root> mcp status --format json` reports `"running": true`, and a second `backlogit --cwd <root> mcp` fails with `errors.Is(err, corerrors.ErrDaemonAlreadyRunning)`.
  * Scenario 2 (`TestU3_MCPStopCommandIdempotent`): `backlogit --cwd <root> mcp stop --format json` terminates the active holder, releases the lock, and outputs `"stopped": true, "running": false`; running `backlogit --cwd <root> mcp stop --format json` again when no daemon is running succeeds idempotently with `"stopped": false, "running": false`.
  * Scenario 3 (`TestU3_MCPCommandReplaceFlag`): `backlogit --cwd <root> mcp --replace` stops the existing holder and starts the new MCP server instance cleanly.
  * `go test -count=1 -run '^(TestU1_|TestU2_|TestU3_|TestOpenMCPServer_)' ./internal/cli` passes.
* **Execution posture**: test-first.

### U4: Update CLI-only registry parity allowlist and verify cross-process subprocess lock contention (`tests`) — `204.004-T`

* **Domain**: `tests`.
* **Depends on**: U3 (`204.003-T`).
* **Changes**:
  * Update `cliOnlyIntentional` in `internal/cli/registry_parity_test.go` to include `"backlogit mcp status"` and `"backlogit mcp stop"` so `TestRegistryParity_DiscoverabilityConsistency` verifies that both new CLI lifecycle subcommands exist on the live command tree and are intentionally CLI-only.
  * Create `internal/cli/mcp_daemon_subprocess_test.go` with `TestU4_CrossProcessMCPDaemonLockAndStop` and subprocess helper `TestU4_HelperHoldMCPDaemonLock` (invoked via `os.Args[0] -test.run=^TestU4_HelperHoldMCPDaemonLock$` with an environment variable pointing at `t.TempDir()`):
    1. Spawn the child helper process, wait for it to acquire the real OS lock (`AcquireMCPDaemonLock`) and print a readiness line on stdout.
    2. From the parent process, assert `InspectMCPDaemonStatus` reports `Running: true` with `PID == child.Process.Pid`, and assert `AcquireMCPDaemonLock(ctx, root, false)` returns `errors.Is(err, corerrors.ErrDaemonAlreadyRunning)`.
    3. Call `StopMCPDaemon(ctx, root)` from the parent process; assert it terminates the child process (`Stopped: true, PID: child.Process.Pid`), waits for OS lock release, and allows the parent process to immediately acquire `AcquireMCPDaemonLock(ctx, root, false)`.
* **Affected files**:
  * `internal/cli/registry_parity_test.go`
  * `internal/cli/mcp_daemon_subprocess_test.go` (new)
* **Acceptance criteria**:
  * `TestU4_CrossProcessMCPDaemonLockAndStop` passes using a real child OS process holding the kernel lock handle.
  * `TestRegistryParity_DiscoverabilityConsistency` and all `TestRegistryParity_*` tests in `internal/cli` pass.
  * `go test -count=1 -race -timeout=10m ./internal/cli/... ./internal/errors/...` passes.
* **Execution posture**: test-first.

### U5: Regenerate CLI reference documentation for `backlogit mcp`, `backlogit mcp status`, and `backlogit mcp stop` (`docs`) — `204.005-T`

* **Domain**: `docs` (`harness-exempt: docs-only`).
* **Depends on**: U4 (`204.004-T`).
* **Changes**:
  * Run `go run ./cmd/gen-docs docs/cli-reference` from the repository root to regenerate `docs/cli-reference/backlogit_mcp.md` (with `--replace` and `SEE ALSO` links) and generate `docs/cli-reference/backlogit_mcp_status.md` and `docs/cli-reference/backlogit_mcp_stop.md`.
  * Verify `go run ./cmd/backlogit docs lint --path docs/cli-reference` and `scripts/md-lint.ps1` pass with 0 violations.
* **Affected files**:
  * `docs/cli-reference/backlogit_mcp.md`
  * `docs/cli-reference/backlogit_mcp_status.md` (new generated file)
  * `docs/cli-reference/backlogit_mcp_stop.md` (new generated file)
* **Acceptance criteria**:
  * `docs/cli-reference/backlogit_mcp_status.md` and `docs/cli-reference/backlogit_mcp_stop.md` exist and `docs/cli-reference/backlogit_mcp.md` includes `--replace` and `SEE ALSO` entries for `backlogit mcp status` and `backlogit mcp stop`.
  * Running `go run ./cmd/gen-docs docs/cli-reference` followed by `git diff --exit-code docs/cli-reference/` exits `0` (zero CLI reference drift).
  * `go run ./cmd/backlogit docs lint --path docs/cli-reference` reports `valid: true` with `0 violations`, and `exempt_verification_command` exits `0` and prints `EXEMPT_VERIFY_OK:204.005-T`.
* **Execution posture**: characterization-first (`docs-only`, `harness-exempt`).

## Dependency Graph

`U1 (204.001-T) -> U2 (204.002-T) -> U3 (204.003-T) -> U4 (204.004-T) -> U5 (204.005-T)`

Five sequential waves (1 task per wave), ensuring zero sibling RED interference and clean per-wave convergence.

## Decisions and Rationale

* **Companion `mcp-daemon.json` alongside `mcp-daemon.lock`**: On Windows, `openSelfUpdateLockFile` uses `CreateFile` with `dwShareMode = 0`, which prevents other processes from opening `mcp-daemon.lock` for reading while held. Keeping `mcp-daemon.lock` as the zero-byte exclusive OS lock file and writing `MCPDaemonInfo` to `mcp-daemon.json` guarantees cross-platform readability while preserving kernel-enforced mutual exclusion and instant crash recovery.
* **Fallback storage root `core.WorkspaceRootCandidates()[0]` (`.backlog`) when uninitialized**: Avoids hardcoded `".backlogit"` literals forbidden by `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` and avoids creating a `.backlogit` directory that would cause a subsequent `backlogit init` to fail.
* **Injectable `serveMCPStdio` and `mcpDaemonOptions` seams**: Allows fast, deterministic in-process CLI tests in U2 and U3 while U4 exercises real cross-process OS lock contention and process termination via a child subprocess.

## Risks and Caveats

* **Risk**: `StopMCPDaemon` reads a stale or corrupted `mcp-daemon.json` and attempts to kill PID `0` or the current process.
  * **Mitigation**: `StopMCPDaemon` only runs when `OpenLock` confirms `mcp-daemon.lock` is actively held by another process, and explicitly rejects `info.PID <= 0` or `info.PID == os.Getpid()` before calling `KillProcess`.
* **Risk**: CI `CLI Reference Drift Check` fails when `newMCPCommand` adds `--replace`, `status`, and `stop`.
  * **Mitigation**: U5 runs `go run ./cmd/gen-docs docs/cli-reference` and verifies `git diff --exit-code docs/cli-reference/` and `backlogit docs lint`.

## Constitution Check

* **I. Safety-First Go / Language**: `pass` — idiomatic Go in `internal/errors` and `internal/cli`, all errors wrapped with `%w`, typed structs with explicit JSON tags, no `unsafe` or `panic()`.
* **II. Test-First Development (NON-NEGOTIABLE)**: `pass` — U1 uses a `go/ast` source-shape harness before declarations land; U2, U3, and U4 use red-first behavioral and cross-process tests; U5 is a P-002.1-valid `docs-only` task with a fail-before-deliverable `exempt_verification_command`.
* **III. Workspace Isolation and Security Boundaries**: `pass` — runtime directory resolution validates containment and rejects symlinks/reparse points via `core.IsSymlinkOrReparsePoint`.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE)**: `pass` — all lock and metadata files reside strictly inside `<storage_root>/runtime/` under the target workspace (`--cwd`).
* **V. Structured Observability**: `pass` — `backlogit mcp status` and `backlogit mcp stop` support `--format json` and structured `log/slog` logging.
* **VI. Single Responsibility**: `pass` — daemon lock and lifecycle logic is isolated in `internal/cli/mcp_daemon.go` with zero third-party dependencies added.
* **VII. Destructive Command Approval (NON-NEGOTIABLE)**: `pass` — `StopMCPDaemon` only targets a verified workspace `backlogit mcp` lock holder when explicitly invoked via `backlogit mcp stop` or `backlogit mcp --replace`.
* **VIII. Explicit Safety Modes for Elevated Risk**: `pass` — PID validation (`PID > 0 && PID != os.Getpid()`) and reparse-point checks fail closed.
* **IX. Git-Friendly Persistence**: `pass` — `<storage_root>/runtime/` is gitignored and ephemeral; `mcp-daemon.json` is written atomically and removed on clean exit.
* **X. Agent Context Efficiency**: `pass` — `backlogit mcp status --format json` and `backlogit mcp stop --format json` return compact, machine-readable JSON envelopes.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE)**: `pass` — shipment `205-S` merges via a merge commit (`--merge`), never squash or rebase (P-009).

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change**: `present` — adds `--replace` flag and `status` / `stop` subcommands to `backlogit mcp`, plus exported sentinel `corerrors.ErrDaemonAlreadyRunning`.
* **security, auth, permission, or compliance-sensitive behavior**: `present` — creates lock/metadata files under `<storage_root>/runtime/` (must reject symlinks/reparse points) and terminates a process by PID in `StopMCPDaemon` (must validate PID safety).
* **migration, backfill, destructive data/config action, or irreversible step**: `present` — `backlogit mcp stop` and `backlogit mcp --replace` terminate a running `backlogit mcp` process holding the workspace lock.
* **external integration, operator checkpoint, or external dependency**: `absent` — pure local CLI and OS file lock mechanism with no external service or dependency.
* **high runtime, rollout, or rollback risk**: `present` — modifies the `backlogit mcp` startup path used by MCP clients.

Requires plan hardening: yes

## Plan Hardening

### 1. Risk Triggers and Protected Invariants

* **Invariant I1 (Single Live MCP Daemon Per Workspace)**: At most one `backlogit mcp` process may hold `<runtime_dir>/mcp-daemon.lock` for a workspace at any moment. A second `backlogit mcp` without `--replace` MUST fail before `openMCPServer` or `mcpinternal.RunStdio` runs.
* **Invariant I2 (Automatic Crash Recovery)**: A crashed or killed `backlogit mcp` process MUST never wedge subsequent `backlogit mcp` starts. Liveness is determined solely by attempting `openSelfUpdateLockFile` on `mcp-daemon.lock`, never by trusting a stale `mcp-daemon.json` PID without checking the OS lock.
* **Invariant I3 (Safe PID Termination)**: `StopMCPDaemon` MUST never signal a PID unless `openSelfUpdateLockFile` just proved `mcp-daemon.lock` is actively contended, and MUST reject `PID <= 0` or `PID == os.Getpid()`.
* **Invariant I4 (Reparse-Point Containment)**: `resolveMCPDaemonRuntimeDir` MUST check `core.IsSymlinkOrReparsePoint` on both the storage root and `runtime` directory so a planted symlink or Windows junction cannot redirect `mcp-daemon.lock` or `mcp-daemon.json` outside the workspace.
* **Invariant I5 (`openMCPServer` Backwards Compatibility)**: `openMCPServer` itself MUST NOT acquire `mcp-daemon.lock` so in-process callers and unit tests (`TestOpenMCPServer_AllowsUninitializedWorkspace`) do not leak lock handles.

### 2. Reinforced Instructions and Learnings Consulted

* `docs/compound/2026-10-27-pin-the-handle-not-the-path.md` — reject symlinks and Windows junctions (`core.IsSymlinkOrReparsePoint`) on the runtime directory before and after `os.MkdirAll`.
* `docs/compound/concurrency-issues/2026-08-09-stable-lock-keys-and-heartbeat-refresh-for-unbounded-holds.md` — key the workspace lock on the canonical storage root (`<storage_root>/runtime/mcp-daemon.lock`) using an OS kernel file handle (`openSelfUpdateLockFile`) held for the server lifetime rather than a TTL-based sidecar.
* `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md` — U1's `go/ast` source-shape harness asserts permanent declarations, JSON tags, and signatures rather than temporary stub bodies.
* `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — U5 regenerates `docs/cli-reference/` via `go run ./cmd/gen-docs docs/cli-reference` rather than hand-editing reference markdown.

### 3. Risky Action Classification (`strict-safety`)

* `ProposedAction`: Acquire exclusive OS lock `<runtime_dir>/mcp-daemon.lock` and write `<runtime_dir>/mcp-daemon.json` in `backlogit mcp` (`U2`/`U3`)
  * `ActionRisk`: `contract` (rejects duplicate `backlogit mcp` starts for the same workspace with `ErrDaemonAlreadyRunning`)
  * `Approval`: Pre-authorized by operator directive (`2026-10-29T20:06:47Z`, Option 2: Single-Daemon-Per-Workspace)
  * `ActionResult`: `planned`
* `ProposedAction`: Terminate active workspace MCP daemon holder PID on `backlogit mcp stop` or `backlogit mcp --replace` (`U3`)
  * `ActionRisk`: `destructive` (terminates the existing `backlogit mcp` OS process holding `mcp-daemon.lock` for the target workspace)
  * `Approval`: Pre-authorized by operator directive (`2026-10-29T20:06:47Z`); guarded at runtime by mandatory OS-lock contention check + `PID > 0 && PID != os.Getpid()` validation
  * `ActionResult`: `planned`

### 4. Review-Gate Capability and Degraded-Mode Handling

* `plan-review` must record literal `dispatch_mode:` (`multi-agent-dispatch` or `single-agent-declared-degradation`) and `decision:` (`PASS` or `ADVISORY` with `operator_authorization: approved`) in `## Plan Review` before harvest.

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * `backlogit mcp [--replace]` CLI entrypoint (`internal/cli/root.go`, `internal/cli/mcp_daemon.go`)
  * `backlogit mcp status [--format text|json]` CLI subcommand
  * `backlogit mcp stop [--format text|json]` CLI subcommand
* **Runtime verification expectations**:
  * Build the fresh binary (`go build -o backlogit.exe ./cmd/backlogit`) and verify `backlogit mcp status --format json` and `backlogit mcp stop --format json` against an isolated temp workspace, plus full unit and cross-process subprocess tests (`go test -count=1 -race ./internal/cli/... ./internal/errors/...`).
  * Do NOT run `backlogit mcp stop` against the live repository workspace root during Ship execution, because the agent's own active `backlogit` MCP server is serving this workspace! Always pass an isolated temp `--cwd` when exercising `mcp`, `mcp status`, and `mcp stop` in tests and runtime verification.
* **Operational closure expectations**:
  * Produce `docs/closure/205-S-204-F-post-merge-closure.md` conforming to the `{shipment_id}-{feature_id}-post-merge-closure.md` filename contract, with top-level `closure_status`, `compaction_status`, and `conditions` (if `READY_WITH_CONDITIONS`).
  * Invoke `compact-context` (`target: all`) per P-020 before finalizing `compaction_status`.
  * **Rollback trigger and procedure**: If `backlogit mcp` fails to start for legitimate single-instance MCP hosts or causes CLI reference drift in CI, revert the `205-S` merge commit via `git revert -m 1 <merge_sha>` on a reviewed PR.

## Plan Review

* dispatch_mode: single-agent-declared-degradation
* TOOL_DEGRADED: reviewer-subagent-dispatch — single-agent persona pass
* decision: PASS
* attempt: 1
* operator_authorization: approved
* reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Security Lens Reviewer (complete sequential rubric pass across all 6 applicable personas).
* plan_hardening: `Requires plan hardening: yes` confirmed and satisfied by `## Plan Hardening` (Invariants I1–I5, learnings trace, and `strict-safety` `ProposedAction` / `ActionRisk` classifications).
* constitution_check: `Constitution Check: pass` confirmed; all 11 constitutional principles mapped with zero violations.
* findings:
  * P0: 0, P1: 0, P2: 0, P3: 2 (applied in place).
  * P3-1 (`Go Reviewer`, U3 `internal/cli/root.go`): Explicitly specify that `defer func() { _ = release() }()` is registered immediately after `AcquireMCPDaemonLock` succeeds and before `openMCPServer` runs so an `openMCPServer` error still releases `mcp-daemon.lock` and removes `mcp-daemon.json`. Applied in place in U3.
  * P3-2 (`Security Lens Reviewer`, U2 `internal/cli/mcp_daemon.go`): Check `os.Lstat` + `core.IsSymlinkOrReparsePoint` on `mcp-daemon.json` in `readMCPDaemonInfo` before reading so a symlink planted at the metadata sidecar leaf is rejected. Applied in place in U2.
* runtime_verification_and_closure: Runtime verification against an isolated temp `--cwd` (preserving the live workspace MCP server), `go run ./cmd/gen-docs docs/cli-reference` drift check, and post-merge closure artifact `docs/closure/205-S-204-F-post-merge-closure.md` with P-020 `compact-context` invocation are explicitly specified.

