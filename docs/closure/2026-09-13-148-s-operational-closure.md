---
chunk_strategy: h1-h2-h3
description: "Post-merge operational closure for shipment 148-S / feature 167-F, governed archived-shipment reconciliation to shipped (#423)"
doc_type: closure
docline:
  date: 2026-09-13T20:38:15Z
  status: accepted
  tags:
    - operational-closure
    - post-merge
    - 148-S
    - 167-F
schema_version: "1.0"
source: docs/closure/2026-09-13-148-s-operational-closure.md
title: "148-S operational closure: governed archived-shipment reconciliation to shipped"
---

# Post-Merge Operational Closure — Shipment 148-S / Feature 167-F

## Summary

Shipment **148-S** (feature **167-F**, GH #423: "governed archived-shipment reconciliation to
shipped") shipped at merge commit `c74a55d1e40d1181688f0c87b85a4ff326fc43de` (PR #440,
`main`). All 20 task-wave members (`167.001-T` … `167.021-T`) plus the covering feature
`167-F` and the shipment `148-S` itself are archived with `archived_status: shipped`
(shipment) / `done` (feature and tasks).

This closure covers a **CLI-only, operator-invoked governance capability**
(`backlogit shipment reconcile-shipped`) with no continuously-running service component.
Runtime verification for this kind of change is scoped to CLI correctness/security evidence
already produced during Ship execution, not live-service health/monitoring.

## Validator evidence

| Surface | Validator | Outcome |
|---|---|---|
| Build | `go build ./...`, `go build ./cmd/backlogit`, `GOOS=linux go build ./...` | PASS |
| Static analysis | `go vet ./...`, `golangci-lint run ./...` | PASS |
| Unit/integration | `go test -count=1 -timeout 25m ./...` | PASS (one pre-existing, unrelated, Windows-checkout-only CRLF flake in `internal/faultline`, tracked under stash `92F79833`) |
| Concurrency | `go test -race -count=1 -run '^TestU20_ReconcileShipmentToShippedConcurrency$' ./internal/core` | PASS, repeated 30+ times across remediation cycles, no deadlock/data race |
| CI (Linux) | GitHub Actions `test` job | PASS |
| CI (Windows) | GitHub Actions `Windows handle/lock tests (167.013-T)` job | PASS |
| Docs | `backlogit docs lint`, CLI Reference Drift check | PASS |
| Security review | 3 local reviewer personas (Correctness/Concurrency/Security) + 7 rounds of iterative GitHub Copilot PR review | All findings fixed-and-resolved or explicitly deferred with rationale (see Follow-ups) |

No automated production-monitoring/dashboard surface applies to this CLI-only capability;
operator-facing behavior is documented in
`docs/cli-reference/backlogit_shipment_reconcile-shipped.md`.

## Releasability

**READY.**

- **Monitoring**: N/A (CLI-only, invoked on demand by an operator; no long-running process to
  monitor). Doctor's existing `missing_shipped_event` check now also recognizes
  `EventShipmentReconciledShipped` records and validates their `item_id` binding, providing an
  ongoing, existing-tooling-based integrity check for any future reconciliation.
- **Rollback**: the governed transaction itself has first-class rollback semantics
  (`ErrWriteNotApplied` restores via `restoreShipmentReconcile`; `ErrWriteIndeterminate` never
  restores, surfacing an `indeterminate` outcome for manual operator review via `backlogit doctor`).
  Rolling back THIS PR (reverting the merge commit) is also a normal, low-risk git operation since
  no other code depends on the new `shipment reconcile-shipped` command or `ReconcileShipmentToShipped`
  API yet.
- **Owner**: backlogit maintainers (no external/customer-facing service ownership implications).
- **Validation window**: none required — a CLI capability invoked on-demand; correctness was
  validated pre-merge via the full CI/review evidence above.
- **Follow-up requirements**: see below.

## Follow-ups (captured as backlog stash entries)

| Stash ID | Priority | Summary |
|---|---|---|
| `FE440C62` | high | `ArchiveItem`'s lock order (B-then-C) is a pre-existing, system-wide inconsistency vs. `AssociateCommit`/the new reconcile transaction (both C-then-B); empirically verified to resolve as bounded contention, not deadlock. |
| `1E0C2251` | high | The CAS guard (`guardArchivedStatusUnchangedSince`) is opt-in, not applied to `RemoveArtifactLink`/`BulkUpdateStatus` (pre-existing, system-wide functions). |
| `A0C733C6` | medium | `findArtifact`'s use in manifest-member reload has a narrower pathname-reopen TOCTOU window; `findArtifact` is shared, general-purpose infrastructure out of this shipment's scope to hardened. |
| `E45E6D65` (supersedes `37699341`) | medium | Windows archive-write TOCTOU can leak real payload bytes (not just an empty placeholder) under active local directory-swap capability; closing fully requires NT-native directory-relative creation APIs. |
| `B633E9B9` | medium | Authenticated (machine-verifiable) operator-authorization boundary is a deferred residual (v1 is confirmation + audit only). |
| `2B4E5AC3` | medium | Verifiable legacy descope-provenance support so a shipment with legitimately-descoped members can eventually be reconciled too. |
| `6EFD39C0` | low | Two minor advisory/clarity nits (Phase B fail-fast ordering, a test-comment wording mismatch). |

None of these follow-ups block this shipment's releasability; each was investigated,
classified against P-021 C1 (same-contract-surface scope test), and captured with explicit
out-of-scope rationale rather than silently expanded into.

## 167.015-T ratification (recorded, pending explicit operator sign-off)

Two conscious v1 scope narrowings from GH #423 are recorded on `167.015-T` and require
operator ratification (or scheduled follow-up acceptance) at or after merge:
1. Authorization is confirmation-gated (`--confirm`/TTY phrase) + audited, **not**
   machine-authenticated (follow-up `B633E9B9`, references residual `866FDC8C`).
2. v1 cannot reconcile a shipment with legitimately-descoped members; it rejects them fail-closed
   (follow-up `2B4E5AC3`).

## Source artifact cleanup

- `source_stash_id` `055D507E` (on `167-F`): already archived prior to this session (harvested
  into `167-F` in an earlier Stage pass); confirmed absent from the active stash list at
  closure time — no action needed.
- No `source_deliberation_id` recorded on `167-F`.

## Compaction status

See `docs/memory/` compact-context run following this closure artifact for P-020 compliance.
