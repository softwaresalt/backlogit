---
chunk_strategy: h1-h2-h3
description: 'Close the two non-ShipShipment paths that can produce archived_status shipped without a durable shipped event.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-08-18-shipment-shipped-prevention-hardening-plan.md
title: 'Prevention hardening for non-ShipShipment shipped transitions and archive stamping'
---

## Problem Frame

Feature 143-F / shipment 127-S (both archived) shipped the DETECTION half of
shipped-event integrity: the governed `core.ShipShipment` envelope appends a
durable `shipment_status_changed` event with `status: shipped`, and the report-only
doctor audit `check_shipped_event_completeness` flags any archived shipment whose
`archived_status: shipped` lacks that durable event. This plan implements the
PREVENTION half so that non-ShipShipment paths can no longer create the residue
the doctor flags.

Source deliberation: `docs/decisions/2026-08-18-shipment-shipped-prevention-hardening-deliberation.md`
(source stash `47B48DB0`).

Concrete holes in `origin/main`. Guard 1 must cover every non-ShipShipment
producer of a shipment `shipped` status; guard 2 covers the archive stamping seam:

* **Guard 1 — generic transition**: `UpdateArtifactWithGate`
  (`internal/core/gate_transition.go:109-116`) rejects a shipment moving to
  `shipped` only when `ws.formalGateEnforced()` is true. With formal enforcement
  OFF (the default unless `BACKLOGIT_FORMAL_GATE_REQUIRED` or workspace opt-in is
  set), a generic `move_item` / `update_item` drives a shipment to `shipped` with
  no gate and no durable event. The gate broker is not wired when disabled, so a
  gate-level rule cannot cover this case.
* **Guard 2 — archive stamping**: `ArchiveItem` (`internal/core/archive.go:222-223`)
  copies the pre-archive status into `archived_status` unconditionally. The
  status-transition validation hook is registered only for `HookUpdateArtifact`,
  not `HookArchiveItem`, so archive bypasses transition checks and can stamp
  `archived_status: shipped` with no durable-event verification.
* **Guard 1 — additional producers (surfaced by plan review)**: two further
  non-ShipShipment paths can set a shipment to `shipped`. The exported
  `MoveShipmentStatus` / `moveShipmentStatusWithHeadGuard(topLevel=true)` path
  transitions status with a best-effort event append, and `CreateArtifact`
  (`internal/core/artifacts.go`, reached by MCP `create_item` / `harvest_stash`
  and CLI `stash harvest`) accepts an initial `status` and today only refuses
  `archived`. Both must be refused so guard 1 is a complete envelope.

The shared durable-event predicate `shippedEventPresence(...)`
(`internal/core/doctor.go:613-650`) already exists and MUST be the single source
of truth both guards reuse, so prevention and detection scan the same literal
contract.

## Requirements Trace

| Requirement (from deliberation) | Implementation action | Unit |
|---|---|---|
| Reject generic shipment to `shipped` (update/move) outside ShipShipment, gate-independent | Unconditional refusal in `UpdateArtifactWithGate` (unlocked-peek fast-path) and in `updateArtifactUngated` after the lock+reload (authoritative check-to-write); refuse ungoverned `MoveShipmentStatus` (`topLevel=true`) | U1, U2, U10, U11 |
| Reject create-time shipment `status: shipped` | Refuse initial `shipped` for a shipment in `CreateArtifact` | U3, U4 |
| Archive refuses `archived_status: shipped` without durable event | Pre-stamp check in `ArchiveItem` reusing `shippedEventPresence`, fail-closed | U5, U6 |
| Reuse the detection predicate (no contract drift) | Call `shippedEventPresence` directly (same `core` package; no extraction) | U6 |
| Do not block done, abandoned, or P-015 safe-close | Scope guard 2 to shipment `artifact_type` with `oldStatus == shipped`; non-shipment artifacts never blocked | U5, U6 |
| MCP and CLI behave identically; stable error contract; no force lever | Enforce in shared core seams; wire sentinel surface mapping (U9); governed-parity tests through the registry | U7, U9 |
| Guards fail closed on missing or unreadable evidence | Missing or unreadable event refuses | U2, U4, U6, U7 |
| ShipShipment envelope remains the one sanctioned producer | ShipShipment exempt by construction; regression-tested | U1, U2, U5, U6, U11 |

## Implementation Units

Each unit obeys the 2-hour rule (fewer than 3 files, fewer than 5 functions,
fewer than 4 test scenarios), width isolation (single domain), and produces a
verifiable milestone. Guard tracks are red-harness-first (TDD), mirroring the
143-F decomposition convention.

### U1 — RED harness for guard 1 transition and move producers (tests)

* Domain: tests (red-harness)
* Changes: failing tests that a shipment `shipped` transition is refused through the
  generic producers — `UpdateArtifactWithGate` with `formalGateEnforced()` false,
  and the exported `MoveShipmentStatus` ungoverned path — while the governed
  `ShipShipment` path still ships
