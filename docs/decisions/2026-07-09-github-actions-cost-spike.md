---
chunk_strategy: h1-h2-h3
description: Spike investigating why this repo's GitHub Actions spend is ~2x the next most expensive repo, ranking cost drivers and recommending reductions
doc_type: decision
docline:
    conclusion: proceed
    confidence: high
    date: 2026-07-09T00:00:00Z
    linked_parent_work_item: 090-F
    promoted_to:
        - none
    tags:
        - github-actions
        - ci-cost
        - ci
        - workflows
    time_box: 2h
    type: spike
ingested_at: "2026-07-09T21:50:00Z"
schema_version: "1.0"
source: docs/decisions/2026-07-09-github-actions-cost-spike.md
title: GitHub Actions cost spike — why backlogit is ~2x the next repo
---

## Goal

Explain why `softwaresalt/backlogit` costs roughly twice as much in GitHub
Actions as the next most expensive repo, rank the cost drivers with evidence,
and recommend concrete reductions. Investigation-only — no workflow changes.

## Method

- Inventoried the three real workflows (`ci.yml`, `cli-reference-drift.yml`,
  `release.yml`) plus the dynamic Copilot review.
- Read each workflow's triggers, runner OS, matrix, gating, and heavy steps.
- Pulled recent run distribution and durations via `gh api .../actions/runs`.
- Confirmed runner OS for every job.

## Findings

### 0. It is NOT a runner-multiplier problem

**Every job runs on `ubuntu-latest`** (Linux = 1× billing). There are no
macOS (10×) or Windows (2×) runners anywhere, including the release build
matrix (cross-compiles with `GOOS`/`GOARCH` on Linux). So the 2× is **volume
of Linux minutes**, driven by structural fan-out — not expensive runners.

### 1. Double execution on `push` AND `pull_request` (primary driver)

Both `ci.yml` and `cli-reference-drift.yml` trigger on **`push: main`** *and*
**`pull_request: main`**. Because merges are PR-based, every change is
validated **twice**: once on the PR head, then again on the merge commit when
it lands on `main`.

Evidence (last 100 runs):

| Workflow | pull_request runs | push runs |
|---|---|---|
| CI | 25 | 11 |
| CLI Reference Drift | 25 | 11 |

The `push`-side runs are largely redundant re-tests of code already validated
on the PR. Two workflows × two events ≈ **4 workflow executions per change
lifecycle**, where a leaner repo runs ~1–2. This structural doubling is the
most likely single cause of "twice as much as the next repo."

### 2. Two parallel Go workflows, each with per-run overhead

`ci.yml` and `cli-reference-drift.yml` are separate workflows that each carry
their own checkout + `setup-go` + a `changes` (paths-filter) job, and each
runs on both events (36 runs each in the sample). Consolidating them would
remove one workflow's worth of double-run and shared setup overhead.

### 3. Heavy `test` matrix: 2 Go versions × `-race` × golangci-lint

The `test` job runs a matrix `["1.23","1.24"]` = **2 parallel jobs**, each
doing `golangci-lint (--timeout=5m)` + `go vet` + **`go test -race`** +
coverage. `-race` is materially slower, and two Go versions doubles it. Billable
minutes = sum of both matrix jobs, so wall-clock (~1.6 min avg) understates
cost on code runs. This is correctly gated to skip on docs/backlog-only changes
(`changes.outputs.code`, added in #189) — that gating is effective for `test`.

### 4. `docs-lint` is ungated (compiles Go on every run)

The `docs-lint` job ("Docline frontmatter gate") has **no `if:` gate**, so it
runs on **every** CI invocation — push and PR, including docs-only changes —
and each time does `setup-go` + `make docs-lint` (`go run ./cmd/backlogit docs
lint`), i.e. a Go toolchain setup + build. The #189 cost-gating covered `test`
but not `docs-lint`, so docs/backlog PRs still spin a Go job (and it double-runs
on push).

### 5. Release re-runs the full `-race` matrix + 6-way build (minor, infrequent)

`release.yml` (tag-triggered; 2 runs in the sample) re-runs the `-race` test
matrix that `main` already ran, plus a `build` matrix of
`{linux,darwin,windows} × {amd64,arm64}` = **6 jobs**, plus npm packaging and
publish. Heavy per release but infrequent — a secondary factor unless releases
are frequent.

### 6. Agent-pipeline volume compounds it

The agentic Stage→Ship pipeline merges frequently (this session alone produced
~5 PRs). Every merge triggers the redundant `push`-side runs from driver #1, so
the double-run tax scales with pipeline throughput.

## Recommendation

**Conclusion: proceed** — the drivers are structural and unambiguous, and the
top fix is low-risk and high-impact. Recommended follow-up work, ranked by
impact/effort:

1. **Eliminate the double-run (biggest win, ~halves CI minutes).** Run CI and
   CLI-Drift on `pull_request` only, or adopt a merge queue (`merge_group`) and
   run heavy jobs there. To keep the tested state == merged state under the
   merge-commit policy (P-009), pair PR-only CI with "require branches up to
   date before merge" or a merge queue rather than testing on both events.
   Keep required-check reporting intact (see the #189 fail-safe gating design).
2. **Trim the test matrix.** Run `-race` on a single Go version (e.g. 1.24) and
   a plain `go test` on the other, or drop 1.23 if it is not a support target.
3. **Gate / lighten `docs-lint`.** Avoid a full Go setup+build on every run
   (and at least skip it on the redundant `push` event); it is a frontmatter
   linter, not a compile gate.
4. **Consolidate CLI-Drift into CI** to remove a whole workflow's double-run
   and duplicated checkout/setup overhead.
5. **Drop the redundant `-race` test job from `release.yml`** (the tagged
   commit was already tested on `main`).

Guardrail: none of these may suppress a branch-protection **required check** in
a way that leaves it permanently "expected" (the footgun documented in
`docs/exec-plans/2026-07-04-ci-cost-gating-plan.md`). Prefer job-level `if:`
gating over trigger-level `paths-ignore`, and keep required jobs reporting.

## Next steps

1. If accepted, stage a follow-up chore/feature to implement #1–#3 (config-only,
   with the required-check-satisfiability acceptance criterion from the #189
   plan reused as the guardrail).
2. Measure before/after billable minutes over a comparable window to confirm the
   reduction (the `/actions/workflows/{id}/timing` API returned no billable
   data here, so use the org/repo Billing → Actions usage report as the source
   of truth).

## References

- `.github/workflows/ci.yml`
- `.github/workflows/cli-reference-drift.yml`
- `.github/workflows/release.yml`
- `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` (required-check-safe gating design; #189)
