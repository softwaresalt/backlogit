---
chunk_strategy: h1-h2-h3
description: "Implementation plan for stash AF1E5075 with the 074-DL layer L1 contract folded in: Orchestrator Step 2 and Ship pre-claim refuse the hard hold labels, treat the conditional-approval label as provenance, document the machine go-signal, and enforce the (P)+(C) condition (b) check with a CONDITION_B_UNSATISFIED halt"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-30-af1e5075-condition-b-preclaim-harness-contract-plan.md
title: "Implementation Plan: Pre-claim hold-label refusal and condition (b) (P)+(C) harness contract (AF1E5075)"
docline:
    stash_id: AF1E5075
    status: draft
    created_at: 2026-09-30T17:20:00Z
---

## Objective

Change the Orchestrator and Ship claim contracts so that agent-mediated claims
honor two things they ignore today:

1. Shipment hold labels. `do-not-claim-until-convergence` and
   `bootstrap-bypass-unapproved` become hard claim refusals. The machine
   go-signal is documented.
2. Condition (b) from 074-DL. A shipment whose `blocks`-edge closure includes
   `154-S` is claimable only when `154-S` has shipped provenance (P), read from
   Markdown, and the operator attestation of scheduler consumption (C) exists.
   Otherwise the agent halts with `CONDITION_B_UNSATISFIED`.

This is layer L1 of the 074-DL design, folded into this release unit by the
operator decision OQ-3. It is governance only: a direct CLI or MCP claim can
bypass it. L1 performs its own (P) read and does not depend on L2. The L2
code guard (`6434A4D7`) adds (P) for every caller once it ships and the
installed binary is upgraded. No layer checks (C) in code today.

## Source and Intake Record

* Stash: `AF1E5075` (high, task, `DEFERRED SCOPE EXPANSION`). A task-shaped
  entry, so this plan synthesizes a covering feature.
* Deliberation (P-021 C6 satisfied, reused, not restarted): `074-DL`,
  `docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md`.
  The operator decided OQ-1 to OQ-4 on 2026-09-28. The stash text carries the
  folded L1 scope and the label-role clarification from PR #455 review thread
  `PRRT_kwDORzozKM6m8bVu`.
* Duplicate scan (P-021 C5 (A)): clean. No other active stash entry targets
  the Orchestrator Step 2 or Ship Step 0.5 label and condition (b) contract.
* Late-identifier reconciliation (P-021 C5 (B)): triggered by
  `review_thread N/A` and `task_ids N/A`. No Ship-owned residual-risk or
  closure record cites `AF1E5075` with a late identifier.
  `docs/closure/154-S-residual-risk-prepr.md` lists it only as an ambiguous
  candidate for a different expansion. Result: no late identifier found; the
  `N/A` values stand as truthful terminal records.
* Live state at planning time: `154-S` is archived with
  `archived_status: shipped` (merge `6d233d21`) and carries the labels
  `bootstrap-exception`, `convergence-prerequisite`, and
  `bootstrap-bypass-approved-conditional`. The hold label
  `do-not-claim-until-convergence` was removed on 2026-09-29 after the
  recorded `C29EBEE5` PASS. No operator attestation comment (C) exists on
  `154-S` yet, and `docs/closure/154-S-173-F-scheduler-baseline-marker-post-merge-closure.md`
  is `READY_WITH_CONDITIONS`.

## Problem Frame

Orchestrator Step 2 (`.github/agents/_orchestrator.agent.md`, "Route to Ship")
decides eligibility from status, `blocks` edges, and queue order. Its explicit
re-check covers only "no unshipped blocking predecessor". Ship Step 0.5
(`.github/agents/_ship.agent.md`, "Shipment Intake") validates status and
membership, then claims. Neither reads labels, and neither knows condition (b).

Two concrete gaps follow:

* Before `154-S` shipped, only a label, a banner, and an operator instruction
  held it. An early routed claim would have activated every `173-F` member.
* Every `blocks` edge onto `154-S` released when `154-S` reached any terminal
  status. Nothing in the contracts distinguishes shipped from abandoned, and
  nothing checks (C). Enforcement is manual today.

## Scope

In scope:

