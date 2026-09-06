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

> **SUPERSEDED MECHANICS NOTE (adversarial review cycle 1 P0):** the "unarchive → update → re-archive" mechanics originally sketched below (D1, D4, D6) were REJECTED during plan review — `ArchiveItem`'s `archiveShippedEventPreflight` (144-F) refuses to archive a `shipped` shipment without a synthetic `shipment_status_changed` event, and the round-trip opens a non-atomic live-queue window. The APPROVED design (see the linked plan) performs an **IN-PLACE ATOMIC edit of the archive file** (`.backlogit/archive/<id>.md`): keep `status: archived`, set `archived_status: shipped` via a dedicated HANDLE-RELATIVE atomic writer bound to the validated archive-directory handle (NOT path-based `replaceFileWithOptions`) + `UpsertItem`, never entering the live queue and never calling `UnarchiveItem`/`ArchiveItem`. Where the decisions below say "re-archive"/"unarchive", read "in-place archive-file edit" per the plan.

### D1 — Surface: dedicated shipment-scoped CLI command (not a general-reconcile extension)

**Options:** (a) extend `reconcile` to allow `shipped` behind shipment-specific guards; (b) a NEW dedicated CLI subcommand under `shipment`.

**Decision: (b)** — `backlogit shipment reconcile-shipped <id>` (CLI-only, explicit-confirmation-gated + fully audited). **Authorization framing (truthful):** the command is NOT machine-authenticated as "operator-only" — an autonomous caller can supply `--confirm` or allocate a TTY, and backlogit has no external operator trust anchor. The `--confirm`/TTY gate is an explicit-confirmation guard (prevents silent/accidental non-interactive invocation) and the durable audit event makes the action attributable and acknowledged; genuine operator-authenticated approval (an operator-issued per-invocation credential the agent cannot mint) is the SAME deferred residual as cryptographic authenticity (`866FDC8C`, broadened to explicitly track authenticated invocation authorization). The #423 issue's "operator-authorized" wording is CONSCIOUSLY REVISED to confirmation-only for v1 (confirmation + audit attribution, not machine-authenticated authorization), with authenticated authorization tracked separately rather than silently dropped. **Scope-narrowing ratification (P-021): because #423's issue text says "operator-authorized" and v1 delivers confirmation-only, this is a CONSCIOUS SCOPE NARROWING that REQUIRES explicit operator / issue-owner ACCEPTANCE before #423 is considered FULLY satisfied. Stage flags this for the operator and tracks the authenticated-authorization follow-up in `866FDC8C`; **this ratification is a BLOCKING gate recorded as task `167.015-T` (blocks 148-S closure) — Ship MUST obtain explicit operator acceptance of the confirmation-only v1 (or schedule `866FDC8C`) before #423 is closed**, rather than treating the shipped v1 as a literal-complete implementation of the issue as worded.** It REUSES the #394 governed reconciliation CONVENTIONS (archived-frontmatter validation, idempotency metadata, distinct durable event, indeterminate-write handling) but performs an IN-PLACE archive-file edit (NOT unarchive/update/re-archive — see the superseded-mechanics note above) and enforces STRICTER shipment-specific preconditions and appends a DISTINCT shipment-reconciliation event. The general `reconcile` allowlist stays UNCHANGED (still rejects `shipped`). Rationale: broadening the general allowlist would weaken a deliberately strict gate and risk ungoverned `shipped` writes on ordinary items; a dedicated command keeps the strict shipment envelope and matches the issue's "shipment-specific (or strictly guarded extension)".

### D2 — Distinct durable event, never a synthetic normal-shipping event

**Decision:** append a NEW event type `shipment_reconciled_shipped` (distinct from `shipment_status_changed`). Delta carries `before` (`status`, `archived_status`), `after` (`archived_status: shipped`), `reason`, `actor`, `idempotency_key`, and `evidence` (merge SHA + closure-evidence ref + verified member terminal set). This satisfies "distinct durable reconciliation event with before/after/evidence" and honors `doctor.go:680-699` (no synthetic `shipment_status_changed -> shipped`). Doctor/topology must recognize this event as a governed shipped-repair marker.

### D3 — Evidence is verified from explicit inputs, never inferred

