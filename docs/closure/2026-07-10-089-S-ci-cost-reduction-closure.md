---
chunk_strategy: h1-h2-h3
description: Closure record for shipment 089-S CI cost reduction.
doc_type: closure
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-10-089-S-ci-cost-reduction-closure.md
title: 089-S CI Cost Reduction Closure
---

## Summary

Shipment `089-S` delivered feature `090-F` and tasks `090.001-T` through
`090.006-T`. The CI workflow now runs on pull requests only, emits the required
`Detect code changes`, `test`, and `Docline frontmatter gate` contexts on PRs,
uses a single Go 1.24 race and coverage path, and contains the consolidated
`CLI Reference Drift` job. The release workflow no longer repeats the two-version
race and coverage matrix; it keeps a lightweight Go 1.24 test plus protected-main
tag provenance before packaging.

## Changed surfaces

| Surface | Closure note |
|---|---|
| `.github/workflows/ci.yml` | Removed `push` trigger, preserved required PR contexts, de-duplicated lint, gated heavy steps, and folded CLI drift into CI |
| `.github/workflows/release.yml` | Replaced release race/coverage matrix with provenance plus plain `go test ./...` |
| `tests/integration/ci_compliance_test.go` | Encodes trigger, required-check, matrix, gating, SHA-pin, drift consolidation, and release-provenance invariants |
| `.backlogit/` | Shipment `089-S`, feature `090-F`, and scoped tasks were shipped and archived |

## Validation

* Red phase: updated CI compliance tests failed against the old workflows before
  workflow changes were applied.
* `go build ./cmd/backlogit` passed.
* `go test ./tests/...` passed.
* `go test ./...` passed.
* `go vet ./...` passed.
* `golangci-lint run` passed.
* `actionlint .github\workflows\ci.yml .github\workflows\release.yml` passed.
* `gofmt -l tests\integration\ci_compliance_test.go` returned empty output.
* `gofmt -l .` was also run and returned pre-existing repository-wide formatting
  output across unchanged Go files. This is recorded as a release condition below
  instead of widening this CI-config shipment into a repository-wide formatting PR.
  `gofmt -l tests\integration\ci_compliance_test.go` returned empty output.
* PR #201 CI passed on the final head `8b2a94177cd35bcedc139fa127f7aff020c759e8`:
  * `Detect code changes`
  * `test`
  * `Docline frontmatter gate`
  * `CLI Reference Drift`
* `LOCAL_REVIEW_READY`: local review confirmed all six task acceptance criteria
  are met and no required context was renamed or suppressed.
* Copilot review was requested through the review-request API. All actionable
  Copilot comments were fixed, replied to, and resolved. The final Copilot review
  on `8b2a94177cd35bcedc139fa127f7aff020c759e8` had no unresolved threads.

## Merge and release record

* Feature branch: `feat/089-S-ci-cost-reduction`
* Feature PR: #201
* Reviewed HEAD: `8b2a94177cd35bcedc139fa127f7aff020c759e8`
* Merge commit: `fd5cc60c92bbcd478de62fac20fa8f2d1d636911`
* Normal merge attempt failed with:
  `X Pull request softwaresalt/backlogit#201 is not mergeable: the base branch policy prohibits the merge.`
* `MERGE_AUTHORIZED (admin)`: dark-mode activation for shipment `089-S` set
  `merge_approval_pre_authorized = true` and `admin_fallback_pre_authorized = true`.
  The normal merge failure was classified as `REVIEW_REQUIRED_BLOCK` because PR #201
  had green required checks and clean Copilot readiness but `reviewDecision` was
  `REVIEW_REQUIRED`; admin fallback used `gh pr merge 201 --merge --admin`.
* P-009 preserved: merge commit was used; no squash or rebase merge was used.

## Ruleset coordination

No required-check-name update is needed if the main ruleset continues to require
exactly `Detect code changes`, `test`, and `Docline frontmatter gate`; all three
reported success on PR #201. Because CI is now PR-only, operators should keep
branch protection requiring up-to-date branches before merge or adopt a merge
queue with an explicit `merge_group` trigger if they want merged-state validation
in addition to PR-head validation.

## Monitoring and rollback

| Evidence | Value |
|---|---|
| Signals / queries | PR #201 GitHub Actions, `gh pr checks 201`, local Go gates, `actionlint` |
| Baseline | CI and CLI Reference Drift ran on both PRs and pushes, with two Go versions and duplicate lint/race work |
| Healthy threshold | Required PR contexts report success; code PRs run Go 1.24 lint/vet/race/coverage; CLI-affecting PRs run drift; release tags verify protected-main provenance and pass plain tests |
| Alert threshold | Any PR waits on a missing required context, `test` reports as a matrix context, CLI drift stops reporting, or release tags bypass protected-main provenance |
| Owner | Repository maintainer during the next PR and release tag |
| Observation window | Next three PRs plus the next release tag |
| Current outcome | Ready with condition: PR #201 merged cleanly and shipment shipped; full `gofmt -l .` still reports pre-existing repository-wide formatting output outside this scope |
| Release condition | Track and clear the existing repository-wide `gofmt -l .` baseline in a separate formatting-only change before treating the global format gate as clean |
| Rollback trigger | A confirmed required-check satisfiability regression or release provenance false positive/negative |
| Rollback procedure | Revert merge commit `fd5cc60c92bbcd478de62fac20fa8f2d1d636911`, rerun CI locally where applicable, and restore the previous workflow trigger model |
