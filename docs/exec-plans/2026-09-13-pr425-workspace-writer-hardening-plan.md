---
chunk_strategy: h1-h2-h3
description: "Hardening-addendum implementation plan amending 168-F/169-F/170-F so the PR #425 workspace-writer threat-model findings (3B661FCE, BFF76433, BF18DA1D, 4E210DB4) are genuinely resolved before Ship claims 149-S/150-S/151-S"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-pr425-workspace-writer-hardening-plan.md
title: "Hardening Addendum Plan: 168-F/169-F/170-F workspace-writer resistance"
docline:
    stash_id: 3B661FCE,BFF76433,BF18DA1D,4E210DB4
    status: approved
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Amend the already-PASSED 866FDC8C trust-boundary split (`168-F`/`169-F`/`170-F`)
with hardening tasks that close four PR #425 review threads under the workspace-
writer threat model, and add those tasks to the existing queued shipments
`149-S`/`150-S`/`151-S` so they are resolved **before** Ship.

Source: `docs/decisions/2026-09-13-pr425-workspace-writer-hardening-deliberation.md`.
Origin plan: `docs/exec-plans/2026-09-13-866fdc8c-trust-boundary-split-decided-plan.md`.

## Requires plan hardening: yes

Security-architecture changes to trust-anchor status binding, attestation
persistence, and anti-replay state. See `## Plan Hardening`.

## Implementation Units (new hardening tasks)

### 168-F (shipment 149-S) — trust-anchor external binding [3B661FCE]

* **168.007-T** Make external-pin **status** authoritative over in-workspace
  config: the resolver treats in-workspace `active`/`revoked` status as advisory
  only; an externally-pinned key that is de-pinned (removed from the external pin
  set) resolves as **not trusted** and a workspace-local edit CANNOT flip a
  de-pinned/revoked key back to active. Revocation-via-external-de-pin is
  monotonic. Fail closed if the external pin set is absent. Domain: code.
  Depends on `168.001-T`, `168.003-T`.
  * AC: a workspace-local edit flipping a revoked/de-pinned key to `active` does
    NOT resolve it as trusted; negative test proves reactivation is rejected.
  * Scope note: this closes the **named** 3B661FCE vector (revoked→active
    reactivation). Because `168.001-T`'s external-pin gate is a **fingerprint-
    membership set only** (no per-key role or validity), role and validity-window
    are NOT yet externally authoritative — see `168.008-T`.
* **168.008-T** Extend the external trust-root representation to carry **role**
  and **validity-window** (`not_before`/`not_after`) per key, and bind the
  resolver to them so a workspace writer cannot widen a key's role or un-expire a
  key by editing in-workspace config. If extending the external representation is
  out of 149-S's bounded scope, this task instead records an explicit, operator-
  visible **tracked sub-vector** (role/validity in-workspace-mutable) with its
  bound — NOT a silent omission. Domain: code (or documented residual). Depends
  on `168.007-T`.
  * AC: role-widening and validity-extension via in-workspace edit are rejected
    OR an explicit residual-risk record documents the sub-vector; if the residual
    option is taken, its recorded **bound** MUST include at minimum a fail-closed
    or monotonic mitigation and an explicit Ship-gate acknowledgement — NOT an
    unbounded deferral (consistency with 170.008-T's no-document-only-closure
    stance for the analogous escalation vector).

### 169-F (shipment 150-S) — durable signed material [BFF76433 + BF18DA1D]

* **169.007-T** Persist a durable **signed envelope** in the `169.003-T` event
  binding, replacing unsigned-metadata+digest, enabling independent
  `doctor`/auditor re-verification against the external root; fail closed when
  signed material is absent. If a durable **protected reference** is used instead
  of the inline envelope, the reference MUST be **digest-pinned** so a swapped or
  tampered target fails closed rather than re-verifying a substituted envelope.
  This task **supersedes** (removes/upgrades) the presence-based, forgeable
  `169.003-T` metadata-presence doctor branch — it is NOT an additive parallel
  accept path. Domain: code. Depends on `169.002-T`, `169.003-T`.
  * AC: `doctor` re-verifies a persisted attestation offline against the external
    root; unsigned-metadata-only events are rejected; a swapped/tampered
    protected reference fails closed.
* **169.008-T** Negative + integration tests: forged unsigned metadata rejected;
  persisted signed envelope re-verifies; tampered envelope / swapped reference
  fails closed; the removed metadata-presence branch no longer accepts. Domain:
  tests. Depends on `169.007-T`.

### 170-F (shipment 151-S) — rollback-resistant nonce ledger [4E210DB4]

* **170.008-T** Move nonce single-use consumption state to externally-protected,
  rollback-resistant storage (monotonic external counter / protected reference)
  so ledger deletion/truncation cannot re-enable replay; fail closed if external
  consumption state is unavailable. **Option A is REQUIRED before Ship claims
  151-S.** Document-only closure is NOT an acceptable Ship outcome for this
  authorization-replay / privilege-escalation vector (consistent with Decision 1,
  which rejects document-only closure for the analogous 168 reactivation vector).
  If externally-protected state is genuinely infeasible within 151-S, the choices
  are: (i) **block Ship for 151-S** and re-plan, or (ii) retain a fold ONLY behind
  a hard-reviewed **non-exploitability proof** — request-identity binding + short
  `not_after` + reconcile idempotency that reduces any replay to a no-op/conflict
  — reviewed and approved as genuinely non-exploitable, NOT a documentation
  checkbox. Domain: code. Depends on `170.003-T`.
  * AC: deleting/truncating the in-workspace ledger does not permit replay of a
    still-valid token (Option A); OR (only if A infeasible) a hard-reviewed
    non-exploitability proof is recorded and approved, otherwise 151-S is blocked.

