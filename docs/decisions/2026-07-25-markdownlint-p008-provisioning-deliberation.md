---
title: "Provision markdownlint tooling to make policy P-008 reproducible and CI-enforced"
description: "Decision to provision a repo-wide markdownlint (MD001/MD025/MD041) gate — config (MD025 front_matter_title retargeted to a _title key), paired md-lint scripts, Makefile/make.ps1 targets, and a standalone repo-wide blocking CI job — plus a P-008 reconciliation, via doctor-to-compliance: the _title config dissolves 229 of 250 violations with zero edits and ~21 structural fixes bring the repo to 0 (stash B3D30415, deliberation 054-DL)."
source: docs/decisions/2026-07-25-markdownlint-p008-provisioning-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Make P-008 markdown heading-hierarchy gate reproducible and CI-enforced repo-wide via config plus minimal structural remediation"
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
(lines ~159-182) declares:

- a **precondition** (present tense) that "The workspace has a `.markdownlint.json`
  config enabling MD001, MD025, MD041", and
- a **postcondition** that `markdownlint "**/*.md"` exits 0 for staged/committed
  Markdown.

Reality: the repository shipped **no `.markdownlint.json`**, **no Makefile `md-lint`
target**, and **no markdownlint CI job**. Only `make docs` (CLI reference
generation) and `make docs-lint` (Go docline frontmatter gate) existed. As a result:

- The P-008 heading-hierarchy gate was **not reproducible** (no config to run) and
  **not CI-enforced** (no job).
- Agents that "run markdownlint" fell back to the tool's built-in defaults (dozens
  of unrelated rules), producing inconsistent, non-authoritative results.

**Complication (initial blast radius).** A read-only inventory — markdownlint-cli2
v0.23.1, config `{ "default": false, "MD001": true, "MD025": true, "MD041": true }`
— found **250 violations** across 1,839 tracked files (MD025 multiple-H1 = 229,
MD041 first-line-H1 = 20, MD001 increment = 1).

**Resolution (the `_title` configuration insight).** Nearly all violations (229 of
250) were MD025 false positives caused by the default `front_matter_title` regex
matching `title:` in YAML frontmatter: the frontmatter `title:` was counted as an
H1, so the body `# H1` became a second top-level heading and MD025 fired.
Retargeting MD025's `front_matter_title` to a non-existent `_title` key eliminates
all 229 MD025 violations with **zero file edits**, reducing the real structural
violation count to 21 (20 MD041 + 1 MD001) — a tractable in-place remediation.

Surfaced by **PR #297** Copilot review of the 125-F docs-consistency plan.

## Research Findings

Grounded in a high-confidence compound learning:

- **`docs/compound/github-actions/F013-workflow-sha-pinning.md`** — all third-party
  actions MUST be pinned to full 40-char SHAs; YAML workflow work uses a
  **characterization-first** posture (integration tests in `tests/integration/`
  parse the YAML and go RED before the edit, GREEN after). 089-S confirmed the
  current contract: `ci.yml` is PR-only, required contexts always report, and
  workflow-level `paths:`/`paths-ignore:` are forbidden.

Codebase facts:

- `.github/workflows/ci.yml`: PR-only; `permissions: contents: read`; jobs `test`,
  `docs-lint`, `cli-reference-drift` run with `${{ always() && !cancelled() }}`.
  No Node.js setup existed (Go repo).
- `tests/integration/ci_compliance_test.go` **iterates every job** to assert SHA
  pins and least-privilege. A new `md-lint` job must be SHA-pinned and covered by
  characterization tests.
- `Makefile` targets: build, test, lint, vet, fmt, cover, docs, docs-lint,
  verify-plugin. No `md-lint`. `make.ps1` enumerates Go-oriented targets.
- Environment: node v24, npx 11.6.2 (markdownlint-cli2 installable via npx), gh
  2.81.0. Repo is CGo-free Go (`modernc.org/sqlite`); Node is a **build/CI-time**
  tool only, not a runtime dependency.