* Contract text in Orchestrator Step 2 and Ship Step 0.5, including the Ship
  direct-assembly fallback path, for hold-label refusal, the
  approval-provenance label and its waiver-condition source, the go-signal,
  and the condition (b) (P)+(C) check.
* A Stage Step 5.5 rule, mirrored in the Ship fallback path, that keeps new
  queued shipments inside the L1 trigger scope unless a valid attestation is
  proven.
* Contract tests in `tests/integration/` that pin the text, the ordering, and
  the state tables for all three agents.
* A drift record in `.autoharness/harness-manifest.yaml` for the three
  rendered agent files, marking the `154-S` clauses as a workspace-local
  overlay excluded from upstream template sync.

Out of scope:

* Any change to Go code. The (P) code guard is `6434A4D7`.
* Writing the `154-S` attestation comment. That is an operator act, and this
  plan must not invent it.
* Changing labels or comments on any existing shipment.
* `plugin/agents/ship.agent.md` (the condensed distributable mirror). See
  Decisions.
* The autoharness `pipeline-topology` gate (`A592FC1C`, upstream) and its
  closure awareness (`F05661B1`).
* Editing the agent contracts in this Stage session. Ship performs U5 to U8.

## Requirements Trace

| ID | Requirement | Source | Units |
|---|---|---|---|
| R1 | Both contracts treat `do-not-claim-until-convergence` and `bootstrap-bypass-unapproved` as hard claim refusal labels | AF1E5075 | U1, U5, U6 |
| R2 | Both contracts treat `bootstrap-bypass-approved-conditional` as approval provenance, not a refusal label. A claim needs no refusal label and every recorded waiver condition satisfied, read from the shipment's Markdown banner or a recorded operator comment, failing closed when the source is missing | AF1E5075 (PR #455 clarification) | U1, U5, U6 |
| R3 | Both contracts document the go-signal: removal of `do-not-claim-until-convergence` counts only after a recorded `C29EBEE5` PASS, read from the evidence source below and failing closed when it is missing. The approval label never bypasses a refusal label or condition (b), and dark mode gives no exemption | AF1E5075; 074-DL OQ-4 | U1, U5, U6 |
| R4 | For a shipment whose transitive `blocks`-edge closure includes `154-S`, read `154-S` provenance with the (P) Read Rule below and fail closed when it is missing | 074-DL L1 (1) | U2, U5, U6 |
| R5 | Require (P): `status: shipped`, or `archived` with `archived_status: shipped`. Abandoned and other terminal states do not count | 074-DL L1 (2) | U2, U5, U6 |
| R6 | Require (C): a valid, unrevoked operator attestation recorded after the `154-S` ship event, per the Attestation Rule below. A manifest-declared capability is not consumption proof | 074-DL L1 (3), OQ-2 | U2, U5, U6 |
| R7 | Halt with the exact token `CONDITION_B_UNSATISFIED` when (P) or (C) fails, and never fail open. Treat a core `shipment_predecessor_not_shipped` claim refusal as a terminal halt. Neither halt is retried; the Ship step 4a `CLAIM_NOT_OBSERVED` retry never applies to them | 074-DL L1 (4); 6434A4D7 | U2, U3, U5, U6 |
| R8 | The checks run before the first `TOPOLOGY_GATE: pre_claim` in Orchestrator Step 2, and in Ship before the branch creation gate and again immediately before every `backlogit_claim_shipment` call, including the fallback path | Time-of-check hardening | U3, U5, U6 |
| R9 | Stage gives every new queued shipment a `blocks` edge onto `154-S` unless a valid attestation is proven under the Attestation Rule. The Ship fallback path applies the same rule when it assembles a shipment | L1 trigger scope | U3, U4, U6, U7 |
| R10 | The rendered-file drift is recorded, and the `154-S` clauses are marked as a workspace-local overlay excluded from upstream template sync | Harness hygiene | U8 |
| R11 | The contract tests pin both the historical label state (`154-S` held by `do-not-claim-until-convergence`) and the current state (approval provenance only, waiver conditions recorded) | AF1E5075 | U1 |

### Label State Table (R1 to R3, R11)

