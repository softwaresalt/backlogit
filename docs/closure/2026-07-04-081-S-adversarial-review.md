---
chunk_strategy: h1-h2-h3
description: 'Adversarial multi-model review (3 reviewers, report-only) of post-merge closure branch post-merge/081-S. All reviewers independently verified the dorny/paths-filter predicate-quantifier every semantics compound entry as factually correct (Tier-3 empirically ran picomatch and fetched src/filter.ts). No consensus or majority findings; three unique LOW-confidence MINOR observations recorded. Gate decision: NOT blocking — PROCEED.'
doc_type: closure
docline:
    ms.date: 2026-07-04T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-04T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-04-081-S-adversarial-review.md
title: 081-S post-merge closure — Adversarial Multi-Model Review
---

# Adversarial Multi-Model Review — post-merge/081-S

**Mode**: report-only (no files modified)
**Scope**: `git diff origin/main..HEAD` on branch `post-merge/081-S` — 2 commits
(`83ffaff` archive backlog artifacts; `abe3ce5` closure + knowledge graduation).
11 files, docs + backlog-state only, **no source code**.
**Reviewers**: 3, multi-model, parallel, independent, JSON-only.

| Reviewer | Tier | Model |
|---|---|---|
| Reviewer-A | Tier 1 (fast) | gemini-3.5-flash |
| Reviewer-B | Tier 2 (standard) | claude-sonnet-4.6 |
| Reviewer-C | Tier 3 (frontier) | claude-opus-4.8 |

## Gate Decision

**PROCEED — not gate-blocking.**

- **Zero** HIGH-confidence (consensus) findings.
- **Zero** MEDIUM-confidence (majority) findings.
- **Three** LOW-confidence (single-reviewer) MINOR observations — all advisory.
- No P0/P1. No CRITICAL or MAJOR from any reviewer.

The central and highest-risk check — factual accuracy of the graduating
dorny/paths-filter compound entry — was **independently confirmed correct by all
three reviewers**. Tier-3 fetched `dorny/paths-filter` `src/filter.ts` and ran
picomatch 4.0.5 empirically; every technical claim held, including the subtle
all-negation-under-`every` case (no picomatch "implicit match-all" gotcha applies
because dorny compiles each pattern as an independent matcher). This is the
strongest possible consensus signal for the item most at risk of enshrining a
factual error into durable institutional knowledge.

## Section 1 — Consensus Findings (confidence: HIGH)

_None._ No finding was flagged by all three reviewers.

**Positive consensus (verified correct, no defect):** the dorny/paths-filter
compound entry's technical assertions — `every` = per-file `patterns.every(...)`
with filter-true-iff-some-file-matches-all; positive disjoint allowlist under
`every` = constant-false no-op; all-negated patterns under `every` = correct
non-docs detector (NOR / De Morgan); default `some` = fail-open for unlisted
paths; `!= 'false'` fail-safe gate direction; `!cancelled()` job guard;
single-pattern quantifier-invariance — are **all accurate**.

## Section 2 — Majority Findings (confidence: MEDIUM)

_None._ No finding was flagged by more than half of reviewers.

## Section 3 — Unique Findings (confidence: LOW)

### L-1 — `schemas/**` cited as "unlisted dir" contradicts the WRONG example
- **Source**: Reviewer-B only · **Severity**: MINOR · **Action class**: advisory
- **File**: `docs/compound/github-actions/dorny-paths-filter-every-quantifier-semantics-2026-07-04.md` (Default-quantifier `some` paragraph)
- **Issue**: The parenthetical lists `schemas/**` among "unlisted dirs … that skip
  the heavy jobs" under `some`, but `schemas/**` is one of the three patterns
  enumerated in the WRONG example filter immediately above. Under `some` a change
  to `schemas/**` *would* correctly set `unsafe=true` in that example — it is not
  fail-open. Only `scripts/**`, `plugin/**`, `retired package tree`, `.mcp.json` are genuinely
  unlisted in the stated example.
- **Disposition**: **Valid internal inconsistency.** The core semantics remain
  correct; this is a self-contradictory illustrative aside, not a semantics error.
  Recommend (non-blocking) dropping `schemas/**` from the "unlisted dirs"
  parenthetical if/when the entry is next touched. Because this doc is graduating
  to durable knowledge, worth a follow-up fix even though it does not block.

