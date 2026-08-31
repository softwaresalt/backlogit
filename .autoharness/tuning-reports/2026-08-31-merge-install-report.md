# Autoharness 1.5.0 Merge-Install Report

Date: 2026-08-31
Agent: auto-mergeinstall
Branch: `chore/autoharness-merge-install-2026-08-31`
Commit: `e806779b`
Result: **PASS** (baseline preserved; 16 artifacts added)

## Scope

Merge-install of autoharness 1.5.0 into the `backlogit` workspace. Purely
additive: no existing harness artifact was overwritten, and the installed
capability-pack selection was left unchanged.

## Source Resolution

Two candidate homes were found and compared:

| Home | Path | Version |
|---|---|---|
| pip (**adopted**) | `C:\Python\Python314\Lib\site-packages\autoharness\data` | 1.5.0 |
| plugin | `.copilot/installed-plugins/autoharness/autoharness` | 1.5.0 |

A CRLF-normalized hash diff across `templates/`, `.github/`, and `schemas/`
showed **0 content differences** (140 template files each). The four
plugin-only files are repo infrastructure (`workflows/ci.yml`,
`workflows/release.yml`, `policies/workflow-policies.md`,
`plugin/marketplace.json`).

Upstream `softwaresalt/autoharness` HEAD was confirmed as `2661c1c8`
("v1.5.0 release preparation and publish"). PyPI's newest published release
is only 1.4.11, so the local git-sourced 1.5.0 **is** the latest available.
`autoharness_home` in the manifest was repointed to the pip path.

## Methodology Note (important for future passes)

`autoharness verify-workspace` reported `New artifacts (uninstalled
templates): 0` while **16 templates were in fact missing**, because the
deterministic verifier enumerates only a 64-artifact subset.

**An independent full template-inventory diff against the installed tree is
mandatory for a merge-install. Do not trust the verifier's new-artifact
count.**

## Artifacts Installed

| Area | Artifacts |
|---|---|
| Instructions | `coding-discipline`, `output-timestamps` |
| Review personas | `correctness-reviewer`, `maintainability-reviewer` |
| Skills | `brainstorm`, `doc-review` |
| Local gates (opt-in) | `pre-commit-markdownlint`, `pre-commit-pipeline-topology`, `pre-push-quality-gates` (`.ps1` + `.sh` each) |
| Tooling | `deploy-harness` (`.ps1` + `.sh`), `start.sh` |
| CI Gate C | `scripts/ci-topology-check.sh` + additive `topology-check` job |

Review personas were placed in `.github/agents/review/` to match this
workspace's established layout rather than the 1.5.0 spec's
`.github/agents/subagents/`. The deterministic verifier passes against the
existing layout, so relocating the other personas would be gratuitous churn.

## CI Edit Rationale

`.github/workflows/ci.yml` is a hand-written Go CI (jobs `changes`, `test`,
`docs-lint`, `cli-reference-drift`, `md-lint`) and is **not** the
`ci.yml.tmpl` output. Overwriting it with the template would have been
destructive. The `topology-check` job was therefore appended:

* no `needs:` and **not** gated on the `changes` path filter — docs- and
  backlog-only commits are exactly the ones that mutate `.backlogit/`
  topology state
* reuses the workflow's own `actions/checkout` v4.2.2 pin for intra-file
  consistency
* left **advisory** via
  `continue-on-error: ${{ vars.PIPELINE_TOPOLOGY_GATE_REQUIRED != 'true' }}`

The gate becomes blocking only when an operator sets that repository
variable to `true`.

## Verification

| Gate | Result |
|---|---|
| `autoharness verify-workspace` | 0 strict-schema blockers, 0 blockers, 0 unresolved placeholders, 80 rendered (was 64) |
| New-artifact checksums | 16 / 16 `unchanged` |
| `markdownlint-cli2@0.23.1` | 0 issues across the 6 new markdown artifacts |
| `bash -n` | all 6 new `.sh` files pass |
| PowerShell AST parse | all 4 new `.ps1` files pass |
| `TestPluginBundleStructurallyValid` | pass |
| `TestGitHubSpikeSkillFrontmatterMatchesPluginCopy` | pass |
| `ci.yml` YAML parse | pass (6 jobs) |

