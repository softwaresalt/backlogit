---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure for shipment 071-S — deterministic-gates slice (PR #156). Records the release-readiness verdict (READY WITH CONDITIONS), invariants to preserve (versioned doctor --target exit-code contract 0/1/2/3/4, non-blocking per-task advisory lock, full-row UpsertItem reconstruction on size mutation), merge-only rollout path, healthy/failure signals, monitoring plan (.autoharness/metrics timeout telemetry), rollback trigger and procedure (revert the merge commit), validation window, ownership, and the two deferred pre-existing follow-ups (J: confineToStorageRoot resolution errors classified as scope; K: nil header-def yields pass) surfaced by the final Copilot review at the review-fix cycle limit.'
doc_type: closure
docline:
    ms.date: 2026-06-30T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-30T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-30-071-S-deterministic-gates-slice-closure.md
title: 071-S Deterministic-Gates Slice — Pre-Merge Operational Closure
---

# Operational Closure — Shipment 071-S (Deterministic-Gates Slice)

- **Mode**: pre-merge
- **PR**: [#156](https://github.com/softwaresalt/backlogit/pull/156)
- **Branch**: `feat/deterministic-gates-slice`
- **HEAD**: `1d7ee3a`
- **Verification report**: `docs/closure/2026-06-30-071-S-deterministic-gates-slice-runtime-verification.md` (verdict: **PASS**)
- **Readiness**: **READY WITH CONDITIONS** (operator merge approval per P-014; two deferred pre-existing follow-ups acknowledged)

## Change Summary

Nine TDD units (U1–U9) delivering the deterministic-gates slice:

- `doctor --target {file}` single-file validation with a real 5s timeout and a versioned, gate-stable exit-code contract (`0` pass / `1` validation / `2` timeout / `3` scope|io / `4` busy).
- Per-task advisory lock sidecar (`.<name>.lock`, ephemeral, gitignored) with non-blocking acquisition and a busy→exit 4 mapping; IO faults are distinguished from contention (→ exit 3).
- Optional `size` T-shirt enum in header-def (stored under `custom_fields.size`).
- `core.SetArtifactSize` body-preserving mutation seam (mdfront + atomicfile + full-row `db.UpsertItem`), plus `update --size` CLI and MCP `backlogit_update_item` `size` parity, and MCP `backlogit_doctor` `target` parity.

Post-review hardening (Copilot cycles 1–3): lock IO-vs-busy classification, symlink scope-escape guard, MCP `ErrTaskBusy`→conflict mapping, stat-failure classification, timeout lock-stranding fix (lock owned outside the timeout goroutine), stale-removal IO classification, and invalid-size→`validation_failed` mapping.

## CI Status and Unresolved Review Items

- **CI (HEAD `1d7ee3a`)**: all 4 checks green — CLI Reference Drift, Docline frontmatter gate, test (1.23), test (1.24).
- **Copilot review**: covers current HEAD (`1d7ee3a`, review submitted 2026-07-01T08:06:41Z). Cycles 1–3 fixed and resolved 14 threads. The final review surfaced **2 new threads at the review-fix cycle limit (3)** — both **pre-existing, non-regression** behaviors, deferred as follow-ups (see below).

## Invariants to Preserve

1. **Exit-code contract is frozen**: `doctor --target` must map outcomes to `0/1/2/3/4` exactly. `doctorTargetExitCode` is the single source of truth; changing it breaks the cross-repo autoharness gate.
2. **Non-blocking lock**: acquisition never blocks; a held lock yields busy (exit 4), never a mid-write read; IO faults yield exit 3, never busy.
3. **No lock stranding on timeout**: the CLI owns the lock in a synchronous frame (`runDoctorTargetMode` → `PrepareDoctorTarget` + `defer unlock`); only the lock-free `ValidateDoctorTargetResolved` runs under the goroutine deadline.
4. **Full-row upsert preservation**: `SetArtifactSize` reconstructs a fully-populated `*models.Artifact` before `db.UpsertItem` (INSERT OR REPLACE on the full row), so `title`/`status`/`priority`/`parent_id` never null out in the index.
5. **Body-preserving writes** go through `internal/mdfront`; atomic writes through `internal/atomicfile.WriteFileAtomic` — never re-inlined.

## Pre-Deploy Audits

- No migrations, no schema changes to `.backlogit` storage (telemetry stays in `.autoharness/metrics`, per plan Q3).
- No new config flags requiring rollout coordination. The `size` enum is optional in header-def.
- Merge strategy: **merge commit only** (P-009 confirmed: squash and rebase disabled at repo level).

## Deployment / Rollout Path

- **Merge-only** to `main`. No deploy, canary, or migration step. The CLI binary is consumed by autoharness as a subprocess gate; the versioned exit codes are backward-additive (no existing code removed).

## Post-Deploy Checks

1. `backlogit doctor --target .backlogit/queue/<some-task>.md` on a real workspace returns exit 0 for a valid file.
2. A missing-required-field task returns exit 1.
3. `backlogit update --size M <id>` sets `custom_fields.size` and preserves body + index columns.
4. Aggregate `backlogit doctor` shows no new orphans/duplicates introduced by this slice.

## Healthy Signals

- autoharness pre-task-completion gates pass/fail deterministically with stable exit codes.
- No stranded `.<name>.lock` sidecars accumulate after timeouts (TTL reclaim still bounds any residue at 60s worst case).
- Size mutations leave `git diff` limited to the `custom_fields.size` frontmatter key and the SQLite index row.

## Failure Signals

- `doctor --target` returning exit 3 for files that are actually inside `.backlogit` (would indicate a scope-confinement regression).
- Busy (exit 4) returned for non-contention IO faults, or vice versa (exit-code contract regression).
- `update --size` nulling `title`/`status`/`priority` in the index (full-row upsert regression).

## Monitoring Plan

- `doctor --target` timeout/exit telemetry is written to `.autoharness/metrics` (unchanged schema). Watch for anomalous exit-2 (timeout) rates that would indicate the 5s deadline is too tight for real workspaces.
- No new dashboards or alerts required; this is a CLI/library surface consumed by an outer harness that owns its own retry policy.

## Rollback Trigger

- Any deterministic-gate exit-code regression observed by autoharness, or index corruption from a size mutation.

## Rollback Procedure

- Revert the merge commit on `main` (`git revert -m 1 <merge-sha>`), open a revert PR, and merge with operator approval. No data migration to unwind — the slice is additive.

## Risky Action Record

- No destructive or approval-gated runtime actions were taken. All mutations were scoped to the feature branch; backlog archival is deferred to post-merge closure.

## Validation Window

- 1 release cycle of autoharness gate usage. Ownership: the operator / maintainers of `softwaresalt/backlogit`.

## Owner

- `softwaresalt` (repo maintainer / operator).

## Deferred Follow-Ups (surfaced at the review-fix cycle limit)

These two Copilot findings arrived on the final review of HEAD `1d7ee3a`, which would have been a **4th** review-fix cycle (limit is 3). Both describe **pre-existing behavior faithfully preserved** by the U5 lock refactor (verified identical at `08eace8`), are non-blocking (P2/P3), and are intentionally deferred rather than expanding scope beyond the approved plan. They remain **unresolved on the PR** for operator visibility.

- **Follow-up J** (`internal/core/doctor_target.go`, `PrepareDoctorTarget`): `confineToStorageRoot` resolution failures are classified as `kind=scope`, which loses the underlying error message. Exit code is unchanged (scope→3 and io→3 both map to exit 3), so the only improvement is preserving the error text in `Message`. Pre-existing (documented `//nolint:nilerr`). Severity: P3.
- **Follow-up K** (`internal/core/doctor_target.go`, `ValidateDoctorTargetResolved`): a nil `ws.HeaderDef` yields `kind=pass`, so `doctor --target` can pass without header-def validation. In a normally-initialized workspace `HeaderDef` is always loaded (`config.WriteDefaults`), so this is defensive hardening, not an exploitable gate hole. Making a missing header-def a deterministic hard-fail is a reasonable gate-contract change but is outside this shipment's approved exit-code plan. Severity: P2.

**Handling**: these follow-ups will be stashed for the Stage agent during post-merge closure (Step 6), where backlog-state additions belong, to avoid triggering fresh CI/Copilot churn on the feature branch at the merge gate.

## Readiness Verdict

**READY WITH CONDITIONS** — merge may proceed once:

1. The operator grants explicit merge approval (P-014).
2. The operator acknowledges the two deferred pre-existing follow-ups (J, K) as post-merge stash items rather than in-PR blockers.

Merge strategy is constrained to a merge commit (P-009, verified).
