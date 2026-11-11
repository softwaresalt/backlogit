---
chunk_strategy: h1-h2-h3
description: "Consolidated root-cause analysis of Shipment 212-S Wave 7 persistence failure, 36,266.58s host-hibernation wall-time overrun, and orchestration overhead."
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-11-07-212-s-delivery-blockers-rca.md
title: "Shipment 212-S Delivery Blockers and Wave 7 Overrun Root-Cause Analysis"
---

## Executive Summary

Shipment `212-S` (`211-F`: Multi-Session Singleton MCP Runtime, Stdio Session Shim, and Channel Operation Coordinator, branch `feat/212-s-backlogit-multi-session-singleton-mcp-runtime-stdio-session-shim-and-channel-operation-coordinator` at `183ec2d52287a6465e4ef2e2952685f0ba0d7eb0`) halted after Wave 7 (`wave7-rearm1-183ec2d`) exited `124` at `36,266.58s` elapsed (`2,700s` configured budget) with `1` assertion failure in `internal/core` and `18/50` packages unexecuted.

Independent read-only root-cause investigations by Ship (source/persistence trace) and Stage (supervisor/host timing and orchestration trace) established three concrete findings:

1. **Source Persistence Failure (`96F1DFFD`, High Confidence)**: `TestNewWorkspace_RemovesWriterTempResidueAndRecoversValidJournal` (`internal/core/shipment_p1_lifecycle_remediation_test.go:88`) failed because startup legacy return-blocked recovery (`internal/core/shipment.go:2979-3014`) unconditionally rewrites the task file via `persistArtifact` $\rightarrow$ `MoveFileEx` (`internal/atomicfile/atomicfile_windows.go:50-53`) even after verifying at `lines 2979-3004` that the on-disk task already matches the desired journal preimage. The redundant physical rewrite is **proven**; the external or OS actor that denied `MoveFileEx` (`Access is denied`) was **not captured** and remains **unproven**.
2. **Wall-Time Overrun (`36,266.58s` vs `2,700s`, High Confidence)**: Windows `System` event logs prove the host entered **S4 Hibernation (`TargetState=5, EffectiveState=5`)** at `2026-11-07T10:59:55.045Z` (`1,828.05s` into test child `PID 10344`) and remained suspended for **`34,435.56s` (`9h 33m 55.6s`)** until `2026-11-07T20:33:46.500Z-20:33:50.601Z`. No user-space watchdog can execute during S4 hibernation; upon wake, `_supervise` (`bin/tmp/212-full-suite-cycle7.py:454-460`) immediately observed `now - start >= 2700` and terminated the job (`native_exit: 124`). The specific trigger that initiated sleep is **unknown** from sanitized structured event fields.
3. **Feature Delivery vs Orchestration Overhead (`High Confidence`)**: Reporting `7/12 done` overstates original feature progress. Only **`2/7` (`28.6%`) of the original requested MCP deliverables (`U1`, `U2`)** are complete alongside **`5/5` mid-flight prerequisites (`U1b`-`U1f`)**. Five original feature tasks (`U3`-`U7`) and successor shipments `213-S`-`215-S` remain unstarted, with `0` source PRs and `0` shipments delivered.

## Honest Delivery Progress Accounting

| Scope Category | Completed | Unstarted | Completion Share |
|---|---|---|---|
| Original `212-S` MCP Deliverables (`U1`-`U7`: `211.001-T`-`211.007-T`) | `2` (`211.001-T`, `211.002-T`) | `5` (`211.003-T`-`211.007-T`) | `28.6%` (`2/7`) |
| Added Prerequisites (`U1b`-`U1f`: `211.008-T`-`211.012-T`) | `5` (`211.008-T`-`211.012-T`) | `0` | `100.0%` (`5/5`) |
| Combined `212-S` Manifest (`M12`) | `7` | `5` | `58.3%` (`7/12`) |
| Queued Shipments Shipped (`212-S`-`215-S`) / Source PRs Opened | `0` | `4` shipments | `0.0%` (`0/4`) |