| Labels present | Waiver source | Outcome |
|---|---|---|
| `do-not-claim-until-convergence` and `bootstrap-bypass-unapproved` | Any | Refuse |
| `do-not-claim-until-convergence` and `bootstrap-bypass-approved-conditional` | Any | Refuse |
| `bootstrap-bypass-approved-conditional` only | Recorded, every condition satisfied | Continue to condition (b) |
| `bootstrap-bypass-approved-conditional` only | Missing, or a condition unsatisfied | Refuse |

The second row is the historical `154-S` state. The third row is the state
after the hold label was removed.

### C29EBEE5 PASS Evidence Source (R3)

The go-signal counts only when the shipment's Markdown banner carries the
`Gate: C29EBEE5 — SATISFIED` record that cites the operator's
`operator_authorization: approved` on the plan's `## Plan Review` section. For
`154-S` that record is in `.backlogit/archive/154-S.md` (operator verbatim
"C29EBEE5 gate approved", 2026-09-29T14:40:58-07:00, relayed by the
Orchestrator). When the banner record is missing, the agent treats the hold
label removal as unverified and refuses the claim.

### (P) Read Rule (R4, R5)

* Locate the `154-S` Markdown file by ID: `.backlogit/queue/154-S.md` while it
  is live, `.backlogit/archive/154-S.md` once it is archived. Parse its YAML
  frontmatter `status` and `archived_status`.
* Do not use `backlogit_get_item`, `backlogit_query_sql`, or any other index
  read for (P). The index does not carry `archived_status`.
* A missing file, unparseable frontmatter, or a missing required field fails
  closed with `CONDITION_B_UNSATISFIED`.

### Attestation Rule (R6)

The contracts state this rule in substance, and U2 pins it:

* Source: the `154-S` item log, `.backlogit/logs/154-S.jsonl`. The log is the
  append-only source of truth for comments. The index also stores comments,
  but it can lag the log, so the check never relies on it. backlogit has no
  item-history read operation, so agents read the file directly.
* A candidate is a `comment` event whose text starts with the line
  `CONDITION_B_ATTESTED: <evidence>` or `CONDITION_B_REVOKED: <reason>`.
* Actor: exactly `operator`, or `operator (relayed by Orchestrator)` only when
  the comment quotes the operator's words verbatim. Agents never originate an
  attestation. The Orchestrator may relay one only at the operator's request.
* Time: compare RFC3339 instants. A candidate counts only when its timestamp is
  after the latest `shipment_status_changed` event to `shipped`. Any
  unparseable timestamp fails closed.
* Newest wins: the latest valid candidate decides. A newer
  `CONDITION_B_REVOKED` cancels an older attestation. When an attestation and
  a revocation share the same instant, the revocation wins.
* Item logs are git-ignored (`.gitignore` has `logs/`). On a clone without the
  log, the check fails closed and the operator re-attests. This is a
  documented, safe-failing gap, not a fail-open path. The closure record
  copies the accepted attestation's timestamp, actor, and leading line into
  tracked history.
* Format publication: before the operator attests, Ship shows the operator
  this leading-line format. An existing attestation that does not conform is
  invalid, and the operator re-attests. No agent writes the attestation.
* Ownership of (C) does not transfer when `A592FC1C` is fixed upstream. A
  correct predecessor derivation does not check consumption. The attestation
  stays required until a recorded operator or Stage decision confirms that the
  gate checks consumption.

## Implementation Units

Every unit follows the 2-hour rule and stays in one skill domain. Test units
come first. The contract tests read each agent file with `testRepoRoot`,
slice the named section, normalize whitespace, and assert literal tokens,
following `tests/integration/shipment_155_harness_contract_test.go`. Each test
function runs one subtest per agent, named `Orchestrator`, `Ship`, or `Stage`,
so later units can select subtests with `-run`.

Section slices:

* Orchestrator: from `### Step 2: Route to Ship` to `### Step 3: Iteration Decision`.
* Ship: from `### Step 0.5: Shipment Intake` to `### Validation Boundary`.
* Ship fallback sub-slice: from `**Fallback path` to `### Validation Boundary`.
* Stage: from `### Step 5.5: Shipment Assembly` to `### Step 5.6`.

