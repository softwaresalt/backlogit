---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Bounded tracked-doc regression guards for docline soft keys'
source: docs/decisions/2026-07-14-docline-soft-key-regression-decision.md
doc_type: decision
description: 'Decision to guard live tracked corpus and hermetic value semantics, and to correct the docline base/extension seam so unknown top-level extension keys (backlogit-owned size metadata) are preserved in place rather than folded under the docline namespace or dropped on serialization — changing both internal/docline/normalize.go (stop folding) and internal/docline/frontmatter.go (BaseFrontmatter carries and re-emits unknown top-level keys), opening the base JSON schema (schemas/docline/base-frontmatter-v1.schema.json) for declared producer-owned top-level extension fields, and rewriting the authoring guide to the inheritance model, gated behind a RED-first integration regression, while preserving production Scope and protected scratch state; no realpath/symlink filesystem containment is implemented.'
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
- The base JSON schema itself also contradicts the open base contract: `schemas/docline/base-frontmatter-v1.schema.json` declares top-level `additionalProperties: false`, which **rejects** producer-owned top-level extension keys. Opening it is a separate width-isolated **schema task** (`107.011-T`) using the least-permissive compatible mechanism (declared producer extensions distinguished from arbitrary legacy keys) with test-first schema-validation coverage. The authoring guide (`docs/docline-frontmatter-authoring-guide.md`), which currently says every non-contract key belongs under `docline` and that migration "moves every unknown key there", is corrected to the base/extension **inheritance model** by a separate **docs task** (`107.012-T`). The earlier "no schema changes" framing was therefore false at the shipment level and is corrected: the Go codec task (`107.009-T`) makes no schema change, but the shipment carries a bounded schema change in `107.011-T`.
- **Forward-only / migration evidence.** A Git inventory of tracked in-Scope docs for already-folded producer keys (`docline.size`, `docline.size_source`, `docline.size_ruleset_version`) found **zero** matches, so the contract is forward-only with **no** back-migration; `107.010-T` re-verifies zero at execution and records a bounded migration requirement only if that changes. The one canonical derived-key location is **top-level**.
- **Consumer evidence.** No active Go consumer reads `docline.size*`, and no `size_source`/`size_ruleset_version` symbol exists anywhere in `internal/` or `cmd/`, so relocating these keys to the top level breaks **no** existing consumer contract.

## Decision

Split the guard into six tasks:

1. `107.001-T`: live Git-tracked corpus plus production Scope and exact soft-key values;
2. `107.008-T`: hermetic value/type/malformed/tracked-versus-untracked fixtures;
3. `107.010-T`: **RED-first integration regression** (`tests/integration/docline_extension_compat_test.go`, test-only) proving representative backlogit-owned extension keys (`size`, `size_source`, `size_ruleset_version`) stay top-level, remain **semantically/deep-value equivalent** with unchanged document body bytes and idempotent normalization — **not** raw frontmatter byte/lexical preservation, since `Normalize`/`mdfront.Encode` reserializes YAML and may canonicalize ordering, quotes, scalar spelling, comments, and anchors (a raw-byte/YAML-node design is out of scope unless explicitly chosen later) — and are never validated/defaulted/emitted by docline; observed **failing at HEAD**, **sequenced after (dependent on) the `107.001-T` live-corpus RED**, and the dependency gate for `107.009-T`;
4. `107.009-T`: **production-contract change** to both docline seams — `internal/docline/normalize.go` stops folding non-contract top-level keys under the `docline` namespace, **and** `internal/docline/frontmatter.go` gains a `BaseFrontmatter` carrier so `FromMap` captures and `ToMap` re-emits unknown top-level keys instead of dropping them (`ToMap` otherwise emits only base-contract fields plus `docline`); the two contradictory normalization unit tests (`TestNormalize_ContractSurfaceHoldsOnlyContractKeys`, `TestNormalize_FoldsLegacyTypeUnderDocline`) in `normalize_test.go` are updated to expect pass-through, while `policy_test.go` `TestIsContractField` remains valid and unchanged. Docline validates/normalizes only its own base fields and must never own, default, validate, or emit extension semantics. This task implements **no** production filesystem containment and asserts **no** realpath/symlink guarantee.
5. `107.011-T`: **schema-contract change** opening `schemas/docline/base-frontmatter-v1.schema.json` for declared producer-owned top-level extension fields using the **least-permissive compatible mechanism** justified by current schema/test patterns (declared producer extensions distinguished from arbitrary legacy keys, not a blanket `additionalProperties: true`), with **test-first** schema-validation coverage; gated behind the `107.010-T` RED and correlated with `107.009-T`. This is the schema change the earlier no-schema-change framing omitted.
6. `107.012-T`: **documentation change** rewriting `docs/docline-frontmatter-authoring-guide.md` from the "move every non-contract key under `docline`" model to the base/extension **inheritance model** — docline owns and validates only base fields; derived producers (backlogit) own top-level extension fields; docline preserves but never interprets, defaults, validates, or emits them; declared producer extensions are distinguished from legacy arbitrary keys; the canonical derived-key location is top-level. Depends on `107.009-T` and `107.011-T`.

