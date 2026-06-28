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
  `docs/compound/2026-06-28-codec-extraction-leaf-packages.md`.
* Migration of ~213 docs with **0 body-byte changes**; single-file re-apply is a
  byte-identical no-op (idempotency proof).
* CI "Docline frontmatter gate" (`.github/workflows/ci.yml` → `make docs-lint`).
* Decision: `docs/decisions/2026-06-22-docline-taxonomy-and-field-mapping.md`.
* Plan: `docs/exec-plans/2026-06-22-docline-frontmatter-standardization-plan.md`.
* Authoring guide: `docs/docline-frontmatter-authoring-guide.md`.

## Applicability

Reuse this four-part pattern for any cross-cutting metadata contract over a large
file corpus (frontmatter, license headers, SPDX tags, codegen banners): preserve
content, make the transform idempotent, make generators born-compliant, then gate.
