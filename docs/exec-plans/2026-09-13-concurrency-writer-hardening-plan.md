---
chunk_strategy: h1-h2-h3
description: "Implementation plan for concurrency-safety hardening of backlogit artifact-mutation writers: canonical lock order (FE440C62) and shared stale-write CAS guard generalization (1E0C2251) as two independent same-surface units (no ordering between them)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-concurrency-writer-hardening-plan.md
title: "Implementation Plan: Concurrency-safety hardening for artifact-mutation writers"
docline:
    stash_id: FE440C62,1E0C2251
    status: approved
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Eliminate two **independent** concurrency defects in `internal/core`
artifact-mutation writers without changing the governed reconcile-to-shipped
contract:

1. **FE440C62** — remove the `ArchiveItem` B→C lock-order inversion so all
   same-item artifact writers acquire locks in one canonical order.
2. **1E0C2251** — extend the `guardArchivedStatusUnchangedSince` **archived_status
   stale-write CAS guard** to the remaining snapshot-before-lock writers
   (`RemoveArtifactLink`, `BulkUpdateStatus`).

**Scope of the CAS guard (narrowed explicitly, everywhere):** the guard protects
exactly the `archived_status` field against a stale overwrite of a
freshly-reconciled `archived_status: shipped` value. It is **not** a general
content-staleness / lost-update guard: it does not compare a revision counter or
canonical-content digest and does not reload-and-merge other fields under lock.
The authorized 1E0C2251 defect is specifically the `archived_status:shipped`
clobber, so this shipment deliberately narrows the claim to `archived_status`.
General content-staleness protection (a revision / canonical-content CAS or a
reload-and-merge-under-lock across all fields) is explicitly **out of scope** and
recorded as a follow-up.

**Dependency note (true-dependency only):** FE440C62 (lock order) and 1E0C2251
(archived_status CAS) touch the **same files/functions**, so they ship in one
covering feature / one PR to avoid conflicting edits — but they are **not** in a
correctness dependency. The `archived_status` CAS guard's soundness depends on
the snapshot-then-compare of the `archived_status` field, NOT on the item-log
(lock C) acquisition order; canonicalizing the lock order is neither a
precondition nor a correctness input for the guard. The earlier "CAS must land on
a canonical lock order" rationale was proximity/same-surface reasoning, not a real
contract edge, and the artificial Unit 2 → Unit 1 dependency is **removed**.

Source: `docs/decisions/2026-09-13-concurrency-writer-hardening-deliberation.md`.

## Requires plan hardening: yes

Concurrency, lock-ordering, and shared-mutation-primitive changes are inherently
high-risk (deadlock / stale-write / availability). See `## Plan Hardening`.

## Implementation Units

### Unit 1 — Canonical lock order (FE440C62)

* **U1.1** Characterize + document the canonical lock-acquisition order across
  **all** artifact-mutation writers — `ArchiveItem`, `AssociateCommit`,
  `RemoveArtifactLink`, `BulkUpdateStatus`, the reconcile transaction, **and the
  batch/multi-item paths**: `ReconcileArchivedLifecycle`
  (`internal/core/archive_reconcile.go:119`, which holds `lockArtifactMutations`
  (B) across the whole batch and then calls `ArchiveItem`/`UnarchiveItem` on the
  same item ids inside `reconcileArchivedItem`) and `ArchiveItem`'s recursive
  cascade (`archiveDescendants`). Produce an analysis note that (a) enumerates
  each writer's current lock order, (b) fixes the canonical order as **C-then-B**
  (matching `AssociateCommit` and the governed reconcile transaction — the
  majority and governed path), (c) states the **load-bearing ordering
  invariants** that keep the batch and cascade paths safe: `lockArtifactMutations`
  is context-reentrant (so an inner `ArchiveItem` under an outer held B does not
  self-deadlock) and lock acquisition is ancestor-before-descendant / sorted-batch
  so the anti-inversion protection holds; (d) records the residual-and-bound
  for any path that cannot be made strictly C-then-B; and (e) states the
  **causal-event-ordering invariant** the item-log (C) must preserve when the
  append is moved (U1.2) — the item-log event for an archive MUST remain causally
  ordered w.r.t. the artifact-mutation it records (no event reordering or
  event-before-mutation / mutation-before-event visibility skew introduced by
  relocating the C append). Domain: docs/analysis.
