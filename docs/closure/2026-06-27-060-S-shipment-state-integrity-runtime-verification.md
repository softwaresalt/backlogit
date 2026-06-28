---
chunk_strategy: h1-h2-h3
description: 'Post-merge lightweight runtime verification for shipment 060-S — atomic shipment-claim rollback (no torn state on mid-flight activation failure) and stale blocked_reason clearing at all backlog re-entry choke points, proven via the core integrity test suite plus a live ship of 060-S'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T00:15:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-060-S-shipment-state-integrity-runtime-verification.md
title: 060-S Shipment State Integrity — Post-Merge Runtime Verification
---

# Runtime Verification — Shipment 060-S (Shipment State Integrity)

- **Surface**: CLI / data-integrity library (`internal/core` shipment lifecycle + artifact status transitions). No runtime service, web, or background-job surface.
- **Mode**: manual + automated test suite (the live ship below is the real archive mutation for 060-S; no additional throwaway mutation needed).
- **Context**: Ship Step 6 post-merge closure for 060-S; merge commit `7a51904bc159d0f16aa5a9d8866e0bd4c324717d` (PR #143), default branch `main`.
- **Feature**: 061-F (tasks 061.001-T, 061.002-T — both done/archived). Benign `NNN-S → (NNN+1)-F` ID offset (cosmetic, per `docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md`).
- **Verdict**: **PASS**

## Invariants under test

1. `ClaimShipment` is **atomic**: a mid-flight item-activation failure leaves *no* partially activated state — the shipment and every touched item revert to `queued`.
2. A successful claim activates the full included scope (shipment + all manifest items) and never leaves a torn intermediate state.
3. Returned/re-entering items **clear stale `blocked_reason`** metadata at every status choke point (`UpdateArtifact`, `setArtifactStatus`, `cascadePersistedParentStatuses`).
4. An item that is *still* blocked **retains** its `blocked_reason` (the clear is gated on a real status change away from `blocked`).
5. Shipping a shipment with already-archived (pre-archived) tasks re-archives them cleanly with canonical `archived_from` and produces **no torn shipment state** (`returned_ids` empty).

## Environment prechecks

- Binary under test: `backlogit.exe` v1.2.0 at repo root, freshly built from current `main` @ `7a51904b` (carries `docs` + `doctor` subcommands).
- Workspace: `.backlogit/` index rebuilt (638 artifacts) before and after the ship.
- Go toolchain present; `go test ./internal/core/...` runnable.
- No external service, port, fixture, or credential dependency — the affected surface is a local CLI/library.

## Execution & evidence

### Automated test suite (fresh, `-count=1`)

`go test ./internal/core/ -run 'TestClaimShipment|TestUpdateArtifact...ClearsStaleBlockedReason' -count=1 -v`:

| Test | Invariant | Result |
|---|---|---|
| `TestClaimShipment_RollsBackOnMidFlightActivationFailure` | 1 | **PASS** (0.27s) |
| `TestClaimShipment_SuccessActivatesAllItems` | 2 | **PASS** (1.87s) |
| `TestClaimShipment_ActivatesIncludedScope` | 2 | **PASS** (0.69s) |
| `TestUpdateArtifact_ClearsStaleBlockedReasonOnReentry` | 3 | **PASS** (1.07s) |
| `TestUpdateArtifact_KeepsBlockedReasonWhileStillBlocked` | 4 | **PASS** (0.43s) |

`ok  github.com/softwaresalt/backlogit/internal/core  6.714s`. Full package run: `ok internal/core` + `ok internal/core/templates` (no failures).

### Live ship of 060-S (invariant 5 — real archive mutation)

`backlogit shipment ship 060-S --sha 7a51904b… --message … --author …`:

```
{
  "shipment_id": "060-S",
  "shipment_status": "shipped",
  "archived_ids": ["061.001-T", "061.002-T", "060-S", "061-F"],
  "returned_ids": [],
  "commit_sha": "7a51904bc159d0f16aa5a9d8866e0bd4c324717d"
}
```

- `returned_ids: []` — **no item was returned/blocked**; the ship completed atomically with no torn state. This is the production-path confirmation of the claim/activation atomicity work.
- Post-ship index query: `060-S`, `061-F`, `061.001-T`, `061.002-T` → all `archived`; adjacent `061-S` / `062-F` → **still `queued` (untouched)**.
- `doctor --check-archived-from` post-ship: **0 self-referential** records; only the 2 known malformed legacy records (`038-DL`, `039-DL`, value `done`, flagged-only). All 4 newly-archived records carry canonical `archived_from` = `.backlogit/queue/<id>.md`.

## Risky-action state

No `strict-safety`-class risky action. The single state-mutating action (`shipment ship 060-S`) was gated behind a pre-mode reconcile (PROCEED) and produced the expected, fully-reconciled archive result (post-mode reconcile PROCEED, P-007 zero deletions).

## Follow-up recommendations

- None blocking. Monitoring is fully covered by the CI integrity suite (`internal/core` shipment tests) plus the `shipment-reconcile` GI/GR gate on every future ship.
- Pre-existing, out-of-scope: 2 malformed legacy `archived_from` records (`038-DL`, `039-DL`) remain flagged-only — tracked independently of this shipment.

## Verdict

**PASS** — all five invariants verified by green tests and a clean live ship (atomic, no torn state, no stale blocked metadata, canonical archive stamping). Fed to operational-closure as PASS.
