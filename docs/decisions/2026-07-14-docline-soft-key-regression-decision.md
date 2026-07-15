---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Bounded tracked-doc regression guards for docline soft keys'
source: docs/decisions/2026-07-14-docline-soft-key-regression-decision.md
doc_type: decision
description: 'Decision to split live corpus, hermetic value, and filesystem-containment checks while preserving production Scope and protected scratch state.'
docline:
    date: 2026-07-14T18:32:00Z
    decision_status: decided
    linked_stash_ids:
        - A4BE2FAD
---

# Bounded Tracked-doc Regression Guards for Docline Soft Keys

## Problem Frame

`chunk_strategy` and `schema_version` have defaults, so normal lint cannot detect source-frontmatter omission. One earlier test task combined live Git inventory, production Scope, YAML semantics, temporary repositories, and platform-specific containment, exceeding the repository's task heuristics.

## Research Findings

- `internal/docline.Scope()` is the production include/exclude authority and must not be copied.
- Nine tracked in-scope documents omit one or both explicit soft keys.
- The intentional untracked scratch spike also omits them and must remain untouched.
- Live corpus becomes all-positive after backfill, so hermetic negative fixtures remain necessary.
- Value parsing and filesystem containment are independent milestones and can use separate one-file integration tests.

## Decision

Split the guard into three tasks:

1. `107.001-T`: live Git-tracked corpus plus production Scope and exact soft-key values;
2. `107.008-T`: hermetic value/type/malformed/tracked-versus-untracked fixtures;
3. `107.009-T`: Scope exclusion and lexical/symlink/junction/reparse containment fixtures.

Each task has one file, fewer than five functions, and at most three scenario groups. Shared production behavior remains `internal/docline.Scope()` and existing YAML decoding; tests do not duplicate a scope table or change schema defaults.

## Tracked Backfill Set

- `107.002-T`: two 092-S closure documents.
- `107.003-T`: parallel-test and lifecycle compound documents.
- `107.004-T`: UTC timestamp compound document.
- `107.005-T`: deterministic-gates decision and plan.
- `107.006-T`: evals design document.
- `107.007-T`: covering-feature plan.

Each backfill task depends only on observing `107.001-T` RED and changes no body bytes.

## Containment and Platform Rules

Git determines tracked candidates. Production Scope filters them. Before any selected file is read, containment rejects invalid or non-regular paths and external-target symlink, junction, or reparse escapes. A platform may skip real-link creation only after an always-run synthetic link-classification negative executes.

## Scope Boundary

The untracked `docs/decisions/2026-07-13-scratch-spike.md` is not read for mutation, edited, deleted, staged, or backfilled. Production docline validation and JSON schemas remain unchanged.

## Constitution Check

- **I:** tests use existing Go and YAML dependencies.
- **II:** live RED precedes backfill; hermetic value and containment negatives stay active after GREEN.
- **III/IV:** Git inventory, production Scope, and real-path containment protect workspace boundaries.
- **V:** every failure names its path and field or containment class.
- **VI:** live, values, and containment are separate one-file tasks with at most three scenario groups.
- **VII:** no protected scratch mutation is authorized.
- **VIII:** platform-specific containment receives its own focused negatives.
- **IX:** the guard protects committed Git-readable state.
- **X:** bounded tests avoid one oversized scenario matrix.
- **XI:** Stage does not merge or ship.

No waiver or constitutional exception is required.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md`.