### U1: Hold-label contract tests (tests)

* File: `tests/integration/af1e5075_claim_label_contract_test.go` (new).
* Scenarios, each asserted in the `Orchestrator` and `Ship` subtests:
  1. Label state table: the four rows of the Label State Table appear with
     their outcomes, both labels appear with the phrase
     `hard claim refusal label`, and the second row pins the historical
     `154-S` state.
  2. Provenance: `bootstrap-bypass-approved-conditional` appears with the
     phrase `approval provenance, not a refusal label`, the phrase
     `recorded waiver conditions`, and a fail-closed statement when the
     waiver source is missing.
  3. Go-signal and bypass: the literal
     `GO_SIGNAL: do-not-claim-until-convergence removed after a recorded C29EBEE5 PASS`,
     the literal `Gate: C29EBEE5 — SATISFIED`, the phrase `never bypasses`,
     and the phrase `no DARK_MODE exemption`.
* RED: `go test ./tests/integration -run '^TestAF1E5075ClaimLabelContract$' -count=1`
  fails.
* Posture: test-first.

### U2: Condition (b) contract tests (tests)

* File: `tests/integration/af1e5075_condition_b_contract_test.go` (new).
* Scenarios, each asserted in the `Orchestrator` and `Ship` subtests:
  1. Trigger and (P): the phrase `blocks-edge closure includes 154-S`, the
     paths `.backlogit/archive/154-S.md` and `.backlogit/queue/154-S.md`, the
     phrase `never an index read`, and the four-row state table: `154-S` not
     shipped, `CONDITION_B_UNSATISFIED`; archived without
     `archived_status: shipped`, `CONDITION_B_UNSATISFIED`; shipped without a
     valid attestation, `CONDITION_B_UNSATISFIED` (the current live state);
     shipped with a valid attestation, proceed.
  2. Attestation rule: the literals `CONDITION_B_ATTESTED:`,
     `CONDITION_B_REVOKED:`, `.backlogit/logs/154-S.jsonl`, `RFC3339`,
     `newest wins`, `revocation wins`, `operator (relayed by Orchestrator)`,
     and `verbatim`.
  3. Boundaries: the phrase `manifest-declared capability is not consumption proof`,
     the phrase `does not transfer (C)`, the phrase `terminal halt` together
     with `shipment_predecessor_not_shipped`, and the phrase
     `workspace-local overlay`.
* RED: `go test ./tests/integration -run '^TestAF1E5075ConditionBContract$' -count=1`
  fails.
* Posture: test-first.

### U3: Ordering contract tests (tests)

* File: `tests/integration/af1e5075_preclaim_order_contract_test.go` (new).
* Scenarios:
  1. `Orchestrator` subtest: the first `hard claim refusal label` and the
     first `CONDITION_B_UNSATISFIED` occur before the first
     `TOPOLOGY_GATE: pre_claim`.
  2. `Ship` subtest, main path: both checks occur before
     `**Branch Creation Gate`; the phrase
     `repeat the hold-label and condition (b) checks` occurs before
     `backlogit_claim_shipment` in item 4; and the phrase
     `never retried` with `CONDITION_B_UNSATISFIED` occurs before
     `CLAIM_NOT_OBSERVED`.
  3. `Ship` subtest, fallback sub-slice: the phrase `blocks edge onto 154-S`
     occurs before `backlogit_claim_shipment`, and the phrase
     `repeat the hold-label and condition (b) checks` occurs before the same
     claim call.
* RED: `go test ./tests/integration -run '^TestAF1E5075PreclaimOrder$' -count=1`
  fails.
* Posture: test-first.

### U4: Stage scope-edge contract tests (tests)

* File: `tests/integration/af1e5075_stage_scope_edge_contract_test.go` (new).
* Scenarios, in the `Stage` subtest:
  1. Default: the phrase `blocks edge onto 154-S`, the phrase
     `unless a valid CONDITION_B_ATTESTED attestation is proven`, and the
     path `.backlogit/logs/154-S.jsonl`.
  2. Revocation: the phrase
     `a later CONDITION_B_REVOKED restores the rule`.
* RED: `go test ./tests/integration -run '^TestAF1E5075StageScopeEdge$' -count=1`
  fails.
