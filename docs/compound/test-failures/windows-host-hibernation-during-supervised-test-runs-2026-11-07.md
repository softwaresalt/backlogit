---
chunk_strategy: h1-h2-h3
description: "Windows S4 hibernation freezes user-space test supervisors across multi-hour sleep intervals and triggers immediate deadline termination on wake."
doc_type: learning
schema_version: "1.0"
source: docs/compound/test-failures/windows-host-hibernation-during-supervised-test-runs-2026-11-07.md
title: "Correlate Windows power events and inhibit idle sleep during long supervised test runs"
problem_type: test_failure
category: test-failures
component: windows-test-supervisor
root_cause: Host entered S4 hibernation after 30.5 minutes of unattended execution while no process-scoped execution-state request was held
resolution_type: harness_fix
severity: high
message: Supervised go test timed out with exit 124 and 36266.58s elapsed against a 2700s deadline
file_path: bin/tmp/212-full-suite-cycle7.py
citations:
  - docs/decisions/2026-11-07-212-s-delivery-blockers-rca.md
  - docs/memory/2026-11-07/212-S-wave7-rearm1-failure-checkpoint.json
tags:
  - windows
  - hibernation
  - kernel-power
  - supervision
  - 212-S
---

## Problem

During Shipment `212-S` Wave 7 (`wave7-rearm1-183ec2d`), the private Python supervisor (`bin/tmp/212-full-suite-cycle7.py`) launched `go test -count=1 -p=1 -timeout=30m -json ./...` (`PID 10344`) at `2026-11-07T10:29:27Z` with a configured `2,700s` (`45m`) deadline. The run terminated at `2026-11-07T20:33:53Z` with `native_exit: 124`, `timed_out: true`, `32/50` packages completed, and `elapsed_seconds: 36266.58098970004` (`10h 04m 26.58s`).

## Root Cause

Bounded read-only inspection of the Windows `System` event log (`10:20:00Z-20:40:00Z`) established that the host entered **S4 Hibernation (`TargetState=5, EffectiveState=5`)** at `2026-11-07T10:59:55.045Z` (`1,828.05s` / `30m 28s` into the test run; `Microsoft-Windows-Kernel-Power` `EventID=187`, `566`, `42`) and remained suspended for **`34,435.56s` (`9h 33m 55.6s`)** until `2026-11-07T20:33:46.500Z-20:33:50.601Z` (`Microsoft-Windows-Kernel-General` `EventID=1`, `Kernel-Power` `EventID=566`, `Microsoft-Windows-Power-Troubleshooter` `EventID=1`).

While the machine is in S4 hibernation:

1. No user-space thread or watchdog loop (`_supervise` `lines 444-473`) can execute or poll pipe/process state.
2. Python's `time.monotonic()` on Windows (QPC-backed, independent of wall-clock adjustments) spans the suspension interval alongside UTC wall time (`1,828.05s` pre-sleep + `34,435.56s` sleep + `2.97s` wake/kill/drain = `36,266.58s`).
3. On the first poll iteration after wake, `now - start >= timeout` (`2,700s`, `line 454`) evaluated true, and `_supervise` immediately terminated the Windows Job Object (`TerminateJobObject(handle, 124)`).

## Resolution

1. **Correlate multi-hour wall-time jumps against Windows power events**: Query `Microsoft-Windows-Kernel-Power` (`EventID` `42`, `107`, `187`, `506`, `507`, `566`), `Microsoft-Windows-Power-Troubleshooter` (`EventID` `1`), and `Microsoft-Windows-Kernel-General` (`EventID` `1`) over the bounded UTC execution window using sanitized structured `EventData` fields before attributing an overrun to a hung pipe read or broken watchdog.
2. **Inhibit automatic idle sleep during active child supervision**: Call `ctypes.windll.kernel32.SetThreadExecutionState(0x80000001)` (`ES_CONTINUOUS | ES_SYSTEM_REQUIRED`) while the supervised child is running and clear it with `SetThreadExecutionState(0x80000000)` (`ES_CONTINUOUS`) in `finally:`. Note that process-scoped execution state inhibits automatic idle sleep only; it does not prevent manual sleep/hibernation, lid/power-button actions, or power loss.
3. **Preserve the wall-time budget and record poll gaps separately**: Do not subtract arbitrary `>5s` poll gaps from the configured deadline (which could mask CPU starvation or blocking and silently extend the authorized timeout). Keep the wall-time deadline unchanged and emit `max_poll_gap_seconds` as separate diagnostic metadata.
