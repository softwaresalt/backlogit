---
description: "Post-merge closure for shipment 113-S (optional task complexity metadata, feature 132-F, tasks 132.001-T–132.008-T, PR #321 merged at 685620ec). Records the GI/GR reconciliation gate, ship_shipment archival, Copilot review findings, and the task-only typed-metadata seam learning."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-30T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-30-113-S-complexity-metadata-closure.md
title: "113-S complexity metadata post-merge closure"
---

## Scope

Shipment **113-S** — add optional task `complexity` metadata mirroring the
existing `size` seam (feature **132-F**, tasks **132.001-T**–**132.008-T**).
Delivered via PR **#321** on branch `feat/132-F-complexity-metadata`, merged to
`main` as merge commit **`685620ec`** (P-009 merge-commit strategy) after
explicit operator approval (P-014).

Complexity is a task-only typed field. It is added to `header-def.yaml`
alongside `size`/`size_source`/`size_ruleset_version`, set through a dedicated
core seam (`internal/core/artifact_complexity.go`, `SetArtifactComplexity`),
surfaced on the CLI (`backlogit update --complexity`, `backlogit list`), the MCP
tool surface, and projected into the SQLite index for task artifacts only.

## Merge Gate (P-014)

- §1.9 pre-merge readiness gate re-verified independently before approval:
  - No pending Copilot review request.
  - Latest Copilot review `commit.oid` == HEAD `08b21c82`.
  - Zero unresolved Copilot-authored threads (5 fixed + resolved).
  - Required checks green: `test`, Markdown lint (P-008), Docline frontmatter
    gate, CLI Reference Drift.
  - `mergeStateStatus: CLEAN`, `mergeable: MERGEABLE`.
- Merged with `--merge` (merge commit only). Branch deleted.

## Copilot Review Findings (all fixed on HEAD 08b21c82)

1. `internal/core/artifact_complexity.go` — enforce `artifactType == "task"`
   before schema/value validation (projection drops non-task complexity).
2. `internal/core/artifact_size.go` — split the size-seam-specific remediation
   message so complexity does not inherit a misleading "audited size seam" error.
3. `internal/cli/update.go` — document the `--complexity ""` clear/unset
   semantics in flag help; regenerate CLI reference.
4. `docs/memory/compacted/2026-07-30-ship-113-S-compacted.md` — remove duplicate
   YAML `source` key.
5. `.backlogit/queue/132-F.md` — keep feature through the normal lifecycle rather
   than declaring it done in-queue (would trip the reconcile status check).

## GI/GR Reconciliation (Step 6)

- **Pre-mode** (`expected_status: done`): all 9 manifest items `pre-archived`
  (archived with `status: done` on the feature branch), no orphans →
  **PROCEED**. Report: `.backlogit/reconcile/113-S-pre-20260730T171400.md`.
- `backlogit shipment ship 113-S --sha 685620ec …` → moved shipment
  queue→archive, stamped merge SHA on 10 archive artifacts.
- **Post-mode**: all 10 archive files matched, no P-007 deletions, 113-S removed
  from queue → **PROCEED**. Report:
  `.backlogit/reconcile/113-S-post-20260730T171430.md`.

## Knowledge Graduated

- New compound learning:
  `docs/compound/2026-07-30-task-only-typed-metadata-seam-enforce-before-schema.md`
  — task-only typed metadata must be guarded in the core setter before schema
  resolution, because the DB projection drop is not sufficient to protect the
  source-of-truth frontmatter under a customized `header-def.yaml`.

## Follow-ups

- None blocking. Complexity ruleset auto-derivation (analogous to `size_source`
  automation) was intentionally out of scope for 113-S.