* Posture: test-first.

### U5: Orchestrator Step 2 contract change (harness)

* File: `.github/agents/_orchestrator.agent.md`.
* In Step 2 item 1, next to "Re-check eligibility before claim", add a
  hold-label sub-bullet with the Label State Table, the waiver source, and
  the go-signal and its evidence source (R1 to R3). Add a condition (b)
  sub-bullet with the (P) Read Rule, the state table, and the Attestation
  Rule (R4 to R7). Both are hard eligibility gates before item 3,
  `TOPOLOGY_GATE: pre_claim`, so a failure halts routing. Keep the existing
  dependency re-check text unchanged.
* Verify: `go test ./tests/integration -run '^TestAF1E5075(ClaimLabelContract|ConditionBContract|PreclaimOrder)$/^Orchestrator$' -count=1`
  passes.

### U6: Ship Step 0.5 contract change (harness)

* File: `.github/agents/_ship.agent.md`.
* Main path: add the same two checks, with the exact tokens from U5, between
  item 3 (covering-feature check) and item 3a (branch creation gate). In
  item 4, add `repeat the hold-label and condition (b) checks` immediately
  before `backlogit_claim_shipment`, and state that a
  `CONDITION_B_UNSATISFIED` halt or a `shipment_predecessor_not_shipped`
  refusal is a terminal halt that is never retried, so the step 4a
  `CLAIM_NOT_OBSERVED` retry does not apply.
* Fallback path: in item 3b, after `backlogit_create_shipment`, add the
  `blocks edge onto 154-S` rule from U7. Rewrite item 4 to name
  `backlogit_claim_shipment` explicitly and to repeat the checks before it.
* Format publication: state that Ship shows the operator the attestation
  leading-line format before relying on a proceed row.
* Verify: `go test ./tests/integration -run '^TestAF1E5075(ClaimLabelContract|ConditionBContract|PreclaimOrder)$/^Ship$' -count=1`
  passes.

### U7: Stage scope-edge rule (harness)

* File: `.github/agents/_stage.agent.md`.
* In Step 5.5, add: give every new queued shipment a `blocks` edge onto
  `154-S`, recorded through `backlogit_add_dependency`, unless a valid
  `CONDITION_B_ATTESTED` attestation is proven under the Attestation Rule
  (`.backlogit/logs/154-S.jsonl`). When proof is uncertain, add the edge. A
  later `CONDITION_B_REVOKED` restores the rule.
* Verify: `go test ./tests/integration -run '^TestAF1E5075' -count=1` passes.

### U8: Record harness drift and the workspace-local overlay (config)

* File: `.autoharness/harness-manifest.yaml`.
* Append to the `drift_reason` of the `_orchestrator.agent.md`,
  `_ship.agent.md`, and `_stage.agent.md` entries: "AF1E5075 hold-label
  refusal and condition (b) pre-claim contract; the 154-S clauses are a
  workspace-local overlay excluded from upstream template sync; generic
  hold-label text is an upstream template sync candidate". Leave
  `drift_allowed: true`.
* Verify: `git grep -n "AF1E5075 hold-label" -- .autoharness/harness-manifest.yaml`
  matches three times. Do not run `autoharness verify-workspace` blindly; if
  Ship runs it, the acceptance is `strict_schema_blockers=[]` and no warning
  beyond the two known advisory portability warnings.

## Dependency Graph

```text
U1 --> U5 --> U6 --> U7 --> U8
U2 --> U5
U3 --> U5
U4 --> U7
```

Shipment edges:

* The shipment carries a `blocks` edge onto `154-S` as an L1 scope marker.
  Under condition (b) and the L1 closure trigger, nothing whose `blocks`
  closure includes `154-S` routes until the `154-S` operator attestation
  exists. This shipment is therefore itself held by the manual 074-DL policy
  and lands after the attestation.
* The `6434A4D7` disposition shipment `blocks` on this shipment, because both
  edit `_orchestrator.agent.md`, `_stage.agent.md`, and the manifest.
* This shipment has no edge onto the `6434A4D7` readiness shipment. L1 does
  its own (P) read.

## Decisions and Rationale

