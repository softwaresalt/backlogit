---
chunk_strategy: h1-h2-h3
description: 'Multi-persona plan review (adversarial review of the DECISIONS) of the pre-task-completion gate-broker phase-2 hardening plan. Six independent reviewer personas (Architecture Strategist, Scope Boundary Auditor, Security Lens, Go Reviewer, SQLite Reviewer, Constitution Reviewer) reviewed the plan grounded in the shipped 082-F code. Result: 3 PASS + 3 ADVISORY, zero FAIL, zero all-persona blockers. All material advisory findings were incorporated into plan revision (F4 composed predicate + comma-ok; F7 omitempty parsed-assertion; F5 ErrorClass cast; Q3 single-source leaf predicate across the one-way core->db boundary; dedicated gate_evidence table; forced_no_run visibility; doctor log-scan staleness fallback; advisory-derived-only marker). Final gate verdict: PASS.'
doc_type: review
docline:
    date: 2026-07-06T00:00:00Z
    gate: PASS
    plan: docs/exec-plans/2026-07-06-gate-broker-phase2-hardening-plan.md
    reviewers:
        - Architecture Strategist
        - Scope Boundary Auditor
        - Security Lens Reviewer
        - Go Reviewer
        - SQLite Reviewer
        - Constitution Reviewer
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/reviews/2026-07-06-gate-broker-phase2-hardening-plan-review.md
title: 'Plan Review: pre-task-completion gate broker — phase-2 hardening'
---

# Plan Review — Pre-task-completion gate broker, phase-2 hardening

- **Plan:** `docs/exec-plans/2026-07-06-gate-broker-phase2-hardening-plan.md`
- **Deliberation:** `docs/decisions/2026-07-06-gate-broker-phase2-hardening-deliberation.md`
- **Method:** six independent reviewer personas dispatched in parallel, each grounding findings in
  the shipped 082-F code (not a blind read of the plan). Adversarial review of the **decisions**.
- **Plan-review attempt:** 1

## Verdict: **PASS** (after incorporation)

No persona returned FAIL and there were no all-persona blocking findings. Three personas returned
PASS outright (Scope, SQLite, Constitution); three returned ADVISORY with material,
actionable findings (Architecture, Security Lens, Go). Every material advisory finding was
**incorporated into the plan** (revision folding), which resolves the ADVISORY conditions and
yields a clean PASS.

| Persona | Initial verdict | Material findings | Disposition |
|---|---|---|---|
| Scope Boundary Auditor | PASS | none material | Confirmed exactly 5 items; strict-mode correctly excluded |
| SQLite Reviewer | PASS | column-vs-table recommendation | Adjudicated (dedicated table) |
| Constitution Reviewer | PASS | 4× P3 advisory | Incorporated (quality-gate exit criteria, ephemeral-cache guard, red-harness micro-step) |
| Architecture Strategist | ADVISORY | P1 predicate duplication; P2 column coupling; P2 doctor staleness | All incorporated |
| Security Lens Reviewer | ADVISORY | P2 composed-predicate order; P2 forced-no-run visibility; P2 predicate drift; P3 attractive-nuisance | All incorporated |
| Go Reviewer | ADVISORY | P1 F7 omitempty; P1 F4 comma-ok; P3 F5 cast; P3 F7 ordering | All incorporated |

## Constitution Check

The Constitution Reviewer mapped every change against Principles I–IX and the backlogit/engram/
intercom overlays and returned **PASS** with only P3 advisories. Highlights:

- **II. Test-First (NON-NEGOTIABLE):** all seven units write a red test proving the current gap
  before the fix; no standalone "tests" task; tests colocated `*_test.go`.
- **IV. CLI Containment (NON-NEGOTIABLE):** all edits within `internal/`; SQLite writes target the
  in-workspace ephemeral cache; no out-of-tree writes.
- **VII. Destructive Approval (NON-NEGOTIABLE):** no destructive op on authoritative data — Q3 is
  additive; the projection is a disposable derived cache; item-log JSONL is never mutated; rollback
  = revert + `sync`.
- **IX. Git-Friendly Persistence:** the read-model is a derived/disposable projection over
  append-only logs; completion writes go to logs, not the projection → no merge-conflict churn.
