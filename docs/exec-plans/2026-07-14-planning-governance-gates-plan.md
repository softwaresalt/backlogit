---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: 'Planning governance gates implementation plan'
source: docs/exec-plans/2026-07-14-planning-governance-gates-plan.md
doc_type: plan
description: 'Implementation plan for attributable formal review, atomic canonical-ledger waivers, fail-closed Stage and direct-harvest enforcement, and explicit Constitution Checks.'
docline:
    date: 2026-07-14T18:35:00Z
    origin: docs/decisions/2026-07-14-plan-review-governance-deliberation.md
    linked_stash_ids:
        - 8CD8F46A
        - CA877CD1
        - 823BADF4
        - DB1F9026
        - 3E12DC97
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
| G5 | Future waiver is narrow and auditable | Units G0A-G0C/G2 require operator, exact plan/digest, reason, issue/expiry, risk, mode, phases, and unique waiver ID. |
| G6 | Reservation is atomic and single-owner | G0B/G0C lock and compare-and-swap ledger state; exactly one concurrent run receives owner/token. |
| G7 | Owner and phase are checked before every mutation | G0C validation binds plan, digest, owner, opaque token, state, authorized phase, and expiry. |
| G8 | Completion timing matches the path | Stage-managed mode consumes after shipment assembly with shipment ID; direct mode consumes after harvest and forbids shipment ID. |
| G9 | Every non-ledger canonical plan byte is digest-bound | G0A uniquely parses one final ledger, hashes every other UTF-8/LF byte, and rejects ambiguous content. |
| G10 | Inline, hosted, and prose assertions are not evidence | Contract tests label inline/hosted review supplemental and reject direct-harvest cleared/ready assertions. |
| G11 | Legacy bypasses cannot evade provenance | Unit G3 routes `skip_review` and `force_harvest_no_gates` through atomic fresh-waiver validation or blocks. |
| G12 | Plans contain `## Constitution Check` | Unit G4 adds the exact section and principle mapping contract to both impl-plan copies. |
| G13 | Mirrored surfaces stay guarded | Unit G1 checks plan-review, Stage, harvest, and impl-plan `.github`/plugin copies. |
| G14 | Generated sources receive upstream closure | Stash `823BADF4` tracks all four external autoharness templates and regeneration parity. |
| G15 | Blocked shipments remain fail-closed | Stash `DB1F9026` tracks supported shipment hold/requeue lifecycle; current manifests cannot re-enter `ClaimShipment`. |
| G16 | Harvested provenance is repairable only through supported tooling | Stash `3E12DC97` tracks an atomic CLI repair because current CLI cannot safely stamp archived/source stash links. |

## Scope Boundaries

### In Scope

- Leaf Go waiver parser/digest and reservation packages with focused tests.
- Repository-native waiver reserve/validate/consume CLI with focused tests.
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
- Mixing stash `823BADF4`, `DB1F9026`, or `3E12DC97` into shipment `094-S` or `095-S`.
- Raw Markdown/JSONL repair of missing harvested-stash provenance.
- Retrofitting unrelated historical plan-review artifacts.

## Implementation Units

### Unit G0A: Implement canonical final-ledger parser and digest

**Files:** `internal/waiver/ledger.go`, `internal/waiver/ledger_test.go`
**Effort:** S, under 2 hours; 2 files; no more than four production/test functions total.
**Skill domain:** Go leaf package.
**Execution posture:** test-first after G1 contract RED.
**Dependencies:** G1 RED.

Implement Markdown-aware final-ledger parsing and canonical UTF-8/LF SHA-256 as specified in G2. Table tests cover fenced-example lookalikes, valid reserved/consumed ledgers, duplicate/missing/malformed/non-final/unknown/trailing variants, CRLF/LF equivalence, BOM/invalid UTF-8/bare CR, and exact non-ledger edit sensitivity.

### Unit G0B: Add atomic reservation ownership and consumption

**Files:** `internal/waiver/reservation.go`, `internal/waiver/reservation_test.go`
**Effort:** S, under 2 hours; 2 files; no more than four production/test functions total.
**Skill domain:** Go concurrency and atomic file state.
**Execution posture:** test-first.
**Dependencies:** G0A.