**Decision:** required inputs `--reason`, `--actor`, `--idempotency-key`, `--merge-sha`, `--closure-evidence <path>`; optional `--second-approver`, `--evidence-ref` (repeated). The op VERIFIES (final contract, per plan U2 — supersedes the weaker sketch): (i) `--merge-sha` validated by `isGitObjectName` then reachable from a TRUSTED REF (`WorkspaceConfig.reconcile.trusted_refs`; default = repo default branch), under `boundedHelperTimeout`; (ii) `--closure-evidence` opened handle-safe (real-path contained, no-follow, regular non-empty) and MUST REFERENCE this shipment id (accepts the real graphtor 048-S closure shape — shipments-list frontmatter + narrative SHA); (iii) EVERY manifest member terminal via the `{done, accepted}` allowlist (a `rejected`/descoped member fails closed `ErrUnsupportedLegacyDescope` — v1 has NO descoping; aligned with 167.014-T + plan U2; this no-descoping narrowing is a CONSCIOUS v1 scope decision RATIFIED via blocking gate 167.015-T, parallel to the authorization narrowing), manifest NON-EMPTY; (iv) NO conflicting/torn history via the shared state classifier; (v) `archived_status` ∈ `{active}` (v1). The manifest match is verified against the shipment's OWN manifest, not required to be enumerated in the closure; a `evidence_digest` over (shipment-id + merge-sha + trusted_ref_name + trusted_ref_tip + manifest-digest + closure content-hash + evidence-refs) binds the evidence; the merge SHA is checked reachable against the RESOLVED PINNED TIP of the selected trusted ref (name + tip recorded in the event), and the `request_identity_digest` is folded into the durable event so the full-event digest covers the replay identity (the frontmatter scalar cannot be swapped independently). Ambiguous/partial/conflicting → fail closed (typed). Nothing is inferred.

### D4 — Repaired shape preserves predecessor semantics + member/parent bytes

**Decision:** the shipment archive file is edited IN PLACE to `status: archived`, `archived_status: shipped` (so downstream predecessor-status checks observe `shipped` from the archived record) — NOT unarchived/re-archived (see superseded-mechanics note). Manifest members and the covering feature/parent are NOT mutated — byte-for-byte preserved. Only the shipment's own frontmatter (`archived_status`, reconciliation metadata) changes.

### D5 — Idempotency

**Decision:** persist `custom_fields.shipment_reconciliation_idempotency_key`, a scalar `shipment_reconciliation_request_identity_digest` (over the request SCALARS — idempotency_key/merge_sha/reason/actor/second_approver/evidence_refs/closure PATH, NO closure content re-read), the CANONICAL PREPARED EVENT PAYLOAD (`shipment_reconciliation_prepared_event`), AND `shipment_reconciliation_event_digest` (over the complete payload); a same-key replay is a NO-OP ONLY when `archived_status: shipped` AND a fully-valid readable event matches AND the incoming REQUEST-IDENTITY digest matches the persisted `shipment_reconciliation_request_identity_digest` (so replay performs NO closure/evidence I/O and still returns no_op if the closure was later renamed/deleted); frontmatter-shipped-but-event-missing → RESUME by appending the persisted canonical payload verbatim (gated on the persisted prepared payload matching `shipment_reconciliation_event_digest`); a different/absent key, or a key match with a different request-identity digest, fails closed (conflict). The full `shipment_reconciliation_event_digest` (content-hash-bound) is reserved for prepared-payload integrity on resume and the audit event, NOT for replay identity. See the plan's total state classifier for the exhaustive rule.

### D6 — Atomicity, concurrency, rollback, indeterminate writes