* Files: `internal/core/gate_transition_test.go` (extend), `internal/core/shipment_test.go` (extend)
* Tests: (1) `UpdateArtifactWithGate` shipment to `shipped` refused with gate OFF;
  (2) exported `MoveShipmentStatus(..., shipped)` ungoverned refused; (3)
  `ShipShipment` governed transition still succeeds
* Execution posture: test-first (red) — compiles and FAILS pre-implementation

### U2 — Implement guard 1 on the generic transition and move seams (core)

* Domain: code (core)
* Changes: refuse a shipment `shipped` transition UNCONDITIONALLY in
  `UpdateArtifactWithGate`, superseding the `formalGateEnforced()`-only branch.
  ShipShipment is exempt by construction — it writes via
  `moveShipmentStatusWithHeadGuard`, not this function — so do NOT thread a
  caller-origin flag through `updates`/`opts` (that would be a forgeable bypass).
  Refuse ungoverned `shipped` in `moveShipmentStatusWithHeadGuard` when
  `topLevel == true` (the governed `ShipShipment` call uses `topLevel == false` and
  stays allowed). Add a dedicated sentinel `ErrShipmentShippedRequiresEnvelope` in
  the leaf `internal/errors` (do NOT reuse `ErrFormalGateRequired`); its MCP
  `error_type` and CLI exit-code mapping is wired in U9. The refusal added here is
  the cheap unlocked-peek fast-path plus the move-seam guard; the authoritative
  post-lock revalidation on the locked write path (`updateArtifactUngated` in
  `internal/core/artifacts.go`) is a separate unit (U10 red harness, U11
  implementation) so this unit stays within the <3 production-file boundary
* Files: `internal/core/gate_transition.go`, `internal/core/shipment.go`,
  `internal/errors/errors.go` (sentinel definition only; surface mapping in U9)
* Tests: U1 harness turns green; existing ShipShipment tests stay green
* Execution posture: test-first (make U1 pass)
* Depends on: U1

### U3 — RED harness for guard 1 create producer (tests)

* Domain: tests (red-harness)
* Changes: failing tests that creating a shipment with an initial `status: shipped`
  is refused through the core `CreateArtifact` seam, while a shipment created at
  `queued` is unaffected. This unit is core-only: a `core`-package unit test cannot
  drive the MCP `create_item` / `harvest_stash` or CLI transport handlers, so
  surface-level create coverage is owned by U7 (parity)
* Files: `internal/core/144_create_guard_test.go` (new; mirrors the existing
  `066_create_guard_test.go` red-harness convention)
* Tests: (1) `CreateArtifact` shipment with `status: shipped` refused; (2)
  `CreateArtifact` shipment created at `queued` unaffected; (3) `CreateArtifact`
  non-shipment (e.g. task) create unaffected
* Execution posture: test-first (red)

### U4 — Implement guard 1 on the create seam (core)

* Domain: code (core)
* Changes: in `CreateArtifact`, reject an initial `status: shipped` for
  `artifact_type == shipment` (mirroring the existing `models.StatusArchived`
  refusal), reusing `ErrShipmentShippedRequiresEnvelope`. Shipments are created,
  then shipped only via `ShipShipment`
* Files: `internal/core/artifacts.go`, `internal/errors/errors.go` (reuse)
* Tests: U3 harness turns green; existing create paths stay green
* Execution posture: test-first (make U3 pass)
* Depends on: U3, U2

### U5 — RED harness for guard 2 archive stamping (tests)

* Domain: tests (red-harness)
* Changes: failing tests that `ArchiveItem` refuses to stamp `archived_status: shipped`
  on a shipment lacking a durable shipped event, allows it when the event is
  present, and leaves non-`shipped` closures unaffected
* Files: `internal/core/archive_test.go` (extend)
* Tests: (1) refuse shipped-without-event; (2) allow shipped-with-event (governed
  archival path); (3) allow `done`/`abandoned` and any non-shipment closure (covers
  P-015 safe-close and the `artifact_type == shipment` scoping)
* Execution posture: test-first (red) — seed a negative fixture so the test genuinely
  fails pre-fix

### U6 — Implement guard 2 in ArchiveItem (core)

* Domain: code (core)
* Changes: in `ArchiveItem`, key the check on the already-loaded artifact type and
  status — `artifactType == shipment && oldStatus == ShipmentShipped` (from the same
  `fm` map the stamp uses — no extra load, no DB read). When it matches, require a
  durable event via a direct call to `shippedEventPresence` (same `core` package; no
  extraction, `doctor.go` unchanged) and refuse fail-closed when the event is absent
  or unreadable. Run the preflight after the parent lock is acquired and before any
  descendant cascade or pre-archive hooks (hoisting the parent status read above
  `archiveDescendants` if needed), so a refusal leaves descendants untouched. Add a
  dedicated sentinel `ErrArchiveShippedRequiresEvent` in the leaf `internal/errors`
  (surface mapping in U9)
