---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Planning governance gates implementation plan'
source: docs/exec-plans/2026-07-14-planning-governance-gates-plan.md
doc_type: plan
description: 'Implementation plan for attributable formal plan-review dispatch, fail-closed fresh-waiver consumption, bypass reconciliation, and explicit impl-plan Constitution Checks.'
docline:
    date: 2026-07-14T18:35:00Z
    origin: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
        - 823BADF4
        - DB1F9026
    review_state: blocked
---

# Planning Governance Gates Implementation Plan

## Problem Frame

The repository declares a supported reviewer dispatch topology, but plan-review does not preflight capability or define auditable evidence, and Stage does not distinguish formal PASS from informal assessment. Existing `skip_review` and `force_harvest_no_gates` paths can bypass provenance. Future waiver language also needs durable one-time consumption, while current plans have no waiver at all. Separately, impl-plan omits the constitution's mandatory labeled section and the generated `.github` targets need an external-template handoff.

**Origin:** `docs/decisions/2026-07-14-plan-review-governance-deliberation.md`; backlog deliberation `052-DL`.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| G1 | Prefer real independent reviewer dispatch | Unit G2 defines capability preflight and dispatch protocol in both plan-review copies. |
| G2 | Prove the formal gate ran | Unit G2 requires attributed dispatch/return outputs for every required persona. |
| G3 | Failure blocks harvest | Unit G3 accepts only complete formal evidence or a fresh, valid, reserved waiver. |
| G4 | Unsupported invocation is explicit | Units G2/G3 record unavailable capability and halt by default. |
| G5 | Future waiver is narrow and auditable | Units G2/G3 require operator, plan/digest, reason, issue/expiry, risk, and unique waiver ID. |
| G6 | Future waiver cannot be reused | Unit G3 reserves before harvest, records consumed-at/harvest IDs afterward, and rejects reserved/consumed IDs. |
| G7 | Inline and hosted reviews do not impersonate personas | Contract tests and instructions label them supplemental/informal only. |
| G8 | Legacy bypasses cannot evade provenance | Unit G3 routes `skip_review` and `force_harvest_no_gates` through fresh-waiver validation or blocks. |
| G9 | Plans contain `## Constitution Check` | Unit G4 adds the exact section and principle mapping contract to both impl-plan copies. |
| G10 | Mirrored surfaces stay guarded | Unit G1 checks both `.github` and plugin copies for required semantics. |
| G11 | Generated sources receive upstream closure | Stash `823BADF4` tracks all three external autoharness templates; closure requires regeneration verification. |
| G12 | Blocked shipments remain fail-closed and resumable | Stash `DB1F9026` tracks supported atomic shipment hold/requeue lifecycle; current manifests cannot re-enter `ClaimShipment` until it lands. |

## Scope Boundaries

### In Scope

- `.github` and plugin plan-review skill copies.
- `.github` and plugin Stage agent copies.
- `.github` and plugin impl-plan skill copies.
- One integration contract test.
- In-repo handoff documentation and stash traceability for generated sources.

### Out of Scope

- Reviewer persona implementation changes.
- New agent-runtime or API dispatch code.
- Treating GitHub Copilot review as formal plan-review evidence.
- Writing external autoharness templates from this workspace.
- Mixing stash `823BADF4` into an in-repo shipment.
- Mixing width-isolated shipment lifecycle stash `DB1F9026` into governance shipment `094-S`.
- Retrofitting old plan-review artifacts.

## Implementation Units

### Unit G1: Add planning-governance contract test

**Files:** `tests/integration/planning_governance_contract_test.go`
**Effort:** S, under 2 hours; 1 file; 3 grouped scenarios with table-driven negative cases.
**Skill domain:** Go integration test.
**Execution posture:** test-first.
**Dependencies:** none.

Create semantic-marker checks over each mirrored pair. Do not assert exact whole-file equality because environment-specific frontmatter/prose differs. Grouped scenarios:

1. both plan-review copies contain dispatch preflight, required reviewer evidence, FAIL-on-incomplete dispatch, future fresh-waiver fields, canonical SHA-256 digest rules, reservation/consumption fields, and no inline/hosted substitution;
2. both Stage copies require complete gate evidence, reject existing waiver reservation/consumption, and negatively prove `skip_review`, `force_harvest_no_gates`, and both together cannot reach harvest without a new explicit valid waiver;
3. both impl-plan copies require the exact `## Constitution Check` heading and principle/deviation mapping.

**RED:** `go test ./tests/integration -run TestPlanningGovernanceContracts -count=1` fails on current markers.
**GREEN:** after G2-G4, the same command passes and names both copies in subtests.

**Acceptance criteria:** Path-specific failures; explicit negative cases for waiver reuse and both legacy bypass inputs; fewer than four grouped scenarios.

### Unit G2: Define formal dispatch and future waiver contract