Use a plan-scoped cross-process lock under `.backlogit/locks/` plus compare-and-swap against expected digest/ledger state. Under lock, re-read and validate the canonical plan, write through temp-file + fsync + atomic rename, then re-read before returning. Reserve changes absent authorization state to `reserved` and returns immutable `reservation_owner` plus a cryptographically random opaque `reservation_token`. A concurrent reserve for the same waiver/plan must conflict; exactly one caller wins.

Owner validation checks waiver ID, plan path/digest, state, owner, token, expiry, completion mode, and authorized phase. Consume is a locked CAS from the same owning reservation to `consumed`. Wrong owner/token, state drift, expiry, plan edit, stale lock uncertainty, or partial failure fails closed. One table/concurrency test proves one winner, loser conflict, owner/phase checks, and no second consume.

### Unit G0C: Expose repository-native waiver lifecycle CLI

**Files:** `internal/cli/waiver.go`, `internal/cli/waiver_test.go`
**Effort:** S, under 2 hours; 2 files; no more than four production/test functions total.
**Skill domain:** CLI orchestration.
**Execution posture:** thin-adapter test-first.
**Dependencies:** G0B.

Expose reserve, validate, and consume actions backed only by G0B; skills must never implement check-then-write themselves. Return machine-readable owner/token/state/conflict results and typed non-zero failures. Validate is called immediately before each authorized backlog mutation. Tests cover concurrent winner, wrong owner/token, wrong phase, expired/digest-changed state, mode-specific completion fields, and consumed reuse.

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

Static contract negatives must cover duplicate, missing-in-waiver-mode, malformed, non-final, unknown-field, and trailing-content ledgers; fenced-example lookalikes; appended non-ledger EOF content; waiver reuse; concurrent reservation with exactly one winner; wrong owner/token/phase; both completion modes; and every direct or legacy bypass. A supported-runtime smoke separately invokes harvest with no valid provenance and proves no feature, task, dependency, link, or shipment mutation occurred.

**RED:** `go test ./tests/integration -run TestPlanningGovernanceContracts -count=1` fails on current markers.
**GREEN:** after G0A-G0C and G2-G5, the same command passes and names every mirrored path in subtests.

**Acceptance criteria:** Path-specific failures, all four grouped scenarios, explicit zero-mutation direct-harvest verification, and fewer than five functions.

### Unit G2: Define formal dispatch and canonical final-ledger waiver contract

**Files:** `.github/skills/plan-review/SKILL.md`, `plugin/skills/plan-review/SKILL.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** plan-review skill authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1, G0C.

Add capability preflight before persona dispatch. If semantic subagent capability, including known `agent` and `agent/runSubagent` names, is unavailable, append BLOCKED provenance and halt unless the operator supplies a new explicit plan-scoped waiver. Never infer authorization from workflow routing or substitute inline assessment.

Formal mode requires every always-on and triggered persona to return. Append persona, definition, dispatch identifier/status, model/provider or `unknown`, finding count, output/disposition, merged severity, and final verdict. Missing or failed required returns produce FAIL.

Future waiver mode requires unique `waiver_id`, exact plan path/digest, explicit authorization reference and authorizer, missing capability, reason, issue/expiry timestamps, acknowledged risk, completion mode, authorized phases, and intended disposition. Decision is WAIVED, never PASS. The current plans have no waiver.

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
completion_mode: stage_managed | direct_harvest
authorized_phases: [harvest, shipment_assembly] | [harvest]
reserved_at: <UTC>
reservation_owner: <run-id>
reservation_token: <opaque-random-token>
consumed_at: <UTC, consumed only>
consumed_by_harvest_ids: [<ids>, consumed only]
shipment_id: <id, stage_managed consumed only>
```
~~~

The Markdown-aware parser recognizes H2 headings only outside fenced code, so examples never count as ledgers. The ledger heading must occur exactly once outside a fence, be the final H2 section, contain exactly one fenced YAML mapping with unique known keys, and end at EOF immediately after the closing fence plus one terminal line ending. Duplicate headings/keys, missing ledger in waiver mode, missing or state-incompatible fields, malformed YAML, unknown fields, another heading, prose, whitespace, or bytes after the allowed terminal line ending all fail closed.

