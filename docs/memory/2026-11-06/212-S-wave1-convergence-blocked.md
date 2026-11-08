---
title: Shipment 212-S wave 1 convergence requires a Stage prerequisite amendment
date: 2026-11-06
shipment_id: 212-S
feature_id: 211-F
phase: wave1-convergence-blocked
status: blocked
---

## Result and preserved scope

Shipment 212-S is not shipped or closed. The existing feature branch remains
active; no other shipment was claimed. No push, PR, merge, runtime rollout,
shipment safe-close, or post-merge closure occurred.

The confirmed restore and bounded summary are recorded in
[the recovery note](212-S-confirmed-resume.md).
The selected checkpoint was successfully resumed through U1 implementation;
its original lint blocker is superseded, not retried.

## Completed task and evidence

U1, `211.001-T`, completed its declaration-only contract:

* Harness commit: `e8fbefb71fa277d387348d45f12377cd7fda2f8d`
* Declaration commit and current HEAD:
  `bcc2809027077752449d8bf38792e2f9f93504a6`
* Protected AST test SHA-256 remained unchanged
* Executable command: `& ./scripts/verify-u1-harness.ps1`
* Actual compile PASS, assertion RED, then named-test GREEN
* Parent independently rechecked all three named scenarios GREEN
* Go 1.24 compile, vet, pinned v1 lint, and source-format gates passed
* Constitution Reviewer, Go Reviewer, and Learnings Researcher returned
  zero concrete findings; P0/P1/P2/P3 counts are 0/0/0/0
* Review readiness is task-local, not release readiness
* Telemetry begin was disabled; no epoch close was attributed
* Official completion gate passed and commit tracking linked the task

Completion moved the task's tracked queue file to an untracked archive file.
Preserve both sides of that tool-owned lifecycle change; do not restore the
queue file manually or delete the archive.

## Wave census and counters

Frozen manifest `M` remains exactly:
`211.001-T`, `211.002-T`, `211.003-T`, `211.004-T`, `211.005-T`,
`211.006-T`, `211.007-T`.

| Census | Count |
| --- | --- |
| Wave | 1 |
| Frozen task members | 7 |
| Terminal success | 1 |
| Queued | 0 |
| Active | 6 |
| Claim-assigned | 6 |
| Active residuals | 0 |
| Blocked task statuses | 0 |
| Unsupported statuses | 0 |

U1 has exactly one valid `WORK_STARTED: 212-S` record in its current claim
epoch. U2-U7 were not started, scaffolded, or dispatched. U2 is dependency
ready but cannot be admitted until wave 1 convergence passes.
There are no red-deliverable mappings and no tolerated-red selectors.

Session counters: one task attempted, zero consecutive task failures,
one wave-convergence failure, no review fixes, no CI-fix cycles.
The full suite ran once. One distinct narrow test reproduced its root cause.
No retry of the full suite or failed lint combinations occurred.

## Full-suite failure

The required unfiltered `go test -timeout=30m ./...` used cached Go 1.24.0
with read-only modules, downloads disabled, and workspace-contained caches.
The CLI package finished FAIL after 311.753 seconds. Sixteen top-level
daemon-related tests failed, including:

* `TestU3_StopMCPDaemon_NotRunning`
* `TestU2_AcquireLockAndContention`
* `TestU3_StopMCPDaemon_TerminatesVerifiedHolder`
* `TestU4_CrossProcessMCPDaemonLockAndStop`

The bounded captured output is
`bin/tmp/1793926978890-copilot-tool-output-7128-22f620ff-2014-470e-a9ab-0d2c65b9cd29.txt`.
The process was stopped after confirmed package failure and root-cause
diagnosis; remaining packages did not provide a complete final inventory.
This is not a full-suite PASS or a completed all-package run.
No failed assertion was skipped, weakened, or reclassified as expected RED.

The diagnostic leaf executed exactly one narrow reproduction:

