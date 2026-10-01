---
chunk_strategy: h1-h2-h3
description: "Plan for stash A592FC1C: a local-only upstream hand-off for the autoharness pipeline-topology numeric-adjacency predecessor defect, with bounded documentation tasks that record the partial supersession of the 6D53A33F disposition, review the hand-off before delivery, and record the operator delivery state; no external implementation"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-30-a592fc1c-pipeline-topology-upstream-handoff-plan.md
title: "Implementation Plan: Upstream hand-off for the pipeline-topology DAG predecessor defect (A592FC1C)"
docline:
    stash_id: A592FC1C
    status: draft
    created_at: 2026-09-30T17:20:00Z
---

## Objective

Hand the autoharness `pipeline-topology` numeric-adjacency defect to its
upstream owner with enough evidence, a bounded fix, and acceptance tests that
the owner can act without re-deriving the analysis. Record locally that the
2026-09-13 `6D53A33F` disposition is superseded in part.

The fix itself lives in the external autoharness workspace. This plan stages
documentation only and does not claim the defect is fixed. Delivering the
hand-off is an operator act outside this plan; the plan only reviews the
content and records the delivery state.

## Source and Intake Record

* Stash: `A592FC1C` (medium, bug, `DEFERRED SCOPE EXPANSION`). A task-shaped
  entry, so this plan synthesizes a covering feature.
* Deliberation (P-021 C6 satisfied, reused, not restarted): `074-DL`,
  `docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md`,
  assigns this defect the L3 precondition role. OQ-2 names the fixed
  `pipeline-topology` pre_claim gate as the long-term owner of (C), which
  needs a later upstream consumption check in addition to this fix.
* Duplicate scan (P-021 C5 (A)): clean. Related but distinct entries:
  `F05661B1` (gate closure awareness, excluded) and `9A8E1879` (supersession
  notice for a different 2026-09-06 numeric-gate record, excluded).
* Late-identifier reconciliation (P-021 C5 (B)): triggered by `task N/A`,
  `PR N/A`, and `review-thread N/A`. No Ship-owned residual-risk or closure
  record cites `A592FC1C` with a late identifier.
  `docs/closure/154-S-residual-risk-prepr.md` lists it only as an ambiguous
  candidate for a different expansion. Result: no late identifier found; the
  `N/A` values stand as truthful terminal records.
* Hand-off authored in this Stage session:
  `docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`.

## Problem Frame

The gate infers a shipment's predecessor from numeric ID adjacency and ignores
the backlogit `blocks` DAG. Commit `69e900be` made `155-S` a DAG root with
`154-S` depending on it, and the gate still blocked `155-S` behind `154-S`. The
advisory `dag-readiness` reporter agreed with the DAG. The 2026-09-13
disposition had called the mismatch presentation-only; the inverted block
proves a real deadlock. backlogit cannot fix the gate. It can only hand the
defect off accurately and keep its own records truthful.

## Scope

In scope:

* The hand-off document (authored by Stage, already present).
* U1: a partial-supersession notice in the 2026-09-13 disposition record.
* U2: a pre-delivery content review of the hand-off, recorded in it.
* U3: recording the upstream delivery state in the hand-off.

Out of scope:

* Any change to autoharness sources or tests, any local copy of them, and any
  write to the external autoharness filesystem.
* The delivery itself, which the operator performs.
* Any change to backlogit Go code, agent contracts, or configuration.
* `F05661B1` closure awareness and any other gate defect.
* The `9A8E1879` supersession notice for the 2026-09-06 record.
* Adding a consumption (C) check to the gate.

## Requirements Trace

| ID | Requirement | Source | Units |
|---|---|---|---|
| R1 | The hand-off states the defect, evidence (`69e900be`, inverted block, `dag-readiness` agreement), the requested DAG-derived fix, and bounded tests T1 to T4 with acceptance | A592FC1C | Hand-off (authored) |
| R2 | The hand-off states the 074-DL L3 role, that the fix alone does not transfer (C), and that a consumption check is a later upstream decision | 074-DL L3, OQ-2 | Hand-off (authored) |
| R3 | The 2026-09-13 disposition carries a notice that it is superseded in part, scoped to the predecessor-derivation conclusion, linking the hand-off | A592FC1C supersession | U1 |
| R4 | The hand-off passes a content review before delivery | Boundary hardening | U2 |
| R5 | The hand-off records whether the operator delivered it, through which channel, without claiming a fix | A592FC1C truthfulness | U3 |
| R6 | No agent writes to any external system; delivery is performed by the operator only | P-017 boundary | U2, U3 |
| R7 | The defect counts as corrected locally only when the installed autoharness version includes the fix and the T1 shape re-checks clean | A592FC1C truthfulness | Closure |

## Implementation Units

### U1: Partial-supersession notice in the 6D53A33F disposition (docs)