* Files: `internal/core/archive.go`, `internal/errors/errors.go` (sentinel definition only)
* Tests: U5 harness turns green; done/abandoned/P-015 archive stays green; the
  governed `ShipShipment` archival still succeeds when the event is present
* Execution posture: test-first (make U5 pass)
* Depends on: U5

### U7 — Governed MCP and CLI parity and integration tests (tests)

* Domain: tests (parity)
* Changes: tests dispatched through the authoritative registry that exercise
  `move_item` / `update_item` and `create_item` / `harvest_stash` (guard 1) and
  `archive_item` (guard 2) on both MCP and CLI surfaces, asserting identical
  refusal, the stable per-sentinel error contract (MCP `error_type` + CLI exit
  code), and durable JSONL + Markdown state (not just the index). Confirm no MCP
  force lever exists. Do NOT perform generic updates on already-archived items
  (they drop `archived_status`)
* Files: `internal/cli/registry_parity_test.go` (extend), `internal/mcp/tools_test.go` (extend)
* Tests: (1) transition/move/create to `shipped` refused identically on MCP and CLI
  with gate OFF; (2) archive of shipped-without-event refused identically, durable
  JSONL + Markdown asserted; (3) the same assertions repeated under a relative
  workspace root
* Execution posture: test-first (parity/characterization)
* Depends on: U2, U4, U6, U9

### U8 — Design-doc and compound learning (docs)

* Domain: docs
* Changes: a design-doc capturing the prevention/detection pairing, the ArchiveItem
  hook-gap, the gate-independent core seam, the full producer set (transition, move,
  create, archive), the peek-vs-locked check-to-write authority (U2 peek / U11 locked
  revalidation), and the fail-closed scoping; a compound learning graduating the
  reusable insight
* Files: `docs/design-docs/2026-08-shipment-shipped-prevention-envelope.md`,
  `docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md`
* Tests: docline frontmatter lints clean (`backlogit docs lint`)
* Execution posture: docs
* Depends on: U2, U4, U6

### U9 — Surface error-adapter mapping for the new sentinels (code)

* Domain: code (surface adapters)
* Changes: map `ErrShipmentShippedRequiresEnvelope` and `ErrArchiveShippedRequiresEvent`
  to a stable MCP `error_type` (plus remediation) and CLI exit code in the existing
  `internal/mcp` / `internal/cli` gate-error adapters, keeping `internal/errors` a
  leaf package with no surface dependency
* Files: `internal/mcp/errors.go` (extend), the `internal/cli` error-mapping site (extend)
* Tests: adapter unit tests assert each sentinel maps to its `error_type` / exit code
* Execution posture: test-first
* Depends on: U2, U4, U6

### U10 — RED harness for guard 1 authoritative post-lock revalidation (tests)

* Domain: tests (red-harness)
* Changes: a failing test that distinguishes the cheap unlocked peek from the
  authoritative locked-state refusal. Because the `core`-package test can call the
  unexported `updateArtifactUngated` directly, it drives the locked write path with a
  shipment `shipped` transition and asserts refusal originates there (not only at the
  `UpdateArtifactWithGate` peek), and that no durable Markdown/JSONL write is applied.
  Reuses `ErrShipmentShippedRequiresEnvelope` (defined in U2)
* Files: `internal/core/144_locked_revalidation_test.go` (new)
* Tests: (1) `updateArtifactUngated` shipment `queued -> shipped` refused with
  `ErrShipmentShippedRequiresEnvelope` after the lock+reload; (2) the refusal leaves
  the durable artifact unmutated (no write applied); (3) a non-shipment task
  transition through `updateArtifactUngated` still writes under lock (no false
  positive)
* Execution posture: test-first (red) — compiles and FAILS pre-implementation because
  the locked path does not yet refuse
* Depends on: U2

### U11 — Implement guard 1 authoritative post-lock revalidation (core)

* Domain: code (core)
* Changes: inside `updateArtifactUngated` (`internal/core/artifacts.go`), after
  `lockArtifactMutations` and the authoritative `findArtifact` reload, refuse a
  shipment `shipped` transition (`artifactType == shipment` and
  `updates["status"] == shipped`) by returning `ErrShipmentShippedRequiresEnvelope`,
  co-locating the check with the locked write so the guarantee does not depend on the
  unlocked peek alone (closes the peek-to-write TOCTOU). ShipShipment stays exempt —
  it ships via `moveShipmentStatusWithHeadGuard`, never this generic function — and
  non-status updates to an already-`shipped` shipment are unaffected (they set no
  `status` in `updates`). No new sentinel; reuses U2's
