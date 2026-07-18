---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Post-merge closure for shipment 095-S — live-corpus docline soft-key regression guard, hermetic value-semantics test, 12-doc corpus backfill, and CI docs-lint enforcement so docs-only PRs cannot bypass the guard. Merged via PR #250 (merge commit ede77ed) under P-017 dark-factory mode."
doc_type: closure
docline:
  ms.date: 2026-07-18T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-18-095-S-docline-soft-key-guard-closure.md
title: "095-S docline soft-key regression guard closure"
---

## Scope

Post-merge closure for shipment **095-S** (Guard tracked docline soft keys),
Model-A reduced scope — members **107.001-T through 107.008-T** under feature
**107-F**. The scope seams 107.009–012-T (retired Model-B) shipped separately
via 097-S and were already archived. Merged via **PR #250**, merge commit
`ede77ed3cd01ea06eb998cd8d0a694f393fcd9aa`, at 2026-07-18T03:05:58Z.

## What shipped

| Deliverable | Item | Detail |
|---|---|---|
| Live-corpus guard | 107.001-T | `tests/integration/docline_soft_keys_test.go` — enumerates `git ls-files` filtered through `docline.Scope()`, requires `chunk_strategy: h1-h2-h3` and `schema_version: "1.0"` with correct value + type. Asserts guard literals equal exported `docline.DefaultChunkStrategy` / `DefaultSchemaVersion`. |
| Hermetic value test | 107.008-T | `tests/integration/docline_soft_key_values_test.go` — temp-git value-semantics test. |
| Corpus backfill | 107.002–007-T | 12 tracked docs corrected (3 value fixes `h1-h2` → `h1-h2-h3`, 9 key additions). Frontmatter-only, body-preserving (added lines exclusively soft keys; zero body edits). |
| CI enforcement | (guard hardening) | `.github/workflows/ci.yml` — guard-enforcement step added to the `docs-lint` job, gated on `docline_required`. Closes the docs-only bypass (the Go `test` job is skipped for markdown-only PRs). |
| Authoring guide | (doc) | `docs/docline-frontmatter-authoring-guide.md` — clarifies both keys are required in repository source. |

### Backfill variance (9 → 12)

The 095-S plan estimated 9 missing-key docs. The live corpus had drifted to
include 3 additional docs carrying the wrong `chunk_strategy` value (`h1-h2`),
so the guard correctly named 12 offenders. All 12 were backfilled; the guard
went GREEN.

## Adversarial review (3 cross-model reviewers)

Reviewers: Opus (code-review), GPT-5.6 (code-review, cross-model), GPT-5.6
(rubber-duck design critique). Outcome: **P0=0, P1=0, P2=1 (resolved),
advisory=4**.

| Finding | Reviewer | Severity | Resolution |
|---|---|---|---|
| "Ship it" — no blocking | Opus | — | n/a |
| Filtered subtests passed vacuously via shared parent state | GPT | P2 | **FIXED** — parent computes `files`/`offenders` eagerly; filtered subtest now runs full corpus computation (verified: 0.65s non-vacuous run). |
| Docs-only PRs bypass the live guard (CI skips Go tests for markdown-only changes) | Duck | blocking (single-reviewer, independently verified) | **FIXED** — added the CI docs-lint enforcement step; `ci_compliance_test.go` protects the wiring. |
| Authoring guide / policy contradiction | Duck | advisory | **FIXED** — added required-in-source note. |
| Guard literals can drift from production constants | Duck | advisory | **FIXED** — constant-equality asserts against exported defaults. |
| Hermetic coverage breadth | Duck | advisory | **Residual** — value test covers value semantics on a temp repo; live-corpus guard covers production scope. |
| Scope-mirroring risk | Duck | advisory | **NON-ISSUE** — guard uses exact `rel == f` match, identical to production `inScope`. |

Local review readiness: **READY_WITH_FOLLOWUPS** (zero unresolved P0/P1;
residual-risk note on hermetic coverage breadth). Reviewed HEAD `9f11df7`.

## Quality gates (HEAD 9f11df7)

`go build ./...` ✅ · `go test ./...` ✅ · `go vet ./...` ✅ ·
`golangci-lint run` ✅ · gofmt (LF blobs) ✅ · `docs lint` ✅ (0 violations).

## CI + Copilot review (PR #250)

CI: all checks green (`test` 2m47s, `Docline frontmatter gate`, `CLI Reference
Drift`, `Detect code changes`). Copilot review completed **COMMENTED**, fresh
(review `commit.oid` == HEAD `9f11df7`), reviewed 28/28 files and **generated
no comments**. §1.9 pre-merge gate: Check 1 (no pending review) ✓, Check 2
(freshness) ✓, Check 3 (zero unresolved Copilot threads) ✓.

## GI/GR reconciliation (shipment-reconcile)

- **Pre-mode**: all 8 members (107.001–008-T) present in archive with
  `status: done`. ✓
- **`ship_shipment 095-S`**: `shipment_status: shipped`; archived 095-S + 8
  members + 107-F; `returned_ids: []`. ✓
- **Post-mode**: 095-S present in archive with `status: archived`; absent from
  queue. ✓

## DARK_MODE (P-017)

- `DARK_MODE_ACTIVE` scope: ship 095-S → 096-S → stage-next (operator AFK).
  `merge_approval_pre_authorized=TRUE`, `admin_fallback_pre_authorized=FALSE`.
- `LOCAL_REVIEW_READY`: HEAD `9f11df7`, READY_WITH_FOLLOWUPS, P0/P1=0.
- `DARK_MODE_MERGE_AUTHORIZED`: PR #250, HEAD `9f11df7`, checks green,
  merge-commit strategy (P-009), approval source = activation record, scope
  match = 095-S. Normal merge path (`NORMAL_MERGE_READY`); no admin fallback.

## Residual risk and follow-ups

- **Residual (advisory)**: the hermetic value test does not exhaustively
  exercise every scope-glob edge. Mitigated by the live-corpus guard which runs
  against the full production scope on every docs-affecting PR. No backlog item
  raised — coverage is adequate for the regression-guard contract.

## Compound-refresh

No existing compound learning was invalidated by this work. The guard
formalizes an existing convention (docline soft keys) rather than changing
runtime behavior; no new hard-won cross-cutting learning warranted a compound
entry beyond this closure record.