* File: `docs/decisions/2026-09-13-6d53a33f-autoharness-advisory-vs-preclaim-disposition.md`.
* Add a short "Superseded in part" section directly under the disposition
  heading. It names stash `A592FC1C`, links the hand-off document, states that
  only the "presentation-only" conclusion about numeric-adjacency predecessors
  is superseded, and states that the workspace-containment determination still
  stands. Do not rewrite the original text.
* Verify (docs RED): before the edit,
  `git grep -n "A592FC1C" -- docs/decisions/2026-09-13-6d53a33f-autoharness-advisory-vs-preclaim-disposition.md`
  returns no match. After the edit it returns the notice, and
  `go run ./cmd/backlogit docs lint --path docs/decisions/2026-09-13-6d53a33f-autoharness-advisory-vs-preclaim-disposition.md`
  reports zero violations.
* Posture: characterization-first.

### U2: Pre-delivery content review (docs)

* File: `docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`.
* Review the hand-off against this checklist and add a `## Delivery Record`
  section with the date and the result of each item:
  1. No secrets, tokens, or machine-local paths.
  2. No copied autoharness source or test code.
  3. Every statement about the upstream fix is phrased as a request or a
     recommendation, and the status line does not say "fixed" or "resolved".
  4. The evidence commit and shipment IDs match the live backlog.
  5. The hand-off contains no agent-run delivery command: no `gh` command,
     no API call, and no instruction for an agent to send or write anything
     outside the repository.
  6. Completeness: the hand-off still names T1 to T4, states that the fix
     does not transfer (C), and excludes `F05661B1`.
* The `## Delivery Record` section records the review date, the result of
  each item, and the commit SHA of the hand-off revision that was reviewed
  (`git log -1 --format=%H -- <hand-off path>`).
* Verify (docs RED): before the edit,
  `git grep -n "## Delivery Record" -- docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`
  returns no match. After the edit:
  * that command matches;
  * `git grep -n -i -e "fixed upstream" -e "resolved upstream" -e "gh issue" -- docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`
    returns no match;
  * `git grep -n -e "| T4 |" -e "does not transfer (C)" -e "F05661B1" -- docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`
    returns all three;
  * docs lint reports zero violations.
* Posture: characterization-first.

### U3: Record the upstream delivery state (docs)

* File: `docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`.
* Ask the operator for the delivery state. The operator delivers the hand-off
  through a channel of their choice. No agent writes to any external system:
  Ship does not open an issue, call an API, send a message, or write to the
  external autoharness filesystem. Ship only transcribes what the operator
  reports:
  * delivered: replace `pending-operator-delivery` with
    `delivered: <channel> <upstream reference>`, plus who approved the
    delivery and when, as the operator reported it;
  * not delivered: keep `pending-operator-delivery` and add the dated reason.
  Never write "fixed" or "resolved".
* Verify: before the edit,
  `git grep -n "pending-operator-delivery" -- docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md`
  matches. After the edit the status line carries either a channel,
  reference, approver, and time, or a dated reason; the U2 wording check
  (including `gh issue`) still returns no match; and docs lint reports zero
  violations.
* Posture: characterization-first.

## Dependency Graph

```text
U1 --> U2 --> U3
```

U2 follows U1 so the reviewed hand-off and the local supersession notice point
at each other, and U3 follows the review.

Shipment edges:

* The shipment carries a `blocks` edge onto `154-S` as an L1 scope marker.
  Under condition (b) and the L1 closure trigger, nothing whose `blocks`
  closure includes `154-S` routes until the `154-S` operator attestation
  exists. The engine treats the edge as satisfied today.
* It has no edge onto the `6434A4D7` or `AF1E5075` shipments, because it
  touches no shared files.

## Decisions and Rationale

* **Stage authors the hand-off; Ship runs the follow-ups.** The hand-off is a
  decision artifact, which Stage may create. Editing an existing decision
  record and recording an external delivery are execution steps, so Ship owns
  them.
* **Delivery is an operator act.** Sending material outside backlogit needs
  the operator, and no agent performs any external write, including opening
  an upstream issue. The plan never depends on delivery; U3 records what the
  operator reports either way.
* **Separate shipment.** The work is documentation in a different domain from
  the core code and harness contract. Bundling it would hold it behind
  unrelated review.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| A reader takes the hand-off as proof of a fix | Medium | The Status section, U2, and U3 forbid "fixed" wording |
| A reader assumes the fix retires the `154-S` attestation | Medium | The hand-off states the fix does not transfer (C) |
| The upstream owner keeps numeric adjacency as the default | Medium | The hand-off labels the default-off switch as a recommendation; the choice stays upstream |
| The supersession notice is read as voiding the whole 6D53A33F record | Low | U1 scopes the notice to one conclusion |
| Sensitive content leaves the workspace | Low | U2 reviews the content before delivery, and only the operator delivers |
| An agent performs the delivery itself | Low | U2 item 5 and the `gh issue` grep; U3 forbids any external write |

## Constitution Check

* I Safety-First Go and II Test-First: no Go code. Documentation units use a
  before and after `git grep` check as their RED step. Pass.
