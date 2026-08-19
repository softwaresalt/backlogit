---
chunk_strategy: h1-h2
description: "Ship agent session memory for 128-S / 144-F implementation"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-08-19/ship-128s-144f-implementation-memory.md
title: "Ship 128-S / 144-F — Prevention hardening implementation memory"
---

## Session Summary

**Date**: 2026-08-19
**Agent**: Ship (depth 1)
**Branch**: `feat/144-shipped-transition-prevention`
**Worktree**: `D:\Source\GitHub\backlogit\.worktrees\stage-47b48db0`
**Shipment**: 128-S (active)
**Feature**: 144-F (active)

## Visibility

`INTERCOM_DEGRADED`: agent-intercom unavailable; degraded operator visibility
recorded per agent-intercom.instructions.md protocol.

## Tasks Completed

| Task | Title | Status |
|---|---|---|
| 144.001-T | RED harness for guard 1 transition and move producers | done |
| 144.002-T | Implement guard 1 on transition/move seams | done |
| 144.003-T | RED harness for guard 1 create producer | done |
| 144.004-T | Implement guard 1 on create seam | done |
| 144.005-T | RED harness for guard 2 archive stamping | done |
| 144.006-T | Implement guard 2 in ArchiveItem | done |
| 144.007-T | Parity tests (MCP + CLI) | done |
| 144.008-T | Design doc + compound learning | done |
| 144.009-T | Surface error-adapter mapping | done |
| 144.010-T | RED harness for guard 1 locked-path revalidation | done |
| 144.011-T | Implement guard 1 authoritative post-lock revalidation | done |

## Files Modified (production)

- `internal/errors/errors.go` — added `ErrShipmentShippedRequiresEnvelope`, `ErrArchiveShippedRequiresEvent`
- `internal/core/gate_transition.go` — guard 1 unlocked-peek (U2): unconditional refusal, supersedes formalGateEnforced()-only branch
- `internal/core/shipment.go` — guard 1 move seam (U2): `moveShipmentStatusWithHeadGuard` refuses when `topLevel && newStatus == ShipmentShipped`
- `internal/core/artifacts.go` — guard 1 create seam (U4) + locked-path revalidation (U11): `CreateArtifact` refuses initial shipped; `updateArtifactUngated` refuses after lock+reload
- `internal/core/archive.go` — guard 2 (U6): `ArchiveItem` refuses `archived_status: shipped` without durable event
- `internal/mcp/errors.go` — U9: `domainError` switch maps new sentinels to `shipment_shipped_requires_envelope` / `archive_shipped_requires_event`
- `internal/cli/gate_exit.go` — U9: `ExitShipmentGovernance = 9`, `shipmentGovernanceExitError`
- `internal/cli/move.go` — U9: `moveGateError` checks governance sentinel before gate-typed errors
- `internal/cli/archive.go` — U9: maps governance sentinel to exit code 9

## Files Modified (tests)

- `internal/core/gate_transition_test.go` — U1 RED + U2 verification
- `internal/core/shipment_test.go` — U1 RED + U2 fixture fixes (4 tests)
- `internal/core/144_create_guard_test.go` — NEW: U3 RED
- `internal/core/archive_test.go` — U5 RED + blerrors import
- `internal/core/144_locked_revalidation_test.go` — NEW: U10 RED
- `internal/core/doctor_shipped_event_test.go` — `seedShippedShipment` uses topLevel=false; `seedArchivedShippedNoEvent` NEW
- `internal/core/queue_test.go` — fixture uses ShipmentAbandoned instead of ShipmentShipped
- `internal/core/shipment_gate_formal_test.go` — updated existing test to expect `ErrShipmentShippedRequiresEnvelope`
- `internal/cli/144_prevention_cli_test.go` — NEW: U7 CLI parity
- `internal/mcp/tools_test.go` — not modified (new file created instead)
- `internal/mcp/144_prevention_parity_test.go` — NEW: U7 MCP parity
- `tests/contract/shipment_tools_test.go` — fixture uses abandoned instead of shipped

## Files Created (docs)

- `docs/design-docs/2026-08-shipment-shipped-prevention-envelope.md` — U8
- `docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md` — U8

## Quality Gate Outcomes

| Gate | Result |
|---|---|
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `golangci-lint run --timeout 300s` | PASS |
| `gofmt -l .` (changed files) | PASS |

## Key Decisions

* **Unconditional refusal, no exemption flag**: `ShipShipment` never calls `UpdateArtifactWithGate` or `updateArtifactUngated`, so no `caller-origin` flag needed — that would be a forgeable bypass.
* **Two-seam TOCTOU close**: unlocked peek (fast path) + locked revalidation (U11) closes the peek-to-write window.
* **Guard 2 uses `shippedEventPresence` directly**: intra-package call; no extraction; `doctor.go` unchanged.
* **Test fixtures use `moveShipmentStatusWithHeadGuard(topLevel=false)`**: bypasses the topLevel guard for in-package test fixtures that need a "shipped" state without the governed ShipShipment.
* **`seedArchivedShippedNoEvent`**: writes archive file directly to bypass guard 2 for doctor detection tests. This is intentional: guard 2 prevents NEW residue; doctor tests detect EXISTING pre-144-F residue.

## Failed Approaches

* Initially tried string-based error check in archive_test.go → replaced with `errors.Is`
* `isArchiveShippedRequiresEvent` unused function → removed

## Open / Next Steps

1. Push branch `feat/144-shipped-transition-prevention`
2. Create implementation PR targeting `main`
3. Request Copilot review
4. Monitor CI
5. Address Copilot findings
6. Run §1.9 defense-in-depth readiness gate
7. Stop at P-014 gate — await operator merge approval

## Residual Risks

* Guard 2 fires on governed `ShipShipment` archival; transient JSONL read failure during archival fails closed → leaves shipment shipped-but-unarchived → acceptable residue for doctor audit.
* Out-of-band edits remain out of scope (doctor audit covers detection).
