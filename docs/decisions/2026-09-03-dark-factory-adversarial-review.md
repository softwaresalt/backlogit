---
chunk_strategy: h1-h2-h3
description: "Consolidated multi-persona adversarial plan-review evidence for the 2026-09-03 dark-factory staging run"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-03-dark-factory-adversarial-review.md
title: "Dark-Factory Adversarial Plan-Review Evidence — 2026-09-03"
---

# Dark-Factory Adversarial Plan-Review Evidence — 2026-09-03

dispatch_mode: multi-agent-dispatch

This record replaces the prior uniform-PASS record. The re-dispatch was required
because the prior record omitted mandatory personas and therefore was invalid as a
plan-review gate. The genuine report executed the mandatory persona set with
cross-model diversity, including the OpenAI `gpt-5.6-sol` Go anchor, across the
nine target plans: S2, S3, S4, S6, S8, S9, S10, S12, and S13.

Overall verdict: **BLOCKED** — 7 FAIL / 2 READY_WITH_FOLLOWUPS. No target plan
survived as a clean PASS.

## Persona × Model dispatch

| Persona (mandatory) | Model | Provider | Dispatch |
|---|---|---|---|
| Constitution Reviewer | `claude-opus-4.8` | Anthropic | full 9-plan pass |
| Go Reviewer (anchor) | `gpt-5.6-sol` (effort high) | OpenAI | full 9-plan pass |
| Scope Boundary Auditor | `gemini-3.7-flash` | Google | full 9-plan pass |
| Correctness Reviewer | `claude-sonnet-4.6` | Anthropic | full 9-plan pass |
| Architecture Strategist | `grok-4.6` | xAI | full 9-plan pass |
| Security Reviewer | `gpt-5.6-terra` | OpenAI | risk-triggered pass over S3, S8, S9, S10, and S12 |
| Learnings Researcher | research/explore | local docs/compound | 10 learnings mapped |

Five distinct model families were used: Anthropic opus and sonnet, OpenAI sol
and terra, Google gemini, and xAI grok. The anchor route was dispatched. The
report also recorded that initially blocked review-persona slots were
re-dispatched as non-review analysis agents on the same distinct models, so the
mandatory cross-model fact-finding requirement was preserved.

## Re-dispatch reason

The invalidated record captured only Correctness and Architecture, with Security
on some plans. It omitted the mandatory Constitution, Go anchor, Scope Boundary,
and Learnings passes that surfaced controlling P1 findings. The omitted passes
changed the gate result from the prior false uniform PASS to the genuine BLOCKED
verdict recorded here.

## Per-plan gate verdicts

| Plan | Verdict | Controlling findings |
|---|---|---|
| S2 | **FAIL** | Nil slices still marshal as `null` after removing `omitempty`; PlanMigration lacks the per-file findings channel required by report-and-continue; P-006 hardening is missing for an in-scope report-contract change |
| S3 | **FAIL** | P-006 hardening gap for additive MCP tool, CLI error envelope, and `JSONRPCError.data`; `unknown_fields` reflection needs bounds/escaping |
| S4 | **FAIL** | Program-wide versioned evidence contract mislabeled hardening-absent; wrong-module ownership; no versioning/compatibility policy; S5-S8 producers not bound |
| S6 | **FAIL** | Two analyzers (`success-after-audit-warning`; `uncancellable-lock timeout`) are underspecified as bounded AST checks. Fuzzing truthfulness REMEDIATED: S6 U-fuzz unit + harvested task `158.008-T` added to shipment `140-S` |
| S8 | **FAIL** | Realized decomposition violation; S8/S10 engine-ownership and forward-reference inconsistency; fail-open gate configuration risk |
| S9 | **FAIL** | Unresolved Go P1×2 (baseline-overlay seam; workspace-contained immutable base runner) force FAIL under the any-unresolved-P1 gate rule; evidence authenticity binding is an additional follow-up |
| S10 | **FAIL** | Review-record integrity gap; waiver authorization spoofable and incompletely guarded; evidence authenticity absent; U3 is over-scoped |
| S12 | **FAIL** | U2 depends on U1 parked state despite being declared independent; TTY/token authorization is spoofable and not enforced at the shared transition choke point; parked state not integrated into the closed taxonomy |
| S13 | **READY_WITH_FOLLOWUPS** | External-dependency hardening note required; U2 acceptance must require actual upstream delivery evidence or remain active/blocked |

## Consolidated Persona Manifest

| Plan | Constitution (opus-4.8) | Go (sol) | Scope (gemini-3.7) | Correctness (sonnet-4.6) | Architecture (grok-4.6) | Security (terra) | Gate |
|---|---|---|---|---|---|---|---|
| S2 | FAIL(P1) | FAIL(P1×2) | PASS | ADV(P2) | ADV(P2) | n/a | **FAIL** |
| S3 | FAIL(P1) | ADV(P2) | PASS | PASS | ADV(P2) | ADV(P2) | **FAIL** |
| S4 | FAIL(P1) | FAIL(P1×2) | PASS(P3) | ADV(P3) | FAIL(P1×3) | n/a | **FAIL** |
| S6 | ADV(P2) | FAIL(P1) | FAIL(P1) | PASS | ADV(P2) | n/a | **FAIL** |
| S8 | FAIL(P1) | ADV(P2) | FAIL(P1) | FAIL(P1) | FAIL(P1) | FAIL(P1) | **FAIL** |
| S9 | ADV(P2) | FAIL(P1×2) | PASS | PASS(P3) | PASS | ADV(P2) | **FAIL** |
| S10 | ADV(P2) | FAIL(P1×3) | FAIL(P1) | FAIL(P1) | FAIL(P1×2) | FAIL(P1×3) | **FAIL** |
| S12 | ADV(P2) | FAIL(P1×2) | PASS | FAIL(P1) | FAIL(P1) | FAIL(P1×2) | **FAIL** |
| S13 | ADV(P2) | PASS | PASS | ADV(P3) | PASS | n/a | **ADVISORY** |

