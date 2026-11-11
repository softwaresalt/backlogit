---
title: "Stage Custody Handoff — 212-S / 211-F / 089-DL Stash EE98533F Absorption, Same-Contract U1j (211.021-T) & U1k (211.022-T) Follow-Ups, and Ordinal 12 Convergence Authorization (M22, S23)"
shipment_id: "212-S"
feature_id: "211-F"
deliberation_id: "089-DL"
phase: "stage-custody-handoff-m22-ordinal12-followup"
---

## Summary of Bounded Same-Manifest Amendment (`M22`, `S23`, Stash `EE98533F` Absorbed)

Under the operator's explicit `"Proceed."` directive (`2026-11-09T23:24:15Z`) following Wave 12 **Ordinal 11** (`bin/_rrb67e310a/wave12-ordinal11/result.json`, SHA-256 `66436878db38b61a41c3beeb3b5559a850362a9ff32405289c48b1dba4fd9040`, recorded in `docs/memory/2026-11-09/212-S-wave12-ordinal11-convergence-blocked.md` at `HEAD` `4389f80b5c3e59bb01b45e75136e9ce742b0604e`), Stage completed the bounded same-manifest (`M22`, `S23`) amendment for Feature `211-F` / Shipment `212-S` (`089-DL` / `D18`):

1. **Absorb and Archive Stash `EE98533F`**:
   * Archived deferred stash entry `EE98533F` via `backlogit_stash_archive(stash_id="EE98533F")` after completing the P-021 C5/C6 unconditional duplicate scan (`CLEAN DUPLICATE SCAN` across `.backlogit/stash.jsonl` and `.backlogit/archive/stash.jsonl`) and mandatory late-identifier reconciliation (`No late identifier found — terminal PR: N/A and review-thread ID: N/A stand`), absorbing both Ordinal 11 items as in-scope completions of `U1j` (`211.021-T`) and `U1k` (`211.022-T`).
2. **Same-Contract `U1j` (`211.021-T`, Preserving `status: done`) Follow-Up in `internal/core/shipment_ops_security_windows_test.go`**:
   * Updated `.backlogit/archive/211.021-T.md` (preserving `status: done` and commit `9b07a7f5de55879054f423db541cc418c0084376`) so `swapShipmentOpsDirectoryObject` (`internal/core/shipment_ops_security_windows_test.go:655`) retries `os.Rename(opsRoot, opsRoot+"-original")` using the same bounded exponential backoff (`1ms, 2ms, 4ms, 8ms, 16ms, 32ms` = `63ms` max total wait across `6` retries / `7` attempts when `isRetryableShipmentOpsError(err)` / `isTransientShipmentOpsWindowsError(err)` is true) so a transient Windows Defender minifilter handle on the just-written `ops/001-S.json` file does not fail `TestShipmentOpsJournalWrite_BindsCreateAndRenameToValidatedDirectoryObjectAfterPathSwap` with `rename .backlogit/ops -> .backlogit/ops-original: Access is denied`.
   * Scoped verification command: `go test -timeout=30m -count=1 -race -run '^(TestU1j_|TestUR1B_|TestP021ClaimSerialization_|TestShipmentOpsJournal|TestWriteShipmentLifecycleJournal_|TestNewWorkspace_Rejects)' ./internal/core`
3. **Same-Contract `U1k` (`211.022-T`, Preserving `status: done`) Follow-Up Across `6` Affected Files Total (`internal/core/gate/baseref.go`, `internal/core/gate/probe.go`, `internal/core/gate/probe_test.go`, `internal/core/barrier_seam_ast_test.go`, `internal/core/shipment_reconcile_evidence_test.go`, `internal/core/custody_exec_test.go`)**:
   * Updated `.backlogit/archive/211.022-T.md` (preserving `status: done` and commit `d6057cd315644a5b415c3fee6b576a5dfe757407`) to include:
     1. `internal/core/gate/probe.go` & `internal/core/gate/probe_test.go`: in `versionCacheKey(binary, dir string, childEnv []string)`, when `filepath.IsAbs(path)` is `true`, set `key.dir = ""` (only retaining `dir` when `!filepath.IsAbs(path)`), because `ExecVersionRunner{Binary: "autoharness", Dir: ws.RootPath}` resolves `autoharness` to an absolute `PATH` binary whose version output is independent of `ws.RootPath`; including `ws.RootPath` (`t.TempDir()`) in the key caused `execVersionCache` to miss on every test workspace (`0%` cross-workspace hit rate). Add a regression assertion in `TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity` proving that two `ExecVersionRunner` calls with the same `Binary` and different `Dir` (`t.TempDir()`) hit the cache (`counter == 1`).
     2. `internal/core/barrier_seam_ast_test.go`: in `TestU172_ItemLogBarrierSetterUnreachableFromProduction`, skip `name == "bin" || strings.HasPrefix(name, "_")` in addition to `strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules"` so `filepath.WalkDir(moduleRoot, ...)` does not walk `bin/_authoring_gomodcache` (`359,000` files, saving `227.8s` = `3.8 minutes`).
     3. `internal/core/shipment_reconcile_evidence_test.go` & `internal/core/custody_exec_test.go`: cache the static git repository seed in `setupShipmentReconcileEvidenceFixture` (`45` `git` commands) and `custExecNewRepo` (`7` `git` commands) using a process-wide `sync.Once` template directory under `os.TempDir()` and copy the template tree into each test's `t.TempDir()`, eliminating `2,400+` redundant `git.exe` subprocesses across `internal/core` (saving `~500s` = `8.3 minutes`).
   * Scoped verification commands:
     * `go test -timeout=30m -count=1 -race -run '^(TestU1k_|TestProbe|TestVersionAtLeast|TestExecVersionRunner_|TestResolveBaseRef|TestExecRunner_|TestResolveChildEnv_)' ./internal/core/gate`
     * `go test -timeout=30m -count=1 -race -run '^(TestU172_|TestShipmentReconcile|TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery)' ./internal/core`

