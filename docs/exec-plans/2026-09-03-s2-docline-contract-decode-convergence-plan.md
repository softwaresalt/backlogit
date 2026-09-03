---
chunk_strategy: h1-h2-h3
description: "Execution plan for S2: docline report-contract array fix and decode-policy convergence"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s2-docline-contract-decode-convergence-plan.md
title: "S2 Execution Plan — docline Contract & Decode Convergence"
---

# S2 Execution Plan — docline Contract & Decode Convergence

**Covering feature**: docline report-contract (always-an-array) and decode-policy convergence
**Stash members**: EC987334, 1787FD85
**Tier**: reliability + simplifying refactor (shipment sequence S2)

## Problem Frame

Two internal/docline defects: MigrateReport collection fields vanish on a
zero-apply run (breaking the always-an-array JSON contract), and LintTree vs
PlanMigration carry two divergent decode policies over one frontmatter grammar.
Both are reliability/composability fixes staged ahead of feature work.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; errors wrapped |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | n/a (in-package) |
| IV. CLI Containment | n/a |
| V. Observability | Report shape made deterministic |
| VI. Single Responsibility | Converges on one decode helper |
| VII. Destructive Approval | none |
| VIII. Safety Modes | PlanMigration read-only report-and-continue is safe |
| IX. Git-Friendly | n/a |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Drop omitempty from MigrateReport collection fields (EC987334)
* Scope: remove omitempty from MigrateReport.Applied and MigrateReport.Skipped (internal/docline/report.go) so a zero-apply migrate report serializes both as `[]` not absent, matching the always-an-array contract (compound: 2026-07-21-omitempty-defeats-arrays-always-json-contract.md).
* Acceptance: a zero-apply MigrateReport marshals `"applied": []` and `"skipped": []`; test asserts presence on the zero case.

### U2 — Converge PlanMigration on classifyDecodeFailure (1787FD85)
* Scope: adopt the unexported classifyDecodeFailure(err) (rule string, fatal error) helper in PlanMigration so it reports-and-continues per-file like LintTree instead of aborting the whole corpus on the first decode failure. PlanMigration is read-only, so ApplyMigration's write-path all-or-nothing rationale does not apply.
* PRECONDITION: the classifyDecodeFailure helper (introduced by 146-F U8 in internal/docline/service.go) MUST already exist. Verify it is present before scheduling; if 146-F U8 has not landed, this unit's scope EXPANDS to first introduce the shared helper (with its containment/decode/read-failure/nil table test) so PlanMigration reuses it rather than writing a second decode policy.
* Acceptance: PlanMigration over a corpus with one undecodable file reports that file and continues over the rest; containment/decode/read-failure/nil table test locks the convergence; exactly one decode policy (the shared helper) exists; a check confirms no divergent second policy was introduced.

## Dependency Graph

U1 and U2 are independent (different files/behaviors). Order: U1, U2.

## Runtime Verification and Closure

U1 changes JSON output of migrate report; U2 changes PlanMigration behavior on
malformed input. Verification via table tests. Closure: regression tests.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Restore the migrate-report JSON contract so `applied` and `skipped` are always arrays | Medium — downstream agents and scripts may depend on the exact report shape | Normalize `MigrateReport.Applied` and `MigrateReport.Skipped` to non-nil empty slices before marshaling; tests must decode the JSON and assert both keys are zero-length arrays, not merely present sibling fields |
| Reuse one docline decode classifier in PlanMigration | Medium — a second policy could diverge from LintTree and create false report-and-continue behavior | Reuse the existing helper when present; if absent, introduce the shared helper first and cover containment, decode, read-failure, and nil cases with table tests |
| Add report-and-continue behavior for per-file decode failures | Medium — PlanMigration currently has no durable findings channel for a per-file failure | Add an explicit per-file findings/report channel before changing control flow; keep ApplyMigration all-or-nothing unchanged |

Rollback: revert to the previous report shape and PlanMigration abort behavior if
consumers cannot tolerate the restored contract. Compatibility: this is
contract-restoring and additive for consumers that already expected arrays, but
legacy consumers that distinguished absent from empty must be checked. Ownership:
`internal/docline` maintainers own the shared decode policy and report contract.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT-minor — U1 makes a promised array always present (contract-restoring, not breaking).
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: yes

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: FAIL

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Security Reviewer (`gpt-5.6-terra`) when risk-triggered for the plan
* Learnings Researcher over `docs/compound/`

Security Reviewer was not risk-triggered for this plan; all other mandatory personas ran.

Controlling P1 findings:
* P-006 hardening was missing despite an in-scope public report-contract signal.
* Removing `omitempty` is insufficient because nil slices still marshal as `null`; the plan must normalize to non-nil empty arrays and assert `applied`/`skipped` directly.
* PlanMigration lacks a per-file findings channel, so it cannot satisfy report-and-continue without additional design.
* The decode-helper reuse path must avoid a second divergent decode policy and avoid an unnecessary second filesystem read.
