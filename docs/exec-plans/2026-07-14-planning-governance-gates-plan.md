---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Planning governance gates implementation plan'
source: docs/exec-plans/2026-07-14-planning-governance-gates-plan.md
doc_type: plan
description: 'Implementation plan for attributable formal review, canonical final-ledger waivers, fail-closed Stage and direct-harvest enforcement, and explicit Constitution Checks.'
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

The repository declares a supported reviewer dispatch topology, but plan-review does not preflight capability or define auditable evidence, and Stage does not distinguish formal PASS from informal assessment. Existing `skip_review` and `force_harvest_no_gates` paths can bypass provenance, and both harvest skill copies can mutate backlog state after only a prose cleared/ready assertion. Future waiver language also needs durable one-time consumption, while current plans have no waiver at all. Separately, impl-plan omits the constitution's mandatory labeled section and the generated `.github` targets need an external-template handoff.

**Origin:** `docs/decisions/2026-07-14-plan-review-governance-deliberation.md`; backlog deliberation `052-DL`.

## Requirements Trace

| ID | Requirement | Implementation |
|---|---|---|
| G1 | Prefer real independent reviewer dispatch | Unit G2 defines capability preflight and dispatch protocol in both plan-review copies. |
| G2 | Prove the formal gate ran | Unit G2 requires attributed dispatch/return outputs for every required persona. |
| G3 | Failure blocks every harvest path | Units G3 and G5 independently validate durable provenance before Stage-mediated or direct-harvest mutation. |
| G4 | Unsupported invocation is explicit | Units G2, G3, and G5 record unavailable capability and halt by default. |
| G5 | Future waiver is narrow and auditable | Units G2/G3/G5 require operator, exact plan/digest, reason, issue/expiry, risk, and unique waiver ID. |
| G6 | Future waiver cannot be reused | Reserved or consumed IDs reject new mutation; successful harvest records consumed-at and harvested IDs. |
| G7 | Every non-ledger canonical plan byte is digest-bound | Unit G2 uniquely parses one final ledger block, hashes every other UTF-8/LF byte, and rejects duplicate, malformed, non-final, or trailing content. |
| G8 | Inline, hosted, and prose assertions are not evidence | Contract tests label inline/hosted review supplemental and reject direct-harvest cleared/ready assertions. |
| G9 | Legacy bypasses cannot evade provenance | Unit G3 routes `skip_review` and `force_harvest_no_gates` through fresh-waiver validation or blocks. |
| G10 | Plans contain `## Constitution Check` | Unit G4 adds the exact section and principle mapping contract to both impl-plan copies. |
| G11 | Mirrored surfaces stay guarded | Unit G1 checks plan-review, Stage, harvest, and impl-plan `.github`/plugin copies. |
| G12 | Generated sources receive upstream closure | Stash `823BADF4` tracks all four external autoharness templates and regeneration parity. |
| G13 | Blocked shipments remain fail-closed | Stash `DB1F9026` tracks supported shipment hold/requeue lifecycle; current manifests cannot re-enter `ClaimShipment`. |

## Scope Boundaries

### In Scope

- `.github` and plugin plan-review skill copies.
- `.github` and plugin Stage agent copies.
- `.github` and plugin harvest skill copies.
- `.github` and plugin impl-plan skill copies.
- One integration contract test with fewer than five functions.
- In-repo handoff documentation and stash traceability for all generated sources.

### Out of Scope

- Reviewer persona implementation changes.
- New agent-runtime or API dispatch code.
- Treating GitHub Copilot review, inline assessment, or prose cleared/ready text as formal evidence.
- Writing external autoharness templates from this workspace.
- Mixing stash `823BADF4` into an in-repo shipment.
- Mixing width-isolated shipment lifecycle stash `DB1F9026` into governance shipment `094-S`.
- Retrofitting old plan-review artifacts.

## Implementation Units

### Unit G1: Add planning-governance contract test

**Files:** `tests/integration/planning_governance_contract_test.go`
**Effort:** S, under 2 hours; 1 file; four grouped table scenarios; fewer than five test/helper functions.
**Skill domain:** Go integration test.
**Execution posture:** test-first.
**Dependencies:** none.

Create path-specific semantic-contract checks rather than whole-file equality. Four grouped scenarios:

1. both plan-review copies require real dispatch evidence and the uniquely parsed final-ledger/digest/error contract;
2. both Stage copies fail closed on missing evidence, invalid/reused waiver, `skip_review`, `force_harvest_no_gates`, and both flags together;
3. both harvest copies independently validate durable provenance immediately before every mutating command and reject direct invocation, prose-only cleared/ready, consumed waiver, or malformed ledger with zero backlog mutations;
4. both impl-plan copies require the exact `## Constitution Check` heading and principle/deviation mapping.