* **Synthesized covering feature.** `AF1E5075` is task-shaped. Stage requires
  a covering feature, so this plan creates one titled for the contract change.
* **Both agents, identical tokens.** Ship can be invoked directly without the
  Orchestrator. A check that lives only in the Orchestrator would not cover a
  direct Ship session. The fallback path is covered for the same reason.
* **Text contract plus tests, no code.** L1 is agent governance by design.
  The tests pin literal tokens and order, so a re-render or edit cannot
  silently drop or reorder the check.
* **The L2 token is declared here first.** The contract names
  `shipment_predecessor_not_shipped`, which `6434A4D7` will emit. The
  `6434A4D7` plan pins the same literal. A change to the token must change
  both.
* **L1 lands after the attestation (option b).** The shipment sits behind
  `154-S`, so the live post-merge state can only show the "proceed" row. The
  three halt rows are verified in a disposable, git-ignored fixture. No
  operator exemption is requested.
* **Closure trigger plus a fail-safe Stage scope rule.** 074-DL scopes L1 to
  the `blocks` closure of `154-S`. Applying the check to every claim would
  exceed that decision. The Stage rule (R9) closes the gap for new shipments
  and defaults to adding the edge.
* **Workspace-local overlay.** The `154-S` clauses only make sense in this
  backlog. The drift record keeps them out of upstream template sync.
* **The mirror stays out of scope.** The hold labels and `154-S` are
  workspace-local vocabulary; no distributed file names them. The condensed
  `plugin/agents/ship.agent.md` has no consumer for them. No deferred
  expansion is captured.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| An autoharness re-render overwrites the edits | Medium | U8 records the drift; the contract tests fail on any re-render that drops or reorders the text |
| The attestation is forged by free-text actor or comment text | Medium | Exact actor values, a structured leading line, verbatim relay, and a newest-wins revoke token. The check stays governance-only, as 074-DL states for L1 |
| The operator attests in a format the rule rejects | Medium | Ship shows the format first; a non-conforming attestation is invalid and the operator re-attests |
| The attestation log is missing on a fresh clone | Medium | The check fails closed and the operator re-attests; the closure record copies the accepted attestation into tracked history |
| An agent reads (P) from the index and misses `archived_status` | Medium | The (P) Read Rule names the Markdown paths and bans index reads; U2 pins it |
| The step 4a retry re-attempts a condition (b) refusal | Low | U6 states the halt is never retried; U3 pins the order before `CLAIM_NOT_OBSERVED` |
| The check halts every shipment behind `154-S` until the attestation exists | High, intended | This matches the 074-DL manual policy. The halt is the designed outcome |
| A reader assumes the `A592FC1C` fix retires the attestation | Medium | The contract states the fix does not transfer (C) |
| Direct CLI or MCP claims bypass L1 | Certain, by design | L2 covers (P) in code once shipped and installed; (C) stays an operator attestation |

## Constitution Check

* I Safety-First Go: no production Go code; contract tests only. Pass.
* II Test-First: U1 to U4 are RED before U5 to U7. Pass.
* III Workspace Isolation and IV CLI Workspace Containment: tests read files
  under the repo root only. The runtime fixtures live in the git-ignored
  `logs/` directory inside the repo. Pass.
* V Observability: halts use exact tokens. Pass.
* VI Single Responsibility: one check per sub-bullet. Pass.
* VII Destructive Command Approval: the only removal is the throwaway fixture
  directory, which needs operator approval. Pass.
* VIII Safety Modes: the checks run in every mode, with no DARK_MODE
  exemption. Pass.
* IX Git-Friendly Persistence and X Context Efficiency: text edits only. The
  git-ignored log gap fails closed, and the closure copies the attestation
  into tracked history. Pass.
* XI Merge Commit History: unaffected. Pass.
* Task Granularity: eight units within the 2-hour rule, each test unit at
  three scenarios or fewer. Pass.

Constitution Check: pass

## Plan Hardening Signals

* Changes agent claim-routing contracts that gate every shipment.
* Fail-closed authorization semantics that are expected to halt routing.
* Rendered harness files with an upstream template source.

Requires plan hardening: yes

## Runtime Verification and Closure