* **U1.2** Reorder `ArchiveItem` (`internal/core/archive.go`) so it is never
  holding B while acquiring C. **Preferred strategy**: move the best-effort
  item-log (C) JSONL append out of the B-held region so the two locks are never
  held overlapping in B→C order — this eliminates the inversion **without**
  widening the C critical section and **preserves** the existing *non-fatal*
  append-failure semantics (warn + skip; archive still succeeds) **and the causal
  ordering of the item-log event relative to the archive mutation** (per the U1.1
  invariant; the relocated append must still record the mutation in causal order,
  not race ahead of or behind it). If instead C is acquired before B, the note
  (U1.1) must record the widened C critical section and its bounded-contention
  trade-off. Specify the chosen C scope, failure handling, and event-ordering
  preservation explicitly. Domain: code. Depends on U1.1, **U1.0** (RED).
* **U1.3** Resolve the batch-path residual: ensure `ReconcileArchivedLifecycle`'s
  outer-B-then-inner-`ArchiveItem`-C shape does not reintroduce a same-item B→C
  inversion after U1.2. **The restructure — acquire C before the batch B (or
  otherwise avoid holding B while acquiring C in the batch path) — is REQUIRED
  unless U1.1's analysis proves no concurrent same-item C-then-B writer can exist
  for the reconcile-lifecycle path.** Context-reentrancy of `lockArtifactMutations`
  prevents self-deadlock on inner B re-acquisition but does NOT cure the cross-lock
  AB-BA ordering against C-then-B writers, so the reentrancy invariant alone is
  insufficient justification to skip the restructure. Domain: code. Depends on
  U1.2.
* **U1.seam-red (source-shape AST RED harness — predecessor)** Write a failing
  source-shape assertion (AST / type-level harness) that asserts the
  artifact-mutation writers expose controllable barrier hook points at **both**
  the B (`lockArtifactMutations`) **and** the C (item-log append) acquisition
  points (per U1.1). Observe it RED: only a pre-B hook
  (`persistArtifactPreLockHook`) exists today and **no C-acquisition seam exists**,
  so the "both B and C hook present" assertion fails before the production seam
  lands. Domain: tests. Depends on U1.1. ≤2 scenarios. (task 172.015-T)
* **U1.seam (no-op production seam declaration — makes the AST harness GREEN)**
  Add the deterministic B/C lock-barrier instrumentation seam to **production**:
  expose injectable, controllable barrier hook points at the B
  (`lockArtifactMutations`) and C (item-log append) acquisition points (per U1.1)
  that let a test force the AB-BA interleaving deterministically. The existing
  pre-B `persistArtifactPreLockHook` is insufficient because **no C seam exists**;
  add the missing controllable C hook (and the paired B hook). Pin where each hook
  injects, that it is enabled ONLY under test (off/no-op on production paths so no
  production behavior changes), and how it preserves the causal item-log
  event-ordering invariant. The hooks are **no-op** by default; this task adds the
  seam ONLY (no lock-order change). It turns U1.seam-red GREEN. The barrier-driven
  RED harness (U1.0) depends on this seam and the implementation (U1.2/U1.3)
  follows the observed-RED evidence. Domain: code. Depends on U1.1, U1.seam-red.
  (task 172.013-T)
* **U1.0 (RED harness — predecessor)** Write the failing lock-order regression
  BEFORE U1.2/U1.3 and observe it RED using a **deterministic lock-barrier /
  instrumentation harness** (inject a controllable barrier at the B/C acquisition
  points so the AB-BA interleaving is forced deterministically), NOT a
  probabilistic `go test -race` repeat. The harness reproduces the same-item B→C
  vs C→B inversion (bounded contention) deterministically and asserts the causal
  item-log event ordering. Domain: tests. Depends on U1.1, U1.seam. ≤3 scenarios.
  * AC: the barrier-driven test deterministically exhibits the pre-fix inversion
    (RED) and the asserted causal-ordering violation, without relying on `-race`
    timing luck.
