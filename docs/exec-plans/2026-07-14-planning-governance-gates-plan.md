---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Planning governance gates implementation plan'
source: docs/exec-plans/2026-07-14-planning-governance-gates-plan.md
doc_type: plan
description: 'Implementation plan for attributable formal plan-review dispatch, a fail-closed waiver contingency, Stage enforcement, and explicit impl-plan Constitution Checks.'
docline:
    date: 2026-07-14T18:35:00Z
    origin: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
    review_state: bootstrap-waived
---

# Planning Governance Gates Implementation Plan

## Problem Frame

The repository already declares a supported reviewer dispatch topology, but the plan-review instructions do not preflight that capability or define auditable evidence and Stage does not distinguish formal PASS from inline self-assessment. In this API invocation, the dispatch tool was omitted entirely. Separately, impl-plan does not emit the constitution's mandatory labeled section. The implementation must make the real path executable, evidence-bearing, and fail-closed without treating hosted or inline review as a formal result.

**Origin:** `docs/decisions/2026-07-14-plan-review-governance-deliberation.md`; backlog deliberation `052-DL`.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| G1 | Prefer real independent reviewer dispatch | Unit G2 defines capability preflight and dispatch protocol in both plan-review copies. |
| G2 | Prove the formal gate ran | Unit G2 requires per-persona dispatch/return evidence and attributed appended outputs. |
| G3 | Failure blocks harvest | Unit G3 makes Stage accept only a complete formal verdict or a valid waiver record. |
| G4 | Unsupported invocation behavior is explicit | Units G2/G3 record unavailable capability and halt by default. |
| G5 | Waiver is narrow and auditable | Units G2/G3 define required operator, plan, reason, timestamp, expiry, risk, and single-use fields; verdict is WAIVED. |
| G6 | Inline and hosted reviews do not impersonate personas | Contract tests and instructions label them supplemental/informal only. |
| G7 | Plans contain `## Constitution Check` | Unit G4 adds the exact section and principle mapping contract to both impl-plan copies. |
| G8 | Mirrored surfaces stay guarded | Unit G1 checks both `.github` and plugin copies for the required contract. |

## Scope Boundaries

### In Scope

- `.github` and plugin plan-review skill copies.
- `.github` and plugin Stage agent copies.
- `.github` and plugin impl-plan skill copies.
- One integration contract test.

### Out of Scope

- Reviewer persona implementation changes.
- New agent-runtime or API dispatch code; supported runtimes already expose the declared tool.
- Treating GitHub Copilot review as formal plan-review evidence.
- External autoharness templates or any out-of-tree write.
- Retrofitting old plan-review artifacts.

## Implementation Units

### Unit G1: Add planning-governance contract test

**Files:** `tests/integration/planning_governance_contract_test.go`
**Effort:** S, under 2 hours; 1 file; fewer than 4 scenarios.
**Skill domain:** Go integration test.
**Execution posture:** test-first.
**Dependencies:** none.

Create table-driven checks over each mirrored pair. The test must assert semantic markers, not exact whole-file equality because the installed and packaged copies have environment-specific frontmatter and surrounding prose. Scenarios:

1. both plan-review copies contain a dispatch capability preflight, required reviewer-evidence fields, explicit FAIL-on-incomplete dispatch, distinct WAIVED mode, and a prohibition on counting inline/hosted review as formal;
2. both Stage copies require complete gate evidence before harvest and enumerate waiver validation fields;
3. both impl-plan copies require the exact `## Constitution Check` heading and principle/deviation mapping.

**RED:** run `go test ./tests/integration -run TestPlanningGovernanceContracts -count=1`; current files must fail required-marker assertions.
**GREEN:** after G2-G4, the same command passes and names both copies in subtests.

**Acceptance criteria:** The test fails before instruction edits, passes after them, and emits path-specific failures.

### Unit G2: Define formal dispatch and evidence contract

**Files:** `.github/skills/plan-review/SKILL.md`, `plugin/skills/plan-review/SKILL.md`
**Effort:** S, under 2 hours; 2 files; instructional workflow only.
**Skill domain:** plan-review skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1.

Add a capability preflight before persona dispatch: detect semantic agent-subagent capability, including known `agent` and `agent/runSubagent` names. If unavailable, append a blocked gate record and halt unless a valid operator waiver is supplied. Do not attempt inline substitution.

