---
title: 212-S Wave 1 U1d harness manifest
date: 2026-11-06
task_id: 211.010-T
feature_id: 211-F
shipment_id: 212-S
phase: harness-red
---

## Contract and scope

Compilation: PASS. Red Phase: CONFIRMED.
Stage baseline: `bf6bf2f6e3503c916dabb1ac40580e8ad9cc5ca8`.
Authority: amended plan U1d, D11 M1-M3, and
`stage-212-s-086-087-dl-u1cde-amendment.md` in this directory.
Closed siblings: `211.001-T` and `211.008-T`; neither is reopened.
Current ready set is exactly `211.009-T`, `211.010-T`, `211.011-T`.

Stage explicitly authorizes existing compiling legacy assertions as RED
witnesses. All three fixture files remain unchanged; no fixture isolation
corrections or production changes have been made. They remain authorized
mutable for build-feature, with original assertion nodes preserved.

## Executable boundary

Run from the repository root in a fresh PowerShell shell:

```powershell
. ./bin/tmp/212-go-env.ps1
./scripts/verify-212-wave1-harness.ps1 -TaskID 211.010-T -Phase Red
```

Build acceptance uses the same command with `-Phase Green` (the default).
Exact native selector:

```powershell
go test -timeout=30m -count=1 -race -json -run '^(TestHeadSHABounded|TestRepairShipmentMemberEvidence_NoRepoHead_Errors|TestU163_002_ExecGitRequiresARepositoryDirectory)$' ./internal/core ./internal/policyengine
```

Declared canonical tests and owned files:

* `TestHeadSHABounded`: `internal/core/shipment_gate_headdrift_test.go`
* `TestRepairShipmentMemberEvidence_NoRepoHead_Errors`:
  `internal/core/shipment_repair_test.go`
* `TestU163_002_ExecGitRequiresARepositoryDirectory`:
  `internal/policyengine/u163_002_record_test.go`

The guard checks exactly one named run and terminal result per canonical test,
exact package identities/count (2), and zero skips. GREEN requires every
selected test and package to PASS, native exit 0. RED requires all three
named assertion traces and their exact domain failure markers, native exit 1.
Panic, build failure, races, undeclared tests, and empty selectors are rejected.
No invented API or top-level Go wrapper is used.

Baseline assertion nodes are compared, not whole-file checksums: legitimate
fixture corrections and new strengthening assertions are accepted. Original
nodes cannot be removed, weakened, renamed, or reduced in multiplicity.
Original assertion-node counts: 5, 7, 2 respectively.

## Observed native proof

Final guard verification at 2026-11-06 06:08 UTC:
native exit **1**, guard exit **0**, 2 packages, 3 canonical runs, zero skips.
All three canonical tests FAIL through compiling assertions:

* `shipment_gate_headdrift_test.go:76`: `Should be empty, but was`
  `bf6bf2f6e3503c916dabb1ac40580e8ad9cc5ca8`; real Git discovered outer HEAD
* `shipment_repair_test.go:248`: prior-gate-evidence error
  `does not contain "HEAD"`; the fixture did not hit the intended HEAD guard
* `u163_002_record_test.go:215`: `An error is expected but got nil`;
  the temp directory discovered the outer repository's requested ref

These are the exact Stage-authorized legacy witnesses, not fabricated
future-selector failures or build errors.

## Quality gates and handoff

Whole-repository compile before and after the U1c harness delta exited 0:
`. ./bin/tmp/212-go-env.ps1; go test -run=^$ -count=1 ./...`.
Whole-repository vet exited 0. Pinned v1 lint over core, policyengine, and
integration exited 0 with `Issues=[]`; scoped tracked-source gofmt was empty.
The [U1c manifest](212-S-wave1-U1c-harness-manifest.md) records exact shared
quality commands and the pinned offline Go 1.24.0 environment.

Stop before fixture GREEN. Preserve the original negative assertions while
adding the Stage-required HEAD-guard and `ErrRefNotFound` strengthening during
build-feature. Do not modify production sanitization or `headSHABounded`.
No claims, starts, moves, pushes, PRs, future-task scaffolding, or backlog staging.
