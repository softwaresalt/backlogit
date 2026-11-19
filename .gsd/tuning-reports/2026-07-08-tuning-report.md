---
doc_type: tuning-report
title: Autoharness Tuning Report — 2026-07-08
docline:
  date: "2026-07-08"
  tags: [autoharness, tune, harness-maintenance, drift]
---

# Autoharness Tuning Report — 2026-07-08

Auto-Tune maintenance run for the `backlogit` workspace harness.

> **Re-tune note (1.4.11):** First produced targeting 1.4.7, then re-run after the
> CLI was upgraded to 1.4.9. **Correction to an earlier draft:** the initial pass
> only diffed `verify-workspace` output (which checks *installed* artifacts) and
> wrongly concluded "no new drift." A proper net-new-artifact scan against the
> 1.4.9 templates found several capabilities that were never installed — most
> visibly the **`/feature-flow` prompt family** and policies **P-015 / P-016 /
> P-017**. The feature-flow capability has now been fully adopted (see
> **New Capability Adopted** below). The manifest is stamped `1.4.11`.
>
> **1.4.11 re-tune:** Upgrading 1.4.9 → 1.4.11 introduced no new drift for this
> workspace: identical 51 targeted checks, no new policies
> (P-017 is still the latest), no net-new prompts/skills/agents, 0 migrations.
> The 3 installed feature-flow prompts still byte-match the 1.4.11 templates.
>
> **Gap-closure pass (all 10 pre-existing gaps resolved):** the 10 remaining
> failing checks — runtime_validation contract, release-closure sequencing,
> source-artifact cleanup, local-review-readiness + P-014 migration, and the two
> new reviewer personas — have now been adopted. **`verify-workspace` targeted
> checks are 51/51 passing, 0 blockers, 0 warnings.** See
> **Gap Closure** below.

## Gap Closure — 10 Pre-existing Gaps Resolved

All 10 remaining failing targeted checks were closed by adopting the
corresponding 1.4.11 capabilities (additive weaving; backups in
`.autoharness/backups/2026-07-08/`):

| # | Check | Change |
|---|---|---|
| 1 | `runtime_validation_profile_contract` | Added schema-valid `runtime_validation` block to `workspace-profile.yaml` (empty surfaces — CLI/MCP tool; `releasability.required: true` because release-observability is enabled) |
| 2 | `ship_runtime_validation_contract` | Ship Step 6 item 2 now invokes runtime-verification with `runtime_validation.validator_manifest`/`validation_expectations` (validator evidence) then operational-closure with `runtime_validation.releasability` (releasability evidence) |
| 3 | `release_observability_instruction` | Added "Releasability Evidence Contract" section (validator + releasability evidence) |
| 4 | `closure_source_artifact_cleanup` | Added Step 5 "Source Artifact Cleanup" to operational-closure (source_stash_id, source_deliberation_id) |
| 5 | `ship_release_closure_sequence` | Added "Release Closure Completion Gate (P-001, NON-NEGOTIABLE)" to ship Step 6 |
| 6 | `orchestrator_release_closure_sequence` | Orchestrator pipelined constraint now blocks a second shipment until post-merge release closure completes |
| 7 | `local_review_readiness_contract` | Added "Local Review Readiness" section to review skill (READY/READY_WITH_FOLLOWUPS/BLOCKED + reviewed HEAD SHA) |
| 8 | `template_integrity_reviewer_routing` | Installed `template-integrity-reviewer.agent.md` + wired review-skill routing |
| 9 | `schema_cli_docs_reviewer_routing` | Installed `schema-cli-docs-coupling-reviewer.agent.md` + wired review-skill routing |
| 10 | `p014_local_review_policy` | Migrated P-014 "Copilot Review Merge Gate" → "Local Review Readiness Merge Gate" (amendment log 1.13.0) |

The 2 new reviewer personas were rendered from templates (Tier-1 model routing:
`claude-haiku-4.5`/`anthropic`/`low`) and registered in the manifest
(59 artifacts total). Manifest checksums refreshed for all modified tracked
artifacts. The 4 `model_routing` `strict_schema_blockers` remain the documented
verifier false-positive (config validates 0 errors against the on-disk schema).