Static contract negatives must cover duplicate, missing-in-waiver-mode, malformed, non-final, unknown-field, and trailing-content ledgers; a ledger-like heading inside a fenced example that must be ignored; appended non-ledger EOF content; waiver reuse; and every direct or legacy bypass. A supported-runtime smoke separately invokes harvest with no valid provenance and proves no feature, task, dependency, link, or shipment mutation occurred.

**RED:** `go test ./tests/integration -run TestPlanningGovernanceContracts -count=1` fails on current markers.
**GREEN:** after G2-G5, the same command passes and names every mirrored path in subtests.

**Acceptance criteria:** Path-specific failures, all four grouped scenarios, explicit zero-mutation direct-harvest verification, and fewer than five functions.

### Unit G2: Define formal dispatch and canonical final-ledger waiver contract

**Files:** `.github/skills/plan-review/SKILL.md`, `plugin/skills/plan-review/SKILL.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** plan-review skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1.

Add capability preflight before persona dispatch. If semantic subagent capability, including known `agent` and `agent/runSubagent` names, is unavailable, append BLOCKED provenance and halt unless the operator supplies a new explicit plan-scoped waiver. Never infer authorization from workflow routing or substitute inline assessment.

Formal mode requires every always-on and triggered persona to return. Append persona, definition, dispatch identifier/status, model/provider or `unknown`, finding count, output/disposition, merged severity, and final verdict. Missing or failed required returns produce FAIL.

Future waiver mode requires unique `waiver_id`, exact plan path/digest, explicit authorization reference and authorizer, missing capability, reason, issue/expiry timestamps, acknowledged risk, and intended disposition. Decision is WAIVED, never PASS. The current plans have no waiver.

Use exactly one line-aware final section with this shape:

~~~markdown
## Operator Waiver Ledger

```yaml
waiver_id: <unique-id>
state: reserved | consumed
plan_path: <exact-path>
plan_digest_sha256: <lowercase-64-hex>
authorizer: <operator>
authorization_reference: <durable-reference>
missing_capability: <value>
reason: <value>
issued_at: <UTC>
expires_at: <UTC>
residual_risk: <value>
reserved_at: <UTC>
reserved_by_stage_session: <session-id>
consumed_at: <UTC, consumed only>
consumed_by_harvest_ids: [<ids>, consumed only]
shipment_id: <id, consumed only>
```
~~~

The Markdown-aware parser recognizes H2 headings only outside fenced code, so examples never count as ledgers. The ledger heading must occur exactly once outside a fence, be the final H2 section, contain exactly one fenced YAML mapping with unique known keys, and end at EOF immediately after the closing fence plus one terminal line ending. Duplicate headings/keys, missing ledger in waiver mode, missing or state-incompatible fields, malformed YAML, unknown fields, another heading, prose, whitespace, or bytes after the allowed terminal line ending all fail closed.

Canonical digest algorithm: decode valid UTF-8 without BOM, reject invalid UTF-8 or bare CR, and normalize CRLF to LF. The canonical document must end in exactly one LF. Before reservation, hash all canonical UTF-8/LF bytes with lowercase SHA-256. Reservation appends one separator line break matching the on-disk file plus the final ledger. On later validation, the Markdown-aware parser normalizes line endings, removes only the exact separator and validated ledger block, and hashes every remaining canonical byte. Every non-ledger content byte is bound while Windows and Unix checkouts yield the same digest. Later plan/review content must be inserted before the ledger, changes the digest, and requires a new explicit waiver; content appended after the ledger is rejected before hashing. Formal mode may omit the ledger.

**Acceptance criteria:** Both copies define executable formal dispatch, the exact final-ledger parser and canonical digest, all fail-closed errors, and G1 scenario 1 passes.

### Unit G3: Enforce provenance, consumption, and bypass closure in Stage

**Files:** `.github/agents/.stage.agent.md`, `plugin/agents/stage.agent.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** Stage agent instruction authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1, G2.

At pre-harvest, accept only formal PASS, formal ADVISORY plus explicit operator proceed, or a fresh valid operator waiver. Missing tool/evidence, informal/hosted review, absent section, formal FAIL, expired authorization, or any ledger/digest error blocks harvest.

For a future waiver, Stage validates scope, authorization, expiry, canonical digest, and unique final ledger; persists `state: reserved` before any backlog mutation; and rejects an existing reserved or consumed ID for new use. A failed/partial harvest leaves the reservation fail-closed. After success, update the same ledger to `state: consumed` with `consumed_at`, exact harvested IDs, and shipment ID.

