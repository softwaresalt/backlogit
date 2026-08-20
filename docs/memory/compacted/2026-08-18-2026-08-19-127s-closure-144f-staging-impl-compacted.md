---
title: "Compacted Memory — 127-S Merge/Closure (2026-08-18) and 144-F/128-S Staging+Implementation (2026-08-18/19, superseded)"
doc_type: memory
schema_version: "1.0"
ingested_at: "2026-08-20T05:10:00Z"
---

Source files archived:

* `docs/archive/memory/2026-08-18/dark-mode-start-127s.md`,
  `dark-mode-scope-127s.md`, `dark-mode-merge-authorized-127s.md`,
  `dark-mode-complete-127s.md`, `local-review-ready-127s.md`,
  `pr-body-127s.md`, `stage-47b48db0-shipment-shipped-prevention-memory.md`
* `docs/archive/memory/2026-08-19/ship-128s-144f-implementation-memory.md`
  (superseded — the full backlog-state detail is preserved below;
  see this session's own
  `docs/closure/2026-08-19/128-s-144-f-runtime-verification-and-closure.md`
  for the merge/closure outcome)
* `docs/archive/memory/2026-08-19/ship-128s-pr370-readiness.md`
  (**explicitly stale** — names parent HEAD `6350115a`, not the actual
  reviewed/merged HEAD `db1300f8`; the authoritative readiness record is
  GitHub PR #370 comment
  <https://github.com/softwaresalt/backlogit/pull/370#issuecomment-5348407251>
  and this session's closure record, not this file)

## 127-S — Shipment Shipped-Event Audit-Log Durability (COMPLETE, shipped/archived)

* Feature 143-F, tasks 143.001-T…143.012-T (12 tasks; see the 2026-08-17
  compacted memory for the full staging/design history).
* Dark-mode trace: `DARK_MODE_START` (2026-08-18T18:58:30-07:00, worktree
  `.worktrees/127-s-reconcile`, branch `feat/143-shipped-event-audit-durability`,
  freeze-scope safety mode) → `DARK_MODE_SCOPE` (127-S/143-F,
  143.001-T…143.012-T; stash `47B48DB0` explicitly excluded) →
  `LOCAL_REVIEW_READY` (reviewed HEAD `97f1fd3b`, 0 P0/P1, 0 unresolved
  threads, 3 review cycles) → `DARK_MODE_MERGE_AUTHORIZED` (PR #367, merge
  SHA `817d4679`) → `DARK_MODE_COMPLETE` (2026-08-19T03:20:00Z).
* PR #367 implementation: `appendShipmentEventErr` (shipment-scoped,
  error-returning event appender; locks item log; classifies
  `ErrWriteNotApplied`/`ErrIndeterminate`; refuses item IDs resolving outside
  the workspace logs dir); `doctor.CheckShippedEventCompleteness`
  (`missing_shipped_event`, `shipped_unarchived_residue` — enumerates feature
  descendants, not just manifest members, labeled "approximate"); MCP
  `check_shipped_event_completeness` on `backlogit_doctor`; CLI
  `--check-shipped-event-completeness`.
* 3 Copilot review cycles, 6 findings, all valid and fixed (path-containment
  check added before lock/write; feature-descendant expansion for the
  residue finding).
* P-016 topology verified clean at dark-mode start (7 worktrees enumerated,
  only `.worktrees/127-s-reconcile` was the active implementation for this
  scope).
* Follow-up: stash `47B48DB0` (prevention of non-`ShipShipment` producers)
  remained active/excluded — **this became feature 144-F / shipment 128-S**,
  which is the subject of this session's closure.

## 144-F / 128-S Staging (Stage, 2026-08-18) — superseded detail preserved

* Planning worktree `.worktrees/stage-47b48db0` on `admin/stage-47b48db0`
  from `origin/main` `244dcec7` (P-016 sanctioned Stage
  planning/research-worktree exception — no implementation performed here).
