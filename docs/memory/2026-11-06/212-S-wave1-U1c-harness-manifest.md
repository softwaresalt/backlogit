---
title: 212-S Wave 1 U1c harness manifest
date: 2026-11-06
task_id: 211.009-T
feature_id: 211-F
shipment_id: 212-S
phase: harness-red
---

## Contract and scope

Compilation: PASS. Red Phase: CONFIRMED.
Stage baseline: `bf6bf2f6e3503c916dabb1ac40580e8ad9cc5ca8`.
Authority: amended plan U1c, D11 M1-M3, and
`stage-212-s-086-087-dl-u1cde-amendment.md` in this directory.
Closed siblings: `211.001-T` and `211.008-T`; neither is reopened.
Current ready set is exactly `211.009-T`, `211.010-T`, `211.011-T`.

Only `TestInGitWorktreeBounded` changed in Go. No production changes,
stubs, wrappers, or helper changes. The four-function overall contract remains
`planArtifactMove`, future `gitEntryPresentBelowWorkTree`,
`inGitWorktreeBounded`, and `TestInGitWorktreeBounded`.

Subcase (b) creates its workspace and cross-platform fake Git before changing
PATH, proves `LookPath` resolves inside `fakeDir`, and passes `(false, nil)`.
Its scoped environment restores PATH before all subsequent real-Git fixtures.
Subcase (e) retains its real empty `.git` directory and assertion block verbatim.
The new hermetic outer-repository subtest executes before (e), so (e)'s legacy
fatal assertion cannot hide either new boundary witness.

## Executable boundary

Run from the repository root in a fresh PowerShell shell:

```powershell
. ./bin/tmp/212-go-env.ps1
./scripts/verify-212-wave1-harness.ps1 -TaskID 211.009-T -Phase Red
```

Build acceptance uses the same command with `-Phase Green` (the default).
The reusable guard requires an explicit TaskID and rejects undeclared tasks.
Its exact native selector is:

```powershell
go test -timeout=30m -count=1 -race -json -run '^(TestPlanArtifactMove_StrategyDetection|TestInGitWorktreeBounded|TestIsAncestor)$' ./internal/core
```

Declared canonical tests:

* `TestPlanArtifactMove_StrategyDetection`
* `TestInGitWorktreeBounded`
* `TestIsAncestor`

Required additional runs under `TestInGitWorktreeBounded`:

* `no_repo_legacy_skip_with_isolated_git`
* `nested_and_intermediate_empty_git_dir_inside_outer_repo_fails_closed/direct`
* `nested_and_intermediate_empty_git_dir_inside_outer_repo_fails_closed/intermediate`

The guard checks exactly one named run and terminal result per canonical test,
exact package identity/count (1), zero skips, and all required nested runs.
GREEN requires every selected test and package to PASS, native exit 0.
RED requires assertion traces, native exit 1, both-API failure messages for
both new boundaries, and fake-Git PASS; panic/build/race/empty selectors fail.
Baseline assertion-node preservation accepts strengthening and fixture edits,
not deletion, weakening, renaming, or reduced duplicate assertion counts.
Original assertion-node counts: 31, 12, 7 respectively.

## Observed native proof

Final guard verification at 2026-11-06 06:08 UTC:
native exit **1**, guard exit **0**, 1 package, 3 canonical runs, zero skips.

* `TestPlanArtifactMove_StrategyDetection/empty_git_directory_fails_closed`:
  `archive_git_test.go:127`, `An error is expected but got nil`
* New `direct` and `intermediate` subtests: both probe and move assertions
  report `An error is expected but got nil` at
  `shipment_gate_ancestry_test.go:182` and `:186`; `errors.Is` assertions at
  `:183` and `:187` also fail because neither API returns `ErrValidation`
* Original (e): `shipment_gate_ancestry_test.go:201`,
  `An error is expected but got nil`
* Isolated (b), clean nested workspace, and `TestIsAncestor`: PASS

## Quality gates and handoff

Every Go invocation dot-sourced the pinned environment: Go 1.24.0,
`GOTOOLCHAIN=local`, readonly modules, offline dependencies, workspace caches.
Before and after scaffolding, whole-repository compilation exited 0:

```powershell
. ./bin/tmp/212-go-env.ps1
go test -run=^$ -count=1 ./...
go vet ./...
& ./bin/golangci-lint.exe run --timeout=5m --out-format=json ./internal/core/... ./internal/policyengine/... ./tests/integration/...
gofmt -l internal/core/archive.go internal/core/shipment_gate.go internal/core/shipment_gate_ancestry_test.go internal/core/shipment_gate_headdrift_test.go internal/core/shipment_repair_test.go internal/policyengine/u163_002_record_test.go internal/core/025_doctor_harness_test.go tests/integration/shipment_workflow_test.go
```

Vet exited 0; pinned v1 lint exited 0 with `Issues=[]`; scoped gofmt output
was empty. No ignored cache tree was formatted. No unfiltered RED suite was run.
Stop before production implementation and fixture GREEN. No claims, starts,
moves, pushes, PRs, future-task scaffolding, or backlog staging.
