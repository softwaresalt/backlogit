---
session: stage
date: 2026-06-30
stash_processed: [AE0838A9]
feature_created: 071-F
shipment_created: 071-S
status: complete
---

# Stage session — backlogit Deterministic-Gates slice (AE0838A9)

## Outcome

Ran the full stash→backlog pipeline for stash `AE0838A9` (feature-shaped, medium)
— the **backlogit-owned** slice of the Deterministic Gates, Telemetry & Evaluation
Engine initiative. Produced a reviewed backlog hierarchy and a **queued shipment**
ready for Ship.

## Artifacts

- **Deliberation**: `docs/decisions/2026-06-30-backlogit-deterministic-gates-slice-deliberation.md`
  - Resolved Q1 (gate-failure state transition reuses existing `move --status blocked/queued`
    — no new command; policy owned by autoharness) and Q3 (telemetry stays in
    `.autoharness/metrics`; no backlogit storage change).
  - Options A1 (dedicated body-preserving `SetArtifactSize` seam), B1 (reuse header-def
    validation for target mode), C1 (advisory sidecar lock).
- **Plan**: `docs/exec-plans/2026-06-30-backlogit-deterministic-gates-slice-plan.md`
  - 9 units (U1–U9), Requirements Trace, Dependency Graph, Plan Hardening (P-006:
    `Requires plan hardening: yes` → hardened), Plan Review.
- **Plan review**: Attempt 1 **FAIL** (convergent P1s on persistence/model/validation
  seam) → plan revised → Attempt 2 **PASS** (Go Reviewer PASS; Architecture one residual
  P1 = full-row `UpsertItem` reconstruction spec-gap, closed by adopting the reviewer's
  exact remediation + a guarding assertion; all P2/P3 incorporated).

## Backlog created

- **Feature 071-F** (queued) — references plan, deliberation, design doc.
- **9 tasks** (all queued, test-first posture), sizes and deps:

| Task | Unit | Size | Phase | Depends on | Summary |
|---|---|---|---|---|---|
| 071.001-T | U1 | S | 1 | — | core DoctorTarget single-file validation |
| 071.002-T | U4 | S | 1 | — | per-task advisory lock sidecar primitive |
| 071.003-T | U6 | XS | 2 | — | optional `size` T-shirt enum in header-def (custom_fields) |
| 071.004-T | U2 | M | 1 | U1 | `doctor --target` CLI + 5s timeout + exit-code table |
| 071.005-T | U3 | S | 1 | U1 | MCP `backlogit_doctor` target param (deferrable) |
| 071.006-T | U5 | S | 1 | U1,U4 | lock in DoctorTarget + busy→exit 4 semantics |
| 071.007-T | U7 | M | 2 | U4,U6 | `core.SetArtifactSize` body-preserving seam |
| 071.008-T | U8 | S | 2 | U7 | `update --size` CLI → SetArtifactSize |
| 071.009-T | U9 | S | 2 | U7 | MCP `backlogit_update_item` size param (deferrable) |

- Roots: U1, U4, U6. Suggested serial order for Ship:
  **U1 → U4 → U5 → U2 → U3 → U6 → U7 → U8 → U9**.

## Shipment (handoff token to Ship)

- **071-S** (queued) — 10 items (071-F + 9 tasks), parent-first, topological order.
  Scope guard applied: only the 10 harvest_ids added; no queue-scavenging.

## Stash

- **AE0838A9 archived** with forward-link marker → 071-F / 071-S.
- Deferred (untouched, unrelated): 21E17BFC (singleton MCP contingency, low),
  D070FD3C (shipment-vs-feature numbering UX, low).

## Key technical decisions (grounding for Ship)

- `size` stored under `custom_fields.size` (model only round-trips known keys + nested
  custom_fields; a top-level key is silently dropped).
- Single write seam `core.SetArtifactSize(ctx,ws,id,size)`: enum-validate → per-task
  lock → mdfront.Decode → set custom_fields.size → mdfront.Encode →
  atomicfile.WriteFileAtomic → db.UpsertItem with a **fully-reconstructed**
  `*models.Artifact` (UpsertItem is INSERT OR REPLACE on the full row — a partial stub
  wipes title/status/priority).
- Targeted enum check at mutation entry (do NOT retrofit global enum onto
  ValidateArtifactFields — regresses legacy artifacts).
- 5s timeout = goroutine + select(ctx.Done()) with buffered channel; exit codes
  0/1/2/3/4; autoharness subprocess `timeout_seconds:5` is the authoritative outer bound.
- Per-path `map[string]*sync.Mutex` + O_EXCL sidecar + 60s stale-TTL; busy = non-blocking
  distinct error → exit 4.
- `--size` mutually exclusive with other update flags (avoids double-write negating body
  preservation).

## Next (Ship's scope — NOT Stage)

Ship claims shipment **071-S**, executes U1→U9 in dependency order on a feature branch,
runs builds/tests/CI, opens the PR. Stage does not build code, branch, or open PRs.