* Deliberation → plan (hardened, 2 plan-review attempts: FAIL then PASS) →
  harvest: feature `144-F` + originally 9 tasks (144.001-T…144.009-T), later
  expanded to 11 tasks after a PR #369 Copilot review-fix cycle added
  `144.010-T`/`144.011-T` (locked-path revalidation, closing a TOCTOU gap
  between the unlocked peek and the authoritative write in
  `updateArtifactUngated`) → shipment `128-S` (feature + 11 tasks, 12
  members).
* **Attempt-1 plan-review P1s** (all fixed before attempt 2 PASS): guard 1
  missed the exported `MoveShipmentStatus(topLevel=true)` producer; a
  create-path bypass (`CreateArtifact`/`create_item`/`harvest_stash`) could
  birth a shipment already at `shipped`; an unsafe `isValidShipmentTransition`
  defense-in-depth would have broken the governed `ShipShipment` path itself.
* Key architecture decision: enforce in gate-independent **core seams**
  (the formal-gate broker is unwired when disabled, so a gate-only check
  would be bypassable); reuse the existing `shippedEventPresence` doctor
  predicate directly (same package, no extraction); scope guard 2 strictly
  to `artifact_type == shipment && oldStatus == shipped`; keep
  `internal/errors` a leaf package, with surface-error mapping isolated to a
  dedicated MCP/CLI adapter unit (U9).

## 144-F / 128-S Implementation (Ship, 2026-08-19) — superseded detail preserved

* Branch `feat/144-shipped-transition-prevention`, same
  `.worktrees/stage-47b48db0` worktree (reused for Ship per P-016).
* Production files touched: `internal/errors/errors.go` (2 new sentinels:
  `ErrShipmentShippedRequiresEnvelope`, `ErrArchiveShippedRequiresEvent`);
  `internal/core/gate_transition.go` (guard 1 unlocked peek — unconditional,
  supersedes the old `formalGateEnforced()`-only branch); `internal/core/shipment.go`
  (guard 1 move seam in `moveShipmentStatusWithHeadGuard`);
  `internal/core/artifacts.go` (guard 1 create seam in `CreateArtifact` +
  locked-path revalidation in `updateArtifactUngated`); `internal/core/archive.go`
  (guard 2 in `archiveShippedEventPreflight`, moved to run BEFORE any cascade
  per a PR review fix); `internal/mcp/errors.go` (sentinel→domain-error
  mapping); `internal/cli/gate_exit.go` + `move.go` + `archive.go` (exit code
  9, `ExitShipmentGovernance`).
* Test fixtures needing a "shipped" shipment state without going through the
  governed path use `moveShipmentStatusWithHeadGuard(topLevel=false)`
  directly (intentional test-only bypass, not a production seam).
  `seedArchivedShippedNoEvent` writes an archive file directly to construct
  PRE-144-F residue for doctor-detection tests — guard 2 prevents NEW
  residue, doctor detects EXISTING residue; these are complementary, not
  redundant.
* PR #370 went through 3 Copilot review cycles before the eventual merge (in
  THIS closure session) at reviewed HEAD `db1300f80c5f03636cbb8b6ac8ca81fdc87115cb`,
  merge commit `461b670c3602ce54fa5e24635f4e2abc50c2b36c`. Key review fixes
  across the cycles: guard 2 moved before cascade; two `doc_type` frontmatter
  corrections (`compound`→`learning`, `design-doc`→`design`); a stale-archive-copy
  read in the preflight fixed via the 060.002-T queue-preference pattern;
  `BulkUpdateStatus` found to bypass guard 1 — fixed to abort the batch with
  the sentinel and map to CLI exit 9.
* Residual P2s (MCP parity tests call handlers directly rather than through
  registered dispatch; CLI parity tests validate the mapper rather than
  every Cobra command's exit code end-to-end) were acknowledged, not fixed,
  and carried through to this session's closure record, which additionally
  performed live CLI-level verification of both guards (exit code 9,
  confirmed via actual process invocation) as a partial independent mitigation.

## Status

Both 127-S and 128-S/144-F are now fully shipped/archived. This compacted
entry is the durable record of the full staging→implementation arc for
144-F; consult the current closure record
(`docs/closure/2026-08-19/128-s-144-f-runtime-verification-and-closure.md`)
for the actual merge, backlog-reconciliation, and runtime-verification
outcome of shipment 128-S itself.