All mandatory personas ran on distinct models. Security was risk-triggered where
applicable. Learnings were applied across the run and informed the controlling
findings.

## Confidence-weighted P0/P1 remediation queue

No P0 findings were reported.

| # | Plan | P1 finding | Confidence | Evidence |
|---|---|---|---|---|
| 1 | S8 | Harvested `160.002-T` and `160.003-T` each bundled four check classes; re-split one independently verifiable task per check class | **HIGH** | Independent queue verification in the report; Constitution and Scope concurred |
| 2 | S10 | Review-record integrity: attempt-2 surfaced a new P1, then the plan was edited with no recorded final re-review; the prior PASS was unconfirmed | **HIGH** | Correctness finding; named suspect |
| 3 | S10 | Waiver authorization spoofable: `TTY OR token` lets PTY confirmation suffice; token must be required, independently verified, and enforced at all waiver entry points | **HIGH** | Go and Security; compound learnings on authenticated filtering and all-entry-point guards |
| 4 | S12 | U1/U2 declared independent even though U2 depends on U1 parked state; graph must be U1 -> U2 | **HIGH** | Correctness and Architecture; named suspect |
| 5 | S12 | TTY/token authorization spoofable and not enforced at shared `queued -> active` transition choke point | **HIGH** | Go and Security |
| 6 | S4 | Program-wide versioned evidence contract mislabeled hardening-absent; wrong-module ownership; no versioning; S5-S8 producers unbound | **HIGH** | Constitution, Architecture, and Go |
| 7 | S10 | U3 is over-scoped: gate, incremental evaluation, waiver authorization, authentication store, tamper store, visualization, and versioning must be split | **HIGH** | Go, Scope, and Constitution |
| 8 | S2 | Removing `omitempty` is insufficient because nil slices marshal as `null`; normalize to non-nil `[]` and assert array presence directly | **MED-HIGH** | Go and omitempty compound learning |
| 9 | S12 | No named atomic repair seam across Markdown, SQLite cache, and JSONL; parked transition table incomplete | **MEDIUM** | Go and Architecture |
| 10 | S10 | Incremental evaluation, tamper evidence, and append atomicity are undesigned | **MEDIUM** | Go |
| 11 | S10/S9 | Evidence is forgeable without producer, task, and commit authenticity binding before applicability filtering | **MEDIUM** | Security |
| 12 | S8 | U3 acceptance that S10 and this gate call the same engine is unverifiable at S8 ship time; engine ownership contract is contradictory | **MEDIUM** | Correctness and Architecture |
| 13 | S8 | Fail-closed gate can fail open through per-rule flags or report-only mode; no all-entry-point assembly choke guard | **MEDIUM** | Security |
| 14 | S2 | PlanMigration has no per-file findings channel to satisfy report-and-continue | **MEDIUM** | Go and Architecture |
| 15 | S6 | Advertised fuzzing unbacked — **REMEDIATED**: S6 U-fuzz unit (`FuzzCompatibilityCorpusDecode`, seed corpus, bounded budget) + harvested task `158.008-T` added to shipment `140-S` | **MEDIUM (resolved)** | Scope P1 and Constitution P2 |
| 16 | S6 | `success-after-audit-warning` and `uncancellable-lock timeout` analyzers likely need CFG/SSA, not bounded AST-only checks | **MEDIUM** | Go |
| 17 | S4 | Literal cross-surface comparison is impossible because exit code is CLI-only; driver seam using process globals is not parallel-test-safe | **MEDIUM** | Go |
| 18 | S3 | P-006 hardening gap for the new MCP tool, structured error envelope, and JSON-RPC data field | **MEDIUM** | Constitution; documented gate rule |
| 19 | S2 | P-006 hardening gap for the in-scope report-contract change | **MEDIUM** | Constitution |
| 20 | S9 | Baseline runner cannot execute newly added test files at base without an overlay seam; containment is unresolved | **LOW-MED** | Go |

## Overall verdict

Overall staging-run verdict: **BLOCKED (8 FAIL / 1 READY_WITH_FOLLOWUPS)**. S2,
S3, S4, S6, S8, S9, S10, and S12 are FAIL — each retains at least one unresolved
P1, and under the plan-review gate any unresolved P1 forces FAIL. S13 is
READY_WITH_FOLLOWUPS (P2/P3 only). The prior uniform-PASS record is contradicted
by the genuine mandatory-persona re-dispatch and is no longer a valid gate record.

Enforcement note: this BLOCKED verdict is a Stage plan-review gate **record**.
`ClaimShipment` does not currently inspect plan-review records, so this record
does not by itself prevent a failed-plan shipment from being claimed once its
separate closure/topology blocker clears; the untriaged follow-up `FF6D467A` is
not an execution dependency. Deterministic enforcement of plan-review verdicts at
claim time is the scope of the S11 workflow-policy-enforcement-engine feature
(`163-F`); until it ships, these FAIL verdicts must be honored by process.
