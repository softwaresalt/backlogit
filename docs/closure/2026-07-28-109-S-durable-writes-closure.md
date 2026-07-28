---
description: "Post-merge closure for shipment 109-S (durable_writes fsync protocol, feature 123-F, PR #308 merged at e0ae3546). Records runtime verification, operational readiness, the four Copilot review cycles, and the deferred next-layer hardening follow-up (stash 50471E28)."
doc_type: closure
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
  ms.date: 2026-07-28T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-28-109-S-durable-writes-closure.md
title: "109-S durable_writes post-merge closure"
---

## Scope

Shipment **109-S** — the `durable_writes` fsync durability protocol (feature
**123-F**, tasks 123.001-T through 123.009-T). Delivered via PR **#308** on
branch `feat/123-F-durable-writes`, merged to `main` as merge commit
**`e0ae3546`** (P-009 merge-commit strategy). Shipped in dark factory mode
(P-017) for the declared 109-S scope.

The feature makes the workspace's Markdown/JSONL write, move, archive, and
directory-creation paths power-loss durable when the opt-in
`durable_writes: true` config flag is set. It introduces a two-class write-error
contract and POSIX directory fsync across the artifact, event-log, archive, and
adopt code paths.

## Opt-In Default

`durable_writes` defaults to **false** (`WorkspaceDurableWrites` returns false
unless `Config.DurableWrites` is explicitly true; the field is `omitempty` and
`TestDurableWrites_DefaultsToFalse` pins the default). Every durable code path
degrades to its prior non-durable behavior in the default configuration:
`mkdirAllDurable(dir, false)` is exactly `os.MkdirAll`, durable atomic writes skip
the fsync, and `ErrWriteIndeterminate` is never surfaced. Default-configuration
behavior is therefore unchanged by this shipment.

## Runtime Verification

- **CI on merged HEAD (`93f8501f`)**: 5/5 checks green — test, CLI Reference
  Drift, Detect code changes, Docline frontmatter gate, Markdown lint (P-008).
- **Local gates (independently re-run each review cycle)**: `go test ./...` exit
  0, `go vet ./...` exit 0, `golangci-lint run` exit 0, `gofmt` clean
  (LF-normalized, BOM-free) on all changed files.
- **Targeted durable tests**: the durable-move source-parent fsync, adopt
  indeterminate-but-applied, and archive/restore new-dir parent-fsync tests pass;
  the full `internal/core` package (including the adopt/rollback harnesses) shows
  no regression.
- **Default-path safety**: because durable mode is opt-in, the default behavior is
  covered by the unchanged existing suite.

Verification method: automated test suite + local quality gates. No browser or
external-integration surfaces are involved (pure filesystem durability logic).

## Operational Readiness

**READY.** Rationale:

- **Rollback**: revert merge commit `e0ae3546`. No data migration, no schema
  change, no index format change.
- **Config**: no default change — `durable_writes` stays opt-in/false, so no
  operator action is required and no runtime surface changes for existing
  workspaces.
- **Monitoring**: no automated SLI applies to a default-off durability flag. For
  workspaces that opt in, the manual observation signal is the `slog.Warn`
  "durable move: directory fsync failed (best-effort)" line and any surfaced
  `ErrWriteIndeterminate` error from adopt/persist paths.

## Review Summary (four Copilot cycles)

| Cycle | Findings | Disposition |
|---|---|---|
| 1 | F5–F9 (2×P1, 3×P2) | Fixed (`c6eb6b7e`..`25553483`), verified, resolved |
| 2 | F10–F13 (durable moves fsync only destination parent) | Fixed (`1c96658c`), resolved |
| 3 | F14 archive-dir dirent, F15 adopt surface indeterminate | Fixed (`93f8501f`), resolved |
| 4 | F16–F20 next-layer caller/retry hardening | Deferred to stash **50471E28** (documented), threads resolved |

Cycle 4 was past the §1.8 three-cycle review-fix limit. All five cycle-4
findings are triple-gated (durable mode ON **and** an actual fsync failure
**and** a retry), inert in the default configuration, and reconcilable because
Markdown is the source of truth and the SQLite index rebuilds from it on `sync`.
They were therefore dispositioned as a documented P2 follow-up rather than a
fourth fix cycle, per the review-fix circuit breaker.

## Residual Risk

Bounded and tracked. The next-layer durability completeness work — full
`ErrWriteIndeterminate` caller reconciliation (UnarchiveItem, AddDependency,
RemoveDependency, MCP `append_comment`) and retry-idempotency for durable
`mkdirAllDurable`/`appendDurable` — is captured in stash **50471E28** (feature,
medium) with five explicit acceptance criteria. This work should land before
`durable_writes` is promoted toward default/GA.

## Dark-Mode Record

- `DARK_MODE_START` / `DARK_MODE_SCOPE`: shipment 109-S, feature 123-F.
- `LOCAL_REVIEW_READY`: reviewed HEAD `93f8501f`, 0 unresolved P0/P1 local
  findings, cycle-4 P2 items deferred with follow-up ID.
- `DARK_MODE_MERGE_AUTHORIZED`: PR #308, HEAD `93f8501f`, checks CLEAN,
  strategy=merge-commit, approval source=dark-mode activation record (in-scope
  109-S), no admin fallback used (`NORMAL_MERGE_READY`).
- `DARK_MODE_COMPLETE`: merged at `e0ae3546`; shipment 109-S archived
  (`archived_status: shipped`); reconcile pre+post PASS; follow-up stash
  50471E28 created.

## Follow-Ups

1. **50471E28** — durable_writes second-layer hardening (5 acceptance criteria).