* Files: `internal/core/artifacts.go`, `internal/errors/errors.go` (reuse only)
* Tests: U10 harness turns green; existing update, gated-completion, and ShipShipment
  paths stay green
* Execution posture: test-first (make U10 pass)
* Depends on: U10, U2

## Dependency Graph

```text
U1 (red) ──▶ U2 (transition/move guard)
U2 ──▶ U10 (red) ──▶ U11 (locked-path revalidation)
U3 (red) ──▶ U4 (create guard)
U5 (red) ──▶ U6 (archive guard)
U2, U4, U6 ──▶ U9 (surface error mapping) ──▶ U7 (parity)
U11 ──▶ U7 (parity)
U2, U4, U6 ──▶ U8 (docs)
```

* U1, U3, and U5 have no upstream dependencies and can start first (all red harnesses)
* U2 depends on U1; U4 depends on U3 and U2 (shared sentinel); U6 depends on U5
* U10 (red) depends on U2; U11 (authoritative locked-path revalidation) depends on U10 and U2
* U9 depends on U2, U4, and U6 (wires all three sentinels' surface mapping)
* U7 depends on U2, U4, U6, U9, and U11; U8 (docs) depends on U2, U4, and U6
* The graph is acyclic

Estimated effort: 11 units times ~2 hours = ~22 hours.

## Decisions and Rationale

* **Enforce in core seams, not the gate**: the gate broker is not wired when
  formal enforcement is off, so a gate rule is a no-op in exactly the case guard 1
  must cover. Refusing in `UpdateArtifactWithGate` is gate-independent and covers
  every surface that funnels through core.
* **Unconditional refusal, no origin flag**: `ShipShipment` never calls
  `UpdateArtifactWithGate` (it writes via `moveShipmentStatusWithHeadGuard`), so the
  refusal at that seam is unconditional and needs no exemption. Do not thread a
  caller-origin flag through `updates`/`opts` — that would be a forgeable MCP/CLI
  bypass lever, violating "no force lever".
* **Do not block on `isValidShipmentTransition`**: that predicate is on the live
  `ShipShipment` path (`moveShipmentStatusWithHeadGuard` depends on it returning
  true for `active -> shipped`). A blanket `shipped` refusal there would break real
  ships. Guard the ungoverned move at `moveShipmentStatusWithHeadGuard(topLevel=true)`
  instead.
* **Cover the create producer**: `CreateArtifact` accepts an initial `status` and
  today only refuses `archived`; refuse an initial `shipped` for shipments so a
  shipment cannot be born outside the envelope.
* **Explicit archive-seam check for guard 2**: the transition-validation hook does
  not fire on `ArchiveItem`, so guard 2 needs its own check. Run it after the parent
  lock and before any cascade or hooks so a refusal leaves descendants untouched.
* **Reuse `shippedEventPresence` directly**: prevention and detection must scan the
  same literal event contract; a single predicate prevents drift. All callers are in
  `package core`, so it is a direct intra-package call — no extraction, and
  `doctor.go` is unchanged.
* **Key guard 2 on shipment type + pre-archive `oldStatus`**: gate on
  `artifactType == shipment && oldStatus == ShipmentShipped`. `ArchiveItem` already
  reads `oldStatus` from the Markdown `fm` and stamps `archived_status` from that
  same map, so the single-load property is intrinsic; test the pre-archive status,
  not a not-yet-written `archived_status`. Non-shipment artifacts are never blocked.
* **Distinct sentinels in a leaf, mapping in the adapters**: define
  `ErrShipmentShippedRequiresEnvelope` and `ErrArchiveShippedRequiresEvent` in the
  leaf `internal/errors` (not `ErrFormalGateRequired`), and wire each to an MCP
  `error_type` and CLI exit code in the surface adapters (U9) so the parity tests
  can assert an identical contract.
* **Scope strictly to `shipped`**: `done`, `abandoned`, and the P-015
  single-artifact safe-close (which archives as `done`) must not be blocked.

## Risks and Caveats

* **False positives blocking legitimate closures** — mitigate by scoping to
  `shipped`, reusing the detection predicate, and explicitly testing done/abandoned/P-015.
* **Durable-write indeterminacy** — any new append must honor the commit-then-surface
  two-class contract (`ErrWriteIndeterminate` vs `ErrWriteNotApplied`); never roll
  back an indeterminate write.
* **MCP/CLI drift** — mitigate with governed-parity tests dispatched through the
  authoritative registry against durable artifacts; no MCP force lever exists.
* **Merged is not operative** — the pinned `backlogit` binary must be rebuilt
  before the guards protect real `.backlogit/` operations; verify the embedded
  commit at closure. The doctor audit remains the after-the-fact net.
* **Guard 2 also runs on governed archival** — `ShipShipment` archival calls the
  same `ArchiveItem` while the shipment sits at `shipped`; normally the durable event
  is present so it is allowed, but a transient JSONL read failure would fail-closed
  and leave shipped-and-unarchived residue (no rollback). This is the accepted
  residue class the doctor audit catches; add a test that governed ship still
  archives when the event is present.
* **Check-to-stamp ordering** — guard 1's authoritative refusal must apply on the
  locked update path (`updateArtifactUngated`, U11) not only the cheap unlocked peek
  (U2); U10/U11 add that locked-path revalidation plus a test that distinguishes it
  from the peek. Guard 2 should hold the shipment item-log lock across the presence
  check and the stamp (or explicitly accept the narrow residual and rely on the
  doctor audit).
* **Out-of-band edits remain out of scope** — direct Markdown/git edits can still
  produce residue; these are left to the report-only doctor audit by design.

## Constitution Check

* **Safety-First Go** (MUST) — pass: guards are Go, errors wrap with `%w`, sentinel
  errors follow the `internal/errors` pattern, no `unsafe`.
* **Test-First Development** (NON-NEGOTIABLE) — pass: every guard lands a failing
  harness before its implementation (U1 before U2, U3 before U4, U5 before U6); U7
  adds parity coverage.
* **Workspace Isolation and Security Boundaries** — pass: guards operate on
  workspace artifacts only; no traversal, no secrets.
* **CLI Workspace Containment** (NON-NEGOTIABLE) — pass: implementation confined to
  the repo tree; this Stage planning session mutates only planning/backlog
  artifacts inside the worktree.
* **Structured Observability** — pass: guards return typed errors; the governed
  path emits durable events; refusals are legible.
* **Single Responsibility** — pass: no new dependencies; reuse `shippedEventPresence`.
* **Destructive Command Approval** (NON-NEGOTIABLE) — N/A: no destructive terminal
  commands; the guards prevent destructive audit-log residue.
* **Explicit Safety Modes for Elevated Risk** (MUST) — pass: this hardened plan
  adopts a careful, freeze-scope posture — enforcement confined to the named core
  seams, the high-risk archive mutation (PA2) gated on operator awareness, and
  degraded operator visibility recorded (see Plan Hardening).
* **Merge Commit History Preservation** (NON-NEGOTIABLE) — pass: ships via a merge
  commit; Ship enforces P-009 at merge.
* **Git-Friendly Persistence** — pass: backlog and docs artifacts are Markdown +
  YAML frontmatter.
* **Agent Context Efficiency** — pass: reuse of the existing predicate and seams
  avoids new scanning surfaces.

Constitution Check: pass

## Plan Hardening Signals

* Public API, schema, or contract change — **present**: the behavior contract of
  generic `move_item` / `update_item` / `create_item` / `harvest_stash` /
  `archive_item` for shipment artifacts changes across both MCP and CLI (a broad
  shared surface).
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  audit-log integrity and governed-envelope enforcement are compliance-sensitive.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: no migration or backfill; existing residue is left to the report-only
  doctor audit.
* External integration, operator checkpoint, or external dependency — **absent**:
  no external systems.
* High runtime, rollout, or rollback risk — **present**: the guards change core
  mutation-path behavior used by every surface, a false positive could block
  legitimate closures, and self-hosted binary skew delays operative protection.

Requires plan hardening: yes

## Runtime Verification and Closure

Runtime surfaces changed: CLI (`backlogit move`, `backlogit update`, `backlogit add`,
`backlogit stash harvest`, `backlogit archive`) and MCP (`move_item`, `update_item`,
`create_item`, `harvest_stash`, `archive_item`) behavior for shipment artifacts. No
HTTP API or browser UI.

Runtime verification (post-build, by Ship):

* Attempt a generic `move`/`update` of a shipment to `shipped` with the formal gate
  OFF and confirm refusal on both CLI and MCP.
* Attempt the exported `MoveShipmentStatus(..., shipped)` ungoverned path and confirm
  refusal, while `ShipShipment` (topLevel=false) still succeeds.
* Attempt to create a shipment with initial `status: shipped` via `create_item` /
  `harvest_stash` and confirm refusal.
* Attempt to `archive` a shipment sitting at `shipped` with no durable event and
  confirm refusal on both surfaces.
* Confirm `ShipShipment` still ships a shipment end to end and appends the durable
  event.
* Confirm `done` / `abandoned` / P-015 safe-close archival is unaffected.
* Run `backlogit doctor` shipped-event completeness and confirm no NEW residue is
  producible.
* Verify the rebuilt binary's embedded commit covers this change (merged is not
  operative).

