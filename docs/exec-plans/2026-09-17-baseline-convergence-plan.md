---
chunk_strategy: h1-h2-h3
description: 'Restore repository-wide test/lint/format green by fixing the CRLF golden-fixture failure and repository line-ending, errcheck, and staticcheck baseline debt, unblocking shipment 149-S.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-17-baseline-convergence-plan.md
title: 'Repository Baseline Convergence Release Unit'
---

# Repository Baseline Convergence Release Unit

Source deliberation: `docs/decisions/2026-09-17-baseline-convergence-deliberation.md`
Lint inventory source: `docs/decisions/2026-09-17-baseline-lint-inventory.md`
Grounding evidence: `docs/memory/2026-09-17/149-s-wave-1-convergence-hard-stop.md`

## Problem Frame

Shipment `149-S` remains blocked by inherited repository baseline debt outside
its authorized change surface. Feature `175-F` is the covering release unit that
restores the repository baseline and lets `149-S` resume after the baseline
shipment ships.

The completed redesign replaces the earlier package or aggregate plan with an
immutable file-owned lint DAG. The executable scope is locked before handoff:
exactly 39 tasks, no aggregate lint task, no separate format task, and no
backlog creation during execution.

Baseline evidence at HEAD `c80d7c6ba968604913614e90522e1380ba84ad99`:

* `go test ./...` exposed the Windows checkout LF/CRLF golden-byte failure in
  `internal/faultline.TestU4aBehaviorCanonicalByteStable`
* `gofmt -l .` listed 518 Go files, exactly matching the 518 tracked CRLF
  Go/JSON files handled by the line-ending migration
* `golangci-lint` v2.13.2 reported 56 findings across 36 flagged files:
  50 `errcheck` findings across 31 files and 9 packages, plus 6 `staticcheck`
  findings across 5 files and 4 packages
* The `errcheck` and `staticcheck` file sets are disjoint; no separate ownership
  artifact or extra dependency edge is required

## Requirements Trace

| Requirement | Implementation unit(s) |
| --- | --- |
| Normalize tracked Go/JSON line endings durably | U1 (`175.001-T`) |
| Preserve `ci.yml` byte-for-byte | U1 and U14 |
| Resolve every linter-flagged file with one file owner | 36 lint tasks (`175.002-T`..`175.011-T`, `175.013-T`, `175.015-T`..`175.039-T`) |
| Add a non-vacuous persistent line-ending guard | U14 (`175.014-T`) |
| Prove repository convergence and evidence integrity | U12 (`175.012-T`) |

## Authoritative Decomposition

Feature `175-F` contains exactly 39 executable tasks:

| Unit set | Task IDs | Ownership |
| --- | --- | --- |
| U1 | `175.001-T` | Homogeneous mechanical line-ending migration |
| Lint tasks | `175.002-T`..`175.011-T`, `175.013-T`, `175.015-T`..`175.039-T` | One flagged source or test file per task |
| U12 | `175.012-T` | Terminal verification-only sink |
| U14 | `175.014-T` | Dedicated line-ending guard workflow |

The full 36-file-to-task mapping is not duplicated here. It lives in
`docs/decisions/2026-09-17-baseline-lint-inventory.md`, including each task's
exact linter, flagged file, finding lines, package, affected functions, scenario
bound, and deterministic harness path. Representative repurposed mappings:

* `175.002-T` now owns
  `internal/atomicfile/atomicfile_bench_test.go` for `errcheck`
* `175.003-T` now owns `internal/canonical/canonical.go` for `staticcheck`
* `175.004-T` through `175.011-T` are file-owned lint tasks, not package sweeps
* `175.013-T` is a file-owned lint task, not a residual package aggregate
* `175.015-T` through `175.039-T` are the remaining file-owned lint tasks

No unit owns more than one flagged source/test file. Each lint unit may modify at
most two files: its flagged file and its deterministic harness file.

## U1 - Line-Ending Migration

U1 (`175.001-T`) is the only homogeneous mechanical migration:

* `.gitattributes` hardening for tracked Go/JSON LF normalization
* path-scoped `git add --renormalize` of exactly 518 tracked `*.go`/`*.json`
  files
* no semantic content change in those 518 files
* no workflow content change; `.github/workflows/ci.yml` stays byte-identical

