---
chunk_strategy: h1-h2-h3
description: 'Sustainable docs-frontmatter contract: body-preserving codec + idempotent seed-once migration + born-compliant generated docs + CI lint gate'
doc_type: learning
docline:
    date: 2026-06-26T00:00:00Z
    severity: high
    tags:
        - docline
        - frontmatter
        - documentation
        - codec
        - migration
        - idempotency
        - ci-gate
        - gen-docs
        - yaml
ingested_at: "2026-06-26T07:06:00Z"
schema_version: "1.0"
source: docs/compound/2026-06-26-docline-frontmatter-contract.md
title: 'Sustainable docs-frontmatter contract: codec + idempotent migration + born-compliant generation + CI gate'
---

# Sustainable Docs-Frontmatter Contract

## Context

Shipment 065-S standardized ~213 repository docs on the docline base frontmatter
v1 schema. The durable lesson is the *combination* of techniques that makes a
documentation-frontmatter contract enforceable without churn or content loss.

## Problem

Retrofitting a frontmatter contract across a large doc corpus risks three
failure modes:

1. **Body corruption** — naive YAML round-tripping rewrites the Markdown body
   (re-wrapping, CRLF flattening, escaping), producing noisy, untrustworthy diffs.
2. **Non-idempotent migration** — re-running the migration keeps producing diffs
   (e.g., re-stamping timestamps), so the corpus never reaches a stable state and
   a CI gate can never be trusted.
3. **Generated-doc drift** — generated docs (`docs/cli-reference/**`) are
   regenerated without frontmatter, so they perpetually fail the gate or require
   a manual post-pass.

## Solution (the four-part pattern)

1. **Body-preserving codec.** Read/write *only* the frontmatter block; never
   touch body bytes or line endings. Verify with a `body_bytes_changed: false`
   assertion in the migrate plan, and prove idempotency by applying to an
   already-compliant file and confirming an empty content diff.
2. **Idempotent, seed-once normalizer.** Canonicalize key ordering/quoting and
   fold legacy keys under a namespace (`docline:`) by *moving*, never
   duplicating. Seed mutable provenance fields (`ingested_at`) exactly once so
   re-runs are byte-stable. Result: the migration converges and stays converged.
3. **Born-compliant generation.** Teach the generator (`cmd/gen-docs`) to emit
   compliant frontmatter at generation time, so generated docs never need a
   second normalization pass and never break the gate.
4. **CI lint gate.** Enforce the contract on every PR (`make docs-lint` →
   `backlogit docs lint`) so the corpus cannot regress. The gate is only
   trustworthy *because* (1)–(3) guarantee a stable, achievable green state.

Together these turn a one-time migration into a self-sustaining contract.

## Pitfall: unquoted `#` in YAML scalar values truncates silently

A YAML scalar containing an unquoted `#` (commonly a PR reference like ` #137`
in a `description`) is parsed as an inline **comment** and silently truncated —
the value loses everything from `#` onward, and no error is raised. The same
hazard applies to leading special characters and embedded `:`.

**Rule:** quote any frontmatter string value that contains `#`, `:`, or a
leading special character. Prefer single quotes for descriptions/titles
(`description: 'Closure for PR #137'`).

This bites hardest in closure/compound docs that reference PR numbers, and the
docline gate will not always catch the *semantic* loss (the truncated value can
still be schema-valid), so it must be handled at authoring time.

## Evidence