Task `107.009-T` is a single-concern production change (the `internal/docline/normalize.go` and `internal/docline/frontmatter.go` top-level pass-through seams) with its co-located unit-test updates, width-isolated to `internal/docline`; the RED integration regression is the separate `107.010-T`, and the other two guards (`107.001-T`, `107.008-T`) each have one file, fewer than five functions, and at most three scenario groups. Shared production behavior remains `internal/docline.Scope()` and existing YAML decoding; tests do not duplicate a scope table, add production containment, or change schema defaults. The correlated non-Go corrections are their own width-isolated tasks so `107.009-T` is not overloaded: `107.011-T` touches only the schema family (`schemas/docline/base-frontmatter-v1.schema.json` plus its schema-validation test) and `107.012-T` touches only the authoring guide (`docs/docline-frontmatter-authoring-guide.md`).

## Tracked Backfill Set

- `107.002-T`: two 092-S closure documents.
- `107.003-T`: parallel-test and lifecycle compound documents.
- `107.004-T`: UTC timestamp compound document.
- `107.005-T`: deterministic-gates decision and plan.
- `107.006-T`: evals design document.
- `107.007-T`: covering-feature plan.

Each backfill task depends only on observing `107.001-T` RED and changes no body bytes.

## Base Contract and Extension-Key Compatibility

Git determines tracked candidates and production `internal/docline.Scope()` filters them; this is the only include/exclude authority. The base docline contract is open/extensible: optional producer-specific keys — including the backlogit-owned `size`, `size_source`, and `size_ruleset_version` extensions — may be present without docline owning their meaning, and MUST be preserved unchanged at the top level. The extension milestone (`107.009-T`) changes both production seams — the normalization seam (`normalize.go`, stop folding under `docline`) and the serialization seam (`frontmatter.go`, `BaseFrontmatter` carries and re-emits unknown top-level keys) — so such optional keys pass through **in place** rather than being folded under the `docline` namespace or dropped by `ToMap`, and updates the two contradictory normalization unit tests; the size-extension regression is the RED-first `107.010-T`. Docline validates/normalizes only its own base fields and never validates, defaults, rewrites, or emits extension semantics. No realpath, symlink, junction, or reparse containment is implemented or claimed by any milestone; production containment stays lexical (`core.SafeResolve`) and is out of scope for this guard. Preservation and validation are intentionally **separate** responsibilities: the codec (`107.009-T`) losslessly preserves all unknown top-level keys in place, while the base schema (`107.011-T`) sanctions only **declared** producer extensions — preserved-but-undeclared legacy keys remain losslessly preserved but are **not** thereby schema-valid, and `107.012-T` documents that distinction and the lint/migration treatment for them. There is no missing selective codec migration.

## Scope Boundary

