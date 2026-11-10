---
title: "Stage Custody Handoff — 212-S / 211-F / 089-DL Same-Contract U6 (211.006-T) Sidecar Replacement Retry, Wave 12 U1k (211.022-T) Gate Subprocess Isolation & Probe Caching, and Ordinal 11 Convergence Amendment (M21 -> M22, S22 -> S23)"
shipment_id: "212-S"
feature_id: "211-F"
deliberation_id: "089-DL"
phase: "stage-custody-handoff-m22-wave12"
---

## Summary of Bounded Plan and Manifest Amendment (`M21 -> M22`, `S22 -> S23`)

Under the operator's explicit `"Proceed."` directive (`2026-11-09T23:24:15Z`) following Wave 11 **Ordinal 10** (`bin/_rr9501da8a/wave11-ordinal10/result.json`, SHA-256 `4f0915989d1a4a56b549264917ad47d5f9954b4e89e9729d2c3450b6a4c77c70`, recorded in `docs/memory/2026-11-09/212-S-wave11-ordinal10-convergence-blocked.md` at `HEAD` `b6fdacfd2a017d49c058de6f0d5fcc30e388d1a1`), Stage completed the bounded `M21 -> M22` (`S22 -> S23`) amendment for Feature `211-F` / Shipment `212-S` (`089-DL` / `D18`):

1. **Same-Contract `U6` (`211.006-T`, Preserving `status: done`) Follow-Up in `internal/cli/mcp_daemon.go` & `internal/cli/mcp_session_shim_test.go`**:
   * `readMCPDaemonInfoAt` in `internal/cli/mcp_daemon.go` retries `Lstat -> rejectLink -> IsRegular -> Open -> Stat -> os.SameFile` up to `4` attempts (with `2ms` sleep between attempts, closing `f` before sleeping/retrying) when `!os.SameFile(opened, linked)` while `opened.Mode().IsRegular()` is true, and `PrewarmMCPDaemon` treats a concurrent regular-file `"changed while opening"` as a retryable readiness-poll condition rather than a fatal `ErrValidation`, while still failing closed immediately with `corerrors.ErrValidation` on any symlink, reparse point, or non-regular file.
   * Scoped verification command: `go test -timeout=30m -count=1 -race -run '^(TestU6_|TestU5a_|TestU4_CrossProcess|TestRegistryParity_)' ./internal/cli`
2. **Wave 12 Prerequisite Task `211.022-T` (`U1k`, `ready_12 = {211.022-T}`)**:
   * **`211.022-T` (`U1k`)**: *Isolate ExecGitRunner.Verify from ancestor checkouts under workspace-contained TEMP and cache ExecVersionRunner.Version probes (U1k)*
     * `parent_id`: `211-F`, `status`: `queued`, `priority`: `high`, `size`: `XS`, `complexity`: `low`, `labels`: `code,track-b,core,gate,windows,u1k`
     * `dependencies`: `211.020-T, 211.021-T` (`dep_type: "blocks"`, both `done` at `9b07a7f5de55879054f423db541cc418c0084376`, placing `211.022-T` in Wave 12)
     * **Affected files (`3` files)**: `internal/core/gate/baseref.go`, `internal/core/gate/probe.go`, `internal/core/gate/probe_test.go` (modifies existing methods `ExecGitRunner.Verify` and `ExecVersionRunner.Version` with zero new exported symbols, so `harness-architect` scaffolds behavioral RED tests `TestU1k_*` in `internal/core/gate/probe_test.go` directly in Wave 12)
     * **Design & Acceptance Criteria**:
       1. In `internal/core/gate/baseref.go` (`ExecGitRunner.Verify`): when `g.Dir != ""`, if `g.Dir` is strictly inside `os.TempDir()` (`tempDir := filepath.Clean(os.TempDir())`) and no `GIT_CEILING_DIRECTORIES` entry is already present in `cmd.Env`, append `"GIT_CEILING_DIRECTORIES=" + tempDir` to `cmd.Env` so `git rev-parse --verify --quiet <ref>^{commit}` stops at `os.TempDir()` (`bin/_t1`) and never walks up into the outer repository checkout (`C:\Source\GitHub\backlogit\.git`) for non-git `t.TempDir()` workspaces (while still resolving normally for any test that runs `git init` inside its `t.TempDir()`).
       2. In `internal/core/gate/probe.go` (`ExecVersionRunner.Version`): cache successful version strings in an unexported package-level `sync.Map` (`var execVersionCache sync.Map`) keyed by `(resolvedBinaryPath, info.ModTime().UnixNano(), info.Size(), strings.Join(cmd.Env, "\x00"))` (resolved via `exec.LookPath(r.Binary)` + `os.Stat`), so repeated `Broker.Evaluate` calls across `1,195` tests in `internal/core` do not spawn a 1-second Python `autoharness version` subprocess on every task completion, while automatically invalidating if the binary path, modification time, size, or environment changes.
       3. Scoped verification command: `go test -timeout=30m -count=1 -race -run '^(TestU1k_|TestProbe|TestVersionAtLeast|TestExecVersionRunner_|TestResolveBaseRef|TestExecRunner_|TestResolveChildEnv_)' ./internal/core/gate`

