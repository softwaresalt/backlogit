---
title: "212-S / 211-F pre-merge runtime verification"
doc_type: reference
source: "docs/closure/2026-11-10-212-s-211-f-runtime-verification.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-10"
created_at: "2026-11-10T17:19:55Z"
mode: pre-merge
shipment_id: 212-S
feature_id: 211-F
reviewed_head: b43a1f3ac0dbf1b128d3c2bd795dbabe7144adee
verified_at: "2026-11-10T18:29:02Z"
verification_verdict: PASS
---

## Scope and verdict

**PASS** for the bounded Windows CLI/MCP runtime checks below. MC-01 through
MC-03 passed with fresh candidate lifecycle and focused automated evidence.
MC-04 remains **PENDING**, delegated to hosted Linux GitHub Actions on the
PR; Windows named-pipe success does not prove POSIX UDS behavior.

This supersedes the earlier blocked smoke-only assessment. The operator
authorized creation and removal of the isolated probe and candidate.
Only those owned artifacts and these two closure documents were changed.
No live repository daemon was inspected or stopped. No Go, backlog,
planning, review, checkpoint, or memory artifact was changed; no full suite,
staging, commit, push, merge, or shipment disposal was performed.
PASS is a local verification verdict, not release or merge approval.

## Live validator contract

Source: `.autoharness\workspace-profile.yaml`, lines 597-615.

```yaml
validator_manifest:
  surfaces: []
validation_expectations:
  required: false
  surfaces_expected: []
  minimum_verdict: PASS
  preserve_invariants: []
  release_blockers: []
releasability:
  required: true
  status_when_satisfied: READY
  required_evidence:
    - {kind: monitoring-plan, required: false}
    - {kind: rollback-procedure, required: false}
    - {kind: owner, required: false}
```

Inferred affected surfaces are CLI and MCP RPC/session lifecycle, with
background daemon/coordinator behavior. With no manifest entries and
`required: false`, the installed skill selects the **manual** adapter.
The command probes below supplement that adapter; they do not invent a
manifest-backed command/HTTP/job validator. HTTP and browser adapters are
not applicable to this stdio/IPC feature.

The empty manifest is not itself a release blocker. The manual adapter
is supplemented by explicit command and focused test evidence, not by an
invented manifest-backed validator. The bounded local verdict meets minimum
`PASS`; outstanding hosted evidence remains a separate release condition.

## Environment prechecks

| Check | Evidence | Outcome |
| --- | --- | --- |
| Source identity | `git rev-parse HEAD` returned the frontmatter HEAD | PASS |
| Starting tree | Existing feature branch with documentation/backlog/checkpoint changes; preserved, not clean | Recorded |
| Go continuity | `git diff --name-only 2af30b0a4b5dbdaddc41595dcb0a13d4f3cc1079 -- '*.go'` returned no paths, exit 0 | PASS |
| Probe collisions | Candidate and isolated workspace both absent before creation | PASS |
| Containment | `git check-ignore` accepted both probe paths, exit 0 | PASS |
| Artifact identity | Fresh build and `go version -m` through the existing Go environment wrapper, exit 0 | PASS |
| Toolchain/platform | Embedded `go1.24.0`, `GOOS=windows`, `GOARCH=amd64` | PASS |
| Binary source stamp | `vcs.revision` equals current HEAD; `vcs.modified=true` honestly reflects existing dirty state | Recorded |
| Candidate/pinned SHA256 | Both equal `6239568e2d009543402f67aa79c2b579d567ae3623416040cc194d5f54e18b63` | PASS |
| Isolated roots | `C:\Source\GitHub\backlogit\bin\_t1\rv-212s`, storage `.backlog` | PASS |
| Non-Windows environment | Not run locally; hosted Linux PR evidence required | PENDING |

Build metadata establishes provenance, not reproducible-build equivalence
or identity of the production daemon. The pinned isolated executable was
a byte-identical copy. No credentials or external service were needed.
The existing `bin\tmp\212-go-env.ps1` supplied Go 1.24.0, local dependency
and build caches, offline/read-only module settings, and workspace-contained
`TEMP`/`TMP` at `bin\_t1`; the wrapper was not modified.

## Structured validator evidence

