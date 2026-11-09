---
title: "212-S M19 Wave 8 U5 execution"
shipment_id: "212-S"
feature_id: "211-F"
phase: "wave8-u5-missing-shutdown-api-halt"
---

## Admission

The immutable exact-ID M19 snapshot contains 16 done and three active
claim-assigned members, with zero queued, blocked, unsupported, indeterminate
or active residual members. Only `211.005-T` is ready; dependencies
`211.004-T`, `211.008-T` and `211.019-T` are done.
Two shipment reads agree on S20, and 212-S is the sole active shipment.
Metadata/database root attestation and no-follow item-log validation passed.
Tasks 005, 006 and 007 each retain zero current-epoch starts.

The tracked scheduler replay passed 279/279 assertions across 38 scenarios.
The live catalog has ten status tokens; the registry mapping/features
were parsed by the replay. The restored M19 and empty green-regression
arrays are unchanged. U2i remains open RED through wave 10.

## Harness

Harness commit `15ac1682d0a658039f1ac87c065bfc5eb1aa0bd4` adds only
`internal/cli/mcp_daemon_multisession_test.go`, SHA-256
`cf3a85a8abe5a3ab3fc1f297a95e3e58b449afc1c7c40cd0a0eb1ea474c4492d`.
Compilation: PASS. Red Phase: CONFIRMED.

All three required named U5 tests fail behavioral assertions against U5a
stubs. There are no new skips, panics, timeouts or race reports.
Five mismatch cases and cold/live non-exclusive probes execute.
Stream and idle checks remain behind fail-fast stub guards.
Compile, vet, pinned lint and scoped format passed; U5a, U1b and
RegistryParity guards remained green. Zero production edits occurred.
The harness-ready label and exact command/evidence were recorded.

The task-scoped command is
`go test -timeout=30m -count=1 -race -v -run '^TestU5_' ./internal/cli`.
Its post-build output must show all three top-level witnesses passing
and execute the stream, graceful drain, live status and idle assertions
that were unreachable behind the RED guards.

## Execution boundary

Careful and freeze-scope posture permits production edits only to
`internal/cli/mcp_daemon.go`. Preserve the harness and existing U1b
absolute leaf-validation helpers. Do not edit `root.go` or implement U6.
U5 is already claim-assigned; do not move it to active again.
Verify and record exactly its first start before dispatch.
Green-regression and sibling-red arrays are empty; the sole tolerated
open selector is `^TestU2i_`. No full-suite run is authorized before U7.

## Build prerequisite halt

The per-task database check and a fresh no-follow log read showed zero
starts. The governed comment append and post-read established exactly
one `WORK_STARTED: 212-S` before any build dispatch.
The claim-assigned task was not moved again. Telemetry begin was disabled;
the lifecycle topology gate passed.

The build-feature delegate used one of five U5 implementation observations
to re-confirm the named RED baseline, then halted before production edits.
Compile passed. The build delegate ran no post-halt vet/lint/format.
Ship's final continuity checks subsequently passed whole-repository vet
and pinned lint; they do not establish U5 GREEN or wave convergence.
The unchanged production file `internal/cli/mcp_daemon.go` has SHA-256
`9aef6f7c133d1d604cec9dd3d9b375bf24b1aa6ab7f73d55f6a8d4fa2391f349`
and Git blob `4559fc3394b7c6611ebdb627a8f3abc7843d1bfc`.

The missing prerequisite is confirmed by the authoritative U5 contract:

* Plan line 649 requires bounded IPC stop to trigger both
  `s.CloseAllSessions()` and `s.coordinator.Close()` before termination
  fallback
* `internal/mcp/session_mux.go:543-574` explicitly stops admission,
  cancels sessions and joins their drains, but intentionally does not
  close the coordinator because direct InvokeTool callers also use it
* `internal/mcp/server.go:53,102` owns and constructs that coordinator
  privately; no exported server-owned coordinator shutdown API was found

The plan does not explicitly require coordinator closure on every idle
or context-return path; that broader claim is not the reason for this halt.
The concrete gap is the prescribed combined graceful-stop drain.
Creating another coordinator in CLI would not close the server's instance.
Changing the documented session-only semantics would also require review.

Adding an MCP-package API exceeds U5's sole authorized production path,
`internal/cli/mcp_daemon.go`. Ship made no such change and authored no
plan amendment. Stage must prepare the prerequisite or clarify the contract.
A first read-only architecture assessment lacked terminal access and did
not prove the prerequisite; a bounded Go evidence pass supplied the exact
contract and lifecycle references above. Neither was an implementation retry.

P-005 halt telemetry was recorded. P-021 C2 capture `0D073C21` records the
missing API with all six fields and `DISCOVERY-STATUS: LOOKUP-UNAVAILABLE`:
the targeted stash index had zero matches, but complete archived-stash and
prior residual-risk coverage could not be established. No PR or review
thread exists; both identifiers were correctly unavailable at capture.
The entry was re-read successfully and will not be edited or duplicated
by Ship. Stage owns deliberation and any reprioritization.

## Preserved handoff

M19 census remains 19 total: 16 done, zero queued, three active,
two claim-assigned, one started cursor (005), zero blocked,
unsupported or indeterminate members. Dependency impact is 006 -> 007.
No return-blocked operation applies because no task entered blocked status.

U5 is active with one start and one of five observations consumed.
Its RED harness is committed and must not be re-scaffolded.
No production delta, U5 completion, review readiness or Wave 8 convergence
is claimed. No `FULL_SUITE_DEFERRED: wave 8` PASS was emitted.
U2i was last re-confirmed assertion RED at Wave 7, closes at Wave 10,
and retains original baseline `30908ecb8438c0b3e7fe8bd514fa3680e35d90e5`.
The one authorized post-U7 full-suite run remains unconsumed.

Wave 9 and Wave 10 are not admitted. U6 and U7 remain claim-assigned with
zero starts. Resume only after Stage/operator disposition, preserving
this started cursor rather than reclaiming, restarting or resetting it.
Do not run initial shipment intake again.

Compact-context `target: all` was invoked as a bounded winddown
assessment. The active cursor, frozen manifest, counters, verdicts and
recovery pointers are retained. Current active-release records are not
archive candidates; no plan or closure artifact is eligible for graduation.
Repository-wide candidate assessment is degraded, with zero files archived
and zero bytes claimed recovered. The installed observe skill is absent;
recurring-signal capture is explicitly degraded rather than fabricated.

This record does not claim implementation, review or Wave 8 convergence.
Residual D2FCA0FB and native POSIX runtime follow-up remain carried
from the [Wave 7 acceptance](212-S-M19-wave7-u4-accepted.md).
Deferred entry `0D073C21` must join them in eventual PR and closure
residual-risk records. No push, PR, merge, closure, branch switch or
successor shipment mutation occurred.

Final recovery pointer: `checkpoint-20261109-062504.json`, re-read as
valid and conforming. This session's superseded checkpoints 060326 and
061412 were resolved individually; the protected historical checkpoint
was not changed. Selection and confirmation remain required on restore.
