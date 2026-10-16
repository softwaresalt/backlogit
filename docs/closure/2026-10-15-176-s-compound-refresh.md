---
chunk_strategy: h1-h2-h3
description: "Compound-refresh report for 176-S (PR #504): added one new learning on strict fixture-input validation in PowerShell lint runners, and kept two related entries unchanged."
doc_type: closure
docline:
  date: 2026-10-15T22:00:00Z
  status: final
  tags:
    - compound-refresh
    - 176-S
    - powershell
schema_version: "1.0"
shipment_id: 176-S
source: docs/closure/176-S-175-F-post-merge-closure.md
title: "176-S compound refresh"
---

# 176-S compound refresh

Context: post-merge closure for shipment 176-S (PR #504, merge commit `d48f15cb83f0ae26b04726995b53bb6dd5ef92c6`). Mode: `apply`, scope: compound entries related to the PowerShell lint-runner bootstrap scripts (`scripts/verify-{task,baseline,terminal}-lint.ps1`) and their Go harnesses.

## Entries reviewed

| Entry | Classification | Evidence | Action |
|---|---|---|---|
| `docs/compound/2026-07-16-copilot-review-loop-complete-class-enumeration.md` | keep | Guidance on enumerating a whole defect class still applies and matches the 176-S review experience. It is prose-consistency specific, so the validation checklist belongs in a new entry. | None; cross-referenced from the new entry. |
| `docs/compound/2026-10-11-unfiltered-suite-evidence-under-load.md` | keep | Quiet-host gate evidence guidance is still accurate. | None; cross-referenced from the new entry. |
| No existing entry covers PowerShell fixture-runner input validation. | create | Grep of `docs/compound` for fixture, PowerShell, TOCTOU, and ordinal matched only unrelated entries. | Created `docs/compound/2026-10-15-powershell-fixture-runner-strict-input-validation.md`. |

## Out of scope

Other entries under `docs/compound/` do not reference the 176-S change set. No compound entry was deleted or consolidated.

## Follow-ups for Stage

* The repository test command note (`go test -count=1 ./cmd/... ./internal/... ./tests/...` instead of `go test ./...`, because of the large `logs/` directory) now lives in the new entry. Stage may want to promote it to contributor docs.