Canonical digest algorithm: decode valid UTF-8 without BOM, reject invalid UTF-8 or bare CR, and normalize CRLF to LF. The canonical document must end in exactly one LF. Before reservation, hash all canonical UTF-8/LF bytes with lowercase SHA-256. Reservation appends one separator line break matching the on-disk file plus the final ledger. On later validation, the Markdown-aware parser normalizes line endings, removes only the exact separator and validated ledger block, and hashes every remaining canonical byte. Every non-ledger content byte is bound while Windows and Unix checkouts yield the same digest. Later plan/review content must be inserted before the ledger, changes the digest, and requires a new explicit waiver; content appended after the ledger is rejected before hashing. Formal mode may omit the ledger. G0C performs append/reserve and every transition under the G0B lock/CAS; skills do not edit the ledger directly.

State invariants are strict: `reserved` forbids all consumed fields; `consumed` requires `consumed_at` and non-empty exact harvested IDs. `stage_managed` requires phases `[harvest, shipment_assembly]` and requires `shipment_id` only when consumed. `direct_harvest` requires phases `[harvest]`, forbids `shipment_id` in every state, consumes at the last harvest mutation, and cannot later authorize shipment assembly. Unknown modes, fields, phases, or combinations fail closed.

**Acceptance criteria:** Both copies define executable formal dispatch, the exact final-ledger parser and canonical digest, all fail-closed errors, and G1 scenario 1 passes.

### Unit G3: Enforce provenance, consumption, and bypass closure in Stage

**Files:** `.github/agents/.stage.agent.md`, `plugin/agents/stage.agent.md`
**Effort:** S, under 2 hours; 2 files.
**Skill domain:** Stage agent instruction authoring.
**Execution posture:** contract-first after G1 RED.
**Dependencies:** G1, G2.

At pre-harvest, accept only formal PASS, formal ADVISORY plus explicit operator proceed, or a fresh valid operator waiver. Missing tool/evidence, informal/hosted review, absent section, formal FAIL, expired authorization, or any ledger/digest error blocks harvest.

For waiver mode, Stage calls the G0C CLI to atomically reserve `completion_mode: stage_managed` with phases `harvest` and `shipment_assembly`; it never performs a separate check then write. Exactly one Stage/direct caller receives the owner/token. Stage and harvest call validate with the same owner/token immediately before every mutation. Wrong owner/token, conflict, expiry, phase mismatch, plan edit, or state drift blocks. Failed/partial work leaves the reservation fail-closed.

Stage keeps the reservation through harvest and shipment assembly. Only after the shipment manifest exists does Stage atomically consume it with `consumed_at`, exact harvested IDs, and required `shipment_id`. `skip_review` and `force_harvest_no_gates` may only request this exact path or be removed. Neither value alone or together suppresses evidence. Harvest independently re-reads and validates rather than trusting Stage.

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

Replace prose-only cleared/ready checks with independent fail-closed validation. On every invocation, harvest re-reads the exact plan and accepts only durable complete formal evidence or a valid atomic reservation. Caller/Stage claims are hints, never proof. Missing/expired/mismatched evidence, conflict, wrong owner/token/phase, invalid ledger, or consumed state halts before the first backlog mutation.

Centralize G0C validate immediately before every create, dependency, link, adoption, or shipment-membership mutation. In `stage_managed` mode, harvest validates the Stage owner/token, performs only harvest-phase mutations, returns exact IDs, and does not consume; Stage retains ownership through shipment assembly and consumes with required shipment ID. In `direct_harvest` mode, authorized phases contain only harvest, the direct owner consumes immediately after the last harvest mutation with exact IDs and no `shipment_id`, and that consumed waiver cannot authorize later shipment assembly.

Negative verification races Stage/direct reservation and requires exactly one winner; invokes harvest with missing evidence, prose-only readiness, wrong owner/token/phase, invalid ledger variants, and consumed waiver; then compares repository-native inventory before/after and requires losers/failures to make zero mutations.

**Acceptance criteria:** Both harvest copies enforce the same independent pre-mutation contract, direct invocation cannot bypass provenance, successful mode-specific consumption is durable, zero-mutation negatives pass, and G1 scenario 3 passes.

