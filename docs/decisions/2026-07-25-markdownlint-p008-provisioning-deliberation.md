---
title: "Provision markdownlint tooling to make policy P-008 reproducible and CI-enforced"
description: "Decision to provision a scoped markdownlint (MD001/MD025/MD041) gate — config, ignore set, Makefile target, and an always-reporting CI job — plus a P-008 wording reconciliation, without a repo-wide retroactive markdown cleanup (stash B3D30415, deliberation 054-DL)."
source: docs/decisions/2026-07-25-markdownlint-p008-provisioning-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Make P-008 markdown heading-hierarchy gate reproducible and CI-enforced without a 248-file retroactive cleanup"
depth: standard
decision_status: decided
promoted_to: plan
stash_id: B3D30415
deliberation_id: 054-DL
linked_artifacts:
  - "docs/exec-plans/2026-07-25-markdownlint-p008-provisioning-plan.md"
tags:
  - "ci"
  - "markdownlint"
  - "policy-p008"
  - "tooling-debt"
  - "github-actions"
---

## Problem Frame

Policy **P-008 (Markdown Conformance)** in `.github/policies/workflow-policies.md`
(lines ~159–182) declares:

- a **precondition** (present tense) that "The workspace has a `.markdownlint.json`
  config enabling MD001, MD025, MD041", and
- a **postcondition** that `markdownlint "**/*.md"` exits 0 for staged/committed
  Markdown.

Reality: the repository ships **no `.markdownlint.json`**, **no Makefile `md-lint`
target**, and **no markdownlint CI job**. Only `make docs` (CLI reference
generation) and `make docs-lint` (Go docline frontmatter gate) exist. As a result:

- The P-008 heading-hierarchy gate is **not reproducible** (no config to run) and
  **not CI-enforced** (no job).
- Agents that "run markdownlint" fall back to the tool's built-in defaults (dozens
  of unrelated rules), producing inconsistent, non-authoritative results.

**Complication (blast radius).** A read-only inventory — markdownlint-cli2 v0.23.1,
config `{ "default": false, "MD001": true, "MD025": true, "MD041": true }`,
restricted to git-tracked `.md` — found **249 violations across 248 tracked files**
(MD025 multiple-H1 = 228, MD041 first-line-H1 = 20, MD001 increment = 1). A literal
`markdownlint "**/*.md"` exit-0 gate therefore **cannot** be switched on repo-wide
today without a large retroactive edit.

Inventory by top directory: `docs` = 171 (closure 74, compound 38, exec-plans 23,
archive 20, reviews 5, design-docs 4, decisions 3, memory 3, research 1),
`.backlogit` = 49 (machine-generated backlog markdown), `.github` = 14 (mostly
template-generated `.github/skills/*/SKILL.md` MD041, plus README/AGENTS MD025),
`plugin` = 8, `.autoharness` = 3, `.copilot-tracking` = 2. Untracked/ephemeral
`.copilot/` (473) and `logs/` also violate but are never committed.

Surfaced by **PR #297** Copilot review of the 125-F docs-consistency plan.

## Research Findings

Grounded in two high-confidence compound learnings:

- **`docs/compound/github-actions/F013-workflow-sha-pinning.md`** — all third-party
  actions MUST be pinned to full 40-char SHAs; YAML workflow work uses a
  **characterization-first** posture (integration tests in `tests/integration/`
  parse the YAML and go RED before the edit, GREEN after). 089-S confirmed the
  current contract: `ci.yml` is PR-only, required contexts always report, and
  workflow-level `paths:`/`paths-ignore:` are forbidden — gate expensive steps
  *inside* always-reporting jobs.
- **`docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md`**
  — a **single-pattern** paths-filter (e.g. `md_touched: ['**/*.md']`) is
  quantifier-invariant, so it fires correctly under the workflow's existing
  `predicate-quantifier: 'every'`. Gate steps with `!= 'false'` (fail-safe) and keep
  the job at `${{ always() && !cancelled() }}` so a skipped step never removes a
  required check context.

Codebase facts:

