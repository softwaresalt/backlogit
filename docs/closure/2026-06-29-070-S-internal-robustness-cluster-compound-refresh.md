---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh for shipment 070-S — classified existing learnings against the shipped internal robustness cluster. One NEW standalone learning graduated (exported short-circuit cache zero-value bypass, from 070.001-T). empty-string-vs-sentinel KEPT and cross-referenced as the data/validation-layer sibling of the same absence-vs-empty principle now extended by 070.003-T. All other entries KEEP — no overlap with shipped scope.'
doc_type: closure
docline:
    ms.date: 2026-06-29T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-29T21:53:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-29-070-S-internal-robustness-cluster-compound-refresh.md
title: 070-S Internal Robustness Cluster — Compound Refresh
---

# Compound Refresh — Shipment 070-S

| Entry | Classification | Rationale |
|---|---|---|
| best-practices/exported-cache-zero-value-bypass-2026-06-29.md | NEW | Graduated this cycle from 070.001-T — exported short-circuit cache must treat its zero value (nil backing map) as unseeded and re-scan, not as an authoritative empty set. High severity (a uniqueness guard could be silently bypassed). |
| best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md | KEEP | Sibling principle (absence ≠ empty value) at the data/classification layer. 070.003-T extends the same idea to docline `ValidateFields` minLength (present-but-empty key vs absent key), and the new 070-S cache learning applies it to a backing map (nil vs empty). Scope of the existing entry is unchanged; cross-references added in the new entry. No content edit required. |
| 2026-06-26-docline-frontmatter-contract.md | KEEP | 070.003-T refines minLength presence semantics but does not change the four-part docline contract (body-preserving codec + idempotent migration + born-compliant generation + CI gate). |
| 2026-06-28-codec-extraction-leaf-packages.md | KEEP | Codec/leaf-package structure untouched by 070-S. |
| others | KEEP | No overlap with shipped scope. |

## New learning?

Yes — one standalone compound entry created:
`docs/compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md`.
It captures a genuinely reusable, non-obvious gotcha (an exported optimization
cache whose zero value silently disables the safety check it was meant to speed
up) that is not subsumed by any existing entry.

The other two tasks are routine extensions of established discipline:
- 070.002-T (logger dependency injection without `slog.SetDefault`) is a standard
  Go DI pattern — no compound warranted.
- 070.003-T (empty-vs-absent key parity in `ValidateFields`) is the
  validation-layer expression of the existing empty-string-vs-sentinel learning —
  captured by cross-reference, not a new entry.
