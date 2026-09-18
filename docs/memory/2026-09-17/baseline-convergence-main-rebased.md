---
doc_type: memory
source: docs/exec-plans/2026-09-17-baseline-convergence-plan.md
date: 2026-09-17
session: stage-baseline-convergence-main
status: complete
---

# Stage session - Baseline convergence release unit recreated from current main

## Why recreated

The first attempt branched off the paused 149-S feature branch, so a direct
staging PR would have carried unrelated Wave 1 source commits. A clean
cherry-pick was aborted because `.backlogit/hooks_queue.jsonl` is tool-managed
event provenance and could not be safely transplanted. The original branch
`stage/baseline-convergence-release-unit` and its commits remain preserved.

This session recreated the reviewed release unit natively from current `main` so
backlog artifacts, event streams, and checkpoint provenance stay internally
consistent with main.

## Current redesign summary

The completed cycle-6 redesign supersedes the earlier package and aggregate
planning shape. The current authoritative design is an immutable file-owned lint
DAG for feature `175-F`:

* exactly 40 executable tasks
* U1 (`175.001-T`) as the sole line-ending migration (548 tracked CRLF paths)
* 36 file-owned lint tasks covering every flagged file in the v2.13.2 baseline
* U40 (`175.040-T`) as the line-ending guard script
* U14 (`175.014-T`) as the dedicated line-ending guard workflow (depends on U40)
* U12 (`175.012-T`) as the only harness-exempt verification sink
* exactly 75 dependency edges
* no aggregate lint unit and no backlog creation during execution

The complete lint file-to-task mapping is recorded in
`docs/decisions/2026-09-17-baseline-lint-inventory.md`.

## Native backlog artifacts

The current main-native release unit consists of:

* covering feature `175-F`
* tasks `175.001-T` through `175.040-T`
* shipment `156-S` for repository baseline convergence
* cross-shipment dependency `149-S -> 156-S (blocks)`

The 40 tasks are assigned as follows:

| Unit | Task ID(s) | Role |
| --- | --- | --- |
| U1 | `175.001-T` | line-ending migration for 548 tracked CRLF Go/JSON paths (518 Go + 30 JSON) |
| lint tasks | `175.002-T`..`175.011-T`, `175.013-T`, `175.015-T`..`175.039-T` | one flagged file each |
| U40 | `175.040-T` | line-ending guard script (`scripts/check-line-endings.ps1`) |
| U14 | `175.014-T` | new dedicated line-ending workflow (depends on U40) |
| U12 | `175.012-T` | terminal verification-only evidence |

U1 and U14 both leave `.github/workflows/ci.yml` byte-identical. U14 owns only
`.github/workflows/line-endings.yml` plus its workflow harness.

## Dependency graph

The DAG is locked before Ship handoff and has 75 edges:

* each of the 36 lint tasks depends on U1 (36 edges)
* U40 depends on U1 (1 edge)
* U14 depends on U40 (1 edge)
* U12 depends on all 36 lint tasks and U14 (37 immediate edges)

U12's dependency on U1 and U40 is transitive only. U12 is the sole terminal sink.
U11 is now a normal file-owned lint task, not a special coordination owner.

## Lint and format evidence

Baseline evidence at HEAD `c80d7c6ba968604913614e90522e1380ba84ad99`:

* `errcheck`: 50 findings across 31 files and 9 packages
* `staticcheck`: 6 findings across 5 files and 4 packages
* total flagged files: 36
* errcheck/staticcheck file intersection: empty
* gofmt-listed Go files: 518
* tracked CRLF Go files: 518
* tracked CRLF Go + JSON paths (U1 renormalization scope): 548 (518 Go + 30 JSON)

The gofmt-listed Go files exactly match the 518 tracked CRLF Go files, so U1
resolves the format drift. No separate gofmt task exists.

## Stash disposition and classification

Feature `175-F` legitimately sources deferred-scope-expansion stash entries
`92F79833` and `4DB1DFF1`.

Local stash `484F2845` is separate. It is an operator-requested active bug intake
about the CheckpointV1 `resume_hint` validation gap. It must not be described as
a P-021 review finding or review-cycle capture.

Stash `71200CBB` is the autoharness-workspace stash for the autoharness-owned
half of the same separate concern.

## Verification contract

U12 owns only
`docs/closure/175-baseline-convergence-convergence-evidence.md`. Its
single-quoted `exempt_verification_command` runs:

```text
go test ./...
go vet ./...
golangci-lint run
gofmt -l .
```

The command fails on any nonzero exit or non-empty `gofmt -l .` output, then
validates the evidence artifact and the machine-readable `//nolint` inventory.
The inventory validation rejects missing, extra, duplicate, empty-justification,
and stale rows.

For file-owned lint tasks, build-only package verification uses
`go test -run=^$ -count=1 <pkg>`.

## Branch and remote

The branch is `stage/baseline-convergence-main`. The work remains Stage-owned
planning and narrative documentation plus governed backlog state. Ship receives a
complete executable scope and does not need to create new backlog items for this
feature during execution.

## Next Orchestrator action

Open and merge the staging PR from `stage/baseline-convergence-main` to `main`,
then route shipment `156-S` to Ship. Shipment `149-S` stays blocked until the
baseline convergence shipment ships.
