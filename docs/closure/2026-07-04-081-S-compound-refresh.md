---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh report for the 081-S post-merge closure. Reviews CI/workflow-related compound entries against the shipped docs-only housekeeping change (no drift; all kept) and records one new capture: the source-verified dorny/paths-filter predicate-quantifier every semantics gotcha discovered during the deferred D760E508 CI cost-gating plan-review, graduated from a transient Stage memory checkpoint into docs/compound/github-actions with an explicit unreviewed/deferred-design caveat.'
doc_type: closure
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-04T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-04-081-S-compound-refresh.md
title: 081-S compound-refresh — dorny/paths-filter every-quantifier capture
---

# Compound Refresh — 081-S post-merge closure

**Context**: Post-merge closure of shipment 081-S (docs/closure compaction, merge `e33a060`). The operator flagged a candidate learning for evaluation: the dorny/paths-filter `predicate-quantifier: every` semantics captured in the Stage session memory.
**Mode**: apply (one new capture) + propose (existing entries reviewed, all kept).

## Phase 1 — Evidence

- Shipped scope: docs-only, archive-only. Touched **no** `.github/workflows/**`, no Go code, no CLI/schema. → No existing compound entry drifts as a result of 081-S itself.
- Candidate learning source: `docs/archive/memory/2026-07-04-stage-ci-gating-closure-compaction-session.md` (rounds 1–4 finding trail) + `docs/exec-plans/2026-07-04-ci-cost-gating-plan.md` (round-4 corrected design). Source-verified against dorny/paths-filter `README.md` + `src/filter.ts`.

## Phase 2 — Classification

| Entry | Classification | Rationale |
|---|---|---|
| `github-actions/F013-workflow-sha-pinning.md` | **keep** | Accurate, distinct (SHA-pinning / Go-version alignment). 081-S did not touch workflows. |
| `workflow-issues/cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md` | **keep** | Still accurate; the CLI Reference Drift gate behavior is unchanged. |
| `github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md` | **new (capture)** | No existing entry covers `predicate-quantifier` semantics. Source-verified, high-value gotcha (cost three failed plan-review rounds). Captured narrowly as tool-factual knowledge. |

## Phase 3 — Applied Maintenance

- **Created** `docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md`.
  - Captures ONLY the verified third-party-tool behavior (constant-false positive-allowlist no-op under `every`; the all-negated-patterns correct pattern; `!= 'false'` fail-safe gate direction; `!cancelled()` job guard; required-SKIP + must-RUN canaries).
  - Explicit caveat: the broader D760E508 CI cost-gating **design remains UNREVIEWED and deferred**; only the tool-semantics kernel is asserted as validated.
  - Rationale for capturing now (rather than at D760E508 ship time): the learning lived only in a memory checkpoint subject to compaction in this same session; graduating the durable, tool-factual kernel prevents loss and makes it discoverable. The design itself is NOT enshrined.
- No existing entries updated, consolidated, replaced, or deleted.

## Phase 4 — Follow-ups

- When D760E508 eventually clears a fresh plan-review and ships, its Ship session should revisit this entry to (a) confirm the corrected design held in practice and (b) optionally add an implementation-level companion entry. Until then the entry stands as tool-semantics reference only.