## New Capability Adopted — Feature Flow (P-015/016/017 + dark factory)

autoharness 1.4.9 ships a developer-facing `/feature-flow` prompt family plus the
policy and agent surfaces that back it. None were installed (the harness was last
merge-installed at 1.4.4). This tune adopted the full, coherent capability:

**Prompts installed** (`.github/prompts/`, static templates, verbatim):

* `feature-flow.prompt.md` — sequential single-PR-at-a-time default (`run pipeline`)
* `feature-flow-parallel.prompt.md` — P-016-compliant planning overlap
* `feature-flow-dark.prompt.md` — bounded P-017 dark factory execution

**Policies added** to `.github/policies/workflow-policies.md` (rendered with repo
values; amendment log updated 1.10.0–1.12.0):

* **P-015** Single-Artifact Shipment Closure (No Cascade Ship)
* **P-016** No Parallel Branch/Worktree Execution
* **P-017** Dark Factory Autonomy Contract

**Coordinated dark-factory weaving** (additive sections, backups in
`.autoharness/backups/2026-07-08/`) so `/feature-flow-dark` is fully coherent —
all 8 `dark_factory_*` verifier checks now pass:

| Surface | Added |
|---|---|
| `_orchestrator.agent.md` | Dark Factory Mode section, P-016 pipelined constraint, feature-flow trigger phrases, `merge_approval_pre_authorized` / reviewed-HEADs contract |
| `.ship.agent.md` | Step 5.5 Dark Factory Execution (LOCAL_REVIEW_READY, DARK_MODE_MERGE_AUTHORIZED, ADMIN_FALLBACK_ATTEMPTED, headRefOid, P-016) |
| `pr-lifecycle/SKILL.md` | Step 5d Merge Execution & Admin Fallback State Machine |
| `github-pr-automation.instructions.md` | §1.9.6 Dark-Mode Merge Authorization and Admin Fallback |
| `agent-intercom.instructions.md` | Dark Factory Visibility Protocol (8 dark-mode events) |
| `AGENTS.md` | Development Workflow: single-active-branch (P-016) + dark factory mode (P-017) |

**Remaining net-new 1.4.9 artifacts (NOT adopted — operator decision):**

| Artifact | Classification | Recommendation |
|---|---|---|
| `doc-review` skill | Growth (applicable) | Adopt if doc-review workflow is wanted |
| `template-integrity-reviewer`, `schema-cli-docs-coupling-reviewer` agents | Growth (applicable) | Adopt with review-skill routing (ties to 2 failing checks) |
| `correctness` / `maintainability` / `technology` reviewer agents | Growth | Repo uses Go-specific reviewers (go-quality, mcp-protocol, sqlite); adopt only if generic personas wanted |
| `coding-discipline`, `output-timestamps`, `graphtor-docs`, `mcp-server` instructions | Growth / engram-related | Evaluate individually; `graphtor-docs` ties to enabled agent-engram pack |
| `browser-automation` skill, `browser-verification` instruction | Pack-gated | N/A — no web UI / browser tooling; pack not enabled |
| `evolve` / `learn` / `observe` skills | Pack-gated | N/A — `continuous-learning` pack not enabled |
| `technology-python/rust/typescript` instructions | Stack N/A | N/A — Go workspace |
| `language-engineer` → `go-engineer`, `technology-go` → `technology.instructions.md` | Rename | Already installed under repo-specific names |

## Summary

| Field | Value |
|---|---|
| Workspace | `C:\Source\GitHub\backlogit` |
| autoharness home | `C:\Python\Python312\Lib\site-packages\autoharness\data` |
| Installed harness version (manifest, pre-tune) | `1.4.4` |
| Current autoharness version | `1.4.11` |
| Manifest contract | `harness-manifest` v1.0.0 — current |
| Config contract | `harness-config` v1.0.0 — current |
| Profile contract | `workspace-profile` v1.0.0 — current |
| Deterministic verify result | exit 1 (drift found) |
| Scope | all |
| auto_apply | false (interactive; only safe non-destructive changes applied) |

