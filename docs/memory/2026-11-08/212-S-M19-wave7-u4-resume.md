---
title: "212-S M19 Wave 7 IPC execution"
shipment_id: "212-S"
feature_id: "211-F"
phase: "wave7-u4-identity-producer-prerequisite-halt"
status: "blocked"
---

## Admission

Wave 6 completed in source `bafdf383` and lifecycle `1381ca66`.
The frozen M19 exact-ID snapshot contains 15 done and four active,
all four claim-assigned, with zero queued, blocked, unsupported or
active residuals. Only 004 is ready; prerequisites 003 and 018 are done.
No frozen member or empty green-regression array changed.

MCP workspace/storage metadata and database attestation passed.
Two shipment reads agreed on S20 and sole active shipment 212-S.
The session-scoped P-012 raw-log declaration remains limited to logs
003-007. No-follow, reparse-rejecting reads verified one shipment claim
and zero starts for each remaining task 004-007.

## Harness and start

Harness commit `d708efe4` adds only
`internal/cli/mcp_ipc_test.go`; production stubs are untouched.
Compilation: PASS. Red Phase: CONFIRMED.
Three new named top-level assertions failed; 16 new results comprised
12 failures and four negative-control passes, with zero new skips.
The broader U4 selector also finds one legacy pass and one legacy
helper skip; neither substitutes for the new witnesses.
Compile, vet, pinned lint, scoped format, U4a and Linux cross-compilation
passed. POSIX native execution is not claimed.

The task carries harness-ready and its exact command/evidence comment.
The per-task database check passed. A no-follow pre-read showed zero
starts; the governed comment append and post-read established exactly
one `WORK_STARTED: 212-S`. The already claim-assigned task was not moved.
Telemetry begin returned disabled. Lifecycle topology passed before build.

## Scope and pending gates

Careful plus freeze-scope mode: production edits only
`internal/cli/mcp_ipc_windows.go` and `internal/cli/mcp_ipc_unix.go`.
The harness is immutable after scaffolding. Risk is moderate and
Option 5 authorizes this work; no concurrent writer is known.
No destructive or external-workspace operation is authorized.

Dispatch command is
`go test -timeout=30m -count=1 -race -v -run '^TestU4_' ./internal/cli`,
with explicit verification that all three new named witnesses execute.
Green-regression array and sibling RED list are empty; the only open
RED selector remains U2i, closing at wave 10. U4a stays green.

## Build and review halt

The two-file implementation draft consumed two of five build-loop
observations: attempt one assertion RED, attempt two race-harness GREEN.
All three new named witnesses executed and passed, with zero new skips.
Evidence includes four clients and 12 frames, six endpoint refusals,
five identity cases, an exited child, positive controls and a live Windows
DACL probe. U4a, compile, pinned lint, Windows/Linux vet, scoped format
and Linux cross-compilation passed. Native POSIX execution is not claimed.
Broad cache-traversing format exits 2 and is not reported as passing.

Standard report-only review is BLOCKED, P0=0/P1=1/P2=3/P3=0:

* P1: exact process identity cannot be verified on non-procfs POSIX hosts
* P2: reject unclean Unix endpoints before bind; cleaned-parent comparison
  alone permits symlink-assisted escape through `x/..`
* P2: preserve direct `io.EOF` on Windows peer disconnect
* P2: preserve direct `net.Error.Timeout()` behavior for deadlines

Adversarial report-only review with three independent reviewers found
no unanimous findings. Majority findings corroborated wrapped EOF and
added identity-checked cleanup after failed post-bind validation.
All confirmed findings are same-contract completions under P-021 C1.
Unproven short-write/cancel-failure observations are not new blockers.
Process-wide umask is explicitly prescribed, not an independent defect.
Potential live-listener displacement requires caller-exclusivity evidence;
it was not treated as an established new admission-policy requirement.

The first review-fix dispatch halted before any edit or test attempt.
`internal/cli/mcp_daemon_lock_unix.go:43-63`, the existing identity
producer, returns constant `1` when `/proc/<pid>/stat` is unavailable.
On macOS this supplies no real creation-time identity. A verifier cannot
recover an exact match from the missing information.
Correcting that producer requires an out-of-scope file. Ship refuses
the expansion and returns the missing prerequisite to Stage.

No third implementation attempt was consumed; U4 remains 2/5.
No confirmed review fix is claimed. No task done transition,
Wave 7 convergence or later-wave admission occurred.
In particular, `FULL_SUITE_DEFERRED: wave 7` is NOT recorded as a
successful gate. The open U2i obligation was last re-confirmed RED
at Wave 6; the single post-U7 full-suite authority is unconsumed.

## Preserved cursor and draft

M19 census: 15 done, zero queued, four active, three claim-assigned,
one started cursor (004), zero blocked statuses, zero unsupported.
Dependency impact is 005 -> 006 -> 007. No return-blocked operation
applies because the member has not entered blocked status.
P-005 halt telemetry and the detailed task comment were recorded.

U4 remains active with exactly one start. Do not reclaim, append another
start, re-scaffold, or silently start a fresh task cycle after amendment.
The new harness is committed; production remains deliberately uncommitted
because the review gate is blocked. Preserve the two-file draft in place.

| File | Git working blob | SHA-256 |
|---|---|---|
| `internal/cli/mcp_ipc_windows.go` | `120ded18489cf5a40cafcaa113fa948288848dde` | `54afaf6efdbd4cd85d81ea8f11d9460b9d12bde84d1cf8f4b8b3cbf225b5d2cb` |
| `internal/cli/mcp_ipc_unix.go` | `a642f0384bb7266a6f9528828055311409f829db` | `435826ce869b81e5107501472e44be93eb8d63ec990cd5ce76986c699fb141f0` |
| `internal/cli/mcp_ipc_test.go` | `129c4bd723c09c43f3764a59c9d26cee27dc1ca8` | `ccdb7b5e77ddd8c295d9b81d6df1df567ae1517dc7705f718c25343c96d8bc45` |

No source delta occurred during either review or the refused fix dispatch.
The latest source HEAD is harness commit `d708efe4`; subsequent continuity
commits may advance HEAD without changing these source identities.

Next action is Stage/operator disposition for the missing producer
prerequisite. Stage must preserve the started cursor and existing harness.
Waves 8-10 cannot proceed until U4 is accepted, done and converged.
No push, PR, merge, closure, branch switch or successor mutation occurred.

Residual `D2FCA0FB` from U3 must remain in eventual PR and closure
records. The original historical counters and protected checkpoint
remain unchanged. See the [Wave 6 record](212-S-M19-wave6-u3-resume.md).

Protected checkpoint re-hash remains
`235844507a4975f27c5a88f4fe3e499fb538110c86e6a264637db1b69837ca70`.
Only the selected prior checkpoint and this session's own superseded
checkpoint may be resolved. All unrelated hooks, memories, historical
checkpoints and prior final-handoff files remain preserved.

Compact-context target all is invoked as a bounded winddown assessment.
Active cursor, frozen manifest, gate verdicts, counters and the unresolved
pointer remain unpruned. Repository-wide candidate assessment is degraded;
no global consolidation or archive is claimed. The installed observe
skill path is absent, so observation capture is explicitly degraded.
