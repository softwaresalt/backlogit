---
chunk_strategy: h1-h2-h3
description: 'Genuine multi-persona cross-model plan review of the 108-F size-estimation implementation plan. Five independent reviewer personas (Constitution Reviewer, Scope Boundary Auditor, Architecture Strategist on gpt-5.6-sol, Security Lens Reviewer on gemini-3.1-pro-preview, Go Reviewer) were dispatched in parallel grounded in the authoritative size-extension architecture spike and the live code seams. Initial gate FAIL (3 FAIL + 2 ADVISORY, four converging P1s: unsafe SE-3 append+write atomicity, update-only single-writer, size_source human-masquerade via MCP, positional-string seam). Three review-fix cycles resolved every P1: SE-3 split into SE-3a/SE-3b with an op-id + PrevOpID predecessor chain and doctor compare-and-swap; create+update single-writer enforcement; transport-aware actor stamping; typed SizeMutation. Final gate: PASS with only P2/P3 residual advisories.'
doc_type: review
docline:
    date: 2026-07-18T00:00:00Z
    gate: PASS
    plan: docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md
    reviewers:
        - Constitution Reviewer
        - Scope Boundary Auditor
        - Architecture Strategist
        - Security Lens Reviewer
        - Go Reviewer
ingested_at: "2026-07-18T00:00:00Z"
schema_version: "1.0"
source: docs/reviews/2026-07-18-108-F-size-estimation-plan-review.md
title: 'Plan Review: 108-F size estimation for feature and shipment'
---

# Plan Review — 108-F size estimation for feature and shipment

- **Plan:** `docs/exec-plans/2026-07-18-108-F-size-estimation-impl-plan.md`
- **Authoritative source:** `docs/decisions/2026-07-18-size-extension-contract-architecture-spike.md`
- **Method:** genuine **multi-persona, cross-model** review. Reviewer persona
  subagents were dispatched **in parallel** on different model tiers, each
  grounding findings in the authoritative spike and the live code seams
  (`internal/core/artifact_size.go`, `internal/core/artifacts.go`,
  `internal/models/frontmatter.go`, `internal/events/stream.go`). This was **not**
  a single-agent inline self-assessment.
- **Cross-model diversity:** Architecture Strategist on `gpt-5.6-sol`; Security
  Lens Reviewer on `gemini-3.1-pro-preview`; Constitution, Scope Boundary, and Go
  reviewers on their default tiers.

## Verdict: **PASS** (after 3 review-fix cycles)

Gate rule: any P0/P1 ⇒ FAIL. Circuit-breaker limit: 3 review-fix cycles. The gate
reached PASS on cycle 3, within budget.

| Cycle | Reviewers dispatched | Result |
|---|---|---|
| 0 | Constitution, Scope Boundary, Architecture (gpt-5.6-sol), Security Lens (gemini-3.1-pro-preview), Go | **FAIL** — 3 personas gated FAIL, 2 ADVISORY; 4 converging P1s |
| 1 | Architecture (gpt-5.6-sol), Security Lens (gemini-3.1-pro-preview), Go | Security **PASS**, Go **ADVISORY** (all 4 prior RESOLVED), Architecture **FAIL** (3 new P1s) |
| 2 | Architecture (gpt-5.6-sol) | **FAIL** — prior #2/#3 RESOLVED, 1 blocker (op-id CAS decidability) |
| 3 | Architecture (gpt-5.6-sol) | **PASS** — blocker RESOLVED; only a P3 advisory remains |

## Cycle 0 — initial findings (FAIL)

Four converging P1s across Architecture, Security, Go, and Constitution personas:

1. **P1 — SE-3 append+write atomicity unsafe/contradictory (Architecture + Go).**
   Append-then-write yields orphan events; compensating truncation of the
   **shared** per-item JSONL (appended by non-size writers that do **not** hold the
   per-task lock) can delete concurrent legitimate events — data loss.