For formal mode, require all always-on and triggered personas to return. Append a reviewer evidence table containing persona, agent definition, dispatch identifier/status, model/provider or `unknown`, finding count, and disposition. Missing or failed required returns produce FAIL. Preserve existing P0/P1/P2/P3 semantics.

Define waiver mode with required fields: exact plan path, operator/authorizer and authorization reference, missing capability, reason, UTC timestamp, single-use/expiry boundary, acknowledged residual risk, and disposition. Name the decision WAIVED, never PASS. Reject missing, expired, reused, or mismatched waivers. Hosted Copilot and inline assessment may be attached only as supplemental evidence.

**Acceptance criteria:** Both copies describe executable formal dispatch, attributed evidence, fail-closed behavior, and the exact narrow waiver contract; G1 scenario 1 passes.

### Unit G3: Enforce review provenance before Stage harvest

**Files:** `.github/agents/.stage.agent.md`, `plugin/agents/stage.agent.md`
**Effort:** S, under 2 hours; 2 files; Stage workflow only.
**Skill domain:** agent instruction authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1, G2.

At the pre-harvest gate, require the appended plan review record to declare one of:

- formal PASS;
- formal ADVISORY plus explicit operator proceed decision;
- valid `operator_waiver` / WAIVED record.

Validate required persona evidence for formal mode and all waiver fields for waiver mode. A missing agent tool, informal self-assessment, hosted review, absent section, formal FAIL, or malformed/expired waiver halts harvest with explicit reason. Preserve the existing review-fix cycle limit.

**Acceptance criteria:** Both Stage copies fail closed and cannot silently relabel informal work; G1 scenario 2 passes.

### Unit G4: Emit an explicit Constitution Check

**Files:** `.github/skills/impl-plan/SKILL.md`, `plugin/skills/impl-plan/SKILL.md`
**Effort:** S, under 2 hours; 2 files; planning output contract only.
**Skill domain:** impl-plan skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1.

Add `## Constitution Check` to the required plan structure and quality criteria. Require mapping of every applicable principle, explicit treatment of NON-NEGOTIABLE principles, any conflict/waiver with justification and rejected simpler alternative, and a clear `No violations or exceptions` statement when clean. Keep the existing Standards Check for coding conventions; do not rename it as constitution evidence.

**Acceptance criteria:** Both impl-plan copies require the exact heading and mapping behavior; G1 scenario 3 passes.

## Dependency Graph

`G1 RED → G2 → G3`; `G1 RED → G4`; then `G1 GREEN`. G2 and G4 may be implemented independently after the red test.

## TDD and Quality-gate Sequence

1. Add G1 and run the targeted integration test; record the expected RED marker.
2. Apply G2-G4 in dependency order.
3. Run the targeted test and record GREEN.
4. Run `go test ./...`.
5. Run `go vet ./...`.
6. Run `golangci-lint run`.
7. Run `gofmt -l .` and require no output.
8. Run `go run ./cmd/backlogit docs lint` and require zero tracked-plan violations in a clean checkout.
9. Verify no unresolved `{{...}}` variables and all referenced reviewer files exist.

## Decisions and Rationale

- **Enable, do not waive by default:** repository depth and tool declarations prove the formal topology is viable.
- **Semantic capability preflight:** runtimes expose different concrete tool names.
- **Evidence in the plan:** harvest consumes the plan, so gate evidence must travel with it.
- **WAIVED is not PASS:** this prevents exceptions from becoming counterfeit review results.
- **Contract test over exact parity:** copies have distinct environment metadata but must share governance semantics.

## Risks and Caveats

- Documentation alone cannot force a host to expose a missing tool; it can only detect, report, and halt.
- A supported-environment smoke run with actual persona dispatch is required before claiming runtime enablement complete.
- G3 must not accidentally permit the current bootstrap waiver globally; the implementation defines validation, not a standing authorization.
- Changes affect agent governance and therefore require hardening.

## Plan Hardening Signals

- **Public API, schema, or contract change:** present — agent/skill workflow contract changes.
- **Security, auth, permission, or compliance:** present — governance/compliance gate provenance.
- **Migration, backfill, destructive action:** absent.
- **External integration or operator checkpoint:** present — runtime dispatch capability and operator waiver authorization.
- **High runtime, rollout, or rollback risk:** moderate — a bad rule could block or bypass all Stage harvests.