Parent retained Ship status recorded `2,796` completed tool calls across `~44h` wall-clock elapsed time; those figures represent wall-clock span (including host hibernation and waiting intervals) and tool invocations, not active CPU time or token consumption.

## Root Cause 1: Startup Legacy Journal Recovery Redundant Task Rewrite (`96F1DFFD`)

### Proven Call Path and Redundant Write

In `bin/_rr8b31a6d2/wave7-rearm1-183ec2d/result.json`, `internal/core` failed `TestNewWorkspace_RemovesWriterTempResidueAndRecoversValidJournal` at `2026-11-07T10:47:16Z` (`02:47:16` local PST; test duration `0.41s`, package duration `1,434.647s`):

```text
recover shipment operations: recover shipment 001-S return-blocked journal:
restore item 001.001-T from legacy journal: write artifact file:
move temp over destination: backlogit: durable write not applied: Access is denied.
```

Ship's read-only source trace established:

* **Fixture behavior (`internal/core/shipment_p1_lifecycle_remediation_test.go:68-85`)**: The test saves preimages for both shipment `001-S` and task `001.001-T` in a legacy `return-blocked` journal, mutates only shipment `001-S` membership on disk, leaves task `001.001-T` byte-unchanged on disk, and closes `Workspace`.
* **Recovery behavior (`internal/core/shipment.go:2979-3014`)**: During `NewWorkspace` auto-recovery, `recoverReturnBlockedLegacy` reads and validates the on-disk task against the journal preimage (`lines 2979-3004`), confirming the task file already matches the desired preimage. At `lines 3010-3014`, it restores the mutated shipment and then **unconditionally calls `persistArtifact` on the unchanged task**.
* **Atomic replacement failure (`internal/atomicfile/atomicfile_windows.go:50-53`)**: `persistArtifact` $\rightarrow$ `writeArtifactFileGoverned` $\rightarrow$ atomic writer writes a temporary file, closes it, and calls `MoveFileEx` over the existing task `.md` path, which returned `Access is denied` (`ErrWriteNotApplied`).
* **Lock and reader discipline**: Recovery holds the global, membership, and artifact mutation locks (`internal/core/shipment.go:2727-2743`, `2830-2850`), whose handles are `.lock` sidecar files rather than the destination `.md` artifact. Repository reads (`parseFile` and snapshot loaders) use `os.ReadFile`, which in Go 1.24 defers `Close()` before returning, and the atomic writer checks temporary-file `Close()` before `MoveFileEx`.

### Proven vs Unproven Attribution

* **Proven**: The physical rewrite of `001.001-T.md` during `TestNewWorkspace_RemovesWriterTempResidueAndRecoversValidJournal` is redundant because `shipment.go:2979-3004` has already verified under recovery locks that the on-disk task matches the target preimage.
* **Unproven**: The exact `Close()` return status on prior readers, target file attributes, and the process/handle/filter actor denying `MoveFileEx` were not captured in the failure output. Neither an internal reader handle leak nor an external OS/filesystem filter can be asserted as the proven denying actor, and Cycle 6's single full-suite pass (`sixth-6-b6d58147`) does not prove intermittence was absent.

### Candidate Minimal Repair and Regression Proof (Not Yet Implemented)

* **Candidate source repair (not yet implemented or verified)**: In `internal/core/shipment.go:2979-3014`, while holding the existing recovery locks and after preimage validation confirms the on-disk task already matches the desired preimage, skip the redundant physical file replacement for the unchanged task, still reconcile SQLite index state and remove the recovery journal only after all recovery steps succeed, and preserve fail-closed `ErrWriteNotApplied` whenever a genuinely mutated artifact fails atomic replacement.
* **Focused three-case regression design (not yet implemented or executed)**: Because `journal.Reason` is empty in the legacy fixture, `itemTargets` admits only the preimage and a mutated on-disk task fails `itemMatches` (`ErrShipmentConflict`) before either restore writer is invoked:
  1. **Unchanged task with injected task-restore writer denial**: Current recovery fails (`RED`), whereas idempotent skip-when-identical recovery performs no physical task write and succeeds (`GREEN`).
  2. **Admitted changed shipment (membership-removed state) with injected shipment-restore writer denial**: Recovery attempts the shipment restore write, fails closed with `ErrWriteNotApplied`, and retains the journal.
  3. **Non-admissible changed task**: Recovery fails the preimage guard (`itemMatches`) with `ErrShipmentConflict`, makes **zero** restore writer calls, and retains the journal.