### L-2 — Archived 081-F body retains pre-remediation "87 files" count
- **Source**: Reviewer-B only (Reviewer-C explicitly assessed and dismissed) · **Severity**: MINOR · **Action class**: advisory
- **File**: `.backlogit/archive/081-F.md` (chore body text) — also present in `081.001-T.md`
- **Issue**: Body says "87 files / ~582 KB"; all closure/memory artifacts and the
  clean arithmetic use 88 (`88 − 37 archived = 51 preserved + 1 summary = 52`).
- **Contested**: Reviewer-C reconciled it (88 = 87 top-level + 1 in the
  `2026-04-06/` subdir) and judged the P3 nit genuinely remediated in the in-scope
  summary; the 87 lives only in the frozen archived-artifact body.
- **Disposition**: **No action.** This is a frozen archived backlog artifact —
  rewriting it post-archive would itself violate archive-only discipline. The
  in-scope, lint-covered summary uses the corrected figure. Advisory only.

### L-3 — Post-merge `docs/closure/` count stated as 52, arguably 54 on this branch
- **Source**: Reviewer-A only (Reviewer-C computed 52 = 51 + 1 and found no contradiction) · **Severity**: MINOR · **Action class**: advisory
- **File**: `docs/closure/2026-07-04-081-S-post-merge-closure.md` (Monitoring §); mirrored in `docs/memory/compacted/2026-07-04-081-S-ship-compacted.md` (out of lint scope)
- **Issue**: "52 files" describes `docs/closure/` state established by the feature
  merge (51 preserved + 1 summary). The post-merge closure branch then adds this
  closure report + the compound-refresh report, so the on-branch count is 54.
- **Disposition**: **No action (pedantic / reference-point dependent).** 52 is the
  correct figure for the shipment-established state the sentence describes;
  closure/refresh meta-artifacts are process output, not part of the compacted
  budget. Immaterial to the stated purpose (headroom is ~380 KB vs 500 KB
  threshold either way). Advisory only.

## Section 4 — Remediation Plan (priority = confidence × severity)

All findings: confidence LOW (weight 1) × severity MINOR (weight 2) = **priority 2**.
Ordered by file path for determinism.

| # | Finding | Priority | Action class | Recommendation |
|---|---|---|---|---|
| 1 | L-1 `schemas/**` inconsistency | 2 | advisory | Fix on next touch of the compound entry (drop `schemas/**` from "unlisted dirs"). Non-blocking. |
| 2 | L-2 archived 87-count | 2 | advisory | No action — frozen archived artifact; corrected figure lives in in-scope summary. |
| 3 | L-3 52-vs-54 count | 2 | advisory | No action — 52 correctly describes the shipment-established state; immaterial to budget. |

**No `safe_auto`, `gated_auto`, or `manual` items. No P0/P1. No backlog work items generated.**

## Section 5 — Bug/Issue Queue Entries

None. No P0 or P1 findings were produced by any reviewer; nothing to enqueue.

## Verification Notes (per-reviewer, for audit)

- **Reviewer-A (Tier 1)**: 2 findings, both the 52-vs-54 count observation (L-3),
  reported against the closure doc and the compacted memory.
- **Reviewer-B (Tier 2)**: 2 findings — L-1 (`schemas/**`) and L-2 (87 count).
- **Reviewer-C (Tier 3)**: 0 findings (`[]`). Empirically verified dorny/paths-filter
  semantics via `src/filter.ts` + picomatch 4.0.5; reconciled 87/88; confirmed
  docline lint (0 violations, H2-start matches established compound convention in
  `F013-workflow-sha-pinning.md`); confirmed pre/post reconcile PROCEED + P-007
  intact + queue→archive lifecycle correct; confirmed no operator WIP leaked.

## Cross-Check Against Requested Focus Areas

1. **Factual accuracy of compound entry** — ✅ verified correct by all 3 (Tier-3
   empirical). One internal-consistency aside (L-1, `schemas/**`), not a semantics
   error.
2. **Internal consistency** (SHA `e33a060`, counts, ACs, dates) — SHA uniform;
   arithmetic sound (88=37+51, result 52=51+1); 87/88 reconciled; date boundaries
   nest consistently (stale ≤2026-05-30 < 14-day cutoff 2026-06-20 < earliest
   preserved). No blocking contradiction.
3. **Docline frontmatter (in-scope docs)** — 0 genuine defects; `backlogit docs
   lint` = 0 full-tree; H2-body-start is the established compound convention.
4. **Reconcile integrity** — pre/post both PROCEED; P-007 no deletions; queue→archive
   move correctly characterized; `expected done` → `status: archived` is correct
   lifecycle, not a contradiction.
5. **Operator WIP / out-of-scope inclusion** — none leaked; WIP remained unstaged
   in the working tree per documented guardrails.