**Decision:** a TWO-PHASE lock protocol (final canonical form — supersedes any earlier sequential ordering): **Phase A** acquire `lockShipmentMembership(shipment)` (HELD across ALL phases — blocks `AddItemToShipment`-path manifest changes); compute the member set + the scalar request-identity digest; **Phase B** acquire the item-log lock C, then the sorted artifact batch B (order C→B matching AssociateCommit), run the identity/location gate + classify on request-identity ONLY (NO evidence I/O); NON-APPEND branches (no_op/conflict/indeterminate) finalize + release B, C, A; the 2b RESUME path runs HERE under the held C+B (re-read log + append the persisted prepared Event verbatim, release B, C, A); only 2d NORMAL-REPAIR releases both B and C (keeps A) and continues to Phase C for slow evidence I/O; **Phase C** (branch 2d only) run the SLOW evidence I/O with only membership lock A held (no B, no C); **Phase D** (2d) acquire C then B (order C→B matching AssociateCommit), member-set equality check (`UpdateArtifact` can change `custom_fields.items` under B without A → mismatch = `indeterminate`), member terminality, authoritative re-classify, snapshot, then write frontmatter via the HANDLE-RELATIVE atomic writer (NOT path-based `replaceFileWithOptions` — see plan U1) AND append the durable event ATOMICALLY under the held C+B (no gap → no concurrent clobber, no split-brain rollback), release B then C; release A. **The transaction holds B and C together only in the C→B order (matching AssociateCommit) for CLOBBER-SAFETY**; there is NO reconcile deadlock. Grounded in code: A (`lockShipmentMembership`) and B (`lockArtifactMutations`) are BOUNDED-wait locks (`lockTaskFileWithHeartbeat`, `task_lock.go:96/111-127` → `ErrGateInProgress` after 3s), so no B/C acquisition order can form an unbreakable cycle; the ONLY indefinite lock is the in-process item-log C mutex (`stream.go:125`), and reconcile holds exactly ONE C-mutex (single shipment item) acquired FIRST, so it is provably not a node in any C-mutex wait cycle. The pre-existing CROSS-ITEM cascade C-mutex ordering (`ArchiveItem` WithCascade parent→child, `archive.go:99,352-362`; `AdoptItem` batch) — RETRACTED: direct code verification shows the existing cascade/adopt C paths are INDEPENDENTLY SAFE (`ArchiveItem` archives descendants BEFORE taking the parent C-lock, `archive.go:119-124` before line 292; `lockAdoptionEventLogs` SORTS ids, `shipment_lifecycle.go:1332-1333`), so there is NO cross-item C-mutex cycle. A "never hold both" alternative was rejected: releasing B before the append opens a window where a concurrent `AssociateCommit` (reads the artifact before taking C, `commits.go:60-102`) or `ArchiveItem` overwrites `archived_status`, and the rollback then erases a valid update or needs B-under-C (deadlock). A concurrency test vs `AssociateCommit` AND `ArchiveItem` is required. SEPARATE serial-clobber gap (PR #424 round 40): a LATER `ArchiveItem` re-archive on the reconciled shipment restamps `archived_status` back to `archived` (`archive.go:230` sets it from `oldStatus`=`archived`); guarded by task `167.021-T` (preserve an existing non-empty `archived_status` on re-archive), a blocking dependency of `167.008`/`167.020`. On `ErrWriteIndeterminate` (`stream.go:207-210`) report indeterminate (NO second append); on `ErrWriteNotApplied` roll back file+index UNDER the held C+B; doctor `detectMissingShippedEvents` is the durable detector. **Rollback triggers** (documented, distinguishing write-outcome class): a FRONTMATTER-write `ErrWriteNotApplied` (proven not written) → restore file+index snapshot (upsert-or-delete by original row-presence); a FRONTMATTER-write `ErrWriteIndeterminate` (rename may already have committed `archived_status: shipped`) → report `indeterminate` and DO NOT restore (restoring the snapshot could OVERWRITE an already-committed repair — explicitly forbidden), leaving a later replay to re-read the durable state; conflicting-history discovered under lock → abort before write; indeterminate event write → indeterminate result + operator remediation guidance. (Full mechanics + the total state classifier are in the plan.)

**D6.1 — Lock-order prerequisite reclassification, then RE-GROUNDING (PR #424 rounds 23-36):** The pre-existing B↔C lock-order difference (ArchiveItem B→C vs AssociateCommit C→B) was initially (rounds 23-24) captured as stash 763A1152, then over-scoped into an 8-task B/C-order migration prerequisite chain (167.017-024) on the belief that all three locks were indefinite-blocking. Round-36 review (thread ft-pJ) prompted DIRECT VERIFICATION of the lock code, which established that A and B are BOUNDED-wait (`task_lock.go:96`, 3s → `ErrGateInProgress`), NOT indefinite — so no B/C order can deadlock, and reconcile (holding a single C-mutex acquired first) is provably deadlock-free. The 8-task migration chain was therefore OVER-SCOPED (a Scope-Boundary / YAGNI violation grounded in a mis-characterized lock model) and was REMOVED: tasks 167.018, 167.021, 167.022, 167.023, 167.024 were deleted; 167.008 now depends only on its RECONCILE-LOCAL safety prerequisites — 167.016 (snapshot/rollback), 167.017 (stable C-lock identity, narrowed to logs-dir reconfiguration), 167.019 (reload/CAS clobber-safety), 167.020 (reconcile-scoped concurrency proof). Round-37 review (threads fuKad/fuKbE) then established — by direct code verification — that even the cross-item cascade/adopt hypothesis is UNFOUNDED: `ArchiveItem` archives descendants before taking the parent C-lock (`archive.go:119-124` before line 292) and `lockAdoptionEventLogs` sorts ids (`shipment_lifecycle.go:1332-1333`), so NO opposite-order multi-C holder exists and there is no deadlock anywhere. The `EB9A4008` follow-up and its predecessor `763A1152` were RETRACTED accordingly. Net: 148-S shrank from 25 to 20 items (19 tasks + feature). This is a truthful SCOPE CORRECTION, not scope creep — the removed work was never a reconcile-correctness prerequisite.

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
