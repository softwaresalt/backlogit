---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash D760E508 (reduce CI GitHub Actions usage by gating heavy Go build/test/lint jobs to code-only changes). Confirms the covering feature scope and selects a required-check-safe gating topology that keeps all four branch-protection required contexts reporting on every PR type, including the pipeline own docs-only closure PRs.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-04-ci-cost-gating-deliberation.md
title: 'CI cost-gating for docs/chore-only PRs: required-check-safe gating topology'
stash_id: D760E508
decision_status: decided
promoted_to: plan
tags:
  - ci
  - github-actions
  - branch-protection
  - cost
---

## Source

- Stash: `D760E508` (kind=task, priority=medium, age 0d) — "Reduce CI build actions to only run on code changes to save GitHub Actions usage." Add path/PR-title gating to `.github/workflows/` so docs- and chore-only PRs skip the heavy Go build/test/lint jobs.
- Session: operator authorized a full autonomous Stage->Ship run (AFK). Operator recommends this CI-workflow change be its own shipment (focused adversarial review on a security-sensitive workflow change).
- Hard constraint (operator): the design MUST keep every branch-protection **required** status check *satisfiable and reporting* on ALL PR types. If a provably safe design is not achievable, DEFER rather than risk breaking merges in an AFK run.

## Grouping rationale (Step 1.5)

Two task-shaped, actionable stash entries were eligible this session: `D760E508` (CI gating, medium) and `2EF8B7AD` (closure-docs compaction, low). Per the risk-isolation recommendation, they are processed as **two separate covering features / shipments**, not grouped:

- They occupy **distinct skill domains** (CI/workflow config + tests vs. docs housekeeping) and share no code surface or dependency edge.
- `D760E508` is **security-sensitive** (it changes merge-gating behavior) and benefits from a focused, isolated adversarial review and a single-purpose PR. Bundling it with low-risk docs housekeeping would dilute review focus and enlarge the blast radius of the higher-risk change.
- Sequencing for the Orchestrator: the lower-risk closure compaction is a good first Ship; this higher-risk CI-gating change should ship second, with the gate-job safeguard proven.

`D760E508` is therefore a **solo group** with an implicit covering feature synthesized in this deliberation.

## Problem frame

`.github/workflows/ci.yml` and `.github/workflows/cli-reference-drift.yml` run on every `push` to `main` and every `pull_request` to `main`. The heavy work — `golangci-lint` (5m timeout), `go vet`, `go test -race -coverprofile`, coverage report, on a Go **matrix** `["1.23","1.24"]`, plus a `go run ./cmd/gen-docs` drift regeneration — executes even for docs-only or backlog-only PRs that cannot affect Go behavior. This burns GitHub Actions minutes needlessly.

The catch: this repository uses **branch-protection required status checks**. From the workflow files, the four required contexts and their sources are:

| Required check context | Workflow file | Job | Cost profile |
|---|---|---|---|
| `test (1.23)` | `ci.yml` | `test` (matrix leg) | heavy (lint + race test) |
| `test (1.24)` | `ci.yml` | `test` (matrix leg) | heavy (lint + race test) |
| `Docline frontmatter gate` | `ci.yml` | `docs-lint` (`make docs-lint`) | light-moderate; **docs-relevant** |
| `CLI Reference Drift` | `cli-reference-drift.yml` | `drift` (`go run ./cmd/gen-docs`) | moderate (go build) |

If a *required* check is suppressed by a workflow-level `paths`/`paths-ignore` filter on a docs-only PR, GitHub never reports that context. Branch protection then holds the check in `Expected — Waiting for status to be reported`, and the PR becomes **permanently unmergeable**. This pipeline itself emits **docs-only post-merge closure PRs** (`docs/closure/**`, `docs/memory/**`); a naive skip would make the pipeline's own output unmergeable — a self-inflicted deadlock in an AFK auto-merge run.

## Research findings

Prior art surfaced by the learnings-researcher (confidence: medium) and codebase inspection:

- `docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md` (Decision 1): the repo has already reasoned about job-vs-step CI gating. It established that a **step-level `if:` guard keeps a job's reported status/context reporting**, while a **job-level / `needs`-based skip renders the job as *skipped*** in the UI. This is the exact mechanic behind the required-check hazard.
- `docs/compound/2026-06-26-docline-frontmatter-contract.md`: the `Docline frontmatter gate` required check = `ci.yml` -> `make docs-lint` -> `backlogit docs lint`. Docs-only PRs genuinely exercise it (075-S/076-S failures). Any filter must NOT `paths-ignore` docs for this check.
- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`: the `CLI Reference Drift` check's real dependency is `internal/cli/**`, `cmd/gen-docs/**`, and `docs/cli-reference/**`. It is a strong candidate for an always-run job that only regenerates when those paths change.
- `docs/compound/github-actions/F013-workflow-sha-pinning.md`: the required contexts `test (1.23)`/`test (1.24)` are the **matrix leg names** and must be preserved exactly; the repo mandates **40-char SHA pinning** for all third-party actions and **characterizes ci.yml via `tests/integration/ci_compliance_test.go`** (`readCIWorkflow`). Any gating change must keep those tests green and should extend them.
- `docs/compound/workflow-issues/ship-agent-post-merge-closure-skipped-on-idle-merge-2026-04-24.md`: confirms docs-only closure PRs are a recurring, first-class pipeline output that must merge cleanly.

Key inference: GitHub treats a **job skipped by `paths`/`paths-ignore` at the workflow trigger level** differently from a **step skipped inside a job that still runs**. Only the latter guarantees the required context reports a real conclusion.

## Options evaluated

### Option A: Workflow-level `paths-ignore` on `pull_request`/`push`

Add `paths-ignore: [docs/**, '**/*.md', .backlogit/**]` to the `on:` triggers so the whole workflow is skipped for docs-only PRs.

