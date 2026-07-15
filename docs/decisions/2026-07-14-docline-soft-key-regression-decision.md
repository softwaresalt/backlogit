---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Bounded tracked-doc regression guards for docline soft keys'
source: docs/decisions/2026-07-14-docline-soft-key-regression-decision.md
doc_type: decision
description: 'Decision to guard live tracked corpus and hermetic value semantics, and to correct the docline base/extension seam so unknown top-level extension keys (backlogit-owned size metadata) are preserved in place rather than folded under the docline namespace or dropped on serialization — changing both internal/docline/normalize.go (stop folding) and internal/docline/frontmatter.go (BaseFrontmatter carries and re-emits unknown top-level keys), gated behind a RED-first integration regression, while preserving production Scope and protected scratch state; no realpath/symlink filesystem containment is implemented.'
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
- Value parsing and open extension-key compatibility are independent milestones and can use separate one-file integration tests.
- Production docline containment is **lexical-only** (`core.SafeResolve`) and `ApplyMigration` explicitly adds **no** symlink-based realpath containment, so a test-only task cannot deliver external-target rejection without unrelated new production behavior. The base docline contract is open/extensible, and `size`/`size_source`/`size_ruleset_version` are optional backlogit-owned extension keys docline never owns. **However, the current production seam contradicts that contract in two places**: `internal/docline/normalize.go` folds every non-contract top-level key under the `docline` namespace (`foldUnderDocline`), driven by the closed `contractFields` set in `internal/docline/policy.go`; **and** `internal/docline/frontmatter.go` serializes through `BaseFrontmatter.ToMap()`, which emits only base-contract fields plus `docline`, so unknown top-level keys are dropped on serialization because `BaseFrontmatter` has no carrier for them. A test-only "tolerance" guard would therefore assert behavior production does not have. The third task is consequently an **executable production-contract change** touching both seams: preserve unknown top-level extension keys in place (stop relocating them under `docline` and stop dropping them in `ToMap`) — not a filesystem-containment guard. The size-extension regression is split into a RED-first integration task observed failing before the seam change.

## Decision

Split the guard into four tasks:

1. `107.001-T`: live Git-tracked corpus plus production Scope and exact soft-key values;
2. `107.008-T`: hermetic value/type/malformed/tracked-versus-untracked fixtures;
3. `107.010-T`: **RED-first integration regression** (`tests/integration/docline_extension_compat_test.go`, test-only) proving representative backlogit-owned extension keys (`size`, `size_source`, `size_ruleset_version`) stay top-level, byte-unchanged, idempotent, and are never validated/defaulted/emitted by docline — observed **failing at HEAD** and the dependency gate for `107.009-T`;
4. `107.009-T`: **production-contract change** to both docline seams — `internal/docline/normalize.go` stops folding non-contract top-level keys under the `docline` namespace, **and** `internal/docline/frontmatter.go` gains a `BaseFrontmatter` carrier so `FromMap` captures and `ToMap` re-emits unknown top-level keys instead of dropping them (`ToMap` otherwise emits only base-contract fields plus `docline`); the two contradictory normalization unit tests (`TestNormalize_ContractSurfaceHoldsOnlyContractKeys`, `TestNormalize_FoldsLegacyTypeUnderDocline`) in `normalize_test.go` are updated to expect pass-through, while `policy_test.go` `TestIsContractField` remains valid and unchanged. Docline validates/normalizes only its own base fields and must never own, default, validate, or emit extension semantics. This task implements **no** production filesystem containment and asserts **no** realpath/symlink guarantee.

Task `107.009-T` is a single-concern production change (the `internal/docline/normalize.go` and `internal/docline/frontmatter.go` top-level pass-through seams) with its co-located unit-test updates, width-isolated to `internal/docline`; the RED integration regression is the separate `107.010-T`, and the other two guards (`107.001-T`, `107.008-T`) each have one file, fewer than five functions, and at most three scenario groups. Shared production behavior remains `internal/docline.Scope()` and existing YAML decoding; tests do not duplicate a scope table, add production containment, or change schema defaults.

