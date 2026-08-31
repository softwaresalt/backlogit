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

---

## Phase 2 — Stale-Artifact Remediation

### Why a second pass was required

Phase 1 diffed the template inventory against the workspace by **file
existence**. That check cannot see an artifact that is present but **stale** —
a file carrying the pre-1.5.0 body while the template has since gained new
guardrail content. Every such artifact was silently reported as "already
installed".

The gap surfaced when the operator asked whether dark-mode installation was
included. All four `feature-flow*` prompt shims existed, so Phase 1 had skipped
them; a byte comparison showed `feature-flow-dark.prompt.md` was 1509 B against
the template's 1966 B and was missing the entire **P-021 non-bypass clause**.

**Methodology correction**: for merge-install, existence is not a sufficient
test. Phase 2 re-ran the comparison on **content**.

### Method

* All 42 variable-free templates were compared byte-for-byte against their
  installed counterparts. 8 diverged.
* For each divergence, `Compare-Object` was used to inspect the
  **installed-only** side. Zero installed-only lines means the installed file is
  a strict subset of the template and can be replaced verbatim; installed-only
  lines that are genuine workspace prose must be preserved.
* Variable-bearing templates (agents, which the variable-free scan cannot
  cover) were compared on **guardrail-marker coverage** — counting P-017
  dark-mode and P-021 markers in template versus installed — and every shortfall
  was spliced in by hand so workspace customizations survived.

### Variable-free artifacts

| Artifact | Verdict |
|---|---|
| `.github/prompts/feature-flow-dark.prompt.md` | **Replaced** — gained the P-021 non-bypass clause |
| `.github/instructions/role-enforcement.instructions.md` | **Replaced** — gained the P-021 capture-only carve-out and the P-013.5 Skill-Delegation Model Inheritance section |
| `.github/instructions/concurrency.instructions.md` | **Replaced** — gained the P-016 Branch and Worktree Boundary section |
| `.github/instructions/capability-pack-enforcement.instructions.md` | Left as-is — template-only lines are all `graphtor-docs`, a pack this workspace deliberately excludes |
| `.github/instructions/release-observability.instructions.md` | Left as-is — trailing-whitespace only, zero line differences |
| `.github/instructions/backlogit.instructions.md` | Left as-is — installed is substantially larger and heavily customized |
| `.github/skills/file-lock/SKILL.md` | Left as-is — `#` vs template `##` heading, the established P-008/MD041 convention here |
| `.github/skills/skill-search/SKILL.md` | Left as-is — same heading convention |

### Pipeline agents (variable-bearing, spliced not replaced)

Marker coverage before remediation:

| Agent | dark-mode (template/installed) | P-021 (template/installed) |
|---|---|---|
| `_stage.agent.md` | 0 / 0 | 7 / **0** |
| `_ship.agent.md` | 8 / 8 | 9 / **8** |
| `_orchestrator.agent.md` | 20 / **12** | 1 / 2 |

Content added:

* **`_stage.agent.md`** — P-021 was entirely absent. Added the Step 1
  deferred-scope-expansion precedence classification (which forces the
  `deliberate` route ahead of shape classification), the extended traceability
  duty, the full **Deferred-Expansion Triage Obligations (P-021 C5/C6)**
  subsection, the Step 1.5 grouping exclusion, and the Step 2
  "Ready for planning" unavailability rule.
* **`_ship.agent.md`** — added the P-021 C5 capture-only carve-out to the Role
  Boundary Backlog row, the P-021 C4 review-fix-cycle-limit annotation, the
  P-020 compaction-status initialization, and the dark-mode closure-summary
  requirement.
* **`_orchestrator.agent.md`** — added the P-021 non-bypass paragraph, the
  multi-shipment **ordered `DARK_MODE_SCOPE` cursor** requirement, P-021 to the
  preserved-safety list, the `dark-factory` mode plus `DARK_MODE_ACTIVE` line in
  the session-state block, and the five Step 2 candidate-selection bullets
  (queue ordering, dark-run scope constraint, dependency re-check, precedence,
  scope-reconstruction caveat) that replaced a bare
  "Select the highest-priority queued shipment".

### A functional break this pass fixed

Ship's Role Boundary forbade **all** stash operations, while P-021 C2/C5
*requires* Ship to create capture-only stash entries for deferred scope
expansions. The replaced `role-enforcement.instructions.md` applies fail-closed
semantics — an unlisted state mutation is treated as forbidden — so Ship's own
boundary table would have blocked the C2 capture the policy mandates. The
carve-out now makes the allowance explicit.

### Deviations from template text

* The orchestrator's ordered-scope paragraph cites the **Shipment Sequencing
  Protocol** in `.github/instructions/backlogit.instructions.md` rather than the
  template's `docs/compound/2026-05-07-backlogit-shipment-status-constraints.md`,
  which does not exist in this workspace.
* The Stage triage subsection cites `.github/policies/workflow-policies.md`
  rather than the template's `templates/policies/workflow-policies.md.tmpl`
  source path.

### Verification