Requires plan hardening: yes

## Plan Hardening

**Mode:** careful and investigate-first.

**Protected invariants:** no silent review skip; no false persona attribution; formal P0/P1 blocks harvest; waiver is explicit and single-use; Stage does not implement or ship work.

**ProposedAction:** Change the planning-gate contract in mirrored installed/plugin instruction surfaces.
**ActionRisk:** Moderate. A malformed gate could deadlock staging or normalize bypass.
**ActionResult:** Planned only; implementation must prove structural contract tests and a supported-runtime formal dispatch smoke run.

**Verification reinforcement:**

- Negative fixtures/checks cover missing dispatch, one missing persona return, expired waiver, scope-mismatched waiver, and inline-only review.
- Positive formal smoke evidence must contain actual persona outputs, not a synthesized caller summary.
- Positive waiver smoke is valid only with a fresh operator authorization for that plan.

**Rollback:** revert the instruction/test commit. No data migration exists. Rollback owner is Ship; validation window is the first Stage run after merge. Trigger rollback or urgent fix if a formal run is mislabeled, P0/P1 fails open, or every supported dispatch environment is blocked.

## Runtime Verification and Closure

This changes agent runtime behavior. Ship must run a supported-environment Stage/plan-review smoke that dispatches at least the always-on personas, verifies attributed returns in the appended plan section, and proves a missing persona blocks harvest. A non-dispatch environment must show blocked status without a fresh waiver. Closure records tested runtime/tool name, reviewer count, gate verdict, and rollback decision.

## Constitution Check

- **I:** Go is limited to the integration test; normal safety gates apply.
- **II (NON-NEGOTIABLE):** G1 is written and observed RED before instruction changes, then GREEN.
- **III/IV (NON-NEGOTIABLE containment):** only listed in-repo paths may change; external templates are excluded.
- **V:** every dispatch, return, verdict, and waiver has durable evidence.
- **VI:** no dependency is added; each unit has one concern.
- **VII (NON-NEGOTIABLE):** no destructive operation is planned.
- **VIII:** hardening and careful/investigate-first posture are explicit.
- **IX:** Markdown gate evidence is reviewable and merge-friendly.
- **X:** compact evidence fields avoid transcript dependence.
- **XI:** downstream PR must use a merge commit; Stage does not merge.

No constitutional violation is planned. The bootstrap review waiver below is an explicit workflow exception authorized by the operator, not a waiver of any NON-NEGOTIABLE constitutional principle.

## Plan Review

### Gate Decision: WAIVED — bootstrap only

**Formal plan-review provenance:** NOT RUN. This Stage invocation has no agent/task dispatch tool, so no reviewer persona subagent was spawned and no independent persona output exists. The following is a single-agent structured assessment and must not be represented as formal multi-persona evidence.

**Bootstrap authorization:** the operator's 2026-07-14 `stage next` request explicitly directed plan/review/harvest in the known non-dispatch environment.
**Scope:** this plan only.
**Missing capability:** semantic agent subagent dispatch (`agent` / `agent/runSubagent`).
**Reason:** bootstrap the fail-closed dispatch/waiver contract itself.
**Authorizer:** operator, via the current request.
**Issued:** 2026-07-14T18:35:00Z.
**Expiry:** single use; expires immediately after this plan is harvested in this Stage session.
**Residual risk acknowledged:** independent personas did not review this plan; staged PR hosted review is supplemental only.
**Disposition:** harvest is authorized once under WAIVED mode; future plans receive no authorization from this record.

### Informal Single-agent Structured Assessment

- **Constitution lens:** PASS with explicit TDD, containment, observability, hardening, and no destructive action.
- **Go lens:** PASS; one small table-driven integration test, no production Go.
- **Scope lens:** PASS; seven listed files across four units, each unit under the file/effort limits.
- **Learnings lens:** applies `docs/compound/2026-07-14-github-plugin-skill-parity-test-gap.md` and avoids claiming one mirror covers the other.
- **Architecture lens:** PASS; host capability remains host-owned while instructions own detection/evidence.
- **Agent-native parity lens:** PASS with the requirement that semantic capability detection tolerate abstract and concrete tool names.

No P0/P1 finding was identified by this non-formal assessment. This statement is not a formal gate PASS.
