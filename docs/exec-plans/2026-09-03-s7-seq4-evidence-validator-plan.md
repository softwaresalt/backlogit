---
chunk_strategy: h1-h2-h3
description: "Execution plan for S7: Sequence 4/7 API-backed evidence and documentation validator"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s7-seq4-evidence-validator-plan.md
title: "S7 Execution Plan — Sequence 4/7 API-Backed Evidence & Documentation Validator"
---

# S7 Execution Plan — API-Backed Evidence & Documentation Validator (Seq 4/7)

**Covering feature**: API-backed evidence and documentation validator
**Stash member**: E053034D (high)
**Tier**: feature (shipment sequence S7)

## Problem Frame

Review/readiness claims are trusted from prose rather than derived from GitHub and
backlogit (reviewed HEADs, review requests/cycles, unresolved threads, CI checks,
merge SHAs, shipment membership/status, task counts, provenance, closure state).
Inaccurate audit trails reach review or closure. Derive and validate those claims
from the APIs, and extend docs linting for required frontmatter, table shape,
heading rules, numeric/topology claims, resolvable file:line/symbol anchors, task
IDs, stale terminology, and implementation-comment drift.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors |
| II. Test-First (P-002) | declaration -> RED -> GREEN per unit |
| III. Workspace Isolation | validator reads only workspace + API |
| IV. CLI Containment | n/a |
| V. Observability | validation report structured |
| VI. Single Responsibility | evidence derivation vs docs-lint extension separated |
| VII. Destructive Approval | none (read-only validator) |
| VIII. Safety Modes | fail-closed on unverifiable claim; API failure degrades explicitly |
| IX. Git-Friendly | text reports |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Evidence derivation from GitHub + backlogit (single derivation source)
* Scope: derive reviewed HEADs, review requests/cycles, unresolved thread counts, CI checks, merge SHAs, shipment membership/status, task counts, provenance, and closure state from the GitHub API (gh) and backlogit. This module is the SINGLE GitHub/git evidence-derivation source for the program — the S11 enforcement engine consumes it for review-complete/CI-pass/merge-ready nodes rather than issuing a second GitHub-API implementation. Token handled via environment/stdin only (never argv); subprocess stderr scrubbed before it reaches any emitted/committed report. To respect the 2-hour rule this unit is harvested as one subtask per evidence-source group (GitHub-review evidence; GitHub-CI/merge evidence; backlogit shipment/task/provenance evidence), each independently verifiable.
* Acceptance: for a sample PR/shipment the derived evidence matches ground truth; API unavailability degrades to an explicit declared-degradation result, never a false pass; token never appears in argv or any emitted report.

### U2 — Claim-vs-evidence validator
* Scope: compare prose-asserted readiness claims against derived evidence and fail closed on any unverifiable or contradicted claim.
* Acceptance: an accurate claim passes; a fabricated/contradicted claim fails with a specific reason. Depends on U1.

### U3 — Docs-lint extension for structural/reference claims
* Scope: extend docs linting for required frontmatter, table shape, heading rules, numeric/topology claims, resolvable file:line or symbol anchors, task IDs, stale terminology, and implementation-comment drift. Anchor/reference resolution is constrained to the workspace root (reject anchors resolving outside it) to avoid a read-path traversal via crafted docs, and shares one reference/anchor resolver with S8. Harvested as one subtask per lint-rule group to respect the 2-hour rule.
* Acceptance: each new rule flags a seeded violation and passes clean docs; an anchor resolving outside the workspace root is rejected.

## Dependency Graph

U1 -> U2; U3 independent (docs-lint track). Single domain per unit.

## Runtime Verification and Closure

Runtime surface: read-only validator invoking external GitHub API. Verification:
sample PR/shipment ground-truth comparison + seeded docs violations. Closure:
declared-degradation behavior documented; validator runs in CI.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| Consume GitHub API with auth token | Medium — token/secret exposure in logs or reports | Read-only scope; token via env/stdin only (never argv); scrub subprocess stderr before any emitted report; never log token |
| Derive readiness from external API | Medium — API outage produces a false pass | Fail-closed: unverifiable claim fails; API unavailability degrades to an explicit declared-degradation result, never silent pass |
| Docs anchor/reference resolution | Low-Medium — read-path traversal via crafted anchor | Constrain anchor resolution to workspace root; reject out-of-root anchors; shared resolver with S8 |

Rollback trigger: validator is read-only; disable the failing rule/check if a
false positive appears. Ownership: harness maintainers. Validation window: run in
report mode against recent PRs/shipments before enforcing.

#### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent.
* security/auth/permission/compliance-sensitive: PRESENT-minor — consumes GitHub tokens/read scope; must not log secrets.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: PRESENT — depends on GitHub API availability and auth.
* high runtime/rollout/rollback risk: absent (read-only).

Requires plan hardening: yes

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on), Security Reviewer (external GitHub API + token trigger). Plan hardening was REQUIRED and is SATISFIED (## Plan Hardening present).

Findings and remediation:
- Correctness P2 (U1 derived ~9 evidence types from two sources in one unit; U3 added 8 lint rules in one unit — over the 2-hour rule): REMEDIATED — U1 harvested as one subtask per evidence-source group, U3 as one subtask per lint-rule group.
- Architecture P2 (U1 GitHub/git derivation duplicated S11 evidence needs): REMEDIATED — U1 declared the single GitHub/git evidence-derivation source; S11 U2 now routes review-complete/CI-pass/merge-ready through it.
- Security P3 (token via argv / subprocess stderr leak; anchor traversal): REMEDIATED — token via env/stdin only, stderr scrubbed, anchor resolution constrained to workspace root.

No P0/P1. No residual blocking items.