## Tracked Backfill Set

- `107.002-T`: two 092-S closure documents.
- `107.003-T`: parallel-test and lifecycle compound documents.
- `107.004-T`: UTC timestamp compound document.
- `107.005-T`: deterministic-gates decision and plan.
- `107.006-T`: evals design document.
- `107.007-T`: covering-feature plan.

Each backfill task depends only on observing `107.001-T` RED and changes no body bytes.

## Base Contract and Extension-Key Compatibility

Git determines tracked candidates and production `internal/docline.Scope()` filters them; this is the only include/exclude authority. The base docline contract is open/extensible: optional producer-specific keys — including the backlogit-owned `size`, `size_source`, and `size_ruleset_version` extensions — may be present without docline owning their meaning, and MUST be preserved unchanged at the top level. The extension milestone (`107.009-T`) changes both production seams — the normalization seam (`normalize.go`, stop folding under `docline`) and the serialization seam (`frontmatter.go`, `BaseFrontmatter` carries and re-emits unknown top-level keys) — so such optional keys pass through **in place** rather than being folded under the `docline` namespace or dropped by `ToMap`, and updates the two contradictory normalization unit tests; the size-extension regression is the RED-first `107.010-T`. Docline validates/normalizes only its own base fields and never validates, defaults, rewrites, or emits extension semantics. No realpath, symlink, junction, or reparse containment is implemented or claimed by any milestone; production containment stays lexical (`core.SafeResolve`) and is out of scope for this guard.

## Scope Boundary

The untracked `docs/decisions/2026-07-13-scratch-spike.md` is not read for mutation, edited, deleted, staged, or backfilled. Production docline JSON schemas and base-field validation remain unchanged; the only production change is the top-level pass-through of non-contract keys across the normalization seam (`normalize.go`) and the serialization seam (`frontmatter.go`). `size` semantics (validation, defaults, provenance, composition) remain backlogit-owned and out of scope for this decision (they are the subject of the separate size spike, 096-S).

## Constitution Check

- **I:** bounded Go changes use existing Go and YAML dependencies — the normalization seam (`normalize.go`) plus the serialization seam (`frontmatter.go`) with co-located unit-test updates, and one RED-first integration regression (`107.010-T`).
- **II:** live RED precedes backfill; for `107.009-T`, failing pass-through/regression assertions precede the seam change; hermetic value negatives stay active after GREEN.
- **III/IV:** Git inventory and production `internal/docline.Scope()` bound the read set and exclude scratch; no real-path/symlink containment is added, and none is claimed.
- **V:** every failure names its path and field, or the specific extension key preserved in place.
- **VI:** live corpus, values, the extension RED regression (`107.010-T`), and the production seam change (`107.009-T`) are separate width-isolated tasks; `107.009-T` stays confined to `internal/docline` (`normalize.go` + `frontmatter.go`).
- **VII:** no protected scratch mutation is authorized.
- **VIII:** the extension pass-through contract receives a focused production change plus regression coverage; no filesystem trust boundary is asserted.
- **IX:** the guard protects committed Git-readable state.
- **X:** bounded tests avoid one oversized scenario matrix.
- **XI:** Stage does not merge or ship.

No waiver or constitutional exception is required.

## Promotion

Promote this decision to `docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md`.

## Superseded Historical Note

> **Superseded — do not treat as current.** An earlier revision of this decision
> framed `107.009-T` as "Scope exclusion and lexical/symlink/junction/reparse
> containment fixtures" and described real-path containment protecting workspace
> boundaries. That framing is **withdrawn**: production docline containment is
> lexical-only (`core.SafeResolve`) and `ApplyMigration` adds no symlink-based
> realpath containment, so a test-only task could not deliver external-target
> rejection without unrelated new production behavior. The authoritative decision
> above (open extension-key compatibility on the base/open-extensible docline
> contract, with **no** realpath/symlink containment) governs. This note is
> retained only for history and must not be read as a current requirement.
