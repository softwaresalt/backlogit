---
title: "Plan Review: Event Traceability and Commit Tracking"
date: 2026-04-10
plan: "docs/exec-plans/2026-04-10-event-traceability-commit-tracking-plan.md"
origin_deliberation: ".backlogit/queue/011-DL.md"
feature: "023-F"
task: "023.008-T"
shipment: "006-S"
gate: advisory
reviewers: [constitution-reviewer, go-quality-reviewer, architecture-strategist, scope-boundary-auditor]
context: "Retroactive plan review for Stage gating remediation"
---

## Gate Decision: ADVISORY

The plan is structurally sound with no P0 or P1 findings. Two P2 advisory
findings and one P3 observation are documented below. The plan may proceed to
harvest with these findings carried into task implementation notes.

## Summary

| Severity | Count | Action          |
|----------|-------|-----------------|
| P0       | 0     | —               |
| P1       | 0     | —               |
| P2       | 2     | User discretion |
| P3       | 1     | Advisory        |
| **Total**| **3** |                 |

## Findings

### P0 — Critical (must fix before proceeding)

None.

### P1 — High (should fix before proceeding)

None.

### P2 — Moderate (user discretion)

**F-001: Decision D3 contradicts Unit 2 file scope**

**Reviewer:** Architecture Strategist
**Unit:** 2 (Core lifecycle threading)
**Finding:** Decision D3 states "Thread through MCP layer not core layer" to
keep core functions commit-agnostic. However, Unit 2 lists core files
(`archive.go`, `shipment_lifecycle.go`, `shipment.go`) as modification targets.
The existing code in `archive.go:87-94` creates events directly in the core
function — there is no MCP-layer interception point for archive events.

**Recommendation:** Clarify D3: core lifecycle functions that already emit
events (archive, ship) receive an optional `commitSHA` parameter. MCP tools
that trigger other mutations (move, comment) add commit_sha at the MCP boundary.
The intent — minimal parameter pollution in core APIs — is preserved, but the
wording should not imply core is untouched.

**Impact on harvest:** Low. Implementation will naturally discover the correct
threading points. Carry this finding into 023.008-T implementation notes.

---

**F-002: Unit 2 effort estimate may be optimistic**

**Reviewer:** Scope Boundary Auditor
**Unit:** 2 (Core lifecycle threading)
**Finding:** Unit 2 modifies three core files (`archive.go`,
`shipment_lifecycle.go`, `shipment.go`) and extends contract tests. The
"medium" effort estimate (~2 hours) is borderline given the number of call sites
and the need for characterization testing across all three files.

**Recommendation:** If implementation exceeds the 2-hour heuristic, split into
Unit 2a (archive path) and Unit 2b (shipment path). The plan is acceptable as-is
for initial execution.

### P3 — Low (advisory)

**F-003: Characterization-first execution posture notation**

**Reviewer:** Constitution Reviewer
**Finding:** Unit 2 specifies "characterization-first" execution posture.
Constitution Principle III (Test-First Development) is non-negotiable, but
characterization-first is a valid test-first variant (run existing tests to
establish baseline before modifying). The notation is acceptable but should be
documented as a deliberate choice, not an omission of test-first discipline.

**Recommendation:** No action required. The plan's verification section for
Unit 2 explicitly includes running existing tests first.

## Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F-001 | Architecture Strategist | claude-sonnet-4.6 |
| F-002 | Scope Boundary Auditor | claude-sonnet-4.6 |
| F-003 | Constitution Reviewer | claude-sonnet-4.6 |

## Next Steps

Gate decision is **ADVISORY**. The plan may proceed to harvest. Findings F-001
and F-002 should be carried into 023.008-T implementation notes so the Ship
agent can address them during execution.