```text
go test -timeout=30m -count=1 -v -run '^TestU3_StopMCPDaemon_NotRunning$' ./internal/cli
```

It exited 1 with the same missing-path error.

## Verified root cause and safe alternatives considered

At `internal/cli/mcp_daemon.go:177`,
`storageHandle.OpenRoot("runtime")` creates a nested pinned root.
Go 1.24's Windows implementation preserves that relative name.
Consequently, `runtimeRoot.Name()` is `"runtime"`, not an absolute path.

At lines 222 and 243, a lock created and statted successfully through the
pinned handle is subsequently validated through the relative
`runtime\mcp-daemon.lock` path. `rejectLink`, lines 91-94, calls
Windows reparse validation. `internal/core/workspace_reparse_windows.go`
uses `GetFileAttributes`, which resolves that path against process CWD.
It queries another location and returns `ERROR_PATH_NOT_FOUND`.
Sidecar paths at lines 270-275 and 314 share the construction.

The holder helper exits before printing readiness when acquisition fails,
explaining the downstream EOFs. This is not a missing-cache, permission,
or missing-file error-classification issue.

Changing temporary directories, CWD, or cache paths cannot repair this
identity mismatch. Precreating a CWD lock would validate the wrong object.
Ignoring missing-path attribute errors would weaken fail-closed checks.
Toolchain replacement, assertion edits, skips, and foreign-host actions
were not used.

## Required Stage decision

The current U1 contract authorizes only declarations in `errors.go` and
`coordinator.go`. The production repair touches the later U5 daemon surface.
P-002.6 requires the current wave to converge before any later task is
scaffolded or implemented. Ship cannot amend the plan, frozen manifest,
dependencies, or acceptance criteria, or move U5 work ahead of this gate.

Stage must authorize a bounded prerequisite path-validation repair and its
regression harness before the blocked convergence gate resumes. Preserve
handle pinning, reparse refusal, and path/handle identity verification;
do not bundle U5's broader daemon behavior or waive the full suite.

Existing deferred entry `C2CFE06E` was positively confirmed by official read
to describe this same missing-path/EOF surface. Reuse that stable identity
in the handoff; no duplicate entry, stash edits, triage, or archival occurred.
Captures `C6E224B8`, `46BD6AEE`, and `259A4F71` remain untouched and are not
current supported-toolchain lint blockers.

## Review, tooling, and knowledge state

Engram restore-time MCP daemon, memory query, and health calls succeeded.
Coordinator CLI symbols, map-code, and impact calls later each failed
Ready30s; no same-operation retry, rebind, managed-graph edit, or unknown
host-process termination occurred. Reviews received explicit degraded
structural context. This later retrieval degradation does not invalidate
the earlier successful restore-time read.

The `observe` skill file is absent despite the installed learning pack.
Manual observation capture is supported by its instructions: this note and
the following compound records capture verified recurrence without inventing
an observation schema or altering harness-managed learning state.

* [CI toolchain remedy](../../compound/build-errors/match-ci-go-and-linter-before-source-repair-2026-11-06.md)
* [Nested-root path diagnosis](../../compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md)

No branch-wide multi-model adversarial review or Copilot review is claimed:
no code PR was submitted. Those gates remain mandatory before submission.
Runtime verification, final full build, required CI, implementation merge,
and all release closure gates remain pending.

## Resumption boundary

The sole current Ship recovery pointer is
`checkpoint-20261106-011453.json`, officially read back as valid and
conforming. The successfully resumed predecessor
`checkpoint-20261106-002340.json` was individually resolved; no other
checkpoint was resolved or pruned.

Keep this branch and worktree intact. Do not reclaim 212-S or write a
second U1 start record. Do not claim 213-S.
The next action is the Stage prerequisite amendment, not another unchanged
lint or full-suite retry. Resume at the existing wave 1 convergence cursor
only after authorized repair and genuine RED-to-GREEN evidence.
