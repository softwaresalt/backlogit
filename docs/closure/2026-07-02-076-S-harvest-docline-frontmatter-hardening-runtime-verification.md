---
chunk_strategy: h1-h2-h3
description: 'Post-merge runtime verification for shipment 076-S — harden the Stage harvest pipeline so exec-plan docs are born-compliant with the docline frontmatter contract and a pre-harvest lint gate blocks invalid frontmatter before it can ride into a Ship feature PR (feat/076-docline-frontmatter-hardening, PR #166, merge ef9dc20). Docs/harness-only change (no Go code): impl-plan/SKILL.md gains a Plan Frontmatter Contract section plus a MANDATORY Phase 4 self-lint, and harvest/SKILL.md gains a Phase 1.5 docline gate that HALTs decomposition on any violation. Verdict PASS, validated behaviorally against the unchanged backlogit docs lint CLI (the shared CI entrypoint). Positive: the shipped plan lints valid with 0 violations both scoped (--path) and repo-wide; the whole corpus stays green. Negative: a temp plan replicating the 075-S PR #164 defect (doc_type: exec-plan, missing top-level title and source) is flagged with exactly 3 violations (title required, source required, doc_type unknown_doc_type) and exit status 1, proving the harvest HALT gate catches non-compliant frontmatter. Source-entrypoint parity confirmed: the self-lint invokes go run ./cmd/backlogit docs lint (== make docs-lint), so a local pass cannot diverge from the CI Docline frontmatter gate.'
doc_type: closure
docline:
    ms.date: 2026-07-02T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-02T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-02-076-S-harvest-docline-frontmatter-hardening-runtime-verification.md
title: 076-S harvest docline frontmatter hardening — Post-Merge Runtime Verification
---

# Runtime Verification — 076-S harden Stage harvest docline frontmatter integrity

- **Date**: 2026-07-02
- **Shipment**: `076-S` · Feature `076-F` · Tasks `076.001-T`, `076.002-T`
- **PR / merge**: #166 · merge commit `ef9dc20468d865bbaf7d7b1e9b982ff7f4045422`
- **Branch**: `feat/076-docline-frontmatter-hardening` (verified from `post-merge/076-docline-frontmatter-hardening`)
- **Surface**: Stage authoring + harvest workflow (agent harness) · **Mode**: behavioral (CLI-backed)
- **Verdict**: **PASS**

## Affected runtime surfaces

This shipment changes **no Go code** (543 insertions, all docs/harness/backlog). The
"runtime surface" it hardens is the **Stage plan-authoring + harvest workflow**, whose
enforcement is delegated to the *unchanged* `backlogit docs lint` CLI (the shared CI
entrypoint). Two shift-left gates were added upstream of every path by which a plan
reaches a commit:

- `.github/skills/impl-plan/SKILL.md` (076.001-T) — a **Plan Frontmatter Contract**
  section (gate-required `doc_type: plan` + top-level `title`/`source`, green-reference
  parity fields, the unquoted-`#` YAML pitfall, optional `backlogit docs migrate`
  derivation) and a **MANDATORY Phase 4 self-lint** run at authoring time.
- `.github/skills/harvest/SKILL.md` (076.002-T) — a **Phase 1.5 docline gate** that runs
  before decomposition/backlog mutation/enclosing Stage harvest commit and **HALTs** on
  any violation, with two new telemetry lines (`Docline gate passed` / `Docline gate HALT`).

Because both gates call the same `backlogit docs lint` source entrypoint that CI's
`make docs-lint` uses, the local self-lint cannot pass while CI fails.

## Environment prechecks

- No binary build required — verification exercises the in-repo `go run ./cmd/backlogit
  docs lint` entrypoint directly (the same code path CI runs via `make docs-lint`).
- `--profile authoring` (the default) is the profile both new gates specify.

## Scenarios executed

| # | Scenario | Command | Expected | Observed | Result |
|---|---|---|---|---|---|
| A | Born-compliant plan passes (scoped) | `docs lint --path docs/exec-plans/2026-07-02-stage-harvest-docline-frontmatter-hardening-plan.md` | `valid: true`, 0 violations | `{valid:true, violation_count:0, findings:[]}` | ✅ |
| B | Corpus stays green (repo-wide) | `docs lint` (no args, == `make docs-lint`) | `valid: true`, 0 violations | `{valid:true, violation_count:0, findings:[]}` | ✅ |
| C | Gate HALTs on the 075-S defect (negative) | `docs lint --path <temp plan with doc_type: exec-plan, no title/source>` | `valid: false`; violations for `title`, `source`, `doc_type` | `{valid:false, violation_count:3}`: `title/required`, `source/required`, `doc_type/unknown_doc_type`; **exit status 1** | ✅ |

Scenario C's temp file (`docs/exec-plans/zzz-076-negative-test-DELETEME.md`) was created,
linted, and removed in a single guarded step — nothing lingered in the tree.

## Load-bearing invariants confirmed at runtime

- **Born-compliant generation.** The Stage-authored plan that ships with this change is
  itself born-compliant (`doc_type: plan`, top-level `title`/`source`) and passes the gate
  with 0 violations — the skills now teach authors to produce exactly this shape. This is
  the "born-compliant generation" pillar of the docline contract extended from
  `cmd/gen-docs` output to **agent-authored** plan docs.
- **Gate catches the regression.** The exact 075-S failure mode (`doc_type: exec-plan`
  outside the closed vocabulary, missing top-level `title`/`source`) is deterministically
  flagged (3 violations, non-zero exit), so the harvest Phase 1.5 gate would HALT before
  such a plan could ride into a Ship feature PR.
- **Source-entrypoint parity.** Both new gates pin to `go run ./cmd/backlogit docs lint`
  (== `make docs-lint`), never a stale installed binary, so the self-lint agrees with the
  non-bypassable CI Docline backstop.

## Evidence

- CI on PR #166 at HEAD `74db7ec`: **4/4 green** (`test (1.23)`, `test (1.24)` required,
  `CLI Reference Drift`, `Docline frontmatter gate`).
- Scoped + repo-wide `backlogit docs lint`: `valid: true`, 0 violations (Scenarios A/B).
- Negative reproduction: `valid: false`, 3 violations, exit 1 (Scenario C).
- No Go code change → no unit-test surface added; the enforcing CLI (`internal/docline`)
  is unchanged and already covered by its shipped suites.
- Report-only review during build + the 3 accepted Copilot P3 threads (see closure).

## Copilot review note

The only unresolved Copilot findings at merge were 3 P3 nitpicks that the `make docs-lint
--path` wording in **ride-along** artifacts (the historical Stage plan and the
auto-generated archive record `076.002-T.md`) reads as if the Makefile target accepts a
`--path` arg. The **shipped SKILL.md deliverables already distinguish** repo-wide `make
docs-lint` from scoped `go run ./cmd/backlogit docs lint --path <file>` (verified in the
diff). Operator accepted the 3 threads as won't-fix on 2026-07-02; a wording-cleanup
follow-up is tracked in stash `B55985DD`.

## Handoff to operational-closure

- Verification verdict: **PASS**
- Surfaces verified: Stage impl-plan self-lint + harvest Phase 1.5 gate, behaviorally via
  the shared `backlogit docs lint` entrypoint (positive + negative).
- BLOCKED prerequisites: none
- Risky action state: none — docs/harness-only, git-reversible, no persistence/schema impact
- Follow-up recommendations: `B55985DD` (reword ride-along `--path` artifacts) and the
  Stage-domain `EED25928` (deferred direction 3 + upstream `.tmpl` drift).
