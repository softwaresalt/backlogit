---
chunk_strategy: h1-h2-h3
description: "Stage dark-factory (P-017) grouping ledger and deliberation record for the 25-entry stash activation on 2026-09-03"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-03-dark-factory-grouping-ledger.md
title: "Dark-Factory Grouping Ledger — 2026-09-03 Stage Activation"
---

# Dark-Factory Grouping Ledger — 2026-09-03

Stage owns this staging run under a P-017 dark-factory activation. Operator is
AFK; autonomous decisions are recorded here with rationale. This ledger is the
Step 1.5 (contextual grouping) and Step 2 (deliberation) durable artifact for
the whole activation, plus the deliberation record for the four entries flagged
"needs deliberation".

## Scope (live stash, re-read through backlogit, unambiguous)

25 active entries: EC987334, 1787FD85, 5F4E0FC3, 360A183F, 63E810D9, 6CE00B88,
5672D73E, EB93E236, 66834D9E, BE32CAE2, 6FA45E69, DBBA62AA, F350503F, A12BBAFA,
633818E1, 302EFF07, A2C91FE5, 5A4DBE3C, C1808666, B212512E, E053034D, 4863B04B,
48F28B8D, C0A382C7, C52993E3.

No entry carries the `DEFERRED SCOPE EXPANSION` marker (P-021 C2 precedence does
not apply). Checkpoint recovery = zero-candidate normal startup (0 anomalies, 0
active stage checkpoints). No queued/active shipment existed at activation.

## Product-outcome prioritization (ordering law)

1. Reliability & security fixes supersede feature work.
2. Composability/interoperability and simplifying refactors supersede feature work.
3. Feature work supersedes documentation-only work.
4. Within a tier: bug -> review-follow-up -> feature -> task -> spike/decision;
   required decisions move ahead of dependent implementation; higher priority and
   foundational dependencies first.

## Shipment sequence (linear `blocks` chain — only the next is eligible)

| Seq | Shipment slug | Tier | Members (stash IDs) | Rationale |
|-----|---------------|------|---------------------|-----------|
| S1 | checkpoint-disposition-hardening | reliability/security | 302EFF07 (bug), A12BBAFA, F350503F, 6FA45E69, DBBA62AA, +6CE00B88 (decision) | Security symlink bug + evidence-integrity hardening + checkpoint schema hygiene; one subsystem (internal/core + internal/events checkpoint disposition). |
| S2 | docline-contract-decode-convergence | reliability/refactor | EC987334, 1787FD85 | Always-an-array contract fix + decode-policy convergence in internal/docline. |
| S3 | cli-mcp-parity-and-governance | composability/interop | 63E810D9, EB93E236, 5672D73E, +5F4E0FC3 (decision) | CLI/MCP structured-error parity, classify tool, and create_checkpoint governance decision. |
| S4 | fault-line-seq1-parity-harness | feature (critical) | 5A4DBE3C | Sequence 1/7 cross-surface golden parity harness (program foundation). |
| S5 | fault-line-seq2-mutation-framework | feature (critical) | C1808666 | Sequence 2/7 mutation postcondition/consistency framework. |
| S6 | fault-line-seq3-compat-corpus | feature (high) | B212512E | Sequence 3/7 compatibility corpus, fuzzing, static analysis. |
| S7 | fault-line-seq4-evidence-validator | feature (high) | E053034D | Sequence 4/7 API-backed evidence and documentation validator. |
| S8 | fault-line-seq5-soundness-linter | feature (high) | 4863B04B | Sequence 5/7 plan and work-item soundness linter. |
| S9 | fault-line-seq6-red-test-honesty | feature (high) | 48F28B8D | Sequence 6/7 red-test honesty gate. |
| S10 | fault-line-seq7-evidence-dag | feature (high, epic) | C0A382C7 | Sequence 7/7 shipment-level fault-line evidence DAG (integrates seq 1-6 as node executors). |
| S11 | workflow-policy-enforcement-engine | feature (high) | A2C91FE5 (decision) | Deterministic harness-wide workflow-policy enforcement engine; consumes seq-6 honesty gate + shares seq-7 DAG architecture -> capstone. |
| S12 | shipment-lifecycle-features | feature (medium/low) | C52993E3, BE32CAE2 (decision) | Parked-state feature + queued-record forward-repair operation. |
| S13 | harness-docs-hygiene | docs-only | 66834D9E, 360A183F, 633818E1 | Commit-scope vocabulary, upstream continuity wording, plugin-bundle scope-boundary confirmation. |

Ordering wired as S1 blocks S2 blocks ... blocks S13 (covering-feature `blocks`
edges). First eligible / restart cursor: **S1**.

## Autonomous deliberation decisions (AFK dark-mode)

### 6CE00B88 — checkpoint context gitignore/redaction posture (in S1)
Decision: DO NOT change the git-tracking posture of `.backlogit/checkpoints/`
mid-stream (consistent with 129-S reasoning; Principle XI forbids history
rewriting so a tracking flip cannot purge already-committed blobs and would
rewrite unrelated repo conventions). Mitigate at the write boundary instead:
add a bounded, fail-closed size guard and an unredacted-durable-state secrets
caveat at the checkpoint-context write path, plus the already-documented U10b
caveat. A key-allowlist is deferred (YAGNI; no evidence of need) and recorded as
an open follow-up, not built now. Carried into the S1 plan.

### 5F4E0FC3 — govern create_checkpoint (in S3)
Decision: YES, make create_checkpoint a governed operation, for consistency with
the governed-parity design. Scope is minimal: add `governed: true` +
`governed_name: checkpoint_create` to the registry and author the named
behavioral fixture that dispatches the authoritative registry (per
docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md).
No new result-shape or flag semantics. Carried into the S3 plan.

### BE32CAE2 — queued-record forward-repair operation (in S12)
Decision: expose it as an operator-only, CLI-only, audited break-glass mirroring
the existing `force_cli_only` / `repair_member_evidence` contract (gate/state
repair is a human-at-a-terminal action; MCP exclusion is deliberate). Acceptable
pre-state is narrowly defined: shipment.status == queued AND all non-descoped
members advanced consistently beyond queued; ambiguous/mixed states are rejected.
Forward-only (queued->active), shipment-record-only, atomic, audited
before/after/result evidence. Explicitly excludes DD957688. Carried into S12 plan.

### A2C91FE5 — workflow-policy enforcement engine (S11)
Decision: proceed as a design-first feature. Because it is a cross-surface,
fail-closed enforcement engine with broad blast radius, S11's plan REQUIRES
hardening and a security/architecture adversarial lens. It consumes the S9
red-test honesty gate and shares the S10 evidence-DAG architecture, so it is
sequenced last among the program features. Incremental-rollout, graph-storage,
and evidence-storage open questions are recorded in the S11 plan's open
questions, not resolved speculatively.

## Entry dispositions (all 25 accounted for)

Every entry is a member of exactly one shipment above; none deferred. 633818E1
is a documented no-action scope-boundary confirmation (harvested as a
documentation record in S13 with acceptance = "recorded as accepted scope
boundary; no code change"). 360A183F carries a cross-repo caveat (the `.tmpl`
target may live in the upstream harness repo) recorded in the S13 plan.