| ID | Surface / adapter | Exact inner command | Expected and observed | Native exit |
| --- | --- | --- | --- | --- |
| RV-01 | Build / command supplement | `go build -o bin/tmp/_backlogit-212s.exe ./cmd/backlogit` | Candidate built; stdout/stderr empty | 0 |
| RV-02 | CLI / command supplement | `.\bin\tmp\_backlogit-212s.exe --cwd bin\_t1\rv-212s init` | `Initialized backlogit workspace at bin\_t1\rv-212s\.backlog` | 0 |
| RV-03 | CLI/MCP lifecycle / command supplement | `.\bin\tmp\_backlogit-212s.exe --cwd bin\_t1\rv-212s mcp prewarm --format json` | Owned daemon running, PID 34776 | 0 |
| RV-04 | CLI/MCP lifecycle / command supplement | `.\bin\tmp\_backlogit-212s.exe --cwd bin\_t1\rv-212s mcp status --format json` | Same PID, roots, endpoint, and protocol; zero sessions | 0 |
| RV-05 | CLI/MCP lifecycle / command supplement | `.\bin\tmp\_backlogit-212s.exe --cwd bin\_t1\rv-212s mcp stop --format json` | Same PID stopped; `running: false` | 0 |
| RV-06 | MCP/stdin/IPC / focused automated supplement | `go test -count=1 -v -run '^TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival$' ./internal/cli` | Real child shims, isolation, persisted mutation, peer survival, graceful stop | 0 |
| RV-07 | Coordinator / focused automated supplement | `go test -count=1 -v -run '^TestU2_CoordinatorFIFOMutationOrderAndConcurrentReads$' ./internal/mcp` | FIFO mutation order, nonoverlap, four concurrent readers, exclusive barrier drain | 0 |

RV-01, RV-06, and RV-07 were each invoked separately as
`powershell.exe -NoProfile -File bin\tmp\212-go-env.ps1` followed by the
exact inner command above. All other lifecycle commands ran separately.
RV-07 was added because RV-06 alone does not demonstrate FIFO ordering
or concurrent-read progress. Neither is a full-suite rerun.

### Exact lifecycle responses

RV-03 and RV-04 returned identical JSON:

```json
{
  "running": true,
  "pid": 34776,
  "started_at": "2026-11-10T18:27:31Z",
  "workspace_root": "C:\\Source\\GitHub\\backlogit\\bin\\_t1\\rv-212s",
  "storage_root": "C:\\Source\\GitHub\\backlogit\\bin\\_t1\\rv-212s\\.backlog",
  "version": "1.12.1-0.20261110174240-b43a1f3ac0db+dirty",
  "lock_path": "C:\\Source\\GitHub\\backlogit\\bin\\_t1\\rv-212s\\.backlog\\runtime\\mcp-daemon.lock",
  "ipc_transport": "named_pipe",
  "endpoint": "\\\\.\\pipe\\backlogit-mcp-103dc24217dba784",
  "active_sessions": 0,
  "protocol_version": "backlogit-mcp-ipc/v1"
}
```

RV-05 returned:

```json
{
  "stopped": true,
  "running": false,
  "pid": 34776,
  "workspace_root": "C:\\Source\\GitHub\\backlogit\\bin\\_t1\\rv-212s",
  "storage_root": "C:\\Source\\GitHub\\backlogit\\bin\\_t1\\rv-212s\\.backlog",
  "lock_path": "C:\\Source\\GitHub\\backlogit\\bin\\_t1\\rv-212s\\.backlog\\runtime\\mcp-daemon.lock"
}
```

### Focused test observations

RV-06 emitted these witness and terminal lines:

```text
=== RUN   TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival
    mcp_session_shim_test.go:384: U6_WITNESS real_stdio_children=2 overlapping_id=1 daemon_pid=39520 killed_client_pid=45736 peer_survived=true stopped=true
--- PASS: TestU6_CrossProcessMultiSessionStdioShimAndDisconnectSurvival (6.70s)
PASS
ok  	github.com/softwaresalt/backlogit/internal/cli	7.218s
```

The test built its own current-source executable in a contained temporary
fixture; it did not reuse the hashed RV-01 candidate. Two real stdio child
processes initialized separate sessions on the same independent daemon.
Equal request ID `1` returned distinct mutation/version responses without
crossover. After killing only the first owned client, the same daemon PID
reported one session, and the peer read the persisted `u6-owned-mutation`.
Explicit daemon stop drained the peer, joined it cleanly, and checked stopped
runtime artifacts.

RV-07 emitted these terminal lines, with every displayed subtest passing:

```text
--- PASS: TestU2_CoordinatorFIFOMutationOrderAndConcurrentReads (8.10s)
PASS
ok  	github.com/softwaresalt/backlogit/internal/mcp	8.592s
```

Eight admitted mutations completed in order `[0,1,2,3,4,5,6,7]` without
overlapping mutating handlers. Four readers entered while the first mutation
held its turn. `sync_index` waited for those readers to drain before its
barrier handler ran. This is in-process coordinator evidence, not a fresh
cross-process FIFO stress/load test. The additional existing C1 fixture
subtests passed, including expected corrupt-cache fallback warnings.
No live backlog data was mutated.

