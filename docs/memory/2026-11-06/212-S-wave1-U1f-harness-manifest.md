---
title: "212-S M12 Wave 1 U1f harness manifest"
description: "Task-scoped compiling assertion RED for 211.012-T only"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-06/212-S-wave1-U1f-harness-manifest.md
date: "2026-11-06"
shipment_id: 212-S
feature_id: 211-F
task_id: 211.012-T
---

## Boundary and provenance

Compilation: PASS. Red Phase: CONFIRMED.

The harness was authored against unchanged production at Stage-owner commit
`9de10c15`. The current branch was retained. This is fresh M12 evidence for
`211.012-T`, not the earlier private mechanism comparison.

Only one Go function was added:
`TestU1f_ExecuteReconstitutionPreliminaryPlanReadsUnderCanonicalLocks`
in `internal/core/shipment_reconstitute_hardening_test.go`.
All closures are inline; the three subtests are sequential.
Existing tests, helpers, assertions, and production functions are unchanged.

The future production budget remains exactly
`loadAndValidateReconstitutePlan` and `executeReconstitution`.
No production changes or stubs were made during harness preparation.

Unchanged production SHA-256:

| File | SHA-256 |
|---|---|
| `internal/core/shipment_reconstitute_plan.go` | `d4037470953af085ee324ff9413771b6a00217a29392a024ea0d1e57dcca376f` |
| `internal/core/shipment_reconstitute_exec.go` | `786b6c2d68f83604776def04561da674045a455408709114337b3776f08b2864` |

## Required scenarios and observed RED

All four exact test names ran once per native invocation: the named top-level
test plus these three subtests. There were zero skips, build failures, panics,
race reports, empty selectors, or test timeouts.

| Subtest | Final baseline result |
|---|---|
| `preliminary_plan_reads_wait_for_stash_global_and_membership_locks` | Assertion FAIL |
| `concurrent_associate_commit_durable_replace_serializes_before_preliminary_plan_reads` | PASS |
| `pure_input_and_plan_validation_fails_fast_before_locks` | Assertion FAIL |

Subtest 1's primary discriminator was:

```text
U1f preliminary-read ordering assertion: returned before canonical locks:
reconstitute: artifact 001.006-T does not exist: backlogit: validation error
```

The error satisfied `errors.Is(err, ErrValidation)` and global acquisition
was false before the deliberate fatal ordering assertion. This was not a
deadlock, timeout, or panic. Cleanup canceled the context, released membership,
joined the worker, restored the removed task bytes, and emitted
`U1f subtest1 cleanup completed`.

Subtest 2 exercised actual `AssociateCommit` and live reconstitution with
`DurableWrites=true`. The first task-5 artifact barrier held the writer until
reconstitution contended on lifecycle-global. `sync.Once` released the writer
before cleanup joins and hook restoration. Both operations succeeded; task
frontmatter contained `feedface`, the matching `commit_links` count was one,
and history contained `commit_tracked`. Cleanup emitted
`U1f subtest2 cleanup completed`.

Subtest 3 checked all six inputs without additional nested subtests. Force
without Reason, unknown YAML, and empty ID satisfied the validation contract.
Traversal, reserved global key, and the whitespace-padded/case-folded duplicate
returned `ErrShipmentNotFound` on baseline instead of `ErrValidation`.
All six still showed zero lifecycle-global attempts and no stash lock sidecar.
The error-category assertions remain strict for GREEN.

Fixtures use the existing public-helper fixture, allocated through the parent
test to shorten Windows paths. Artifact paths, IDs, types, and task parent
headers were read back before the controlled test operations.

## Reproducible commands

Run from the repository root. Every Go invocation uses the cached local
Go 1.24.0 environment, readonly module mode, offline dependencies, and
workspace caches. The short TEMP is owned and ignored, not timestamp-derived.

```powershell
. ./bin/tmp/212-go-env.ps1
$env:TEMP = Join-Path $PWD 'bin/_h8a2f10c4/t'
$env:TMP = $env:TEMP
New-Item -ItemType Directory -Force $env:TEMP | Out-Null
go test -run=^$ -count=1 ./...
```

Task-native selector, executed by the guard:

```powershell
go test -count=1 -timeout=30m -race -json -run '^TestU1f_' ./internal/core
```

RED observer and downstream GREEN gate:

```powershell
& ./scripts/verify-u1f-harness.ps1 -Phase Red
& ./scripts/verify-u1f-harness.ps1 -Phase Green
```

The guard sources the pinned environment and overrides TEMP/TMP itself.
RED may return observer success only with native exit 1, the genuine named
Subtest 1 assertion, completed cleanup, all exact test run/terminal events,
and a passing durable replacement subtest. GREEN requires native exit 0 and
all four named tests passing, with zero skips. GREEN was not claimed or run
against the intentionally unpatched baseline.

Quality gates used the same sourced environment and short TEMP:

```powershell
go vet ./...
& ./bin/golangci-lint.exe run --timeout=5m --out-format=json
gofmt -l internal/core/shipment_reconstitute_plan.go internal/core/shipment_reconstitute_exec.go internal/core/shipment_reconstitute_hardening_test.go
```

## Native result accounting

* Whole-repository compile-only runs: three, all native exit 0, including
  the final harness revision
* Vet runs: two, both native exit 0
* Full pinned lint runs: two, both native exit 0 with `Issues: []`
* Source formatting: PASS, empty output; scoped diff whitespace check: PASS
* Task-native race runs: two, both native exit 1 from genuine product
  assertion RED; no native build/runtime infrastructure failures
* First RED observer: exit 1 because its initial check incorrectly required
  the validation guard to pass on baseline; this private guard mistake was
  corrected once, not hidden or relabeled as success
* Final RED observer: exit 0 while reporting native exit 1; Subtest 1 and
  validation assertions failed as documented, durable replacement passed
* One optional `.golangci.yml` file-view probe found no file; no failed native
  lint invocation or infrastructure retry resulted

The final evidence was emitted during this run at
`2026-11-06T11:21:51-08:00` through `11:22:01-08:00`; final compile, vet,
lint, and formatting gates subsequently passed.

## Handoff restrictions

The official update applied only `harness-ready` and implementation notes
on `211.012-T`; returned status remains queued, with no claim/start/move.
No acceptance criteria, plan, dependency, shipment, or future task changes
are part of this harness.

No repo-wide test-running command was executed. The three prior convergence
failures remain recorded; the approved fourth full-suite cycle remains
reserved and unconsumed. No push, PR, branch change, stash disposition,
checkpoint change, or Stage-only command was performed.

The harness commit includes only the test delta, executable guard, and this
manifest. Backlog, stash, old checkpoints, and unrelated dirty paths remain
outside staging.
