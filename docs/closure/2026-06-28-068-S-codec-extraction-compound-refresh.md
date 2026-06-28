---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh review for the 068-S shared frontmatter codec extraction — classifies the one overlapping docline-frontmatter-contract entry as update (codec location drifted to internal/mdfront) and records the new leaf-package/type-alias/golden-byte-equality learning'
doc_type: closure
docline:
    ms.date: 2026-06-28T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T19:56:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-28-068-S-codec-extraction-compound-refresh.md
title: 068-S Shared Frontmatter Codec Extraction — Compound Refresh
---

# Compound Refresh — Shipment 068-S (Shared Frontmatter Codec Extraction)

- **Scope**: `recent` + entries overlapping the docline / frontmatter-codec surface
- **Mode**: apply (one surgical citation update; one new entry authored separately)
- **Context**: Post-merge closure of 068-S (PR #148, merge `7450271a`)

## Entries reviewed

| Entry | Overlap with 068-S | Classification | Rationale |
|---|---|---|---|
| `docs/compound/2026-06-26-docline-frontmatter-contract.md` | Directly cites `internal/docline/ (codec, ...)`; the body-preserving codec it describes is exactly the logic 068-S extracted | **update** | The four-part contract pattern (body-preserving codec + idempotent seed-once migration + born-compliant generation + CI gate) is **fully intact and still accurate**. Only the codec's *location* drifted: it now lives in the stdlib-only leaf package `internal/mdfront` (atomic write in `internal/atomicfile`), re-exported by `internal/docline` via a true type alias. Applied a surgical Evidence-section note recording the relocation and cross-linking the new 068-S entry. No body of guidance changed; all original citations preserved. |
| `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` | Shares the "avoid import cycle" theme (Rule 1: supply cross-layer data by DI) | **keep** | Distinct mechanism. That entry solves a cycle between *parallel entry-point layers* (`mcp`/`cli`) via **dependency injection**; 068-S solves a cycle between *peer domain packages* via **extraction to a shared leaf**. Complementary, not redundant — the new 068-S entry references the DI approach as the sibling technique. |

## New entry authored

- `docs/compound/2026-06-28-codec-extraction-leaf-packages.md` — captures the durable trio:
  (1) break a duplication/import cycle by extracting shared logic into a stdlib-only **leaf package**;
  (2) preserve the original package's API including an **inherited method** with a **true type alias** (`type T = pkg.T`) rather than a re-declaration;
  (3) prove a behavior-preserving refactor with **differential golden byte-equality** tests + idempotency + live dogfooding. This is a **new** learning (`compound`), not a refresh of an existing one.

## Evidence used

- Shipped code at merge `7450271a` (PR #148): `internal/mdfront/`, `internal/atomicfile/`, the `internal/docline` alias + forwarders, `internal/core/doctor.go`. Confirmed `mdfront` imports only `bytes`/`fmt`/`yaml.v3` and `atomicfile` is pure stdlib (leaf-package claim), and that `docline.Markdown` is a true alias of `mdfront.Markdown` with `Encode` inherited.
- Live workspace evidence from this closure: targeted suites green; `gen-docs` 0 drift; `docs migrate` 0 body-byte changes; live ship re-stamped 6 records canonically (0 doctor self-refs, body preserved).

## Actions taken

- **Updated**: `docs/compound/2026-06-26-docline-frontmatter-contract.md` — Evidence section now records the codec relocation to `internal/mdfront`/`internal/atomicfile` and cross-links the 068-S entry. Guidance unchanged; citations preserved.
- **Consolidated/replaced/deleted**: none.
- **Marked stale**: none.
- **Kept as-is**: `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`.

## Follow-ups requiring manual review

- None. The two cycle-avoidance entries (DI seam vs. shared leaf) are complementary; the contract entry's pattern is intact with only a location note added.