* `internal/docline/` (codec, normalize, classify, policy, validate, service) +
  `cmd/gen-docs` — merged via PR #136 (`2a5df85b`) and PR #137 (`23a8b045`).
  Note: as of 068-S (PR #148, merge `7450271a`) the body-preserving **codec**
  was extracted to the stdlib-only leaf package `internal/mdfront` (and the
  atomic-write helper to `internal/atomicfile`); `internal/docline` re-exports
  it via a true type alias. The four-part contract pattern below is unchanged —
  only the codec's location moved. See
  `docs/compound/2026-06-28-codec-extraction-leaf-packages.md`. As of 069-S
  (PR #152, merge `1dd4e69a`) the gate hardened on two axes: `ValidateFields`
  now enforces the full pattern/minLength/additionalProperties schema
  (`ErrSchemaViolation`, 0 new deps), and `ApplyMigration` re-reads targets at
  apply time, aborting on drift with `ErrConcurrentEdit` to keep migration
  body-preserving under concurrent edits.
* Migration of ~213 docs with **0 body-byte changes**; single-file re-apply is a
  byte-identical no-op (idempotency proof).
* CI "Docline frontmatter gate" (`.github/workflows/ci.yml` → `make docs-lint`).
* Decision: `docs/decisions/2026-06-22-docline-taxonomy-and-field-mapping.md`.
* Plan: `docs/exec-plans/2026-06-22-docline-frontmatter-standardization-plan.md`.
* Authoring guide: `docs/docline-frontmatter-authoring-guide.md`.

## Reinforcement — 076-S: born-compliant *agent-authored* plans + self-lint against the same gate

Shipment 076-S (PR #166, merge `ef9dc20`) extends the **born-compliant generation**
pillar (solution part 3) from machine-generated docs (`cmd/gen-docs`) to
**agent-authored** plan docs under `docs/exec-plans/**`. Root cause: in 075-S (PR #164)
a Stage-authored plan was hand-written with `doc_type: exec-plan` (a natural but invalid
guess, outside the closed vocabulary) and no top-level `title`/`source`; that harvest
commit rode into the Ship feature branch and failed the CI Docline gate with 3 violations.

The fix hardens the *authoring* surface with two shift-left gates (docs/harness only, no
Go code):

1. **Contract-at-authoring** — `.github/skills/impl-plan/SKILL.md` now specifies the
   gate-required frontmatter (`doc_type: plan` + top-level `title`/`source`) with a copyable
   template and the unquoted-`#` pitfall, so plans are *born* compliant.
2. **Producer self-lints against the consumer's gate** — both `impl-plan` (mandatory Phase 4
   self-lint) and `harvest` (Phase 1.5 HALT gate) run `go run ./cmd/backlogit docs lint`,
   the **same source entrypoint** CI's `make docs-lint` uses. The generalizable pattern: a
   producer validates its own output against the *identical* gate that will judge it
   downstream, pinned to the CI entrypoint (never a stale installed binary) so a local pass
   cannot diverge from the CI backstop. Verified behaviorally in 076-S runtime verification:
   the shipped plan lints clean; a replica of the 075-S defect is flagged (3 violations,
   exit 1).

Caveat worth remembering (surfaced in PR #166 review): the authoring-profile self-lint
checks `source` only for **presence** (non-empty), not format or path-match — a stale copied
`source` passes silently. Set `source` to the file's own path at authoring time.

## Reinforcement — 091-S: born-compliant *example blocks* inside agent-authoring skills

Shipment 091-S (PR #231, merge `ec2b859`) extends born-compliant authoring one
level further — from the plan/prose an agent writes to the **fenced example block
a skill shows the agent to copy**. The `spike` skill's Phase 5 "Write Findings
Artifact" section embedded a YAML frontmatter example that predated the docline
contract: `type: spike` at top level (should be `doc_type: decision` for the
`docs/decisions/**` output path), no `source`, and eight non-contract keys
(`type`/`date`/`time_box`/`conclusion`/`confidence`/`linked_parent_work_item`/
`promoted_to`/`tags`) at top level. An agent following it verbatim would author a
findings artifact that fails the CI Docline gate.

The fix replaced the example — identically in both in-repo copies
`plugin/skills/spike/SKILL.md` and `.github/skills/spike/SKILL.md` — with a
docline-conformant block: top-level `title`/`source`/`doc_type: decision`/
`description`, all non-contract keys nested under `docline:` (4-space indent,
matching this repo's gold-standard spike artifacts
`docs/decisions/2026-05-05-telemetry-gap-analysis-spike.md` and
`docs/decisions/2026-07-09-github-actions-cost-spike.md`). Verification authored a
throwaway `docs/decisions/*-spike.md` from the reconciled example and confirmed
`backlogit docs lint --profile authoring` reports 0 findings — the example now
*demonstrates* the same gate it will be judged by.

Generalizable rule: **an instructional example is a generator too.** If a skill
shows an agent a copyable artifact template, that template must itself be
born-compliant with any downstream gate; validate it by authoring-from-the-example
and linting, not by eyeballing. One more durable note from this shipment:
**generated-vs-source drift is inherent when you fix a generated copy in-tree.**
Both edited SKILL.md files are generated from an upstream `spike/SKILL.md.tmpl`
that lives in the *external* autoharness repo (Principle IV — never edited from
this workspace). The in-repo fix is correct but the next regeneration overwrites
it unless the upstream template is also updated; that is tracked as follow-up
stash `7F0A6E89`. When you must fix a generated artifact in-tree, always record
the source-template follow-up so the fix is not silently lost.

## Applicability

Reuse this four-part pattern for any cross-cutting metadata contract over a large
file corpus (frontmatter, license headers, SPDX tags, codegen banners): preserve
content, make the transform idempotent, make generators born-compliant, then gate.
Extend "generators" to include **agent authoring surfaces** — teach the skill/prompt the
gate-required shape and make it self-lint against the same CI entrypoint before handoff.
Extend it once more to **instructional example blocks** a skill instructs the agent to
copy: validate them by authoring-from-the-example and linting, and record a source-template
follow-up whenever the fix lands on a generated copy whose upstream template is out of tree.