U1 deliberately exceeds the `<3 files` heuristic because the migration is a
single 518-file byte-only operation. This is a documented heuristic exceedance,
not an exemption from the 2-hour human-effort bound. The bound holds because the
change is homogeneous, mechanically verified, and has zero semantic delta.

Required gates retained for U1:

* operator-only approval before staging or refreshing any renormalized path
* clean-tree fail-closed gate before the migration begins
* path-scoped refresh only; no whole-tree forced refresh
* semantic-diff prohibition for the 518 migrated files
* exact evidence that all 518 tracked Go/JSON files were included
* exclusive harness `tests/lineending_baseline_175_test.go` with a genuine
  red-before-green LF-assertion selector

## File-Owned Lint Tasks

The 36 lint tasks are normal harness-required implementation tasks. Each task:

* owns exactly one flagged source or test file from the v2.13.2 lint baseline
* owns exactly one deterministic harness file named
  `<pkg>/<linter>_<basename>_175_test.go`
* lists exact linter, file, finding lines, package, affected functions, and
  fewer than 4 scenarios in the lint inventory
* affects fewer than 5 functions
* modifies at most its flagged file plus its harness
* verifies package buildability with `go test -run=^$ -count=1 <pkg>` when a
  build-only package check is needed

The linter evidence is fixed:

| Linter | Findings | Files | Packages |
| --- | --- | --- | --- |
| `errcheck` | 50 | 31 | 9 |
| `staticcheck` | 6 | 5 | 4 |

The file sets are disjoint. Each flagged file has exactly one task owner, so no
separate cross-linter ownership artifact exists.

### Lint remediation strategy

Errcheck fixes follow the existing Go error-handling policy:

* deferred writable `Close` operations use named returns and `errors.Join`
* write and short-write paths use checked helper patterns already documented in
  compound learnings
* genuinely discardable return values use explicit `_ =` with an inline reason
* tests prefer `require.NoError` or `t.Setenv` where applicable
* propagated errors use `fmt.Errorf("context: %w", err)` while preserving
  sentinel classification where required

Staticcheck fixes prefer behavior-preserving code changes. A `//nolint` is
allowed only when the evidence artifact records a current, non-empty
justification and the suppression still matches a live finding.

## U14 - Dedicated Line-Ending Guard Workflow

U14 (`175.014-T`) owns only a new workflow file:
`.github/workflows/line-endings.yml`. It does not modify
`.github/workflows/ci.yml`; that workflow remains byte-identical.

The new workflow contract:

* always runs on `pull_request` and protected-branch `push` to `main`
* uses `windows-latest` so checkout-time CRLF drift is observable
* sets `permissions: contents: read`
* checks out with
  `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`
* sets `persist-credentials: false`
* uses `concurrency` keyed on `github.ref` with `cancel-in-progress: true`
* has no changed-file skip path and no dependency on the existing CI workflow

The guard is fail-closed over a fixed tracked `*.go`/`*.json` set. It asserts
three independent properties so narrowing or deleting `.gitattributes` cannot
make the check vacuous:

1. the required `.gitattributes` contract is present
2. the checked-out working tree has LF for the fixed tracked set
3. the stored/index blobs are LF-normalized

U14 owns harness `tests/lineendings_guard_175_test.go`. The harness creates a
disposable `t.TempDir` repository, extracts and executes the committed guard
script, and covers red scenarios for missing workflow, removed attributes, CRLF
working tree, CRLF committed blob, plus a green LF scenario.

## U12 - Terminal Verification Sink

U12 (`175.012-T`) is the only member of the closed P-002.1 harness-exempt set.
It is verification-only and owns only
`docs/closure/175-baseline-convergence-convergence-evidence.md`.

The runnable single-quoted `exempt_verification_command` must execute the real
repository gates:

```text
go test ./...
go vet ./...
golangci-lint run
gofmt -l .
```

It fails on any nonzero exit and separately fails if `gofmt -l .` prints any
path. Only after the gates pass does it validate the convergence evidence
artifact and the machine-readable `//nolint` inventory. The inventory check
rejects missing, extra, duplicate, empty-justification, and stale rows.

U12 has no green guard harness and no source/config deliverable. Its dependency
on U1 is transitive through the tasks it depends on.

## Frozen DAG

The DAG is acyclic and contains 74 edges total:

* 36 lint tasks depend on U1
* U14 depends on U1
* U12 depends on every other executable task except U1 directly: all 36 lint
  tasks plus U14