## Deliberate Upstream Divergences

Both are recorded with `drift_allowed: true` in the manifest so a future
auto-tune does not silently revert them.

1. **`start.sh`** — the template's final `exec` uses
   `"${copilot_arguments[@]:-}"`, which under `set -u` expands an **empty**
   array to one empty-string word and passes a spurious empty argument to
   `copilot` (empirically confirmed). Rendered instead with the `set -u`-safe
   idiom `${copilot_arguments[@]+"${copilot_arguments[@]}"}`.
2. **`brainstorm/SKILL.md`** — the template titles the document
   `## Brainstorm`, which fails this repo's P-008 markdownlint gate (MD041
   first-line-h1) and is inconsistent with all 24 other installed skills,
   every one of which uses a single `#` title. Promoted to `# Brainstorm`.
   No other heading changed; the `# <Topic>` in the artifact-template section
   is inside a fenced block, so MD025 does not fire.

Both should be reported upstream and dropped once the templates are fixed.

## Known Pre-Existing Noise (unchanged by this pass)

* **`pipeline_topology_gate_ship_agent_wiring` FAIL** — an upstream verifier
  defect. `_ship.agent.md` carries all six `TOPOLOGY_GATE` markers in the
  correct order, but `backlogit_claim_shipment` also appears in the YAML
  frontmatter `tools:` list (because `{{BACKLOG_TOOLS}}` expands to an
  explicit list, whereas the template retains the unexpanded variable),
  which spuriously trips the `must_precede` ordering constraint. Collapsing
  the tools list to a wildcard was rejected: it risks silently removing
  runtime tool access.
* **2 P1 portability warnings** — both flag `~/.autoharness` in
  `_orchestrator.agent.md`, text present verbatim in the upstream template
  as the documented home-resolution fallback order.
* **`docs lint` failure** — 3 findings, all in
  `docs/closure/2026-08-24-130-s-adversarial-review.md`, an **untracked**
  file from a prior session. Not created or committed by this pass; a clean
  checkout of `e806779b` passes.
* **`make md-lint` cannot run from a Windows shell** — `scripts/md-lint.sh`
  is stored LF but checked out CRLF under `core.autocrlf=true`. This is a
  pre-existing local-environment condition affecting the repo's own script;
  CI on Linux is unaffected. Markdownlint was invoked directly instead.

## Operator Follow-Ups

1. **Real P-016 violation surfaced by the new gate.** A local
   `autoharness gate pipeline-topology --mode manual --phase ambient` run
   reports `MULTIPLE_IMPLEMENTATION_WORKTREES`: 9 stale worktrees under
   `.copilot/session-state/`. Worktree pruning is destructive and was
   deliberately **not** performed — it requires explicit operator approval.
2. **Hooks are inert.** Per P-019 nothing was written to `.git/hooks` and
   `core.hooksPath` was not set. If both pre-commit scripts are adopted they
   must be chained from a single dispatcher.
3. **Capability packs unchanged** — `backlogit`, `strict-safety`,
   `agent-engram`, `adversarial-review`, `release-observability` remain
   enabled. `agent-intercom` stays deliberately excluded per the 2026-08-30
   tune; `browser-verification`, `continuous-learning`, and `graphtor-docs`
   remain disabled. Enabling any of these is an operator decision and was
   not made unilaterally.
4. **`model_routing.alt_doc_review` is unconfigured**, so the new
   `doc-review` skill renders its alternate-model references empty. This
   matches the existing `adversarial-review.instructions.md` convention.
   Configure the block to bind a concrete alternate model.

## Commit Scope

The commit contains **39 harness files only**
(`.autoharness/`, `.github/`, `scripts/`, `start.sh`). It also lands the
previously-uncommitted 2026-08-30 tuning pass, whose edits are interleaved in
the same manifest and profile files and cannot be split without leaving the
branch referencing artifacts absent from the tree.

Unrelated in-flight work was left untouched: `.backlogit/` state,
`.gitignore`, `docs/` session artifacts, `new_findings_detail.json`, and
`test_rename.go`.

Nothing was pushed, and no change was made to the default branch.