* `markdownlint-cli2@0.23.1` over all 6 changed files — 0 issues.
* Manifest checksums refreshed (LF-normalized SHA-256) for all 6.
* `autoharness verify-workspace` — 0 strict-schema blockers, 0 blockers, 0
  unresolved placeholders, 80 rendered, 2 known false-positive warnings, and the
  1 known-defective `pipeline_topology_gate_ship_agent_wiring` FAIL. Identical
  to the Phase 1 baseline: no regression.
* All `P-021 C1`–`C6` clause labels referenced by the new agent text were
  confirmed present in `.github/policies/workflow-policies.md`.

Pre-edit copies of all 6 files are in
`.autoharness/backups/2026-08-31/`.

---

## Phase 3 — Full Content Sweep

Phase 2 fixed the artifacts the operator's dark-mode question pointed at, but it
still only examined variable-free templates plus the three pipeline agents. The
methodology was then applied to **every** installed artifact.

### Method

`autoharness verify-workspace` stages a fully-rendered copy of each template
under `.autoharness/staging/` using this workspace's own profile. That render is
the exact "what 1.5.0 would install here" baseline, so installed artifacts were
diffed against it directly — this covers variable-bearing templates that a raw
template comparison cannot.

Two sweeps were run over the manifest's 80 artifacts:

1. **Policy-reference coverage** — every `P-0NN` identifier present in the
   rendered template but absent from the installed file.
2. **Section coverage** — every heading present in the render but absent from
   the installed file.

### Policy-reference gaps (6 found, all closed)

| Artifact | Missing | Action |
|---|---|---|
| `.github/instructions/circuit-breaker.instructions.md` | P-021 | Replaced — installed was 149 lines against 412; gained *Same-Operation Identity and Hidden Details*, *Counted Diagnostic Transport*, *Cooldown Delay (No Auto-Reset)*, and *Frontmatter YAML-Safety Regression Cases*. All installed-only lines were older wordings, not customizations. |
| `.github/instructions/github-pr-automation.instructions.md` | P-018, P-021 | Replaced — 570 lines against 802; gained *Shell-Safe Comment Body Construction*, *Local Readiness Record*, *Local Review Readiness*, and *Advisory Bot Identity*. Upstream generalized "Copilot Review" to "Shadow Review", but all 62 Copilot references and the concrete `copilot-pull-request-reviewer` bot identity survive the rename, so GitHub-specific mechanics are unchanged. |
| `.github/instructions/context-efficiency.instructions.md` | P-020 | Replaced — strict subset (zero installed-only lines). |
| `.github/skills/plan-harden/SKILL.md` | P-012 | Replaced — gained review-gate capability risks and `dispatch_mode:` / `decision:` marker carry-forward. `#` h1 preserved. |
| `AGENTS.md` | P-020 | Spliced — post-merge compaction added to the dark-factory contract and to *Closure before forgetfulness*. |
| `.github/skills/operational-closure/SKILL.md` | P-001, P-020 | Spliced — added the compaction-status field and releasability-evidence outputs. The workspace-only *Source Artifact Cleanup (backlogit)* section was preserved. |

### Section gaps

Most heading differences were benign — renamed headings, or the `graphtor-docs`
sections for a pack this workspace excludes. Two were real:

* **`.github/policies/workflow-policies.md`** was missing **P-013.5
  (Invocation-Time Model-Routing Enforcement)** and **P-013.6 (Telemetry-driven
  Auto-escalation Protocol)** — it stopped at P-013.4. This mattered because the
  installed `escalation-protocol.instructions.md` cites both policies
  extensively, so every one of those references was dangling. Both were inserted
  ahead of P-014.
* **`.github/skills/review/SKILL.md`** carried a condensed dark-mode readiness
  paragraph missing two bullets: the `READY_WITH_FOLLOWUPS` follow-up-ID
  requirement, and the rule that advisory shadow-review comments are follow-ups
  by default unless explicitly elevated. Expanded to the full upstream list.

### Incidental consistency fix

`operational-closure/SKILL.md` called `backlogit_stash_remove`, which the
backlogit instructions deprecate and the ship agent already avoids in the
equivalent step. Aligned to `backlogit_stash_archive`.

### Verification

* Post-sweep re-scan: **0 artifacts** missing policy references.
* `markdownlint-cli2@0.23.1` over all 8 changed files — 0 issues.
* Manifest checksums refreshed for all 8.
* `autoharness verify-workspace` — 0 strict-schema blockers, 0 blockers, 0
  unresolved placeholders, 80 rendered, 2 known false-positive warnings, 1
  known-defective FAIL. Process exit status improved from 1 to 0.
* `go test ./tests/...` for `TestPluginBundleStructurallyValid` and
  `TestGitHubSpikeSkillFrontmatterMatchesPluginCopy` — pass.

---

## Phase 4 — Operator-Directed Follow-Ups

### P-016 worktree topology (operator approved removal)

