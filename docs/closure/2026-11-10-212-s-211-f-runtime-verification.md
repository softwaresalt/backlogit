---
title: "212-S / 211-F pre-merge runtime verification"
date: "2026-11-10"
created_at: "2026-11-10T17:19:55Z"
mode: pre-merge
shipment_id: 212-S
feature_id: 211-F
reviewed_head: 2af30b0a4b5dbdaddc41595dcb0a13d4f3cc1079
verification_verdict: BLOCKED
---

## Scope and verdict

**BLOCKED** for meaningful end-to-end verification of the singleton MCP
runtime, independent stdio sessions, and channel operation coordinator.
Read-only CLI discovery smoke passed. This is not a failing runtime probe,
nor evidence that the implementation is defective. Safe smoke alone does
not demonstrate session isolation, operation ordering, or lifecycle recovery.

The operator authorized pre-merge verification only. No full suite was rerun,
fixture was created, daemon was started/stopped, shipment was disposed, or
backlog mutation was requested. No helper process requiring cleanup was
started. No merge readiness or approval is implied.

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
cannot establish the core runtime invariants in this bounded session.
The observed overall verdict does not meet the configured minimum `PASS`.

## Environment prechecks

| Check | Evidence | Outcome |
| --- | --- | --- |
| Source identity | `git rev-parse HEAD` returned the frontmatter HEAD | PASS |
| Starting tree | `git status --short` returned no entries | PASS |
| Artifact identity | `go version -m .\bin\tmp\_backlogit-212s.exe` | PASS |
| Toolchain/platform | Embedded `go1.24.0`, `GOOS=windows`, `GOARCH=amd64` | PASS |
| Binary source stamp | `vcs.revision` equals HEAD; `vcs.modified=false` | PASS |
| Binary SHA256 | `eee8d440e2743dbe0a576477de411ca9f10457d849d3430766ec60dc5fd054bc` | Recorded |
| Served roots | Operator supplied prior read-only MCP metadata attestation: workspace `C:\Source\GitHub\backlogit`, storage `.backlogit` | Inherited, not re-attested |
| MCP read reachability | Configured `backlogit_get_item(id="211-F")` succeeded | PASS, narrow read only |
| Served binary equals candidate | Not established by that read or the prior root attestation | UNVERIFIED |
| Non-Windows environment | No authorized non-Windows runtime available here | BLOCKED checkpoint |
| Operator-assisted runtime scenarios | No recorded manual confirmation supplied | BLOCKED checkpoint |

Build metadata establishes provenance of the provided executable, not
reproducible-build equivalence or identity of an already-running daemon.
No credentials, browser tooling, or external services were needed.

## Structured validator evidence

| ID | Surface / adapter | Exact probe | Expected and observed | Native exit |
| --- | --- | --- | --- | --- |
| RV-01 | CLI / command supplement | `.\bin\tmp\_backlogit-212s.exe --help` | Root usage and command catalog rendered | 0 |
| RV-02 | CLI/MCP / command supplement | `.\bin\tmp\_backlogit-212s.exe mcp --help` | Stdio entry, daemon, prewarm, status, stop, and replace help rendered; no server launched | 0 |
| RV-03 | CLI/MCP / command supplement | `.\bin\tmp\_backlogit-212s.exe manifest --help` | Manifest usage and JSON tools/list-compatible output contract rendered; manifest execution not performed | 0 |
| RV-04 | MCP / read-only configured tool | `backlogit_get_item(id="211-F")` | Exact feature returned, including provenance | Tool success |
| RV-05 | CLI/API/background-job / manual | Independent sessions, coordinator ordering, disconnect/cancellation, recovery | Not executed; required environment/authorization for meaningful isolated checks not established | Not run |

RV-04 proves only configured-tool reachability and feature retrieval. It
does not validate the candidate executable's stdio shim or transport.
RV-03 validates help, not actual schema enumeration or handler execution.

### Existing suite evidence

The only full-suite receipt used is Ordinal18. It is automated test
evidence, not a substitute for missing manual runtime evidence.

* Command: `go test -count=1 -p=1 -timeout=30m -json ./...`
* Receipt: `bin\_rr7c19a6f2\wave12-ordinal18\result.json`
* SHA256 independently checked:
  `6bd4d4e23d0c41b8bd72771f23b3faaa172549b6af556871e5e5da4b2546e4a2`
* Receipt HEAD before/after: frontmatter HEAD
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
| MC-01 | Concurrent stdio clients share one canonical workspace/store daemon without cross-session response leakage | Suite receipt only; no candidate end-to-end observation | BLOCKED |
| MC-02 | Conflicting operations serialize; read/mutation boundaries and cancellation preserve durable state | Suite receipt only; no isolated manual exercise | BLOCKED |
| MC-03 | Disconnect/reconnect and lifecycle recovery do not terminate unrelated clients or processes | Not exercised against shared daemon | BLOCKED |
| MC-04 | Applicable non-Windows IPC, locking, containment, and lifecycle behavior | Windows environment only | UNVERIFIED |

These are verification gaps, not newly configured mandatory profile
surfaces. Resolve them with bounded, explicitly authorized owned fixtures
or supplied operator/CI evidence. Never exercise destructive lifecycle
operations against the served workspace, and never run shipment disposal,
including on fixtures.

## Safety and retrieval record

```yaml
ProposedAction:
  summary: Inspect candidate provenance and read-only CLI help
  targets:
    - 'bin\tmp\_backlogit-212s.exe'
    - '211-F via configured read-only MCP tool'
  change_kind: read-only
  rollback: No runtime state change intended; no rollback required
  approval_required: false
ActionRisk: low
ActionResult: applied
```

`mcp status` was deliberately not executed. Source confirmation in
`internal\cli\mcp_daemon.go` shows `InspectMCPDaemonStatus` calls
`probeMCPDaemonInfo`, whose stale-holder path can acquire a cleanup lock.
This is a safety exclusion, not an observed status failure.

Workspace structural discovery used
`bin\engram.exe --workspace C:\Source\GitHub\backlogit workspace-status`
and `search 'newMCPStatusCmd' --region code --limit 3`.
Status confirmed this workspace/branch and `stale_files: false`, with
scanning still running; indexed results were used only to locate source.
Known exact source paths were then inspected directly.
No index/binding/store mutation was requested.

## Operational handoff

Canonical monitoring, rollback, ownership, and release conditions are in
[the pre-merge closure record](2026-11-10-212-s-211-f-closure.md).
Monitoring is proposed, not activated; rollback has not been rehearsed.

Next action: obtain bounded evidence for MC-01 through MC-04 without
rerunning the full suite. Then reassess the verdict against minimum
`PASS`. Current-HEAD standard/adversarial review, hosted CI/Copilot, and
merge approval remain separate pending gates.
