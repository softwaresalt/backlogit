---
chunk_strategy: h1-h2-h3
description: 'Decision on excluding intrinsic Git metadata from Markdown lint runners.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-11-05-markdown-lint-git-metadata-exclusion-deliberation.md
title: 'Markdown Lint Git Metadata Exclusion Decision'
docline:
  deliberation_id: 084-DL
  decision_status: decided
  promoted_to: none
  stash_id: AB17AF01
---

## Problem Frame

The P-008 Markdown lint wrapper is blocked by Git metadata Markdown files
rather than reported violations in the nonignored source corpus. The wrapper
must remain a real, repo-wide source gate. The proposed runner correction is
a separate contract surface from the Stage hierarchy and plan-namespace work.

## Research Findings

* `scripts/md-lint.ps1` and `scripts/md-lint.sh` both invoke
  `markdownlint-cli2@0.23.1` with the `**/*.md` source glob
* PromptBuilder validation, relayed by the operator, reported that the actual
  wrapper exited 1 on 23 pre-existing `.git/*.md` metadata files. The pinned
  CLI over all nonignored source with only intrinsic `.git` excluded reported
  7,729 files and zero issues; `git ls-files` reported no tracked `.git` paths
* The earlier stash capture described that evidence as reported by the
  operator. Its underlying source is PromptBuilder validation; the original
  capture remains unchanged
* `docs/compound/2026-07-26-markdownlint-frontmatter-title-double-count.md`
  and `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`
  emphasize the pinned tool, broad corpus, and distinct Markdown gate

## Options Evaluated

### A: Exclude only intrinsic Git metadata

Add `!.git/**` to both runner invocations. Preserve all nonignored source,
the pinned version, rule configuration, and failure behavior.

### B: Leave the runners unchanged

Make no runner change. This avoids changing the invocation but leaves the
reported metadata failure blocking publication.

## Trade-off Comparison

| Criterion | A: intrinsic exclusion | B: unchanged |
|---|---|---|
| Nonignored source coverage | Preserved | Preserved |
| Metadata-only blocker | Addressed, pending validation | Remains |
| Scope | Two runners only | No runner change |

## Decision

At `2026-11-05T05:07:18Z`, the operator answered **"Yes"** to the bounded
choice to add only the intrinsic `.git` metadata exclusion to both runners.
Choose option A: add `!.git/**` to `scripts/md-lint.ps1` and
`scripts/md-lint.sh`; retain `markdownlint-cli2@0.23.1`, MD001, MD025,
MD041, and all nonignored source files. Do not edit `.git`, broaden ignores,
change rules, or weaken exit behavior.

## Rejected Alternatives

Leave the known metadata blocker in place, edit Git internals, exclude
additional paths, or weaken the pinned rules. None meets the approved scope.

## Provenance and Reconciliation

* Surviving stash entry: `AB17AF01`; indexed active and historical stash
  searches found no duplicate. Clean duplicate scan recorded
* Originating PR: #643; review-thread, task, feature, and shipment IDs: `N/A`
* Indexed backlog search by `AB17AF01` found no later identifier. No late
  identifier found; the captured `N/A` values remain unchanged
* No duplicate entry was archived or removed

## Risks and Mitigations

PromptBuilder owns any separately authorized runner changes and validation.
Verify both runner invocations, their exit behavior, retained source coverage,
and the literal P-008 wrapper before reporting success. The source-corpus
control run is not a pass of the actual wrapper. This decision does not
authorize Stage to edit source, run tests or linters, or claim a shipment.