The topology gate reported `MULTIPLE_IMPLEMENTATION_WORKTREES` with **17**
implementation worktrees against a limit of one — more than the 9 first
estimated, because `.copilot/worktrees/` held 8 in addition to the 9 under
`.copilot/session-state/`.

Audit before removal:

* **14 branches** were fully merged into `origin/main`.
* **2 branches** carried unmerged commits — `chore/134-s-closure` (`daf1dd29`)
  and `chore/cycle-24-remediation` (`cd2ad50b`). Both survive removal untouched
  because `git worktree remove` deletes only the working directory; branch refs
  are unaffected. No branch was deleted.
* **6 worktrees** held uncommitted work, and spot-checking showed those backlog
  mutations were *not* mirrored in the main worktree — so they could not be
  discarded as redundant.

Everything uncommitted was therefore captured first to
`.autoharness/backups/worktree-preservation-2026-08-31/` (gitignored): a
`STATUS.txt`, a `git diff HEAD` patch, and verbatim copies of every untracked
file per worktree, plus a `README.md` documenting the restore procedure. 28
files preserved.

All 16 extra worktrees were then removed, `git worktree prune` run, and the
empty `.copilot/worktrees/` directory deleted. `git fsck` is clean and the
topology gate now reports **pass**.

The ~130 remaining directories under `.copilot/session-state/` are ordinary
Copilot session artifacts, not worktrees. They were left untouched.

### Alternate documentation-review route

`model_routing.alt_doc_review` was unset, which was not a neutral no-op — it
rendered four broken fragments into the installed `doc-review` skill, including
the instruction step `1. Read `` and ``.` and the dangling
`When `` / `` are set:`.

Bound to **google / gemini-3.1-pro-preview**. Google is deliberately a *third*
provider: the tiers are Anthropic and escalation is OpenAI, so routing the
documentation review pass to Google maximizes cross-model diversity — the review
is least likely to share blind spots with whichever model authored the docs.

Wiring required two edits, because the verifier seeds its template variables
from the manifest rather than re-reading the config:

1. `.autoharness/config.yaml` — added the `model_routing.alt_doc_review` block
   (validated against `harness-config.schema.json`; the schema permits only
   `model_provider` and `model_family`, with `additionalProperties: false`).
2. `.autoharness/harness-manifest.yaml` — set `ALT_DOC_REVIEW_PROVIDER` and
   `ALT_DOC_REVIEW_FAMILY` under `variables_used`, which
   `_derive_template_variables` reads. Without this the staged render stays
   empty regardless of config, and the artifact would show perpetual false
   drift on every future tune.

The skill was then reinstalled from the corrected staging render. One workspace
customization was re-applied by hand: the staged template regresses the
follow-up command to `backlogit add --type {{artifact_type}} --title {{title}}`,
whereas the workspace resolves it to the concrete
`backlogit add --type task --title <title>`. Keeping the concrete form is also
consistent with the doc-review skill's own P0 rule against unresolved
placeholders in installed output.

Verification: `doc-review/SKILL.md` reports `unchanged` in the checksum scan,
markdownlint is clean, and `verify-workspace` holds at 0 blockers / 0 unresolved
placeholders / 80 rendered with only the 2 known-false-positive warnings.

## Phase 5 — P-018 enforcement restored (guardrail regression from Phase 3)

Phase 3 replaced `github-pr-automation.instructions.md` with the upstream 1.5.0
render. That replacement was correct as a merge, but it carried a **behavioural**
change that the content diff did not flag as risky:

| | Before (installed) | After (upstream 1.5.0) |
|---|---|---|
| §1.1 request review | mandatory | "optionally request ... during the migration period" |
| §1.8 cycle limits | "Cycle limits do not clear the merge gate" — unresolved threads **always** merge-blocking | "do not make shadow review merge-blocking **by default**" — blocking only when P-018 is engaged |

Under `copilot_review.enforcement: auto`, P-018 treats a PR with no Copilot
engagement signal as `NOT_APPLICABLE` (PASS). Combined with review requests
becoming optional, a PR could reach merge with unresolved review threads.

Verified directly against the shipped gate rather than inferred from prose:

```text
enforcement=auto      -> NOT_APPLICABLE      PASS - merge allowed
enforcement=required  -> WAITING_FOR_REVIEW  BLOCK - merge held
```

(synthetic `ReviewState`: Copilot never requested, two unresolved threads)

**Resolution.** `.autoharness/workspace-profile.yaml`:

* `copilot_review.enforcement`: `auto` -> `required` — forces `engaged = True`
  regardless of per-PR signal, restoring the pre-merge strictness.
* `copilot_review.max_wait_seconds`: `0` -> `900` — with `0` the gate returns
  `WAITING_FOR_REVIEW` the instant a review has not yet landed; `900` matches the
  15-minute review budget in §1.2 so the gate polls rather than halting.

**Wedge risk assessed, not assumed.** `auto` exists to avoid waiting for a
reviewer that never comes. Sampling PRs #394-#399 showed Copilot review present
on every one (1, 7, 3, 1, 4, 4 reviews), so `required` cannot wedge this repo.