- `.github/workflows/ci.yml`: PR-only; `permissions: contents: read`; a `changes`
  job (dorny/paths-filter pinned `d1c1ffe0…` v3.0.3) already classifies `code`,
  `docline_required`, `plugin_bundle`, `cli_reference`, and **already excludes
  `.backlogit/**`**. Jobs `test`, `docs-lint`, `cli-reference-drift` all run
  `${{ always() && !cancelled() }}` with step-level gating. No Node.js setup exists
  (Go repo). `cli_reference` classification includes `.github/workflows/ci.yml`, so
  editing the workflow re-runs the drift job (harmless).
- `tests/integration/ci_compliance_test.go` **iterates every job** to assert SHA
  pins and least-privilege, and asserts named jobs (`changes`, `test`, `docs-lint`,
  `cli-reference-drift`) exist with the always-reporting `if`. A new `md-lint` job
  must therefore be SHA-pinned and covered by an extended characterization test; it
  does **not** require changing an exact-job-set assertion (none exists).
- `Makefile` targets: build, test, lint, vet, fmt, cover, docs, docs-lint,
  verify-plugin. No `md-lint`.
- Environment: node v24, npx 11.6.2 (markdownlint-cli2 installable via npx), gh
  2.81.0. Repo is CGo-free Go (`modernc.org/sqlite`); Node is a **build/CI-time**
  tool only, not a runtime dependency.
- No prior compound learning specifically about markdownlint provisioning
  (`docs/compound/` search returned no markdownlint/MD025/MD041/md-lint match).

## Options Evaluated

### Option A: Fix all 248 files, then gate repo-wide (`**/*.md`)

Literal satisfaction of P-008's postcondition.

- **Pros**: P-008 text needs no change; the whole corpus is clean.
- **Cons**: 248 files is a multi-hour, multi-domain edit that violates the 2-hour /
  single-skill-domain task-granularity rule. It churns **historical/archival**
  content (docs/closure 74, docs/compound 38, docs/archive 20 — rewriting shipped
  history) and **template-generated** files (`.github/skills/*/SKILL.md`), where an
  in-repo fix is **overwritten on autoharness regeneration** and durable fixing is
  effectively an out-of-tree concern (Constitution Principle IV). High effort, low
  durability, high risk.

### Option B: Scoped rollout with a curated ignore set, widen over time (RECOMMENDED)

Provision the tool + a config that enables **exactly** MD001/MD025/MD041, plus an
ignore set covering every currently-violating tracked directory and all ephemeral
dirs, so the gate is **green on Day 1** and **fail-closed for new/edited files** in
non-ignored locations. Wire an always-reporting `md-lint` CI job. Widening
(bringing ignored dirs into scope after remediation) is separate, honestly-sized
future backlog.

- **Pros**: Immediately reproducible and CI-enforced; matches the intent of P-008
  ("generated and committed Markdown MUST conform" — forward-looking); aligns the
  gate scope with CI's existing `.backlogit/**` exclusion; respects task
  granularity and Principle IV (no churn of archival/template-generated files);
  small, shippable, reversible.
- **Cons**: The enforced scope is narrower than a literal `**/*.md`; P-008 wording
  must be reconciled to describe the config-driven scope; historical dirs stay
  ignored until deliberately remediated.

### Option C: Baseline existing violations, gate only new ones

Generate a baseline snapshot of the 249 violations; the gate fails only on
violations **not** in the baseline.

- **Pros**: Keeps a path toward eventual full coverage while unblocking today;
  touched files still get checked against new violations.
- **Cons**: markdownlint-cli2 has **no first-class baseline** feature (unlike some
  linters); it would need bespoke diff/parse tooling to maintain — extra machinery
  and a new failure surface. A stale baseline **hides regressions** in files that
  are edited but whose pre-existing violations remain baselined. More moving parts
  than Option B for no material gain over an ignore set.

## Trade-off Comparison

| Criterion | A: Fix-all + repo-wide | B: Scoped ignore set | C: Baseline |
|---|---|---|---|
| Effort to first green | Very high (248 files) | Low (config + job) | Medium (baseline tooling) |
| Task granularity fit | Violates 2h/single-domain | Fits | Fits, adds tooling task |
| Principle IV / regeneration risk | High (churns template-gen + archival) | None | Low |
| Durability | Low (regeneration re-dirties) | High | Medium (baseline drift) |
| Reproducible + CI-enforced Day 1 | No | Yes | Yes |
| Hides future regressions | No | No (fail-closed for live dirs) | Yes (stale baseline) |
| Reversibility | Low | High (remove job/config) | Medium |