## Root Cause 2: Host S4 Hibernation During Wave 7 Supervision (`F2509AE3`)

### Supervisor Clock and Job Mechanics

`bin/tmp/212-full-suite-cycle7.py` (`SHA256: 8de6d56e...`) and `bin/tmp/212-full-suite.py` (`SHA256: 8058bd5e...`) are identical except for line `535` (`"ordinal": 7, "failed_count": 5` vs `"ordinal": 5, "failed_count": 4`):

* `_OwnedJob` (`lines 253-332`) configures `LimitFlags = 0x2000` (`KILL_ON_JOB_CLOSE` at `line 294`) and terminates via `TerminateJobObject(self.handle, 124)` (`lines 324-326`).
* `_supervise` (`lines 334-516`) records `started = time.monotonic()` (`line 339`) and post-resume `start = time.monotonic()` (`line 440`), then polls `process.poll()` and `now = time.monotonic()` (`lines 445-446`) with non-blocking `PeekNamedPipe` reads (`lines 313-322, 461-473`). When `now - start >= timeout` (`_BUDGET = 2700` at `line 34`, checked at `line 454`), it sets `info["timed_out"] = True`, calls `job.kill()`, drains for `drain_seconds=2.0` (`line 459`), and records `info["elapsed_seconds"] = time.monotonic() - started` (`line 513`).

### Sanitized Windows System Power Event Evidence (`2026-11-07T10:20:00Z-20:40:00Z`)

The full 12-event sanitized query result is preserved at `.copilot\session-state\4a60e334-a29e-4db4-9f3a-be1e5f382920\files\rca-power-events.json` (`SHA256: a546b1b7ce87d0de84ffeb57fd4501bcaa325f6f4d511352f996cc97b67aa5be`, `7,214` bytes). The durable public subset below correlates the Wave 7 timestamps directly to kernel power transitions:

| UTC Timestamp | Source / Provider | Event ID | Sanitized Structured Evidence |
|---|---|---|---|
| `2026-11-07T10:29:14.7992909Z` | Wrapper (`PID 12796`) | — | Wrapper started; finished `20:34:23.2875587Z` (`36,308.49s` total wrapper wall span) |
| `2026-11-07T10:29:15Z` | Discovery (`PID 5372`) | — | `go list ./...` (`lines 606-608`) finished `10:29:27Z` (`elapsed_seconds: 12.311934`, `native_exit: 0`, `50` packages) |
| `2026-11-07T10:29:27Z` | Test Child (`PID 10344`) | — | `go test -count=1 -p=1 -timeout=30m -json ./...` (`lines 619-623`) resumed (`deadline_seconds: 2700`) |
| `2026-11-07T10:59:55.0445718Z` | `Microsoft-Windows-Kernel-Power` | `187` | `SystemAction=3, LightestSystemState=2` (sleep transition begins after `1,828.05s` / `30m 28s` of test execution; `32/50` packages done) |
| `2026-11-07T10:59:55.0453220Z` | `Microsoft-Windows-Kernel-Power` | `566` | `BootId=34, Reason=25, PreviousSessionId=132, PreviousSessionType=0, NextSessionId=133, NextSessionType=3, PowerStateAc=true, MonitorReason=25` |
| `2026-11-07T10:59:56.2849670Z` | `Microsoft-Windows-Kernel-Power` | `42` | `TargetState=5, EffectiveState=5, Reason=4, Flags=0, TransitionsToOn=34` (system entering S4 sleep/hibernation) |
| `2026-11-07T11:01:02.8982301Z` | `Microsoft-Windows-Kernel-Power` | `107` | `TargetState=5, EffectiveState=5, WakeFromState=5` (hibernation image write/resume boundary) |
| `2026-11-07T20:33:46.5009178Z` | `Microsoft-Windows-Kernel-General` | `1` | `OldTime=2026-11-07T11:01:02.8979581Z, NewTime=2026-11-07T20:33:46.5000000Z, TimeDeltaInMs=34363602, Reason=2` |
| `2026-11-07T20:33:50.6014034Z` | `Microsoft-Windows-Kernel-Power` | `566` | `PreviousSessionId=133, PreviousSessionType=3, PreviousSessionDurationInUs=34435556275` (`34,435.56s` = `9h 33m 55.6s`), `NextSessionId=134, NextSessionType=1, Reason=20, MonitorReason=20` |
| `2026-11-07T20:33:53Z` | Test Child (`PID 10344`) | — | `_supervise` wakes, observes `now - start >= 2700`, calls `job.kill()`, drains `2.0s`, exits `124` (`elapsed_seconds: 36266.58098970004`) |
| `2026-11-07T20:34:01.3996639Z` | `Microsoft-Windows-Power-Troubleshooter` | `1` | `SleepTime=2026-11-07T10:59:55.0451513Z, WakeTime=2026-11-07T20:33:59.4512037Z, SleepDuration=64946, WakeDuration=12948, HiberWriteDuration=34023, HiberReadDuration=64852, TargetState=5, EffectiveState=5, WakeSourceType=0` |

