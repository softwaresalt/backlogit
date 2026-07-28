---
chunk_strategy: h1-h2-h3
description: 'Stage session memory — staged stash 50471E28 (durable_writes second-layer hardening) into queued shipment 111-S. Created covering feature 130-F decomposed into five test-first tasks (130.001-T..130.005-T), one per durable_writes site, wired U1/U2 -> U4 blocks dependencies. Deliberation + impl-plan + plan-harden + two-attempt multi-agent plan-review (attempt 1 FAIL on 2 P1 U5-contract findings; attempt 2 PASS after revision). Stash 50471E28 harvested (source_stash_id linked); the other 3 non-stageable entries (6FA0829B, 7F0A6E89, 918BCDAF) left active. Committed to local main, unpushed.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-07-28/130-F-durable-writes-second-layer-hardening-stage-memory.md
title: 130-F durable_writes second-layer hardening — Stage session memory
---

# 130-F durable_writes Second-Layer Hardening — Stage Session Memory

## Scope

Stage session (single-entry mode). Staged exactly one stash entry —
`50471E28` (feature, medium, P2): durable_writes second-layer hardening,
completing `ErrWriteIndeterminate` caller reconciliation and durable
mkdir/append retry idempotency across five sites, dispositioned from Copilot
review cycle-4 on PR #308 (feature 123-F, shipment 109-S). Left the other three
active stash entries untouched (non-stageable from this workspace).

## Artifacts created

- Deliberation: `docs/decisions/2026-07-28-durable-writes-second-layer-hardening-deliberation.md`
  (decided; Option B — five independent test-first tasks, one per file).
- Plan: `docs/exec-plans/2026-07-28-durable-writes-second-layer-hardening-plan.md`
  (impl-plan + plan-harden; `Requires plan hardening: yes` driven by the AC5 MCP
  contract signal; `Constitution Check: pass`; docline lint clean).
- Feature `130-F` (queued), `source_stash_id: 50471E28`.
- Tasks (all queued, parent `130-F`):
  - `130.001-T` U1 — reconcile ErrWriteIndeterminate in UnarchiveItem non-git restore (`internal/core/archive.go`)
  - `130.002-T` U2 — explicit indeterminate reconciliation for dependency callers (`internal/core/dependencies.go`)
  - `130.003-T` U3 — re-attempt parent flush on durable append retry (`internal/events/stream.go`)
  - `130.004-T` U4 — re-fsync existing dir in core mkdirAllDurable durable retry (`internal/core/durable_fs.go`)
  - `130.005-T` U5 — map durability classes to explicit MCP append_comment outcomes (`internal/mcp/tools.go`)
- Dependencies (`dep_type: blocks`): `130.001-T -> 130.004-T`, `130.002-T -> 130.004-T`
  (U4 changes the shared `core.mkdirAllDurable` consumed at runtime by U1/U2).
- Shipment `111-S` (queued): items `[130-F, 130.004-T, 130.001-T, 130.002-T, 130.003-T, 130.005-T]`
  (parent-first, dependency order: U4 upstream before U1/U2).

## Plan-review (multi-agent-dispatch, 2 attempts)

- Attempt 1 FAIL: Agent-Native Parity Reviewer raised 2 P1 — (1) U5 never pinned
  whether the indeterminate outcome is error- vs success-shaped nor a stable
  retryable signal (agents could auto-retry and duplicate comments); (2) the
  Risks section claimed a U5 exactly-once retry test that did not exist.
  Corroborated P2s: U2 seam does not fire on the `relocate=false` path (real
  indeterminate is the atomicfile post-rename parent fsync, not the
  `mkdirDirSyncFn` source-dir seam); U1 seam must write-then-return-indeterminate;
  U3/U4 "confirmed" framing was unimplementable (fix = unconditional re-fsync);
  U4 shared-function blast radius into U1/U2 (independence claim imprecise).
- Revision resolved every P1/P2; pinned the U5 outcome to the real
  `gate_errors.go` envelope (`"error": write_not_applied|write_indeterminate` +
  `"retryable"`, via `marshalGateError`/`NewToolResultError`), added the U5
  retry-idempotency test, added U4->U1/U2 dependency edges + cross-consumer
  regression, corrected the U2 seam + propagation precondition, added
  Constitution VIII/IX/X line-items.
- Attempt 2 PASS: re-dispatched Agent-Native Parity + Go + Architecture; all
  confirmed RESOLVED, no new P0/P1. Review record appended to the plan.

## Grounding facts (for Ship)

- Two-class contract lives in `internal/errors/durability_errors.go`
  (`ErrWriteNotApplied` safe-retry / pre-commit; `ErrWriteIndeterminate`
  never-roll-back / post-commit). Governing rule: commit-then-surface
  (`docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`).
- Existing fault-injection seams per site: `replaceFileWriteFn` (U1),
  atomicfile write seam / new `persistArtifact` seam (U2), `EventWriter.fsyncDirImpl`
  (U3), `mkdirDirSyncEnabled`/`mkdirDirSyncFn` (U4), `append_comment_test.go` (U5).
  Tests swapping package-global seams must NOT use `t.Parallel`.
- gofmt-on-Windows: verify formatting on LF-normalized BOM-free copies (CRLF +
  autocrlf flags ~96 files falsely).
- Deferred follow-up (Ship to stash at closure): extract a shared durable-mkdir
  primitive into the `fsutil` leaf so `internal/events` + `internal/core` stop
  maintaining two copies of the level-by-level durable-mkdir algorithm.

## Deferred / untouched stash entries

- `6FA0829B` (low) — external autoharness `plan-review/SKILL.md.tmpl` write; BLOCKED by Principle IV. Active.
- `7F0A6E89` (low) — external autoharness `spike/SKILL.md.tmpl` write; BLOCKED by Principle IV. Active.
- `918BCDAF` (medium) — GitHub branch-protection required-status-check config; operator/admin action outside repo tree. Active.

## Handoff

Shipment `111-S` (queued) is the handoff token to Ship. Stage did not build code,
create a feature branch, or open a PR. Backlog + planning artifacts committed to
local `main` (unpushed); remote staging deferred to Ship's Step 1.5 gate.
