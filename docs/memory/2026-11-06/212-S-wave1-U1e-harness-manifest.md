---
title: 212-S Wave 1 U1e harness manifest
date: 2026-11-06
task_id: 211.011-T
feature_id: 211-F
shipment_id: 212-S
phase: harness-red
---

## Contract and scope

Compilation: PASS. Red Phase: CONFIRMED.
Stage baseline: `bf6bf2f6e3503c916dabb1ac40580e8ad9cc5ca8`.
Authority: amended plan U1e, D11 M1-M3, and
`stage-212-s-086-087-dl-u1cde-amendment.md` in this directory.
Closed siblings: `211.001-T` and `211.008-T`; neither is reopened.
Current ready set is exactly `211.009-T`, `211.010-T`, `211.011-T`.

Stage explicitly authorizes existing compiling legacy assertions as RED
witnesses. Both fixture files remain unchanged; no completion sequencing,
GateBroker corrections, or production changes have been made.
`blocking_cascade_test.go` is untouched and outside this task's scope.

## Executable boundary

Run from the repository root in a fresh PowerShell shell:

```powershell
. ./bin/tmp/212-go-env.ps1
./scripts/verify-212-wave1-harness.ps1 -TaskID 211.011-T -Phase Red
```

Build acceptance uses the same command with `-Phase Green` (the default).
Exact native selector:

```powershell
go test -timeout=30m -count=1 -race -json -run '^(TestDoctor_IgnoresIntentionalOrphans|TestShipmentWorkflow_ShipmentReleaseCleanup)$' ./internal/core ./tests/integration
```

Declared canonical tests and owned files:

* `TestDoctor_IgnoresIntentionalOrphans`:
  `internal/core/025_doctor_harness_test.go`
* `TestShipmentWorkflow_ShipmentReleaseCleanup`:
  `tests/integration/shipment_workflow_test.go`

The guard checks exactly one named run and terminal result per canonical test,
exact package identities/count (2), and zero skips. GREEN requires every
selected test and package to PASS, native exit 0. RED requires both
named assertion traces and the exact active-before-ship refusal, native exit 1.
Panic, build failure, races, undeclared tests, and empty selectors are rejected.
No invented API or top-level Go wrapper is used.

Baseline assertion nodes are compared, not whole-file checksums: legitimate
fixture corrections and new strengthening assertions are accepted. Original
Doctor and release-cleanup assertions cannot be removed, weakened, renamed,
or reduced in multiplicity. Original assertion-node counts: 12 and 26.

## Observed native proof

Final guard verification at 2026-11-06 06:08 UTC:
native exit **1**, guard exit **0**, 2 packages, 2 canonical runs, zero skips.
Both canonical tests FAIL through `require.NoError` assertions:

* `025_doctor_harness_test.go:98`
* `shipment_workflow_test.go:199`

Both report:

```text
ship shipment 001-S: shipment refused: member 001.001-T is active (not completed through the gate): gate blocked: 001.001-T remains active
```

These are Stage-authorized legacy witnesses. Their original downstream
Doctor and partial-release assertions remain verbatim, even though the
current fatal ship assertion prevents reaching them until fixture GREEN.

## Quality gates and handoff

Whole-repository compile before and after the U1c harness delta exited 0:
`. ./bin/tmp/212-go-env.ps1; go test -run=^$ -count=1 ./...`.
Whole-repository vet exited 0. Pinned v1 lint over core, policyengine, and
integration exited 0 with `Issues=[]`; scoped tracked-source gofmt was empty.
The [U1c manifest](212-S-wave1-U1c-harness-manifest.md) records exact shared
quality commands and the pinned offline Go 1.24.0 environment.

Stop before fixture GREEN. Later build-feature may change only the two
authorized fixtures, disabling the unconfigured broker and using governed
`UpdateArtifact` completion after claim and before ship. Never modify
`blocking_cascade_test.go` or production gates.
No claims, starts, moves, pushes, PRs, future-task scaffolding, or backlog staging.