### Clock and Trigger Interpretation

* **Reconciled duration**: `1,828.05s` pre-sleep execution (`10:29:27Z` $\rightarrow$ `10:59:55.045Z`) + `34,435.56s` S4 hibernation (`PreviousSessionDurationInUs=34435556275`) + `2.97s` wake poll/kill/drain = **`36,266.58s`**.
* **Guard correctness on resume**: While the host was in S4 hibernation, no user-space watchdog could run. Python's QPC-backed `time.monotonic()` (independent of CMOS wall-clock adjustments) spanned the suspension interval and exceeded `2,700s` immediately on resume, so `_supervise` killed the job on wake as designed. Because the `2,700s` wall-time budget was exceeded and `internal/core` had already failed at `10:47:16Z`, termination on resume was required guard behavior.
* **Initiating sleep trigger (`Unknown`)**: Sanitized structured fields (`Kernel-Power 42` `Reason=4`, `Kernel-Power 566` `Reason=25`/`MonitorReason=25`, `PowerStateAc=true`) do not distinguish idle policy timeout from manual or application-initiated sleep. Absence of `SetThreadExecutionState` in `_supervise` is a mitigable idle-sleep exposure, not a proven cause of sleep initiation.

## Root Cause 3: Full-Suite Serialization and Orchestration Overhead

1. **Package-serial full-suite cost (`bin/tmp/212-full-suite-cycle7.py:619`)**:
   * `go test -count=1 -p=1 -timeout=30m -json ./...` serializes all `50` packages (`~41.7-61.4 min` per run; `internal/core` alone requires `1,434.6s-1,820.8s`).
2. **Cycle 4 (`61.4 min` wall time, `2026-11-06T19:57:11Z-20:58:37Z`)**:
   * `45/50` packages and all `U1f` concurrency receipts passed; `internal/core` failed after `1,820.79s`, but `_Recorder` dropped test-associated panic lines (`docs/memory/2026-11-06/212-S-fourth-convergence-result.md`), requiring a recorder fix and operator re-authorization.
3. **Cycles 5 and 6 (`84.19 min` combined test-child runtime across two runs; `~41.66 min` + rearm delay avoidable)**:
   * **Cycle 5 (`2026-11-07T06:07:14.659Z-06:49:59.828Z` wrapper = `42.75 min`; discovery `PID 12480` = `13.21s`; test child `PID 23988` = `2,551.59s` / `42.53 min`)**: Passed all `45` code packages (`internal/core` PASS) and failed solely in `tests` (`TestU175_001_BaselineEol`) due to `6` CRLF JSON receipts in `docs/memory/2026-11-06/212-S-R3-stash-busy-a91/*.json`.
   * **Cycle 6 (`2026-11-07T07:34:15Z-08:15:55Z` test child `PID 58968` = `2,499.46s` / `41.66 min`; discovery `PID 15212` = `28.80s`; wrapper ended `08:15:55.342Z`)**: Passed `50/50` packages on unchanged HEAD `d6b23ed3`. Two runs consumed `84.19 min` of test-child execution; because one successful `~42 min` full verification was still required, a pre-flight `TestU175_001_BaselineEol` check would have avoided the extra `~41.66 min` rerun plus rearm delay (not all `84.19 min`). Once Cycle 5 failed, Cycle 6 was mandatory because composite passes are not permitted.