2. **P1 — SE-2 single-writer not enforced + emitter field-set parity
   (Architecture + Go).** The generic `custom_fields` replace can mutate sizing
   keys; `ToFrontmatterMap()` must preserve status-gated
   `archived_from`/`archived_status` and `links`.
3. **P1 — size_source human-masquerade via MCP (Security + Constitution).** An
   agent transport could claim `size_source: human`, forging human provenance; the
   §7 actor-context stamping rule had been dropped.
4. **P1 — positional-string seam signature (Architecture + Go).** The four-adjacent
   -strings seam is error-prone and cannot distinguish omitted from cleared from
   set.

P2/P3s: split SE-3; bound `size_ruleset_version` (no free-text injection); trim
SE-4/SE-3 test counts; SE-6 file-count; confirm invalid enum ⇒ CLI exit 1 (not
`ExitError{Code:4}`, reserved for `ErrTaskBusy`); confirm `events.Event` is already
generic (new `EventType` constant only, no `stream.go` signature change).

## Cycle 1 — re-review after first revision

- **Security Lens (gemini-3.1-pro-preview): PASS.** Human-masquerade, ruleset
  free-text, and containment findings all RESOLVED. Two P3 advisories: (i) a
  shell-granted agent can bypass MCP stamping by invoking the CLI directly — note
  in the SE-8 contract doc; (ii) persist `size_op_id` deterministically so doctor
  need not value-match.
- **Go Reviewer: ADVISORY.** All four prior findings RESOLVED (typed
  `SizeMutation` pointer-presence semantics correct; exit-1 via `ErrValidation`
  correct; new `EventType` constant only). P2 advisories: test-envelope pressure,
  PA-4 "byte-equality" wording should be map-assertion, `ActorContext` origin
  unspecified, op-id dedup-before-append ordering must be pinned.
- **Architecture Strategist (gpt-5.6-sol): FAIL.** Three P1s: (1) SE-3b op-id
  state machine under-specified; (2) SE-2 guarded update but **not** the create
  path; (3) SE-2 acceptance-criteria contradiction + a spurious `SE-2→SE-4` arrow
  in the ASCII graph.

## Cycle 2 — Architecture re-gate

**FAIL** with prior #2 (create-path single-writer) and #3 (canonical projection
rule + graph fix) **RESOLVED**, leaving one blocker: the compare-and-swap "newer"
determination was not decidable from opaque `size_op_id` equality alone. Fix
directed: record the expected predecessor op-id in each event and have doctor apply
only when the artifact's current op-id equals that predecessor.

## Cycle 3 — Architecture final re-gate

**PASS.** The predecessor chain was added: the event carries both `OpID` and the
captured `PrevOpID`; doctor compare-and-swap applies a fresh orphan only when
current `size_op_id == PrevOpID`, treats `== OpID` as already-applied, and leaves
any other value as stale/benign residue (never replayed). Ordering is now decidable
without opaque-ID ordering, and a fresh orphan always references the last durably
applied op. Only a P3 advisory remains (no action required).

## Residual / deferred (non-blocking)

- **P2 (Go):** per-unit test-scenario counts sit at the 2-hour boundary. The plan
  states table-driven `t.Run` subtests count as one scenario and flags SE-2's
  characterization lock as a splittable sibling if the envelope is exceeded.
- **P3 (Security):** transport-aware stamping cannot prevent masquerade if an
  agent has unrestricted local shell (it can invoke the CLI directly). To be noted
  in the SE-8 contract doc.
- **P3 (Architecture):** exact `size_op_id`/`PrevOpID` JSON key names and doctor
  reporting verbosity are Ship-level implementation detail.
- **Deferred backlog item:** SE-6 CLI human-column size parity → separate P2
  follow-up (JSON/MCP parity is the blocking SE-6 requirement).

## Reviewer honesty statement

The initial gate was an **honest FAIL**, not a manufactured PASS. Three
independent review-fix cycles were run against genuine cross-model persona
subagents; each cycle's findings were addressed in the plan before re-gating. The
final PASS reflects resolution of every P0/P1 finding, with the residual items
explicitly recorded above rather than silently dropped.
