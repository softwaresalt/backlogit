---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 177-S and covering feature 176-F: governed 30m full-suite budget pinned in the CI and release workflow test steps, with a docs-lint prefix selector closing the Markdown-only CI gap."
doc_type: closure
schema_version: "1.0"
shipment_id: 177-S
feature_id: 176-F
source: docs/closure/177-S-176-F-post-merge-closure.md
title: "177-S / 176-F Post-Merge Closure"
---

# 177-S / 176-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `177-S` (073-DL ensemble E1, stash `CB8887AF`) |
| Covering feature | `176-F` |
| Tasks | `176.001-T`, `176.002-T`, `176.003-T` |
| Implementation PR | [#598](https://github.com/softwaresalt/backlogit/pull/598) (merge commit) |
| Merge commit | `4bb707bb1c011136ab1d3360153f126190018136` |
| Reviewed implementation HEAD | `58710773d4b101ba47ea3f29130c2e1ff0f2cc31` |
| Closure branch | `post-merge/177-s-governed-test-budget-e1` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `target: all` invoked) |

## Delivered

* `176.001-T` (RED deliverable): `tests/integration/governed_budget_workflows_test.go`, `TestGovernedFullSuiteBudget_Workflows`. It was observed RED on assertions naming ci.yml and release.yml, with `go vet ./tests/integration` passing.
* `176.002-T` (green maker): `ci.yml` test step is `go test -race -timeout=30m -coverprofile=coverage.out ./...` and `release.yml` test step is `go test -timeout=30m ./...`.
* `176.003-T`: the `docs-lint` job runs `^TestGovernedFullSuiteBudget_` with a non-vacuity guard, gated by `docline_required != 'false'`. `ci_compliance_test.go` requires the step (observed RED, then GREEN).

## Gates

* Local: `go build ./cmd/backlogit`, `go vet ./...`, `go test ./tests/integration/...` and LF-normalized `gofmt` all passed.
* Review: local review `READY_WITH_FOLLOWUPS` at the reviewed HEAD (P0 0, P1 0). The adversarial pass was a single reviewer, not a 3-model consensus.
* CI: every required check passed for the merged HEAD, including Docline frontmatter gate and Linux `test`.
* P-018: `autoharness gate copilot-review 598 --enforcement required` returned `SATISFIED` twice, including the last-mile re-run. The Copilot review had no threads and no open issues.
* Merge: merge commit (P-009), `MERGE_CONFIRMED`, and local `main` equal to `origin/main` at `4bb707bb`.

## Shipment closure

* Closed with `backlogit shipment ship 177-S --sha 4bb707bb1c011136ab1d3360153f126190018136` (CLI).
* Archived IDs: `176.001-T`, `176.002-T`, `176.003-T`, `176-F`, `177-S`; no returned IDs. `backlogit sync` indexed 2117 artifacts.

## Deviations and residual risk

* Local full suite: `go test -timeout=30m ./...` fails only the `internal/cli` `TestU175_0xx_LintFix` tests. The local `golangci-lint` v1.64.8 was built with go1.26 and cannot import the installed toolchain's export data, which is an environment limitation unrelated to this change. Linux CI `test` (race and coverage) passed.
* Pre-closure baseline: `.backlogit` queue changes were already present in the worktree before closure. They came solely from this run's own claim and task-done transitions and are staged here as the closure's allowlisted paths; no foreign changes were present.
* Review follow-ups, accepted as residual risk and not captured as stash entries:
  * F1 (LOW P2): the docs-lint guard requires at least one PASS rather than naming each test, which matches the AC of `176.003-T`.
  * F2: the detector is line-based, so a future `\` continuation could evade it. The exact-command equality assertion covers the current steps.
  * F3: the guard's PASS count relies on `-v` output format.
  * F4: no `timeout-minutes` on the `test` job. This is pre-existing and unconfirmed.
* No source stash or deliberation artifacts were archived or mutated. Provenance stash `CB8887AF` was already archived by Stage.

## Compaction and knowledge graduation

* compact-context (`target: all`): the 2026-10-22 160-S group (10 files) was consolidated into `docs/memory/compacted/2026-10-22-160s-ship-session-compacted.md`, with originals moved to `docs/archive/memory/2026-10-22/`.
* compound-refresh: all `docs/compound` entries classified Keep; none invalidated by this shipment.
* No documentation graduation was required beyond this record.
