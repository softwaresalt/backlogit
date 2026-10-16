---
chunk_strategy: h1-h2-h3
description: "Compound-refresh report for 158-S (PR #509): added one learning about test callers of error-returning signature changes; two lint-related entries kept unchanged."
doc_type: closure
docline:
  date: 2026-10-16T15:50:00Z
  status: final
  tags:
    - compound-refresh
    - 158-S
    - lint
schema_version: "1.0"
shipment_id: 158-S
source: docs/closure/158-S-175-F-post-merge-closure.md
title: "158-S compound refresh"
---

# 158-S compound refresh

Context: post-merge closure for shipment 158-S (PR #509, merge commit `ba2c333a8be44851cadd97ac38cdc5d605d7d715`). Mode: `apply`, scope: compound entries about lint, errcheck, golangci-lint and line-ending guards.

## Entries reviewed

| Entry | Classification | Evidence | Action |
|---|---|---|---|
| `docs/compound/github-actions/F013-workflow-sha-pinning.md` | keep | Matched on `golangci-lint` only; concerns action pinning, not the shipped change. | None. |
| `docs/compound/workflow-issues/ship-agent-incomplete-git-staging-pr-bypass-2026-04-14.md` | keep | Matched on a lint mention; staging guidance is unaffected. | None. |
| No entry covers test callers of error-returning signature changes. | create | Grep for `errcheck` found nothing relevant. | Created `docs/compound/2026-10-16-errcheck-signature-change-test-callers.md`. |

## Follow-ups for Stage

None beyond the closure record.
