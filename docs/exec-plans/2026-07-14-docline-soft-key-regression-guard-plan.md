---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Bounded docline soft-key regression guard implementation plan'
source: docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md
doc_type: plan
description: 'Test-first plan separating live tracked corpus, hermetic value semantics, and filesystem containment before metadata backfill.'
docline:
    date: 2026-07-14T18:38:00Z
    origin: docs/decisions/2026-07-14-docline-soft-key-regression-decision.md
    linked_stash_ids:
        - A4BE2FAD
    review_state: blocked
---

# Bounded Docline Soft-key Regression Guard Implementation Plan

## Problem Frame

Docline defaults make missing `chunk_strategy` and `schema_version` invisible to current validation. The guard must persist after backfill without combining live inventory, value parsing, temporary Git repositories, and platform containment into one oversized task. The protected untracked scratch file remains outside all mutations.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| D1 | Guard live committed corpus | `106.001-T` uses Git plus production Scope and exact values. |
| D2 | Preserve value negatives | `106.008-T` owns bounded hermetic value/type/malformed cases. |
| D3 | Prove untracked exclusion | `106.008-T` compares invalid tracked and untracked fixtures. |
| D4 | Contain reads | `106.009-T` owns Scope exclusion and link/reparse escape negatives. |
| D5 | Observe RED then GREEN | `106.001-T` first names nine omissions; `106.002-T`–`106.007-T` backfill them. |
| D6 | Preserve scratch | Git inventory excludes it and no task references it for mutation. |
| D7 | Keep implementation bounded | Every guard task is one file, fewer than five functions, and at most three scenario groups. |

## Task Map

### `106.001-T` — Live tracked corpus

**File:** `tests/integration/docline_soft_keys_test.go`

Use `git ls-files -z`, filter candidates only through exported `internal/docline.Scope()`, and require `chunk_strategy: h1-h2-h3` plus `schema_version` as YAML string `"1.0"`. At most three groups: Git/Scope inventory, exact value/type validation, and deterministic path-specific reporting.

**RED:** name exactly the nine tracked omissions.

**GREEN:** pass after all backfills.

### `106.008-T` — Hermetic value semantics

**File:** `tests/integration/docline_soft_key_values_test.go`

Temporary Git repositories call the same production Scope/value helper. At most three groups:

1. compliant and each missing key;
2. wrong value/type and malformed YAML;
3. invalid tracked detection while invalid untracked input is excluded.

These negatives remain active after the live corpus is compliant.

### `106.009-T` — Scope and filesystem containment

**File:** `tests/integration/docline_soft_key_containment_test.go`

At most three groups:

1. production Scope exclusions;
2. missing, non-regular, or lexical-invalid paths;
3. external-target symlink, junction, or reparse rejection before read.

If real link creation lacks platform privilege, skip only that fixture after an always-run synthetic classification negative.

### Metadata backfills

| Task | Files | Dependency |
|---|---|---|
| `106.002-T` | two 092-S closure docs | `106.001-T` RED |
| `106.003-T` | two compound test/lifecycle docs | `106.001-T` RED |
| `106.004-T` | one UTC compound doc | `106.001-T` RED |
| `106.005-T` | deterministic-gates decision and plan | `106.001-T` RED |
| `106.006-T` | evals design doc | `106.001-T` RED |
| `106.007-T` | covering-feature plan | `106.001-T` RED |

Backfills add only canonical keys and preserve body bytes and unrelated frontmatter.

## Dependency Graph

Observe `106.001-T` RED first. Then `{106.002-T–106.009-T}` may proceed in parallel using its shared inventory/value helper; after backfills, `106.001-T` returns GREEN.

## TDD and Quality-gate Sequence

1. Add the live test and record nine path-specific omissions.
2. Add hermetic value fixtures and run all three groups independently.
3. Add containment fixtures and run synthetic plus supported real-link cases.
4. Apply six bounded backfill tasks without body changes.
5. Require all three guard files GREEN.
6. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and require `gofmt -l .` to emit no output.
7. Run `go run ./cmd/backlogit docs lint` in a clean checkout.
8. Verify protected scratch remains untracked, untouched, and unstaged.

## Risks and Mitigations

- Git is required; repository CI already depends on it.
- Production Scope changes intentionally affect the live and hermetic corpus.
- Real-link privileges vary; synthetic classification always runs.
- A deliberate schema-version upgrade changes test and corpus together.

## Constitution Check

- **I:** only bounded Go integration tests and metadata are planned.
- **II (NON-NEGOTIABLE):** live RED precedes backfill, while hermetic negatives remain active.
- **III/IV:** Git, production Scope, and containment protect every read and exclude scratch.
- **V:** failures identify exact paths and fields.
- **VI:** three one-file guard tasks each contain at most three scenario groups and fewer than five functions.
- **VII:** no scratch deletion, edit, or staging is authorized.
- **VIII:** containment receives focused platform-aware tests.
- **IX:** committed source metadata remains the guarded truth.
- **X:** split tests are easier to execute and diagnose.
- **XI:** Stage does not merge or ship.

No constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: BLOCKED

**Formal plan-review provenance:** NOT RUN. This invocation cannot dispatch independent reviewer personas.

**Waiver authorization:** NONE. The PASS-only governance simplification is not a bootstrap approval.

**Current disposition:** shipment `095-S`, feature `106-F`, and all nine tasks remain blocked.

**Required unblock:** successful formal multi-persona evidence for these exact final plan bytes, or a separate durable operator bootstrap approval explicitly scoped to installing PASS-only governance and accepting this paired docline plan without claiming formal PASS.