## Dependency Graph

`G1 RED → G0A → G0B → G0C → G2 → {G3, G5}`; `G1 RED → G4`; then `G1 GREEN`. Every unit remains one or two files and under five functions; G3 and G5 may proceed in parallel after G2.

## TDD and Quality-gate Sequence

1. Add G1 and record RED for final-ledger, atomic reservation, Stage bypass, direct-harvest, and Constitution Check contracts.
2. Apply G0A-G0C test-first, then G2, then G3/G5 in parallel; apply G4 independently after RED.
3. Prove concurrent Stage/direct reserve yields exactly one owner/token; validate wrong owner/token/phase and both completion modes.
4. Run the targeted test and require GREEN across all eight mirrored files.
5. In a supported runtime, invoke harvest directly with no evidence, prose-only readiness, invalid owner/token/phase, malformed/final-ledger variants, and consumed waiver; diff repository-native backlog state and require zero mutations.
6. Run positive `stage_managed` and `direct_harvest` mode smokes; direct completion must omit shipment ID and reject later shipment use.
7. Run a positive formal smoke with actual persona outputs; exercise waiver mode only after a new exact-plan operator authorization.
8. Run `go test ./...`.
9. Run `go vet ./...`.
10. Run `golangci-lint run`.
11. Run `gofmt -l .` and require no output.
12. Run `go run ./cmd/backlogit docs lint`.
13. Verify no unresolved `{{...}}` variables and all reviewer/template references exist.

## Decisions and Rationale

- **Enable, do not waive by default:** repository depth and tool declarations prove formal topology is viable.
- **Semantic capability preflight:** runtimes expose different tool names.
- **Evidence in the plan:** harvest consumes the plan, so evidence travels with it.
- **Atomic reserve, not check-then-write:** a plan lock plus CAS gives concurrent callers one owner/token; a crash remains fail-closed.
- **Two completion modes:** Stage-managed ownership spans shipment assembly; direct harvest consumes without a shipment ID and grants no later shipment authority.
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

Do not add this out-of-tree work to shipment `094-S` or `095-S`. Governance closure requires all four external changes to land, regeneration of all four `.github` targets, and parity verification that dispatch, canonical final-ledger digest, atomic owner/token/phase validation, both completion modes, legacy bypass, direct-harvest pre-mutation, and Constitution Check contracts survive regeneration.

## Shipment Hold and Requeue Prerequisite

Generic artifact update accepted `queued -> blocked` for shipments `094-S` and `095-S`, but the shipment lifecycle supports only `queued -> active` and `ClaimShipment` only accepts queued manifests. There is no supported `blocked -> queued` transition. Therefore the current blocked manifests are the safest fail-closed state required by the operator, but they are intentionally not directly claimable or resumable.

High-priority stash `DB1F9026` tracks width-isolated Go/CLI work for atomic `queued -> blocked -> queued` shipment lifecycle and tests proving requeue restores normal `ClaimShipment` member activation. Do not use generic `blocked -> active`, because it bypasses atomic claim activation. Do not add this lifecycle concern to shipment `094-S` or `095-S`.

Approval of either plan alone does not make its current shipment Ship-ready. After valid plan evidence exists, normal intake additionally requires `DB1F9026` to land and a supported requeue, or a new explicit operator-authorized replacement-shipment procedure that preserves every harvested artifact.

## Harvested Stash Provenance Hold

Repository sync rehydrates harvested-stash links only from artifact `custom_fields.source_stash_id`, but the current CLI cannot add that field or safely amend archived stash provenance. The preserved mappings are `8CD8F46A -> 105-F`, `CA877CD1 -> 105.004-T`, and `A4BE2FAD -> 106-F`. High-priority stash `3E12DC97` tracks an atomic supported repair command with post-sync verification. Do not edit `.backlogit/archive/stash.jsonl` or artifact Markdown directly; this remains an explicit traceability blocker rather than a claimed fix.

## Risks and Caveats