* III Workspace Isolation and IV CLI Workspace Containment: no write outside
  the repo. Pass.
* V to X: documentation only, Markdown in git. Pass.
* XI Merge Commit History: unaffected. Pass.
* Task Granularity: three single-file units. Pass.

Constitution Check: pass

## Plan Hardening Signals

* External system boundary (P-017) and a cross-workspace hand-off.
* Truthfulness risk: the local record could be mistaken for a fix.

Requires plan hardening: yes

## Runtime Verification and Closure

* Pre-merge: docs lint with zero violations on both files, and the U1 to U3
  `git grep` checks.
* Closure: the closure record states the upstream delivery state from U3 and
  that the defect stays open locally. backlogit treats it as corrected only
  after the installed autoharness version includes the upstream fix and a
  re-run of the gate against the T1 fixture shape returns the expected
  result. A report from the owner alone does not close it.

## Plan Hardening

Hardening required: yes, for boundary and truthfulness reasons rather than
code risk.

### Inputs

* Learnings: `docs/decisions/2026-09-13-6d53a33f-autoharness-advisory-vs-preclaim-disposition.md`
  (workspace-containment determination) and
  `docs/decisions/2026-09-27-autoharness-upstream-fix-proposals.md` (format
  precedent for upstream proposals).
* Instructions consulted: the Markdown and writing-style instructions and the
  constitution.

### Protected Invariants

* No file outside this repository is written.
* No agent writes to any external system.
* No backlogit record says the upstream defect is fixed until the installed
  autoharness version includes the fix and the T1 shape re-checks clean.
* The original 6D53A33F text is not rewritten; only a scoped notice is added.

### Risky Actions

| ProposedAction | ActionRisk | Approval | Rollback |
|---|---|---|---|
| PA1: operator delivery of the hand-off (outside the plan, recorded by U3) | Medium: external boundary | Only the operator performs it; no agent performs any external write | Not applicable; without delivery U3 records `pending-operator-delivery` with a dated reason |
| PA2: supersession notice (U1) | Low | Plan-review PASS | Revert the notice |

### Added Verification

* U2 reviews the content and checks the "fixed" wording with `git grep`.
* Blocked path: if the operator withholds delivery, U3 still completes by
  recording the reason; the shipment does not wait on the external owner.

### Review-Gate Capability

* Plan review must emit the literal markers `dispatch_mode:` and `decision:`.
  Expected `dispatch_mode: multi-agent-dispatch` with Constitution, Scope
  Boundary, Learnings, and Security Lens (external integration) personas.

### Unresolved Operator Decisions

Delivery is an operator act at execution time and does not block harvest.

## Plan Review

* review_attempt: 1
* reviewed_at: 2026-09-30T18:40:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: initial draft (U1 to U2)
* P1 findings: none specific to this plan; the gate failed for the bundle.
* P2 findings: delivery was framed as a plan step rather than an operator act,
  with no named channel and no ban on external filesystem writes; no
  pre-delivery content review; the "retire upstream tests" and "default off"
  items were not labeled as recommendations; the hand-off overstated the (C)
  ownership transfer; the `154-S` edge rationale; `rg` in verify commands.
* Disposition: all findings are addressed in the revision above (U1 to U3).

<!-- plan-review-attempt: 1 -->

## Plan Review

* review_attempt: 2
* reviewed_at: 2026-09-30T20:10:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: attempt-1 revision (U1 to U3)
* P1 findings: U3 and the hand-off Status line allowed Ship to run
  `gh issue create` with operator approval, which is an agent-run external
  write.
* P2 findings: U2 did not record the reviewed revision or check for an
  agent-run delivery command; no completeness checks for T4, the (C)
  non-transfer statement, and the `F05661B1` exclusion; the closure rule
  relied on an owner report rather than the installed version.
* Disposition: all findings are addressed in the revision above (R6, R7,
  U2 items 5 and 6, the U3 rewrite, and the closure rule).

<!-- plan-review-attempt: 2 -->

## Plan Review

* review_attempt: 3
* reviewed_at: 2026-09-30T21:30:00Z
* dispatch_mode: multi-agent-dispatch
* personas: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor,
  Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer,
  Security Lens Reviewer
* decision: FAIL
* reviewed_revision: attempt-2 revision (U1 to U3, R1 to R7)
* P1 findings: none specific to this plan; the gate failed for the bundle
  (Go Reviewer P1 findings on the `6434A4D7` Feature B units).
* P2 findings: none open; reviewers confirmed the attempt-2 findings are
  resolved.
* Disposition: the plan has no open findings of its own, but it stays
  unharvested because this session gates the bundle as one unit.
* Escalation: the review cycle limit is reached (attempt counter 3). The
  escalation route (gpt-6-sol, openai, xhigh) differs from the Stage route,
  but engram is degraded, so no analysis hand-off is possible:
  ESCALATION_DEGRADED. Stage halted for operator intervention. No harvest,
  shipment, or stash archive happened.

<!-- plan-review-attempt: 3 -->