Operational closure (release-observability seed):

* Monitoring: the doctor shipped-event completeness audit is the healthy-signal
  check; after merge, confirm zero NEW archived shipments with `archived_status:
  shipped` lacking a durable event.
* Baseline: no new residue producible via generic or archive paths.
* Alert threshold: any NEW post-merge archived shipment flagged by the doctor audit.
* Rollback trigger: a legitimate `done` / `abandoned` / P-015 closure blocked by
  the guard; rollback procedure is to revert the guard commit(s).
* Owner and validation window: Ship, during post-merge closure.

## Plan Hardening

Hardening required: yes. Three signals are present — a behavior-contract change
across the shared MCP and CLI mutation surface, compliance-sensitive audit-log
integrity, and high runtime/rollback risk compounded by self-hosted binary skew.
Operator visibility is degraded this session (agent-intercom tool surface is not
exposed), so the high-risk archive mutation (PA2) is upgraded to a REQUIRED
operator gate at Ship implementation time rather than interactive Stage approval.

### Protected invariants

1. The governed `core.ShipShipment` envelope remains the ONLY sanctioned producer
   of a shipment `shipped` status and `archived_status: shipped`, always paired
   with a durable `shipment_status_changed` / `status: shipped` event. Every other
   producer — generic update/move, exported `MoveShipmentStatus` (`topLevel=true`),
   and create-time status — is refused.
