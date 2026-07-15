---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Bounded docline soft-key regression guard implementation plan'
source: docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md
doc_type: plan
description: 'Test-first plan separating live tracked corpus, hermetic value semantics, and open extension-key compatibility before metadata backfill.'
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

Docline defaults make missing `chunk_strategy` and `schema_version` invisible to current validation. The guard must persist after backfill without combining live inventory, value parsing, temporary Git repositories, and open extension-key compatibility into one oversized task. The protected untracked scratch file remains outside all mutations.

**Base-contract / extension ownership (authoritative for PR #241).** Docline owns only the base Markdown/frontmatter ingestion contract and its compatibility rules. That base contract is open/extensible: producer-specific optional properties (for example the backlogit-owned `size`, `size_source`, `size_ruleset_version`) may be added without making docline own their meaning. Consumers such as graphtor and engram ingest the stable base contract and tolerate/preserve or safely ignore extension properties per existing codec behavior. This plan's guard therefore protects base-contract stability and extension tolerance — it does not add production filesystem containment, which is unrelated to the base-contract/extension boundary.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| D1 | Guard live committed corpus | `107.001-T` uses Git plus production Scope and exact values. |
| D2 | Preserve value negatives | `107.008-T` owns bounded hermetic value/type/malformed cases. |
| D3 | Prove untracked exclusion | `107.008-T` compares invalid tracked and untracked fixtures. |
| D4 | Guard open extension compatibility | `107.009-T` proves optional backlogit-owned extension keys (size metadata) do not break base docline ingestion/lint/migration and that docline never owns extension semantics. |
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

### `107.009-T` — Open extension-key compatibility guard

**File:** `tests/integration/docline_extension_compat_test.go`

**Scope reclassification (PR #241):** filesystem symlink/junction/reparse containment is removed from this task. Production docline containment is lexical-only via `core.SafeResolve` (`internal/core/workspace.go`), and `ApplyMigration` explicitly documents that it adds no symlink-based realpath containment (`internal/docline/service.go`). A test-only task cannot deliver external-target rejection without implementing new production behavior, and that behavior is unrelated to the base-contract/extension boundary this shipment guards. No production containment is planned or implemented here.

Instead, this guard reuses production `internal/docline.Scope()` and the existing decode path to prove the base contract stays open/extensible. At most three groups:

1. representative optional backlogit-owned extension keys (`size`, `size_source`, `size_ruleset_version`) added to an in-scope doc leave base ingestion valid;
2. the same extension keys survive a lint/migration round-trip without body mutation and without docline validating or emitting their semantics;
3. docline neither defaults, rewrites, nor rejects unknown extension keys — it tolerates/preserves them (ownership stays with the producer).

The guard asserts base-contract stability and extension tolerance only; it makes no claim about filesystem containment.

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
3. Add open extension-key compatibility fixtures and assert base ingestion/lint/migration tolerate optional backlogit-owned extension keys without docline owning their semantics.
4. Apply six bounded backfill tasks without body changes.
5. Require all three guard files GREEN.
6. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and require `gofmt -l` to emit no output **for the Go files changed by this shipment** (the three new guard test files under `tests/integration/`). A repo-wide `gofmt -l .` is intentionally NOT required as a pass criterion: the repository carries pre-existing formatting debt in files unrelated to this work (26 files at time of writing), and forcing a repo-wide cleanup here would violate width isolation and silently expand scope. Quality-gate honesty: this criterion verifies the changed files are formatted, not that unrelated pre-existing debt has been remediated.
7. Run `go run ./cmd/backlogit docs lint` in a clean checkout.
8. Verify protected scratch remains untracked, untouched, and unstaged.

## Risks and Mitigations

- Git is required; repository CI already depends on it.
- Production Scope changes intentionally affect the live and hermetic corpus.
- Extension keys must remain producer-owned; the guard asserts docline tolerates and preserves them rather than validating or emitting them.
- A deliberate schema-version upgrade changes test and corpus together.

## Constitution Check

- **I:** only bounded Go integration tests and metadata are planned.
- **II (NON-NEGOTIABLE):** live RED precedes backfill, while hermetic negatives remain active.
- **III/IV:** Git inventory and production `internal/docline.Scope()` bound every read and exclude scratch; this guard adds no realpath/symlink filesystem containment and claims none.
- **V:** failures identify exact paths and fields.
- **VI:** three one-file guard tasks each contain at most three scenario groups and fewer than five functions.
- **VII:** no scratch deletion, edit, or staging is authorized.
- **VIII:** open extension compatibility receives a focused one-file guard.
- **IX:** committed source metadata remains the guarded truth.
- **X:** split tests are easier to execute and diagnose.
- **XI:** Stage does not merge or ship.

No constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: PASS

**Formal plan-review provenance:** RE-RUN on 2026-07-15 by the Stage agent following the `plan-review` skill against these exact final plan bytes after the PR #241 scope reclassification (D4 filesystem containment → open extension-key compatibility). Cross-model reviewer invocation was unavailable in this environment; per the skill's explicit fallback ("If cross-model invocation is not available, run all personas with the caller's model. Multi-model is preferred but not blocking."), all reviewer personas were executed with the caller's model. This is a single-model multi-persona review, disclosed as such — not a manufactured or waiver-based pass.

**Reviewer personas executed:**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | No violation. Principle II (live RED before backfill, hermetic negatives persist), VI/Task Granularity (three one-file guard tasks, ≤3 scenario groups, <5 functions), V (path/field-specific failures) satisfied. `107.009-T` no longer asserts production filesystem containment it cannot deliver, so the III/IV overreach is removed. |
| Go Reviewer | always-on | Integration tests placed in `tests/integration/`, reuse production `internal/docline.Scope()` and existing YAML decoding (no duplication), table-driven groups. The reworked `107.009-T` adds no production code and asserts only base-contract tolerance of extension keys. No P0/P1. |
| Scope Boundary Auditor | always-on | Scope creep REMOVED: the prior `107.009-T` implied a symlink/realpath containment guarantee that production docline does not provide (lexical `core.SafeResolve`; `ApplyMigration` adds no realpath containment), which would have forced either test-local behavior divergence or a hidden production containment task. Reclassifying to open extension-key compatibility aligns the task with the base-contract/extension boundary this shipment actually owns. Production docline validation and JSON schemas unchanged; protected scratch untouched; decoupled from formal-gate governance (105-F/106-F). No P0/P1. |
| Learnings Researcher | always-on | Consistent with prior docline standardization/doctor-hardening; the open/extensible base-contract framing matches the shared-frontmatter-codec direction (producers own extension semantics, docline tolerates them). No contradiction of a past resolution. No P0/P1. |
| Architecture Strategist | always-on | Cohesive three-milestone split (live corpus, value semantics, open extension-key compatibility) with a clean dependency chain on `107.001-T` RED; no coupling to formal-gate work. The extension-compat guard directly encodes the authoritative base-class/extension model. No P0/P1. |
| Security Lens Reviewer | not triggered (containment/trust-boundary claim removed; task is now test-only base-contract tolerance with no filesystem trust boundary) | — |
| Agent-Native Parity Reviewer | not triggered (test-only + metadata; no MCP or agent-facing surface) | — |

**Findings disposition:** P0 = 0, P1 = 0, P2 = 0, P3 = advisory only (exact test helper signatures left to implementation, appropriate at planning granularity). The prior latent P1 — `107.009-T` promising external-target rejection that production docline cannot deliver from test code — is resolved by the reclassification. No finding blocks harvest.

**Plan hardening:** Not required. The plan is additive test code plus canonical-metadata backfill with no production-logic, schema, containment, or CLI-distribution changes and no destructive operations, so it does not exhibit the elevated-blast-radius hardening signals that would gate on `plan-harden`.

**Factual verification supporting the gate:**

- `internal/core/workspace.go` `SafeResolve` performs lexical `filepath.Abs`/`Clean` prefix containment only (no realpath/symlink resolution); `internal/docline/service.go` `ApplyMigration` explicitly states it "does not add symlink-based realpath containment" — confirming the removed containment claim was unimplementable from a test-only task.
- No `size_source`/`size_ruleset_version` keys exist anywhere in `internal/`, `schemas/`, or `.backlogit/header-def.yaml`, so the extension-compat guard exercises genuinely optional producer-owned keys against the base contract.
- Shipment `095-S` (feature `107-F` plus nine tasks `107.001-T`–`107.009-T`) matches the plan's task map, with backfill and the extension-compat guard depending on `107.001-T`.

**Disposition:** Gate PASS (re-run). Shipment `095-S` remains queued and ready for Ship to claim. This docline plan stays decoupled from the formal-gate governance work and from the size extension-contract spike (096-S); the formal-gate implementation (`106-F`) stays blocked behind the time-boxed architecture spike (`105.001-T`) and is unaffected by this PASS.