## Wave 12 Census & Post-Wave-12 Ordinal 11 Convergence Authorization

* **Shipment `212-S` Manifest (`S23` = `23` items, `M22` = `22` tasks)**:
  * `terminal_success`: `21` tasks (`211.001-T` through `211.021-T`, all `done` and archived in `.backlogit/archive/`, `1` `WORK_STARTED` record each — never re-claimed, re-started, or re-completed).
  * `ready_12`: `{211.022-T}` (`1` `queued` task with `0` `WORK_STARTED` records).
  * `open_red_deliverables`: `∅` (`211.015-T` `^TestU2i_` closed GREEN at Wave 10).
  * `closed_exempt_set`: `["211.007-T"]`.
* **Post-Wave-12 Unfiltered Full-Suite Convergence Gate (Ordinal 11)**:
  * `ordinal`: `11`
  * `failed_count`: `9` (preserving all `9` prior failed full-suite outcomes without counter reset)
  * `budget_seconds`: `2700` (`1,800s` `-timeout=30m` per-package budget)
  * `discovered_packages`: `46` first-party packages (reconciled complete unfiltered first-party repository census)
  * `command`: `go test -count=1 -p=1 -timeout=30m -json ./...`

## SHA-256 Digests of Stage-Authored Files (Excluding Self-Digest)

| File Path | SHA-256 Digest |
|---|---|
| `.backlogit/archive/211.006-T.md` | `217b72b764935f835362af8e9c0a52f60de3232a42c03ecc2839d3aff618aab4` |
| `.backlogit/archive/089-DL.md` | `3b7690ddae4dc696594789771a42219f30dc32f7e27e89d27998af6c78a4f60a` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `32ba3b50f7b676e0e6d83c02b6250c651c620b4c7978d5dcd3d993db8dd02dce` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `f6b9e31e2307fdf7468085e5e8d87f22c07b683a79774911c93b2b7d528204ea` |
| `.backlogit/queue/211-F.md` | `4433eb6bfed8599bf3132cb3972dabba28cb5e70bfff8fc712c79ba8b1648abd` |
| `.backlogit/queue/212-S.md` | `142e30cc36917adeb8be4e40a1fd3410c901e69c051a0b01eadd9167e5944e54` |
| `.backlogit/queue/211.022-T.md` | `f974335bf5ec98f0198b34cf9942fd6631d7058dd33677ab679c511cb164fa34` |
| `.backlogit/logs/211.022-T.jsonl` | `ad74f865937d27136c97dbcafe87e4b1c93afec419d07cfa21e1b15e6b763df0` |
| `.backlogit/logs/212-S.jsonl` | `c077d3d940bec1decc78d5aaaa1cef484b978755633e64af30ef8090aca268d1` |
| `.backlogit/hooks_queue.jsonl` | `365591417dfda77c7c047382b4826c4d8b34189d8efbdba760a954229860a8f9` |
| `.backlogit/memories.json` | `8d6c53a1591c6d576e64c84e9a5b78db1ead62d4d2f32b78627f61d5708d5f45` |

## Custody and Boundary Attestations (`NOFEATURECOMMIT`)

* `.backlogit/archive/211.006-T.md` preserves `status: done` and its existing commit reference (`e6c2c62d`).
* Zero Git commits, staging (`git add`), or pushes were performed by Stage (`NOFEATURECOMMIT`).
* Zero Go source/test files or checkpoint files were modified.
* Protected checkpoints `.backlogit/checkpoints/checkpoint-20261109-224251.json` (`12a969161ffa5d2ed67314b44d1451ee216daf62b162bd8a9fd597add56fb122`) and `.backlogit/checkpoints/checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`) remain untouched on disk.
