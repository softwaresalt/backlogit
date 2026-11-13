---
title: "213-S post-merge runtime verification"
doc_type: closure
source: docs/closure/213-S-post-merge-runtime-verification.md
ingested_at: "2026-11-13T07:16:50Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: 213-S
feature_id: 212-F
merge_commit: b470866d02819c978216eb14c90e6a97dd805f11
verification_verdict: PASS_WITH_FOLLOW_UP
---

## Contract and scope

Invoked `runtime-verification` with surface `auto`, mode `manual`, and target
`internal/toolchain` after PR 647 merged at `2026-11-13T06:35:19Z`.
The profile's `runtime_validation` contract declares `surfaces: []`,
`validation_expectations.required: false`, `minimum_verdict: PASS`, and no
expected surfaces, invariants or release blockers. Releasability is required;
monitoring-plan, rollback-procedure and owner each have `required: false`.

The inferred surface is a pure internal Level 1 API library. No registered
validator exists; the skill's optional-contract fallback selects the manual
adapter with command-backed existing test evidence. This is not an HTTP,
browser, live CLI/MCP launcher, or doctor validation. Those consumers remain
future `214-S` / U12 work.

## Environment prechecks

Verified branch `post-merge/213-s-closure`, HEAD
`8fdede0043af465349fe620a4c611733c48e0110`. Parent supplied safe local-main
synchronization and governed archive completion. Git comparison found no
tree changes at all between the feature CI head
`a4f44381baf901c6c88df14ce5058d303f71e239` and merge commit.
The later closure/archive commit is not claimed to have feature-PR CI.

Used the exact environment and selector in
[pre-merge verification](213-S-pre-merge-runtime-verification.md):
unset `GOMAXPROCS`, `GOGC`, `GOMEMLIMIT`;
`GOFLAGS=-count=1 -p=1 -parallel=2`;
`GIT_CONFIG_COUNT=1`, `GIT_CONFIG_KEY_0=core.fsmonitor`,
`GIT_CONFIG_VALUE_0=false`; `GOMODCACHE=C:\Users\dewilliams\go\pkg\mod`;
`GOPATH=C:\Source\GitHub\backlogit\bin\_authoring_gopath`;
`TEMP` and `TMP` were `C:\Source\GitHub\backlogit\bin\_t1`.
PATH began with workspace `bin` and its cached Go 1.24.0 toolchain.

No new source or fixture definitions were written. The existing test harness
created and cleaned its repository-local fixtures. No service, port,
credential, browser or live deployment was needed. Engram PID 49128 stayed
stopped; no restart or indexing was attempted.

## Structured validator evidence

```text
go test -timeout=30m -json -run '^TestU9_(VerifyWorkspaceToolchainAcceptsPinnedLocalToolsAndReportsPrereqsAndRemote|VerifyWorkspaceToolchainRejectsGlobalPATHShadowToolsAndCrossWorkspacePaths|LoadToolchainManifestRejectsDuplicateYAMLKeys|ValidateMCPConfigPinningRejectsDuplicateJSONKeys)$' ./internal/toolchain
```

| Probe | Expected and observed |
| --- | --- |
| Pinned local tools, host and remote classes | Existing positive assertions passed |
| Shadow/global and cross-workspace declarations | Existing rejection assertions passed |
| Manifest YAML and static MCP JSON | Valid parsing and duplicate-key refusal passed |
| Non-vacuity | 4 top-level tests, 10 subtests; 14 run and 14 pass events |
| Failure/skip counts | 0 failures; 0 skips |
| Process | Exit 0; package elapsed 1.686s |
| Actual execution | 2026-11-13T07:14:17.6328332Z to 2026-11-13T07:15:01.1071778Z |
| Cleanup | 0 remaining `internal\toolchain\u9-fixture-*` directories |
| Manual checkpoints | None required for the selected library scope |
| Blocked prerequisites | None for these probes |

These assertions exercise `VerifyWorkspaceToolchain`, `LoadToolchainManifest`
and `ValidateMCPConfigPinning`. Static declaration/path/hash checks do not prove
binary authenticity, host trust, OS/container isolation or live consumption.
`OSContainerSandboxDeferred=true` remains an invariant.

## CI and actual observation window

At this invocation's remote check, PR 647's final head `a4f44381` had all eight
checks SUCCESS in runs `43378256123` and `43378256381`. The merge-tip
`Line endings guard` succeeded at `2026-11-13T06:35:43Z`, run `43379748161`.
Go CI is PR-only: merged-tip Go jobs are **not applicable**, not fabricated
passes. Exact feature/merge tree equality makes the successful feature CI
source-equivalent, not a new merged-tip Go run.

The pre-merge plan's actual 30-minute window was
`2026-11-13T06:35:19Z` through `2026-11-13T07:05:19Z`; it was not restarted.
LineEndings completed within it. This agent began after the deadline and
performed the smoke test afterwards, so no continuous observation or
within-window smoke is claimed. Retrospective outcome: sampled checks healthy,
late smoke PASS, no observed rollback trigger. There is no live-service
telemetry for this library-only release.

## Verdict and handoff

`PASS_WITH_FOLLOW_UP`: the selected existing probes pass without vacuity.
Deferred consumers and the 21 adjudicated follow-ups stay unchanged; they are
not missing prerequisites for this optional library validator.
`6FADB54A` stays Stage/operator-adjudicated P2/out of scope;
`EC589C00` stays Level 2 provenance; relative-root semantics reuse `8AD5625B`.

Parent Ship owns the manual smoke/check review and operator handoff. Any
assertion failure, skip, unexpected count, CI failure or incorrect static
acceptance triggers a hold on downstream adoption and operator disposition.
Rollback is an operator-authorized corrective PR or merge-commit revert under
normal gates, never a history rewrite or shipment disposition.
The [canonical closure](213-S-212-F-post-merge-closure.md) carries releasability,
monitoring ownership and unchanged discovery qualifications.