`skip_review` and `force_harvest_no_gates` may only request this exact validation path or be removed. Neither value alone or together suppresses evidence. Stage passes an immutable exact-plan gate reference to harvest, but harvest independently re-reads and revalidates it.

**Acceptance criteria:** Both Stage copies fail closed, direct harvest receives no trusted caller-only assertion, all legacy bypass negatives pass, and G1 scenario 2 passes.

### Unit G4: Emit an explicit Constitution Check

**Files:** `.github/skills/impl-plan/SKILL.md`, `plugin/skills/impl-plan/SKILL.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** impl-plan skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1.

Add `## Constitution Check` to required plan structure and quality criteria. Require every applicable principle, explicit NON-NEGOTIABLE treatment, any conflict/waiver with justification and rejected simpler alternative, and `No violations or exceptions` when clean. Keep Standards Check distinct.

**Acceptance criteria:** Both copies require the exact heading and mapping behavior; G1 scenario 4 passes.

### Unit G5: Enforce provenance in direct harvest

**Files:** `.github/skills/harvest/SKILL.md`, `plugin/skills/harvest/SKILL.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** harvest skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1, G2.

Replace prose-only cleared/ready checks with independent fail-closed validation. On every invocation, harvest re-reads the exact plan and accepts only durable complete formal evidence or a valid exact-plan waiver ledger already in `state: reserved`. Caller/Stage claims are hints, never proof. An absent, expired, mismatched, duplicate, malformed, non-final, consumed, or reused record halts before the first backlog mutation.

Centralize the precondition in the workflow so it runs immediately before every create, dependency, link, adoption, or shipment-membership mutation. The immutable validated plan path/digest/mode must match throughout the run. A consumed record can verify prior completion but never authorizes a new mutation. Successful waiver harvest changes only the validated final ledger to `state: consumed` and records exact IDs; failures leave reservation fail-closed.

Negative verification directly invokes harvest with missing evidence, prose-only cleared/ready, invalid ledger variants, and a consumed waiver, then compares the repository-native backlog inventory before/after and requires zero mutations.

**Acceptance criteria:** Both harvest copies enforce the same independent pre-mutation contract, direct invocation cannot bypass provenance, successful consumption is durable, zero-mutation negatives pass, and G1 scenario 3 passes.

## Dependency Graph

`G1 RED → G2 → {G3, G5}`; `G1 RED → G4`; then `G1 GREEN`. G3 and G5 are separate two-file concerns and may proceed in parallel after G2.

## TDD and Quality-gate Sequence

1. Add G1 and record RED for final-ledger, Stage bypass, direct-harvest, and Constitution Check contracts.
2. Apply G2, then G3/G5 in parallel; apply G4 independently after RED.
3. Run the targeted test and require GREEN across all eight mirrored files.
4. In a supported runtime, invoke harvest directly with no evidence, prose-only readiness, malformed/final-ledger variants, and consumed waiver; diff repository-native backlog state and require zero mutations.
5. Run a positive formal smoke with actual persona outputs; exercise waiver mode only after a new exact-plan operator authorization.
6. Run `go test ./...`.
7. Run `go vet ./...`.
8. Run `golangci-lint run`.
9. Run `gofmt -l .` and require no output.
10. Run `go run ./cmd/backlogit docs lint`.
11. Verify no unresolved `{{...}}` variables and all reviewer/template references exist.

## Decisions and Rationale

- **Enable, do not waive by default:** repository depth and tool declarations prove formal topology is viable.
- **Semantic capability preflight:** runtimes expose different tool names.
- **Evidence in the plan:** harvest consumes the plan, so evidence travels with it.
- **Reservation before mutation:** a crash remains fail-closed and cannot enable reuse.
- **Defense at both entry points:** Stage validates before delegation and harvest independently validates before every mutation.
- **Compatibility flags are not bypasses:** old names may route to waiver validation only.
- **WAIVED is not PASS:** exceptions cannot counterfeit review.
- **Contract test over exact parity:** mirrors have different metadata but share governance semantics.

## Upstream Template Handoff and Closure

Stash `823BADF4` tracks external Principle IV-bounded changes to:

- `templates/agents/stage.agent.md.tmpl`
- `templates/skills/plan-review/SKILL.md.tmpl`
- `templates/skills/impl-plan/SKILL.md.tmpl`
- `templates/skills/harvest/SKILL.md.tmpl`

Do not add this out-of-tree work to shipment `094-S` or `095-S`. Governance closure requires all four external changes to land, regeneration of all four `.github` targets, and parity verification that dispatch, final-ledger digest, reservation/consumption, legacy bypass, direct-harvest pre-mutation, and Constitution Check contracts survive regeneration.

## Shipment Hold and Requeue Prerequisite

Generic artifact update accepted `queued -> blocked` for shipments `094-S` and `095-S`, but the shipment lifecycle supports only `queued -> active` and `ClaimShipment` only accepts queued manifests. There is no supported `blocked -> queued` transition. Therefore the current blocked manifests are the safest fail-closed state required by the operator, but they are intentionally not directly claimable or resumable.

High-priority stash `DB1F9026` tracks width-isolated Go/CLI work for atomic `queued -> blocked -> queued` shipment lifecycle and tests proving requeue restores normal `ClaimShipment` member activation. Do not use generic `blocked -> active`, because it bypasses atomic claim activation. Do not add this lifecycle concern to shipment `094-S` or `095-S`.

Approval of either plan alone does not make its current shipment Ship-ready. After valid plan evidence exists, normal intake additionally requires `DB1F9026` to land and a supported requeue, or a new explicit operator-authorized replacement-shipment procedure that preserves every harvested artifact.

## Risks and Caveats

- Documentation cannot force a host to expose a tool; it can detect, report, and halt.
- A supported-runtime smoke run with actual persona outputs is required before runtime enablement is complete.
- Reservation may strand a waiver after a crash; fail closed and require a new explicit operator decision rather than clearing it automatically.
- External-template drift remains open until stash `823BADF4` closes across all four generated targets.
- Direct harvest remains a mutation bypass until both harvest copies enforce independent provenance.
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

**Protected invariants:** no silent review skip; no false attribution; P0/P1 blocks harvest; generic commands never authorize waiver; reservation/consumption prevents reuse; legacy bypass flags and direct harvest never evade the gate; the ledger is unique/final and every non-ledger canonical UTF-8/LF byte is hashed; Stage does not implement or ship work.

**ProposedAction:** Change planning-gate contracts in mirrored installed/plugin surfaces.
**ActionRisk:** Moderate. A malformed gate could deadlock staging or normalize bypass.
**ActionResult:** Planned only; current plan remains BLOCKED.

**Verification reinforcement:**

- Negative checks cover missing dispatch/persona, expired or scope-mismatched waiver, duplicate/missing/malformed/non-final ledger, fenced-example lookalikes, trailing or appended EOF content, inline/prose-only evidence, reused reserved/consumed waiver, direct harvest, `skip_review`, `force_harvest_no_gates`, and both legacy values together.
- Positive formal smoke evidence contains actual persona outputs.
- Positive future-waiver smoke requires a new explicit operator authorization, durable reservation, and consumed-at/harvest-ID record.

**Rollback:** revert instruction/test commit. No data migration exists. Rollback owner is Ship; validation window is the first Stage run after merge.

## Runtime Verification and Closure

This changes agent runtime behavior. Ship must run supported-environment formal dispatch, verify attributed returns, prove missing persona blocks harvest, and exercise future-waiver reservation/consumption only with new explicit authorization. Closure also requires direct-harvest zero-mutation smoke evidence and stash `823BADF4` landing/regeneration verification across all four generated targets.

## Constitution Check

- **I:** Go is limited to the integration test; normal gates apply.
- **II (NON-NEGOTIABLE):** G1 is observed RED before instruction changes, then GREEN.
- **III/IV (NON-NEGOTIABLE containment):** only listed in-repo paths change; external templates are stash-only.
- **V:** dispatch, verdict, reservation, and consumption have durable evidence.
- **VI:** no dependency is added; Stage, harvest, test, impl-plan, and external handoff concerns remain width-isolated.
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
**Refinement authorization:** the operator authorized one plan/backlog refinement cycle only; this is not review evidence or waiver authorization.
**Current disposition:** shipment `094-S`, feature `105-F`, and all five member tasks are blocked. No harvest or Ship readiness may be inferred from the preserved artifacts.
**Required plan-gate unblock:** either append successful formal multi-persona review evidence for this exact plan, or obtain a new explicit operator waiver naming this plan, scope, risk, expiry, and authorization, then reserve/consume it using the future contract. Ship intake remains separately blocked until supported requeue from stash `DB1F9026` lands or the operator explicitly authorizes artifact-preserving replacement shipment assembly.

### Informal Single-agent Assessment

An informal assessment exists for planning context only. It is not a formal gate verdict and cannot unblock harvest or Ship.