## Constitution Check

* **Single-domain tasks**: 168.007 code, 168.008 code/residual, 169.007 code,
  169.008 tests, 170.008 code. Pass.
* **2-hour rule**: each task bounded to its feature's existing surface. Pass.
* **Fail-closed security posture**: every task fails closed on missing external
  material. Pass.
* **Workspace containment (P-017)**: edits confined to 168/169/170 feature
  surfaces; no autoharness change. Pass.
* **Dependency integrity (P-003)**: each hardening task depends on the base task
  it hardens (168.007→168.001/168.003; 168.008→168.007; 169.007→169.002/169.003;
  169.008→169.007; 170.008→170.003). Parents 168-F/169-F/170-F precede their
  children. The new hardening tasks are inserted into 149-S/150-S/151-S at
  shipment assembly (Stage Step 5.5); Ship MUST confirm each hardening task is
  present in its shipment AND sequenced after its base dependency before claiming
  149-S/150-S/151-S. Pass (membership added at assembly; Ship-gate confirmation
  required).

Constitution Check: pass

## Plan Hardening

* **ProposedAction**: make external-pin revocation authoritative over in-workspace
  status. **ActionRisk**: high (auth/authz trust decision). Mitigation: advisory-
  only in-workspace status + monotonic external revocation; negative test for
  reactivation.
* **ProposedAction**: persist signed envelope in event schema. **ActionRisk**:
  high (evidence-integrity / event-schema change). Mitigation: fail-closed on
  absent signed material; independent re-verification test; supersedes the
  forgeable unsigned-metadata path rather than adding a desyncable sidecar.
* **ProposedAction**: externally-protected nonce state. **ActionRisk**: high
  (anti-replay). Mitigation: justify-or-fold with an explicit residual-risk record
  if Option A infeasible; fail-closed on unavailable external state.
* **Trust-boundary invariant** (from origin plan): never treat the workspace as
  its own root of trust — every task binds to / verifies against the external
  root.
* **Rollback**: each task is additive to its feature; reverting a task restores
  the pre-hardening (reviewed-PASS) behavior.

## Verification

* `go test ./internal/core/... ./cmd/... -race` for the new negative/integration
  tests (169.008-T) and per-task unit coverage.
* `doctor` independent re-verification path exercised for 169.007-T.

## Traceability

Threads: `PRRT_kwDORzozKM6fvp_k` (3B661FCE), `PRRT_kwDORzozKM6fvp_X` (BFF76433),
`PRRT_kwDORzozKM6fvp_1` (BF18DA1D), `PRRT_kwDORzozKM6fvp_g` (4E210DB4); PR #425;
features 168-F/169-F/170-F; shipments 149-S/150-S/151-S.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 1. Personas: Security Lens / Security Reviewer (specialist), Correctness
+ Architecture Strategist (cross-cut). Coverage complete.

**Gate rationale**: one P1 finding → FAIL per rubric.

Findings:
* **P1 (Security)** — 170.008-T justify-or-fold permitted document-only closure
  of an authorization-replay / privilege-escalation vector, inconsistent with
  Decision 1 (which rejects document-only closure for the analogous 168
  reactivation vector). Not "genuinely resolved before Ship."
* **P2 (Security)** — 168.007-T claimed to bind role+status+validity externally,
  but 168.001-T external-pin is fingerprint-membership only; role/validity remain
  in-workspace-mutable.
* **P2 (Security)** — hardening tasks not yet in 149-S/150-S/151-S manifests;
  Ship-gate must confirm insertion + sequencing.
* **P3 (Security)** — 169.007-T protected reference should be digest-pinned and
  must supersede (not parallel) the forgeable metadata-presence branch.

Plan hardening required: yes; present. Remediation applied in this revision
(170.008-T Option A now REQUIRED before Ship or Ship blocked; new 168.008-T for
role/validity external binding or tracked residual; 169.007-T digest-pinned +
supersedes forgeable branch; Constitution Check P-003 note on assembly-time
membership + Ship-gate confirmation). Re-review below.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2. Personas: Security Lens / Security Reviewer (specialist, re-review).
Coverage complete. All four prior findings confirmed resolved at plan level:
(1) 170.008-T now REQUIRES Option A before Ship or blocks 151-S, fold gated behind
hard-reviewed non-exploitability proof (P1 resolved); (2) 168.007-T honestly
scoped to the status vector + new 168.008-T for role/validity external binding or
bounded tracked residual (P2 resolved); (3) Constitution Check states
assembly-time membership + Ship-gate confirmation (P2 resolved); (4) 169.007-T
digest-pinned + supersedes forgeable branch (P3 resolved). The attempt-2 P3
(168.008-T residual bound) was tightened to require a fail-closed/monotonic
mitigation + Ship-gate acknowledgement.

**Gate rationale**: no P0/P1/P2 remain. Plan hardening required: yes; present and
adequate. Security findings are genuinely resolved before Ship (hardening tasks
added to 149-S/150-S/151-S at assembly; Ship-gate must confirm sequencing).

<!-- plan-review-attempt: 2 -->
