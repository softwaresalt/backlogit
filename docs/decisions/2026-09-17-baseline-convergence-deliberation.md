---
chunk_strategy: h1-h2-h3
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-17-baseline-convergence-deliberation.md
title: "Repository Baseline Convergence Release Unit"
description: "Deliberation grouping two deferred-scope-expansion stash entries into one baseline-convergence covering feature that restores repository-wide test, lint, and format green."
topic: "Baseline convergence: CRLF golden-fixture failure + repository-wide lint/gofmt drift"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-09-17-baseline-convergence-plan.md"
tags:
  - "baseline-convergence"
  - "line-endings"
  - "errcheck"
  - "staticcheck"
  - "gofmt"
  - "deferred-scope-expansion"
---

## Problem Frame

Shipment `149-S` was governed-handback to `queued` after wave 1 and cannot resume
until inherited repository baseline debt is resolved. Feature `175-F` remains the
covering feature for that debt because shipment covering-item derivation requires
a dotless feature root.

The completed redesign groups the baseline debt into an immutable file-owned lint
DAG of exactly 39 executable tasks. The DAG is locked before handoff and contains
no aggregate lint task, no separate format task, and no backlog creation during
execution.

Baseline evidence at HEAD `c80d7c6ba968604913614e90522e1380ba84ad99`:

* 56 `golangci-lint` v2.13.2 findings
* 50 `errcheck` findings across 31 files and 9 packages
* 6 `staticcheck` findings across 5 files and 4 packages
* 36 total flagged files
* disjoint `errcheck` and `staticcheck` file sets
* 518 gofmt-listed Go files exactly matching the 518 tracked CRLF Go/JSON files

Because the linter file sets are disjoint, the redesign needs no separate
cross-linter ownership artifact and no extra dependency edge for such an
artifact. Because the gofmt-listed files exactly match the tracked CRLF set, U1's
line-ending migration resolves the format drift wholesale and no separate gofmt
remediation task exists.

## Grouped Stash Entries

Feature `175-F` legitimately sources these deferred-scope-expansion entries:

| Stash ID | Kind | Summary | requires-deliberation |
| --- | --- | --- | --- |
| `92F79833` | bug | `TestU4aBehaviorCanonicalByteStable` CRLF/LF golden-byte mismatch in `internal/faultline` | true |
| `4DB1DFF1` | task | 50 errcheck + 6 staticcheck + gofmt drift in inherited baseline files | false; marker forced deliberation |

The local stash `484F2845` is not part of this P-021 intake. It is an
operator-requested active bug about the CheckpointV1 `resume_hint` validation
gap. Stash `71200CBB` is the autoharness-workspace stash for the
autoharness-owned half of that separate concern.

## Research Findings

Learnings retrieval supported three durable principles:

* normalize byte form once, then assert the exact canonical form
* treat unchecked error returns as correctness defects, not lint noise
* baseline empirical linter evidence before assigning ownership

The redesign applies those principles with a root-cause line-ending migration, a
file-owned lint inventory, and a terminal verification artifact that validates
both command outcomes and suppression inventory.

## Options Evaluated

### Option A - Immutable file-owned lint DAG plus line-ending migration (chosen)

Create one covering feature with U1 for homogeneous line-ending normalization,
36 one-file lint tasks, U14 for the dedicated guard workflow, and U12 for final
verification.

* **Pros:** fixed executable scope, exact file ownership, no aggregate lint
  surface, durable line-ending guard, deterministic final evidence
* **Cons:** more backlog tasks than an aggregate plan, but each lint task is
  independently bounded and reviewable

### Option B - Package-scoped lint remediation

Group lint findings by package and let each package task address every finding in
that package.

* **Pros:** fewer task records
* **Cons:** couples unrelated files, obscures exact ownership, and no longer
  matches the completed baseline inventory. Rejected.

### Option C - Spot-fix line endings without durable attributes

Hand-normalize the failing golden or run local formatting without hardening the
Git checkout contract.

* **Pros:** smaller immediate diff
* **Cons:** Windows checkout drift would recur. Rejected.

### Option D - One combined baseline cleanup task

Resolve CRLF, errcheck, staticcheck, gofmt, and guard workflow in one execution
unit.

* **Pros:** fewest task records
* **Cons:** violates width isolation and hides review boundaries. Rejected.

## Decision

Adopt Option A. Feature `175-F` is decomposed into exactly 39 executable tasks:

| Unit | Task ID(s) | Decision |
| --- | --- | --- |
| U1 | `175.001-T` | sole homogeneous line-ending migration |
| file-owned lint tasks | `175.002-T`..`175.011-T`, `175.013-T`, `175.015-T`..`175.039-T` | one flagged file and one harness per task |
| U12 | `175.012-T` | terminal verification-only sink |
| U14 | `175.014-T` | dedicated line-ending guard workflow |