- **2-Hour Rule:** each unit is < 3 files, single skill domain; Q3 was decomposed *because* it
  exceeded the heuristic.

## Material findings and their incorporation

### Architecture
- **P1 — Q3 predicate/constant duplication across the one-way `core → db` boundary.** `internal/db`
  cannot import `internal/core`, so the projection could not reuse core's `latestGatePassEvidence`
  or event constants → structural drift risk. **Incorporated:** added **Subtask Q3.0** extracting
  the composed predicate + event-type constants into a shared **leaf** package consumed by both
  `core` (F4) and `db` (Q3.2) — single source of truth.
- **P2 — column-on-`items` cross-phase UPDATE coupling.** Gate evidence is log-sourced in a later
  rehydrate phase than the frontmatter-sourced `items` batch-insert. **Incorporated:** adjudicated
  to a dedicated `gate_evidence` table sourced purely in the log phase (SQLite persona's
  column option retained as a documented fallback).
- **P2 — doctor advisory staleness regression.** The projection rebuilds only on `sync`, while the
  live path indexes events incrementally; repointing doctor to a sync-only cache could emit false
  negatives, and a no-op fallback would silently stop auditing. **Incorporated:** Q3.3 falls back
  to the authoritative log-scan when the projection is absent/stale.

### Security Lens
- **F4 forced-completion policy — ADJUDICATED:** keep `EventGateForced` acceptance **unconditional**
  (Decision 2 is correct). The F4 gap is the silent, automatic fail-open `EventGatePassed{ran:false}`
  no-run; force is a deliberate, CLI-only, reason-bearing, durably-audited break-glass and is a
  different threat class. Rejecting audited force at ship time would break its documented semantics.
- **P2 — composed-predicate order under-specified.** **Incorporated:** plan now specifies the
  explicit predicate `(EventGateForced) OR (EventGatePassed AND ran==true)` and an interleaved-events
  test.
- **P2 — forced-no-run members invisible at ship time.** **Incorporated:** Q3.2 emits a distinct
  `forced_no_run` status token; Q3.3 surfaces it as an informational advisory.
- **P2 — predicate drift / P3 — attractive nuisance.** **Incorporated:** single-source leaf
  predicate (Q3.0); projection marked advisory-derived-only and stores only status token + evidence
  SHA (no report JSON/stderr/`force_reason`).

### Go
- **P1 — F7 `retryable,omitempty`.** A config-class `*GateError` omits the key; the AC's literal
  `retryable:false` was unattainable via the shared struct, and dropping `omitempty` would regress
  the blocked payload. **Incorporated:** ACs/tests now assert the **parsed** value / key-absence;
  `omitempty` retained; check `*GateBlockedError` before `*GateError`.
- **P1 — F4 `delta["ran"]` panic.** **Incorporated:** plan mandates the comma-ok assertion
  `ran, _ := ev.Delta["ran"].(bool)` (mirrors the `head_sha` idiom); missing/non-bool → not-ran.
- **P3 — F5 `ErrorClass` defined-type cast.** **Incorporated:** `class := string(ev.Decision.ErrorClass)`
  with `""→"config"` default, mirroring `errorGate`.
- **F1 — PASS:** `ResolvedBase.OverrideShadowed` field is idiomatic and keeps `baseref.go` pure.

### Constitution (P3 advisories)
- **Incorporated:** added an explicit per-unit **quality-gate exit criteria** section
  (`gofmt`/`go vet`/`golangci-lint`/`go test`) and the compiling-red-harness micro-step; reaffirmed
  the projection lives only in the ephemeral gitignored cache (never committed state).

## Residual (non-blocking) notes carried to Ship

- Column-on-`items` remains an acceptable Q3.1 fallback if Ship finds the cross-phase concern
  immaterial; document whichever shape is chosen.
- The `forced_no_run` advisory surface in doctor is informational only — not a new gate/enforcement.

## Gate outcome

**PASS.** The plan is coherent, in-scope (exactly the 5 sanctioned items), constitutionally
compliant, test-first, and grounded in real symbols. All material advisory findings from the three
ADVISORY personas were folded into the plan. Advance to harvest.
