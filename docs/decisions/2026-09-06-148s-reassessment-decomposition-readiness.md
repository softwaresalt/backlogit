---
chunk_strategy: h1-h2-h3
description: "Reassessment of shipment 148-S / feature 167-F (#423) decomposition and pre-Ship readiness"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-06-148s-reassessment-decomposition-readiness.md
title: "Reassessment — 148-S / 167-F (#423) release-unit width, decomposition, and sizing"
---

# Reassessment — 148-S / 167-F (#423) decomposition & readiness

**Shipment:** 148-S (queued, critical) — covering feature 167-F "Governed
archived-shipment reconciliation to shipped (#423)"
**Members at reassessment:** 167-F + 20 tasks (167.001–167.017, 167.019, 167.020,
167.021) — all previously **unsized**.
**Upstream plan:** `docs/exec-plans/2026-09-05-423-archived-shipment-reconciliation-to-shipped-plan.md` (Plan Review: 5 cycles, multi-model, **PASS**)
**Date:** 2026-09-06

## Question posed

Is 148-S still **too broad** as a single release unit despite task-level slicing?
If so, restructure into smaller coherent features/shipments with explicit
dependency ordering and clean domain boundaries. If it is already sufficiently
decomposed, **produce evidence** rather than asserting it.

## Verdict

**Do NOT split. 148-S is already correctly decomposed as a single coherent
release unit.** The one genuine deficiency — all 20 tasks unsized — has been
remedied (see §Sizing). The remainder of this document is the evidence.

## Evidence

### E1 — Single responsibility: one command, one contract

All 20 tasks deliver exactly ONE user-facing capability: the
`backlogit shipment reconcile-shipped` command and the core primitive behind it.
The plan's Implementation Units U1–U5 and deliberation D1/D8 establish this as a
single, indivisible governed-reconcile contract (explicitly distinguished from
the separate S12/164.002-T live-queue contract, D8). There is no second
deliverable hiding inside 167-F to peel off.

### E2 — Convergent dependency web, not separable clusters

The task graph is a single connected DAG that **converges on 167.008-T**
(the reconciliation transaction), which has 9 upstream prerequisites
(167.007, 167.001, 167.010, 167.014, 167.016, 167.017, 167.019, 167.020, 167.021)
and 3 downstream dependents (167.004 CLI, 167.005 integration, 167.009 docs).
The "clusters" one might split out — locks (167.011, 167.017), atomic writer
(167.006), snapshot/rollback (167.016), CAS clobber-safety (167.019), classifier
(167.010), event (167.002, 167.007), config (167.012) — are **internal building
blocks of the transaction**, not independently shippable capabilities. A
"primitives-only" shipment would deliver **no user-observable behavior** and
would be dead, unexercised code until the transaction shipment landed, violating
the atomic-milestone rule (each shipment must produce a verifiable, meaningful
outcome). Splitting here trades one coherent release unit for two, one of which
cannot be validated on its own.

### E3 — Trust-boundary separation has ALREADY been performed

The operator directive favors clean trust-boundary separation. 167-F operates
entirely **within one trust boundary**: the workspace-local governed reconcile
operation. The cross-trust-boundary work — cryptographic authenticity of
evidence and authenticated operator authorization — was consciously **carved out
of 167-F during #423 Plan Review cycle 3** into stash `866FDC8C`, which this same
Stage session has now decomposed into three isolated trust-boundary features
(`168-F`/`149-S`, `169-F`/`150-S`, `170-F`/`151-S`). The separable trust
boundaries are therefore already isolated at the release-unit level, **outside**
148-S. What remains in 148-S is a single-trust-boundary unit that should not be
fragmented further.

### E4 — Direct precedent: prior over-decomposition of this exact work was rejected

Deliberation §D6.1 records that an earlier attempt split this work by adding an
8-task lock-order migration prerequisite chain (167.017–167.024). Round-36/37
review, via **direct code verification** of the lock semantics (A and B are
bounded-wait; only the in-process C-mutex is indefinite; reconcile holds one
C-mutex acquired first), found that chain to be a **Scope-Boundary / YAGNI
violation grounded in a mis-characterized lock model**, and it was removed
(167.018, and the old 167.021–024 deleted; 148-S shrank 25→21 items). Re-splitting
148-S now would repeat exactly the failure mode already investigated and
rejected. (Note: the CURRENT 167.021-T is a distinct ArchiveItem preservation
guard, not one of the deleted lock-order tasks — not resurrected.)

### E5 — Already gated HARVEST-READY by multi-model review

The plan's Plan Review reached **PASS** across 5 cross-model cycles (Security,
Correctness, Concurrency, Architecture personas) and concluded verbatim:
"HARVEST-READY … decomposed into a coherent 20-task release unit." The present
reassessment confirms that verdict; it does not overturn it.

### E6 — Every task satisfies the 2-hour / width / atomic-milestone rules

Each of the 20 tasks is single-domain (the plan's harvest notes flag width
isolation per task — e.g. config split into 167.012, writer/lock split into
167.006/167.011, CI split into 167.013), TDD-ordered (RED harness before code),
and acceptance-bearing (acceptance + TDD (a)(b)(c) specs embedded in each task
body). None spans multiple skill domains.

## Sizing (deficiency remedied)

The single actionable gap — the shipment was **entirely unsized** — is fixed.
All 20 tasks now carry explicit `size` (source: agent, ruleset
`stage-2h-rule-v1`) plus `complexity`:

| Size | Count | Complexity high | Complexity medium | Complexity low |
|---|---|---|---|---|
| S | 7 | — | 017, 021 | 003, 009, 012, 013, 015 |
| M | 13 | 001, 006, 007, 008, 010, 019, 020 | 002, 004, 005, 011, 014, 016 | — |

**No task is L or XL** — quantitative confirmation that every unit is within the
2-hour ceiling. `size` (volume) and `complexity` (difficulty/uncertainty) are
recorded separately so the genuinely hard-but-small orchestration/concurrency
units (e.g. 167.008 transaction, 167.019 CAS, 167.020 proof) are visible as
`M`/`high` rather than mislabeled as large.

## Dependency ordering (already explicit)

Dependency edges are explicit in the backlog (not prose-only): 167.008-T's 9
prerequisites and the downstream 167.004/005/009 edges, plus the
167.001↔167.012, 167.010↔167.002/003, 167.011↔167.017 chains, were verified
present. The `pipeline-topology` gate passes, confirming an acyclic,
predecessor-consistent graph. No ordering is left implicit.

## Readiness for Ship (unchanged gates)

148-S is **decomposition-complete and sizing-complete**, but per the plan's
Plan Review it is **not CLOSED** until the blocking `167.015-T` ratification gate
is satisfied (explicit operator/issue-owner acceptance of the confirmation-only
v1 authorization narrowing + the no-descoping narrowing). Feature C (`170-F`,
this session) is the mechanism that later upgrades that narrowing; it does not
retroactively close `167.015-T`. Ship may build and review 148-S; final closure
still awaits `167.015-T`.

## Outcome

- 148-S structure: **unchanged** (no split) — evidence E1–E6.
- 148-S sizing: **20/20 tasks sized** (13 M, 7 S; 0 unsized; 0 L/XL) + complexity.
- Dependency ordering: confirmed explicit and acyclic.
- Next eligible for Ship: **148-S** remains the next release unit (the three new
  866FDC8C shipments 149/150/151-S depend on 148-S and on 149-S).
