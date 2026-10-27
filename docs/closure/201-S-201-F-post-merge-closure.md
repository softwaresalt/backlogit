---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 201-S (CY-C: harden filesystem containment and snapshot parsing)."
doc_type: closure
docline:
  date: 2026-10-27T17:00:00Z
  status: final
  tags:
    - closure
    - 201-S
schema_version: "1.0"
shipment_id: 201-S
source: docs/closure/201-S-201-F-post-merge-closure.md
title: "201-S closure"
---

# 201-S closure

Context: feature 201-F (CY-C), PR #591, merge `f3b43e52bff174f155e83c55394d4ef562ef6d07`. Members: 201-F plus 201.001-T through 201.010-T (11). Changes: manifest members reload through a handle-contained read (POSIX descriptor-relative `Openat` walk, Windows handle with exact final-path match); Windows archive and append writers bind to a pinned directory handle (spike `docs/decisions/2026-10-08-cy-c-windows-relative-create-spike.md`); blocked-snapshot read is contained and no-follow; one shared fold-aware, depth-limited duplicate JSON member scanner rejects duplicate members before decoding; the append writer refuses an outside-pointing logs directory; a reloaded manifest member whose ID differs from the manifest ID is refused.

## Releasability

`closure_status: READY`. No runtime surface (internal library hardening), so runtime-verification is not applicable. Monitoring: watch for `ErrValidation` refusals from reconcile member, snapshot and append reads on workspaces whose storage root sits behind a subst drive or mapped drive (the exact final-path match refuses those, as the earlier containment check did). Rollback: revert PR #591. Owner: operator. Validation window: next shipment closure (the reconcile read paths are exercised).

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`. docs/memory held 141 files / 697 KB, so it archived 2 files that had no incoming references and wrote one summary (`docs/memory/compacted/2026-10-27-197s-orchestrator-dark-run-compacted.md`). The other 31 older files are still cited and stay.

## Evidence

* Windows ran, not skipped: the C1, C4, C6 and C8 harnesses and the later junction/symlink swap tests all ran on Windows using `mklink /J` junctions (no privilege needed). RED was observed on pre-implementation code for C1, C4, C6 and C8.
* Local review: adversarial review round 1 BLOCKED (key-fold mismatch, POSIX read not ancestor-safe, Windows pin re-resolution). All fixed test-first. Round 2 READY_WITH_FOLLOWUPS (append logs-directory containment and a depth limit fixed). Code-review of each later remediation READY.
* Copilot: two iterations. Round 1 (4 threads): in-root junction swap fix `5ab9a354` and manifest ID check `c95689ed`; two deferred. Round 2 (1 thread): deferred. All threads replied to and resolved; P-018 gate SATISFIED at `5a940d11`.
* CI: all required checks green at `5a940d11`, including the Windows handle/lock tests. Merge strategy: merge commit.
* Local gates: build, `go vet ./...`, linux and darwin vet of `internal/core`, and gofmt clean. `go test` of the whole module passed with `*LintFix` and `TestU172_ItemLogBarrierSetterUnreachableFromProduction` skipped. `golangci-lint` v1.64.8 was built with go1.26 and cannot load go1.27 export data, so lint ran only in CI. TestU172 walks a gitignored `logs\` directory of about 3 GB that could not be parked (rename denied).
* Transient: backlogit MCP claim, one task move and ship timed out client-side and converged server-side; verified by polling, no retry.

## Source artifact provenance (untouched)

Source stash IDs 0FFBF819, 45BD3B36, A0C733C6 and E45E6D65 were archived by Stage at harvest. No source stash or deliberation was a shipment member, and none was mutated here.

## Compound refresh

New entry `docs/compound/2026-10-27-pin-the-handle-not-the-path.md`. No existing entry is superseded.

## Follow-ups

* Stash `D7558EE1` (DEFERRED SCOPE EXPANSION): pre-existing checkpoint no-follow readers, the events strict walker, `shipment_reconcile_snapshot_unix.go` hardening, and Windows read-path anchoring.
* Stash `ECC01D88` (DEFERRED SCOPE EXPANSION): Unix append writer ancestor-safe directory walk and reconcile-specific member discovery.
* Stash `02AE2138` (DEFERRED SCOPE EXPANSION): pin the storage-root identity across validation and read.
* Advisory: the CY-C closure run of the Windows tests is this document; the Linux-only Unix read tests were vet-checked but not executed locally (CI `test` job runs them).