2. Legitimate `done`, `abandoned`, and P-015 single-artifact safe-close archival
   MUST NOT be blocked — guard 2 is scoped strictly to shipment artifacts with
   pre-archive `oldStatus == shipped`; non-shipment artifacts are never blocked.
3. Prevention and detection scan the SAME literal event contract via the SAME
   predicate (`shippedEventPresence`), called directly in `package core` (no
   relocation; `doctor.go` unchanged).
4. Any durable append honors the commit-then-surface two-class contract; an
   `ErrWriteIndeterminate` outcome is never rolled back or blind-retried.
5. MCP and CLI enforce identically; no MCP force lever is introduced (forcing
   stays CLI-only, and no new force flag is added for these guards).
6. `archived_status` and provenance are read from Markdown (not the DB index),
   using one authoritative load for both the guard check and the mutation.
7. Archive invertibility is preserved: stamping must keep `archived_from` intact so
   `UnarchiveItem` still works.
8. No caller-origin or force flag is threaded through `updates`/`opts` to exempt a
   caller; each seam refusal is unconditional and unforgeable, and each carries a
   dedicated sentinel with a stable MCP `error_type` + CLI exit-code mapping.

### Learnings and instructions consulted

* `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md` —
  ArchiveItem bypasses the transition hook; fail-closed on missing/unrecognized
  provenance behind a recognized-status allowlist.
