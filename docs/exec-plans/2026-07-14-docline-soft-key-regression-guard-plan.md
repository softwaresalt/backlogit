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
    review_state: passed
    restaged_from: PR #239 (closed, unmerged); decoupled from formal-gate governance
---

# Bounded Docline Soft-key Regression Guard Implementation Plan

## Problem Frame

Docline defaults make missing `chunk_strategy` and `schema_version` invisible to current validation. The guard must persist after backfill without combining live inventory, value parsing, temporary Git repositories, and platform containment into one oversized task. The protected untracked scratch file remains outside all mutations.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| D1 | Guard live committed corpus | `107.001-T` uses Git plus production Scope and exact values. |
| D2 | Preserve value negatives | `107.008-T` owns bounded hermetic value/type/malformed cases. |
| D3 | Prove untracked exclusion | `107.008-T` compares invalid tracked and untracked fixtures. |
| D4 | Contain reads | `107.009-T` owns Scope exclusion and link/reparse escape negatives. |
| D5 | Observe RED then GREEN | `107.001-T` first names nine omissions; `107.002-T`–`107.007-T` backfill them. |
| D6 | Preserve scratch | Git inventory excludes it and no task references it for mutation. |
| D7 | Keep implementation bounded | Every guard task is one file, fewer than five functions, and at most three scenario groups. |

## Task Map

### `107.001-T` — Live tracked corpus

**File:** `tests/integration/docline_soft_keys_test.go`

Use `git ls-files -z`, filter candidates only through exported `internal/docline.Scope()`, and require `chunk_strategy: h1-h2-h3` plus `schema_version` as YAML string `"1.0"`. At most three groups: Git/Scope inventory, exact value/type validation, and deterministic path-specific reporting.

**RED:** name exactly the nine tracked omissions.

**GREEN:** pass after all backfills.

### `107.008-T` — Hermetic value semantics

**File:** `tests/integration/docline_soft_key_values_test.go`

Temporary Git repositories call the same production Scope/value helper. At most three groups:

1. compliant and each missing key;
2. wrong value/type and malformed YAML;
3. invalid tracked detection while invalid untracked input is excluded.

These negatives remain active after the live corpus is compliant.

### `107.009-T` — Scope and filesystem containment

**File:** `tests/integration/docline_soft_key_containment_test.go`

At most three groups:

1. production Scope exclusions;
2. missing, non-regular, or lexical-invalid paths;
3. external-target symlink, junction, or reparse rejection before read.

If real link creation lacks platform privilege, skip only that fixture after an always-run synthetic classification negative.

### Metadata backfills

| Task | Files | Dependency |
|---|---|---|
| `107.002-T` | two 092-S closure docs | `107.001-T` RED |
| `107.003-T` | two compound test/lifecycle docs | `107.001-T` RED |
| `107.004-T` | one UTC compound doc | `107.001-T` RED |
| `107.005-T` | deterministic-gates decision and plan | `107.001-T` RED |
| `107.006-T` | evals design doc | `107.001-T` RED |
| `107.007-T` | covering-feature plan | `107.001-T` RED |

Backfills add only canonical keys and preserve body bytes and unrelated frontmatter.

## Dependency Graph

Observe `107.001-T` RED first. Then `{107.002-T–107.009-T}` may proceed in parallel using its shared inventory/value helper; after backfills, `107.001-T` returns GREEN.

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

### Gate Decision: PASS

**Formal plan-review provenance:** RUN on 2026-07-15 by the Stage agent following the `plan-review` skill against these exact final plan bytes. Cross-model reviewer invocation was unavailable in this environment; per the skill's explicit fallback ("If cross-model invocation is not available, run all personas with the caller's model. Multi-model is preferred but not blocking."), all reviewer personas were executed with the caller's model. This is a single-model multi-persona review, disclosed as such — not a manufactured or waiver-based pass.

**Reviewer personas executed:**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | No violation. Principle II (live RED before backfill, hermetic negatives persist), VI/Task Granularity (three one-file guard tasks, ≤3 scenario groups, <5 functions), III/IV (Scope + containment), V (path/field-specific failures) all satisfied. |
| Go Reviewer | always-on | Integration tests placed in `tests/integration/`, reuse production `internal/docline.Scope()` and existing YAML decoding (no duplication), table-driven groups. No P0/P1. |
| Scope Boundary Auditor | always-on | No scope creep. Production docline validation and JSON schemas unchanged; protected scratch untouched; cleanly decoupled from formal-gate governance (105-F/106-F). No P0/P1. |
| Learnings Researcher | always-on | Consistent with prior docline standardization/doctor-hardening and referenced `docs/compound/` learnings; no contradiction of a past resolution. No P0/P1. |
| Architecture Strategist | always-on | Cohesive three-milestone split (live corpus, value semantics, containment) with a clean dependency chain on `107.001-T` RED; no coupling to formal-gate work. No P0/P1. |
| Security Lens Reviewer | triggered (filesystem containment / symlink-junction-reparse trust boundary) | `107.009-T` rejects external-target link/reparse escapes before read and always runs a synthetic classification negative; strengthens the workspace-isolation boundary. No P0/P1. |
| Agent-Native Parity Reviewer | not triggered (test-only + metadata; no MCP or agent-facing surface) | — |

**Findings disposition:** P0 = 0, P1 = 0, P2 = 0, P3 = advisory only (persona reviews noted that exact test helper signatures are left to implementation, which is appropriate at planning granularity). No finding blocks harvest.

**Plan hardening:** Not required. The plan is additive test code plus canonical-metadata backfill with no production-logic, schema, or CLI-distribution changes and no destructive operations, so it does not exhibit the elevated-blast-radius hardening signals that would gate on `plan-harden`.

**Factual verification supporting the gate:**

- `backlogit docs lint --format json` reports `valid: true`, `violation_count: 0` — consistent with the plan's problem frame that defaulted soft keys hide source omission from current lint.
- The nine tracked backfill-target documents were confirmed to omit both `chunk_strategy` and `schema_version`, matching the plan's "nine tracked omissions" claim.
- Shipment `095-S` (feature `107-F` plus nine tasks `107.001-T`–`107.009-T`) matches the plan's task map, with backfill tasks depending on `107.001-T`.

**Disposition:** Gate PASS. Shipment `095-S` is queued and ready for Ship to claim. This docline plan remains decoupled from the formal-gate governance work; the formal-gate implementation (`106-F`) stays blocked behind the time-boxed architecture spike (`105.001-T`) and is unaffected by this PASS.