The harness is a deliberate in-repo fork: 52 of 54 manifest-tracked artifacts
are `user-modified`. No breaking stack drift was found — the Go 1.24 toolchain,
build/test/lint commands, GitHub Actions CI, and backlogit backlog wiring all
still match the profile. The actionable drift is a stale manifest, a set of
capability gaps where autoharness 1.4.9 expects concepts the installed harness
lacks, and a strict-validator false-positive on `model_routing`.

## Version and Environment Drift

* Manifest recorded `autoharness_version: 1.4.4`; current CLI is `1.4.11`.
* Manifest recorded `autoharness_home: D:\Source\GitHub\autoharness` (a source
  checkout); the resolved home is now the pip install at
  `C:\Python\Python312\Lib\site-packages\autoharness\data`.
* `tuned_at` was `2026-05-17`; last on-disk tuning report predates the manifest.

These were refreshed during this run (see Applied Changes).

## Deterministic Verification (`verify-workspace --json`)

| Section | Result |
|---|---|
| `strict_schema_blockers` | 4 (model_routing — see below) |
| `warnings` | 0 |
| `blockers` | 0 |
| `rendered` | 54 |
| `unresolved` (staging render) | 108 placeholders |
| `checksum_scan` | 52 user-modified, 2 unchanged |
| `schema_contracts` | 3 current, 0 drifted |
| `migration_proposals` | 0 |
| `targeted_checks` | 51 pass, 0 fail (after gap closure) |
| `learning_signals` | empty (no mined patterns) |
| `portability_findings` | 0 |

## Schema-Contract Status

All three versioned contracts (`manifest`, `config`, `profile`) report
`status: current` at v1.0.0. No contract migration is required
(`migration_proposals` is empty).

### model_routing strict-validator false-positive (P3 — tooling)

`verify-workspace` emitted 4 `strict_schema_blockers` claiming
`model_routing.{tier1,tier2,tier3,orchestrator}` "is not of type 'string'".
Independent validation of `.autoharness/config.yaml` against the current
on-disk `schemas/harness-config.schema.json` (Draft-07) returns **0 errors**:
that schema defines each tier as `oneOf: [string, object]`, and the object form
(`model` + `reasoning_effort` + `model_provider` + `model_family`) is the
documented modern shape.

Conclusion: the CLI's strict pre-validator lags the published `oneOf` schema.
The richer object-form config is correct and should be kept. **No config change
is recommended** — rewriting the config to bare strings would discard the
per-tier routing metadata (provider/family/effort) the workspace intentionally
declares. Track upstream for a verifier fix.

## Checksum Drift Scan

52 of 54 manifest-tracked artifacts are `user-modified`; 2 are `unchanged`
(`.github/copilot-instructions.md`, `.github/instructions/technology.instructions.md`).
This is expected — the harness is hand-maintained in this repo.

**Action taken:** created `.autoharness/drift-ignore` recording the 48
functionally-current user-modified artifacts as intentional customizations, so
future tune runs classify them as `ignored` rather than noise. The 7
capability-gap files below were deliberately left out of the ignore set so they
remain visible for adoption decisions.

## Proposed Changes (ordered by priority)

### P0 — Breaking

None. No harness artifact references a non-existent file, tool, or command.
Lock scripts and search scripts exist; `.gitignore` contains `.*.lock`.

### P1 — Degrading (capability gaps — RESOLVED, see Gap Closure above)

> **Status: all 10 resolved.** The gaps below were the original 1.4.7-era
> findings. They have since been adopted — see the **Gap Closure** table near the
> top of this report. Retained here for historical trace.

These installed artifacts are heavily user-modified, so they were **not**
auto-overwritten. Each proposal is to weave the missing 1.4.9 concept into the
existing local file by hand (or re-render from template and re-apply local
edits), preserving repo-specific content.