* Pre-merge: `go test ./tests/integration -run '^TestAF1E5075' -count=1`,
  `go test ./... -count=1`, and `go run ./cmd/backlogit docs lint` with zero
  violations.
* Halt rows, in a disposable fixture, never the live backlog:
  * Location: `logs/af1e5075-fixture/<row>/`, which `.gitignore` already
    excludes. Ship seeds each row with the backlogit CLI run from inside that
    directory: `154-S` as queued, as archived abandoned, and as archived
    shipped without an attestation, plus one dependent shipment with a
    `blocks` edge onto `154-S`.
  * Pass definition: a Ship session invoked as `ship <dependent-shipment-id>`
    against the fixture, with an explicit instruction to stop before any
    claim, halts with `CONDITION_B_UNSATISFIED` before the branch creation
    gate and before any `backlogit_claim_shipment` call. Ship records the
    transcript excerpt for each row in the closure artifact.
  * Approval basis: the fixture is throwaway, so no live state changes.
    Removing the fixture directory afterwards needs operator approval.
* Proceed row, in the live workspace after merge: Ship first shows the
  operator the attestation format and checks the existing attestation against
  the rule. When it is valid, an Orchestrator dry routing pass for a shipment
  behind `154-S` reaches the step before `TOPOLOGY_GATE: pre_claim` and stops.
  No gate runs and no claim happens.
* Closure: the closure record copies the accepted attestation's timestamp,
  actor, and leading line, and states that L1 covers agent-mediated claims
  only, that direct claims depend on L2 for (P), and that (C) stays an
  operator attestation until a recorded decision says otherwise.

## Plan Hardening

Hardening required: yes. The plan changes the claim-routing contracts that gate
every shipment and adds a fail-closed check that is expected to halt routing.

### Inputs

* Learnings consulted (Step 1.8, `docs/compound/`):
  * `2026-07-20-ship-gate-descoped-archived-member-exemption.md`: the index
    omits `archived_status`; read Markdown, use an allowlist, fail closed.
  * `2026-07-17-backlogit-update-drops-archive-provenance.md`: archive
    provenance survives only through `ArchiveItem`; the update bug is fixed.
  * Prior claim-guard learnings: refusals belong in the core seam with no
    forgeable exemption; peek, then re-check under the lock.
* Instructions consulted: `.github/instructions/backlogit.instructions.md`,
  the Markdown and writing-style instructions, and the constitution.
* Live evidence: the `154-S` item log stores operator comments as
  `event_type: comment` with actors `operator` and
  `operator (relayed by Orchestrator)`, and the ship transition as a
  `shipment_status_changed` event with `status: shipped` at
  `2026-09-30T08:44:25-07:00`. The `154-S` banner carries the
  `Gate: C29EBEE5 — SATISFIED` record.

### Protected Invariants

* The existing dependency re-check, precedence rule, TOPOLOGY_GATE pre_claim
  and post_claim text, the step 4a retry carve-out, and the uninstalled-gate
  bootstrap exemption keep their wording and order.
* The new checks only add refusals. They never write labels, comments, or
  shipment state.
* No agent originates a `CONDITION_B_ATTESTED` comment.

### Risky Actions

| ProposedAction | ActionRisk | Approval | Rollback |
|---|---|---|---|
| PA1: Orchestrator Step 2 hard gates (U5) | High: halts routing for every shipment behind `154-S` until the attestation exists | Plan-review PASS; Ship review | Revert the U5 commit; the manual 074-DL policy resumes |
| PA2: Ship Step 0.5 hard gates (U6) | High: same scope for direct Ship sessions and the fallback path | Plan-review PASS; Ship review | Revert the U6 commit |
| PA3: Stage scope-edge rule (U7) | Medium: adds an edge to new shipments | Plan-review PASS | Revert the U7 commit |
| PA4: manifest drift record (U8) | Low | Plan-review PASS | Revert the entry |
| PA5: throwaway fixture under `logs/` | Low: git-ignored, no live state | Plan-review PASS; operator approval to remove | Remove the fixture directory |

### Added Verification

* U3 pins the ordering against `TOPOLOGY_GATE: pre_claim`, the branch
  creation gate, the step 4a retry, and every claim call, including the
  fallback path.
