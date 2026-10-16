---
doc_type: memory
schema_version: "1.0"
title: "Compacted memory: 154-S / 173-F Ship wave checkpoints (W1-W4 red/green convergence)"
---

# Compacted Memory: 154-S / 173-F Ship Wave Checkpoints

This file was compacted on 2026-10-16 during the P-020 compact-context run for the
`157-S` post-merge closure. It replaces six Ship checkpoint memories for shipment
`154-S` (feature `173-F`, branch `feat/154-s-shipment-claim-scheduler-baseline-marker`)
that no other file referenced by name. `154-S` and `173-F` are archived. Originals were
moved, not deleted, to `docs/archive/memory/2026-09-30/`:

* `2026-09-30-ship-154s-173007-red-confirmed.md`
* `2026-09-30-ship-154s-173008-red-confirmed.md`
* `2026-09-30-ship-154s-w4-converged.md`
* `2026-09-30-ship-154s-wave1-converged.md`
* `2026-09-30-ship-154s-wave1-resumed.md`
* `2026-09-30-ship-154s-wave3-task-173001.md`

The other eleven `154-S` memory files are still cited by checkpoints, plans, decisions or
closure records and stay in place so those references resolve.

## Outcome

* Seven-task frozen set in five waves: W1 `173.006-T`, `173.007-T`, `173.008-T` (red
  deliverables); W2 `173.009-T`; W3 `173.001-T`; W4 `173.003-T`; W5 `173.005-T` (docs).
  The 154-S bootstrap exception bypassed only the active-residual halt, not dependency
  order, harness gates, review, CI or merge approval.
* W1 harnesses were assertion-RED on named functions (`TestClaimMarkerRed_`,
  `TestClaimMarkerReadSurface_(CLI|MCP)$/^RED_`, `TestClaimMarkerRecoveryRed_`). A false
  hierarchy-path assertion and vacuous custom-field checks were corrected as same-surface
  P-021 C1 findings (commit `269a7fbd`). The unfiltered suite was explicitly deferred
  (`FULL_SUITE_DEFERRED`) while the open-red set was non-empty.
* Green-makers closed the reds: `173.009-T` in W2 (recovery), `173.001-T` in W3
  (commit `519879b6`, local review READY 0/0/0/0). The unfiltered suite
  `go test -timeout=30m ./...` first ran when the open-red set emptied (W3: `internal/core`
  520.6s; W4: 27m48s, exit 0). W4 `173.003-T` (commit `ceda9a79`) ran the `-race`
  regression command in 13m47s, exit 0.

## Decisions and lessons

* The CI-pinned lint command
  `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=10m`
  is authoritative. A local PATH `golangci-lint` v2.13.2 has a different default linter
  set; the mismatch is gate-command drift, not a waiver (stash `AD5AECAF`).
* Gofmt is checked LF-normalized, excluding `.copilot/session-state`.
* Run the full suite only at the first convergence gate with an empty open-red set; a run
  earlier would classify rather than verify and can absorb a real failure.