This yields 37 edges into U1 under backlogit's `depends_on` orientation and 37
edges from U12 to its immediate prerequisites. U12 is the sole terminal sink; no
lint task or U14 depends on another lint task.

```text
U1 line-ending migration
  <- 36 file-owned lint tasks
  <- U14 dedicated workflow guard

U12 terminal verification
  -> every file-owned lint task
  -> U14
```

No U11 special edge exists. The old U11 role has been repurposed as one normal
file-owned lint task.

## Harness-Exempt Set

| Task | Class | Harness owner | Deliverable |
| --- | --- | --- | --- |
| `175.012-T` | `verification-only` | `none` | `docs/closure/175-baseline-convergence-convergence-evidence.md` |

Every other task is normal harness-required. U1, U14, and all 36 lint tasks own
real red-before-green harnesses.

## Stash Classification

Feature `175-F` legitimately sources the deferred-scope-expansion stash entries
`92F79833` and `4DB1DFF1`. They remain the intake roots for this covering
feature.

Local stash `484F2845` is separate. It is an operator-requested active bug intake
about a CheckpointV1 `resume_hint` validation gap. It is not a P-021 review
finding and is not part of this feature's deferred-scope-expansion capture.

Stash `71200CBB` is the autoharness-workspace stash for the autoharness-owned
half of that separate concern.

## Decisions and Rationale

* A feature remains the covering root because shipment covering-item derivation
  and manifest-binding digest logic require a dotless feature root
* The line-ending migration is isolated in U1 so semantic lint fixes never mix
  with byte-only normalization
* The one-file-per-task lint DAG keeps ownership auditable and prevents hidden
  cross-file aggregation
* U14 is a new workflow because modifying `ci.yml` would enlarge blast radius and
  violate the byte-identical requirement
* U12 is verification-only because it observes final convergence instead of
  implementing a code/config change

## Risks and Mitigations

* **Renormalization blast radius:** U1 requires operator approval, clean-tree
  gating, exact path evidence, scoped refresh, and semantic-diff prohibition
* **Binary or fixture corruption:** U1 limits the migration to the tracked
  Go/JSON set and proves the 518-file set exactly
* **Lint ownership drift:** the lint inventory is the source of truth for every
  flagged file and reserved harness path
* **Guard vacuity:** U14 validates attributes, working-tree EOL, and stored blob
  EOL independently
* **Suppression rot:** U12 rejects stale or unjustified `//nolint` inventory rows

## Constitution Check

* **Safety-First Go:** pass. Error handling changes are file-local and follow
  existing wrapping and sentinel policies.
* **Test-First Development:** pass. U1, U14, and every lint task are normal
  harness-required tasks with red-before-green harnesses. U12 is the sole
  verification-only harness-exempt task.
* **Workspace Isolation:** pass. All planned edits are within the repository.
* **CLI Workspace Containment:** pass. No file operation leaves the workspace.
* **Destructive Command Approval:** pass with mandatory U1 operator approval
  before renormalization staging or working-tree refresh.
* **Task Granularity:** pass. U1 exceeds a file-count heuristic for a homogeneous
  byte-only migration while preserving the 2-hour human-effort bound; all lint
  tasks are one flagged file each.
* **Quality Gates:** pass by U12 evidence: `go test ./...`, `go vet ./...`,
  `golangci-lint run`, and empty `gofmt -l .`.
* **Merge Commit History Preservation:** pass. Shipping remains merge-commit
  governed by the Ship workflow.

Constitution Check: documented-deviations

## Plan Review Status

The earlier per-package and aggregate design was superseded by the cycle-5
file-owned lint DAG redesign. The PASS conclusion is retained only for the
current contract described in this document:

* exactly 39 executable tasks
* 36 file-owned lint tasks
* U1 as the only line-ending migration
* U14 as a new dedicated workflow with no `ci.yml` edit
* U12 as the only harness-exempt verification sink
* 74 DAG edges
* no aggregate lint unit and no execution-time backlog creation

## Runtime Verification and Closure

Runtime surface changes are limited to repository hygiene, error handling, and a
new guard workflow. U12 records final absorption by capturing all four mandatory
quality gates, line-ending evidence, and `//nolint` inventory validation in
`docs/closure/175-baseline-convergence-convergence-evidence.md`.

Rollback remains commit-scoped: revert U1 to undo the mechanical normalization,
revert any individual lint task for its file-local fix, and revert U14 to remove
the dedicated workflow guard.