**Files:** `.github/skills/plan-review/SKILL.md`, `plugin/skills/plan-review/SKILL.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** plan-review skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1.

Add a capability preflight before persona dispatch, recognizing semantic subagent capability including known `agent` and `agent/runSubagent` names. If unavailable, append BLOCKED provenance and halt unless the operator supplies a new explicit plan-scoped waiver. Never infer authorization from a workflow command and never substitute inline assessment.

Formal mode requires every always-on and triggered persona to return. Append persona, agent definition, dispatch identifier/status, model/provider or `unknown`, finding count, output/disposition, and merged severity. Missing or failed required returns produce FAIL.

A future waiver request requires a unique `waiver_id`, exact plan path and digest, operator/authorizer and explicit authorization reference, missing capability, reason, issue/expiry timestamps, acknowledged residual risk, and intended disposition. Decision is WAIVED, never PASS. This contract does not create a waiver for the current plan.

Define the digest as lowercase hexadecimal SHA-256 over the exact UTF-8 bytes from byte zero up to, but excluding, the exact heading `## Operator Waiver Ledger` and its immediately preceding separator line break; when no ledger exists, hash the entire file. Reservation appends exactly one additional line-break sequence matching the file before that heading, then the ledger, without changing any pre-existing byte. The excluded suffix starts at the first byte of that added line break. Every reservation, validation, and consumption recomputes the same ledger-excluded range; any earlier plan edit changes the digest and invalidates the waiver, while ledger state updates do not.

**Acceptance criteria:** Both copies describe executable formal dispatch, attributed evidence, fail-closed behavior, and a fresh future-waiver contract; G1 scenario 1 passes.

### Unit G3: Enforce provenance, consumption, and bypass closure

**Files:** `.github/agents/.stage.agent.md`, `plugin/agents/stage.agent.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** Stage agent instruction authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1, G2.

At pre-harvest, accept only formal PASS, formal ADVISORY plus explicit operator proceed, or a fresh valid operator waiver. A missing tool, informal/hosted review, absent section, formal FAIL, or malformed/expired waiver blocks harvest.

For a future waiver, Stage must:

1. reject generic workflow commands as authorization;
2. verify scope/path and the lowercase SHA-256 digest over the ledger-excluded canonical byte range, then confirm no matching reservation/consumption record;
3. append a durable pre-harvest record to the plan with `waiver_id`, `state: reserved`, `reserved_at`, `reserved_by_stage_session`, and plan digest before any backlog mutation;
4. treat a failed/partial harvest as consumed-for-safety because the reservation remains;
5. after successful harvest, update the record to `state: consumed` with `consumed_at`, `consumed_by_harvest_ids`, and `shipment_id`;
6. reject any later use of a reserved or consumed waiver ID.

Retain `skip_review` and `force_harvest_no_gates` only as compatibility requests to enter this exact waiver-validation path, or remove them. Neither flag, alone or together, may suppress evidence checks or reach harvest without a fresh explicit waiver and reservation. Preserve review-fix cycle limits.

**Acceptance criteria:** Both Stage copies fail closed; reuse is durably rejected; negative checks cover each bypass input and their combination; G1 scenario 2 passes.

### Unit G4: Emit an explicit Constitution Check

**Files:** `.github/skills/impl-plan/SKILL.md`, `plugin/skills/impl-plan/SKILL.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** impl-plan skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1.

Add `## Constitution Check` to the required plan structure and quality criteria. Require mapping of every applicable principle, explicit treatment of NON-NEGOTIABLE principles, any conflict/waiver with justification and rejected simpler alternative, and `No violations or exceptions` when clean. Keep Standards Check distinct.

**Acceptance criteria:** Both copies require the exact heading and mapping behavior; G1 scenario 3 passes.

## Dependency Graph

`G1 RED → G2 → G3`; `G1 RED → G4`; then `G1 GREEN`. G2 and G4 may proceed independently after RED.

## TDD and Quality-gate Sequence

1. Add G1 and record expected RED, including reuse and bypass negative failures.
2. Apply G2-G4 in dependency order.
3. Run the targeted test and record GREEN.
4. Run `go test ./...`.
5. Run `go vet ./...`.
6. Run `golangci-lint run`.
7. Run `gofmt -l .` and require no output.
8. Run `go run ./cmd/backlogit docs lint`.
9. Verify no unresolved `{{...}}` variables and all reviewer references exist.
10. In a supported runtime, prove actual formal dispatch and prove missing persona, reused waiver, `skip_review`, and `force_harvest_no_gates` all block correctly.

## Decisions and Rationale

- **Enable, do not waive by default:** repository depth and tool declarations prove formal topology is viable.
- **Semantic capability preflight:** runtimes expose different tool names.
- **Evidence in the plan:** harvest consumes the plan, so evidence travels with it.
- **Reservation before mutation:** a crash remains fail-closed and cannot enable reuse.
- **Compatibility flags are not bypasses:** old names may route to waiver validation only.
- **WAIVED is not PASS:** exceptions cannot counterfeit review.
- **Contract test over exact parity:** mirrors have different metadata but share governance semantics.

## Upstream Template Handoff and Closure

