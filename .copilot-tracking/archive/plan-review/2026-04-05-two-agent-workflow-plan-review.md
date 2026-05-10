---
title: "Plan Review: Two-agent workflow refactor"
date: 2026-04-05
plan: "docs/exec-plans/2026-04-05-two-agent-workflow-plan.md"
gate: pass
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
passes: 2
---

# Plan Review: Two-agent workflow refactor

## Gate Decision: PASS

No P0 or P1 findings remain. All prior blocking findings resolved through plan
revision and Constitution v2.1.0 amendment. Remaining P2 and P3 findings are
advisory and can be addressed during implementation.

## Review History

### Pass 1 (pre-revision)

Gate: FAIL — 2 P0, 6 P1, 5 P2, 2 P3

Prior P0 findings:
- F-01 (migration-first execution): **RESOLVED** — Units 4 and 8 changed to
  test-first with detailed verification criteria.
- F-02 (stash JSONL/CQRS): **RESOLVED** — Constitution v2.1.0 adds layer 4
  (JSONL queues for transient intake). Stash is the canonical example.

Prior P1 findings:
- F-03 (observability): **RESOLVED** — Units 2, 3, 4 now specify slog entries
  and events.jsonl schemas.
- F-04 (error types): **RESOLVED** — Unit 2 defines 4 sentinel errors with
  errors.Is wrapping.
- F-05 (schema/prefix): **RESOLVED** — Shipment Schema Contract section defines
  prefix S, 8 frontmatter fields with validators, template sections, and
  association model.
- F-06 (alpha scope): **RESOLVED** — CQRS concern removed; scope is intentional.
- F-07 (rehydration coordination): **RESOLVED** — Unit 4 depends on Unit 2;
  dependency graph updated.
- F-08 (tool contract gate): **RESOLVED** — Slice 3 gate requires Unit 3
  contract tests to pass before agent authoring.

### Pass 2 (post-revision)

Gate: PASS — 0 P0, 0 P1, 4 P2, 4 P3

## Summary

| Severity | Count |
|----------|-------|
| P0       | 0     |
| P1       | 0     |
| P2       | 4     |
| P3       | 4     |

## Findings

### P0 — Critical

None.

### P1 — High

None.

### P2 — Moderate (address during implementation)

#### F-16: Rehydration load order between Units 2 and 4 needs implementation spec

Both units modify `internal/db/rehydration.go`. The dependency graph now
sequences Unit 4 after Unit 2, but the plan does not specify which rehydration
methods run first or how their index zones are isolated. Add an integration
test in Unit 4 that verifies shipment index and stash index coexist cleanly
after a full rehydration cycle.

**Sources:** AS-1-REFINEMENT, AS-NEW-3, SB-NEW-4

---

#### F-17: Blocked-item return spans two file modifications

Returning a blocked item requires modifying both the shipment artifact (remove
from items list) and the item artifact (set status to blocked, add reason).
If the first write succeeds and the second fails, the files become
inconsistent. Specify a recovery model: either write both atomically (temp
files, then rename both) or detect inconsistency during rehydration and
auto-repair.

**Sources:** AS-NEW-1

---

#### F-18: Agent invocation pattern not specified for shipment operations

Unit 3 exposes shipment through both CLI and MCP. Units 5-6 create groomer and
shipper agent docs that reference shipment commands, but the plan does not
state which surface agents should prefer. Resolve before Unit 5-6 authoring:
agents should use MCP tools (per Constitution Principle IX, agent context
efficiency) with CLI as the developer-facing fallback.

**Sources:** SB-8-PARTIAL, SB-NEW-1

---

#### F-19: Unit 3 contract test scope needs enumeration

Unit 3 verification says "key success and error responses" without listing
specific scenarios. Enumerate during implementation: create with valid inputs,
create with missing items, claim nonexistent shipment, return-blocked with
invalid item ID, pre-init tool discovery, CLI arg parsing errors.

**Sources:** SB-NEW-5, AS-NEW-4

### P3 — Low (advisory)

#### F-20: Gate enforcement mechanism for Slice 3 not specified

The Slice 3 gate ("Unit 3 contract tests must pass before agent authoring") is
declared but the enforcement model (backlogit dependency, CI gate, or manual
review) is not specified. Resolve during harvesting.

**Sources:** AS-3-QUALIFICATION

---

#### F-21: Split-brain mitigation during stash migration

The plan flags split-brain risk but does not specify a write-lock or
single-writer guarantee during the brief migration window. Since migration is
repo-local only with no concurrent external writers, this is low risk. Add a
simple file-lock or sequential migration step during implementation.

**Sources:** SB-NEW-2

---

#### F-22: Rollback strategy implicit but not documented

D1 (sidecar bootstrap) and the legacy-agent preservation non-goal provide an
implicit rollback path. Document the rollback procedure explicitly during
Slice 4 dogfooding: can shipment be disabled via config? Can legacy agents
resume full ownership?

**Sources:** SB-NEW-3

---

#### F-23: Deprecation timing for legacy agents

Non-goal 2 says deprecate "after the new pipeline passes verification." Units
5-6 say "update instructions without removing yet." Clarify during
implementation: Units 5-6 introduce new agents as preferred alternatives;
explicit deprecation markers wait until after Unit 8 verification.

**Sources:** SB-NEW-7

## Reviewer Attribution

| Finding | Reviewer(s)                                    | Model        |
|---------|------------------------------------------------|--------------|
| F-16    | Architecture Strategist, Scope Boundary Auditor | Claude Opus 4.6 |
| F-17    | Architecture Strategist                         | Claude Opus 4.6 |
| F-18    | Scope Boundary Auditor                          | Claude Opus 4.6 |
| F-19    | Scope Boundary Auditor, Architecture Strategist | Claude Opus 4.6 |
| F-20    | Architecture Strategist                         | Claude Opus 4.6 |
| F-21    | Scope Boundary Auditor                          | Claude Opus 4.6 |
| F-22    | Scope Boundary Auditor                          | Claude Opus 4.6 |
| F-23    | Scope Boundary Auditor                          | Claude Opus 4.6 |

## Next Steps

Plan passes the review gate. Proceed to:

1. Harvest the approved plan into backlogit work items using the
   backlog-harvester agent.
2. Start with Slice 1 (Units 1 and 2).
3. Address P2 findings F-16 through F-19 during implementation of the
   relevant units.
| F-10    | Constitution Reviewer                        | Claude Opus 4.6 |
| F-11    | Scope Boundary Auditor, Go Quality Reviewer  | Claude Opus 4.6 |
| F-12    | Scope Boundary Auditor, Constitution Reviewer | Claude Opus 4.6 |
| F-13    | Architecture Strategist                      | Claude Opus 4.6 |
| F-14    | Go Quality Reviewer                          | Claude Opus 4.6 |
| F-15    | Architecture Strategist                      | Claude Opus 4.6 |