The complete file-to-task mapping lives in
`docs/decisions/2026-09-17-baseline-lint-inventory.md` and is the source of
truth for linter, flagged file, finding lines, package, affected functions,
scenario bounds, and reserved harness paths.

Repurposed IDs are intentional. `175.002-T` through `175.011-T` and `175.013-T`
are now file-owned lint tasks. New IDs `175.015-T` through `175.039-T` complete
the 36 flagged-file coverage. U12 and U14 retain their specialized meanings.

## Unit Contracts

### U1 - Line-ending migration

U1 owns `.gitattributes` hardening plus path-scoped renormalization of exactly
518 tracked `*.go`/`*.json` files. It retains these gates:

* operator-only pre-execution approval
* clean-tree fail-closed precondition
* path-scoped refresh after renormalization
* semantic-diff prohibition for the 518 migrated files
* exact 518-file evidence
* exclusive harness `tests/lineending_baseline_175_test.go`

U1 changes no workflow content. `.github/workflows/ci.yml` remains
byte-identical.

### File-owned lint tasks

Each lint task owns exactly one flagged source/test file and one deterministic
harness file named `<pkg>/<linter>_<basename>_175_test.go`. Each task records its
exact linter, finding lines, package, affected functions, and scenario bound in
the lint inventory. Each task modifies at most 2 files and affects fewer than 5
functions.

When a build-only package verification is needed, the correct command is
`go test -run=^$ -count=1 <pkg>`.

### U14 - Dedicated guard workflow

U14 owns only `.github/workflows/line-endings.yml`. It leaves
`.github/workflows/ci.yml` byte-identical.

The workflow always runs on pull requests and protected-branch pushes to `main`,
uses `windows-latest`, and checks out with
`actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`,
`persist-credentials: false`, and `permissions: contents: read`. Its concurrency
group is keyed on `github.ref` with `cancel-in-progress: true`.

The guard fails closed over a fixed tracked `*.go`/`*.json` set by independently
asserting the required `.gitattributes` contract, working-tree EOL, and
stored/index blob LF normalization. U14 owns
`tests/lineendings_guard_175_test.go`, which exercises missing workflow, removed
attributes, CRLF working tree, CRLF committed blob, and green LF scenarios in a
disposable test repository.

### U12 - Verification-only sink

U12 is the sole member of the closed P-002.1 harness-exempt set. It owns only
`docs/closure/175-baseline-convergence-convergence-evidence.md`.

Its single-quoted `exempt_verification_command` runs:

```text
go test ./...
go vet ./...
golangci-lint run
gofmt -l .
```

It fails on any nonzero exit and on any non-empty `gofmt -l .` output. It then
validates the evidence artifact and a machine-readable `//nolint` inventory,
rejecting missing, extra, duplicate, empty-justification, and stale rows.

## Dependency Graph

The DAG is acyclic and has exactly 74 edges:

* each of the 36 lint tasks depends on U1
* U14 depends on U1
* U12 depends on every other executable task: all 36 lint tasks plus U14

U12's dependency on U1 is transitive. U12 is the sole terminal sink. There is no
extra U11 ownership edge because U11 is now a normal file-owned lint task.

```text
36 lint tasks -> U1
U14 -> U1
U12 -> 36 lint tasks
U12 -> U14
```

## Harness-Exempt Set

| Task | Class | Harness owner | Deliverable |
| --- | --- | --- | --- |
| `175.012-T` | `verification-only` | `none` | `docs/closure/175-baseline-convergence-convergence-evidence.md` |

All other tasks are normal harness-required tasks.

## Rejected Alternatives

* Package-scoped lint tasks - rejected because the completed evidence supports
  one-file ownership and disjoint linter sets
* Separate gofmt task - rejected because the gofmt-listed files are exactly the
  tracked CRLF files handled by U1
* Modifying `.github/workflows/ci.yml` - rejected because U1 and U14 both require
  `ci.yml` to remain byte-identical
* Creating execution-time tasks - rejected because the executable DAG is complete
  before handoff
* Combined mega-task - rejected because it violates width isolation

## Risks and Mitigations

* **Renormalization blast radius:** mitigated by operator approval, clean-tree
  gating, exact path evidence, and semantic-diff prohibition
* **Lint ownership drift:** mitigated by the durable lint inventory source of
  truth
* **Suppression rot:** mitigated by U12's machine-readable `//nolint` inventory
  validation
* **Vacuous guard:** mitigated by U14's independent attributes, working-tree, and
  blob checks
* **Workflow blast radius:** mitigated by adding a dedicated line-ending workflow
  instead of editing existing CI

## Outcome

The deliberation resolves to the file-owned lint DAG captured in the linked plan.
The release unit remains a Stage-owned narrative and backlog design. Ship
receives a complete executable scope: 39 tasks, 74 dependency edges, one
line-ending migration, 36 file-owned lint remediations, one dedicated guard
workflow, and one verification-only terminal sink.
