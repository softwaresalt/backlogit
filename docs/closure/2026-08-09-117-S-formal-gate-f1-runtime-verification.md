---
chunk_strategy: h1-h2-h3
description: "Runtime verification for 117-S (Formal Gate F1 - evidence authenticity and manifest binding), covering CLI surfaces and the round-8 shipment gate bypass fix specifically."
doc_type: closure
docline:
    date: 2026-08-09T00:00:00Z
    status: accepted
    tags:
        - runtime-verification
        - formal-gate
        - 117-S
schema_version: "1.0"
source: docs/closure/2026-08-09-117-S-formal-gate-f1-runtime-verification.md
title: "117-S Formal Gate F1 — Runtime Verification"
---

# 117-S Formal Gate F1 — Runtime Verification

## Verdict: PASS

## Surface

CLI (`cmd/backlogit`), backed by `internal/core` (gate transition, shipment
gate, gateevidence) and `internal/config` (formal gate anchor). MCP tool
surfaces (`backlogit_move_item`, `backlogit_update_item`,
`backlogit_ship_shipment`) share the same `core.UpdateArtifactWithGate` /
`core.ShipShipment` entry points exercised below, so this CLI-level
verification covers their behavior by construction.

## Environment Precheck

* Built a fresh binary directly from the merged `origin/main` tip
  (commit `23d88904`, merge commit for PR #333) via `go build -o
  backlogit-verify.exe ./cmd/backlogit` — NOT the pre-existing `C:\Tools\
  backlogit.exe`, which predates this shipment's changes.
* Verification ran in an isolated, disposable scratch workspace under
  `docs/scratch/runtime-verify-117-S/` (deleted after use; no residue,
  confirmed via `git status --short`).
* Extensive automated coverage (27 Go packages, full suite green,
  including `-race` runs on every concurrency-sensitive change) already
  exercises the gateevidence/gateproof/canonical signing and verification
  paths in depth; this runtime pass targets the CLI-facing behaviors and
  the specific fix least amenable to a pure unit-test — the round-8
  cross-tool bypass.

## What Was Verified

### 1. Basic CLI sanity (fresh binary)

`init`, `add --type feature`, `add --type task --parent`, `move --status
active`, `list`, `shipment create`, `shipment claim` all executed without
error and produced the expected JSON/table output and status transitions.

### 2. Round 8 fix — shipment-to-shipped bypass via `move` is refused

* Created shipment `001-S` with member task `001.001-T`, claimed it to
  `active`.
* With `BACKLOGIT_FORMAL_GATE_REQUIRED=true` set, ran:
  `backlogit-verify.exe move 001-S --status shipped`
* **Expected**: refusal, since a shipment must transition to `shipped`
  exclusively through `ShipShipment` under formal enforcement.
* **Observed**: exit code 1;
  `Error: shipment 001-S refused: shipments must be shipped via
  ShipShipment, not a direct status update, while formal gate evidence is
  enforced: backlogit: formal gate evidence required but could not be
  satisfied`
* Re-read the shipment afterward: `status: active` — unchanged, no partial
  state corruption from the refused attempt.

### 3. Legitimate ship path continues to work (no regression)

* Completed the member task (`move 001.001-T --status done`).
* Ran `backlogit-verify.exe shipment ship 001-S` (legacy/non-formal-enforced
  path, `BACKLOGIT_FORMAL_GATE_REQUIRED` unset for this step).
* **Observed**: `shipment_status: "shipped"`, `archived_ids: ["001.001-T",
  "001-S"]`, `returned_ids: []` — the shipment and its member archived
  correctly via the intended path.

## Evidence

Commands, arguments, and raw output are reproduced verbatim in the session
transcript for this closure; the two most load-bearing observations are
quoted above (the exact refusal message and exit code, and the successful
`shipment_status: "shipped"` JSON).

## Follow-Up Risks

* None blocking. Two lower-severity findings from the review process are
  tracked as backlog follow-ups (`106.032-T` — base_ref binding schema v2
  candidate; `106.033-T` — repository-ref CAS/guard for the narrow
  post-manifest-signing HEAD-drift window) rather than verified here, since
  both are intentionally deferred design work, not regressions in currently
  shipped behavior.

## Handoff to Operational Closure

* **Verdict**: PASS
* **Surfaces verified**: CLI (`init`, `add`, `move`, `list`, `shipment
  create/claim/ship`), exercising the same core entry points MCP tools share
* **Evidence**: quoted command output above; full transcript in session history
* **Risky action state**: N/A (no destructive actions taken outside the
  disposable scratch workspace, which was fully removed after verification)
* **Follow-up recommendations**: monitor for `formal_gate_proof_invalid` /
  `formal_gate_proof_unverifiable` / `ErrFormalGateRequired` refusal rates
  once formal enforcement is enabled in a real deployment, to catch
  unexpected key-rotation or config-drift issues early (see operational
  closure monitoring plan)
