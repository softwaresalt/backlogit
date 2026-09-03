---
chunk_strategy: h1-h2-h3
description: "Consolidated multi-persona adversarial plan-review evidence for the 2026-09-03 dark-factory staging run"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-03-dark-factory-adversarial-review.md
title: "Dark-Factory Adversarial Plan-Review Evidence — 2026-09-03"
---

# Dark-Factory Adversarial Plan-Review Evidence — 2026-09-03

Preserves the plan-review gate evidence for the 13 shipment plans (S1-S13). The
per-plan verdicts live in each plan's `## Plan Review` section; this is the
consolidated record. Operator required multi-persona + adversarial review before
implementation PRs; that requirement is satisfied at the plan gate here (code-
level adversarial review remains Ship's responsibility per role boundary).

## Dispatch

- Mode: `multi-agent-dispatch` (real reviewer sub-agents), recorded on every plan.
- Always-on personas: Correctness Reviewer, Architecture Strategist.
- Triggered persona: Security Reviewer over security-touching plans (S1, S3, S7, S10, S11, S12).
- Post-remediation Adversarial Review re-review over the P1-failing plans (S9, S10, S11).

## Gate outcomes

| Plan | Initial | Final | Hardening | Notable P1/P2 remediated |
|------|---------|-------|-----------|--------------------------|
| S1 | ADVISORY | PASS | yes/satisfied | P2 symlink parent-dir traversal + TOCTOU (full-path resolve); P2 secret control (fail-closed scan) |
| S2 | ADVISORY | PASS | no | P2 unmerged decode-helper prereq (precondition + fallback) |
| S3 | PASS | PASS | no | clean |
| S4 | ADVISORY | PASS | no | P2 missing shared evidence contract (added U4) |
| S5 | PASS | PASS | no | clean |
| S6 | ADVISORY | PASS | no | P2 five-analyzer bundling over 2h (per-analyzer subtasks) |
| S7 | ADVISORY | PASS | yes/satisfied | P2 evidence/rule bundling (subtasks); P2 GitHub dup (single source); P3 token/anchor |
| S8 | ADVISORY | PASS | yes/satisfied | P2 8-check bundling (subtasks); P2 hardening inconsistency (report-only gate) |
| S9 | FAIL(P1) | PASS | no | P1 evidence-schema ownership (emit against S4 U4) — re-review RESOLVED |
| S10 | FAIL(P1) | PASS | yes/satisfied | P1 duplicate graph engine + evidence interface; re-review NEW P1 waiver auth (authenticated/agent-denied) |
| S11 | FAIL(P1) | PASS | yes/satisfied | P1 self-asserted override bypass (authenticated out-of-band); P1 duplicate graph engine (consume S10 core) |
| S12 | ADVISORY | PASS | yes/satisfied | P2 CLI-only != operator-only (enforced TTY/confirmation); P3 parked-state recognition |
| S13 | PASS | PASS | no | P3 U2 no-op risk (verifiable in-repo fallback) |

## Cross-plan architecture resolutions (program S4-S11)

1. Single shared fault-line evidence-artifact contract owned by S4 U4; all detectors (S5-S9) emit against it; S10/S11 consume it.
2. Single shared graph-evaluation/evidence-binding core owned by S10 U1; S11 consumes it as a library and adds only policy node types — no duplicate engine.
3. S10 terminal-node DAG is the authoritative review-ready gate; S11 composes policy nodes into it (no competing gate).
4. S7 is the single GitHub/git evidence-derivation source; S11 routes review-complete/CI-pass through it.
5. One soundness engine library (S8) invoked by both the pre-assembly gate and the S10 node.

## Residual risks (non-blocking, carried forward)

- S1: checkpoint-context key-allowlist deferred (YAGNI) — recorded open follow-up above the enforced fail-closed secret-scan floor.
- S2 U2: scope expands to introduce the shared decode helper if 146-F U8 has not landed at execution time.
- S7/S11: external GitHub API availability — handled by declared-degradation, never a false pass.
- Program gates (S8/S10/S11) ship report-only before fail-closed enforcement; incremental rollout required before enablement.

## Plan-review cycle accounting

S9, S10, S11 each: attempt 1 FAIL (P1) -> in-scope remediation -> attempt 2 re-review.
S10 attempt 2 surfaced one new consistency P1 (waiver auth boundary), remediated by
mirroring the already-accepted S11 authenticated/agent-denied override control. All
within the 2-cycle limit. No plan remains at FAIL.