### Existing suite evidence

The only full-suite receipt used is Ordinal18. Its hash was rechecked;
the suite was not rerun. It supplements, rather than replaces, fresh
bounded runtime evidence.

* Command: `go test -count=1 -p=1 -timeout=30m -json ./...`
* Receipt: `bin\_rr7c19a6f2\wave12-ordinal18\result.json`
* SHA256 independently checked:
  `6bd4d4e23d0c41b8bd72771f23b3faaa172549b6af556871e5e5da4b2546e4a2`
* Execution HEAD before/after:
  `2af30b0a4b5dbdaddc41595dcb0a13d4f3cc1079`
* Current source HEAD differs, but no Go file changed since execution
* Suite/native exit: 0; `evidence_valid: true`
* Package results: 46 passes; missing/extra packages empty
* Top-level passes: 3922; reported failures: 0
* Started: `2026-11-10T16:39:50Z`; finished: `2026-11-10T17:17:52Z`
* Elapsed: approximately 2282.113677 seconds
* Receipt records 78 skips; passing package totals do not prove every
  platform-specific scenario ran

### Manual checkpoints and invariants

| ID | Required observation | Current evidence | Disposition |
| --- | --- | --- | --- |
| MC-01 | Independent sessions share one daemon without response leakage | RV-06: two real stdio children, same daemon, overlapping ID 1, distinct payload routing | PASS |
| MC-02 | FIFO mutation serialization and concurrent-read/barrier boundaries | RV-07: eight ordered nonoverlapping writes, four concurrent reads, barrier after read drain; RV-06: persisted mutation visible to peer | PASS |
| MC-03 | Client disconnect survival and explicit graceful shutdown | RV-06: killed first client, unchanged daemon PID, surviving peer read, clean drain; RV-03 through RV-05: candidate lifecycle | PASS |
| MC-04 | Linux POSIX UDS, containment, locking, and lifecycle | Delegated to hosted Linux GitHub Actions on PR; no hosted result examined here | PENDING |

PASS applies only to the stated observations. Arbitrary cancellation,
reconnect-after-daemon-crash recovery, sustained load/resource bounds,
all peer-identity attacks, and cross-process FIFO stress were not newly
proven. No Windows run is represented as Linux/macOS/BSD evidence.
MC-04 remains a pre-merge release condition despite local PASS.

## Safety and retrieval record

```yaml
ProposedAction:
  summary: Build and verify an owned isolated runtime, then remove it
  targets:
    - 'bin\tmp\_backlogit-212s.exe'
    - 'bin\_t1\rv-212s'
    - 'focused test-owned fixtures under bin\_t1'
  change_kind: isolated runtime lifecycle and owned-artifact cleanup
  rollback: Stop only verified owned fixture processes, then remove owned artifacts
  approval_required: true
ActionRisk: destructive
approval: Operator explicitly authorized candidate/probe creation and removal
ActionResult: applied
```

`mcp status` ran only inside the owned isolated workspace, where any
cleanup-lock side effect was contained. The production daemon remained
untouched. No broad code/graph search or raw diff dump was needed; exact
contract/test paths and literal test-name lookups supplied confirmation.

After capture, `Get-Process` checks confirmed owned PIDs 34776, 39520,
and 45736 absent. The initial absence-only `Get-Item` returned exit 1
with no output; explicit `Test-Path` checks then confirmed no collisions.
An attempt to parse the bounded Ordinal18 raw log encountered its partial
final JSON line (exit 1); tolerant read-only parsing succeeded (exit 0)
but found no retained individual FIFO terminal event. No pass was inferred
from that missing event; RV-07 supplied fresh direct evidence.

`Remove-Item -LiteralPath bin\_t1\rv-212s,bin\tmp\_backlogit-212s.exe -Recurse`
completed successfully, exit 0. Subsequent `Test-Path` checks returned
`False` for both paths, exit 0. Only these explicitly authorized owned paths
were removed; test fixtures used their existing automatic cleanup.

## Operational handoff

Canonical monitoring, rollback, ownership, and release conditions are in
[the pre-merge closure record](2026-11-10-212-s-211-f-closure.md).
Monitoring is proposed, not activated; rollback has not been rehearsed.

Next action: obtain hosted Linux MC-04 and hosted CI evidence, complete
P-018 Copilot and P-014 current-HEAD readiness/explicit merge approval,
and acknowledge monitoring/rollback ownership. Stage's authoritative
review disposition and captured residual risks are carried in closure;
no finding was repaired or recategorized here.