4. **Accurate partitioning of the `08:16Z-10:29Z` post-Cycle-6 window**:
   * **`08:16Z-10:00Z` (`~1h 44m` active `U2` delivery)**: Legitimate `U2` (`211.002-T`) harness, implementation, review regression harness, and review-fix commits (`ad6732a7` at `08:52Z`, `568ded42` at `09:27Z`, `183ec2d5` at `09:35Z`; `docs/memory/2026-11-07/212-S-U2-done-await-wave-convergence.json` recorded at `10:00:06Z`).
   * **`10:00Z-10:29Z` (`~29 min` mixed prelaunch/coordination)**: Prelaunch `go list ./...` (`PID 43024`) timed out at `30.036s` (`line 606`) after `directory.mkdir(exist_ok=False)` (`line 589`) had already claimed `bin/_rr8b31a6d2/wave7-183ec2d`. That triggered **one** new supervised discovery diagnostic (`bin/_d7a3/receipt.json`, `PID 60276`, `10:24:54Z-10:25:14Z`, `20.0989s` elapsed, labeled `attempt: 3` across the `2` prior historical prelaunch failures; memory recorded `10:25:40Z`), followed by coordinator receipt retirement and `wave7-rearm1` re-issuance.

## Corrective Recommendations (Governed Authorization Required)

1. **Focused `96F1DFFD` Repair and Regression Proof Before Any Full Suite**:
   * Authorize Ship to implement the minimal idempotent legacy-journal task restore check in `internal/core/shipment.go:2979-3014` and prove it with the three-case regression suite (`unchanged-task writer denial` current RED / future idempotence GREEN; `admitted changed-shipment restore writer denial` $\rightarrow$ fail-closed `ErrWriteNotApplied` + retained journal; `non-admissible changed task` $\rightarrow$ `ErrShipmentConflict` + zero restore writer calls + retained journal) plus targeted `./internal/core` verification.
2. **Private Supervisor Hygiene (`F2509AE3`)**:
   * Assert process-scoped `SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED)` (`0x80000001`) during active `_supervise` child execution and restore `SetThreadExecutionState(ES_CONTINUOUS)` (`0x80000000`) in `finally:` (mitigates automatic idle sleep only; does not prevent manual sleep/hibernation or power loss).
   * Preserve the `2,700s` wall-time budget (`line 454`) without subtracting poll gaps (which could mask starvation/blocking), and record `max_poll_gap_seconds` separately in `supervision` metadata.
   * Run targeted `TestU175_001_BaselineEol` as a mandatory pre-flight check before any full suite. Preserve unique per-attempt evidence directories (`line 589`, including prelaunch failures so `go-list.raw`, `go-list.stderr.raw`, and `go-list-result.json` remain captured inside each attempt's exclusive directory), decouple test-execution receipt consumption from prelaunch attempt state (where no test allowance is consumed), and use a stable immutable driver reading expected `ordinal`/`failed_count` from coordinator receipt data to avoid source-copy/rearm rituals.
3. **Governed Gate-Cadence Disposition**:
   * Targeted test passes do not unilaterally discharge the mandatory unfiltered full-suite gate. Recommend operator authorization to either (a) verify the `96F1DFFD` repair and complete `U3`-`U7` under scoped package/race gates + pre-flight `TestU175_001_BaselineEol`, running the unfiltered `-p=1` 50-package suite once at the final branch/PR boundary after `U7`, or (b) re-arm the Wave 2 unfiltered suite only after `96F1DFFD` and supervisor hygiene are verified.
