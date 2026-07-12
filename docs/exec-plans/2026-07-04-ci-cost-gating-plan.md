---
chunk_strategy: h1-h2-h3
description: 'Implementation plan to gate the heavy CI Go build/test/lint jobs and CLI-reference-drift regeneration to code-relevant changes only, using a dorny/paths-filter change-detection job plus step-level if-gating on always-running jobs, so all four branch-protection required checks keep reporting success on every PR type including the pipeline own docs-only closure PRs.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-04-ci-cost-gating-plan.md
title: 'CI cost-gating for docs/chore-only PRs (required-check-safe)'
---

## Source

> **⚠️ STATUS: DEFERRED (2026-07-04). Plan-review cycle budget exhausted after 3 consecutive FAILs on the core merge-safety mechanism.** The plan body below reflects the round-4, source-verified `unsafe` all-negation-under-`every` gate design (validated against `dorny/paths-filter` `src/filter.ts`), but **this revision has NOT itself passed a plan-review** and MUST clear a fresh plan-review cycle before harvest. Per the operator's AFK guidance ("DEFER rather than risk breaking merges"), `D760E508` is not harvested this session and its stash entry remains active. See the `## Plan Review` section for the full round-by-round finding trail. Some earlier prose (AC-1, Decisions, INV-3) may still describe superseded framings; the **authoritative gate mechanism is Units A/B + the Plan Review round-3/4 record**.

- Deliberation: `docs/decisions/2026-07-04-ci-cost-gating-deliberation.md` (decided, promoted_to: plan; Option C chosen).
- Stash: `D760E508` (kind=task, priority=medium) — reduce GitHub Actions usage by gating heavy CI jobs to code changes.
- Prior art: `docs/compound/github-actions/F013-workflow-sha-pinning.md`, `docs/compound/2026-06-26-docline-frontmatter-contract.md`, `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`, `docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md`.

## Problem frame