* `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` —
  `archived_status` omitted from `selectCols`; single Markdown load for check + mutate.
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`
  and `2026-07-29-durable-writes-test-seam-patterns.md` — indeterminate-write handling
  and serial function-var test seams.
* `docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md`
  and `2026-07-23-machine-readable-governance-field-contract.md` — parity through the
  registry against durable artifacts; exact producer/consumer field contract.
* `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`
  — do not block the `done`-archive safe-close path.
* `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`
  — merged is not operative until the pinned binary is rebuilt.
* Instruction files: `role-enforcement`, `strict-safety`, `concurrency`,
  `release-observability`.

### Risky actions (ProposedAction / ActionRisk / ActionResult)

These describe the implementation actions Ship will execute; Stage classifies them
here so downstream review and closure inherit the safety framing.

* **PA1 — Refuse ungoverned shipment to `shipped` on the transition and move seams**
  * targets: `internal/core/gate_transition.go` (`UpdateArtifactWithGate`, unconditional),
    `internal/core/shipment.go` (`moveShipmentStatusWithHeadGuard` when `topLevel=true`),
    `internal/errors` (new `ErrShipmentShippedRequiresEnvelope`; surface mapping in U9);
    contract on MCP `move_item`/`update_item` and CLI `move`/`update`
  * change_kind: config-independent contract change in the core mutation path; no
    caller-origin flag (unforgeable)
  * rollback: revert the guard commit; behavior returns to the gate-dependent check
  * ActionRisk: moderate — non-destructive but changes a shared mutation contract
  * approval_required: no (governed behavior guard); ActionResult: planned
* **PA2 — Refuse `archived_status: shipped` stamping without a durable event in `ArchiveItem`**
  * targets: `internal/core/archive.go`, provenance stamping, `UnarchiveItem` invertibility
  * change_kind: local edit on the archive seam with provenance sensitivity
  * rollback: revert the guard commit; archive stamping returns to unconditional copy
  * ActionRisk: high — touches archive provenance/invertibility and a false positive
    could block legitimate closures
  * approval_required: yes — REQUIRED operator gate at Ship review before merge
    (degraded intercom visibility this session); ActionResult: planned
* **PA3 — Refuse create-time `status: shipped` for shipments in `CreateArtifact`**
  * targets: `internal/core/artifacts.go` (mirror the existing `StatusArchived`
    refusal), reusing `ErrShipmentShippedRequiresEnvelope`
  * change_kind: local guard on the create path; MCP `create_item` / `harvest_stash`
    and CLI `stash harvest`
  * rollback: revert the guard commit; create accepts any initial status again
  * ActionRisk: moderate — non-destructive contract change; ActionResult: planned
  * note: `shippedEventPresence` needs NO relocation — all callers are in
    `package core`, so it is a direct intra-package call and `doctor.go` is untouched
* **PA4 — Do NOT migrate or backfill existing residue**
  * change_kind: deliberate non-action; existing residue stays with the report-only doctor audit
  * ActionRisk: low; ActionResult: planned (explicitly out of scope)

### Deepened runtime verification

* Environment prechecks: rebuild the `backlogit` binary from the merged code and
  confirm `backlogit version` embeds the change commit before any dogfood check
  (merged is not operative).
* Target scenarios (run on BOTH MCP and CLI): generic move/update to `shipped` with
  formal gate OFF (expect refusal) and ON (expect refusal or `ErrFormalGateRequired`);
  archive of shipped-without-event (expect refusal); archive of shipped-with-event
  (expect success); `ShipShipment` end to end (expect success + durable event);
  `done`/`abandoned`/P-015 archival (expect success).
* Blocked-path handling: if a guard blocks a legitimate closure during dogfooding,
  treat it as the rollback trigger below rather than forcing past it.

### Deepened operational closure

* Monitoring signal: `backlogit doctor` shipped-event completeness audit — healthy
  when zero NEW post-merge archived shipments carry `archived_status: shipped`
  without a durable event.
* Rollback trigger: any legitimate `done`/`abandoned`/P-015 closure blocked by a
  guard, or any indeterminate-write regression.
* Rollback procedure: revert the guard commit(s) (PA1/PA2/PA3); the report-only
  doctor audit continues to catch residue in the interim.
* Owner: Ship. Validation window: post-merge closure, on the rebuilt binary.

### Unresolved operator decisions (non-blocking)

* Final sentinel-error names and their exact home in `internal/errors` (the plan
  proposes `ErrShipmentShippedRequiresEnvelope` and `ErrArchiveShippedRequiresEvent`).
* Whether guard 2 holds the item-log lock across the presence check and the stamp,
  or accepts the narrow check-to-stamp residual and relies on the doctor audit.

Resolved during hardening: `isValidShipmentTransition` MUST NOT carry a blanket
`shipped` refusal (it is on the live `ShipShipment` path); `shippedEventPresence`
needs no relocation (same `package core`).

These are implementation-level choices for Ship's go-engineer and do not block
plan review.

## Plan Review

<!-- plan-review-attempt: 1 -->

dispatch_mode: multi-agent-dispatch
decision: FAIL

Reviewed revision: initial plan (pre-revision). The gate probed sub-agent dispatch
(available) and dispatched all four always-on personas (Constitution, Go, Scope
Boundary Auditor, Learnings Researcher) plus Architecture Strategist, Agent-Native
Parity Reviewer, and Security Lens Reviewer across diverse model tiers. Operator
visibility was degraded (agent-intercom not exposed); the outcome is recorded here
rather than broadcast.

### Findings (initial pass)

P0: none.

P1 (blocking — all addressed in the in-place revision above):

* [Architecture Strategist, Security Lens — consensus] Guard 1 incomplete: the
  exported `MoveShipmentStatus(..., shipped)` (`topLevel=true`) path is another
  non-ShipShipment producer. Fix: U1/U2 refuse it at
  `moveShipmentStatusWithHeadGuard(topLevel=true)`; `ShipShipment` (topLevel=false)
  stays allowed.
* [Agent-Native Parity, Security Lens — consensus] Create-path bypass: `CreateArtifact`
  / `create_item` / `harvest_stash` can birth a shipment at `status: shipped`. Fix:
  new U3/U4 refuse create-time `shipped`; U7 covers those surfaces.
* [Go Reviewer] Unsafe defense-in-depth: a `shipped` refusal in
  `isValidShipmentTransition` would break the governed `ShipShipment` path that
  depends on it, and the "refuse unless originates from ShipShipment" wording
  implied a forgeable origin flag. Fix: removed the suggestion; guard-1 refusal at
  `UpdateArtifactWithGate` is unconditional with no origin flag (invariant 8).

P2/P3 (addressed):

* [Go] Dropped the dead `shippedEventPresence` extraction (same `package core`);
  keyed guard 2 on the already-loaded `oldStatus == ShipmentShipped`; added the
  sentinel-to-surface mapping (MCP `error_type` + CLI exit code) so U7 asserts an
  identical contract.
* [Architecture, Security] Guard-2 preflight runs under the parent lock before
  cascade/hooks; item-log lock across check-to-stamp or accept the narrow residual.
  Added to Decisions and Risks.
* [Constitution] Added Principle VIII (Safety Modes), corrected the Principle I
  label, and upgraded PA2 to a REQUIRED operator gate under degraded visibility.
* [Learnings] Confirmed compound-library alignment; noted the already-archived-item
  generic-update provenance-drop caveat and a relative-workspace-root scenario in U7.
* [Scope Boundary] No scope creep — the added producers fall inside the stash's
  "universally reject" intent; unit sizing tightened.

Gate action: FAIL on the initial pass (P1 present). Revisions applied in place;
re-review recorded below.

## Plan Review

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

Reviewed revision: post-revision plan. The attempt-1 pass established full
seven-persona coverage; this re-review re-dispatched the four personas that raised
P1 findings (Go Reviewer, Architecture Strategist, Agent-Native Parity Reviewer,
Security Lens Reviewer) across diverse model tiers to verify closure and catch any
new blocking findings. The two non-blocking always-on personas (Scope Boundary
Auditor, Learnings Researcher) returned no blocking findings on attempt 1 and their
advisory notes were incorporated. Plan hardening was required (P-006) and is
present with `ProposedAction` / `ActionRisk` classification. Operator visibility is
degraded (agent-intercom not exposed); the outcome is recorded here rather than
broadcast.

### Verification results

* Go Reviewer — all prior findings (F1-F7) RESOLVED; no new P0/P1.
* Agent-Native Parity Reviewer — both prior findings RESOLVED; no new P0/P1.
* Security Lens Reviewer — all residual-bypass findings RESOLVED; confirmed no
  remaining in-process non-ShipShipment producer of a shipment `shipped` /
  `archived_status: shipped`; fail-closed posture and `shipped`-only scope correct.
* Architecture Strategist — four prior findings RESOLVED, plus two new refinements
  that were addressed in place:
  * guard 2 must also require `artifact_type == shipment` so non-shipment artifacts
    at `shipped` are not made unarchivable — applied to U5, U6, Decisions,
    invariant 2, and the Requirements Trace.
  * the sentinel surface mapping belongs in the `internal/mcp` / `internal/cli`
    adapters, keeping `internal/errors` a leaf — extracted into the new unit U9,
    on which U7 now depends.

### Findings summary

* P0: none.
* P1: none remaining. Every attempt-1 P1 (the `MoveShipmentStatus` producer, the
  create-path bypass, and the unsafe `isValidShipmentTransition` defense-in-depth)
  and both attempt-2 refinements are closed in the revised plan.
* P2/P3: advisory precision notes (guard-2 check-to-stamp lock choice; exact adapter
  file names) are captured as unresolved implementation decisions for Ship's
  go-engineer; none block harvest.

Whether plan hardening was required: yes; satisfied. Runtime verification and
operational closure are present for every changed surface.

Gate action: PASS. Proceed to harvest.

### PR #369 Copilot current-HEAD review-fix cycle (2026-08-19)

Copilot's current-HEAD review of `admin/stage-47b48db0` @ `442a5a47` raised five
threads against the plan and the harvested backlog. All five were validated against
source and addressed in place; the PASS gate action above is unchanged — the fixes
tighten coherence and add two TDD-ordered units without altering the enforcement
design.

* **U2 / 144.002-T locked-path revalidation (peek-to-write TOCTOU)** — confirmed:
  the shipment refusal in `UpdateArtifactWithGate` runs on the unlocked peek
  (`gate_transition.go`), while the authoritative write locks and reloads in
  `updateArtifactUngated` (`artifacts.go`). Folding the locked check into U2 would
  push it to four production files. Fix: added U10 (RED harness, `144.010-T`) and
  U11 (implementation in `updateArtifactUngated`, `144.011-T`), each single-domain
  and under three files; U2 / 144.002-T reworded to the peek fast-path plus
  move-seam guard that points at U10/U11 for the check-to-write guarantee; U7 now
  depends on U11.
* **U3 / 144.003-T core-vs-surface mismatch** — confirmed: a `core` unit test
  cannot drive MCP `create_item` / `harvest_stash`, and `internal/core/artifacts_test.go`
  does not exist. Fix: narrowed U3 / 144.003-T to a core-only `CreateArtifact` RED
  harness in a new file `internal/core/144_create_guard_test.go` (mirroring the
  existing `066_create_guard_test.go`); surface-level create coverage stays with
  U7 / 144.007-T (parity).
* **Runtime-surface inventory omission** — confirmed: the "Runtime surfaces changed"
  summary omitted the create producers. Fix: added CLI `backlogit add` /
  `backlogit stash harvest` and MCP `create_item` / `harvest_stash`.

Post-fix backlog: 11 units (144.001-T through 144.011-T), all parented under 144-F
and in shipment 128-S; dependencies TDD-ordered; docs lint clean; `doctor` reports
no new orphans for the 144.x hierarchy.
