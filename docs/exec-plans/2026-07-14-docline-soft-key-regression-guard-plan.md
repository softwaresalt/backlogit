---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Bounded docline soft-key regression guard implementation plan'
source: docs/exec-plans/2026-07-14-docline-soft-key-regression-guard-plan.md
doc_type: plan
description: 'Test-first plan separating live tracked corpus, hermetic value semantics, and a production docline normalization pass-through change (preserve unknown top-level extension keys in place) before metadata backfill.'
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

**Base-contract / extension ownership (authoritative for PR #241).** Docline owns only the base Markdown/frontmatter ingestion contract and its compatibility rules. That base contract is open/extensible: producer-specific optional properties (for example the backlogit-owned `size`, `size_source`, `size_ruleset_version`) may be added and MUST be preserved unchanged at the top level. The **current production seam contradicts this**: `internal/docline/normalize.go` folds every non-contract top-level key into the `docline` namespace (`foldUnderDocline`), and `internal/docline/policy.go` defines a **closed** `contractFields` set that drives that relocation. This plan's `107.009-T` therefore becomes an **executable production-contract task**: change the normalization seam so unknown top-level extension properties pass through **in place** (docline validates/normalizes only its own base fields and must not relocate extension fields under `docline`), update the contradictory normalization tests, and add size-extension regression coverage. Pass-through is **docline** base-contract behavior; `size` semantics (validation, defaults, provenance, composition) remain **backlogit-owned** and are out of scope here. This plan adds no production filesystem containment, which is unrelated to the base-contract/extension boundary.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| D1 | Guard live committed corpus | `107.001-T` uses Git plus production Scope and exact values. |
| D2 | Preserve value negatives | `107.008-T` owns bounded hermetic value/type/malformed cases. |
| D3 | Prove untracked exclusion | `107.008-T` compares invalid tracked and untracked fixtures. |
| D4 | Preserve extension keys in place (production seam) | `107.009-T` changes `internal/docline/normalize.go` so unknown top-level extension keys (backlogit-owned `size`, `size_source`, `size_ruleset_version`) pass through **unchanged in place** instead of being folded under `docline`, updates the contradictory normalization tests, and adds size-extension regression coverage; docline never owns/validates/emits extension semantics. |
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

### `107.009-T` — Preserve unknown top-level extension keys in docline normalization (production pass-through)

**Files (production + tests):** `internal/docline/normalize.go` (seam change), `internal/docline/normalize_test.go` and `internal/docline/policy_test.go` (update contradictory assertions), `tests/integration/docline_extension_compat_test.go` (size-extension regression).

**Production-contract correction (PR #241):** the current seam contradicts the authoritative base/extension contract. `Normalize` folds every non-contract top-level key under the `docline` namespace via `foldUnderDocline`, and `policy.go` defines a **closed** `contractFields` set that drives that relocation. The authoritative contract requires docline to preserve unknown top-level extension properties **unchanged in place** and to **never relocate** them under `docline`, validating/normalizing only its own base fields.

This is an **executable ≤2h production-contract task**, not a test-only guard:

1. **Seam change** — `internal/docline/normalize.go`: non-contract top-level keys pass through **unchanged in place** instead of being folded under `docline`. Docline still reads/emits its own `docline` namespace and base contract fields; body bytes and idempotency (`Normalize(Normalize(x)) == Normalize(x)`) are preserved.
2. **Update contradictory tests** — `TestNormalize_ContractSurfaceHoldsOnlyContractKeys` (asserts every top-level key is a contract field) and `TestNormalize_FoldsLegacyTypeUnderDocline` (asserts non-contract keys are folded away) are updated to assert top-level pass-through. `policy_test.go` `TestIsContractField` remains valid (the closed contract set still governs which base fields docline normalizes; it no longer governs relocation of unknown keys).
3. **Size-extension regression** — representative backlogit-owned extension keys (`size`, `size_source`, `size_ruleset_version`) added to an in-scope doc:
   * stay **top-level** and **byte-unchanged** through `Normalize` (never relocated under `docline`);
   * are **never** defaulted, rewritten, validated, or emitted by docline;
   * survive a lint/migration round-trip without body mutation; the transform stays idempotent.

Ownership stays explicit: top-level pass-through is **docline** base-contract behavior; `size` validation, defaults, provenance, and composition remain **backlogit-owned** and are out of scope here. Test-first: the failing pass-through/regression assertions are written before the seam change. No schema or CLI-distribution changes.

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

Observe `107.001-T` RED first. The six metadata backfills (`107.002-T`–`107.007-T`) and the hermetic value fixtures (`107.008-T`) then proceed in parallel using the shared inventory/value helper; after backfills, `107.001-T` returns GREEN. `107.009-T` is a **production-contract** task (docline normalization seam) that is sequenced after the `107.001-T` RED observation to keep the RED-first discipline, but it does **not** share the corpus inventory helper and is independently verifiable through the `internal/docline` unit tests plus the size-extension regression.

## TDD and Quality-gate Sequence

1. Add the live test and record nine path-specific omissions.
2. Add hermetic value fixtures and run all three groups independently.
3. For `107.009-T`, write the failing pass-through/regression assertions first (size extension keys must stay top-level and unchanged; the two contradictory normalization tests are updated to expect pass-through), then change the `internal/docline/normalize.go` seam so unknown top-level keys are preserved in place instead of folded under `docline`.
4. Apply six bounded backfill tasks without body changes.
5. Require the live corpus, hermetic value, and normalization/extension tests all GREEN.
6. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and require `gofmt -l` to emit no output **for the Go files changed by this shipment** (the two new guard test files under `tests/integration/`, the changed `internal/docline/normalize.go`, and the updated `internal/docline/*_test.go`). A repo-wide `gofmt -l .` is intentionally NOT required as a pass criterion: the repository carries pre-existing formatting debt in files unrelated to this work (26 files at time of writing), and forcing a repo-wide cleanup here would violate width isolation and silently expand scope. Quality-gate honesty: this criterion verifies the changed files are formatted, not that unrelated pre-existing debt has been remediated.
7. Run `go run ./cmd/backlogit docs lint` in a clean checkout.
8. Verify protected scratch remains untracked, untouched, and unstaged.

## Risks and Mitigations

- Git is required; repository CI already depends on it.
- Production Scope changes intentionally affect the live and hermetic corpus.
- The `107.009-T` seam change alters where non-contract top-level keys land (preserved in place rather than folded under `docline`). This is a production behavior change: the two normalization tests that encode the old fold behavior are updated in the same task, and the size-extension regression pins the extension-key contract. The change is strictly bounded to `internal/docline` normalization, is idempotent, and preserves body bytes; it makes no schema or CLI-distribution change.
- Extension keys must remain producer-owned; docline preserves them in place and never validates, defaults, or emits them.
- A deliberate schema-version upgrade changes test and corpus together.

## Constitution Check

- **I:** bounded Go changes only — one normalization seam (`internal/docline/normalize.go`), the two contradictory unit tests, one integration regression file, plus canonical-metadata backfills.
- **II (NON-NEGOTIABLE):** live RED precedes backfill; for `107.009-T`, the failing pass-through/regression assertions precede the seam change; hermetic negatives remain active.
- **III/IV:** Git inventory and production `internal/docline.Scope()` bound every read and exclude scratch; this plan adds no realpath/symlink filesystem containment and claims none.
- **V:** failures identify exact paths, fields, and extension keys.
- **VI:** the metadata/value guards stay one-file with ≤3 scenario groups; `107.009-T` is a single-concern normalization seam change with its co-located test updates and one regression file — width-isolated to `internal/docline`.
- **VII:** no scratch deletion, edit, or staging is authorized.
- **VIII:** the extension pass-through contract receives a focused production change plus regression coverage.
- **IX:** committed source metadata remains the guarded truth.
- **X:** split tests are easier to execute and diagnose.
- **XI:** Stage does not merge or ship.

No constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: PASS

**Formal plan-review provenance:** RE-RUN on 2026-07-15 by the Stage agent following the `plan-review` skill against these exact final plan bytes after the PR #241 correction reclassifying `107.009-T` from a test-only tolerance guard into an **executable production-contract task** (change the docline normalization seam to preserve unknown top-level extension keys in place). Cross-model reviewer invocation was unavailable in this environment; per the skill's explicit fallback ("If cross-model invocation is not available, run all personas with the caller's model. Multi-model is preferred but not blocking."), all reviewer personas were executed with the caller's model. This is a single-model multi-persona review, disclosed as such — not a manufactured or waiver-based pass.

**Reviewer personas executed:**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | No violation. Principle II satisfied for both the corpus guards (live RED before backfill, hermetic negatives persist) and `107.009-T` (failing pass-through/regression assertions precede the seam change). Task Granularity/VI holds: `107.009-T` is a single-concern normalization seam change with co-located test updates and one regression file, width-isolated to `internal/docline`. No III/IV containment overreach remains (no filesystem containment is added or claimed). No P0/P1. |
| Go Reviewer | always-on | The seam change removes the `foldUnderDocline` relocation of non-contract top-level keys in `internal/docline/normalize.go`, preserving them in place; `docline` namespace read/emit and base-field defaulting are unchanged; body bytes and idempotency preserved. The two contradictory tests (`TestNormalize_ContractSurfaceHoldsOnlyContractKeys`, `TestNormalize_FoldsLegacyTypeUnderDocline`) are updated in the same task; `TestIsContractField` stays valid. Integration regression added under `tests/integration/`. Errors and idempotency contracts respected. No P0/P1. |
| Scope Boundary Auditor | always-on | The task is now an honest production change confined to `internal/docline` normalization plus its co-located tests and one integration regression. No schema, header-def, or CLI-distribution change; no filesystem containment; protected scratch untouched; decoupled from formal-gate governance (105-F/106-F) and the size spike (096-S). Ownership is explicit: pass-through is docline base-contract behavior; size validation/defaults/provenance/composition stay backlogit-owned and out of scope. No P0/P1. |
| Learnings Researcher | always-on | Consistent with the shared-frontmatter-codec direction (producers own extension semantics; docline owns only the base contract). The correction resolves a real contradiction — the prior plan asserted docline already "tolerates/preserves" extension keys while production actually folds them under `docline`. No contradiction of a past resolution. No P0/P1. |
| Architecture Strategist | always-on | The change makes production behavior match the authoritative base/extension model rather than encoding the mismatch in a test. Clean seam (`normalize.go`); the closed `contractFields` set is retained for base-field normalization but no longer drives relocation of unknown keys. No coupling to formal-gate or size work. No P0/P1. |
| Security Lens Reviewer | not triggered (no filesystem trust boundary or containment claim; pass-through is strictly a frontmatter-shape change) | — |
| Agent-Native Parity Reviewer | not triggered (no MCP tool or agent-facing surface changes; docline normalization is not an agent command surface) | — |

**Findings disposition:** P0 = 0, P1 = 0, P2 = 1 (advisory, dispositioned), P3 = advisory. The prior latent P1 — the plan asserting docline already preserves extension keys when production folds them under `docline` — is resolved by making the production seam actually preserve them and updating the contradictory tests. **P2 (downstream-consumer behavior change):** relocating fewer keys under `docline` (keeping them top-level) is a behavior change any consumer that reads `docline.<key>` could observe. Dispositioned: the change is bounded to `internal/docline`, idempotent, body-preserving, and covered by the updated normalization tests plus the size-extension regression; consumers ingest the stable base contract and the extension keys are backlogit-owned, so no consumer contract for those keys exists to break. No finding blocks harvest.

**Plan hardening:** Evaluated (not merely skipped). `107.009-T` is now a production-logic change, so the elevated-blast-radius signals were checked explicitly: it does **not** touch JSON schemas, CLI distribution, or multiple template families, and performs no destructive operations, so `plan-harden` is not mandated. The one non-trivial signal — a shared normalization codec consumed by other producers — is mitigated in-plan by the updated unit tests, the size-extension regression, idempotency/body-preservation invariants, and the width-isolation boundary (`internal/docline` only). The change is recorded honestly as a production change, not as "additive test code."

**Factual verification supporting the gate:**

- `internal/docline/normalize.go` currently folds every non-contract top-level key under the `docline` namespace via `foldUnderDocline` (the "move, never drop" loop), and `internal/docline/policy.go` defines a closed `contractFields` set — confirming the production seam contradicts the authoritative preserve-in-place contract the plan now corrects.
- `internal/docline/normalize_test.go` `TestNormalize_ContractSurfaceHoldsOnlyContractKeys` asserts every top-level key is a contract field, and `TestNormalize_FoldsLegacyTypeUnderDocline` asserts non-contract keys are folded away — these are the contradictory tests the task updates.
- No `size_source`/`size_ruleset_version` keys exist anywhere in `internal/`, `schemas/`, or `.backlogit/header-def.yaml`, so the regression exercises genuinely optional backlogit-owned extension keys against the base contract.
- Shipment `095-S` (feature `107-F` plus nine tasks `107.001-T`–`107.009-T`) matches the plan's task map, with backfills and `107.009-T` sequenced after the `107.001-T` RED observation.

**Disposition:** Gate PASS (re-run) — the plan is executable and honest: `107.009-T` is a bounded, testable production-contract change, not a test-only task asserting behavior production does not have. Shipment `095-S` remains queued and ready for Ship to claim. This docline plan stays decoupled from the formal-gate governance work and from the size extension-contract spike (096-S); the formal-gate implementation (`106-F`) stays blocked behind the time-boxed architecture spike (`105.001-T`) and is unaffected by this PASS.