- **Pros**: One-line change; maximal minute savings (zero jobs run).
- **Cons**: **Fatal.** The required contexts (`test (1.23)`, `test (1.24)`, `Docline frontmatter gate`, `CLI Reference Drift`) never report -> branch protection stalls -> docs-only PRs (including the pipeline's own closure PRs) become unmergeable. Directly violates the hard constraint.
- **Effort**: low. **Fit**: unacceptable.

### Option B: PR-title `if:` gating (skip when title starts with `chore:`/`docs:`)

Gate the heavy jobs on `if: !startsWith(github.event.pull_request.title, 'docs:') && ...`.

- **Pros**: Simple; no new action.
- **Cons**: Title-based classification is unreliable and spoofable — a `docs:` title on a PR that actually changes code would skip tests and (in an AFK auto-merge) merge unverified code. Job-level `if:` also yields a *skipped* context (matrix sharp edge). Does not key on the actual changed files. Fails the fail-safe requirement.
- **Effort**: low. **Fit**: poor (correctness + skipped-context risk).

### Option C (CHOSEN): `dorny/paths-filter` change-detection job + **step-level** gating; jobs always run

Add a lightweight `changes` job (using `dorny/paths-filter`, SHA-pinned) that outputs boolean flags from the actual changed-file set. The existing required jobs keep their names and matrix and **always run**; only their **expensive steps** are guarded by `if: needs.changes.outputs.<flag> == 'true'`. No workflow-level path filtering is added.

- Topology:
  - `ci.yml` `changes` job -> outputs `code`, `docs`.
  - `test` matrix job: `needs: changes`; guard `setup-go` / `go mod download` / lint / vet / test / coverage behind `code == 'true'`. Job always runs -> `test (1.23)`/`test (1.24)` report **genuine success** on docs-only PRs.
  - `docs-lint` (Docline gate) job: `needs: changes`; guard the `make docs-lint` step behind `docs == 'true'`. Runs the linter on docs-only PRs (correct — it validates the very change); reports success on pure-code PRs.
  - `cli-reference-drift.yml` `changes` job -> output `cli` (`internal/**`, `cmd/**`, `**/*.go`, `docs/cli-reference/**`, `go.mod`, `go.sum`, `Makefile`); guard the generate + `git diff` drift steps behind `cli == 'true'`. Job always runs -> `CLI Reference Drift` reports genuine success on unrelated docs-only PRs.
- **Pros**: **Provably preserves required-check satisfiability** — every required context reports a real `success` on every PR type because the owning jobs always run to completion. Keys on the actual changed files (not a spoofable title). Fail-safe filters (broad `code` set incl. `.github/workflows/**` so any workflow edit re-runs full CI). Saves ~90%+ of minutes on docs-only PRs (skips golangci-lint 5m + race tests + gen-docs build). No branch-protection reconfiguration needed (required context *names* are unchanged).
- **Cons**: Adds a third-party action (`dorny/paths-filter`) — mitigated by SHA-pin + `contents: read` only. Residual per-job cost of ~15-30s (runner spin-up + checkout + filter) that a full workflow-skip would avoid; an acceptable price for guaranteed mergeability.
- **Effort**: medium. **Fit**: strong — the only option that satisfies the hard constraint with a fail-safe classifier.

### Option D: Native `git diff` change-detection step (no third-party action)

Same as Option C but compute changed files with a bash `git diff --name-only` step instead of `dorny/paths-filter`.

- **Pros**: No supply-chain surface.
- **Cons**: Must hand-handle PR (base..head) vs. push (before..after), first-commit, force-push, and merge-base edge cases. A subtle bug could mis-classify a code PR as docs-only and skip tests under auto-merge. More custom logic to characterize/test.
- **Effort**: medium-high. **Fit**: acceptable fallback, but higher correctness risk than the battle-tested action for equal savings.

## Trade-off comparison

| Criterion | A (paths-ignore) | B (PR title) | C (paths-filter + step gating) | D (native git diff) |
|---|---|---|---|---|
| Required checks always report | No (fatal) | No (skipped context) | **Yes (genuine success)** | Yes |
| Classifier reliability | n/a | Spoofable | **Changed-file based** | Changed-file based (custom) |
| Fail-safe on ambiguity | n/a | No | **Yes** | Depends on custom logic |
| Supply-chain surface | none | none | one pinned action | none |
| Minute savings on docs-only | max | high | high (~90%+) | high |
| Branch-protection reconfig needed | no | no | **no** | no |
| Correctness risk under auto-merge | fatal | high | **low** | medium |

## Decision

**Adopt Option C**: a `dorny/paths-filter` change-detection job (SHA-pinned, `contents: read`) plus **step-level `if:` gating on jobs that always run**, applied to both `ci.yml` (gating the `test` matrix and the `docs-lint` step) and `cli-reference-drift.yml` (gating the drift steps). No workflow-level `paths`/`paths-ignore` is introduced.

Rationale — why this is **provably** required-check-safe:

1. The four required jobs (`test`, `docs-lint`, `drift`) **always run** on every `push`/`pull_request`. GitHub therefore always reports their contexts. Because the jobs run to completion (heavy steps merely skipped), the reported conclusion is a genuine `success`, not the ambiguous `skipped` state that a job-level `if:`/`needs`-skip produces. This sidesteps the matrix-skip sharp edge entirely.
2. The required context **names are unchanged** (`test (1.23)`, `test (1.24)` remain matrix leg names; `docs-lint` keeps `name: Docline frontmatter gate`; `drift` keeps `name: CLI Reference Drift`). No branch-protection settings change is required — an important property because branch-protection edits are an admin action outside Stage/Ship's authority.
3. The classifier is **fail-safe**: the `code` filter is deliberately broad (`**/*.go`, `go.mod`, `go.sum`, `Makefile`, `cmd/**`, `internal/**`, `tests/**`, `.github/workflows/**`). Anything code-ish runs the full suite; only a provably docs/backlog-only change skips it. A `docs:`-titled PR that actually edits code still runs tests. This protects the AFK auto-merge path from shipping unverified code.
4. Security-sensitive gates are **not weakened**: `Docline frontmatter gate` still runs on docs changes (its relevant trigger); `CLI Reference Drift` still runs on CLI/code changes; the full lint/vet/test suite still runs on any code change. The change only suppresses work that is provably irrelevant to the diff.

Because a design that provably preserves required-check satisfiability **is** clearly achievable, the item is **PROMOTED**, not deferred.

Covering feature (synthesized): **"CI cost-gating for docs/chore-only PRs (required-check-safe)"**.

Task scope confirmed (promoted to `impl-plan`):
1. (config) `ci.yml`: add `changes` job; gate `test` matrix heavy steps behind `code`; gate `docs-lint` step behind `docs`.
2. (config) `cli-reference-drift.yml`: add `changes` job with `cli` filter; gate drift steps behind `cli`.
3. (tests) Extend `tests/integration/ci_compliance_test.go` to assert the gating invariants and required-context preservation.

Inline YAML comments in both workflows will document the required-check-satisfiability invariant (cheaper and more discoverable than a separate design doc — avoids adding an extra in-scope docline artifact); a compound learning will be captured at session close.

## Rejected alternatives

- **Option A** (`paths-ignore`): rejected — directly breaks required-check reporting and would deadlock the pipeline's own docs-only closure PRs.
- **Option B** (PR-title gating): rejected — spoofable/unreliable classifier and job-level skip yields an ambiguous `skipped` context; unsafe for auto-merge.
- **Option D** (native git diff): rejected as the primary path — equal savings but higher mis-classification risk than the battle-tested SHA-pinned action; retained as a documented fallback if supply-chain policy later forbids the action.
- **Separate CI design doc**: deferred in favor of inline workflow comments + a compound learning, to avoid adding a new in-scope `docs/**` file (YAGNI).

## Unresolved questions

- None blocking. The exact SHA to pin for `dorny/paths-filter` is an execution detail resolved by Ship at implementation time (pin the current release commit).
- Confirmation that the four contexts are the *complete* required-check set relies on the workflow inventory; if branch protection later requires additional contexts, the same always-run gate pattern extends to them.

## Risks and mitigations

- **Risk**: a required check silently stops reporting -> unmergeable PR. **Mitigation**: no workflow-level path filtering; jobs always run; integration tests assert the invariant; first post-merge docs-only closure PR is the live canary.
- **Risk**: mis-classification skips tests on real code under auto-merge. **Mitigation**: broad fail-safe `code` filter incl. `.github/workflows/**`; changed-file-based detection; step-level gating so contexts still report.
- **Risk**: supply-chain exposure from a new action. **Mitigation**: 40-char SHA pin (enforced by `TestAllActionsUseSHAPins`), `contents: read` scope, read-only usage.
- **Risk**: characterization tests break on the structural change. **Mitigation**: task 3 updates `ci_compliance_test.go` (parser extended for `if:`/`outputs`/`needs`) as part of the same feature.

Full hardening (verification depth, rollback, monitoring, ProposedAction/ActionRisk) is carried into the plan's `## Plan Hardening` section (this item is risk-triggered).