## Decision

**Adopt Option B — scoped rollout with a curated ignore set.**

Concrete direction (planned as Ship tasks — Stage does not implement):

1. **Config** — add `.markdownlint.json` enabling **exactly** MD001, MD025, MD041
   with `default: false`, plus `.markdownlintignore` listing the currently-violating
   tracked dirs (`.backlogit/`, `docs/archive/`, `docs/closure/`, `docs/compound/`,
   `.github/skills/`) and ephemeral dirs (`.copilot/`, `.copilot-tracking/`,
   `logs/`). Rationale for the filename pair: it honors P-008's existing
   `.markdownlint.json` name (minimizing policy churn) and keeps rule config and
   path-scope cleanly separated; markdownlint-cli2 auto-discovers both.
   (`.markdownlint-cli2.jsonc` with embedded `config` + `ignores` was the considered
   single-file alternative.)
2. **Makefile** — add a `md-lint` target invoking `npx markdownlint-cli2` over the
   configured scope (plus `.PHONY`), so Ship and humans reproduce the exact gate.
3. **CI** — add a new always-reporting `md-lint` job to `ci.yml`: a `md_touched:
   ['**/*.md']` classification output (quantifier-invariant single pattern),
   SHA-pinned `actions/setup-node`, step-level gating on `md_touched != 'false'`,
   job `if: ${{ always() && !cancelled() }}`. No trigger-level path filters.
4. **Characterization test** — extend `tests/integration/ci_compliance_test.go`
   (test-first, RED→GREEN) to assert the `md-lint` job exists, is always-reporting,
   is SHA-pinned, and is wired to `md_touched`.
5. **Policy reconciliation** — update P-008: fix the precondition so it no longer
   overstates that the config already exists (describe the provisioned
   `.markdownlint.json` + `.markdownlintignore` and the enforced scope), and adjust
   the postcondition to reflect config-driven scoped enforcement rather than a bare
   literal `**/*.md`. Keep MD001/MD025/MD041 named.

**Recommended answers to the open questions** (final call deferred to
operator/Ship): make the check **non-required for its first green cycle**, then
promote it to a required status check once observed green on a real PR; keep
historical/archival dirs (`docs/closure`, `docs/compound`, `docs/archive`) and
template-generated `.github/skills` **permanently ignored** (regeneration + shipped
history), and treat only actively-authored live dirs as future widening candidates.

## Rejected Alternatives

- **Option A** rejected: violates task granularity, churns archival + regenerated
  content, and durably fails against autoharness regeneration (Principle IV).
- **Option C** rejected: no native baseline in markdownlint-cli2; bespoke baseline
  tooling adds a failure surface and a stale baseline hides regressions in edited
  files — strictly worse than an ignore set for this repo.

## Unresolved Questions

- Required-vs-advisory timing for the new check (recommend advisory for one cycle,
  then required).
- The final widening schedule and which live dirs graduate out of `.markdownlintignore`
  first (candidate: docs/decisions, docs/memory, docs/research, docs/design-docs —
  small buckets). Sized in the plan's future-widening section; not harvested here.
- Exact `actions/setup-node` pin SHA (Ship resolves at implementation, per F013).

## Risks and Mitigations

- **Fail-open gate** (silently skips) — mitigate with `!= 'false'` step gating and
  `!cancelled()` job `if`, plus a characterization test; per the dorny compound
  learning.
- **New required check blocks unrelated PRs** — mitigate by shipping advisory first,
  promoting to required after one green cycle; the ignore set guarantees Day-1 green.
- **New Node/npm dependency in a Go repo** (Single Responsibility, Principle VI) —
  justified: it is a build/CI-time doc-lint tool only (no runtime/go.mod impact),
  invoked via `npx` with no committed `node_modules`; the action is SHA-pinned.
- **Config drift from P-008 text** — mitigate by reconciling the policy wording in
  the same shipment (task 5) so precondition/postcondition match the provisioned
  reality.
