---
chunk_strategy: h1-h2-h3
description: "Compound-refresh report for 157-S (PR #507): added one new learning on refreshing worktree files after an eol=lf renormalization, and kept two related entries unchanged."
doc_type: closure
docline:
  date: 2026-10-16T08:40:00Z
  status: final
  tags:
    - compound-refresh
    - 157-S
    - line-endings
schema_version: "1.0"
shipment_id: 157-S
source: docs/closure/157-S-175-F-post-merge-closure.md
title: "157-S compound refresh"
---

# 157-S compound refresh

Context: post-merge closure for shipment 157-S (PR #507, merge commit `8ccd75e084a1e29c7b0754e9bea016195fb0632c`). Mode: `apply`, scope: compound entries related to line endings, `.gitattributes` and baseline renormalization.

## Entries reviewed

| Entry | Classification | Evidence | Action |
|---|---|---|---|
| `docs/compound/2026-07-13-utc-frontmatter-timestamp-normalization.md` | keep | Concerns timestamp normalization, not file line endings; matched on the word "normalization" only. | None. |
| `docs/compound/2026-06-26-docline-frontmatter-contract.md` | keep | Frontmatter contract still accurate; the closure docs here follow it. | None. |
| No existing entry covers `eol=lf` renormalization or the `git restore` no-op. | create | Grep of `docs/compound` for `eol=lf`, `renormaliz`, `crlf` and `git restore` matched only unrelated entries. | Created `docs/compound/2026-10-16-gitattributes-eol-renormalization-worktree-refresh.md`. |

## Out of scope

Other entries under `docs/compound/` do not reference the 157-S change set. No compound entry was deleted or consolidated.

## Follow-ups for Stage

* The local `golangci-lint` toolchain mismatch (built go1.24, dependencies need go1.25) is an environment issue, not a compound learning; it is tracked in the closure record.