The untracked `docs/decisions/2026-07-13-scratch-spike.md` is not read for mutation, edited, deleted, staged, or backfilled. The production changes in this guard are bounded to three concerns in three width-isolated tasks: the top-level pass-through of non-contract keys across the normalization seam (`normalize.go`) and the serialization seam (`frontmatter.go`) in `107.009-T`; the base JSON schema opening for declared producer-owned top-level extension fields (`schemas/docline/base-frontmatter-v1.schema.json`) in `107.011-T`; and the authoring-guide inheritance-model rewrite (`docs/docline-frontmatter-authoring-guide.md`) in `107.012-T`. Base-field validation semantics (required fields, closed doc_type vocabulary, minLength, content_sha256 pattern) remain unchanged. `size` semantics (validation, defaults, provenance, composition) remain backlogit-owned and out of scope for this decision (they are the subject of the separate size spike, 096-S).

## Constitution Check

- **I:** bounded Go changes use existing Go and YAML dependencies — the normalization seam (`normalize.go`) plus the serialization seam (`frontmatter.go`) with co-located unit-test updates, and one RED-first integration regression (`107.010-T`); the base JSON schema opening (`107.011-T`) is a bounded, test-first schema-contract change; no `unsafe`, and existing lint/vet gates apply.
- **II:** live RED precedes backfill; for `107.009-T`, failing pass-through/regression assertions precede the seam change; hermetic value negatives stay active after GREEN.
- **III/IV:** Git inventory and production `internal/docline.Scope()` bound the read set and exclude scratch; no real-path/symlink containment is added, and none is claimed.
- **V:** every failure names its path and field, or the specific extension key preserved in place.
- **VI:** live corpus, values, the extension RED regression (`107.010-T`), the production seam change (`107.009-T`), the schema opening (`107.011-T`), and the authoring-guide rewrite (`107.012-T`) are separate width-isolated tasks; `107.009-T` stays confined to `internal/docline` (`normalize.go` + `frontmatter.go`), `107.011-T` to the schema family, and `107.012-T` to the guide.
- **VII:** no protected scratch mutation is authorized.
- **VIII:** the extension pass-through contract receives a focused production change plus regression coverage; no filesystem trust boundary is asserted.
- **IX:** the guard protects committed Git-readable state.
- **X:** bounded tests avoid one oversized scenario matrix.
- **XI:** Stage does not merge or ship.

### Conflict-Resolution Records (bounded exceptions)

This decision documents two bounded exceptions rather than claiming zero deviation. Each records the governing principle/heuristic, its justification, and the simpler alternative that was rejected.

- **Scoped-vs-repo-wide gofmt gate.** *Principle/heuristic:* quality-gate honesty + width isolation — the formatting gate verifies only the files this shipment changes. *Justification:* the repository carries pre-existing `gofmt` debt (26 unrelated files at authoring time); a repo-wide `gofmt -l .` clean requirement would force remediation of unrelated debt and silently expand blast radius. *Rejected simpler alternative:* requiring repo-wide `gofmt -l .` to emit nothing (rejected because it couples this bounded guard to unrelated cleanup and violates width isolation).
- **Three-file `107.009-T` task heuristic.** *Principle/heuristic:* task granularity (single concern, roughly ≤3 files / <5 functions, ≤2h). *Justification:* the honest codec correction spans two production seams (`normalize.go`, `frontmatter.go`) plus their co-located unit test (`normalize_test.go`) = three files, still within the heuristic; the RED integration regression, the schema opening, and the authoring-guide rewrite are split into `107.010-T`, `107.011-T`, and `107.012-T` instead of being folded into `107.009-T`. *Rejected simpler alternative:* a single "docline extension" task doing codec + schema + docs + integration test together (rejected because it mixes Go/schema/docs concerns, breaks width isolation, and exceeds the 2h rule).

No constitutional principle is violated and no formal waiver is required; the two items above are documented bounded exceptions, not silent deviations.

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