## Wave 12 Census & Post-Follow-Up Ordinal 12 Convergence Authorization

* **Shipment `212-S` Manifest (`S23` = `23` items, `M22` = `22` tasks)**:
  * `terminal_success`: `22` tasks (`211.001-T` through `211.022-T`, all `done` and archived in `.backlogit/archive/`, `1` `WORK_STARTED` record each — never re-claimed, re-started, or re-completed).
  * `open_red_deliverables`: `∅` (`211.015-T` `^TestU2i_` closed GREEN at Wave 10).
  * `closed_exempt_set`: `["211.007-T"]`.
* **Post-Wave-12 Unfiltered Full-Suite Convergence Gate (Ordinal 12)**:
  * `ordinal`: `12`
  * `failed_count`: `10` (preserving all `10` prior failed full-suite outcomes without counter reset)
  * `budget_seconds`: `2700` (`1,800s` `-timeout=30m` per-package budget)
  * `discovered_packages`: `46` first-party packages (reconciled complete unfiltered first-party repository census)
  * `command`: `go test -count=1 -p=1 -timeout=30m -json ./...`

## SHA-256 Digests of Stage-Authored Files (Excluding Self-Digest)

| File Path | SHA-256 Digest |
|---|---|
| `.backlogit/archive/211.021-T.md` | `f103a8db978f4fde01e063c6cf0bde3d2c2f5c788e99b0df51159861c7d04ad3` |
| `.backlogit/archive/211.022-T.md` | `8afccf07709fdcaa108ed58aa388449820fec808d011afc076570354c801eae0` |
| `.backlogit/archive/089-DL.md` | `af353527950d8b0deeb2e9d1dce07a22a8e5a71beee0a57646ce2e7750092406` |
| `docs/decisions/2026-11-04-multi-session-mcp-runtime-deliberation.md` | `2a5cae3382b1fba351c9c9cf8ccce0cdd14c7cad295ed90b7c260f81fe925c41` |
| `docs/plans/2026-11-04-multi-session-mcp-runtime-plan.md` | `bf467055cf8a12381147b4b293b5954141ed63f67860bb356ae182bfc296d0d1` |
| `.backlogit/queue/211-F.md` | `ee9b337e7c81147971735e16806760a7279c36d21950a5e635bb21d1227d2784` |
| `.backlogit/queue/212-S.md` | `f08fdaf6af7af3e978db49769e37be49b3a6b61353cfec81bdfa4ed65e02e8fe` |
| `.backlogit/stash.jsonl` | `c4739bb01a652e48ad044a6009f8dc4079ea22f26f307560199b4d16316fbbba` |
| `.backlogit/archive/stash.jsonl` | `2ac560de3871e9d73adb2471670260c68323065fefb10e71e08e57bfad0514db` |
| `.backlogit/hooks_queue.jsonl` | `872a1ce28feca337547ba7b15e2543c2c5ec78b84201d161a3d2f1948ad00737` |

## Custody and Boundary Attestations (`NOFEATURECOMMIT`)

* `.backlogit/archive/211.021-T.md` and `.backlogit/archive/211.022-T.md` preserve `status: done` and their existing commit references (`9b07a7f5de55879054f423db541cc418c0084376` and `d6057cd315644a5b415c3fee6b576a5dfe757407`).
* Zero Git commits, staging (`git add`), or pushes were performed by Stage (`NOFEATURECOMMIT`).
* Zero Go source/test files or checkpoint files were modified.
* Protected checkpoints `.backlogit/checkpoints/checkpoint-20261110-014835.json` (`30e466e0230964bc59a7b74aed254035345e97379693cf630330e058e9beb229`) and `.backlogit/checkpoints/checkpoint-20261106-210317.json` (`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`) remain untouched on disk.
