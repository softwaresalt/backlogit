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

**Base-contract / extension ownership (authoritative for PR #241).** Docline owns only the base Markdown/frontmatter ingestion contract and its compatibility rules. That base contract is open/extensible: producer-specific optional properties (for example the backlogit-owned `size`, `size_source`, `size_ruleset_version`) may be added and MUST be preserved unchanged at the top level. The **current production seam contradicts this**: `internal/docline/normalize.go` folds every non-contract top-level key into the `docline` namespace (`foldUnderDocline`), and `internal/docline/policy.go` defines a **closed** `contractFields` set that drives that relocation. This plan's `107.009-T` therefore becomes an **executable production-contract task**, and the correction is honest about the full seam: `Normalize` serializes through `BaseFrontmatter.ToMap()` in `internal/docline/frontmatter.go`, which emits **only** the base-contract fields plus the `docline` namespace, so even after the fold is removed the unknown top-level keys are **dropped on serialization** unless `frontmatter.go` gains a carrier for them. `107.009-T` therefore changes **both** production seams — `internal/docline/normalize.go` (stop folding non-contract top-level keys under `docline`) **and** `internal/docline/frontmatter.go` (capture unknown top-level, non-`docline` keys in `FromMap` into a carrier on `BaseFrontmatter` and re-emit them unchanged in `ToMap`) — so unknown top-level extension properties pass through **in place** (docline validates/normalizes only its own base fields and must not relocate extension fields under `docline`), and updates the contradictory normalization unit tests. The size-extension regression is split into a **RED-first integration task `107.010-T`** (`tests/integration/docline_extension_compat_test.go`), observed failing before the seam change; `107.009-T` depends on `107.010-T` and turns it GREEN. Pass-through is **docline** base-contract behavior; `size` semantics (validation, defaults, provenance, composition) remain **backlogit-owned** and are out of scope here. This plan adds no production filesystem containment, which is unrelated to the base-contract/extension boundary.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| D1 | Guard live committed corpus | `107.001-T` uses Git plus production Scope and exact values. |
| D2 | Preserve value negatives | `107.008-T` owns bounded hermetic value/type/malformed cases. |
| D3 | Prove untracked exclusion | `107.008-T` compares invalid tracked and untracked fixtures. |
| D4 | Preserve extension keys in place (production seam) | `107.009-T` changes **both** `internal/docline/normalize.go` (stop folding non-contract top-level keys under `docline`) and `internal/docline/frontmatter.go` (`BaseFrontmatter` carries and re-emits unknown top-level keys that `ToMap` would otherwise drop) so unknown top-level extension keys (backlogit-owned `size`, `size_source`, `size_ruleset_version`) pass through **unchanged in place**, and updates the contradictory normalization unit tests; the RED-first size-extension regression is `107.010-T`. Docline never owns/validates/emits extension semantics. |
| D4a | Observe extension RED before the seam change | `107.010-T` (`tests/integration/docline_extension_compat_test.go`) fails at current HEAD (keys dropped) and is the dependency gate for `107.009-T`. |
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

### `107.010-T` — RED-first extension pass-through regression (test-only)

**File:** `tests/integration/docline_extension_compat_test.go`

RED-first integration regression written **before** the `107.009-T` seam change. Representative backlogit-owned extension keys (`size`, `size_source`, `size_ruleset_version`) added to an in-scope doc must:

* stay **top-level** and **byte-unchanged** through `Normalize` (never relocated under `docline`, never dropped on serialization);
* be **never** defaulted, rewritten, validated, or emitted by docline;
* survive a lint/migration round-trip without body mutation; the transform stays idempotent.

These assertions **fail at current HEAD** for two reasons the plan states honestly: `normalize.go` folds non-contract top-level keys under `docline`, **and** `BaseFrontmatter.ToMap()` (`frontmatter.go`) emits only base-contract fields plus `docline`, dropping unknown keys on serialization. `107.010-T` is the dependency gate for `107.009-T`; test-only, no production change.

### `107.009-T` — Preserve unknown top-level extension keys in docline normalization (production pass-through)

**Files (production + tests):** `internal/docline/normalize.go` (stop folding non-contract keys under `docline`), `internal/docline/frontmatter.go` (`BaseFrontmatter` carrier + `FromMap`/`ToMap` re-emission of unknown top-level keys), `internal/docline/normalize_test.go` (update contradictory assertions). `internal/docline/policy_test.go` is **not** changed. The size-extension regression lives in `107.010-T`, not here.

**Production-contract correction (PR #241):** the current seam contradicts the authoritative base/extension contract in **two** places. (1) `Normalize` folds every non-contract top-level key under the `docline` namespace via `foldUnderDocline`, and `policy.go` defines a **closed** `contractFields` set that drives that relocation. (2) `Normalize` serializes through `BaseFrontmatter.ToMap()`, which emits **only** base-contract fields plus `docline`, so unknown top-level keys are **dropped on serialization** — `BaseFrontmatter` has no carrier for them. The authoritative contract requires docline to preserve unknown top-level extension properties **unchanged in place** and to **never relocate** them under `docline`, validating/normalizing only its own base fields.

This is an **executable ≤2h production-contract task**, gated behind the `107.010-T` RED:

1. **Serialization seam** — `internal/docline/frontmatter.go`: `BaseFrontmatter` gains a carrier for non-contract, non-`docline` top-level keys; `FromMap` captures them and `ToMap` re-emits them unchanged at the top level (without this, removing the fold alone would still drop the keys).
2. **Normalization seam** — `internal/docline/normalize.go`: non-contract top-level keys pass through **unchanged in place** instead of being folded under `docline`. Docline still reads/emits its own `docline` namespace and base contract fields; body bytes and idempotency (`Normalize(Normalize(x)) == Normalize(x)`) are preserved.
3. **Update contradictory tests** — `TestNormalize_ContractSurfaceHoldsOnlyContractKeys` (asserts every top-level key is a contract field) and `TestNormalize_FoldsLegacyTypeUnderDocline` (asserts non-contract keys are folded away) are updated to assert top-level pass-through. `policy_test.go` `TestIsContractField` **remains valid and unchanged** (the closed contract set still governs which base fields docline normalizes; it no longer governs relocation of unknown keys).

Ownership stays explicit: top-level pass-through is **docline** base-contract behavior; `size` validation, defaults, provenance, and composition remain **backlogit-owned** and are out of scope here. Test-first: the `107.010-T` failing assertions are observed before the seam change. No schema or CLI-distribution changes.

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

Observe `107.001-T` RED first. The six metadata backfills (`107.002-T`–`107.007-T`) and the hermetic value fixtures (`107.008-T`) then proceed in parallel using the shared inventory/value helper; after backfills, `107.001-T` returns GREEN. The extension pass-through work is split test-first: `107.010-T` (`tests/integration/docline_extension_compat_test.go`) is a **RED-first integration regression** observed failing at HEAD, and `107.009-T` is the **production-contract** task (docline normalization seam in `normalize.go` **plus** the serialization seam in `frontmatter.go`) that **depends on `107.010-T`** and turns it GREEN. `107.009-T` does **not** share the corpus inventory helper and is independently verifiable through the `internal/docline` unit tests plus the `107.010-T` integration regression.

## TDD and Quality-gate Sequence

1. Add the live test and record nine path-specific omissions.
2. Add hermetic value fixtures and run all three groups independently.
3. For the extension pass-through, first add the `107.010-T` RED integration regression (`tests/integration/docline_extension_compat_test.go`) and observe it **FAIL at HEAD** (unknown top-level keys are folded under `docline` by `normalize.go` and dropped by `BaseFrontmatter.ToMap()`); then in `107.009-T` change **both** the serialization seam (`internal/docline/frontmatter.go`: carry and re-emit unknown top-level keys) and the normalization seam (`internal/docline/normalize.go`: stop folding non-contract top-level keys under `docline`), and update the two contradictory unit tests in `internal/docline/normalize_test.go` to expect pass-through.
4. Apply six bounded backfill tasks without body changes.
5. Require the live corpus, hermetic value, and normalization/extension tests all GREEN.
6. Run `go test ./...`, `go vet ./...`, `golangci-lint run`, and require `gofmt -l` to emit no output **for the Go files changed by this shipment** (the three new guard/regression test files under `tests/integration/` — `docline_soft_keys_test.go`, `docline_soft_key_values_test.go`, `docline_extension_compat_test.go`; the changed production files `internal/docline/normalize.go` and `internal/docline/frontmatter.go`; and the updated `internal/docline/normalize_test.go` — `internal/docline/policy_test.go` is **not** changed). A repo-wide `gofmt -l .` is intentionally NOT required as a pass criterion: the repository carries pre-existing formatting debt in files unrelated to this work (26 files at time of writing), and forcing a repo-wide cleanup here would violate width isolation and silently expand scope. Quality-gate honesty: this criterion verifies the changed files are formatted, not that unrelated pre-existing debt has been remediated.
7. Run `go run ./cmd/backlogit docs lint` in a clean checkout.
8. Verify protected scratch remains untracked, untouched, and unstaged.

## Risks and Mitigations

- Git is required; repository CI already depends on it.
- Production Scope changes intentionally affect the live and hermetic corpus.
- The `107.009-T` change alters where non-contract top-level keys land (preserved in place rather than folded under `docline`) **and** adds a `BaseFrontmatter` carrier so `ToMap` re-emits them instead of dropping them. This is a production behavior change across two seams (`normalize.go`, `frontmatter.go`): the two normalization unit tests that encode the old fold behavior are updated in the same task, and the RED-first integration regression `107.010-T` pins the extension-key contract. The change is strictly bounded to `internal/docline`, is idempotent, and preserves body bytes; it makes no schema or CLI-distribution change.
- Extension keys must remain producer-owned; docline preserves them in place and never validates, defaults, or emits them.
- A deliberate schema-version upgrade changes test and corpus together.

## Constitution Check

- **I:** bounded Go changes only — the normalization seam (`internal/docline/normalize.go`) and the serialization seam (`internal/docline/frontmatter.go`), the two contradictory unit tests (`internal/docline/normalize_test.go`), the RED-first integration regression (`107.010-T`), plus canonical-metadata backfills.
- **II (NON-NEGOTIABLE):** live RED precedes backfill; for `107.009-T`, the failing pass-through/regression assertions precede the seam change; hermetic negatives remain active.
- **III/IV:** Git inventory and production `internal/docline.Scope()` bound every read and exclude scratch; this plan adds no realpath/symlink filesystem containment and claims none.
- **V:** failures identify exact paths, fields, and extension keys.
- **VI:** the metadata/value guards stay one-file with ≤3 scenario groups; `107.009-T` is a single-concern top-level pass-through change across the normalization and serialization seams in `internal/docline` with its co-located unit-test updates, and the extension integration regression is the separate width-isolated `107.010-T` (`tests/integration/`).
- **VII:** no scratch deletion, edit, or staging is authorized.
- **VIII:** the extension pass-through contract receives a focused production change plus regression coverage.
- **IX:** committed source metadata remains the guarded truth.
- **X:** split tests are easier to execute and diagnose.
- **XI:** Stage does not merge or ship.

No constitutional violation, waiver, or exception is planned.

## Plan Review

### Gate Decision: PASS

**Formal plan-review provenance:** RE-RUN on 2026-07-15 by the Stage agent following the `plan-review` skill against these exact final plan bytes after the PR #241 correction. The correction reclassified `107.009-T` from a test-only tolerance guard into an **executable production-contract task** and — following a LOCAL review of the current code at HEAD `ebf0ebe` — was further corrected to state the **full** production seam honestly: `Normalize` serializes through `BaseFrontmatter.ToMap()` in `internal/docline/frontmatter.go`, which emits only base-contract fields plus the `docline` namespace, so unknown top-level keys are **dropped on serialization** unless `frontmatter.go` gains a carrier for them. `107.009-T` therefore changes **both** `internal/docline/normalize.go` and `internal/docline/frontmatter.go`, and the size-extension regression is split into the **RED-first integration task `107.010-T`** (a dependency of `107.009-T`); `internal/docline/policy_test.go` is not changed. Cross-model reviewer invocation was unavailable in this environment; per the skill's explicit fallback ("If cross-model invocation is not available, run all personas with the caller's model. Multi-model is preferred but not blocking."), all reviewer personas were executed with the caller's model. This is a single-model multi-persona review, disclosed as such — not a manufactured or waiver-based pass.

**Reviewer personas executed:**

| Persona | Trigger | Result |
|---|---|---|
| Constitution Reviewer | always-on | No violation. Principle II satisfied for both the corpus guards (live RED before backfill, hermetic negatives persist) and the extension work (`107.010-T` RED integration regression observed failing before the `107.009-T` seam change; `107.009-T` depends on it). Task Granularity/VI holds: `107.009-T` is a single-concern top-level pass-through change across the `normalize.go` and `frontmatter.go` seams with co-located unit-test updates, width-isolated to `internal/docline`; the integration regression is the separate `107.010-T`. No III/IV containment overreach remains (no filesystem containment is added or claimed). No P0/P1. |
| Go Reviewer | always-on | The change removes the `foldUnderDocline` relocation of non-contract top-level keys in `internal/docline/normalize.go` **and** adds a `BaseFrontmatter` carrier in `internal/docline/frontmatter.go` so `FromMap` captures and `ToMap` re-emits unknown top-level keys (verified necessary: `ToMap` currently emits only base-contract fields plus `docline`, so removing the fold alone would still drop the keys). `docline` namespace read/emit and base-field defaulting are unchanged; body bytes and idempotency preserved. The two contradictory tests (`TestNormalize_ContractSurfaceHoldsOnlyContractKeys`, `TestNormalize_FoldsLegacyTypeUnderDocline`) are updated in `normalize_test.go`; `TestIsContractField` in `policy_test.go` stays valid and unchanged. The RED integration regression is `107.010-T` under `tests/integration/`. Errors and idempotency contracts respected. No P0/P1. |
| Scope Boundary Auditor | always-on | The task is now an honest production change confined to `internal/docline` (normalization + serialization seams) plus its co-located unit tests; the integration regression is the width-isolated `107.010-T`. No schema, header-def, or CLI-distribution change; no filesystem containment; protected scratch untouched; decoupled from formal-gate governance (105-F/106-F) and the size spike (096-S). Ownership is explicit: pass-through is docline base-contract behavior; size validation/defaults/provenance/composition stay backlogit-owned and out of scope. No P0/P1. |
| Learnings Researcher | always-on | Consistent with the shared-frontmatter-codec direction (producers own extension semantics; docline owns only the base contract). The correction resolves a real contradiction — the prior plan asserted docline already "tolerates/preserves" extension keys while production actually folds them under `docline`. No contradiction of a past resolution. No P0/P1. |
| Architecture Strategist | always-on | The change makes production behavior match the authoritative base/extension model rather than encoding the mismatch in a test. Two clean seams (`normalize.go` stops relocating unknown keys; `frontmatter.go` carries and re-emits them); the closed `contractFields` set is retained for base-field normalization but no longer drives relocation of unknown keys. No coupling to formal-gate or size work. No P0/P1. |
| Security Lens Reviewer | not triggered (no filesystem trust boundary or containment claim; pass-through is strictly a frontmatter-shape change) | — |
| Agent-Native Parity Reviewer | not triggered (no MCP tool or agent-facing surface changes; docline normalization is not an agent command surface) | — |

**Findings disposition:** P0 = 0, P1 = 0, P2 = 1 (advisory, dispositioned), P3 = advisory. The **prior latent P1 raised by LOCAL review** — the plan (and task) scoping the production change to `normalize.go` plus tests while `Normalize` serializes through `BaseFrontmatter.ToMap()` in `internal/docline/frontmatter.go`, which emits only base-contract fields plus `docline`, so unknown top-level keys could not survive without changing that serialization seam — is **resolved**: `107.009-T` now explicitly includes `internal/docline/frontmatter.go` (carrier + `FromMap`/`ToMap` re-emission), the size-extension regression is split into the RED-first `107.010-T` (a dependency of `107.009-T`), and `policy_test.go` is removed from the changed-file set (`TestIsContractField` verified still valid). **P2 (downstream-consumer behavior change):** relocating fewer keys under `docline` (keeping them top-level) is a behavior change any consumer that reads `docline.<key>` could observe. Dispositioned: the change is bounded to `internal/docline`, idempotent, body-preserving, and covered by the updated normalization unit tests plus the `107.010-T` integration regression; consumers ingest the stable base contract and the extension keys are backlogit-owned, so no consumer contract for those keys exists to break. No finding blocks harvest.

**Plan hardening:** Evaluated (not merely skipped). `107.009-T` is a production-logic change across two `internal/docline` seams, so the elevated-blast-radius signals were checked explicitly: it does **not** touch JSON schemas, CLI distribution, or multiple template families, and performs no destructive operations, so `plan-harden` is not mandated. The one non-trivial signal — a shared normalization/serialization codec consumed by other producers — is mitigated in-plan by the updated unit tests, the `107.010-T` integration regression, idempotency/body-preservation invariants, and the width-isolation boundary (`internal/docline` only). The change is recorded honestly as a production change touching both `normalize.go` and `frontmatter.go`, not as "additive test code."

**Factual verification supporting the gate:**

- `internal/docline/normalize.go` currently folds every non-contract top-level key under the `docline` namespace via `foldUnderDocline` (the "move, never drop" loop), and `internal/docline/policy.go` defines a closed `contractFields` set — confirming the production seam contradicts the authoritative preserve-in-place contract the plan now corrects.
- `internal/docline/frontmatter.go` `BaseFrontmatter.ToMap()` emits **only** the base-contract fields (title, source, ingested_at, doc_type, description, chunk_strategy, schema_version, optional content_sha256/source_path) plus the `docline` namespace, and `BaseFrontmatter` has **no field** for unknown top-level keys — confirming that removing the fold alone would still drop extension keys on serialization, which is why `107.009-T` must change `frontmatter.go` too.
- `internal/docline/normalize_test.go` `TestNormalize_ContractSurfaceHoldsOnlyContractKeys` asserts every top-level key is a contract field, and `TestNormalize_FoldsLegacyTypeUnderDocline` asserts non-contract keys are folded away — these are the contradictory unit tests the task updates; `internal/docline/policy_test.go` `TestIsContractField` remains valid and is not changed.
- No `size_source`/`size_ruleset_version` keys exist anywhere in `internal/`, `schemas/`, or `.backlogit/header-def.yaml`, so the `107.010-T` regression exercises genuinely optional backlogit-owned extension keys against the base contract.
- Shipment `095-S` (feature `107-F` plus ten tasks `107.001-T`–`107.010-T`) matches the plan's task map, with backfills, the `107.010-T` RED integration regression, and the `107.009-T` production change (depending on `107.010-T`) sequenced after the `107.001-T` RED observation.

**Disposition:** Gate PASS (re-run) — the plan is executable and honest: `107.009-T` is a bounded, testable production-contract change, not a test-only task asserting behavior production does not have. Shipment `095-S` remains queued and ready for Ship to claim. This docline plan stays decoupled from the formal-gate governance work and from the size extension-contract spike (096-S); the formal-gate implementation (`106-F`) stays blocked behind the time-boxed architecture spike (`105.001-T`) and is unaffected by this PASS.
