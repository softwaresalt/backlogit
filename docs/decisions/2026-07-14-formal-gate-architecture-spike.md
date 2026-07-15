---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Formal-gate architecture spike charter'
source: docs/decisions/2026-07-14-formal-gate-architecture-spike.md
doc_type: decision
description: 'Time-boxed architecture spike charter defining the trust/atomicity contract that must be resolved before any formal PASS-only planning-gate implementation.'
docline:
    type: spike
    date: 2026-07-14T00:00:00Z
    time_box: "2h"
    conclusion: "pending"
    confidence: "low"
    linked_parent_work_item: null
    review_state: chartered
    supersedes: PR #239 (closed, unmerged) formal-gate implementation loop
    tags:
        - governance
        - architecture
        - planning-gate
---

# Formal-gate Architecture Spike Charter

## Context

The prior formal PASS-only planning-governance work (closed, unmerged PR #239 /
branch-local shipment 094-S / feature 105-F) collapsed into an unbounded
architecture/review-fix loop. Root cause: implementation and review-fix patching
proceeded while foundational trust and atomicity questions were still open, so
each review round surfaced a new architectural contradiction instead of a
bounded fix.

This charter re-frames that work as a **bounded, time-boxed architecture spike
that runs FIRST**. No formal-gate implementation, review-fix plan, or broad
governance task may be created until this spike resolves a single coherent
trust/atomicity contract. This is an investigation deliverable (findings +
contract), not an implementation deliverable.

## Goal

Within a strict 2-hour box, produce a **first-pass trust/atomicity contract
sketch** for a fail-closed formal PASS-only planning gate across the seven open
questions below, and a `proceed`/`pivot`/`defer` conclusion with a confidence
rating. The spike frames: *"What is the minimal, forgery-resistant, atomic
contract under which a formal PASS-only gate could be implemented without
re-opening architecture questions during implementation?"* Fully resolving all
seven questions coherently is a **stretch outcome, not a requirement**: at the
2-hour limit the spike MAY output unresolved decisions and follow-up
recommendations rather than a complete contract. It MUST NOT expand into broad
formal-gate implementation planning.

## Time Box

**2 hours** of human-equivalent effort (investigation only), consistent with the
constitutional 2-Hour Rule for the executable spike task (`105.001-T`). If the
contract is not coherent at the limit, the conclusion is `defer` with the
specific unresolved question(s) and any follow-up recommendations named — not an
extension.

## Open Questions (the contract sketch should address all seven; unresolved ones are named, not extended)

1. **Formal evidence trust/forgery model** — What makes a PASS evidence record
   trustworthy and non-forgeable? Who/what may author it, how is authorship
   attributed and verified, and what prevents a hand-written or replayed record
   from being accepted as genuine gate evidence?
2. **Mutation-manifest replay/binding** — How is a governed mutation manifest
   bound to the exact evidence and plan state that authorized it, so a stale or
   replayed manifest cannot be re-applied against a different state?
3. **Exact-byte CRLF semantics** — What are the exact-byte hashing/normalization
   semantics across CRLF/LF, trailing newline, and encoding, so evidence
   verification is deterministic on every platform (Windows included)?
4. **Two inconsistent status rules** — Reconcile the two conflicting status
   rules observed in the prior work into one authoritative status model (what
   states exist, who may transition them, and how a shipment/plan status is
   derived vs. authored).
5. **Dependency type durability** — Are dependency edge types (blocks,
   relates_to, parent_of) durable and authoritative across index rehydration
   and out-of-band branch changes, and what guarantees their persistence?
6. **Partial core-mutation rollback guarantees** — If a governed core mutation
   partially fails, what is the exact rollback/atomicity guarantee (all-or-
   nothing, journaled, idempotent replay) and how is a partial state detected
   and recovered?
7. **CLI parity documentation** — What is the documented parity contract between
   MCP tool surfaces and CLI entrypoints for every governed operation, so the
   gate cannot pass via one surface while failing via the other?

## Evidence to Inspect (read-only)

* Closed PR #239 diff and the branch-local artifacts it staged (feature 105-F,
  shipment 094-S, and the ten commits on `chore/stage-governance-docline`) —
  as the record of where the loop diverged.
* The prior governance planning lineage referenced by 105-F
  (`docs/exec-plans/2026-07-14-planning-governance-gates-plan.md` and its
  deliberation) — deliberately NOT reused as an implementation plan; inspected
  only to enumerate the contradictions that broke the loop.
* `internal/` codec, docline, and artifact-write paths for exact-byte and
  CRLF behavior (questions 3, 6).
* Dependency edge persistence and index rehydration paths (question 5).
* Existing status enums for shipment/review/plan artifacts (question 4).
* MCP tool registry vs. `cmd/backlogit` CLI command surface (question 7).

## Decision Outputs

At conclusion the spike must produce:

* A written first-pass trust/atomicity contract sketch addressing the seven
  questions, with any question left unresolved explicitly named alongside a
  follow-up recommendation.
* A conclusion of `proceed` (contract sketch coherent enough — promote to
  impl-plan for bounded follow-up units), `pivot` (a materially different gate
  design is required), or `defer` (contract not coherent within the 2h box;
  named unresolved questions and follow-up recommendations).
* A confidence rating (high/medium/low).
* If `proceed`: an enumeration of bounded, ≤2h implementation units the contract
  decomposes into — created only after this spike, never before.

## Explicit Non-Goals

* NO formal-gate implementation code, schema changes, or CLI changes.
* NO broad formal-gate implementation tasks or review-fix plans created before
  the contract is resolved.
* NO machine-waiver or ADVISORY-admission design (explicitly deferred).
* NO reuse or continuation of the closed PR #239 commit series.
* NO coupling to the docline soft-key guard or the size-estimation work staged
  alongside this spike.

## Exit Criteria

The spike is complete when EITHER:

* A coherent first-pass contract sketch is produced with a `proceed` or `pivot`
  conclusion and a confidence rating (and, if `proceed`, a bounded follow-up
  decomposition), with any still-open question named; OR
* The 2h time box is reached, in which case the conclusion is `defer` with each
  unresolved question and follow-up recommendation named and the partial
  contract sketch recorded.

No formal-gate implementation unit may be opened until this charter's decision
output records `proceed` (or `pivot` with a replacement contract).

## References

* Closed PR #239: https://github.com/softwaresalt/backlogit/pull/239
* Branch-local (unmerged) prior scope: feature 105-F, shipment 094-S on
  `chore/stage-governance-docline`.