`.github/workflows/ci.yml` (jobs `test` [matrix `1.23`,`1.24`] and `docs-lint` [name "Docline frontmatter gate"]) and `.github/workflows/cli-reference-drift.yml` (job `drift` [name "CLI Reference Drift"]) run in full on every `push` to `main` and every `pull_request`. The four contexts they produce — `test (1.23)`, `test (1.24)`, `Docline frontmatter gate`, `CLI Reference Drift` — are **branch-protection required checks**. We want docs-/backlog-only PRs to skip the expensive Go work (golangci-lint 5m, `go test -race`, coverage, `go run ./cmd/gen-docs`) to save Actions minutes, **without** suppressing any required context (a suppressed required check never reports and permanently blocks merge under branch protection — fatal for the pipeline's own docs-only closure PRs).

Chosen topology (deliberation Option C): a lightweight `changes` job (`dorny/paths-filter`, SHA-pinned, `contents: read`) computes boolean flags from the actual changed-file set; the required jobs **always run** (keeping their names and matrix), and only their **expensive steps** are guarded with `if: needs.changes.outputs.<flag> == 'true'`. No workflow-level `paths`/`paths-ignore` is introduced. Because the owning jobs run to completion, every required context reports a genuine `success` on every PR type.

## Requirements trace

| Source requirement | Implementation unit |
|---|---|
| `D760E508`: skip heavy Go build/test/lint on docs/chore-only PRs to save minutes | Unit A (ci.yml), Unit B (cli-reference-drift.yml) |
| Hard constraint: keep every required check **satisfiable and reporting** on ALL PR types (AC-1 below) | Units A + B (always-run jobs, no path-level suppression) + Unit C (asserts invariant) |
| Do not weaken security-sensitive gates (Docline, CLI drift, lint/vet/test) | Units A + B (gates run on every non-safe change; fail-safe negative predicate; `docs-lint` always runs) |
| Keep required context **names** unchanged (no branch-protection reconfig) | Units A + B (matrix + job names preserved) |
| SHA-pin new actions; keep characterization tests green | Unit C (extends `ci_compliance_test.go`) |

### AC-1 (required-check-satisfiability — MANDATORY acceptance criterion)

**On a docs-only or backlog-only PR (changes confined to `docs/**`, `**/*.md`, and/or `.backlogit/**`, with no code), all four required status checks — `test (1.23)`, `test (1.24)`, `CLI Reference Drift`, and `Docline frontmatter gate` — MUST report a `success` conclusion (not left `pending`/`expected`), so the PR remains mergeable under branch protection.** This is verified by (1) Unit C integration tests asserting no workflow-level path filtering exists on the required workflows, that the required jobs are unconditional (always run), and that heavy steps use the **fail-safe negative gate** (`docs_only != 'true'` / `drift_irrelevant != 'true'`, computed with `predicate-quantifier: every` over a closed safe allowlist); and (2) the first post-merge docs-only closure PR merging cleanly (live canary). Conversely, **any PR that touches even one file outside the safe allowlist (including unenumerated/new paths) MUST still run the full `test` matrix**, and any PR touching CLI-relevant paths or `docs/cli-reference/**` MUST run the drift regeneration. The gate is fail-safe by construction: heavy work is skipped only when the change is *provably* docs/backlog-only.

## Implementation units

### Unit A — Gate ci.yml heavy steps behind a fail-safe `changes` job (config)

- **Files**: `.github/workflows/ci.yml` (1 file).
- **Design principle (FAIL-SAFE NEGATIVE GATING — source-verified round 4)**: heavy work is skipped **only** when a change is *provably* docs/backlog-only. Detect the **existence of a file outside the safe allowlist** with an `unsafe` filter of **all-negated patterns under `predicate-quantifier: 'every'`**, then gate heavy steps `if: needs.changes.outputs.unsafe != 'false'`. Per `dorny/paths-filter` `src/filter.ts`, `every` means `patterns.every(rule => rule.isMatch(file))` with each pattern an independent `picomatch`; so `unsafe` = `['!docs/**','!**/*.md','!.backlogit/**']` matches a file iff it is NOT under `docs/`, NOT `*.md`, and NOT under `.backlogit/` — i.e. outside the allowlist — and the output is `'true'` iff any changed file is unsafe. (NOTE: a *positive* allowlist under `every` is a **constant-false no-op** — no file matches all disjoint patterns — see Plan Review round 3; and a positive allowlist under `some` is **fail-open** — see round 1. Only the all-negation-under-`every` form is correct.) Any unenumerated/new path (`schemas/**`, `scripts/**`, `plugin/**`, `retired package tree`, `.mcp.json`, root configs, `.github/**` non-`.md`) is `unsafe` -> heavy work runs. Satisfies INV-3.
- **Changes**:
  1. Add a `changes` job:
     - `runs-on: ubuntu-latest`; `permissions: { contents: read }` (git-mode paths-filter needs no `pull-requests` scope — least privilege); `timeout-minutes: 5` (bounded SPOF).
     - `outputs: { unsafe: ${{ steps.filter.outputs.unsafe }} }`.
     - Step 1: `actions/checkout@<existing-SHA>` with `persist-credentials: false` (enables git-mode diff against base/before).
     - Step 2: `dorny/paths-filter@<40-char-SHA>` (`id: filter`) with **`predicate-quantifier: 'every'`** and a single filter:
       - `unsafe`: `'!docs/**'`, `'!**/*.md'`, `'!.backlogit/**'`.
       - Output `'true'` iff any changed file is outside the safe allowlist; `'false'` iff every changed file is safe (docs-only).
  2. `test` job: add `needs: changes` **and job-level `if: ${{ !cancelled() }}`** (fail-safe: if `changes` fails/absent, the job still runs and empty output `'' != 'false'` -> heavy steps run). Keep the `checkout` step unconditional. Add `if: needs.changes.outputs.unsafe != 'false'` to: `setup-go`, `Install dependencies`, `Lint`, `Vet`, `Test`, `Coverage report`. Matrix `["1.23","1.24"]` unchanged -> contexts `test (1.23)`/`test (1.24)` preserved and always report a genuine conclusion. (Use `!= 'false'`, NOT `== 'true'`, so the empty-output infra-failure case runs heavy work.)
  3. `docs-lint` job (name "Docline frontmatter gate"): **always runs, ungated** — `make docs-lint` is cheap relative to the Go matrix, is always relevant on docs PRs, and is harmless on pure-code PRs. Removing its gate eliminates any risk of skipping frontmatter validation and keeps the required context a genuine run on every PR. (No `needs: changes` required.)
  4. Add a top-of-file comment documenting the required-check-satisfiability invariant (jobs always run; only steps are gated; gate is the fail-safe `unsafe != 'false'` form over an all-negation `every` filter; never add workflow-level `paths`/`paths-ignore` to required workflows).
- **Tests**: covered by Unit C (integration assertions).
- **Execution posture**: characterization-first — Unit C tests define the target structure; edit YAML to satisfy.
- **Atomic milestone**: `ci.yml` parses; `changes` job present with a single `unsafe` output computed under `predicate-quantifier: every` over all-negated patterns; `test` heavy steps gated with the `unsafe != 'false'` fail-safe form; `docs-lint` always runs; required context names/matrix unchanged.

### Unit B — Gate cli-reference-drift.yml drift steps behind a fail-safe `changes` job (config)

- **Files**: `.github/workflows/cli-reference-drift.yml` (1 file).
- **Design principle**: same source-verified `unsafe` all-negation-under-`every` detector as Unit A, PLUS a single-pattern `cli_ref_touched` filter, because drift must ALSO run on manual edits to the generated CLI reference (`docs/cli-reference/**`) — the drift check exists precisely to catch those (see the `cli-reference-drift-check-manual-edits-bypass` compound learning). **Do NOT express the cli-reference carve-out as an in-list negation appended to a positive filter** (round-2 fail-open) — combine two independent outputs in the gate `if:`.
- **Changes**:
  1. Add a `changes` job (same shape as Unit A: `contents: read`, `timeout-minutes: 5`, checkout `persist-credentials: false`, paths-filter SHA-pinned, `predicate-quantifier: 'every'`) exposing **two** outputs:
     - `unsafe` — `['!docs/**','!**/*.md','!.backlogit/**']`; `'true'` iff any changed file is outside the allowlist (identical to Unit A).
     - `cli_ref_touched` — `['docs/cli-reference/**']`; a single pattern, so `every` and `some` are equivalent -> `'true'` iff any changed file is under the generated CLI reference.
  2. `drift` job (name "CLI Reference Drift"): add `needs: changes` **and job-level `if: ${{ !cancelled() }}`** (see fail-safe note below). Keep `checkout` unconditional. Gate `setup-go`, `Generate CLI reference`, and `Check for drift` with:
     - `if: needs.changes.outputs.unsafe != 'false' || needs.changes.outputs.cli_ref_touched == 'true'`
     - i.e. **run drift UNLESS** the change is provably docs/backlog-only AND leaves `docs/cli-reference/**` untouched. A pure code PR (`unsafe='true'`) runs drift; a `docs/cli-reference/*.md` edit (`cli_ref_touched='true'`) runs drift; only a `docs/foo.md`-only change (`unsafe='false'`, `cli_ref_touched='false'`) skips it; an infra failure (`unsafe=''`) runs it. Name preserved -> context "CLI Reference Drift" preserved and always reports a genuine conclusion.
  3. Add the same invariant comment, plus an inline cross-reference comment noting the `unsafe` allowlist is contractually identical to `ci.yml`'s `changes` job (Unit C asserts consistency).
- **Fail-safe on `changes` infra failure**: because the `drift` job carries job-level `if: ${{ !cancelled() }}`, a failed or absent `changes` job still runs `drift`; the step-level `if:` reads empty outputs (`'' != 'false'` -> true) so the drift steps RUN. This makes a change-detection infra failure fail *toward running* heavy verification (fail-safe), not toward a silently-passing skipped context.
- **Tests**: covered by Unit C.
- **Execution posture**: characterization-first.
- **Atomic milestone**: `cli-reference-drift.yml` parses; `drift` job always runs (`!cancelled()`); regen/drift steps gated behind the two-positive-output fail-safe combination; context name unchanged.

### Unit C — Extend CI workflow characterization tests (tests)

- **Files**: `tests/integration/ci_compliance_test.go` (1 file).
- **CRITICAL parsing constraint (shared struct / release.yml compatibility)**: `readCIWorkflow`/`ciWorkflow`/`ciJob` are **shared** and also parse `release.yml`, which uses BOTH scalar (`needs: test`) and sequence (`needs: [changelog, build]`) forms of `needs:`. Typing a new `ciJob.Needs` field as `string` **or** `[]string` makes `yaml.Unmarshal` fail on release.yml and regresses the 4 existing green tests. (The "YAML ignores unknown fields" backward-compat property applies only to *unknown keys*, NOT to a newly-typed field whose value shape varies.) **Therefore**: if `Needs` is captured at all, type it as `Needs any` (or a custom `stringOrSlice` with an `UnmarshalYAML` that accepts both scalar and sequence). **Preferred: omit `Needs` entirely** — the gating assertions rely on step-level `If`, job `Outputs`, job `Name`, and matrix, none of which require `Needs`.
- **Changes**:
  1. Extend the YAML parse structs **conservatively** to capture what the invariants need, without breaking release.yml parsing:
     - `ciStep.If string \`yaml:"if"\`` (step-level gate).
     - `ciJob.Outputs map[string]string \`yaml:"outputs"\`` (change-detection outputs).
     - `ciJob.Name string \`yaml:"name"\`` (required for the "Docline frontmatter gate" / "CLI Reference Drift" name assertions — previously missing from the enumeration).
     - `Strategy.Matrix` capture for the `["1.23","1.24"]` assertion (e.g. `ciJob.Strategy struct{ Matrix map[string][]any \`yaml:"matrix"\` } \`yaml:"strategy"\``). Use `[]any` (not `[]string`) for forward-safety — a future matrix using `include:`/`exclude:` sequences-of-mappings or unquoted numerics would break `[]string` in the SHARED reader; read `1.23`/`1.24` via string coercion.
     - Do **NOT** add a typed `Needs` field unless typed as `any`/custom-unmarshaler (see constraint above).
     - Add a helper to read `cli-reference-drift.yml`.
  2. Detect workflow-level path filtering via a **raw-content scan anchored to the `on:` block** (not struct absence, and not a whole-file `strings.Contains`): `On`->`bool` under YAML 1.1 and yaml.v3's silent drop of unknown keys can make a struct-field `On.Push.Paths == nil` pass *vacuously*. Read the raw workflow bytes, scope to the `on:` block, and assert no `paths:`/`paths-ignore:` key appears there (scoping avoids a future false positive from an unrelated `paths:` in a step; note `path:` singular in release.yml must not match), AND add a **positive parse canary** (assert the parsed `On.PullRequest.Branches` contains `main`; sound under yaml.v3 which — unlike yaml.v2 — does not coerce `on`->bool, so the `yaml:"on"` field populates) to prove the `on:` block actually parsed rather than silently dropping.
  3. Add assertions (grouped as subtests under a small number of top-level `Test...` functions to respect the granularity budget):
     - **Required contexts preserved**: `ci.yml` job `test` exists with matrix values `1.23` and `1.24`; job `docs-lint` sets `name: Docline frontmatter gate`; `cli-reference-drift.yml` job `drift` sets `name: CLI Reference Drift`.
     - **No workflow-level path filtering (fail-safe)**: raw-content scan (per step 2) confirms neither required workflow declares `paths`/`paths-ignore` under `on:`; positive canary confirms `on:` parsed.
     - **Fail-safe gate DIRECTION (not just presence)**: in `ci.yml` `test`, the `Lint`/`Vet`/`Test` steps carry an `If` containing `needs.changes.outputs.docs_only` AND the negative form `!= 'true'` (asserts the *fail-safe* direction — heavy work runs unless provably docs-only), verified with `strings.Contains`. For `cli-reference-drift.yml` `drift`, the regen steps' `If` contains BOTH `docs_only != 'true'` and `cli_ref_touched == 'true'` combined with `||` (run drift unless provably docs-only AND cli-reference untouched).
     - **Fail-safe on changes-failure**: assert the `test` and `drift` jobs carry a job-level `If` of `${{ !cancelled() }}` (so a failed/absent `changes` job still runs heavy verification rather than reporting a silently-passing skipped context).
     - **`docs-lint` ungated / always-run**: assert `docs-lint`'s `make docs-lint` step has no `needs.changes`-based `If` (it always runs).
     - **Unenumerated-path fail-safe (regression guard)**: assert the `docs_only` filters (both workflows) use `predicate-quantifier: every` and enumerate ONLY the safe allowlist (`docs/**`, `**/*.md`, `.backlogit/**`) with NO in-list negation, so a fabricated unenumerated path such as `schemas/x.json` (matching no safe pattern) necessarily yields `docs_only=false` -> heavy jobs run. (Encoded as an allowlist-membership assertion over the filter patterns, since the test cannot invoke the Action; documents the invariant that the safe set is a closed allowlist, never a code denylist, and never a trailing negation.)
     - **Cross-file filter consistency**: assert the `docs_only` safe allowlist (`docs/**`, `**/*.md`, `.backlogit/**`) is identical across both workflows' `changes` jobs, guarding against silent filter drift between the two duplicated jobs.
     - **Behavioral file-set matrix (drift gate)**: encode the intended drift-gate truth table as documented target scenarios that Ship MUST observe on the introducing/canary PRs (since the test cannot invoke the Action): (a) a `cmd/gen-docs/**` or other code change -> drift RUNS; (b) a `docs/cli-reference/*.md` edit -> drift RUNS; (c) a `docs/foo.md`-only change -> drift MAY skip. This closes the "green-context-that-verified-nothing" gap.
     - **`changes` job present**: both workflows define a `changes` job exposing the expected single output.
     - **SHA-pin + persist-credentials coverage extended to `cli-reference-drift.yml`** (the existing `TestAllActionsUseSHAPins`/`TestCheckoutStepsNoPersistCredentials` only cover `ci.yml` + `release.yml`; add `cli-reference-drift.yml` so the new `dorny/paths-filter` and checkout are pin-checked).
- **Execution posture**: **test-first (red phase mandatory)** — Ship commits Unit C first and observes it FAIL (red) against the unmodified workflows before editing YAML, proving the assertions bite; Units A+B then make them green. This prevents a vacuous/always-green test.
- **Atomic milestone**: `go test ./tests/integration/...` passes and asserts every invariant above; the 4 pre-existing tests (incl. release.yml parsing) stay green.

## Dependency graph

- Unit A and Unit B are independent (different files) and can be developed in parallel.
- Unit C (tests) encodes the contract that Units A and B satisfy. **Test-first**: Ship commits Unit C first and observes the new assertions FAIL (red) against the unmodified workflows, then applies Units A+B to turn them green. Model as: **C depends on A and C depends on B** (C's green state requires A+B). All three land in a single Ship PR; CI runs on the combined diff, so intra-PR ordering does not gate mergeability.
- No cycles.

## Decisions and rationale

- **Step-level gating on always-running jobs (not job-level `if:`/`needs`-skip)**: guarantees a genuine `success` conclusion for every required context, avoiding the ambiguous `skipped` state (and the matrix-skip sharp edge) that could stall branch protection. Endorsed by the repo's own `2026-07-04-release-docs-hygiene-deliberation.md`.
- **`dorny/paths-filter` (SHA-pinned) over native `git diff`**: battle-tested handling of PR (base..head) vs push (before..after), force-push, and first-commit edge cases; a custom differ risks mis-classifying a code PR as docs-only and skipping tests under auto-merge. `TestAllActionsUseSHAPins` enforces the pin.
- **Fail-safe NEGATIVE gating (`docs_only`/`drift_irrelevant` with `predicate-quantifier: every`) over a positive code allowlist**: heavy work skips only when EVERY changed file is inside a closed safe allowlist (`docs/**`, `**/*.md`, `.backlogit/**`). A positive `code` allowlist would be **fail-open** — any path matching neither `code` nor `docs` (`schemas/**`, `scripts/**`, `plugin/**`, `retired package tree`, `.mcp.json`, `.github/**` non-`.md`, root configs) would set the flag false and skip verification, contradicting INV-3 under AFK auto-merge. The negative predicate makes unenumerated/new paths default to running heavy jobs.
- **`docs-lint` (Docline gate) always runs, ungated**: `make docs-lint` is cheap relative to the Go matrix and always relevant on docs PRs / harmless on code PRs; leaving it ungated removes a whole gating dimension (and its skip-risk) and keeps the required context a genuine run every time.
- **Preserve required context names / matrix**: no branch-protection reconfiguration is needed — an admin action outside Stage/Ship authority. The design is entirely in-tree workflow + test edits.
- **Inline invariant comments instead of a separate design doc**: keeps the warning where a future editor will see it and avoids adding a new in-scope `docs/**` docline artifact (YAGNI).

## Risks and caveats

- **A required check silently stops reporting -> unmergeable PR.** Mitigation: no workflow-level path filtering (asserted by Unit C); jobs always run; first post-merge docs-only closure PR is the live canary.
- **Mis-classification skips tests on real code under AFK auto-merge.** Mitigation: **fail-safe negative** classification — heavy work skips only when EVERY changed file is in the closed safe allowlist; any unenumerated/new/code path forces the full matrix. Step-level gating so contexts still report genuine success.
- **Supply-chain exposure from `dorny/paths-filter`.** Mitigation: 40-char SHA pin, `contents: read` only, read-only usage.
- **Existing characterization tests break on the structural change.** Mitigation: Unit C extends the parser conservatively — new struct fields are captured as unknown-key-tolerant additions, and any `Needs` capture is typed `any`/custom-unmarshaler (NOT `string`/`[]string`) so the SHARED `readCIWorkflow` still parses `release.yml`'s dual scalar/sequence `needs:` forms; the 4 pre-existing tests stay green.
- **`changes` job infra failure runs heavy verification (fail-safe).** The `test`/`drift` jobs carry job-level `if: ${{ !cancelled() }}`, so a failed/absent `changes` job does not skip them — the step-level `if:` reads empty outputs (`'' != 'true'` -> true) and heavy work RUNS. This deliberately avoids relying on branch protection's ambiguous treatment of a `skipped` required check (often treated as passing), which would be fail-open. An infra failure thus blocks merge via a genuine test failure/run, not a silent skip.

## Plan Hardening Signals (REQUIRED)

- **public API, schema, or contract change** — PRESENT (contract sense): the *set of branch-protection required status-check contexts* is a merge contract. The plan must preserve `test (1.23)`, `test (1.24)`, `Docline frontmatter gate`, `CLI Reference Drift` exactly. Justification: any rename/suppression breaks mergeability.
- **security, auth, permission, or compliance-sensitive behavior** — PRESENT: the change alters merge-gating behavior and touches security-relevant gates (Docline frontmatter, CLI drift, lint/vet/test). Weakening or unreporting them is a compliance/merge-safety hazard. Justification: must not weaken security-sensitive gates; must keep them reporting.
- **migration, backfill, destructive data/config action, or irreversible step** — ABSENT: workflow-file edits only; fully reversible by revert; no data/config migration.
- **external integration, operator checkpoint, or external dependency** — PRESENT: introduces a third-party GitHub Action (`dorny/paths-filter`). Justification: supply-chain surface; SHA-pin + read-only scope required.
- **high runtime, rollout, or rollback risk** — PRESENT (moderate): a mis-designed filter could make PRs unmergeable (the core hazard) or skip tests on real code under auto-merge. Justification: warrants explicit verification + rollback detail and a live canary.

**Requires plan hardening: yes**

## Runtime verification and closure

- **Runtime surface changed**: CI/CD automation (GitHub Actions), which gates every merge — a high-leverage runtime surface.
- **Runtime verification (before considered absorbed)**:
  1. Local: `go test ./tests/integration/...` green (Unit C asserts the invariants). Ship runs this — Stage does not.
  2. On the introducing PR: because it touches `.github/workflows/**` and `tests/**` (neither in the safe allowlist), `docs_only` is `false` -> the full `test` matrix and drift regen run and pass on the PR itself (proves the gated path still executes heavy work when non-docs files change), AND all four required contexts report `success`.
  3. Post-merge canary: the next docs-only closure PR (this pipeline's own output) must show all four required contexts as `success` and be mergeable. Observe on that PR that the heavy STEPS were skipped (minute savings realized) while contexts stayed green.
- **Operational closure artifact**: post-merge closure record noting (a) the introducing PR's check outcomes, (b) the first docs-only PR canary result, (c) monitoring signal + rollback trigger below.
  - **Monitoring signal**: (a) any required check left in `Expected`/`Pending` on a docs-only PR; or (b) **on a PR that touches any non-safe (code/config/schema) path, the `Lint`/`Vet`/`Test` STEPS reported `skipped`** (the matrix JOB always runs, so a skipped-job signal can never fire — the real failure mode is skipped STEPS on a code-bearing PR). Surface the resolved `docs_only`/`drift_irrelevant` flag and the changed-file classification in the job summary/log so this is observable.
  - **Rollback trigger**: either signal on the first post-merge docs-only PR or the first subsequent code PR.
  - **Rollback procedure**: revert the two workflow files (single-commit revert restores prior always-run behavior); no data/state to unwind.
  - **Owner**: Ship (execution) with operator monitoring the first post-merge docs-only PR.
  - **Validation window**: through the first post-merge docs-only PR and the first subsequent code PR.

## Plan Hardening

**Hardening required: yes.** Triggered by three present signals — (1) contract-preservation of the branch-protection required-check context set, (2) security/compliance-sensitive merge-gating behavior, and (3) a new external third-party action — plus moderate rollout/rollback risk. This section reinforces verification, rollback, and guardrails before review.

### Learnings and instruction files consulted

- `docs/compound/github-actions/F013-workflow-sha-pinning.md` — required contexts are matrix leg names; 40-char SHA pinning mandatory; ci.yml is characterized by `tests/integration/ci_compliance_test.go`.
- `docs/decisions/2026-07-04-release-docs-hygiene-deliberation.md` — step-level `if:` guards keep a job's context reporting; job-level/`needs`-skip yields an ambiguous `skipped` context.
- `docs/compound/2026-06-26-docline-frontmatter-contract.md` — Docline gate = ci.yml `make docs-lint`; docs-only PRs genuinely exercise it.
- `docs/compound/workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` — the drift check's real dependency paths (`internal/**`, `cmd/**`, `docs/cli-reference/**`).
- `.github/instructions/ci-security.instructions.md`, `.github/instructions/workflows.instructions.md` — SHA-pinning, least-privilege `permissions`, `persist-credentials: false`, concurrency conventions (to re-read at execution time).

### Protected invariants (must hold after the change)

1. **INV-1 (required-check satisfiability):** the four contexts `test (1.23)`, `test (1.24)`, `Docline frontmatter gate`, `CLI Reference Drift` report a `success` conclusion on every PR type (incl. docs-only closure PRs). No workflow-level `paths`/`paths-ignore` on required workflows. `docs-lint` is genuinely unconditional; `test`/`drift` run on any successful change-detection outcome AND run heavy work (fail-safe) if the `changes` node itself fails (job-level `if: ${{ !cancelled() }}`), so they never resolve to a silently-passing skipped context.
2. **INV-2 (name/matrix stability):** required context names and the `["1.23","1.24"]` matrix are unchanged, so no branch-protection reconfiguration is required.
3. **INV-3 (fail-safe classification):** any code-touching change runs the full `test` matrix; only provably docs/backlog-only diffs skip heavy work. Ambiguity resolves toward running heavy jobs.
4. **INV-4 (no gate weakening):** Docline, CLI-drift, and lint/vet/test still execute on their relevant paths; the change only suppresses provably-irrelevant work.

### Risky actions (ProposedAction / ActionRisk / ActionResult)

- **ProposedAction PA-1**: Add `dorny/paths-filter` third-party action to `ci.yml` and `cli-reference-drift.yml`.
  - **ActionRisk**: MEDIUM — supply-chain surface on a merge-gating workflow.
  - **Guardrails / approval**: pin to a full 40-char commit SHA (enforced by `TestAllActionsUseSHAPins`, extended to cli-reference-drift.yml in Unit C); grant only `contents: read` (git-mode diff via checkout; no `pull-requests` scope needed — least privilege); read-only usage (no outputs beyond boolean flags). No operator approval needed beyond plan-review PASS.
  - **ActionResult (expected)**: pinned action present; SHA-pin test green.
- **ProposedAction PA-2**: Gate heavy CI steps (`test` matrix, drift regen) behind the **fail-safe negative** change-detection outputs (`docs_only`/`drift_irrelevant` `!= 'true'`, computed under `predicate-quantifier: every`). `docs-lint` stays ungated (always runs).
  - **ActionRisk**: MEDIUM — a mis-designed filter could skip verification on real code under AFK auto-merge, or (if mis-designed) suppress a required context.
  - **Guardrails**: step-level `if:` on always-running jobs (contexts always report genuine success); **closed safe allowlist with negative predicate** so unenumerated/new paths default to running heavy jobs (fail-safe, not fail-open); Unit C asserts INV-1..INV-4 including the fail-safe gate *direction* and unenumerated-path regression guard. No destructive/irreversible effect.
  - **ActionResult (expected)**: on the introducing PR (touches `.github/workflows/**` + `tests/**`, both non-safe) the full matrix + drift run and pass and all four contexts report success; on the first docs-only PR, heavy steps skip while all four contexts report success.
- No destructive, migration, or data/config actions are proposed (workflow + test edits only).

### Added verification depth

- **Environment precheck**: confirm the pinned `dorny/paths-filter` SHA corresponds to a legitimate release tag before merge (Ship verifies at pin time).
- **Target scenarios** (must be observed): (a) non-safe PR (code/config/schema) -> `test (1.23)`/`test (1.24)` run heavy work and pass; (b) docs-only PR -> all four contexts report `success` with heavy STEPS skipped; (c) `docs/cli-reference/**` or CLI-source PR -> `CLI Reference Drift` runs regen; (d) fabricated unenumerated path (e.g. `schemas/x.json`) -> `docs_only=false` -> heavy jobs run (fail-safe).
- **Blocked-path handling**: if the `changes` job fails (infra), the `test`/`drift` jobs still run (`if: ${{ !cancelled() }}`) with empty outputs -> heavy steps execute -> genuine pass/fail reported (fail-safe toward running verification, not toward a silent skip).
- **Local gate (Ship)**: run the full ordered quality suite before opening the PR — `gofmt`/`goimports` check, `go vet ./...`, `golangci-lint run`, then `go test ./...` (not only `./tests/integration/...`) — and observe the Unit C red phase first (tests fail against unmodified workflows) before applying Units A+B.

### Rollback

- **Trigger**: any required context left `Expected`/`Pending` on the first post-merge docs-only PR, or a PR touching non-safe paths whose `Lint`/`Vet`/`Test` STEPS reported `skipped`.
- **Procedure**: `git revert` the commit touching the two workflow files (and the test file if needed). Single-commit, workflow-only revert restores the prior always-run behavior. No data/state to unwind. Blast radius: CI config only.
- **Owner**: Ship executes; operator monitors the first post-merge docs-only PR (validation window: first docs-only PR + first subsequent code PR).

### Unresolved operator decisions

- None blocking. The concrete `dorny/paths-filter` pin SHA is an execution-time detail for Ship. Branch-protection settings are intentionally untouched (no admin action required).

## Plan Review

<!-- plan-review-attempt: 1 -->

### Round 1 — verdict: FAIL (revised)

A 5-persona panel (Security Lens, Architecture Strategist, Scope Boundary Auditor, Constitution Reviewer, Go Reviewer) reviewed the pre-revision plan. Scope Boundary = PASS; Constitution = ADVISORY; **Security Lens, Architecture Strategist, and Go Reviewer = FAIL** on two converging P1s. All reviewers agreed the core topology (always-run jobs + step-level `if:` gating preserving genuine `success` for required contexts; SHA-pinning; least privilege; `pull_request` not `pull_request_target`; Principle IV in-tree-only; name/matrix preservation) is sound.

**P1-A (fail-open filter) — Security Lens + Architecture Strategist, independently.** The original positive `code` allowlist was fail-OPEN: any changed path matching neither `code` nor `docs` (`schemas/**` incl. `schemas/docline/base-frontmatter-v1.schema.json`, non-`.md` `scripts/**`, `plugin/**`, `retired package tree`, `.mcp.json`, non-workflow `.github/**` like `.golangci.yml`/`dependabot.yml`/composite `action.yml`) set both flags false -> heavy steps skipped -> unverified auto-merge, contradicting INV-3.
- **Resolution**: inverted to a **fail-safe negative predicate** — a single `docs_only` (ci.yml) / `drift_irrelevant` (cli-reference-drift.yml) computed with `predicate-quantifier: 'every'` over a closed safe allowlist (`docs/**`, `**/*.md`, `.backlogit/**`; drift excludes `docs/cli-reference/**`). Heavy steps gated `if: needs.changes.outputs.<flag> != 'true'`. Unenumerated/new paths default to running heavy jobs. `docs-lint` made always-run (ungated). Unit C adds a fail-safe-DIRECTION assertion and an unenumerated-path (`schemas/x.json`) regression guard. (Units A, B, C; Decisions; Risks; AC-1; PA-2.)

**P1-B (shared struct breaks release.yml) — Go Reviewer.** Adding `Needs` to the shared `ciJob` struct as `string`/`[]string` breaks yaml.v3 parsing of `release.yml` (which uses both scalar `needs: test` and sequence `needs: [changelog, build]` forms) -> `Unmarshal` error -> 4 existing green tests regress. The line-100 "backward-compatible" claim was wrong (it holds only for unknown YAML *keys*, not newly-typed fields).
- **Resolution**: Unit C now omits `Needs` (preferred) or types it `any`/custom `stringOrSlice` unmarshaler; corrected the backward-compat rationale in Risks; added the previously-missing `ciJob.Name` and `Strategy.Matrix` fields needed by the name/matrix assertions.

**P2s folded in.** (a) Monitoring signal redefined to detect skipped STEPS on a non-safe PR (the matrix job always runs; only steps skip). (b) Vacuous `on:`-guard risk addressed via raw-content scan for `paths:`/`paths-ignore:` plus a positive parse canary (`On.PullRequest.Branches` contains `main`). (c) Cross-file `changes`-filter consistency assertion added. (d) Least-privilege: dropped `pull-requests: read` (git-mode `contents: read` only); added `timeout-minutes: 5` on the `changes` SPOF job. (e) Constitution: test-first red-phase mandated; full ordered local quality suite (`gofmt`/`vet`/`golangci-lint`/`go test ./...`) specified.

<!-- plan-review-attempt: 2 -->

### Round 2 — verdict: FAIL (revised)

Re-review of the round-1 revision by the three previously-failing personas: **Architecture Strategist = PASS** (3 P3s), **Go Reviewer = PASS** (4 P3s — both confirmed P1-A and P1-B resolved for `ci.yml`/Unit C), but **Security Lens = FAIL** on a *new* P1 introduced by the round-1 fix:

**P1-C (round-1 revision relocated a fail-open into Unit B's drift filter).** The `drift_irrelevant` negation approach (`!docs/cli-reference/**` appended last, relying on "gitignore last-match-wins") is wrong: dorny/paths-filter evaluates each file with per-file OR (`.some()`) semantics, NOT gitignore last-match-wins. Two fail-open consequences: (1) a `docs/cli-reference/*.md` edit still matches the earlier `docs/**`/`**/*.md` patterns → classified `drift_irrelevant=true` → drift SKIPS (the exact manual-edit bypass the drift check exists to catch); (2) `!docs/cli-reference/**` matches every file NOT under that path (including code), so under `every` a pure code PR touching `cmd/gen-docs/**` yields `drift_irrelevant=true` → drift regen SKIPS. The `CLI Reference Drift` context still reports green `success` (job runs, steps skip), masking the gap under AFK auto-merge. Also **P2**: `test`/`drift` carry `needs: changes`; if `changes` fails (infra), the dependents report `skipped`, which branch protection typically treats as passing → fail-open.
- **Resolution (round 3)**: replaced the negation filter with **two independent positive outputs** in the drift `changes` job — `docs_only` (every changed file in the safe allowlist, no negation) and `cli_ref_touched` (some changed file under `docs/cli-reference/**`, default `some` quantifier). Gate drift steps `if: needs.changes.outputs.docs_only != 'true' || needs.changes.outputs.cli_ref_touched == 'true'` — run drift unless the change is provably docs/backlog-only AND leaves cli-reference untouched. For the `needs: changes` infra-failure hazard, added job-level `if: ${{ !cancelled() }}` to the `test` and `drift` jobs so a failed/absent `changes` output (empty string `!= 'true'`) still RUNS heavy work (fail-safe), removing reliance on the ambiguous skipped-conclusion behavior. Unit C strengthened from structure-only to a behavioral file-set matrix (target scenarios: `cmd/gen-docs` change MUST run drift; `docs/cli-reference/*.md` edit MUST run drift; `docs/foo.md`-only MAY skip). INV-1 wording tightened (Architecture P3). Go P3s (forward-safe `Strategy.Matrix`, anchored raw scan) noted for Ship.

<!-- plan-review-attempt: 3 -->

### Round 3 — verdict: FAIL

Re-review of the round-2 revision by Security Lens returned **FAIL** on a subtle but decisive P1 about the *actual* semantics of `dorny/paths-filter`'s `predicate-quantifier: 'every'`:

**P1-D (`docs_only` positive-pattern filter under `every` is a constant-false no-op).** The round-2 design assumed `predicate-quantifier: 'every'` over a positive allowlist (`docs/**`, `**/*.md`, `.backlogit/**`) yields `'true'` iff *every changed file is in the allowlist*. That mental model — shared by the author and the round-1/round-2 reviewers — is **wrong**. Verified against the authoritative source (`dorny/paths-filter` `README.md` and `src/filter.ts`): under `every`, `isMatch(file) = patterns.every(rule => rule.isMatch(file))` and each pattern is an independent `picomatch(pattern)`; the filter output is `'true'` iff **some changed file matches ALL patterns**. The three allowlist patterns are mutually disjoint (no file is under `docs/` AND `*.md` AND under `.backlogit/` simultaneously), so `docs_only` is **constant-false for every diff** → heavy steps always run → the design is safe-by-accident, never realizes the cost objective, and the inevitable "why no savings?" correction risks re-opening the rounds 1–2 fail-open. (The `!cancelled()` job-guard fix from round 2 was confirmed correct.)

- **Correct construction (source-verified, round 4)**: detect the *existence of a file outside the allowlist* using **all-negated patterns under `every`** — `unsafe: ['!docs/**','!**/*.md','!.backlogit/**']` with `predicate-quantifier: 'every'` → `unsafe='true'` iff any changed file matches all three negations (i.e. is outside the allowlist). Gate heavy steps `if: needs.changes.outputs.unsafe != 'false'` (runs on `'true'` and on empty `''` from an infra failure → fail-safe; skips only on an explicit `'false'` = provably docs-only). For drift, add a single-pattern `cli_ref_touched: ['docs/cli-reference/**']` (quantifier-invariant for one pattern) and gate `if: needs.changes.outputs.unsafe != 'false' || needs.changes.outputs.cli_ref_touched == 'true'`. This truth table has no fail-open: code PR → runs; cli-reference edit → runs; docs-only → skips; infra failure → runs. Unit C must add a **required-SKIP canary** (a docs-only PR must be observed to actually SKIP heavy steps) so a future regression back to a no-op or a fail-open `some` form is detectable.

### Gate outcome: DEFER (cycle budget exhausted)

**D760E508 is DEFERRED — not harvested this session.** Rationale:

1. **Plan-review cycle budget exhausted.** Three consecutive FAILs (attempt 1: fail-open positive allowlist + shared-struct regression; attempt 2: drift-filter negation fail-open; attempt 3: `every`-semantics constant-false no-op), each on the *core merge-safety mechanism*. The Step-4 contract halts at attempt count 3.
2. **Operator's explicit AFK guidance for this exact item.** "If a design that provably preserves required-check satisfiability is not clearly achievable, DEFER `D760E508` with a documented plan-review finding rather than risk breaking merges in an AFK run." Three rounds of the core control being wrong — each in a way that fooled the author and multiple expert reviewers — is strong evidence the design is **not *clearly* achievable within a safe review budget**, even though a correct construction now exists on paper.
3. **P-005 integrity.** Self-certifying a 4th, unreviewed revision past an exhausted gate would bypass the very control that just caught three real defects (including a confidently-wrong round-3 self-assessment). Under AFK auto-merge, the failure mode is exactly the operator's stated worst case (unmergeable required checks / unverified auto-merge).
4. **Low cost of deferral.** The correct, source-verified design is captured above and in the plan body (below) for a future fresh-budget Stage session (ideally operator-attended) to implement and pass through a clean plan-review. The closure-compaction item (`2EF8B7AD`) still ships value this session.

**State handling**: stash entry `D760E508` remains **active** (not archived). No feature/task/shipment is created for it this session. The plan body below has been updated to the round-4 source-verified `unsafe`-negation design and is marked **UNREVIEWED — pending a fresh plan-review cycle before harvest**.

<!-- plan-review-status: DEFERRED — cycle budget exhausted after 3 consecutive FAILs; round-4 corrected design is UNREVIEWED and must pass a fresh plan-review before harvest -->
<!-- plan-review-attempt: 3 (max re-entry cycles reached; halted per Step-4 contract) -->
