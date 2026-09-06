---
chunk_strategy: h1-h2-h3
description: "Deliberation: governed archived-shipment reconciliation to shipped (GH #423)"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-05-423-archived-shipment-reconciliation-to-shipped.md
title: "Deliberation — Governed archived-shipment reconciliation to shipped (#423)"
---

# Deliberation — Governed archived-shipment reconciliation to shipped (#423)

**Stash:** 055D507E (critical, feature)
**Issue:** softwaresalt/backlogit#423
**Date:** 2026-09-05

## Problem Frame

Legacy shipments delivered before the shipped-envelope hardening were archived
directly from `active`, so they carry `status: archived`, `archived_status: active`,
and **no** `shipment_status_changed -> shipped` event. The general
archived-lifecycle reconciliation from PR #394 (`core.ReconcileArchivedLifecycle`,
`internal/core/archive_reconcile.go`) deliberately rejects `--target-status shipped`
(allowed: `done, accepted, rejected, abandoned`; validation at
`archive_reconcile.go:111-116`, rationale comment at `:24`), while `core.ShipShipment`
(`internal/core/shipment_lifecycle.go:365`) requires a LIVE `active` shipment. The
record is therefore permanently unrepairable, and downstream predecessor-status
gates (e.g. graphtor topology `PREDECESSOR_NOT_SHIPPED`) block successors.

`internal/core/doctor.go:680-699` is authoritative: a missing shipped event is
treated as permanently absent and **must never be synthesized**. Any repair MUST
NOT fabricate a normal `shipment_status_changed -> shipped` event.

## Code Grounding (current reality)