* **U1.4** Lock-order GREEN regression: with U1.2/U1.3 applied, the U1.0
  deterministic barrier harness now shows no inversion and bounded, deterministic
  contention resolution; item-log causal ordering holds; run additionally under
  `-race` as a secondary (non-primary) net. `-race` alone is NOT treated as proof
  of no ABBA inversion — the deterministic barrier is the primary control. Domain:
  tests. Depends on U1.2, U1.3, U1.0.

### Unit 2 — Generalized archived_status stale-write CAS guard (1E0C2251) — INDEPENDENT of Unit 1

Unit 2 shares the code surface with Unit 1 (one PR) but carries **no correctness
dependency** on the lock-order canonicalization. The tasks below do NOT depend on
U1.2/U1.3.

* **U2.decl-red (source-shape AST RED harness — predecessor)** Write a failing
  source-shape assertion (AST / type-level harness) that asserts `BulkUpdateResult`
  exposes the ADDITIVE typed conflict-detail surface ALONGSIDE the preserved
  `Failed []string`: a NEW `FailedDetails []BulkUpdateConflict` field whose element
  type exposes `{ID, Err, FromStatus, ToStatus}`, AND that `Failed []string` is
  unchanged. Observe it RED: the field/type do not yet exist, so the harness fails
  before the production field declaration lands. Domain: tests. No deps
  (predecessor). ≤2 scenarios. (task 172.014-T)
* **U2.decl (production field declaration — makes the AST harness GREEN)** Add the
  ADDITIVE typed conflict-detail surface to `BulkUpdateResult` in **production**: a
  NEW `FailedDetails []BulkUpdateConflict{ID, Err, FromStatus, ToStatus}` field and
  type, ADDED ALONGSIDE the existing `Failed []string`, which is PRESERVED
  unchanged (no signature change / removal / retype) for backward compatibility.
  This task declares the field/type ONLY — it does NOT populate `FailedDetails` and
  does NOT change `BulkUpdateStatus` behavior (that is U2.2). It turns U2.decl-red
  GREEN. Pin the field/struct names and the stale-write→typed-conflict mapping as
  the source-of-truth contract for U2.caller, U2.2, and U2.3b. Domain: code.
  Depends on U2.decl-red. (task 172.011-T)
* **U2.caller (caller compatibility/adaptation — predecessor)** Enumerate every
  `BulkUpdateStatus` caller / `BulkUpdateResult` consumer; confirm each stays
  source- and behavior-compatible with the PRESERVED `Failed []string`, and adapt
  the callers that should consume the new additive typed detail to read it WITHOUT
  changing the `Failed []string` contract. Lands BEFORE the behavior impl (U2.2)
  and its GREEN test (U2.3b) so the additive surface has adapted consumers, not an
  orphaned field. Domain: code. Depends on U2.decl. (task 172.012-T)
* **U2.0 (RED harness — predecessor)** Write the failing `archived_status`
  stale-write tests BEFORE U2.1/U2.2 and observe them RED, using a deterministic
  barrier that forces the snapshot-then-stale-overwrite interleaving: (1) a
  concurrent `RemoveArtifactLink` clobbers a freshly-reconciled
  `archived_status: shipped`; (2) a concurrent `BulkUpdateStatus` clobbers it and
  a per-item conflict is silently lost. Domain: tests. No deps (predecessor).
  ≤2 scenarios.
  * AC: both tests deterministically reproduce the pre-fix `archived_status`
    clobber (RED).
