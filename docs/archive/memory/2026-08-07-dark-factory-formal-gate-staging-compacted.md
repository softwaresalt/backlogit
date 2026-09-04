---
title: "Compacted Memory — 2026-08-07 Dark-Factory Formal Gate Staging Cycle"
doc_type: memory
schema_version: "1.0"
ingested_at: "2026-08-20T04:30:00Z"
---

Source files (archived to `docs/archive/memory/2026-08-07/`):

* `dark-factory-stage-formal-gate-memory.md`
* `dark-mode-checkpoint-disposition-memory.md`
* `dark-mode-halted-scope-expansion-checkpoint.md`

## Outcome

Stage ran a dark-factory (P-017) staging cycle on branch
`admin/dark-stage-formal-gate` planning the remainder of the `106-F` formal-gate
series (F1, F4, F6, F5) plus a default-workspace-directory rename and a
`054.001-R` review-artifact lifecycle disposition. Mid-cycle, the operator added
two new stash entries (`FDEDE39A`, `B5D7E401`); Stage correctly halted
(`DARK_MODE_HALTED`, scope-expansion) at a safe boundary with no partial
mutations, then was reactivated with an amended scope that staged `FDEDE39A`
(checkpoint administrative disposition) as `122-S`/`136-F`, explicitly excluding
`B5D7E401` per operator directive.

**All shipments produced by this cycle have since shipped and archived**
(verified against `gh pr list`): `117-S`→#334, `118-S`→#336, `119-S`→#339,
`120-S`→#341, `121-S`→#346, `122-S`→#344. No further action needed.

## Key Decisions Preserved

* **F4/F6 kept as separate release units** despite one shared file
  (`internal/mcp/tools.go`) — different seams (data-model/persistence vs.
  commit-association/parity), no shared logic to amortize.
* **`106-F` covering feature excluded from partial F-series shipments** (F1,
  F4, F6) per the `114-S`/P-015 precedent, only included in the final F5
  shipment (`120-S`) to avoid premature covering-feature archival stranding
  remaining units.
* **F5 scope shrank** after finding `atomicfile` already does atomic
  single-file replacement on both platforms (`MoveFileEx` w/
  `MOVEFILE_REPLACE_EXISTING` on Windows, `os.Rename` elsewhere) — the spike's
  Windows-fallback premise was obsolete.
* **Checkpoint disposition (`122-S`/`136-F`)**: `abandon` (valid checkpoints) and
  `quarantine` (malformed-only) kept disjoint by design; audit runs **before**
  the move (not after) so an indeterminate write is never compensated;
  `disposition_operator` never defaults silently — CLI resolves
  `--operator` → `BACKLOGIT_OPERATOR` → OS user, MCP requires an explicit
  parameter.
* **Ordering enforced via explicit `blocks` dependency edges**, not prose:
  `F1 → F4 → F6 → F5 → 9370A18C(121-S)`, and `122-S` sequenced between `120-S`
  and `121-S`.
* **`054.001-R`**: minimal lifecycle disposition only (`review`→`accepted`→
  archived); no new feature work created; one advisory-only residual
  observation recorded, not harvested.

## Gate Outcomes (for future reference on plan-review rigor)

* F-series formal-gate plans (5 plans): cycle 1 FAIL (0 P0/18 P1/12 P2), cycle 2
  PASS (0/0) after full remediation, adversarial multi-model dispatch used
  (security-sensitive + 3+ P0/P1 threshold met).
* Checkpoint disposition plan (`122-S`): cycle 1 FAIL (2 P0/2 P1/5 P2/3 P3),
  cycle 2 FAIL (0/5/3/0), cycle 3 PASS (0/0) — 2 remediation cycles, within the
  3-attempt breaker.

## Pre-Existing Condition Noted (still may be unresolved)

`backlogit doctor` reported one orphan at session start:
`[orphaned_artifact] 016.001-R` (no `parent_id`, no `returned_to_backlog`
event). Was out of scope for this cycle and deliberately left alone. Worth a
`backlogit doctor` check in a future session to confirm whether it has since
been resolved.
