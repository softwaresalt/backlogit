---
title: "213-S pre-merge runtime verification"
doc_type: closure
source: docs/closure/213-S-pre-merge-runtime-verification.md
ingested_at: "2026-11-13T06:00:05Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-13"
  mode: pre-merge
  shipment_id: 213-S
  feature_id: 212-F
  pr: 647
  evidence_head: 9dd89b3b576ec3893783aafa84466d9a084789c6
  verification_verdict: PASS_WITH_FOLLOW_UP
---

## Scope and validator contract

The affected surface is the standalone `internal/toolchain` Level 1 API
library. Live CLI/MCP launch and doctor consumption belong to future
`214-S`, not this verification. Risk review must precede U11 (`213.001-T`),
the first CLI consumer: `VerifyLauncherScripts` / `AuditRepositoryToolchain`
and `mcp status --toolchain` / `--strict`. U12 (`213.002-T`) separately
provisions `WriteToolchainManifest` / `RenderPinnedMCPConfig` and E2E isolation.

Loaded `.autoharness/workspace-profile.yaml`, lines 597-615:

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
    - kind: monitoring-plan
      required: false
    - kind: rollback-procedure
      required: false
    - kind: owner
      required: false
```

`surface=auto` infers an API library, but no manifest entry exists. The
skill's optional-contract fallback selects the manual adapter. Existing Go
tests provide command-backed manual evidence; they are not a registered
HTTP/RPC validator. The configured PASS floor does not block this optional
validation; `PASS_WITH_FOLLOW_UP` records explicitly deferred consumption.

## Environment prechecks

* Confirmed current feature branch, clean initial tree and exact evidence HEAD
* Existing Go package and U9 assertions were available; no ports, credentials,
  server, browser, deployed binary or external service were required
* Daemon PID 49128 remained off; no daemon lifecycle operation was performed
* No new fixture/source definitions were authored; the existing U9 harness
  created its normal repository-local scratch fixtures and cleaned them up
* No shipment-dispose operation, CLI/MCP launch or doctor probe was invoked

Reproduction environment: unset `GOMAXPROCS`, `GOGC`, `GOMEMLIMIT`;
`GOFLAGS=-count=1 -p=1 -parallel=2`; `GIT_CONFIG_COUNT=1`,
`GIT_CONFIG_KEY_0=core.fsmonitor`, `GIT_CONFIG_VALUE_0=false`;
`GOMODCACHE=C:\Users\dewilliams\go\pkg\mod`;
`GOPATH=C:\Source\GitHub\backlogit\bin\_authoring_gopath`;
`TEMP` and `TMP` set to `C:\Source\GitHub\backlogit\bin\_t1`.
Prepend `bin` and
`bin\_authoring_gomodcache\golang.org\toolchain@v0.0.1-go1.24.0.windows-amd64\bin`
to PATH, relative to the repository root.

## Structured validator evidence

Executed at evidence HEAD:

```text
go test -timeout=30m -json -run '^TestU9_(VerifyWorkspaceToolchainAcceptsPinnedLocalToolsAndReportsPrereqsAndRemote|VerifyWorkspaceToolchainRejectsGlobalPATHShadowToolsAndCrossWorkspacePaths|LoadToolchainManifestRejectsDuplicateYAMLKeys|ValidateMCPConfigPinningRejectsDuplicateJSONKeys)$' ./internal/toolchain
```

| Evidence | Expected and observed result |
| --- | --- |
| Pinned local classification | Valid report, zero violations; host, remote and built-in classes retained |
| Unsafe executable/config paths | Missing binary/pin, bare command, cross-workspace paths, traversal, digest mismatch and shadow config refused |
| Manifest YAML loading | Valid load accepted; duplicate YAML key rejected |
| MCP config parsing API | Valid static config accepted; duplicate JSON key rejected |
| Non-vacuity | 4 top-level tests + 10 subtests; 14 run/pass events |
| Process result | Exit 0; 0 failures; 0 skips; package elapsed 1.916s |
| Cleanup | No `internal\toolchain\u9-fixture-*` directories remained; tree stayed clean |

The assertions call `VerifyWorkspaceToolchain`, `LoadToolchainManifest` and
`ValidateMCPConfigPinning`. They verify static declarations, paths and
expected hashes, not binary authenticity, actual sandboxing, host executable
trust, live MCP behavior or launcher automation.

## Invariants and manual checkpoints

* Preserve Level 1 classification and `OSContainerSandboxDeferred=true`
* Preserve tested rejection of unpinned/global and cross-workspace declarations
* Preserve tested duplicate-key refusal and explicit violation findings
* Keep remote/host prerequisites declarative, not authenticated or sandboxed
* Do not elevate the root-equivalence shortcut to proven containment for every
  spelling; `6FADB54A` remains an adjudicated P2 residual
* No operator-assisted runtime checkpoint is required for this library-only
  scope; future live integration is unverified, not silently passed

## Verdict and releasability handoff

`PASS_WITH_FOLLOW_UP`: meaningful existing API probes passed with no blocked
prerequisite for the selected scope. Preserve all 21 existing follow-ups and
their discovery qualifications; no new finding or capture was generated.
Live consumption, renamed-interpreter provenance and relative-root semantics
remain deferred, not repaired by these probes.

The [pre-merge readiness record](213-S-pre-merge-readiness.md) carries the
monitoring plan, owner, validation window, rollback procedure and Stage's
adjudicated risk inventory. This is historical evidence at `9dd89b3b`; it
does not certify CI or review for a future documentation commit.