* **U2.1** Apply `guardArchivedStatusUnchangedSince` to `RemoveArtifactLink`
  (`internal/core/artifacts.go:981`). NOTE: `RemoveArtifactLink` persists via
  `persistArtifactWithoutDBOnlyLinks` (which strips DB-only link edges from the
  Markdown write), **not** `persistArtifactWithGuard`. Introduce a combined
  **guarded + without-DB-only-links** persist variant (or thread the guard param
  through `persistArtifactWithoutDBOnlyLinks`) so both the `archived_status`
  stale-write guard AND the DB-only-link stripping are preserved. Snapshot
  `preLockArchivedStatus` before the `findArtifact`→persist window. **Source-of-truth
  read (prior art `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`):**
  the `preLockArchivedStatus` snapshot AND the in-lock comparand MUST load
  `archived_status` from the Markdown source of truth via `findArtifact`, NOT the
  SQLite fast-path projection (`loadArtifact`/`selectCols`), which omits
  `archived_status` and returns a zero value — a snapshot loaded from the DB
  projection would compare zero==zero and silently admit the exact stale clobber
  this guard exists to prevent. Ensure the
  early "link not present / nothing removed" branch (which calls `db.RemoveLink`
  directly with no artifact persist) is **not** routed through the guard. Domain:
  code. Depends on U2.0 (RED). (Scope check: touches `artifacts.go` + the shared
  persist helper — still ≤ 3 files; the combined variant is a small helper
  addition.)
  * AC: the `archived_status` snapshot and comparand are read from the Markdown
    source of truth (`findArtifact`), never the DB fast-path projection; a test
    asserts the guard is sound when the DB projection would have returned a
    zero-valued `archived_status`.
* **U2.2** Apply the guard to `BulkUpdateStatus` (`internal/core/queue.go:404`)
  via `persistArtifactWithGuard` with a per-item pre-lock `preLockArchivedStatus`
  snapshot **read from the Markdown source of truth (`findArtifact`), not the DB
  fast-path projection** (same source-of-truth requirement as U2.1). Observable
  behavior change to assert (**BACKWARD-COMPATIBLE + ADDITIVE**): the existing
  `BulkUpdateResult.Failed []string` field is **PRESERVED** and continues to be
  populated for every failed item (no signature change / removal / retype) so
  existing callers keep compiling and behaving unchanged; a per-item stale-write
  (`ErrShipmentConflict`) is **ADDITIONALLY** recorded in a NEW additive typed
  detail surface (e.g. `FailedDetails []BulkUpdateConflict{ID, Err, FromStatus,
  ToStatus}`) carrying the item id and the conflicting status transition — added
  **alongside** `Failed []string`, NOT as a replacement — so new callers can
  distinguish a stale-write conflict from other per-item failures, rather than
  aborting or silently clobbering the batch. Depends on U2.decl (source-shape
  declaration) and U2.caller (caller adaptation), both landing first. Domain:
  code. Depends on U2.0 (RED), U2.decl, U2.caller.
* **U2.3a (GREEN — RemoveArtifactLink)** Stale-write GREEN regression for
  `RemoveArtifactLink`: concurrent `RemoveArtifactLink` vs reconcile-to-shipped
  cannot clobber `archived_status: shipped`; `ErrNotFound` is not treated as
  success; the DB-only-link stripping is still honored under the guarded path.
  Domain: tests. Depends on U2.1, U2.0. ≤3 scenarios.
* **U2.3b (GREEN — BulkUpdateStatus)** Stale-write GREEN regression for
  `BulkUpdateStatus`: concurrent `BulkUpdateStatus` vs reconcile-to-shipped cannot
  clobber `archived_status: shipped`; the backward-compatible
  `BulkUpdateResult.Failed []string` remains populated for the clobbered item AND
  a per-item stale-write is ADDITIONALLY recorded in the new additive typed detail
  surface (id + status transition) — additive, NOT a replacement for
  `Failed []string`; other per-item results are unaffected. Domain: tests. Depends
  on U2.2, U2.0. ≤3 scenarios.

## Constitution Check

* **Test-first ordering (NON-NEGOTIABLE)**: RED harnesses precede the code they
  cover, enforced by dependency edges. **Source-shape AST RED harnesses precede
  the production seam/field declarations they cover** — U1.seam-red (172.015-T)
  before the no-op production seam U1.seam (172.013-T); U2.decl-red (172.014-T)
  before the production field declaration U2.decl (172.011-T). **Behavior RED
  harnesses precede the behavior impl** — U1.0 (deterministic lock-barrier) before
  U1.2/U1.3; U2.0 before U2.1/U2.2. The production seam/field declarations and the
  U2.caller adaptation land before their dependent behavior-RED/impl units. U1.4
  and U2.3a/U2.3b are the GREEN regressions. Pass.