| Fact | Location |
|---|---|
| General reconcile allowed targets = {done, accepted, rejected, abandoned}; `shipped` rejected | `archive_reconcile.go:18-24, 111-116` |
| Reconcile already appends a DISTINCT `lifecycle_reconciliation` event (not a transition event) | `archive_reconcile.go:304-320` |
| Reconcile mechanics: frontmatter validate (`status==archived` + nonempty `archived_status`), idempotency key `custom_fields.reconciliation_idempotency_key`, `CheckChildrenTerminal`, `UnarchiveItem` → update → `ArchiveItem`, atomic `replaceFileWithOptions`, clean-failure re-archive, `lifecycle_reconciliation_forward_recovery` on re-archive failure | `archive_reconcile.go:155-320` |
| `ShipShipment` requires `active`; membership lock; member+shipment gate (`gateShipmentCompletion`); snapshot/`restoreShipArtifacts` rollback; `moveShipmentStatusWithHeadGuard(..., ShipmentShipped)` | `shipment_lifecycle.go:365-687`, `:467-510` |
| Normal ship event = `shipment_status_changed {status: shipped}` | `shipment.go:240-243` |
| Shipment statuses (queued/active/shipped/abandoned) | `shipment.go:27-36`; `models/artifact.go:10-24` (adds `archived`) |
| Archive at `.backlogit/archive/`; `status: archived` + `archived_status: <pre>`; `ArchiveItem`/`UnarchiveItem`; `FindArtifactPath` | `archive.go:101-171, 775`; `artifacts.go:720` |
| Event struct + append (`appendShipmentEventErr` w/ `LockItemLogCrossProcess`); event append NOT atomic → `ErrWriteIndeterminate` | `events/stream.go:20-28, 207-210`; `shipment_events.go:54-113` |
| `MoveShipmentStatus` rejects ungoverned top-level `active->shipped` with `ErrShipmentShippedRequiresEnvelope` | `shipment.go:198-207` |
| Predecessor-shipped satisfied by archived record with `status: archived` + `archived_status: shipped` under `.backlogit/archive/` (no literal `PREDECESSOR_NOT_SHIPPED` in backlogit core; it is graphtor's gate) | `status_taxonomy.go:44-54`; `models/artifact.go:20-24` |
| S12/164.002-T = queued→active forward repair — QUEUED backlog item, NOT implemented; normal queued→active = `ClaimShipment` | `164-F`, `164.002-T`; `shipment.go:127-134` |

## Decisions

> **SUPERSEDED MECHANICS NOTE (adversarial review cycle 1 P0):** the "unarchive → update → re-archive" mechanics originally sketched below (D1, D4, D6) were REJECTED during plan review — `ArchiveItem`'s `archiveShippedEventPreflight` (144-F) refuses to archive a `shipped` shipment without a synthetic `shipment_status_changed` event, and the round-trip opens a non-atomic live-queue window. The APPROVED design (see the linked plan) performs an **IN-PLACE ATOMIC edit of the archive file** (`.backlogit/archive/<id>.md`): keep `status: archived`, set `archived_status: shipped` via `replaceFileWithOptions` + `UpsertItem`, never entering the live queue and never calling `UnarchiveItem`/`ArchiveItem`. Where the decisions below say "re-archive"/"unarchive", read "in-place archive-file edit" per the plan.

### D1 — Surface: dedicated shipment-scoped CLI command (not a general-reconcile extension)

**Options:** (a) extend `reconcile` to allow `shipped` behind shipment-specific guards; (b) a NEW dedicated CLI subcommand under `shipment`.

**Decision: (b)** — `backlogit shipment reconcile-shipped <id>` (CLI-only, operator-authorized). It REUSES the #394 governed reconciliation CONVENTIONS (archived-frontmatter validation, idempotency metadata, distinct durable event, indeterminate-write handling) but performs an IN-PLACE archive-file edit (NOT unarchive/update/re-archive — see the superseded-mechanics note above) and enforces STRICTER shipment-specific preconditions and appends a DISTINCT shipment-reconciliation event. The general `reconcile` allowlist stays UNCHANGED (still rejects `shipped`). Rationale: broadening the general allowlist would weaken a deliberately strict gate and risk ungoverned `shipped` writes on ordinary items; a dedicated command keeps the strict shipment envelope and matches the issue's "shipment-specific (or strictly guarded extension)".

### D2 — Distinct durable event, never a synthetic normal-shipping event

**Decision:** append a NEW event type `shipment_reconciled_shipped` (distinct from `shipment_status_changed`). Delta carries `before` (`status`, `archived_status`), `after` (`archived_status: shipped`), `reason`, `actor`, `idempotency_key`, and `evidence` (merge SHA + closure-evidence ref + verified member terminal set). This satisfies "distinct durable reconciliation event with before/after/evidence" and honors `doctor.go:680-699` (no synthetic `shipment_status_changed -> shipped`). Doctor/topology must recognize this event as a governed shipped-repair marker.

### D3 — Evidence is verified from explicit inputs, never inferred

**Decision:** required inputs `--reason`, `--actor`, `--idempotency-key`, `--merge-sha`, `--closure-evidence <path>` (and optional `--evidence-ref` repeated). The op VERIFIES: (i) `--merge-sha` resolves to a real commit in the repo; (ii) `--closure-evidence` path exists and is non-empty; (iii) every non-descoped manifest member is terminal (`done`) via a shipment-member check; (iv) NO conflicting `shipment_status_changed -> shipped` OR prior `shipment_reconciled_shipped` event exists; (v) `archived_status` ∈ the explicitly-supported legacy pre-state set (`{active}` at v1). Ambiguous/partial/conflicting → fail closed (typed error). Nothing is inferred.

### D4 — Repaired shape preserves predecessor semantics + member/parent bytes

**Decision:** the shipment archive file is edited IN PLACE to `status: archived`, `archived_status: shipped` (so downstream predecessor-status checks observe `shipped` from the archived record) — NOT unarchived/re-archived (see superseded-mechanics note). Manifest members and the covering feature/parent are NOT mutated — byte-for-byte preserved. Only the shipment's own frontmatter (`archived_status`, reconciliation metadata) changes.

### D5 — Idempotency

**Decision:** persist `custom_fields.shipment_reconciliation_idempotency_key`; a replay with the same key + same target is a NO-OP returning the ORIGINAL result (mirrors `archive_reconcile.go:196-203`). A DIFFERENT key against an already-reconciled record fails closed (conflicting-history check D3.iv).

### D6 — Atomicity, concurrency, rollback, indeterminate writes

**Decision:** membership → shipment artifact → (slow evidence I/O) → member artifact locks across the transition; atomic `replaceFileWithOptions` for the IN-PLACE shipment archive-file rewrite; snapshot the shipment file BYTES and the SQLite index row before mutation and restore both on clean failure. The event append is the LAST step via a DURABLE fsync-backed appender in a single handle-bound critical section: on `ErrWriteIndeterminate` (`stream.go:207-210`) report an indeterminate result (NO second append); on `ErrWriteNotApplied` roll back file+index; doctor `detectMissingShippedEvents` is the durable detector (no synthetic forward-recovery event to the same log). **Rollback triggers** (documented): frontmatter write/index-upsert failure → restore snapshot; conflicting-history discovered under lock → abort before write; indeterminate event write → indeterminate result + operator remediation guidance. (Full mechanics + the total state classifier are in the plan.)

### D7 — Dry-run / inspection + fail-closed

**Decision:** `--dry-run` prints the full precondition evaluation (pre-state, member terminality, evidence verification, conflict scan, planned before/after + event) and makes NO writes. All failures are typed and fail closed.

### D8 — Composability with S12 (164.002-T) — no conflation, no hard dependency

**Decision:** #423 and S12/164.002-T both build on the #394 governed reconciliation foundation but are DISTINCT contracts: 164.002-T repairs a LIVE `queued -> active` record; #423 repairs an ARCHIVED `active -> shipped` historical record with the strongest evidence gate. They MUST NOT be conflated (different pre-states, live-queue vs archive provenance, and #423 requires shipment gates 164.002-T does not). **No hard dependency edge** — #423 reuses EXISTING #394 primitives directly so it is NOT blocked by 164-F. **Ordering:** #423 is prioritized AHEAD of 164-F (critical: reliability + audit integrity; unblocks graphtor 049-S). If a shared governed-reconciliation primitive is later extracted, #423 (shipping first) establishes it and S12 reuses it; the plan keeps the contracts composable (shared validation/idempotency/event helpers) without a build-time dependency.

## Scope Guardrails

- Do NOT modify `softwaresalt/graphtor-docs` from this workspace. graphtor `048-S` is an EXTERNAL acceptance scenario only — reproduced as a fixture in backlogit tests.
- Do NOT broaden general `reconcile` to `shipped`.
- Preserve P-001 (single active implementation unit; do not interrupt an active shipment — none is active).
- Stage produces artifacts only; NO source implementation in this pipeline.

## Done Looks Like

A new CLI `shipment reconcile-shipped` + core primitive that repairs a legacy
archived-active shipment to `archived_status: shipped` with a distinct durable
`shipment_reconciled_shipped` event, strict verified evidence, idempotent replay,
dry-run, fail-closed rejects, and CLI/core/event/concurrency/rollback/integration
tests including a graphtor `048-S`-shaped fixture. The repaired record satisfies
downstream predecessor-status checks.