Stash `823BADF4` tracks external Principle IV-bounded changes to:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`

Do not add this out-of-tree work to shipment `094-S` or `095-S`. Governance closure requires the external changes to land, regeneration of all three `.github` targets, and verification that dispatch, consumption, bypass, and Constitution Check contracts survive regeneration.

## Shipment Hold and Requeue Prerequisite

Generic artifact update accepted `queued -> blocked` for shipments `094-S` and `095-S`, but the shipment lifecycle supports only `queued -> active` and `ClaimShipment` only accepts queued manifests. There is no supported `blocked -> queued` transition. Therefore the current blocked manifests are the safest fail-closed state required by the operator, but they are intentionally not directly claimable or resumable.

High-priority stash `DB1F9026` tracks width-isolated Go/CLI work for atomic `queued -> blocked -> queued` shipment lifecycle and tests proving requeue restores normal `ClaimShipment` member activation. Do not use generic `blocked -> active`, because it bypasses atomic claim activation. Do not add this lifecycle concern to shipment `094-S` or `095-S`.

Approval of either plan alone does not make its current shipment Ship-ready. After valid plan evidence exists, normal intake additionally requires `DB1F9026` to land and a supported requeue, or a new explicit operator-authorized replacement-shipment procedure that preserves every harvested artifact.
## Risks and Caveats

- Documentation cannot force a host to expose a tool; it can detect, report, and halt.
- A supported-runtime smoke run with actual persona outputs is required before runtime enablement is complete.
- Reservation may strand a waiver after a crash; fail closed and require a new explicit operator decision rather than clearing it automatically.
- External-template drift remains open until stash `823BADF4` closes.
- Changes affect agent governance and require hardening.

## Plan Hardening Signals

- **Public API, schema, or contract change:** present — agent/skill workflow contract changes.
- **Security, auth, permission, or compliance:** present — governance provenance and waiver authority.
- **Migration, backfill, destructive action:** absent.
- **External integration or operator checkpoint:** present — runtime dispatch, explicit operator waiver, external template handoff.
- **High runtime, rollout, or rollback risk:** moderate — a bad rule could block or bypass Stage harvests.

Requires plan hardening: yes

## Plan Hardening

**Mode:** careful and investigate-first.

**Protected invariants:** no silent review skip; no false attribution; P0/P1 blocks harvest; generic commands never authorize waiver; reservation/consumption prevents reuse; legacy bypass flags never evade the gate; Stage does not implement or ship work.

**ProposedAction:** Change planning-gate contracts in mirrored installed/plugin surfaces.
**ActionRisk:** Moderate. A malformed gate could deadlock staging or normalize bypass.
**ActionResult:** Planned only; current plan remains BLOCKED.

**Verification reinforcement:**

- Negative checks cover missing dispatch, missing persona, expired/scope-mismatched waiver, inline-only review, reused reserved/consumed waiver, `skip_review`, `force_harvest_no_gates`, and both bypass values together.
- Positive formal smoke evidence contains actual persona outputs.
- Positive future-waiver smoke requires a new explicit operator authorization, durable reservation, and consumed-at/harvest-ID record.

**Rollback:** revert instruction/test commit. No data migration exists. Rollback owner is Ship; validation window is the first Stage run after merge.

## Runtime Verification and Closure

This changes agent runtime behavior. Ship must run supported-environment formal dispatch, verify attributed returns, prove missing persona blocks harvest, and exercise future-waiver reservation/consumption only with new explicit authorization. Closure also requires stash `823BADF4` external-template landing and regeneration verification.

## Constitution Check

- **I:** Go is limited to the integration test; normal gates apply.
- **II (NON-NEGOTIABLE):** G1 is observed RED before instruction changes, then GREEN.
- **III/IV (NON-NEGOTIABLE containment):** only listed in-repo paths change; external templates are stash-only.
- **V:** dispatch, verdict, reservation, and consumption have durable evidence.
- **VI:** no dependency is added; units and external handoff are isolated.
- **VII (NON-NEGOTIABLE):** no destructive operation is planned or authorized.
- **VIII:** hardening and careful/investigate-first posture are explicit.
- **IX:** plan evidence is Git-friendly.
- **X:** compact fields avoid transcript dependence.
- **XI:** downstream PR uses merge commit; Stage does not merge.

No constitutional violation or current waiver exists.

## Plan Review

### Gate Decision: BLOCKED

**Formal plan-review provenance:** NOT RUN. This invocation has no agent/task dispatch tool, so no reviewer persona subagent was spawned and no independent persona output exists.

**Waiver authorization:** NONE. The operator's generic `stage next` command is not a waiver or approval signal.
**Missing capability:** semantic agent subagent dispatch (`agent` / `agent/runSubagent`).
**Current disposition:** shipment `094-S`, feature `105-F`, and all member tasks are blocked. No harvest or Ship readiness may be inferred from the preserved artifacts.
**Required plan-gate unblock:** either append successful formal multi-persona review evidence for this exact plan, or obtain a new explicit operator waiver naming this plan, scope, risk, expiry, and authorization, then reserve/consume it using the future contract. Ship intake remains separately blocked until supported requeue from stash `DB1F9026` lands or the operator explicitly authorizes artifact-preserving replacement shipment assembly.

### Informal Single-agent Assessment

An informal assessment exists for planning context only. It is not a formal gate verdict and cannot unblock harvest or Ship.