* **True dependency only**: Unit 2 (archived_status CAS) carries NO correctness
  dependency on Unit 1 (lock order); the artificial Unit 2 → Unit 1 edge is
  removed. They share a code surface (one PR) only. Pass.
* **Single-domain tasks**: each unit is code OR docs OR tests. Pass.
* **2-hour rule**: each unit < 3 files / < 5 functions / ≤ 3 test scenarios.
  Pass.
* **Workspace containment (P-017)**: all edits in `internal/core`; no autoharness
  change. Pass.
* **No governed-contract regression**: reconcile-to-shipped observable behavior
  unchanged; only lock order + `archived_status` guard coverage change. Pass.
* **Fail-closed**: CAS guard rejects stale `archived_status` writes; ErrNotFound
  must not be swallowed as success. Pass.

Constitution Check: pass

## Plan Hardening

* **ProposedAction**: reorder `ArchiveItem` locks C→B. **ActionRisk**: high
  (lock-order change on a system-wide primitive). Mitigation: U1.1 canonical-
  order analysis precedes the change; the **deterministic lock-barrier harness
  (U1.0/U1.4) is the primary control** proving no new inversion and bounded
  contention; `go test -race` is a secondary net only and is NOT treated as proof
  of no ABBA inversion (AB-BA deadlocks are timing-flaky). Causal item-log event
  ordering is asserted preserved when the C append is relocated.
* **ProposedAction**: generalize the `archived_status` CAS guard to two more
  writers. **ActionRisk**: medium (could surface latent stale-write callers as new
  CAS rejections). Mitigation: guard only rejects genuinely-stale `archived_status`
  transitions (NOT general content staleness — that is out of scope); U2.3a/U2.3b
  cover the ErrNotFound-as-success failure mode; `BulkUpdateStatus` preserves typed
  per-item conflict information.
* **Rollback**: Unit 1 and Unit 2 are **independent** and each independently
  revertible; reverting one does not require reverting the other (no cross-unit
  dependency). Reverting either restores the pre-change (reviewed-PASS) behavior
  of that unit only; no governed reconcile-to-shipped contract change is involved.
* **Blast-radius bound**: no change to public API signatures —
  `BulkUpdateResult.Failed []string` is **preserved unchanged** (backward-
  compatible); the only surface addition is a NEW **additive** typed
  `BulkUpdateResult` per-item conflict-detail field **alongside** `Failed []string`
  (not a replacement); no new lock primitives introduced.
* **Residual risk**: (1) the fully-automatic shared-persist-path enforcement
  (deliberation Option C) is intentionally deferred (see Follow-ups); any *future*
  new snapshot-before-lock writer must opt in — U1.1's note documents this. (2)
  General content-staleness protection (revision / canonical-content CAS or
  reload-and-merge) is out of scope; only `archived_status` is guarded.

## Verification

* `go test ./internal/core/...` with the **deterministic barrier** U1.0/U1.4
  lock-order tests and the U2.0/U2.3a/U2.3b stale-write tests; `-race` as a
  secondary net.
* Existing `TestU20_*` concurrency suite remains green.

## Follow-ups (out of this shipment)

* Option C: make the CAS guard automatic for **every** snapshot-before-lock
  writer via shared persist-path enforcement (larger architectural pass).
  **Audit-every-writer obligation (prior art
  `docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`):**
  deferring Option C leaves this exact class open — any *future* snapshot-before-lock
  writer added after this shipment reaches the same `archived_status` transition
  unguarded. Before this shipment closes, enumerate every current writer that can
  clobber a reconciled `archived_status: shipped` (`RemoveArtifactLink`,
  `BulkUpdateStatus`, and any other snapshot-before-lock persist path) and confirm
  each is guarded or explicitly excluded; record the standing obligation that new
  such writers must adopt the guard.
* Related deferred entry A0C733C6 (`findArtifact` TOCTOU) is a **separate**
  shared-infrastructure hardening and is NOT in this shipment's scope.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 1. Personas dispatched: Concurrency Reviewer (specialist), Correctness
Reviewer (cross-cut). Coverage complete.