* Blocked path: if Ship cannot place the checks before the branch gate
  without reordering existing gate text, stop and return to Stage.

### Decision Added by Hardening

The interim (C) signal is an operator comment (074-DL OQ-2). Hardening makes
it machine-checkable through the Attestation Rule above. This narrows the
decided mechanism without changing it.

### Operational Closure

* Monitoring after merge: a routing pass that routes a shipment behind
  `154-S` without a valid attestation is a rollback trigger for PA1.
* Owner: the Ship session that merges the shipment. Validation window: the
  first routing pass after merge.

### Review-Gate Capability

* Plan review must emit the literal markers `dispatch_mode:` and `decision:`.
  Expected `dispatch_mode: multi-agent-dispatch` with Constitution, Scope
  Boundary, Learnings, Architecture, Agent-Native Parity, Go, and Security
  Lens personas.

### Unresolved Operator Decisions

None block harvest. Writing the `154-S` attestation is an operator act outside
this plan.

## Plan Review

* review_attempt: 1
* reviewed_at: 2026-09-30T18:40:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: initial draft (U1 to U5)
* P1 findings:
  * The post-merge halt check was unobservable, because the shipment sits
    behind `154-S` and nothing behind `154-S` routes before the attestation.
* P2 findings: the attestation rule was forgeable and had no revoke, time, or
  clone-gap handling; the (C) ownership transfer to `A592FC1C` was overstated;
  the trigger scope left new shipments uncovered; the `154-S` clause needed a
  workspace-local overlay note; the core claim refusal code had no defined
  handling; ordering assertions exceeded the scenario limit and had no exact
  slices or phrases; the waiver-condition source, the `C29EBEE5` PASS
  requirement, and the bypass and dark-mode interactions were missing; the
  "L2 covers (P)" rationale was false until `6434A4D7` ships.
* Disposition: all findings are addressed in the attempt-2 revision.

<!-- plan-review-attempt: 1 -->

## Plan Review

* review_attempt: 2
* reviewed_at: 2026-09-30T20:10:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: attempt-1 revision (U1 to U7)
* P1 findings: none specific to this plan; the gate failed for the bundle.
* P2 findings: runtime fixtures had no contained location and no exact pass
  definition, and the live proceed row did not stop before the gate; the
  attestation format was not published to the operator first; no label state
  table or historical-state requirement; no `C29EBEE5` PASS evidence source;
  the Ship fallback path assembled and claimed shipments without the checks;
  the Stage rule had no fail-safe default or revocation note; the (P) read
  path allowed an index read; the step 4a retry precedence was undefined.
* P3 findings: exact `-run` subtest patterns, a tracked copy of the
  attestation, the index-versus-log justification, the equal-instant tie,
  and the early declaration of the L2 token.
* Disposition: all findings are addressed in the revision above (U1 to U8,
  R11, the Label State Table, the evidence source, the (P) Read Rule, and the
  revised runtime verification).

<!-- plan-review-attempt: 2 -->

## Plan Review

* review_attempt: 3
* reviewed_at: 2026-09-30T21:30:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: attempt-2 revision (U1 to U8)
* P1 findings: none specific to this plan; the gate failed for the bundle
  (Go Reviewer P1 findings on the `6434A4D7` Feature B units).
* P2 findings: the Stage standing scope-edge rule (R9, U4, U7, and the Ship
  fallback edge) goes beyond the authorized L1 scope, so defer it to a stash
  entry or get operator acknowledgment (Scope); halt rows must run with
  Ship's working directory set to the fixture, and each transcript must show
  the row-specific failure (Constitution); name the safety mode for Ship
  execution of the agent-contract edits (Constitution).
* Disposition: open. A new Stage session must fix these findings, or the
  operator must approve them, before harvest.
* Escalation: the review cycle limit is reached (attempt counter 3). The
  escalation route (gpt-6-sol, openai, xhigh) differs from the Stage route,
  but engram is degraded, so no analysis hand-off is possible:
  ESCALATION_DEGRADED. Stage halted for operator intervention. No harvest,
  shipment, or stash archive happened.

<!-- plan-review-attempt: 3 -->
