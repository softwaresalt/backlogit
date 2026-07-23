---
chunk_strategy: h1-h2-h3
description: 'Session memory — DARK_MODE p3 deliberation for stash 8CD8F46A: made the plan-review -> harvest gate capability-aware so it is satisfiable in every environment and never silently skips its own dispatch step. Decision artifact docs/decisions/2026-07-22-plan-review-dispatch-capability-deliberation.md (Option B: capability-aware gate + P-012 declared-degradation *principle* fallback, since P-012s registry mechanism does not model sub-agent dispatch). Amended .github/skills/plan-review/SKILL.md with a dispatch-capability probe, multi-agent-dispatch (TOOL_OK) vs single-agent-declared-degradation (TOOL_DEGRADED) modes, full-coverage terminal states (halt TOOL_UNAVAILABLE on incomplete coverage), and a persona rubric ADAPTER (identity .agent.md file + plan-focused Focus lens -> normalized to P0-P3). Shipped PR #284 merge 8944d277 after a REAL 4-persona adversarial dispatch panel + 4 Copilot review rounds. Follow-ups: stash 6FA0829B (upstream plan-review/SKILL.md.tmpl template parity) + 4CECCEAA (end-to-end Stage skip_review/harvest enforcement). Stash 8CD8F46A archived.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-22/8CD8F46A-plan-review-dispatch-memory.md
title: 8CD8F46A plan-review dispatch capability (DARK_MODE) — session memory
---

# 8CD8F46A Plan-Review Dispatch Capability (DARK_MODE) — Session Memory

## Scope

DARK_MODE (P-017), operator AFK, PR + merge pre-authorized, admin fallback
NOT authorized, agent-intercom UNAVAILABLE (degraded visibility → inline
DARK_MODE event logging in `plan.md`). Operator routing directive:

> `8CD8F46A` needs a real persona-dispatch path, so should get a full
> deliberation and decision outcome through staging workflow (p3).

Stash `8CD8F46A` asked to establish a real multi-persona plan-review dispatch
path (or a formal waiver) so the `plan-review → harvest` gate is not silently
skipped in environments that cannot dispatch reviewer sub-agents.

## Decision (Option B)

Recorded in `docs/decisions/2026-07-22-plan-review-dispatch-capability-deliberation.md`.

- **Chosen**: capability-aware plan-review gate. Probe dispatch capability, prefer
  real persona sub-agent dispatch (`dispatch_mode: multi-agent-dispatch`,
  `TOOL_OK`), else a single-agent persona pass
  (`dispatch_mode: single-agent-declared-degradation`, `TOOL_DEGRADED`). Both
  modes require full coverage of every selected persona; a mid-gate dispatch
  failure forces a complete sequential rubric pass; if neither can complete,
  **halt** with `TOOL_UNAVAILABLE` (no partial gate). A gate decision with **no**
  `dispatch_mode` record is a **local plan-review gate-integrity (contract)
  violation**.
- **Rejected**: Option A (mandatory dispatch — blocks Stage in incapable envs);
  Option C (per-plan operator waiver — rejected by the 2026-07-17 formal-gate
  spike; breaks DARK_MODE/AFK).

## Key correctness nuances (drove the review rounds)

- **P-012 scope**: P-012 (`.github/policies/workflow-policies.md`) is scoped to
  *backlog-registry* tools with `cli_command` fallbacks. Sub-agent dispatch is
  **not** a registry tool, so P-012's *mechanism* does not model it. The
  amendment applies P-012's declared-degradation **principle** locally and does
  **not** emit `POLICY_VIOLATION: P-012` / P-005 telemetry for a missing
  `dispatch_mode` (that would be inaccurate). Reclassification to a P-012
  violation is deferred until P-012 is generalized to capabilities.
- **Persona rubric adapter**: the `.github/agents/review/*.agent.md` files are
  **code-review** personas (e.g. `go-quality-reviewer.agent.md` checks
  `rows.Close()` / SQL; Learnings Researcher emits a `relevant_solutions`
  object). The authoritative *plan-review* rubric is each persona's **Focus
  column** in the SKILL's Reviewer Personas tables. Each persona is applied as an
  **adapter**: identity file + plan-focused Focus criteria → normalized to
  mergeable P0–P3 findings. Both modes MUST use the adapter so the degraded pass
  cannot silently apply code-review criteria in place of the plan lens.

## Adversarial review (REAL multi-agent dispatch — demonstrates the restored path)

4 background persona sub-agents (Constitution / Template-Integrity / Scope /
Architecture, cross-model gpt-5.6-terra). Adjudicated: reframed overclaimed P-012
authority to principle; added full-coverage terminal-state machine; regrandfathered
091/092/093-S as a pre-policy exception; corrected persona→file manifest
(`go-quality-reviewer`, `research/learnings-researcher`). Rejected false positives
(docline flat-frontmatter P1 ×2 — lint 0 violations + sibling shape;
`shipment_gate.go` scope P1 — 0 diff lines vs main). LOCAL_REVIEW_READY on
`f3a14fab` = READY_WITH_FOLLOWUPS.

## Copilot review (4 rounds, converged 3 → 1 → 1 → 0)

- **r1** (3 threads, HEAD `f3a14fab`): (1) manifest needs exact filenames → added
  persona→file table; (2) P-012 telemetry inaccuracy → reclassified as local
  contract violation; (3) "never silently skipped in any environment" over-claims
  end-to-end gate → narrowed to plan-review-contract scope. → `83911fb0`.
- **r2** (1 thread, HEAD `83911fb0`): the agent files are code-review personas,
  not plan lenses → reframed manifest as an **adapter** (identity + plan Focus →
  normalized P0–P3). → `8e383016`.
- **r3** (1 thread, HEAD `8e383016`): template-parity recorded only as prose →
  generated-artifact rule requires a concrete linked follow-up → created stash
  **6FA0829B** (upstream `plan-review/SKILL.md.tmpl`) + **4CECCEAA** (end-to-end
  Stage/harvest enforcement), linked both IDs in the decision Unresolved
  Questions. → `2b8518fe`.
- **r4** (HEAD `2b8518fe`): CLEAN. §1.9 passed (review fresh, no pending requests,
  zero unresolved threads); CI 4/4 green; P-009 verified (merge-commit only).

All threads replied + resolved via `gh api graphql resolveReviewThread`.

## Ship + closure

- Impl PR **#284** → merge **8944d277** (2 files SKILL.md + decision doc, plus the
  2 follow-up stash entries). DARK_MODE_MERGE_AUTHORIZED (in scope; merge
  pre-authorized; §1.9 + CI + P-009 green).
- Branch `chore/plan-review-dispatch-capability` deleted. main synced.
- Stash **8CD8F46A archived** (resolved by the decision artifact; provenance via
  `stash_ids: 8CD8F46A` in the decision frontmatter).
- Gates green throughout: `backlogit docs lint` 0 violations; soft-keys corpus
  test ok.

## Follow-ups (active stash)

- **6FA0829B** (low): propagate the gate to the external `plan-review/SKILL.md.tmpl`.
- **4CECCEAA** (medium): wire end-to-end enforcement — reject Stage `skip_review`
  without a valid review record; `harvest` halts on missing/invalid `dispatch_mode`.

## Next steps

Remaining operator-gated stash after 8CD8F46A: `0F2E5BA9` (deliberation, p2),
`131CEAE4` (isolated spike), `9D5BB492` (distinct spike), `7F0A6E89` (EXTERNAL
autoharness repo — Principle IV, cannot perform), plus the two self-generated
follow-ups above. HALT — await operator routing per the DARK_MODE scope contract.