**Gate rationale**: one P1 finding → FAIL per rubric (any P0/P1).

Findings:
* **P1 (Concurrency)** — `ReconcileArchivedLifecycle`
  (`archive_reconcile.go:119`) holds batch lock B then calls `ArchiveItem` on the
  same item ids; omitted from Unit 1, so canonicalizing `ArchiveItem` alone leaves
  a residual same-item B→C inversion (bounded contention, not a hang) on which
  Unit 2's CAS guard is premised.
* **P2 (Concurrency)** — U2.1 mis-stated the persist path: `RemoveArtifactLink`
  uses `persistArtifactWithoutDBOnlyLinks`, not `persistArtifactWithGuard`; needs
  a combined guarded + without-DB-only-links variant.
* **P2 (Concurrency)** — U1.2 under-specified the C-lock scope/failure semantics
  (late best-effort C vs early mandatory C widening the critical section).
* **P2 (Concurrency)** — U1.1 should cover cascade/multi-item ordering invariants.
* **P3 (Correctness)** — U2→U1 edge is rollback-coupling, not a build dependency.

Plan hardening required: yes; present. Remediation applied in this revision (see
revised Units 1–2: added U1.1 batch/cascade coverage + invariants, U1.2 C-scope
+ failure-semantics preservation, new U1.3 batch-path residual task, corrected
U2.1 persist path, U2.2 behavior-change assertion). Re-review below.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2. Personas: Concurrency Reviewer (specialist, re-review). Coverage
complete. Prior P1 (batch B→C inversion) confirmed resolved: now covered by U1.1
(analysis + invariants), U1.3 (code remediation, restructure now REQUIRED unless
proven unnecessary), and U1.4 (batch-path -race regression). Prior P2s (persist
path, C-scope/failure semantics, cascade invariants) all resolved.

**Gate rationale**: no P0/P1/P2 remain after the U1.3 tightening; residual items
are P3 implementation-detail advisories. Plan hardening required: yes; present
and adequate.

Findings (residual, non-blocking):
* P3 — AB-BA deadlocks are timing-flaky; U1.4 batch-path -race test is a partial
  (not exhaustive) mitigation. Accepted; the U1.3 restructure is the primary
  control.

<!-- plan-review-attempt: 2 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 3 (review-fix cycle 1). Personas dispatched (7, coverage complete):
Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher,
Architecture Strategist, Security Lens Reviewer, Agent-Native Parity Reviewer.

**Findings and remediation:**
* **P1 (Learnings Researcher)** — the `archived_status` snapshot-before-lock CAS
  ignored prior art
  (`docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`):
  the SQLite fast-path projection (`selectCols`) omits `archived_status`, so a
  snapshot loaded from the DB would compare zero==zero and admit the clobber.
  **REMEDIATED**: U2.1/U2.2 (and tasks 172.005-T/172.006-T) now REQUIRE the
  snapshot AND in-lock comparand to read `archived_status` from the Markdown
  source of truth (`findArtifact`), never the DB fast-path, with an explicit AC.
  The audit-every-writer obligation
  (`docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`)
  was added to Follow-ups (Option C deferral leaves future snapshot-before-lock
  writers unguarded; enumerate + confirm before close).
* **P2 (Constitution, Go, Scope, Architecture — unanimous)** — 172.010-T reintroduced
  out-of-scope "revision/canonical-content CAS or reload-and-merge" vocabulary,
  contradicting the narrowed archived_status-only scope. **REMEDIATED**: 172.010-T
  reworded to the archived_status snapshot-then-compare guard only (matching
  172.006-T and the plan Objective).

**Gate rationale**: after remediation no P0/P1/P2 remain; residual items are P3
implementation-seam advisories (deterministic-barrier injection seam; map
ownership under the claim lock; BulkUpdateResult typed-vs-additive API
classification) recorded for the implementer. Plan hardening required: yes;
present and adequate.

