---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Tracked-doc regression guard for docline soft keys'
source: docs/decisions/2026-07-14-docline-soft-key-regression-decision.md
doc_type: decision
description: 'Decision to guard explicit chunk_strategy and schema_version values on every tracked in-scope Markdown document without touching the intentional untracked scratch file.'
docline:
    date: 2026-07-14T18:32:00Z
    decision_status: decided
    linked_stash_ids:
        - A4BE2FAD
---

# Tracked-doc Regression Guard for Docline Soft Keys

## Problem Frame

`chunk_strategy` and `schema_version` are contract fields with defaults. `FromMap` supplies their values when keys are absent, so `docs lint` validates the defaulted object and cannot detect source-frontmatter drift. A persistent guard is needed without changing or deleting the operator-owned untracked `docs/decisions/2026-07-13-scratch-spike.md`.

## Research Findings

- `internal/docline/frontmatter.go` captures original key presence before applying defaults, but current validation only uses presence for optional min-length fields.
- The JSON schema defines defaults and does not list either soft key as required. Turning them into schema-required fields would change ingestion semantics, not merely repository authoring discipline.
- A repository scan found nine tracked in-scope documents missing one or both keys: two closure docs, three compound docs, one decision, one design doc, and two plans.
- The intentional untracked scratch spike also lacks both keys. A full filesystem lint rule would make this operator workspace fail until that unapproved file changed.
- CI and future regressions concern committed content. A git-tracked integration guard can enforce repository authoring discipline while deliberately ignoring untracked operator scratch state.
- After backfill the live corpus has no missing-key negative, and CI never sees the local scratch file; hermetic temporary Git repositories are required to keep missing-key and untracked-exclusion behavior testable.

## Decision

Add a Go integration test that enumerates candidates with `git ls-files`, filters them through the exported production `internal/docline.Scope()` descriptor rather than a copied scope table, rejects symlink/junction/reparse or resolved-target escapes before reading, parses YAML frontmatter, and requires:

- `chunk_strategy: h1-h2-h3`;
- `schema_version: "1.0"`.

The guard applies to every tracked in-scope document regardless of `doc_type`; these are common base-contract authoring conventions. It is deliberately a test rather than a production lint-rule change because the requirement is repository persistence, while the schema defaults remain valid for external ingestion. The live-corpus test's first run must fail on the nine tracked omissions. Backfill only those tracked files, in small file-family slices, then confirm green.

The same test file creates hermetic temporary Git repositories and exercises compliant tracked input, each missing key, wrong value/type, malformed YAML, invalid untracked exclusion, production-Scope exclusions, and an external-target symlink/reparse escape. Live and hermetic paths call the same inventory/Scope/containment/parser helper. If real link creation is unavailable, an explicit platform skip is allowed only alongside an always-run synthetic link-classification negative.

## Tracked Backfill Set

- `docs/closure/2026-07-13-092-S-compound-refresh.md`
- `docs/closure/2026-07-13-092-S-item-writer-utc-closure.md`
- `docs/compound/2026-07-13-parallel-test-safe-tz-subprocess-red-phase.md`
- `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`
- `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md`
- `docs/decisions/2026-06-30-backlogit-deterministic-gates-slice-deliberation.md`
- `docs/design-docs/autoharness-evals-gates-design.md`
- `docs/exec-plans/2026-06-30-backlogit-deterministic-gates-slice-plan.md`
- `docs/exec-plans/2026-07-02-shipment-covering-feature-display-plan.md`

## Scope Boundary

The untracked scratch spike is not read, edited, deleted, staged, or backfilled. Production docline validation behavior and the JSON schema remain unchanged.

## Constitution Check

- **I — Safety-First Go:** the guard uses existing Go/test dependencies and standard error handling.
- **II — Test-First:** observe the live guard fail on known omissions before backfill, then pass while hermetic negative cases remain active.
- **III/IV — Isolation and containment:** enumerate Git-tracked candidates, filter through production Scope, and reject link/reparse or resolved-target escapes before reading.
- **V — Observability:** live and synthetic failures name every offending file and key.
- **VI — Single Responsibility:** one test guards authoring convention; backfill tasks contain only Markdown metadata.
- **VII — Destructive approval:** the scratch file is preserved untouched; no deletion or overwrite is authorized.
- **VIII — Elevated risk:** no runtime/schema contract change; hardening is unnecessary.
- **IX — Git-friendly persistence:** the guard targets committed Markdown state.
- **X — Context efficiency:** one deterministic inventory replaces ad hoc audits.
- **XI — Merge history:** downstream delivery remains merge-commit-only.

No waiver or constitutional exception is required.

## Risks and Mitigations

- **Git dependency in integration tests:** this repository's CI always checks out Git history and existing integration helpers already resolve the repository root. Fail clearly if `git ls-files` cannot run.
- **Scope drift:** import `internal/docline.Scope()` as the sole filter so production changes cannot leave a stale copied table.
- **False confidence after backfill/platform limits:** synthetic staged/untracked/scope/link-classification fixtures always run; real-link privilege skips are explicit and never the only containment assertion.
- **Future schema version:** a deliberate contract upgrade must update this guard and corpus together.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md`.