- Documentation cannot force a host to expose a tool; it can detect, report, and halt.
- A supported-runtime smoke run with actual persona outputs is required before runtime enablement is complete.
- Atomic reservation may strand a waiver after a crash; owner/token recovery never auto-clears it and a new explicit operator decision is required.
- External-template drift remains open until stash `823BADF4` closes across all four generated targets.
- Direct harvest remains a mutation bypass until both harvest copies enforce independent provenance.
- Missing harvested-stash provenance remains open until supported repair stash `3E12DC97` lands.
- Changes affect agent governance and repository-native waiver state, so hardening is required.

## Plan Hardening Signals

- **Public API, schema, or contract change:** present — agent/skill workflow plus repository-native CLI/state contract changes.
- **Security, auth, permission, or compliance:** present — governance provenance and waiver authority.
- **Migration, backfill, destructive action:** absent.
- **External integration or operator checkpoint:** present — runtime dispatch, explicit operator waiver, external template handoff.
- **High runtime, rollout, or rollback risk:** moderate — a bad rule could block or bypass Stage harvests.

Requires plan hardening: yes

## Plan Hardening

**Mode:** careful and investigate-first.

**Protected invariants:** no silent review skip; no false attribution; P0/P1 blocks harvest; generic commands never authorize waiver; lock/CAS reservation, owner/token validation, and mode-specific consumption prevent reuse; legacy bypass flags and direct harvest never evade the gate; the ledger is unique/final and every non-ledger canonical UTF-8/LF byte is hashed; Stage does not implement or ship work.

**ProposedAction:** Add a leaf Go parser/reservation lifecycle and change planning-gate contracts in mirrored installed/plugin surfaces.
**ActionRisk:** Moderate. A malformed gate could deadlock staging or normalize bypass.
**ActionResult:** Planned only; current plan remains BLOCKED.

**Verification reinforcement:**

- Negative checks cover missing dispatch/persona, expired or scope-mismatched waiver, duplicate/missing/malformed/non-final/unknown-field ledger, fenced-example lookalikes, trailing or appended EOF content, concurrent reservation, wrong owner/token/phase, invalid completion fields, inline/prose-only evidence, reused reserved/consumed waiver, direct harvest, `skip_review`, `force_harvest_no_gates`, and both legacy values together.
- Positive formal smoke evidence contains actual persona outputs.
- Positive future-waiver smokes require new explicit operator authorization, exactly one atomic owner/token, and mode-valid consumed-at/harvest-ID/shipment fields.

**Rollback:** revert instruction/test commit. No data migration exists. Rollback owner is Ship; validation window is the first Stage run after merge.

## Runtime Verification and Closure

This changes agent runtime behavior. Ship must run supported-environment formal dispatch, verify attributed returns, prove missing persona blocks harvest, and exercise atomic concurrent reservation, owner/token validation, and both completion modes only with new explicit authorization. Closure also requires direct-harvest zero-mutation smoke evidence and stash `823BADF4` landing/regeneration verification across all four generated targets.

## Constitution Check

- **I:** Go is limited to leaf waiver lifecycle code, its focused tests, a thin CLI adapter, and one integration contract test; normal gates apply.
- **II (NON-NEGOTIABLE):** G1 is observed RED; G0A-G0C are implemented test-first; all focused and full suites end GREEN.
- **III/IV (NON-NEGOTIABLE containment):** only listed in-repo paths change; external templates are stash-only.
- **V:** dispatch, verdict, atomic owner/token reservation, phase validation, and mode-specific consumption have durable evidence.
- **VI:** no dependency is added; parser, reservation, CLI, Stage, harvest, integration-test, impl-plan, and external handoff concerns remain width-isolated.
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
**Current disposition:** shipment `094-S`, feature `105-F`, and all eight member tasks are blocked. No harvest or Ship readiness may be inferred from the preserved artifacts.
**Required plan-gate unblock:** either append successful formal multi-persona review evidence for this exact plan, or obtain a new explicit operator waiver naming this plan, scope, risk, expiry, and authorization, then atomically reserve/consume it using the future owner/token and completion-mode contract. Ship intake remains separately blocked until supported requeue from stash `DB1F9026` lands or the operator explicitly authorizes artifact-preserving replacement shipment assembly.

### Informal Single-agent Assessment

An informal assessment exists for planning context only. It is not a formal gate verdict and cannot unblock harvest or Ship.