<!-- plan-review-attempt: 3 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Review-fix cycle 2 (staging PR #442). Re-reviewed after reconciling Copilot
comments 3, 4, and 9. Item 3: `BulkUpdateResult.Failed []string` preserved for
backward compatibility with an additive typed `FailedDetails` conflict surface;
predecessor source-shape declaration task 172.011-T (no deps) and caller
compatibility/adaptation task 172.012-T (deps 172.011-T) precede behavior work;
172.006-T deps expanded to {172.009-T, 172.011-T, 172.012-T}; GREEN task
172.010-T assertions are additive. Item 4: seam declaration task 172.013-T (deps
172.001-T) precedes the RED barrier harness; 172.008-T deps include 172.013-T;
plan shows U1.seam before U1.0. Item 9: Unit 1 (lock-order) and Unit 2
(archived_status CAS) documented as independent shared-surface work with no
"first, then" sequencing. Dependency DAG re-verified acyclic (500 edges / 436
nodes). No P0/P1 findings.

<!-- plan-review-attempt: 4 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Review-fix cycle 3 (staging PR #442, hard cap). Personas dispatched (2, coverage
complete): Correctness Reviewer, Scope Boundary Auditor. Re-reviewed after
decomposing the two mislabeled "declaration" tasks into strict test-first
source-shape-AST-RED-harness → production-declaration pairs.

**Threads remediated:**
* **Thread 1 (Unit 2, `BulkUpdateResult` typed detail, 1E0C2251)** — 172.011-T was
  a docs/analysis note that established neither an AST harness nor a production
  `FailedDetails` field/type. Decomposed: **172.014-T** (tests, source-shape AST
  RED harness asserting the additive `FailedDetails []BulkUpdateConflict{ID,Err,
  FromStatus,ToStatus}` alongside preserved `Failed []string`; no deps) →
  **172.011-T** (retyped docs/analysis→**code**, production field/type declaration,
  deps 172.014-T) → 172.012-T (caller adaptation) → 172.006-T (behavior, deps
  172.009-T RED + 172.011-T + 172.012-T) → 172.010-T (GREEN). `Failed []string`
  backward compatibility preserved end to end.
* **Thread 2 (Unit 1, B/C barrier seam, FE440C62)** — 172.013-T was a docs/analysis
  note; a docs declaration is insufficient. Decomposed: **172.015-T** (tests,
  source-shape AST RED harness asserting BOTH B and C barrier hook points; RED
  because the pre-B `persistArtifactPreLockHook` fires before acquisition and no C
  seam exists; deps 172.001-T) → **172.013-T** (retyped docs/analysis→**code**,
  no-op production seam adding controllable B and C acquisition hooks, deps
  172.001-T + 172.015-T) → 172.008-T (barrier-driven behavior RED harness) →
  172.002-T/172.003-T (impl). Test-only enablement, causal item-log ordering
  preserved, no production behavior change in the seam.

Dependency DAG re-verified acyclic (15 nodes / 21 edges over the 172 subgraph;
automated DFS cycle check = ACYCLIC). Shipment 153-S manifest updated to 16 items
(parent-first, dependency-ordered) including 172.014-T and 172.015-T. Docs
authoring lint: valid, 0 violations. Index sync: OK.

**Gate rationale**: no P0/P1 findings after remediation. Both threads' ID-reuse is
accurate (docs/analysis→code retype with no history loss; both tasks were never
executed). Residual findings are non-blocking advisories.

Findings (residual, non-blocking):
* P2 (Correctness) — the `ReconcileArchivedLifecycle` batch-path residual
  (172.003-T) is verified deterministically only through 172.004-T's `-race`
  regression; the deterministic-barrier harness (172.008-T) scopes the same-item
  B→C vs C→B case and does not explicitly exercise the batch path. This is a
  **different contract surface** than the two remediated threads (source-shape
  declaration decomposition) and pre-existing (172.003-T/172.008-T passed prior
  cycles); recorded as an implementer advisory, not expanded into this cycle.
* P3 (Scope) — clarified in 172.013-T that the B/C hooks are centralized at the
  shared `lockArtifactMutations` entry and item-log append helper (≤3 files) and
  why the pre-B hook is insufficient; clarified in 172.011-T that the
  ErrShipmentConflict→typed-entry mapping is documented contract, populated by
  172.006-T (not behavior in the declaration task).

<!-- plan-review-attempt: 5 -->