- An existing guard test
  (`tests/integration/plugin_manifest_test.go::TestActivePluginDocsDoNotReferenceRetiredNPMWrapper`)
  forbids the substring `npx ` in a fixed `activePaths` list that includes the
  Makefile — so the Node/npx invocation lives in `scripts/md-lint.{sh,ps1}` (the
  repo's established paired-script convention), and the Makefile target calls bash.

## Options Evaluated

### Option A: Fix all files, then gate repo-wide

Literal satisfaction of P-008's postcondition.

- **Pros**: P-008 text needs no change; the whole corpus is clean.
- **Cons (original assessment)**: 250 violations across 1,839 tracked files appeared
  to require a multi-hour, multi-domain retroactive edit violating the 2-hour /
  single-skill-domain task-granularity rule, plus churning historical and
  template-generated content with low durability.
- **Revised assessment**: the `_title` MD025 `front_matter_title` configuration
  dissolves 229 of 250 violations with zero file edits, reducing the real
  remediation to 21 structural fixes (20 SKILL.md missing leading `# H1` + 1
  heading-increment skip). This makes a refined version of Option A — repo-wide
  doctor-to-compliance — tractable within task granularity.

### Option B: Scoped rollout with a curated ignore set (SUPERSEDED)

Provision the tool with an `ignores` glob array covering every currently-violating
tracked directory, so the gate is green on Day 1 for a subset of files. Widening
(bringing ignored dirs into scope) deferred as future backlog.

- **Pros**: Immediately reproducible and CI-enforced for new/edited files in
  non-ignored locations; small and shippable.
- **Cons**: The enforced scope was narrower than a literal `**/*.md`; P-008 wording
  would need to describe a config-driven scope rather than universal enforcement;
  historical dirs would stay ignored indefinitely. **Superseded** by the `_title`
  configuration insight, which makes repo-wide compliance achievable without the
  complexity of an ignore-set mechanism.

### Option C: Baseline existing violations, gate only new ones

Generate a baseline snapshot of violations; the gate fails only on violations not
in the baseline.

- **Pros**: Keeps a path toward eventual full coverage while unblocking today.
- **Cons**: markdownlint-cli2 has no first-class baseline feature; bespoke
  baseline tooling adds a failure surface. A stale baseline hides regressions in
  edited files. More moving parts than the adopted approach for no material gain.

## Trade-off Comparison

| Criterion | A (refined): Config + doctor | B (SUPERSEDED) | C: Baseline |
|---|---|---|---|
| Effort to first green | Low (config + 21 fixes) | Low (config + job) | Medium (baseline tooling) |
| Task granularity fit | Fits (21 edits, 3 domains) | Fits | Fits, adds tooling task |
| Principle IV / regeneration risk | Low (SKILL.md fixed in-place; upstream template follow-up) | None | Low |
| Durability | High (repo-wide, no maintenance) | Medium (requires maintenance) | Medium (baseline drift) |
| Reproducible + CI-enforced Day 1 | Yes | Yes | Yes |
| Hides future regressions | No | No (fail-closed for live dirs only) | Yes (stale baseline) |
| P-008 fully satisfied | Yes (repo-wide exit 0) | No (subset) | Partially |
| Reversibility | High (remove job/config) | High (remove job/config) | Medium |

## Decision

**Adopt refined Option A — repo-wide doctor-to-compliance.** This supersedes the
original Option B recommendation after a feasibility read showed the MD025 `_title`
`front_matter_title` configuration dissolves 229 of 250 violations with zero file
edits, making repo-wide compliance tractable.

### As-Built Configuration

The `.markdownlint.json` config:

```json
{
  "default": false,
  "MD001": true,
  "MD025": { "front_matter_title": "^\\s*_title\\s*[:=]" },
  "MD041": true
}
```

**MD025 `_title` rationale.** Backlog and doc artifacts use YAML frontmatter
`title:` PLUS a body `# H1`. The default MD025 `front_matter_title` regex matches
`title:` and counts it as a heading, so the body `# H1` becomes a second top-level
heading — MD025 fires on every such file (all 229 violations). Retargeting
`front_matter_title` to a non-existent `_title` key means the frontmatter `title:`
is no longer counted as a heading, so the single body `# H1` passes. This
eliminates all 229 MD025 violations with zero file edits.

**MD041 guardrail (HARD).** MD041 stays `true` with default options. MD041's default
`front_matter_title` regex matches `title:`, which means a file with frontmatter
`title:` satisfies MD041 ("first line must be H1 or frontmatter title") without
needing a body `# H1`. If MD041 were retargeted to `_title` (or `""`), every
frontmatter file would fail MD041 (~1,262 files). Therefore **only MD025 is
retargeted; MD041 is never retargeted**.

### Runner Configuration

`.markdownlint-cli2.jsonc` contains only `{ "gitignore": true }` (plus comments).
`gitignore: true` makes cli2 skip gitignored scratch (`.copilot/`, `.autoharness/`,
`logs/`) and lint the non-gitignored Markdown corpus. In a clean checkout (CI) that
equals exactly the tracked set (1,839 files, 0 violations verified); locally it also
covers new/untracked non-ignored Markdown, which is intentional (pre-commit checking) —
genuine scratch must be gitignored. This is a runner option for local-equals-CI parity,
not a scoping mechanism.

### Empirical Verification

| Stage | Violations | Detail |
|---|---|---|
| Default rules (MD001/MD025/MD041, no `_title` config) | 250 | MD001=1, MD025=229, MD041=20 |
| With `_title` config, zero file edits | 21 | 20 SKILL.md MD041 + 1 MD001 |
| Config + 21 in-place remediations | **0** | 1,839 tracked files, confirmed via `make md-lint` |

### Concrete Direction (as implemented)

1. **Config** — `.markdownlint.json` (rule config with the MD025 `_title`
   `front_matter_title`) plus `.markdownlint-cli2.jsonc` (`gitignore: true` for
   local-equals-CI parity). cli2 auto-discovers both files.
2. **Remediation** — 20 SKILL.md files missing a leading `# H1` (MD041): 13 under
   `.github/skills/*/SKILL.md` and 7 under `plugin/skills/*/SKILL.md`. 1 MD001
   heading-increment fix at
   `docs/exec-plans/2026-07-06-pre-task-completion-gate-broker-plan.md` line ~83.
   SKILL.md files are generated from external autoharness templates not present in
   this repo — fixed in place; upstream template fix tracked as follow-up.
3. **Invocation** — paired scripts `scripts/md-lint.sh` (bash, `set -euo pipefail`,
   `npx --yes markdownlint-cli2@0.23.1 "**/*.md"`) and `scripts/md-lint.ps1`
   (PowerShell wrapper, same pinned npx). Makefile `md-lint` target calls
   `bash scripts/md-lint.sh`; `make.ps1` `md-lint` case calls `scripts/md-lint.ps1`.
   Node/npx invocation lives in scripts (not inline in Makefile) because an existing
   guard test forbids `npx ` in the Makefile.
4. **CI gate** — standalone repo-wide `md-lint` job in `.github/workflows/ci.yml`.
   No `needs`, no `if`, no path-filter — always runs. `permissions: contents: read`.
   SHA-pinned `actions/checkout` and `actions/setup-node` (node-version `"22"`, matching markdownlint-cli2@0.23.1 `engines.node: ">=22"`),
   then `run: make md-lint`. The `md-lint` job **hard-fails the CI run** from its
   first run (the repo is already 0-violation repo-wide, so the job is green
   day-one yet fails on any future regression). Making it a **required**
   branch-protection status check is a separate external admin action tracked in
   follow-up stash `918BCDAF`.
5. **P-008 reconciliation + guard tests** —
   `tests/integration/markdownlint_gate_test.go` guards the config (exactly
   MD001/MD025/MD041 with the MD025 `_title` `front_matter_title` value) and that
   the CI job is repo-wide and SHA-pinned. P-008 reconciled to name the active rule
   set and document the `_title` configuration.

## Rejected Alternatives

- **Option B (scoped rollout)** — superseded. The `_title`
  configuration insight makes ignore-set machinery unnecessary: repo-wide
  compliance is achievable with 21 in-place fixes. A scoped approach would leave
  P-008 only partially satisfied and require ongoing maintenance of an ignore list.
- **Option C (baseline)** — rejected. No native baseline in markdownlint-cli2;
  bespoke baseline tooling adds a failure surface and a stale baseline hides
  regressions in edited files.

## Unresolved Questions

- Registration of `md-lint` as a **required** branch-protection status check is an
  external admin action tracked as follow-up stash `918BCDAF`.
- The 20 fixed SKILL.md files are generated from external autoharness templates not
  present in this repo. The repo-wide gate catches future regeneration
  drift, but an upstream template fix is tracked as a separate follow-up.

## Risks and Mitigations

- **SKILL.md regeneration drift** — autoharness regeneration may reintroduce missing
  leading `# H1` in SKILL.md files. Mitigation: the repo-wide CI gate
  catches this immediately; upstream template fix tracked as follow-up.
- **New Node/npm dependency in a Go repo** (Single Responsibility, Principle VI) —
  justified: build/CI-time doc-lint tool only (no runtime/`go.mod` impact), invoked
  via `npx` with no committed `node_modules`; actions are SHA-pinned.
- **Config drift from P-008 text** — mitigated by reconciling the policy wording in
  the same shipment so precondition/postcondition match the provisioned reality.
- **MD025 `_title` fragility** — if a future contributor changes the
  `front_matter_title` value or retargets MD041, the gate would either fire on
  ~229 files (MD025) or ~1,262 files (MD041). Mitigation: the guard test in
  `markdownlint_gate_test.go` asserts the exact config values; CI breaks on drift.
- **`npx` network fetch flakiness in CI** — mitigated by pinning
  `markdownlint-cli2@0.23.1` in the `npx` invocation for deterministic runs.