| # | Artifact | Missing concept |
|---|---|---|
| 1 | `.autoharness/workspace-profile.yaml` | `runtime_validation` block (validator_manifest, validation_expectations, releasability) |
| 2 | `.github/agents/.ship.agent.md` | `runtime_validation.validator_manifest` / `validation_expectations`, validator + releasability evidence, "Release Closure Completion Gate (P-001, NON-NEGOTIABLE)", post-merge closure sequencing |
| 3 | `.github/agents/_orchestrator.agent.md` | "awaiting required post-merge release closure"; must not route a second shipment to Ship until closure completes |
| 4 | `.github/skills/review/SKILL.md` | `READY_WITH_FOLLOWUPS` / `BLOCKED` / "reviewed HEAD SHA" readiness contract; routing for Template Integrity Reviewer and Schema-CLI-Docs Coupling Reviewer |
| 5 | `.github/policies/workflow-policies.md` | P-014 "Local Review Readiness Merge Gate" (READY_WITH_FOLLOWUPS, reviewed HEAD SHA) |
| 6 | `.github/skills/operational-closure/SKILL.md` | "Source artifact cleanup" (source_stash_id, source_deliberation_id) |
| 7 | `.github/instructions/release-observability.instructions.md` | "validator evidence" and "releasability evidence" |

Related growth (P2): if proposals 2/4 are adopted, the two new reviewer personas
`template-integrity-reviewer.agent.md` and
`schema-cli-docs-coupling-reviewer.agent.md` from 1.4.9 templates should be
installed under `.github/agents/review/` and cross-referenced in the review skill
and plan-review routing.

### P2 — Growth

* Install the two new review personas noted above (Template Integrity,
  Schema-CLI-Docs Coupling) if the review-skill routing gaps (proposal 4) are
  adopted.

### P3 — Cosmetic / Tooling

* model_routing strict-validator false-positive (documented above) — no action.
* `harness-manifest.yaml` `profile_hash` / `config_hash` carry the literal
  placeholder `"updated"` from a prior tune; harmless, left as-is.

## Learning-Driven Findings

`learning_signals` was empty: the verifier mined no compound patterns,
promotion-ready instincts, workflow-phase hotspots, or recurring closure issues
for this run. No learning-driven proposals or dynamic policy candidates were
generated. (The repo's `docs/compound/` library exists but was not surfaced by
the verifier's signal miner in this pass.)

## Applied Changes

**Metadata & drift bookkeeping (non-destructive):**

1. **Manifest metadata refresh** — `.autoharness/harness-manifest.yaml`:
   `autoharness_version` 1.4.4 → 1.4.11; `autoharness_home` → pip data path;
   `tuned_at` → 2026-07-08T13:49:03Z. Backups at
   `.autoharness/backups/2026-07-08/` (`harness-manifest.yaml` = original 1.4.4,
   `harness-manifest.pre-149.yaml` = interim 1.4.7 state).
2. **drift-ignore created** — `.autoharness/drift-ignore` recording intentional
   local customizations. Files re-baselined during feature-flow adoption
   (AGENTS.md, agent-intercom, github-pr-automation) were removed from the ignore
   set so future template updates surface.

**Feature-flow capability adoption (additive; see "New Capability Adopted"):**

3. **3 prompts installed** + registered in the manifest `artifacts:` list with
   sha256 checksums.
4. **P-015 / P-016 / P-017** appended to `workflow-policies.md`; manifest checksum
   updated.
5. **Dark-factory sections woven** into 6 surfaces (orchestrator, ship,
   pr-lifecycle, github-pr-automation, agent-intercom, AGENTS.md); manifest
   checksums refreshed for tracked artifacts. All 8 `dark_factory_*` verifier
   checks pass.

Every modified generated artifact was backed up first. Pre-existing
heavily-customized content was preserved — dark-factory content was added as new
sections, not by re-rendering whole files. The 10 pre-existing capability gaps
were left for review at the close of this feature-flow pass and were
**subsequently adopted** in the gap-closure pass — see
**Gap Closure — 10 Pre-existing Gaps Resolved** near the top of this report.

## Branch Safety (NON-NEGOTIABLE)

The workspace is on the default branch `main`. Autoharness tune output MUST NOT
be committed or pushed to the default branch.

**Recommended workflow:**

1. Create a feature branch, e.g. `chore/autoharness-tune-2026-07-08`.
2. Review this report and the applied manifest/drift-ignore changes on that
   branch.
3. The P1 capability-gap adoptions were completed in this session (see Gap
   Closure) — no further decision needed.
4. Open a pull request.

The repo already has unrelated uncommitted changes on `main` (agent-file renames
and `start.ps1`); those were left untouched.

## Next Tuning

Recommended after the next major release or ~monthly. All P1 capability gaps
were adopted this session; `verify-workspace` targeted checks are at 51/51.
